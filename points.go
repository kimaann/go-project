package main

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ===== Aturan Bisnis Poin =====

const (
	// PointValidityMonths adalah masa berlaku poin sejak poin ditambahkan.
	PointValidityMonths = 6
	// PointsPerRupiah adalah nilai transaksi yang menghasilkan 1 poin.
	PointsPerRupiah = 1000
	// RupiahPerPoint adalah nilai tukar 1 poin saat ditukarkan.
	// Selaras dengan tabel penukaran di halaman /poin: 100 poin = Rp 5.000.
	RupiahPerPoint = 50
	// PointExpiryWarningDays adalah ambang "akan kedaluwarsa" pada laporan.
	PointExpiryWarningDays = 30
	// MinRedeemPoints adalah minimum penukaran poin dari halaman pengguna.
	MinRedeemPoints = 100
)

// ===== Error =====

var (
	ErrInvalidPointAmount  = fmt.Errorf("jumlah poin harus lebih dari nol")
	ErrInsufficientPoints  = fmt.Errorf("saldo poin tidak mencukupi")
	ErrRedemptionNotFound  = fmt.Errorf("data penukaran poin tidak ditemukan")
	ErrRedemptionNotActive = fmt.Errorf("penukaran poin sudah dibatalkan atau dipulihkan")
)

// ===== Model =====

// PointLot adalah satu batch poin yang masuk pada waktu tertentu.
// Setiap lot memiliki masa berlaku sendiri sehingga penukaran dapat
// memprioritaskan poin yang paling cepat kedaluwarsa (FIFO berdasarkan expiry).
type PointLot struct {
	ID              int64
	UserID          string
	OrderID         string
	PointsTotal     int
	PointsRemaining int
	EarnedAt        time.Time
	ExpiresAt       time.Time
	Status          string
	Note            string

	// Field turunan untuk kebutuhan tampilan.
	DaysLeft    int
	ValueRupiah int
	StatusLabel string
}

// PointAllocation mencatat lot mana saja yang dipakai pada satu penukaran.
type PointAllocation struct {
	LotID  int64 `json:"lot_id"`
	Points int   `json:"points"`
}

// PointRedemption adalah satu transaksi penukaran poin beserta rincian lot
// yang dikonsumsi. Rincian disimpan sebagai JSON agar pemulihan (restoration)
// dapat mengembalikan poin ke lot asalnya.
type PointRedemption struct {
	ID          int64
	UserID      string
	UserName    string
	Username    string
	OrderID     string
	Points      int
	RupiahValue int
	Allocations []PointAllocation
	Status      string
	VoucherCode string
	Note        string
	CreatedAt   time.Time
	ReversedAt  *time.Time
}

// PointLedgerEntry adalah catatan audit setiap pergerakan poin.
type PointLedgerEntry struct {
	ID           int64
	UserID       string
	UserName     string
	Username     string
	LotID        *int64
	RedemptionID *int64
	OrderID      string
	EntryType    string
	TypeLabel    string
	PointsDelta  int
	BalanceAfter int
	Note         string
	CreatedAt    time.Time
}

// ===== Skema Database =====

