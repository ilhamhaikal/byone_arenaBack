package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type foodOrderRepository struct {
	db *gorm.DB
}

// NewFoodOrderRepository membuat instance baru FoodOrderRepository
func NewFoodOrderRepository(db *gorm.DB) repository.FoodOrderRepository {
	return &foodOrderRepository{db: db}
}

func (r *foodOrderRepository) FindAll(ctx context.Context) ([]*entity.FoodOrder, error) {
	var orders []*entity.FoodOrder
	err := r.db.WithContext(ctx).
		Preload("Customer").
		Preload("Session").
		Preload("Items.MenuItem").
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

func (r *foodOrderRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.FoodOrder, error) {
	var order entity.FoodOrder
	err := r.db.WithContext(ctx).
		Preload("Customer").
		Preload("Session").
		Preload("Items.MenuItem").
		First(&order, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *foodOrderRepository) FindBySessionID(ctx context.Context, sessionID uuid.UUID) ([]*entity.FoodOrder, error) {
	var orders []*entity.FoodOrder
	err := r.db.WithContext(ctx).
		Preload("Items.MenuItem").
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

func (r *foodOrderRepository) FindByStatus(ctx context.Context, status entity.OrderStatus) ([]*entity.FoodOrder, error) {
	var orders []*entity.FoodOrder
	err := r.db.WithContext(ctx).
		Preload("Customer").
		Preload("Items.MenuItem").
		Where("status = ?", status).
		Order("created_at ASC").
		Find(&orders).Error
	return orders, err
}

// Create menggunakan byoneCreateFoodOrder untuk atomisitas
func (r *foodOrderRepository) Create(ctx context.Context, order *entity.FoodOrder, items []repository.FoodOrderItemInput) error {
	// Bangun JSON array untuk items
	type spItem struct {
		MenuItemID string `json:"menu_item_id"`
		Quantity   int    `json:"quantity"`
		Notes      string `json:"notes"`
	}
	spItems := make([]spItem, len(items))
	for i, it := range items {
		spItems[i] = spItem{
			MenuItemID: it.MenuItemID.String(),
			Quantity:   it.Quantity,
			Notes:      it.Notes,
		}
	}

	itemsJSON, err := json.Marshal(spItems)
	if err != nil {
		return fmt.Errorf("gagal encode items: %w", err)
	}

	type spResult struct {
		OrderID     uuid.UUID `gorm:"column:order_id"`
		OrderNumber string    `gorm:"column:order_number"`
		TotalAmount float64   `gorm:"column:total_amount"`
	}

	var result spResult
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT * FROM %s(?, ?, ?, ?::jsonb)", spname.Ident("CreateFoodOrder")),
		order.SessionID,
		order.CustomerID,
		order.Notes,
		string(itemsJSON),
	).Scan(&result)

	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}

	order.ID = result.OrderID
	order.OrderNumber = result.OrderNumber
	order.TotalAmount = result.TotalAmount
	order.Status = entity.OrderStatusPending
	return nil
}

// UpdateStatus menggunakan stored procedure untuk validasi transisi status
func (r *foodOrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.OrderStatus) error {
	tx := r.db.WithContext(ctx).Exec(fmt.Sprintf("SELECT %s(?, ?)", spname.Ident("UpdateFoodOrderStatus")), id, status)
	return parseStoredProcError(tx.Error)
}

func (r *foodOrderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Hapus items terlebih dahulu (meski ada ON DELETE CASCADE, lebih eksplisit)
	if err := r.db.WithContext(ctx).Delete(&entity.FoodOrderItem{}, "order_id = ?", id).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&entity.FoodOrder{}, "id = ?", id).Error
}
