package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderTaskTable_EmptyTasks(t *testing.T) {
	got := renderTaskTable("sayHello", nil, "run-1")
	if got != "" {
		t.Fatalf("expected empty string for nil tasks, got %q", got)
	}
	got = renderTaskTable("sayHello", []*state.Task{}, "run-1")
	if got != "" {
		t.Fatalf("expected empty string for empty tasks, got %q", got)
	}
}

func TestRenderTaskTable_HeaderPresent(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 1, Name: "sayHello (1)", Status: "COMPLETED", Duration: 5000, CPUPercent: 50.0, PeakRSS: 1048576},
	}
	got := renderTaskTable("sayHello", tasks, "run-1")

	if !strings.Contains(got, `class="task-table-header"`) {
		t.Fatal("missing task-table-header")
	}
	// Header should have 5 column labels (2 empty + duration, cpu, memory)
	for _, label := range []string{"duration", "cpu", "memory"} {
		want := fmt.Sprintf(`<span class="resource-col-label">%s</span>`, label)
		if !strings.Contains(got, want) {
			t.Fatalf("missing header label %q in:\n%s", label, got)
		}
	}
	// Two empty labels for chevron + name columns
	emptyLabel := `<span class="resource-col-label"></span>`
	if strings.Count(got, emptyLabel) < 2 {
		t.Fatalf("expected at least 2 empty resource-col-labels, got %d in:\n%s", strings.Count(got, emptyLabel), got)
	}
}

func TestRenderTaskTable_SingleCompleted(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID:     7,
			Name:       "sayHello (1)",
			Status:     "COMPLETED",
			Duration:   5000,
			CPUPercent: 50.0,
			PeakRSS:    1048576,
			Exit:       0,
			Workdir:    "/work/ab/cd1234",
			Submit:     1700000000000,
			Start:      1700000001000,
			Complete:   1700000006000,
		},
	}
	got := renderTaskTable("sayHello", tasks, "run-1")

	// Outer wrapper
	if !strings.Contains(got, `class="task-table"`) {
		t.Fatal("missing task-table wrapper")
	}
	// Row class
	if !strings.Contains(got, `class="task-table-row"`) {
		t.Fatal("missing task-table-row")
	}
	// Chevron
	if !strings.Contains(got, `class="chevron"`) {
		t.Fatal("missing chevron")
	}
	if !strings.Contains(got, "▶") {
		t.Fatal("missing chevron character ▶")
	}
	// Name stripped to parens part
	if !strings.Contains(got, `<span class="task-table-name">(1)</span>`) {
		t.Fatalf("expected stripped name (1), got:\n%s", got)
	}
	// Status badge
	if !strings.Contains(got, `class="badge status-completed"`) {
		t.Fatal("missing status badge")
	}
	if !strings.Contains(got, `>COMPLETED</span>`) {
		t.Fatal("missing COMPLETED text in badge")
	}
	// Resource bar cells (3 of them)
	if strings.Count(got, `class="resource-bar-cell"`) != 3 {
		t.Fatalf("expected 3 resource-bar-cells, got %d", strings.Count(got, `class="resource-bar-cell"`))
	}
	// Duration bar: single task is max → 100% width
	if !strings.Contains(got, `style="width: 100%"`) {
		t.Fatalf("expected 100%% bar for max value, got:\n%s", got)
	}
	// Duration label
	if !strings.Contains(got, "5.0s") {
		t.Fatalf("expected duration label 5.0s, got:\n%s", got)
	}
	// CPU label
	if !strings.Contains(got, "50%") {
		t.Fatalf("expected CPU label 50%%, got:\n%s", got)
	}
	// Memory label
	if !strings.Contains(got, "1.0 MB") {
		t.Fatalf("expected memory label 1.0 MB, got:\n%s", got)
	}
	// Detail panel
	if !strings.Contains(got, `class="task-detail"`) {
		t.Fatal("missing task-detail panel")
	}
	if !strings.Contains(got, `data-show="$expandedTask === 7"`) {
		t.Fatal("missing data-show for detail panel")
	}
	// Detail panel contains exit code, workdir, timestamps
	if !strings.Contains(got, "Exit Code") {
		t.Fatal("missing Exit Code in detail")
	}
	if !strings.Contains(got, "/work/ab/cd1234") {
		t.Fatal("missing workdir in detail")
	}
	if !strings.Contains(got, "Submitted") {
		t.Fatal("missing Submitted timestamp label")
	}
	if !strings.Contains(got, "Started") {
		t.Fatal("missing Started timestamp label")
	}
	if !strings.Contains(got, "Completed") {
		t.Fatal("missing Completed timestamp label")
	}
	// Detail panel should NOT contain CPU/Memory/Peak Memory labels
	detailIdx := strings.Index(got, `class="task-detail"`)
	detailSection := got[detailIdx:]
	if strings.Contains(detailSection, `"detail-label">CPU<`) {
		t.Fatal("detail panel should not contain CPU label")
	}
	if strings.Contains(detailSection, `"detail-label">Memory<`) {
		t.Fatal("detail panel should not contain Memory label")
	}
	if strings.Contains(detailSection, `"detail-label">Peak Memory<`) {
		t.Fatal("detail panel should not contain Peak Memory label")
	}
}

