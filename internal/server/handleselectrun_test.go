package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestHandleSelectRun_SetsSSEHeaders(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverForSSE(store)

	req := httptest.NewRequest("GET", "/select-run/r1", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()

	s.handleSelectRun(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want \"text/event-stream\"", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want \"no-cache\"", cc)
	}
}

func TestHandleSelectRun_RunNotFound(t *testing.T) {
	s := serverForSSE(state.NewStore())

	req := httptest.NewRequest("GET", "/select-run/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()

	s.handleSelectRun(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<div id="dashboard">`) {
		t.Errorf("expected dashboard div in error response, got:\n%s", body)
	}
	if !strings.Contains(body, `Run not found`) {
		t.Errorf("expected 'Run not found' error message, got:\n%s", body)
	}
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE fragment event, got:\n%s", body)
	}
}

func TestHandleSelectRun_SendsDashboardWithDataInit(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "abc123", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverForSSE(store)

	req := httptest.NewRequest("GET", "/select-run/abc123", nil)
	req.SetPathValue("id", "abc123")
	rec := httptest.NewRecorder()

	s.handleSelectRun(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<div id="dashboard" data-init="@get('/sse/run/abc123')">`) {
		t.Errorf("expected dashboard div with data-init for run abc123, got:\n%s", body)
	}
}

func TestHandleSelectRun_SendsSelectedRunSignal(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r42", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverForSSE(store)

	req := httptest.NewRequest("GET", "/select-run/r42", nil)
	req.SetPathValue("id", "r42")
	rec := httptest.NewRecorder()

	s.handleSelectRun(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "event: datastar-patch-signals") {
		t.Errorf("expected datastar-patch-signals event, got:\n%s", body)
	}
	if !strings.Contains(body, `selectedRun`) {
		t.Errorf("expected selectedRun in signals, got:\n%s", body)
	}
	if !strings.Contains(body, `r42`) {
		t.Errorf("expected run ID r42 in signals, got:\n%s", body)
	}
}

func TestHandleSelectRun_ContainsRunDetail(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{ProjectName: "my-pipeline"},
		},
	})
	s := serverForSSE(store)

	req := httptest.NewRequest("GET", "/select-run/r1", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()

	s.handleSelectRun(rec, req)

	body := rec.Body.String()
	// renderRunDetail includes the pipeline name in an h1
	if !strings.Contains(body, "my-pipeline") {
		t.Errorf("expected pipeline name 'my-pipeline' in response, got:\n%s", body)
	}
}

func TestHandleSelectRun_IsOneShot(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverForSSE(store)

	req := httptest.NewRequest("GET", "/select-run/r1", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()

	// If handler blocks (non-one-shot), this would hang.
	// httptest.NewRecorder completes only for handlers that return.
	s.handleSelectRun(rec, req)

	body := rec.Body.String()
	if len(body) == 0 {
		t.Fatal("expected non-empty response body")
	}
	// Verify the response is finite and contains both events
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Error("missing elements event in one-shot response")
	}
	if !strings.Contains(body, "event: datastar-patch-signals") {
		t.Error("missing signals event in one-shot response")
	}
}
