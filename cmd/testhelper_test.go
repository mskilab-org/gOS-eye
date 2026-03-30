package main

import (
	"path/filepath"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/db"
)

// mustOpenTestEventStore creates a temporary SQLite EventStore for testing.
// The store is automatically closed when the test completes via t.Cleanup.
func mustOpenTestEventStore(t *testing.T) *db.EventStore {
	t.Helper()
	es, err := db.OpenEventStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenEventStore() failed: %v", err)
	}
	t.Cleanup(func() { es.Close() })
	return es
}
