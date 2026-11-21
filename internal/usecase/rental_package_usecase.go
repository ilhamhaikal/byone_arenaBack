package usecase

import (
	"context"
	"errors"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// RentalPackageUseCase — logika bisnis paket rental.
type RentalPackageUseCase interface {
	List(ctx context.Context) ([]*entity.RentalPackage, error)
	Save(ctx context.Context, req *SaveRentalPackageRequest) (*entity.RentalPackage, error)
	Delete(ctx context.Context, id uuid.UUID) error
	StartWithPackage(ctx context.Context, req *StartWithPackageRequest) (*StartSessionResponse, error)
}

// SaveRentalPackageRequest payload simpan paket.
type SaveRentalPackageRequest struct {
	ID          uuid.UUID                `json:"id"`
	Name        string                   `json:"name"      validate:"required,min=2,max=150"`
	StartTime   string                   `json:"startTime" validate:"required"` // "HH:MM"
	EndTime     string                   `json:"endTime"   validate:"required"` // "HH:MM"
	Price       float64                  `json:"price"     validate:"required,gte=0"`
	Description string                   `json:"description"`
	IsActive    *bool                    `json:"isActive"`
	Items       []RentalPackageItemInput `json:"items"`
}

// RentalPackageItemInput payload satu baris item bundling.
type RentalPackageItemInput struct {
	MenuItemID uuid.UUID `json:"menuItemId" validate:"required"`
	Quantity   int       `json:"quantity"   validate:"required,min=1"`
}

// StartWithPackageRequest payload memulai sesi dengan paket.
type StartWithPackageRequest struct {
	ConsoleID    uuid.UUID  `json:"consoleId"    validate:"required"`
	CustomerID   *uuid.UUID `json:"customerId"`
	PackageID    uuid.UUID  `json:"packageId"    validate:"required"`
	CashReceived float64    `json:"cashReceived" validate:"omitempty,min=0"`
	Notes        string     `json:"notes"`
	CreatedBy    *uuid.UUID `json:"-"`
}

type rentalPackageUseCase struct {
	repo        repository.RentalPackageRepository
	sessionRepo repository.SessionRepository
}

// NewRentalPackageUseCase membuat instance baru RentalPackageUseCase
func NewRentalPackageUseCase(repo repository.RentalPackageRepository, sessionRepo repository.SessionRepository) RentalPackageUseCase {
	return &rentalPackageUseCase{repo: repo, sessionRepo: sessionRepo}
}

func (uc *rentalPackageUseCase) List(ctx context.Context) ([]*entity.RentalPackage, error) {
	return uc.repo.List(ctx, false)
}

func (uc *rentalPackageUseCase) Save(ctx context.Context, req *SaveRentalPackageRequest) (*entity.RentalPackage, error) {
	if req.StartTime == "" || req.EndTime == "" {
		return nil, errors.New("jam mulai dan selesai wajib diisi")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	repoItems := make([]repository.RentalPackageItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		repoItems = append(repoItems, repository.RentalPackageItemInput{
			MenuItemID: it.MenuItemID,
			Quantity:   it.Quantity,
		})
	}
	id, err := uc.repo.Save(ctx, &entity.RentalPackage{
		ID:          req.ID,
		Name:        req.Name,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Price:       req.Price,
		Description: req.Description,
		IsActive:    active,
	}, repoItems)
	if err != nil {
		return nil, err
	}
	list, err := uc.repo.List(ctx, true)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, errors.New("paket tersimpan tetapi tidak ditemukan kembali")
}

func (uc *rentalPackageUseCase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *rentalPackageUseCase) StartWithPackage(ctx context.Context, req *StartWithPackageRequest) (*StartSessionResponse, error) {
	session, payment, err := uc.repo.StartSessionWithPackage(
		ctx, req.ConsoleID, req.CustomerID, req.PackageID, req.CashReceived, req.Notes, req.CreatedBy)
	if err != nil {
		return nil, err
	}

	// Waktu persiapan konsol gratis, sama seperti sesi per jam.
	if updated, graceErr := uc.sessionRepo.AddSetupGrace(ctx, session.ID, SetupGraceMinutes); graceErr == nil && updated != nil {
		session = updated
	}
	return &StartSessionResponse{Session: session, Payment: payment}, nil
}
