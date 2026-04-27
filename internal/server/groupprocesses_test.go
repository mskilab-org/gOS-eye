package server

import (
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestGroupProcesses_EmptyMap(t *testing.T) {
	got := groupProcesses(map[int]*state.Task{})
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d groups", len(got))
	}
}

func TestGroupProcesses_NilMap(t *testing.T) {
	got := groupProcesses(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty slice for nil map, got %d groups", len(got))
	}
}

func TestGroupProcesses_SingleTask(t *testing.T) {
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "sayHello", Status: "COMPLETED"},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if g.Name != "sayHello" {
		t.Errorf("Name = %q, want %q", g.Name, "sayHello")
	}
	if g.Total != 1 {
		t.Errorf("Total = %d, want 1", g.Total)
	}
	if g.Completed != 1 {
		t.Errorf("Completed = %d, want 1", g.Completed)
	}
	if g.Running != 0 {
		t.Errorf("Running = %d, want 0", g.Running)
	}
	if g.Failed != 0 {
		t.Errorf("Failed = %d, want 0", g.Failed)
	}
	if g.Submitted != 0 {
		t.Errorf("Submitted = %d, want 0", g.Submitted)
	}
}

func TestGroupProcesses_MultipleTasksSameProcess(t *testing.T) {
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "align", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "align", Status: "COMPLETED"},
		3: {TaskID: 3, Process: "align", Status: "RUNNING"},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if g.Name != "align" {
		t.Errorf("Name = %q, want %q", g.Name, "align")
	}
	if g.Total != 3 {
		t.Errorf("Total = %d, want 3", g.Total)
	}
	if g.Completed != 2 {
		t.Errorf("Completed = %d, want 2", g.Completed)
	}
	if g.Running != 1 {
		t.Errorf("Running = %d, want 1", g.Running)
	}
}

func TestGroupProcesses_MultipleProcessesSortedAlphabetically(t *testing.T) {
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "zebra", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "alpha", Status: "RUNNING"},
		3: {TaskID: 3, Process: "middle", Status: "FAILED"},
	}
	got := groupProcesses(tasks)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(got))
	}
	if got[0].Name != "alpha" {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, "alpha")
	}
	if got[1].Name != "middle" {
		t.Errorf("got[1].Name = %q, want %q", got[1].Name, "middle")
	}
	if got[2].Name != "zebra" {
		t.Errorf("got[2].Name = %q, want %q", got[2].Name, "zebra")
	}
}

func TestGroupProcesses_MixedStatusesSingleProcess(t *testing.T) {
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "say", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "say", Status: "RUNNING"},
		3: {TaskID: 3, Process: "say", Status: "FAILED"},
		4: {TaskID: 4, Process: "say", Status: "SUBMITTED"},
		5: {TaskID: 5, Process: "say", Status: "COMPLETED"},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if g.Completed != 2 {
		t.Errorf("Completed = %d, want 2", g.Completed)
	}
	if g.Running != 1 {
		t.Errorf("Running = %d, want 1", g.Running)
	}
	if g.Failed != 1 {
		t.Errorf("Failed = %d, want 1", g.Failed)
	}
	if g.Submitted != 1 {
		t.Errorf("Submitted = %d, want 1", g.Submitted)
	}
	if g.Total != 5 {
		t.Errorf("Total = %d, want 5", g.Total)
	}
}

func TestGroupProcesses_TotalEqualsSumOfStatuses(t *testing.T) {
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "align", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "align", Status: "RUNNING"},
		3: {TaskID: 3, Process: "call", Status: "FAILED"},
		4: {TaskID: 4, Process: "call", Status: "SUBMITTED"},
		5: {TaskID: 5, Process: "call", Status: "COMPLETED"},
	}
	got := groupProcesses(tasks)
	for _, g := range got {
		sum := g.Completed + g.Running + g.Failed + g.Submitted
		if g.Total != sum {
			t.Errorf("group %q: Total=%d != sum of statuses=%d", g.Name, g.Total, sum)
		}
	}
}

