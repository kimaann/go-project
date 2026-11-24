package main
import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ===== Tipe dan Konstanta =====

type PromoType string

const (
	PromoProductPercent  PromoType = "PRODUCT_PERCENT"   // diskon persen untuk produk/kategori
	PromoMinSpendNominal PromoType = "MIN_SPEND_NOMINAL" // potongan nominal bila belanja >= MinSpend
	PromoBuy2Get1        PromoType = "BUY2GET1"          // beli 2 gratis 1
	PromoVoucher         PromoType = "VOUCHER"           // potongan dengan kode voucher
)

type MemberTier string

const (
	TierSilver MemberTier = "SILVER"
	TierGold   MemberTier = "GOLD"
)

type DiscountChoice string

const (
	ChoiceAuto     DiscountChoice = "AUTO"      // otomatis yang paling menguntungkan pelanggan
	ChoiceMinSpend DiscountChoice = "MIN_SPEND" // pilihan kasir: potongan minimal belanja
	ChoiceVoucher  DiscountChoice = "VOUCHER"   // pilihan kasir: voucher
)

var ErrInvalidCartItem = errors.New("item keranjang tidak valid: qty harus > 0 dan harga tidak boleh negatif")

// ===== Konfigurasi =====

type CartConfig struct {
	RupiahPerPoint   int
	EarnPerRupiah    int
	MaxRedeemPercent int

	SingleProductPromoPerLine bool
	VoucherRequiresMember     bool
	EarnBeforeRedeem          bool
}

func DefaultCartConfig() CartConfig {
	return CartConfig{
		RupiahPerPoint:   RupiahPerPoint,
		EarnPerRupiah:    PointsPerRupiah,
		MaxRedeemPercent: 50,
	}
}

func (c CartConfig) normalized() CartConfig {
	d := DefaultCartConfig()
	if c.RupiahPerPoint <= 0 {
		c.RupiahPerPoint = d.RupiahPerPoint
	}
	if c.EarnPerRupiah <= 0 {
		c.EarnPerRupiah = d.EarnPerRupiah
	}
	if c.MaxRedeemPercent <= 0 || c.MaxRedeemPercent > 100 {
		c.MaxRedeemPercent = d.MaxRedeemPercent
	}
	return c
}

type CartPromo struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Type           PromoType `json:"type"`
	Value          int       `json:"value"`
	ValueIsPercent bool      `json:"value_is_percent"`
	MinSpend       int       `json:"min_spend"`

	Code  string `json:"code"`  // khusus VOUCHER
	Quota int    `json:"quota"` // khusus VOUCHER; 0 = tanpa batas
	Used  int    `json:"used"`

	// Masa berlaku per tanggal (inklusif, hanya tanggalnya yang dibandingkan).
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	// Jam berlaku "HH:MM" (atau "HH:MM:SS"); kosong = seharian.
	// Bila StartClock > EndClock, promo melewati tengah malam (22:00-02:00).
	StartClock string `json:"start_clock"`
	EndClock   string `json:"end_clock"`

	Branches   []string `json:"branches"`    // kosong = semua cabang
	ProductIDs []int    `json:"product_ids"` // produk/kategori kosong semua = semua produk
	Categories []string `json:"categories"`
	Active     bool     `json:"active"`
}

func parseClock(s string) (int, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"15:04", "15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Hour()*60 + t.Minute(), nil
		}
	}
	return 0, fmt.Errorf("format jam tidak valid: %q", s)
}

