package traceimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseFile_HappyPathParsesSupportedColumns(t *testing.T) {
	path := writeTraceTSV(t, strings.Join([]string{
		"status\t%cpu\ttask_id\tpeak_rss\tname\tsubmit\tstart\tcomplete\tduration\trealtime\thash\texit\tprocess\ttag\tworkdir",
		"CACHED\t87.5%\t7\t10 MB\tPROC (sample-tag)\t2026-04-27 12:50:17.250\t2026-04-27 12:50:18.000\t2026-04-27 12:51:20.500\t1m 2s\t59.5s\tab/cdef12\t0\tPROC\tsample-tag\t/work/ab/cdef12",
		"CACHED\t-\t8\t-\tPROC (2)\t2026-04-27 12:50:21.000\t-\t-\t0ms\t0ms\tcd/ef3456\t0\tPROC\t-\t-",
	}, "\n"))

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(got))
	}

	wantFirst := Row{
		TaskID:     7,
		Hash:       "ab/cdef12",
		Name:       "PROC (sample-tag)",
		Status:     "CACHED",
		Exit:       0,
		Submit:     mustTraceUnixMilli(t, "2026-04-27 12:50:17.250"),
		Start:      mustTraceUnixMilli(t, "2026-04-27 12:50:18.000"),
		Complete:   mustTraceUnixMilli(t, "2026-04-27 12:51:20.500"),
		Duration:   62000,
		Realtime:   59500,
		CPUPercent: 87.5,
		PeakRSS:    10 * 1024 * 1024,
		Process:    "PROC",
		Tag:        "sample-tag",
		Workdir:    "/work/ab/cdef12",
	}
	if got[0] != wantFirst {
		t.Fatalf("first row = %#v, want %#v", got[0], wantFirst)
	}

	wantSecond := Row{
		TaskID:   8,
		Hash:     "cd/ef3456",
		Name:     "PROC (2)",
		Status:   "CACHED",
		Exit:     0,
		Submit:   mustTraceUnixMilli(t, "2026-04-27 12:50:21.000"),
		Duration: 0,
		Realtime: 0,
		Process:  "PROC",
	}
	if got[1] != wantSecond {
		t.Fatalf("second row = %#v, want %#v", got[1], wantSecond)
	}
}

func TestParseFile_MissingOptionalColumnsDefaultsZeroValuesAndInfersIdentity(t *testing.T) {
	path := writeTraceTSV(t, "task_id\tstatus\tname\n1\tCACHED\tSAY_HELLO (1)\n")

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	want := []Row{{TaskID: 1, Status: "CACHED", Name: "SAY_HELLO (1)", Process: "SAY_HELLO", Tag: "1"}}
	if len(got) != len(want) {
		t.Fatalf("len(rows) = %d, want %d", len(got), len(want))
	}
	if got[0] != want[0] {
		t.Fatalf("row = %#v, want %#v", got[0], want[0])
	}
}

func TestParseFile_DefaultNextflowTraceInfersProcessAndTagFromName(t *testing.T) {
	path := writeTraceTSV(t, strings.Join([]string{
		"task_id\thash\tnative_id\tname\tstatus\texit\tsubmit\tduration\trealtime\t%cpu\tpeak_rss\tpeak_vmem\trchar\twchar",
		"7\tab/cdef12\t12345\tBWA_MEM (PATIENT_03:SAMPLE_T03)\tCACHED\t0\t2026-04-27 12:50:17.250\t1m 2s\t59.5s\t87.5%\t10 MB\t20 MB\t1 KB\t2 KB",
		"8\tcd/ef3456\t12346\tFRAGCOUNTER\tCACHED\t0\t2026-04-27 12:50:21.000\t0ms\t0ms\t-\t-\t-\t-\t-",
	}, "\n"))

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(got))
	}

	if got[0].Process != "BWA_MEM" {
		t.Fatalf("first row Process = %q, want BWA_MEM", got[0].Process)
	}
	if got[0].Tag != "PATIENT_03:SAMPLE_T03" {
		t.Fatalf("first row Tag = %q, want PATIENT_03:SAMPLE_T03", got[0].Tag)
	}
	if got[0].Workdir != "" {
		t.Fatalf("first row Workdir = %q, want empty when trace lacks workdir", got[0].Workdir)
	}
	if got[0].PeakRSS != 10*1024*1024 {
		t.Fatalf("first row PeakRSS = %d, want 10 MiB", got[0].PeakRSS)
	}

	if got[1].Process != "FRAGCOUNTER" {
		t.Fatalf("second row Process = %q, want FRAGCOUNTER", got[1].Process)
	}
	if got[1].Tag != "" {
		t.Fatalf("second row Tag = %q, want empty for name without parentheses", got[1].Tag)
	}
}

func TestParseFile_DoesNotOverwriteExplicitProcessOrTagColumns(t *testing.T) {
	path := writeTraceTSV(t, strings.Join([]string{
		"task_id\tstatus\tname\tprocess\ttag",
		"1\tCACHED\tBWA_MEM (from-name)\tEXPLICIT_PROCESS\texplicit-tag",
	}, "\n"))

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(got))
	}
	if got[0].Process != "EXPLICIT_PROCESS" {
		t.Fatalf("Process = %q, want explicit process column", got[0].Process)
	}
	if got[0].Tag != "explicit-tag" {
		t.Fatalf("Tag = %q, want explicit tag column", got[0].Tag)
	}
}

func TestParseFile_HeaderOnlyReturnsNoRows(t *testing.T) {
	path := writeTraceTSV(t, "task_id\tstatus\tname\n")

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(rows) = %d, want 0", len(got))
	}
}

func TestParseFile_InvalidTaskIDReturnsUsefulError(t *testing.T) {
	path := writeTraceTSV(t, "task_id\tstatus\noops\tCACHED\n")

	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task_id") {
		t.Fatalf("error = %q, want mention of task_id", err)
	}
	if !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("error = %q, want mention of row 2", err)
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Fatalf("error = %q, want mention of bad value", err)
	}
}

func TestParseFile_InvalidDurationReturnsUsefulError(t *testing.T) {
	path := writeTraceTSV(t, "task_id\tstatus\tduration\n1\tCACHED\tnot-a-duration\n")

	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "duration") {
		t.Fatalf("error = %q, want mention of duration", err)
	}
	if !strings.Contains(err.Error(), "not-a-duration") {
		t.Fatalf("error = %q, want mention of bad value", err)
	}
}

func writeTraceTSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func mustTraceUnixMilli(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000", value, time.Local)
	if err != nil {
		t.Fatalf("ParseInLocation(%q): %v", value, err)
	}
	return parsed.UnixMilli()
}
