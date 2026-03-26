package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLocalPipeline_EmptyString(t *testing.T) {
	got := isLocalPipeline("")
	if got != false {
		t.Fatalf("got %v, want false", got)
	}
}

func TestIsLocalPipeline_RelativePath(t *testing.T) {
	got := isLocalPipeline("main.nf")
	if got != false {
		t.Fatalf("got %v, want false", got)
	}
}

func TestIsLocalPipeline_RelativePathWithDir(t *testing.T) {
	got := isLocalPipeline("pipelines/rnaseq/main.nf")
	if got != false {
		t.Fatalf("got %v, want false", got)
	}
}

func TestIsLocalPipeline_AbsoluteLocalPath(t *testing.T) {
	got := isLocalPipeline("/home/user/pipelines/rnaseq/main.nf")
	if got != true {
		t.Fatalf("got %v, want true", got)
	}
}

func TestIsLocalPipeline_AbsoluteLocalPathRoot(t *testing.T) {
	got := isLocalPipeline("/tmp/main.nf")
	if got != true {
		t.Fatalf("got %v, want true", got)
	}
}

func TestIsLocalPipeline_UnderNextflowAssets(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home directory: %v", err)
	}
	scriptFile := filepath.Join(home, ".nextflow", "assets", "nf-core", "rnaseq", "main.nf")
	got := isLocalPipeline(scriptFile)
	if got != false {
		t.Fatalf("got %v, want false for path under ~/.nextflow/assets/", got)
	}
}

func TestIsLocalPipeline_ExactAssetsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home directory: %v", err)
	}
	// The assets directory itself (with trailing separator) should be false
	scriptFile := filepath.Join(home, ".nextflow", "assets") + string(filepath.Separator)
	got := isLocalPipeline(scriptFile)
	if got != false {
		t.Fatalf("got %v, want false for exact assets dir path", got)
	}
}

func TestIsLocalPipeline_NextflowButNotAssets(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home directory: %v", err)
	}
	// Under ~/.nextflow/ but NOT under assets/ — this is local
	scriptFile := filepath.Join(home, ".nextflow", "plugins", "main.nf")
	got := isLocalPipeline(scriptFile)
	if got != true {
		t.Fatalf("got %v, want true for path under ~/.nextflow/ but not assets/", got)
	}
}
