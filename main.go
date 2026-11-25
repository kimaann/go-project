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
	"sort"
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
	IsMember     bool
	MemberSince  *time.Time
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

type MemberPage struct {
	PageData
	User        *User
	IsMember    bool
	MemberSince *time.Time
	MemberID    string
	Points      int
	TierName    string
	TierIcon    string
	Lots        []PointLot
}

type MembersListPage struct {
	PageData
	Members     []User
	Total       int
	TotalPoints int
	BronzeCount int
	SilverCount int
	GoldCount   int
	PlatCount   int
	Query       string
	TierFilter  string
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
	"mul": func(a, b int) int { return a * b },
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
	tplAdminMembers = parseSimple("admin_members.html", "templates/admin_members.html")
	tplPointReport  = parsePage("templates/poin_report.html")
	tplMembership   = parsePage("templates/membership.html")
	tplMembersList  = parsePage("templates/members.html")
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

// ===== Aturan Membership Tier =====

const GoldMinimumSpending = 1500000

func GetTier(memberID string, waktu time.Time) (MemberTier, int64, error) {
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		return "", 0, fmt.Errorf("ID member tidak boleh kosong")
	}
	waktuMulai := waktu.AddDate(0, -3, 0)
	var totalBelanja int64
	err := db.QueryRowContext(
		context.Background(),
		`SELECT COALESCE(SUM(subtotal - voucher_discount), 0)
		 FROM transactions
		 WHERE member_id = ? AND status = 'paid' AND created_at >= ? AND created_at <= ?`,
		memberID, waktuMulai, waktu,
	).Scan(&totalBelanja)
	if err != nil {
		return "", 0, fmt.Errorf("gagal menghitung total belanja member: %w", err)
	}
	if totalBelanja >= GoldMinimumSpending {
		return TierGold, totalBelanja, nil
	}
	return TierSilver, totalBelanja, nil
}

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
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	_, err := db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("gagal membuat tabel member_tier_history: %w", err)
	}
	return nil
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
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("gagal menyiapkan tabel users di database kopiku_db: %w", err)
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE users CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return fmt.Errorf("gagal menyelaraskan collation tabel users: %w", err)
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
	if err := initCashierSchema(pointCtx); err != nil {
		return fmt.Errorf("gagal menyiapkan sistem kasir: %w", err)
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
	var isMember bool
	var memberSince sql.NullTime
	err := db.QueryRowContext(context.Background(), `
		SELECT id, name, username, phone, COALESCE(email,''), password_hash, role, points, is_active,
		       COALESCE(is_member,0), member_since
		FROM users
		WHERE username = ?
		LIMIT 1`, username).Scan(
		&u.ID, &u.Name, &u.Username, &u.Phone, &u.Email,
		&passwordHash, &u.Role, &u.Points, &active, &isMember, &memberSince,
	)
	if err != nil {
		return nil, "", err
	}
	u.PasswordHash = passwordHash
	u.IsActive = active
	u.IsMember = isMember
	if memberSince.Valid {
		u.MemberSince = &memberSince.Time
	}
	return &u, passwordHash, nil
}

