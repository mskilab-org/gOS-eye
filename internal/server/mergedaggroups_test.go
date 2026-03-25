package server

import (
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestMergeDAGGroups_EmptyLayout(t *testing.T) {
	layout := &dag.Layout{Nodes: []dag.NodeLayout{}}
	groups := []ProcessGroup{{Name: "A", Total: 1}}
	got := mergeDAGGroups(layout, groups)
	// Groups not in DAG are appended at end
	if len(got) != 1 || got[0].Name != "A" {
		t.Errorf("expected [A], got %v", groupNames(got))
	}
}

func TestMergeDAGGroups_AllProcessesPresent(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "FASTQC", Layer: 0},
			{Name: "ALIGN", Layer: 1},
			{Name: "COUNT", Layer: 2},
		},
	}
	groups := []ProcessGroup{
		{Name: "COUNT", Total: 2},
		{Name: "ALIGN", Total: 3},
		{Name: "FASTQC", Total: 5},
	}
	got := mergeDAGGroups(layout, groups)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(got))
	}
	// Order should follow DAG, not alphabetical
	if got[0].Name != "FASTQC" || got[1].Name != "ALIGN" || got[2].Name != "COUNT" {
		t.Errorf("expected DAG order [FASTQC, ALIGN, COUNT], got %v", groupNames(got))
	}
	// Data preserved
	if got[0].Total != 5 || got[1].Total != 3 || got[2].Total != 2 {
		t.Errorf("expected totals [5, 3, 2], got [%d, %d, %d]", got[0].Total, got[1].Total, got[2].Total)
	}
}

func TestMergeDAGGroups_PreSeedsMissingProcesses(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "FASTQC", Layer: 0},
			{Name: "ALIGN", Layer: 1},
			{Name: "COUNT", Layer: 2},
		},
	}
	// Only FASTQC has tasks so far
	groups := []ProcessGroup{
		{Name: "FASTQC", Total: 3, Completed: 1},
	}
	got := mergeDAGGroups(layout, groups)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups (all DAG nodes), got %d", len(got))
	}
	if got[0].Name != "FASTQC" || got[0].Total != 3 {
		t.Errorf("expected FASTQC with 3 tasks, got %s with %d", got[0].Name, got[0].Total)
	}
	// ALIGN and COUNT should be empty placeholders
	if got[1].Name != "ALIGN" || got[1].Total != 0 {
		t.Errorf("expected ALIGN with 0 tasks, got %s with %d", got[1].Name, got[1].Total)
	}
	if got[2].Name != "COUNT" || got[2].Total != 0 {
		t.Errorf("expected COUNT with 0 tasks, got %s with %d", got[2].Name, got[2].Total)
	}
}

func TestMergeDAGGroups_NoTasksYet(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0},
			{Name: "B", Layer: 1},
		},
	}
	got := mergeDAGGroups(layout, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Errorf("expected [A, B], got %v", groupNames(got))
	}
	if got[0].Total != 0 || got[1].Total != 0 {
		t.Errorf("expected all totals 0")
	}
}

func TestMergeDAGGroups_PreservesTaskSlice(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "PROC", Layer: 0},
		},
	}
	tasks := []*state.Task{
		{TaskID: 1, Name: "PROC (1)", Status: "COMPLETED"},
	}
	groups := []ProcessGroup{
		{Name: "PROC", Total: 1, Completed: 1, Tasks: tasks},
	}
	got := mergeDAGGroups(layout, groups)
	if len(got[0].Tasks) != 1 || got[0].Tasks[0].TaskID != 1 {
		t.Errorf("expected task slice preserved")
	}
}

func TestMergeDAGGroups_GroupNotInDAG(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0},
		},
	}
	groups := []ProcessGroup{
		{Name: "A", Total: 1},
		{Name: "ORPHAN", Total: 2},
	}
	got := mergeDAGGroups(layout, groups)
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	if got[0].Name != "A" || got[1].Name != "ORPHAN" {
		t.Errorf("expected [A, ORPHAN], got %v", groupNames(got))
	}
}

func groupNames(groups []ProcessGroup) []string {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Name
	}
	return names
}
