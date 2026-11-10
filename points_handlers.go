package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

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
