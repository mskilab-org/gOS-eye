package server

import (
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderDAGTaskPanel_EmptyGroups(t *testing.T) {
	got := renderDAGTaskPanel(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil groups, got %q", got)
	}
	got = renderDAGTaskPanel([]ProcessGroup{})
	if got != "" {
		t.Fatalf("expected empty string for empty groups, got %q", got)
	}
}

func TestRenderDAGTaskPanel_ContainsDagTaskPanelID(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Process: "ALIGN"},
			},
		},
	}
	got := renderDAGTaskPanel(groups)
	if !strings.Contains(got, `id="dag-task-panel"`) {
		t.Errorf("expected dag-task-panel id for Datastar morphing, got:\n%s", got)
	}
	if !strings.Contains(got, `class="dag-task-panel"`) {
		t.Errorf("expected dag-task-panel class, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_SingleGroup_DataShow(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "sayHello",
			Total:     2,
			Completed: 2,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "sayHello (1)", Status: "COMPLETED", Process: "sayHello"},
				{TaskID: 2, Name: "sayHello (2)", Status: "COMPLETED", Process: "sayHello"},
			},
		},
	}
	got := renderDAGTaskPanel(groups)

	// data-show should use $_dagSelectedProcess with the process name
	if !strings.Contains(got, `data-show="$_dagSelectedProcess === 'sayHello'"`) {
		t.Errorf("expected data-show for sayHello, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_SingleGroup_SectionHeader(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     3,
			Completed: 2,
			Running:   1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Process: "ALIGN"},
				{TaskID: 2, Name: "ALIGN (2)", Status: "COMPLETED", Process: "ALIGN"},
				{TaskID: 3, Name: "ALIGN (3)", Status: "RUNNING", Process: "ALIGN"},
			},
		},
	}
	got := renderDAGTaskPanel(groups)

	if !strings.Contains(got, `dag-task-section-header`) {
		t.Errorf("expected dag-task-section-header class, got:\n%s", got)
	}
	if !strings.Contains(got, `dag-task-section`) {
		t.Errorf("expected dag-task-section class, got:\n%s", got)
	}
	// Process name in header
	if !strings.Contains(got, "ALIGN") {
		t.Errorf("expected process name ALIGN in header, got:\n%s", got)
	}
	// Counts: completed/total
	if !strings.Contains(got, "2/3") {
		t.Errorf("expected counts 2/3, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_SingleGroup_TaskRowsPresent(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "SORT",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 5, Name: "SORT (1)", Status: "COMPLETED", Process: "SORT", Duration: 1500},
			},
		},
	}
	got := renderDAGTaskPanel(groups)

	// renderTaskRows produces task-row divs
	if !strings.Contains(got, `class="task-row"`) {
		t.Errorf("expected task-row from renderTaskRows, got:\n%s", got)
	}
	if !strings.Contains(got, "SORT (1)") {
		t.Errorf("expected task name SORT (1), got:\n%s", got)
	}
	// Task list wrapper
	if !strings.Contains(got, `class="task-list"`) {
		t.Errorf("expected task-list class, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_MultipleGroups_EachHasDataShow(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     1,
			Completed: 1,
			Tasks: []*state.Task{
				{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Process: "ALIGN"},
			},
		},
		{
			Name:      "SORT",
			Total:     2,
			Completed: 1,
			Running:   1,
			Tasks: []*state.Task{
				{TaskID: 2, Name: "SORT (1)", Status: "COMPLETED", Process: "SORT"},
				{TaskID: 3, Name: "SORT (2)", Status: "RUNNING", Process: "SORT"},
			},
		},
		{
			Name:      "INDEX",
			Total:     1,
			Completed: 0,
			Tasks: []*state.Task{
				{TaskID: 4, Name: "INDEX (1)", Status: "SUBMITTED", Process: "INDEX"},
			},
		},
	}
	got := renderDAGTaskPanel(groups)

	// Each group has its own data-show
	if !strings.Contains(got, `data-show="$_dagSelectedProcess === 'ALIGN'"`) {
		t.Errorf("expected data-show for ALIGN, got:\n%s", got)
	}
	if !strings.Contains(got, `data-show="$_dagSelectedProcess === 'SORT'"`) {
		t.Errorf("expected data-show for SORT, got:\n%s", got)
	}
	if !strings.Contains(got, `data-show="$_dagSelectedProcess === 'INDEX'"`) {
		t.Errorf("expected data-show for INDEX, got:\n%s", got)
	}

	// All three section headers present
	sectionCount := strings.Count(got, `dag-task-section-header`)
	if sectionCount != 3 {
		t.Errorf("expected 3 dag-task-section-header occurrences, got %d", sectionCount)
	}
}

func TestRenderDAGTaskPanel_StatusDot_Failed(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     3,
			Completed: 1,
			Running:   1,
			Failed:    1,
			Tasks:     []*state.Task{{TaskID: 1, Name: "ALIGN (1)", Status: "FAILED", Process: "ALIGN"}},
		},
	}
	got := renderDAGTaskPanel(groups)
	// Failed takes priority over running and completed
	if !strings.Contains(got, `status-failed`) {
		t.Errorf("expected status-failed dot when group has failed tasks, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_StatusDot_Running(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     3,
			Completed: 1,
			Running:   2,
			Tasks:     []*state.Task{{TaskID: 1, Name: "ALIGN (1)", Status: "RUNNING", Process: "ALIGN"}},
		},
	}
	got := renderDAGTaskPanel(groups)
	// Running takes priority over completed
	if !strings.Contains(got, `status-running`) {
		t.Errorf("expected status-running dot, got:\n%s", got)
	}
	if strings.Contains(got, `status-failed`) {
		t.Errorf("expected no status-failed dot when no failures, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_StatusDot_Completed(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     2,
			Completed: 2,
			Tasks:     []*state.Task{{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Process: "ALIGN"}},
		},
	}
	got := renderDAGTaskPanel(groups)
	if !strings.Contains(got, `status-completed`) {
		t.Errorf("expected status-completed dot, got:\n%s", got)
	}
	if strings.Contains(got, `status-failed`) || strings.Contains(got, `status-running`) {
		t.Errorf("expected only status-completed, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_StatusDot_Pending(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     2,
			Completed: 0,
			Submitted: 2,
			Tasks:     []*state.Task{{TaskID: 1, Name: "ALIGN (1)", Status: "SUBMITTED", Process: "ALIGN"}},
		},
	}
	got := renderDAGTaskPanel(groups)
	if !strings.Contains(got, `status-pending`) {
		t.Errorf("expected status-pending dot, got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_DoesNotContainProcessGroupClass(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "ALIGN",
			Total:     1,
			Completed: 1,
			Tasks:     []*state.Task{{TaskID: 1, Name: "ALIGN (1)", Status: "COMPLETED", Process: "ALIGN"}},
		},
	}
	got := renderDAGTaskPanel(groups)
	if strings.Contains(got, `process-group`) {
		t.Errorf("must NOT contain process-group class (breaks TestRenderRunDetail_WithLayout), got:\n%s", got)
	}
}

func TestRenderDAGTaskPanel_CountsShownCorrectly(t *testing.T) {
	groups := []ProcessGroup{
		{
			Name:      "FOO",
			Total:     10,
			Completed: 7,
			Running:   2,
			Failed:    1,
			Tasks:     []*state.Task{{TaskID: 1, Name: "FOO (1)", Status: "COMPLETED", Process: "FOO"}},
		},
	}
	got := renderDAGTaskPanel(groups)
	if !strings.Contains(got, "7/10") {
		t.Errorf("expected counts 7/10, got:\n%s", got)
	}
}
