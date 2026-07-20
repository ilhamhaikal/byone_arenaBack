package entity

import "time"

// DashboardSummary adalah agregat data pendapatan untuk dashboard
type DashboardSummary struct {
	Date               string               `json:"date"`
	TotalRevenue       float64              `json:"totalRevenue"`
	TotalBaseAmount    float64              `json:"totalBaseAmount"`
	TotalTransactions  int64                `json:"totalTransactions"`
	TotalDiscount      float64              `json:"totalDiscount"`
	TotalAutoDiscount  float64              `json:"totalAutoDiscount"`
	VoucherUsageCount  int64                `json:"voucherUsageCount"`
	TotalCashReceived  float64              `json:"totalCashReceived"`
	TotalChange        float64              `json:"totalChange"`
	DailyRentalRevenue float64              `json:"dailyRentalRevenue"`
	DailyRentalCount   int64                `json:"dailyRentalCount"`
	MembershipRevenue  float64              `json:"membershipRevenue"`
	MembershipCount    int64                `json:"membershipCount"`
	ActiveSessions     int                  `json:"activeSessions"`
	AvailableConsoles  int                  `json:"availableConsoles"`
	TotalConsoles      int                  `json:"totalConsoles"`
	VoucherDetails     []VoucherUsageDetail `json:"voucherDetails"`
	GeneratedAt        time.Time            `json:"generatedAt"`
}

// VoucherUsageDetail adalah detail penggunaan satu voucher dalam laporan
type VoucherUsageDetail struct {
	VoucherName   string  `json:"voucherName"`
	VoucherCode   string  `json:"voucherCode"`
	UsageCount    int64   `json:"usageCount"`
	TotalDiscount float64 `json:"totalDiscount"`
}

