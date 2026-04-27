package state

import (
	"reflect"
	"testing"
)

func TestLiveWebhookTaskFromTrace_NilTraceReturnsNil(t *testing.T) {
	if got := liveWebhookTaskFromTrace(nil); got != nil {
		t.Fatalf("liveWebhookTaskFromTrace(nil) = %#v, want nil", got)
	}
}

func TestLiveWebhookTaskFromTrace_CopiesLiveTraceFieldsAndStampsLiveProvenance(t *testing.T) {
	tr := &Trace{
		TaskID:     42,
		Hash:       "ab/cdef01",
		Name:       "ALIGN (sample-1)",
		Process:    "ALIGN",
		Tag:        "sample-1",
		Status:     TaskStatusFailed,
		Submit:     1000,
		Start:      2000,
		Complete:   3000,
		Duration:   4000,
		Realtime:   5000,
		CPUPercent: 87.5,
		RSS:        123456,
		PeakRSS:    789012,
		Exit:       137,
		Workdir:    "/work/ab/cdef01",
	}

	want := &Task{
		TaskID:            42,
		Hash:              "ab/cdef01",
		Name:              "ALIGN (sample-1)",
		Process:           "ALIGN",
		Tag:               "sample-1",
		Status:            TaskStatusFailed,
		Source:            TaskSourceLiveWebhook,
		Submit:            1000,
		Start:             2000,
		Complete:          3000,
		Duration:          4000,
		Realtime:          5000,
		CPUPercent:        87.5,
		RSS:               123456,
		PeakRSS:           789012,
		Exit:              137,
		Workdir:           "/work/ab/cdef01",
		WorkdirProvenance: WorkdirProvenanceLiveWebhook,
	}

	if got := liveWebhookTaskFromTrace(tr); !reflect.DeepEqual(got, want) {
		t.Fatalf("task = %#v, want %#v", got, want)
	}
}

func TestLiveWebhookTaskFromTrace_EmptyWorkdirLeavesProvenanceAndWarningEmpty(t *testing.T) {
	got := liveWebhookTaskFromTrace(&Trace{
		TaskID: 7,
		Status: TaskStatusSubmitted,
	})
	if got == nil {
		t.Fatal("liveWebhookTaskFromTrace returned nil, want task")
	}
	if got.Source != TaskSourceLiveWebhook {
		t.Fatalf("Source = %q, want %q", got.Source, TaskSourceLiveWebhook)
	}
	if got.Workdir != "" {
		t.Fatalf("Workdir = %q, want empty", got.Workdir)
	}
	if got.WorkdirProvenance != WorkdirProvenanceUnknown {
		t.Fatalf("WorkdirProvenance = %q, want unknown/empty", got.WorkdirProvenance)
	}
	if got.WorkdirWarning != "" {
		t.Fatalf("WorkdirWarning = %q, want empty", got.WorkdirWarning)
	}
}
