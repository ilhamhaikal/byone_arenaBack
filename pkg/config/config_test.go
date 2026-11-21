package config

import (
	"strings"
	"testing"
)

func TestIsWeakJWTSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		weak   bool
	}{
		{name: "pendek", secret: "abc123", weak: true},
		{name: "32 karakter berulang", secret: strings.Repeat("a", 32), weak: true},
		{name: "32 karakter acak", secret: "k7Qm2vX9pL4sT8wR1yN6bC3dF5gH0jZ2", weak: false},
		{name: "passphrase panjang", secret: "byone-arena-secret-key-yang-cukup-panjang-2026", weak: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWeakJWTSecret(tt.secret); got != tt.weak {
				t.Fatalf("isWeakJWTSecret(%q) = %v, want %v", tt.secret, got, tt.weak)
			}
		})
	}
}
