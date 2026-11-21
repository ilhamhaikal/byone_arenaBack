package handler

import (
	"sort"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReportHandler menangani HTTP request untuk laporan
type ReportHandler struct {
	paymentRepo repository.PaymentRepository
	db          *gorm.DB
}

// NewReportHandler membuat instance baru ReportHandler
func NewReportHandler(paymentRepo repository.PaymentRepository, db *gorm.DB) *ReportHandler {
	return &ReportHandler{paymentRepo: paymentRepo, db: db}
}

// GetSummary godoc
// @Summary      Laporan komprehensif (rentang tanggal)
// @Description  Mengembalikan laporan lengkap: pendapatan, penggunaan voucher, total jam main, rincian per konsol, rincian per hari.\n\n**Memerlukan autentikasi.**\n\nQuery param:\n- `startDate` (YYYY-MM-DD, default 7 hari lalu)
// - `endDate` (YYYY-MM-DD, default hari ini)
// @Tags         Laporan
// @Produce      json
// @Security     BearerAuth
// @Param        startDate  query     string  false  "Tanggal mulai (YYYY-MM-DD)"
// @Param        endDate    query     string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Success      200  {object}  response.Response{data=entity.ReportSummary}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/reports/summary [get]
func (h *ReportHandler) GetSummary(c *fiber.Ctx) error {
	startDate := c.Query("startDate", "")
	endDate := c.Query("endDate", "")

	report, err := h.paymentRepo.GetReportSummary(c.Context(), startDate, endDate)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	if methods, mErr := h.paymentRepo.GetPaymentMethodSummary(c.Context(), startDate, endDate); mErr == nil {
		report.PaymentMethods = methods
	}

	return response.OK(c, "Laporan berhasil diambil", report)
}

// GetStaffReport godoc
// @Summary      Laporan pendapatan & login per petugas
// @Description  Laporan per user: login–logout, pendapatan rental, makanan, rental harian, membership, dan game.
// @Tags         Laporan
// @Produce      json
// @Security     BearerAuth
// @Param        startDate  query  string  false  "Tanggal mulai (YYYY-MM-DD)"
// @Param        endDate    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        userId     query  string  false  "Filter user (UUID)"
// @Success      200  {object}  response.Response{data=entity.StaffReport}
// @Router       /api/v1/reports/staff [get]
func (h *ReportHandler) GetStaffReport(c *fiber.Ctx) error {
	startDate := c.Query("startDate", "")
	endDate := c.Query("endDate", "")
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	var userID *uuid.UUID
	if raw := c.Query("userId", ""); raw != "" {
		if parsed, err := uuid.Parse(raw); err == nil {
			userID = &parsed
		}
	}

	var userArg interface{}
	if userID != nil {
		userArg = *userID
	}

	var revenueRows []entity.StaffDailyRevenueRow
	if err := h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("StaffDailyRevenue")+"(?::DATE, ?::DATE, ?::UUID)",
		startDate, endDate, userArg,
	).Scan(&revenueRows).Error; err != nil {
		return response.BadRequest(c, err.Error())
	}

	var loginRows []entity.StaffLoginRow
	if err := h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("StaffLoginSessions")+"(?::DATE, ?::DATE, ?::UUID)",
		startDate, endDate, userArg,
	).Scan(&loginRows).Error; err != nil {
		return response.BadRequest(c, err.Error())
	}

	// Kumpulkan ID user untuk lookup nama/role.
	idSet := make(map[uuid.UUID]struct{})
	for _, row := range revenueRows {
		if row.UserID != nil {
			idSet[*row.UserID] = struct{}{}
		}
	}
	for _, row := range loginRows {
		idSet[row.UserID] = struct{}{}
	}
	ids := make([]uuid.UUID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	userMap := make(map[uuid.UUID]entity.User)
	if len(ids) > 0 {
		var users []entity.User
		_ = h.db.WithContext(c.Context()).Where("id IN ?", ids).Find(&users).Error
		for _, u := range users {
			userMap[u.ID] = u
		}
	}

	staffMap := make(map[string]*entity.StaffReportUser)
	var unknown *entity.StaffReportUser

	ensureUser := func(id *uuid.UUID) *entity.StaffReportUser {
		if id == nil {
			if unknown == nil {
				unknown = &entity.StaffReportUser{
					Username: "Tidak diketahui",
					FullName: "Tidak diketahui",
					Role:     "-",
				}
			}
			return unknown
		}
		key := id.String()
		if u := staffMap[key]; u != nil {
			return u
		}
		u := &entity.StaffReportUser{UserID: id, Username: "-", FullName: "-", Role: "-"}
		if info, ok := userMap[*id]; ok {
			u.Username = info.Username
			u.FullName = info.FullName
			u.Role = string(info.Role)
		}
		staffMap[key] = u
		return u
	}

	ensureDay := func(u *entity.StaffReportUser, date string) *entity.StaffReportDay {
		for i := range u.Days {
			if u.Days[i].Date == date {
				return &u.Days[i]
			}
		}
		u.Days = append(u.Days, entity.StaffReportDay{
			Date:     date,
			Sessions: []entity.StaffLoginSession{},
		})
		return &u.Days[len(u.Days)-1]
	}

	for _, row := range revenueRows {
		u := ensureUser(row.UserID)
		day := ensureDay(u, row.ReportDate.Format("2006-01-02"))
		day.Revenue.Rental += row.RentalRevenue
		day.Revenue.Food += row.FoodRevenue
		day.Revenue.DailyRental += row.DailyRentalRevenue
		day.Revenue.Membership += row.MembershipRevenue
		day.Revenue.RentalTrx += row.RentalTrx
		day.Revenue.FoodTrx += row.FoodTrx
		day.Revenue.DailyRentalTrx += row.DailyRentalTrx
		day.Revenue.MembershipTrx += row.MembershipTrx
	}

	for _, row := range loginRows {
		u := ensureUser(&row.UserID)
		day := ensureDay(u, row.LoginAt.Format("2006-01-02"))
		day.Sessions = append(day.Sessions, entity.StaffLoginSession{
			LoginAt:         row.LoginAt,
			LogoutAt:        row.LogoutAt,
			DurationMinutes: row.DurationMinutes,
			StillLoggedIn:   row.StillLoggedIn,
		})
	}

	staff := make([]entity.StaffReportUser, 0, len(staffMap))
	for _, u := range staffMap {
		finalizeStaffUser(u)
		staff = append(staff, *u)
	}
	if unknown != nil {
		finalizeStaffUser(unknown)
	}
	sort.Slice(staff, func(i, j int) bool {
		if staff[i].FullName == staff[j].FullName {
			return staff[i].Username < staff[j].Username
		}
		return staff[i].FullName < staff[j].FullName
	})

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}

	return response.OK(c, "Laporan per petugas berhasil diambil", entity.StaffReport{
		Period: entity.ReportPeriod{
			StartDate: startDate,
			EndDate:   endDate,
			TotalDays: days,
		},
		Staff:       staff,
		Unknown:     unknown,
		GeneratedAt: time.Now(),
	})
}

