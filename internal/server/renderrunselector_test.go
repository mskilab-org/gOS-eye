package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderRunSelector_EmptyRuns(t *testing.T) {
	got := renderRunSelector(nil, "")
	if got != "" {
		t.Fatalf("expected empty string for nil runs, got %q", got)
	}
	got = renderRunSelector([]*state.Run{}, "latest1")
	if got != "" {
		t.Fatalf("expected empty string for empty runs, got %q", got)
	}
}

func TestRenderRunSelector_SingleRun(t *testing.T) {
	runs := []*state.Run{
		{
			RunName:     "happy_euler",
			RunID:       "run1",
			ProjectName: "nf-core/rnaseq",
			Status:      "running",
			StartTime:   "2024-01-15T10:00:00Z",
		},
	}
	got := renderRunSelector(runs, "run1")

	// Wrapper div
	if !strings.Contains(got, `id="run-selector"`) {
		t.Fatal("missing id=\"run-selector\"")
	}
	if !strings.Contains(got, `class="run-selector"`) {
		t.Fatal("missing class=\"run-selector\"")
	}
	// Run entry div with click handler
	if !strings.Contains(got, `data-on:click="$selectedRun = 'run1'"`) {
		t.Fatal("missing data-on:click for selectedRun signal")
	}
	// Active highlighting via data-class:active
	if !strings.Contains(got, `data-class:active="($selectedRun || $latestRun) === 'run1'"`) {
		t.Fatal("missing data-class:active for run highlighting")
	}
	// Pipeline name
	if !strings.Contains(got, "nf-core/rnaseq") {
		t.Fatal("missing pipeline name")
	}
	// Run name
	if !strings.Contains(got, "happy_euler") {
		t.Fatal("missing run name")
	}
	// Status badge with lowercase class and uppercase text
	if !strings.Contains(got, `class="badge status-running"`) {
		t.Fatal("missing badge status-running class")
	}
	if !strings.Contains(got, ">RUNNING</span>") {
		t.Fatal("missing uppercase RUNNING label")
	}
	// Start timestamp
	if !strings.Contains(got, "2024-01-15T10:00:00Z") {
		t.Fatal("missing start timestamp")
	}
}

func TestRenderRunSelector_DefaultPipelineName(t *testing.T) {
	runs := []*state.Run{
		{
			RunName:     "angry_pasteur",
			RunID:       "run2",
			ProjectName: "", // empty → should default to "Pipeline"
			Status:      "completed",
			StartTime:   "2024-02-10T08:30:00Z",
		},
	}
	got := renderRunSelector(runs, "run2")

	if !strings.Contains(got, "Pipeline") {
		t.Fatal("missing default pipeline name 'Pipeline' when ProjectName is empty")
	}
}

func TestRenderRunSelector_MultipleRunsSortedByStartTimeDesc(t *testing.T) {
	runs := []*state.Run{
		{
			RunName:   "run_oldest",
			RunID:     "r1",
			Status:    "completed",
			StartTime: "2024-01-01T00:00:00Z",
		},
		{
			RunName:   "run_newest",
			RunID:     "r3",
			Status:    "running",
			StartTime: "2024-03-01T00:00:00Z",
		},
		{
			RunName:   "run_middle",
			RunID:     "r2",
			Status:    "error",
			StartTime: "2024-02-01T00:00:00Z",
		},
	}
	got := renderRunSelector(runs, "r3")

	// Verify newest first: run_newest should appear before run_middle, which appears before run_oldest
	newestIdx := strings.Index(got, "run_newest")
	middleIdx := strings.Index(got, "run_middle")
	oldestIdx := strings.Index(got, "run_oldest")

	if newestIdx < 0 || middleIdx < 0 || oldestIdx < 0 {
		t.Fatal("not all runs are present in output")
	}
	if newestIdx >= middleIdx {
		t.Fatal("newest run should appear before middle run")
	}
	if middleIdx >= oldestIdx {
		t.Fatal("middle run should appear before oldest run")
	}
}