func ymd(t time.Time) int { return t.Year()*10000 + int(t.Month())*100 + t.Day() }
func (p CartPromo) IsActiveAt(branch string, at time.Time) bool {
	if !p.Active {
		return false
	}
	if len(p.Branches) > 0 && !containsFold(p.Branches, branch) {
		return false
	}

	day := at
	if p.StartClock != "" || p.EndClock != "" {
		if p.StartClock == "" || p.EndClock == "" {
			return false // konfigurasi jam tidak lengkap: anggap tidak berlaku
		}
		start, err1 := parseClock(p.StartClock)
		end, err2 := parseClock(p.EndClock)
		if err1 != nil || err2 != nil {
			return false
		}
		cur := at.Hour()*60 + at.Minute()
		switch {
		case start == end: // seharian
		case start < end:
			if cur < start || cur >= end {
				return false
			}
		default: // melewati tengah malam
			switch {
			case cur >= start: // malam hari: tanggal hari ini
			case cur < end: // dini hari: bagian dari promo kemarin
				day = at.AddDate(0, 0, -1)
			default:
				return false
			}
		}
	}

	if p.StartDate != nil && ymd(day) < ymd(*p.StartDate) {
		return false
	}
	if p.EndDate != nil && ymd(day) > ymd(*p.EndDate) {
		return false
	}
	return true
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

func (p CartPromo) appliesTo(l *cartLine) bool {
	if len(p.ProductIDs) == 0 && len(p.Categories) == 0 {
		return true
	}
	for _, id := range p.ProductIDs {
		if id == l.ProductID && id != 0 {
			return true
		}
	}
	return containsFold(p.Categories, l.Category)
}

// ===== Input dan Hasil =====

type CartLineInput struct {
	ProductID int
	Name      string
	Category  string
	Price     int
	Qty       int
}

type CartInput struct {
	Branch string
	At     time.Time
	Items  []CartLineInput
	Promos []CartPromo

	VoucherCode string
	Choice      DiscountChoice // kosong = AUTO

	MemberID        string
	Tier            MemberTier
	PointBalance    int
	PointsToRedeem  int
	RedeemMaxPoints bool

	Config *CartConfig // nil = DefaultCartConfig()
}

type AppliedPromo struct {
	PromoID int64     `json:"promo_id"`
	Name    string    `json:"name"`
	Type    PromoType `json:"type"`
	Code    string    `json:"code,omitempty"`
	Amount  int       `json:"amount"`
}

type CartLine struct {
	ProductID int            `json:"product_id"`
	Name      string         `json:"name"`
	Category  string         `json:"category"`
	Qty       int            `json:"qty"`
	UnitPrice int            `json:"unit_price"`
	Subtotal  int            `json:"subtotal"`
	FreeQty   int            `json:"free_qty"`
	Discount  int            `json:"discount"`
	Total     int            `json:"total"`
	Promos    []AppliedPromo `json:"promos"`
}

type CartResult struct {
	Lines []CartLine `json:"lines"`

	Subtotal             int  `json:"subtotal"`
	ProductPromoDiscount int  `json:"product_promo_discount"`
	SpendOrVoucherDisc   int  `json:"spend_or_voucher_discount"`
	TotalAfterPromo      int  `json:"total_after_promo"`
	MaxRedeemablePoints  int  `json:"max_redeemable_points"`
	PointsRedeemed       int  `json:"points_redeemed"`
	PointsDiscount       int  `json:"points_discount"`
	TotalPayable         int  `json:"total_payable"`
	PointsEarned         int  `json:"points_earned"`
	PointsAdjusted       bool `json:"points_adjusted"` // true bila permintaan tukar poin dikoreksi

	SpendOrVoucherKind DiscountChoice `json:"spend_or_voucher_kind"` // MIN_SPEND / VOUCHER / "" (tidak ada)
	VoucherCode        string         `json:"voucher_code"`
	VoucherApplied     bool           `json:"voucher_applied"` // true => kuota voucher harus dikonsumsi saat simpan
	VoucherMessage     string         `json:"voucher_message"`

	Promos   []AppliedPromo `json:"promos"` // seluruh potongan (produk + min.belanja/voucher)
	Warnings []string       `json:"warnings"`
}

// ===== CalculateCart =====

type cartLine struct {
	CartLineInput
	FreeQty  int
	Discount int
	applied  bool // sudah menerima promo produk (untuk mode tanpa tumpuk)
	promos   []AppliedPromo
}

func (l *cartLine) subtotal() int  { return l.Price * l.Qty }
func (l *cartLine) remaining() int { return l.subtotal() - l.Discount }

func (l *cartLine) add(p CartPromo, amount, freeQty int) {
	if amount <= 0 {
		return
	}
	l.Discount += amount
	l.FreeQty += freeQty
	l.applied = true
	l.promos = append(l.promos, AppliedPromo{PromoID: p.ID, Name: p.Name, Type: p.Type, Amount: amount})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CalculateCart menghitung seluruh keranjang sesuai urutan rancangan.
func CalculateCart(in CartInput) (CartResult, error) {
	cfg := DefaultCartConfig()
	if in.Config != nil {
		cfg = in.Config.normalized()
	}
	res := CartResult{Lines: []CartLine{}, Promos: []AppliedPromo{}, Warnings: []string{}}

	// --- Siapkan baris (gabung product_id ganda, validasi) ---
	lines := make([]*cartLine, 0, len(in.Items))
	byProduct := map[int]*cartLine{}
	for _, it := range in.Items {
		if it.Qty <= 0 || it.Price < 0 {
			return CartResult{}, ErrInvalidCartItem
		}
		if it.ProductID > 0 {
			if ex, ok := byProduct[it.ProductID]; ok {
				ex.Qty += it.Qty
				continue
			}
		}
		l := &cartLine{CartLineInput: it}
		if it.ProductID > 0 {
			byProduct[it.ProductID] = l
		}
		lines = append(lines, l)
	}

	// --- Saring promo yang berlaku pada cabang dan waktu ini ---
	var active []CartPromo
	for _, p := range in.Promos {
		if p.IsActiveAt(in.Branch, in.At) {
			active = append(active, p)
		}
	}

	// --- 1. Subtotal ---
	for _, l := range lines {
		res.Subtotal += l.subtotal()
	}

	// --- 2. Promo produk ---
	res.Promos = append(res.Promos, applyProductPromos(lines, active, cfg)...)
	for _, l := range lines {
		res.ProductPromoDiscount += l.Discount
	}
	afterProduct := res.Subtotal - res.ProductPromoDiscount

	// --- 3. Min. belanja ATAU voucher ---
	hasMember := strings.TrimSpace(in.MemberID) != ""
	applySpendOrVoucher(&res, in, active, afterProduct, hasMember, cfg)

	// --- 4. Total setelah promo ---
	res.TotalAfterPromo = afterProduct - res.SpendOrVoucherDisc

	// --- 5. Tukar poin ---
	redeemPoints(&res, in, hasMember, cfg)

	// --- 6. Total bayar ---
	res.TotalPayable = res.TotalAfterPromo - res.PointsDiscount

	// --- 7. Poin didapat ---
	if hasMember {
		base := res.TotalPayable
		if cfg.EarnBeforeRedeem {
			base = res.TotalAfterPromo
		}
		res.PointsEarned = earnedPoints(base, in.Tier, cfg)
	}

	for _, l := range lines {
		res.Lines = append(res.Lines, CartLine{
			ProductID: l.ProductID, Name: l.Name, Category: l.Category,
			Qty: l.Qty, UnitPrice: l.Price, Subtotal: l.subtotal(),
			FreeQty: l.FreeQty, Discount: l.Discount, Total: l.remaining(),
			Promos: append([]AppliedPromo{}, l.promos...),
		})
	}
	return res, nil
}

func earnedPoints(total int, tier MemberTier, cfg CartConfig) int {
	if total <= 0 {
		return 0
	}
	base := total / cfg.EarnPerRupiah
	if tier == TierGold {
		return base * 3 / 2
	}
	return base
}

func applyProductPromos(lines []*cartLine, active []CartPromo, cfg CartConfig) []AppliedPromo {
	var b2g1, pct []CartPromo
	for _, p := range active {
		switch p.Type {
		case PromoBuy2Get1:
			b2g1 = append(b2g1, p)
		case PromoProductPercent:
			pct = append(pct, p)
		}
	}
	byID := func(s []CartPromo) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].ID < s[j].ID })
	}
	byID(b2g1)
	byID(pct)
	if cfg.SingleProductPromoPerLine {
		// Tanpa tumpuk: tiap baris hanya mendapat satu promo persen, yang terbesar lebih dulu.
		sort.SliceStable(pct, func(i, j int) bool { return pct[i].Value > pct[j].Value })
	}

	var out []AppliedPromo

	for _, p := range b2g1 {
		// Kumpulkan semua unit yang memenuhi syarat dari semua baris yang cocok.
		var elig []*cartLine
		total := 0
		for _, l := range lines {
			if !p.appliesTo(l) || (cfg.SingleProductPromoPerLine && l.applied) {
				continue
			}
			if avail := l.Qty - l.FreeQty; avail > 0 {
				elig = append(elig, l)
				total += avail
			}
		}
		// Tiap kelompok 3 unit: 1 gratis (sisa dari kelipatan 3 tidak gratis, mis. 5 item -> 1 gratis).
		free := total / 3
		if free == 0 {
			continue
		}
		// Yang digratiskan adalah unit termurah.
		sort.SliceStable(elig, func(i, j int) bool { return elig[i].Price < elig[j].Price })
		amount := 0
		for _, l := range elig {
			if free == 0 {
				break
			}
			n := minInt(free, l.Qty-l.FreeQty)
			l.add(p, n*l.Price, n)
			amount += n * l.Price
			free -= n
		}
		if amount > 0 {
			out = append(out, AppliedPromo{PromoID: p.ID, Name: p.Name, Type: p.Type, Amount: amount})
		}
	}

	for _, p := range pct {
		percent := p.Value
		if percent <= 0 {
			continue
		}
		if percent > 100 {
			percent = 100
		}
		amount := 0
		for _, l := range lines {
			if !p.appliesTo(l) || (cfg.SingleProductPromoPerLine && l.applied) {
				continue
			}
			d := l.remaining() * percent / 100 // dibulatkan ke bawah
			if d > 0 {
				l.add(p, d, 0)
				amount += d
			}
		}
		if amount > 0 {
			out = append(out, AppliedPromo{PromoID: p.ID, Name: p.Name, Type: p.Type, Amount: amount})
		}
	}
	return out
}

