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

func TestHandleTaskPageNav_ReplacesWrapper(t *testing.T) {
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
	mux.HandleFunc("/run/{id}/tasks/{process}/page", s.handleTaskPageNav)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/run/run-1/tasks/proc/page?page=2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// Response should use replace mode (not default morph).
	if !strings.Contains(event, "data: mode replace") {
		t.Errorf("expected replace mode in SSE event, got:\n%s", event)
	}

	// Response targets the outer task-panel wrapper (not task-content).
	if !strings.Contains(event, `id="task-panel-proc"`) {
		t.Errorf("expected outer task-panel-proc wrapper, got:\n%s", event)
	}

	// Wrapper has data-ignore-morph and data-init pointing to page 2.
	if !strings.Contains(event, `data-ignore-morph`) {
		t.Errorf("expected data-ignore-morph on wrapper, got:\n%s", event)
	}
	if !strings.Contains(event, `@get('/sse/run/run-1/tasks/proc?page=2&gen=1')`) {
		t.Errorf("expected data-init with page=2 and gen URL, got:\n%s", event)
	}

	// Inner content should have page 2 tasks (5 rows for tasks 11-15).
	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 5 {
		t.Errorf("expected 5 task-table-row on page 2, got %d", rowCount)
	}

	// Connection should close (one-shot) — reading another event should fail/timeout.
	// The handler returns after sending, so the body should be at EOF.
}

func TestHandleTaskPageNav_PaginationButtonsUseOneShot(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 25; i++ {
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
	mux.HandleFunc("/run/{id}/tasks/{process}/page", s.handleTaskPageNav)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Request page 2 — buttons should point to the one-shot /run/.../page endpoint.
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/run/run-1/tasks/proc/page?page=2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// Pagination buttons should use the one-shot /run/.../page route, NOT /sse/run/.../tasks.
	// (The outer wrapper's data-init correctly uses /sse/run/... for the persistent stream.)
	if strings.Contains(event, `btn-page" data-on:click="@get('/sse/run/`) {
		t.Errorf("pagination buttons should NOT use persistent /sse/ endpoint, got:\n%s", event)
	}
	// First page button should point to /run/.../page?page=1
	if !strings.Contains(event, `/run/run-1/tasks/proc/page?page=1`) {
		t.Errorf("expected first-page button with /run/.../page?page=1, got:\n%s", event)
	}
}

func TestHandleTaskPanel_NewConnectionKillsOld(t *testing.T) {
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
	mux.HandleFunc("/run/{id}/tasks/{process}/page", s.handleTaskPageNav)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Open persistent connection for page 1 (simulates data-init).
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	req1, _ := http.NewRequestWithContext(ctx1, "GET", ts.URL+"/sse/run/run-1/tasks/proc", nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	scanner1 := bufio.NewScanner(resp1.Body)

	// Read initial page 1 event.
	event1 := readSSEEvent(t, scanner1, 2*time.Second)
	if !strings.Contains(event1, "task-table-row") {
		t.Fatalf("expected task rows in initial event, got:\n%s", event1)
	}

	// Simulate pagination: call one-shot endpoint to navigate to page 2.
	// This bumps the generation and kills the page-1 connection.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	req2, _ := http.NewRequestWithContext(ctx2, "GET", ts.URL+"/run/run-1/tasks/proc/page?page=2", nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	scanner2 := bufio.NewScanner(resp2.Body)
	navEvent := readSSEEvent(t, scanner2, 2*time.Second)
	if !strings.Contains(navEvent, `page=2&gen=1`) {
		t.Fatalf("expected page=2&gen=1 in nav response, got:\n%s", navEvent)
	}

	// Open persistent connection for page 2 with correct gen (simulates new data-init).
	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	req3, _ := http.NewRequestWithContext(ctx3, "GET", ts.URL+"/sse/run/run-1/tasks/proc?page=2&gen=1", nil)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	scanner3 := bufio.NewScanner(resp3.Body)

	// Read initial page 2 event.
	event3 := readSSEEvent(t, scanner3, 2*time.Second)
	rows := strings.Count(event3, "task-table-row")
	if rows != 5 {
		t.Fatalf("expected 5 rows (page 2 of 15), got %d", rows)
	}

	// Trigger a webhook update — only the page-2 connection should respond.
	s.runBroker.Publish("run-1", "trigger")
	event3b := readSSEEvent(t, scanner3, 2*time.Second)
	rows3b := strings.Count(event3b, "task-table-row")
	if rows3b != 5 {
		t.Errorf("page-2 connection should still show 5 rows after update, got %d", rows3b)
	}

	// The old page-1 connection should be closed — scanner1 should get no more events.
	// We verify by checking that the old connection's goroutine exited:
	// scanner1.Scan() should return false (EOF) quickly.
	done := make(chan bool, 1)
	go func() {
		got := scanner1.Scan()
		done <- got
	}()
	select {
	case got := <-done:
		if got {
			t.Errorf("old page-1 connection should be closed, but scanner1.Scan() returned true")
		}
	case <-time.After(2 * time.Second):
		t.Errorf("old page-1 connection should have been closed by now (timed out)")
	}
}

func TestHandleTaskPanel_StaleRetryRejected(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "proc (1)", Process: "proc", Status: "COMPLETED"},
	})

	s := serverForSSE(store)

	// Bump generation to 5 (simulates 5 page navigations).
	s.panelMu.Lock()
	s.panelGen["run-1/proc"] = 5
	s.panelMu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Try connecting with gen=2 (stale — less than current gen=5).
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?gen=2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Stale connection should be rejected — server returns immediately with no SSE events.
	// The response body should be empty or EOF.
	scanner := bufio.NewScanner(resp.Body)
	gotData := scanner.Scan()
	if gotData && strings.TrimSpace(scanner.Text()) != "" {
		t.Errorf("stale retry (gen=2 < current=5) should get no data, got: %q", scanner.Text())
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

// --- filterTasks unit tests ---

func makeTasks() []*state.Task {
	return []*state.Task{
		{Name: "PATIENT_0042 (1)", Tag: "sample_A", Status: "COMPLETED"},
		{Name: "PATIENT_0042 (2)", Tag: "sample_B", Status: "FAILED"},
		{Name: "align (1)", Tag: "genome_X", Status: "RUNNING"},
		{Name: "align (2)", Tag: "genome_Y", Status: "COMPLETED"},
		{Name: "merge (1)", Tag: "final", Status: "FAILED"},
	}
}

func TestFilterTasks_ByName(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "align", "")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks matching name 'align', got %d", len(got))
	}
	for _, tk := range got {
		if !strings.Contains(strings.ToLower(tk.Name), "align") {
			t.Errorf("task %q should contain 'align'", tk.Name)
		}
	}
}

