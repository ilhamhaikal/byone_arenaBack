package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// CustomerRepository mendefinisikan kontrak akses data untuk entitas Customer
type CustomerRepository interface {
	FindAll(ctx context.Context) ([]*entity.Customer, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Customer, error)
	FindByPhone(ctx context.Context, phone string) (*entity.Customer, error)
	Create(ctx context.Context, customer *entity.Customer) error
	Update(ctx context.Context, customer *entity.Customer) error
	Delete(ctx context.Context, id uuid.UUID) error
}
