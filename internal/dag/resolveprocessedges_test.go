package dag

import "testing"

// Helper: build nodesByID from a slice of dotNodes
func makeNodesByID(nodes []dotNode) map[string]dotNode {
	m := make(map[string]dotNode, len(nodes))
	for _, n := range nodes {
		m[n.ID] = n
	}
	return m
}

// Helper: check that edges match expected (order-insensitive)
func edgesMatch(t *testing.T, got []Edge, want []Edge) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d edges, got %d: %v", len(want), len(got), got)
	}
	type key struct{ From, To string }
	counts := map[key]int{}
	for _, e := range want {
		counts[key{e.From, e.To}]++
	}
	for _, e := range got {
		k := key{e.From, e.To}
		counts[k]--
		if counts[k] < 0 {
			t.Fatalf("unexpected edge %v in result %v", e, got)
		}
	}
}

func TestResolveProcessEdges_Empty(t *testing.T) {
	// No processes, no edges → empty result
	edges := resolveProcessEdges(nil, nil, nil)
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
}

func TestResolveProcessEdges_DirectProcessToProcess(t *testing.T) {
	// P1 -> P2 directly (no intermediate operator)
	// v1 -> v2
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{{From: "v1", To: "v2"}}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	edgesMatch(t, got, []Edge{{From: "P1", To: "P2"}})
}

func TestResolveProcessEdges_ThroughOneOperator(t *testing.T) {
	// P1 -> op -> P2 (one operator in between)
	// v1 -> v3 -> v2
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
		{ID: "v3", Label: "", Shape: "circle"},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{
		{From: "v1", To: "v3"},
		{From: "v3", To: "v2"},
	}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	edgesMatch(t, got, []Edge{{From: "P1", To: "P2"}})
}

func TestResolveProcessEdges_ThroughMultipleOperators(t *testing.T) {
	// P1 -> op1 -> op2 -> op3 -> P2 (chain of operators)
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
		{ID: "v3", Label: "", Shape: "circle"},
		{ID: "v4", Label: "", Shape: "point"},
		{ID: "v5", Label: "", Shape: "circle"},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{
		{From: "v1", To: "v3"},
		{From: "v3", To: "v4"},
		{From: "v4", To: "v5"},
		{From: "v5", To: "v2"},
	}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	edgesMatch(t, got, []Edge{{From: "P1", To: "P2"}})
}

func TestResolveProcessEdges_FanOut(t *testing.T) {
	// P1 fans out to P2 and P3 through operators
	// v1 -> v10 -> v2, v10 -> v3
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
		{ID: "v3", Label: "P3", Shape: ""},
		{ID: "v10", Label: "", Shape: "circle"},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2", "v3": "P3"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{
		{From: "v1", To: "v10"},
		{From: "v10", To: "v2"},
		{From: "v10", To: "v3"},
	}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	edgesMatch(t, got, []Edge{
		{From: "P1", To: "P2"},
		{From: "P1", To: "P3"},
	})
}

func TestResolveProcessEdges_FanIn(t *testing.T) {
	// P1 and P2 both feed into P3 through an operator
	// v1 -> v10, v2 -> v10, v10 -> v3
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
		{ID: "v3", Label: "P3", Shape: ""},
		{ID: "v10", Label: "", Shape: "circle"},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2", "v3": "P3"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{
		{From: "v1", To: "v10"},
		{From: "v2", To: "v10"},
		{From: "v10", To: "v3"},
	}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	edgesMatch(t, got, []Edge{
		{From: "P1", To: "P3"},
		{From: "P2", To: "P3"},
	})
}

func TestResolveProcessEdges_DoesNotBFSThroughProcessNodes(t *testing.T) {
	// P1 -> P2 -> P3: BFS from P1 should only reach P2, not P3
	// Because P2 is a process node and is a boundary
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
		{ID: "v3", Label: "P3", Shape: ""},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2", "v3": "P3"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{
		{From: "v1", To: "v2"},
		{From: "v2", To: "v3"},
	}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	// P1→P2 from BFS(P1), P2→P3 from BFS(P2). No P1→P3.
	edgesMatch(t, got, []Edge{
		{From: "P1", To: "P2"},
		{From: "P2", To: "P3"},
	})
}

func TestResolveProcessEdges_NoOutgoingEdges(t *testing.T) {
	// Single process with no outgoing edges (terminal)
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
	}
	processIDs := map[string]string{"v1": "P1"}
	nodesByID := makeNodesByID(nodes)

	got := resolveProcessEdges(processIDs, nodesByID, nil)
	if len(got) != 0 {
		t.Fatalf("expected 0 edges for terminal process, got %d: %v", len(got), got)
	}
}

func TestResolveProcessEdges_DiamondGraph(t *testing.T) {
	// Diamond: P1 -> op1 -> P2, P1 -> op2 -> P3, P2 -> op3 -> P4, P3 -> op3 -> P4
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v2", Label: "P2", Shape: ""},
		{ID: "v3", Label: "P3", Shape: ""},
		{ID: "v4", Label: "P4", Shape: ""},
		{ID: "v10", Label: "", Shape: "circle"},
		{ID: "v11", Label: "", Shape: "circle"},
		{ID: "v12", Label: "", Shape: "circle"},
	}
	processIDs := map[string]string{"v1": "P1", "v2": "P2", "v3": "P3", "v4": "P4"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{
		{From: "v1", To: "v10"},
		{From: "v10", To: "v2"},
		{From: "v1", To: "v11"},
		{From: "v11", To: "v3"},
		{From: "v2", To: "v12"},
		{From: "v3", To: "v12"},
		{From: "v12", To: "v4"},
	}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	edgesMatch(t, got, []Edge{
		{From: "P1", To: "P2"},
		{From: "P1", To: "P3"},
		{From: "P2", To: "P4"},
		{From: "P3", To: "P4"},
	})
}

func TestResolveProcessEdges_OperatorDeadEnd(t *testing.T) {
	// P1 -> op (op has no further edges) → no edges emitted
	nodes := []dotNode{
		{ID: "v1", Label: "P1", Shape: ""},
		{ID: "v10", Label: "", Shape: "point"},
	}
	processIDs := map[string]string{"v1": "P1"}
	nodesByID := makeNodesByID(nodes)
	dotEdges := []dotEdge{{From: "v1", To: "v10"}}

	got := resolveProcessEdges(processIDs, nodesByID, dotEdges)
	if len(got) != 0 {
		t.Fatalf("expected 0 edges when operator is dead-end, got %d: %v", len(got), got)
	}
}
