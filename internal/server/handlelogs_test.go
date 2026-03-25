package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// helper: build a Server for handleLogs tests
func serverForLogs(store *state.Store) *Server {
	return &Server{
		store:  store,
		broker: NewBroker(),
	}
}

// runHandleLogs runs handleLogs in a goroutine with a cancelable context,
// waits briefly for the handler to write its initial response, then cancels.
// Returns the recorded response body.
func runHandleLogs(t *testing.T, s *Server, url string) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, url, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		s.handleLogs(rec, req)
		close(done)
	}()

	// Give the handler time to write its initial response
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleLogs did not return after context cancel")
	}

	return rec.Body.String()
}

func TestHandleLogs_MethodNotAllowed(t *testing.T) {
	s := serverForLogs(state.NewStore())
	req := httptest.NewRequest(http.MethodPost, "/logs?run=r1&task=1", nil)
	rec := httptest.NewRecorder()

	s.handleLogs(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /logs: status = %d; want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleLogs_MissingQueryParams(t *testing.T) {
	s := serverForLogs(state.NewStore())
	body := runHandleLogs(t, s, "/logs")

	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE event format, got:\n%s", body)
	}
	if !strings.Contains(body, "missing") {
		t.Errorf("expected error about missing query params, got:\n%s", body)
	}
}

func TestHandleLogs_RunNotFound(t *testing.T) {
	s := serverForLogs(state.NewStore())
	body := runHandleLogs(t, s, "/logs?run=nonexistent&task=1")

	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE event format, got:\n%s", body)
	}
	if !strings.Contains(body, "not found") {
		t.Errorf("expected 'not found' error message, got:\n%s", body)
	}
}

func TestHandleLogs_TaskNotFound(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "process_submitted",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "align (1)",
			Process: "align",
			Status:  "SUBMITTED",
		},
	})
	s := serverForLogs(store)
	body := runHandleLogs(t, s, "/logs?run=r1&task=99")

	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE event format, got:\n%s", body)
	}
	if !strings.Contains(body, "not found") {
		t.Errorf("expected 'not found' error message, got:\n%s", body)
	}
}

func TestHandleLogs_TaskFoundWithLogFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .command.log and .command.err files
	if err := os.WriteFile(filepath.Join(tmpDir, ".command.log"), []byte("hello stdout"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".command.err"), []byte("hello stderr"), 0644); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "align (1)",
			Process: "align",
			Status:  "COMPLETED",
			Workdir: tmpDir,
		},
	})
	s := serverForLogs(store)
	body := runHandleLogs(t, s, "/logs?run=r1&task=1")

	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE event format, got:\n%s", body)
	}
	if !strings.Contains(body, "hello stdout") {
		t.Errorf("expected stdout content in response, got:\n%s", body)
	}
	if !strings.Contains(body, "hello stderr") {
		t.Errorf("expected stderr content in response, got:\n%s", body)
	}
	if !strings.Contains(body, "align (1)") {
		t.Errorf("expected task name in response, got:\n%s", body)
	}
	// Check that the _logOpen signal is sent
	if !strings.Contains(body, "event: datastar-patch-signals") {
		t.Errorf("expected SSE signals event, got:\n%s", body)
	}
	if !strings.Contains(body, "_logOpen") {
		t.Errorf("expected _logOpen signal, got:\n%s", body)
	}
}

func TestHandleLogs_TaskFoundNoLogFiles(t *testing.T) {
	// Use a temp dir that exists but has no log files
	tmpDir := t.TempDir()

	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "align (1)",
			Process: "align",
			Status:  "COMPLETED",
			Workdir: tmpDir,
		},
	})
	s := serverForLogs(store)
	body := runHandleLogs(t, s, "/logs?run=r1&task=1")

	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Errorf("expected SSE event format, got:\n%s", body)
	}
	if !strings.Contains(body, "(empty)") {
		t.Errorf("expected '(empty)' for missing log content, got:\n%s", body)
	}
}
