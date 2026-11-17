package main

import (
	"context"
	"database/sql"
	"time"
)

// PointLotView adalah baris lot aktif pada laporan.
type PointLotView struct {
	PointLot
	UserName string
	Username string
}

// PointReport adalah data lengkap laporan poin aktif.
type PointReport struct {
	GeneratedAt       time.Time
	RupiahPerPoint    int
	ValidityMonths    int
	ExpiryWarningDays int

	TotalActivePoints int
	TotalRupiahValue  int
	TotalExpiringSoon int
	TotalEarned       int
	TotalRedeemed     int
	TotalExpired      int

	Users       []UserPointSummary
	Lots        []PointLotView
	Ledger      []PointLedgerEntry
	Redemptions []PointRedemption
}

// BuildPointsReport menyusun laporan poin yang masih berlaku beserta
// nilai rupiahnya, rincian per user, lot, penukaran, dan ledger terbaru.
func BuildPointsReport() (*PointReport, error) {
	ctx := context.Background()
	now := time.Now()
	report := &PointReport{
		GeneratedAt:       now,
		RupiahPerPoint:    RupiahPerPoint,
		ValidityMonths:    PointValidityMonths,
		ExpiryWarningDays: PointExpiryWarningDays,
		Users:             make([]UserPointSummary, 0),
		Lots:              make([]PointLotView, 0),
		Ledger:            make([]PointLedgerEntry, 0),
		Redemptions:       make([]PointRedemption, 0),
	}

	// Ringkasan per pengguna.
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.name, u.username, COALESCE(u.email, ''),
		       COALESCE(SUM(CASE WHEN l.status = 'active' AND l.expires_at > ? THEN l.points_remaining ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN l.status = 'active' AND l.expires_at > ? AND l.expires_at <= ? THEN l.points_remaining ELSE 0 END), 0),
		       MIN(CASE WHEN l.status = 'active' AND l.expires_at > ? THEN l.expires_at END),
		       COALESCE(SUM(l.points_total), 0),
		       COALESCE(SUM(CASE WHEN l.status = 'expired' THEN l.points_remaining ELSE 0 END), 0),
		       COUNT(CASE WHEN l.status = 'active' AND l.expires_at > ? THEN 1 END)
		FROM users u
		LEFT JOIN point_lots l ON l.user_id = u.id
		GROUP BY u.id, u.name, u.username, u.email
		ORDER BY u.name ASC`,
		now, now, now.AddDate(0, 0, PointExpiryWarningDays), now, now)
	if err != nil {
		return nil, err
	}

	type summaryRow struct {
		summary    UserPointSummary
		nextExpiry sql.NullTime
	}
	var summaries []summaryRow
	for rows.Next() {
		var s summaryRow
		if err := rows.Scan(&s.summary.UserID, &s.summary.Name, &s.summary.Username,
			&s.summary.Email, &s.summary.ActivePoints, &s.summary.ExpiringSoon,
			&s.nextExpiry, &s.summary.LifetimeEarned, &s.summary.LifetimeExpired,
			&s.summary.ActiveLots); err != nil {
			rows.Close()
			return nil, err
		}
		if s.nextExpiry.Valid {
			s.summary.NextExpiry = &s.nextExpiry.Time
			s.summary.NextExpiryText = s.nextExpiry.Time.Format("02 Jan 2006")
		}
		summaries = append(summaries, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Total penukaran per pengguna.
	redeemedByUser := map[string]int{}
	redeemRows, err := db.QueryContext(ctx, `
		SELECT user_id, COALESCE(SUM(points), 0)
		FROM point_redemptions
		WHERE status IN ('active', 'restored', 'reapplied')
		GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	for redeemRows.Next() {
		var userID string
		var total int
		if err := redeemRows.Scan(&userID, &total); err != nil {
			redeemRows.Close()
			return nil, err
		}
		redeemedByUser[userID] = total
	}
	redeemRows.Close()
	if err := redeemRows.Err(); err != nil {
		return nil, err
	}

	// Total poin masuk bersih per pengguna (earn + reapply + reverse).
	earnedByUser := map[string]int{}
	earnRows, err := db.QueryContext(ctx, `
		SELECT user_id, COALESCE(SUM(points_delta), 0)
		FROM point_ledger
		WHERE entry_type IN ('earn', 'reapply', 'reverse')
		GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	for earnRows.Next() {
		var userID string
		var total int
		if err := earnRows.Scan(&userID, &total); err != nil {
			earnRows.Close()
			return nil, err
		}
		earnedByUser[userID] = total
	}
	earnRows.Close()
	if err := earnRows.Err(); err != nil {
		return nil, err
	}

	for _, s := range summaries {
		if earned, ok := earnedByUser[s.summary.UserID]; ok {
			s.summary.LifetimeEarned = earned
		}
		s.summary.LifetimeRedeemed = redeemedByUser[s.summary.UserID]
		s.summary.RupiahValue = s.summary.ActivePoints * RupiahPerPoint
		s.summary.ExpiringRupiah = s.summary.ExpiringSoon * RupiahPerPoint

		report.TotalActivePoints += s.summary.ActivePoints
		report.TotalExpiringSoon += s.summary.ExpiringSoon
		report.TotalEarned += s.summary.LifetimeEarned
		report.TotalRedeemed += s.summary.LifetimeRedeemed
		report.TotalExpired += s.summary.LifetimeExpired
		report.Users = append(report.Users, s.summary)
	}
	report.TotalRupiahValue = report.TotalActivePoints * RupiahPerPoint

	// Daftar lot yang masih aktif, diurutkan dari yang paling cepat kedaluwarsa.
	lotRows, err := db.QueryContext(ctx, `
		SELECT l.id, l.user_id, COALESCE(l.order_id, ''), l.points_total, l.points_remaining,
		       l.earned_at, l.expires_at, l.status, l.note,
		       COALESCE(u.name, ''), COALESCE(u.username, '')
		FROM point_lots l
		LEFT JOIN users u ON u.id = l.user_id
		WHERE l.status = 'active' AND l.expires_at > ?
		ORDER BY l.expires_at ASC, l.id ASC
		LIMIT 300`, now)
	if err != nil {
		return nil, err
	}
	for lotRows.Next() {
		var v PointLotView
		if err := lotRows.Scan(&v.ID, &v.UserID, &v.OrderID, &v.PointsTotal,
			&v.PointsRemaining, &v.EarnedAt, &v.ExpiresAt, &v.Status, &v.Note,
			&v.UserName, &v.Username); err != nil {
			lotRows.Close()
			return nil, err
		}
		v.DaysLeft = daysUntil(v.ExpiresAt)
		v.ValueRupiah = v.PointsRemaining * RupiahPerPoint
		v.StatusLabel = lotStatusLabel(v.Status)
		report.Lots = append(report.Lots, v)
	}
	lotRows.Close()
	if err := lotRows.Err(); err != nil {
		return nil, err
	}

	// Riwayat penukaran terbaru.
	redemptionRows, err := db.QueryContext(ctx, `
		SELECT r.id, r.user_id, COALESCE(u.name, ''), COALESCE(u.username, ''),
		       COALESCE(r.order_id, ''), r.points, r.rupiah_value, r.status,
		       COALESCE(r.voucher_code, ''), r.note, r.created_at, r.reversed_at
		FROM point_redemptions r
		LEFT JOIN users u ON u.id = r.user_id
		ORDER BY r.id DESC
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	for redemptionRows.Next() {
		var (
			r          PointRedemption
			reversedAt sql.NullTime
		)
		if err := redemptionRows.Scan(&r.ID, &r.UserID, &r.UserName, &r.Username,
			&r.OrderID, &r.Points, &r.RupiahValue, &r.Status, &r.VoucherCode,
			&r.Note, &r.CreatedAt, &reversedAt); err != nil {
			redemptionRows.Close()
			return nil, err
		}
		if reversedAt.Valid {
			r.ReversedAt = &reversedAt.Time
		}
		report.Redemptions = append(report.Redemptions, r)
	}
	redemptionRows.Close()
	if err := redemptionRows.Err(); err != nil {
		return nil, err
	}

	// Ledger terbaru.
	ledger, err := queryPointLedger(ctx, `ORDER BY g.id DESC LIMIT 150`)
	if err != nil {
		return nil, err
	}
	report.Ledger = ledger

	return report, nil
}
