package handler

import "testing"

func TestSelectDailyPrice(t *testing.T) {
	tests := []struct {
		name         string
		consolePrice float64
		globalPrice  float64
		want         float64
		wantErr      bool
	}{
		{name: "harga konsol menang", consolePrice: 60000, globalPrice: 50000, want: 60000},
		{name: "fallback global", consolePrice: 0, globalPrice: 50000, want: 50000},
		{name: "keduanya kosong", consolePrice: 0, globalPrice: 0, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectDailyPrice(tt.consolePrice, tt.globalPrice)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("price = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateDailyRentalDates(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		end     string
		wantErr bool
	}{
		{name: "valid 3 hari", start: "2026-11-21", end: "2026-11-24"},
		{name: "valid tepat 30 hari", start: "2026-11-01", end: "2026-12-01"},
		{name: "tanggal selesai sama", start: "2026-11-21", end: "2026-11-21", wantErr: true},
		{name: "tanggal selesai sebelum", start: "2026-11-24", end: "2026-11-21", wantErr: true},
		{name: "lebih dari 30 hari", start: "2026-11-01", end: "2026-12-02", wantErr: true},
		{name: "format mulai salah", start: "21-11-2026", end: "2026-11-24", wantErr: true},
		{name: "format selesai salah", start: "2026-11-21", end: "24-11-2026", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDailyRentalDates(tt.start, tt.end)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
