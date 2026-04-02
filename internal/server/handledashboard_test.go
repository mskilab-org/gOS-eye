package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestHandleDashboard_ReturnsHTML(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{ProjectName: "my-pipeline"},
		},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
	})

	s := &Server{store: store, layouts: make(map[string]*dag.Layout)}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/r1", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()

	s.handleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q; want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "my-pipeline") {
		t.Errorf("expected pipeline name in body, got:\n%s", body)
	}
	if !strings.Contains(body, "align") {
		t.Errorf("expected process name 'align' in body, got:\n%s", body)
	}
	// Should contain a progress bar
	if !strings.Contains(body, "progress-bar") {
		t.Errorf("expected progress bar in body, got:\n%s", body)
	}
}

func TestHandleDashboard_UnknownRun_404(t *testing.T) {
	s := &Server{store: state.NewStore(), layouts: make(map[string]*dag.Layout)}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()

	s.handleDashboard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
}

func TestHandleDashboard_ContainsProcessTable(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r2", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r2", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})

	s := &Server{store: store, layouts: make(map[string]*dag.Layout)}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/r2", nil)
	req.SetPathValue("id", "r2")
	rec := httptest.NewRecorder()

	s.handleDashboard(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "process-table") {
		t.Errorf("expected process-table div in body, got:\n%s", body)
	}
}

func TestHandleDashboard_ContainsDashboardID(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "r3", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})

	s := &Server{store: store, layouts: make(map[string]*dag.Layout)}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/r3", nil)
	req.SetPathValue("id", "r3")
	rec := httptest.NewRecorder()

	s.handleDashboard(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id="dashboard"`) {
		t.Errorf("expected <div id=\"dashboard\"> wrapper, got:\n%s", body)
	}
}
