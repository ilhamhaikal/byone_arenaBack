package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// VoucherRepository mendefinisikan kontrak akses data untuk voucher
type VoucherRepository interface {
	FindAll(ctx context.Context) ([]*entity.Voucher, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Voucher, error)
	FindByCode(ctx context.Context, code string) (*entity.Voucher, error)
	Create(ctx context.Context, voucher *entity.Voucher) error
	Update(ctx context.Context, voucher *entity.Voucher) error
	Delete(ctx context.Context, id uuid.UUID) error
}
