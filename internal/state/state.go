// Package state manages in-memory pipeline run state updated by Nextflow webhook events.
package state

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ---- Data Definitions: Webhook JSON Schema ----

// WebhookEvent is the top-level JSON envelope POSTed by Nextflow's -with-weblog.
// Event field determines which optional sub-struct is populated:
//   - "started", "completed", "error" → Metadata is set
//   - "process_submitted", "process_started", "process_completed", "process_failed" → Trace is set
type WebhookEvent struct {
	RunName  string    `json:"runName"`
	RunID    string    `json:"runId"`
	Event    string    `json:"event"`
	UTCTime  string    `json:"utcTime"`
	Metadata *Metadata `json:"metadata,omitempty"`
	Trace    *Trace    `json:"trace,omitempty"`
}

// Metadata carries workflow-level info, present in "started" events.
type Metadata struct {
	Workflow WorkflowInfo `json:"workflow"`
}

// WorkflowInfo describes the pipeline being executed.
// Start is json.RawMessage because Nextflow sends it as a complex ZonedDateTime object,
// not a simple string. We don't parse it — we use event.UTCTime for timestamps.
// ErrorMessage is sent by Nextflow in "completed" (with errors) and "error" events.
type WorkflowInfo struct {
	ProjectName  string          `json:"projectName"`
	ScriptFile   string          `json:"scriptFile"`
	Start        json.RawMessage `json:"start"`
	ConfigFiles  []string        `json:"configFiles"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Manifest     Manifest        `json:"manifest"`
}

// Manifest carries pipeline metadata from nextflow.config's manifest block.
type Manifest struct {
	Name string `json:"name"`
}

// Trace carries task-level details, present in process_* events.
// JSON field "%cpu" requires custom unmarshalling.
type Trace struct {
	TaskID     int     `json:"task_id"`
	Status     string  `json:"status"`
	Hash       string  `json:"hash"`
	Name       string  `json:"name"`
	Process    string  `json:"process"`
	Tag        string  `json:"tag"`
	Submit     int64   `json:"submit"`
	Start      int64   `json:"start"`
	Complete   int64   `json:"complete"`
	Duration   int64   `json:"duration"`
	Realtime   int64   `json:"realtime"`
	CPUPercent float64 `json:"-"` // populated from "%cpu" via custom unmarshal
	RSS        int64   `json:"rss"`
	VMem       int64   `json:"vmem"`
	PeakRSS    int64   `json:"peak_rss"`
	CPUs       int     `json:"cpus"`
	Memory     int64   `json:"memory"`
	Exit       int     `json:"exit"`
	Workdir    string  `json:"workdir"`
	Script     string  `json:"script"`
}

// UnmarshalJSON handles the "%cpu" field that Go's struct tags can't express.
// Uses a type alias to decode standard fields (avoiding infinite recursion),
// then extracts "%cpu" from a raw map. Absent or null "%cpu" defaults to 0.
func (t *Trace) UnmarshalJSON(data []byte) error {
	// Type alias has same fields but no methods, so json.Unmarshal won't recurse.
	type traceAlias Trace
	var alias traceAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*t = Trace(alias)

	// Extract "%cpu" from the raw JSON map.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if cpuRaw, ok := raw["%cpu"]; ok && string(cpuRaw) != "null" {
		var cpu float64
		if err := json.Unmarshal(cpuRaw, &cpu); err != nil {
			return fmt.Errorf("parsing %%cpu: %w", err)
		}
		t.CPUPercent = cpu
	}
	return nil
}

// ---- Data Definitions: In-Memory State ----

// Task is the in-memory representation of a single pipeline task.
// Identity fields (TaskID, Hash, Name, Process, Status) plus resource/timing metrics
// copied from the Trace on each process_* event.
type Task struct {
	TaskID     int
	Hash       string
	Name       string
	Process    string
	Status     string
	Submit     int64   // epoch millis when task was submitted
	Start      int64   // epoch millis when task started executing
	Complete   int64   // epoch millis when task finished
	Duration   int64   // wall-clock duration in milliseconds
	Realtime   int64   // actual CPU time in milliseconds
	CPUPercent float64 // CPU usage percentage (from Trace's "%cpu")
	RSS        int64   // resident set size in bytes
	PeakRSS    int64   // peak resident set size in bytes
	Exit       int     // process exit code (0 = success)
	Workdir    string  // working directory path for this task
}

// Run represents one pipeline execution, keyed by RunID.
// Tasks is keyed by TaskID for O(1) upsert from webhook events.
// CompleteTime is set when the run finishes (status "completed" or "error").
// ErrorMessage is extracted from metadata.workflow.errorMessage on "completed" (with errors)
// or "error" events. Defaults to a generic message if the error event lacks a message.
type Run struct {
	RunName      string
	RunID        string
	ProjectName  string // pipeline name from metadata.workflow.projectName
	Status       string
	StartTime    string
	CompleteTime string // UTC timestamp when run finished (from "completed"/"error" event)
	ErrorMessage string // error message from Nextflow, empty if run succeeded
	Tasks        map[int]*Task
}

// Store is the concurrent state container for all pipeline runs.
// Protected by RWMutex: webhook handler takes write lock, SSE fan-out takes read lock.
type Store struct {
	mu          sync.RWMutex
	Runs        map[string]*Run
	latestRunID string // tracks the most recently updated run (set by HandleEvent)
}

// ---- Constructor ----

// NewStore creates an empty Store ready to receive webhook events.
func NewStore() *Store {
	return &Store{
		Runs: make(map[string]*Run),
	}
}

// ---- Methods on Store ----

// HandleEvent processes a single WebhookEvent, updating or creating Run/Task state.
// Event type dispatch:
//   - "started"           → create/update Run with metadata, set status "running"
//   - "process_submitted" → upsert Task with status from trace
//   - "process_started"   → upsert Task with status from trace
//   - "process_completed" → upsert Task with status from trace
//   - "process_failed"    → upsert Task with status from trace
//   - "completed"         → set Run status "completed"
//   - "error"             → set Run status "error"
func (s *Store) HandleEvent(event WebhookEvent) {
	switch event.Event {
	case "started":
		s.mu.Lock()
		defer s.mu.Unlock()

		r := s.ensureRun(event)
		r.Status = "running"
		r.StartTime = event.UTCTime
		if event.Metadata != nil {
			r.ProjectName = event.Metadata.Workflow.ProjectName
			// For local pipelines, projectName is just the script filename
			// (e.g. "main.nf"). Prefer manifest.name when available.
			if event.Metadata.Workflow.Manifest.Name != "" {
				r.ProjectName = event.Metadata.Workflow.Manifest.Name
			}
		}
		s.latestRunID = event.RunID

	case "process_submitted", "process_started", "process_completed", "process_failed":
		if event.Trace == nil {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()

		r := s.ensureRun(event)
		tr := event.Trace
		r.Tasks[tr.TaskID] = &Task{
			TaskID:     tr.TaskID,
			Hash:       tr.Hash,
			Name:       tr.Name,
			Process:    tr.Process,
			Status:     tr.Status,
			Submit:     tr.Submit,
			Start:      tr.Start,
			Complete:   tr.Complete,
			Duration:   tr.Duration,
			Realtime:   tr.Realtime,
			CPUPercent: tr.CPUPercent,
			RSS:        tr.RSS,
			PeakRSS:    tr.PeakRSS,
			Exit:       tr.Exit,
			Workdir:    tr.Workdir,
		}
		s.latestRunID = event.RunID

	case "completed":
		s.mu.Lock()
		defer s.mu.Unlock()
		r := s.ensureRun(event)
		r.CompleteTime = event.UTCTime
		if event.Metadata != nil && event.Metadata.Workflow.ErrorMessage != "" {
			r.Status = "failed"
			r.ErrorMessage = event.Metadata.Workflow.ErrorMessage
		} else {
			r.Status = "completed"
		}
		s.latestRunID = event.RunID

	case "error":
		s.mu.Lock()
		defer s.mu.Unlock()
		r := s.ensureRun(event)
		r.Status = "failed"
		r.CompleteTime = event.UTCTime
		if event.Metadata != nil && event.Metadata.Workflow.ErrorMessage != "" {
			r.ErrorMessage = event.Metadata.Workflow.ErrorMessage
		} else {
			r.ErrorMessage = "Pipeline error (no message provided)"
		}
		s.latestRunID = event.RunID
	}
}

// ensureRun returns the Run for event.RunID, creating it if absent.
// Caller must hold s.mu write lock.
func (s *Store) ensureRun(event WebhookEvent) *Run {
	r, ok := s.Runs[event.RunID]
	if !ok {
		r = &Run{
			RunName: event.RunName,
			RunID:   event.RunID,
			Tasks:   make(map[int]*Task),
		}
		s.Runs[event.RunID] = r
	}
	return r
}

// GetRun returns the Run for a given runID, or nil if not found. Read-locked.
func (s *Store) GetRun(runID string) *Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Runs[runID]
}

// GetLatestRunID returns the RunID of the most recently updated Run, or "" if none.
func (s *Store) GetLatestRunID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestRunID
}

// GetLatestRun returns the most recently updated Run, or nil if no events have been received.
func (s *Store) GetLatestRun() *Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latestRunID == "" {
		return nil
	}
	return s.Runs[s.latestRunID]
}

// GetAllRuns returns all known Runs. Read-locked.
// Returns a slice for stable iteration (map order is random).
func (s *Store) GetAllRuns() []*Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := make([]*Run, 0, len(s.Runs))
	for _, r := range s.Runs {
		runs = append(runs, r)
	}
	return runs
}

// Ensure json import is used (for UnmarshalJSON).
var _ = json.Unmarshal
