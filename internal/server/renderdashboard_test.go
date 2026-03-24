package server

import (
	"fmt"
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
	// Run name is shown in sidebar (renderRunList), not in the main dashboard detail
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

func TestRenderDashboard_StatusFailedProgressFill(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "error",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Badge should say FAILED with status-failed class
	if !strings.Contains(got, `class="badge status-failed"`) {
		t.Errorf("expected badge with status-failed class, got:\n%s", got)
	}
	// Progress fill should also have status-failed
	if !strings.Contains(got, `progress-fill status-failed`) {
		t.Errorf("expected progress-fill to have status-failed class, got:\n%s", got)
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

// --- Expand/collapse container structure tests ---

func TestRenderDashboard_ProcessGroupContainerClass(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.Contains(got, `class="process-group-container`) {
		t.Errorf("expected process-group-container class, got:\n%s", got)
	}
}

func TestRenderDashboard_DataOnClickToggle(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// data-on:click toggles $expandedGroup between the process name and empty string
	expected := `data-on:click="$expandedGroup = $expandedGroup === 'sayHello' ? '' : 'sayHello'"`
	if !strings.Contains(got, expected) {
		t.Errorf("expected data-on:click toggle for sayHello, got:\n%s", got)
	}
}

func TestRenderDashboard_ChevronPresent(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	if !strings.Contains(got, `class="chevron"`) {
		t.Errorf("expected chevron span, got:\n%s", got)
	}
	// Chevron should have data-class:expanded for rotation
	if !strings.Contains(got, `data-class:expanded="$expandedGroup === 'sayHello'"`) {
		t.Errorf("expected data-class:expanded attribute on chevron, got:\n%s", got)
	}
}

func TestRenderDashboard_TaskListDataShow(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// task-list div with data-show matching the process name
	expected := `<div class="task-list" data-show="$expandedGroup === 'sayHello'">`
	if !strings.Contains(got, expected) {
		t.Errorf("expected task-list div with data-show for sayHello, got:\n%s", got)
	}
}

func TestRenderDashboard_TaskListMultipleProcesses(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_started",
		Trace: &state.Trace{TaskID: 2, Name: "count (1)", Process: "count", Status: "RUNNING"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Each process gets its own task-list with its own data-show
	if !strings.Contains(got, `data-show="$expandedGroup === 'align'"`) {
		t.Errorf("expected data-show for align, got:\n%s", got)
	}
	if !strings.Contains(got, `data-show="$expandedGroup === 'count'"`) {
		t.Errorf("expected data-show for count, got:\n%s", got)
	}
}

func TestRenderDashboard_GroupStatusIndicator_Failed(t *testing.T) {
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
		Trace: &state.Trace{TaskID: 2, Name: "align (2)", Process: "align", Status: "FAILED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Red dot when any task FAILED
	if !strings.Contains(got, `<span class="group-status-indicator status-failed">●</span>`) {
		t.Errorf("expected red status-failed indicator dot, got:\n%s", got)
	}
	// Container should also have group-has-failed class
	if !strings.Contains(got, `class="process-group-container group-has-failed"`) {
		t.Errorf("expected group-has-failed class on container, got:\n%s", got)
	}
}

func TestRenderDashboard_GroupStatusIndicator_Running(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_started",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "RUNNING"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Blue dot when any task RUNNING (and none FAILED)
	if !strings.Contains(got, `<span class="group-status-indicator status-running">●</span>`) {
		t.Errorf("expected blue status-running indicator dot, got:\n%s", got)
	}
	// Container should have group-has-running class
	if !strings.Contains(got, `class="process-group-container group-has-running"`) {
		t.Errorf("expected group-has-running class on container, got:\n%s", got)
	}
}

func TestRenderDashboard_GroupStatusIndicator_AllCompleted(t *testing.T) {
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
		Trace: &state.Trace{TaskID: 2, Name: "align (2)", Process: "align", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Green dot when all tasks COMPLETED
	if !strings.Contains(got, `<span class="group-status-indicator status-completed">●</span>`) {
		t.Errorf("expected green status-completed indicator dot, got:\n%s", got)
	}
}

func TestRenderDashboard_GroupStatusIndicator_AllSubmitted(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_submitted",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "SUBMITTED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_submitted",
		Trace: &state.Trace{TaskID: 2, Name: "align (2)", Process: "align", Status: "SUBMITTED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Gray dot when no failed, no running, not all completed (i.e., all submitted)
	if !strings.Contains(got, `<span class="group-status-indicator status-pending">●</span>`) {
		t.Errorf("expected gray status-pending indicator dot, got:\n%s", got)
	}
}

func TestRenderDashboard_GroupStatusIndicator_FailedTakesPriority(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	// Mix of FAILED and RUNNING — FAILED takes priority
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_started",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "RUNNING"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 2, Name: "align (2)", Process: "align", Status: "FAILED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Failed takes priority over running
	if !strings.Contains(got, `<span class="group-status-indicator status-failed">●</span>`) {
		t.Errorf("expected status-failed indicator (priority over running), got:\n%s", got)
	}
	// Should NOT have running indicator
	if strings.Contains(got, `status-running">●</span>`) {
		t.Errorf("should not have running indicator when failed exists, got:\n%s", got)
	}
}

func TestRenderDashboard_TaskRowsInsideTaskList(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// The task-list div should contain renderTaskRows output
	// renderTaskRows produces task-row divs with task-name spans
	taskListStart := strings.Index(got, `<div class="task-list" data-show="$expandedGroup === 'sayHello'">`)
	if taskListStart == -1 {
		t.Fatalf("expected task-list div, got:\n%s", got)
	}

	// Find the content after the task-list opening tag
	afterTaskList := got[taskListStart:]

	// Task row content should be inside the task-list div
	if !strings.Contains(afterTaskList, `class="task-row"`) {
		t.Errorf("expected task-row inside task-list, got:\n%s", afterTaskList)
	}
	if !strings.Contains(afterTaskList, `<span class="task-name">sayHello (1)</span>`) {
		t.Errorf("expected task-name span for 'sayHello (1)' inside task-list, got:\n%s", afterTaskList)
	}
	if !strings.Contains(afterTaskList, `<span class="badge status-completed">COMPLETED</span>`) {
		t.Errorf("expected COMPLETED badge in task row, got:\n%s", afterTaskList)
	}
}

func TestRenderDashboard_TaskRowsCorrectProcess(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run1", RunID: "run1", Event: "process_started",
		Trace: &state.Trace{TaskID: 2, Name: "count (1)", Process: "count", Status: "RUNNING"},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Verify that each process's task-list contains only its own tasks
	// Find align's task-list section
	alignIdx := strings.Index(got, `data-show="$expandedGroup === 'align'"`)
	countIdx := strings.Index(got, `data-show="$expandedGroup === 'count'"`)
	if alignIdx == -1 || countIdx == -1 {
		t.Fatalf("expected both align and count task-list divs, got:\n%s", got)
	}

	// Between align's task-list start and count's container, we should see align (1) but not count (1)
	var alignSection string
	if alignIdx < countIdx {
		alignSection = got[alignIdx:countIdx]
	} else {
		alignSection = got[alignIdx:]
	}

	if !strings.Contains(alignSection, "align (1)") {
		t.Errorf("expected align (1) in align's task-list section")
	}
	if strings.Contains(alignSection, "count (1)") {
		t.Errorf("did not expect count (1) in align's task-list section")
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

// --- Multi-run orchestrator tests ---

func TestRenderDashboard_LatestRunSignalPresent(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Outer dashboard div should have data-signals:latest-run
	if !strings.Contains(got, `data-signals:latest-run="'run1'"`) {
		t.Errorf("expected data-signals:latest-run on dashboard div, got:\n%s", got)
	}
}

func TestRenderDashboard_SingleRun_NoSelector(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Run list is now rendered separately by renderSidebar, not inside dashboard
	if strings.Contains(got, `id="run-list"`) {
		t.Error("dashboard should not contain run-list (it's in the sidebar)")
	}
	// Should still have run content wrapped in data-show div
	if !strings.Contains(got, `data-show="($selectedRun || $latestRun) === 'run1'"`) {
		t.Errorf("expected data-show wrapper for single run, got:\n%s", got)
	}
}

func TestRenderDashboard_MultipleRuns_NoSelectorInDashboard(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_alpha", RunID: "runA", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_beta", RunID: "runB", Event: "started", UTCTime: "2024-01-02T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Run list is now rendered separately by renderSidebar, not inside dashboard
	if strings.Contains(got, `id="run-list"`) {
		t.Errorf("dashboard should not contain run-list (it's in the sidebar), got:\n%s", got)
	}
	// Should have data-show divs for both runs
	if !strings.Contains(got, `data-show="($selectedRun || $latestRun) === 'runA'"`) {
		t.Errorf("expected data-show wrapper for runA, got:\n%s", got)
	}
	if !strings.Contains(got, `data-show="($selectedRun || $latestRun) === 'runB'"`) {
		t.Errorf("expected data-show wrapper for runB, got:\n%s", got)
	}
}

func TestRenderDashboard_MultipleRuns_EachHasDetail(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_alpha", RunID: "runA", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
		Metadata: &state.Metadata{Workflow: state.WorkflowInfo{ProjectName: "projA"}},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_beta", RunID: "runB", Event: "started", UTCTime: "2024-01-02T00:00:00Z",
		Metadata: &state.Metadata{Workflow: state.WorkflowInfo{ProjectName: "projB"}},
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Each run's renderRunDetail output should be present (project names in headings)
	if !strings.Contains(got, "<h1>projA</h1>") {
		t.Errorf("expected projA heading in output, got:\n%s", got)
	}
	if !strings.Contains(got, "<h1>projB</h1>") {
		t.Errorf("expected projB heading in output, got:\n%s", got)
	}
	// Run names are shown in sidebar (renderRunList), not in the main dashboard detail
}

func TestRenderDashboard_MultipleRuns_LatestRunSignal(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_alpha", RunID: "runA", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "run_beta", RunID: "runB", Event: "started", UTCTime: "2024-01-02T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// Latest run signal should reflect the most recently updated run
	latestID := store.GetLatestRunID()
	expected := fmt.Sprintf(`data-signals:latest-run="'%s'"`, latestID)
	if !strings.Contains(got, expected) {
		t.Errorf("expected latest-run signal for %s, got:\n%s", latestID, got)
	}
}

func TestRenderDashboard_NoStartTimeOnOuterDiv(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	s := serverWithStore(store)
	got := s.renderDashboard()

	// The outer <div id="dashboard"> should NOT have data-signals:start-time
	// (that's now inside renderRunDetail on the run-header)
	dashIdx := strings.Index(got, `<div id="dashboard"`)
	if dashIdx == -1 {
		t.Fatal("expected dashboard div")
	}
	// Find the closing > of the opening dashboard tag
	closeIdx := strings.Index(got[dashIdx:], ">")
	openingTag := got[dashIdx : dashIdx+closeIdx+1]
	if strings.Contains(openingTag, `data-signals:start-time`) {
		t.Errorf("outer dashboard div should NOT have start-time signal, got opening tag:\n%s", openingTag)
	}
	// But the signal should still exist somewhere (from renderRunDetail)
	if !strings.Contains(got, `data-signals:start-time="'2024-01-01T00:00:00Z'"`) {
		t.Errorf("expected data-signals:start-time inside run detail, got:\n%s", got)
	}
}
