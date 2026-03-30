package server

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/sayHello", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, `task-content-sayHello`) {
		t.Errorf("expected task-content-sayHello div, got:\n%s", event)
	}
	if !strings.Contains(event, "task-table-row") {
		t.Errorf("expected task-table-row content from renderTaskTable, got:\n%s", event)
	}
	if !strings.Contains(event, "event: datastar-patch-elements") {
		t.Errorf("expected SSE format, got:\n%s", event)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
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

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/NONEXISTENT", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, `task-content-NONEXISTENT`) {
		t.Errorf("expected task-content-NONEXISTENT div, got:\n%s", event)
	}
	if strings.Contains(event, "task-table-row") {
		t.Errorf("expected no task-table-row for unknown process, got:\n%s", event)
	}
	if strings.Contains(event, "task-pagination") {
		t.Errorf("expected no pagination controls for unknown process, got:\n%s", event)
	}
}

func TestHandleTaskPanel_UnknownRun_Returns404(t *testing.T) {
	store := state.NewStore()
	s := serverForSSE(store)

	req := httptest.NewRequest("GET", "/sse/run/no-such-run/tasks/sayHello", nil)
	req.SetPathValue("id", "no-such-run")
	req.SetPathValue("process", "sayHello")
	rec := httptest.NewRecorder()

	s.handleTaskPanel(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTaskPanel_PersistentSSE(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/sayHello", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Read initial event
	event1 := readSSEEvent(t, scanner, 2*time.Second)
	if !strings.Contains(event1, "task-table-row") {
		t.Errorf("initial event should contain task-table-row, got:\n%s", event1)
	}

	// Add a new task and trigger broker update
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "COMPLETED"},
	})
	s.runBroker.Publish("run-1", "trigger")

	// Read second event — should now contain both tasks.
	// renderTaskTable strips process prefix, so task "sayHello (2)" renders as "(2)".
	// We verify task 2 is present by checking its expandedTask reference.
	event2 := readSSEEvent(t, scanner, 2*time.Second)
	if !strings.Contains(event2, "$expandedTask === 2") {
		t.Errorf("second event should contain task 2, got:\n%s", event2)
	}
}

func TestHandleTaskPanel_DefaultPage(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 10 {
		t.Errorf("expected 10 task-table-row on default page, got %d", rowCount)
	}
}

func TestHandleTaskPanel_Page2(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 5 {
		t.Errorf("expected 5 task-table-row on page 2 of 15, got %d", rowCount)
	}
}

func TestHandleTaskPanel_LastPage(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 17; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 7 {
		t.Errorf("expected 7 task-table-row on last page of 17, got %d", rowCount)
	}
}

func TestHandleTaskPanel_PageOutOfRange(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// page=999 should clamp to last page (page 2 with 5 tasks)
	ctx999, cancel999 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel999()
	req999, _ := http.NewRequestWithContext(ctx999, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=999", nil)
	resp999, err := http.DefaultClient.Do(req999)
	if err != nil {
		t.Fatal(err)
	}
	defer resp999.Body.Close()
	scanner999 := bufio.NewScanner(resp999.Body)
	event999 := readSSEEvent(t, scanner999, 2*time.Second)
	rowCount := strings.Count(event999, "task-table-row")
	if rowCount != 5 {
		t.Errorf("page=999: expected 5 task-table-row (clamped to last page), got %d", rowCount)
	}

	// page=0 should clamp to page 1 (10 tasks)
	ctx0, cancel0 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel0()
	req0, _ := http.NewRequestWithContext(ctx0, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=0", nil)
	resp0, err := http.DefaultClient.Do(req0)
	if err != nil {
		t.Fatal(err)
	}
	defer resp0.Body.Close()
	scanner0 := bufio.NewScanner(resp0.Body)
	event0 := readSSEEvent(t, scanner0, 2*time.Second)
	rowCount0 := strings.Count(event0, "task-table-row")
	if rowCount0 != 10 {
		t.Errorf("page=0: expected 10 task-table-row (clamped to page 1), got %d", rowCount0)
	}

	// page=-1 should clamp to page 1 (10 tasks)
	ctxNeg, cancelNeg := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelNeg()
	reqNeg, _ := http.NewRequestWithContext(ctxNeg, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=-1", nil)
	respNeg, err := http.DefaultClient.Do(reqNeg)
	if err != nil {
		t.Fatal(err)
	}
	defer respNeg.Body.Close()
	scannerNeg := bufio.NewScanner(respNeg.Body)
	eventNeg := readSSEEvent(t, scannerNeg, 2*time.Second)
	rowCountNeg := strings.Count(eventNeg, "task-table-row")
	if rowCountNeg != 10 {
		t.Errorf("page=-1: expected 10 task-table-row (clamped to page 1), got %d", rowCountNeg)
	}
}

func TestHandleTaskPanel_PaginationControls(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// Check info text
	if !strings.Contains(event, "1\xe2\x80\x9310 of 15") && !strings.Contains(event, "1–10 of 15") {
		t.Errorf("expected 'Showing 1–10 of 15' text, got:\n%s", event)
	}

	// Check pagination controls present
	if !strings.Contains(event, "task-pagination") {
		t.Errorf("expected task-pagination div, got:\n%s", event)
	}

	// Next and last buttons should be present and active
	if !strings.Contains(event, "page=2") {
		t.Errorf("expected next/last button with page=2, got:\n%s", event)
	}

	// First and prev buttons should be disabled on page 1
	if !strings.Contains(event, "disabled") {
		t.Errorf("expected disabled first/prev buttons on page 1, got:\n%s", event)
	}
}

func TestHandleTaskPanel_SmallGroup_NoPagination(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 5; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// All 5 tasks should be shown
	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 5 {
		t.Errorf("expected 5 task-table-row for small group, got %d", rowCount)
	}

	// No pagination controls
	if strings.Contains(event, "btn-page") {
		t.Errorf("expected no pagination buttons for small group, got:\n%s", event)
	}
	if strings.Contains(event, "task-pagination") {
		t.Errorf("expected no task-pagination div for small group, got:\n%s", event)
	}
}
