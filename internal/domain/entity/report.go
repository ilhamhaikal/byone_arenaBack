package entity

import (
	"time"

	"github.com/google/uuid"
)

// ReportSummary adalah laporan komprehensif untuk periode tertentu
type ReportSummary struct {
	Period              ReportPeriod          `json:"period"`
	Revenue             ReportRevenue         `json:"revenue"`
	Transactions        ReportTransactions    `json:"transactions"`
	Sessions            ReportSessions        `json:"sessions"`
	Vouchers            []ReportVoucherUsage  `json:"vouchers"`
	Consoles            []ReportConsoleUsage  `json:"consoles"`
	DailyBreakdown      []ReportDailyItem     `json:"dailyBreakdown"`
	ActiveDiscountRules []ReportDiscountRule  `json:"activeDiscountRules"`
	FoodSales           ReportFoodSales       `json:"foodSales"`
	PaymentMethods      []ReportPaymentMethod `json:"paymentMethods"`
	GeneratedAt         time.Time             `json:"generatedAt"`
}

type ReportPeriod struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	TotalDays int    `json:"totalDays"`
}

type ReportRevenue struct {
	TotalRevenue        float64 `json:"totalRevenue"`
	TotalBaseAmount     float64 `json:"totalBaseAmount"`
	VoucherDiscount     float64 `json:"voucherDiscount"`
	AutoDiscount        float64 `json:"autoDiscount"`
	TotalDiscount       float64 `json:"totalDiscount"`
	TotalCashReceived   float64 `json:"totalCashReceived"`
	TotalChange         float64 `json:"totalChange"`
	DailyRentalRevenue  float64 `json:"dailyRentalRevenue"`
	DailyRentalCount    int     `json:"dailyRentalCount"`
	MembershipRevenue   float64 `json:"membershipRevenue"`
	MembershipCount     int     `json:"membershipCount"`
	FoodSalesRevenue    float64 `json:"foodSalesRevenue"`
	FoodSalesBaseAmount float64 `json:"foodSalesBaseAmount"`
	FoodSalesDiscount   float64 `json:"foodSalesDiscount"`
	FoodSalesHpp        float64 `json:"foodSalesHpp"`
	FoodSalesNetRevenue float64 `json:"foodSalesNetRevenue"`
	FoodSalesCount      int     `json:"foodSalesCount"`
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
	BaseAmount        float64          `json:"baseAmount"`
	DiscountAmount    float64          `json:"discountAmount"`
	TotalHpp          float64          `json:"totalHpp"`
	NetRevenue        float64          `json:"netRevenue"`
	TotalOrders       int              `json:"totalOrders"`
	AverageOrderValue float64          `json:"averageOrderValue"`
	TopItems          []ReportFoodItem `json:"topItems"`
}

// ReportFoodItem adalah rincian item menu terlaris dalam periode laporan
type ReportFoodItem struct {
	ItemName       string  `json:"itemName"`
	Category       string  `json:"category"`
	QuantitySold   int     `json:"quantitySold"`
	Revenue        float64 `json:"revenue"`
	Hpp            float64 `json:"hpp"`
	NetRevenue     float64 `json:"netRevenue"`
	RevenuePerUnit float64 `json:"revenuePerUnit"`
	HppPerUnit     float64 `json:"hppPerUnit"`
	ProfitPerUnit  float64 `json:"profitPerUnit"`
}

// ReportPaymentMethod ringkasan pendapatan per metode pembayaran.
type ReportPaymentMethod struct {
	Method string  `json:"method"`
	Total  float64 `json:"total"`
	Trx    int     `json:"trx"`
}

// StaffRevenueTotals ringkasan pendapatan per sumber untuk satu petugas/hari.
type StaffRevenueTotals struct {
	Rental      float64 `json:"rental"`
	Food        float64 `json:"food"`
	DailyRental float64 `json:"dailyRental"`
	Membership  float64 `json:"membership"`
	Game        float64 `json:"game"`
	Total       float64 `json:"total"`

	RentalTrx      int `json:"rentalTrx"`
	FoodTrx        int `json:"foodTrx"`
	DailyRentalTrx int `json:"dailyRentalTrx"`
	MembershipTrx  int `json:"membershipTrx"`
	GameTrx        int `json:"gameTrx"`
	TotalTrx       int `json:"totalTrx"`
}

// StaffLoginSession satu sesi login–logout petugas.
type StaffLoginSession struct {
	LoginAt         time.Time  `json:"loginAt"`
	LogoutAt        *time.Time `json:"logoutAt,omitempty"`
	DurationMinutes int        `json:"durationMinutes"`
	StillLoggedIn   bool       `json:"stillLoggedIn"`
}

// StaffReportDay laporan satu petugas pada satu tanggal.
type StaffReportDay struct {
	Date     string              `json:"date"`
	Sessions []StaffLoginSession `json:"sessions"`
	Revenue  StaffRevenueTotals  `json:"revenue"`
}

// StaffReportUser laporan lengkap satu petugas.
type StaffReportUser struct {
	UserID   *uuid.UUID         `json:"userId,omitempty"`
	Username string             `json:"username"`
	FullName string             `json:"fullName"`
	Role     string             `json:"role"`
	Days     []StaffReportDay   `json:"days"`
	Totals   StaffRevenueTotals `json:"totals"`
}

// StaffReport laporan per petugas pada satu periode.
type StaffReport struct {
	Period      ReportPeriod      `json:"period"`
	Staff       []StaffReportUser `json:"staff"`
	Unknown     *StaffReportUser  `json:"unknown,omitempty"`
	GeneratedAt time.Time         `json:"generatedAt"`
}

// StaffDailyRevenueRow baris mentah dari SP StaffDailyRevenue.
type StaffDailyRevenueRow struct {
	UserID             *uuid.UUID `gorm:"column:user_id"`
	ReportDate         time.Time  `gorm:"column:report_date"`
	RentalRevenue      float64    `gorm:"column:rental_revenue"`
	FoodRevenue        float64    `gorm:"column:food_revenue"`
	DailyRentalRevenue float64    `gorm:"column:daily_rental_revenue"`
	MembershipRevenue  float64    `gorm:"column:membership_revenue"`
	RentalTrx          int        `gorm:"column:rental_trx"`
	FoodTrx            int        `gorm:"column:food_trx"`
	DailyRentalTrx     int        `gorm:"column:daily_rental_trx"`
	MembershipTrx      int        `gorm:"column:membership_trx"`
}

// StaffLoginRow baris mentah dari SP StaffLoginSessions.
type StaffLoginRow struct {
	UserID          uuid.UUID  `gorm:"column:user_id"`
	LoginAt         time.Time  `gorm:"column:login_at"`
	LogoutAt        *time.Time `gorm:"column:logout_at"`
	DurationMinutes int        `gorm:"column:duration_minutes"`
	StillLoggedIn   bool       `gorm:"column:still_logged_in"`
}
