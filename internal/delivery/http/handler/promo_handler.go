package handler

import (
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/promo"
	"byone-arena/pkg/audit"
	"byone-arena/pkg/response"
	"byone-arena/pkg/spname"
	"byone-arena/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PromoHandler mengelola aturan bonus waktu gratis dan penerapannya ke sesi.
type PromoHandler struct {
	db        *gorm.DB
	validator *validator.Validator
}

func NewPromoHandler(db *gorm.DB, v *validator.Validator) *PromoHandler {
	return &PromoHandler{db: db, validator: v}
}

type promoRuleRequest struct {
	Name           string  `json:"name"           validate:"required,min=2,max=100"`
	DaysOfWeek     string  `json:"daysOfWeek"     validate:"required"`
	MinPaidMinutes int     `json:"minPaidMinutes" validate:"required,min=1"`
	BonusMinutes   int     `json:"bonusMinutes"   validate:"required,min=1"`
	MaxPerSession  int     `json:"maxPerSession"  validate:"min=0"`
	ValidFrom      *string `json:"validFrom"`
	ValidTo        *string `json:"validTo"`
	IsActive       *bool   `json:"isActive"`
}

func parseDate(v *string) (*time.Time, error) {
	if v == nil || *v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *v)
	if err != nil {
		return nil, errors.New("format tanggal harus YYYY-MM-DD")
	}
	return &t, nil
}

// List godoc
// @Summary      Daftar aturan bonus waktu
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/promo-bonuses [get]
func (h *PromoHandler) List(c *fiber.Ctx) error {
	var rules []entity.PromoBonusRule
	if err := h.db.WithContext(c.Context()).Order("created_at DESC").Find(&rules).Error; err != nil {
		return response.InternalServerError(c, "Gagal mengambil aturan promo")
	}
	return response.OK(c, "Daftar aturan promo bonus", rules)
}

// Active godoc
// @Summary      Aturan bonus yang berlaku hari ini
// @Description  Dipakai aplikasi untuk menampilkan opsi bonus saat memulai sesi.
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/promo-bonuses/active [get]
func (h *PromoHandler) Active(c *fiber.Ctx) error {
	rule, err := promo.ActiveRuleToday(h.db.WithContext(c.Context()))
	if err != nil {
		return response.OK(c, "Tidak ada promo bonus hari ini", nil)
	}
	return response.OK(c, "Promo bonus hari ini", rule)
}

// Create godoc
// @Summary      Buat aturan bonus waktu
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/promo-bonuses [post]
func (h *PromoHandler) Create(c *fiber.Ctx) error {
	req := new(promoRuleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	from, err := parseDate(req.ValidFrom)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	to, err := parseDate(req.ValidTo)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	rule := entity.PromoBonusRule{
		Name:           req.Name,
		DaysOfWeek:     req.DaysOfWeek,
		MinPaidMinutes: req.MinPaidMinutes,
		BonusMinutes:   req.BonusMinutes,
		MaxPerSession:  req.MaxPerSession,
		ValidFrom:      from,
		ValidTo:        to,
		IsActive:       active,
	}
	if err := h.db.WithContext(c.Context()).Create(&rule).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	audit.Log(h.db, c, "promo.create", "promo_bonus_rule", nil, map[string]any{
		"name": rule.Name, "daysOfWeek": rule.DaysOfWeek,
		"minPaidMinutes": rule.MinPaidMinutes, "bonusMinutes": rule.BonusMinutes,
	})
	return response.Created(c, "Aturan promo dibuat", rule)
}

// Update godoc
// @Summary      Ubah aturan bonus waktu
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/promo-bonuses/{id} [put]
func (h *PromoHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID tidak valid")
	}
	req := new(promoRuleRequest)
	if err := c.BodyParser(req); err != nil {
		return response.BadRequest(c, "Format request tidak valid")
	}
	if err := h.validator.ValidateStruct(req); err != nil {
		return response.BadRequest(c, validator.FormatError(err))
	}
	from, err := parseDate(req.ValidFrom)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	to, err := parseDate(req.ValidTo)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	updates := map[string]any{
		"name":             req.Name,
		"days_of_week":     req.DaysOfWeek,
		"min_paid_minutes": req.MinPaidMinutes,
		"bonus_minutes":    req.BonusMinutes,
		"max_per_session":  req.MaxPerSession,
		"valid_from":       from,
		"valid_to":         to,
		"updated_at":       promo.Now(),
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	res := h.db.WithContext(c.Context()).Model(&entity.PromoBonusRule{}).
		Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return response.InternalServerError(c, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return response.NotFound(c, "Aturan promo tidak ditemukan")
	}

	var rule entity.PromoBonusRule
	h.db.WithContext(c.Context()).First(&rule, "id = ?", id)
	audit.Log(h.db, c, "promo.update", "promo_bonus_rule", &id, map[string]any{
		"name": req.Name, "daysOfWeek": req.DaysOfWeek,
		"minPaidMinutes": req.MinPaidMinutes, "bonusMinutes": req.BonusMinutes,
	})
	return response.OK(c, "Aturan promo diperbarui", rule)
}

// Delete godoc
// @Summary      Hapus aturan bonus waktu
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/promo-bonuses/{id} [delete]
func (h *PromoHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID tidak valid")
	}
	if err := h.db.WithContext(c.Context()).
		Delete(&entity.PromoBonusRule{}, "id = ?", id).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	audit.Log(h.db, c, "promo.delete", "promo_bonus_rule", &id, nil)
	return response.OK(c, "Aturan promo dihapus", nil)
}

