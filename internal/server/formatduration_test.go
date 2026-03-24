package server

import "testing"

func TestFormatDuration_Zero(t *testing.T) {
	got := formatDuration(0)
	if got != "0s" {
		t.Fatalf("got %q, want %q", got, "0s")
	}
}

func TestFormatDuration_Milliseconds_One(t *testing.T) {
	got := formatDuration(1)
	if got != "1ms" {
		t.Fatalf("got %q, want %q", got, "1ms")
	}
}

func TestFormatDuration_Milliseconds_500(t *testing.T) {
	got := formatDuration(500)
	if got != "500ms" {
		t.Fatalf("got %q, want %q", got, "500ms")
	}
}

func TestFormatDuration_Milliseconds_999(t *testing.T) {
	got := formatDuration(999)
	if got != "999ms" {
		t.Fatalf("got %q, want %q", got, "999ms")
	}
}

func TestFormatDuration_Seconds_Exact(t *testing.T) {
	got := formatDuration(1000)
	if got != "1.0s" {
		t.Fatalf("got %q, want %q", got, "1.0s")
	}
}

func TestFormatDuration_Seconds_WithDecimal(t *testing.T) {
	got := formatDuration(3800)
	if got != "3.8s" {
		t.Fatalf("got %q, want %q", got, "3.8s")
	}
}

func TestFormatDuration_Seconds_OneAndHalf(t *testing.T) {
	got := formatDuration(1500)
	if got != "1.5s" {
		t.Fatalf("got %q, want %q", got, "1.5s")
	}
}

func TestFormatDuration_Seconds_TruncatesNotRounds(t *testing.T) {
	// 59999ms = 59.999s → truncated to one decimal = "59.9s"
	got := formatDuration(59999)
	if got != "59.9s" {
		t.Fatalf("got %q, want %q", got, "59.9s")
	}
}

func TestFormatDuration_Seconds_ExactWhole(t *testing.T) {
	got := formatDuration(59000)
	if got != "59.0s" {
		t.Fatalf("got %q, want %q", got, "59.0s")
	}
}

func TestFormatDuration_Minutes_Exact(t *testing.T) {
	got := formatDuration(60000)
	if got != "1m 0s" {
		t.Fatalf("got %q, want %q", got, "1m 0s")
	}
}

func TestFormatDuration_Minutes_WithSeconds(t *testing.T) {
	// 135000ms = 2m 15s
	got := formatDuration(135000)
	if got != "2m 15s" {
		t.Fatalf("got %q, want %q", got, "2m 15s")
	}
}

func TestFormatDuration_Minutes_UpperBound(t *testing.T) {
	// 3599000ms = 59m 59s
	got := formatDuration(3599000)
	if got != "59m 59s" {
		t.Fatalf("got %q, want %q", got, "59m 59s")
	}
}

func TestFormatDuration_Minutes_SubSecondTruncated(t *testing.T) {
	// 135500ms = 2m 15.5s → shows "2m 15s" (sub-second truncated)
	got := formatDuration(135500)
	if got != "2m 15s" {
		t.Fatalf("got %q, want %q", got, "2m 15s")
	}
}

func TestFormatDuration_Hours_Exact(t *testing.T) {
	got := formatDuration(3600000)
	if got != "1h 0m" {
		t.Fatalf("got %q, want %q", got, "1h 0m")
	}
}

func TestFormatDuration_Hours_WithMinutes(t *testing.T) {
	// 3780000ms = 1h 3m
	got := formatDuration(3780000)
	if got != "1h 3m" {
		t.Fatalf("got %q, want %q", got, "1h 3m")
	}
}

func TestFormatDuration_Hours_TwoHours(t *testing.T) {
	got := formatDuration(7200000)
	if got != "2h 0m" {
		t.Fatalf("got %q, want %q", got, "2h 0m")
	}
}

func TestFormatDuration_Hours_Large(t *testing.T) {
	// 90060000ms = 25h 1m
	got := formatDuration(90060000)
	if got != "25h 1m" {
		t.Fatalf("got %q, want %q", got, "25h 1m")
	}
}

func TestFormatDuration_Hours_SubMinuteTruncated(t *testing.T) {
	// 3630000ms = 1h 0m 30s → shows "1h 0m" (seconds truncated)
	got := formatDuration(3630000)
	if got != "1h 0m" {
		t.Fatalf("got %q, want %q", got, "1h 0m")
	}
}
