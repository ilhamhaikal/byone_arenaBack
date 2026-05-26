package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// DiscountRuleRepository mendefinisikan kontrak akses data untuk aturan diskon otomatis
type DiscountRuleRepository interface {
	FindAll(ctx context.Context) ([]*entity.DiscountRule, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.DiscountRule, error)
	FindActive(ctx context.Context) ([]*entity.DiscountRule, error)
	Create(ctx context.Context, rule *entity.DiscountRule) error
	Update(ctx context.Context, rule *entity.DiscountRule) error
	Delete(ctx context.Context, id uuid.UUID) error
}
