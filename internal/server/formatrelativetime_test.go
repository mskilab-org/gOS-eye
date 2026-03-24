package server

import (
	"testing"
	"time"
)

func TestFormatRelativeTime_EmptyInput(t *testing.T) {
	got := formatRelativeTime("", time.Now())
	if got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestFormatRelativeTime_InvalidTimestamp(t *testing.T) {
	got := formatRelativeTime("not-a-timestamp", time.Now())
	if got != "not-a-timestamp" {
		t.Errorf("expected raw input for invalid timestamp, got %q", got)
	}
}

func TestFormatRelativeTime_JustNow(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 30, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:00:00Z", now)
	if got != "just now" {
		t.Errorf("expected 'just now' for 30s ago, got %q", got)
	}
}

func TestFormatRelativeTime_MinutesAgo(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:00:00Z", now)
	if got != "5m ago" {
		t.Errorf("expected '5m ago', got %q", got)
	}
}

func TestFormatRelativeTime_HoursAgo(t *testing.T) {
	now := time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:00:00Z", now)
	if got != "3h ago" {
		t.Errorf("expected '3h ago', got %q", got)
	}
}

func TestFormatRelativeTime_OlderThanDay(t *testing.T) {
	now := time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:30:00Z", now)
	// Should use short date format, not relative
	if got == "" || got == "2024-01-15T10:30:00Z" {
		t.Errorf("expected short date format for >24h ago, got %q", got)
	}
	// Should contain the month and day
	if len(got) < 5 {
		t.Errorf("expected date-like format, got %q", got)
	}
}

func TestFormatRelativeTime_FutureTimestamp(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:00:00Z", now)
	// Future timestamps should show "just now" (clamped to 0)
	if got != "just now" {
		t.Errorf("expected 'just now' for future timestamp, got %q", got)
	}
}

func TestFormatRelativeTime_ExactlyOneMinute(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 1, 0, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:00:00Z", now)
	if got != "1m ago" {
		t.Errorf("expected '1m ago', got %q", got)
	}
}

func TestFormatRelativeTime_ExactlyOneHour(t *testing.T) {
	now := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	got := formatRelativeTime("2024-01-15T10:00:00Z", now)
	if got != "1h ago" {
		t.Errorf("expected '1h ago', got %q", got)
	}
}
