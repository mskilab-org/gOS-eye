package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderResumeCommand_NilRun(t *testing.T) {
	got := renderResumeCommand(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil run, got %q", got)
	}
}

func TestRenderResumeCommand_RunningStatus(t *testing.T) {
	run := &state.Run{
		Status:      "running",
		SessionID:   "abc-123",
		ProjectName: "nf-core/rnaseq",
		WorkDir:     "/work",
	}
	got := renderResumeCommand(run)
	if got != "" {
		t.Fatalf("expected empty for running status, got %q", got)
	}
}

func TestRenderResumeCommand_EmptyStatus(t *testing.T) {
	run := &state.Run{
		Status:      "",
		SessionID:   "abc-123",
		ProjectName: "nf-core/rnaseq",
		WorkDir:     "/work",
	}
	got := renderResumeCommand(run)
	if got != "" {
		t.Fatalf("expected empty for empty status, got %q", got)
	}
}

func TestRenderResumeCommand_EmptySessionID(t *testing.T) {
	run := &state.Run{
		Status:      "failed",
		SessionID:   "",
		ProjectName: "nf-core/rnaseq",
		WorkDir:     "/work",
	}
	got := renderResumeCommand(run)
	if got != "" {
		t.Fatalf("expected empty for empty SessionID, got %q", got)
	}
}

func TestRenderResumeCommand_CompletedStatus(t *testing.T) {
	run := &state.Run{
		Status:      "completed",
		SessionID:   "sess-456",
		ProjectName: "nf-core/rnaseq",
		WorkDir:     "/work",
	}
	got := renderResumeCommand(run)
	if got == "" {
		t.Fatal("expected non-empty for completed run with SessionID")
	}
	if !strings.Contains(got, `<div class="resume-command"`) {
		t.Error("missing resume-command wrapper div")
	}
	if !strings.Contains(got, `<pre id="resume-cmd" class="resume-cmd-text">`) {
		t.Error("missing pre element with id and class")
	}
	if !strings.Contains(got, `class="btn-copy"`) {
		t.Error("missing btn-copy class on button")
	}
	if !strings.Contains(got, `data-on:click`) {
		t.Error("missing data-on:click attribute (Datastar v1 colon syntax)")
	}
	if strings.Contains(got, `data-on-click=`) {
		t.Error("should use data-on:click (colon), not data-on-click (hyphen)")
	}
	if !strings.Contains(got, "nf-core/rnaseq") {
		t.Error("pre content should contain the resume command text")
	}
	if !strings.Contains(got, "sess-456") {
		t.Error("pre content should contain the session ID")
	}
	// Collapsible: starts collapsed
	if !strings.Contains(got, `data-signals:_show-resume="false"`) {
		t.Error("resume command should have _show-resume signal defaulting to false")
	}
	if !strings.Contains(got, `data-show="$_showResume"`) {
		t.Error("resume command body should use data-show for collapsing")
	}
}

func TestRenderResumeCommand_FailedStatus(t *testing.T) {
	run := &state.Run{
		Status:      "failed",
		SessionID:   "sess-789",
		ProjectName: "nf-core/sarek",
		WorkDir:     "/scratch/work",
		Params:      map[string]any{"input": "samples.csv"},
	}
	got := renderResumeCommand(run)
	if got == "" {
		t.Fatal("expected non-empty for failed run with SessionID")
	}
	if !strings.Contains(got, `<div class="resume-command"`) {
		t.Error("missing resume-command wrapper div")
	}
	if !strings.Contains(got, `Resume Command`) {
		t.Error("missing 'Resume Command' label")
	}
	if !strings.Contains(got, `class="detail-label"`) {
		t.Error("missing detail-label class on label span")
	}
	if !strings.Contains(got, "nf-core/sarek") {
		t.Error("pre content should contain project name")
	}
	if !strings.Contains(got, "sess-789") {
		t.Error("pre content should contain session ID")
	}
}

func TestRenderResumeCommand_HTMLEscaping(t *testing.T) {
	run := &state.Run{
		Status:      "failed",
		SessionID:   "sess-esc",
		ProjectName: "nf-core/test",
		WorkDir:     "/work",
		Params:      map[string]any{"filter": "a<b & c>d"},
	}
	got := renderResumeCommand(run)
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	// The command text inside <pre> should be HTML-escaped
	if strings.Contains(got, "a<b & c>d") {
		t.Error("param value with < and & should be HTML-escaped in pre content")
	}
	if !strings.Contains(got, "&amp;") || !strings.Contains(got, "&lt;") {
		t.Error("expected HTML entities for & and < in pre content")
	}
}

func TestRenderResumeCommand_CopyButtonClipboard(t *testing.T) {
	run := &state.Run{
		Status:      "completed",
		SessionID:   "sess-copy",
		ProjectName: "nf-core/rnaseq",
		WorkDir:     "/work",
	}
	got := renderResumeCommand(run)
	if !strings.Contains(got, `copyText(evt.target`) {
		t.Error("copy button should use copyText helper")
	}
	if !strings.Contains(got, `document.getElementById(&#39;resume-cmd&#39;)`) &&
		!strings.Contains(got, `document.getElementById('resume-cmd')`) {
		t.Error("copy button should reference resume-cmd element by ID")
	}
}

func TestRenderResumeCommand_HeaderStructure(t *testing.T) {
	run := &state.Run{
		Status:      "failed",
		SessionID:   "sess-hdr",
		ProjectName: "nf-core/rnaseq",
		WorkDir:     "/work",
	}
	got := renderResumeCommand(run)
	if !strings.Contains(got, `<div class="resume-command-header"`) {
		t.Error("missing resume-command-header div")
	}
	if !strings.Contains(got, `<span class="detail-label">Resume Command</span>`) {
		t.Error("missing label span with exact text")
	}
	if !strings.Contains(got, `<button class="btn-copy"`) {
		t.Error("missing button with btn-copy class")
	}
	if !strings.Contains(got, `>Copy</button>`) {
		t.Error("button should contain 'Copy' text")
	}
}
