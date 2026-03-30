package db

import (
	"testing"
)

func TestSaveDAG_SingleInsert(t *testing.T) {
	es := mustOpenTestStore(t)

	dot := []byte("digraph { A -> B }")
	if err := es.SaveDAG("run-1", "my-project", dot); err != nil {
		t.Fatalf("SaveDAG returned error: %v", err)
	}

	var runID, projectName string
	var dotText []byte
	err := es.db.QueryRow("SELECT run_id, project_name, dot_text FROM dag_dots").Scan(&runID, &projectName, &dotText)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if runID != "run-1" {
		t.Fatalf("run_id: expected %q, got %q", "run-1", runID)
	}
	if projectName != "my-project" {
		t.Fatalf("project_name: expected %q, got %q", "my-project", projectName)
	}
	if string(dotText) != string(dot) {
		t.Fatalf("dot_text: expected %q, got %q", dot, dotText)
	}
}

func TestSaveDAG_ReplaceOnSameRunID(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.SaveDAG("run-1", "proj-v1", []byte("digraph { A -> B }")); err != nil {
		t.Fatalf("first SaveDAG: %v", err)
	}
	if err := es.SaveDAG("run-1", "proj-v2", []byte("digraph { X -> Y -> Z }")); err != nil {
		t.Fatalf("second SaveDAG: %v", err)
	}

	// Should have exactly 1 row — the replacement
	var count int
	if err := es.db.QueryRow("SELECT COUNT(*) FROM dag_dots").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after replace, got %d", count)
	}

	var projectName string
	var dotText []byte
	err := es.db.QueryRow("SELECT project_name, dot_text FROM dag_dots WHERE run_id = ?", "run-1").Scan(&projectName, &dotText)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if projectName != "proj-v2" {
		t.Fatalf("project_name: expected %q, got %q", "proj-v2", projectName)
	}
	if string(dotText) != "digraph { X -> Y -> Z }" {
		t.Fatalf("dot_text: expected replaced content, got %q", dotText)
	}
}

func TestSaveDAG_MultipleDifferentRunIDs(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.SaveDAG("run-a", "proj", []byte("digraph { A }")); err != nil {
		t.Fatalf("SaveDAG run-a: %v", err)
	}
	if err := es.SaveDAG("run-b", "proj", []byte("digraph { B }")); err != nil {
		t.Fatalf("SaveDAG run-b: %v", err)
	}
	if err := es.SaveDAG("run-c", "proj", []byte("digraph { C }")); err != nil {
		t.Fatalf("SaveDAG run-c: %v", err)
	}

	var count int
	if err := es.db.QueryRow("SELECT COUNT(*) FROM dag_dots").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 rows, got %d", count)
	}
}

func TestSaveDAG_PreservesExactBytes(t *testing.T) {
	es := mustOpenTestStore(t)

	// Binary-ish content with special chars and unicode
	dot := []byte("digraph { \"héllo\" -> \"wörld\" }\x00\xff")
	if err := es.SaveDAG("run-1", "proj", dot); err != nil {
		t.Fatalf("SaveDAG: %v", err)
	}

	var got []byte
	if err := es.db.QueryRow("SELECT dot_text FROM dag_dots WHERE run_id = ?", "run-1").Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if string(got) != string(dot) {
		t.Fatalf("round-trip mismatch:\n  want: %q\n  got:  %q", dot, got)
	}
}

func TestSaveDAG_EmptyDotText(t *testing.T) {
	es := mustOpenTestStore(t)

	// Empty byte slice is still a valid NOT NULL blob
	if err := es.SaveDAG("run-1", "proj", []byte{}); err != nil {
		t.Fatalf("SaveDAG(empty): %v", err)
	}

	var got []byte
	if err := es.db.QueryRow("SELECT dot_text FROM dag_dots WHERE run_id = ?", "run-1").Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty blob, got %q", got)
	}
}

func TestSaveDAG_SetsCreatedAt(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.SaveDAG("run-1", "proj", []byte("digraph {}")); err != nil {
		t.Fatalf("SaveDAG: %v", err)
	}

	var createdAt string
	if err := es.db.QueryRow("SELECT created_at FROM dag_dots WHERE run_id = ?", "run-1").Scan(&createdAt); err != nil {
		t.Fatalf("select: %v", err)
	}
	if createdAt == "" {
		t.Fatal("expected non-empty created_at")
	}
}
