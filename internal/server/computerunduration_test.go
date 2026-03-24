package server

import "testing"

func TestComputeRunDuration_BothEmpty(t *testing.T) {
	got := computeRunDuration("", "")
	if got != "—" {
		t.Fatalf("got %q, want %q", got, "—")
	}
}

func TestComputeRunDuration_StartEmpty(t *testing.T) {
	got := computeRunDuration("", "2024-01-15T10:30:00Z")
	if got != "—" {
		t.Fatalf("got %q, want %q", got, "—")
	}
}

func TestComputeRunDuration_CompleteEmpty(t *testing.T) {
	got := computeRunDuration("2024-01-15T10:30:00Z", "")
	if got != "—" {
		t.Fatalf("got %q, want %q", got, "—")
	}
}

func TestComputeRunDuration_StartUnparseable(t *testing.T) {
	got := computeRunDuration("not-a-date", "2024-01-15T10:30:00Z")
	if got != "—" {
		t.Fatalf("got %q, want %q", got, "—")
	}
}

func TestComputeRunDuration_CompleteUnparseable(t *testing.T) {
	got := computeRunDuration("2024-01-15T10:30:00Z", "garbage")
	if got != "—" {
		t.Fatalf("got %q, want %q", got, "—")
	}
}

func TestComputeRunDuration_BothUnparseable(t *testing.T) {
	got := computeRunDuration("abc", "xyz")
	if got != "—" {
		t.Fatalf("got %q, want %q", got, "—")
	}
}

func TestComputeRunDuration_SameTime(t *testing.T) {
	got := computeRunDuration("2024-01-15T10:30:00Z", "2024-01-15T10:30:00Z")
	if got != "0s" {
		t.Fatalf("got %q, want %q", got, "0s")
	}
}

func TestComputeRunDuration_Seconds(t *testing.T) {
	// 3.8 seconds apart
	got := computeRunDuration("2024-01-15T10:30:00Z", "2024-01-15T10:30:03Z")
	if got != "3.0s" {
		t.Fatalf("got %q, want %q", got, "3.0s")
	}
}

func TestComputeRunDuration_Minutes(t *testing.T) {
	// 2m 15s apart
	got := computeRunDuration("2024-01-15T10:30:00Z", "2024-01-15T10:32:15Z")
	if got != "2m 15s" {
		t.Fatalf("got %q, want %q", got, "2m 15s")
	}
}

func TestComputeRunDuration_Hours(t *testing.T) {
	// 1h 3m apart
	got := computeRunDuration("2024-01-15T10:30:00Z", "2024-01-15T11:33:00Z")
	if got != "1h 3m" {
		t.Fatalf("got %q, want %q", got, "1h 3m")
	}
}

func TestComputeRunDuration_LargeDuration(t *testing.T) {
	// Cross day boundary: 25h 1m
	got := computeRunDuration("2024-01-15T10:30:00Z", "2024-01-16T11:31:00Z")
	if got != "25h 1m" {
		t.Fatalf("got %q, want %q", got, "25h 1m")
	}
}
