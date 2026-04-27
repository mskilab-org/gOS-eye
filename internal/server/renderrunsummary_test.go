package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderRunSummary_NilRun(t *testing.T) {
	got := renderRunSummary(nil)
	if got != "" {
		t.Errorf("expected empty string for nil run, got: %s", got)
	}
}

func TestRenderRunSummary_RunningStatus(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "running",
		Tasks:   map[int]*state.Task{},
	}
	got := renderRunSummary(run)
	if got != "" {
		t.Errorf("expected empty string for running status, got: %s", got)
	}
}

func TestRenderRunSummary_EmptyStatus(t *testing.T) {
	run := &state.Run{
		RunName: "happy_euler",
		RunID:   "run1",
		Status:  "",
		Tasks:   map[int]*state.Task{},
	}
	got := renderRunSummary(run)
	if got != "" {
		t.Errorf("expected empty string for empty status, got: %s", got)
	}
}

func TestRenderRunSummary_CompletedRun_BasicStructure(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 1024 * 1024},
		},
	}
	got := renderRunSummary(run)

	// Wrapper div
	if !strings.Contains(got, `<div class="run-summary">`) {
		t.Error("expected run-summary wrapper div")
	}
	// Detail grid pattern
	if !strings.Contains(got, `class="detail-grid"`) {
		t.Error("expected detail-grid container")
	}
	if !strings.Contains(got, `class="detail-label"`) {
		t.Error("expected detail-label spans")
	}
	if !strings.Contains(got, `class="detail-value"`) {
		t.Error("expected detail-value spans")
	}
}

func TestRenderRunSummary_CompletedRun_ShowsDuration(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks:        map[int]*state.Task{},
	}
	got := renderRunSummary(run)

	// Should contain the duration computed by computeRunDuration
	expected := computeRunDuration("2024-01-15T10:00:00Z", "2024-01-15T10:05:00Z")
	if !strings.Contains(got, expected) {
		t.Errorf("expected duration %q in output, got:\n%s", expected, got)
	}
}

func TestRenderRunSummary_CompletedRun_TaskCounts(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 100},
			2: {TaskID: 2, Status: "COMPLETED", PeakRSS: 200},
			3: {TaskID: 3, Status: "COMPLETED", PeakRSS: 300},
			4: {TaskID: 4, Status: "COMPLETED", PeakRSS: 400},
			5: {TaskID: 5, Status: "FAILED", PeakRSS: 50},
		},
	}
	got := renderRunSummary(run)

	// Should show "4 completed" and "1 failed"
	if !strings.Contains(got, "4 completed") {
		t.Errorf("expected '4 completed' in task counts, got:\n%s", got)
	}
	if !strings.Contains(got, "1 failed") {
		t.Errorf("expected '1 failed' in task counts, got:\n%s", got)
	}
	// Should NOT show "0 running" or "0 submitted"
	if strings.Contains(got, "running") {
		t.Errorf("should not show zero-count running status, got:\n%s", got)
	}
	if strings.Contains(got, "submitted") {
		t.Errorf("should not show zero-count submitted status, got:\n%s", got)
	}
}

func TestRenderRunSummary_CachedTasksRenderSeparateFromCompleted(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 100},
			2: {TaskID: 2, Status: "COMPLETED", PeakRSS: 200},
			3: {TaskID: 3, Status: "CACHED", PeakRSS: 300},
		},
	}
	got := renderRunSummary(run)

	if !strings.Contains(got, "2 completed") {
		t.Errorf("expected executed completed count to exclude cached tasks, got:\n%s", got)
	}
	if strings.Contains(got, "3 completed") {
		t.Errorf("cached tasks should not be lumped into completed count, got:\n%s", got)
	}
	if !strings.Contains(got, "1 cached") {
		t.Errorf("expected distinct cached task summary label, got:\n%s", got)
	}
	if !strings.Contains(got, formatBytes(200)) {
		t.Errorf("expected peak memory to come from executed completed tasks, got:\n%s", got)
	}
	if strings.Contains(got, formatBytes(300)) {
		t.Errorf("cached task memory should not contribute to current-run peak memory, got:\n%s", got)
	}
}

func TestRenderRunSummary_CompletedRun_AllStatusTypes(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 100},
			2: {TaskID: 2, Status: "FAILED", PeakRSS: 200},
			3: {TaskID: 3, Status: "RUNNING", PeakRSS: 300},
			4: {TaskID: 4, Status: "SUBMITTED", PeakRSS: 400},
		},
	}
	got := renderRunSummary(run)

	if !strings.Contains(got, "1 completed") {
		t.Errorf("expected '1 completed', got:\n%s", got)
	}
	if !strings.Contains(got, "1 failed") {
		t.Errorf("expected '1 failed', got:\n%s", got)
	}
	if !strings.Contains(got, "1 running") {
		t.Errorf("expected '1 running', got:\n%s", got)
	}
	if !strings.Contains(got, "1 submitted") {
		t.Errorf("expected '1 submitted', got:\n%s", got)
	}
}

