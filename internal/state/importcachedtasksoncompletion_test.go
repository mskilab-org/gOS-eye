package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleEvent_CompletedImportsCachedRowsFromCurrentTrace(t *testing.T) {
	dir := t.TempDir()
	tracePath := writeTraceImportTestFile(t, dir, "trace-current.txt", strings.Join([]string{
		"task_id\tstatus\thash\tname\tprocess\ttag\tworkdir",
		"2\tCACHED\tbb/22\tALIGN (sample)\tALIGN\tsample\t/work/bb/22",
		"3\tCOMPLETED\tcc/33\tALIGN (other)\tALIGN\tother\t/work/cc/33",
	}, "\n")+"\n")

	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "current_run",
		RunID:   "session-1",
		Event:   "started",
		UTCTime: "2024-01-15T10:30:00Z",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			CommandLine: "nextflow run main.nf -with-trace trace-current.txt",
			LaunchDir:   dir,
		}},
	})
	s.HandleEvent(WebhookEvent{
		RunName: "current_run",
		RunID:   "session-1",
		Event:   "process_completed",
		UTCTime: "2024-01-15T10:31:00Z",
		Trace: &Trace{
			TaskID:  1,
			Hash:    "aa/11",
			Name:    "LIVE (1)",
			Process: "LIVE",
			Status:  TaskStatusCompleted,
		},
	})
	s.HandleEvent(WebhookEvent{
		RunName: "current_run",
		RunID:   "session-1",
		Event:   "completed",
		UTCTime: "2024-01-15T10:32:00Z",
	})

	run := s.Runs["session-1"]
	if run == nil {
		t.Fatal("expected monitor run session-1 to exist")
	}
	if run.Status != "completed" {
		t.Fatalf("Status = %q, want completed", run.Status)
	}
	if run.TraceImport.RequestedPath != "trace-current.txt" {
		t.Fatalf("RequestedPath = %q, want trace-current.txt", run.TraceImport.RequestedPath)
	}
	if run.TraceImport.ResolvedPath != tracePath {
		t.Fatalf("ResolvedPath = %q, want %q", run.TraceImport.ResolvedPath, tracePath)
	}
	if run.TraceImport.Warning != "" {
		t.Fatalf("Warning = %q, want empty", run.TraceImport.Warning)
	}
	if !run.TraceImport.Imported {
		t.Fatal("Imported = false, want true")
	}
	if run.TraceImport.CachedTasks != 1 {
		t.Fatalf("CachedTasks = %d, want 1", run.TraceImport.CachedTasks)
	}

	if got := run.Tasks[1]; got == nil || got.Status != TaskStatusCompleted {
		t.Fatalf("live task 1 = %#v, want completed live task preserved", got)
	}
	cached := run.Tasks[2]
	if cached == nil {
		t.Fatal("expected cached task 2 imported from trace")
	}
	if cached.Status != TaskStatusCached || cached.Hash != "bb/22" || cached.Process != "ALIGN" || cached.Tag != "sample" || cached.Workdir != "/work/bb/22" {
		t.Fatalf("cached task 2 = %#v, want fields imported from trace", cached)
	}
	if _, ok := run.Tasks[3]; ok {
		t.Fatal("non-CACHED trace row task 3 should not be imported")
	}
}

func TestImportCachedTasksOnCompletion_DefaultNextflowTraceDoesNotCreateBlankProcess(t *testing.T) {
	dir := t.TempDir()
	writeTraceImportTestFile(t, dir, "trace-default.txt", strings.Join([]string{
		"task_id\thash\tnative_id\tname\tstatus\texit\tsubmit\tduration\trealtime\t%cpu\tpeak_rss\tpeak_vmem\trchar\twchar",
		"7\tab/cdef12\t12345\tBWA_MEM (PATIENT_03:SAMPLE_T03)\tCACHED\t0\t2026-04-27 12:50:17.250\t1m 2s\t59.5s\t87.5%\t10 MB\t20 MB\t1 KB\t2 KB",
		"8\tcd/ef3456\t12346\tSAGE\tCACHED\t0\t2026-04-27 12:50:21.000\t0ms\t0ms\t-\t-\t-\t-\t-",
	}, "\n")+"\n")

	run := &Run{
		CommandLine: "nextflow run main.nf -with-trace trace-default.txt",
		LaunchDir:   dir,
		Tasks:       make(map[int]*Task),
	}

	s := NewStore()
	if err := s.importCachedTasksOnCompletion(run); err != nil {
		t.Fatalf("importCachedTasksOnCompletion returned error: %v", err)
	}

	if !run.TraceImport.Imported {
		t.Fatal("Imported = false, want true")
	}
	if run.TraceImport.CachedTasks != 2 {
		t.Fatalf("CachedTasks = %d, want 2", run.TraceImport.CachedTasks)
	}

	bwa := run.Tasks[7]
	if bwa == nil {
		t.Fatal("expected cached task 7 imported from trace")
	}
	if bwa.Process != "BWA_MEM" || bwa.Tag != "PATIENT_03:SAMPLE_T03" {
		t.Fatalf("task 7 identity = process %q tag %q, want inferred BWA_MEM/PATIENT_03:SAMPLE_T03", bwa.Process, bwa.Tag)
	}
	if bwa.Process == "" {
		t.Fatal("task 7 Process is empty; UI grouping would create a blank accordion")
	}

	sage := run.Tasks[8]
	if sage == nil {
		t.Fatal("expected cached task 8 imported from trace")
	}
	if sage.Process != "SAGE" || sage.Tag != "" {
		t.Fatalf("task 8 identity = process %q tag %q, want inferred SAGE with empty tag", sage.Process, sage.Tag)
	}
}

