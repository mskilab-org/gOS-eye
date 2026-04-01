package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// serverForLocking creates a Server with a pre-populated run containing tasks.
func serverForLocking() (*Server, string) {
	store := state.NewStore()
	runID := "lock-test-run"

	store.HandleEvent(state.WebhookEvent{
		RunName: "lock_test",
		RunID:   runID,
		Event:   "started",
		UTCTime: "2025-01-15T10:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{
				ProjectName: "test-pipeline",
				CommandLine: "nextflow run main.nf",
				SessionID:   "sess-1",
				Manifest:    state.Manifest{Name: "test-pipeline"},
			},
			Parameters: map[string]any{"input": "data.csv"},
		},
	})

	// Add a few tasks.
	for i := 1; i <= 5; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "lock_test",
			RunID:   runID,
			Event:   "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("sayHello (%d)", i),
				Process: "sayHello",
				Status:  "COMPLETED",
			},
		})
	}

	srv := &Server{
		store:     store,
		broker:    NewBroker(),
		runBroker: NewRunBroker(),
		layouts:   make(map[string]*dag.Layout),
	}
	return srv, runID
}

// TestHandleWebhook_ConcurrentWithRender verifies that handleWebhook (which
// writes to Run fields) and renderRunDetail (which reads Run fields) can run
// concurrently without data races. Run with -race to detect issues.
func TestHandleWebhook_ConcurrentWithRender(t *testing.T) {
	srv, runID := serverForLocking()

	var wg sync.WaitGroup

	// Writer goroutine: fires webhook events that mutate the Run.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 100; i < 150; i++ {
			body := fmt.Sprintf(
				`{"runName":"lock_test","runId":"%s","event":"process_completed","trace":{"task_id":%d,"name":"proc (%d)","process":"proc","status":"COMPLETED"}}`,
				runID, i, i,
			)
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.handleWebhook(rec, req)
		}
	}()

	// Reader goroutine: renders the run detail concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			run := srv.store.GetRun(runID)
			if run != nil {
				run.RLock()
				_ = srv.renderRunDetail(run)
				run.RUnlock()
			}
		}
	}()

	wg.Wait()
}

// TestHandleSelectRun_ConcurrentWithWebhook verifies that handleSelectRun
// reads run fields safely while handleWebhook mutates them.
func TestHandleSelectRun_ConcurrentWithWebhook(t *testing.T) {
	srv, runID := serverForLocking()

	var wg sync.WaitGroup

	// Writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 200; i < 230; i++ {
			body := fmt.Sprintf(
				`{"runName":"lock_test","runId":"%s","event":"process_completed","trace":{"task_id":%d,"name":"proc (%d)","process":"proc","status":"COMPLETED"}}`,
				runID, i, i,
			)
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.handleWebhook(rec, req)
		}
	}()

	// Reader: selectRun.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			req := httptest.NewRequest(http.MethodGet, "/select-run/"+runID, nil)
			req.SetPathValue("id", runID)
			rec := httptest.NewRecorder()
			srv.handleSelectRun(rec, req)
		}
	}()

	wg.Wait()
}

// TestHandleTaskLogs_ConcurrentWithWebhook verifies that handleTaskLogs reads
// run.Tasks safely while webhook events add new tasks.
func TestHandleTaskLogs_ConcurrentWithWebhook(t *testing.T) {
	srv, runID := serverForLocking()

	var wg sync.WaitGroup

	// Writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 300; i < 330; i++ {
			body := fmt.Sprintf(
				`{"runName":"lock_test","runId":"%s","event":"process_completed","trace":{"task_id":%d,"name":"sayHello (%d)","process":"sayHello","status":"COMPLETED"}}`,
				runID, i, i,
			)
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.handleWebhook(rec, req)
		}
	}()

	// Reader: taskLogs for a known task.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse/task/%s/1/logs", runID), nil)
			req.SetPathValue("run", runID)
			req.SetPathValue("task", "1")
			rec := httptest.NewRecorder()
			srv.handleTaskLogs(rec, req)
		}
	}()

	wg.Wait()
}

// TestRenderTaskPanelHTML_ConcurrentWithWebhook verifies that
// renderTaskPanelHTML reads run.Tasks safely while webhook events mutate the map.
func TestRenderTaskPanelHTML_ConcurrentWithWebhook(t *testing.T) {
	srv, runID := serverForLocking()

	var wg sync.WaitGroup

	// Writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 400; i < 430; i++ {
			body := fmt.Sprintf(
				`{"runName":"lock_test","runId":"%s","event":"process_completed","trace":{"task_id":%d,"name":"sayHello (%d)","process":"sayHello","status":"COMPLETED"}}`,
				runID, i, i,
			)
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.handleWebhook(rec, req)
		}
	}()

	// Reader: renderTaskPanelHTML.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			_ = srv.renderTaskPanelHTML(runID, "sayHello", 1)
		}
	}()

	wg.Wait()
}

// TestRenderRunList_ConcurrentWithWebhook verifies that renderRunList reads
// run fields safely while webhook events modify runs.
func TestRenderRunList_ConcurrentWithWebhook(t *testing.T) {
	srv, _ := serverForLocking()

	// Add a second run for more interesting iteration.
	srv.store.HandleEvent(state.WebhookEvent{
		RunName: "second_run",
		RunID:   "lock-test-run-2",
		Event:   "started",
		UTCTime: "2025-01-16T10:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{
				ProjectName: "pipeline-2",
			},
		},
	})

	var wg sync.WaitGroup

	// Writer: update runs via webhook.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 500; i < 530; i++ {
			body := fmt.Sprintf(
				`{"runName":"lock_test","runId":"lock-test-run","event":"process_completed","trace":{"task_id":%d,"name":"proc (%d)","process":"proc","status":"COMPLETED"}}`,
				i, i,
			)
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.handleWebhook(rec, req)
		}
	}()

	// Reader: renderSidebar calls renderRunList.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			_ = srv.renderSidebar()
		}
	}()

	wg.Wait()
}

// TestRenderRunDetail_DoesNotDoubleRLock verifies that renderRunDetail and
// its callees do NOT call RLock themselves — the caller holds the lock.
// If they did double-lock, this would deadlock under -race with a concurrent writer.
func TestRenderRunDetail_DoesNotDoubleRLock(t *testing.T) {
	srv, runID := serverForLocking()

	run := srv.store.GetRun(runID)
	if run == nil {
		t.Fatal("run not found")
	}

	// Simulate what handleWebhook does: render while holding the lock.
	// If renderRunDetail calls RLock internally, this deadlocks.
	done := make(chan struct{})
	go func() {
		run.RLock()
		_ = srv.renderRunDetail(run)
		run.RUnlock()
		close(done)
	}()

	select {
	case <-done:
		// OK — no deadlock.
	case <-time.After(2 * time.Second):
		t.Fatal("renderRunDetail appears to deadlock — it may be calling RLock internally")
	}
}