func finalizeStaffUser(u *entity.StaffReportUser) {
	sort.Slice(u.Days, func(i, j int) bool { return u.Days[i].Date < u.Days[j].Date })
	for i := range u.Days {
		sort.Slice(u.Days[i].Sessions, func(a, b int) bool {
			return u.Days[i].Sessions[a].LoginAt.Before(u.Days[i].Sessions[b].LoginAt)
		})
		r := u.Days[i].Revenue
		r.Total = r.Rental + r.Food + r.DailyRental + r.Membership + r.Game
		r.TotalTrx = r.RentalTrx + r.FoodTrx + r.DailyRentalTrx + r.MembershipTrx + r.GameTrx
		u.Days[i].Revenue = r

		u.Totals.Rental += r.Rental
		u.Totals.Food += r.Food
		u.Totals.DailyRental += r.DailyRental
		u.Totals.Membership += r.Membership
		u.Totals.Game += r.Game
		u.Totals.RentalTrx += r.RentalTrx
		u.Totals.FoodTrx += r.FoodTrx
		u.Totals.DailyRentalTrx += r.DailyRentalTrx
		u.Totals.MembershipTrx += r.MembershipTrx
		u.Totals.GameTrx += r.GameTrx
	}
	u.Totals.Total = u.Totals.Rental + u.Totals.Food + u.Totals.DailyRental + u.Totals.Membership + u.Totals.Game
	u.Totals.TotalTrx = u.Totals.RentalTrx + u.Totals.FoodTrx + u.Totals.DailyRentalTrx + u.Totals.MembershipTrx + u.Totals.GameTrx
}
