package handler

import (
	"byone-arena/internal/usecase"
	"byone-arena/pkg/response"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConsoleHandler menangani HTTP request untuk manajemen konsol
type ConsoleHandler struct {
	consoleUC usecase.ConsoleUseCase
	validator *validator.Validator
	db        *gorm.DB
}

// NewConsoleHandler membuat instance baru ConsoleHandler
func NewConsoleHandler(consoleUC usecase.ConsoleUseCase, v *validator.Validator, db *gorm.DB) *ConsoleHandler {
	return &ConsoleHandler{consoleUC: consoleUC, validator: v, db: db}
}

// GetAll godoc
// @Summary      Ambil semua konsol
// @Description  Mengembalikan daftar seluruh unit konsol / TV Android
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Console}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/consoles [get]
func (h *ConsoleHandler) GetAll(c *fiber.Ctx) error {
	consoles, err := h.consoleUC.GetAllConsoles(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data konsol")
	}
	return response.OK(c, "Data konsol berhasil diambil", consoles)
}

// GetAvailable godoc
// @Summary      Ambil konsol yang tersedia
// @Description  Mengembalikan daftar konsol dengan status available
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=[]entity.Console}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/available [get]
func (h *ConsoleHandler) GetAvailable(c *fiber.Ctx) error {
	consoles, err := h.consoleUC.GetAvailableConsoles(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil data konsol tersedia")
	}
	return response.OK(c, "Konsol tersedia berhasil diambil", consoles)
}

// GetByID godoc
// @Summary      Ambil konsol berdasarkan ID
// @Description  Mengembalikan detail satu unit konsol
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID (UUID)"
// @Success      200  {object}  response.Response{data=entity.Console}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id} [get]
func (h *ConsoleHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	console, err := h.consoleUC.GetConsoleByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.OK(c, "Data konsol berhasil diambil", console)
}

// Create godoc
// @Summary      Tambah konsol baru
// @Description  Menambahkan unit konsol PlayStation baru ke sistem
// @Tags         Konsol
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      usecase.CreateConsoleRequest  true  "Data konsol baru"
// @Success      201   {object}  response.Response{data=entity.Console}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      500   {object}  response.ErrorResponse
// @Router       /api/v1/consoles [post]
func (h *ConsoleHandler) Create(c *fiber.Ctx) error {
	req := new(usecase.CreateConsoleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	console, err := h.consoleUC.CreateConsole(c.Context(), req)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "Konsol berhasil ditambahkan", console)
}

// Update godoc
// @Summary      Update data konsol
// @Description  Memperbarui informasi unit konsol (nama, tipe, harga, status, deskripsi)
// @Tags         Konsol
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string                        true  "Console ID (UUID)"
// @Param        body  body      usecase.UpdateConsoleRequest  true  "Data konsol yang diperbarui"
// @Success      200   {object}  response.Response{data=entity.Console}
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id} [put]
func (h *ConsoleHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	req := new(usecase.UpdateConsoleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	console, err := h.consoleUC.UpdateConsole(c.Context(), id, req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Data konsol berhasil diperbarui", console)
}

// Delete godoc
// @Summary      Hapus konsol
// @Description  Menghapus unit konsol dari sistem secara permanen
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Console ID (UUID)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id} [delete]
func (h *ConsoleHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	if err := h.consoleUC.DeleteConsole(c.Context(), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, "Konsol berhasil dihapus", nil)
}

// GetOverview godoc
// @Summary      Dashboard overview semua konsol (PUBLIK)
// @Description  Endpoint publik untuk client Android TV — mengembalikan semua konsol beserta sesi aktif masing-masing (jika ada).\n\nTidak memerlukan autentikasi.\n\nSetiap item mencakup:\n- Data konsol (nama, tipe, IP, status, harga/jam)\n- `activeSession`: null jika konsol kosong, atau berisi info sesi aktif termasuk `remainingMinutes` (sisa menit dari durasi yang dipesan; -1 = open-ended).
// @Tags         Konsol
// @Produce      json
// @Success      200  {object}  response.Response{data=[]usecase.ConsoleOverviewItem}
// @Failure      500  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/overview [get]
func (h *ConsoleHandler) GetOverview(c *fiber.Ctx) error {
	items, err := h.consoleUC.GetConsoleOverview(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil overview konsol")
	}
	return response.OK(c, "Overview konsol berhasil diambil", items)
}

// PreviewPrice godoc
// @Summary      Kalkulasi harga sebelum sewa
// @Description  Menghitung estimasi harga untuk durasi tertentu. Termasuk diskon otomatis (happy hour, member) dan validasi voucher opsional.
// @Tags         Konsol
// @Produce      json
// @Security     BearerAuth
// @Param        id           path      string  true   "Console ID"
// @Param        duration     query     int     true   "Durasi (menit), minimal 30"
// @Param        voucherCode  query     string  false  "Kode voucher (opsional)"
// @Param        customerId   query     string  false  "Customer ID (opsional, untuk cek member)"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Router       /api/v1/consoles/{id}/price [get]
func (h *ConsoleHandler) PreviewPrice(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "Format ID tidak valid")
	}

	duration := c.QueryInt("duration", 0)
	if duration < 30 {
		return response.BadRequest(c, "Durasi minimal 30 menit")
	}

	voucherCode := c.Query("voucherCode", "")
	customerID := c.Query("customerId", "")

	type spResult struct {
		PricePerHour   float64 `gorm:"column:price_per_hour"`
		DurationMin    int     `gorm:"column:duration_minutes"`
		BaseAmount     float64 `gorm:"column:base_amount"`
		AutoDiscount   float64 `gorm:"column:auto_discount"`
		VoucherDisc    float64 `gorm:"column:voucher_discount"`
		TotalDiscount  float64 `gorm:"column:total_discount"`
		FinalAmount    float64 `gorm:"column:final_amount"`
		VoucherApplied bool    `gorm:"column:voucher_applied"`
		VoucherName    *string `gorm:"column:voucher_name"`
		Message        string  `gorm:"column:message"`
	}

	var result spResult
	tx := h.db.WithContext(c.Context()).Raw(
		`SELECT * FROM "byonePreviewPrice"(?, ?, ?, NULLIF(?, '')::UUID)`,
		id, duration, voucherCode, customerID,
	).Scan(&result)

	if tx.Error != nil {
		return response.BadRequest(c, tx.Error.Error())
	}

	return response.OK(c, result.Message, fiber.Map{
		"consoleId":       id,
		"durationMinutes": result.DurationMin,
		"pricePerHour":    result.PricePerHour,
		"baseAmount":      result.BaseAmount,
		"autoDiscount":    result.AutoDiscount,
		"voucherDiscount": result.VoucherDisc,
		"totalDiscount":   result.TotalDiscount,
		"finalAmount":     result.FinalAmount,
		"voucherApplied":  result.VoucherApplied,
		"voucherName":     result.VoucherName,
	})
}

