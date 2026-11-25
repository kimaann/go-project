package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ===== Aturan Membership =====

type MemberTier string

const (
	TierSilver MemberTier = "Silver"
	TierGold   MemberTier = "Gold"

	GoldMinimumSpending = 1500000
)

// ===== Fungsi Menghitung Tingkat Member =====

// GetTier menentukan tingkat member berdasarkan
// total belanja selama tiga bulan terakhir.
func GetTier(memberID string, waktu time.Time) (MemberTier, int64, error) {

	// Validasi ID member
	memberID = strings.TrimSpace(memberID)

	if memberID == "" {
		return "", 0, fmt.Errorf("ID member tidak boleh kosong")
	}

	// Menentukan awal periode tiga bulan terakhir
	waktuMulai := waktu.AddDate(0, -3, 0)

	// Menghitung total belanja yang memenuhi syarat
	// Belanja dihitung setelah diskon voucher,
	// tetapi sebelum potongan penukaran poin.
	var totalBelanja int64

	err := db.QueryRowContext(
		context.Background(),
		`
		SELECT COALESCE(
			SUM(subtotal - voucher_discount), 0
		)
		FROM transactions
		WHERE member_id = ?
		  AND status = 'paid'
		  AND created_at >= ?
		  AND created_at <= ?
		`,
		memberID,
		waktuMulai,
		waktu,
	).Scan(&totalBelanja)

	if err != nil {
		return "", 0, fmt.Errorf(
			"gagal menghitung total belanja member: %w", err,
		)
	}

	// Menentukan tingkat membership
	if totalBelanja >= GoldMinimumSpending {
		return TierGold, totalBelanja, nil
	}

	return TierSilver, totalBelanja, nil
}

// ===== Inisialisasi Tabel Riwayat Membership =====

func initMemberTierSchema(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS member_tier_history (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		member_id VARCHAR(64) NOT NULL,
		previous_tier VARCHAR(20) DEFAULT NULL,
		new_tier VARCHAR(20) NOT NULL,
		total_spending BIGINT NOT NULL DEFAULT 0,
		period_start DATETIME NOT NULL,
		period_end DATETIME NOT NULL,
		evaluated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

		PRIMARY KEY (id),
		KEY idx_member_tier_member (member_id, evaluated_at),
		KEY idx_member_tier_level (new_tier, evaluated_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	_, err := db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf(
			"gagal membuat tabel member_tier_history: %w", err,
		)
	}

	return nil
}
