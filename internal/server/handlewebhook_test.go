package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// helper: build a Server with real store and broker (no mux needed for direct handler calls)
func serverForWebhook() *Server {
	return NewServer(state.NewStore(), nil)
}

func postWebhookEvent(t *testing.T, s *Server, event state.WebhookEvent) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal webhook event: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	s.handleWebhook(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	return rec
}

func TestHandleWebhook_GET_Returns405(t *testing.T) {
	s := serverForWebhook()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rec.Code)
	}
}

func TestHandleWebhook_PUT_Returns405(t *testing.T) {
	s := serverForWebhook()
	body := `{"runName":"x","runId":"y","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPut, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for PUT, got %d", rec.Code)
	}
}

func TestHandleWebhook_ValidEvent_Returns200(t *testing.T) {
	s := serverForWebhook()
	body := `{"runName":"happy_euler","runId":"abc123","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleWebhook_InvalidJSON_Returns400(t *testing.T) {
	s := serverForWebhook()
	body := `{not valid json`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleWebhook_EmptyBody_Returns400(t *testing.T) {
	s := serverForWebhook()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(""))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleWebhook_UpdatesStore(t *testing.T) {
	s := serverForWebhook()
	body := `{"runName":"happy_euler","runId":"abc123","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	run := s.store.GetRun("abc123")
	if run == nil {
		t.Fatal("expected store to contain run abc123 after webhook")
	}
	if run.RunName != "happy_euler" {
		t.Errorf("expected RunName 'happy_euler', got %q", run.RunName)
	}
}

func TestHandleWebhook_PublishesFragment(t *testing.T) {
	s := serverForWebhook()
	sidebarCh := s.broker.Subscribe()

	body := `{"runName":"happy_euler","runId":"abc123","event":"process_completed","trace":{"task_id":1,"name":"sayHello (1)","process":"sayHello","status":"COMPLETED"}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	// Broker receives pre-formatted SSE: sidebar fragment + dashboard fragment.
	select {
	case fragment := <-sidebarCh:
		// Sidebar content (run list + run-selector)
		if !strings.Contains(fragment, `id="run-list"`) {
			t.Errorf("published SSE missing id=\"run-list\", got:\n%s", fragment)
		}
		if !strings.Contains(fragment, "happy_euler") {
			t.Errorf("published SSE missing run name 'happy_euler', got:\n%s", fragment)
		}
		if !strings.Contains(fragment, `id="run-selector"`) {
			t.Errorf("published SSE missing run-selector trigger div, got:\n%s", fragment)
		}
		// Dashboard push with CSS selector targeting
		if !strings.Contains(fragment, `data: selector #dashboard[data-run="abc123"]`) {
			t.Errorf("published SSE missing dashboard selector for run abc123, got:\n%s", fragment)
		}
		if !strings.Contains(fragment, `data-run="abc123"`) {
			t.Errorf("published SSE missing data-run attribute, got:\n%s", fragment)
		}
	default:
		t.Error("expected a fragment to be published, got nothing")
	}
}

func TestHandleWebhook_ResumedAttemptPublishesDashboardForMonitorRunID(t *testing.T) {
	s := serverForWebhook()

	postWebhookEvent(t, s, state.WebhookEvent{
		RunID:   "nf-1",
		RunName: "first_name",
		Event:   "started",
		UTCTime: "2024-01-01T00:00:00Z",
		Metadata: &state.Metadata{Workflow: state.WorkflowInfo{
			SessionID: "sess-1",
		}},
	})
	postWebhookEvent(t, s, state.WebhookEvent{
		RunID:   "nf-1",
		RunName: "second_name",
		Event:   "started",
		UTCTime: "2024-01-01T00:01:00Z",
		Metadata: &state.Metadata{Workflow: state.WorkflowInfo{
			SessionID: "sess-1",
			Resume:    true,
		}},
	})

	dashboardCh := s.broker.Subscribe()
	postWebhookEvent(t, s, state.WebhookEvent{
		RunID:   "nf-1",
		RunName: "second_name",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "align (1)",
			Process: "align",
			Status:  state.TaskStatusCompleted,
		},
	})

	const secondMonitorID = "nf-1--attempt-2"
	select {
	case fragment := <-dashboardCh:
		if !strings.Contains(fragment, `data: selector #dashboard[data-run="`+secondMonitorID+`"]`) {
			t.Fatalf("published SSE missing dashboard selector for monitor run %q, got:\n%s", secondMonitorID, fragment)
		}
		if !strings.Contains(fragment, `data-run="`+secondMonitorID+`"`) {
			t.Fatalf("published SSE missing dashboard data-run for monitor run %q, got:\n%s", secondMonitorID, fragment)
		}
		if strings.Contains(fragment, `data: selector #dashboard[data-run="nf-1"]`) {
			t.Fatalf("published SSE targeted raw first-attempt run ID instead of second monitor ID; got:\n%s", fragment)
		}
	default:
		t.Fatal("expected a fragment to be published, got nothing")
	}
}

func TestHandleWebhook_NoSubscribers_StillReturns200(t *testing.T) {
	s := serverForWebhook()
	// No subscribers — Publish should not panic
	body := `{"runName":"happy_euler","runId":"abc123","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleWebhook_ProcessEvent_UpdatesTaskInStore(t *testing.T) {
	s := serverForWebhook()
	body := `{"runName":"happy_euler","runId":"run1","event":"process_submitted","trace":{"task_id":5,"name":"align (1)","status":"SUBMITTED","process":"align"}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	run := s.store.GetRun("run1")
	if run == nil {
		t.Fatal("expected run to exist")
	}
	task, ok := run.Tasks[5]
	if !ok {
		t.Fatal("expected task 5 to exist in run")
	}
	if task.Status != "SUBMITTED" {
		t.Errorf("expected task status SUBMITTED, got %q", task.Status)
	}
}
