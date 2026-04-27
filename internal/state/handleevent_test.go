package state

import (
	"encoding/json"
	"os"
	"testing"
)

// --- "started" event ---

func TestHandleEvent_Started_CreatesRunWithMetadata(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName:  "happy_darwin",
		RunID:    "run-1",
		Event:    "started",
		UTCTime:  "2024-01-15T10:30:00Z",
		Metadata: &Metadata{},
	})

	r := s.Runs["run-1"]
	if r == nil {
		t.Fatal("expected run to be created")
	}
	if r.RunName != "happy_darwin" {
		t.Errorf("RunName = %q, want %q", r.RunName, "happy_darwin")
	}
	if r.RunID != "run-1" {
		t.Errorf("RunID = %q, want %q", r.RunID, "run-1")
	}
	if r.Status != "running" {
		t.Errorf("Status = %q, want %q", r.Status, "running")
	}
	if r.StartTime != "2024-01-15T10:30:00Z" {
		t.Errorf("StartTime = %q, want %q", r.StartTime, "2024-01-15T10:30:00Z")
	}
	if r.Tasks == nil {
		t.Error("Tasks map should be initialized")
	}
}

func TestHandleEvent_Started_NoMetadata_FallsBackToUTCTime(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "happy_darwin",
		RunID:   "run-1",
		Event:   "started",
		UTCTime: "2024-01-15T10:30:00Z",
	})

	r := s.Runs["run-1"]
	if r == nil {
		t.Fatal("expected run to be created")
	}
	if r.StartTime != "2024-01-15T10:30:00Z" {
		t.Errorf("StartTime = %q, want %q (UTCTime fallback)", r.StartTime, "2024-01-15T10:30:00Z")
	}
}

func TestHandleEvent_ReturnsMonitorRunIDForStartedAndRoutedEvents(t *testing.T) {
	s := NewStore()

	firstID := s.HandleEvent(WebhookEvent{
		RunID:   "nf-1",
		RunName: "first_name",
		Event:   "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})
	secondID := s.HandleEvent(WebhookEvent{
		RunID:   "nf-1",
		RunName: "second_name",
		Event:   "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
			Resume:    true,
		}},
	})
	processID := s.HandleEvent(WebhookEvent{
		RunID:   "nf-1",
		RunName: "second_name",
		Event:   "process_completed",
		Trace: &Trace{
			TaskID:  1,
			Name:    "align (1)",
			Process: "align",
			Status:  TaskStatusCompleted,
		},
	})

	if firstID != "nf-1" {
		t.Fatalf("first started returned monitor ID %q, want %q", firstID, "nf-1")
	}
	if secondID != "nf-1--attempt-2" {
		t.Fatalf("second started returned monitor ID %q, want %q", secondID, "nf-1--attempt-2")
	}
	if processID != secondID {
		t.Fatalf("process event returned monitor ID %q, want second attempt ID %q", processID, secondID)
	}
	if s.Runs[secondID].Tasks[1] == nil {
		t.Fatalf("expected process event to update second attempt run %q", secondID)
	}
}

// --- process_submitted ---

func TestHandleEvent_ProcessSubmitted_UpsertsTask(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r",
		RunID:   "run-1",
		Event:   "started",
		UTCTime: "2024-01-15T10:30:00Z",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r",
		RunID:   "run-1",
		Event:   "process_submitted",
		UTCTime: "2024-01-15T10:30:01Z",
		Trace: &Trace{
			TaskID:  1,
			Hash:    "ab/cdef01",
			Name:    "sayHello (1)",
			Process: "sayHello",
			Status:  "SUBMITTED",
		},
	})

	r := s.Runs["run-1"]
	task := r.Tasks[1]
	if task == nil {
		t.Fatal("expected task 1 to be created")
	}
	if task.TaskID != 1 {
		t.Errorf("TaskID = %d, want 1", task.TaskID)
	}
	if task.Hash != "ab/cdef01" {
		t.Errorf("Hash = %q, want %q", task.Hash, "ab/cdef01")
	}
	if task.Name != "sayHello (1)" {
		t.Errorf("Name = %q, want %q", task.Name, "sayHello (1)")
	}
	if task.Process != "sayHello" {
		t.Errorf("Process = %q, want %q", task.Process, "sayHello")
	}
	if task.Status != "SUBMITTED" {
		t.Errorf("Status = %q, want %q", task.Status, "SUBMITTED")
	}
}

// --- process_started ---

func TestHandleEvent_ProcessStarted_UpdatesTask(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "process_submitted", UTCTime: "t1",
		Trace: &Trace{TaskID: 1, Hash: "ab/01", Name: "say (1)", Process: "say", Status: "SUBMITTED"},
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "process_started", UTCTime: "t2",
		Trace: &Trace{TaskID: 1, Hash: "ab/01", Name: "say (1)", Process: "say", Status: "RUNNING"},
	})

	task := s.Runs["run-1"].Tasks[1]
	if task.Status != "RUNNING" {
		t.Errorf("Status = %q, want %q after process_started", task.Status, "RUNNING")
	}
}

// --- process_completed ---

func TestHandleEvent_ProcessCompleted_UpdatesTask(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "process_completed", UTCTime: "t3",
		Trace: &Trace{TaskID: 5, Hash: "cd/99", Name: "align (1)", Process: "align", Status: "COMPLETED"},
	})

	task := s.Runs["run-1"].Tasks[5]
	if task == nil {
		t.Fatal("expected task 5 to be created")
	}
	if task.Status != "COMPLETED" {
		t.Errorf("Status = %q, want %q", task.Status, "COMPLETED")
	}
}

// --- process_failed ---