func TestFilterTasks_ByTag(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "genome", "")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks matching tag 'genome', got %d", len(got))
	}
	for _, tk := range got {
		if !strings.Contains(strings.ToLower(tk.Tag), "genome") {
			t.Errorf("task tag %q should contain 'genome'", tk.Tag)
		}
	}
}

func TestFilterTasks_ByStatus(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "", "FAILED")
	if len(got) != 2 {
		t.Fatalf("expected 2 FAILED tasks, got %d", len(got))
	}
	for _, tk := range got {
		if tk.Status != "FAILED" {
			t.Errorf("expected FAILED status, got %q", tk.Status)
		}
	}
}

func TestFilterTasks_MultiStatus(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "", "FAILED,RUNNING")
	if len(got) != 3 {
		t.Fatalf("expected 3 tasks (2 FAILED + 1 RUNNING), got %d", len(got))
	}
	for _, tk := range got {
		if tk.Status != "FAILED" && tk.Status != "RUNNING" {
			t.Errorf("expected FAILED or RUNNING, got %q", tk.Status)
		}
	}
}

func TestFilterTasks_CaseInsensitive(t *testing.T) {
	tasks := makeTasks()
	// Lowercase query should match uppercase Name
	got := filterTasks(tasks, "patient", "")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks matching 'patient' (case-insensitive), got %d", len(got))
	}

	// Status filter should also be case-insensitive
	got2 := filterTasks(tasks, "", "failed")
	if len(got2) != 2 {
		t.Fatalf("expected 2 tasks with status 'failed' (case-insensitive), got %d", len(got2))
	}
}

