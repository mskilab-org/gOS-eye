package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadLogTail_ExistsWithContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".command.log")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadLogTail_NotExist(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "no-such-file")

	got, err := readLogTail(p, 50)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}

func TestReadLogTail_TruncatesToLastN(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "line4\nline5\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadLogTail_MaxLinesZero_ReturnsAll(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "aaa\nbbb\nccc\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadLogTail_MaxLinesNegative_ReturnsAll(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "x\ny\nz\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadLogTail_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	if err := os.WriteFile(p, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestReadLogTail_SingleLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "only line\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadLogTail_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "line1\nline2\nline3"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "line2\nline3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadLogTail_MaxLinesExactlyFileLength(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "a\nb\nc\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadLogTail_MaxLinesOne_MultipleLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	content := "first\nsecond\nthird\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLogTail(p, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "third\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadLogTail_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "log")
	if err := os.WriteFile(p, []byte("secret\n"), 0000); err != nil {
		t.Fatal(err)
	}

	_, err := readLogTail(p, 10)
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
}
