package handler

import (
	"testing"
	"time"

	"byone-arena/internal/domain/entity"
)

func TestFinalizeStaffUserAggregatesDaysAndSessions(t *testing.T) {
	u := &entity.StaffReportUser{
		Days: []entity.StaffReportDay{
			{
				Date: "2026-11-22",
				Sessions: []entity.StaffLoginSession{
					{LoginAt: time.Date(2026, 11, 22, 14, 0, 0, 0, time.UTC)},
					{LoginAt: time.Date(2026, 11, 22, 8, 0, 0, 0, time.UTC)},
				},
				Revenue: entity.StaffRevenueTotals{
					Rental: 100000, Food: 20000, DailyRental: 50000,
					Membership: 30000, Game: 10000,
					RentalTrx: 2, FoodTrx: 1, DailyRentalTrx: 1,
					MembershipTrx: 1, GameTrx: 1,
				},
			},
			{
				Date: "2026-11-21",
				Revenue: entity.StaffRevenueTotals{
					Rental: 40000, RentalTrx: 1,
				},
			},
		},
	}

	finalizeStaffUser(u)

	if u.Days[0].Date != "2026-11-21" {
		t.Fatalf("hari harus terurut: %+v", u.Days)
	}
	if !u.Days[1].Sessions[0].LoginAt.Before(u.Days[1].Sessions[1].LoginAt) {
		t.Fatal("sesi harus terurut berdasarkan login_at")
	}
	if u.Days[1].Revenue.Total != 210000 {
		t.Fatalf("total harian = %v, want 210000", u.Days[1].Revenue.Total)
	}
	if u.Days[1].Revenue.TotalTrx != 6 {
		t.Fatalf("total trx harian = %v, want 6", u.Days[1].Revenue.TotalTrx)
	}
	if u.Totals.Total != 250000 {
		t.Fatalf("total periode = %v, want 250000", u.Totals.Total)
	}
	if u.Totals.TotalTrx != 7 {
		t.Fatalf("total trx periode = %v, want 7", u.Totals.TotalTrx)
	}
}
