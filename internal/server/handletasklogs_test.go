package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestHandleTaskLogs_RunNotFound(t *testing.T) {
	s := &Server{store: state.NewStore(), runBroker: NewRunBroker()}
	req := httptest.NewRequest("GET", "/sse/task/missing/1/logs", nil)
	req.SetPathValue("run", "missing")
	req.SetPathValue("task", "1")
	rec := httptest.NewRecorder()

	s.handleTaskLogs(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTaskLogs_TaskNotFound(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "r", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/task/run1/999/logs", nil)
	req.SetPathValue("run", "run1")
	req.SetPathValue("task", "999")
	rec := httptest.NewRecorder()

	s.handleTaskLogs(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTaskLogs_InvalidTaskID(t *testing.T) {
	s := &Server{store: state.NewStore(), runBroker: NewRunBroker()}
	req := httptest.NewRequest("GET", "/sse/task/run1/abc/logs", nil)
	req.SetPathValue("run", "run1")
	req.SetPathValue("task", "abc")
	rec := httptest.NewRecorder()

	s.handleTaskLogs(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleTaskLogs_NoWorkdir(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "r", RunID: "run1", Event: "process_submitted",
		Trace: &state.Trace{TaskID: 1, Name: "proc (1)", Process: "proc", Status: "SUBMITTED"},
	})
	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/task/run1/1/logs", nil)
	req.SetPathValue("run", "run1")
	req.SetPathValue("task", "1")
	rec := httptest.NewRecorder()

	s.handleTaskLogs(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id="task-logs-1"`) {
		t.Errorf("expected task-logs-1 div, got:\n%s", body)
	}
	if !strings.Contains(body, "workdir not set") {
		t.Errorf("expected workdir error message, got:\n%s", body)
	}
}

func TestHandleTaskLogs_WithLogFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".command.log"), []byte("hello stdout\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".command.err"), []byte("hello stderr\n"), 0644)

	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "r", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 5, Name: "proc (1)", Process: "proc", Status: "COMPLETED", Workdir: dir},
	})
	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/task/run1/5/logs", nil)
	req.SetPathValue("run", "run1")
	req.SetPathValue("task", "5")
	rec := httptest.NewRecorder()

	s.handleTaskLogs(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id="task-logs-5"`) {
		t.Errorf("expected task-logs-5 div, got:\n%s", body)
	}
	if !strings.Contains(body, "hello stdout") {
		t.Errorf("expected stdout content, got:\n%s", body)
	}
	if !strings.Contains(body, "hello stderr") {
		t.Errorf("expected stderr content, got:\n%s", body)
	}
	if !strings.Contains(body, ".command.log") {
		t.Errorf("expected .command.log label, got:\n%s", body)
	}
	if !strings.Contains(body, ".command.err") {
		t.Errorf("expected .command.err label, got:\n%s", body)
	}
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE format, got:\n%s", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestHandleTaskLogs_MissingLogFiles(t *testing.T) {
	dir := t.TempDir() // empty dir, no log files

	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "r", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 3, Name: "proc (1)", Process: "proc", Status: "COMPLETED", Workdir: dir},
	})
	s := &Server{store: store, runBroker: NewRunBroker()}

	req := httptest.NewRequest("GET", "/sse/task/run1/3/logs", nil)
	req.SetPathValue("run", "run1")
	req.SetPathValue("task", "3")
	rec := httptest.NewRecorder()

	s.handleTaskLogs(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id="task-logs-3"`) {
		t.Errorf("expected task-logs-3 div, got:\n%s", body)
	}
	if !strings.Contains(body, "(no log file)") {
		t.Errorf("expected '(no log file)' placeholder, got:\n%s", body)
	}
}
