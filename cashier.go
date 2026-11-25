package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ===== Struct Data Kasir & Transaksi =====

type ProductDB struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Price    int    `json:"price"`
	Category string `json:"category"`
	Icon     string `json:"icon"`
	IsActive bool   `json:"is_active"`
}

type VoucherDB struct {
	ID            int64      `json:"id"`
	Code          string     `json:"code"`
	Title         string     `json:"title"`
	DiscountType  string     `json:"discount_type"`  // "percent" atau "fixed"
	DiscountValue int        `json:"discount_value"` // persen (0-100) atau nominal rupiah
	MinPurchase   int        `json:"min_purchase"`
	Quota         int        `json:"quota"`
	UsedCount     int        `json:"used_count"`
	ValidUntil    *time.Time `json:"valid_until"`
	IsActive      bool       `json:"is_active"`
}

type TransactionRecord struct {
	ID              string
	CashierID       string
	CashierName     string
	MemberID        string
	MemberName      string
	MemberUsername  string
	Branch          string
	Subtotal        int
	VoucherDiscount int
	PointsUsed      int
	PointsDiscount  int
	TotalDiscount   int
	GrandTotal      int
	PointsEarned    int
	PayMethod       string
	CashReceived    int
	ChangeAmount    int
	Status          string
	CreatedAt       time.Time
	Items           []TransactionItemSnapshot
	Promos          []TransactionPromoSnapshot
}

type TransactionItemSnapshot struct {
	ID                  int64
	TransactionID       string
	ProductID           int
	ProductNameSnapshot string
	CategorySnapshot    string
	UnitPriceSnapshot   int
	Qty                 int
	SubtotalSnapshot    int
}

type TransactionPromoSnapshot struct {
	ID                     int64
	TransactionID          string
	VoucherID              *int64
	PromoCodeSnapshot      string
	PromoTitleSnapshot     string
	DiscountTypeSnapshot   string
	DiscountValueSnapshot  int
	DiscountAmountSnapshot int
}

type KasirPage struct {
	PageData
	Products       []ProductDB
	Branches       []Branch
	RupiahPerPoint int
}

type TransaksiListPage struct {
	PageData
	Transactions []TransactionRecord
	SearchQuery  string
	FilterDate   string
	TotalOmzet   int
}

type TransaksiDetailPage struct {
	PageData
	Trx *TransactionRecord
}

// ===== Template Halaman Kasir & Transaksi =====

var (
	tplKasir           = parsePage("templates/kasir.html")
	tplTransaksiList   = parsePage("templates/transaksi_list.html")
	tplTransaksiDetail = parsePage("templates/transaksi_detail.html")
)

// ===== Inisialisasi Skema Database Kasir & Transaksi =====

func initCashierSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT,
			name VARCHAR(150) NOT NULL,
			description VARCHAR(255) NOT NULL DEFAULT '',
			price INT NOT NULL,
			category VARCHAR(60) NOT NULL,
			icon VARCHAR(16) NOT NULL DEFAULT '☕',
			is_active TINYINT(1) NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uq_products_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS vouchers (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			code VARCHAR(50) NOT NULL,
			title VARCHAR(150) NOT NULL,
			discount_type VARCHAR(16) NOT NULL DEFAULT 'percent',
			discount_value INT NOT NULL,
			min_purchase INT NOT NULL DEFAULT 0,
			quota INT NOT NULL DEFAULT 100,
			used_count INT NOT NULL DEFAULT 0,
			valid_until DATETIME DEFAULT NULL,
			is_active TINYINT(1) NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uq_vouchers_code (code)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS transactions (
			id VARCHAR(64) NOT NULL,
			cashier_id VARCHAR(64) NOT NULL,
			cashier_name VARCHAR(150) NOT NULL,
			member_id VARCHAR(64) DEFAULT NULL,
			member_name VARCHAR(150) NOT NULL DEFAULT 'Walk-in Guest',
			branch VARCHAR(100) NOT NULL,
			subtotal INT NOT NULL,
			voucher_discount INT NOT NULL DEFAULT 0,
			points_used INT NOT NULL DEFAULT 0,
			points_discount INT NOT NULL DEFAULT 0,
			total_discount INT NOT NULL DEFAULT 0,
			grand_total INT NOT NULL,
			points_earned INT NOT NULL DEFAULT 0,
			pay_method VARCHAR(50) NOT NULL,
			cash_received INT NOT NULL DEFAULT 0,
			change_amount INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'paid',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_transactions_created (created_at),
			KEY idx_transactions_member (member_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS transaction_items (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			transaction_id VARCHAR(64) NOT NULL,
			product_id INT UNSIGNED NOT NULL,
			product_name_snapshot VARCHAR(150) NOT NULL,
			category_snapshot VARCHAR(60) NOT NULL DEFAULT '',
			unit_price_snapshot INT NOT NULL,
			qty INT NOT NULL,
			subtotal_snapshot INT NOT NULL,
			PRIMARY KEY (id),
			KEY idx_trx_items_trx (transaction_id),
			CONSTRAINT fk_trx_items_trx FOREIGN KEY (transaction_id)
				REFERENCES transactions (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS transaction_promos (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			transaction_id VARCHAR(64) NOT NULL,
			voucher_id BIGINT UNSIGNED DEFAULT NULL,
			promo_code_snapshot VARCHAR(50) NOT NULL,
			promo_title_snapshot VARCHAR(150) NOT NULL,
			discount_type_snapshot VARCHAR(20) NOT NULL,
			discount_value_snapshot INT NOT NULL,
			discount_amount_snapshot INT NOT NULL,
			PRIMARY KEY (id),
			KEY idx_trx_promos_trx (transaction_id),
			CONSTRAINT fk_trx_promos_trx FOREIGN KEY (transaction_id)
				REFERENCES transactions (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("gagal menyiapkan tabel kasir: %w", err)
		}
	}

	// Seed tabel products dari slice menuItems bila masih kosong
	var prodCount int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM products").Scan(&prodCount)
	if prodCount == 0 {
		for _, m := range menuItems {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO products (name, description, price, category, icon, is_active)
				VALUES (?, ?, ?, ?, ?, 1)`,
				m.Name, m.Desc, m.Price, m.Category, m.Icon)
		}
	}

	// Seed tabel vouchers dari slice promos bila masih kosong
	var voucherCount int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM vouchers").Scan(&voucherCount)
	if voucherCount == 0 {
		expiry := time.Now().AddDate(0, 6, 0)
		for _, p := range promos {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO vouchers (code, title, discount_type, discount_value, min_purchase, quota, used_count, valid_until, is_active)
				VALUES (?, ?, 'percent', ?, 0, 500, 0, ?, 1)`,
				p.Code, p.Title, p.Discount, expiry)
		}
	}

	return nil
}

func getActiveProducts(ctx context.Context) ([]ProductDB, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, description, price, category, icon, is_active
		FROM products
		WHERE is_active = 1
		ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]ProductDB, 0)
	for rows.Next() {
		var p ProductDB
		if err := rows.Scan(&p.ID, &p.Name, &p.Desc, &p.Price, &p.Category, &p.Icon, &p.IsActive); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// ===== Handler Halaman Kasir =====

func kasirHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if !isStaff(sess) {
		addFlash(w, "Silakan login sebagai kasir atau admin.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	products, err := getActiveProducts(r.Context())
	if err != nil {
		log.Printf("Gagal memuat produk kasir: %v", err)
		http.Error(w, "Gagal memuat katalog produk.", http.StatusInternalServerError)
		return
	}

	_ = tplKasir.ExecuteTemplate(w, "layout", KasirPage{
		PageData: PageData{
			Title:     "Kasir POS",
			ActiveNav: "kasir",
			Session:   sess,
			Flash:     getFlash(w, r),
		},
		Products:       products,
		Branches:       branches,
		RupiahPerPoint: RupiahPerPoint,
	})
}

// ===== API Cari Member & Saldo Poin Aktif =====

func apiKasirSearchMemberHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}

	like := "%" + q + "%"
	rows, err := db.QueryContext(r.Context(), `
		SELECT id, name, username, phone, email
		FROM users
		WHERE role = 'user' AND is_active = 1
		  AND (name LIKE ? OR username LIKE ? OR phone LIKE ? OR email LIKE ?)
		ORDER BY name ASC
		LIMIT 10`, like, like, like, like)
	if err != nil {
		http.Error(w, "Gagal mencari member", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type memberDTO struct {
		ID            string     `json:"id"`
		Name          string     `json:"name"`
		Username      string     `json:"username"`
		Phone         string     `json:"phone"`
		Email         string     `json:"email"`
		ActivePoints  int        `json:"active_points"`
		RupiahValue   int        `json:"rupiah_value"`
		Tier          MemberTier `json:"tier"`
		TotalSpending int64      `json:"total_spending"`
	}

	results := make([]memberDTO, 0)
	for rows.Next() {
		var m memberDTO
		if err := rows.Scan(&m.ID, &m.Name, &m.Username, &m.Phone, &m.Email); err != nil {
			continue
		}
		// Sinkronisasi saldo poin aktif
		pts, err := ActivePointBalance(m.ID)
		if err == nil {
			m.ActivePoints = pts
			m.RupiahValue = pts * RupiahPerPoint
		}

		// Menghitung tingkat membership
		tier, totalSpending, err := GetTier(m.ID, time.Now())
		if err != nil {
			http.Error(w, "Gagal menghitung tingkat membership", http.StatusInternalServerError)
			return
		}

		m.Tier = tier
		m.TotalSpending = totalSpending
		results = append(results, m)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

// ===== API Validasi Voucher =====

func calculateVoucherDiscount(v VoucherDB, subtotal int) (int, error) {
	if !v.IsActive {
		return 0, errors.New("voucher tidak aktif")
	}
	if v.ValidUntil != nil && !v.ValidUntil.After(time.Now()) {
		return 0, errors.New("masa berlaku voucher sudah habis")
	}
	if v.UsedCount >= v.Quota {
		return 0, errors.New("kuota voucher sudah habis")
	}
	if subtotal < v.MinPurchase {
		return 0, fmt.Errorf("minimal belanja untuk voucher ini adalah %s", formatRupiah(v.MinPurchase))
	}

	discount := 0
	if v.DiscountType == "fixed" {
		discount = v.DiscountValue
	} else {
		discount = subtotal * v.DiscountValue / 100
	}
	if discount > subtotal {
		discount = subtotal
	}
	return discount, nil
}

func apiKasirCheckVoucherHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code     string `json:"code"`
		Subtotal int    `json:"subtotal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Format request tidak valid")
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "Kode voucher wajib diisi")
		return
	}

	var v VoucherDB
	var validUntil sql.NullTime
	err := db.QueryRowContext(r.Context(), `
		SELECT id, code, title, discount_type, discount_value, min_purchase, quota, used_count, valid_until, is_active
		FROM vouchers
		WHERE code = ?
		LIMIT 1`, code).Scan(
		&v.ID, &v.Code, &v.Title, &v.DiscountType, &v.DiscountValue,
		&v.MinPurchase, &v.Quota, &v.UsedCount, &validUntil, &v.IsActive,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSONError(w, http.StatusNotFound, "Kode voucher tidak ditemukan")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal memeriksa voucher")
		return
	}
	if validUntil.Valid {
		v.ValidUntil = &validUntil.Time
	}

	discount, err := calculateVoucherDiscount(v, req.Subtotal)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"voucher_id":      v.ID,
		"code":            v.Code,
		"title":           v.Title,
		"discount_type":   v.DiscountType,
		"discount_value":  v.DiscountValue,
		"discount_amount": discount,
	})
}

// ===== API Checkout Kasir (Satu DB Transaction + FOR UPDATE + Snapshot) =====

type checkoutItemPayload struct {
	ProductID int `json:"product_id"`
	Qty       int `json:"qty"`
}

type kasirCheckoutPayload struct {
	Branch       string                `json:"branch"`
	MemberID     string                `json:"member_id"`
	GuestName    string                `json:"guest_name"`
	VoucherCode  string                `json:"voucher_code"`
	PointsUsed   int                   `json:"points_used"`
	PayMethod    string                `json:"pay_method"`
	CashReceived int                   `json:"cash_received"`
	Items        []checkoutItemPayload `json:"items"`
}

func apiKasirCheckoutHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if !isStaff(sess) {
		writeJSONError(w, http.StatusForbidden, "Akses ditolak")
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req kasirCheckoutPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON tidak valid")
		return
	}

	req.Branch = strings.TrimSpace(req.Branch)
	req.MemberID = strings.TrimSpace(req.MemberID)
	req.GuestName = strings.TrimSpace(req.GuestName)
	req.VoucherCode = strings.ToUpper(strings.TrimSpace(req.VoucherCode))
	req.PayMethod = strings.TrimSpace(req.PayMethod)

	if req.Branch == "" {
		writeJSONError(w, http.StatusBadRequest, "Cabang wajib dipilih")
		return
	}
	if len(req.Items) == 0 {
		writeJSONError(w, http.StatusBadRequest, "Keranjang belanja masih kosong")
		return
	}
	if req.PointsUsed < 0 {
		writeJSONError(w, http.StatusBadRequest, "Jumlah poin tidak valid")
		return
	}
	if req.PointsUsed > 0 && req.MemberID == "" {
		writeJSONError(w, http.StatusBadRequest, "Penukaran poin hanya berlaku untuk member terdaftar")
		return
	}
	if req.PayMethod == "" {
		req.PayMethod = "Tunai"
	}

	// Gabungkan qty jika ada product_id yang dikirim ganda
	qtyByProduct := make(map[int]int)
	orderedIDs := make([]int, 0, len(req.Items))
	for _, it := range req.Items {
		if it.ProductID <= 0 || it.Qty <= 0 {
			writeJSONError(w, http.StatusBadRequest, "Item atau jumlah produk tidak valid")
			return
		}
		if _, exists := qtyByProduct[it.ProductID]; !exists {
			orderedIDs = append(orderedIDs, it.ProductID)
		}
		qtyByProduct[it.ProductID] += it.Qty
	}

	ctx := r.Context()
	// Mulai SATU DB Transaction untuk keseluruhan proses penyimpanan
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Gagal memulai DB transaction: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Gagal memulai transaksi database")
		return
	}
	defer tx.Rollback()

	// 1. Ambil harga asli produk dari DB untuk membentuk snapshot transaction_items
	itemSnapshots := make([]TransactionItemSnapshot, 0, len(orderedIDs))
	subtotal := 0
	for _, pid := range orderedIDs {
		var p ProductDB
		err := tx.QueryRowContext(ctx, `
			SELECT id, name, category, price, is_active
			FROM products
			WHERE id = ?`, pid).Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.IsActive)
		if errors.Is(err, sql.ErrNoRows) || !p.IsActive {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Produk ID %d tidak tersedia", pid))
			return
		}
		if err != nil {
			log.Printf("Error membaca produk: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Gagal memvalidasi harga produk")
			return
		}

		qty := qtyByProduct[pid]
		lineSubtotal := p.Price * qty
		subtotal += lineSubtotal

		itemSnapshots = append(itemSnapshots, TransactionItemSnapshot{
			ProductID:           p.ID,
			ProductNameSnapshot: p.Name,
			CategorySnapshot:    p.Category,
			UnitPriceSnapshot:   p.Price,
			Qty:                 qty,
			SubtotalSnapshot:    lineSubtotal,
		})
	}

	trxID := "TRX-" + time.Now().Format("20060102") + "-" + randID()

	// 2. Kunci baris voucher dengan FOR UPDATE (bila voucher digunakan)
	voucherDiscount := 0
	var promoSnapshot *TransactionPromoSnapshot
	if req.VoucherCode != "" {
		var v VoucherDB
		var validUntil sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT id, code, title, discount_type, discount_value, min_purchase, quota, used_count, valid_until, is_active
			FROM vouchers
			WHERE code = ?
			FOR UPDATE`, req.VoucherCode).Scan(
			&v.ID, &v.Code, &v.Title, &v.DiscountType, &v.DiscountValue,
			&v.MinPurchase, &v.Quota, &v.UsedCount, &validUntil, &v.IsActive,
		)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusBadRequest, "Kode voucher tidak ditemukan")
			return
		}
		if err != nil {
			log.Printf("Error mengunci voucher: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Gagal memvalidasi voucher")
			return
		}
		if validUntil.Valid {
			v.ValidUntil = &validUntil.Time
		}

		voucherDiscount, err = calculateVoucherDiscount(v, subtotal)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE vouchers SET used_count = used_count + 1 WHERE id = ?`, v.ID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Gagal memperbarui kuota voucher")
			return
		}

		vID := v.ID
		promoSnapshot = &TransactionPromoSnapshot{
			TransactionID:          trxID,
			VoucherID:              &vID,
			PromoCodeSnapshot:      v.Code,
			PromoTitleSnapshot:     v.Title,
			DiscountTypeSnapshot:   v.DiscountType,
			DiscountValueSnapshot:  v.DiscountValue,
			DiscountAmountSnapshot: voucherDiscount,
		}
	}

	// 3. Validasi member & kunci lot poin FIFO dengan FOR UPDATE (bila menukar poin)
	memberName := "Walk-in Guest"
	if req.GuestName != "" {
		memberName = req.GuestName
	}

	pointsDiscount := 0
	if req.MemberID != "" {
		var mName string
		var mActive bool
		err := tx.QueryRowContext(ctx, `
			SELECT name, is_active
			FROM users
			WHERE id = ? AND role = 'user'
			FOR UPDATE`, req.MemberID).Scan(&mName, &mActive)
		if errors.Is(err, sql.ErrNoRows) || !mActive {
			writeJSONError(w, http.StatusBadRequest, "Data member tidak ditemukan atau tidak aktif")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Gagal memeriksa data member")
			return
		}
		memberName = mName

		if req.PointsUsed > 0 {
			remainingAfterVoucher := subtotal - voucherDiscount
			maxUsefulPoints := remainingAfterVoucher / RupiahPerPoint
			if req.PointsUsed > maxUsefulPoints {
				writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Maksimal poin yang dapat dipakai untuk tagihan ini adalah %d poin", maxUsefulPoints))
				return
			}

			// redeemPointsTx mengunci point_lots aktif dengan FOR UPDATE (urut expires_at ASC),
			// memotong FIFO, mencatat point_redemptions, dan menulis mutasi ke point_ledger
			if _, err := redeemPointsTx(ctx, tx, req.MemberID, req.PointsUsed, trxID, "Penukaran poin kasir "+trxID, ""); err != nil {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			pointsDiscount = req.PointsUsed * RupiahPerPoint
		}
	}

	totalDiscount := voucherDiscount + pointsDiscount
	if totalDiscount > subtotal {
		totalDiscount = subtotal
	}
	grandTotal := subtotal - totalDiscount

	// Validasi pembayaran tunai
	changeAmount := 0
	if strings.EqualFold(req.PayMethod, "Tunai") {
		if req.CashReceived < grandTotal {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Uang tunai kurang dari total tagihan (%s)", formatRupiah(grandTotal)))
			return
		}
		changeAmount = req.CashReceived - grandTotal
	} else {
		req.CashReceived = grandTotal
	}

	// Hitung perolehan poin baru untuk member
	pointsEarned := 0
	if req.MemberID != "" && grandTotal >= PointsPerRupiah {
		pointsEarned = grandTotal / PointsPerRupiah
	}

	// 4. Simpan header transaksi ke tabel transactions
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			id, cashier_id, cashier_name, member_id, member_name, branch,
			subtotal, voucher_discount, points_used, points_discount,
			total_discount, grand_total, points_earned,
			pay_method, cash_received, change_amount, status
		) VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'paid')`,
		trxID, sess.UserID, sess.Name, req.MemberID, memberName, req.Branch,
		subtotal, voucherDiscount, req.PointsUsed, pointsDiscount,
		totalDiscount, grandTotal, pointsEarned,
		req.PayMethod, req.CashReceived, changeAmount,
	); err != nil {
		log.Printf("Gagal menyimpan header transaksi: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Gagal menyimpan transaksi")
		return
	}

	// 5. Simpan snapshot item ke transaction_items
	for _, item := range itemSnapshots {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaction_items (
				transaction_id, product_id, product_name_snapshot,
				category_snapshot, unit_price_snapshot, qty, subtotal_snapshot
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			trxID, item.ProductID, item.ProductNameSnapshot,
			item.CategorySnapshot, item.UnitPriceSnapshot, item.Qty, item.SubtotalSnapshot,
		); err != nil {
			log.Printf("Gagal menyimpan snapshot item transaksi: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Gagal menyimpan rincian item transaksi")
			return
		}
	}

	// 6. Simpan snapshot promo/voucher ke transaction_promos
	if promoSnapshot != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaction_promos (
				transaction_id, voucher_id, promo_code_snapshot, promo_title_snapshot,
				discount_type_snapshot, discount_value_snapshot, discount_amount_snapshot
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			trxID, promoSnapshot.VoucherID, promoSnapshot.PromoCodeSnapshot, promoSnapshot.PromoTitleSnapshot,
			promoSnapshot.DiscountTypeSnapshot, promoSnapshot.DiscountValueSnapshot, promoSnapshot.DiscountAmountSnapshot,
		); err != nil {
			log.Printf("Gagal menyimpan snapshot promo transaksi: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Gagal menyimpan rincian promo transaksi")
			return
		}
	}

	// 7. Tambahkan lot poin baru (masa berlaku 6 bulan) di dalam DB transaction yang sama
	if req.MemberID != "" && pointsEarned > 0 {
		if _, err := addPointsTx(ctx, tx, req.MemberID, pointsEarned, trxID, "Poin transaksi kasir "+trxID); err != nil {
			log.Printf("Gagal menambahkan poin transaksi: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Gagal mencatat perolehan poin member")
			return
		}
	}

	// Commit seluruh operasi secara atomik
	if err := tx.Commit(); err != nil {
		log.Printf("Gagal commit transaksi kasir: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Gagal menyelesaikan transaksi")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"transaction_id": trxID,
		"grand_total":    grandTotal,
		"change_amount":  changeAmount,
		"points_earned":  pointsEarned,
	})
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "error",
		"message": msg,
	})
}

