package handler

import (
	"testing"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/usecase"
)

func TestNormalizeNotificationRequest(t *testing.T) {
	req := &usecase.CreateNotificationRequest{
		Title:        "  Promo  ",
		Message:      "  Diskon malam  ",
		Priority:     "",
		LoopEnabled:  true,
		LoopInterval: 0,
		AutoShow:     false,
	}

	normalizeNotificationRequest(req)

	if req.Title != "Promo" {
		t.Fatalf("title harus di-trim, dapat %q", req.Title)
	}
	if req.Message != "Diskon malam" {
		t.Fatalf("message harus di-trim, dapat %q", req.Message)
	}
	if req.Priority != entity.NotificationPriorityNormal {
		t.Fatalf("priority kosong harus jadi normal, dapat %q", req.Priority)
	}
	if req.LoopInterval != 30 {
		t.Fatalf("interval tidak valid harus jadi 30, dapat %d", req.LoopInterval)
	}
	if req.LoopEnabled {
		t.Fatal("notifikasi manual (autoShow=false) tidak boleh looping")
	}
}

func TestNormalizeNotificationRequestKeepsValidLoop(t *testing.T) {
	req := &usecase.CreateNotificationRequest{
		Priority:     entity.NotificationPriorityHigh,
		LoopEnabled:  true,
		LoopInterval: 15,
		AutoShow:     true,
	}

	normalizeNotificationRequest(req)

	if !req.LoopEnabled || req.LoopInterval != 15 {
		t.Fatalf("loop valid tidak boleh diubah: %+v", req)
	}
}

func TestValidateNotificationTarget(t *testing.T) {
	cases := []struct {
		name    string
		req     usecase.CreateNotificationRequest
		wantErr bool
	}{
		{
			name: "semua konsol",
			req:  usecase.CreateNotificationRequest{TargetAll: true},
		},
		{
			name:    "target kosong ditolak",
			req:     usecase.CreateNotificationRequest{TargetAll: false},
			wantErr: true,
		},
		{
			name: "target spesifik diterima",
			req: usecase.CreateNotificationRequest{
				TargetAll:  false,
				ConsoleIDs: []string{"bd7edb34-0000-0000-0000-000000000000"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateNotificationTarget(&tc.req)
			if (got != "") != tc.wantErr {
				t.Fatalf("validateNotificationTarget() = %q, wantErr %v", got, tc.wantErr)
			}
		})
	}
}

func TestNotificationIsLoopEligible(t *testing.T) {
	base := entity.TvNotification{IsActive: true, AutoShow: true, LoopEnabled: true}
	if !notificationIsLoopEligible(&base) {
		t.Fatal("notifikasi aktif + otomatis + loop harus eligible")
	}

	manual := base
	manual.AutoShow = false
	if notificationIsLoopEligible(&manual) {
		t.Fatal("notifikasi manual tidak boleh eligible untuk loop")
	}

	inactive := base
	inactive.IsActive = false
	if notificationIsLoopEligible(&inactive) {
		t.Fatal("notifikasi nonaktif tidak boleh eligible untuk loop")
	}

	if notificationIsLoopEligible(nil) {
		t.Fatal("nil tidak boleh eligible")
	}
}
