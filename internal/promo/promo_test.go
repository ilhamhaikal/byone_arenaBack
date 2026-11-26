package promo

import (
	"testing"
	"time"

	"byone-arena/internal/domain/entity"
)

func TestParseDaysAcceptsSundayAlias(t *testing.T) {
	days := ParseDays("6,7")

	if !days[6] {
		t.Fatal("Sabtu (6) harus aktif")
	}
	if !days[0] {
		t.Fatal("7 harus diterima sebagai alias Minggu (0)")
	}
}

func TestFirstEligibleRuleUsesWeekendAndMinimumPaid(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	saturday := time.Date(2026, 11, 28, 20, 0, 0, 0, jakarta)

	rules := []entity.PromoBonusRule{
		{
			Name:           "Bonus Weekend Umum",
			DaysOfWeek:     "0,6",
			MinPaidMinutes: 60,
			BonusMinutes:   30,
			Audience:       entity.PromoAudienceGeneral,
			IsActive:       true,
		},
	}

	if rule := firstEligibleRule(rules, saturday, 30); rule != nil {
		t.Fatal("bonus tidak boleh berlaku sebelum minimal menit terpenuhi")
	}
	if rule := firstEligibleRule(rules, saturday, 60); rule == nil {
		t.Fatal("bonus harus berlaku saat minimal menit terpenuhi")
	}
	if rule := firstEligibleRule(rules, saturday.AddDate(0, 0, 1), 60); rule == nil {
		t.Fatal("bonus harus berlaku pada Minggu")
	}
	if rule := firstEligibleRule(rules, saturday.AddDate(0, 0, 4), 60); rule != nil {
		t.Fatal("bonus tidak boleh berlaku pada hari kerja")
	}
}