func TestFilterTasks_EmptyQ(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "", "COMPLETED")
	if len(got) != 2 {
		t.Fatalf("expected 2 COMPLETED tasks when q is empty, got %d", len(got))
	}

	// Totally empty filters should return all
	all := filterTasks(tasks, "", "")
	if len(all) != len(tasks) {
		t.Fatalf("expected all %d tasks when both filters empty, got %d", len(tasks), len(all))
	}
}

func TestFilterTasks_EmptyStatus(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "merge", "")
	if len(got) != 1 {
		t.Fatalf("expected 1 task matching 'merge' with empty statusFilter, got %d", len(got))
	}
	if got[0].Name != "merge (1)" {
		t.Errorf("expected 'merge (1)', got %q", got[0].Name)
	}
}

func TestFilterTasks_BothFilters(t *testing.T) {
	tasks := makeTasks()
	// q="patient" matches 2 tasks, statusFilter="FAILED" matches 2 tasks,
	// intersection is 1: PATIENT_0042 (2) with Status=FAILED
	got := filterTasks(tasks, "patient", "FAILED")
	if len(got) != 1 {
		t.Fatalf("expected 1 task matching both filters, got %d", len(got))
	}
	if got[0].Name != "PATIENT_0042 (2)" {
		t.Errorf("expected 'PATIENT_0042 (2)', got %q", got[0].Name)
	}
	if got[0].Status != "FAILED" {
		t.Errorf("expected FAILED, got %q", got[0].Status)
	}
}

func TestFilterTasks_NoMatch(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "nonexistent", "")
	if len(got) != 0 {
		t.Fatalf("expected 0 tasks for 'nonexistent', got %d", len(got))
	}

	got2 := filterTasks(tasks, "", "CACHED")
	if len(got2) != 0 {
		t.Fatalf("expected 0 tasks for status 'CACHED', got %d", len(got2))
	}

	got3 := filterTasks(tasks, "align", "FAILED")
	if len(got3) != 0 {
		t.Fatalf("expected 0 tasks for 'align' + 'FAILED', got %d", len(got3))
	}
}

// --- Tests for renderTaskPanelHTML with filter/status support ---

// populateTaskPanel creates a store with a run and multiple tasks for the given process.
// Returns the store and the run ID.
func populateTaskPanel(process string, tasks []struct {
	id      int
	name    string
	tag     string
	status  string
}) *state.Store {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for _, tk := range tasks {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  tk.id,
				Name:    tk.name,
				Process: process,
				Tag:     tk.tag,
				Status:  tk.status,
			},
		})
	}
	return store
}

func TestRenderTaskPanelHTML_FilterApplied(t *testing.T) {
	store := populateTaskPanel("align", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "align (sample_alpha)", "sample_alpha", "COMPLETED"},
		{2, "align (sample_beta)", "sample_beta", "COMPLETED"},
		{3, "align (sample_alpha2)", "sample_alpha2", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "align", "alpha", "", 1)

	// "alpha" matches tasks 1 and 3 by tag, but not task 2 ("sample_beta").
	if strings.Contains(html, "sample_beta") {
		t.Error("expected 'sample_beta' task to be filtered out by q='alpha'")
	}
	if !strings.Contains(html, "sample_alpha)") {
		t.Error("expected 'sample_alpha' task to be present")
	}
	if !strings.Contains(html, "sample_alpha2)") {
		t.Error("expected 'sample_alpha2' task to be present")
	}
}

func TestRenderTaskPanelHTML_StatusFilterApplied(t *testing.T) {
	store := populateTaskPanel("proc", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "proc (item_one)", "item_one", "COMPLETED"},
		{2, "proc (item_two)", "item_two", "FAILED"},
		{3, "proc (item_three)", "item_three", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "proc", "", "FAILED", 1)

	// Only task 2 (FAILED) should appear. renderTaskTable strips process prefix,
	// so check for the parenthetical part.
	if !strings.Contains(html, "item_two") {
		t.Error("expected FAILED task with tag 'item_two' to be present")
	}
	if strings.Contains(html, "item_one") {
		t.Error("expected COMPLETED task 'item_one' to be filtered out")
	}
	if strings.Contains(html, "item_three") {
		t.Error("expected COMPLETED task 'item_three' to be filtered out")
	}
}

