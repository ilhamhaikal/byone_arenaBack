package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/license"
	"byone-arena/pkg/audit"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// maxDailyRentalDays membatasi durasi rental harian agar harga & jadwal tidak
// dibuat terlalu jauh ke depan.
const maxDailyRentalDays = 30

// RentalHandler menangani daily rental + booking
type RentalHandler struct {
	db        *gorm.DB
	validator *validator.Validator
	license   *license.Service
}

func NewRentalHandler(db *gorm.DB, v *validator.Validator, licenseSvc *license.Service) *RentalHandler {
	return &RentalHandler{db: db, validator: v, license: licenseSvc}
}

// ============== DAILY RENTAL ==============

// CreateDailyRental godoc
// @Summary      Buat rental harian (console dibawa pulang)
// @Tags         Rental Harian
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object  true  "Data rental: consoleId, customerId, startDate, endDate, voucherCode, notes (dailyPrice dari client diabaikan)"
// @Success      201  {object}  response.Response
// @Router       /api/v1/daily-rentals [post]
func (h *RentalHandler) CreateDailyRental(c *fiber.Ctx) error {
	var req struct {
		ConsoleID     uuid.UUID  `json:"consoleId"     validate:"required"`
		CustomerID    *uuid.UUID `json:"customerId"`
		StartDate     string     `json:"startDate"     validate:"required"`
		EndDate       string     `json:"endDate"       validate:"required"`
		DailyPrice    float64    `json:"dailyPrice"` // diabaikan; harga ditentukan server
		VoucherCode   string     `json:"voucherCode"`
		Notes         string     `json:"notes"`
		PaymentMethod string     `json:"paymentMethod" validate:"omitempty,oneof=cash qris transfer"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(&req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	// Lisensi: blokir rental harian baru bila lisensi sudah berakhir.
	if h.license != nil {
		if err := h.license.EnsureCanStartBusiness(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	// Harga TIDAK diambil dari client. Nilai dari request hanya dipakai untuk
	// audit bila berbeda (kompatibilitas aplikasi lama).
	requestedDailyPrice := req.DailyPrice
	resolvedDailyPrice, err := h.resolveDailyPrice(c, req.ConsoleID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	req.DailyPrice = resolvedDailyPrice

	if err := validateDailyRentalDates(req.StartDate, req.EndDate); err != nil {
		return response.BadRequest(c, err.Error())
	}

	// --- Daily Rental SP ---
	type spResult struct {
		RentalID       uuid.UUID `gorm:"column:rental_id"`
		TotalDays      int       `gorm:"column:total_days"`
		TotalAmount    float64   `gorm:"column:total_amount"`
		DiscountAmount float64   `gorm:"column:discount_amount"`
		FreeDaysUsed   int       `gorm:"column:free_days_used"`
		FinalAmount    float64   `gorm:"column:final_amount"`
		Status         string    `gorm:"column:status"`
	}
	var result spResult
	method := normalizePaymentMethod(req.PaymentMethod)
	var createdBy *uuid.UUID
	if uid, ok := c.Locals("userID").(uuid.UUID); ok {
		createdBy = &uid
	}
	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(
			fmt.Sprintf(`SELECT * FROM %s(?, ?, ?::DATE, ?::DATE, ?, ?, ?)`, spname.Ident("CreateDailyRental")),
			req.ConsoleID, req.CustomerID, req.StartDate, req.EndDate,
			req.DailyPrice, req.VoucherCode, req.Notes,
		).Scan(&result).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			"SELECT "+spname.Ident("SetDailyRentalPaymentMethod")+"(?::uuid, ?::varchar)",
			result.RentalID, method,
		).Error; err != nil {
			return err
		}
		if createdBy != nil {
			return tx.Model(&entity.DailyRental{}).
				Where("id = ?", result.RentalID).
				Update("created_by", *createdBy).Error
		}
		return nil
	})
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	audit.Log(h.db, c, "daily_rental.create", "daily_rental", &result.RentalID, map[string]any{
		"consoleId":           req.ConsoleID,
		"dailyPrice":          req.DailyPrice,
		"requestedDailyPrice": requestedDailyPrice, // hanya untuk audit; tidak dipakai
		"totalDays":           result.TotalDays,
		"totalAmount":         result.TotalAmount,
		"finalAmount":         result.FinalAmount,
	})

	// Metode pembayaran sudah disimpan atomik bersama pembuatan rental.

	// Load full object
	var rental entity.DailyRental
	h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").
		First(&rental, "id = ?", result.RentalID)

	return response.Created(c, "Rental harian berhasil dibuat", rental)
}

// resolveDailyPrice menentukan harga rental harian dari server:
// harga khusus konsol, lalu setting global. Nilai dari client tidak dipakai.
func (h *RentalHandler) resolveDailyPrice(c *fiber.Ctx, consoleID uuid.UUID) (float64, error) {
	var console struct {
		DailyPrice float64 `gorm:"column:daily_price"`
	}
	if err := h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("ConsoleDailyPrice")+"(?::uuid)", consoleID,
	).Scan(&console).Error; err != nil {
		return 0, err
	}
	if console.DailyPrice > 0 {
		return selectDailyPrice(console.DailyPrice, 0)
	}

	var setting struct {
		Value string `gorm:"column:value"`
	}
	if err := h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "daily_price",
	).Scan(&setting).Error; err != nil {
		return 0, err
	}

	globalPrice, _ := strconv.ParseFloat(strings.TrimSpace(setting.Value), 64)
	return selectDailyPrice(0, globalPrice)
}

// selectDailyPrice menerapkan aturan bisnis: harga khusus konsol menang,
// lalu fallback ke harga global. Keduanya 0 berarti belum diatur.
func selectDailyPrice(consolePrice, globalPrice float64) (float64, error) {
	if consolePrice > 0 {
		return consolePrice, nil
	}
	if globalPrice > 0 {
		return globalPrice, nil
	}
	return 0, errors.New("Harga per hari belum diatur. Admin: PUT /settings/daily-price")
}

// validateDailyRentalDates memastikan format tanggal, urutan, dan batas durasi
// rental harian sesuai aturan bisnis.
func validateDailyRentalDates(startRaw, endRaw string) error {
	startDate, err := time.Parse("2006-01-02", startRaw)
	if err != nil {
		return errors.New("Format tanggal mulai harus YYYY-MM-DD")
	}
	endDate, err := time.Parse("2006-01-02", endRaw)
	if err != nil {
		return errors.New("Format tanggal selesai harus YYYY-MM-DD")
	}
	if !endDate.After(startDate) {
		return errors.New("Tanggal selesai harus setelah tanggal mulai")
	}
	if endDate.Sub(startDate) > maxDailyRentalDays*24*time.Hour {
		return fmt.Errorf("Rental harian maksimal %d hari", maxDailyRentalDays)
	}
	return nil
}

// GetAllDailyRentals godoc
// @Summary      List semua rental harian
// @Tags         Rental Harian
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.DailyRental}
// @Router       /api/v1/daily-rentals [get]
func (h *RentalHandler) GetAllDailyRentals(c *fiber.Ctx) error {
	var list []entity.DailyRental
	h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").
		Order("created_at DESC").Find(&list)
	if list == nil {
		list = []entity.DailyRental{}
	}
	return response.OK(c, "Data rental harian", list)
}

// ReturnDailyRental godoc
// @Summary      Kembalikan rental harian
// @Tags         Rental Harian
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Rental ID"
// @Success      200  {object}  response.Response
// @Router       /api/v1/daily-rentals/{id}/return [post]
func (h *RentalHandler) ReturnDailyRental(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	tx := h.db.WithContext(c.Context()).Exec(fmt.Sprintf(`SELECT %s(?)`, spname.Ident("ReturnDailyRental")), id)
	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}
	// Return full object
	var rental entity.DailyRental
	h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").First(&rental, "id = ?", id)
	return response.OK(c, "Konsol berhasil dikembalikan", rental)
}

// ============== BOOKING ==============

// CreateBooking godoc
// @Summary      Buat booking/reservasi konsol
// @Tags         Booking
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object  true  "Data booking: consoleId, customerId, bookingDate, startHour, startMinute, durationMinutes, notes"
// @Success      201  {object}  response.Response
// @Router       /api/v1/bookings [post]
func (h *RentalHandler) CreateBooking(c *fiber.Ctx) error {
	var req struct {
		ConsoleID       uuid.UUID  `json:"consoleId"       validate:"required"`
		CustomerID      *uuid.UUID `json:"customerId"      validate:"omitempty"`
		BookingDate     string     `json:"bookingDate"     validate:"required"`
		StartHour       int        `json:"startHour"       validate:"required,min=0,max=23"`
		StartMinute     int        `json:"startMinute"     validate:"min=0,max=59"`
		DurationMinutes int        `json:"durationMinutes" validate:"required,min=30"`
		Notes           string     `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(&req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	// Lisensi: blokir booking baru bila lisensi sudah berakhir.
	if h.license != nil {
		if err := h.license.EnsureCanStartBusiness(c.Context()); err != nil {
			return response.Forbidden(c, err.Error())
		}
	}

	type spResult struct {
		BookingID uuid.UUID `gorm:"column:booking_id"`
		Status    string    `gorm:"column:status"`
	}
	var result spResult
	tx := h.db.WithContext(c.Context()).Raw(
		fmt.Sprintf(`SELECT * FROM %s(?, ?, ?::DATE, ?, ?, ?, ?)`, spname.Ident("CreateBooking")),
		req.ConsoleID, req.CustomerID, req.BookingDate,
		req.StartHour, req.StartMinute, req.DurationMinutes, req.Notes,
	).Scan(&result)
	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	// Load full object
	var booking entity.Booking
	h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").
		First(&booking, "id = ?", result.BookingID)
	return response.Created(c, "Booking berhasil dibuat", booking)
}

// GetAllBookings godoc
// @Summary      List semua booking
// @Tags         Booking
// @Produce      json
// @Security     BearerAuth
// @Param        date  query  string  false  "Filter tanggal (YYYY-MM-DD)"
// @Success      200   {object}  response.Response{data=[]entity.Booking}
// @Router       /api/v1/bookings [get]
func (h *RentalHandler) GetAllBookings(c *fiber.Ctx) error {
	var list []entity.Booking
	query := h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").
		Order("booking_date ASC, start_hour ASC")
	if date := c.Query("date", ""); date != "" {
		query = query.Where("booking_date = ?", date)
	}
	query.Find(&list)
	if list == nil {
		list = []entity.Booking{}
	}
	return response.OK(c, "Data booking", list)
}

// UpdateBookingStatus godoc
// @Summary      Update status booking (confirm/cancel/complete)
// @Tags         Booking
// @Produce      json
// @Security     BearerAuth
// @Param        id      path  string  true  "Booking ID"
// @Param        status  query  string  true  "Status baru: confirmed/cancelled/completed"
// @Success      200  {object}  response.Response
// @Router       /api/v1/bookings/{id}/status [patch]
func (h *RentalHandler) UpdateBookingStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}
	status := c.Query("status", "")
	if status == "" {
		return response.BadRequest(c, "Status tidak boleh kosong")
	}
	validStatuses := map[string]bool{"confirmed": true, "cancelled": true, "completed": true}
	if !validStatuses[status] {
		return response.BadRequest(c, "Status tidak valid: gunakan confirmed/cancelled/completed")
	}

	tx := h.db.WithContext(c.Context()).Model(&entity.Booking{}).
		Where("id = ?", id).Update("status", status)
	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	var booking entity.Booking
	h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").First(&booking, "id = ?", id)
	return response.OK(c, "Status booking diupdate", booking)
}
