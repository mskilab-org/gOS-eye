package server

import (
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestTaskSourceLabel_LiveWebhook(t *testing.T) {
	got := taskSourceLabel(state.TaskSourceLiveWebhook)
	want := "Live webhook"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTaskSourceLabel_CachedTrace(t *testing.T) {
	got := taskSourceLabel(state.TaskSourceCachedTrace)
	want := "Cached trace import"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTaskSourceLabel_Unknown(t *testing.T) {
	got := taskSourceLabel(state.TaskSource("future_source"))
	want := ""
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTaskSourceLabel_Empty(t *testing.T) {
	got := taskSourceLabel(state.TaskSourceUnknown)
	want := ""
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
