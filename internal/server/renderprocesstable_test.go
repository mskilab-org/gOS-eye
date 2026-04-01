package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderProcessTable_EmptyGroups(t *testing.T) {
	got := renderProcessTable(nil, "run-1")
	if got != "" {
		t.Fatalf("expected empty string for nil groups, got %q", got)
	}
	got = renderProcessTable([]ProcessGroup{}, "run-1")
	if got != "" {
		t.Fatalf("expected empty string for empty groups, got %q", got)
	}
}

func TestRenderProcessTable_SingleProcessBarsAt100(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "FASTQC",
			Total:     2,
			Completed: 2,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "FASTQC (1)", Status: "COMPLETED", Duration: 10000, CPUPercent: 80.0, PeakRSS: 2097152},
				{TaskID: 2, Name: "FASTQC (2)", Status: "COMPLETED", Duration: 5000, CPUPercent: 40.0, PeakRSS: 1048576},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	// With a single process, its max values ARE the column max, so bars should be at 100%.
	if !strings.Contains(got, `width: 100%`) {
		t.Fatal("expected bars at 100% for single process with completed tasks")
	}
	// Extract just the process-table-row content (before the nested task table)
	rowEnd := strings.Index(got, `class="process-table-tasks"`)
	if rowEnd < 0 {
		t.Fatal("missing process-table-tasks section")
	}
	rowSection := got[:rowEnd]
	// The process row should have 3 bars at 100%
	if strings.Count(rowSection, `width: 100%`) != 3 {
		t.Fatalf("expected 3 bars at 100%% in process row, got %d in:\n%s", strings.Count(rowSection, `width: 100%`), rowSection)
	}
	// Check bar labels: max duration=10s, max CPU=80%, max peakRSS=2MB
	if !strings.Contains(got, "10.0s") {
		t.Fatalf("expected duration label '10.0s' in:\n%s", got)
	}
	if !strings.Contains(got, "80%") {
		t.Fatalf("expected CPU label '80%%' in:\n%s", got)
	}
	if !strings.Contains(got, "2.0 MB") {
		t.Fatalf("expected memory label '2.0 MB' in:\n%s", got)
	}
}

func TestRenderProcessTable_MultipleProcessesBarScaling(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "FASTQC",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "FASTQC (1)", Status: "COMPLETED", Duration: 10000, CPUPercent: 100.0, PeakRSS: 10485760},
			},
		},
		{
			Name:      "ALIGN",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 2, Name: "ALIGN (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1048576},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	// FASTQC has the column max for all metrics → 100% bars
	if strings.Count(got, `width: 100%`) < 3 {
		t.Fatalf("expected at least 3 bars at 100%% (FASTQC), got %d in:\n%s", strings.Count(got, `width: 100%`), got)
	}
	// ALIGN's duration is 1000/10000 = 10% (above 5% minimum), CPU is 10/100 = 10%, memory is 1M/10M = 10%
	if strings.Count(got, `width: 10%`) < 3 {
		t.Fatalf("expected at least 3 bars at 10%% (ALIGN), got %d in:\n%s", strings.Count(got, `width: 10%`), got)
	}
}

func TestRenderProcessTable_MultipleProcessesBarMinimum5Pct(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "BIG",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "BIG (1)", Status: "COMPLETED", Duration: 100000, CPUPercent: 200.0, PeakRSS: 104857600},
			},
		},
		{
			Name:      "TINY",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 2, Name: "TINY (1)", Status: "COMPLETED", Duration: 100, CPUPercent: 1.0, PeakRSS: 1024},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	// TINY's values are ~0.1% of BIG's → barWidth clamps to 5%
	if strings.Count(got, `width: 5%`) < 3 {
		t.Fatalf("expected at least 3 bars at 5%% (TINY, clamped minimum), got %d in:\n%s", strings.Count(got, `width: 5%`), got)
	}
}