func applySpendOrVoucher(res *CartResult, in CartInput, active []CartPromo, base int, hasMember bool, cfg CartConfig) {
	res.VoucherCode = strings.ToUpper(strings.TrimSpace(in.VoucherCode))

	// Kandidat potongan minimal belanja terbaik.
	var minPromo *CartPromo
	minDisc := 0
	for i := range active {
		p := active[i]
		if p.Type != PromoMinSpendNominal || p.Value <= 0 || base < p.MinSpend {
			continue
		}
		if d := minInt(p.Value, base); d > minDisc {
			minDisc, minPromo = d, &active[i]
		}
	}

	// Kandidat voucher.
	var vPromo *CartPromo
	vDisc := 0
	if res.VoucherCode != "" {
		vPromo, vDisc, res.VoucherMessage = evalVoucher(res.VoucherCode, active, base, hasMember, cfg)
	}

	choice := in.Choice
	if choice == "" {
		choice = ChoiceAuto
	}
	useVoucher := false
	switch choice {
	case ChoiceVoucher:
		useVoucher = vPromo != nil
		if res.VoucherCode == "" {
			res.Warnings = append(res.Warnings, "Pilihan voucher dipilih tetapi kode voucher kosong.")
		}
	case ChoiceMinSpend:
		useVoucher = false
		if vPromo != nil {
			res.VoucherMessage = "Voucher tidak dipakai karena kasir memilih potongan minimal belanja."
		}
	default: // AUTO: yang paling menguntungkan pelanggan; seri -> min. belanja (hemat kuota voucher)
		useVoucher = vPromo != nil && vDisc > minDisc
		if vPromo != nil && !useVoucher {
			if minPromo != nil {
				res.VoucherMessage = "Voucher tidak dipakai karena potongan minimal belanja lebih menguntungkan."
			} else {
				res.VoucherMessage = "Voucher tidak memberi potongan."
			}
		}
	}

	switch {
	case useVoucher:
		res.SpendOrVoucherDisc = vDisc
		res.SpendOrVoucherKind = ChoiceVoucher
		res.VoucherApplied = true
		res.VoucherMessage = fmt.Sprintf("Voucher %s dipakai.", vPromo.Code)
		res.Promos = append(res.Promos, AppliedPromo{PromoID: vPromo.ID, Name: vPromo.Name, Type: vPromo.Type, Code: vPromo.Code, Amount: vDisc})
	case choice != ChoiceVoucher && minPromo != nil:
		res.SpendOrVoucherDisc = minDisc
		res.SpendOrVoucherKind = ChoiceMinSpend
		res.Promos = append(res.Promos, AppliedPromo{PromoID: minPromo.ID, Name: minPromo.Name, Type: minPromo.Type, Amount: minDisc})
	}
}

