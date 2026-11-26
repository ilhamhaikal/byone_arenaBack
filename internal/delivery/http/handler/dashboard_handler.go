package handler

import (
	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DashboardHandler menangani HTTP request untuk dashboard & laporan
type DashboardHandler struct {
	paymentRepo repository.PaymentRepository
	db          *gorm.DB
}

// NewDashboardHandler membuat instance baru DashboardHandler
func NewDashboardHandler(paymentRepo repository.PaymentRepository, db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{paymentRepo: paymentRepo, db: db}
}

// GetSummary godoc
// @Summary      Ringkasan dashboard
// @Description  Mengembalikan ringkasan pendapatan harian, penggunaan voucher, sesi aktif, dan status konsol.\n\n**Memerlukan autentikasi.**\n\nQuery param `date` opsional (default hari ini): `?date=2026-07-03`
// @Tags         Dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        date  query     string  false  "Tanggal (YYYY-MM-DD), default hari ini"
// @Success      200   {object}  response.Response{data=entity.DashboardSummary}
// @Failure      401   {object}  response.ErrorResponse
// @Failure      500   {object}  response.ErrorResponse
// @Router       /api/v1/dashboard/summary [get]
func (h *DashboardHandler) GetSummary(c *fiber.Ctx) error {
	date := c.Query("date", "") // kosong = hari ini
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	summary, err := h.paymentRepo.GetDashboardSummary(c.Context(), date)
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil ringkasan dashboard")
	}

	// Rincian pendapatan per metode pembayaran (tunai/qris/…) hari ini.
	// SP ini menghitung SEMUA sumber (termasuk makanan); komponen makanan
	// dibuang di bawah karena makanan punya kartu sendiri.
	// Gagal → kosong (UI menampilkan fallback), jangan gagalkan summary.
	revenueMethods := []entity.ReportPaymentMethod{}
	if methods, mErr := h.paymentRepo.GetPaymentMethodSummary(c.Context(), date, date); mErr == nil {
		revenueMethods = methods
	}

	// Rincian penjualan makanan per metode pembayaran hari ini.
	// Kolom payment_method ditambahkan oleh migrasi 000021 — bila DB client
	// belum termigrasi, lewati dengan halus (jangan tulis ERROR ke log).
	foodMethods := []entity.ReportPaymentMethod{}
	silent := h.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	// SP mengembalikan kolom `payment_method`; petakan eksplisit ke entity.
	type foodMethodRow struct {
		Method string  `gorm:"column:payment_method"`
		Total  float64 `gorm:"column:total"`
		Trx    int     `gorm:"column:trx"`
	}
	var foodRows []foodMethodRow
	silent.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("FoodPaymentMethodSummary")+"(?::DATE, ?::DATE)",
		date, date,
	).Scan(&foodRows)
	for _, r := range foodRows {
		foodMethods = append(foodMethods, entity.ReportPaymentMethod{
			Method: r.Method,
			Total:  r.Total,
			Trx:    r.Trx,
		})
	}
	summary.FoodMethods = foodMethods

	// Kartu "Pendapatan" = sewa (sesi) + rental harian + membership, TANPA
	// makanan (makanan punya kartu sendiri). Buang komponen makanan dari
	// rincian per metode agar total dan rinciannya konsisten.
	summary.RevenueMethods = subtractPaymentMethods(revenueMethods, foodMethods)

	// Sesi terputus (listrik mati / TV offline) yang bisa dipulihkan admin.
	// Logika query ada di stored procedure InterruptedSessions.
	var interrupted []entity.InterruptedSessionSummary
	_ = h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("InterruptedSessions")+"(?)", 20,
	).Scan(&interrupted).Error
	if interrupted == nil {
		interrupted = []entity.InterruptedSessionSummary{}
	}
	summary.InterruptedSessions = interrupted

	return response.OK(c, "Ringkasan dashboard berhasil diambil", summary)
}

// subtractPaymentMethods mengurangkan komponen per metode dari daftar rincian
// pembayaran. Dipakai untuk membuang komponen "makanan" dari kartu Pendapatan,
// sehingga kartu itu hanya berisi sewa konsol + rental harian (+ membership).
func subtractPaymentMethods(all, minus []entity.ReportPaymentMethod) []entity.ReportPaymentMethod {
	minusTotal := make(map[string]float64, len(minus))
	minusTrx := make(map[string]int, len(minus))
	for _, m := range minus {
		minusTotal[m.Method] += m.Total
		minusTrx[m.Method] += m.Trx
	}

	out := make([]entity.ReportPaymentMethod, 0, len(all))
	for _, m := range all {
		total := m.Total - minusTotal[m.Method]
		trx := m.Trx - minusTrx[m.Method]
		if total < 0 {
			total = 0
		}
		if trx < 0 {
			trx = 0
		}
		if total == 0 && trx == 0 {
			continue
		}
		out = append(out, entity.ReportPaymentMethod{Method: m.Method, Total: total, Trx: trx})
	}
	return out
}

// RevenueChartItem satu titik data grafik pendapatan.
// rentalRevenue     = sewa konsol (sesi) + rental harian.
// membershipRevenue = penjualan membership.
// foodRevenue       = penjualan makanan/minuman.
type RevenueChartItem struct {
	Date              string  `json:"date"              gorm:"column:date"`
	RentalRevenue     float64 `json:"rentalRevenue"     gorm:"column:rental_revenue"`
	MembershipRevenue float64 `json:"membershipRevenue" gorm:"column:membership_revenue"`
	FoodRevenue       float64 `json:"foodRevenue"       gorm:"column:food_revenue"`
	Revenue           float64 `json:"revenue"           gorm:"column:revenue"`
	Sessions          int     `json:"sessions"          gorm:"column:sessions"`
}

// GetRevenueChart godoc
// @Summary      Grafik pendapatan harian (2 seri: rental & makanan)
// @Description  Data pendapatan per hari untuk N hari terakhir (default 7), dipisah menjadi seri `rentalRevenue` (sewa konsol + rental harian + membership) dan `foodRevenue` (makanan/minuman) agar bisa digambar dalam satu grafik dengan gaya berbeda.
// @Tags         Dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        days  query     int  false  "Jumlah hari (default 7, max 30)"
// @Success      200   {object}  response.Response{data=[]handler.RevenueChartItem}
// @Router       /api/v1/dashboard/revenue-chart [get]
func (h *DashboardHandler) GetRevenueChart(c *fiber.Ctx) error {
	days := c.QueryInt("days", 7)
	if days < 1 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	// Diambil dari stored procedure (bukan query di aplikasi) agar perbaikan
	// logika cukup di procedure, tanpa perlu build ulang aplikasi.
	var items []RevenueChartItem
	if err := h.db.Raw(
		"SELECT * FROM "+spname.Ident("RevenueChart")+"(?)", days,
	).Scan(&items).Error; err != nil {
		return response.InternalServerError(c, "Gagal mengambil data grafik")
	}

	// Fallback kompatibilitas: bila SP masih versi lama (hanya kolom revenue),
	// tampilkan total sebagai seri rental agar grafik tidak kosong.
	for i := range items {
		if items[i].RentalRevenue == 0 && items[i].FoodRevenue == 0 && items[i].Revenue > 0 {
			items[i].RentalRevenue = items[i].Revenue
		}
		items[i].Revenue = items[i].RentalRevenue + items[i].FoodRevenue
	}
	if len(items) == 0 {
		items = []RevenueChartItem{}
	}

	return response.OK(c, "Data grafik pendapatan", items)
}
