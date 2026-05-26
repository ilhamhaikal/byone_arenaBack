package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// DiscountRuleUseCase mendefinisikan logika bisnis untuk manajemen aturan diskon otomatis
type DiscountRuleUseCase interface {
	GetAllRules(ctx context.Context) ([]*entity.DiscountRule, error)
	GetRuleByID(ctx context.Context, id uuid.UUID) (*entity.DiscountRule, error)
	GetActiveRules(ctx context.Context) ([]*entity.DiscountRule, error)
	CreateRule(ctx context.Context, req *CreateDiscountRuleRequest) (*entity.DiscountRule, error)
	UpdateRule(ctx context.Context, id uuid.UUID, req *UpdateDiscountRuleRequest) (*entity.DiscountRule, error)
	DeleteRule(ctx context.Context, id uuid.UUID) error
	ToggleRule(ctx context.Context, id uuid.UUID) (*entity.DiscountRule, error)
}

type discountRuleUseCase struct {
	ruleRepo repository.DiscountRuleRepository
}

// NewDiscountRuleUseCase membuat instance baru DiscountRuleUseCase
func NewDiscountRuleUseCase(ruleRepo repository.DiscountRuleRepository) DiscountRuleUseCase {
	return &discountRuleUseCase{ruleRepo: ruleRepo}
}

// CreateDiscountRuleRequest payload untuk membuat aturan diskon baru
type CreateDiscountRuleRequest struct {
	Name          string               `json:"name"          validate:"required,min=3,max=150"`
	RuleType      entity.RuleType      `json:"ruleType"      validate:"required,oneof=always happy_hour member day_of_week"`
	DiscountType  entity.DiscountType  `json:"discountType"  validate:"required,oneof=percentage fixed_amount"`
	DiscountValue float64              `json:"discountValue" validate:"required,gt=0"`
	MaxDiscount   float64              `json:"maxDiscount"   validate:"gte=0"`
	MinPurchase   float64              `json:"minPurchase"   validate:"gte=0"`
	StartHour     *int                 `json:"startHour"` // wajib jika rule_type=happy_hour (0-23)
	EndHour       *int                 `json:"endHour"`   // wajib jika rule_type=happy_hour (0-23)
	DaysOfWeek    string               `json:"daysOfWeek"` // wajib jika rule_type=day_of_week, format "0,1,2"
	Priority      int                  `json:"priority"   validate:"gte=0"`
}

// UpdateDiscountRuleRequest payload untuk mengubah aturan diskon
type UpdateDiscountRuleRequest struct {
	Name          string               `json:"name"          validate:"omitempty,min=3,max=150"`
	RuleType      entity.RuleType      `json:"ruleType"      validate:"omitempty,oneof=always happy_hour member day_of_week"`
	DiscountType  entity.DiscountType  `json:"discountType"  validate:"omitempty,oneof=percentage fixed_amount"`
	DiscountValue float64              `json:"discountValue" validate:"omitempty,gt=0"`
	MaxDiscount   *float64             `json:"maxDiscount"   validate:"omitempty,gte=0"`
	MinPurchase   *float64             `json:"minPurchase"   validate:"omitempty,gte=0"`
	StartHour     *int                 `json:"startHour"`
	EndHour       *int                 `json:"endHour"`
	DaysOfWeek    *string              `json:"daysOfWeek"`
	Priority      *int                 `json:"priority"      validate:"omitempty,gte=0"`
	IsActive      *bool                `json:"isActive"`
}

func (uc *discountRuleUseCase) GetAllRules(ctx context.Context) ([]*entity.DiscountRule, error) {
	return uc.ruleRepo.FindAll(ctx)
}

func (uc *discountRuleUseCase) GetRuleByID(ctx context.Context, id uuid.UUID) (*entity.DiscountRule, error) {
	r, err := uc.ruleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("aturan diskon tidak ditemukan")
	}
	return r, nil
}

func (uc *discountRuleUseCase) GetActiveRules(ctx context.Context) ([]*entity.DiscountRule, error) {
	return uc.ruleRepo.FindActive(ctx)
}

