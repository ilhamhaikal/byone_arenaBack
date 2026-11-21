package paymentmethod

import "testing"

func TestNormalizeAndIsCashless(t *testing.T) {
	tests := []struct {
		input    string
		want     string
		cashless bool
	}{
		{input: "cash", want: "cash"},
		{input: "QRIS", want: "qris", cashless: true},
		{input: " transfer ", want: "transfer", cashless: true},
		{input: "lainnya", want: "cash"},
	}
	for _, tt := range tests {
		if got := Normalize(tt.input); got != tt.want {
			t.Fatalf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
		if got := IsCashless(tt.input); got != tt.cashless {
			t.Fatalf("IsCashless(%q) = %v, want %v", tt.input, got, tt.cashless)
		}
	}
}
