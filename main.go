package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
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
	BadgeType string
	Discount  int
}

type PointTier struct {
	Name     string
	MinPoint int
	Benefit  string
	Icon     string
}

type User struct {
	ID       string
	Username string
	Password string
	Name     string
	Email    string
	Phone    string
	Role     string
	Points   int
}

type CartItem struct {
	Item     MenuItem
	Qty      int
	Subtotal int
}

type Order struct {
	ID         string
	UserID     string
	Username   string
	UserName   string
	Branch     string
	Items      []CartItem
	PromoCode  string
	PromoTitle string
	Discount   int
	Subtotal   int
	Total      int
	Status     string
	PayMethod  string
	CreatedAt  time.Time
	PaidAt     *time.Time
}

type Session struct {
	UserID    string
	Username  string
	Role      string
	Name      string
	CartItems []CartItem
	Branch    string
	PromoCode string
}

// ===== Page Data Structs =====

type PageData struct {
	Title     string
	ActiveNav string
	Session   *Session
	Flash     string
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

type OrderPage struct {
	PageData
	Branches  []Branch
	Promos    []Promo
	Items     []MenuItem
	CartItems []CartItem
	Subtotal  int
	Discount  int
	Total     int
	Branch    string
	PromoCode string
}

type PaymentPage struct {
	PageData
	Order *Order
}

type PromoUsageStat struct {
	Code      string
	Title     string
	UsedBy    string
	Username  string
	OrderID   string
	Discount  int
	CreatedAt time.Time
}

type AdminPage struct {
	PageData
	Orders       []Order
	PromoUsage   []PromoUsageStat
	TotalRev     int
	TodayRev     int
	TotalOrders  int
	PendingCount int
	PaidCount    int
	MenuItems    []MenuItem
	Branches     []Branch
	Promos       []Promo
	Users        []User
}

type OrderHistoryPage struct {
	PageData
	Orders []Order
}

type RegisterData struct {
	Flash string
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
	{Title: "Buy 1 Get 1 Cold Brew", Desc: "Setiap Selasa & Rabu, beli satu Cold Brew gratis satu lagi!", Code: "COLDBREW2", Until: "31 Desember 2026", Badge: "🔥 Hot Deal", BadgeType: "hot", Discount: 50},
	{Title: "Diskon 30% Menu Baru", Desc: "Coba menu-menu terbaru kami dengan diskon spesial 30%.", Code: "NEWMENU30", Until: "30 November 2026", Badge: "✨ New", BadgeType: "new", Discount: 30},
	{Title: "Free Croissant setiap Pagi", Desc: "Order minuman apapun sebelum jam 09.00 dan dapatkan croissant gratis!", Code: "EARLYBIRD", Until: "31 Oktober 2026", Badge: "⏰ Limited", BadgeType: "limited", Discount: 20},
	{Title: "Ulang Tahun? Kopi Gratis!", Desc: "Tunjukkan KTP di hari ulang tahunmu dan dapatkan minuman gratis.", Code: "HAPPYBDAY", Until: "Berlaku Sepanjang Tahun", Badge: "🎂 Spesial", BadgeType: "new", Discount: 100},
}

// ===== In-Memory Storage =====

var (
	mu       sync.RWMutex
	sessions = map[string]*Session{}
	orders   = []Order{}
	users    = []User{
		{ID: "u1", Username: "admin", Password: "admin123", Name: "Administrator", Email: "admin@kopiku.id", Phone: "08111111111", Role: "admin"},
		{ID: "u2", Username: "kasir", Password: "kasir123", Name: "Kasir Sudirman", Email: "kasir@kopiku.id", Phone: "08222222222", Role: "kasir"},
		{ID: "u3", Username: "user1", Password: "user123", Name: "Budi Santoso", Email: "budi@email.com", Phone: "08333333333", Role: "user", Points: 350},
		{ID: "u4", Username: "user2", Password: "user456", Name: "Siti Rahayu", Email: "siti@email.com", Phone: "08444444444", Role: "user", Points: 1200},
	}
	nextUserID = 5

	// Default fallback session agar website langsung dapat dijalankan dan dipesan tanpa wajib login
	defaultGuestSession = &Session{
		UserID:   "u3",
		Username: "user1",
		Role:     "user",
		Name:     "Budi Santoso",
	}
)

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
	"add": func(a, b int) int { return a + b },
	"cartLen": func(items []CartItem) int {
		total := 0
		for _, c := range items {
			total += c.Qty
		}
		return total
	},
	"formatTime": func(t time.Time) string {
		return t.Format("02 Jan 2006, 15:04")
	},
	"tierName": func(points int) string {
		name := "Bronze"
		for _, t := range pointTiers {
			if points >= t.MinPoint {
				name = t.Name
			}
		}
		return name
	},
	"tierIcon": func(points int) string {
		icon := "🥉"
		for _, t := range pointTiers {
			if points >= t.MinPoint {
				icon = t.Icon
			}
		}
		return icon
	},
}

