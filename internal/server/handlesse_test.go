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

// helper: build a Server with store and broker for SSE tests
func serverForSSE(store *state.Store) *Server {
	return &Server{
		store:   store,
		broker:  NewBroker(),
	}
}

// nonFlusherWriter is a minimal ResponseWriter that does NOT implement http.Flusher.
type nonFlusherWriter struct {
	code   int
	header http.Header
	body   []byte
}

func (w *nonFlusherWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *nonFlusherWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *nonFlusherWriter) WriteHeader(code int) {
	w.code = code
}

// TestHandleSSE_NonFlusherReturns500 tests that a ResponseWriter that does not
// implement http.Flusher gets a 500 error response.
func TestHandleSSE_NonFlusherReturns500(t *testing.T) {
	s := serverForSSE(state.NewStore())
	w := &nonFlusherWriter{}
	r := httptest.NewRequest("GET", "/sse", nil)

	s.handleSSE(w, r)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for non-flusher, got %d", w.code)
	}
}

// readSSEEvent reads lines from a scanner until a blank line (SSE event terminator)
// or timeout. Returns the joined lines.
func readSSEEvent(t *testing.T, scanner *bufio.Scanner, timeout time.Duration) string {
	t.Helper()
	done := make(chan []string, 1)
	go func() {
		var lines []string
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				break
			}
			lines = append(lines, line)
		}
		done <- lines
	}()
	select {
	case lines := <-done:
		return strings.Join(lines, "\n")
	case <-time.After(timeout):
		t.Fatal("timed out waiting for SSE event")
		return ""
	}
}

// TestHandleSSE_SetsCorrectHeaders verifies Content-Type and Cache-Control headers.
func TestHandleSSE_SetsCorrectHeaders(t *testing.T) {
	s := serverForSSE(state.NewStore())
	ts := httptest.NewServer(http.HandlerFunc(s.handleSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
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
}

// TestHandleSSE_InitialRenderSent verifies the first SSE event contains the
// sidebar run-list in Datastar fragment format.
func TestHandleSSE_InitialRenderSent(t *testing.T) {
	s := serverForSSE(state.NewStore())
	ts := httptest.NewServer(http.HandlerFunc(s.handleSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
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
	if !strings.Contains(event, "run-list") {
		t.Errorf("initial event should contain sidebar run-list HTML, got:\n%s", event)
	}
}

// TestHandleSSE_InitialRenderWithTasks verifies the initial SSE events include
// sidebar run-list and a latestRun signal when the store has runs.
func TestHandleSSE_InitialRenderWithTasks(t *testing.T) {
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
		},
	})
	s := serverForSSE(store)
	ts := httptest.NewServer(http.HandlerFunc(s.handleSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	// First event: sidebar fragment with run-list
	sidebarEvent := readSSEEvent(t, scanner, 2*time.Second)
	if !strings.Contains(sidebarEvent, `id="run-list"`) {
		t.Errorf("initial event should contain sidebar run-list, got:\n%s", sidebarEvent)
	}
	if !strings.Contains(sidebarEvent, "test_run") {
		t.Errorf("initial event should contain run name, got:\n%s", sidebarEvent)
	}

	// Second event: latestRun signal
	signalEvent := readSSEEvent(t, scanner, 2*time.Second)
	if !strings.Contains(signalEvent, "event: datastar-patch-signals") {
		t.Errorf("expected latestRun signal event, got:\n%s", signalEvent)
	}
	if !strings.Contains(signalEvent, "latestRun") || !strings.Contains(signalEvent, "r1") {
		t.Errorf("signal should contain latestRun with run ID r1, got:\n%s", signalEvent)
	}
}

// TestHandleSSE_NoSignalWhenNoRuns verifies that no latestRun signal is sent
// when the store has no runs.
func TestHandleSSE_NoSignalWhenNoRuns(t *testing.T) {
	s := serverForSSE(state.NewStore())
	ts := httptest.NewServer(http.HandlerFunc(s.handleSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	// First event: sidebar fragment (empty run-list)
	event := readSSEEvent(t, scanner, 2*time.Second)
	if !strings.Contains(event, "event: datastar-patch-elements") {
		t.Errorf("expected sidebar fragment event, got:\n%s", event)
	}

	// Publish something to prove the connection works, then read it.
	// If a latestRun signal had been sent, it would appear before this.
	// Broker carries pre-formatted SSE strings.
	s.broker.Publish(formatSSEFragment(`<div id="run-list">updated</div>`))
	next := readSSEEvent(t, scanner, 2*time.Second)
	if strings.Contains(next, "datastar-patch-signals") {
		t.Errorf("should not send latestRun signal when no runs exist, got:\n%s", next)
	}
}

// TestHandleSSE_PublishedDataStreamed verifies that data published to the broker
// after initial connect arrives as an SSE event.
func TestHandleSSE_PublishedDataStreamed(t *testing.T) {
	s := serverForSSE(state.NewStore())
	ts := httptest.NewServer(http.HandlerFunc(s.handleSSE))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Read past the initial event
	_ = readSSEEvent(t, scanner, 2*time.Second)

	// Publish a pre-formatted SSE fragment (broker carries formatted strings)
	s.broker.Publish(formatSSEFragment(`<div id="dashboard"><p>test: COMPLETED</p></div>`))

	// Read the published event
	event := readSSEEvent(t, scanner, 2*time.Second)

	if !strings.Contains(event, "event: datastar-patch-elements") {
		t.Errorf("published event missing 'event: datastar-patch-elements', got:\n%s", event)
	}
	if !strings.Contains(event, "test: COMPLETED") {
		t.Errorf("published event should contain published data, got:\n%s", event)
	}
}

// TestHandleSSE_UnsubscribesOnDisconnect verifies that canceling the request
// context causes the handler to call Unsubscribe, removing the subscriber.
func TestHandleSSE_UnsubscribesOnDisconnect(t *testing.T) {
	s := serverForSSE(state.NewStore())
	ts := httptest.NewServer(http.HandlerFunc(s.handleSSE))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	// Read the initial event so we know the handler is running
	scanner := bufio.NewScanner(resp.Body)
	_ = readSSEEvent(t, scanner, 2*time.Second)

	// Should have 1 subscriber now
	s.broker.mu.RLock()
	before := len(s.broker.subscribers)
	s.broker.mu.RUnlock()
	if before != 1 {
		t.Errorf("expected 1 subscriber before cancel, got %d", before)
	}

	// Cancel the context (simulates client disconnect)
	cancel()
	resp.Body.Close()

	// Give time for cleanup goroutine
	time.Sleep(200 * time.Millisecond)

	s.broker.mu.RLock()
	after := len(s.broker.subscribers)
	s.broker.mu.RUnlock()
	if after != 0 {
		t.Errorf("expected 0 subscribers after disconnect, got %d", after)
	}
}
