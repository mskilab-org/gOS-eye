package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// helper: build a minimal Server for renderRunDetail tests (no DAG layout)
func serverForDetail() *Server {
	return &Server{store: state.NewStore(), broker: NewBroker()}
}

func TestRenderRunDetail_NilRun(t *testing.T) {
	got := serverForDetail().renderRunDetail(nil)
	if got != "" {
		t.Errorf("expected empty string for nil run, got: %s", got)
	}
}

func TestRenderRunDetail_NoTasks_DefaultPipelineName(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	// Default pipeline name when ProjectName is empty
	if !strings.Contains(got, "<h1>Pipeline</h1>") {
		t.Errorf("expected <h1>Pipeline</h1> when ProjectName is empty, got:\n%s", got)
	}
	// Run name is shown in sidebar, not in detail header
	// Run header
	if !strings.Contains(got, `class="run-header"`) {
		t.Error("expected run-header div")
	}
	// Progress bar at 0/0 (0%)
	if !strings.Contains(got, `class="progress-bar"`) {
		t.Error("expected progress-bar")
	}
	if !strings.Contains(got, "0/0 (0%)") {
		t.Errorf("expected 0/0 (0%%) in progress, got:\n%s", got)
	}
	// No process table when tasks are empty
	if strings.Contains(got, `class="process-table"`) {
		t.Error("expected no process-table when tasks are empty")
	}
}

func TestRenderRunDetail_ProjectNameSet(t *testing.T) {
	run := &state.Run{
		RunName:     "happy_euler",
		RunID:       "run1",
		ProjectName: "nf-core/rnaseq",
		Status:      "started",
		Tasks:       map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, "<h1>nf-core/rnaseq</h1>") {
		t.Errorf("expected <h1>nf-core/rnaseq</h1>, got:\n%s", got)
	}
}

func TestRenderRunDetail_ElapsedTimer_Running(t *testing.T) {
	run := &state.Run{
		RunName:   "happy_euler",
		RunID:     "run1",
		Status:    "running",
		StartTime: "2024-01-01T00:00:00Z",
		Tasks:     map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	// Running run: live timer with start time embedded, empty complete time
	if !strings.Contains(got, `formatElapsed('2024-01-01T00:00:00Z', '')`) {
		t.Errorf("expected live elapsed timer with start time, got:\n%s", got)
	}
	if !strings.Contains(got, `class="elapsed-time"`) {
		t.Errorf("expected elapsed-time class, got:\n%s", got)
	}
}

func TestRenderRunDetail_ElapsedTimer_Completed(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-01T00:00:00Z",
		CompleteTime: "2024-01-01T00:00:31Z",
		Tasks:        map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	// Completed run: frozen timer with both timestamps
	if !strings.Contains(got, `formatElapsed('2024-01-01T00:00:00Z', '2024-01-01T00:00:31Z')`) {
		t.Errorf("expected frozen elapsed timer, got:\n%s", got)
	}
}

func TestRenderRunDetail_NoStartTime_NoTimer(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if strings.Contains(got, `elapsed-time`) {
		t.Errorf("expected no elapsed timer when StartTime is empty, got:\n%s", got)
	}
}

func TestRenderRunDetail_StatusBadge(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="badge status-started"`) {
		t.Errorf("expected badge with status-started class, got:\n%s", got)
	}
	if !strings.Contains(got, `>STARTED</span>`) {
		t.Errorf("expected STARTED label in badge, got:\n%s", got)
	}
}

func TestRenderRunDetail_StatusCompletedProgressFill(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "completed",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="badge status-completed"`) {
		t.Errorf("expected badge with status-completed class, got:\n%s", got)
	}
	if !strings.Contains(got, `progress-fill status-completed`) {
		t.Errorf("expected progress-fill to have status-completed class, got:\n%s", got)
	}
}

