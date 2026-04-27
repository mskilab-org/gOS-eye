package state

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/traceimport"
)

func TestMergeCachedTraceRows_ImportsOnlyCachedRowsAndMapsFields(t *testing.T) {
	s := NewStore()
	run := &Run{}
	rows := []traceimport.Row{
		{
			TaskID:     7,
			Hash:       "ab/cdef12",
			Name:       "PROC (sample-tag)",
			Status:     "CACHED",
			Exit:       0,
			Submit:     1000,
			Start:      2000,
			Complete:   3000,
			Duration:   4000,
			Realtime:   5000,
			CPUPercent: 87.5,
			PeakRSS:    10 * 1024 * 1024,
			Process:    "PROC",
			Tag:        "sample-tag",
			Workdir:    "/work/ab/cdef12",
		},
		{
			TaskID:   8,
			Hash:     "cd/ef3456",
			Name:     "PROC (2)",
			Status:   "COMPLETED",
			Process:  "PROC",
			Tag:      "not-imported",
			Workdir:  "/work/cd/ef3456",
			Duration: 9999,
		},
	}

	s.mergeCachedTraceRows(run, rows)

	if run.Tasks == nil {
		t.Fatal("Tasks map should be initialized")
	}
	if len(run.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(run.Tasks))
	}
	if _, ok := run.Tasks[8]; ok {
		t.Fatal("non-CACHED row should not be imported")
	}

	want := &Task{
		TaskID:            7,
		Hash:              "ab/cdef12",
		Name:              "PROC (sample-tag)",
		Process:           "PROC",
		Tag:               "sample-tag",
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
		Workdir:           "/work/ab/cdef12",
		WorkdirProvenance: WorkdirProvenanceTraceWorkdir,
	}
	if !reflect.DeepEqual(run.Tasks[7], want) {
		t.Fatalf("task = %#v, want %#v", run.Tasks[7], want)
	}
	if run.TraceImport.CachedTasks != 1 {
		t.Fatalf("CachedTasks = %d, want 1", run.TraceImport.CachedTasks)
	}
	if run.TraceImport.CachedWorkdirsExplicit != 1 {
		t.Fatalf("CachedWorkdirsExplicit = %d, want 1", run.TraceImport.CachedWorkdirsExplicit)
	}
	if run.TraceImport.CachedWorkdirsHashResolved != 0 || run.TraceImport.CachedWorkdirsUnresolved != 0 || run.TraceImport.CachedWorkdirsAmbiguous != 0 {
		t.Fatalf("unexpected cached workdir aggregates: %#v", run.TraceImport)
	}
}

func TestMergeCachedTraceRows_DoesNotOverwriteExistingLiveTask(t *testing.T) {
	s := NewStore()
	live := &Task{
		TaskID:     7,
		Hash:       "live/hash",
		Name:       "PROC (live)",
		Process:    "PROC",
		Tag:        "live-tag",
		Status:     TaskStatusCompleted,
		Submit:     11,
		Start:      22,
		Complete:   33,
		Duration:   44,
		Realtime:   55,
		CPUPercent: 66.5,
		RSS:        77,
		PeakRSS:    88,
		Exit:       99,
		Workdir:    "/live/workdir",
	}
	wantLive := *live
	run := &Run{Tasks: map[int]*Task{7: live}}
	rows := []traceimport.Row{
		{
			TaskID:     7,
			Hash:       "cached/hash",
			Name:       "PROC (cached)",
			Status:     "CACHED",
			Exit:       0,
			Submit:     1000,
			Start:      2000,
			Complete:   3000,
			Duration:   4000,
			Realtime:   5000,
			CPUPercent: 87.5,
			PeakRSS:    10 * 1024 * 1024,
			Process:    "PROC",
			Tag:        "cached-tag",
			Workdir:    "/cached/workdir",
		},
		{
			TaskID:  8,
			Hash:    "new/hash",
			Name:    "NEW (cached)",
			Status:  "CACHED",
			Process: "NEW",
			Tag:     "sample-8",
		},
	}

	s.mergeCachedTraceRows(run, rows)

	if !reflect.DeepEqual(*run.Tasks[7], wantLive) {
		t.Fatalf("live task was overwritten: got %#v, want %#v", *run.Tasks[7], wantLive)
	}
	if got := run.Tasks[8]; got == nil || got.Status != TaskStatusCached {
		t.Fatalf("task 8 = %#v, want imported cached task", got)
	}
	if run.TraceImport.CachedTasks != 1 {
		t.Fatalf("CachedTasks = %d, want 1 imported task", run.TraceImport.CachedTasks)
	}
	if run.TraceImport.CachedWorkdirsExplicit != 0 {
		t.Fatalf("CachedWorkdirsExplicit = %d, want 0 because skipped live task should not be counted", run.TraceImport.CachedWorkdirsExplicit)
	}
	if run.TraceImport.CachedWorkdirsUnresolved != 1 {
		t.Fatalf("CachedWorkdirsUnresolved = %d, want 1 for imported task 8", run.TraceImport.CachedWorkdirsUnresolved)
	}
}

