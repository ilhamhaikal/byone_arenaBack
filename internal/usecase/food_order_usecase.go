package usecase

import (
	"context"
	"errors"
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// FoodOrderUseCase mendefinisikan logika bisnis untuk pesanan makanan
type FoodOrderUseCase interface {
	GetAllOrders(ctx context.Context) ([]*entity.FoodOrder, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*entity.FoodOrder, error)
	GetOrdersBySession(ctx context.Context, sessionID uuid.UUID) ([]*entity.FoodOrder, error)
	GetOrdersByStatus(ctx context.Context, status string) ([]*entity.FoodOrder, error)
	CreateOrder(ctx context.Context, req *CreateFoodOrderRequest) (*entity.FoodOrder, error)
	UpdateOrderStatus(ctx context.Context, id uuid.UUID, req *UpdateOrderStatusRequest) (*entity.FoodOrder, error)
	CancelOrder(ctx context.Context, id uuid.UUID) (*entity.FoodOrder, error)
	DeleteOrder(ctx context.Context, id uuid.UUID) error
}

type foodOrderUseCase struct {
	orderRepo repository.FoodOrderRepository
}

// NewFoodOrderUseCase membuat instance baru FoodOrderUseCase
func NewFoodOrderUseCase(orderRepo repository.FoodOrderRepository) FoodOrderUseCase {
	return &foodOrderUseCase{orderRepo: orderRepo}
}

// FoodOrderItemRequest payload satu item dalam pesanan
type FoodOrderItemRequest struct {
	MenuItemID uuid.UUID `json:"menuItemId" validate:"required"`
	Quantity   int       `json:"quantity"   validate:"required,min=1"`
	Notes      string    `json:"notes"`
}

// CreateFoodOrderRequest payload untuk membuat pesanan baru
type CreateFoodOrderRequest struct {
	SessionID  *uuid.UUID             `json:"sessionId"`  // opsional: sesi PS yang sedang berjalan
	CustomerID *uuid.UUID             `json:"customerId"` // opsional: pelanggan terdaftar
	Notes      string                 `json:"notes"`
	Items      []FoodOrderItemRequest `json:"items" validate:"required,min=1,dive"`
}

// UpdateOrderStatusRequest payload untuk update status pesanan
type UpdateOrderStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=pending preparing served cancelled"`
}

func (uc *foodOrderUseCase) GetAllOrders(ctx context.Context) ([]*entity.FoodOrder, error) {
	return uc.orderRepo.FindAll(ctx)
}

func (uc *foodOrderUseCase) GetOrderByID(ctx context.Context, id uuid.UUID) (*entity.FoodOrder, error) {
	order, err := uc.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("pesanan tidak ditemukan")
	}
	return order, nil
}

func (uc *foodOrderUseCase) GetOrdersBySession(ctx context.Context, sessionID uuid.UUID) ([]*entity.FoodOrder, error) {
	return uc.orderRepo.FindBySessionID(ctx, sessionID)
}

func (uc *foodOrderUseCase) GetOrdersByStatus(ctx context.Context, status string) ([]*entity.FoodOrder, error) {
	os := entity.OrderStatus(strings.ToLower(status))
	switch os {
	case entity.OrderStatusPending, entity.OrderStatusPreparing,
		entity.OrderStatusServed, entity.OrderStatusCancelled:
	default:
		return nil, errors.New("status tidak valid, pilih: pending, preparing, served, cancelled")
	}
	return uc.orderRepo.FindByStatus(ctx, os)
}

func (uc *foodOrderUseCase) CreateOrder(ctx context.Context, req *CreateFoodOrderRequest) (*entity.FoodOrder, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("pesanan harus memiliki minimal 1 item")
	}

	// Konversi items ke input repository
	repoItems := make([]repository.FoodOrderItemInput, len(req.Items))
	for i, it := range req.Items {
		repoItems[i] = repository.FoodOrderItemInput{
			MenuItemID: it.MenuItemID,
			Quantity:   it.Quantity,
			Notes:      it.Notes,
		}
	}

	order := &entity.FoodOrder{
		SessionID:  req.SessionID,
		CustomerID: req.CustomerID,
		Notes:      req.Notes,
	}

	if err := uc.orderRepo.Create(ctx, order, repoItems); err != nil {
		return nil, err
	}

	// Ambil data lengkap dengan relasi
	full, err := uc.orderRepo.FindByID(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	return full, nil
}

func (uc *foodOrderUseCase) UpdateOrderStatus(ctx context.Context, id uuid.UUID, req *UpdateOrderStatusRequest) (*entity.FoodOrder, error) {
	order, err := uc.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("pesanan tidak ditemukan")
	}

	newStatus := entity.OrderStatus(req.Status)
	if err := uc.orderRepo.UpdateStatus(ctx, id, newStatus); err != nil {
		return nil, err
	}

	order.Status = newStatus
	return order, nil
}

func (uc *foodOrderUseCase) CancelOrder(ctx context.Context, id uuid.UUID) (*entity.FoodOrder, error) {
	return uc.UpdateOrderStatus(ctx, id, &UpdateOrderStatusRequest{Status: "cancelled"})
}

func (uc *foodOrderUseCase) DeleteOrder(ctx context.Context, id uuid.UUID) error {
	order, err := uc.orderRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if order == nil {
		return errors.New("pesanan tidak ditemukan")
	}
	if order.Status != entity.OrderStatusCancelled {
		return errors.New("hanya pesanan yang sudah dibatalkan yang bisa dihapus")
	}
	return uc.orderRepo.Delete(ctx, id)
}