func TestRenderRunDetail_StatusFailedProgressFill(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "failed",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="badge status-failed"`) {
		t.Errorf("expected badge with status-failed class, got:\n%s", got)
	}
	if !strings.Contains(got, `progress-fill status-failed`) {
		t.Errorf("expected progress-fill to have status-failed class, got:\n%s", got)
	}
}

func TestRenderRunDetail_SingleTaskCompleted(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	// Progress 1/1 (100%)
	if !strings.Contains(got, "1/1 (100%)") {
		t.Errorf("expected 1/1 (100%%) in progress, got:\n%s", got)
	}
	// One process in the process table
	if !strings.Contains(got, `class="process-table"`) {
		t.Error("expected process-table")
	}
	if !strings.Contains(got, `<span class="process-table-name">sayHello</span>`) {
		t.Errorf("expected process-table-name sayHello, got:\n%s", got)
	}
	if !strings.Contains(got, `<span class="process-table-counts">1/1</span>`) {
		t.Errorf("expected process-table-counts 1/1, got:\n%s", got)
	}
}

func TestRenderRunDetail_MixedStatusTasks(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
			2: {TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "RUNNING"},
			3: {TaskID: 3, Name: "sayHello (3)", Process: "sayHello", Status: "SUBMITTED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	// 1 of 3 completed → 33%
	if !strings.Contains(got, "1/3 (33%)") {
		t.Errorf("expected 1/3 (33%%) in progress, got:\n%s", got)
	}
	if !strings.Contains(got, `<span class="process-table-counts">1/3</span>`) {
		t.Errorf("expected process-table-counts 1/3, got:\n%s", got)
	}
}

func TestRenderRunDetail_MultipleProcesses(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
			2: {TaskID: 2, Name: "count (1)", Process: "count", Status: "COMPLETED"},
			3: {TaskID: 3, Name: "count (2)", Process: "count", Status: "SUBMITTED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	// Two process rows in the process table
	groupCount := strings.Count(got, `class="process-table-row"`)
	if groupCount != 2 {
		t.Errorf("expected 2 process-table-rows, got %d\n%s", groupCount, got)
	}
	if !strings.Contains(got, `<span class="process-table-name">align</span>`) {
		t.Error("expected process-table-name 'align'")
	}
	if !strings.Contains(got, `<span class="process-table-name">count</span>`) {
		t.Error("expected process-table-name 'count'")
	}
	// Overall: 2 completed out of 3 → 66%
	if !strings.Contains(got, "2/3 (66%)") {
		t.Errorf("expected 2/3 (66%%) in progress, got:\n%s", got)
	}
}

func TestRenderRunDetail_ProcessTableGroupClass(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="process-table-group`) {
		t.Errorf("expected process-table-group class, got:\n%s", got)
	}
}

func TestRenderRunDetail_DataOnClickToggle(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	expected := `data-on:click="$_logAbort && $_logAbort.abort(); $_logAbort = null; $_logOpen = false; $expandedGroup = $expandedGroup === 'sayHello' ? '' : 'sayHello'"`
	if !strings.Contains(got, expected) {
		t.Errorf("expected data-on:click toggle for sayHello, got:\n%s", got)
	}
}

func TestRenderRunDetail_ChevronPresent(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="chevron"`) {
		t.Errorf("expected chevron span, got:\n%s", got)
	}
	if !strings.Contains(got, `data-class:expanded="$expandedGroup === 'sayHello'"`) {
		t.Errorf("expected data-class:expanded attribute on chevron, got:\n%s", got)
	}
}

func TestRenderRunDetail_TasksDataShow(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	expected := `<div class="process-table-tasks" data-show="$expandedGroup === 'sayHello'"`
	if !strings.Contains(got, expected) {
		t.Errorf("expected process-table-tasks div with data-show for sayHello, got:\n%s", got)
	}
}

func TestRenderRunDetail_TasksMultipleProcesses(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
			2: {TaskID: 2, Name: "count (1)", Process: "count", Status: "RUNNING"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `data-show="$expandedGroup === 'align'"`) {
		t.Errorf("expected data-show for align, got:\n%s", got)
	}
	if !strings.Contains(got, `data-show="$expandedGroup === 'count'"`) {
		t.Errorf("expected data-show for count, got:\n%s", got)
	}
}

func TestRenderRunDetail_GroupStatusIndicator_Failed(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
			2: {TaskID: 2, Name: "align (2)", Process: "align", Status: "FAILED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `<span class="group-status-indicator status-failed">●</span>`) {
		t.Errorf("expected red status-failed indicator dot, got:\n%s", got)
	}
	if !strings.Contains(got, `class="process-table-group group-has-failed`) {
		t.Errorf("expected group-has-failed class on process-table-group, got:\n%s", got)
	}
}

func TestRenderRunDetail_GroupStatusIndicator_Running(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "RUNNING"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `<span class="group-status-indicator status-running">●</span>`) {
		t.Errorf("expected blue status-running indicator dot, got:\n%s", got)
	}
	if !strings.Contains(got, `class="process-table-group group-has-running`) {
		t.Errorf("expected group-has-running class on process-table-group, got:\n%s", got)
	}
}

