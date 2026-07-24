package handler

import (
	"byone-arena/internal/domain/entity"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RentalHandler menangani daily rental + booking
type RentalHandler struct {
	db        *gorm.DB
	validator *validator.Validator
}

func NewRentalHandler(db *gorm.DB, v *validator.Validator) *RentalHandler {
	return &RentalHandler{db: db, validator: v}
}

// ============== DAILY RENTAL ==============

// CreateDailyRental godoc
// @Summary      Buat rental harian (console dibawa pulang)
// @Tags         Rental Harian
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object  true  "Data rental: consoleId, customerId, startDate, endDate, dailyPrice, voucherCode, notes"
// @Success      201  {object}  response.Response
// @Router       /api/v1/daily-rentals [post]
func (h *RentalHandler) CreateDailyRental(c *fiber.Ctx) error {
	var req struct {
		ConsoleID     uuid.UUID  `json:"consoleId"     validate:"required"`
		CustomerID    *uuid.UUID `json:"customerId"`
		StartDate     string     `json:"startDate"     validate:"required"`
		EndDate       string     `json:"endDate"       validate:"required"`
		DailyPrice    float64    `json:"dailyPrice"`    // opsional, auto dari settings jika 0
		VoucherCode   string     `json:"voucherCode"`
		Notes         string     `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(&req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	// Auto-fill dailyPrice dari console jika tidak diisi
	if req.DailyPrice == 0 {
		var console struct{ DailyPrice float64 `gorm:"column:daily_price"` }
		h.db.WithContext(c.Context()).Raw(
			"SELECT daily_price FROM consoles WHERE id = ?", req.ConsoleID,
		).Scan(&console)
		if console.DailyPrice > 0 {
			req.DailyPrice = console.DailyPrice
		}
	}
	if req.DailyPrice <= 0 {
		return response.BadRequest(c, "Harga per hari belum diatur. Admin: PUT /settings/daily-price")
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
	tx := h.db.WithContext(c.Context()).Raw(
		fmt.Sprintf(`SELECT * FROM %s(?, ?, ?::DATE, ?::DATE, ?, ?, ?)`, spname.Ident("CreateDailyRental")),
		req.ConsoleID, req.CustomerID, req.StartDate, req.EndDate,
		req.DailyPrice, req.VoucherCode, req.Notes,
	).Scan(&result)
	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	// Load full object
	var rental entity.DailyRental
	h.db.WithContext(c.Context()).Preload("Console").Preload("Customer").
		First(&rental, "id = ?", result.RentalID)

	return response.Created(c, "Rental harian berhasil dibuat", rental)
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
		ConsoleID       uuid.UUID `json:"consoleId"       validate:"required"`
		CustomerID      *uuid.UUID `json:"customerId"      validate:"omitempty"`
		BookingDate     string    `json:"bookingDate"     validate:"required"`
		StartHour       int       `json:"startHour"       validate:"required,min=0,max=23"`
		StartMinute     int       `json:"startMinute"     validate:"min=0,max=59"`
		DurationMinutes int       `json:"durationMinutes" validate:"required,min=30"`
		Notes           string    `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(&req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
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
