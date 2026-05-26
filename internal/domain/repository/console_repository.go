package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// ConsoleRepository mendefinisikan kontrak akses data untuk entitas Console
type ConsoleRepository interface {
	FindAll(ctx context.Context) ([]*entity.Console, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Console, error)
	FindByStatus(ctx context.Context, status entity.ConsoleStatus) ([]*entity.Console, error)
	Create(ctx context.Context, console *entity.Console) error
	Update(ctx context.Context, console *entity.Console) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.ConsoleStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
}
