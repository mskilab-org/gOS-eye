package server

import (
	"strings"
	"testing"
)

func TestFormatSSESignals_SimpleJSON(t *testing.T) {
	got := formatSSESignals(`{"startTime": 1234567890}`)
	want := "event: datastar-patch-signals\ndata: signals {\"startTime\": 1234567890}\n\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatSSESignals_EmptyJSON(t *testing.T) {
	got := formatSSESignals(`{}`)
	want := "event: datastar-patch-signals\ndata: signals {}\n\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatSSESignals_StartsWithEventLine(t *testing.T) {
	got := formatSSESignals(`{"x":1}`)
	if !strings.HasPrefix(got, "event: datastar-patch-signals\n") {
		t.Fatalf("output does not start with event line:\n%q", got)
	}
}

func TestFormatSSESignals_DataHasSignalsPrefix(t *testing.T) {
	got := formatSSESignals(`{"x":1}`)
	lines := strings.Split(got, "\n")
	// lines[0] = event line, lines[1] = data line, lines[2] = "", lines[3] = ""
	if !strings.HasPrefix(lines[1], "data: signals ") {
		t.Fatalf("data line missing 'data: signals ' prefix: %q", lines[1])
	}
}

func TestFormatSSESignals_EndsWithDoubleNewline(t *testing.T) {
	got := formatSSESignals(`{"startTime": 1234567890}`)
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("output does not end with double newline:\n%q", got)
	}
}
