package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// ---- Test doubles ----

// spyPersister records every Save call.
type spyPersister struct {
	mu    sync.Mutex
	calls [][]byte
	err   error // if non-nil, Save returns this
}

func (p *spyPersister) Save(rawJSON []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]byte, len(rawJSON))
	copy(cp, rawJSON)
	p.calls = append(p.calls, cp)
	return p.err
}

func (p *spyPersister) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *spyPersister) lastCall() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.calls) == 0 {
		return nil
	}
	return p.calls[len(p.calls)-1]
}

// ---- Tests ----

func TestHandleWebhook_Persist_SaveCalledBeforeUnmarshal(t *testing.T) {
	spy := &spyPersister{}
	s := &Server{
		store:      state.NewStore(),
		broker:     NewBroker(),
		runBroker:  NewRunBroker(),
		eventStore: spy,
	}
	body := `{"runName":"r","runId":"id1","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if spy.callCount() != 1 {
		t.Fatalf("expected Save called once, got %d", spy.callCount())
	}
	if string(spy.lastCall()) != body {
		t.Errorf("expected Save to receive raw body %q, got %q", body, string(spy.lastCall()))
	}
}

func TestHandleWebhook_Persist_NilPersister_StillWorks(t *testing.T) {
	// eventStore == nil → persistence skipped, no panic
	s := &Server{
		store:      state.NewStore(),
		broker:     NewBroker(),
		runBroker:  NewRunBroker(),
		eventStore: nil,
	}
	body := `{"runName":"r","runId":"id2","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleWebhook_Persist_SaveError_LogsAndContinues(t *testing.T) {
	spy := &spyPersister{err: errors.New("disk full")}
	s := &Server{
		store:      state.NewStore(),
		broker:     NewBroker(),
		runBroker:  NewRunBroker(),
		eventStore: spy,
	}
	body := `{"runName":"r","runId":"id3","event":"started","utcTime":"2024-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	// Webhook must still succeed (log-and-continue)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 even on Save error, got %d", rec.Code)
	}
	// Store should still have the run
	if s.store.GetRun("id3") == nil {
		t.Error("expected store to contain run after Save error")
	}
}

func TestHandleWebhook_Persist_InvalidJSON_SaveStillCalled(t *testing.T) {
	// Save is called BEFORE Unmarshal, so even invalid JSON gets persisted
	spy := &spyPersister{}
	s := &Server{
		store:      state.NewStore(),
		broker:     NewBroker(),
		runBroker:  NewRunBroker(),
		eventStore: spy,
	}
	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	// Request fails with 400 (invalid JSON)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	// But Save was still called with the raw bytes
	if spy.callCount() != 1 {
		t.Fatalf("expected Save called once for invalid JSON, got %d", spy.callCount())
	}
	if string(spy.lastCall()) != body {
		t.Errorf("expected Save to receive %q, got %q", body, string(spy.lastCall()))
	}
}

func TestHandleWebhook_Persist_NotCalledForGET(t *testing.T) {
	spy := &spyPersister{}
	s := &Server{
		store:      state.NewStore(),
		broker:     NewBroker(),
		runBroker:  NewRunBroker(),
		eventStore: spy,
	}
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()

	s.handleWebhook(rec, req)

	if spy.callCount() != 0 {
		t.Errorf("Save should not be called for GET, got %d calls", spy.callCount())
	}
}

func TestHandleWebhook_Persist_MultipleCalls(t *testing.T) {
	spy := &spyPersister{}
	s := &Server{
		store:      state.NewStore(),
		broker:     NewBroker(),
		runBroker:  NewRunBroker(),
		eventStore: spy,
	}

	bodies := []string{
		`{"runName":"r","runId":"id4","event":"started","utcTime":"2024-01-01T00:00:00Z"}`,
		`{"runName":"r","runId":"id4","event":"process_submitted","trace":{"task_id":1,"name":"foo (1)","status":"SUBMITTED","process":"foo"}}`,
	}
	for _, body := range bodies {
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
		rec := httptest.NewRecorder()
		s.handleWebhook(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	}

	if spy.callCount() != 2 {
		t.Errorf("expected 2 Save calls, got %d", spy.callCount())
	}
}