func TestRenderTaskTable_MultipleCompletedBarsScale(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 1, Name: "ALIGN (sample_01)", Status: "COMPLETED", Duration: 120000, CPUPercent: 80.0, PeakRSS: 2097152},
		{TaskID: 2, Name: "ALIGN (sample_02)", Status: "COMPLETED", Duration: 60000, CPUPercent: 40.0, PeakRSS: 1048576},
	}
	got := renderTaskTable("ALIGN", tasks, "run-1")

	// Largest task gets 100% bar
	if !strings.Contains(got, `style="width: 100%"`) {
		t.Fatalf("expected 100%% bar for max value, got:\n%s", got)
	}
	// Smaller task's duration: 60000/120000 = 50%
	if !strings.Contains(got, `style="width: 50%"`) {
		t.Fatalf("expected 50%% bar for half-max value, got:\n%s", got)
	}
}

func TestRenderTaskTable_RunningTask(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 3, Name: "process (1)", Status: "RUNNING", Duration: 0, CPUPercent: 0, PeakRSS: 0},
	}
	got := renderTaskTable("process", tasks, "run-1")

	// Row should exist
	if !strings.Contains(got, `class="task-table-row"`) {
		t.Fatal("missing row for running task")
	}
	// Status badge
	if !strings.Contains(got, `class="badge status-running"`) {
		t.Fatal("missing running status badge")
	}
	// Bars should be 0% width
	if !strings.Contains(got, `style="width: 0%"`) {
		t.Fatalf("expected 0%% bars for running task, got:\n%s", got)
	}
	// Labels should show "···"
	if strings.Count(got, `>···</span>`) != 3 {
		t.Fatalf("expected 3 '···' labels for running task, got %d in:\n%s", strings.Count(got, `>···</span>`), got)
	}
}

func TestRenderTaskTable_MixedCompletedAndRunning(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 1, Name: "proc (1)", Status: "COMPLETED", Duration: 10000, CPUPercent: 90.0, PeakRSS: 5242880},
		{TaskID: 2, Name: "proc (2)", Status: "RUNNING", Duration: 0, CPUPercent: 0, PeakRSS: 0},
	}
	got := renderTaskTable("proc", tasks, "run-1")

	// Completed task should have real bar values
	if !strings.Contains(got, `style="width: 100%"`) {
		t.Fatal("completed task should have 100% bar")
	}
	if !strings.Contains(got, "10.0s") {
		t.Fatal("completed task should have duration label")
	}
	// Running task should have "···" labels
	if !strings.Contains(got, `>···</span>`) {
		t.Fatal("running task should show ··· labels")
	}
	if !strings.Contains(got, `style="width: 0%"`) {
		t.Fatal("running task should have 0% bar")
	}
}

func TestRenderTaskTable_NameStripping(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"sayHello (1)", "(1)"},
		{"ALIGN (sample_01)", "(sample_01)"},
		{"singleProcess", "singleProcess"},
	}
	for _, tc := range tests {
		tasks := []*state.Task{
			{TaskID: 1, Name: tc.name, Status: "COMPLETED", Duration: 1000},
		}
		got := renderTaskTable("proc", tasks, "run-1")
		want := fmt.Sprintf(`<span class="task-table-name">%s</span>`, tc.want)
		if !strings.Contains(got, want) {
			t.Errorf("name %q: expected %q in output, got:\n%s", tc.name, want, got)
		}
	}
}

func TestRenderTaskTable_FailedTask(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID:   5,
			Name:     "proc (1)",
			Status:   "FAILED",
			Duration: 2000,
			Exit:     1,
			Workdir:  "/work/xx/yy",
			Submit:   1700000000000,
			Start:    1700000001000,
			Complete: 1700000003000,
		},
	}
	got := renderTaskTable("proc", tasks, "run-1")

	// Row should have "failed" class
	if !strings.Contains(got, `task-table-row failed`) {
		t.Fatalf("expected 'task-table-row failed' class, got:\n%s", got)
	}
	// Status badge
	if !strings.Contains(got, `class="badge status-failed"`) {
		t.Fatal("missing failed status badge")
	}
	if !strings.Contains(got, `>FAILED</span>`) {
		t.Fatal("missing FAILED text")
	}
	// Exit code should have exit-error class
	if !strings.Contains(got, `exit-error`) {
		t.Fatalf("expected exit-error class for non-zero exit code, got:\n%s", got)
	}
}

