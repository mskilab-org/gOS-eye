package state

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/traceimport"
)

func TestCachedTraceTaskFromRow_MapsCachedFieldsAndExplicitWorkdirProvenance(t *testing.T) {
	runWorkDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runWorkDir, "88", "fa3254def789"), 0o755); err != nil {
		t.Fatalf("MkdirAll hash workdir: %v", err)
	}
	run := &Run{WorkDir: runWorkDir}
	row := traceimport.Row{
		TaskID:     7,
		Hash:       "88/fa3254",
		Name:       "BWA_MEM (PATIENT_03:SAMPLE_T03)",
		Status:     "CACHED",
		Exit:       0,
		Submit:     1000,
		Start:      2000,
		Complete:   3000,
		Duration:   4000,
		Realtime:   5000,
		CPUPercent: 87.5,
		PeakRSS:    10 * 1024 * 1024,
		Process:    "BWA_MEM",
		Tag:        "PATIENT_03:SAMPLE_T03",
		Workdir:    "/trace/explicit-workdir",
	}

	got := cachedTraceTaskFromRow(run, row)

	want := &Task{
		TaskID:            7,
		Hash:              "88/fa3254",
		Name:              "BWA_MEM (PATIENT_03:SAMPLE_T03)",
		Process:           "BWA_MEM",
		Tag:               "PATIENT_03:SAMPLE_T03",
		Status:            TaskStatusCached,
		Source:            TaskSourceCachedTrace,
		Submit:            1000,
		Start:             2000,
		Complete:          3000,
		Duration:          4000,
		Realtime:          5000,
		CPUPercent:        87.5,
		PeakRSS:           10 * 1024 * 1024,
		Exit:              0,
		Workdir:           "/trace/explicit-workdir",
		WorkdirProvenance: WorkdirProvenanceTraceWorkdir,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("task = %#v, want %#v", got, want)
	}
}

func TestCachedTraceTaskFromRow_ResolvesMissingWorkdirFromRunWorkDirAndHash(t *testing.T) {
	runWorkDir := t.TempDir()
	wantWorkdir := filepath.Join(runWorkDir, "88", "fa3254def789")
	if err := os.MkdirAll(wantWorkdir, 0o755); err != nil {
		t.Fatalf("MkdirAll wantWorkdir: %v", err)
	}

	got := cachedTraceTaskFromRow(&Run{WorkDir: runWorkDir}, traceimport.Row{
		TaskID:  8,
		Hash:    "88/fa3254",
		Name:    "SAGE",
		Process: "SAGE",
		Status:  "CACHED",
	})

	if got == nil {
		t.Fatal("task = nil, want visible cached task")
	}
	if got.Status != TaskStatusCached {
		t.Fatalf("Status = %q, want %q", got.Status, TaskStatusCached)
	}
	if got.Source != TaskSourceCachedTrace {
		t.Fatalf("Source = %q, want %q", got.Source, TaskSourceCachedTrace)
	}
	if got.Workdir != wantWorkdir {
		t.Fatalf("Workdir = %q, want hash-resolved path %q", got.Workdir, wantWorkdir)
	}
	if got.WorkdirProvenance != WorkdirProvenanceHashResolved {
		t.Fatalf("WorkdirProvenance = %q, want %q", got.WorkdirProvenance, WorkdirProvenanceHashResolved)
	}
	if got.WorkdirWarning != "" {
		t.Fatalf("WorkdirWarning = %q, want empty", got.WorkdirWarning)
	}
}

func TestCachedTraceTaskFromRow_NilRunKeepsVisibleTaskWithWorkdirWarning(t *testing.T) {
	got := cachedTraceTaskFromRow(nil, traceimport.Row{
		TaskID:  9,
		Hash:    "88/fa3254",
		Name:    "ALIGN (sample)",
		Process: "ALIGN",
		Tag:     "sample",
		Status:  "CACHED",
	})

	if got == nil {
		t.Fatal("task = nil, want visible cached task")
	}
	if got.TaskID != 9 || got.Name != "ALIGN (sample)" || got.Process != "ALIGN" || got.Tag != "sample" {
		t.Fatalf("identity fields = taskID %d name %q process %q tag %q, want row identity preserved", got.TaskID, got.Name, got.Process, got.Tag)
	}
	if got.Status != TaskStatusCached {
		t.Fatalf("Status = %q, want %q", got.Status, TaskStatusCached)
	}
	if got.Source != TaskSourceCachedTrace {
		t.Fatalf("Source = %q, want %q", got.Source, TaskSourceCachedTrace)
	}
	if got.Workdir != "" {
		t.Fatalf("Workdir = %q, want empty unresolved workdir", got.Workdir)
	}
	if got.WorkdirProvenance != WorkdirProvenanceUnresolved {
		t.Fatalf("WorkdirProvenance = %q, want %q", got.WorkdirProvenance, WorkdirProvenanceUnresolved)
	}
	if !strings.Contains(got.WorkdirWarning, "blank run workDir") {
		t.Fatalf("WorkdirWarning = %q, want mention of blank run workDir", got.WorkdirWarning)
	}
}

func TestCachedTraceTaskFromRow_DoesNotRecordAggregateTraceImportState(t *testing.T) {
	run := &Run{
		TraceImport: TraceImportState{
			CachedTasks:                     2,
			CachedWorkdirsExplicit:          3,
			CachedWorkdirsHashResolved:      4,
			CachedWorkdirsUnresolved:        5,
			CachedWorkdirsAmbiguous:         6,
			CachedWorkdirResolutionWarnings: []string{"existing warning"},
		},
	}
	wantTraceImport := TraceImportState{
		CachedTasks:                     2,
		CachedWorkdirsExplicit:          3,
		CachedWorkdirsHashResolved:      4,
		CachedWorkdirsUnresolved:        5,
		CachedWorkdirsAmbiguous:         6,
		CachedWorkdirResolutionWarnings: []string{"existing warning"},
	}

	got := cachedTraceTaskFromRow(run, traceimport.Row{
		TaskID: 1,
		Hash:   "bad-hash",
		Name:   "BROKEN (cached)",
		Status: "CACHED",
	})

	if got == nil || got.Status != TaskStatusCached {
		t.Fatalf("task = %#v, want visible cached task", got)
	}
	if !reflect.DeepEqual(run.TraceImport, wantTraceImport) {
		t.Fatalf("TraceImport mutated to %#v, want %#v", run.TraceImport, wantTraceImport)
	}
}
