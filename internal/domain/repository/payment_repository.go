package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// PaymentRepository mendefinisikan kontrak akses data untuk entitas Payment
type PaymentRepository interface {
	FindAll(ctx context.Context) ([]*entity.Payment, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)
	FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*entity.Payment, error)
	Create(ctx context.Context, payment *entity.Payment) error
	Update(ctx context.Context, payment *entity.Payment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.PaymentStatus) error
}
