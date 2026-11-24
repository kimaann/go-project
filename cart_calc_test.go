package main

// Unit test mesin hitung keranjang (CalculateCart) dan pratinjau keranjang.
// Semua tes murni (tanpa database): jalankan dengan `go test -run Cart ./...`.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// testCartConfig memakai angka rancangan (Rp1.000 per poin, 1 poin per Rp10.000, poin dari total
// setelah promo) agar angka di test tetap mudah dihitung dan tidak ikut berubah bila konstanta
// poin proyek diubah. Kesesuaian dengan konstanta proyek diuji terpisah.
func testCartConfig() CartConfig {
	return CartConfig{RupiahPerPoint: 1000, EarnPerRupiah: 10000, MaxRedeemPercent: 50, EarnBeforeRedeem: true}
}

func calcT(in CartInput) (CartResult, error) {
	if in.Config == nil {
		c := testCartConfig()
		in.Config = &c
	}
	return CalculateCart(in)
}

var wib = time.FixedZone("WIB", 7*3600)

func at(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, wib)
}

func dateOf(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, wib)
	return &t
}

func line(id int, name, cat string, price, qty int) CartLineInput {
	return CartLineInput{ProductID: id, Name: name, Category: cat, Price: price, Qty: qty}
}

func eqInt(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", name, got, want)
	}
}

func mustCalc(t *testing.T, in CartInput) CartResult {
	t.Helper()
	res, err := calcT(in)
	if err != nil {
		t.Fatalf("CalculateCart error: %v", err)
	}
	return res
}

var noon = at(2026, time.November, 24, 12, 0)

func b2g1(id int64, cat string) CartPromo {
	return CartPromo{ID: id, Name: "B2G1 " + cat, Type: PromoBuy2Get1, Categories: []string{cat}, Active: true}
}

func pctPromo(id int64, pct int, cat string) CartPromo {
	return CartPromo{ID: id, Name: "Diskon", Type: PromoProductPercent, Value: pct, Categories: []string{cat}, Active: true}
}

func minSpend(id int64, value, min int) CartPromo {
	return CartPromo{ID: id, Name: "Min belanja", Type: PromoMinSpendNominal, Value: value, MinSpend: min, Active: true}
}

func voucher(id int64, code string, value int, percent bool) CartPromo {
	return CartPromo{ID: id, Name: "Voucher " + code, Type: PromoVoucher, Code: code, Value: value, ValueIsPercent: percent, Active: true}
}

// ---------- Urutan perhitungan ----------

func TestCartCalc_TanpaPromoTanpaMember(t *testing.T) {
	res := mustCalc(t, CartInput{Branch: "Seduh A", At: noon, Items: []CartLineInput{
		line(1, "Latte", "Kopi", 25000, 2), line(2, "Croissant", "Makanan", 22000, 1),
	}})
	eqInt(t, "subtotal", res.Subtotal, 72000)
	eqInt(t, "total bayar", res.TotalPayable, 72000)
	eqInt(t, "poin didapat (tanpa member)", res.PointsEarned, 0)
}

// Meniru sketsa halaman POS: Latte x2 + Croissant, promo produk, potongan min. belanja, tukar 15 poin.
func TestCartCalc_UrutanLengkap(t *testing.T) {
	res := mustCalc(t, CartInput{
		Branch: "Seduh A", At: noon,
		Items:    []CartLineInput{line(1, "Latte", "Kopi", 25000, 2), line(2, "Croissant", "Makanan", 22000, 1)},
		Promos:   []CartPromo{pctPromo(1, 10, "Kopi"), minSpend(2, 10000, 50000)},
		MemberID: "m1", Tier: TierSilver, PointBalance: 38, PointsToRedeem: 15,
	})
	eqInt(t, "subtotal", res.Subtotal, 72000)
	eqInt(t, "promo produk", res.ProductPromoDiscount, 5000)
	eqInt(t, "min belanja", res.SpendOrVoucherDisc, 10000)
	eqInt(t, "total setelah promo", res.TotalAfterPromo, 57000)
	eqInt(t, "poin ditukar", res.PointsRedeemed, 15)
	eqInt(t, "nilai poin", res.PointsDiscount, 15000)
	eqInt(t, "total bayar", res.TotalPayable, 42000) // sesuai sketsa: Rp 42.000
	eqInt(t, "poin didapat", res.PointsEarned, 5)    // floor(57000/10000) x 1
}

// ---------- Beli 2 gratis 1 (B2G1) ----------

func TestCartCalc_B2G1(t *testing.T) {
	cases := []struct {
		name     string
		qty      int
		wantFree int
	}{
		{"1 item", 1, 0},
		{"2 item", 2, 0},
		{"3 item -> 1 gratis", 3, 1},
		{"5 item -> floor(5/3)=1 gratis", 5, 1},
		{"6 item -> 2 gratis", 6, 2},
		{"8 item -> 2 gratis", 8, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := mustCalc(t, CartInput{Branch: "A", At: noon,
				Items:  []CartLineInput{line(1, "Latte", "Kopi", 30000, c.qty)},
				Promos: []CartPromo{b2g1(1, "Kopi")}})
			eqInt(t, "free qty", res.Lines[0].FreeQty, c.wantFree)
			eqInt(t, "diskon", res.ProductPromoDiscount, c.wantFree*30000)
			eqInt(t, "total", res.TotalPayable, (c.qty-c.wantFree)*30000)
		})
	}
}

