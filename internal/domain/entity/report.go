package entity

import "time"

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
	GeneratedAt         time.Time             `json:"generatedAt"`
}

type ReportPeriod struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	TotalDays int    `json:"totalDays"`
}

type ReportRevenue struct {
	TotalRevenue      float64 `json:"totalRevenue"`
	TotalBaseAmount   float64 `json:"totalBaseAmount"`
	VoucherDiscount   float64 `json:"voucherDiscount"`
	AutoDiscount      float64 `json:"autoDiscount"`
	TotalDiscount     float64 `json:"totalDiscount"`
	TotalCashReceived float64 `json:"totalCashReceived"`
	TotalChange       float64 `json:"totalChange"`
}

type ReportTransactions struct {
	TotalTransactions  int     `json:"totalTransactions"`
	VoucherTransactions int    `json:"voucherTransactions"`
	AveragePerDay      float64 `json:"averagePerDay"`
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
	ConsoleName    string `json:"consoleName"`
	ConsoleType    string `json:"consoleType"`
	TotalSessions  int    `json:"totalSessions"`
	TotalMinutes   int    `json:"totalMinutes"`
}

type ReportDailyItem struct {
	Date         string  `json:"date"`
	Revenue      float64 `json:"revenue"`
	Transactions int     `json:"transactions"`
	Sessions     int     `json:"sessions"`
	PlayMinutes  int     `json:"playMinutes"`
}

type ReportDiscountRule struct {
	RuleName      string  `json:"ruleName"`
	RuleType      string  `json:"ruleType"`
	DiscountType  string  `json:"discountType"`
	DiscountValue float64 `json:"discountValue"`
	IsActive      bool    `json:"isActive"`
}