// ---- Tests for Tasks slice population and sort order ----

func TestGroupProcesses_TasksSlicePopulated(t *testing.T) {
	// Tasks slice should contain pointers to the exact task objects
	t1 := &state.Task{TaskID: 1, Process: "align", Name: "align (1)", Status: "COMPLETED"}
	t2 := &state.Task{TaskID: 2, Process: "align", Name: "align (2)", Status: "RUNNING"}
	tasks := map[int]*state.Task{1: t1, 2: t2}

	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if len(g.Tasks) != 2 {
		t.Fatalf("Tasks length = %d, want 2", len(g.Tasks))
	}
	// Verify the pointers are the same objects we passed in
	foundT1, foundT2 := false, false
	for _, task := range g.Tasks {
		if task == t1 {
			foundT1 = true
		}
		if task == t2 {
			foundT2 = true
		}
	}
	if !foundT1 {
		t.Error("Tasks slice missing pointer to t1")
	}
	if !foundT2 {
		t.Error("Tasks slice missing pointer to t2")
	}
}

func TestGroupProcesses_TasksSortedFailedFirst(t *testing.T) {
	// Within a group, FAILED tasks should appear before non-failed tasks
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "say", Name: "say (1)", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "say", Name: "say (2)", Status: "FAILED"},
		3: {TaskID: 3, Process: "say", Name: "say (3)", Status: "RUNNING"},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if len(g.Tasks) != 3 {
		t.Fatalf("Tasks length = %d, want 3", len(g.Tasks))
	}
	// First task must be the FAILED one
	if g.Tasks[0].Status != "FAILED" {
		t.Errorf("Tasks[0].Status = %q, want FAILED", g.Tasks[0].Status)
	}
	// Remaining should be non-failed
	for i := 1; i < len(g.Tasks); i++ {
		if g.Tasks[i].Status == "FAILED" {
			t.Errorf("Tasks[%d].Status = FAILED, should be before non-failed tasks", i)
		}
	}
}

func TestGroupProcesses_TasksSortedAlphabeticallyWithinSameStatus(t *testing.T) {
	// Non-failed tasks should be sorted alphabetically by Name
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "align", Name: "align (zz)", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "align", Name: "align (aa)", Status: "COMPLETED"},
		3: {TaskID: 3, Process: "align", Name: "align (mm)", Status: "COMPLETED"},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if len(g.Tasks) != 3 {
		t.Fatalf("Tasks length = %d, want 3", len(g.Tasks))
	}
	if g.Tasks[0].Name != "align (aa)" {
		t.Errorf("Tasks[0].Name = %q, want %q", g.Tasks[0].Name, "align (aa)")
	}
	if g.Tasks[1].Name != "align (mm)" {
		t.Errorf("Tasks[1].Name = %q, want %q", g.Tasks[1].Name, "align (mm)")
	}
	if g.Tasks[2].Name != "align (zz)" {
		t.Errorf("Tasks[2].Name = %q, want %q", g.Tasks[2].Name, "align (zz)")
	}
}

func TestGroupProcesses_TasksSortFailedFirstThenAlphabetical(t *testing.T) {
	// Full sort: multiple FAILED tasks sorted by name first, then non-failed sorted by name
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "call", Name: "call (c)", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "call", Name: "call (b)", Status: "FAILED"},
		3: {TaskID: 3, Process: "call", Name: "call (a)", Status: "RUNNING"},
		4: {TaskID: 4, Process: "call", Name: "call (d)", Status: "FAILED"},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if len(g.Tasks) != 4 {
		t.Fatalf("Tasks length = %d, want 4", len(g.Tasks))
	}
	// Expected order: call(b) FAILED, call(d) FAILED, call(a) RUNNING, call(c) COMPLETED
	wantNames := []string{"call (b)", "call (d)", "call (a)", "call (c)"}
	wantStatuses := []state.TaskStatus{"FAILED", "FAILED", "RUNNING", "COMPLETED"}
	for i, task := range g.Tasks {
		if task.Name != wantNames[i] {
			t.Errorf("Tasks[%d].Name = %q, want %q", i, task.Name, wantNames[i])
		}
		if task.Status != wantStatuses[i] {
			t.Errorf("Tasks[%d].Status = %q, want %q", i, task.Status, wantStatuses[i])
		}
	}
}

