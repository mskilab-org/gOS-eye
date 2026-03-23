package state

import "testing"

func TestNewStore_ReturnsNonNil(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore() returned nil")
	}
}

func TestNewStore_RunsMapInitialized(t *testing.T) {
	s := NewStore()
	if s.Runs == nil {
		t.Fatal("NewStore().Runs is nil, expected initialized empty map")
	}
}

func TestNewStore_RunsMapEmpty(t *testing.T) {
	s := NewStore()
	if len(s.Runs) != 0 {
		t.Fatalf("NewStore().Runs has %d entries, expected 0", len(s.Runs))
	}
}

func TestNewStore_IndependentInstances(t *testing.T) {
	s1 := NewStore()
	s2 := NewStore()
	s1.Runs["test"] = &Run{RunID: "test"}
	if len(s2.Runs) != 0 {
		t.Fatal("modifying one Store affected another — maps are shared")
	}
}