func TestRenderProcessTable_NoCompletedTasks(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "PENDING",
			Total:     2,
			Completed: 0,
			Running:   1,
			Submitted: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "PENDING (1)", Status: "RUNNING"},
				{TaskID: 2, Name: "PENDING (2)", Status: "SUBMITTED"},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	// Should still render the table (non-empty groups)
	if got == "" {
		t.Fatal("expected non-empty output for groups with no completed tasks")
	}
	// Bars should be at 0% with "···" label
	if strings.Count(got, `width: 0%`) < 3 {
		t.Fatalf("expected 3 bars at 0%%, got %d in:\n%s", strings.Count(got, `width: 0%`), got)
	}
	if strings.Count(got, "···") < 3 {
		t.Fatalf("expected 3 placeholder labels '···', got %d in:\n%s", strings.Count(got, "···"), got)
	}
}

func TestRenderProcessTable_ClickHandler(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "FASTQC",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "FASTQC (1)", Status: "COMPLETED", Duration: 5000, CPUPercent: 50.0, PeakRSS: 1048576},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	want := `data-on:click="$expandedGroup = $expandedGroup === 'FASTQC' ? '' : 'FASTQC'"`
	if !strings.Contains(got, want) {
		t.Fatalf("missing click handler, want %q in:\n%s", want, got)
	}
}

func TestRenderProcessTable_ExpandedSectionContainsTaskTable(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Duration: 3000, CPUPercent: 25.0, PeakRSS: 524288},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	// The expanded section should contain the lazy-load task-panel placeholder (with data-ignore-morph)
	// and inner task-content div.
	if !strings.Contains(got, `id="task-panel-ALIGN" data-ignore-morph`) {
		t.Fatalf("expected task-panel placeholder with id and data-ignore-morph in:\n%s", got)
	}
	if !strings.Contains(got, `id="task-content-ALIGN"`) {
		t.Fatalf("expected inner task-content-ALIGN div in:\n%s", got)
	}
	if !strings.Contains(got, `data-init="@get('/sse/run/run-1/tasks/ALIGN')"`) {
		t.Fatalf("expected data-init for lazy-load in:\n%s", got)
	}
	// The expanded section should have data-show binding
	want := `data-show="$expandedGroup === 'ALIGN'"`
	if !strings.Contains(got, want) {
		t.Fatalf("missing data-show binding, want %q in:\n%s", want, got)
	}
	// Hidden by default
	if !strings.Contains(got, `style="display: none"`) {
		t.Fatalf("expected expanded section to be hidden by default in:\n%s", got)
	}
}

func TestRenderProcessTable_ChevronExpandedBinding(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:  "TRIM",
			Total: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "TRIM (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
			},
			Completed: 1,
		},
	}
	got := renderProcessTable(groups, "run-1")

	// Chevron should have expanded class binding
	want := `data-class:expanded="$expandedGroup === 'TRIM'"`
	if !strings.Contains(got, want) {
		t.Fatalf("missing chevron expanded binding, want %q in:\n%s", want, got)
	}
	// Chevron character should be present
	if !strings.Contains(got, `class="chevron"`) {
		t.Fatalf("missing chevron class in:\n%s", got)
	}
	if !strings.Contains(got, "▶") {
		t.Fatalf("missing chevron character ▶ in:\n%s", got)
	}
}

func TestRenderProcessTable_StatusDotPresent(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "FASTQC",
			Total:     3,
			Completed: 3,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "FASTQC (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
				{TaskID: 2, Name: "FASTQC (2)", Status: "COMPLETED", Duration: 2000, CPUPercent: 20.0, PeakRSS: 2048},
				{TaskID: 3, Name: "FASTQC (3)", Status: "COMPLETED", Duration: 3000, CPUPercent: 30.0, PeakRSS: 3072},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	if !strings.Contains(got, "group-status-indicator") {
		t.Fatalf("missing group-status-indicator class in:\n%s", got)
	}
	// All completed → should have status-completed
	if !strings.Contains(got, "status-completed") {
		t.Fatalf("expected status-completed for fully completed group in:\n%s", got)
	}
}

