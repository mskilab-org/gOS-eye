package dag

import (
	"reflect"
	"testing"
)

func TestDeduplicateEdges_Empty(t *testing.T) {
	got := deduplicateEdges([]Edge{})
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestDeduplicateEdges_Nil(t *testing.T) {
	got := deduplicateEdges(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty result for nil input, got %v", got)
	}
}

func TestDeduplicateEdges_SingleEdge(t *testing.T) {
	edges := []Edge{{From: "A", To: "B"}}
	got := deduplicateEdges(edges)
	want := []Edge{{From: "A", To: "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeduplicateEdges_NoDuplicates(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "C"},
		{From: "A", To: "C"},
	}
	got := deduplicateEdges(edges)
	want := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "C"},
		{From: "A", To: "C"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeduplicateEdges_WithDuplicates(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "A", To: "C"},
		{From: "A", To: "B"},
	}
	got := deduplicateEdges(edges)
	want := []Edge{
		{From: "A", To: "B"},
		{From: "A", To: "C"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeduplicateEdges_AllDuplicates(t *testing.T) {
	edges := []Edge{
		{From: "X", To: "Y"},
		{From: "X", To: "Y"},
		{From: "X", To: "Y"},
	}
	got := deduplicateEdges(edges)
	want := []Edge{{From: "X", To: "Y"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeduplicateEdges_DirectionMatters(t *testing.T) {
	// A->B and B->A are different edges
	edges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "A"},
	}
	got := deduplicateEdges(edges)
	want := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "A"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeduplicateEdges_PreservesFirstOccurrenceOrder(t *testing.T) {
	edges := []Edge{
		{From: "C", To: "D"},
		{From: "A", To: "B"},
		{From: "C", To: "D"},
		{From: "E", To: "F"},
		{From: "A", To: "B"},
	}
	got := deduplicateEdges(edges)
	want := []Edge{
		{From: "C", To: "D"},
		{From: "A", To: "B"},
		{From: "E", To: "F"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
