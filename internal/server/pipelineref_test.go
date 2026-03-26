package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestPipelineRef_EmptyScriptFile(t *testing.T) {
	run := &state.Run{
		ScriptFile:  "",
		ProjectName: "nf-core/rnaseq",
	}
	got := pipelineRef(run)
	if got != "nf-core/rnaseq" {
		t.Fatalf("got %q, want %q", got, "nf-core/rnaseq")
	}
}

func TestPipelineRef_RemotePipeline(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home directory: %v", err)
	}
	run := &state.Run{
		ScriptFile:  filepath.Join(home, ".nextflow", "assets", "nf-core", "rnaseq", "main.nf"),
		ProjectName: "nf-core/rnaseq",
	}
	got := pipelineRef(run)
	if got != "nf-core/rnaseq" {
		t.Fatalf("got %q, want %q", got, "nf-core/rnaseq")
	}
}

func TestPipelineRef_LocalPipeline(t *testing.T) {
	run := &state.Run{
		ScriptFile:  "/some/path/main.nf",
		ProjectName: "nf-core/rnaseq",
	}
	got := pipelineRef(run)
	if got != "/some/path" {
		t.Fatalf("got %q, want %q", got, "/some/path")
	}
}

func TestPipelineRef_RelativeScriptFile(t *testing.T) {
	run := &state.Run{
		ScriptFile:  "pipelines/main.nf",
		ProjectName: "nf-core/rnaseq",
	}
	got := pipelineRef(run)
	if got != "nf-core/rnaseq" {
		t.Fatalf("got %q, want %q", got, "nf-core/rnaseq")
	}
}