// ===== Handler Daftar Transaksi (/transaksi) =====

func transaksiListHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if !isStaff(sess) {
		addFlash(w, "Silakan login sebagai kasir atau admin.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	dateFilter := strings.TrimSpace(r.URL.Query().Get("date"))

	query := `
		SELECT t.id, t.cashier_id, t.cashier_name, COALESCE(t.member_id, ''), t.member_name,
		       COALESCE(u.username, ''), t.branch, t.subtotal, t.voucher_discount,
		       t.points_used, t.points_discount, t.total_discount, t.grand_total,
		       t.points_earned, t.pay_method, t.cash_received, t.change_amount,
		       t.status, t.created_at
		FROM transactions t
		LEFT JOIN users u ON u.id = t.member_id
		WHERE 1=1`
	args := make([]any, 0)

	if search != "" {
		like := "%" + search + "%"
		query += ` AND (t.id LIKE ? OR t.member_name LIKE ? OR t.cashier_name LIKE ? OR u.username LIKE ?)`
		args = append(args, like, like, like, like)
	}
	if dateFilter != "" {
		query += ` AND DATE(t.created_at) = ?`
		args = append(args, dateFilter)
	}
	query += ` ORDER BY t.created_at DESC LIMIT 200`

	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		log.Printf("Gagal memuat daftar transaksi: %v", err)
		http.Error(w, "Gagal memuat daftar transaksi.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	list := make([]TransactionRecord, 0)
	totalOmzet := 0
	for rows.Next() {
		var tr TransactionRecord
		if err := rows.Scan(
			&tr.ID, &tr.CashierID, &tr.CashierName, &tr.MemberID, &tr.MemberName,
			&tr.MemberUsername, &tr.Branch, &tr.Subtotal, &tr.VoucherDiscount,
			&tr.PointsUsed, &tr.PointsDiscount, &tr.TotalDiscount, &tr.GrandTotal,
			&tr.PointsEarned, &tr.PayMethod, &tr.CashReceived, &tr.ChangeAmount,
			&tr.Status, &tr.CreatedAt,
		); err != nil {
			log.Printf("Error scan transaksi: %v", err)
			continue
		}
		if tr.Status == "paid" {
			totalOmzet += tr.GrandTotal
		}
		list = append(list, tr)
	}

	_ = tplTransaksiList.ExecuteTemplate(w, "layout", TransaksiListPage{
		PageData: PageData{
			Title:     "Daftar Transaksi",
			ActiveNav: "transaksi",
			Session:   sess,
			Flash:     getFlash(w, r),
		},
		Transactions: list,
		SearchQuery:  search,
		FilterDate:   dateFilter,
		TotalOmzet:   totalOmzet,
	})
}

// ===== Handler Detail Transaksi (/transaksi/{id}) =====

func transaksiDetailHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if !isStaff(sess) {
		addFlash(w, "Silakan login sebagai kasir atau admin.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	trxID := strings.TrimPrefix(r.URL.Path, "/transaksi/")
	trxID = strings.TrimSpace(trxID)
	if trxID == "" {
		http.Redirect(w, r, "/transaksi", http.StatusSeeOther)
		return
	}

	var tr TransactionRecord
	err := db.QueryRowContext(r.Context(), `
		SELECT t.id, t.cashier_id, t.cashier_name, COALESCE(t.member_id, ''), t.member_name,
		       COALESCE(u.username, ''), t.branch, t.subtotal, t.voucher_discount,
		       t.points_used, t.points_discount, t.total_discount, t.grand_total,
		       t.points_earned, t.pay_method, t.cash_received, t.change_amount,
		       t.status, t.created_at
		FROM transactions t
		LEFT JOIN users u ON u.id = t.member_id
		WHERE t.id = ?
		LIMIT 1`, trxID).Scan(
		&tr.ID, &tr.CashierID, &tr.CashierName, &tr.MemberID, &tr.MemberName,
		&tr.MemberUsername, &tr.Branch, &tr.Subtotal, &tr.VoucherDiscount,
		&tr.PointsUsed, &tr.PointsDiscount, &tr.TotalDiscount, &tr.GrandTotal,
		&tr.PointsEarned, &tr.PayMethod, &tr.CashReceived, &tr.ChangeAmount,
		&tr.Status, &tr.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("Gagal memuat detail transaksi: %v", err)
		http.Error(w, "Gagal memuat detail transaksi.", http.StatusInternalServerError)
		return
	}

	// Ambil snapshot item dari transaction_items
	itemRows, err := db.QueryContext(r.Context(), `
		SELECT id, transaction_id, product_id, product_name_snapshot,
		       category_snapshot, unit_price_snapshot, qty, subtotal_snapshot
		FROM transaction_items
		WHERE transaction_id = ?
		ORDER BY id ASC`, trxID)
	if err == nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var it TransactionItemSnapshot
			if err := itemRows.Scan(
				&it.ID, &it.TransactionID, &it.ProductID, &it.ProductNameSnapshot,
				&it.CategorySnapshot, &it.UnitPriceSnapshot, &it.Qty, &it.SubtotalSnapshot,
			); err == nil {
				tr.Items = append(tr.Items, it)
			}
		}
	}

	// Ambil snapshot promo dari transaction_promos
	promoRows, err := db.QueryContext(r.Context(), `
		SELECT id, transaction_id, voucher_id, promo_code_snapshot, promo_title_snapshot,
		       discount_type_snapshot, discount_value_snapshot, discount_amount_snapshot
		FROM transaction_promos
		WHERE transaction_id = ?
		ORDER BY id ASC`, trxID)
	if err == nil {
		defer promoRows.Close()
		for promoRows.Next() {
			var pr TransactionPromoSnapshot
			var vID sql.NullInt64
			if err := promoRows.Scan(
				&pr.ID, &pr.TransactionID, &vID, &pr.PromoCodeSnapshot, &pr.PromoTitleSnapshot,
				&pr.DiscountTypeSnapshot, &pr.DiscountValueSnapshot, &pr.DiscountAmountSnapshot,
			); err == nil {
				if vID.Valid {
					idVal := vID.Int64
					pr.VoucherID = &idVal
				}
				tr.Promos = append(tr.Promos, pr)
			}
		}
	}

	_ = tplTransaksiDetail.ExecuteTemplate(w, "layout", TransaksiDetailPage{
		PageData: PageData{
			Title:     "Detail Transaksi " + tr.ID,
			ActiveNav: "transaksi",
			Session:   sess,
			Flash:     getFlash(w, r),
		},
		Trx: &tr,
	})
}