func parsePage(files ...string) *template.Template {
	allFiles := append([]string{
		"templates/layout.html",
		"templates/header.html",
		"templates/sidebar.html",
		"templates/footer.html",
	}, files...)
	return template.Must(template.New("layout").Funcs(funcs).ParseFiles(allFiles...))
}

func parseSimple(name string, files ...string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).ParseFiles(files...))
}

var (
	tplHome         = parsePage("templates/home.html")
	tplMenu         = parsePage("templates/menu.html")
	tplCabang       = parsePage("templates/cabang.html")
	tplPromo        = parsePage("templates/promo.html")
	tplPoin         = parsePage("templates/poin.html")
	tplLogin        = parseSimple("login.html", "templates/login.html")
	tplRegister     = parseSimple("register.html", "templates/register.html")
	tplOrder        = parsePage("templates/order.html")
	tplPayment      = parsePage("templates/payment.html")
	tplOrderHistory = parsePage("templates/order_history.html")
	tplAdmin        = parseSimple("admin.html", "templates/admin.html")
)

// ===== Helper Functions =====

func randID() string {
	return fmt.Sprintf("%06d", rand.Intn(999999)+1)
}

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		mu.RLock()
		s, exists := sessions[cookie.Value]
		mu.RUnlock()
		if exists && s != nil {
			return s
		}
	}
	// Fallback session agar seluruh fitur pemesanan tetap berjalan tanpa terhalang login
	return defaultGuestSession
}

func findUser(username, password string) *User {
	for i := range users {
		if users[i].Username == username && users[i].Password == password {
			return &users[i]
		}
	}
	return nil
}

func findUserByID(id string) *User {
	for i := range users {
		if users[i].ID == id {
			return &users[i]
		}
	}
	return nil
}

func findPromo(code string) *Promo {
	code = strings.ToUpper(strings.TrimSpace(code))
	for i := range promos {
		if promos[i].Code == code {
			return &promos[i]
		}
	}
	return nil
}

func cartSubtotal(items []CartItem) int {
	total := 0
	for _, c := range items {
		total += c.Subtotal
	}
	return total
}

func addFlash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: msg, Path: "/", MaxAge: 5})
}

func getFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie("flash")
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1})
	return c.Value
}

// ===== Public Handlers =====

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	sess := getSession(r)
	tplHome.ExecuteTemplate(w, "layout", PageData{Title: "Beranda", ActiveNav: "home", Session: sess})
}

func menuHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	tplMenu.ExecuteTemplate(w, "layout", MenuPage{
		PageData:   PageData{Title: "Menu", ActiveNav: "menu", Session: sess},
		Categories: []string{"Kopi", "Non-Kopi", "Makanan"},
		Items:      menuItems,
	})
}

func cabangHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	tplCabang.ExecuteTemplate(w, "layout", BranchPage{
		PageData: PageData{Title: "Cabang", ActiveNav: "cabang", Session: sess},
		Branches: branches,
	})
}

func promoHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	tplPromo.ExecuteTemplate(w, "layout", PromoPage{
		PageData: PageData{Title: "Promo", ActiveNav: "promo", Session: sess},
		Promos:   promos,
	})
}

func poinHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	tplPoin.ExecuteTemplate(w, "layout", PointPage{
		PageData: PageData{Title: "Sistem Poin", ActiveNav: "poin", Session: sess},
		Tiers:    pointTiers,
	})
}

// ===== Auth Handlers =====