// ApplyToSession godoc
// @Summary      Pakai bonus waktu pada sesi aktif
// @Description  Menambah menit bonus ke sesi tanpa menambah tagihan. Hanya
//
//	sekali per sesi dan hanya bila syarat promo terpenuhi.
//
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/sessions/{id}/promo-bonus [post]
func (h *PromoHandler) ApplyToSession(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID sesi tidak valid")
	}

	session, rule, err := promo.Apply(h.db.WithContext(c.Context()), id.String())
	if err != nil {
		switch {
		case errors.Is(err, promo.ErrLimitReached):
			return response.Conflict(c, err.Error())
		case errors.Is(err, promo.ErrNotEligible), errors.Is(err, promo.ErrNoActiveRule):
			return response.BadRequest(c, err.Error())
		case errors.Is(err, promo.ErrNotActive):
			return response.BadRequest(c, err.Error())
		case errors.Is(err, gorm.ErrRecordNotFound):
			return response.NotFound(c, "Sesi tidak ditemukan")
		default:
			return response.BadRequest(c, err.Error())
		}
	}

	audit.Log(h.db, c, "session.promo_bonus", "session", &id, map[string]any{
		"ruleName":          rule.Name,
		"bonusMinutes":      rule.BonusMinutes,
		"paidMinutes":       promo.PaidMinutes(session),
		"totalBonusMinutes": session.PromoBonusMinutes,
		"applyCount":        session.PromoApplyCount,
	})

	return response.OK(c, "Bonus waktu berhasil diterapkan", session)
}

// PromoReport godoc
// @Summary      Laporan bonus waktu gratis
// @Description  Rekap menit gratis yang diberikan per hari dalam rentang tanggal.
// @Tags         Promo Bonus
// @Security     BearerAuth
// @Router       /api/v1/reports/promo-bonus [get]
func (h *PromoHandler) PromoReport(c *fiber.Ctx) error {
	start := c.Query("startDate", promo.Now().AddDate(0, 0, -7).Format("2006-01-02"))
	end := c.Query("endDate", promo.Now().Format("2006-01-02"))

	type row struct {
		Date              string `json:"date"`
		SessionsWithBonus int    `json:"sessionsWithBonus"`
		TotalBonusMinutes int    `json:"totalBonusMinutes"`
	}
	var rows []row
	err := h.db.WithContext(c.Context()).Raw(
		"SELECT * FROM "+spname.Ident("PromoBonusReport")+"(?::DATE, ?::DATE)", start, end,
	).Scan(&rows).Error
	if err != nil {
		return response.InternalServerError(c, "Gagal mengambil laporan bonus")
	}

	totalSessions := 0
	totalMinutes := 0
	for _, r := range rows {
		totalSessions += r.SessionsWithBonus
		totalMinutes += r.TotalBonusMinutes
	}

	return response.OK(c, "Laporan bonus waktu", fiber.Map{
		"startDate":         start,
		"endDate":           end,
		"sessionsWithBonus": totalSessions,
		"totalBonusMinutes": totalMinutes,
		"daily":             rows,
	})
}
