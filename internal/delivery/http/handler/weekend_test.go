package handler

import (
	"testing"
	"time"
)

func TestIsWeekendWIBAt(t *testing.T) {
	loc := wibLocation()

	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"Jumat", time.Date(2026, 11, 27, 21, 0, 0, 0, loc), false},
		{"Sabtu", time.Date(2026, 11, 28, 21, 0, 0, 0, loc), true},
		{"Minggu", time.Date(2026, 11, 29, 23, 0, 0, 0, loc), true},
		{"Senin", time.Date(2026, 11, 30, 1, 0, 0, 0, loc), false},
		// Jumat 18:00 UTC = Sabtu 01:00 WIB.
		{"Sabtu WIB dari UTC", time.Date(2026, 11, 27, 18, 0, 0, 0, time.UTC), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWeekendWIBAt(tc.at); got != tc.want {
				t.Fatalf("isWeekendWIBAt(%s) = %v, want %v", tc.at, got, tc.want)
			}
		})
	}
}

func TestIsPackageRecoveryWindowWIBAt(t *testing.T) {
	loc := wibLocation()

	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"Sabtu 20:59", time.Date(2026, 11, 28, 20, 59, 0, 0, loc), false},
		{"Sabtu 21:00", time.Date(2026, 11, 28, 21, 0, 0, 0, loc), true},
		{"Sabtu 21:59", time.Date(2026, 11, 28, 21, 59, 0, 0, loc), true},
		{"Sabtu 22:00", time.Date(2026, 11, 28, 22, 0, 0, 0, loc), false},
		{"Minggu 21:30", time.Date(2026, 11, 29, 21, 30, 0, 0, loc), true},
		{"Senin 21:00", time.Date(2026, 11, 30, 21, 0, 0, 0, loc), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPackageRecoveryWindowWIBAt(tc.at); got != tc.want {
				t.Fatalf("isPackageRecoveryWindowWIBAt(%s) = %v, want %v", tc.at, got, tc.want)
			}
		})
	}
}
