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

func TestRenderDashboard_NoRuns(t *testing.T) {
	store := state.NewStore()
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.Contains(got, `id="dashboard"`) {
		t.Error(`expected id="dashboard" in output`)
	}
	// container class is on the parent wrapper in index.html, not on #dashboard
	if strings.Contains(got, `class="container"`) {
		t.Error(`unexpected class="container" on #dashboard div — should be on parent wrapper`)
	}
	if !strings.Contains(got, "Waiting for pipeline events...") {
		t.Error("expected waiting message when no runs exist")
	}
	if !strings.Contains(got, `class="waiting"`) {
		t.Error(`expected class="waiting" on waiting paragraph`)
	}
	// Should NOT have run-header or progress-bar
	if strings.Contains(got, `class="run-header"`) {
		t.Error("should not have run-header when no runs")
	}
}

func TestRenderDashboard_RunWithNoTasks(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Header present with default "Pipeline" name
	if !strings.Contains(got, `class="run-header"`) {
		t.Error("expected run-header when run exists")
	}
	if !strings.Contains(got, "<h1>Pipeline</h1>") {
		t.Errorf("expected <h1>Pipeline</h1> when ProjectName is empty, got:\n%s", got)
	}
	if !strings.Contains(got, "happy_euler") {
		t.Error("expected run name in output")
	}
	// Progress bar at 0/0 (0%)
	if !strings.Contains(got, `class="progress-bar"`) {
		t.Error("expected progress-bar")
	}
	if !strings.Contains(got, "0/0 (0%)") {
		t.Errorf("expected 0/0 (0%%) in progress, got:\n%s", got)
	}
	// No process groups
	if strings.Contains(got, `class="process-group"`) {
		t.Error("expected no process groups when tasks are empty")
	}
}

func TestRenderDashboard_SingleTaskCompleted(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "process_completed",
		Trace: &state.Trace{
			TaskID:  1,
			Name:    "sayHello (1)",
			Process: "sayHello",
			Status:  "COMPLETED",
		},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Progress 1/1 (100%)
	if !strings.Contains(got, "1/1 (100%)") {
		t.Errorf("expected 1/1 (100%%) in progress, got:\n%s", got)
	}
	// One process group with correct counts
	if !strings.Contains(got, `class="process-group"`) {
		t.Error("expected at least one process group")
	}
	if !strings.Contains(got, `<span class="process-name">sayHello</span>`) {
		t.Errorf("expected process-name sayHello, got:\n%s", got)
	}
	if !strings.Contains(got, `<span class="process-counts">1/1</span>`) {
		t.Errorf("expected process-counts 1/1, got:\n%s", got)
	}
}

func TestRenderDashboard_MultipleTasksMixedStatuses(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_started",
		Trace: &state.Trace{TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "RUNNING"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_submitted",
		Trace: &state.Trace{TaskID: 3, Name: "sayHello (3)", Process: "sayHello", Status: "SUBMITTED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// 1 of 3 completed → 33%
	if !strings.Contains(got, "1/3 (33%)") {
		t.Errorf("expected 1/3 (33%%) in progress, got:\n%s", got)
	}
	// Process group should show 1/3 (completed/total)
	if !strings.Contains(got, `<span class="process-counts">1/3</span>`) {
		t.Errorf("expected process-counts 1/3, got:\n%s", got)
	}
}

func TestRenderDashboard_MultipleProcesses(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 2, Name: "count (1)", Process: "count", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_submitted",
		Trace: &state.Trace{TaskID: 3, Name: "count (2)", Process: "count", Status: "SUBMITTED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Two process groups
	groupCount := strings.Count(got, `class="process-group"`)
	if groupCount != 2 {
		t.Errorf("expected 2 process groups, got %d\n%s", groupCount, got)
	}
	// align: 1/1
	if !strings.Contains(got, `<span class="process-name">align</span>`) {
		t.Error("expected process name 'align'")
	}
	if !strings.Contains(got, `<span class="process-name">count</span>`) {
		t.Error("expected process name 'count'")
	}
	// Overall: 2 completed out of 3 → 66%
	if !strings.Contains(got, "2/3 (66%)") {
		t.Errorf("expected 2/3 (66%%) in progress, got:\n%s", got)
	}
}

func TestRenderDashboard_ProjectNameSet(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler",
		RunID:   "run1",
		Event:   "started",
		UTCTime: "2024-01-01T00:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{
				ProjectName: "nf-core/rnaseq",
			},
		},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.Contains(got, "<h1>nf-core/rnaseq</h1>") {
		t.Errorf("expected <h1>nf-core/rnaseq</h1>, got:\n%s", got)
	}
}

func TestRenderDashboard_StatusCompletedBadge(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "completed",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.Contains(got, `class="badge status-completed"`) {
		t.Errorf("expected badge with status-completed class, got:\n%s", got)
	}
	// Progress fill should also have status-completed
	if !strings.Contains(got, `progress-fill status-completed`) {
		t.Errorf("expected progress-fill to have status-completed class, got:\n%s", got)
	}
}

func TestRenderDashboard_StatusErrorProgressFill(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "error",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Badge should say status-error
	if !strings.Contains(got, `class="badge status-error"`) {
		t.Errorf("expected badge with status-error class, got:\n%s", got)
	}
	// Progress fill should have status-failed (not status-error)
	if !strings.Contains(got, `progress-fill status-failed`) {
		t.Errorf("expected progress-fill to have status-failed class on error, got:\n%s", got)
	}
}

func TestRenderDashboard_StartTimeSignal(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.Contains(got, `data-signals:start-time="'2024-01-01T00:00:00Z'"`) {
		t.Errorf("expected data-signals:start-time attribute, got:\n%s", got)
	}
}

func TestRenderDashboard_HTMLStructure(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.HasPrefix(got, `<div id="dashboard"`) {
		t.Errorf("expected output to start with <div id=\"dashboard\", got:\n%.100s...", got)
	}
	if !strings.Contains(got, `class="run-header"`) {
		t.Error(`expected class="run-header" in output`)
	}
	if !strings.Contains(got, `class="progress-bar"`) {
		t.Error(`expected class="progress-bar" in output`)
	}
	if !strings.Contains(got, `class="process-group"`) {
		t.Error(`expected class="process-group" in output`)
	}
	if !strings.HasSuffix(got, "</div>") {
		t.Errorf("expected output to end with </div>, got:\n%s", got)
	}
}
