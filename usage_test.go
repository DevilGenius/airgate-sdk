package sdk

import (
	"testing"
	"time"
)

func TestResetAtFromBaseClampsNegativeSeconds(t *testing.T) {
	base := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	resetAt := ResetAtFromBase(base, -30)
	if resetAt == nil {
		t.Fatalf("expected resetAt")
	}
	if !resetAt.Equal(base) {
		t.Fatalf("expected resetAt=%s, got %s", base, *resetAt)
	}
}

func TestRemainingSecondsUntil(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	resetAt := time.Date(2026, 3, 27, 10, 5, 0, 0, time.UTC)
	if got := RemainingSecondsUntil(&resetAt, now); got != 300 {
		t.Fatalf("expected 300, got %d", got)
	}
}

func TestRemainingSecondsUntilExpired(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 5, 0, 0, time.UTC)
	resetAt := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	if got := RemainingSecondsUntil(&resetAt, now); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestNewAccountUsageWindow(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	resetAt := time.Date(2026, 3, 27, 10, 5, 0, 0, time.UTC)
	window := NewAccountUsageWindow("5h", "5h", 25, &resetAt, now)
	if window.Key != "5h" {
		t.Fatalf("expected key=5h, got %q", window.Key)
	}
	if window.ResetAt != "2026-03-27T10:05:00Z" {
		t.Fatalf("expected reset_at to be serialized, got %q", window.ResetAt)
	}
	if window.ResetSeconds != 300 {
		t.Fatalf("expected reset_seconds=300, got %d", window.ResetSeconds)
	}
}
