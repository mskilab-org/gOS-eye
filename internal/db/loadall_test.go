package db

import (
	"path/filepath"
	"testing"
)

func TestLoadAll_EmptyTable(t *testing.T) {
	dir := t.TempDir()
	es, err := OpenEventStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer es.Close()

	blobs, err := es.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(blobs) != 0 {
		t.Fatalf("expected 0 blobs, got %d", len(blobs))
	}
}

func TestLoadAll_SingleEvent(t *testing.T) {
	dir := t.TempDir()
	es, err := OpenEventStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer es.Close()

	input := []byte(`{"event":"started"}`)
	if err := es.Save(input); err != nil {
		t.Fatalf("Save: %v", err)
	}

	blobs, err := es.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(blobs) != 1 {
		t.Fatalf("expected 1 blob, got %d", len(blobs))
	}
	if string(blobs[0]) != string(input) {
		t.Fatalf("expected %q, got %q", input, blobs[0])
	}
}

func TestLoadAll_MultipleEventsOrderedByID(t *testing.T) {
	dir := t.TempDir()
	es, err := OpenEventStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer es.Close()

	inputs := []string{`{"seq":1}`, `{"seq":2}`, `{"seq":3}`}
	for _, s := range inputs {
		if err := es.Save([]byte(s)); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	blobs, err := es.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(blobs) != 3 {
		t.Fatalf("expected 3 blobs, got %d", len(blobs))
	}
	for i, want := range inputs {
		if string(blobs[i]) != want {
			t.Fatalf("blob[%d]: expected %q, got %q", i, want, blobs[i])
		}
	}
}

func TestLoadAll_RoundTripPreservesBytes(t *testing.T) {
	dir := t.TempDir()
	es, err := OpenEventStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer es.Close()

	// Include special characters, unicode, nested JSON
	input := []byte(`{"msg":"héllo\nworld","data":[1,2,3],"nested":{"ok":true}}`)
	if err := es.Save(input); err != nil {
		t.Fatalf("Save: %v", err)
	}

	blobs, err := es.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(blobs) != 1 {
		t.Fatalf("expected 1 blob, got %d", len(blobs))
	}
	if string(blobs[0]) != string(input) {
		t.Fatalf("round-trip mismatch:\n  want: %q\n  got:  %q", input, blobs[0])
	}
}

func TestLoadAll_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open, save, close
	es1, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	if err := es1.Save([]byte(`{"run":"abc"}`)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	es1.Close()

	// Reopen and load
	es2, err := OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	defer es2.Close()

	blobs, err := es2.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(blobs) != 1 {
		t.Fatalf("expected 1 blob after reopen, got %d", len(blobs))
	}
	if string(blobs[0]) != `{"run":"abc"}` {
		t.Fatalf("expected %q, got %q", `{"run":"abc"}`, blobs[0])
	}
}
