package entity

import (
	"time"

	"github.com/google/uuid"
)

// DashboardSummary adalah agregat data pendapatan untuk dashboard
type DashboardSummary struct {
	Date                string               `json:"date"`
	TotalRevenue        float64              `json:"totalRevenue"`
	TotalBaseAmount     float64              `json:"totalBaseAmount"`
	TotalTransactions   int64                `json:"totalTransactions"`
	TotalDiscount       float64              `json:"totalDiscount"`
	TotalAutoDiscount   float64              `json:"totalAutoDiscount"`
	VoucherUsageCount   int64                `json:"voucherUsageCount"`
	TotalCashReceived   float64              `json:"totalCashReceived"`
	TotalChange         float64              `json:"totalChange"`
	DailyRentalRevenue  float64              `json:"dailyRentalRevenue"`
	DailyRentalCount    int64                `json:"dailyRentalCount"`
	MembershipRevenue   float64              `json:"membershipRevenue"`
	MembershipCount     int64                `json:"membershipCount"`
	FoodSalesRevenue    float64              `json:"foodSalesRevenue"`
	FoodOrderCount      int64                `json:"foodOrderCount"`
	FoodSalesHPP        float64              `json:"foodSalesHpp"`
	FoodSalesNetRevenue float64              `json:"foodSalesNetRevenue"`
	PendingFoodOrders   int                  `json:"pendingFoodOrders"`
	ActiveSessions      int                  `json:"activeSessions"`
	AvailableConsoles   int                  `json:"availableConsoles"`
	TotalConsoles       int                  `json:"totalConsoles"`
	VoucherDetails      []VoucherUsageDetail `json:"voucherDetails"`
	// RevenueMethods: rincian pendapatan hari ini per metode pembayaran
	// (tunai/qris/…) dari semua sumber — untuk detail saat kartu diklik.
	RevenueMethods []ReportPaymentMethod `json:"revenueMethods"`
	// FoodMethods: rincian penjualan makanan hari ini per metode pembayaran.
	FoodMethods []ReportPaymentMethod `json:"foodMethods"`
	GeneratedAt time.Time             `json:"generatedAt"`
	// InterruptedSessions: sesi yang terputus (listrik mati/TV offline) dan
	// sisa waktunya bisa dipulihkan admin dengan voucher GRATIS.
	InterruptedSessions []InterruptedSessionSummary `json:"interruptedSessions"`
}

// VoucherUsageDetail adalah detail penggunaan satu voucher dalam laporan
type VoucherUsageDetail struct {
	VoucherName   string  `json:"voucherName"`
	VoucherCode   string  `json:"voucherCode"`
	UsageCount    int64   `json:"usageCount"`
	TotalDiscount float64 `json:"totalDiscount"`
}

// InterruptedSessionSummary ringkasan sesi yang bisa dipulihkan.
type InterruptedSessionSummary struct {
	SessionID        uuid.UUID  `json:"sessionId"        gorm:"column:session_id"`
	ConsoleID        uuid.UUID  `json:"consoleId"        gorm:"column:console_id"`
	ConsoleName      string     `json:"consoleName"      gorm:"column:console_name"`
	CustomerName     string     `json:"customerName"     gorm:"column:customer_name"`
	RemainingMinutes int        `json:"remainingMinutes" gorm:"column:remaining_minutes"`
	InterruptedAt    *time.Time `json:"interruptedAt"    gorm:"column:interrupted_at"`
	Reason           string     `json:"reason"           gorm:"column:reason"`
	Notes            string     `json:"notes"            gorm:"column:notes"`
	// SavedTimeExpiresAt: batas pemulihan (24 jam untuk non-member).
	SavedTimeExpiresAt *time.Time `json:"savedTimeExpiresAt,omitempty" gorm:"column:saved_time_expires_at"`
	IsPackage          bool       `json:"isPackage" gorm:"column:is_package"`
}
