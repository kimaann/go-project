package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// ensurePointTestDB menyiapkan koneksi database untuk tes integrasi.
// Tes dilewati (skip) bila MySQL/MariaDB tidak tersedia.
func ensurePointTestDB(t *testing.T) {
	t.Helper()
	if db == nil {
		if err := initDB(); err != nil {
			t.Skipf("database tidak tersedia untuk tes poin: %v", err)
		}
	}
}

// newPointTestUser membuat user sementara dan membersihkan seluruh data
// poinnya setelah tes selesai.
func newPointTestUser(t *testing.T) string {
	t.Helper()
	ensurePointTestDB(t)

	// users.id pada database yang sudah ada bertipe varchar(36), jadi ID tes
	// harus muat dalam 36 karakter agar tidak terpotong.
	id := "t-" + strings.ReplaceAll(newRandomID(), "-", "")
	username := "test_" + id[len(id)-12:]
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO users (id, name, username, phone, email, password_hash, role, points, is_active)
		VALUES (?, 'Test Poin', ?, '0800000000', ?, 'x', 'user', 0, 1)`,
		id, username, username+"@example.com")
	if err != nil {
		t.Fatalf("gagal membuat user tes: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM point_ledger WHERE user_id = ?`, id)
		_, _ = db.ExecContext(ctx, `DELETE FROM point_redemptions WHERE user_id = ?`, id)
		_, _ = db.ExecContext(ctx, `DELETE FROM point_lots WHERE user_id = ?`, id)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	})
	return id
}

func insertPointTestLot(t *testing.T, userID string, points int, expiresAt time.Time) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(), `
		INSERT INTO point_lots
			(user_id, order_id, points_total, points_remaining, earned_at, expires_at, status, note)
		VALUES (?, NULL, ?, ?, ?, ?, 'active', 'lot tes')`,
		userID, points, points, time.Now().Add(-time.Hour), expiresAt)
	if err != nil {
		t.Fatalf("gagal membuat lot tes: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("gagal membaca id lot tes: %v", err)
	}
	return id
}

func mustPointBalance(t *testing.T, userID string) int {
	t.Helper()
	balance, err := ActivePointBalance(userID)
	if err != nil {
		t.Fatalf("gagal menghitung saldo poin: %v", err)
	}
	return balance
}

