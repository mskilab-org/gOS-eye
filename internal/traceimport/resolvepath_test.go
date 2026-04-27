package traceimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolvePath_NoTraceRequested_ReturnsZeroResolution(t *testing.T) {
	got, err := ResolvePath(RunContext{CommandLine: "nextflow run main.nf -with-weblog http://localhost:8080/webhook"})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got != (PathResolution{}) {
		t.Fatalf("got %#v, want zero resolution", got)
	}
}

func TestResolvePath_ExplicitAbsolutePath_UsesPathAsIs(t *testing.T) {
	dir := t.TempDir()
	tracePath := mustWriteTraceFile(t, dir, "trace.txt", time.Now())

	got, err := ResolvePath(RunContext{CommandLine: "nextflow run main.nf -with-trace=" + tracePath})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.RequestedPath != tracePath {
		t.Fatalf("RequestedPath = %q, want %q", got.RequestedPath, tracePath)
	}
	if got.ResolvedPath != tracePath {
		t.Fatalf("ResolvedPath = %q, want %q", got.ResolvedPath, tracePath)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
}

func TestResolvePath_ExplicitSeparateRelativePath_ResolvesAgainstLaunchDir(t *testing.T) {
	dir := t.TempDir()
	tracePath := mustWriteTraceFile(t, dir, "custom trace.txt", time.Now())

	got, err := ResolvePath(RunContext{
		CommandLine: `nextflow run main.nf -with-trace "custom trace.txt"`,
		LaunchDir:   dir,
	})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.RequestedPath != "custom trace.txt" {
		t.Fatalf("RequestedPath = %q, want %q", got.RequestedPath, "custom trace.txt")
	}
	if got.ResolvedPath != tracePath {
		t.Fatalf("ResolvedPath = %q, want %q", got.ResolvedPath, tracePath)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
}

func TestResolvePath_ExplicitEqualsQuotedPath_ResolvesAgainstLaunchDir(t *testing.T) {
	dir := t.TempDir()
	tracePath := mustWriteTraceFile(t, dir, filepath.Join("logs", "run trace.txt"), time.Now())

	got, err := ResolvePath(RunContext{
		CommandLine: `nextflow run main.nf -with-trace='logs/run trace.txt'`,
		LaunchDir:   dir,
	})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.RequestedPath != filepath.Join("logs", "run trace.txt") {
		t.Fatalf("RequestedPath = %q, want %q", got.RequestedPath, filepath.Join("logs", "run trace.txt"))
	}
	if got.ResolvedPath != tracePath {
		t.Fatalf("ResolvedPath = %q, want %q", got.ResolvedPath, tracePath)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
}

func TestResolvePath_ExplicitMissingFile_WarnsButDoesNotError(t *testing.T) {
	dir := t.TempDir()

	got, err := ResolvePath(RunContext{
		CommandLine: "nextflow run main.nf -with-trace missing-trace.txt",
		LaunchDir:   dir,
	})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.RequestedPath != "missing-trace.txt" {
		t.Fatalf("RequestedPath = %q, want %q", got.RequestedPath, "missing-trace.txt")
	}
	if got.ResolvedPath != "" {
		t.Fatalf("ResolvedPath = %q, want empty", got.ResolvedPath)
	}
	if !strings.Contains(got.Warning, "missing-trace.txt") {
		t.Fatalf("Warning = %q, want mention of missing path", got.Warning)
	}
}

func TestResolvePath_BareTrace_DiscoversSingleTraceFile(t *testing.T) {
	dir := t.TempDir()
	tracePath := mustWriteTraceFile(t, dir, "trace-20260427-1.txt", time.Now())

	got, err := ResolvePath(RunContext{
		CommandLine: "nextflow run main.nf -with-trace",
		LaunchDir:   dir,
	})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.RequestedPath != "" {
		t.Fatalf("RequestedPath = %q, want empty", got.RequestedPath)
	}
	if got.ResolvedPath != tracePath {
		t.Fatalf("ResolvedPath = %q, want %q", got.ResolvedPath, tracePath)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
}

func TestResolvePath_BareTrace_PrefersCandidateNearestCompletionTime(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	complete := start.Add(5 * time.Minute)
	nearComplete := mustWriteTraceFile(t, dir, "trace-near-complete.txt", complete.Add(-1*time.Minute))
	_ = mustWriteTraceFile(t, dir, "trace-much-later.txt", complete.Add(55*time.Minute))

	got, err := ResolvePath(RunContext{
		CommandLine:  "nextflow run main.nf -with-trace",
		LaunchDir:    dir,
		StartTime:    start.Format(time.RFC3339),
		CompleteTime: complete.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.ResolvedPath != nearComplete {
		t.Fatalf("ResolvedPath = %q, want %q", got.ResolvedPath, nearComplete)
	}
	if got.Warning == "" {
		t.Fatalf("Warning = %q, want non-empty warning when guessing among multiple trace files", got.Warning)
	}
}

func TestResolvePath_BareTrace_NoMatchWarnsButDoesNotError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not-a-trace.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ResolvePath(RunContext{
		CommandLine: "nextflow run main.nf -with-trace",
		LaunchDir:   dir,
	})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.ResolvedPath != "" {
		t.Fatalf("ResolvedPath = %q, want empty", got.ResolvedPath)
	}
	if !strings.Contains(got.Warning, "trace-*.txt") {
		t.Fatalf("Warning = %q, want mention of trace-*.txt discovery", got.Warning)
	}
}

func TestResolvePath_BareTrace_WithoutLaunchDirWarnsButDoesNotError(t *testing.T) {
	got, err := ResolvePath(RunContext{CommandLine: "nextflow run main.nf -with-trace"})
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if got.ResolvedPath != "" {
		t.Fatalf("ResolvedPath = %q, want empty", got.ResolvedPath)
	}
	if !strings.Contains(strings.ToLower(got.Warning), "launchdir") {
		t.Fatalf("Warning = %q, want mention of launchDir", got.Warning)
	}
}

func mustWriteTraceFile(t *testing.T, dir, rel string, modTime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("trace\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	return path
}
