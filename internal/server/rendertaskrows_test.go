package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderTaskRows_EmptySlice(t *testing.T) {
	got := renderTaskRows("sayHello", nil)
	if got != "" {
		t.Fatalf("expected empty string for nil tasks, got %q", got)
	}
	got = renderTaskRows("sayHello", []*state.Task{})
	if got != "" {
		t.Fatalf("expected empty string for empty tasks, got %q", got)
	}
}

func TestRenderTaskRows_SingleCompletedTask(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID:     1,
			Name:       "sayHello (1)",
			Status:     "COMPLETED",
			Duration:   3800,
			CPUPercent: 92.5,
			RSS:        10485760, // 10 MB
			PeakRSS:    10485760,
			Exit:       0,
			Workdir:    "/work/ab/cdef123",
			Submit:     1705312201000, // 2024-01-15 09:50:01 UTC
			Start:      0,
			Complete:   0,
		},
	}
	got := renderTaskRows("sayHello", tasks)

	// Task row div
	if !strings.Contains(got, `class="task-row"`) {
		t.Fatal("missing task-row class")
	}
	// Should NOT have "failed" class
	if strings.Contains(got, `class="task-row failed"`) {
		t.Fatal("completed task should not have failed class")
	}
	// Datastar click toggle with task ID
	if !strings.Contains(got, `data-on:click__stop="$expandedTask = $expandedTask === 1 ? 0 : 1"`) {
		t.Fatal("missing or wrong data-on:click__stop attribute")
	}
	// Task name
	if !strings.Contains(got, `<span class="task-name">sayHello (1)</span>`) {
		t.Fatal("missing task name span")
	}
	// Status badge with lowercase class
	if !strings.Contains(got, `<span class="badge status-completed">COMPLETED</span>`) {
		t.Fatal("missing status badge")
	}
	// Duration
	if !strings.Contains(got, `<span class="task-duration">3.8s</span>`) {
		t.Fatal("missing or wrong duration")
	}
	// Detail panel show condition
	if !strings.Contains(got, `data-show="$expandedTask === 1"`) {
		t.Fatal("missing data-show for task detail")
	}
	// CPU
	if !strings.Contains(got, `<span class="detail-value">92.5%</span>`) {
		t.Fatal("missing CPU value")
	}
	// Memory
	if !strings.Contains(got, `<span class="detail-label">Memory</span><span class="detail-value">10.0 MB</span>`) {
		t.Fatal("missing memory value")
	}
	// Peak Memory
	if !strings.Contains(got, `<span class="detail-label">Peak Memory</span><span class="detail-value">10.0 MB</span>`) {
		t.Fatal("missing peak memory value")
	}
	// Exit code 0, no exit-error class
	if !strings.Contains(got, `<span class="detail-value">0</span>`) {
		t.Fatal("missing exit code value")
	}
	if strings.Contains(got, `exit-error`) {
		t.Fatal("exit code 0 should not have exit-error class")
	}
	// Workdir with monospace class
	if !strings.Contains(got, `<span class="detail-value workdir">/work/ab/cdef123</span>`) {
		t.Fatal("missing workdir with workdir class")
	}
	// Submitted timestamp
	if !strings.Contains(got, `<span class="detail-value">2024-01-15 09:50:01 UTC</span>`) {
		t.Fatal("missing submitted timestamp")
	}
	// Started/Completed should be em-dash since epoch is 0
	// Count occurrences of the em-dash detail value (Start and Complete)
	dashCount := strings.Count(got, `<span class="detail-value">—</span>`)
	if dashCount < 2 {
		t.Fatalf("expected at least 2 em-dash values for Start/Complete, got %d", dashCount)
	}
}