func TestRenderRunDetail_GroupStatusIndicator_AllCompleted(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
			2: {TaskID: 2, Name: "align (2)", Process: "align", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `<span class="group-status-indicator status-completed">●</span>`) {
		t.Errorf("expected green status-completed indicator dot, got:\n%s", got)
	}
}

func TestRenderRunDetail_GroupStatusIndicator_AllSubmitted(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "SUBMITTED"},
			2: {TaskID: 2, Name: "align (2)", Process: "align", Status: "SUBMITTED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `<span class="group-status-indicator status-pending">●</span>`) {
		t.Errorf("expected gray status-pending indicator dot, got:\n%s", got)
	}
}

func TestRenderRunDetail_GroupStatusIndicator_FailedTakesPriority(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "RUNNING"},
			2: {TaskID: 2, Name: "align (2)", Process: "align", Status: "FAILED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `<span class="group-status-indicator status-failed">●</span>`) {
		t.Errorf("expected status-failed indicator (priority over running), got:\n%s", got)
	}
	if strings.Contains(got, `status-running">●</span>`) {
		t.Errorf("should not have running indicator when failed exists, got:\n%s", got)
	}
}

func TestRenderRunDetail_TaskTableInsideProcessTable(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	tasksStart := strings.Index(got, `<div class="process-table-tasks"`)
	if tasksStart == -1 {
		t.Fatalf("expected process-table-tasks div, got:\n%s", got)
	}

	afterTasks := got[tasksStart:]
	if !strings.Contains(afterTasks, `class="task-table-row"`) {
		t.Errorf("expected task-table-row inside process-table-tasks, got:\n%s", afterTasks)
	}
	if !strings.Contains(afterTasks, `<span class="task-table-name">(1)</span>`) {
		t.Errorf("expected task-table-name span for '(1)' inside process-table-tasks, got:\n%s", afterTasks)
	}
	if !strings.Contains(afterTasks, `<span class="badge status-completed">COMPLETED</span>`) {
		t.Errorf("expected COMPLETED badge in task-table-row, got:\n%s", afterTasks)
	}
}

func TestRenderRunDetail_NoDashboardWrapper(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if strings.Contains(got, `id="dashboard"`) {
		t.Errorf("renderRunDetail should NOT include dashboard wrapper, got:\n%s", got)
	}
}

func TestRenderRunDetail_ResourceBarsInProcessTable(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED", Duration: 5000},
			2: {TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "SUBMITTED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="resource-bar-cell"`) {
		t.Errorf("expected resource-bar-cell in process table, got:\n%s", got)
	}
	if !strings.Contains(got, `class="resource-bar-fill"`) {
		t.Errorf("expected resource-bar-fill in process table, got:\n%s", got)
	}
}