func TestCartCalc_B2G1_UnitTermurahDigratiskan(t *testing.T) {
	// 2 Espresso + 1 Cappuccino sekategori: 3 unit -> 1 gratis, yang termurah (Espresso).
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "Cappuccino", "Kopi", 35000, 1), line(2, "Espresso", "Kopi", 25000, 2)},
		Promos: []CartPromo{b2g1(1, "Kopi")}})
	eqInt(t, "subtotal", res.Subtotal, 85000)
	eqInt(t, "diskon", res.ProductPromoDiscount, 25000)
	eqInt(t, "free Cappuccino", res.Lines[0].FreeQty, 0)
	eqInt(t, "free Espresso", res.Lines[1].FreeQty, 1)
	eqInt(t, "total bayar", res.TotalPayable, 60000)
}

func TestCartCalc_B2G1_LintasProdukDanKategoriLain(t *testing.T) {
	// 2 kopi + 1 makanan: kategori Kopi hanya 2 unit -> tidak ada gratis walau keranjang 3 item.
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "Latte", "Kopi", 30000, 2), line(2, "Croissant", "Makanan", 20000, 1)},
		Promos: []CartPromo{b2g1(1, "Kopi")}})
	eqInt(t, "diskon", res.ProductPromoDiscount, 0)
	// Dibatasi ke produk tertentu (ID), bukan kategori.
	p := CartPromo{ID: 2, Name: "B2G1 Croissant", Type: PromoBuy2Get1, ProductIDs: []int{2}, Active: true}
	res = mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "Latte", "Kopi", 30000, 3), line(2, "Croissant", "Makanan", 20000, 3)},
		Promos: []CartPromo{p}})
	eqInt(t, "diskon hanya croissant", res.ProductPromoDiscount, 20000)
	eqInt(t, "free Latte", res.Lines[0].FreeQty, 0)
}

// ---------- Promo bertumpuk ----------

func TestCartCalc_Tumpuk_B2G1LaluPersen(t *testing.T) {
	// 3 x 30.000: B2G1 -> gratis 30.000, sisa 60.000; lalu diskon 10% atas sisa = 6.000.
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "Latte", "Kopi", 30000, 3)},
		Promos: []CartPromo{pctPromo(2, 10, "Kopi"), b2g1(1, "Kopi")}}) // urutan input sengaja dibalik
	eqInt(t, "diskon item", res.Lines[0].Discount, 36000)
	eqInt(t, "total", res.TotalPayable, 54000)
	if len(res.Promos) != 2 || res.Promos[0].Type != PromoBuy2Get1 || res.Promos[0].Amount != 30000 || res.Promos[1].Amount != 6000 {
		t.Errorf("rincian promo tidak sesuai: %+v", res.Promos)
	}
}

func TestCartCalc_Tumpuk_DuaPersenPadaProdukSama(t *testing.T) {
	items := []CartLineInput{line(1, "V60", "Kopi", 100000, 1)}
	promos := []CartPromo{pctPromo(1, 20, "Kopi"), pctPromo(2, 10, "Kopi")}

	res := mustCalc(t, CartInput{Branch: "A", At: noon, Items: items, Promos: promos})
	// 20% = 20.000, lalu 10% dari sisa 80.000 = 8.000.
	eqInt(t, "tumpuk", res.ProductPromoDiscount, 28000)
	eqInt(t, "total", res.TotalPayable, 72000)

	cfg := testCartConfig()
	cfg.SingleProductPromoPerLine = true
	res = mustCalc(t, CartInput{Branch: "A", At: noon, Items: items, Promos: promos, Config: &cfg})
	eqInt(t, "tanpa tumpuk: hanya yang terbesar", res.ProductPromoDiscount, 20000)
}

func TestCartCalc_Tumpuk_PromoProdukDenganMinBelanja(t *testing.T) {
	// Promo produk boleh digabung dengan potongan min. belanja.
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "V60", "Kopi", 100000, 2)},
		Promos: []CartPromo{pctPromo(1, 10, "Kopi"), minSpend(2, 15000, 150000)}})
	eqInt(t, "promo produk", res.ProductPromoDiscount, 20000)
	eqInt(t, "min belanja (180rb >= 150rb)", res.SpendOrVoucherDisc, 15000)
	eqInt(t, "total", res.TotalAfterPromo, 165000)
}

func TestCartCalc_AmbangMinBelanjaDinilaiSetelahPromoProduk(t *testing.T) {
	// Subtotal 100.000 lolos min 60.000, tetapi setelah diskon 50% tinggal 50.000 -> tidak lolos.
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "V60", "Kopi", 100000, 1)},
		Promos: []CartPromo{pctPromo(1, 50, "Kopi"), minSpend(2, 10000, 60000)}})
	eqInt(t, "min belanja tidak berlaku", res.SpendOrVoucherDisc, 0)
	eqInt(t, "total", res.TotalPayable, 50000)
}

func TestCartCalc_MinBelanjaTerbaikDipilih(t *testing.T) {
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "V60", "Kopi", 100000, 1)},
		Promos: []CartPromo{minSpend(1, 5000, 50000), minSpend(2, 12000, 100000), minSpend(3, 30000, 200000)}})
	eqInt(t, "hanya satu min belanja (terbesar yang lolos)", res.SpendOrVoucherDisc, 12000)
	if len(res.Promos) != 1 || res.Promos[0].PromoID != 2 {
		t.Errorf("rincian promo: %+v", res.Promos)
	}
}

// ---------- Min. belanja ATAU voucher ----------

func voucherScenario(code string, choice DiscountChoice, v CartPromo, minValue int) CartResult {
	res, _ := calcT(CartInput{Branch: "A", At: noon,
		Items:       []CartLineInput{line(1, "V60", "Kopi", 100000, 1)},
		Promos:      []CartPromo{minSpend(1, minValue, 50000), v},
		VoucherCode: code, Choice: choice})
	return res
}

