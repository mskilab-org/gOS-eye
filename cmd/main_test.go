package main

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/db"
	"github.com/mskilab-org/nextflow-monitor/internal/server"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// TestMainWiring verifies that the Store and Server wire together correctly
// and the server accepts HTTP connections (the core logic of main minus blocking).
func TestMainWiring(t *testing.T) {
	store := state.NewStore()
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}

	srv := server.NewServer(store, nil)
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}

	// Start the server on a random available port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	// Give server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Verify the server responds to the index route (may 500 if web/index.html
	// is missing, but the route must be registered — not 404).
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("GET / returned 404, expected route to be registered")
	}
}

// TestMainWiringWithEventStore verifies that an EventStore is accepted as
// the persister for NewServer and the server still accepts connections.
func TestMainWiringWithEventStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	eventStore, err := db.OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("OpenEventStore() failed: %v", err)
	}
	defer eventStore.Close()

	store := state.NewStore()
	srv := server.NewServer(store, eventStore)
	if srv == nil {
		t.Fatal("NewServer(store, eventStore) returned nil")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("GET / returned 404, expected route to be registered")
	}
}

// TestReplayEvents_EmptyDB verifies replay with an empty database loads zero events.
func TestReplayEvents_EmptyDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")

	eventStore, err := db.OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("OpenEventStore() failed: %v", err)
	}
	defer eventStore.Close()

	store := state.NewStore()
	count := replayEvents(eventStore, store)

	if count != 0 {
		t.Errorf("replayEvents() = %d, want 0 for empty DB", count)
	}

	runs := store.GetAllRuns()
	if len(runs) != 0 {
		t.Errorf("store has %d runs after empty replay, want 0", len(runs))
	}
}

// TestReplayEvents_SingleStartedEvent verifies a single "started" event is
// replayed into the Store, creating one run with correct metadata.
func TestReplayEvents_SingleStartedEvent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "single.db")

	eventStore, err := db.OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("OpenEventStore() failed: %v", err)
	}
	defer eventStore.Close()

	evt := state.WebhookEvent{
		RunName: "happy_tesla",
		RunID:   "abc123",
		Event:   "started",
		UTCTime: "2025-01-15T10:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{ProjectName: "nf-hello"},
		},
	}
	raw, _ := json.Marshal(evt)
	if err := eventStore.Save(raw); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	store := state.NewStore()
	count := replayEvents(eventStore, store)

	if count != 1 {
		t.Errorf("replayEvents() = %d, want 1", count)
	}

	run := store.GetRun("abc123")
	if run == nil {
		t.Fatal("expected run abc123 to exist after replay")
	}
	if run.Status != "running" {
		t.Errorf("run status = %q, want %q", run.Status, "running")
	}
	if run.ProjectName != "nf-hello" {
		t.Errorf("run project = %q, want %q", run.ProjectName, "nf-hello")
	}
}

// TestReplayEvents_MultipleEvents verifies a full run lifecycle replays correctly.
func TestReplayEvents_MultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "multi.db")

	eventStore, err := db.OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("OpenEventStore() failed: %v", err)
	}
	defer eventStore.Close()

	events := []state.WebhookEvent{
		{
			RunName: "test_run", RunID: "run1", Event: "started",
			UTCTime: "2025-01-15T10:00:00Z",
			Metadata: &state.Metadata{
				Workflow: state.WorkflowInfo{ProjectName: "my-pipeline"},
			},
		},
		{
			RunName: "test_run", RunID: "run1", Event: "process_submitted",
			UTCTime: "2025-01-15T10:00:01Z",
			Trace:   &state.Trace{TaskID: 1, Name: "foo (1)", Process: "foo", Status: "SUBMITTED"},
		},
		{
			RunName: "test_run", RunID: "run1", Event: "completed",
			UTCTime: "2025-01-15T10:01:00Z",
		},
	}

	for _, evt := range events {
		raw, _ := json.Marshal(evt)
		if err := eventStore.Save(raw); err != nil {
			t.Fatalf("Save() failed: %v", err)
		}
	}

	store := state.NewStore()
	count := replayEvents(eventStore, store)

	if count != 3 {
		t.Errorf("replayEvents() = %d, want 3", count)
	}

	run := store.GetRun("run1")
	if run == nil {
		t.Fatal("expected run run1 to exist after replay")
	}
	if run.Status != "completed" {
		t.Errorf("run status = %q, want %q", run.Status, "completed")
	}
	if len(run.Tasks) != 1 {
		t.Errorf("run has %d tasks, want 1", len(run.Tasks))
	}
}