// getAllUsers mengambil daftar pengguna untuk tabel pengguna di dashboard admin.
func getAllUsers() ([]User, error) {
	rows, err := db.QueryContext(context.Background(), `
		SELECT id, name, username, COALESCE(email,''), phone, role, points, is_active,
		       COALESCE(is_member,0), member_since
		FROM users
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		var isMember bool
		var memberSince sql.NullTime
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.Phone, &u.Role, &u.Points, &u.IsActive, &isMember, &memberSince); err != nil {
			return nil, err
		}
		u.IsMember = isMember
		if memberSince.Valid {
			u.MemberSince = &memberSince.Time
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

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	sess := getSession(r)

	if sess == nil || sess.Role != "admin" {
		http.Error(w, "Akses ditolak. Halaman ini khusus admin.", http.StatusForbidden)
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

// ===== Membership Handlers (Customer) =====

func membershipHandler(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if sess == nil {
		addFlash(w, "Silakan login terlebih dahulu untuk mengakses halaman membership.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	u, _, err := getUserByUsername(sess.Username)
	if err != nil {
		http.Error(w, "Pengguna tidak ditemukan", http.StatusNotFound)
		return
	}

	pts, err := ActivePointBalance(u.ID)
	if err == nil {
		u.Points = pts
	}

	page := MemberPage{
		PageData: PageData{
			Title:     "Membership Kopiku",
			ActiveNav: "membership",
			Session:   sess,
			Flash:     getFlash(w, r),
		},
		User:        u,
		IsMember:    u.IsMember,
		MemberSince: u.MemberSince,
		MemberID:    fmt.Sprintf("KPK-%s", strings.ToUpper(u.ID)),
		Points:      u.Points,
		TierName:    funcs["tierName"].(func(int) string)(u.Points),
		TierIcon:    funcs["tierIcon"].(func(int) string)(u.Points),
	}

	if u.IsMember {
		if _, lots, _, err := GetUserPointDetail(u.ID); err == nil {
			page.Lots = lots
		}
	}

	if err := tplMembership.ExecuteTemplate(w, "layout", page); err != nil {
		log.Printf("Gagal render membership template: %v", err)
		http.Error(w, "Terjadi kesalahan internal", http.StatusInternalServerError)
	}
}

func membershipApplyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess := getSession(r)
	if sess == nil {
		addFlash(w, "Silakan login terlebih dahulu untuk mendaftar membership.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	now := time.Now()
	_, err := db.ExecContext(context.Background(), `
		UPDATE users 
		SET is_member = 1, 
		    member_since = COALESCE(member_since, ?)
		WHERE id = ?`, now, sess.UserID)
	if err != nil {
		log.Printf("Gagal mendaftar membership: %v", err)
		addFlash(w, "Gagal mengaktifkan membership. Silakan coba kembali.")
		http.Redirect(w, r, "/membership", http.StatusSeeOther)
		return
	}

	// Berikan Welcome Bonus Poin 50 jika belum pernah menerima
	var hasWelcomeLot int
	_ = db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM point_lots 
		WHERE user_id = ? AND note LIKE '%Welcome Bonus%'`, sess.UserID).Scan(&hasWelcomeLot)
	if hasWelcomeLot == 0 {
		_, _ = AddPoints(sess.UserID, 50, "", "Welcome Bonus Membership Kopiku")
	}

	addFlash(w, "🎉 Selamat! Anda resmi terdaftar sebagai Member Kopiku! Bonus 50 poin telah masuk ke akun Anda.")
	http.Redirect(w, r, "/membership", http.StatusSeeOther)
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

func adminMembersHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}

	sess := getSession(r)

	users, err := getAllUsers()
	if err != nil {
		log.Printf("Gagal mengambil data member: %v", err)
		http.Error(w, "Gagal mengambil data member", http.StatusInternalServerError)
		return
	}

	// Filter hanya pengguna dengan role user
	memberUsers := make([]User, 0)

	for _, u := range users {
		if strings.EqualFold(strings.TrimSpace(u.Role), "user") {
			if pts, err := ActivePointBalance(u.ID); err == nil {
				u.Points = pts
			}
			memberUsers = append(memberUsers, u)
		}
	}

	data := AdminPage{
		PageData: PageData{
			Title:     "Data Member",
			ActiveNav: "adminmembers",
			Session:   sess,
		},
		Users: memberUsers,
	}

	err = tplAdminMembers.ExecuteTemplate(w, "admin_members.html", data)
	if err != nil {
		log.Printf("Gagal membuka halaman data member: %v", err)
		http.Error(w, "Gagal membuka halaman data member", http.StatusInternalServerError)
	}
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

