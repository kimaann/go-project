package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ===== Penambahan Poin =====

// AddPoints menambahkan poin baru sebagai satu lot dengan masa berlaku
// PointValidityMonths (6 bulan) sejak saat ini.
func AddPoints(userID string, points int, orderID, note string) (*PointLot, error) {
	if points <= 0 {
		return nil, ErrInvalidPointAmount
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	lot, err := addPointsTx(ctx, tx, userID, points, orderID, note)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return lot, nil
}

func addPointsTx(ctx context.Context, tx *sql.Tx, userID string, points int, orderID, note string) (*PointLot, error) {
	if _, err := expireLotsTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.AddDate(0, PointValidityMonths, 0)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO point_lots
			(user_id, order_id, points_total, points_remaining, earned_at, expires_at, status, note)
		VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, 'active', ?)`,
		userID, orderID, points, points, now, expiresAt, note)
	if err != nil {
		return nil, err
	}
	lotID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	balance, err := activeBalanceTx(ctx, tx, userID, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO point_ledger
			(user_id, lot_id, order_id, entry_type, points_delta, balance_after, note)
		VALUES (?, ?, NULLIF(?, ''), 'earn', ?, ?, ?)`,
		userID, lotID, orderID, points, balance, note); err != nil {
		return nil, err
	}
	if err := syncLegacyPointsTx(ctx, tx, userID, balance); err != nil {
		return nil, err
	}

	return &PointLot{
		ID:              lotID,
		UserID:          userID,
		OrderID:         orderID,
		PointsTotal:     points,
		PointsRemaining: points,
		EarnedAt:        now,
		ExpiresAt:       expiresAt,
		Status:          "active",
		Note:            note,
		DaysLeft:        daysUntil(expiresAt),
		ValueRupiah:     points * RupiahPerPoint,
		StatusLabel:     lotStatusLabel("active"),
	}, nil
}

// ===== Penukaran Poin (FIFO) =====

// RedeemPoints menukar poin dengan memprioritaskan lot yang paling cepat
// kedaluwarsa (ORDER BY expires_at ASC, id ASC).
func RedeemPoints(userID string, points int, orderID, note string) (*PointRedemption, error) {
	if points <= 0 {
		return nil, ErrInvalidPointAmount
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	redemption, err := redeemPointsTx(ctx, tx, userID, points, orderID, note, newVoucherCode())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return redemption, nil
}

func redeemPointsTx(ctx context.Context, tx *sql.Tx, userID string, points int, orderID, note, voucher string) (*PointRedemption, error) {
	if _, err := expireLotsTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	now := time.Now()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, points_remaining
		FROM point_lots
		WHERE user_id = ? AND status = 'active' AND expires_at > ? AND points_remaining > 0
		ORDER BY expires_at ASC, id ASC
		FOR UPDATE`, userID, now)
	if err != nil {
		return nil, err
	}

	type availableLot struct {
		id        int64
		remaining int
	}
	var lots []availableLot
	available := 0
	for rows.Next() {
		var l availableLot
		if err := rows.Scan(&l.id, &l.remaining); err != nil {
			rows.Close()
			return nil, err
		}
		lots = append(lots, l)
		available += l.remaining
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if available < points {
		return nil, fmt.Errorf("%w: saldo aktif %d poin, dibutuhkan %d poin", ErrInsufficientPoints, available, points)
	}

	allocations := make([]PointAllocation, 0, len(lots))
	remaining := points
	for _, l := range lots {
		if remaining == 0 {
			break
		}
		take := l.remaining
		if take > remaining {
			take = remaining
		}
		newRemaining := l.remaining - take
		status := "active"
		if newRemaining == 0 {
			status = "exhausted"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE point_lots SET points_remaining = ?, status = ? WHERE id = ?`,
			newRemaining, status, l.id); err != nil {
			return nil, err
		}
		allocations = append(allocations, PointAllocation{LotID: l.id, Points: take})
		remaining -= take
	}

	allocationJSON, err := json.Marshal(allocations)
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO point_redemptions
			(user_id, order_id, points, rupiah_value, allocations, status, voucher_code, note)
		VALUES (?, NULLIF(?, ''), ?, ?, ?, 'active', NULLIF(?, ''), ?)`,
		userID, orderID, points, points*RupiahPerPoint, string(allocationJSON), voucher, note)
	if err != nil {
		return nil, err
	}
	redemptionID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	balance := available
	for _, allocation := range allocations {
		balance -= allocation.Points
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO point_ledger
				(user_id, lot_id, redemption_id, order_id, entry_type, points_delta, balance_after, note)
			VALUES (?, ?, ?, NULLIF(?, ''), 'redeem', ?, ?, ?)`,
			userID, allocation.LotID, redemptionID, orderID, -allocation.Points, balance, note); err != nil {
			return nil, err
		}
	}
	if err := syncLegacyPointsTx(ctx, tx, userID, balance); err != nil {
		return nil, err
	}

	return &PointRedemption{
		ID:          redemptionID,
		UserID:      userID,
		OrderID:     orderID,
		Points:      points,
		RupiahValue: points * RupiahPerPoint,
		Allocations: allocations,
		Status:      "active",
		VoucherCode: voucher,
		Note:        note,
		CreatedAt:   now,
	}, nil
}

