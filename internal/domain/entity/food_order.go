package entity

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus mendefinisikan status pesanan makanan
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"   // pesanan baru, menunggu diproses dapur
	OrderStatusPreparing OrderStatus = "preparing" // sedang diproses dapur
	OrderStatusServed    OrderStatus = "served"    // sudah diantarkan ke pelanggan
	OrderStatusCancelled OrderStatus = "cancelled" // dibatalkan
)

// FoodOrder merepresentasikan header satu pesanan makanan
type FoodOrder struct {
	ID          uuid.UUID   `json:"id"          gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	OrderNumber string      `json:"orderNumber" gorm:"not null;uniqueIndex;size:20"`
	SessionID   *uuid.UUID  `json:"sessionId,omitempty"  gorm:"type:uuid;index"`   // opsional: terhubung ke sesi PS
	CustomerID  *uuid.UUID  `json:"customerId,omitempty" gorm:"type:uuid;index"`   // opsional: walk-in
	Status      OrderStatus `json:"status"      gorm:"not null;default:'pending';size:20;index"`
	TotalAmount float64     `json:"totalAmount" gorm:"not null;default:0;type:numeric(10,2)"`
	Notes       string      `json:"notes,omitempty" gorm:"type:text"`
	CreatedAt   time.Time   `json:"createdAt"   gorm:"autoCreateTime"`
	UpdatedAt   time.Time   `json:"updatedAt"   gorm:"autoUpdateTime"`

	// Relasi
	Session  *Session        `json:"session,omitempty"  gorm:"foreignKey:SessionID"`
	Customer *Customer       `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
	Items    []*FoodOrderItem `json:"items,omitempty"    gorm:"foreignKey:OrderID"`
}

func (FoodOrder) TableName() string { return "food_orders" }

// FoodOrderItem merepresentasikan satu baris item dalam pesanan makanan
type FoodOrderItem struct {
	ID         uuid.UUID `json:"id"        gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	OrderID    uuid.UUID `json:"orderId"   gorm:"type:uuid;not null;index"`
	MenuItemID uuid.UUID `json:"menuItemId" gorm:"type:uuid;not null;index"`
	Quantity   int       `json:"quantity"  gorm:"not null"`
	UnitPrice  float64   `json:"unitPrice" gorm:"not null;type:numeric(10,2)"` // snapshot harga saat pesan
	Subtotal   float64   `json:"subtotal"  gorm:"not null;type:numeric(10,2)"` // quantity × unit_price
	Notes      string    `json:"notes,omitempty" gorm:"type:text"`
	CreatedAt  time.Time `json:"createdAt" gorm:"autoCreateTime"`

	// Relasi
	MenuItem *MenuItem `json:"menuItem,omitempty" gorm:"foreignKey:MenuItemID"`
}

func (FoodOrderItem) TableName() string { return "food_order_items" }
