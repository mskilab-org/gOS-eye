package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// helper: build a Server with a given store and nil broker/mux
func serverWithStore(store *state.Store) *Server {
	return &Server{store: store}
}

func TestRenderTaskList_NoRuns(t *testing.T) {
	store := state.NewStore()
	s := serverWithStore(store)
	got := s.renderTaskList()

	if !strings.Contains(got, `id="task-list"`) {
		t.Error("expected id=\"task-list\" in output")
	}
	if !strings.Contains(got, "Waiting for pipeline events...") {
		t.Error("expected waiting message when no tasks exist")
	}
}

func TestRenderTaskList_RunWithNoTasks(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderTaskList()

	if !strings.Contains(got, `id="task-list"`) {
		t.Error("expected id=\"task-list\" in output")
	}
	if !strings.Contains(got, "Waiting for pipeline events...") {
		t.Error("expected waiting message when run has no tasks")
	}
}

func TestRenderTaskList_SingleTask(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID: 1,
			Name:   "sayHello (1)",
			Status: "COMPLETED",
		},
	})
	s := serverWithStore(store)
	got := s.renderTaskList()

	if !strings.Contains(got, `id="task-list"`) {
		t.Error("expected id=\"task-list\" in output")
	}
	if !strings.Contains(got, "<p>") {
		t.Error("expected <p> tag wrapping task")
	}
	if !strings.Contains(got, "sayHello (1): COMPLETED") {
		t.Errorf("expected task line 'sayHello (1): COMPLETED' in output, got:\n%s", got)
	}
	if strings.Contains(got, "Waiting for pipeline events...") {
		t.Error("should not show waiting message when tasks exist")
	}
}

func TestRenderTaskList_MultipleTasks(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID: 1,
			Name:   "sayHello (1)",
			Status: "COMPLETED",
		},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "process_started",
		Trace: &state.Trace{
			TaskID: 2,
			Name:   "sayHello (2)",
			Status: "RUNNING",
		},
	})
	s := serverWithStore(store)
	got := s.renderTaskList()

	if !strings.Contains(got, "sayHello (1): COMPLETED") {
		t.Errorf("expected task 'sayHello (1): COMPLETED' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "sayHello (2): RUNNING") {
		t.Errorf("expected task 'sayHello (2): RUNNING' in output, got:\n%s", got)
	}
	// Count <p> tags — should be at least 2
	count := strings.Count(got, "<p>")
	if count < 2 {
		t.Errorf("expected at least 2 <p> tags, got %d", count)
	}
}

func TestRenderTaskList_MultipleRuns(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_a",
		RunID:   "id_a",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID: 1,
			Name:   "align (1)",
			Status: "COMPLETED",
		},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_b",
		RunID:   "id_b",
		Event:   "process_submitted",
		Trace: &state.Trace{
			TaskID: 1,
			Name:   "count (1)",
			Status: "SUBMITTED",
		},
	})
	s := serverWithStore(store)
	got := s.renderTaskList()

	if !strings.Contains(got, "align (1): COMPLETED") {
		t.Errorf("expected 'align (1): COMPLETED', got:\n%s", got)
	}
	if !strings.Contains(got, "count (1): SUBMITTED") {
		t.Errorf("expected 'count (1): SUBMITTED', got:\n%s", got)
	}
}

func TestRenderTaskList_HTMLStructure(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID: 1,
			Name:   "sayHello (1)",
			Status: "COMPLETED",
		},
	})
	s := serverWithStore(store)
	got := s.renderTaskList()

	if !strings.HasPrefix(got, `<div id="task-list">`) {
		t.Errorf("expected output to start with <div id=\"task-list\">, got:\n%s", got)
	}
	if !strings.HasSuffix(got, "</div>") {
		t.Errorf("expected output to end with </div>, got:\n%s", got)
	}
}
