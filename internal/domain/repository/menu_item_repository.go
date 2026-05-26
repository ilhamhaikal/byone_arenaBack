package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// MenuItemRepository mendefinisikan kontrak akses data untuk menu makanan
type MenuItemRepository interface {
	FindAll(ctx context.Context) ([]*entity.MenuItem, error)
	FindAvailable(ctx context.Context) ([]*entity.MenuItem, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error)
	FindByCategory(ctx context.Context, category entity.MenuItemCategory) ([]*entity.MenuItem, error)
	Create(ctx context.Context, item *entity.MenuItem) error
	Update(ctx context.Context, item *entity.MenuItem) error
	Delete(ctx context.Context, id uuid.UUID) error
}
