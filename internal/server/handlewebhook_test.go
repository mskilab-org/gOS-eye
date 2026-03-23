package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// helper: build a Server with real store and broker (no mux needed for direct handler calls)
func serverForWebhook() *Server {
	return &Server{
		store:  state.NewStore(),
		broker: NewBroker(),
	}
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
	ch := s.broker.Subscribe()

	body := `{"runName":"happy_euler","runId":"abc123","event":"process_completed","trace":{"task_id":1,"name":"sayHello (1)","process":"sayHello","status":"COMPLETED"}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	select {
	case fragment := <-ch:
		if !strings.Contains(fragment, `id="dashboard"`) {
			t.Errorf("published fragment missing id=\"dashboard\", got:\n%s", fragment)
		}
		if !strings.Contains(fragment, `class="process-group"`) {
			t.Errorf("published fragment missing process-group, got:\n%s", fragment)
		}
		if !strings.Contains(fragment, "sayHello") {
			t.Errorf("published fragment missing process name 'sayHello', got:\n%s", fragment)
		}
	default:
		t.Error("expected a fragment to be published to subscriber, got nothing")
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