func TestCartCalc_VoucherAtauMinBelanja(t *testing.T) {
	t.Run("AUTO: voucher lebih besar -> voucher saja", func(t *testing.T) {
		res := voucherScenario("hemat", ChoiceAuto, voucher(9, "HEMAT", 15000, false), 10000)
		eqInt(t, "potongan (tidak digabung)", res.SpendOrVoucherDisc, 15000)
		if !res.VoucherApplied || res.SpendOrVoucherKind != ChoiceVoucher {
			t.Errorf("voucher seharusnya dipakai: %+v", res)
		}
		if len(res.Promos) != 1 {
			t.Errorf("hanya satu potongan yang boleh tercatat, dapat %d", len(res.Promos))
		}
	})
	t.Run("AUTO: min belanja lebih besar -> voucher tidak terpakai", func(t *testing.T) {
		res := voucherScenario("HEMAT", "", voucher(9, "HEMAT", 5000, false), 10000)
		eqInt(t, "potongan", res.SpendOrVoucherDisc, 10000)
		if res.VoucherApplied {
			t.Error("kuota voucher tidak boleh dikonsumsi")
		}
		if res.SpendOrVoucherKind != ChoiceMinSpend || res.VoucherMessage == "" {
			t.Errorf("harus min belanja dengan pesan: %+v", res)
		}
	})
	t.Run("AUTO: seri -> min belanja (hemat kuota voucher)", func(t *testing.T) {
		res := voucherScenario("HEMAT", ChoiceAuto, voucher(9, "HEMAT", 10000, false), 10000)
		if res.VoucherApplied {
			t.Error("pada seri, voucher tidak dipakai")
		}
	})
	t.Run("pilihan kasir MIN_SPEND walau voucher lebih besar", func(t *testing.T) {
		res := voucherScenario("HEMAT", ChoiceMinSpend, voucher(9, "HEMAT", 30000, false), 10000)
		eqInt(t, "potongan", res.SpendOrVoucherDisc, 10000)
		if res.VoucherApplied {
			t.Error("voucher tidak boleh terpakai")
		}
	})
	t.Run("pilihan kasir VOUCHER walau lebih kecil", func(t *testing.T) {
		res := voucherScenario("HEMAT", ChoiceVoucher, voucher(9, "HEMAT", 3000, false), 10000)
		eqInt(t, "potongan", res.SpendOrVoucherDisc, 3000)
		if !res.VoucherApplied {
			t.Error("voucher harus terpakai")
		}
	})
	t.Run("voucher persen dihitung dari total setelah promo produk", func(t *testing.T) {
		res, _ := calcT(CartInput{Branch: "A", At: noon,
			Items:       []CartLineInput{line(1, "V60", "Kopi", 100000, 1)},
			Promos:      []CartPromo{pctPromo(1, 20, "Kopi"), voucher(9, "PCT", 10, true)},
			VoucherCode: "PCT"})
		eqInt(t, "voucher 10% dari 80.000", res.SpendOrVoucherDisc, 8000)
		eqInt(t, "total", res.TotalAfterPromo, 72000)
	})
	t.Run("potongan tidak melebihi total", func(t *testing.T) {
		res, _ := calcT(CartInput{Branch: "A", At: noon,
			Items:       []CartLineInput{line(1, "Es", "Kopi", 20000, 1)},
			Promos:      []CartPromo{voucher(9, "BESAR", 50000, false)},
			VoucherCode: "BESAR"})
		eqInt(t, "potongan dibatasi", res.SpendOrVoucherDisc, 20000)
		eqInt(t, "total", res.TotalPayable, 0)
	})
}

func TestCartCalc_VoucherTidakValid(t *testing.T) {
	exp := voucher(1, "HABIS", 10000, false)
	exp.Quota, exp.Used = 5, 5
	min := voucher(2, "MINIM", 10000, false)
	min.MinSpend = 500000
	off := voucher(3, "MATI", 10000, false)
	off.Active = false
	cases := map[string]string{
		"HABIS": "Kuota", "MINIM": "Minimal belanja", "MATI": "tidak berlaku", "NGAWUR": "tidak ditemukan",
	}
	for code, wantMsg := range cases {
		t.Run(code, func(t *testing.T) {
			res := mustCalc(t, CartInput{Branch: "A", At: noon,
				Items:       []CartLineInput{line(1, "V60", "Kopi", 100000, 1)},
				Promos:      []CartPromo{exp, min, off},
				VoucherCode: code})
			eqInt(t, "tanpa potongan", res.SpendOrVoucherDisc, 0)
			if res.VoucherApplied {
				t.Error("voucher tidak valid tidak boleh terpakai")
			}
			if !strings.Contains(res.VoucherMessage, wantMsg) {
				t.Errorf("pesan %q harus memuat %q", res.VoucherMessage, wantMsg)
			}
		})
	}
}

func TestCartCalc_VoucherTanpaMember(t *testing.T) {
	in := CartInput{Branch: "A", At: noon,
		Items:       []CartLineInput{line(1, "V60", "Kopi", 100000, 1)},
		Promos:      []CartPromo{voucher(1, "V1", 10000, false)},
		VoucherCode: "V1"}
	res := mustCalc(t, in)
	eqInt(t, "default: voucher boleh tanpa member", res.SpendOrVoucherDisc, 10000)
	eqInt(t, "tanpa member tidak dapat poin", res.PointsEarned, 0)

	cfg := testCartConfig()
	cfg.VoucherRequiresMember = true
	in.Config = &cfg
	res = mustCalc(t, in)
	eqInt(t, "wajib member: ditolak", res.SpendOrVoucherDisc, 0)
}

