package usecase

import (
	"context"
	"testing"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

type fakeSessionRepo struct {
	repository.SessionRepository
	session     *entity.Session
	setupCalled bool
	setupMinute int
}

func (f *fakeSessionRepo) FindActiveByConsoleID(context.Context, uuid.UUID) (*entity.Session, error) {
	return nil, nil
}

func (f *fakeSessionRepo) CreateWithPayment(
	_ context.Context, s *entity.Session, _ float64, _ string, _ int, _ int, _ string,
) (*entity.Payment, error) {
	start := time.Date(2026, 11, 21, 7, 0, 0, 0, time.UTC)
	end := start.Add(time.Duration(s.BookedDurationMinutes) * time.Minute)
	s.ID = uuid.New()
	s.StartTime = start
	s.EndScheduledAt = &end
	f.session = s
	return &entity.Payment{}, nil
}

func (f *fakeSessionRepo) AddSetupGrace(_ context.Context, _ uuid.UUID, minutes int) (*entity.Session, error) {
	f.setupCalled = true
	f.setupMinute = minutes
	f.session.ApplySetupGrace(minutes)
	return f.session, nil
}

type fakeConsoleRepo struct {
	repository.ConsoleRepository
}

func (f *fakeConsoleRepo) FindByID(context.Context, uuid.UUID) (*entity.Console, error) {
	return &entity.Console{
		ID:     uuid.New(),
		Status: entity.ConsoleStatusAvailable,
	}, nil
}

func TestStartSessionAppliesSetupGraceAtStart(t *testing.T) {
	sessionRepo := &fakeSessionRepo{}
	uc := NewSessionUseCase(sessionRepo, &fakeConsoleRepo{})

	start := time.Date(2026, 11, 21, 7, 0, 0, 0, time.UTC)
	res, err := uc.StartSession(context.Background(), &StartSessionRequest{
		ConsoleID:             uuid.New(),
		BookedDurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
	if !sessionRepo.setupCalled {
		t.Fatal("AddSetupGrace tidak dipanggil saat mulai sesi")
	}
	if sessionRepo.setupMinute != SetupGraceMinutes {
		t.Fatalf("setup minutes = %d, want %d", sessionRepo.setupMinute, SetupGraceMinutes)
	}

	wantStart := start.Add(SetupGraceMinutes * time.Minute)
	wantEnd := start.Add((60 + SetupGraceMinutes) * time.Minute)
	if !res.Session.StartTime.Equal(wantStart) {
		t.Fatalf("start_time = %v, want %v", res.Session.StartTime, wantStart)
	}
	if res.Session.EndScheduledAt == nil || !res.Session.EndScheduledAt.Equal(wantEnd) {
		t.Fatalf("end_scheduled_at = %v, want %v", res.Session.EndScheduledAt, wantEnd)
	}
	if got := res.Session.EndScheduledAt.Sub(res.Session.StartTime); got != 60*time.Minute {
		t.Fatalf("durasi berbayar berubah: got %v, want 60m", got)
	}
}
