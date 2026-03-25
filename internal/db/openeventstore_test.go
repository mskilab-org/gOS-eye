package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenEventStore_CreatesNewDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	es, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer es.Close()

	// File should exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("expected database file to be created")
	}
}

func TestOpenEventStore_ReturnsNonNilEventStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	es, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer es.Close()

	if es == nil {
		t.Fatal("expected non-nil EventStore")
	}
}

func TestOpenEventStore_CreatesEventsTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	es, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer es.Close()

	// Query sqlite_master to verify the events table exists
	var name string
	err = es.db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='events'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("events table not found: %v", err)
	}
	if name != "events" {
		t.Fatalf("expected table name 'events', got %q", name)
	}
}

func TestOpenEventStore_EventsTableSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	es, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer es.Close()

	// Verify columns via PRAGMA table_info
	rows, err := es.db.Query("PRAGMA table_info(events)")
	if err != nil {
		t.Fatalf("PRAGMA table_info failed: %v", err)
	}
	defer rows.Close()

	type colInfo struct {
		name    string
		colType string
		notNull bool
		pk      bool
	}
	cols := make(map[string]colInfo)
	for rows.Next() {
		var cid int
		var cname, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan error: %v", err)
		}
		cols[cname] = colInfo{name: cname, colType: ctype, notNull: notnull == 1, pk: pk == 1}
	}

	// id column
	id, ok := cols["id"]
	if !ok {
		t.Fatal("missing 'id' column")
	}
	if !id.pk {
		t.Fatal("'id' should be primary key")
	}

	// raw_json column
	rj, ok := cols["raw_json"]
	if !ok {
		t.Fatal("missing 'raw_json' column")
	}
	if !rj.notNull {
		t.Fatal("'raw_json' should be NOT NULL")
	}

	// created_at column
	if _, ok := cols["created_at"]; !ok {
		t.Fatal("missing 'created_at' column")
	}
}

func TestOpenEventStore_WALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	es, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer es.Close()

	var mode string
	err = es.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode failed: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected WAL journal mode, got %q", mode)
	}
}

func TestOpenEventStore_IdempotentTableCreation(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open twice — second open should not fail on existing table
	es1, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	es1.Close()

	es2, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("second open should not fail: %v", err)
	}
	es2.Close()
}

func TestOpenEventStore_InvalidPath(t *testing.T) {
	// Path in a non-existent directory
	_, err := OpenEventStore("/no/such/dir/test.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestClose_DelegateToSqlDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	es, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should succeed
	if err := es.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	// After close, querying should fail
	err = es.db.Ping()
	if err == nil {
		t.Fatal("expected error after Close(), but Ping() succeeded")
	}
}