// ---------- Tukar poin ----------

func TestCartCalc_TukarPoin(t *testing.T) {
	base := func() CartInput {
		return CartInput{Branch: "A", At: noon, MemberID: "m1", Tier: TierSilver,
			Items: []CartLineInput{line(1, "Latte", "Kopi", 20000, 2)}} // total 40.000 -> maks 20 poin
	}
	t.Run("dalam batas", func(t *testing.T) {
		in := base()
		in.PointBalance, in.PointsToRedeem = 100, 10
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 10)
		eqInt(t, "total bayar", res.TotalPayable, 30000)
		if res.PointsAdjusted {
			t.Error("tidak ada koreksi")
		}
	})
	t.Run("dibatasi 50% total", func(t *testing.T) {
		in := base()
		in.PointBalance, in.PointsToRedeem = 100, 30
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 20)
		eqInt(t, "maks", res.MaxRedeemablePoints, 20)
		eqInt(t, "total bayar", res.TotalPayable, 20000)
		if !res.PointsAdjusted || len(res.Warnings) == 0 {
			t.Error("harus ada koreksi dan peringatan")
		}
	})
	t.Run("dibatasi saldo", func(t *testing.T) {
		in := base()
		in.PointBalance, in.PointsToRedeem = 5, 15
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 5)
		if !strings.Contains(strings.Join(res.Warnings, " "), "Saldo poin") {
			t.Errorf("peringatan harus menyebut saldo: %v", res.Warnings)
		}
	})
	t.Run("tukar maksimal", func(t *testing.T) {
		in := base()
		in.PointBalance, in.RedeemMaxPoints = 12, true
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 12)
	})
	t.Run("batas dihitung dari total setelah promo, dibulatkan ke bawah", func(t *testing.T) {
		in := base()
		in.Items = []CartLineInput{line(1, "Latte", "Kopi", 25500, 1)} // 50% = 12.750 -> 12 poin
		in.PointBalance, in.PointsToRedeem = 100, 50
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 12)
		eqInt(t, "total bayar", res.TotalPayable, 13500)
	})
	t.Run("poin negatif", func(t *testing.T) {
		in := base()
		in.PointBalance, in.PointsToRedeem = 100, -3
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 0)
		if !res.PointsAdjusted {
			t.Error("harus ditandai dikoreksi")
		}
	})
	t.Run("tanpa member", func(t *testing.T) {
		in := base()
		in.MemberID, in.PointBalance, in.PointsToRedeem = "", 100, 10
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 0)
		eqInt(t, "total bayar", res.TotalPayable, 40000)
		if len(res.Warnings) == 0 {
			t.Error("harus ada peringatan")
		}
	})
	t.Run("batas memakai total setelah SEMUA promo", func(t *testing.T) {
		in := base()
		in.Promos = []CartPromo{pctPromo(1, 50, "Kopi")} // 40.000 -> 20.000 -> maks 10 poin
		in.PointBalance, in.PointsToRedeem = 100, 20
		res := mustCalc(t, in)
		eqInt(t, "ditukar", res.PointsRedeemed, 10)
		eqInt(t, "total bayar", res.TotalPayable, 10000)
	})
}

// ---------- Poin didapat ----------

func TestCartCalc_PoinDidapat(t *testing.T) {
	cases := []struct {
		name  string
		price int
		tier  MemberTier
		want  int
	}{
		{"Silver 57.000 -> 5", 57000, TierSilver, 5},
		{"Gold 57.000 -> 5 x 1,5 = 7,5 -> 7", 57000, TierGold, 7},
		{"Gold 30.000 -> 3 x 1,5 = 4,5 -> 4", 30000, TierGold, 4},
		{"Gold 40.000 -> 4 x 1,5 = 6", 40000, TierGold, 6},
		{"Gold 9.999 -> 0", 9999, TierGold, 0},
		{"Silver 10.000 -> 1", 10000, TierSilver, 1},
		{"tingkat kosong dianggap Silver", 30000, "", 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := mustCalc(t, CartInput{Branch: "A", At: noon, MemberID: "m1", Tier: c.tier,
				Items: []CartLineInput{line(1, "X", "Kopi", c.price, 1)}})
			eqInt(t, "poin", res.PointsEarned, c.want)
		})
	}
}

func TestCartCalc_PoinDidapatDariTotalSetelahPromoSebelumTukarPoin(t *testing.T) {
	in := CartInput{Branch: "A", At: noon, MemberID: "m1", Tier: TierSilver,
		Items:        []CartLineInput{line(1, "X", "Kopi", 100000, 1)},
		PointBalance: 50, PointsToRedeem: 50}
	res := mustCalc(t, in)
	eqInt(t, "tukar 50 poin", res.PointsRedeemed, 50)
	eqInt(t, "total bayar", res.TotalPayable, 50000)
	eqInt(t, "poin dari langkah 4 (100.000)", res.PointsEarned, 10)

	cfg := testCartConfig()
	cfg.EarnBeforeRedeem = false
	in.Config = &cfg
	res = mustCalc(t, in)
	eqInt(t, "alternatif: poin dari langkah 6 (50.000)", res.PointsEarned, 5)
}

// ---------- Jam / tanggal / cabang promo ----------