func TestMergeCachedTraceRows_UpdatesCachedOrEmptyExistingTask(t *testing.T) {
	cases := []struct {
		name     string
		existing *Task
	}{
		{name: "cached", existing: &Task{TaskID: 3, Status: TaskStatusCached, Name: "old"}},
		{name: "empty", existing: &Task{TaskID: 3}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore()
			run := &Run{
				Tasks: map[int]*Task{3: tc.existing},
				TraceImport: TraceImportState{
					CachedTasks: 99,
				},
			}
			rows := []traceimport.Row{{
				TaskID:     3,
				Hash:       "xy/123456",
				Name:       "CACHE (sample)",
				Status:     "CACHED",
				Exit:       0,
				Submit:     10,
				Start:      11,
				Complete:   12,
				Duration:   13,
				Realtime:   14,
				CPUPercent: 15.5,
				PeakRSS:    16,
				Process:    "CACHE",
				Tag:        "sample",
				Workdir:    "/work/xy/123456",
			}}

			s.mergeCachedTraceRows(run, rows)

			want := &Task{
				TaskID:            3,
				Hash:              "xy/123456",
				Name:              "CACHE (sample)",
				Process:           "CACHE",
				Tag:               "sample",
				Status:            TaskStatusCached,
				Source:            TaskSourceCachedTrace,
				Submit:            10,
				Start:             11,
				Complete:          12,
				Duration:          13,
				Realtime:          14,
				CPUPercent:        15.5,
				PeakRSS:           16,
				Exit:              0,
				Workdir:           "/work/xy/123456",
				WorkdirProvenance: WorkdirProvenanceTraceWorkdir,
			}
			if !reflect.DeepEqual(run.Tasks[3], want) {
				t.Fatalf("task = %#v, want %#v", run.Tasks[3], want)
			}
			if run.TraceImport.CachedTasks != 1 {
				t.Fatalf("CachedTasks = %d, want 1", run.TraceImport.CachedTasks)
			}
		})
	}
}