func TestRenderTaskPanelHTML_FilterAndPaginate(t *testing.T) {
	// Create 15 tasks matching filter and 5 not matching → 15 filtered,
	// page 1 should have 10 tasks, page 2 should have 5.
	var tasks []struct {
		id      int
		name    string
		tag     string
		status  string
	}
	for i := 1; i <= 15; i++ {
		tasks = append(tasks, struct {
			id      int
			name    string
			tag     string
			status  string
		}{i, fmt.Sprintf("proc (match_%02d)", i), "match", "COMPLETED"})
	}
	for i := 16; i <= 20; i++ {
		tasks = append(tasks, struct {
			id      int
			name    string
			tag     string
			status  string
		}{i, fmt.Sprintf("proc (other_%02d)", i), "other", "COMPLETED"})
	}
	store := populateTaskPanel("proc", tasks)
	s := serverForSSE(store)

	// Page 1 with q="match": 15 match → 10 on page 1.
	html1 := s.renderTaskPanelHTML("run-1", "proc", "match", "", 1)
	// Should contain pagination (15 > tasksPerPage).
	if !strings.Contains(html1, "task-pagination") {
		t.Error("expected pagination controls on page 1 of filtered results")
	}
	// Should NOT contain "other" tasks.
	if strings.Contains(html1, "other_") {
		t.Error("expected 'other' tasks to be filtered out")
	}

	// Page 2: remaining 5 tasks.
	html2 := s.renderTaskPanelHTML("run-1", "proc", "match", "", 2)
	if !strings.Contains(html2, "task-pagination") {
		t.Error("expected pagination controls on page 2")
	}
	// "11–15 of 15" should appear in the pagination info.
	if !strings.Contains(html2, "11–15 of 15") {
		t.Errorf("expected '11–15 of 15' pagination info on page 2, got:\n%s", html2)
	}
}

func TestRenderTaskPanelHTML_NoResults(t *testing.T) {
	store := populateTaskPanel("proc", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "proc (a)", "a", "COMPLETED"},
		{2, "proc (b)", "b", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "proc", "nonexistent", "", 1)

	if !strings.Contains(html, "No matching tasks") {
		t.Error("expected 'No matching tasks' message when filter matches nothing")
	}
	// Should NOT contain a task table.
	if strings.Contains(html, "task-table") {
		t.Error("expected no task-table when filter matches nothing")
	}
}

func TestRenderTaskPanelHTML_IncludesFilterBar(t *testing.T) {
	store := populateTaskPanel("sayHello", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "sayHello (1)", "tag1", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "sayHello", "", "", 1)

	if !strings.Contains(html, `task-filter-sayHello`) {
		t.Error("expected task-filter-bar div with process-specific ID")
	}
	if !strings.Contains(html, `task-filter-bar`) {
		t.Error("expected task-filter-bar class in output")
	}
}

func TestRenderTaskPanelHTML_ResultsDiv(t *testing.T) {
	store := populateTaskPanel("sayHello", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "sayHello (1)", "tag1", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "sayHello", "", "", 1)

	if !strings.Contains(html, `id="task-results-sayHello"`) {
		t.Errorf("expected task-results-sayHello div, got:\n%s", html)
	}
	// Outer wrapper should NOT have data-ignore-morph anymore.
	// Find the task-content div's opening tag.
	idx := strings.Index(html, `id="task-content-sayHello"`)
	if idx < 0 {
		t.Fatal("expected task-content-sayHello div")
	}
	// Extract the opening tag (from < to >).
	tagStart := strings.LastIndex(html[:idx], "<")
	tagEnd := strings.Index(html[idx:], ">") + idx
	openTag := html[tagStart : tagEnd+1]
	if strings.Contains(openTag, "data-ignore-morph") {
		t.Errorf("outer task-content div should NOT have data-ignore-morph, got: %s", openTag)
	}
}