func TestCartPromo_IsActiveAt_Jam(t *testing.T) {
	day := func(h, m int) time.Time { return at(2026, time.May, 10, h, m) }
	same := CartPromo{Active: true, StartClock: "10:00", EndClock: "12:00"}
	night := CartPromo{Active: true, StartClock: "22:00", EndClock: "02:00"}
	cases := []struct {
		name string
		p    CartPromo
		t    time.Time
		want bool
	}{
		{"10:00-12:00 pukul 09:59", same, day(9, 59), false},
		{"10:00-12:00 pukul 10:00 (awal inklusif)", same, day(10, 0), true},
		{"10:00-12:00 pukul 11:59", same, day(11, 59), true},
		{"10:00-12:00 pukul 12:00 (akhir eksklusif)", same, day(12, 0), false},
		{"22:00-02:00 pukul 21:59", night, day(21, 59), false},
		{"22:00-02:00 pukul 22:00", night, day(22, 0), true},
		{"22:00-02:00 pukul 23:59", night, day(23, 59), true},
		{"22:00-02:00 pukul 00:00", night, day(0, 0), true},
		{"22:00-02:00 pukul 01:59", night, day(1, 59), true},
		{"22:00-02:00 pukul 02:00", night, day(2, 0), false},
		{"22:00-02:00 pukul 12:00", night, day(12, 0), false},
		{"format dengan detik", CartPromo{Active: true, StartClock: "22:00:00", EndClock: "02:00:00"}, day(23, 0), true},
		{"tanpa jam = seharian", CartPromo{Active: true}, day(3, 33), true},
		{"jam mulai = jam selesai = seharian", CartPromo{Active: true, StartClock: "08:00", EndClock: "08:00"}, day(3, 0), true},
		{"jam tidak lengkap -> tidak berlaku", CartPromo{Active: true, StartClock: "08:00"}, day(9, 0), false},
		{"format jam rusak -> tidak berlaku", CartPromo{Active: true, StartClock: "pagi", EndClock: "siang"}, day(9, 0), false},
		{"promo nonaktif", CartPromo{Active: false}, day(9, 0), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.p.IsActiveAt("A", c.t); got != c.want {
				t.Errorf("IsActiveAt = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCartPromo_IsActiveAt_LintasTengahMalamDanTanggal(t *testing.T) {
	// Promo 22:00-02:00 dengan periode 9-10 Mei.
	p := CartPromo{Active: true, StartClock: "22:00", EndClock: "02:00",
		StartDate: dateOf(2026, time.May, 9), EndDate: dateOf(2026, time.May, 10)}
	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"8 Mei 23:00 (sebelum periode)", at(2026, time.May, 8, 23, 0), false},
		{"9 Mei 01:00 (lanjutan 8 Mei, belum periode)", at(2026, time.May, 9, 1, 0), false},
		{"9 Mei 22:30 (hari pertama)", at(2026, time.May, 9, 22, 30), true},
		{"10 Mei 01:00 (lanjutan 9 Mei)", at(2026, time.May, 10, 1, 0), true},
		{"10 Mei 23:30 (hari terakhir)", at(2026, time.May, 10, 23, 30), true},
		{"11 Mei 01:00 (lanjutan hari terakhir, masih berlaku)", at(2026, time.May, 11, 1, 0), true},
		{"11 Mei 22:30 (setelah periode)", at(2026, time.May, 11, 22, 30), false},
		{"11 Mei 02:00 (jam selesai)", at(2026, time.May, 11, 2, 0), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := p.IsActiveAt("A", c.t); got != c.want {
				t.Errorf("IsActiveAt = %v, want %v", got, c.want)
			}
		})
	}
	// Pergantian bulan/tahun: 1 Jan 01:00 adalah lanjutan 31 Des.
	ny := CartPromo{Active: true, StartClock: "22:00", EndClock: "02:00", EndDate: dateOf(2026, time.December, 31)}
	if !ny.IsActiveAt("A", at(2027, time.January, 1, 1, 0)) {
		t.Error("1 Jan 01:00 harus masih lanjutan 31 Des")
	}
}

func TestCartPromo_IsActiveAt_Cabang(t *testing.T) {
	p := CartPromo{Active: true, Branches: []string{"Seduh A", "Seduh B"}}
	if !p.IsActiveAt("seduh a", noon) {
		t.Error("cabang tidak peka huruf besar/kecil")
	}
	if p.IsActiveAt("Seduh C", noon) {
		t.Error("cabang lain harus ditolak")
	}
	if !(CartPromo{Active: true}).IsActiveAt("Seduh C", noon) {
		t.Error("tanpa cabang = semua cabang")
	}
}

func TestCartCalc_PromoDiLuarJamTidakDipakai(t *testing.T) {
	happy := pctPromo(1, 50, "Kopi")
	happy.StartClock, happy.EndClock = "22:00", "02:00"
	items := []CartLineInput{line(1, "Latte", "Kopi", 40000, 1)}

	res := mustCalc(t, CartInput{Branch: "A", At: at(2026, time.November, 24, 15, 0), Items: items, Promos: []CartPromo{happy}})
	eqInt(t, "siang: tanpa diskon", res.TotalPayable, 40000)

	res = mustCalc(t, CartInput{Branch: "A", At: at(2026, time.November, 25, 0, 30), Items: items, Promos: []CartPromo{happy}})
	eqInt(t, "lewat tengah malam: diskon 50%", res.TotalPayable, 20000)
}

func TestCartCalc_PromoCabangLain(t *testing.T) {
	p := pctPromo(1, 90, "Kopi")
	p.Branches = []string{"Seduh B"}
	res := mustCalc(t, CartInput{Branch: "Seduh A", At: noon,
		Items: []CartLineInput{line(1, "Latte", "Kopi", 40000, 1)}, Promos: []CartPromo{p}})
	eqInt(t, "promo cabang lain diabaikan", res.TotalPayable, 40000)
}

// ---------- Skenario gabungan ----------

func complexScenario(at time.Time) CartInput {
	night := pctPromo(2, 50, "Makanan")
	night.StartClock, night.EndClock = "22:00", "02:00"
	other := pctPromo(5, 90, "Kopi")
	other.Branches = []string{"Seduh B"}
	expired := pctPromo(6, 70, "Kopi")
	expired.EndDate = dateOf(2026, time.October, 31)
	return CartInput{
		Branch: "Seduh A", At: at,
		Items: []CartLineInput{
			line(1, "Latte", "Kopi", 40000, 3),
			line(2, "Espresso", "Kopi", 25000, 1),
			line(3, "Croissant", "Makanan", 32000, 2),
		},
		Promos: []CartPromo{
			b2g1(1, "Kopi"), night, minSpend(3, 20000, 150000),
			voucher(4, "HEMAT10", 10, true), other, expired,
		},
		VoucherCode: "hemat10",
		MemberID:    "m1", Tier: TierGold, PointBalance: 200, PointsToRedeem: 100,
	}
}

func TestCartCalc_SkenarioGabungan_Malam(t *testing.T) {
	res := mustCalc(t, complexScenario(at(2026, time.November, 24, 23, 30)))
	eqInt(t, "subtotal", res.Subtotal, 209000)
	// Kopi: 3 Latte + 1 Espresso = 4 unit -> 1 gratis (Espresso, termurah) = 25.000
	// Makanan: Croissant 64.000 x 50% (jam malam) = 32.000
	eqInt(t, "promo produk", res.ProductPromoDiscount, 57000)
	// Setelah promo produk 152.000 (>= 150.000): min belanja 20.000 > voucher 10% = 15.200
	eqInt(t, "min belanja vs voucher", res.SpendOrVoucherDisc, 20000)
	if res.VoucherApplied || res.SpendOrVoucherKind != ChoiceMinSpend {
		t.Errorf("seharusnya min belanja, voucher tidak terpakai: %+v", res)
	}
	eqInt(t, "total setelah promo", res.TotalAfterPromo, 132000)
	// Batas 50% = 66.000 = 66 poin (saldo 200 cukup); diminta 100 -> dikoreksi
	eqInt(t, "poin ditukar", res.PointsRedeemed, 66)
	eqInt(t, "total bayar", res.TotalPayable, 66000)
	// floor(132000/10000)=13 x 1,5 = 19,5 -> 19
	eqInt(t, "poin didapat (Gold)", res.PointsEarned, 19)
	eqInt(t, "free Espresso", res.Lines[1].FreeQty, 1)
	eqInt(t, "diskon Croissant", res.Lines[2].Discount, 32000)

	// Konsistensi: jumlah potongan tercatat = selisih subtotal dan total setelah promo.
	sum := 0
	for _, p := range res.Promos {
		sum += p.Amount
	}
	eqInt(t, "jumlah transaction_promos", sum, res.Subtotal-res.TotalAfterPromo)
	// Jumlah diskon per item = total promo produk.
	lineSum := 0
	for _, l := range res.Lines {
		lineSum += l.Discount
	}
	eqInt(t, "jumlah diskon item", lineSum, res.ProductPromoDiscount)
}

func TestCartCalc_SkenarioGabungan_Siang(t *testing.T) {
	res := mustCalc(t, complexScenario(at(2026, time.November, 24, 15, 0)))
	eqInt(t, "promo produk (tanpa diskon malam)", res.ProductPromoDiscount, 25000)
	eqInt(t, "min belanja", res.SpendOrVoucherDisc, 20000) // 184.000 >= 150.000
	eqInt(t, "total setelah promo", res.TotalAfterPromo, 164000)
	eqInt(t, "poin ditukar (maks 82)", res.PointsRedeemed, 82)
	eqInt(t, "total bayar", res.TotalPayable, 82000)
	eqInt(t, "poin didapat", res.PointsEarned, 24) // 16 x 1,5
}

// ---------- Kasus tepi ----------

func TestCartCalc_ProdukIDGandaDigabung(t *testing.T) {
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "Latte", "Kopi", 30000, 1), line(1, "Latte", "Kopi", 30000, 2)},
		Promos: []CartPromo{b2g1(1, "Kopi")}})
	if len(res.Lines) != 1 || res.Lines[0].Qty != 3 {
		t.Fatalf("baris harus digabung: %+v", res.Lines)
	}
	eqInt(t, "gratis 1 dari gabungan 3", res.ProductPromoDiscount, 30000)
}

