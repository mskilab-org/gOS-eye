package server

import "testing"

func TestFormatTimestamp_Zero(t *testing.T) {
	got := formatTimestamp(0)
	want := "—"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatTimestamp_Negative(t *testing.T) {
	got := formatTimestamp(-1)
	want := "—"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatTimestamp_LargeNegative(t *testing.T) {
	got := formatTimestamp(-999999999)
	want := "—"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatTimestamp_KnownEpoch(t *testing.T) {
	// 2024-01-15 10:30:01 UTC = 1705314601000 ms
	got := formatTimestamp(1705314601000)
	want := "2024-01-15 10:30:01 UTC"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatTimestamp_UnixEpochStart(t *testing.T) {
	// 1 ms after epoch → 1970-01-01 00:00:00 UTC
	got := formatTimestamp(1)
	want := "1970-01-01 00:00:00 UTC"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatTimestamp_AnotherKnownTime(t *testing.T) {
	// 2025-06-15 14:00:00 UTC = 1749996000000 ms
	got := formatTimestamp(1749996000000)
	want := "2025-06-15 14:00:00 UTC"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
