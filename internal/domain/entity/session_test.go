package entity

import (
	"testing"
	"time"
)

func TestApplySetupGraceShiftsStartAndEnd(t *testing.T) {
	start := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	end := start.Add(60 * time.Minute)
	s := &Session{StartTime: start, EndScheduledAt: &end}

	if !s.ApplySetupGrace(2) {
		t.Fatal("ApplySetupGrace should return true for a scheduled session")
	}

	wantStart := start.Add(2 * time.Minute)
	wantEnd := end.Add(2 * time.Minute)
	if !s.StartTime.Equal(wantStart) {
		t.Fatalf("start_time = %v, want %v", s.StartTime, wantStart)
	}
	if s.EndScheduledAt == nil || !s.EndScheduledAt.Equal(wantEnd) {
		t.Fatalf("end_scheduled_at = %v, want %v", s.EndScheduledAt, wantEnd)
	}
	if got := s.EndScheduledAt.Sub(s.StartTime); got != 60*time.Minute {
		t.Fatalf("durasi berbayar berubah: got %v, want 60m", got)
	}
}

func TestApplySetupGraceNoSchedule(t *testing.T) {
	start := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	s := &Session{StartTime: start}

	if s.ApplySetupGrace(2) {
		t.Fatal("ApplySetupGrace should return false for open-ended session")
	}
	if !s.StartTime.Equal(start) {
		t.Fatalf("start_time berubah padahal tidak ada jadwal: %v", s.StartTime)
	}
}

func TestApplySetupGraceZeroMinutes(t *testing.T) {
	start := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	s := &Session{StartTime: start, EndScheduledAt: &end}

	if s.ApplySetupGrace(0) {
		t.Fatal("ApplySetupGrace should return false for 0 minutes")
	}
	if !s.StartTime.Equal(start) || !s.EndScheduledAt.Equal(end) {
		t.Fatal("session tidak boleh berubah untuk 0 menit")
	}
}