func TestCartCalc_KeranjangKosongDanTidakValid(t *testing.T) {
	res := mustCalc(t, CartInput{Branch: "A", At: noon, MemberID: "m1", PointBalance: 10, PointsToRedeem: 5})
	eqInt(t, "total kosong", res.TotalPayable, 0)
	eqInt(t, "tidak ada poin ditukar", res.PointsRedeemed, 0)
	if res.Lines == nil || res.Promos == nil || res.Warnings == nil {
		t.Error("slice hasil tidak boleh nil (JSON harus [] bukan null)")
	}
	for _, it := range []CartLineInput{line(1, "X", "Kopi", 1000, 0), line(1, "X", "Kopi", 1000, -1), line(1, "X", "Kopi", -5, 1)} {
		if _, err := calcT(CartInput{Branch: "A", At: noon, Items: []CartLineInput{it}}); !errors.Is(err, ErrInvalidCartItem) {
			t.Errorf("item %+v harus ditolak, err=%v", it, err)
		}
	}
}

func TestCartCalc_PromoPenuhGratisTotalNol(t *testing.T) {
	res := mustCalc(t, CartInput{Branch: "A", At: noon, MemberID: "m1", Tier: TierGold, PointBalance: 100, PointsToRedeem: 10,
		Items:  []CartLineInput{line(1, "Latte", "Kopi", 30000, 1)},
		Promos: []CartPromo{pctPromo(1, 100, "Kopi")}})
	eqInt(t, "total", res.TotalPayable, 0)
	eqInt(t, "tidak bisa tukar poin", res.PointsRedeemed, 0)
	eqInt(t, "tidak dapat poin", res.PointsEarned, 0)
}