func TestMergeCachedTraceRows_RecordsWorkdirResolutionAggregatesForImportedCachedRows(t *testing.T) {
	runWorkDir := t.TempDir()
	resolvedWorkdir := filepath.Join(runWorkDir, "aa", "123456789")
	if err := os.MkdirAll(resolvedWorkdir, 0o755); err != nil {
		t.Fatalf("MkdirAll resolved workdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runWorkDir, "bb"), 0o755); err != nil {
		t.Fatalf("MkdirAll unresolved parent workdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runWorkDir, "cc", "456aaa"), 0o755); err != nil {
		t.Fatalf("MkdirAll ambiguous workdir A: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runWorkDir, "cc", "456bbb"), 0o755); err != nil {
		t.Fatalf("MkdirAll ambiguous workdir B: %v", err)
	}

	run := &Run{WorkDir: runWorkDir}
	rows := []traceimport.Row{
		{TaskID: 1, Status: "CACHED", Hash: "zz/explicit", Name: "EXPLICIT", Process: "EXPLICIT", Workdir: "/trace/workdir"},
		{TaskID: 2, Status: "CACHED", Hash: "aa/123456", Name: "HASH", Process: "HASH"},
		{TaskID: 3, Status: "CACHED", Hash: "bb/missing", Name: "MISSING", Process: "MISSING"},
		{TaskID: 4, Status: "CACHED", Hash: "cc/456", Name: "AMBIG", Process: "AMBIG"},
		{TaskID: 5, Status: "COMPLETED", Hash: "aa/123456", Name: "LIVE", Process: "LIVE"},
	}

	NewStore().mergeCachedTraceRows(run, rows)

	if run.TraceImport.CachedTasks != 4 {
		t.Fatalf("CachedTasks = %d, want 4 imported cached rows", run.TraceImport.CachedTasks)
	}
	if run.TraceImport.CachedWorkdirsExplicit != 1 {
		t.Fatalf("CachedWorkdirsExplicit = %d, want 1", run.TraceImport.CachedWorkdirsExplicit)
	}
	if run.TraceImport.CachedWorkdirsHashResolved != 1 {
		t.Fatalf("CachedWorkdirsHashResolved = %d, want 1", run.TraceImport.CachedWorkdirsHashResolved)
	}
	if run.TraceImport.CachedWorkdirsUnresolved != 1 {
		t.Fatalf("CachedWorkdirsUnresolved = %d, want 1", run.TraceImport.CachedWorkdirsUnresolved)
	}
	if run.TraceImport.CachedWorkdirsAmbiguous != 1 {
		t.Fatalf("CachedWorkdirsAmbiguous = %d, want 1", run.TraceImport.CachedWorkdirsAmbiguous)
	}
	if len(run.TraceImport.CachedWorkdirResolutionWarnings) != 2 {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want unresolved and ambiguous warnings", run.TraceImport.CachedWorkdirResolutionWarnings)
	}
	if got := run.Tasks[1]; got == nil || got.WorkdirProvenance != WorkdirProvenanceTraceWorkdir || got.Workdir != "/trace/workdir" {
		t.Fatalf("explicit task = %#v, want trace workdir provenance", got)
	}
	if got := run.Tasks[2]; got == nil || got.WorkdirProvenance != WorkdirProvenanceHashResolved || got.Workdir != resolvedWorkdir {
		t.Fatalf("hash-resolved task = %#v, want workdir %q", got, resolvedWorkdir)
	}
	if got := run.Tasks[3]; got == nil || got.WorkdirProvenance != WorkdirProvenanceUnresolved || !strings.Contains(got.WorkdirWarning, "no directory") {
		t.Fatalf("unresolved task = %#v, want unresolved warning", got)
	}
	if got := run.Tasks[4]; got == nil || got.WorkdirProvenance != WorkdirProvenanceAmbiguousHash || !strings.Contains(got.WorkdirWarning, "ambiguous") {
		t.Fatalf("ambiguous task = %#v, want ambiguous warning", got)
	}
	if _, ok := run.Tasks[5]; ok {
		t.Fatal("non-CACHED task 5 should not be imported")
	}
}

func TestMergeCachedTraceRows_ResetsCachedWorkdirAggregatesBetweenImportPasses(t *testing.T) {
	runWorkDir := t.TempDir()
	resolvedWorkdir := filepath.Join(runWorkDir, "aa", "123456789")
	if err := os.MkdirAll(resolvedWorkdir, 0o755); err != nil {
		t.Fatalf("MkdirAll resolved workdir: %v", err)
	}
	run := &Run{
		WorkDir: runWorkDir,
		TraceImport: TraceImportState{
			RequestedPath:                   "trace.txt",
			ResolvedPath:                    "/resolved/trace.txt",
			Warning:                         "path warning",
			Imported:                        true,
			CachedTasks:                     99,
			CachedWorkdirsExplicit:          98,
			CachedWorkdirsHashResolved:      97,
			CachedWorkdirsUnresolved:        96,
			CachedWorkdirsAmbiguous:         95,
			CachedWorkdirResolutionWarnings: []string{"stale cached workdir warning"},
		},
	}

	NewStore().mergeCachedTraceRows(run, []traceimport.Row{{
		TaskID:  1,
		Status:  "CACHED",
		Hash:    "zz/explicit",
		Name:    "EXPLICIT",
		Process: "EXPLICIT",
		Workdir: "/trace/workdir",
	}})
	NewStore().mergeCachedTraceRows(run, []traceimport.Row{{
		TaskID:  2,
		Status:  "CACHED",
		Hash:    "aa/123456",
		Name:    "HASH",
		Process: "HASH",
	}})

	if run.TraceImport.RequestedPath != "trace.txt" || run.TraceImport.ResolvedPath != "/resolved/trace.txt" || run.TraceImport.Warning != "path warning" || !run.TraceImport.Imported {
		t.Fatalf("non-aggregate TraceImport fields were not preserved: %#v", run.TraceImport)
	}
	if run.TraceImport.CachedTasks != 1 {
		t.Fatalf("CachedTasks = %d, want 1 from latest pass only", run.TraceImport.CachedTasks)
	}
	if run.TraceImport.CachedWorkdirsExplicit != 0 {
		t.Fatalf("CachedWorkdirsExplicit = %d, want reset to 0 on second pass", run.TraceImport.CachedWorkdirsExplicit)
	}
	if run.TraceImport.CachedWorkdirsHashResolved != 1 {
		t.Fatalf("CachedWorkdirsHashResolved = %d, want 1 from latest pass", run.TraceImport.CachedWorkdirsHashResolved)
	}
	if run.TraceImport.CachedWorkdirsUnresolved != 0 || run.TraceImport.CachedWorkdirsAmbiguous != 0 {
		t.Fatalf("stale unresolved/ambiguous counts remain: %#v", run.TraceImport)
	}
	if len(run.TraceImport.CachedWorkdirResolutionWarnings) != 0 {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want stale warnings reset", run.TraceImport.CachedWorkdirResolutionWarnings)
	}
	if got := run.Tasks[1]; got == nil || got.WorkdirProvenance != WorkdirProvenanceTraceWorkdir {
		t.Fatalf("first-pass cached task should remain in current run tasks: %#v", got)
	}
	if got := run.Tasks[2]; got == nil || got.Workdir != resolvedWorkdir || got.WorkdirProvenance != WorkdirProvenanceHashResolved {
		t.Fatalf("second-pass cached task = %#v, want hash-resolved workdir %q", got, resolvedWorkdir)
	}
}
