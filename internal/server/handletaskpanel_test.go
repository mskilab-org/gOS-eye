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
