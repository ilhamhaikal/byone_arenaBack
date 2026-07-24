package entity

import "time"

// ReportSummary adalah laporan komprehensif untuk periode tertentu
type ReportSummary struct {
	Period              ReportPeriod         `json:"period"`
	Revenue             ReportRevenue        `json:"revenue"`
	Transactions        ReportTransactions   `json:"transactions"`
	Sessions            ReportSessions       `json:"sessions"`
	Vouchers            []ReportVoucherUsage `json:"vouchers"`
	Consoles            []ReportConsoleUsage `json:"consoles"`
	DailyBreakdown      []ReportDailyItem    `json:"dailyBreakdown"`
	ActiveDiscountRules []ReportDiscountRule `json:"activeDiscountRules"`
	FoodSales           ReportFoodSales      `json:"foodSales"`
	GeneratedAt         time.Time            `json:"generatedAt"`
}

type ReportPeriod struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	TotalDays int    `json:"totalDays"`
}

type ReportRevenue struct {
	TotalRevenue       float64 `json:"totalRevenue"`
	TotalBaseAmount    float64 `json:"totalBaseAmount"`
	VoucherDiscount    float64 `json:"voucherDiscount"`
	AutoDiscount       float64 `json:"autoDiscount"`
	TotalDiscount      float64 `json:"totalDiscount"`
	TotalCashReceived  float64 `json:"totalCashReceived"`
	TotalChange        float64 `json:"totalChange"`
	DailyRentalRevenue float64 `json:"dailyRentalRevenue"`
	DailyRentalCount   int     `json:"dailyRentalCount"`
	MembershipRevenue  float64 `json:"membershipRevenue"`
	MembershipCount    int     `json:"membershipCount"`
	FoodSalesRevenue   float64 `json:"foodSalesRevenue"`
	FoodSalesCount     int     `json:"foodSalesCount"`
}

type ReportTransactions struct {
	TotalTransactions   int     `json:"totalTransactions"`
	VoucherTransactions int     `json:"voucherTransactions"`
	AveragePerDay       float64 `json:"averagePerDay"`
}

type ReportSessions struct {
	TotalSessions    int     `json:"totalSessions"`
	TotalPlayMinutes int     `json:"totalPlayMinutes"`
	TotalPlayHours   float64 `json:"totalPlayHours"`
	AverageMinutes   int     `json:"averageMinutes"`
}

type ReportVoucherUsage struct {
	VoucherName   string  `json:"voucherName"`
	VoucherCode   string  `json:"voucherCode"`
	DiscountType  string  `json:"discountType"`
	UsageCount    int     `json:"usageCount"`
	TotalDiscount float64 `json:"totalDiscount"`
}

type ReportConsoleUsage struct {
	ConsoleName   string `json:"consoleName"`
	ConsoleType   string `json:"consoleType"`
	TotalSessions int    `json:"totalSessions"`
	TotalMinutes  int    `json:"totalMinutes"`
}

type ReportDailyItem struct {
	Date         string  `json:"date"`
	Revenue      float64 `json:"revenue"`
	Transactions int     `json:"transactions"`
	Sessions     int     `json:"sessions"`
	PlayMinutes  int     `json:"playMinutes"`
	FoodRevenue  float64 `json:"foodRevenue"`
	FoodOrders   int     `json:"foodOrders"`
}

type ReportDiscountRule struct {
	RuleName      string  `json:"ruleName"`
	RuleType      string  `json:"ruleType"`
	DiscountType  string  `json:"discountType"`
	DiscountValue float64 `json:"discountValue"`
	IsActive      bool    `json:"isActive"`
}

// ReportFoodSales adalah ringkasan penjualan makanan/minuman dalam periode laporan
type ReportFoodSales struct {
	TotalRevenue      float64          `json:"totalRevenue"`
	TotalOrders       int              `json:"totalOrders"`
	AverageOrderValue float64          `json:"averageOrderValue"`
	TopItems          []ReportFoodItem `json:"topItems"`
}

// ReportFoodItem adalah rincian item menu terlaris dalam periode laporan
type ReportFoodItem struct {
	ItemName     string  `json:"itemName"`
	Category     string  `json:"category"`
	QuantitySold int     `json:"quantitySold"`
	Revenue      float64 `json:"revenue"`
}