// ===== Pembalikan (Deduction) =====

// ReversePoints menarik poin kembali secara FIFO. Dipakai saat transaksi
// yang menghasilkan poin dibatalkan. Bila saldo tidak cukup, seluruh saldo
// aktif ditarik dan kekurangannya dilaporkan (shortfall).
func ReversePoints(userID string, points int, orderID, note string) (deducted int, shortfall int, err error) {
	if points <= 0 {
		return 0, 0, ErrInvalidPointAmount
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	deducted, shortfall, err = reversePointsTx(ctx, tx, userID, points, orderID, note)
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return deducted, shortfall, nil
}

func reversePointsTx(ctx context.Context, tx *sql.Tx, userID string, points int, orderID, note string) (int, int, error) {
	if _, err := expireLotsTx(ctx, tx, userID); err != nil {
		return 0, 0, err
	}

	now := time.Now()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, points_remaining
		FROM point_lots
		WHERE user_id = ? AND status = 'active' AND expires_at > ? AND points_remaining > 0
		ORDER BY expires_at ASC, id ASC
		FOR UPDATE`, userID, now)
	if err != nil {
		return 0, 0, err
	}

	type availableLot struct {
		id        int64
		remaining int
	}
	var lots []availableLot
	available := 0
	for rows.Next() {
		var l availableLot
		if err := rows.Scan(&l.id, &l.remaining); err != nil {
			rows.Close()
			return 0, 0, err
		}
		lots = append(lots, l)
		available += l.remaining
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	deducted := points
	if available < points {
		deducted = available
	}
	shortfall := points - deducted

	balance := available
	remaining := deducted
	for _, l := range lots {
		if remaining == 0 {
			break
		}
		take := l.remaining
		if take > remaining {
			take = remaining
		}
		newRemaining := l.remaining - take
		status := "active"
		if newRemaining == 0 {
			status = "exhausted"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE point_lots SET points_remaining = ?, status = ? WHERE id = ?`,
			newRemaining, status, l.id); err != nil {
			return 0, 0, err
		}
		balance -= take
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO point_ledger
				(user_id, lot_id, order_id, entry_type, points_delta, balance_after, note)
			VALUES (?, ?, NULLIF(?, ''), 'reverse', ?, ?, ?)`,
			userID, l.id, orderID, -take, balance, note); err != nil {
			return 0, 0, err
		}
		remaining -= take
	}

	if shortfall > 0 {
		shortNote := fmt.Sprintf("%s (kekurangan %d poin tidak dapat ditarik)", note, shortfall)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO point_ledger
				(user_id, order_id, entry_type, points_delta, balance_after, note)
			VALUES (?, NULLIF(?, ''), 'reverse', 0, ?, ?)`,
			userID, orderID, balance, shortNote); err != nil {
			return 0, 0, err
		}
	}
	if err := syncLegacyPointsTx(ctx, tx, userID, balance); err != nil {
		return 0, 0, err
	}
	return deducted, shortfall, nil
}

