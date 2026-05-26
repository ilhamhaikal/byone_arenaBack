package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// ShiftRepository mendefinisikan kontrak akses data untuk entitas Shift
type ShiftRepository interface {
	FindAll(ctx context.Context) ([]*entity.Shift, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Shift, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Shift, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Shift, error)
	Create(ctx context.Context, shift *entity.Shift) error
	Update(ctx context.Context, shift *entity.Shift) error
	Delete(ctx context.Context, id uuid.UUID) error
}