func loginHandler(w http.ResponseWriter, r *http.Request) {
	flash := getFlash(w, r)
	if r.Method == http.MethodGet {
		tplLogin.ExecuteTemplate(w, "login.html", map[string]string{"Flash": flash})
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	u := findUser(username, password)
	if u == nil {
		addFlash(w, "Username atau password salah.")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	sessionID := fmt.Sprintf("%s-%d", randID(), time.Now().UnixNano())
	sess := &Session{UserID: u.ID, Username: u.Username, Role: u.Role, Name: u.Name}
	mu.Lock()
	sessions[sessionID] = sess
	mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: sessionID, Path: "/", MaxAge: 86400})
	if u.Role == "admin" || u.Role == "kasir" {
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		http.Redirect(w, r, "/order", http.StatusFound)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	flash := getFlash(w, r)
	if r.Method == http.MethodGet {
		tplRegister.ExecuteTemplate(w, "register.html", RegisterData{Flash: flash})
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	password := r.FormValue("password")
	confirmPw := r.FormValue("confirm_password")

	if name == "" || username == "" || password == "" {
		addFlash(w, "Semua field wajib diisi.")
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}
	if password != confirmPw {
		addFlash(w, "Konfirmasi password tidak cocok.")
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}
	if len(password) < 4 {
		addFlash(w, "Password minimal 4 karakter.")
		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	mu.Lock()
	// Check username exists
	for _, u := range users {
		if u.Username == username {
			mu.Unlock()
			addFlash(w, "Username sudah dipakai, pilih yang lain.")
			http.Redirect(w, r, "/register", http.StatusFound)
			return
		}
	}
	newUser := User{
		ID:       fmt.Sprintf("u%d", nextUserID),
		Username: username,
		Password: password,
		Name:     name,
		Email:    email,
		Phone:    phone,
		Role:     "user",
		Points:   0,
	}
	nextUserID++
	users = append(users, newUser)
	mu.Unlock()

	addFlash(w, "Registrasi berhasil! Silakan login.")
	http.Redirect(w, r, "/login", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		mu.Lock()
		delete(sessions, cookie.Value)
		mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusFound)
}

// ===== Order Handlers (User) =====

func orderHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	subtotal := cartSubtotal(sess.CartItems)
	discount := 0
	promoTitle := ""
	if sess.PromoCode != "" {
		p := findPromo(sess.PromoCode)
		if p != nil {
			discount = subtotal * p.Discount / 100
			promoTitle = p.Title
		}
	}
	tplOrder.ExecuteTemplate(w, "layout", OrderPage{
		PageData:  PageData{Title: "Pesan", ActiveNav: "order", Session: sess, Flash: getFlash(w, r)},
		Branches:  branches,
		Promos:    promos,
		Items:     menuItems,
		CartItems: sess.CartItems,
		Subtotal:  subtotal,
		Discount:  discount,
		Total:     subtotal - discount,
		Branch:    sess.Branch,
		PromoCode: promoTitle,
	})
}

func addToCartHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	itemName := r.FormValue("item")
	qty, _ := strconv.Atoi(r.FormValue("qty"))
	if qty < 1 {
		qty = 1
	}
	var found *MenuItem
	for i := range menuItems {
		if menuItems[i].Name == itemName {
			found = &menuItems[i]
			break
		}
	}
	if found == nil {
		http.Redirect(w, r, "/order", http.StatusFound)
		return
	}
	mu.Lock()
	updated := false
	for i := range sess.CartItems {
		if sess.CartItems[i].Item.Name == itemName {
			sess.CartItems[i].Qty += qty
			sess.CartItems[i].Subtotal = sess.CartItems[i].Qty * found.Price
			updated = true
			break
		}
	}
	if !updated {
		sess.CartItems = append(sess.CartItems, CartItem{Item: *found, Qty: qty, Subtotal: qty * found.Price})
	}
	mu.Unlock()
	addFlash(w, found.Name+" ditambahkan ke keranjang! 🛒")
	http.Redirect(w, r, "/order", http.StatusFound)
}

func removeFromCartHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	itemName := r.FormValue("item")
	mu.Lock()
	newCart := []CartItem{}
	for _, c := range sess.CartItems {
		if c.Item.Name != itemName {
			newCart = append(newCart, c)
		}
	}
	sess.CartItems = newCart
	mu.Unlock()
	http.Redirect(w, r, "/order", http.StatusFound)
}

func setBranchHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	branch := r.FormValue("branch")
	mu.Lock()
	sess.Branch = branch
	mu.Unlock()
	addFlash(w, "Cabang "+branch+" dipilih! 📍")
	http.Redirect(w, r, "/order", http.StatusFound)
}

func applyPromoHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	code := strings.ToUpper(strings.TrimSpace(r.FormValue("promo_code")))
	p := findPromo(code)
	if p == nil {
		addFlash(w, "Kode promo tidak valid! ❌")
	} else {
		mu.Lock()
		sess.PromoCode = p.Code
		mu.Unlock()
		addFlash(w, "Promo \""+p.Title+"\" berhasil diaplikasikan! Diskon "+strconv.Itoa(p.Discount)+"% ✅")
	}
	http.Redirect(w, r, "/order", http.StatusFound)
}

func removePromoHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	mu.Lock()
	sess.PromoCode = ""
	mu.Unlock()
	http.Redirect(w, r, "/order", http.StatusFound)
}

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if len(sess.CartItems) == 0 {
		addFlash(w, "Keranjang kosong, tambahkan item terlebih dahulu!")
		http.Redirect(w, r, "/order", http.StatusFound)
		return
	}
	if sess.Branch == "" {
		addFlash(w, "Pilih cabang terlebih dahulu! 📍")
		http.Redirect(w, r, "/order", http.StatusFound)
		return
	}
	subtotal := cartSubtotal(sess.CartItems)
	discount := 0
	promoTitle := ""
	promoCode := ""
	if sess.PromoCode != "" {
		p := findPromo(sess.PromoCode)
		if p != nil {
			discount = subtotal * p.Discount / 100
			promoTitle = p.Title
			promoCode = p.Code
		}
	}
	order := Order{
		ID:         "ORD-" + randID(),
		UserID:     sess.UserID,
		Username:   sess.Username,
		UserName:   sess.Name,
		Branch:     sess.Branch,
		Items:      append([]CartItem{}, sess.CartItems...),
		PromoCode:  promoCode,
		PromoTitle: promoTitle,
		Discount:   discount,
		Subtotal:   subtotal,
		Total:      subtotal - discount,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	mu.Lock()
	orders = append(orders, order)
	sess.CartItems = nil
	sess.PromoCode = ""
	sess.Branch = ""
	mu.Unlock()
	http.Redirect(w, r, "/payment/"+order.ID, http.StatusFound)
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	orderID := strings.TrimPrefix(r.URL.Path, "/payment/")
	mu.RLock()
	var found *Order
	for i := range orders {
		if orders[i].ID == orderID {
			copy := orders[i]
			found = &copy
			break
		}
	}
	mu.RUnlock()
	if found == nil {
		http.Redirect(w, r, "/order", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		payMethod := r.FormValue("pay_method")
		now := time.Now()
		mu.Lock()
		for i := range orders {
			if orders[i].ID == orderID {
				orders[i].Status = "paid"
				orders[i].PayMethod = payMethod
				orders[i].PaidAt = &now
				// Add points to user
				u := findUserByID(orders[i].UserID)
				if u != nil {
					u.Points += orders[i].Total / 1000
				}
				break
			}
		}
		mu.Unlock()
		addFlash(w, "Pembayaran berhasil! Terima kasih, pesananmu sedang diproses. ☕")
		http.Redirect(w, r, "/orders", http.StatusFound)
		return
	}
	tplPayment.ExecuteTemplate(w, "layout", PaymentPage{
		PageData: PageData{Title: "Pembayaran", ActiveNav: "order", Session: sess},
		Order:    found,
	})
}

func orderHistoryHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	mu.RLock()
	var myOrders []Order
	for i := len(orders) - 1; i >= 0; i-- {
		// Tampilkan pesanan user atau semua pesanan jika guest
		if orders[i].UserID == sess.UserID || sess.UserID == "u3" {
			myOrders = append(myOrders, orders[i])
		}
	}
	mu.RUnlock()
	tplOrderHistory.ExecuteTemplate(w, "layout", OrderHistoryPage{
		PageData: PageData{Title: "Riwayat Pesanan", ActiveNav: "orders", Session: sess, Flash: getFlash(w, r)},
		Orders:   myOrders,
	})
}

// ===== Admin Handlers =====

func adminHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	// Jika belum login admin/kasir, berikan session admin default agar dashboard langsung bisa diakses
	if sess == nil || (sess.Role != "admin" && sess.Role != "kasir") {
		sess = &Session{UserID: "u1", Username: "admin", Role: "admin", Name: "Administrator"}
	}

	mu.RLock()
	allOrders := append([]Order{}, orders...)
	allUsers := append([]User{}, users...)
	mu.RUnlock()
	for i, j := 0, len(allOrders)-1; i < j; i, j = i+1, j-1 {
		allOrders[i], allOrders[j] = allOrders[j], allOrders[i]
	}
	totalRev, todayRev := 0, 0
	pendingCount, paidCount := 0, 0
	today := time.Now().Format("2006-01-02")
	var promoStats []PromoUsageStat
	for _, o := range allOrders {
		if o.Status == "paid" {
			totalRev += o.Total
			paidCount++
			if o.CreatedAt.Format("2006-01-02") == today {
				todayRev += o.Total
			}
		}
		if o.Status == "pending" {
			pendingCount++
		}
		if o.PromoCode != "" {
			promoStats = append(promoStats, PromoUsageStat{
				Code:     o.PromoCode,
				Title:    o.PromoTitle,
				UsedBy:   o.UserName,
				Username: o.Username,
				OrderID:  o.ID,
				Discount: o.Discount,
				CreatedAt: o.CreatedAt,
			})
		}
	}
	tplAdmin.ExecuteTemplate(w, "admin.html", AdminPage{
		PageData:     PageData{Title: "Dashboard Admin", Session: sess},
		Orders:       allOrders,
		PromoUsage:   promoStats,
		TotalRev:     totalRev,
		TodayRev:     todayRev,
		TotalOrders:  len(allOrders),
		PendingCount: pendingCount,
		PaidCount:    paidCount,
		MenuItems:    menuItems,
		Branches:     branches,
		Promos:       promos,
		Users:        allUsers,
	})
}

func adminUpdateOrderHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.FormValue("order_id")
	status := r.FormValue("status")
	mu.Lock()
	for i := range orders {
		if orders[i].ID == orderID {
			orders[i].Status = status
			if status == "paid" && orders[i].PaidAt == nil {
				now := time.Now()
				orders[i].PaidAt = &now
			}
			break
		}
	}
	mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Admin: Create order as cashier
func adminCreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	customerName := r.FormValue("customer_name")
	branch := r.FormValue("branch")
	promoCode := strings.ToUpper(strings.TrimSpace(r.FormValue("promo_code")))
	itemNames := r.Form["items[]"]
	itemQtys := r.Form["qtys[]"]

	if customerName == "" || branch == "" || len(itemNames) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Data tidak lengkap"})
		return
	}

	var cartItems []CartItem
	for idx, name := range itemNames {
		qty := 1
		if idx < len(itemQtys) {
			qty, _ = strconv.Atoi(itemQtys[idx])
			if qty < 1 {
				qty = 1
			}
		}
		for _, mi := range menuItems {
			if mi.Name == name {
				cartItems = append(cartItems, CartItem{Item: mi, Qty: qty, Subtotal: qty * mi.Price})
				break
			}
		}
	}

	subtotal := cartSubtotal(cartItems)
	discount := 0
	promoTitle := ""
	if promoCode != "" {
		p := findPromo(promoCode)
		if p != nil {
			discount = subtotal * p.Discount / 100
			promoTitle = p.Title
		} else {
			promoCode = ""
		}
	}

	order := Order{
		ID:         "ORD-" + randID(),
		UserID:     "walk-in",
		Username:   "walk-in",
		UserName:   customerName,
		Branch:     branch,
		Items:      cartItems,
		PromoCode:  promoCode,
		PromoTitle: promoTitle,
		Discount:   discount,
		Subtotal:   subtotal,
		Total:      subtotal - discount,
		Status:     "paid",
		PayMethod:  r.FormValue("pay_method"),
		CreatedAt:  time.Now(),
	}
	now := time.Now()
	order.PaidAt = &now

	mu.Lock()
	orders = append(orders, order)
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"order_id": order.ID,
		"total":    order.Total,
	})
}

func main() {
	rand.Seed(time.Now().UnixNano())
	mux := http.NewServeMux()

	// Public Pages
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/menu", menuHandler)
	mux.HandleFunc("/cabang", cabangHandler)
	mux.HandleFunc("/promo", promoHandler)
	mux.HandleFunc("/poin", poinHandler)

	// Auth (Login & Register - tidak saling memblokir / bebas diakses)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/logout", logoutHandler)

	// User order flow (Langsung bisa diakses tanpa kewajiban login)
	mux.HandleFunc("/order", orderHandler)
	mux.HandleFunc("/cart/add", addToCartHandler)
	mux.HandleFunc("/cart/remove", removeFromCartHandler)
	mux.HandleFunc("/order/branch", setBranchHandler)
	mux.HandleFunc("/order/promo", applyPromoHandler)
	mux.HandleFunc("/order/promo/remove", removePromoHandler)
	mux.HandleFunc("/checkout", checkoutHandler)
	mux.HandleFunc("/payment/", paymentHandler)
	mux.HandleFunc("/orders", orderHistoryHandler)

	// Admin Dashboard & Kasir POS (Bisa langsung dibuka di /admin)
	mux.HandleFunc("/admin", adminHandler)
	mux.HandleFunc("/admin/order/update", adminUpdateOrderHandler)
	mux.HandleFunc("/admin/order/create", adminCreateOrderHandler)

	log.Println("Server berjalan di http://localhost:8000")
	log.Fatal(http.ListenAndServe("localhost:8000", mux))
}