func TestRenderRunSummary_CompletedRun_PeakMemory(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 1024},
			2: {TaskID: 2, Status: "COMPLETED", PeakRSS: 2048},
			3: {TaskID: 3, Status: "COMPLETED", PeakRSS: 512},
		},
	}
	got := renderRunSummary(run)

	// Max PeakRSS is 2048 → formatBytes(2048) = "2.0 KB"
	expected := formatBytes(2048)
	if !strings.Contains(got, expected) {
		t.Errorf("expected peak memory %q in output, got:\n%s", expected, got)
	}
}

func TestRenderRunSummary_CompletedRun_PeakMemoryLarge(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 500 * 1024 * 1024},
			2: {TaskID: 2, Status: "COMPLETED", PeakRSS: 1024 * 1024 * 1024},
		},
	}
	got := renderRunSummary(run)

	// Max PeakRSS is 1 GB
	expected := formatBytes(1024 * 1024 * 1024)
	if !strings.Contains(got, expected) {
		t.Errorf("expected peak memory %q in output, got:\n%s", expected, got)
	}
}

func TestRenderRunSummary_ErrorRun_ShowsErrorMessage(t *testing.T) {
	run := &state.Run{
		RunName:      "angry_fermat",
		RunID:        "run2",
		Status:       "failed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:02:30Z",
		ErrorMessage: "Process `align` terminated with an error exit status (137)",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 1024},
			2: {TaskID: 2, Status: "FAILED", PeakRSS: 2048},
		},
	}
	got := renderRunSummary(run)

	// Should have run-summary wrapper
	if !strings.Contains(got, `<div class="run-summary">`) {
		t.Error("expected run-summary wrapper")
	}
	// Should show error message in error-message div
	if !strings.Contains(got, `class="error-message"`) {
		t.Error("expected error-message div")
	}
	if !strings.Contains(got, "Process `align` terminated with an error exit status (137)") {
		t.Errorf("expected error text in output, got:\n%s", got)
	}
}

func TestRenderRunSummary_ErrorRun_NoErrorMessage(t *testing.T) {
	run := &state.Run{
		RunName:      "angry_fermat",
		RunID:        "run2",
		Status:       "failed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:02:30Z",
		ErrorMessage: "",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED", PeakRSS: 1024},
		},
	}
	got := renderRunSummary(run)

	// Should NOT contain error-message div when ErrorMessage is empty
	if strings.Contains(got, `class="error-message"`) {
		t.Error("should not show error-message div when ErrorMessage is empty")
	}
}

func TestRenderRunSummary_NoTasks(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks:        map[int]*state.Task{},
	}
	got := renderRunSummary(run)

	// Should still render the summary (just with zero peak memory)
	if !strings.Contains(got, `<div class="run-summary">`) {
		t.Error("expected run-summary wrapper even with no tasks")
	}
	// Peak memory should be "0 B"
	if !strings.Contains(got, formatBytes(0)) {
		t.Errorf("expected peak memory '0 B' with no tasks, got:\n%s", got)
	}
}

func TestRenderRunSummary_NilTasksMap(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks:        nil,
	}
	got := renderRunSummary(run)

	// Should still render without panicking
	if !strings.Contains(got, `<div class="run-summary">`) {
		t.Error("expected run-summary wrapper even with nil tasks")
	}
}

func TestRenderRunSummary_HasDurationLabel(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks:        map[int]*state.Task{},
	}
	got := renderRunSummary(run)

	// Check for Duration label in the detail grid
	if !strings.Contains(got, "Duration") {
		t.Errorf("expected 'Duration' label, got:\n%s", got)
	}
	// Check for Tasks label
	if !strings.Contains(got, "Tasks") {
		t.Errorf("expected 'Tasks' label, got:\n%s", got)
	}
	// Check for Peak Memory label
	if !strings.Contains(got, "Peak Memory") {
		t.Errorf("expected 'Peak Memory' label, got:\n%s", got)
	}
}

func TestRenderRunSummary_OnlyCompletedTasks_CountString(t *testing.T) {
	run := &state.Run{
		RunName:      "happy_euler",
		RunID:        "run1",
		Status:       "completed",
		StartTime:    "2024-01-15T10:00:00Z",
		CompleteTime: "2024-01-15T10:05:00Z",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Status: "COMPLETED"},
			2: {TaskID: 2, Status: "COMPLETED"},
			3: {TaskID: 3, Status: "COMPLETED"},
		},
	}
	got := renderRunSummary(run)

	// Should show "3 completed" with no commas (only one non-zero count)
	if !strings.Contains(got, "3 completed") {
		t.Errorf("expected '3 completed', got:\n%s", got)
	}
	// Should not contain "failed" since count is zero
	if strings.Contains(got, "failed") {
		t.Errorf("should not contain 'failed' when no tasks failed, got:\n%s", got)
	}
}
