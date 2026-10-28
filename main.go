package main

import (
	"html/template"
	"log"
	"net/http"
)

// ===== Data Structs =====

type MenuItem struct {
	Name     string
	Desc     string
	Price    int
	Category string
	Icon     string
}

type Branch struct {
	Name    string
	Address string
	City    string
	Hours   string
	Phone   string
	Maps    string
}

type Promo struct {
	Title     string
	Desc      string
	Code      string
	Until     string
	Badge     string
	BadgeType string // "hot" | "new" | "limited"
}

type PointTier struct {
	Name     string
	MinPoint int
	Benefit  string
	Icon     string
}

type PageData struct {
	Title     string
	ActiveNav string
}

type MenuPage struct {
	PageData
	Categories []string
	Items      []MenuItem
}

type BranchPage struct {
	PageData
	Branches []Branch
}

type PromoPage struct {
	PageData
	Promos []Promo
}

type PointPage struct {
	PageData
	Tiers []PointTier
}

// ===== Static Data =====

var menuItems = []MenuItem{
	// Kopi
	{Name: "Espresso", Desc: "Shot espresso pekat dengan crema sempurna", Price: 25000, Category: "Kopi", Icon: "☕"},
	{Name: "Americano", Desc: "Espresso dengan air panas, bold dan clean", Price: 28000, Category: "Kopi", Icon: "☕"},
	{Name: "Cappuccino", Desc: "Espresso, steamed milk, dan foam tebal", Price: 35000, Category: "Kopi", Icon: "☕"},
	{Name: "Caramel Latte", Desc: "Latte manis dengan drizzle karamel premium", Price: 40000, Category: "Kopi", Icon: "☕"},
	{Name: "V60 Single Origin", Desc: "Manual brew pilihan biji kopi lokal terbaik", Price: 45000, Category: "Kopi", Icon: "☕"},
	{Name: "Cold Brew", Desc: "Diseduh dingin 16 jam, smooth dan kaya rasa", Price: 38000, Category: "Kopi", Icon: "🧊"},
	// Non-Kopi
	{Name: "Matcha Latte", Desc: "Matcha premium Jepang dengan susu oat", Price: 38000, Category: "Non-Kopi", Icon: "🍵"},
	{Name: "Chocolate Hazelnut", Desc: "Dark chocolate blend dengan hazelnut roasted", Price: 38000, Category: "Non-Kopi", Icon: "🍫"},
	{Name: "Taro Latte", Desc: "Taro creamy dengan topping bubble pearl", Price: 36000, Category: "Non-Kopi", Icon: "🫧"},
	{Name: "Sparkling Lemon", Desc: "Soda lemon segar dengan mint dan basil", Price: 32000, Category: "Non-Kopi", Icon: "🍋"},
	// Makanan
	{Name: "Croissant Butter", Desc: "Croissant lapis-lapis renyah, dipanggang segar", Price: 32000, Category: "Makanan", Icon: "🥐"},
	{Name: "Banana Bread", Desc: "Banana bread lembut dengan walnut dan madu", Price: 28000, Category: "Makanan", Icon: "🍞"},
	{Name: "Avocado Toast", Desc: "Sourdough panggang, avocado, telur, dan microgreens", Price: 48000, Category: "Makanan", Icon: "🥑"},
	{Name: "Granola Bowl", Desc: "Granola homemade, yogurt Greek, dan buah segar", Price: 42000, Category: "Makanan", Icon: "🥣"},
}

var branches = []Branch{
	{
		Name:    "Kopiku — Sudirman",
		Address: "Jl. Jend. Sudirman No. 12, Gedung Kopi Tower Lt. 1",
		City:    "Jakarta Pusat",
		Hours:   "07.00 – 22.00",
		Phone:   "(021) 5555-0101",
		Maps:    "#",
	},
	{
		Name:    "Kopiku — Kemang",
		Address: "Jl. Kemang Raya No. 48",
		City:    "Jakarta Selatan",
		Hours:   "08.00 – 23.00",
		Phone:   "(021) 5555-0202",
		Maps:    "#",
	},
	{
		Name:    "Kopiku — Dago",
		Address: "Jl. Ir. H. Djuanda No. 99 (Dago)",
		City:    "Bandung",
		Hours:   "07.00 – 22.00",
		Phone:   "(022) 5555-0303",
		Maps:    "#",
	},
	{
		Name:    "Kopiku — Sagan",
		Address: "Jl. Colombo No. 15, Sagan",
		City:    "Yogyakarta",
		Hours:   "08.00 – 22.00",
		Phone:   "(0274) 555-0404",
		Maps:    "#",
	},
	{
		Name:    "Kopiku — Tunjungan",
		Address: "Tunjungan Plaza 6, Jl. Embong Malang No. 1",
		City:    "Surabaya",
		Hours:   "09.00 – 22.00",
		Phone:   "(031) 5555-0505",
		Maps:    "#",
	},
	{
		Name:    "Kopiku — Renon",
		Address: "Jl. Raya Puputan No. 77, Renon",
		City:    "Bali",
		Hours:   "07.00 – 23.00",
		Phone:   "(0361) 555-0606",
		Maps:    "#",
	},
}

