package usecase

import (
	"context"
	"testing"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

type fakeFoodOrderRepo struct {
	repository.FoodOrderRepository
	t         *testing.T
	created   *entity.FoodOrder
	items     []repository.FoodOrderItemInput
	findCalls int
}

func (f *fakeFoodOrderRepo) Create(_ context.Context, order *entity.FoodOrder, items []repository.FoodOrderItemInput) error {
	// Nilai dari client harus sudah tidak diteruskan ke repository.
	if order.DiscountAmount != 0 {
		f.t.Fatalf("discount dari client tidak boleh diteruskan: got %v", order.DiscountAmount)
	}
	order.ID = uuid.New()
	order.TotalAmount = 100000
	// Simulasi SP yang menghitung diskon server-side.
	order.DiscountAmount = 15000
	f.created = order
	f.items = items
	return nil
}

func (f *fakeFoodOrderRepo) FindByID(context.Context, uuid.UUID) (*entity.FoodOrder, error) {
	f.findCalls++
	return f.created, nil
}

func TestCreateFoodOrderIgnoresClientDiscount(t *testing.T) {
	repo := &fakeFoodOrderRepo{t: t}
	uc := NewFoodOrderUseCase(repo)
	menuID := uuid.New()
	customerID := uuid.New()

	order, err := uc.CreateOrder(context.Background(), &CreateFoodOrderRequest{
		CustomerID:   &customerID,
		AutoDiscount: 999999, // harus diabaikan
		Items: []FoodOrderItemRequest{
			{MenuItemID: menuID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder error: %v", err)
	}
	if order.DiscountAmount != 15000 {
		t.Fatalf("discount = %v, want diskon dari server 15000", order.DiscountAmount)
	}
	if order.CustomerID == nil || *order.CustomerID != customerID {
		t.Fatal("customerId tidak diteruskan ke repository")
	}
	if len(repo.items) != 1 || repo.items[0].Quantity != 2 {
		t.Fatalf("items tidak diteruskan dengan benar: %+v", repo.items)
	}
}
