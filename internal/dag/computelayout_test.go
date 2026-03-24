package dag

import (
	"reflect"
	"testing"
)

func TestComputeLayout_Empty(t *testing.T) {
	dag := &DAG{
		Processes: []string{},
		Edges:     []Edge{},
		Children:  map[string][]string{},
		Parents:   map[string][]string{},
	}
	got := ComputeLayout(dag)
	if len(got.Nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(got.Edges))
	}
	if got.LayerCount != 0 {
		t.Fatalf("expected LayerCount 0, got %d", got.LayerCount)
	}
	if got.MaxWidth != 0 {
		t.Fatalf("expected MaxWidth 0, got %d", got.MaxWidth)
	}
}

func TestComputeLayout_SingleNode(t *testing.T) {
	dag := &DAG{
		Processes: []string{"A"},
		Edges:     []Edge{},
		Children:  map[string][]string{},
		Parents:   map[string][]string{},
	}
	got := ComputeLayout(dag)
	if got.LayerCount != 1 {
		t.Fatalf("expected LayerCount 1, got %d", got.LayerCount)
	}
	if got.MaxWidth != 1 {
		t.Fatalf("expected MaxWidth 1, got %d", got.MaxWidth)
	}
	if len(got.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(got.Nodes))
	}
	wantNode := NodeLayout{Name: "A", Layer: 0, Index: 0}
	if got.Nodes[0] != wantNode {
		t.Fatalf("got node %+v, want %+v", got.Nodes[0], wantNode)
	}
	if len(got.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(got.Edges))
	}
}

func TestComputeLayout_MockPipeline(t *testing.T) {
	// Mock pipeline DAG:
	//   BWA_MEM → FRAGCOUNTER, GRIDSS, SAGE, AMBER
	//   FRAGCOUNTER → DRYCLEAN
	//   DRYCLEAN → CBS, PURPLE
	//   CBS → JABBA, PURPLE → JABBA
	//   JABBA → EVENTS_FUSIONS
	//
	// Expected layers:
	//   0: BWA_MEM
	//   1: FRAGCOUNTER, GRIDSS, SAGE, AMBER
	//   2: DRYCLEAN
	//   3: CBS, PURPLE
	//   4: JABBA
	//   5: EVENTS_FUSIONS
	edges := []Edge{
		{From: "BWA_MEM", To: "FRAGCOUNTER"},
		{From: "BWA_MEM", To: "GRIDSS"},
		{From: "BWA_MEM", To: "SAGE"},
		{From: "BWA_MEM", To: "AMBER"},
		{From: "FRAGCOUNTER", To: "DRYCLEAN"},
		{From: "DRYCLEAN", To: "CBS"},
		{From: "DRYCLEAN", To: "PURPLE"},
		{From: "CBS", To: "JABBA"},
		{From: "PURPLE", To: "JABBA"},
		{From: "JABBA", To: "EVENTS_FUSIONS"},
	}
	dag := &DAG{
		Processes: []string{
			"BWA_MEM", "FRAGCOUNTER", "GRIDSS", "SAGE", "AMBER",
			"DRYCLEAN", "CBS", "PURPLE", "JABBA", "EVENTS_FUSIONS",
		},
		Edges: edges,
		Children: map[string][]string{
			"BWA_MEM":        {"FRAGCOUNTER", "GRIDSS", "SAGE", "AMBER"},
			"FRAGCOUNTER":    {"DRYCLEAN"},
			"GRIDSS":         {},
			"SAGE":           {},
			"AMBER":          {},
			"DRYCLEAN":       {"CBS", "PURPLE"},
			"CBS":            {"JABBA"},
			"PURPLE":         {"JABBA"},
			"JABBA":          {"EVENTS_FUSIONS"},
			"EVENTS_FUSIONS": {},
		},
		Parents: map[string][]string{
			"BWA_MEM":        {},
			"FRAGCOUNTER":    {"BWA_MEM"},
			"GRIDSS":         {"BWA_MEM"},
			"SAGE":           {"BWA_MEM"},
			"AMBER":          {"BWA_MEM"},
			"DRYCLEAN":       {"FRAGCOUNTER"},
			"CBS":            {"DRYCLEAN"},
			"PURPLE":         {"DRYCLEAN"},
			"JABBA":          {"CBS", "PURPLE"},
			"EVENTS_FUSIONS": {"JABBA"},
		},
	}

	got := ComputeLayout(dag)

	if got.LayerCount != 6 {
		t.Fatalf("expected LayerCount 6, got %d", got.LayerCount)
	}
	if got.MaxWidth != 4 {
		t.Fatalf("expected MaxWidth 4, got %d", got.MaxWidth)
	}
	if len(got.Nodes) != 10 {
		t.Fatalf("expected 10 nodes, got %d", len(got.Nodes))
	}

	wantNodes := []NodeLayout{
		{Name: "BWA_MEM", Layer: 0, Index: 0},
		{Name: "FRAGCOUNTER", Layer: 1, Index: 0},
		{Name: "GRIDSS", Layer: 1, Index: 1},
		{Name: "SAGE", Layer: 1, Index: 2},
		{Name: "AMBER", Layer: 1, Index: 3},
		{Name: "DRYCLEAN", Layer: 2, Index: 0},
		{Name: "CBS", Layer: 3, Index: 0},
		{Name: "PURPLE", Layer: 3, Index: 1},
		{Name: "JABBA", Layer: 4, Index: 0},
		{Name: "EVENTS_FUSIONS", Layer: 5, Index: 0},
	}
	if !reflect.DeepEqual(got.Nodes, wantNodes) {
		t.Fatalf("nodes mismatch\ngot:  %+v\nwant: %+v", got.Nodes, wantNodes)
	}

	if !reflect.DeepEqual(got.Edges, edges) {
		t.Fatalf("edges mismatch\ngot:  %+v\nwant: %+v", got.Edges, edges)
	}
}