var promos = []Promo{
	{
		Title:     "Buy 1 Get 1 Cold Brew",
		Desc:      "Setiap Selasa & Rabu, beli satu Cold Brew gratis satu lagi untuk teman kamu!",
		Code:      "COLDBREW2",
		Until:     "31 Desember 2026",
		Badge:     "🔥 Hot Deal",
		BadgeType: "hot",
	},
	{
		Title:     "Diskon 30% Menu Baru",
		Desc:      "Coba menu-menu terbaru kami dengan diskon spesial 30% selama bulan November.",
		Code:      "NEWMENU30",
		Until:     "30 November 2026",
		Badge:     "✨ New",
		BadgeType: "new",
	},
	{
		Title:     "Free Croissant setiap Pagi",
		Desc:      "Order minuman apapun sebelum jam 09.00 dan dapatkan croissant gratis!",
		Code:      "EARLYBIRD",
		Until:     "31 Oktober 2026",
		Badge:     "⏰ Limited",
		BadgeType: "limited",
	},
	{
		Title:     "Ulang Tahun? Kopi Gratis!",
		Desc:      "Tunjukkan KTP di hari ulang tahunmu dan dapatkan minuman pilihan gratis.",
		Code:      "HAPPYBDAY",
		Until:     "Berlaku Sepanjang Tahun",
		Badge:     "🎂 Spesial",
		BadgeType: "new",
	},
}

var pointTiers = []PointTier{
	{
		Name:     "Bronze",
		MinPoint: 0,
		Benefit:  "Kumpulkan 1 poin setiap Rp 1.000. Akses promo anggota dasar.",
		Icon:     "🥉",
	},
	{
		Name:     "Silver",
		MinPoint: 500,
		Benefit:  "1,5× poin setiap transaksi. Gratis upgrade size minuman sekali per bulan.",
		Icon:     "🥈",
	},
	{
		Name:     "Gold",
		MinPoint: 1500,
		Benefit:  "2× poin. Prioritas antrian & diskon 10% setiap hari. Early access promo.",
		Icon:     "🥇",
	},
	{
		Name:     "Platinum",
		MinPoint: 5000,
		Benefit:  "3× poin. Free delivery, akses menu eksklusif, dan undangan event private.",
		Icon:     "💎",
	},
}

// ===== Template Setup =====

var funcs = template.FuncMap{
	"rupiah": func(n int) string {
		if n == 0 {
			return "Rp0"
		}
		digits := []byte{}
		val := n
		for val > 0 {
			digits = append([]byte{byte('0' + val%10)}, digits...)
			val /= 10
		}
		result := ""
		for i, c := range digits {
			if i > 0 && (len(digits)-i)%3 == 0 {
				result += "."
			}
			result += string(c)
		}
		return "Rp" + result
	},
}

func parseHalaman(files ...string) *template.Template {
	allFiles := append([]string{
		"templates/layout.html",
		"templates/header.html",
		"templates/sidebar.html",
		"templates/footer.html",
	}, files...)
	return template.Must(template.New("layout").Funcs(funcs).ParseFiles(allFiles...))
}

var (
	tplHome   = parseHalaman("templates/home.html")
	tplMenu   = parseHalaman("templates/menu.html")
	tplCabang = parseHalaman("templates/cabang.html")
	tplPromo  = parseHalaman("templates/promo.html")
	tplPoin   = parseHalaman("templates/poin.html")
)

// ===== Handlers =====

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tplHome.ExecuteTemplate(w, "layout", PageData{Title: "Beranda", ActiveNav: "home"})
}

func menuHandler(w http.ResponseWriter, r *http.Request) {
	tplMenu.ExecuteTemplate(w, "layout", MenuPage{
		PageData:   PageData{Title: "Menu", ActiveNav: "menu"},
		Categories: []string{"Kopi", "Non-Kopi", "Makanan"},
		Items:      menuItems,
	})
}

func cabangHandler(w http.ResponseWriter, r *http.Request) {
	tplCabang.ExecuteTemplate(w, "layout", BranchPage{
		PageData: PageData{Title: "Cabang", ActiveNav: "cabang"},
		Branches: branches,
	})
}

func promoHandler(w http.ResponseWriter, r *http.Request) {
	tplPromo.ExecuteTemplate(w, "layout", PromoPage{
		PageData: PageData{Title: "Promo", ActiveNav: "promo"},
		Promos:   promos,
	})
}

func poinHandler(w http.ResponseWriter, r *http.Request) {
	tplPoin.ExecuteTemplate(w, "layout", PointPage{
		PageData: PageData{Title: "Sistem Poin", ActiveNav: "poin"},
		Tiers:    pointTiers,
	})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", homeHandler)
	mux.HandleFunc("GET /menu", menuHandler)
	mux.HandleFunc("GET /cabang", cabangHandler)
	mux.HandleFunc("GET /promo", promoHandler)
	mux.HandleFunc("GET /poin", poinHandler)

	log.Println("Server berjalan di http://localhost:8000")
	log.Fatal(http.ListenAndServe("localhost:8000", mux))
}
