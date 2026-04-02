package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// ---- renderUndoToast ----

func TestRenderUndoToast_Empty(t *testing.T) {
	got := renderUndoToast("", "")
	want := `<div id="undo-toast"></div>`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderUndoToast_WithRun(t *testing.T) {
	got := renderUndoToast("run-1", "happy_euler")
	if !strings.Contains(got, `id="undo-toast"`) {
		t.Fatal("missing id")
	}
	if !strings.Contains(got, `"happy_euler" hidden`) {
		t.Fatal("missing run name in message")
	}
	if !strings.Contains(got, `/unhide-run/run-1`) {
		t.Fatal("missing unhide URL")
	}
	if !strings.Contains(got, `class="btn-undo"`) {
		t.Fatal("missing undo button")
	}
}

func TestRenderUndoToast_EscapesRunName(t *testing.T) {
	got := renderUndoToast("r1", `<script>alert("xss")</script>`)
	if strings.Contains(got, "<script>") {
		t.Fatal("run name should be HTML-escaped")
	}
}

// ---- latestVisibleRunID ----

func TestLatestVisibleRunID_NoHidden(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "r", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := NewServer(store, nil)

	got := s.latestVisibleRunID()
	if got != "run1" {
		t.Fatalf("expected run1, got %q", got)
	}
}

func TestLatestVisibleRunID_LatestIsHidden(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "a", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	store.HandleEvent(state.WebhookEvent{RunName: "b", RunID: "r2", Event: "started", UTCTime: "2024-02-01T00:00:00Z"})
	s := NewServer(store, nil)
	s.SetHiddenRuns([]string{"r2"})

	got := s.latestVisibleRunID()
	if got != "r1" {
		t.Fatalf("expected r1 (fallback), got %q", got)
	}
}

func TestLatestVisibleRunID_AllHidden(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "a", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := NewServer(store, nil)
	s.SetHiddenRuns([]string{"r1"})

	got := s.latestVisibleRunID()
	if got != "" {
		t.Fatalf("expected empty when all hidden, got %q", got)
	}
}

// ---- renderSidebar filters hidden ----

func TestRenderSidebar_FiltersHiddenRuns(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "visible_run", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	store.HandleEvent(state.WebhookEvent{RunName: "hidden_run", RunID: "r2", Event: "started", UTCTime: "2024-02-01T00:00:00Z"})
	s := NewServer(store, nil)
	s.SetHiddenRuns([]string{"r2"})

	got := s.renderSidebar()
	if !strings.Contains(got, "visible_run") {
		t.Fatal("visible run should be in sidebar")
	}
	if strings.Contains(got, "hidden_run") {
		t.Fatal("hidden run should NOT be in sidebar")
	}
}

// ---- handleHideRun ----

func TestHandleHideRun_ReturnsUndoToast(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "happy_euler", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := NewServer(store, nil)

	req := httptest.NewRequest(http.MethodGet, "/hide-run/r1", nil)
	req.SetPathValue("id", "r1")
	w := httptest.NewRecorder()
	s.handleHideRun(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "undo-toast") {
		t.Fatal("response should contain undo toast")
	}
	if !strings.Contains(body, "happy_euler") {
		t.Fatal("undo toast should contain run name")
	}
	if !strings.Contains(body, "/unhide-run/r1") {
		t.Fatal("undo toast should contain unhide URL")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
}

func TestHandleHideRun_MarksRunHidden(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "r", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := NewServer(store, nil)

	req := httptest.NewRequest(http.MethodGet, "/hide-run/r1", nil)
	req.SetPathValue("id", "r1")
	w := httptest.NewRecorder()
	s.handleHideRun(w, req)

	// Sidebar should no longer contain the hidden run
	sidebar := s.renderSidebar()
	if strings.Contains(sidebar, "r1") {
		t.Fatal("hidden run should not appear in sidebar after hiding")
	}
}

// ---- handleUnhideRun ----

func TestHandleUnhideRun_RestoresRun(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "happy_euler", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := NewServer(store, nil)
	s.SetHiddenRuns([]string{"r1"})

	// Verify hidden first
	if strings.Contains(s.renderSidebar(), "happy_euler") {
		t.Fatal("run should be hidden initially")
	}

	req := httptest.NewRequest(http.MethodGet, "/unhide-run/r1", nil)
	req.SetPathValue("id", "r1")
	w := httptest.NewRecorder()
	s.handleUnhideRun(w, req)

	// Response should dismiss the toast
	body := w.Body.String()
	if strings.Contains(body, "btn-undo") {
		t.Fatal("unhide response should contain empty undo toast (no button)")
	}

	// Sidebar should now contain the run again
	sidebar := s.renderSidebar()
	if !strings.Contains(sidebar, "happy_euler") {
		t.Fatal("run should be visible in sidebar after unhiding")
	}
}

// ---- renderRunList trash button ----

func TestRenderRunList_HasTrashButton(t *testing.T) {
	runs := []*state.Run{
		{RunName: "run_a", RunID: "r1", Status: "completed", StartTime: "2024-01-01T00:00:00Z"},
	}
	got := renderRunList(runs, "r1")

	if !strings.Contains(got, `class="btn-hide-run"`) {
		t.Fatal("each run entry should have a hide button")
	}
	if !strings.Contains(got, `/hide-run/r1`) {
		t.Fatal("hide button should target the correct run ID")
	}
	if !strings.Contains(got, `data-on:click__stop`) {
		t.Fatal("hide button should use __stop to prevent run selection")
	}
}

// ---- route registration ----

func TestNewServer_HideRunRouteRegistered(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{RunName: "r", RunID: "r1", Event: "started", UTCTime: "2024-01-01T00:00:00Z"})
	s := NewServer(store, nil)

	req := httptest.NewRequest(http.MethodGet, "/hide-run/r1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("GET /hide-run/{id} returned 404; route not registered")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
}

func TestNewServer_UnhideRunRouteRegistered(t *testing.T) {
	s := NewServer(state.NewStore(), nil)

	req := httptest.NewRequest(http.MethodGet, "/unhide-run/r1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("GET /unhide-run/{id} returned 404; route not registered")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
}
