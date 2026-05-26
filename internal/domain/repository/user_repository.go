package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// UserRepository mendefinisikan kontrak akses data untuk entitas User
type UserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
}
