package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// TestHandleRunSSE_NonFlusherReturns500 verifies that a ResponseWriter without
// http.Flusher support gets a 500 error.
func TestHandleRunSSE_NonFlusherReturns500(t *testing.T) {
	s := serverForSSE(state.NewStore())
	w := &nonFlusherWriter{}
	r := httptest.NewRequest("GET", "/sse/run/r1", nil)
	r.SetPathValue("id", "r1")

	s.handleRunSSE(w, r)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for non-flusher, got %d", w.code)
	}
}

// TestHandleRunSSE_SetsSSEHeaders verifies Content-Type, Cache-Control, and
// Connection headers are set correctly.
func TestHandleRunSSE_SetsSSEHeaders(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "started",
	})
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/r1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want \"text/event-stream\"", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want \"no-cache\"", cc)
	}
	if conn := resp.Header.Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want \"keep-alive\"", conn)
	}
}

// TestHandleRunSSE_InitialRenderSent verifies the first SSE event contains the
// run detail wrapped in <div id="dashboard"> in Datastar format.
func TestHandleRunSSE_InitialRenderSent(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "started",
	})
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/r1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, "event: datastar-patch-elements") {
		t.Errorf("initial event missing 'event: datastar-patch-elements', got:\n%s", event)
	}
	if !strings.Contains(event, "data: elements") {
		t.Errorf("initial event missing 'data: elements', got:\n%s", event)
	}
	if !strings.Contains(event, `id="dashboard"`) {
		t.Errorf("initial event should contain dashboard div, got:\n%s", event)
	}
	if !strings.Contains(event, `data-init="@get('/sse/run/r1')"`) {
		t.Errorf("initial dashboard div should have data-init with SSE URL, got:\n%s", event)
	}
}

// TestHandleRunSSE_InitialRenderContainsRunDetail verifies the initial render
// includes run-specific content (run header, process table, summary).
func TestHandleRunSSE_InitialRenderContainsRunDetail(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "fancy_run",
		RunID:   "r1",
		Event:   "started",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{
				ProjectName: "MyPipeline",
			},
		},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "fancy_run",
		RunID:   "r1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "align (1)",
			Process: "align",
			Status:  "COMPLETED",
		},
	})
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/r1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	// Should contain pipeline name from renderRunDetail
	if !strings.Contains(event, "MyPipeline") {
		t.Errorf("initial event should contain pipeline name, got:\n%s", event)
	}
	// Should contain process name from renderProcessTable
	if !strings.Contains(event, "align") {
		t.Errorf("initial event should contain process name 'align', got:\n%s", event)
	}
}

// TestHandleRunSSE_RunNotFound verifies that requesting a non-existent run
// sends a dashboard div with an error message.
func TestHandleRunSSE_RunNotFound(t *testing.T) {
	s := serverForSSE(state.NewStore())

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, "event: datastar-patch-elements") {
		t.Errorf("error event missing 'event: datastar-patch-elements', got:\n%s", event)
	}
	if !strings.Contains(event, `id="dashboard"`) {
		t.Errorf("error event should contain dashboard div, got:\n%s", event)
	}
	// Should contain some error indication
	if !strings.Contains(event, "not found") && !strings.Contains(event, "Run not found") {
		t.Errorf("error event should indicate run not found, got:\n%s", event)
	}
}

// TestHandleRunSSE_PublishedDataStreamed verifies that data published to the
// runBroker for the correct runID arrives as an SSE event.
func TestHandleRunSSE_PublishedDataStreamed(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "started",
	})
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/r1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Read past the initial event
	_ = readSSEEvent(t, scanner, 2*time.Second)

	// Publish an update via runBroker (same format as handleWebhook)
	s.runBroker.Publish("r1", `<div id="dashboard"><p>updated content</p></div>`)

	// Read the streamed event
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, "event: datastar-patch-elements") {
		t.Errorf("published event missing 'event: datastar-patch-elements', got:\n%s", event)
	}
	if !strings.Contains(event, "updated content") {
		t.Errorf("published event should contain published data, got:\n%s", event)
	}
}

// TestHandleRunSSE_UnsubscribesOnDisconnect verifies that canceling the request
// causes the handler to unsubscribe from the runBroker.
func TestHandleRunSSE_UnsubscribesOnDisconnect(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run",
		RunID:   "r1",
		Event:   "started",
	})
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/r1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	// Read the initial event so we know the handler is running
	scanner := bufio.NewScanner(resp.Body)
	_ = readSSEEvent(t, scanner, 2*time.Second)

	// Should have 1 subscriber for "r1"
	s.runBroker.mu.RLock()
	before := len(s.runBroker.subs["r1"])
	s.runBroker.mu.RUnlock()
	if before != 1 {
		t.Errorf("expected 1 subscriber before cancel, got %d", before)
	}

	// Cancel the context (simulates client disconnect)
	cancel()
	resp.Body.Close()

	// Give time for cleanup
	time.Sleep(200 * time.Millisecond)

	s.runBroker.mu.RLock()
	after := len(s.runBroker.subs["r1"])
	s.runBroker.mu.RUnlock()
	if after != 0 {
		t.Errorf("expected 0 subscribers after disconnect, got %d", after)
	}
}

// TestHandleRunSSE_DifferentRunIDIsolation verifies that publishing to a
// different runID does NOT affect clients subscribed to another runID.
func TestHandleRunSSE_DifferentRunIDIsolation(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_a",
		RunID:   "r1",
		Event:   "started",
	})
	s := serverForSSE(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/run/{id}", s.handleRunSSE)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/sse/run/r1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Read past the initial event
	_ = readSSEEvent(t, scanner, 2*time.Second)

	// Publish to a DIFFERENT runID — should NOT reach client subscribed to r1
	s.runBroker.Publish("r2", `<div id="dashboard"><p>wrong run</p></div>`)

	// Now publish to the correct runID
	s.runBroker.Publish("r1", `<div id="dashboard"><p>correct run</p></div>`)

	event := readSSEEvent(t, scanner, 2*time.Second)

	if strings.Contains(event, "wrong run") {
		t.Errorf("should not receive data for different runID, got:\n%s", event)
	}
	if !strings.Contains(event, "correct run") {
		t.Errorf("should receive data for subscribed runID, got:\n%s", event)
	}
}
