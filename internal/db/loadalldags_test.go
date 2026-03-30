package db

import (
	"path/filepath"
	"testing"
)

func TestLoadAllDAGs_EmptyTable(t *testing.T) {
	es := mustOpenTestStore(t)

	recs, err := es.LoadAllDAGs()
	if err != nil {
		t.Fatalf("LoadAllDAGs: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 records, got %d", len(recs))
	}
}

func TestLoadAllDAGs_SingleRecord(t *testing.T) {
	es := mustOpenTestStore(t)

	dot := []byte("digraph { A -> B }")
	if err := es.SaveDAG("run-1", "my-project", dot); err != nil {
		t.Fatalf("SaveDAG: %v", err)
	}

	recs, err := es.LoadAllDAGs()
	if err != nil {
		t.Fatalf("LoadAllDAGs: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].RunID != "run-1" {
		t.Fatalf("RunID: expected %q, got %q", "run-1", recs[0].RunID)
	}
	if recs[0].ProjectName != "my-project" {
		t.Fatalf("ProjectName: expected %q, got %q", "my-project", recs[0].ProjectName)
	}
	if string(recs[0].DotText) != string(dot) {
		t.Fatalf("DotText: expected %q, got %q", dot, recs[0].DotText)
	}
}

func TestLoadAllDAGs_MultipleRecordsOrderedByCreatedAt(t *testing.T) {
	es := mustOpenTestStore(t)

	// Insert with explicit created_at to guarantee ordering
	_, err := es.db.Exec(
		"INSERT INTO dag_dots (run_id, project_name, dot_text, created_at) VALUES (?, ?, ?, ?)",
		"run-a", "proj-a", []byte("digraph { A }"), "2025-01-01 00:00:01",
	)
	if err != nil {
		t.Fatalf("insert run-a: %v", err)
	}
	_, err = es.db.Exec(
		"INSERT INTO dag_dots (run_id, project_name, dot_text, created_at) VALUES (?, ?, ?, ?)",
		"run-b", "proj-b", []byte("digraph { B }"), "2025-01-01 00:00:02",
	)
	if err != nil {
		t.Fatalf("insert run-b: %v", err)
	}
	_, err = es.db.Exec(
		"INSERT INTO dag_dots (run_id, project_name, dot_text, created_at) VALUES (?, ?, ?, ?)",
		"run-c", "proj-c", []byte("digraph { C }"), "2025-01-01 00:00:03",
	)
	if err != nil {
		t.Fatalf("insert run-c: %v", err)
	}

	recs, err := es.LoadAllDAGs()
	if err != nil {
		t.Fatalf("LoadAllDAGs: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}

	wantIDs := []string{"run-a", "run-b", "run-c"}
	wantProjects := []string{"proj-a", "proj-b", "proj-c"}
	wantDots := []string{"digraph { A }", "digraph { B }", "digraph { C }"}
	for i := range recs {
		if recs[i].RunID != wantIDs[i] {
			t.Fatalf("rec[%d].RunID: expected %q, got %q", i, wantIDs[i], recs[i].RunID)
		}
		if recs[i].ProjectName != wantProjects[i] {
			t.Fatalf("rec[%d].ProjectName: expected %q, got %q", i, wantProjects[i], recs[i].ProjectName)
		}
		if string(recs[i].DotText) != wantDots[i] {
			t.Fatalf("rec[%d].DotText: expected %q, got %q", i, wantDots[i], recs[i].DotText)
		}
	}
}

func TestLoadAllDAGs_RoundTripPreservesBytes(t *testing.T) {
	es := mustOpenTestStore(t)

	// Binary-ish content with special chars and unicode
	dot := []byte("digraph { \"héllo\" -> \"wörld\" }\x00\xff")
	if err := es.SaveDAG("run-1", "proj-üñ", dot); err != nil {
		t.Fatalf("SaveDAG: %v", err)
	}

	recs, err := es.LoadAllDAGs()
	if err != nil {
		t.Fatalf("LoadAllDAGs: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].RunID != "run-1" {
		t.Fatalf("RunID mismatch")
	}
	if recs[0].ProjectName != "proj-üñ" {
		t.Fatalf("ProjectName: expected %q, got %q", "proj-üñ", recs[0].ProjectName)
	}
	if string(recs[0].DotText) != string(dot) {
		t.Fatalf("round-trip mismatch:\n  want: %q\n  got:  %q", dot, recs[0].DotText)
	}
}

func TestLoadAllDAGs_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open, save, close
	es1, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	if err := es1.SaveDAG("run-1", "proj", []byte("digraph { A -> B }")); err != nil {
		t.Fatalf("SaveDAG: %v", err)
	}
	es1.Close()

	// Reopen and load
	es2, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	defer es2.Close()

	recs, err := es2.LoadAllDAGs()
	if err != nil {
		t.Fatalf("LoadAllDAGs: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record after reopen, got %d", len(recs))
	}
	if recs[0].RunID != "run-1" {
		t.Fatalf("RunID: expected %q, got %q", "run-1", recs[0].RunID)
	}
	if string(recs[0].DotText) != "digraph { A -> B }" {
		t.Fatalf("DotText: expected %q, got %q", "digraph { A -> B }", recs[0].DotText)
	}
}
