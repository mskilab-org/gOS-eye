package server

import (
	"fmt"
	"os"
	"path/filepath"
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
	// Detail panel should contain Process, Name, Duration, CPU, Memory labels
	detailIdx := strings.Index(got, `class="task-detail"`)
	detailSection := got[detailIdx:]
	for _, label := range []string{"Process", "Name", "Duration", "CPU", "Memory"} {
		want := fmt.Sprintf(`"detail-label">%s<`, label)
		if !strings.Contains(detailSection, want) {
			t.Fatalf("detail panel missing %q label in:\n%s", label, detailSection)
		}
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

func taskTableRowSection(t *testing.T, html string, taskID int) string {
	t.Helper()

	rowMarker := fmt.Sprintf(`data-on:click__stop="$expandedTask = $expandedTask === %d ? 0 : %d"`, taskID, taskID)
	markerIdx := strings.Index(html, rowMarker)
	if markerIdx < 0 {
		t.Fatalf("missing task row marker %q in:\n%s", rowMarker, html)
	}
	rowStart := strings.LastIndex(html[:markerIdx], `<div class="task-table-row`)
	if rowStart < 0 {
		t.Fatalf("missing task-table-row start before task %d in:\n%s", taskID, html)
	}
	detailMarker := fmt.Sprintf(`<div class="task-detail" data-show="$expandedTask === %d">`, taskID)
	detailIdx := strings.Index(html[markerIdx:], detailMarker)
	if detailIdx < 0 {
		t.Fatalf("missing task detail marker %q in:\n%s", detailMarker, html)
	}
	return html[rowStart : markerIdx+detailIdx]
}

func taskDetailGridSection(t *testing.T, html string, taskID int) string {
	t.Helper()

	detailMarker := fmt.Sprintf(`<div class="task-detail" data-show="$expandedTask === %d">`, taskID)
	detailIdx := strings.Index(html, detailMarker)
	if detailIdx < 0 {
		t.Fatalf("missing task detail marker %q in:\n%s", detailMarker, html)
	}
	gridMarker := `<div class="detail-grid">`
	gridStartRel := strings.Index(html[detailIdx:], gridMarker)
	if gridStartRel < 0 {
		t.Fatalf("missing detail-grid for task %d in:\n%s", taskID, html)
	}
	gridStart := detailIdx + gridStartRel
	gridEndRel := strings.Index(html[gridStart:], `</div>`)
	if gridEndRel < 0 {
		t.Fatalf("missing detail-grid close for task %d in:\n%s", taskID, html)
	}
	return html[gridStart : gridStart+gridEndRel]
}

func TestRenderTaskTable_CachedTaskDoesNotContributeResourceBars(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 1, Name: "process (executed)", Status: state.TaskStatusCompleted, Duration: 1000, CPUPercent: 10.0, PeakRSS: 1048576},
		{TaskID: 8, Name: "process (cached)", Status: state.TaskStatusCached, Duration: 5000, CPUPercent: 75.0, PeakRSS: 2097152},
	}
	got := renderTaskTable("process", tasks, "run-1")

	if !strings.Contains(got, `class="badge status-cached"`) {
		t.Fatal("missing cached status badge")
	}
	if !strings.Contains(got, `>CACHED</span>`) {
		t.Fatal("missing CACHED text in badge")
	}

	executedRow := taskTableRowSection(t, got, 1)
	if strings.Count(executedRow, `style="width: 100%"`) != 3 {
		t.Fatalf("executed task should scale against only current-run resource data, got row:\n%s", executedRow)
	}

	cachedRow := taskTableRowSection(t, got, 8)
	if strings.Count(cachedRow, `>···</span>`) != 3 {
		t.Fatalf("cached task should show placeholder resource labels in task row, got:\n%s", cachedRow)
	}
	if strings.Count(cachedRow, `style="width: 0%"`) != 3 {
		t.Fatalf("cached task should render zero-width resource bars in task row, got:\n%s", cachedRow)
	}
}

