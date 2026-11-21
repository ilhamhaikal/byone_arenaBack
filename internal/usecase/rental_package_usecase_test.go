package usecase

import (
	"context"
	"testing"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

type fakeRentalPackageRepo struct {
	repository.RentalPackageRepository
	session *entity.Session
}

func (f *fakeRentalPackageRepo) StartSessionWithPackage(
	_ context.Context, consoleID uuid.UUID, customerID *uuid.UUID, _ uuid.UUID, _ float64, _ string, _ *uuid.UUID,
) (*entity.Session, *entity.Payment, error) {
	start := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	end := start.Add(120 * time.Minute)
	f.session = &entity.Session{
		ID:                    uuid.New(),
		ConsoleID:             consoleID,
		CustomerID:            customerID,
		StartTime:             start,
		EndScheduledAt:        &end,
		BookedDurationMinutes: 120,
		Status:                entity.SessionStatusActive,
	}
	return f.session, &entity.Payment{}, nil
}

type graceFakeSessionRepo struct {
	repository.SessionRepository
	getSession  func() *entity.Session
	setupCalled bool
}

func (f *graceFakeSessionRepo) AddSetupGrace(_ context.Context, _ uuid.UUID, minutes int) (*entity.Session, error) {
	f.setupCalled = true
	s := f.getSession()
	s.ApplySetupGrace(minutes)
	return s, nil
}

func TestStartPackageAppliesSetupGraceAtStart(t *testing.T) {
	packageRepo := &fakeRentalPackageRepo{}
	sessionRepo := &graceFakeSessionRepo{
		getSession: func() *entity.Session { return packageRepo.session },
	}
	uc := NewRentalPackageUseCase(packageRepo, sessionRepo)

	packageStart := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	res, err := uc.StartWithPackage(context.Background(), &StartWithPackageRequest{
		ConsoleID: uuid.New(),
		PackageID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("StartWithPackage error: %v", err)
	}
	if !sessionRepo.setupCalled {
		t.Fatal("AddSetupGrace tidak dipanggil untuk sesi paket")
	}

	wantStart := packageStart.Add(SetupGraceMinutes * time.Minute)
	wantEnd := packageStart.Add((120 + SetupGraceMinutes) * time.Minute)
	if !res.Session.StartTime.Equal(wantStart) {
		t.Fatalf("start_time = %v, want %v", res.Session.StartTime, wantStart)
	}
	if res.Session.EndScheduledAt == nil || !res.Session.EndScheduledAt.Equal(wantEnd) {
		t.Fatalf("end_scheduled_at = %v, want %v", res.Session.EndScheduledAt, wantEnd)
	}
	if got := res.Session.EndScheduledAt.Sub(res.Session.StartTime); got != 120*time.Minute {
		t.Fatalf("durasi paket berubah: got %v, want 120m", got)
	}
}
