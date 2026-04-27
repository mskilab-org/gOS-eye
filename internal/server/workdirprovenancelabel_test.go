package server

import (
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestWorkdirProvenanceLabel_KnownValues(t *testing.T) {
	// These strings are shown directly in task detail rows, so keep them concise,
	// static, and understandable without exposing raw enum names.
	tests := []struct {
		name       string
		provenance state.WorkdirProvenance
		want       string
	}{
		{"live webhook", state.WorkdirProvenanceLiveWebhook, "Live webhook"},
		{"explicit trace workdir", state.WorkdirProvenanceTraceWorkdir, "Explicit trace workdir"},
		{"hash resolved", state.WorkdirProvenanceHashResolved, "Hash-resolved workdir"},
		{"unresolved", state.WorkdirProvenanceUnresolved, "Unresolved workdir"},
		{"ambiguous hash", state.WorkdirProvenanceAmbiguousHash, "Ambiguous workdir hash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workdirProvenanceLabel(tt.provenance)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkdirProvenanceLabel_UnknownAndEmpty(t *testing.T) {
	tests := []struct {
		name       string
		provenance state.WorkdirProvenance
	}{
		{"empty", state.WorkdirProvenanceUnknown},
		{"future value", state.WorkdirProvenance("future_value")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workdirProvenanceLabel(tt.provenance)
			if got != "" {
				t.Fatalf("got %q, want empty string", got)
			}
		})
	}
}
