package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// FoodOrderRepository mendefinisikan kontrak akses data untuk pesanan makanan
type FoodOrderRepository interface {
	FindAll(ctx context.Context) ([]*entity.FoodOrder, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.FoodOrder, error)
	FindBySessionID(ctx context.Context, sessionID uuid.UUID) ([]*entity.FoodOrder, error)
	FindByStatus(ctx context.Context, status entity.OrderStatus) ([]*entity.FoodOrder, error)
	// Create menggunakan byoneCreateFoodOrder (atomik: order + items + total)
	Create(ctx context.Context, order *entity.FoodOrder, items []FoodOrderItemInput) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.OrderStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// FoodOrderItemInput data item pesanan yang dikirim ke stored procedure
type FoodOrderItemInput struct {
	MenuItemID uuid.UUID
	Quantity   int
	Notes      string
}