func TestRenderRunDetail_ProgressFillWidth(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
			2: {TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "SUBMITTED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	// 1/2 = 50%
	if !strings.Contains(got, `style="width: 50%"`) {
		t.Errorf("expected progress fill width 50%%, got:\n%s", got)
	}
	if !strings.Contains(got, "1/2 (50%)") {
		t.Errorf("expected 1/2 (50%%) progress label, got:\n%s", got)
	}
}

func TestRenderRunDetail_RunNameInHeader(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	// Run name should appear in the detail header
	if !strings.Contains(got, `<span class="run-name">happy_euler</span>`) {
		t.Errorf("expected run-name span with 'happy_euler' in header, got:\n%s", got)
	}
}

func TestRenderRunDetail_RunNameEmpty(t *testing.T) {
	run := &state.Run{
		RunName: "",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	// Run name span should be present with empty string
	if !strings.Contains(got, `<span class="run-name"></span>`) {
		t.Errorf("expected run-name span with empty value, got:\n%s", got)
	}
}

// --- DAG layout conditional tests ---

func TestRenderRunDetail_WithLayout_ContainsDAGView(t *testing.T) {
	layout := &dag.Layout{
		Nodes:      []dag.NodeLayout{{Name: "foo", Layer: 0, Index: 0}},
		Edges:      nil,
		LayerCount: 1,
		MaxWidth:   1,
	}
	srv := &Server{store: state.NewStore(), broker: NewBroker(), layouts: map[string]*dag.Layout{"myPipeline": layout}}
	run := &state.Run{
		RunName:     "test_run",
		RunID:       "run1",
		ProjectName: "myPipeline",
		Status:      "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "foo", Status: "COMPLETED"},
		},
	}
	got := srv.renderRunDetail(run)

	if !strings.Contains(got, `dag-view`) {
		t.Errorf("expected dag-view when layout is set, got:\n%s", got)
	}
	// Run header and progress bar should still be present
	if !strings.Contains(got, `class="run-header"`) {
		t.Error("expected run-header to still be present with DAG layout")
	}
	if !strings.Contains(got, `class="progress-bar"`) {
		t.Error("expected progress-bar to still be present with DAG layout")
	}
	// Progress counts from run.Tasks
	if !strings.Contains(got, "1/1 (100%)") {
		t.Errorf("expected progress 1/1 (100%%) from run.Tasks, got:\n%s", got)
	}
	// Process table should also be present when layout is set (unified rendering)
	if !strings.Contains(got, `class="process-table"`) {
		t.Errorf("expected process-table when layout is set, got:\n%s", got)
	}
}

func TestRenderRunDetail_ProcessTableWithLayout(t *testing.T) {
	layout := &dag.Layout{
		Nodes:      []dag.NodeLayout{{Name: "align", Layer: 0, Index: 0}},
		Edges:      nil,
		LayerCount: 1,
		MaxWidth:   1,
	}
	srv := &Server{store: state.NewStore(), broker: NewBroker(), layouts: map[string]*dag.Layout{"pipeline1": layout}}
	run := &state.Run{
		RunName:     "test_run",
		RunID:       "run1",
		ProjectName: "pipeline1",
		Status:      "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "align", Status: "RUNNING"},
			2: {TaskID: 2, Process: "align", Status: "COMPLETED"},
		},
	}
	got := srv.renderRunDetail(run)

	// process-table is always rendered (unified rendering replaces dag-task-panel)
	if !strings.Contains(got, `class="process-table"`) {
		t.Errorf("expected process-table when layout is set, got:\n%s", got)
	}
	// The table should contain the process name
	if !strings.Contains(got, `process-table-name`) {
		t.Errorf("expected process-table-name inside table, got:\n%s", got)
	}
}

func TestRenderRunDetail_NoDAGWithoutLayout(t *testing.T) {
	// No layout → no DAG view, but process table is still rendered
	srv := &Server{store: state.NewStore(), broker: NewBroker(), layouts: map[string]*dag.Layout{}}
	run := &state.Run{
		RunName:     "test_run",
		RunID:       "run1",
		ProjectName: "myPipeline",
		Status:      "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "baz", Status: "RUNNING"},
		},
	}
	got := srv.renderRunDetail(run)

	if strings.Contains(got, `dag-view`) {
		t.Errorf("expected no dag-view when layout is nil, got:\n%s", got)
	}
	if !strings.Contains(got, `class="process-table"`) {
		t.Errorf("expected process-table even without layout, got:\n%s", got)
	}
}

func TestRenderRunDetail_WithLayout_NilRun(t *testing.T) {
	layout := &dag.Layout{
		Nodes:      []dag.NodeLayout{{Name: "bar", Layer: 0, Index: 0}},
		Edges:      nil,
		LayerCount: 1,
		MaxWidth:   1,
	}
	srv := &Server{store: state.NewStore(), broker: NewBroker(), layouts: map[string]*dag.Layout{"bar": layout}}
	got := srv.renderRunDetail(nil)

	// nil run returns empty — DAG doesn't change that
	if got != "" {
		t.Errorf("expected empty string for nil run even with layout, got: %s", got)
	}
}

func TestRenderRunDetail_NilLayout_ContainsProcessTable(t *testing.T) {
	srv := &Server{store: state.NewStore(), broker: NewBroker(), layouts: map[string]*dag.Layout{}}
	run := &state.Run{
		RunName:     "test_run",
		RunID:       "run1",
		ProjectName: "myPipeline",
		Status:      "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "baz", Status: "RUNNING"},
		},
	}
	got := srv.renderRunDetail(run)

	if !strings.Contains(got, `class="process-table"`) {
		t.Errorf("expected process-table when layout is nil, got:\n%s", got)
	}
	if strings.Contains(got, `dag-view`) {
		t.Errorf("expected no dag-view when layout is nil, got:\n%s", got)
	}
}
