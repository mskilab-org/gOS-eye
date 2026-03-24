package dag

import (
	"reflect"
	"testing"
)

func TestTopoSort_Empty(t *testing.T) {
	got, err := topoSort([]string{}, []Edge{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestTopoSort_SingleProcess_NoEdges(t *testing.T) {
	got, err := topoSort([]string{"A"}, []Edge{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_TwoProcesses_OneEdge(t *testing.T) {
	got, err := topoSort([]string{"A", "B"}, []Edge{{From: "A", To: "B"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_TwoProcesses_ReverseInputOrder(t *testing.T) {
	// B comes before A in input, but A→B means A should be first in output
	got, err := topoSort([]string{"B", "A"}, []Edge{{From: "A", To: "B"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_LinearChain(t *testing.T) {
	// A → B → C → D
	processes := []string{"A", "B", "C", "D"}
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "C"},
		{From: "C", To: "D"},
	}
	got, err := topoSort(processes, edges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "B", "C", "D"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_MultipleRoots_StableOrder(t *testing.T) {
	// No edges — all are roots. Output should match input order.
	processes := []string{"C", "A", "B"}
	got, err := topoSort(processes, []Edge{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"C", "A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_DiamondDAG(t *testing.T) {
	//   A
	//  / \
	// B   C
	//  \ /
	//   D
	processes := []string{"A", "B", "C", "D"}
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "A", To: "C"},
		{From: "B", To: "D"},
		{From: "C", To: "D"},
	}
	got, err := topoSort(processes, edges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A first (only root), then B before C (input order), then D
	want := []string{"A", "B", "C", "D"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_StableOrderAmongSameLayer(t *testing.T) {
	// Two independent chains: X→Y and A→B
	// Input order: X, A, Y, B
	// Both X and A are roots; stable order means X before A
	// After X processed: Y becomes available. After A processed: B becomes available.
	// Y and B become zero-in-degree; Y appears before B in input, so Y first.
	processes := []string{"X", "A", "Y", "B"}
	edges := []Edge{
		{From: "X", To: "Y"},
		{From: "A", To: "B"},
	}
	got, err := topoSort(processes, edges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"X", "A", "Y", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_CycleDetected_TwoNodes(t *testing.T) {
	// A → B → A (cycle)
	processes := []string{"A", "B"}
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "A"},
	}
	_, err := topoSort(processes, edges)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

func TestTopoSort_CycleDetected_ThreeNodes(t *testing.T) {
	// A → B → C → A (cycle)
	processes := []string{"A", "B", "C"}
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "C"},
		{From: "C", To: "A"},
	}
	_, err := topoSort(processes, edges)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

func TestTopoSort_ComplexDAG(t *testing.T) {
	//   A   B
	//   |\ /|
	//   | C |
	//   |/ \|
	//   D   E
	processes := []string{"A", "B", "C", "D", "E"}
	edges := []Edge{
		{From: "A", To: "C"},
		{From: "B", To: "C"},
		{From: "A", To: "D"},
		{From: "C", To: "D"},
		{From: "C", To: "E"},
		{From: "B", To: "E"},
	}
	got, err := topoSort(processes, edges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A, B are roots (A before B in input order)
	// After A, B processed: C becomes zero (A before B in input, but C depends on both)
	// After C: D becomes zero (needed A and C), E becomes zero (needed C and B)
	// D before E in input order
	want := []string{"A", "B", "C", "D", "E"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopoSort_NilEdges(t *testing.T) {
	got, err := topoSort([]string{"A", "B"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
