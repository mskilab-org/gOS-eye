package server

import (
	"html"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderTaskProvenanceDetailRows_NilTask(t *testing.T) {
	got := renderTaskProvenanceDetailRows(nil)
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestRenderTaskProvenanceDetailRows_UnknownAndEmptyMetadata(t *testing.T) {
	task := &state.Task{
		Source:            state.TaskSource("future_source"),
		WorkdirProvenance: state.WorkdirProvenance("future_provenance"),
	}

	got := renderTaskProvenanceDetailRows(task)
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestRenderTaskProvenanceDetailRows_RendersKnownSourceAndProvenance(t *testing.T) {
	task := &state.Task{
		Source:            state.TaskSourceCachedTrace,
		WorkdirProvenance: state.WorkdirProvenanceHashResolved,
	}

	got := renderTaskProvenanceDetailRows(task)
	want := `<span class="detail-label">Task Source</span><span class="detail-value">Cached trace import</span>` +
		`<span class="detail-label">Workdir Source</span><span class="detail-value">Hash-resolved workdir</span>`

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderTaskProvenanceDetailRows_RendersEscapedWarningOnly(t *testing.T) {
	warning := `cached workdir unresolved: cannot scan "<missing>" & gave up`
	task := &state.Task{WorkdirWarning: warning}

	got := renderTaskProvenanceDetailRows(task)

	if !strings.Contains(got, `<span class="detail-label">Workdir Warning</span>`) {
		t.Fatalf("missing Workdir Warning label in %q", got)
	}
	if !strings.Contains(got, html.EscapeString(warning)) {
		t.Fatalf("missing escaped warning text in %q", got)
	}
	if strings.Contains(got, warning) || strings.Contains(got, `"<missing>"`) {
		t.Fatalf("warning text was not escaped defensively: %q", got)
	}
	if strings.Contains(got, "Task Source") || strings.Contains(got, "Workdir Source") {
		t.Fatalf("unexpected provenance rows for warning-only task: %q", got)
	}
}

func TestRenderTaskProvenanceDetailRows_RendersAllRowsInDetailGridOrder(t *testing.T) {
	task := &state.Task{
		Source:            state.TaskSourceCachedTrace,
		WorkdirProvenance: state.WorkdirProvenanceAmbiguousHash,
		WorkdirWarning:    `cached workdir ambiguous: 2 matches`,
	}

	got := renderTaskProvenanceDetailRows(task)
	want := `<span class="detail-label">Task Source</span><span class="detail-value">Cached trace import</span>` +
		`<span class="detail-label">Workdir Source</span><span class="detail-value">Ambiguous workdir hash</span>` +
		`<span class="detail-label">Workdir Warning</span><span class="detail-value">cached workdir ambiguous: 2 matches</span>`

	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if strings.Count(got, `<span class="detail-label">`) != 3 || strings.Count(got, `<span class="detail-value">`) != 3 {
		t.Fatalf("expected alternating detail-label/detail-value spans, got %q", got)
	}
}