func TestRenderProcessTable_TaskCountFormat(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     5,
			Completed: 3,
			Running:   2,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
				{TaskID: 2, Name: "ALIGN (2)", Status: "COMPLETED", Duration: 2000, CPUPercent: 20.0, PeakRSS: 2048},
				{TaskID: 3, Name: "ALIGN (3)", Status: "COMPLETED", Duration: 3000, CPUPercent: 30.0, PeakRSS: 3072},
				{TaskID: 4, Name: "ALIGN (4)", Status: "RUNNING"},
				{TaskID: 5, Name: "ALIGN (5)", Status: "RUNNING"},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	// Count format: "Completed/Total"
	want := fmt.Sprintf(`<span class="process-table-counts">3/5</span>`)
	if !strings.Contains(got, want) {
		t.Fatalf("missing task count '3/5', want %q in:\n%s", want, got)
	}
}

func TestRenderProcessTable_HeaderLabels(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "X",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "X (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	if !strings.Contains(got, `class="process-table-header"`) {
		t.Fatal("missing process-table-header")
	}
	for _, label := range []string{"duration", "cpu", "memory"} {
		want := fmt.Sprintf(`<span class="resource-col-label">%s</span>`, label)
		if !strings.Contains(got, want) {
			t.Fatalf("missing header label %q in:\n%s", label, got)
		}
	}
	// Should have some empty labels for non-metric columns
	emptyLabel := `<span class="resource-col-label"></span>`
	if strings.Count(got, emptyLabel) < 3 {
		t.Fatalf("expected at least 3 empty header labels, got %d in:\n%s", strings.Count(got, emptyLabel), got)
	}
}

func TestRenderProcessTable_GroupStatusClasses(t *testing.T) {
	// Group with failed tasks
	groups := []ProcessGroup{
		{
			Name:      "FAIL_GROUP",
			Total:     2,
			Completed: 1,
			Failed:    1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "FAIL_GROUP (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
				{TaskID: 2, Name: "FAIL_GROUP (2)", Status: "FAILED", Duration: 500, CPUPercent: 5.0, PeakRSS: 512},
			},
		},
	}
	got := renderProcessTable(groups, "run-1")

	if !strings.Contains(got, "group-has-failed") {
		t.Fatalf("expected group-has-failed class for group with failed tasks in:\n%s", got)
	}

	// Group with running tasks
	groups = []ProcessGroup{
		{
			Name:      "RUN_GROUP",
			Total:     2,
			Completed: 1,
			Running:   1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "RUN_GROUP (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
				{TaskID: 2, Name: "RUN_GROUP (2)", Status: "RUNNING"},
			},
		},
	}
	got = renderProcessTable(groups, "run-1")

	if !strings.Contains(got, "group-has-running") {
		t.Fatalf("expected group-has-running class for group with running tasks in:\n%s", got)
	}

	// Group with both failed and running
	groups = []ProcessGroup{
		{
			Name:    "BOTH",
			Total:   3,
			Failed:  1,
			Running: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "BOTH (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
				{TaskID: 2, Name: "BOTH (2)", Status: "FAILED", Duration: 500, CPUPercent: 5.0, PeakRSS: 512},
				{TaskID: 3, Name: "BOTH (3)", Status: "RUNNING"},
			},
			Completed: 1,
		},
	}
	got = renderProcessTable(groups, "run-1")

	if !strings.Contains(got, "group-has-failed") {
		t.Fatalf("expected group-has-failed for group with both failed and running in:\n%s", got)
	}
	if !strings.Contains(got, "group-has-running") {
		t.Fatalf("expected group-has-running for group with both failed and running in:\n%s", got)
	}

	// Group with no failed/running → no status classes
	groups = []ProcessGroup{
		{
			Name:      "CLEAN",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "CLEAN (1)", Status: "COMPLETED", Duration: 1000, CPUPercent: 10.0, PeakRSS: 1024},
			},
		},
	}
	got = renderProcessTable(groups, "run-1")

	if strings.Contains(got, "group-has-failed") {
		t.Fatalf("unexpected group-has-failed for clean group in:\n%s", got)
	}
	if strings.Contains(got, "group-has-running") {
		t.Fatalf("unexpected group-has-running for clean group in:\n%s", got)
	}
}
