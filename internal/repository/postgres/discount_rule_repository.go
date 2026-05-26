package postgres

import (
	"context"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type discountRuleRepository struct {
	db *gorm.DB
}

// NewDiscountRuleRepository membuat instance baru DiscountRuleRepository
func NewDiscountRuleRepository(db *gorm.DB) repository.DiscountRuleRepository {
	return &discountRuleRepository{db: db}
}

func (r *discountRuleRepository) FindAll(ctx context.Context) ([]*entity.DiscountRule, error) {
	var rules []*entity.DiscountRule
	if err := r.db.WithContext(ctx).Order("priority DESC, created_at ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *discountRuleRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.DiscountRule, error) {
	var rule entity.DiscountRule
	if err := r.db.WithContext(ctx).First(&rule, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

func (r *discountRuleRepository) FindActive(ctx context.Context) ([]*entity.DiscountRule, error) {
	var rules []*entity.DiscountRule
	if err := r.db.WithContext(ctx).
		Where("is_active = TRUE").
		Order("priority DESC, created_at ASC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *discountRuleRepository) Create(ctx context.Context, rule *entity.DiscountRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *discountRuleRepository) Update(ctx context.Context, rule *entity.DiscountRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *discountRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.DiscountRule{}, "id = ?", id).Error
}
