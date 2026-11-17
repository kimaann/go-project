package main

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
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
	ID           string
	Username     string
	PasswordHash string
	Name         string
	Email        string
	Phone        string
	Role         string
	Points       int
	IsActive     bool
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

	// Data saldo poin member (diisi bila session ber-role user).
	ActivePoints     int
	RupiahValue      int
	ExpiringSoon     int
	ExpiringRupiah   int
	NextExpiryText   string
	LifetimeEarned   int
	LifetimeRedeemed int
	Lots             []PointLot
	Ledger           []PointLedgerEntry
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

// ===== Application Storage =====
// Akun pengguna disimpan permanen di MySQL.
// Session dan data pesanan pada versi ini masih disimpan di memori.
var (
	mu       sync.RWMutex
	sessions = map[string]*Session{}
	orders   = []Order{}
	db       *sql.DB
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
	tplPointReport  = parsePage("templates/poin_report.html")
)

// ===== Helper Functions =====

func randID() string {
	return fmt.Sprintf("%06d", rand.Intn(999999)+1)
}

// getEnv membaca environment variable dan menggunakan nilai default jika kosong.
func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// initDB membuka koneksi MySQL menggunakan konfigurasi environment variable.
// Default konfigurasi sesuai instalasi XAMPP lokal: root tanpa password.
func initDB() error {
	cfg := mysql.Config{
		User:                 getEnv("DB_USER", "root"),
		Passwd:               os.Getenv("DB_PASSWORD"),
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(getEnv("DB_HOST", "127.0.0.1"), getEnv("DB_PORT", "3306")),
		DBName:               getEnv("DB_NAME", "kopiku_db"),
		ParseTime:            true,
		Loc:                  time.Local,
		Collation:            "utf8mb4_unicode_ci",
		AllowNativePasswords: true,
		Params:               map[string]string{"charset": "utf8mb4"},
	}

	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("gagal membuka koneksi MySQL: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("tidak dapat terhubung ke MySQL: %w", err)
	}

	// Buat tabel users jika belum ada
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(150) NOT NULL,
		username VARCHAR(100) UNIQUE NOT NULL,
		phone VARCHAR(30) DEFAULT '',
		email VARCHAR(150) UNIQUE DEFAULT NULL,
		password_hash VARCHAR(255) NOT NULL,
		role VARCHAR(20) NOT NULL DEFAULT 'user',
		points INT NOT NULL DEFAULT 0,
		is_active TINYINT(1) NOT NULL DEFAULT 1,
		last_login_at DATETIME NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("gagal menyiapkan tabel users di database kopiku_db: %w", err)
	}

	// Seed akun default bila tabel masih kosong
	var userCount int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	if userCount == 0 {
		type seedUser struct {
			id, name, username, phone, email, pass, role string
			points                                       int
		}
		defaultUsers := []seedUser{
			{"u1", "Administrator", "admin", "08111111111", "admin@kopiku.id", "admin123", "admin", 0},
			{"u2", "Kasir Sudirman", "kasir", "08222222222", "kasir@kopiku.id", "kasir123", "kasir", 0},
			{"u3", "Budi Santoso", "user1", "08333333333", "budi@email.com", "user123", "user", 350},
			{"u4", "Siti Rahayu", "user2", "08444444444", "siti@email.com", "user456", "user", 1200},
		}
		for _, du := range defaultUsers {
			hash, err := bcrypt.GenerateFromPassword([]byte(du.pass), 10)
			if err == nil {
				_, _ = db.ExecContext(ctx, `
					INSERT INTO users (id, name, username, phone, email, password_hash, role, points, is_active)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
					du.id, du.name, du.username, du.phone, du.email, string(hash), du.role, du.points)
			}
		}
		log.Println("Data akun awal (admin, kasir, user1, user2) berhasil di-seed ke kopiku_db")
	}

	// Siapkan tabel dan mesin poin (lot, penukaran, ledger).
	pointCtx, pointCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer pointCancel()
	if err := initPointsSchema(pointCtx); err != nil {
		return fmt.Errorf("gagal menyiapkan sistem poin: %w", err)
	}

	return nil
}

// newRandomID menghasilkan ID acak untuk session dan akun.
func newRandomID() string {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	// Format UUID v4.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func setSessionCookie(w http.ResponseWriter, sessionID string) {
	secure := strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true")
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true"),
		SameSite: http.SameSiteLaxMode,
	})
}

// getSession hanya mengembalikan session yang benar-benar terdaftar.
// Tidak ada lagi fallback menjadi admin atau user demo secara otomatis.
func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session_id")
	if err != nil || cookie.Value == "" {
		return nil
	}
	mu.RLock()
	sess := sessions[cookie.Value]
	mu.RUnlock()
	return sess
}

// getOrCreateSession membuat session tamu untuk alur pemesanan tanpa login.
// ID tamu dibuat unik agar keranjang dan riwayat antar pengunjung tidak tercampur.
func getOrCreateSession(w http.ResponseWriter, r *http.Request) *Session {
	if sess := getSession(r); sess != nil {
		return sess
	}
	sessionID := newRandomID()
	sess := &Session{
		UserID:   "guest-" + sessionID,
		Username: "guest",
		Role:     "guest",
		Name:     "Tamu",
	}
	mu.Lock()
	sessions[sessionID] = sess
	mu.Unlock()
	setSessionCookie(w, sessionID)
	return sess
}

// getUserByUsername mengambil akun dan password hash dari MySQL.
func getUserByUsername(username string) (*User, string, error) {
	var u User
	var passwordHash string
	var active bool
	err := db.QueryRowContext(context.Background(), `
		SELECT id, name, username, phone, email, password_hash, role, points, is_active
		FROM users
		WHERE username = ?
		LIMIT 1`, username).Scan(
		&u.ID, &u.Name, &u.Username, &u.Phone, &u.Email,
		&passwordHash, &u.Role, &u.Points, &active,
	)
	if err != nil {
		return nil, "", err
	}
	u.PasswordHash = passwordHash
	u.IsActive = active
	return &u, passwordHash, nil
}

// getAllUsers mengambil daftar pengguna untuk tabel pengguna di dashboard admin.
func getAllUsers() ([]User, error) {
	rows, err := db.QueryContext(context.Background(), `
		SELECT id, name, username, email, phone, role, points, is_active
		FROM users
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.Phone, &u.Role, &u.Points, &u.IsActive); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func isStaff(sess *Session) bool {
	return sess != nil && (sess.Role == "admin" || sess.Role == "kasir")
}

func requireStaff(w http.ResponseWriter, r *http.Request) bool {
	sess := getSession(r)
	if !isStaff(sess) {
		http.Error(w, "Akses ditolak. Silakan login sebagai admin atau kasir.", http.StatusForbidden)
		return false
	}
	return true
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
	page := PointPage{
		PageData: PageData{Title: "Sistem Poin", ActiveNav: "poin", Session: sess, Flash: getFlash(w, r)},
		Tiers:    pointTiers,
	}

	// Tampilkan saldo poin aktif untuk member yang sudah login.
	if sess != nil && sess.Role == "user" {
		if summary, lots, ledger, err := GetUserPointDetail(sess.UserID); err != nil {
			log.Printf("Gagal memuat saldo poin pengguna: %v", err)
		} else {
			page.ActivePoints = summary.ActivePoints
			page.RupiahValue = summary.RupiahValue
			page.ExpiringSoon = summary.ExpiringSoon
			page.ExpiringRupiah = summary.ExpiringRupiah
			page.NextExpiryText = summary.NextExpiryText
			page.LifetimeEarned = summary.LifetimeEarned
			page.LifetimeRedeemed = summary.LifetimeRedeemed
			page.Lots = lots
			page.Ledger = ledger
		}
	}

	tplPoin.ExecuteTemplate(w, "layout", page)
}

// ===== Auth Handlers =====

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		flash := getFlash(w, r)
		_ = tplLogin.ExecuteTemplate(w, "login.html", map[string]string{"Flash": flash})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		addFlash(w, "Data login tidak valid.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		addFlash(w, "Username dan password wajib diisi.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	u, passwordHash, err := getUserByUsername(username)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("Gagal mengambil akun saat login: %v", err)
		}
		addFlash(w, "Username atau password salah.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !u.IsActive || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		addFlash(w, "Username atau password salah.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Catat waktu login terakhir.
	if _, err := db.ExecContext(context.Background(),
		"UPDATE users SET last_login_at = NOW() WHERE id = ?", u.ID); err != nil {
		log.Printf("Gagal memperbarui last_login_at: %v", err)
	}

	// Rotasi session ID untuk mencegah session fixation.
	if oldCookie, err := r.Cookie("session_id"); err == nil {
		mu.Lock()
		delete(sessions, oldCookie.Value)
		mu.Unlock()
	}
	sessionID := newRandomID()
	sess := &Session{UserID: u.ID, Username: u.Username, Role: u.Role, Name: u.Name}
	mu.Lock()
	sessions[sessionID] = sess
	mu.Unlock()
	setSessionCookie(w, sessionID)

	if u.Role == "admin" || u.Role == "kasir" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/order", http.StatusSeeOther)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		flash := getFlash(w, r)
		_ = tplRegister.ExecuteTemplate(w, "register.html", RegisterData{Flash: flash})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		addFlash(w, "Data registrasi tidak valid.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	phone := strings.TrimSpace(r.FormValue("phone"))
	password := r.FormValue("password")
	confirmPw := r.FormValue("confirm_password")

	if name == "" || username == "" || email == "" || phone == "" || password == "" || confirmPw == "" {
		addFlash(w, "Semua field wajib diisi.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	if len(username) < 3 || len(username) > 50 {
		addFlash(w, "Username harus terdiri dari 3 sampai 50 karakter.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	parsedEmail, emailErr := mail.ParseAddress(email)
	if emailErr != nil || parsedEmail.Address != email {
		addFlash(w, "Format email tidak valid.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	if password != confirmPw {
		addFlash(w, "Konfirmasi password tidak cocok.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	if len(password) < 8 {
		addFlash(w, "Password minimal 8 karakter.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Printf("Gagal membuat password hash: %v", err)
		addFlash(w, "Registrasi gagal. Silakan coba kembali.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	_, err = db.ExecContext(context.Background(), `
		INSERT INTO users
		(id, name, username, phone, email, password_hash, role, points, is_active)
		VALUES (?, ?, ?, ?, ?, ?, 'user', 0, 1)`,
		newRandomID(), name, username, phone, email, string(passwordHash))
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			addFlash(w, "Username atau email sudah digunakan. Gunakan data lain.")
		} else {
			log.Printf("Gagal menyimpan akun baru: %v", err)
			addFlash(w, "Registrasi gagal karena masalah database. Silakan coba kembali.")
		}
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	addFlash(w, "Registrasi berhasil! Silakan login.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie("session_id"); err == nil {
		mu.Lock()
		delete(sessions, cookie.Value)
		mu.Unlock()
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ===== Order Handlers (User) =====

func orderHandler(w http.ResponseWriter, r *http.Request) {
	sess := getOrCreateSession(w, r)
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
	sess := getOrCreateSession(w, r)
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
	sess := getOrCreateSession(w, r)
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
	sess := getOrCreateSession(w, r)
	branch := r.FormValue("branch")
	mu.Lock()
	sess.Branch = branch
	mu.Unlock()
	addFlash(w, "Cabang "+branch+" dipilih! 📍")
	http.Redirect(w, r, "/order", http.StatusFound)
}

func applyPromoHandler(w http.ResponseWriter, r *http.Request) {
	sess := getOrCreateSession(w, r)
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
	sess := getOrCreateSession(w, r)
	mu.Lock()
	sess.PromoCode = ""
	mu.Unlock()
	http.Redirect(w, r, "/order", http.StatusFound)
}

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	sess := getOrCreateSession(w, r)
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
	sess := getOrCreateSession(w, r)
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
	if found == nil || (found.UserID != sess.UserID && !isStaff(sess)) {
		http.Redirect(w, r, "/order", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		payMethod := strings.TrimSpace(r.FormValue("pay_method"))
		now := time.Now()
		var pointsUserID string
		var pointsToAdd int
		mu.Lock()
		for i := range orders {
			if orders[i].ID == orderID {
				// Poin hanya ditambahkan saat status berubah dari belum dibayar menjadi paid.
				if orders[i].Status != "paid" {
					orders[i].Status = "paid"
					orders[i].PayMethod = payMethod
					orders[i].PaidAt = &now
					pointsUserID = orders[i].UserID
					pointsToAdd = orders[i].Total / 1000
				}
				break
			}
		}
		mu.Unlock()

		if pointsUserID != "" && !strings.HasPrefix(pointsUserID, "guest-") && pointsUserID != "walk-in" && pointsToAdd > 0 {
			// Poin dicatat sebagai lot berumur 6 bulan + entri ledger.
			if _, err := AddPoints(pointsUserID, pointsToAdd, orderID, "Poin transaksi "+orderID); err != nil {
				log.Printf("Gagal menambahkan poin pelanggan: %v", err)
			}
		}
		addFlash(w, "Pembayaran berhasil! Terima kasih, pesananmu sedang diproses. ☕")
		http.Redirect(w, r, "/orders", http.StatusSeeOther)
		return
	}
	tplPayment.ExecuteTemplate(w, "layout", PaymentPage{
		PageData: PageData{Title: "Pembayaran", ActiveNav: "order", Session: sess},
		Order:    found,
	})
}

func orderHistoryHandler(w http.ResponseWriter, r *http.Request) {
	sess := getOrCreateSession(w, r)
	mu.RLock()
	var myOrders []Order
	for i := len(orders) - 1; i >= 0; i-- {
		// Setiap pengguna hanya melihat pesanan milik session-nya sendiri.
		if orders[i].UserID == sess.UserID {
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
	if !isStaff(sess) {
		addFlash(w, "Silakan login menggunakan akun admin atau kasir.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	mu.RLock()
	allOrders := append([]Order{}, orders...)
	mu.RUnlock()
	allUsers, err := getAllUsers()
	if err != nil {
		log.Printf("Gagal mengambil daftar pengguna untuk dashboard: %v", err)
		allUsers = []User{}
	}
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
				Code:      o.PromoCode,
				Title:     o.PromoTitle,
				UsedBy:    o.UserName,
				Username:  o.Username,
				OrderID:   o.ID,
				Discount:  o.Discount,
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
	if !requireStaff(w, r) {
		return
	}
	orderID := r.FormValue("order_id")
	status := r.FormValue("status")
	oldStatus := ""
	mu.Lock()
	for i := range orders {
		if orders[i].ID == orderID {
			oldStatus = orders[i].Status
			orders[i].Status = status
			if status == "paid" && orders[i].PaidAt == nil {
				now := time.Now()
				orders[i].PaidAt = &now
			}
			break
		}
	}
	mu.Unlock()

	// Sinkronkan poin saat pesanan dibatalkan / diaktifkan kembali.
	if oldStatus != "" && oldStatus != status {
		switch {
		case status == "cancelled":
			if err := ApplyOrderCancellation(orderID); err != nil {
				log.Printf("Gagal memproses pembatalan poin pesanan %s: %v", orderID, err)
			}
		case oldStatus == "cancelled":
			if err := ApplyOrderReactivation(orderID); err != nil {
				log.Printf("Gagal memproses pengaktifan ulang poin pesanan %s: %v", orderID, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Admin: Create order as cashier
func adminCreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
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


// ============================================================================
// SISTEM POIN LOYALITAS (TERINTEGRASI DARI points*.go)
// ============================================================================

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

// PointReportPage adalah data untuk halaman laporan poin admin.
type PointReportPage struct {
	PageData
	Report *PointReport
}

// pointsReportHandler menampilkan laporan poin aktif dan nilai rupiahnya.
func pointsReportHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	report, err := BuildPointsReport()
	if err != nil {
		log.Printf("Gagal membangun laporan poin: %v", err)
		http.Error(w, "Gagal memuat laporan poin.", http.StatusInternalServerError)
		return
	}
	tplPointReport.ExecuteTemplate(w, "layout", PointReportPage{
		PageData: PageData{
			Title:     "Laporan Poin",
			ActiveNav: "adminpoin",
			Session:   getSession(r),
			Flash:     getFlash(w, r),
		},
		Report: report,
	})
}

// pointsAddHandler menambahkan poin secara manual (staff).
func pointsAddHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	points, _ := strconv.Atoi(r.FormValue("points"))
	orderID := strings.TrimSpace(r.FormValue("order_id"))
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		note = "Penambahan poin manual oleh staf"
	}

	u, _, err := getUserByUsername(username)
	if err != nil {
		addFlash(w, "Pengguna @"+username+" tidak ditemukan.")
		http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
		return
	}

	lot, err := AddPoints(u.ID, points, orderID, note)
	if err != nil {
		addFlash(w, "Gagal menambah poin: "+err.Error())
	} else {
		addFlash(w, fmt.Sprintf("%d poin ditambahkan untuk @%s, berlaku sampai %s.",
			lot.PointsTotal, u.Username, lot.ExpiresAt.Format("02 Jan 2006")))
	}
	http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
}

// pointsRedeemHandler menukarkan poin atas nama pengguna (staff).
func pointsRedeemHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	points, _ := strconv.Atoi(r.FormValue("points"))
	orderID := strings.TrimSpace(r.FormValue("order_id"))
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		note = "Penukaran poin oleh staf"
	}

	u, _, err := getUserByUsername(username)
	if err != nil {
		addFlash(w, "Pengguna @"+username+" tidak ditemukan.")
		http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
		return
	}

	redemption, err := RedeemPoints(u.ID, points, orderID, note)
	if err != nil {
		addFlash(w, "Gagal menukar poin: "+err.Error())
	} else {
		addFlash(w, fmt.Sprintf("Penukaran %d poin (senilai %s) berhasil. Voucher: %s.",
			redemption.Points, formatRupiah(redemption.RupiahValue), redemption.VoucherCode))
	}
	http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
}

// pointsReverseHandler menarik poin (pembalikan/deduction).
// Bila order_id diisi dan points = 0, sistem menarik poin yang pernah
// diberikan untuk pesanan tersebut.
func pointsReverseHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	points, _ := strconv.Atoi(r.FormValue("points"))
	orderID := strings.TrimSpace(r.FormValue("order_id"))
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		note = "Pembalikan poin oleh staf"
	}

	u, _, err := getUserByUsername(username)
	if err != nil {
		addFlash(w, "Pengguna @"+username+" tidak ditemukan.")
		http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
		return
	}

	var (
		deducted  int
		shortfall int
	)
	if orderID != "" && points == 0 {
		deducted, shortfall, err = ReverseOrderEarning(orderID, u.ID)
	} else {
		deducted, shortfall, err = ReversePoints(u.ID, points, orderID, note)
	}
	if err != nil {
		addFlash(w, "Gagal menarik poin: "+err.Error())
	} else if shortfall > 0 {
		addFlash(w, fmt.Sprintf("%d poin ditarik. Kekurangan %d poin tidak dapat ditarik karena saldo aktif tidak cukup.",
			deducted, shortfall))
	} else {
		addFlash(w, fmt.Sprintf("%d poin berhasil ditarik dari @%s.", deducted, u.Username))
	}
	http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
}

// pointsRestoreHandler memulihkan poin dari penukaran yang dibatalkan.
func pointsRestoreHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	redemptionID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("redemption_id")), 10, 64)
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		note = "Restorasi penukaran oleh staf"
	}

	_, restored, err := RestoreRedemption(redemptionID, note)
	if err != nil {
		addFlash(w, "Gagal memulihkan poin: "+err.Error())
	} else {
		addFlash(w, fmt.Sprintf("%d poin berhasil dipulihkan dari penukaran #%d.", restored, redemptionID))
	}
	http.Redirect(w, r, "/admin/poin", http.StatusSeeOther)
}

// userRedeemPointsHandler menukarkan poin dari halaman /poin milik member.
func userRedeemPointsHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if sess == nil || sess.Role != "user" {
		addFlash(w, "Silakan login sebagai member untuk menukar poin.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	points, _ := strconv.Atoi(r.FormValue("points"))
	if points < MinRedeemPoints {
		addFlash(w, fmt.Sprintf("Minimum penukaran adalah %d poin.", MinRedeemPoints))
		http.Redirect(w, r, "/poin", http.StatusSeeOther)
		return
	}

	redemption, err := RedeemPoints(sess.UserID, points, "", "Penukaran poin dari halaman poin")
	if err != nil {
		addFlash(w, "Gagal menukar poin: "+err.Error())
	} else {
		addFlash(w, fmt.Sprintf("Penukaran %d poin berhasil! Voucher %s senilai %s.",
			redemption.Points, redemption.VoucherCode, formatRupiah(redemption.RupiahValue)))
	}
	http.Redirect(w, r, "/poin", http.StatusSeeOther)
}

// apiPointsBalanceHandler mengembalikan saldo poin aktif dalam bentuk JSON.
// Staff dapat meminta ?username=..., member hanya bisa melihat saldonya sendiri.
func apiPointsBalanceHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	username := strings.TrimSpace(r.URL.Query().Get("username"))

	var userID string
	switch {
	case username != "" && isStaff(sess):
		u, _, err := getUserByUsername(username)
		if err != nil {
			http.Error(w, "Pengguna tidak ditemukan.", http.StatusNotFound)
			return
		}
		userID = u.ID
	case sess != nil:
		userID = sess.UserID
	default:
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	summary, _, _, err := GetUserPointDetail(userID)
	if err != nil {
		log.Printf("Gagal mengambil saldo poin: %v", err)
		http.Error(w, "Gagal mengambil saldo poin.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user_id":           summary.UserID,
		"username":          summary.Username,
		"active_points":     summary.ActivePoints,
		"rupiah_value":      summary.RupiahValue,
		"expiring_soon":     summary.ExpiringSoon,
		"expiring_rupiah":   summary.ExpiringRupiah,
		"next_expiry":       summary.NextExpiry,
		"lifetime_earned":   summary.LifetimeEarned,
		"lifetime_redeemed": summary.LifetimeRedeemed,
		"lifetime_expired":  summary.LifetimeExpired,
		"rupiah_per_point":  RupiahPerPoint,
		"validity_months":   PointValidityMonths,
	})
}

// formatRupiah memformat angka rupiah sederhana untuk pesan flash.
func formatRupiah(value int) string {
	negative := value < 0
	if negative {
		value = -value
	}
	digits := strconv.Itoa(value)
	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)
	result := "Rp"
	for i, part := range parts {
		if i > 0 {
			result += "."
		}
		result += part
	}
	if negative {
		return "-" + result
	}
	return result
}


func main() {
	rand.Seed(time.Now().UnixNano())

	if err := initDB(); err != nil {
		log.Fatalf("Database Kopiku tidak siap: %v", err)
	}
	defer db.Close()
	log.Println("Berhasil terhubung ke database kopiku_db")

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

	// Poin: lot, penukaran FIFO, pembalikan, restorasi, dan laporan.
	mux.HandleFunc("/admin/poin", pointsReportHandler)
	mux.HandleFunc("/admin/poin/add", pointsAddHandler)
	mux.HandleFunc("/admin/poin/redeem", pointsRedeemHandler)
	mux.HandleFunc("/admin/poin/reverse", pointsReverseHandler)
	mux.HandleFunc("/admin/poin/restore", pointsRestoreHandler)
	mux.HandleFunc("/poin/redeem", userRedeemPointsHandler)
	mux.HandleFunc("/api/points/balance", apiPointsBalanceHandler)

	server := &http.Server{
		Addr:              "localhost:8000",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("Server berjalan di http://localhost:8000")
	log.Fatal(server.ListenAndServe())
}
