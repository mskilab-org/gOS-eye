package db

import (
	"sort"
	"testing"
)

func TestHideRun_PersistsAndLoads(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.HideRun("run-1"); err != nil {
		t.Fatalf("HideRun: %v", err)
	}
	if err := es.HideRun("run-2"); err != nil {
		t.Fatalf("HideRun: %v", err)
	}

	ids, err := es.LoadHiddenRuns()
	if err != nil {
		t.Fatalf("LoadHiddenRuns: %v", err)
	}
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != "run-1" || ids[1] != "run-2" {
		t.Fatalf("expected [run-1 run-2], got %v", ids)
	}
}

func TestHideRun_DuplicateIsNoop(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.HideRun("run-1"); err != nil {
		t.Fatalf("first HideRun: %v", err)
	}
	if err := es.HideRun("run-1"); err != nil {
		t.Fatalf("duplicate HideRun should not error: %v", err)
	}

	ids, err := es.LoadHiddenRuns()
	if err != nil {
		t.Fatalf("LoadHiddenRuns: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 hidden run after duplicate, got %d", len(ids))
	}
}

func TestUnhideRun_RemovesFromHidden(t *testing.T) {
	es := mustOpenTestStore(t)

	es.HideRun("run-1")
	es.HideRun("run-2")

	if err := es.UnhideRun("run-1"); err != nil {
		t.Fatalf("UnhideRun: %v", err)
	}

	ids, err := es.LoadHiddenRuns()
	if err != nil {
		t.Fatalf("LoadHiddenRuns: %v", err)
	}
	if len(ids) != 1 || ids[0] != "run-2" {
		t.Fatalf("expected [run-2], got %v", ids)
	}
}

func TestUnhideRun_NonexistentIsNoop(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.UnhideRun("nonexistent"); err != nil {
		t.Fatalf("UnhideRun on nonexistent should not error: %v", err)
	}
}

func TestLoadHiddenRuns_EmptyByDefault(t *testing.T) {
	es := mustOpenTestStore(t)

	ids, err := es.LoadHiddenRuns()
	if err != nil {
		t.Fatalf("LoadHiddenRuns: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty list, got %v", ids)
	}
}
