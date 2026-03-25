package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLogFile_ExistsWithContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".command.log")
	content := "hello\nworld\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogFile(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadLogFile_NotExist(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "no-such-file")

	got, err := readLogFile(p)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}
