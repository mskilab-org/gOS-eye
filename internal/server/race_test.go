package server

import (
	"fmt"
	"sync"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// TestConcurrentWebhookAndRender is a stress test that reproduces the original
// crash scenario: concurrent map iteration (render) + map write (webhook) on
// Run.Tasks. It must FAIL under `go test -race` if the RWMutex locking is
// removed, and PASS with locking in place.
func TestConcurrentWebhookAndRender(t *testing.T) {
	// Setup: create a store with a run and some initial tasks.
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "stress_run", RunID: "run-stress", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 10; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "stress_run", RunID: "run-stress", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  i,
				Name:    fmt.Sprintf("proc (%d)", i),
				Process: "proc",
				Status:  "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)

	const (
		numWriters      = 5
		numReaders      = 5
		opsPerGoroutine = 100
	)

	var wg sync.WaitGroup

	// Writers: continuously add new tasks via store.HandleEvent (simulating
	// rapid webhook events).
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				taskID := writerID*1000 + i
				store.HandleEvent(state.WebhookEvent{
					RunName: "stress_run", RunID: "run-stress", Event: "process_completed",
					Trace: &state.Trace{
						TaskID:  taskID,
						Name:    fmt.Sprintf("proc (%d)", taskID),
						Process: "proc",
						Status:  "COMPLETED",
					},
				})
			}
		}(w)
	}

	// Readers: continuously render the run detail (iterates run.Tasks).
	run := store.GetRun("run-stress")
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				run.RLock()
				_ = s.renderRunDetail(run)
				run.RUnlock()
			}
		}()
	}

	// Also test renderTaskPanelHTML concurrently — it does its own
	// RLock/RUnlock internally, so call it directly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < opsPerGoroutine; i++ {
			_ = s.renderTaskPanelHTML("run-stress", "proc", 1)
		}
	}()

	wg.Wait()
	// If we get here without crashing or race-detector errors, the test passes.
}