func (uc *discountRuleUseCase) CreateRule(ctx context.Context, req *CreateDiscountRuleRequest) (*entity.DiscountRule, error) {
	if req.DiscountType == entity.DiscountTypePercentage && req.DiscountValue > 100 {
		return nil, errors.New("diskon persentase tidak boleh melebihi 100%")
	}

	if err := validateRuleTypeFields(req.RuleType, req.StartHour, req.EndHour, req.DaysOfWeek); err != nil {
		return nil, err
	}

	rule := &entity.DiscountRule{
		Name:          req.Name,
		RuleType:      req.RuleType,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		MaxDiscount:   req.MaxDiscount,
		MinPurchase:   req.MinPurchase,
		StartHour:     req.StartHour,
		EndHour:       req.EndHour,
		DaysOfWeek:    normalizeDaysOfWeek(req.DaysOfWeek),
		Priority:      req.Priority,
		IsActive:      true,
	}

	if err := uc.ruleRepo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (uc *discountRuleUseCase) UpdateRule(ctx context.Context, id uuid.UUID, req *UpdateDiscountRuleRequest) (*entity.DiscountRule, error) {
	rule, err := uc.ruleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, errors.New("aturan diskon tidak ditemukan")
	}

	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.RuleType != "" {
		rule.RuleType = req.RuleType
	}
	if req.DiscountType != "" {
		rule.DiscountType = req.DiscountType
	}
	if req.DiscountValue > 0 {
		if rule.DiscountType == entity.DiscountTypePercentage && req.DiscountValue > 100 {
			return nil, errors.New("diskon persentase tidak boleh melebihi 100%")
		}
		rule.DiscountValue = req.DiscountValue
	}
	if req.MaxDiscount != nil {
		rule.MaxDiscount = *req.MaxDiscount
	}
	if req.MinPurchase != nil {
		rule.MinPurchase = *req.MinPurchase
	}
	if req.StartHour != nil {
		rule.StartHour = req.StartHour
	}
	if req.EndHour != nil {
		rule.EndHour = req.EndHour
	}
	if req.DaysOfWeek != nil {
		rule.DaysOfWeek = normalizeDaysOfWeek(*req.DaysOfWeek)
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}

	if err := validateRuleTypeFields(rule.RuleType, rule.StartHour, rule.EndHour, rule.DaysOfWeek); err != nil {
		return nil, err
	}

	if err := uc.ruleRepo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (uc *discountRuleUseCase) DeleteRule(ctx context.Context, id uuid.UUID) error {
	rule, err := uc.ruleRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if rule == nil {
		return errors.New("aturan diskon tidak ditemukan")
	}
	return uc.ruleRepo.Delete(ctx, id)
}

func (uc *discountRuleUseCase) ToggleRule(ctx context.Context, id uuid.UUID) (*entity.DiscountRule, error) {
	rule, err := uc.ruleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, errors.New("aturan diskon tidak ditemukan")
	}
	rule.IsActive = !rule.IsActive
	if err := uc.ruleRepo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// validateRuleTypeFields memvalidasi field tambahan sesuai rule_type
func validateRuleTypeFields(ruleType entity.RuleType, startHour, endHour *int, daysOfWeek string) error {
	switch ruleType {
	case entity.RuleTypeHappyHour:
		if startHour == nil || endHour == nil {
			return errors.New("startHour dan endHour wajib diisi untuk tipe happy_hour")
		}
		if *startHour < 0 || *startHour > 23 || *endHour < 0 || *endHour > 23 {
			return errors.New("startHour dan endHour harus antara 0-23")
		}
	case entity.RuleTypeDayOfWeek:
		if daysOfWeek == "" {
			return errors.New("daysOfWeek wajib diisi untuk tipe day_of_week, contoh: \"1,2,3,4,5\"")
		}
		if err := validateDaysOfWeek(daysOfWeek); err != nil {
			return err
		}
	}
	return nil
}

// validateDaysOfWeek memvalidasi format dan nilai days_of_week
func validateDaysOfWeek(s string) error {
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		d, err := strconv.Atoi(p)
		if err != nil || d < 0 || d > 6 {
			return errors.New("daysOfWeek harus berisi angka 0-6 dipisah koma (0=Minggu, 6=Sabtu)")
		}
	}
	return nil
}

// normalizeDaysOfWeek membersihkan spasi dari string days_of_week
func normalizeDaysOfWeek(s string) string {
	parts := strings.Split(s, ",")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, ",")
}