func TestRenderTaskTable_CachedProvenanceRowsInDetailGridEscaped(t *testing.T) {
	warning := `cached workdir unresolved: cannot scan "<missing>" & gave up`
	tasks := []*state.Task{
		{
			TaskID:            11,
			Name:              "proc (cached)",
			Process:           "proc",
			Status:            state.TaskStatusCached,
			Source:            state.TaskSourceCachedTrace,
			WorkdirProvenance: state.WorkdirProvenanceAmbiguousHash,
			WorkdirWarning:    warning,
		},
	}
	got := renderTaskTable("proc", tasks, "run-1")
	grid := taskDetailGridSection(t, got, 11)

	for _, want := range []string{
		`<span class="detail-label">Task Source</span><span class="detail-value">Cached trace import</span>`,
		`<span class="detail-label">Workdir Source</span><span class="detail-value">Ambiguous workdir hash</span>`,
		`<span class="detail-label">Workdir Warning</span><span class="detail-value">cached workdir unresolved: cannot scan &#34;&lt;missing&gt;&#34; &amp; gave up</span>`,
	} {
		if !strings.Contains(grid, want) {
			t.Fatalf("missing provenance detail row %q in detail grid:\n%s\nfull output:\n%s", want, grid, got)
		}
	}
	if strings.Contains(grid, warning) || strings.Contains(grid, `"<missing>"`) {
		t.Fatalf("workdir warning was not escaped in detail grid:\n%s", grid)
	}
}

func TestRenderTaskTable_UnknownProvenanceMetadataDoesNotRenderRows(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID:            12,
			Name:              "proc (1)",
			Process:           "proc",
			Status:            state.TaskStatusCompleted,
			Source:            state.TaskSource("future_source"),
			WorkdirProvenance: state.WorkdirProvenance("future_provenance"),
		},
	}
	got := renderTaskTable("proc", tasks, "run-1")
	grid := taskDetailGridSection(t, got, 12)

	for _, label := range []string{"Task Source", "Workdir Source", "Workdir Warning"} {
		if strings.Contains(grid, label) {
			t.Fatalf("unexpected provenance label %q for unknown/empty metadata in detail grid:\n%s", label, grid)
		}
	}
}

func TestRenderTaskTable_HashResolvedCachedWorkdirReadsCommandLogAndErr(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, ".command.log"), []byte("stdout <ok> & more\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .command.log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workdir, ".command.err"), []byte("stderr <warn> & more\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .command.err: %v", err)
	}
	tasks := []*state.Task{
		{
			TaskID:            13,
			Name:              "proc (cached)",
			Process:           "proc",
			Status:            state.TaskStatusCached,
			Source:            state.TaskSourceCachedTrace,
			Workdir:           workdir,
			WorkdirProvenance: state.WorkdirProvenanceHashResolved,
		},
	}
	got := renderTaskTable("proc", tasks, "run-1")
	grid := taskDetailGridSection(t, got, 13)

	if !strings.Contains(grid, `<span class="detail-label">Workdir Source</span><span class="detail-value">Hash-resolved workdir</span>`) {
		t.Fatalf("missing hash-resolved workdir provenance row in detail grid:\n%s", grid)
	}
	for _, want := range []string{
		`<div class="log-section-label">.command.log</div>`,
		`stdout &lt;ok&gt; &amp; more`,
		`<div class="log-section-label">.command.err</div>`,
		`stderr &lt;warn&gt; &amp; more`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing log output %q for hash-resolved cached workdir in:\n%s", want, got)
		}
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

	// Click handler toggles expanded state
	if !strings.Contains(got, `$expandedTask = $expandedTask === 42 ? 0 : 42`) {
		t.Fatalf("missing expandedTask toggle for taskID 42, got:\n%s", got)
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
	for _, label := range []string{"Process", "Name", "Duration", "CPU", "Memory", "Exit Code", "Work Dir", "Submitted", "Started", "Completed"} {
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

	// Workdir value with copy button
	if !strings.Contains(got, `class="detail-value workdir-row"`) {
		t.Fatal("missing workdir-row class")
	}
	if !strings.Contains(got, `class="btn-copy"`) {
		t.Fatal("missing copy button for workdir")
	}
	if !strings.Contains(got, `copyText(evt.currentTarget`) {
		t.Fatal("missing copyText call in copy button")
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
