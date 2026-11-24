package main

// cart_preview.go — Endpoint pratinjau keranjang untuk halaman POS (modul Aulia).
//
//	POST /api/kasir/cart/preview
//
// Server menghitung ulang seluruh rincian harga setiap keranjang berubah
// (rancangan bagian 3): harga diambil dari DB, bukan dari browser.
// Endpoint ini TIDAK menyimpan apa pun dan tidak mengonsumsi voucher maupun poin.
//
// Sumber data lintas modul dipasang lewat variabel fungsi di bawah, sehingga
// modul Firman (promo), Alyssa (tingkat), dan Rifqi/Gagah (poin) dapat dicolok
// tanpa mengubah file ini, dan handler dapat diuji tanpa database.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// cartConfig adalah aturan hitung yang dipakai endpoint; bawaannya selaras dengan modul poin.
var cartConfig = DefaultCartConfig()

const (
	maxCartLines = 100
	maxLineQty   = 1000
)

// ===== Titik sambung antar modul =====

var (
	cartNow = time.Now

	// Kontrak Firman: GetActivePromos(cabang, waktu) []Promo.
	// Sementara kosong; ganti dengan fungsi milik Firman bila sudah jadi.
	// Untuk mencoba/presentasi, jalankan server dengan CART_DEMO_PROMOS=1 agar promo contoh aktif.
	cartActivePromos = func(ctx context.Context, branch string, at time.Time) ([]CartPromo, error) {
		if cartDemoPromosEnabled() {
			return demoCartPromos(), nil
		}
		return nil, nil
	}

	// Kontrak Alyssa: GetTier(memberID, waktu) Tier.
	// Sementara semua member Silver; ganti dengan GetTier milik Alyssa.
	cartMemberTier = func(ctx context.Context, memberID string, at time.Time) (MemberTier, error) {
		return TierSilver, nil
	}

	// Kontrak Rifqi/Gagah: GetActivePoints(memberID, waktu) int.
	cartPointBalance = func(ctx context.Context, memberID string, at time.Time) (int, error) {
		return ActivePointBalance(memberID)
	}

	// Sumber DB yang sudah ada di proyek ini.
	cartLoadProducts = loadProductsByIDs
	cartLoadVoucher  = loadVoucherAsPromo
)

func cartDemoPromosEnabled() bool { return os.Getenv("CART_DEMO_PROMOS") == "1" }

// demoCartPromos adalah promo contoh (hanya untuk demo, aktif bila CART_DEMO_PROMOS=1).
func demoCartPromos() []CartPromo {
	return []CartPromo{
		{ID: 101, Name: "Beli 2 Gratis 1 Kopi", Type: PromoBuy2Get1, Categories: []string{"Kopi"}, Active: true},
		{ID: 102, Name: "Happy Hour Makanan 20%", Type: PromoProductPercent, Value: 20, Categories: []string{"Makanan"},
			StartClock: "14:00", EndClock: "17:00", Active: true},
		{ID: 103, Name: "Promo Malam Non-Kopi 30%", Type: PromoProductPercent, Value: 30, Categories: []string{"Non-Kopi"},
			StartClock: "22:00", EndClock: "02:00", Active: true},
		{ID: 104, Name: "Potongan Belanja Rp150rb", Type: PromoMinSpendNominal, Value: 15000, MinSpend: 150000, Active: true},
	}
}

