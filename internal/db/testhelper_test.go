package db

import (
	"path/filepath"
	"testing"
)

// mustOpenTestStore creates a temporary SQLite EventStore for testing.
// The store is automatically closed when the test completes via t.Cleanup.
func mustOpenTestStore(t *testing.T) *EventStore {
	t.Helper()
	es, err := OpenEventStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { es.Close() })
	return es
}
