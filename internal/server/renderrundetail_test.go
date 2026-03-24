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
	// No process groups
	if strings.Contains(got, `class="process-group"`) {
		t.Error("expected no process groups when tasks are empty")
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

func TestRenderRunDetail_StartTimeSignal(t *testing.T) {
	run := &state.Run{
		RunName:   "happy_euler",
		RunID:     "run1",
		Status:    "started",
		StartTime: "2024-01-01T00:00:00Z",
		Tasks:     map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `data-signals:start-time="'2024-01-01T00:00:00Z'"`) {
		t.Errorf("expected data-signals:start-time attribute, got:\n%s", got)
	}
}

func TestRenderRunDetail_NoStartTime_NoSignal(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if strings.Contains(got, `data-signals:start-time`) {
		t.Errorf("expected no data-signals:start-time when StartTime is empty, got:\n%s", got)
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

func TestRenderRunDetail_StatusErrorProgressFill(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "error",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="badge status-error"`) {
		t.Errorf("expected badge with status-error class, got:\n%s", got)
	}
	if !strings.Contains(got, `progress-fill status-failed`) {
		t.Errorf("expected progress-fill to have status-failed class on error, got:\n%s", got)
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
	// One process group
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
	if !strings.Contains(got, `<span class="process-counts">1/3</span>`) {
		t.Errorf("expected process-counts 1/3, got:\n%s", got)
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

	// Two process groups
	groupCount := strings.Count(got, `class="process-group"`)
	if groupCount != 2 {
		t.Errorf("expected 2 process groups, got %d\n%s", groupCount, got)
	}
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

func TestRenderRunDetail_ProcessGroupContainerClass(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	if !strings.Contains(got, `class="process-group-container`) {
		t.Errorf("expected process-group-container class, got:\n%s", got)
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

	expected := `data-on:click="$expandedGroup = $expandedGroup === 'sayHello' ? '' : 'sayHello'"`
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

func TestRenderRunDetail_TaskListDataShow(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	expected := `<div class="task-list" data-show="$expandedGroup === 'sayHello'">`
	if !strings.Contains(got, expected) {
		t.Errorf("expected task-list div with data-show for sayHello, got:\n%s", got)
	}
}

func TestRenderRunDetail_TaskListMultipleProcesses(t *testing.T) {
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
	if !strings.Contains(got, `class="process-group-container group-has-failed"`) {
		t.Errorf("expected group-has-failed class on container, got:\n%s", got)
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
	if !strings.Contains(got, `class="process-group-container group-has-running"`) {
		t.Errorf("expected group-has-running class on container, got:\n%s", got)
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

func TestRenderRunDetail_TaskRowsInsideTaskList(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
		},
	}
	got := serverForDetail().renderRunDetail(run)

	taskListStart := strings.Index(got, `<div class="task-list" data-show="$expandedGroup === 'sayHello'">`)
	if taskListStart == -1 {
		t.Fatalf("expected task-list div, got:\n%s", got)
	}

	afterTaskList := got[taskListStart:]
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

func TestRenderRunDetail_MiniBar(t *testing.T) {
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

	if !strings.Contains(got, `class="mini-bar"`) {
		t.Errorf("expected mini-bar in process group, got:\n%s", got)
	}
	if !strings.Contains(got, `class="mini-fill"`) {
		t.Errorf("expected mini-fill in process group, got:\n%s", got)
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

func TestRenderRunDetail_NoRunNameInHeader(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "started",
		Tasks:   map[int]*state.Task{},
	}
	got := serverForDetail().renderRunDetail(run)

	// Run name is shown in the sidebar, not duplicated in the detail header
	if strings.Contains(got, `<span class="run-name">`) {
		t.Errorf("run-name should not be in detail header (shown in sidebar), got:\n%s", got)
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
	if strings.Contains(got, `process-group`) {
		t.Errorf("expected no process-group when layout is set, got:\n%s", got)
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

func TestRenderRunDetail_NilLayout_ContainsProcessGroup(t *testing.T) {
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

	if !strings.Contains(got, `process-group`) {
		t.Errorf("expected process-group when layout is nil, got:\n%s", got)
	}
	if strings.Contains(got, `dag-view`) {
		t.Errorf("expected no dag-view when layout is nil, got:\n%s", got)
	}
}
