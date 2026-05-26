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
	GetConsoleOverview(ctx context.Context) ([]*ConsoleOverviewItem, error)
	CreateConsole(ctx context.Context, req *CreateConsoleRequest) (*entity.Console, error)
	UpdateConsole(ctx context.Context, id uuid.UUID, req *UpdateConsoleRequest) (*entity.Console, error)
	DeleteConsole(ctx context.Context, id uuid.UUID) error
}

// ConsoleOverviewItem adalah data konsol lengkap dengan sesi aktif yang sedang berjalan
type ConsoleOverviewItem struct {
	entity.Console
	ActiveSession *ActiveSessionInfo `json:"activeSession"`
}

// ActiveSessionInfo ringkasan sesi aktif untuk tampilan dashboard
type ActiveSessionInfo struct {
	ID                    uuid.UUID  `json:"id"`
	CustomerID            *uuid.UUID `json:"customerId"`
	CustomerName          string     `json:"customerName"`
	StartTime             time.Time  `json:"startTime"`
	BookedDurationMinutes int        `json:"bookedDurationMinutes"`
	EndScheduledAt        *time.Time `json:"endScheduledAt"`
	RemainingMinutes      int        `json:"remainingMinutes"` // -1 = open-ended
	Notes                 string     `json:"notes,omitempty"`
}

type consoleUseCase struct {
	consoleRepo repository.ConsoleRepository
	sessionRepo repository.SessionRepository
}

// NewConsoleUseCase membuat instance baru ConsoleUseCase
func NewConsoleUseCase(consoleRepo repository.ConsoleRepository, sessionRepo repository.SessionRepository) ConsoleUseCase {
	return &consoleUseCase{consoleRepo: consoleRepo, sessionRepo: sessionRepo}
}

// CreateConsoleRequest payload untuk membuat konsol / TV Android baru
type CreateConsoleRequest struct {
	// Nama tampilan konsol, contoh: "TV 01"
	Name         string             `json:"name"         validate:"required,min=2,max=100"                       example:"TV 01"`
	// Tipe konsol: PS3, PS4, PS5, atau AndroidTV
	ConsoleType  entity.ConsoleType `json:"consoleType"  validate:"required,oneof=PS3 PS4 PS5 AndroidTV" enums:"PS3,PS4,PS5,AndroidTV" example:"AndroidTV"`
	// Alamat IP TV Android (wajib untuk AndroidTV)
	IPAddress    *string            `json:"ipAddress"    validate:"omitempty,max=50"                           example:"192.168.1.101"`
	PricePerHour float64            `json:"pricePerHour" validate:"required,gt=0"                              example:"10000"`
	Description  string             `json:"description"                                                        example:"TV 43 inch ruang A"`
}

// UpdateConsoleRequest payload untuk memperbarui data konsol
type UpdateConsoleRequest struct {
	Name         string               `json:"name"         validate:"omitempty,min=2,max=100"                       example:"TV 01"`
	ConsoleType  entity.ConsoleType   `json:"consoleType"  validate:"omitempty,oneof=PS3 PS4 PS5 AndroidTV" enums:"PS3,PS4,PS5,AndroidTV" example:"AndroidTV"`
	IPAddress    *string              `json:"ipAddress"    validate:"omitempty,max=50"                           example:"192.168.1.101"`
	Status       entity.ConsoleStatus `json:"status"       validate:"omitempty,oneof=available in_use maintenance" example:"available"`
	PricePerHour float64              `json:"pricePerHour" validate:"omitempty,gt=0"                              example:"10000"`
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

// GetConsoleOverview mengembalikan semua konsol beserta sesi aktif masing-masing.
// Melakukan 2 query: satu untuk semua konsol, satu untuk semua sesi aktif, lalu digabung di memory.
func (uc *consoleUseCase) GetConsoleOverview(ctx context.Context) ([]*ConsoleOverviewItem, error) {
	consoles, err := uc.consoleRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	activeSessions, err := uc.sessionRepo.FindActiveSession(ctx)
	if err != nil {
		return nil, err
	}

	// Build map consoleID → active session
	activeMap := make(map[uuid.UUID]*entity.Session, len(activeSessions))
	for _, s := range activeSessions {
		activeMap[s.ConsoleID] = s
	}

	items := make([]*ConsoleOverviewItem, 0, len(consoles))
	for _, c := range consoles {
		item := &ConsoleOverviewItem{Console: *c}
		if s, ok := activeMap[c.ID]; ok {
			info := &ActiveSessionInfo{
				ID:                    s.ID,
				CustomerID:            s.CustomerID,
				StartTime:             s.StartTime,
				BookedDurationMinutes: s.BookedDurationMinutes,
				EndScheduledAt:        s.EndScheduledAt,
				RemainingMinutes:      s.RemainingMinutes(),
				Notes:                 s.Notes,
			}
			if s.Customer != nil {
				info.CustomerName = s.Customer.Name
			}
			item.ActiveSession = info
		}
		items = append(items, item)
	}
	return items, nil
}

func (uc *consoleUseCase) CreateConsole(ctx context.Context, req *CreateConsoleRequest) (*entity.Console, error) {
	now := time.Now()
	console := &entity.Console{
		ID:           uuid.New(),
		Name:         req.Name,
		ConsoleType:  req.ConsoleType,
		IPAddress:    req.IPAddress,
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
	if req.IPAddress != nil {
		console.IPAddress = req.IPAddress
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
