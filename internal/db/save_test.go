package db

import (
	"testing"
)

func TestSave_SingleBlob(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.Save([]byte(`{"event":"started"}`)); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Verify row was inserted
	var count int
	if err := es.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestSave_MultipleBlobsAutoIncrementID(t *testing.T) {
	es := mustOpenTestStore(t)

	blobs := []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}
	for _, b := range blobs {
		if err := es.Save([]byte(b)); err != nil {
			t.Fatalf("Save(%q): %v", b, err)
		}
	}

	var count int
	if err := es.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 rows, got %d", count)
	}

	// Verify IDs are sequential
	rows, err := es.db.Query("SELECT id FROM events ORDER BY id")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	for i := 0; i < len(ids)-1; i++ {
		if ids[i+1] <= ids[i] {
			t.Fatalf("ids not ascending: %v", ids)
		}
	}
}

func TestSave_PreservesExactBytes(t *testing.T) {
	es := mustOpenTestStore(t)

	blob := []byte(`{"key":"value","nested":{"x":42}}`)
	if err := es.Save(blob); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var got []byte
	if err := es.db.QueryRow("SELECT raw_json FROM events").Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if string(got) != string(blob) {
		t.Fatalf("expected %q, got %q", blob, got)
	}
}

func TestSave_EmptyBlob(t *testing.T) {
	es := mustOpenTestStore(t)

	// Empty byte slice is still a valid NOT NULL blob
	if err := es.Save([]byte{}); err != nil {
		t.Fatalf("Save(empty): %v", err)
	}
}

func TestSave_SetsCreatedAt(t *testing.T) {
	es := mustOpenTestStore(t)

	if err := es.Save([]byte(`{}`)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var createdAt string
	if err := es.db.QueryRow("SELECT created_at FROM events").Scan(&createdAt); err != nil {
		t.Fatalf("select: %v", err)
	}
	if createdAt == "" {
		t.Fatal("expected non-empty created_at")
	}
}