// initPointsSchema membuat tabel point_lots, point_redemptions, dan point_ledger
// bila belum ada, lalu memigrasikan saldo users.points lama ke sistem lot.
// Fungsi ini idempoten sehingga aman dipanggil setiap kali aplikasi start.
func initPointsSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS point_lots (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			user_id VARCHAR(64) NOT NULL,
			order_id VARCHAR(64) DEFAULT NULL,
			points_total INT NOT NULL,
			points_remaining INT NOT NULL,
			earned_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			note VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_point_lots_user_fifo (user_id, status, expires_at, id),
			KEY idx_point_lots_order (order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS point_redemptions (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			user_id VARCHAR(64) NOT NULL,
			order_id VARCHAR(64) DEFAULT NULL,
			points INT NOT NULL,
			rupiah_value BIGINT NOT NULL,
			allocations LONGTEXT DEFAULT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			voucher_code VARCHAR(40) DEFAULT NULL,
			note VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reversed_at DATETIME DEFAULT NULL,
			PRIMARY KEY (id),
			KEY idx_point_redemptions_user (user_id, created_at),
			KEY idx_point_redemptions_order (order_id, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS point_ledger (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			user_id VARCHAR(64) NOT NULL,
			lot_id BIGINT UNSIGNED DEFAULT NULL,
			redemption_id BIGINT UNSIGNED DEFAULT NULL,
			order_id VARCHAR(64) DEFAULT NULL,
			entry_type VARCHAR(16) NOT NULL,
			points_delta INT NOT NULL,
			balance_after INT NOT NULL,
			note VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_point_ledger_user (user_id, id),
			KEY idx_point_ledger_order (order_id, entry_type),
			KEY idx_point_ledger_lot (lot_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("gagal menyiapkan tabel poin: %w", err)
		}
	}

	return backfillLegacyPointLots(ctx)
}

// backfillLegacyPointLots memindahkan saldo users.points yang sudah ada
// sebelum fitur ini dipasang ke satu lot baru. Hanya dijalankan untuk user
// yang belum memiliki lot sama sekali, sehingga aman dan tidak menggandakan saldo.
func backfillLegacyPointLots(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.points
		FROM users u
		WHERE u.points > 0
		  AND NOT EXISTS (SELECT 1 FROM point_lots l WHERE l.user_id = u.id)`)
	if err != nil {
		return fmt.Errorf("gagal membaca saldo poin lama: %w", err)
	}

	type legacyBalance struct {
		userID string
		points int
	}
	var balances []legacyBalance
	for rows.Next() {
		var b legacyBalance
		if err := rows.Scan(&b.userID, &b.points); err != nil {
			rows.Close()
			return err
		}
		balances = append(balances, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now()
	for _, b := range balances {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO point_lots
				(user_id, order_id, points_total, points_remaining, earned_at, expires_at, status, note)
			VALUES (?, NULL, ?, ?, ?, ?, 'active', ?)`,
			b.userID, b.points, b.points, now, now.AddDate(0, PointValidityMonths, 0),
			"Migrasi saldo poin lama")
		if err != nil {
			tx.Rollback()
			return err
		}
		lotID, _ := res.LastInsertId()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO point_ledger
				(user_id, lot_id, entry_type, points_delta, balance_after, note)
			VALUES (?, ?, 'earn', ?, ?, ?)`,
			b.userID, lotID, b.points, b.points, "Migrasi saldo poin lama"); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// ===== Primitif Saldo =====

// activeBalanceTx menghitung saldo poin aktif (belum kedaluwarsa) dalam transaksi.
func activeBalanceTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (int, error) {
	var balance int
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(points_remaining), 0)
		FROM point_lots
		WHERE user_id = ? AND status = 'active' AND expires_at > ?`,
		userID, now).Scan(&balance)
	return balance, err
}

// expireLotsTx menandai lot yang sudah lewat masa berlaku dan mencatatnya ke ledger.
// Saldo aktif tidak berubah karena lot kedaluwarsa memang tidak pernah dihitung.
func expireLotsTx(ctx context.Context, tx *sql.Tx, userID string) (int, error) {
	now := time.Now()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, points_remaining
		FROM point_lots
		WHERE user_id = ? AND status = 'active' AND points_remaining > 0 AND expires_at <= ?
		FOR UPDATE`, userID, now)
	if err != nil {
		return 0, err
	}

	type expiringLot struct {
		id        int64
		remaining int
	}
	var lots []expiringLot
	for rows.Next() {
		var l expiringLot
		if err := rows.Scan(&l.id, &l.remaining); err != nil {
			rows.Close()
			return 0, err
		}
		lots = append(lots, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(lots) == 0 {
		return 0, nil
	}

	balance, err := activeBalanceTx(ctx, tx, userID, now)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, l := range lots {
		if _, err := tx.ExecContext(ctx, `UPDATE point_lots SET status = 'expired' WHERE id = ?`, l.id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO point_ledger
				(user_id, lot_id, entry_type, points_delta, balance_after, note)
			VALUES (?, ?, 'expire', ?, ?, ?)`,
			userID, l.id, -l.remaining, balance, "Poin kedaluwarsa"); err != nil {
			return 0, err
		}
		total += l.remaining
	}
	return total, nil
}

// syncLegacyPointsTx menyelaraskan kolom users.points (dipakai UI lama)
// dengan saldo aktif dari sistem lot.
func syncLegacyPointsTx(ctx context.Context, tx *sql.Tx, userID string, balance int) error {
	_, err := tx.ExecContext(ctx, `UPDATE users SET points = ? WHERE id = ?`, balance, userID)
	return err
}

// ActivePointBalance mengembalikan saldo poin aktif seorang pengguna.
// Lot yang sudah kedaluwarsa ikut diproses (lazy expiration) sebelum dihitung.
func ActivePointBalance(userID string) (int, error) {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := expireLotsTx(ctx, tx, userID); err != nil {
		return 0, err
	}
	balance, err := activeBalanceTx(ctx, tx, userID, time.Now())
	if err != nil {
		return 0, err
	}
	if err := syncLegacyPointsTx(ctx, tx, userID, balance); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return balance, nil
}

// ===== Helper =====

// newVoucherCode membuat kode voucher penukaran poin, mis. PTS-7F3A9C.
func newVoucherCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 6)
	if _, err := crand.Read(buf); err != nil {
		return fmt.Sprintf("PTS-%06d", time.Now().UnixNano()%1000000)
	}
	var b strings.Builder
	b.WriteString("PTS-")
	for _, v := range buf {
		b.WriteByte(alphabet[int(v)%len(alphabet)])
	}
	return b.String()
}

func ledgerTypeLabel(entryType string) string {
	switch entryType {
	case "earn":
		return "Poin Masuk"
	case "redeem":
		return "Penukaran"
	case "reverse":
		return "Pembalikan"
	case "restore":
		return "Restorasi"
	case "expire":
		return "Kedaluwarsa"
	case "reapply":
		return "Aktivasi Ulang"
	default:
		return "Penyesuaian"
	}
}

func lotStatusLabel(status string) string {
	switch status {
	case "active":
		return "Aktif"
	case "exhausted":
		return "Terpakai Habis"
	case "expired":
		return "Kedaluwarsa"
	default:
		return status
	}
}

func daysUntil(t time.Time) int {
	days := int(time.Until(t).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
