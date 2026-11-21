package entity

import "testing"

func intPtr(v int) *int { return &v }

func TestPricingTierListValidate(t *testing.T) {
	tests := []struct {
		name    string
		tiers   PricingTierList
		wantErr bool
	}{
		{
			name:  "kosong valid (pakai harga flat)",
			tiers: PricingTierList{},
		},
		{
			name: "berurutan valid",
			tiers: PricingTierList{
				{StartMinute: 0, EndMinute: intPtr(60), Price: 9000},
				{StartMinute: 60, EndMinute: intPtr(120), Price: 8000},
				{StartMinute: 120, EndMinute: nil, Price: 7000},
			},
		},
		{
			name: "harus mulai dari 0",
			tiers: PricingTierList{
				{StartMinute: 30, EndMinute: intPtr(60), Price: 9000},
			},
			wantErr: true,
		},
		{
			name: "overlap ditolak",
			tiers: PricingTierList{
				{StartMinute: 0, EndMinute: intPtr(90), Price: 9000},
				{StartMinute: 60, EndMinute: nil, Price: 8000},
			},
			wantErr: true,
		},
		{
			name: "celah ditolak",
			tiers: PricingTierList{
				{StartMinute: 0, EndMinute: intPtr(60), Price: 9000},
				{StartMinute: 90, EndMinute: nil, Price: 8000},
			},
			wantErr: true,
		},
		{
			name: "unlimited harus di akhir",
			tiers: PricingTierList{
				{StartMinute: 0, EndMinute: nil, Price: 9000},
				{StartMinute: 60, EndMinute: nil, Price: 8000},
			},
			wantErr: true,
		},
		{
			name: "harga negatif ditolak",
			tiers: PricingTierList{
				{StartMinute: 0, EndMinute: nil, Price: -1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tiers.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