// --- handleTaskPanel filter integration tests ---

// populateFilterTasks creates a store with varied task names and statuses for filter testing.
func populateFilterTasks() *state.Store {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	tasks := []struct {
		id     int
		name   string
		tag    string
		status string
	}{
		{1, "proc (PATIENT_001)", "PATIENT_001", "COMPLETED"},
		{2, "proc (PATIENT_002)", "PATIENT_002", "FAILED"},
		{3, "proc (PATIENT_003)", "PATIENT_003", "RUNNING"},
		{4, "proc (SAMPLE_A)", "SAMPLE_A", "COMPLETED"},
		{5, "proc (SAMPLE_B)", "SAMPLE_B", "FAILED"},
		{6, "proc (CONTROL_X)", "CONTROL_X", "COMPLETED"},
	}
	for _, tk := range tasks {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  tk.id,
				Name:    tk.name,
				Process: "proc",
				Tag:     tk.tag,
				Status:  tk.status,
			},
		})
	}
	return store
}

func TestHandleTaskPanel_FilterByName(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?q=PATIENT", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 3 {
		t.Errorf("expected 3 task-table-row for q=PATIENT, got %d", rowCount)
	}
	if !strings.Contains(event, "PATIENT_001") {
		t.Errorf("expected PATIENT_001 in results")
	}
	if strings.Contains(event, "SAMPLE_A") {
		t.Errorf("expected SAMPLE_A to be filtered out")
	}
	if strings.Contains(event, "CONTROL_X") {
		t.Errorf("expected CONTROL_X to be filtered out")
	}
}

func TestHandleTaskPanel_FilterByStatus(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?status=FAILED", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 2 {
		t.Errorf("expected 2 task-table-row for status=FAILED, got %d", rowCount)
	}
	if !strings.Contains(event, "PATIENT_002") {
		t.Errorf("expected PATIENT_002 (FAILED) in results")
	}
	if !strings.Contains(event, "SAMPLE_B") {
		t.Errorf("expected SAMPLE_B (FAILED) in results")
	}
	if strings.Contains(event, "PATIENT_001") {
		t.Errorf("expected PATIENT_001 (COMPLETED) to be filtered out")
	}
}

func TestHandleTaskPanel_FilterAndPaginate(t *testing.T) {
	// Create 15 PATIENT tasks + 5 SAMPLE tasks = 20 total.
	// Filtering by q=PATIENT gives 15 tasks → page 1 = 10, page 2 = 5.
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
				Name:    fmt.Sprintf("proc (PATIENT_%03d)", i),
				Process: "proc",
				Tag:     fmt.Sprintf("PATIENT_%03d", i),
				Status:  "COMPLETED",
			},
		})
	}
	for i := 16; i <= 20; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (SAMPLE_%03d)", i),
				Process: "proc",
				Tag:     fmt.Sprintf("SAMPLE_%03d", i),
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Page 1 of filtered set: 10 rows
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	req1, _ := http.NewRequestWithContext(ctx1, "GET", ts.URL+"/sse/run/run-1/tasks/proc?q=PATIENT&page=1", nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()

	scanner1 := bufio.NewScanner(resp1.Body)
	event1 := readSSEEvent(t, scanner1, 2*time.Second)

	rowCount1 := strings.Count(event1, "task-table-row")
	if rowCount1 != 10 {
		t.Errorf("expected 10 rows on page 1 of filtered set, got %d", rowCount1)
	}
	if strings.Contains(event1, "SAMPLE_") {
		t.Errorf("expected no SAMPLE tasks in filtered results")
	}

	// Page 2 of filtered set: 5 rows
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	req2, _ := http.NewRequestWithContext(ctx2, "GET", ts.URL+"/sse/run/run-1/tasks/proc?q=PATIENT&page=2", nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	scanner2 := bufio.NewScanner(resp2.Body)
	event2 := readSSEEvent(t, scanner2, 2*time.Second)

	rowCount2 := strings.Count(event2, "task-table-row")
	if rowCount2 != 5 {
		t.Errorf("expected 5 rows on page 2 of filtered set, got %d", rowCount2)
	}
}

func TestHandleTaskPanel_FilterEmptyFilter(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// q="" should return all 6 tasks (same as no filter)
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?q=", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 6 {
		t.Errorf("expected 6 task-table-row with empty q filter, got %d", rowCount)
	}
}