func TestGroupProcesses_EmptyMapTasksSliceIsNil(t *testing.T) {
	// For an empty map, no groups are returned so there's nothing to check,
	// but verify the result is safely usable (no panic on iteration)
	got := groupProcesses(map[int]*state.Task{})
	for _, g := range got {
		if g.Tasks != nil {
			t.Errorf("unexpected Tasks slice in empty result: %v", g.Tasks)
		}
	}
}

func TestGroupProcesses_MultipleProcessesEachHasOwnTasks(t *testing.T) {
	// Tasks are correctly partitioned into their respective groups
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "alpha", Name: "alpha (1)", Status: "COMPLETED"},
		2: {TaskID: 2, Process: "beta", Name: "beta (1)", Status: "RUNNING"},
		3: {TaskID: 3, Process: "alpha", Name: "alpha (2)", Status: "FAILED"},
	}
	got := groupProcesses(tasks)
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	// Groups are sorted alphabetically: alpha, beta
	alphaGroup := got[0]
	betaGroup := got[1]
	if alphaGroup.Name != "alpha" {
		t.Fatalf("got[0].Name = %q, want alpha", alphaGroup.Name)
	}
	if betaGroup.Name != "beta" {
		t.Fatalf("got[1].Name = %q, want beta", betaGroup.Name)
	}
	if len(alphaGroup.Tasks) != 2 {
		t.Errorf("alpha group Tasks length = %d, want 2", len(alphaGroup.Tasks))
	}
	if len(betaGroup.Tasks) != 1 {
		t.Errorf("beta group Tasks length = %d, want 1", len(betaGroup.Tasks))
	}
	// Alpha group: FAILED first, then completed
	if alphaGroup.Tasks[0].Status != "FAILED" {
		t.Errorf("alpha Tasks[0].Status = %q, want FAILED", alphaGroup.Tasks[0].Status)
	}
	if alphaGroup.Tasks[1].Status != "COMPLETED" {
		t.Errorf("alpha Tasks[1].Status = %q, want COMPLETED", alphaGroup.Tasks[1].Status)
	}
	// Beta group: single task
	if betaGroup.Tasks[0].Name != "beta (1)" {
		t.Errorf("beta Tasks[0].Name = %q, want %q", betaGroup.Tasks[0].Name, "beta (1)")
	}
}

func TestGroupProcesses_CachedCountsAsCompleted(t *testing.T) {
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "say", Status: state.TaskStatusCached},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if g.Total != 1 {
		t.Errorf("Total = %d, want 1", g.Total)
	}
	if g.Completed != 1 {
		t.Errorf("Completed = %d, want 1", g.Completed)
	}
	if g.Running != 0 || g.Failed != 0 || g.Submitted != 0 {
		t.Errorf("expected cached task to count only as completed, got C=%d R=%d F=%d S=%d",
			g.Completed, g.Running, g.Failed, g.Submitted)
	}
}

func TestGroupProcesses_UnknownStatusCountsInTotalOnly(t *testing.T) {
	// A task with an unrecognized status should still be counted in Total
	// but not in any specific status bucket.
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "say", Status: state.TaskStatus("UNKNOWN")},
	}
	got := groupProcesses(tasks)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if g.Total != 1 {
		t.Errorf("Total = %d, want 1", g.Total)
	}
	if g.Completed != 0 || g.Running != 0 || g.Failed != 0 || g.Submitted != 0 {
		t.Errorf("expected all status counts 0 for unknown status, got C=%d R=%d F=%d S=%d",
			g.Completed, g.Running, g.Failed, g.Submitted)
	}
}