func TestRenderTaskRows_FailedTask(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID:     5,
			Name:       "align (3)",
			Status:     "FAILED",
			Duration:   500,
			CPUPercent: 0,
			RSS:        0,
			PeakRSS:    0,
			Exit:       1,
			Workdir:    "/work/xy/fail456",
			Submit:     0,
			Start:      0,
			Complete:   0,
		},
	}
	got := renderTaskRows("align", tasks)

	// Failed class on task-row
	if !strings.Contains(got, `class="task-row failed"`) {
		t.Fatal("failed task should have 'task-row failed' class")
	}
	// Status badge
	if !strings.Contains(got, `class="badge status-failed"`) {
		t.Fatal("missing status-failed badge class")
	}
	// Exit code with exit-error class
	if !strings.Contains(got, `<span class="detail-value exit-error">1</span>`) {
		t.Fatal("non-zero exit code should have exit-error class")
	}
	// CPU zero should show em-dash
	if !strings.Contains(got, `<span class="detail-label">CPU</span><span class="detail-value">—</span>`) {
		t.Fatal("zero CPU should show em-dash")
	}
	// Memory zero shows formatBytes(0) = "0 B"
	if !strings.Contains(got, `<span class="detail-label">Memory</span><span class="detail-value">0 B</span>`) {
		t.Fatal("zero RSS should show '0 B'")
	}
}

func TestRenderTaskRows_RunningTask(t *testing.T) {
	tasks := []*state.Task{
		{
			TaskID: 10,
			Name:   "index (1)",
			Status: "RUNNING",
		},
	}
	got := renderTaskRows("index", tasks)

	// Running task: no "failed" class
	if strings.Contains(got, "failed") {
		t.Fatal("running task should not have failed class")
	}
	// Status badge
	if !strings.Contains(got, `class="badge status-running">RUNNING</span>`) {
		t.Fatal("missing status-running badge")
	}
	// Datastar attributes use task ID 10
	if !strings.Contains(got, `$expandedTask === 10 ? 0 : 10`) {
		t.Fatal("wrong task ID in data-on-click")
	}
	if !strings.Contains(got, `data-show="$expandedTask === 10"`) {
		t.Fatal("wrong task ID in data-show")
	}
}

func TestRenderTaskRows_MultipleTasks(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 2, Name: "proc (1)", Status: "FAILED", Exit: 137},
		{TaskID: 1, Name: "proc (2)", Status: "COMPLETED", Exit: 0},
		{TaskID: 3, Name: "proc (3)", Status: "SUBMITTED", Exit: 0},
	}
	got := renderTaskRows("proc", tasks)

	// All three tasks should be present
	if strings.Count(got, `class="task-row`) != 3 {
		t.Fatalf("expected 3 task-row divs, got %d", strings.Count(got, `class="task-row`))
	}
	// 3 detail panels
	if strings.Count(got, `class="task-detail"`) != 3 {
		t.Fatalf("expected 3 task-detail divs, got %d", strings.Count(got, `class="task-detail"`))
	}
	// First task should be failed
	firstRowIdx := strings.Index(got, `class="task-row`)
	failedIdx := strings.Index(got, `class="task-row failed"`)
	if failedIdx != firstRowIdx {
		t.Fatal("first task-row should be the failed one (pre-sorted)")
	}
	// Status badge for submitted task
	if !strings.Contains(got, `class="badge status-submitted">SUBMITTED</span>`) {
		t.Fatal("missing status-submitted badge")
	}
	// Exit 137 should have exit-error
	if !strings.Contains(got, `<span class="detail-value exit-error">137</span>`) {
		t.Fatal("exit 137 should have exit-error class")
	}
}

func TestRenderTaskRows_StructureOrder(t *testing.T) {
	// Verify task-row comes before task-detail for same task
	tasks := []*state.Task{
		{TaskID: 7, Name: "step (1)", Status: "COMPLETED"},
	}
	got := renderTaskRows("step", tasks)

	rowIdx := strings.Index(got, `class="task-row"`)
	detailIdx := strings.Index(got, `class="task-detail"`)
	if rowIdx < 0 || detailIdx < 0 {
		t.Fatal("missing task-row or task-detail")
	}
	if rowIdx >= detailIdx {
		t.Fatal("task-row should come before task-detail")
	}
	// Detail grid should be inside task-detail
	if !strings.Contains(got, `class="detail-grid"`) {
		t.Fatal("missing detail-grid inside task-detail")
	}
}

func TestRenderTaskRows_NegativeExitCode(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 99, Name: "crash (1)", Status: "FAILED", Exit: -1},
	}
	got := renderTaskRows("crash", tasks)
	if !strings.Contains(got, `<span class="detail-value exit-error">-1</span>`) {
		t.Fatal("negative exit code should have exit-error class")
	}
}
