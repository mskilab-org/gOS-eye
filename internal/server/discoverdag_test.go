package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDAG_ValidFixture(t *testing.T) {
	// Use the real fixture at tests/mock-pipeline/dag.dot
	// discoverDAG takes a scriptFile path and looks for dag.dot in the same directory
	scriptFile := filepath.Join("../../tests/mock-pipeline", "main.nf")
	layout := discoverDAG(scriptFile)
	if layout == nil {
		t.Fatal("expected non-nil Layout for valid dag.dot fixture")
	}
	// The fixture has 10 process nodes (per parsedot_test.go)
	if len(layout.Nodes) != 10 {
		t.Errorf("expected 10 nodes, got %d", len(layout.Nodes))
	}
}

func TestDiscoverDAG_NoDotFile(t *testing.T) {
	// Point to a directory that exists but has no dag.dot
	dir := t.TempDir()
	scriptFile := filepath.Join(dir, "main.nf")
	layout := discoverDAG(scriptFile)
	if layout != nil {
		t.Errorf("expected nil when dag.dot doesn't exist, got %+v", layout)
	}
}

func TestDiscoverDAG_InvalidDotFile(t *testing.T) {
	// Create a directory with a dag.dot that contains invalid/unparseable content
	dir := t.TempDir()
	dotPath := filepath.Join(dir, "dag.dot")
	err := os.WriteFile(dotPath, []byte("this is not valid DOT content\n{{{}}}\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid dag.dot: %v", err)
	}
	scriptFile := filepath.Join(dir, "main.nf")
	layout := discoverDAG(scriptFile)
	// Invalid DOT that parses into zero processes should still return a layout
	// (ParseDOT doesn't error on garbage, it just finds no nodes)
	// But the layout would have 0 nodes. Let's verify it doesn't panic.
	// Actually, ParseDOT won't error on garbage text — it just returns an empty DAG.
	// The function should handle this gracefully either way.
	_ = layout // no panic = success
}

func TestDiscoverDAG_EmptyScriptFile(t *testing.T) {
	// Empty string → filepath.Dir("") = "." — unlikely to have dag.dot
	// but should not panic
	layout := discoverDAG("")
	// We can't guarantee there's no dag.dot in ".", so just verify no panic
	_ = layout
}

func TestDiscoverDAG_EmptyDotFile(t *testing.T) {
	// Create a directory with an empty dag.dot file
	dir := t.TempDir()
	dotPath := filepath.Join(dir, "dag.dot")
	err := os.WriteFile(dotPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write empty dag.dot: %v", err)
	}
	scriptFile := filepath.Join(dir, "main.nf")
	layout := discoverDAG(scriptFile)
	// Empty file parses into empty DAG — ComputeLayout should still work
	// Just ensure no panic
	_ = layout
}
