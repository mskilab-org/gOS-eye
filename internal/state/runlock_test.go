package state

import (
	"sync"
	"testing"
)

// --- RLock / RUnlock exist and are callable ---

func TestRun_RLock_RUnlock_Callable(t *testing.T) {
	r := &Run{RunID: "r1", Tasks: make(map[int]*Task)}
	r.RLock()
	r.RUnlock()
	// No panic = pass
}

func TestRun_MultipleReadersAllowed(t *testing.T) {
	r := &Run{RunID: "r1", Tasks: make(map[int]*Task)}
	// Two concurrent read locks should not deadlock.
	r.RLock()
	r.RLock()
	r.RUnlock()
	r.RUnlock()
}

// --- Concurrent HandleEvent + read via RLock ---

func TestRun_RLock_ConcurrentWithHandleEvent(t *testing.T) {
	s := NewStore()
	// Seed a run so readers have something to look at.
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0",
	})

	var wg sync.WaitGroup
	const N = 100

	// Writer goroutine: sends process_completed events.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= N; i++ {
			s.HandleEvent(WebhookEvent{
				RunName: "r", RunID: "run-1", Event: "process_completed", UTCTime: "t1",
				Trace: &Trace{TaskID: i, Hash: "ab/01", Name: "task", Process: "proc", Status: "COMPLETED"},
			})
		}
	}()

	// Reader goroutines: read Run fields under RLock.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < N; j++ {
				r := s.GetRun("run-1")
				if r == nil {
					continue
				}
				r.RLock()
				_ = r.Status
				_ = r.StartTime
				// Iterate tasks — this is the main race we're protecting.
				for _, task := range r.Tasks {
					_ = task.Status
				}
				r.RUnlock()
			}
		}()
	}

	wg.Wait()
	// No race detector failures = pass.
}

// --- Verify HandleEvent still works correctly with locking ---

func TestRun_HandleEvent_Started_StillSetsFields(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "test_run",
		RunID:   "r-1",
		Event:   "started",
		UTCTime: "2024-06-01T00:00:00Z",
		Metadata: &Metadata{
			Workflow: WorkflowInfo{
				ProjectName: "nf-core/rnaseq",
				CommandLine: "nextflow run main.nf",
				SessionID:   "sess-1",
				WorkDir:     "/work",
				LaunchDir:   "/launch",
			},
			Parameters: map[string]any{"input": "samples.csv"},
		},
	})

	r := s.Runs["r-1"]
	if r.Status != "running" {
		t.Errorf("Status = %q, want %q", r.Status, "running")
	}
	if r.ProjectName != "nf-core/rnaseq" {
		t.Errorf("ProjectName = %q, want %q", r.ProjectName, "nf-core/rnaseq")
	}
	if r.CommandLine != "nextflow run main.nf" {
		t.Errorf("CommandLine = %q, want %q", r.CommandLine, "nextflow run main.nf")
	}
}

func TestRun_HandleEvent_Completed_StillSetsFields(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{RunName: "r", RunID: "r-1", Event: "started", UTCTime: "t0"})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "r-1", Event: "completed", UTCTime: "2024-06-01T01:00:00Z",
	})

	r := s.Runs["r-1"]
	if r.Status != "completed" {
		t.Errorf("Status = %q, want %q", r.Status, "completed")
	}
	if r.CompleteTime != "2024-06-01T01:00:00Z" {
		t.Errorf("CompleteTime = %q, want %q", r.CompleteTime, "2024-06-01T01:00:00Z")
	}
}

func TestRun_HandleEvent_Error_StillSetsFields(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{RunName: "r", RunID: "r-1", Event: "started", UTCTime: "t0"})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "r-1", Event: "error", UTCTime: "2024-06-01T01:00:00Z",
		Metadata: &Metadata{
			Workflow: WorkflowInfo{ErrorMessage: "OOM killed"},
		},
	})

	r := s.Runs["r-1"]
	if r.Status != "failed" {
		t.Errorf("Status = %q, want %q", r.Status, "failed")
	}
	if r.ErrorMessage != "OOM killed" {
		t.Errorf("ErrorMessage = %q, want %q", r.ErrorMessage, "OOM killed")
	}
}

func TestRun_HandleEvent_ProcessEvent_StillUpsertsTask(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{RunName: "r", RunID: "r-1", Event: "started", UTCTime: "t0"})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "r-1", Event: "process_completed", UTCTime: "t1",
		Trace: &Trace{TaskID: 42, Hash: "ff/00", Name: "align (1)", Process: "align", Status: "COMPLETED", PeakRSS: 1024},
	})

	r := s.Runs["r-1"]
	task := r.Tasks[42]
	if task == nil {
		t.Fatal("expected task 42 to exist")
	}
	if task.Status != "COMPLETED" {
		t.Errorf("Status = %q, want %q", task.Status, "COMPLETED")
	}
	if task.PeakRSS != 1024 {
		t.Errorf("PeakRSS = %d, want %d", task.PeakRSS, 1024)
	}
}
