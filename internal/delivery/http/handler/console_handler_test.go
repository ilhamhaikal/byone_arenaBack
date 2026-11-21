package handler

import (
	"testing"

	"byone-arena/internal/domain/entity"
	"byone-arena/pkg/deviceauth"

	"github.com/google/uuid"
)

func TestValidateDeviceCredentials(t *testing.T) {
	secret := "secret-tv-contoh"
	deviceID := uuid.New()
	valid := &entity.Console{
		DeviceID:         &deviceID,
		DeviceSecretHash: deviceauth.HashSecret(secret),
	}

	tests := []struct {
		name     string
		console  *entity.Console
		secret   string
		deviceID string
		want     bool
	}{
		{name: "valid", console: valid, secret: secret, deviceID: deviceID.String(), want: true},
		{name: "secret salah", console: valid, secret: "salah", deviceID: deviceID.String()},
		{name: "device id salah", console: valid, secret: secret, deviceID: uuid.New().String()},
		{name: "tanpa secret", console: valid, secret: "", deviceID: deviceID.String()},
		{name: "tanpa hash", console: &entity.Console{DeviceID: &deviceID}, secret: secret, deviceID: deviceID.String()},
		{name: "tanpa device id", console: &entity.Console{DeviceSecretHash: deviceauth.HashSecret(secret)}, secret: secret, deviceID: deviceID.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateDeviceCredentials(tt.console, tt.secret, tt.deviceID)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
