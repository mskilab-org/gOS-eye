package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestHandleTaskPanel_ReturnsTaskTable(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "COMPLETED"},
	})

	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/run/run-1/tasks/sayHello", nil)
	req.SetPathValue("id", "run-1")
	req.SetPathValue("process", "sayHello")
	rec := httptest.NewRecorder()

	s.handleTaskPanel(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `task-panel-sayHello`) {
		t.Errorf("expected task-panel-sayHello div, got:\n%s", body)
	}
	if !strings.Contains(body, "task-table-row") {
		t.Errorf("expected task-table-row content from renderTaskTable, got:\n%s", body)
	}
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE format, got:\n%s", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestHandleTaskPanel_UnknownProcess_ReturnsEmpty(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})

	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/run/run-1/tasks/NONEXISTENT", nil)
	req.SetPathValue("id", "run-1")
	req.SetPathValue("process", "NONEXISTENT")
	rec := httptest.NewRecorder()

	s.handleTaskPanel(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `task-panel-NONEXISTENT`) {
		t.Errorf("expected task-panel-NONEXISTENT div, got:\n%s", body)
	}
	if strings.Contains(body, "task-table-row") {
		t.Errorf("expected no task-table-row for unknown process, got:\n%s", body)
	}
}

func TestHandleTaskPanel_UnknownRun_Returns404(t *testing.T) {
	store := state.NewStore()
	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/run/no-such-run/tasks/sayHello", nil)
	req.SetPathValue("id", "no-such-run")
	req.SetPathValue("process", "sayHello")
	rec := httptest.NewRecorder()

	s.handleTaskPanel(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
