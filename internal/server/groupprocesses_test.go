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

func TestGroupProcesses_UnknownStatusCountsInTotalOnly(t *testing.T) {
	// A task with an unrecognized status should still be counted in Total
	// but not in any specific status bucket.
	tasks := map[int]*state.Task{
		1: {TaskID: 1, Process: "say", Status: "CACHED"},
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
