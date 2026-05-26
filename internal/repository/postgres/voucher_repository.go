package postgres

import (
	"context"
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type voucherRepository struct {
	db *gorm.DB
}

// NewVoucherRepository membuat instance baru VoucherRepository
func NewVoucherRepository(db *gorm.DB) repository.VoucherRepository {
	return &voucherRepository{db: db}
}

func (r *voucherRepository) FindAll(ctx context.Context) ([]*entity.Voucher, error) {
	var vouchers []*entity.Voucher
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&vouchers).Error; err != nil {
		return nil, err
	}
	return vouchers, nil
}

func (r *voucherRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Voucher, error) {
	var voucher entity.Voucher
	if err := r.db.WithContext(ctx).First(&voucher, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &voucher, nil
}

func (r *voucherRepository) FindByCode(ctx context.Context, code string) (*entity.Voucher, error) {
	var voucher entity.Voucher
	if err := r.db.WithContext(ctx).
		First(&voucher, "code = ?", strings.ToUpper(code)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &voucher, nil
}

func (r *voucherRepository) Create(ctx context.Context, voucher *entity.Voucher) error {
	voucher.Code = strings.ToUpper(voucher.Code)
	return r.db.WithContext(ctx).Create(voucher).Error
}

func (r *voucherRepository) Update(ctx context.Context, voucher *entity.Voucher) error {
	voucher.Code = strings.ToUpper(voucher.Code)
	return r.db.WithContext(ctx).Save(voucher).Error
}

func (r *voucherRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Voucher{}, "id = ?", id).Error
}
