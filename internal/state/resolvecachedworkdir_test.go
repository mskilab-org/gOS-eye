package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/traceimport"
)

func TestResolveCachedWorkdir_ExplicitTraceWorkdirWinsWithoutWarning(t *testing.T) {
	got := resolveCachedWorkdir("", traceimport.Row{
		Hash:    "",
		Workdir: "/trace/workdir",
	})

	if got.Workdir != "/trace/workdir" {
		t.Fatalf("Workdir = %q, want explicit trace workdir", got.Workdir)
	}
	if got.Provenance != WorkdirProvenanceTraceWorkdir {
		t.Fatalf("Provenance = %q, want %q", got.Provenance, WorkdirProvenanceTraceWorkdir)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
	if got.MatchCount != 0 {
		t.Fatalf("MatchCount = %d, want 0", got.MatchCount)
	}
}

func TestResolveCachedWorkdir_ExplicitTraceWorkdirWinsOverResolvableHash(t *testing.T) {
	runWorkDir := t.TempDir()
	hashWorkdir := filepath.Join(runWorkDir, "88", "fa3254def789")
	if err := os.MkdirAll(hashWorkdir, 0o755); err != nil {
		t.Fatalf("MkdirAll hashWorkdir: %v", err)
	}

	got := resolveCachedWorkdir(runWorkDir, traceimport.Row{
		Hash:    "88/fa3254",
		Workdir: "/trace/workdir",
	})

	if got.Workdir != "/trace/workdir" {
		t.Fatalf("Workdir = %q, want explicit trace workdir instead of hash workdir %q", got.Workdir, hashWorkdir)
	}
	if got.Provenance != WorkdirProvenanceTraceWorkdir {
		t.Fatalf("Provenance = %q, want %q", got.Provenance, WorkdirProvenanceTraceWorkdir)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
}

func TestResolveCachedWorkdir_EmptyWorkdirDelegatesToHashResolution(t *testing.T) {
	runWorkDir := t.TempDir()
	match := filepath.Join(runWorkDir, "88", "fa3254def789")
	if err := os.MkdirAll(match, 0o755); err != nil {
		t.Fatalf("MkdirAll match: %v", err)
	}

	got := resolveCachedWorkdir(runWorkDir, traceimport.Row{
		Hash: "88/fa3254",
	})

	if got.Workdir != match {
		t.Fatalf("Workdir = %q, want hash-resolved workdir %q", got.Workdir, match)
	}
	if got.Provenance != WorkdirProvenanceHashResolved {
		t.Fatalf("Provenance = %q, want %q", got.Provenance, WorkdirProvenanceHashResolved)
	}
	if got.Warning != "" {
		t.Fatalf("Warning = %q, want empty", got.Warning)
	}
	if got.MatchCount != 1 {
		t.Fatalf("MatchCount = %d, want 1", got.MatchCount)
	}
}

func TestResolveCachedWorkdir_EmptyWorkdirInvalidInputsAreUnresolvedWithWarning(t *testing.T) {
	runWorkDir := t.TempDir()
	cases := []struct {
		name       string
		runWorkDir string
		row        traceimport.Row
		wantInWarn string
	}{
		{name: "blank run workdir", runWorkDir: "", row: traceimport.Row{Hash: "88/fa3254"}, wantInWarn: "workDir"},
		{name: "blank hash", runWorkDir: runWorkDir, row: traceimport.Row{Hash: ""}, wantInWarn: "hash"},
		{name: "malformed hash", runWorkDir: runWorkDir, row: traceimport.Row{Hash: "88fa3254"}, wantInWarn: "hash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCachedWorkdir(tc.runWorkDir, tc.row)

			if got.Workdir != "" {
				t.Fatalf("Workdir = %q, want empty", got.Workdir)
			}
			if got.Provenance != WorkdirProvenanceUnresolved {
				t.Fatalf("Provenance = %q, want %q", got.Provenance, WorkdirProvenanceUnresolved)
			}
			if got.MatchCount != 0 {
				t.Fatalf("MatchCount = %d, want 0", got.MatchCount)
			}
			assertWarningMentions(t, got.Warning, tc.wantInWarn)
		})
	}
}