func TestRenderTaskTable_ClickHandler(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 42, Name: "proc (1)", Status: "COMPLETED", Duration: 1000},
	}
	got := renderTaskTable("proc", tasks, "run-1")

	// Click handler toggles expanded state and lazy-fetches logs
	if !strings.Contains(got, `$expandedTask = $expandedTask === 42 ? 0 : 42`) {
		t.Fatalf("missing expandedTask toggle for taskID 42, got:\n%s", got)
	}
	if !strings.Contains(got, `/sse/task/run-1/42/logs`) {
		t.Fatalf("missing lazy log fetch URL for taskID 42, got:\n%s", got)
	}
	// Chevron data-class binding
	wantChevron := `data-class:expanded="$expandedTask === 42"`
	if !strings.Contains(got, wantChevron) {
		t.Fatalf("missing chevron expanded binding, got:\n%s", got)
	}
}

func TestRenderTaskTable_DetailPanel(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID:   10,
			Name:     "proc (1)",
			Status:   "COMPLETED",
			Duration: 5000,
			Exit:     0,
			Workdir:  "/work/ab/cd1234",
			Submit:   1700000000000,
			Start:    1700000001000,
			Complete: 1700000006000,
		},
	}
	got := renderTaskTable("proc", tasks, "run-1")

	// Detail panel
	if !strings.Contains(got, `data-show="$expandedTask === 10"`) {
		t.Fatal("missing data-show on detail panel")
	}
	if !strings.Contains(got, `class="detail-grid"`) {
		t.Fatal("missing detail-grid")
	}

	// Required detail fields
	for _, label := range []string{"Exit Code", "Work Dir", "Submitted", "Started", "Completed"} {
		want := fmt.Sprintf(`<span class="detail-label">%s</span>`, label)
		if !strings.Contains(got, want) {
			t.Fatalf("missing detail label %q in:\n%s", label, got)
		}
	}

	// Exit code value: 0 should NOT have exit-error class
	// Find the exit code span
	if strings.Contains(got, `exit-error`) {
		t.Fatal("exit code 0 should not have exit-error class")
	}

	// Workdir value
	if !strings.Contains(got, `class="detail-value workdir"`) {
		t.Fatal("missing workdir class")
	}

	// Should NOT contain CPU/Memory/Peak Memory
	detailIdx := strings.Index(got, `class="task-detail"`)
	detailEnd := strings.Index(got[detailIdx:], `</div></div>`)
	detailSection := got[detailIdx : detailIdx+detailEnd+12]
	for _, forbidden := range []string{">CPU<", ">Memory<", ">Peak Memory<"} {
		if strings.Contains(detailSection, forbidden) {
			t.Fatalf("detail panel should not contain %q", forbidden)
		}
	}
}

func TestRenderTaskTable_BarFloor(t *testing.T) {
	// Small value relative to max should get at least 5% width
	tasks := []*state.Task{
		{TaskID: 1, Name: "proc (1)", Status: "COMPLETED", Duration: 100000, CPUPercent: 100.0, PeakRSS: 10485760},
		{TaskID: 2, Name: "proc (2)", Status: "COMPLETED", Duration: 1000, CPUPercent: 1.0, PeakRSS: 10240},
	}
	got := renderTaskTable("proc", tasks, "run-1")

	// Duration: 1000/100000 = 1% → floor to 5%
	if !strings.Contains(got, `style="width: 5%"`) {
		t.Fatalf("expected 5%% floor bar for small value, got:\n%s", got)
	}
}

func TestRenderTaskTable_SubmittedTask(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 9, Name: "proc (1)", Status: "SUBMITTED", Duration: 0, CPUPercent: 0, PeakRSS: 0},
	}
	got := renderTaskTable("proc", tasks, "run-1")

	// Row present
	if !strings.Contains(got, `class="task-table-row"`) {
		t.Fatal("missing row for submitted task")
	}
	if !strings.Contains(got, `class="badge status-submitted"`) {
		t.Fatal("missing submitted status badge")
	}
	// ··· labels
	if strings.Count(got, `>···</span>`) != 3 {
		t.Fatalf("expected 3 '···' labels for submitted task, got %d", strings.Count(got, `>···</span>`))
	}
}