func TestCartCalc_PersenDibulatkanKeBawah(t *testing.T) {
	res := mustCalc(t, CartInput{Branch: "A", At: noon,
		Items:  []CartLineInput{line(1, "X", "Kopi", 33333, 1)},
		Promos: []CartPromo{pctPromo(1, 15, "Kopi")}}) // 4999,95 -> 4999
	eqInt(t, "diskon", res.ProductPromoDiscount, 4999)
}

// Konfigurasi bawaan harus selaras dengan modul poin yang sudah ada (main.go).
func TestCartCalc_KonfigBawaanSelarasModulPoin(t *testing.T) {
	d := DefaultCartConfig()
	eqInt(t, "Rp per poin", d.RupiahPerPoint, RupiahPerPoint)
	eqInt(t, "Rp per poin didapat", d.EarnPerRupiah, PointsPerRupiah)
	eqInt(t, "batas tukar", d.MaxRedeemPercent, 50)

	// Config nil / nol memakai bawaan: 1 poin = Rp50, 1 poin per Rp1.000 dari total bayar.
	for _, cfg := range []*CartConfig{nil, {}} {
		res, err := CalculateCart(CartInput{Branch: "A", At: noon, MemberID: "m1", Tier: TierSilver,
			PointBalance: 100, PointsToRedeem: 100, Config: cfg,
			Items: []CartLineInput{line(1, "X", "Kopi", 40000, 1)}})
		if err != nil {
			t.Fatal(err)
		}
		// Batas 50% = Rp20.000 = 400 poin, tetapi saldo hanya 100 -> 100 poin = Rp5.000.
		eqInt(t, "poin ditukar", res.PointsRedeemed, 100)
		eqInt(t, "nilai poin", res.PointsDiscount, 100*RupiahPerPoint)
		eqInt(t, "total bayar", res.TotalPayable, 40000-100*RupiahPerPoint)
		// Seperti checkout lama: floor(total bayar 35.000 / 1.000) = 35 poin.
		eqInt(t, "poin didapat", res.PointsEarned, 35000/PointsPerRupiah)
	}
}

// ---------- Pratinjau (tanpa database) ----------

func stubCartDeps(t *testing.T) {
	t.Helper()
	oldNow, oldProd, oldPromos, oldTier, oldBal, oldV := cartNow, cartLoadProducts, cartActivePromos, cartMemberTier, cartPointBalance, cartLoadVoucher
	t.Cleanup(func() {
		cartNow, cartLoadProducts, cartActivePromos, cartMemberTier, cartPointBalance, cartLoadVoucher = oldNow, oldProd, oldPromos, oldTier, oldBal, oldV
	})
	oldCfg := cartConfig
	t.Cleanup(func() { cartConfig = oldCfg })
	cartConfig = testCartConfig()
	cartNow = func() time.Time { return at(2026, time.November, 24, 12, 0) }
	cartLoadProducts = func(ctx context.Context, ids []int) (map[int]ProductDB, error) {
		all := map[int]ProductDB{
			1: {ID: 1, Name: "Latte", Category: "Kopi", Price: 40000, IsActive: true},
			2: {ID: 2, Name: "Croissant", Category: "Makanan", Price: 32000, IsActive: true},
			3: {ID: 3, Name: "Lama", Category: "Kopi", Price: 10000, IsActive: false},
		}
		out := map[int]ProductDB{}
		for _, id := range ids {
			if p, ok := all[id]; ok {
				out[id] = p
			}
		}
		return out, nil
	}
	cartActivePromos = func(ctx context.Context, branch string, at time.Time) ([]CartPromo, error) {
		return []CartPromo{b2g1(1, "Kopi")}, nil
	}
	cartMemberTier = func(ctx context.Context, id string, at time.Time) (MemberTier, error) { return TierGold, nil }
	cartPointBalance = func(ctx context.Context, id string, at time.Time) (int, error) { return 30, nil }
	cartLoadVoucher = func(ctx context.Context, code string, at time.Time) (*CartPromo, error) {
		if code == "HEMAT" {
			v := voucher(9, "HEMAT", 10000, false)
			return &v, nil
		}
		return nil, nil
	}
}

func TestCartPreview_Sukses(t *testing.T) {
	stubCartDeps(t)
	res, status, msg := buildCartPreview(context.Background(), cartPreviewRequest{
		Branch: "Seduh A", MemberID: "m1", VoucherCode: " hemat ", PointsUsed: 100,
		Items: []cartPreviewItem{{1, 2}, {1, 1}, {2, 1}}, // Latte digabung menjadi 3
	})
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, msg)
	}
	eqInt(t, "subtotal (harga dari DB)", res.Subtotal, 152000)
	eqInt(t, "B2G1 3 Latte", res.ProductPromoDiscount, 40000)
	eqInt(t, "voucher", res.SpendOrVoucherDisc, 10000)
	eqInt(t, "total setelah promo", res.TotalAfterPromo, 102000)
	eqInt(t, "poin ditukar dibatasi saldo 30", res.PointsRedeemed, 30)
	eqInt(t, "total bayar", res.TotalPayable, 72000)
	eqInt(t, "poin Gold 10 x 1,5", res.PointsEarned, 15)
	if !res.VoucherApplied {
		t.Error("voucher harus terpakai")
	}
}

