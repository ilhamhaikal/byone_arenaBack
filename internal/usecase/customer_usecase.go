package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// CustomerUseCase mendefinisikan logika bisnis untuk manajemen pelanggan
type CustomerUseCase interface {
	GetAllCustomers(ctx context.Context) ([]*entity.Customer, error)
	GetCustomerByID(ctx context.Context, id uuid.UUID) (*entity.Customer, error)
	GetCustomerByPhone(ctx context.Context, phone string) (*entity.Customer, error)
	CreateCustomer(ctx context.Context, req *CreateCustomerRequest) (*entity.Customer, error)
	UpdateCustomer(ctx context.Context, id uuid.UUID, req *UpdateCustomerRequest) (*entity.Customer, error)
	DeleteCustomer(ctx context.Context, id uuid.UUID) error
}

type customerUseCase struct {
	customerRepo repository.CustomerRepository
}

// NewCustomerUseCase membuat instance baru CustomerUseCase
func NewCustomerUseCase(customerRepo repository.CustomerRepository) CustomerUseCase {
	return &customerUseCase{customerRepo: customerRepo}
}

// CreateCustomerRequest payload untuk mendaftarkan pelanggan baru
type CreateCustomerRequest struct {
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Phone string `json:"phone" validate:"required,min=8,max=20"`
	Email string `json:"email" validate:"omitempty,email"`
	IsMember        bool    `json:"isMember"`
	MembershipType  *string `json:"membershipType"`
	MembershipStart *string `json:"membershipStart"`
	MembershipExpiry *string `json:"membershipExpiry"`
	MembershipPrice float64 `json:"membershipPrice"`
}

// UpdateCustomerRequest payload untuk memperbarui data pelanggan
type UpdateCustomerRequest struct {
	Name  string `json:"name" validate:"omitempty,min=2,max=100"`
	Phone string `json:"phone" validate:"omitempty,min=8,max=20"`
	Email string `json:"email" validate:"omitempty,email"`
	IsMember        *bool    `json:"isMember"`
	MembershipType  *string  `json:"membershipType"`
	MembershipStart *string  `json:"membershipStart"`
	MembershipExpiry *string `json:"membershipExpiry"`
	MembershipPrice *float64 `json:"membershipPrice"`
}

func (uc *customerUseCase) GetAllCustomers(ctx context.Context) ([]*entity.Customer, error) {
	return uc.customerRepo.FindAll(ctx)
}

func (uc *customerUseCase) GetCustomerByID(ctx context.Context, id uuid.UUID) (*entity.Customer, error) {
	customer, err := uc.customerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, errors.New("pelanggan tidak ditemukan")
	}
	return customer, nil
}

func (uc *customerUseCase) GetCustomerByPhone(ctx context.Context, phone string) (*entity.Customer, error) {
	return uc.customerRepo.FindByPhone(ctx, phone)
}

func (uc *customerUseCase) CreateCustomer(ctx context.Context, req *CreateCustomerRequest) (*entity.Customer, error) {
	// Cek apakah nomor telepon sudah terdaftar
	existing, err := uc.customerRepo.FindByPhone(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("nomor telepon sudah terdaftar")
	}

	now := time.Now()
	customer := &entity.Customer{
		ID:        uuid.New(),
		Name:      req.Name,
		Phone:     req.Phone,
		Email:     req.Email,
		// IsMember selalu false saat create — membership diaktifkan via SP oleh handler
		IsMember:         false,
		MembershipPrice:  0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := uc.customerRepo.Create(ctx, customer); err != nil {
		return nil, err
	}
	return customer, nil
}

func (uc *customerUseCase) UpdateCustomer(ctx context.Context, id uuid.UUID, req *UpdateCustomerRequest) (*entity.Customer, error) {
	customer, err := uc.customerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, errors.New("pelanggan tidak ditemukan")
	}

	if req.Name != "" {
		customer.Name = req.Name
	}
	if req.Phone != "" {
		customer.Phone = req.Phone
	}
	if req.Email != "" {
		customer.Email = req.Email
	}
	if req.IsMember != nil {
		customer.IsMember = *req.IsMember
	}
	if req.MembershipType != nil {
		customer.MembershipType = req.MembershipType
	}
	if req.MembershipStart != nil {
		customer.MembershipStart = req.MembershipStart
	}
	if req.MembershipExpiry != nil {
		customer.MembershipExpiry = req.MembershipExpiry
	}
	if req.MembershipPrice != nil {
		customer.MembershipPrice = *req.MembershipPrice
	}
	customer.UpdatedAt = time.Now()

	if err := uc.customerRepo.Update(ctx, customer); err != nil {
		return nil, err
	}
	return customer, nil
}

func (uc *customerUseCase) DeleteCustomer(ctx context.Context, id uuid.UUID) error {
	customer, err := uc.customerRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if customer == nil {
		return errors.New("pelanggan tidak ditemukan")
	}
	return uc.customerRepo.Delete(ctx, id)
}