func adminToggleMemberHandler(w http.ResponseWriter, r *http.Request) {
	if !requireStaff(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		http.Error(w, "User ID wajib diisi", http.StatusBadRequest)
		return
	}

	var isMember bool
	err := db.QueryRowContext(context.Background(), `SELECT COALESCE(is_member, 0) FROM users WHERE id = ?`, userID).Scan(&isMember)
	if err != nil {
		addFlash(w, "Pengguna tidak ditemukan.")
		http.Redirect(w, r, "/admin/members", http.StatusSeeOther)
		return
	}

	newStatus := 1
	if isMember {
		newStatus = 0
	}

	_, err = db.ExecContext(context.Background(), `
		UPDATE users 
		SET is_member = ?,
		    member_since = CASE WHEN ? = 1 AND member_since IS NULL THEN NOW() ELSE member_since END
		WHERE id = ?`, newStatus, newStatus, userID)
	if err != nil {
		addFlash(w, "Gagal mengubah status membership.")
	} else {
		if newStatus == 1 {
			addFlash(w, "Status membership pengguna berhasil diaktifkan.")
		} else {
			addFlash(w, "Status membership pengguna berhasil dinonaktifkan.")
		}
	}

	http.Redirect(w, r, "/admin/members", http.StatusSeeOther)
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

// ============================================================================
// MODUL KASIR & KERANJANG (TERINTEGRASI)
// ============================================================================

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

var cartConfig = DefaultCartConfig()

const (
	maxCartLines = 100
	maxLineQty   = 1000
)

// ===== Titik sambung antar modul =====

var (
	cartNow = time.Now

	cartActivePromos = func(ctx context.Context, branch string, at time.Time) ([]CartPromo, error) {
		if cartDemoPromosEnabled() {
			return demoCartPromos(), nil
		}
		return nil, nil
	}

	cartMemberTier = func(ctx context.Context, memberID string, at time.Time) (MemberTier, error) {
		pts, _ := ActivePointBalance(memberID)
		if pts >= 1500 {
			return TierGold, nil
		}
		return TierSilver, nil
	}

	cartPointBalance = func(ctx context.Context, memberID string, at time.Time) (int, error) {
		return ActivePointBalance(memberID)
	}

	cartLoadProducts = loadProductsByIDs
	cartLoadVoucher  = loadVoucherAsPromo
)

func cartDemoPromosEnabled() bool { return os.Getenv("CART_DEMO_PROMOS") == "1" }

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
		SELECT id, name, username, phone, email, COALESCE(is_member, 0)
		FROM users
		WHERE role = 'user' AND is_active = 1
		  AND (name LIKE ? OR username LIKE ? OR phone LIKE ? OR email LIKE ?)
		ORDER BY is_member DESC, name ASC
		LIMIT 10`, like, like, like, like)
	if err != nil {
		http.Error(w, "Gagal mencari member", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type memberDTO struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Username     string `json:"username"`
		Phone        string `json:"phone"`
		Email        string `json:"email"`
		ActivePoints int    `json:"active_points"`
		RupiahValue  int    `json:"rupiah_value"`
		IsMember     bool   `json:"is_member"`
		Tier         string `json:"tier"`
	}

	results := make([]memberDTO, 0)
	for rows.Next() {
		var m memberDTO
		var isMember bool
		if err := rows.Scan(&m.ID, &m.Name, &m.Username, &m.Phone, &m.Email, &isMember); err != nil {
			continue
		}
		m.IsMember = isMember
		// Sinkronisasi kedaluwarsa & hitung saldo aktif real-time
		pts, err := ActivePointBalance(m.ID)
		if err == nil {
			m.ActivePoints = pts
			m.RupiahValue = pts * RupiahPerPoint
		}
		m.Tier = funcs["tierName"].(func(int) string)(m.ActivePoints)
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
	mux.HandleFunc("/membership", membershipHandler)
	mux.HandleFunc("/membership/apply", membershipApplyHandler)

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
	mux.HandleFunc("/admin/members", adminMembersHandler)
	mux.HandleFunc("/admin/members/toggle", adminToggleMemberHandler)
	mux.HandleFunc("/admin/order/update", adminUpdateOrderHandler)
	mux.HandleFunc("/admin/order/create", adminCreateOrderHandler)

	// Kasir POS & Riwayat Transaksi
	mux.HandleFunc("/kasir", kasirHandler)
	mux.HandleFunc("/api/kasir/members", apiKasirSearchMemberHandler)
	mux.HandleFunc("/api/kasir/voucher/check", apiKasirCheckVoucherHandler)
	mux.HandleFunc("/api/kasir/checkout", apiKasirCheckoutHandler)
	mux.HandleFunc("/keranjang", keranjangHandler)
	mux.HandleFunc("/api/kasir/cart/preview", apiKasirCartPreviewHandler)
	mux.HandleFunc("/transaksi", transaksiListHandler)
	mux.HandleFunc("/transaksi/", transaksiDetailHandler)

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

