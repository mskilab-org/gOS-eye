package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCachedWorkdirFromHash_ExactlyOnePrefixDirectoryMatch(t *testing.T) {
	runWorkDir := t.TempDir()
	match := filepath.Join(runWorkDir, "88", "fa3254def789")
	if err := os.MkdirAll(match, 0o755); err != nil {
		t.Fatalf("MkdirAll match: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runWorkDir, "88", "fa3254-file-not-dir"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile non-directory match: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runWorkDir, "88", "not-fa3254"), 0o755); err != nil {
		t.Fatalf("MkdirAll nonmatch: %v", err)
	}

	got := resolveCachedWorkdirFromHash(runWorkDir, "88/fa3254")

	if got.Workdir != match {
		t.Fatalf("Workdir = %q, want %q", got.Workdir, match)
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

func TestResolveCachedWorkdirFromHash_NoMatchesIsUnresolved(t *testing.T) {
	runWorkDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runWorkDir, "88", "different"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got := resolveCachedWorkdirFromHash(runWorkDir, "88/fa3254")

	if got.Workdir != "" {
		t.Fatalf("Workdir = %q, want empty", got.Workdir)
	}
	if got.Provenance != WorkdirProvenanceUnresolved {
		t.Fatalf("Provenance = %q, want %q", got.Provenance, WorkdirProvenanceUnresolved)
	}
	if got.MatchCount != 0 {
		t.Fatalf("MatchCount = %d, want 0", got.MatchCount)
	}
	assertWarningMentions(t, got.Warning, "88/fa3254")
}

func TestResolveCachedWorkdirFromHash_AmbiguousMatchesDoNotChoose(t *testing.T) {
	runWorkDir := t.TempDir()
	for _, name := range []string{"fa3254aaa", "fa3254bbb"} {
		if err := os.MkdirAll(filepath.Join(runWorkDir, "88", name), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", name, err)
		}
	}

	got := resolveCachedWorkdirFromHash(runWorkDir, "88/fa3254")

	if got.Workdir != "" {
		t.Fatalf("Workdir = %q, want empty on ambiguity", got.Workdir)
	}
	if got.Provenance != WorkdirProvenanceAmbiguousHash {
		t.Fatalf("Provenance = %q, want %q", got.Provenance, WorkdirProvenanceAmbiguousHash)
	}
	if got.MatchCount != 2 {
		t.Fatalf("MatchCount = %d, want 2", got.MatchCount)
	}
	assertWarningMentions(t, got.Warning, "88/fa3254")
	assertWarningMentions(t, got.Warning, "2")
}

func TestResolveCachedWorkdirFromHash_InvalidInputsAreUnresolvedWithWarning(t *testing.T) {
	runWorkDir := t.TempDir()
	cases := []struct {
		name       string
		runWorkDir string
		hash       string
		wantInWarn string
	}{
		{name: "blank run workdir", runWorkDir: " ", hash: "88/fa3254", wantInWarn: "workDir"},
		{name: "blank hash", runWorkDir: runWorkDir, hash: " ", wantInWarn: "hash"},
		{name: "hash without slash", runWorkDir: runWorkDir, hash: "88fa3254", wantInWarn: "hash"},
		{name: "hash without basename prefix", runWorkDir: runWorkDir, hash: "88/", wantInWarn: "hash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCachedWorkdirFromHash(tc.runWorkDir, tc.hash)

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

func assertWarningMentions(t *testing.T, warning, wantSubstring string) {
	t.Helper()
	if warning == "" {
		t.Fatalf("Warning is empty, want mention of %q", wantSubstring)
	}
	if !strings.Contains(warning, wantSubstring) {
		t.Fatalf("Warning = %q, want mention of %q", warning, wantSubstring)
	}
}