// loadProductsByIDs mengambil produk aktif (harga resmi dari DB).
func loadProductsByIDs(ctx context.Context, ids []int) (map[int]ProductDB, error) {
	out := make(map[int]ProductDB, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		marks[i] = "?"
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, category, price, is_active
		FROM products
		WHERE id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p ProductDB
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.IsActive); err != nil {
			return nil, err
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}

// loadVoucherAsPromo membaca tabel vouchers yang sudah ada dan mengubahnya
// menjadi CartPromo bertipe VOUCHER. Kembali nil bila kode tidak ditemukan.
func loadVoucherAsPromo(ctx context.Context, code string, at time.Time) (*CartPromo, error) {
	var v VoucherDB
	var validUntil sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT id, code, title, discount_type, discount_value, min_purchase, quota, used_count, valid_until, is_active
		FROM vouchers WHERE code = ? LIMIT 1`, code).Scan(
		&v.ID, &v.Code, &v.Title, &v.DiscountType, &v.DiscountValue,
		&v.MinPurchase, &v.Quota, &v.UsedCount, &validUntil, &v.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	active := v.IsActive && (!validUntil.Valid || validUntil.Time.After(at))
	return &CartPromo{
		ID: v.ID, Name: v.Title, Type: PromoVoucher, Code: v.Code,
		Value: v.DiscountValue, ValueIsPercent: v.DiscountType != "fixed",
		MinSpend: v.MinPurchase, Quota: v.Quota, Used: v.UsedCount, Active: active,
	}, nil
}

// ===== Request / Handler =====

type cartPreviewItem struct {
	ProductID int `json:"product_id"`
	Qty       int `json:"qty"`
}

type cartPreviewRequest struct {
	Branch         string            `json:"branch"`
	MemberID       string            `json:"member_id"`
	VoucherCode    string            `json:"voucher_code"`
	At             string            `json:"at"`              // opsional, simulasi waktu: "2006-01-02T15:04"
	DiscountChoice string            `json:"discount_choice"` // AUTO (default) | MIN_SPEND | VOUCHER
	PointsUsed     int               `json:"points_used"`
	RedeemMax      bool              `json:"redeem_max"`
	Items          []cartPreviewItem `json:"items"`
}

// buildCartPreview menyusun CartInput dari request lalu memanggil CalculateCart.
// Mengembalikan status HTTP dan pesan bila gagal.
func buildCartPreview(ctx context.Context, req cartPreviewRequest) (CartResult, int, string) {
	now := cartNow()
	if strings.TrimSpace(req.At) != "" {
		t, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(req.At), now.Location())
		if err != nil {
			return CartResult{}, http.StatusBadRequest, "Format waktu simulasi tidak valid"
		}
		now = t
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		return CartResult{}, http.StatusBadRequest, "Cabang wajib dipilih"
	}
	if len(req.Items) > maxCartLines {
		return CartResult{}, http.StatusBadRequest, "Jumlah item keranjang terlalu banyak"
	}

	choice := DiscountChoice(strings.ToUpper(strings.TrimSpace(req.DiscountChoice)))
	switch choice {
	case "", ChoiceAuto, ChoiceMinSpend, ChoiceVoucher:
	default:
		return CartResult{}, http.StatusBadRequest, "discount_choice harus AUTO, MIN_SPEND, atau VOUCHER"
	}

	// Gabungkan product_id ganda, pertahankan urutan.
	qty := map[int]int{}
	ids := make([]int, 0, len(req.Items))
	for _, it := range req.Items {
		if it.ProductID <= 0 || it.Qty <= 0 || it.Qty > maxLineQty {
			return CartResult{}, http.StatusBadRequest, "Item atau jumlah produk tidak valid"
		}
		if _, ok := qty[it.ProductID]; !ok {
			ids = append(ids, it.ProductID)
		}
		qty[it.ProductID] += it.Qty
		if qty[it.ProductID] > maxLineQty {
			return CartResult{}, http.StatusBadRequest, "Item atau jumlah produk tidak valid"
		}
	}

	products, err := cartLoadProducts(ctx, ids)
	if err != nil {
		log.Printf("cart preview: gagal memuat produk: %v", err)
		return CartResult{}, http.StatusInternalServerError, "Gagal memuat harga produk"
	}
	items := make([]CartLineInput, 0, len(ids))
	for _, id := range ids {
		p, ok := products[id]
		if !ok || !p.IsActive {
			return CartResult{}, http.StatusBadRequest, "Produk tidak tersedia"
		}
		items = append(items, CartLineInput{ProductID: p.ID, Name: p.Name, Category: p.Category, Price: p.Price, Qty: qty[id]})
	}

	promos, err := cartActivePromos(ctx, branch, now)
	if err != nil {
		log.Printf("cart preview: gagal memuat promo: %v", err)
		return CartResult{}, http.StatusInternalServerError, "Gagal memuat promo"
	}
	code := strings.ToUpper(strings.TrimSpace(req.VoucherCode))
	if code != "" {
		v, err := cartLoadVoucher(ctx, code, now)
		if err != nil {
			log.Printf("cart preview: gagal memuat voucher: %v", err)
			return CartResult{}, http.StatusInternalServerError, "Gagal memeriksa voucher"
		}
		if v != nil {
			promos = append(append([]CartPromo{}, promos...), *v)
		}
	}

	cfg := cartConfig
	in := CartInput{
		Config: &cfg,
		Branch: branch, At: now, Items: items, Promos: promos,
		VoucherCode: code, Choice: choice,
		MemberID: strings.TrimSpace(req.MemberID), Tier: TierSilver,
		PointsToRedeem: req.PointsUsed, RedeemMaxPoints: req.RedeemMax,
	}
	if in.MemberID != "" {
		if in.PointBalance, err = cartPointBalance(ctx, in.MemberID, now); err != nil {
			log.Printf("cart preview: gagal memuat saldo poin: %v", err)
			return CartResult{}, http.StatusInternalServerError, "Gagal memuat saldo poin member"
		}
		if in.Tier, err = cartMemberTier(ctx, in.MemberID, now); err != nil {
			log.Printf("cart preview: gagal memuat tingkat member: %v", err)
			return CartResult{}, http.StatusInternalServerError, "Gagal memuat tingkat member"
		}
	}

	res, err := CalculateCart(in)
	if err != nil {
		return CartResult{}, http.StatusBadRequest, err.Error()
	}
	return res, http.StatusOK, ""
}

// apiKasirCartPreviewHandler melayani POST /api/kasir/cart/preview.
func apiKasirCartPreviewHandler(w http.ResponseWriter, r *http.Request) {
	if !isStaff(getSession(r)) {
		writeJSONError(w, http.StatusForbidden, "Akses ditolak")
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req cartPreviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON tidak valid")
		return
	}

	res, status, msg := buildCartPreview(r.Context(), req)
	if status != http.StatusOK {
		writeJSONError(w, status, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "cart": res})
}

// ===== Halaman Kalkulator Keranjang =====

type KeranjangPage struct {
	PageData
	Products       []ProductDB
	Branches       []Branch
	RupiahPerPoint int
	DemoPromos     bool
}

var tplKeranjang = parsePage("templates/keranjang.html")

// keranjangHandler melayani GET /keranjang (kasir/admin).
func keranjangHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if !isStaff(sess) {
		addFlash(w, "Silakan login sebagai kasir atau admin.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	products, err := getActiveProducts(r.Context())
	if err != nil {
		log.Printf("Gagal memuat produk kalkulator keranjang: %v", err)
		http.Error(w, "Gagal memuat katalog produk.", http.StatusInternalServerError)
		return
	}
	_ = tplKeranjang.ExecuteTemplate(w, "layout", KeranjangPage{
		PageData: PageData{
			Title:     "Kalkulator Keranjang",
			ActiveNav: "keranjang",
			Session:   sess,
			Flash:     getFlash(w, r),
		},
		Products:       products,
		Branches:       branches,
		RupiahPerPoint: RupiahPerPoint,
		DemoPromos:     cartDemoPromosEnabled(),
	})
}