// ReverseOrderEarning menarik poin yang pernah diberikan untuk sebuah pesanan.
// Hanya dilakukan bila siklus terakhir pesanan tersebut memang masih "earn".
func ReverseOrderEarning(orderID, userID string) (int, int, error) {
	ctx := context.Background()
	var lastType string
	err := db.QueryRowContext(ctx, `
		SELECT entry_type FROM point_ledger
		WHERE order_id = ? AND entry_type IN ('earn', 'reverse', 'reapply')
		ORDER BY id DESC LIMIT 1`, orderID).Scan(&lastType)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if lastType == "reverse" {
		return 0, 0, nil
	}

	var lastReversalID int64
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(id), 0) FROM point_ledger
		WHERE order_id = ? AND entry_type = 'reverse'`, orderID).Scan(&lastReversalID); err != nil {
		return 0, 0, err
	}
	var awarded int
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(points_delta), 0) FROM point_ledger
		WHERE order_id = ? AND user_id = ? AND entry_type IN ('earn', 'reapply') AND id > ?`,
		orderID, userID, lastReversalID).Scan(&awarded); err != nil {
		return 0, 0, err
	}
	if awarded <= 0 {
		return 0, 0, nil
	}
	return ReversePoints(userID, awarded, orderID, "Pembatalan pesanan "+orderID)
}

// ===== Restorasi =====

// RestoreRedemption mengembalikan poin yang sudah ditukar ke lot asalnya.
// Jika lot asal sudah kedaluwarsa, dibuat lot pengganti dengan masa berlaku baru.
func RestoreRedemption(redemptionID int64, note string) (*PointRedemption, int, error) {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	redemption, restored, err := restoreRedemptionTx(ctx, tx, redemptionID, note)
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return redemption, restored, nil
}