// TestReplayEvents_InvalidJSON verifies that invalid JSON blobs are skipped
// without crashing, and valid events before/after are still replayed.
func TestReplayEvents_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "invalid.db")

	eventStore, err := db.OpenEventStore(dbPath)
	if err != nil {
		t.Fatalf("OpenEventStore() failed: %v", err)
	}
	defer eventStore.Close()

	// Save a valid event, then invalid JSON, then another valid event
	evt1 := state.WebhookEvent{
		RunName: "r1", RunID: "id1", Event: "started", UTCTime: "2025-01-15T10:00:00Z",
	}
	raw1, _ := json.Marshal(evt1)
	eventStore.Save(raw1)
	eventStore.Save([]byte(`{not valid json!!!`))

	evt2 := state.WebhookEvent{
		RunName: "r2", RunID: "id2", Event: "started", UTCTime: "2025-01-15T11:00:00Z",
	}
	raw2, _ := json.Marshal(evt2)
	eventStore.Save(raw2)

	store := state.NewStore()
	count := replayEvents(eventStore, store)

	// 2 valid events replayed (invalid one skipped)
	if count != 2 {
		t.Errorf("replayEvents() = %d, want 2", count)
	}

	if store.GetRun("id1") == nil {
		t.Error("expected run id1 to exist")
	}
	if store.GetRun("id2") == nil {
		t.Error("expected run id2 to exist")
	}
}

// TestDBFlagDefault verifies the env var fallback logic for the db path.
func TestDBFlagDefault(t *testing.T) {
	// With no env var set, default should be "./nextflow-monitor.db"
	got := dbDefault()
	if got != "./nextflow-monitor.db" {
		t.Errorf("dbDefault() = %q, want %q", got, "./nextflow-monitor.db")
	}

	// With env var set, default should use env var
	t.Setenv("NEXTFLOW_MONITOR_DB", "/tmp/custom.db")
	got = dbDefault()
	if got != "/tmp/custom.db" {
		t.Errorf("dbDefault() with env = %q, want %q", got, "/tmp/custom.db")
	}

	// With empty env var, should fall back to default
	t.Setenv("NEXTFLOW_MONITOR_DB", "")
	got = dbDefault()
	if got != "./nextflow-monitor.db" {
		t.Errorf("dbDefault() with empty env = %q, want %q", got, "./nextflow-monitor.db")
	}
}

// TestBuildAddr verifies address string construction from host and port.
func TestBuildAddr(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "default localhost:8080", host: "localhost", port: 8080, want: "localhost:8080"},
		{name: "empty host", host: "", port: 8080, want: ":8080"},
		{name: "all interfaces", host: "0.0.0.0", port: 3000, want: "0.0.0.0:3000"},
		{name: "custom host and port", host: "10.0.0.1", port: 9090, want: "10.0.0.1:9090"},
		{name: "port zero", host: "localhost", port: 0, want: "localhost:0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAddr(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("buildAddr(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

// TestMainServerHandlesWebhook verifies the wired server accepts webhook POSTs.
func TestMainServerHandlesWebhook(t *testing.T) {
	store := state.NewStore()
	srv := server.NewServer(store, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	time.Sleep(50 * time.Millisecond)

	// Webhook route should be registered (GET will likely fail with bad method or bad JSON,
	// but it must not return 404).
	resp, err := http.Get("http://" + ln.Addr().String() + "/webhook")
	if err != nil {
		t.Fatalf("GET /webhook failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("GET /webhook returned 404, expected route to be registered")
	}
}