func TestHandleTaskPanel_FilterNoResults(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?q=NONEXISTENT", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, "No matching tasks") {
		t.Errorf("expected 'No matching tasks' message, got:\n%s", event)
	}
	if strings.Contains(event, "task-table-row") {
		t.Errorf("expected no task-table-row when nothing matches")
	}
}

func TestHandleTaskPanel_FilterCaseInsensitive(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}/tasks/{process}", s.handleTaskPanel)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// lowercase "patient" should match uppercase "PATIENT" in task names
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/run-1/tasks/proc?q=patient", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 3 {
		t.Errorf("expected 3 task-table-row for case-insensitive q=patient, got %d", rowCount)
	}
}

// TestHandleTaskPageNav_FilterPassedToContent verifies that q and status query
// params are passed through to renderTaskPanelHTML, filtering the returned content.
func TestHandleTaskPageNav_FilterPassedToContent(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	// 3 COMPLETED tasks, 2 FAILED tasks
	for i := 1; i <= 3; i++ {
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
	for i := 4; i <= 5; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "FAILED",
			},
		})
	}

	s := serverForSSE(store)
	mux := http.NewServeMux()
	mux.HandleFunc("/run/{id}/tasks/{process}/page", s.handleTaskPageNav)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Filter by status=FAILED — should only see 2 rows.
	req, _ := http.NewRequestWithContext(ctx, "GET",
		ts.URL+"/run/run-1/tasks/proc/page?page=1&status=FAILED", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	rowCount := strings.Count(event, "task-table-row")
	if rowCount != 2 {
		t.Errorf("expected 2 FAILED task rows, got %d", rowCount)
	}
}

// TestHandleTaskPageNav_FilterInDataInitURL verifies that the wrapper div's
// data-init URL includes q and status query params when they are non-empty.
func TestHandleTaskPageNav_FilterInDataInitURL(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "proc (1)",
			Process: "proc",
			Status:  "COMPLETED",
		},
	})

	s := serverForSSE(store)
	mux := http.NewServeMux()
	mux.HandleFunc("/run/{id}/tasks/{process}/page", s.handleTaskPageNav)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		ts.URL+"/run/run-1/tasks/proc/page?page=1&q=hello&status=FAILED", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// data-init URL must include q and status params.
	if !strings.Contains(event, "q=hello") {
		t.Errorf("expected q=hello in data-init URL, got:\n%s", event)
	}
	if !strings.Contains(event, "status=FAILED") {
		t.Errorf("expected status=FAILED in data-init URL, got:\n%s", event)
	}
	// Verify it's in the data-init attribute specifically.
	if !strings.Contains(event, `data-init="@get('/sse/run/run-1/tasks/proc?page=1&gen=1&q=hello&status=FAILED')`) {
		t.Errorf("expected full data-init URL with filters, got:\n%s", event)
	}
}

// TestHandleTaskPageNav_EmptyFilter verifies that when q and status are empty,
// the data-init URL does NOT include q or status params.
func TestHandleTaskPageNav_EmptyFilter(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "proc (1)",
			Process: "proc",
			Status:  "COMPLETED",
		},
	})

	s := serverForSSE(store)
	mux := http.NewServeMux()
	mux.HandleFunc("/run/{id}/tasks/{process}/page", s.handleTaskPageNav)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// No q or status params.
	req, _ := http.NewRequestWithContext(ctx, "GET",
		ts.URL+"/run/run-1/tasks/proc/page?page=1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// The data-init URL should end with page=N&gen=N') — no q or status appended.
	// (q= and status= appear elsewhere in the filter bar JS, so we check data-init specifically.)
	if !strings.Contains(event, `data-init="@get('/sse/run/run-1/tasks/proc?page=1&gen=1')`) {
		t.Errorf("expected data-init with page and gen only (no filter params), got:\n%s", event)
	}
}
