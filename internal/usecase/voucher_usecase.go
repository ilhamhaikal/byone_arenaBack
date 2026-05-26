package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// VoucherUseCase mendefinisikan logika bisnis untuk manajemen voucher diskon
type VoucherUseCase interface {
	GetAllVouchers(ctx context.Context) ([]*entity.Voucher, error)
	GetVoucherByID(ctx context.Context, id uuid.UUID) (*entity.Voucher, error)
	GetVoucherByCode(ctx context.Context, code string) (*entity.Voucher, error)
	CreateVoucher(ctx context.Context, req *CreateVoucherRequest) (*entity.Voucher, error)
	UpdateVoucher(ctx context.Context, id uuid.UUID, req *UpdateVoucherRequest) (*entity.Voucher, error)
	DeleteVoucher(ctx context.Context, id uuid.UUID) error
	ToggleVoucher(ctx context.Context, id uuid.UUID) (*entity.Voucher, error)
}

type voucherUseCase struct {
	voucherRepo repository.VoucherRepository
}

// NewVoucherUseCase membuat instance baru VoucherUseCase
func NewVoucherUseCase(voucherRepo repository.VoucherRepository) VoucherUseCase {
	return &voucherUseCase{voucherRepo: voucherRepo}
}

// CreateVoucherRequest payload untuk membuat voucher baru
type CreateVoucherRequest struct {
	Code          string               `json:"code"          validate:"required,min=3,max=50"`
	Name          string               `json:"name"          validate:"required,min=3,max=150"`
	DiscountType  entity.DiscountType  `json:"discountType"  validate:"required,oneof=percentage fixed_amount"`
	DiscountValue float64              `json:"discountValue" validate:"required,gt=0"`
	MinPurchase   float64              `json:"minPurchase"   validate:"gte=0"`
	MaxDiscount   float64              `json:"maxDiscount"   validate:"gte=0"`  // hanya berlaku untuk percentage
	MaxUsage      int                  `json:"maxUsage"      validate:"gte=0"`  // 0 = tidak terbatas
	ExpiresAt     *time.Time           `json:"expiresAt"`
}

// UpdateVoucherRequest payload untuk mengubah data voucher
type UpdateVoucherRequest struct {
	Code          string               `json:"code"          validate:"omitempty,min=3,max=50"`
	Name          string               `json:"name"          validate:"omitempty,min=3,max=150"`
	DiscountType  entity.DiscountType  `json:"discountType"  validate:"omitempty,oneof=percentage fixed_amount"`
	DiscountValue float64              `json:"discountValue" validate:"omitempty,gt=0"`
	MinPurchase   *float64             `json:"minPurchase"   validate:"omitempty,gte=0"`
	MaxDiscount   *float64             `json:"maxDiscount"   validate:"omitempty,gte=0"`
	MaxUsage      *int                 `json:"maxUsage"      validate:"omitempty,gte=0"`
	IsActive      *bool                `json:"isActive"`
	ExpiresAt     *time.Time           `json:"expiresAt"`
}

func (uc *voucherUseCase) GetAllVouchers(ctx context.Context) ([]*entity.Voucher, error) {
	return uc.voucherRepo.FindAll(ctx)
}

func (uc *voucherUseCase) GetVoucherByID(ctx context.Context, id uuid.UUID) (*entity.Voucher, error) {
	v, err := uc.voucherRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, errors.New("voucher tidak ditemukan")
	}
	return v, nil
}

func (uc *voucherUseCase) GetVoucherByCode(ctx context.Context, code string) (*entity.Voucher, error) {
	v, err := uc.voucherRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, errors.New("kode voucher tidak ditemukan")
	}
	return v, nil
}

func (uc *voucherUseCase) CreateVoucher(ctx context.Context, req *CreateVoucherRequest) (*entity.Voucher, error) {
	// Validasi: diskon persentase harus 1-100
	if req.DiscountType == entity.DiscountTypePercentage && req.DiscountValue > 100 {
		return nil, errors.New("diskon persentase tidak boleh melebihi 100%")
	}

	// Cek duplikasi kode
	existing, err := uc.voucherRepo.FindByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("kode voucher sudah digunakan")
	}

	voucher := &entity.Voucher{
		Code:          strings.ToUpper(req.Code),
		Name:          req.Name,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		MinPurchase:   req.MinPurchase,
		MaxDiscount:   req.MaxDiscount,
		MaxUsage:      req.MaxUsage,
		IsActive:      true,
		ExpiresAt:     req.ExpiresAt,
	}

	if err := uc.voucherRepo.Create(ctx, voucher); err != nil {
		return nil, err
	}
	return voucher, nil
}

func (uc *voucherUseCase) UpdateVoucher(ctx context.Context, id uuid.UUID, req *UpdateVoucherRequest) (*entity.Voucher, error) {
	voucher, err := uc.GetVoucherByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Code != "" {
		newCode := strings.ToUpper(req.Code)
		if newCode != voucher.Code {
			existing, err := uc.voucherRepo.FindByCode(ctx, newCode)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				return nil, errors.New("kode voucher sudah digunakan")
			}
		}
		voucher.Code = newCode
	}
	if req.Name != "" {
		voucher.Name = req.Name
	}
	if req.DiscountType != "" {
		voucher.DiscountType = req.DiscountType
	}
	if req.DiscountValue > 0 {
		if voucher.DiscountType == entity.DiscountTypePercentage && req.DiscountValue > 100 {
			return nil, errors.New("diskon persentase tidak boleh melebihi 100%")
		}
		voucher.DiscountValue = req.DiscountValue
	}
	if req.MinPurchase != nil {
		voucher.MinPurchase = *req.MinPurchase
	}
	if req.MaxDiscount != nil {
		voucher.MaxDiscount = *req.MaxDiscount
	}
	if req.MaxUsage != nil {
		voucher.MaxUsage = *req.MaxUsage
	}
	if req.IsActive != nil {
		voucher.IsActive = *req.IsActive
	}
	if req.ExpiresAt != nil {
		voucher.ExpiresAt = req.ExpiresAt
	}

	if err := uc.voucherRepo.Update(ctx, voucher); err != nil {
		return nil, err
	}
	return voucher, nil
}

func (uc *voucherUseCase) DeleteVoucher(ctx context.Context, id uuid.UUID) error {
	if _, err := uc.GetVoucherByID(ctx, id); err != nil {
		return err
	}
	return uc.voucherRepo.Delete(ctx, id)
}

func (uc *voucherUseCase) ToggleVoucher(ctx context.Context, id uuid.UUID) (*entity.Voucher, error) {
	voucher, err := uc.GetVoucherByID(ctx, id)
	if err != nil {
		return nil, err
	}
	voucher.IsActive = !voucher.IsActive
	if err := uc.voucherRepo.Update(ctx, voucher); err != nil {
		return nil, err
	}
	return voucher, nil
}