func TestAddPointsCreatesSixMonthLot(t *testing.T) {
	userID := newPointTestUser(t)

	lot, err := AddPoints(userID, 120, "ORD-TEST-ADD", "tes tambah")
	if err != nil {
		t.Fatalf("AddPoints gagal: %v", err)
	}
	if lot.PointsTotal != 120 || lot.PointsRemaining != 120 {
		t.Fatalf("lot tidak sesuai: total=%d remaining=%d", lot.PointsTotal, lot.PointsRemaining)
	}
	wantExpiry := lot.EarnedAt.AddDate(0, PointValidityMonths, 0)
	if !lot.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("masa berlaku salah: dapat %s, ingin %s", lot.ExpiresAt, wantExpiry)
	}
	if got := mustPointBalance(t, userID); got != 120 {
		t.Fatalf("saldo aktif = %d, ingin 120", got)
	}

	var legacy int
	if err := db.QueryRow(`SELECT points FROM users WHERE id = ?`, userID).Scan(&legacy); err != nil {
		t.Fatalf("gagal membaca users.points: %v", err)
	}
	if legacy != 120 {
		t.Fatalf("users.points = %d, ingin 120", legacy)
	}

	var earnEntries int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM point_ledger
		WHERE user_id = ? AND entry_type = 'earn'`, userID).Scan(&earnEntries); err != nil {
		t.Fatalf("gagal menghitung ledger: %v", err)
	}
	if earnEntries != 1 {
		t.Fatalf("entri earn = %d, ingin 1", earnEntries)
	}
}

func TestRedeemPointsUsesSoonestExpiryFirst(t *testing.T) {
	userID := newPointTestUser(t)
	now := time.Now()
	soonID := insertPointTestLot(t, userID, 100, now.AddDate(0, 0, 10))
	laterID := insertPointTestLot(t, userID, 100, now.AddDate(0, 0, 100))

	redemption, err := RedeemPoints(userID, 150, "ORD-TEST-FIFO", "tes fifo")
	if err != nil {
		t.Fatalf("RedeemPoints gagal: %v", err)
	}
	if len(redemption.Allocations) != 2 {
		t.Fatalf("jumlah alokasi = %d, ingin 2", len(redemption.Allocations))
	}
	if redemption.Allocations[0].LotID != soonID || redemption.Allocations[0].Points != 100 {
		t.Fatalf("alokasi pertama salah: %+v", redemption.Allocations[0])
	}
	if redemption.Allocations[1].LotID != laterID || redemption.Allocations[1].Points != 50 {
		t.Fatalf("alokasi kedua salah: %+v", redemption.Allocations[1])
	}

	var soonRemaining, laterRemaining int
	if err := db.QueryRow(`SELECT points_remaining FROM point_lots WHERE id = ?`, soonID).Scan(&soonRemaining); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT points_remaining FROM point_lots WHERE id = ?`, laterID).Scan(&laterRemaining); err != nil {
		t.Fatal(err)
	}
	if soonRemaining != 0 || laterRemaining != 50 {
		t.Fatalf("sisa lot salah: soon=%d later=%d, ingin 0 dan 50", soonRemaining, laterRemaining)
	}
	if got := mustPointBalance(t, userID); got != 50 {
		t.Fatalf("saldo aktif = %d, ingin 50", got)
	}
}

func TestExpiredLotsExcludedFromBalance(t *testing.T) {
	userID := newPointTestUser(t)
	expiredID := insertPointTestLot(t, userID, 40, time.Now().Add(-24*time.Hour))
	insertPointTestLot(t, userID, 60, time.Now().AddDate(0, 0, 30))

	if got := mustPointBalance(t, userID); got != 60 {
		t.Fatalf("saldo aktif = %d, ingin 60 (lot kedaluwarsa tidak dihitung)", got)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM point_lots WHERE id = ?`, expiredID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "expired" {
		t.Fatalf("status lot = %s, ingin expired", status)
	}

	var expireEntries int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM point_ledger
		WHERE user_id = ? AND entry_type = 'expire'`, userID).Scan(&expireEntries); err != nil {
		t.Fatal(err)
	}
	if expireEntries != 1 {
		t.Fatalf("entri expire = %d, ingin 1", expireEntries)
	}
}

func TestRestoreRedemptionReturnsPoints(t *testing.T) {
	userID := newPointTestUser(t)
	if _, err := AddPoints(userID, 120, "ORD-TEST-RESTORE", "tes"); err != nil {
		t.Fatalf("AddPoints gagal: %v", err)
	}
	redemption, err := RedeemPoints(userID, 70, "ORD-TEST-RESTORE", "tes")
	if err != nil {
		t.Fatalf("RedeemPoints gagal: %v", err)
	}
	if got := mustPointBalance(t, userID); got != 50 {
		t.Fatalf("saldo setelah tukar = %d, ingin 50", got)
	}

	restored, total, err := RestoreRedemption(redemption.ID, "pesanan dibatalkan")
	if err != nil {
		t.Fatalf("RestoreRedemption gagal: %v", err)
	}
	if total != 70 || restored.Status != "restored" {
		t.Fatalf("restorasi tidak sesuai: total=%d status=%s", total, restored.Status)
	}
	if got := mustPointBalance(t, userID); got != 120 {
		t.Fatalf("saldo setelah restorasi = %d, ingin 120", got)
	}

	if _, _, err := RestoreRedemption(redemption.ID, "restorasi ganda"); !errors.Is(err, ErrRedemptionNotActive) {
		t.Fatalf("restorasi kedua harus gagal dengan ErrRedemptionNotActive, dapat: %v", err)
	}
}