func TestRenderRunSelector_StatusBadgeVariants(t *testing.T) {
	runs := []*state.Run{
		{RunName: "r1", RunID: "id1", Status: "completed", StartTime: "2024-01-01T00:00:00Z"},
		{RunName: "r2", RunID: "id2", Status: "error", StartTime: "2024-01-02T00:00:00Z"},
		{RunName: "r3", RunID: "id3", Status: "running", StartTime: "2024-01-03T00:00:00Z"},
	}
	got := renderRunSelector(runs, "id3")

	if !strings.Contains(got, `class="badge status-completed">COMPLETED</span>`) {
		t.Fatal("missing status-completed badge")
	}
	if !strings.Contains(got, `class="badge status-error">ERROR</span>`) {
		t.Fatal("missing status-error badge")
	}
	if !strings.Contains(got, `class="badge status-running">RUNNING</span>`) {
		t.Fatal("missing status-running badge")
	}
}

func TestRenderRunSelector_EachRunHasClickHandler(t *testing.T) {
	runs := []*state.Run{
		{RunName: "a", RunID: "id-a", Status: "completed", StartTime: "2024-01-01T00:00:00Z"},
		{RunName: "b", RunID: "id-b", Status: "running", StartTime: "2024-01-02T00:00:00Z"},
	}
	got := renderRunSelector(runs, "id-b")

	if !strings.Contains(got, `$selectedRun = 'id-a'`) {
		t.Fatal("missing click handler for run id-a")
	}
	if !strings.Contains(got, `$selectedRun = 'id-b'`) {
		t.Fatal("missing click handler for run id-b")
	}
}

func TestRenderRunSelector_ActiveHighlightForEachRun(t *testing.T) {
	runs := []*state.Run{
		{RunName: "a", RunID: "id-a", Status: "completed", StartTime: "2024-01-01T00:00:00Z"},
		{RunName: "b", RunID: "id-b", Status: "running", StartTime: "2024-01-02T00:00:00Z"},
	}
	got := renderRunSelector(runs, "id-b")

	if !strings.Contains(got, `($selectedRun || $latestRun) === 'id-a'`) {
		t.Fatal("missing active highlight expression for run id-a")
	}
	if !strings.Contains(got, `($selectedRun || $latestRun) === 'id-b'`) {
		t.Fatal("missing active highlight expression for run id-b")
	}
}

func TestRenderRunSelector_EmptyStartTime(t *testing.T) {
	runs := []*state.Run{
		{
			RunName:   "no_time",
			RunID:     "rt1",
			Status:    "running",
			StartTime: "", // no start time yet
		},
	}
	got := renderRunSelector(runs, "rt1")

	// Should still render the entry (not crash)
	if !strings.Contains(got, "no_time") {
		t.Fatal("missing run name when start time is empty")
	}
	if !strings.Contains(got, `id="run-selector"`) {
		t.Fatal("missing wrapper div")
	}
}

func TestRenderRunSelector_RunEntryHasClass(t *testing.T) {
	runs := []*state.Run{
		{RunName: "x", RunID: "rx", Status: "completed", StartTime: "2024-01-01T00:00:00Z"},
	}
	got := renderRunSelector(runs, "rx")

	if !strings.Contains(got, `class="run-entry"`) {
		t.Fatal("each run entry should have class=\"run-entry\"")
	}
}

func TestRenderRunSelector_DoesNotMutateInput(t *testing.T) {
	runs := []*state.Run{
		{RunName: "oldest", RunID: "r1", Status: "completed", StartTime: "2024-01-01T00:00:00Z"},
		{RunName: "newest", RunID: "r2", Status: "running", StartTime: "2024-06-01T00:00:00Z"},
	}
	// Capture original order
	origFirst := runs[0].RunName

	_ = renderRunSelector(runs, "r2")

	// Input slice order should not be changed
	if runs[0].RunName != origFirst {
		t.Fatal("renderRunSelector should not mutate the input slice order")
	}
}