func evalVoucher(code string, active []CartPromo, base int, hasMember bool, cfg CartConfig) (*CartPromo, int, string) {
	var v *CartPromo
	for i := range active {
		if active[i].Type == PromoVoucher && strings.EqualFold(strings.TrimSpace(active[i].Code), code) {
			v = &active[i]
			break
		}
	}
	switch {
	case v == nil:
		return nil, 0, "Kode voucher tidak ditemukan atau sedang tidak berlaku."
	case cfg.VoucherRequiresMember && !hasMember:
		return nil, 0, "Voucher hanya berlaku untuk member."
	case v.Quota > 0 && v.Used >= v.Quota:
		return nil, 0, "Kuota voucher sudah habis."
	case base < v.MinSpend:
		return nil, 0, fmt.Sprintf("Minimal belanja voucher adalah %s.", formatRupiah(v.MinSpend))
	}
	d := v.Value
	if v.ValueIsPercent {
		pct := minInt(v.Value, 100)
		d = base * pct / 100
	}
	d = minInt(d, base)
	if d <= 0 {
		return nil, 0, "Voucher tidak memberi potongan."
	}
	return v, d, ""
}

// redeemPoints mengisi langkah 5: maks 50% dari total setelah promo, tidak melebihi saldo.
func redeemPoints(res *CartResult, in CartInput, hasMember bool, cfg CartConfig) {
	if !hasMember {
		if in.PointsToRedeem > 0 || in.RedeemMaxPoints {
			res.Warnings = append(res.Warnings, "Tukar poin hanya untuk member terdaftar.")
			res.PointsAdjusted = true
		}
		return
	}
	maxByRule := res.TotalAfterPromo * cfg.MaxRedeemPercent / 100 / cfg.RupiahPerPoint
	balance := in.PointBalance
	if balance < 0 {
		balance = 0
	}
	maxPts := minInt(maxByRule, balance)
	res.MaxRedeemablePoints = maxPts

	req := in.PointsToRedeem
	if in.RedeemMaxPoints {
		req = maxPts
	}
	if req < 0 {
		req = 0
		res.PointsAdjusted = true
		res.Warnings = append(res.Warnings, "Jumlah poin tidak boleh negatif; diubah menjadi 0.")
	}
	if req > maxPts {
		res.PointsAdjusted = true
		if req > balance {
			res.Warnings = append(res.Warnings, fmt.Sprintf("Saldo poin hanya %d; penukaran disesuaikan menjadi %d poin.", balance, maxPts))
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf("Penukaran dibatasi %d%% dari total; disesuaikan menjadi %d poin.", cfg.MaxRedeemPercent, maxPts))
		}
		req = maxPts
	}
	res.PointsRedeemed = req
	res.PointsDiscount = req * cfg.RupiahPerPoint
}