func TestReversePointsDeductsFIFOAndReportsShortfall(t *testing.T) {
	userID := newPointTestUser(t)
	if _, err := AddPoints(userID, 100, "ORD-TEST-REV", "tes"); err != nil {
		t.Fatalf("AddPoints gagal: %v", err)
	}

	deducted, shortfall, err := ReversePoints(userID, 30, "ORD-TEST-REV", "tes")
	if err != nil {
		t.Fatalf("ReversePoints gagal: %v", err)
	}
	if deducted != 30 || shortfall != 0 {
		t.Fatalf("pembalikan salah: deducted=%d shortfall=%d", deducted, shortfall)
	}
	if got := mustPointBalance(t, userID); got != 70 {
		t.Fatalf("saldo setelah pembalikan = %d, ingin 70", got)
	}

	deducted, shortfall, err = ReversePoints(userID, 100, "ORD-TEST-REV", "tes")
	if err != nil {
		t.Fatalf("ReversePoints kedua gagal: %v", err)
	}
	if deducted != 70 || shortfall != 30 {
		t.Fatalf("pembalikan kedua salah: deducted=%d shortfall=%d, ingin 70/30", deducted, shortfall)
	}
	if got := mustPointBalance(t, userID); got != 0 {
		t.Fatalf("saldo akhir = %d, ingin 0", got)
	}
}

func TestOrderCancellationAndReactivation(t *testing.T) {
	userID := newPointTestUser(t)
	orderID := "ORD-TEST-CANCEL-" + userID[len(userID)-6:]

	if _, err := AddPoints(userID, 100, orderID, "tes"); err != nil {
		t.Fatalf("AddPoints gagal: %v", err)
	}
	if _, err := RedeemPoints(userID, 40, orderID, "tes"); err != nil {
		t.Fatalf("RedeemPoints gagal: %v", err)
	}
	if got := mustPointBalance(t, userID); got != 60 {
		t.Fatalf("saldo sebelum batal = %d, ingin 60", got)
	}

	if err := ApplyOrderCancellation(orderID); err != nil {
		t.Fatalf("ApplyOrderCancellation gagal: %v", err)
	}
	if got := mustPointBalance(t, userID); got != 0 {
		t.Fatalf("saldo setelah batal = %d, ingin 0", got)
	}
	var status string
	if err := db.QueryRow(`
		SELECT status FROM point_redemptions
		WHERE order_id = ? ORDER BY id DESC LIMIT 1`, orderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "restored" {
		t.Fatalf("status penukaran = %s, ingin restored", status)
	}

	if err := ApplyOrderReactivation(orderID); err != nil {
		t.Fatalf("ApplyOrderReactivation gagal: %v", err)
	}
	if got := mustPointBalance(t, userID); got != 60 {
		t.Fatalf("saldo setelah aktif kembali = %d, ingin 60", got)
	}
}

func TestPointsReportIncludesUser(t *testing.T) {
	userID := newPointTestUser(t)
	if _, err := AddPoints(userID, 200, "", "tes report"); err != nil {
		t.Fatalf("AddPoints gagal: %v", err)
	}

	report, err := BuildPointsReport()
	if err != nil {
		t.Fatalf("BuildPointsReport gagal: %v", err)
	}

	var found *UserPointSummary
	for i := range report.Users {
		if report.Users[i].UserID == userID {
			found = &report.Users[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("user tes tidak ditemukan di laporan")
	}
	if found.ActivePoints != 200 {
		t.Fatalf("poin aktif di laporan = %d, ingin 200", found.ActivePoints)
	}
	if found.RupiahValue != 200*RupiahPerPoint {
		t.Fatalf("nilai rupiah di laporan = %d, ingin %d", found.RupiahValue, 200*RupiahPerPoint)
	}
}
