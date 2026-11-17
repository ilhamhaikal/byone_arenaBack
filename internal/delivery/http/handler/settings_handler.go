package handler

import (
	"fmt"

	"byone-arena/pkg/audit"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// SettingsHandler menangani pengaturan global aplikasi
type SettingsHandler struct {
	db        *gorm.DB
	validator *validator.Validator
}

func NewSettingsHandler(db *gorm.DB, v *validator.Validator) *SettingsHandler {
	return &SettingsHandler{db: db, validator: v}
}

// GetMembershipPrice godoc
// @Summary      Ambil harga membership (PUBLIK)
// @Description  Mengembalikan harga membership yang dikonfigurasi. Frontend konsume ini, tidak perlu hardcode harga.
// @Tags         Pengaturan
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/v1/settings/membership [get]
func (h *SettingsHandler) GetMembershipPrice(c *fiber.Ctx) error {
	var row struct {
		Value string `gorm:"column:value"`
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "membership_price",
	).Scan(&row)

	return response.OK(c, "Harga membership", fiber.Map{
		"membershipPrice": row.Value,
	})
}

// UpdateMembershipPrice godoc
// @Summary      Update harga membership (admin)
// @Description  Admin mengatur harga membership. Harga ini akan digunakan saat menjual membership ke customer.
// @Tags         Pengaturan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object  true  "{\"membershipPrice\": 50000}"
// @Success      200  {object}  response.Response
// @Router       /api/v1/settings/membership [put]
func (h *SettingsHandler) UpdateMembershipPrice(c *fiber.Ctx) error {
	var req struct {
		MembershipPrice float64 `json:"membershipPrice" validate:"gte=0"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(&req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}

	// Nilai di-cast ke float lalu diformat numerik murni sebelum masuk SQL;
	// tidak ada string dari klien yang disisipkan apa adanya.
	price := fmt.Sprintf("%.0f", req.MembershipPrice)
	h.db.WithContext(c.Context()).Exec(
		"SELECT "+spname.Ident("SetAppSetting")+"(?::text, ?::text)", "membership_price", price,
	)

	audit.Log(h.db, c, "settings.membership_price", "setting", nil, map[string]any{
		"membershipPrice": req.MembershipPrice,
	})

	return response.OK(c, "Harga membership diupdate", fiber.Map{
		"membershipPrice": req.MembershipPrice,
	})
}

// GetDailyPrice godoc
// @Summary      Ambil harga sewa harian (PUBLIK)
// @Description  Mengembalikan harga sewa harian default yang dikonfigurasi.
// @Tags         Pengaturan
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/v1/settings/daily-price [get]
func (h *SettingsHandler) GetDailyPrice(c *fiber.Ctx) error {
	var row struct {
		Value string `gorm:"column:value"`
	}
	h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("AppSettingValue")+"(?::text)", "daily_price",
	).Scan(&row)
	return response.OK(c, "Harga sewa harian", fiber.Map{"dailyPrice": row.Value})
}

// UpdateDailyPrice godoc
// @Summary      Update harga sewa harian (admin)
// @Description  Admin mengatur harga sewa harian default.
// @Tags         Pengaturan
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object  true  "{\"dailyPrice\": 50000}"
// @Success      200  {object}  response.Response
// @Router       /api/v1/settings/daily-price [put]
func (h *SettingsHandler) UpdateDailyPrice(c *fiber.Ctx) error {
	var req struct {
		DailyPrice float64 `json:"dailyPrice" validate:"gte=0"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(&req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	h.db.WithContext(c.Context()).Exec(
		"SELECT "+spname.Ident("SetAppSetting")+"(?::text, ?::text)",
		"daily_price", fmt.Sprintf("%.0f", req.DailyPrice),
	)
	return response.OK(c, "Harga sewa harian diupdate", fiber.Map{"dailyPrice": req.DailyPrice})
}
