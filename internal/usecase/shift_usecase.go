package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// ShiftUseCase mendefinisikan logika bisnis untuk manajemen shift kasir
type ShiftUseCase interface {
	GetAllShifts(ctx context.Context) ([]*entity.Shift, error)
	GetShiftsByUser(ctx context.Context, userID uuid.UUID) ([]*entity.Shift, error)
	GetShiftByID(ctx context.Context, id uuid.UUID) (*entity.Shift, error)
	CreateShift(ctx context.Context, req *CreateShiftRequest) (*entity.Shift, error)
	UpdateShift(ctx context.Context, id uuid.UUID, req *UpdateShiftRequest) (*entity.Shift, error)
	DeleteShift(ctx context.Context, id uuid.UUID) error
}

type shiftUseCase struct {
	shiftRepo repository.ShiftRepository
	userRepo  repository.UserRepository
}

// NewShiftUseCase membuat instance baru ShiftUseCase
func NewShiftUseCase(shiftRepo repository.ShiftRepository, userRepo repository.UserRepository) ShiftUseCase {
	return &shiftUseCase{
		shiftRepo: shiftRepo,
		userRepo:  userRepo,
	}
}

// CreateShiftRequest payload untuk membuat shift baru
type CreateShiftRequest struct {
	UserID    uuid.UUID `json:"userId"    validate:"required"`
	Name      string    `json:"name"      validate:"required,min=2,max=100"`
	StartHour int       `json:"startHour" validate:"min=0,max=23"`
	EndHour   int       `json:"endHour"   validate:"min=0,max=23"`
	Is24Hour  bool      `json:"is24Hour"`
}

// UpdateShiftRequest payload untuk mengubah data shift
type UpdateShiftRequest struct {
	Name      string             `json:"name"      validate:"omitempty,min=2,max=100"`
	StartHour *int               `json:"startHour" validate:"omitempty,min=0,max=23"`
	EndHour   *int               `json:"endHour"   validate:"omitempty,min=0,max=23"`
	Is24Hour  *bool              `json:"is24Hour"`
	Status    entity.ShiftStatus `json:"status"    validate:"omitempty,oneof=active inactive"`
}

func (uc *shiftUseCase) GetAllShifts(ctx context.Context) ([]*entity.Shift, error) {
	return uc.shiftRepo.FindAll(ctx)
}

func (uc *shiftUseCase) GetShiftsByUser(ctx context.Context, userID uuid.UUID) ([]*entity.Shift, error) {
	return uc.shiftRepo.FindByUserID(ctx, userID)
}

func (uc *shiftUseCase) GetShiftByID(ctx context.Context, id uuid.UUID) (*entity.Shift, error) {
	shift, err := uc.shiftRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if shift == nil {
		return nil, errors.New("shift tidak ditemukan")
	}
	return shift, nil
}

func (uc *shiftUseCase) CreateShift(ctx context.Context, req *CreateShiftRequest) (*entity.Shift, error) {
	// Validasi user harus ada dan berperan kasir
	user, err := uc.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("pengguna tidak ditemukan")
	}
	if user.Role != entity.UserRoleKasir {
		return nil, errors.New("shift hanya dapat dibuat untuk pengguna dengan role kasir")
	}

	// Jika bukan 24 jam, start dan end hour tidak boleh sama
	if !req.Is24Hour && req.StartHour == req.EndHour {
		return nil, errors.New("jam mulai dan jam selesai tidak boleh sama")
	}

	now := time.Now()
	shift := &entity.Shift{
		ID:        uuid.New(),
		UserID:    req.UserID,
		Name:      req.Name,
		StartHour: req.StartHour,
		EndHour:   req.EndHour,
		Is24Hour:  req.Is24Hour,
		Status:    entity.ShiftStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := uc.shiftRepo.Create(ctx, shift); err != nil {
		return nil, err
	}
	return shift, nil
}

func (uc *shiftUseCase) UpdateShift(ctx context.Context, id uuid.UUID, req *UpdateShiftRequest) (*entity.Shift, error) {
	shift, err := uc.shiftRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if shift == nil {
		return nil, errors.New("shift tidak ditemukan")
	}

	if req.Name != "" {
		shift.Name = req.Name
	}
	if req.StartHour != nil {
		shift.StartHour = *req.StartHour
	}
	if req.EndHour != nil {
		shift.EndHour = *req.EndHour
	}
	if req.Is24Hour != nil {
		shift.Is24Hour = *req.Is24Hour
	}
	if req.Status != "" {
		shift.Status = req.Status
	}
	shift.UpdatedAt = time.Now()

	if !shift.Is24Hour && shift.StartHour == shift.EndHour {
		return nil, errors.New("jam mulai dan jam selesai tidak boleh sama")
	}

	if err := uc.shiftRepo.Update(ctx, shift); err != nil {
		return nil, err
	}
	return shift, nil
}

func (uc *shiftUseCase) DeleteShift(ctx context.Context, id uuid.UUID) error {
	shift, err := uc.shiftRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if shift == nil {
		return errors.New("shift tidak ditemukan")
	}
	return uc.shiftRepo.Delete(ctx, id)
}
