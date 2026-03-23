package state

import (
	"sync"
	"testing"
)

// helper: creates a Store with pre-populated Runs map (bypasses NewStore which may panic).
func storeWith(runs map[string]*Run) *Store {
	return &Store{Runs: runs}
}

func TestGetRun_ExistingRun(t *testing.T) {
	r := &Run{RunID: "abc-123", RunName: "happy_run", Status: "running"}
	s := storeWith(map[string]*Run{"abc-123": r})

	got := s.GetRun("abc-123")
	if got != r {
		t.Fatalf("expected the stored *Run, got %v", got)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	s := storeWith(map[string]*Run{})

	got := s.GetRun("nonexistent")
	if got != nil {
		t.Fatalf("expected nil for missing run, got %v", got)
	}
}

func TestGetRun_EmptyMap(t *testing.T) {
	s := storeWith(map[string]*Run{})

	got := s.GetRun("")
	if got != nil {
		t.Fatalf("expected nil for empty-string key in empty map, got %v", got)
	}
}

func TestGetRun_MultipleRuns(t *testing.T) {
	r1 := &Run{RunID: "run-1", RunName: "first"}
	r2 := &Run{RunID: "run-2", RunName: "second"}
	s := storeWith(map[string]*Run{"run-1": r1, "run-2": r2})

	got := s.GetRun("run-2")
	if got != r2 {
		t.Fatalf("expected run-2, got %v", got)
	}

	// Verify run-1 is still accessible
	got1 := s.GetRun("run-1")
	if got1 != r1 {
		t.Fatalf("expected run-1, got %v", got1)
	}
}

func TestGetRun_ConcurrentReads(t *testing.T) {
	r := &Run{RunID: "concurrent", RunName: "test_run"}
	s := storeWith(map[string]*Run{"concurrent": r})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := s.GetRun("concurrent")
			if got != r {
				t.Errorf("expected stored *Run, got %v", got)
			}
		}()
	}
	wg.Wait()
}