func restoreRedemptionTx(ctx context.Context, tx *sql.Tx, redemptionID int64, note string) (*PointRedemption, int, error) {
	var (
		r            PointRedemption
		orderID      sql.NullString
		voucher      sql.NullString
		existingNote sql.NullString
		allocationJS sql.NullString
		reversedAt   sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `
		SELECT id, user_id, order_id, points, rupiah_value, allocations, status,
		       voucher_code, note, created_at, reversed_at
		FROM point_redemptions WHERE id = ? FOR UPDATE`, redemptionID).Scan(
		&r.ID, &r.UserID, &orderID, &r.Points, &r.RupiahValue, &allocationJS,
		&r.Status, &voucher, &existingNote, &r.CreatedAt, &reversedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrRedemptionNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	if r.Status != "active" {
		return nil, 0, fmt.Errorf("%w (status: %s)", ErrRedemptionNotActive, r.Status)
	}

	r.OrderID = orderID.String
	r.VoucherCode = voucher.String
	if allocationJS.Valid && allocationJS.String != "" {
		if err := json.Unmarshal([]byte(allocationJS.String), &r.Allocations); err != nil {
			return nil, 0, err
		}
	}
	if len(r.Allocations) == 0 {
		r.Allocations = []PointAllocation{{Points: r.Points}}
	}

	now := time.Now()
	balance, err := activeBalanceTx(ctx, tx, r.UserID, now)
	if err != nil {
		return nil, 0, err
	}

	restored := 0
	for _, allocation := range r.Allocations {
		targetLot := allocation.LotID
		restoredToOriginal := false

		if allocation.LotID > 0 {
			var (
				status    string
				expiresAt time.Time
				remaining int
			)
			err := tx.QueryRowContext(ctx, `
				SELECT status, expires_at, points_remaining
				FROM point_lots WHERE id = ? FOR UPDATE`, allocation.LotID).Scan(
				&status, &expiresAt, &remaining)
			switch {
			case err == nil && status != "expired" && expiresAt.After(now):
				if _, err := tx.ExecContext(ctx, `
					UPDATE point_lots SET points_remaining = ?, status = 'active' WHERE id = ?`,
					remaining+allocation.Points, allocation.LotID); err != nil {
					return nil, 0, err
				}
				restoredToOriginal = true
			case err != nil && !errors.Is(err, sql.ErrNoRows):
				return nil, 0, err
			}
		}

		if !restoredToOriginal {
			expiresAt := now.AddDate(0, PointValidityMonths, 0)
			res, err := tx.ExecContext(ctx, `
				INSERT INTO point_lots
					(user_id, order_id, points_total, points_remaining, earned_at, expires_at, status, note)
				VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, 'active', ?)`,
				r.UserID, r.OrderID, allocation.Points, allocation.Points, now, expiresAt,
				fmt.Sprintf("Restorasi penukaran poin #%d", r.ID))
			if err != nil {
				return nil, 0, err
			}
			targetLot, err = res.LastInsertId()
			if err != nil {
				return nil, 0, err
			}
		}

		balance += allocation.Points
		restored += allocation.Points
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO point_ledger
				(user_id, lot_id, redemption_id, order_id, entry_type, points_delta, balance_after, note)
			VALUES (?, ?, ?, NULLIF(?, ''), 'restore', ?, ?, ?)`,
			r.UserID, targetLot, r.ID, r.OrderID, allocation.Points, balance, note); err != nil {
			return nil, 0, err
		}
	}

	combinedNote := strings.Trim(strings.Join([]string{existingNote.String, note}, " | "), " |")
	if _, err := tx.ExecContext(ctx, `
		UPDATE point_redemptions SET status = 'restored', reversed_at = ?, note = ? WHERE id = ?`,
		now, combinedNote, r.ID); err != nil {
		return nil, 0, err
	}
	if err := syncLegacyPointsTx(ctx, tx, r.UserID, balance); err != nil {
		return nil, 0, err
	}

	r.Status = "restored"
	r.Note = combinedNote
	r.ReversedAt = &now
	return &r, restored, nil
}

// RestoreRedemptionsForOrder memulihkan seluruh penukaran aktif milik pesanan.
func RestoreRedemptionsForOrder(orderID string) error {
	rows, err := db.QueryContext(context.Background(), `
		SELECT id FROM point_redemptions
		WHERE order_id = ? AND status = 'active' ORDER BY id ASC`, orderID)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if _, _, err := RestoreRedemption(id, "Pembatalan pesanan "+orderID); err != nil {
			return err
		}
	}
	return nil
}

// ===== Siklus Status Pesanan =====

// ApplyOrderCancellation menarik poin yang diberikan oleh pesanan dan
// memulihkan poin yang sempat ditukarkan pada pesanan tersebut.
func ApplyOrderCancellation(orderID string) error {
	ctx := context.Background()

	// Pulihkan poin yang sempat ditukar pada pesanan ini terlebih dahulu,
	// agar saldo cukup ketika poin yang dihasilkan pesanan ditarik kembali.
	if err := RestoreRedemptionsForOrder(orderID); err != nil {
		return err
	}

	var userID string
	err := db.QueryRowContext(ctx, `
		SELECT user_id FROM point_ledger
		WHERE order_id = ? AND entry_type IN ('earn', 'reapply')
		ORDER BY id DESC LIMIT 1`, orderID).Scan(&userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if userID != "" {
		if _, _, err := ReverseOrderEarning(orderID, userID); err != nil {
			return err
		}
	}
	return nil
}

// ApplyOrderReactivation mengembalikan kondisi poin saat pesanan yang
// sebelumnya dibatalkan diaktifkan kembali.
func ApplyOrderReactivation(orderID string) error {
	ctx := context.Background()
	var lastType string
	err := db.QueryRowContext(ctx, `
		SELECT entry_type FROM point_ledger
		WHERE order_id = ? AND entry_type IN ('earn', 'reverse', 'reapply')
		ORDER BY id DESC LIMIT 1`, orderID).Scan(&lastType)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if lastType == "reverse" {
		var userID string
		if err := db.QueryRowContext(ctx, `
			SELECT user_id FROM point_ledger
			WHERE order_id = ? AND entry_type = 'reverse'
			ORDER BY id DESC LIMIT 1`, orderID).Scan(&userID); err == nil {
			var lastAwardID int64
			if err := db.QueryRowContext(ctx, `
				SELECT COALESCE(MAX(id), 0) FROM point_ledger
				WHERE order_id = ? AND entry_type IN ('earn', 'reapply')`, orderID).Scan(&lastAwardID); err != nil {
				return err
			}
			var reversed int
			if err := db.QueryRowContext(ctx, `
				SELECT COALESCE(-SUM(points_delta), 0) FROM point_ledger
				WHERE order_id = ? AND entry_type = 'reverse' AND id > ?`,
				orderID, lastAwardID).Scan(&reversed); err != nil {
				return err
			}
			if reversed > 0 {
				if _, err := AddPoints(userID, reversed, orderID, "Pengaktifan ulang pesanan "+orderID); err != nil {
					return err
				}
			}
		}
	}

	type restoredRedemption struct {
		id     int64
		userID string
		points int
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, points FROM point_redemptions
		WHERE order_id = ? AND status = 'restored' ORDER BY id ASC`, orderID)
	if err != nil {
		return err
	}
	var pending []restoredRedemption
	for rows.Next() {
		var r restoredRedemption
		if err := rows.Scan(&r.id, &r.userID, &r.points); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range pending {
		if _, err := RedeemPoints(r.userID, r.points, orderID, "Pengaktifan ulang pesanan "+orderID); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			UPDATE point_redemptions SET status = 'reapplied' WHERE id = ?`, r.id); err != nil {
			return err
		}
	}
	return nil
}

// ===== Detail Pengguna =====

// UserPointSummary adalah ringkasan poin satu pengguna.
type UserPointSummary struct {
	UserID           string
	Name             string
	Username         string
	Email            string
	ActivePoints     int
	RupiahValue      int
	ExpiringSoon     int
	ExpiringRupiah   int
	NextExpiry       *time.Time
	NextExpiryText   string
	LifetimeEarned   int
	LifetimeRedeemed int
	LifetimeExpired  int
	ActiveLots       int
}

// GetUserPointDetail mengambil ringkasan, daftar lot, dan ledger terbaru
// untuk satu pengguna.
func GetUserPointDetail(userID string) (*UserPointSummary, []PointLot, []PointLedgerEntry, error) {
	if _, err := ActivePointBalance(userID); err != nil {
		return nil, nil, nil, err
	}
	ctx := context.Background()
	now := time.Now()

	summary := &UserPointSummary{UserID: userID}
	if err := db.QueryRowContext(ctx, `
		SELECT name, username, COALESCE(email, '') FROM users WHERE id = ?`, userID).Scan(
		&summary.Name, &summary.Username, &summary.Email); err != nil {
		return nil, nil, nil, err
	}

	var nextExpiry sql.NullTime
	if err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'active' AND expires_at > ? THEN points_remaining ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'active' AND expires_at > ? AND expires_at <= ? THEN points_remaining ELSE 0 END), 0),
			MIN(CASE WHEN status = 'active' AND expires_at > ? THEN expires_at END),
			COALESCE(SUM(points_total), 0),
			COALESCE(SUM(CASE WHEN status = 'expired' THEN points_remaining ELSE 0 END), 0),
			COUNT(CASE WHEN status = 'active' AND expires_at > ? THEN 1 END)
		FROM point_lots WHERE user_id = ?`,
		now, now, now.AddDate(0, 0, PointExpiryWarningDays), now, now, userID).Scan(
		&summary.ActivePoints, &summary.ExpiringSoon, &nextExpiry,
		&summary.LifetimeEarned, &summary.LifetimeExpired, &summary.ActiveLots); err != nil {
		return nil, nil, nil, err
	}
	if nextExpiry.Valid {
		summary.NextExpiry = &nextExpiry.Time
		summary.NextExpiryText = nextExpiry.Time.Format("02 Jan 2006")
	}

	// Total poin masuk bersih (earn + reapply + reverse) dari ledger,
	// sehingga poin yang sudah dibalik tidak dihitung dua kali.
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(points_delta), 0) FROM point_ledger
		WHERE user_id = ? AND entry_type IN ('earn', 'reapply', 'reverse')`, userID).Scan(
		&summary.LifetimeEarned); err != nil {
		return nil, nil, nil, err
	}

	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(points), 0) FROM point_redemptions
		WHERE user_id = ? AND status IN ('active', 'restored', 'reapplied')`, userID).Scan(
		&summary.LifetimeRedeemed); err != nil {
		return nil, nil, nil, err
	}
	summary.RupiahValue = summary.ActivePoints * RupiahPerPoint
	summary.ExpiringRupiah = summary.ExpiringSoon * RupiahPerPoint

	lots, err := queryPointLots(ctx, `WHERE user_id = ? ORDER BY expires_at ASC, id ASC`, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	ledger, err := queryPointLedger(ctx, `WHERE user_id = ? ORDER BY id DESC LIMIT 50`, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	return summary, lots, ledger, nil
}

// ===== Query Helper =====

func queryPointLots(ctx context.Context, where string, args ...any) ([]PointLot, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, COALESCE(order_id, ''), points_total, points_remaining,
		       earned_at, expires_at, status, note
		FROM point_lots `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lots := make([]PointLot, 0)
	for rows.Next() {
		var l PointLot
		if err := rows.Scan(&l.ID, &l.UserID, &l.OrderID, &l.PointsTotal, &l.PointsRemaining,
			&l.EarnedAt, &l.ExpiresAt, &l.Status, &l.Note); err != nil {
			return nil, err
		}
		l.DaysLeft = daysUntil(l.ExpiresAt)
		l.ValueRupiah = l.PointsRemaining * RupiahPerPoint
		l.StatusLabel = lotStatusLabel(l.Status)
		lots = append(lots, l)
	}
	return lots, rows.Err()
}

func queryPointLedger(ctx context.Context, where string, args ...any) ([]PointLedgerEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT g.id, g.user_id, COALESCE(u.name, ''), COALESCE(u.username, ''),
		       g.lot_id, g.redemption_id, COALESCE(g.order_id, ''), g.entry_type,
		       g.points_delta, g.balance_after, g.note, g.created_at
		FROM point_ledger g
		LEFT JOIN users u ON u.id = g.user_id
		`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]PointLedgerEntry, 0)
	for rows.Next() {
		var (
			e            PointLedgerEntry
			lotID        sql.NullInt64
			redemptionID sql.NullInt64
		)
		if err := rows.Scan(&e.ID, &e.UserID, &e.UserName, &e.Username,
			&lotID, &redemptionID, &e.OrderID, &e.EntryType,
			&e.PointsDelta, &e.BalanceAfter, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		if lotID.Valid {
			id := lotID.Int64
			e.LotID = &id
		}
		if redemptionID.Valid {
			id := redemptionID.Int64
			e.RedemptionID = &id
		}
		e.TypeLabel = ledgerTypeLabel(e.EntryType)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