func TestImportCachedTasksOnCompletion_MissingTraceCapturesWarningAndClearsStaleState(t *testing.T) {
	dir := t.TempDir()
	run := &Run{
		CommandLine: "nextflow run main.nf -with-trace missing-trace.txt",
		LaunchDir:   dir,
		TraceImport: TraceImportState{
			RequestedPath: "stale-requested.txt",
			ResolvedPath:  "/stale/resolved.txt",
			Warning:       "stale warning",
			Imported:      true,
			CachedTasks:   7,
		},
		Tasks: map[int]*Task{1: {TaskID: 1, Status: TaskStatusCompleted}},
	}

	s := NewStore()
	if err := s.importCachedTasksOnCompletion(run); err != nil {
		t.Fatalf("importCachedTasksOnCompletion returned error: %v", err)
	}

	if run.TraceImport.RequestedPath != "missing-trace.txt" {
		t.Fatalf("RequestedPath = %q, want missing-trace.txt", run.TraceImport.RequestedPath)
	}
	if run.TraceImport.ResolvedPath != "" {
		t.Fatalf("ResolvedPath = %q, want empty", run.TraceImport.ResolvedPath)
	}
	if !strings.Contains(run.TraceImport.Warning, "missing-trace.txt") {
		t.Fatalf("Warning = %q, want mention of missing trace path", run.TraceImport.Warning)
	}
	if run.TraceImport.Imported {
		t.Fatal("Imported = true, want false when no trace file was resolved")
	}
	if run.TraceImport.CachedTasks != 0 {
		t.Fatalf("CachedTasks = %d, want 0 when no trace file was resolved", run.TraceImport.CachedTasks)
	}
	if got := run.Tasks[1]; got == nil || got.Status != TaskStatusCompleted {
		t.Fatalf("existing task = %#v, want preserved completed task", got)
	}
}

func TestImportCachedTasksOnCompletion_ParseErrorCapturedAsWarning(t *testing.T) {
	dir := t.TempDir()
	tracePath := writeTraceImportTestFile(t, dir, "bad-trace.txt", "task_id\tstatus\nnot-an-int\tCACHED\n")
	run := &Run{
		CommandLine: "nextflow run main.nf -with-trace bad-trace.txt",
		LaunchDir:   dir,
		Tasks:       map[int]*Task{1: {TaskID: 1, Status: TaskStatusCompleted}},
	}

	s := NewStore()
	if err := s.importCachedTasksOnCompletion(run); err != nil {
		t.Fatalf("importCachedTasksOnCompletion returned hard error for parse warning: %v", err)
	}

	if run.TraceImport.RequestedPath != "bad-trace.txt" {
		t.Fatalf("RequestedPath = %q, want bad-trace.txt", run.TraceImport.RequestedPath)
	}
	if run.TraceImport.ResolvedPath != tracePath {
		t.Fatalf("ResolvedPath = %q, want %q", run.TraceImport.ResolvedPath, tracePath)
	}
	if !strings.Contains(run.TraceImport.Warning, "parse trace") || !strings.Contains(run.TraceImport.Warning, "task_id") {
		t.Fatalf("Warning = %q, want parse warning mentioning task_id", run.TraceImport.Warning)
	}
	if run.TraceImport.Imported {
		t.Fatal("Imported = true, want false when trace parsing fails")
	}
	if run.TraceImport.CachedTasks != 0 {
		t.Fatalf("CachedTasks = %d, want 0 when trace parsing fails", run.TraceImport.CachedTasks)
	}
	if got := run.Tasks[1]; got == nil || got.Status != TaskStatusCompleted {
		t.Fatalf("existing task = %#v, want preserved completed task", got)
	}
}

func TestImportCachedTasksOnCompletion_UsesOnlyCurrentRunTrace(t *testing.T) {
	dir := t.TempDir()
	writeTraceImportTestFile(t, dir, "trace-current.txt", "task_id\tstatus\thash\tname\tprocess\n2\tCACHED\tbb/22\tCURRENT (cached)\tCURRENT\n")

	s := NewStore()
	s.Runs["previous"] = &Run{
		ID: "previous",
		Tasks: map[int]*Task{
			42: {TaskID: 42, Status: TaskStatusCompleted, Name: "PREVIOUS (completed)", Process: "PREVIOUS"},
		},
	}
	current := &Run{
		ID:          "current",
		CommandLine: "nextflow run main.nf -with-trace trace-current.txt",
		LaunchDir:   dir,
		Tasks:       make(map[int]*Task),
	}

	if err := s.importCachedTasksOnCompletion(current); err != nil {
		t.Fatalf("importCachedTasksOnCompletion returned error: %v", err)
	}

	if _, ok := current.Tasks[42]; ok {
		t.Fatal("current run imported task 42 from previous run; current trace should be authoritative")
	}
	if got := current.Tasks[2]; got == nil || got.Status != TaskStatusCached || got.Process != "CURRENT" {
		t.Fatalf("current task 2 = %#v, want cached task from current trace", got)
	}
	if current.TraceImport.CachedTasks != 1 {
		t.Fatalf("CachedTasks = %d, want 1 from current trace", current.TraceImport.CachedTasks)
	}
	if previousTask := s.Runs["previous"].Tasks[42]; previousTask == nil || previousTask.Process != "PREVIOUS" {
		t.Fatalf("previous run task mutated: %#v", previousTask)
	}
}

func writeTraceImportTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
