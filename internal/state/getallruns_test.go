package state

import (
	"testing"
)

func TestGetAllRuns_EmptyStore(t *testing.T) {
	s := &Store{Runs: map[string]*Run{}}
	got := s.GetAllRuns()
	if len(got) != 0 {
		t.Errorf("expected 0 runs, got %d", len(got))
	}
}

func TestGetAllRuns_SingleRun(t *testing.T) {
	r := &Run{RunID: "abc123", RunName: "happy_euler", Status: "running", Tasks: map[int]*Task{}}
	s := &Store{Runs: map[string]*Run{"abc123": r}}

	got := s.GetAllRuns()
	if len(got) != 1 {
		t.Fatalf("expected 1 run, got %d", len(got))
	}
	if got[0].RunID != "abc123" {
		t.Errorf("expected RunID abc123, got %s", got[0].RunID)
	}
}

func TestGetAllRuns_MultipleRuns(t *testing.T) {
	r1 := &Run{RunID: "run1", RunName: "jolly_turing", Status: "running", Tasks: map[int]*Task{}}
	r2 := &Run{RunID: "run2", RunName: "sad_euler", Status: "completed", Tasks: map[int]*Task{}}
	r3 := &Run{RunID: "run3", RunName: "angry_curie", Status: "error", Tasks: map[int]*Task{}}
	s := &Store{Runs: map[string]*Run{
		"run1": r1,
		"run2": r2,
		"run3": r3,
	}}

	got := s.GetAllRuns()
	if len(got) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(got))
	}

	// Collect returned RunIDs into a set (order is arbitrary)
	ids := make(map[string]bool)
	for _, r := range got {
		ids[r.RunID] = true
	}
	for _, expected := range []string{"run1", "run2", "run3"} {
		if !ids[expected] {
			t.Errorf("missing RunID %s in result", expected)
		}
	}
}

func TestGetAllRuns_ReturnsSamePointers(t *testing.T) {
	r := &Run{RunID: "x", RunName: "test", Status: "running", Tasks: map[int]*Task{}}
	s := &Store{Runs: map[string]*Run{"x": r}}

	got := s.GetAllRuns()
	if len(got) != 1 {
		t.Fatalf("expected 1 run, got %d", len(got))
	}
	if got[0] != r {
		t.Error("expected returned pointer to be the same *Run stored in the map")
	}
}

func TestGetAllRuns_NilRunsMap(t *testing.T) {
	s := &Store{Runs: nil}
	got := s.GetAllRuns()
	if len(got) != 0 {
		t.Errorf("expected 0 runs for nil map, got %d", len(got))
	}
}