func TestHandleEvent_ProcessFailed_UpdatesTask(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "process_failed", UTCTime: "t3",
		Trace: &Trace{TaskID: 7, Hash: "ff/00", Name: "bad (1)", Process: "bad", Status: "FAILED"},
	})

	task := s.Runs["run-1"].Tasks[7]
	if task == nil {
		t.Fatal("expected task 7 to be created")
	}
	if task.Status != "FAILED" {
		t.Errorf("Status = %q, want %q", task.Status, "FAILED")
	}
}

func TestHandleEvent_ProcessEvents_CreateLiveWebhookTasksWithWorkdirProvenance(t *testing.T) {
	cases := []struct {
		name   string
		event  string
		status TaskStatus
	}{
		{name: "submitted", event: "process_submitted", status: TaskStatusSubmitted},
		{name: "started", event: "process_started", status: TaskStatusRunning},
		{name: "completed", event: "process_completed", status: TaskStatusCompleted},
		{name: "failed", event: "process_failed", status: TaskStatusFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore()
			s.HandleEvent(WebhookEvent{RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0"})

			monitorRunID := s.HandleEvent(WebhookEvent{
				RunName: "r", RunID: "run-1", Event: tc.event, UTCTime: "t1",
				Trace: &Trace{
					TaskID:  10,
					Hash:    "ab/cdef01",
					Name:    "align (1)",
					Process: "align",
					Status:  tc.status,
					Workdir: "/work/ab/cdef01",
				},
			})

			if monitorRunID != "run-1" {
				t.Fatalf("HandleEvent returned monitor run ID %q, want %q", monitorRunID, "run-1")
			}
			task := s.Runs["run-1"].Tasks[10]
			if task == nil {
				t.Fatal("expected task 10 to be created")
			}
			if task.Source != TaskSourceLiveWebhook {
				t.Errorf("Source = %q, want %q", task.Source, TaskSourceLiveWebhook)
			}
			if task.Workdir != "/work/ab/cdef01" {
				t.Errorf("Workdir = %q, want %q", task.Workdir, "/work/ab/cdef01")
			}
			if task.WorkdirProvenance != WorkdirProvenanceLiveWebhook {
				t.Errorf("WorkdirProvenance = %q, want %q", task.WorkdirProvenance, WorkdirProvenanceLiveWebhook)
			}
			if task.Status != tc.status {
				t.Errorf("Status = %q, want %q", task.Status, tc.status)
			}
		})
	}
}

// --- "completed" event ---

func TestHandleEvent_Completed_SetsRunStatus(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "completed", UTCTime: "t5",
	})

	if s.Runs["run-1"].Status != "completed" {
		t.Errorf("Status = %q, want %q", s.Runs["run-1"].Status, "completed")
	}
}

// --- "error" event ---

func TestHandleEvent_Error_SetsRunStatus(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "started", UTCTime: "t0",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "error", UTCTime: "t5",
	})

	if s.Runs["run-1"].Status != "failed" {
		t.Errorf("Status = %q, want %q", s.Runs["run-1"].Status, "failed")
	}
}

// --- Out-of-order: process event before "started" ---

func TestHandleEvent_OutOfOrder_ProcessBeforeStarted(t *testing.T) {
	s := NewStore()
	// Send a process_submitted before "started" — Run should be auto-created
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "process_submitted", UTCTime: "t1",
		Trace: &Trace{TaskID: 1, Hash: "ab/01", Name: "say (1)", Process: "say", Status: "SUBMITTED"},
	})

	r := s.Runs["run-1"]
	if r == nil {
		t.Fatal("expected Run to be auto-created for out-of-order process event")
	}
	if r.Tasks[1] == nil {
		t.Fatal("expected task 1 to exist")
	}
	if r.Tasks[1].Status != "SUBMITTED" {
		t.Errorf("Status = %q, want %q", r.Tasks[1].Status, "SUBMITTED")
	}
}

// --- Unknown event type is a no-op ---

func TestHandleEvent_UnknownEvent_NoOp(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "r", RunID: "run-1", Event: "unknown_event_type", UTCTime: "t0",
	})
	if len(s.Runs) != 0 {
		t.Errorf("expected no runs for unknown event, got %d", len(s.Runs))
	}
}

// --- Integration test with fixture ---

func TestHandleEvent_HelloRunFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/hello_run.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var events []WebhookEvent
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshalling fixture: %v", err)
	}

	s := NewStore()
	for _, ev := range events {
		s.HandleEvent(ev)
	}

	// Should have exactly 1 run
	if len(s.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(s.Runs))
	}

	r := s.Runs["a1b2c3d4-e5f6-7890-abcd-ef1234567890"]
	if r == nil {
		t.Fatal("expected run with ID a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}

	if r.RunName != "happy_darwin" {
		t.Errorf("RunName = %q, want %q", r.RunName, "happy_darwin")
	}
	if r.Status != "completed" {
		t.Errorf("Status = %q, want %q", r.Status, "completed")
	}
	if r.StartTime != "2024-01-15T10:30:00Z" {
		t.Errorf("StartTime = %q, want %q", r.StartTime, "2024-01-15T10:30:00Z")
	}

	// Should have 4 tasks, all COMPLETED
	if len(r.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(r.Tasks))
	}
	for id := 1; id <= 4; id++ {
		task := r.Tasks[id]
		if task == nil {
			t.Errorf("task %d missing", id)
			continue
		}
		if task.Status != "COMPLETED" {
			t.Errorf("task %d: Status = %q, want %q", id, task.Status, "COMPLETED")
		}
		if task.Process != "sayHello" {
			t.Errorf("task %d: Process = %q, want %q", id, task.Process, "sayHello")
		}
	}
}
