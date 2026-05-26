package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// ConsoleUseCase mendefinisikan logika bisnis untuk manajemen konsol
type ConsoleUseCase interface {
	GetAllConsoles(ctx context.Context) ([]*entity.Console, error)
	GetConsoleByID(ctx context.Context, id uuid.UUID) (*entity.Console, error)
	GetAvailableConsoles(ctx context.Context) ([]*entity.Console, error)
	CreateConsole(ctx context.Context, req *CreateConsoleRequest) (*entity.Console, error)
	UpdateConsole(ctx context.Context, id uuid.UUID, req *UpdateConsoleRequest) (*entity.Console, error)
	DeleteConsole(ctx context.Context, id uuid.UUID) error
}

type consoleUseCase struct {
	consoleRepo repository.ConsoleRepository
}

// NewConsoleUseCase membuat instance baru ConsoleUseCase
func NewConsoleUseCase(consoleRepo repository.ConsoleRepository) ConsoleUseCase {
	return &consoleUseCase{consoleRepo: consoleRepo}
}

// CreateConsoleRequest payload untuk membuat konsol baru
type CreateConsoleRequest struct {
	Name         string              `json:"name"         validate:"required,min=2,max=100"`
	ConsoleType  entity.ConsoleType  `json:"consoleType"  validate:"required,oneof=PS3 PS4 PS5"`
	PricePerHour float64             `json:"pricePerHour" validate:"required,gt=0"`
	Description  string              `json:"description"`
}

// UpdateConsoleRequest payload untuk memperbarui data konsol
type UpdateConsoleRequest struct {
	Name         string               `json:"name"         validate:"omitempty,min=2,max=100"`
	ConsoleType  entity.ConsoleType   `json:"consoleType"  validate:"omitempty,oneof=PS3 PS4 PS5"`
	Status       entity.ConsoleStatus `json:"status"       validate:"omitempty,oneof=available in_use maintenance"`
	PricePerHour float64              `json:"pricePerHour" validate:"omitempty,gt=0"`
	Description  string               `json:"description"`
}

func (uc *consoleUseCase) GetAllConsoles(ctx context.Context) ([]*entity.Console, error) {
	return uc.consoleRepo.FindAll(ctx)
}

func (uc *consoleUseCase) GetConsoleByID(ctx context.Context, id uuid.UUID) (*entity.Console, error) {
	console, err := uc.consoleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if console == nil {
		return nil, errors.New("konsol tidak ditemukan")
	}
	return console, nil
}

func (uc *consoleUseCase) GetAvailableConsoles(ctx context.Context) ([]*entity.Console, error) {
	return uc.consoleRepo.FindByStatus(ctx, entity.ConsoleStatusAvailable)
}

func (uc *consoleUseCase) CreateConsole(ctx context.Context, req *CreateConsoleRequest) (*entity.Console, error) {
	now := time.Now()
	console := &entity.Console{
		ID:           uuid.New(),
		Name:         req.Name,
		ConsoleType:  req.ConsoleType,
		Status:       entity.ConsoleStatusAvailable,
		PricePerHour: req.PricePerHour,
		Description:  req.Description,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := uc.consoleRepo.Create(ctx, console); err != nil {
		return nil, err
	}
	return console, nil
}

func (uc *consoleUseCase) UpdateConsole(ctx context.Context, id uuid.UUID, req *UpdateConsoleRequest) (*entity.Console, error) {
	console, err := uc.consoleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if console == nil {
		return nil, errors.New("konsol tidak ditemukan")
	}

	if req.Name != "" {
		console.Name = req.Name
	}
	if req.ConsoleType != "" {
		console.ConsoleType = req.ConsoleType
	}
	if req.Status != "" {
		console.Status = req.Status
	}
	if req.PricePerHour > 0 {
		console.PricePerHour = req.PricePerHour
	}
	if req.Description != "" {
		console.Description = req.Description
	}
	console.UpdatedAt = time.Now()

	if err := uc.consoleRepo.Update(ctx, console); err != nil {
		return nil, err
	}
	return console, nil
}

func (uc *consoleUseCase) DeleteConsole(ctx context.Context, id uuid.UUID) error {
	console, err := uc.consoleRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if console == nil {
		return errors.New("konsol tidak ditemukan")
	}
	if console.Status == entity.ConsoleStatusInUse {
		return errors.New("tidak dapat menghapus konsol yang sedang digunakan")
	}
	return uc.consoleRepo.Delete(ctx, id)
}