func TestCartPreview_Validasi(t *testing.T) {
	stubCartDeps(t)
	cases := map[string]cartPreviewRequest{
		"cabang kosong":        {Items: []cartPreviewItem{{1, 1}}},
		"qty nol":              {Branch: "A", Items: []cartPreviewItem{{1, 0}}},
		"produk tidak ada":     {Branch: "A", Items: []cartPreviewItem{{99, 1}}},
		"produk nonaktif":      {Branch: "A", Items: []cartPreviewItem{{3, 1}}},
		"qty kebanyakan":       {Branch: "A", Items: []cartPreviewItem{{1, 1001}}},
		"qty gabungan terlalu": {Branch: "A", Items: []cartPreviewItem{{1, 600}, {1, 600}}},
		"pilihan tak dikenal":  {Branch: "A", DiscountChoice: "BEBAS", Items: []cartPreviewItem{{1, 1}}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, status, _ := buildCartPreview(context.Background(), req); status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
		})
	}
}

func TestCartPreview_KeranjangKosongOK(t *testing.T) {
	stubCartDeps(t)
	res, status, _ := buildCartPreview(context.Background(), cartPreviewRequest{Branch: "A"})
	if status != http.StatusOK || res.TotalPayable != 0 {
		t.Errorf("keranjang kosong harus OK bernilai 0, status=%d res=%+v", status, res)
	}
}

func TestCartPreview_ErrorSumberData(t *testing.T) {
	stubCartDeps(t)
	cartPointBalance = func(ctx context.Context, id string, at time.Time) (int, error) { return 0, errors.New("db mati") }
	_, status, _ := buildCartPreview(context.Background(), cartPreviewRequest{Branch: "A", MemberID: "m1", Items: []cartPreviewItem{{1, 1}}})
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
}

// ---------- Simulasi waktu & promo demo ----------

func TestCartPreview_SimulasiWaktu(t *testing.T) {
	stubCartDeps(t)
	night := pctPromo(2, 50, "Kopi")
	night.StartClock, night.EndClock = "22:00", "02:00"
	cartActivePromos = func(ctx context.Context, branch string, at time.Time) ([]CartPromo, error) {
		return []CartPromo{night}, nil
	}
	req := cartPreviewRequest{Branch: "A", Items: []cartPreviewItem{{1, 1}}}

	res, status, msg := buildCartPreview(context.Background(), req)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, msg)
	}
	eqInt(t, "siang (12.00): tanpa diskon", res.TotalPayable, 40000)

	req.At = "2026-11-24T23:15"
	res, _, _ = buildCartPreview(context.Background(), req)
	eqInt(t, "malam (23.15): diskon 50%", res.TotalPayable, 20000)

	req.At = "2026-11-25T01:30"
	res, _, _ = buildCartPreview(context.Background(), req)
	eqInt(t, "dini hari (01.30): diskon 50%", res.TotalPayable, 20000)

	req.At = "besok pagi"
	if _, status, _ = buildCartPreview(context.Background(), req); status != http.StatusBadRequest {
		t.Errorf("format waktu salah harus 400, dapat %d", status)
	}
}

func TestCartPreview_PromoDemo(t *testing.T) {
	t.Setenv("CART_DEMO_PROMOS", "1")
	if !cartDemoPromosEnabled() {
		t.Fatal("mode demo harus aktif")
	}
	promos := demoCartPromos()
	// Siang pukul 15.00: B2G1 Kopi dan Happy Hour Makanan berlaku, promo malam tidak.
	res := mustCalc(t, CartInput{Branch: "Seduh A", At: at(2026, time.November, 24, 15, 0), Promos: promos,
		Items: []CartLineInput{
			line(1, "Latte", "Kopi", 30000, 3),
			line(2, "Croissant", "Makanan", 20000, 2),
			line(3, "Matcha", "Non-Kopi", 25000, 1),
		}})
	eqInt(t, "subtotal", res.Subtotal, 155000)
	eqInt(t, "B2G1 (30.000) + happy hour 20% (8.000)", res.ProductPromoDiscount, 38000)
	eqInt(t, "min belanja: 117.000 < 150.000 -> tidak berlaku", res.SpendOrVoucherDisc, 0)
	// Malam 23.00: Non-Kopi 30% berlaku, Happy Hour tidak.
	res = mustCalc(t, CartInput{Branch: "Seduh A", At: at(2026, time.November, 24, 23, 0), Promos: promos,
		Items: []CartLineInput{line(3, "Matcha", "Non-Kopi", 25000, 2), line(2, "Croissant", "Makanan", 20000, 5)}})
	eqInt(t, "malam: 30% x 50.000", res.ProductPromoDiscount, 15000)
	eqInt(t, "min belanja: 135.000 < 150.000", res.SpendOrVoucherDisc, 0)
	res = mustCalc(t, CartInput{Branch: "Seduh A", At: at(2026, time.November, 24, 23, 0), Promos: promos,
		Items: []CartLineInput{line(3, "Matcha", "Non-Kopi", 25000, 2), line(2, "Croissant", "Makanan", 20000, 6)}})
	eqInt(t, "min belanja tercapai (150.000-15.000=155.000 >= 150.000)", res.SpendOrVoucherDisc, 15000)

	t.Setenv("CART_DEMO_PROMOS", "")
	if cartDemoPromosEnabled() {
		t.Error("tanpa env, mode demo harus mati")
	}
}
