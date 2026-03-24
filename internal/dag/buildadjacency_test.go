package dag

import (
	"reflect"
	"testing"
)

func TestBuildAdjacency_Empty(t *testing.T) {
	children, parents := buildAdjacency([]Edge{})
	if len(children) != 0 {
		t.Fatalf("expected empty children, got %v", children)
	}
	if len(parents) != 0 {
		t.Fatalf("expected empty parents, got %v", parents)
	}
}

func TestBuildAdjacency_Nil(t *testing.T) {
	children, parents := buildAdjacency(nil)
	if len(children) != 0 {
		t.Fatalf("expected empty children, got %v", children)
	}
	if len(parents) != 0 {
		t.Fatalf("expected empty parents, got %v", parents)
	}
}

func TestBuildAdjacency_SingleEdge(t *testing.T) {
	edges := []Edge{{From: "A", To: "B"}}
	children, parents := buildAdjacency(edges)

	wantChildren := map[string][]string{"A": {"B"}, "B": {}}
	wantParents := map[string][]string{"A": {}, "B": {"A"}}

	if !reflect.DeepEqual(children, wantChildren) {
		t.Fatalf("children: got %v, want %v", children, wantChildren)
	}
	if !reflect.DeepEqual(parents, wantParents) {
		t.Fatalf("parents: got %v, want %v", parents, wantParents)
	}
}

func TestBuildAdjacency_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> C  (from the task example)
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "A", To: "C"},
		{From: "B", To: "C"},
	}
	children, parents := buildAdjacency(edges)

	wantChildren := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {},
	}
	wantParents := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A", "B"},
	}

	if !reflect.DeepEqual(children, wantChildren) {
		t.Fatalf("children: got %v, want %v", children, wantChildren)
	}
	if !reflect.DeepEqual(parents, wantParents) {
		t.Fatalf("parents: got %v, want %v", parents, wantParents)
	}
}

func TestBuildAdjacency_LinearChain(t *testing.T) {
	// A -> B -> C -> D
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "C"},
		{From: "C", To: "D"},
	}
	children, parents := buildAdjacency(edges)

	wantChildren := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"D"},
		"D": {},
	}
	wantParents := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
		"D": {"C"},
	}

	if !reflect.DeepEqual(children, wantChildren) {
		t.Fatalf("children: got %v, want %v", children, wantChildren)
	}
	if !reflect.DeepEqual(parents, wantParents) {
		t.Fatalf("parents: got %v, want %v", parents, wantParents)
	}
}

func TestBuildAdjacency_AllNodesHaveEntries(t *testing.T) {
	// Single leaf and single root should both have map entries (empty slices)
	edges := []Edge{{From: "ROOT", To: "LEAF"}}
	children, parents := buildAdjacency(edges)

	// ROOT has no parents entry but must still exist in parents map
	if _, ok := parents["ROOT"]; !ok {
		t.Fatal("ROOT missing from parents map")
	}
	if len(parents["ROOT"]) != 0 {
		t.Fatalf("ROOT should have 0 parents, got %v", parents["ROOT"])
	}

	// LEAF has no children entry but must still exist in children map
	if _, ok := children["LEAF"]; !ok {
		t.Fatal("LEAF missing from children map")
	}
	if len(children["LEAF"]) != 0 {
		t.Fatalf("LEAF should have 0 children, got %v", children["LEAF"])
	}
}

func TestBuildAdjacency_MultipleRootsAndLeaves(t *testing.T) {
	// Two roots (A, B) both feed into C
	edges := []Edge{
		{From: "A", To: "C"},
		{From: "B", To: "C"},
	}
	children, parents := buildAdjacency(edges)

	wantChildren := map[string][]string{
		"A": {"C"},
		"B": {"C"},
		"C": {},
	}
	wantParents := map[string][]string{
		"A": {},
		"B": {},
		"C": {"A", "B"},
	}

	if !reflect.DeepEqual(children, wantChildren) {
		t.Fatalf("children: got %v, want %v", children, wantChildren)
	}
	if !reflect.DeepEqual(parents, wantParents) {
		t.Fatalf("parents: got %v, want %v", parents, wantParents)
	}
}
