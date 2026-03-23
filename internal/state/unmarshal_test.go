package state

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalJSON_HappyPath(t *testing.T) {
	// Normal trace JSON with all fields including %cpu
	data := []byte(`{
		"task_id": 1,
		"status": "COMPLETED",
		"hash": "ab/cdef01",
		"name": "sayHello (1)",
		"process": "sayHello",
		"tag": "",
		"submit": 1705312201000,
		"start": 1705312202000,
		"complete": 1705312203000,
		"duration": 2000,
		"realtime": 1500,
		"%cpu": 42.5,
		"rss": 1024,
		"vmem": 2048,
		"peak_rss": 1536,
		"cpus": 2,
		"memory": 1073741824,
		"exit": 0,
		"workdir": "/home/user/work/ab/cdef01",
		"script": "echo hello"
	}`)

	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tr.TaskID != 1 {
		t.Errorf("TaskID = %d, want 1", tr.TaskID)
	}
	if tr.Status != "COMPLETED" {
		t.Errorf("Status = %q, want COMPLETED", tr.Status)
	}
	if tr.Hash != "ab/cdef01" {
		t.Errorf("Hash = %q, want ab/cdef01", tr.Hash)
	}
	if tr.Name != "sayHello (1)" {
		t.Errorf("Name = %q, want sayHello (1)", tr.Name)
	}
	if tr.Process != "sayHello" {
		t.Errorf("Process = %q, want sayHello", tr.Process)
	}
	if tr.CPUPercent != 42.5 {
		t.Errorf("CPUPercent = %f, want 42.5", tr.CPUPercent)
	}
	if tr.RSS != 1024 {
		t.Errorf("RSS = %d, want 1024", tr.RSS)
	}
	if tr.CPUs != 2 {
		t.Errorf("CPUs = %d, want 2", tr.CPUs)
	}
	if tr.Exit != 0 {
		t.Errorf("Exit = %d, want 0", tr.Exit)
	}
	if tr.Workdir != "/home/user/work/ab/cdef01" {
		t.Errorf("Workdir = %q, want /home/user/work/ab/cdef01", tr.Workdir)
	}
}

func TestUnmarshalJSON_CPUAbsent(t *testing.T) {
	// %cpu key missing entirely → CPUPercent should default to 0
	data := []byte(`{
		"task_id": 2,
		"status": "SUBMITTED",
		"hash": "cd/ef0123",
		"name": "sayHello (2)",
		"process": "sayHello"
	}`)

	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tr.CPUPercent != 0 {
		t.Errorf("CPUPercent = %f, want 0 (absent)", tr.CPUPercent)
	}
	if tr.TaskID != 2 {
		t.Errorf("TaskID = %d, want 2", tr.TaskID)
	}
}

func TestUnmarshalJSON_CPUNull(t *testing.T) {
	// %cpu is explicitly null → CPUPercent should default to 0
	data := []byte(`{
		"task_id": 3,
		"status": "RUNNING",
		"%cpu": null
	}`)

	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tr.CPUPercent != 0 {
		t.Errorf("CPUPercent = %f, want 0 (null)", tr.CPUPercent)
	}
}

func TestUnmarshalJSON_CPUZero(t *testing.T) {
	// %cpu is explicitly 0 → CPUPercent should be 0
	data := []byte(`{
		"task_id": 4,
		"status": "SUBMITTED",
		"%cpu": 0
	}`)

	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tr.CPUPercent != 0 {
		t.Errorf("CPUPercent = %f, want 0", tr.CPUPercent)
	}
}

func TestUnmarshalJSON_CPUInteger(t *testing.T) {
	// %cpu as integer (JSON number without decimal) → should work
	data := []byte(`{
		"task_id": 5,
		"status": "COMPLETED",
		"%cpu": 100
	}`)

	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tr.CPUPercent != 100 {
		t.Errorf("CPUPercent = %f, want 100", tr.CPUPercent)
	}
}

func TestUnmarshalJSON_InvalidJSON(t *testing.T) {
	data := []byte(`not valid json`)

	var tr Trace
	err := json.Unmarshal(data, &tr)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestUnmarshalJSON_NullTag(t *testing.T) {
	// tag: null is common in Nextflow webhooks; should unmarshal to ""
	data := []byte(`{
		"task_id": 6,
		"status": "SUBMITTED",
		"tag": null,
		"%cpu": 3.14
	}`)

	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tr.Tag != "" {
		t.Errorf("Tag = %q, want empty string", tr.Tag)
	}
	if tr.CPUPercent != 3.14 {
		t.Errorf("CPUPercent = %f, want 3.14", tr.CPUPercent)
	}
}

func TestUnmarshalJSON_ViaWebhookEvent(t *testing.T) {
	// Ensure UnmarshalJSON works when Trace is nested inside WebhookEvent
	data := []byte(`{
		"runName": "happy_darwin",
		"runId": "abc-123",
		"event": "process_completed",
		"utcTime": "2024-01-15T10:30:05Z",
		"trace": {
			"task_id": 1,
			"status": "COMPLETED",
			"hash": "ab/cdef01",
			"name": "sayHello (1)",
			"process": "sayHello",
			"%cpu": 87.3,
			"rss": 4096,
			"exit": 0
		}
	}`)

	var evt WebhookEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evt.Trace == nil {
		t.Fatal("Trace is nil, want non-nil")
	}
	if evt.Trace.CPUPercent != 87.3 {
		t.Errorf("Trace.CPUPercent = %f, want 87.3", evt.Trace.CPUPercent)
	}
	if evt.Trace.TaskID != 1 {
		t.Errorf("Trace.TaskID = %d, want 1", evt.Trace.TaskID)
	}
	if evt.Trace.Status != "COMPLETED" {
		t.Errorf("Trace.Status = %q, want COMPLETED", evt.Trace.Status)
	}
}
