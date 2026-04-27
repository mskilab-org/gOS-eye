// Package state manages in-memory pipeline run state updated by Nextflow webhook events.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mskilab-org/nextflow-monitor/internal/traceimport"
)

// ---- Data Definitions: Webhook JSON Schema ----

// WebhookEvent is the top-level JSON envelope POSTed by Nextflow's -with-weblog.
// Event field determines which optional sub-struct is populated:
//   - "started", "completed", "error" → Metadata is set
//   - "process_submitted", "process_started", "process_completed", "process_failed" → Trace is set
//
// Webhook RunID is the raw Nextflow session identity emitted by nf-weblog. Under
// -resume, multiple launches may reuse this same RunID while changing RunName.
type WebhookEvent struct {
	RunName  string    `json:"runName"`
	RunID    string    `json:"runId"`
	Event    string    `json:"event"`
	UTCTime  string    `json:"utcTime"`
	Metadata *Metadata `json:"metadata,omitempty"`
	Trace    *Trace    `json:"trace,omitempty"`
}

// Metadata carries workflow-level info, present in "started" events.
// Parameters is a top-level field in the Nextflow webhook metadata (sibling to workflow),
// containing pipeline parameters (e.g. --input, --outdir) as a flat key-value map.
type Metadata struct {
	Workflow   WorkflowInfo   `json:"workflow"`
	Parameters map[string]any `json:"parameters"`
}

// WorkflowInfo describes the pipeline being executed.
// Start is json.RawMessage because Nextflow sends it as a complex ZonedDateTime object,
// not a simple string. We don't parse it — we use event.UTCTime for timestamps.
// ErrorMessage is sent by Nextflow in "completed" (with errors) and "error" events.
// CommandLine, SessionID, WorkDir, and LaunchDir are captured from the "started"
// event for resume-command reconstruction and trace resolution.
type WorkflowInfo struct {
	ProjectName  string          `json:"projectName"`
	ScriptFile   string          `json:"scriptFile"`
	Start        json.RawMessage `json:"start"`
	ConfigFiles  []string        `json:"configFiles"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Manifest     Manifest        `json:"manifest"`
	CommandLine  string          `json:"commandLine"`
	SessionID    string          `json:"sessionId"`
	WorkDir      string          `json:"workDir"`
	LaunchDir    string          `json:"launchDir"`
	Resume       bool            `json:"resume"`
}

// Manifest carries pipeline metadata from nextflow.config's manifest block.
type Manifest struct {
	Name string `json:"name"`
}

// TaskStatus is the set of task lifecycle states rendered by the monitor.
type TaskStatus string

const (
	TaskStatusSubmitted TaskStatus = "SUBMITTED"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusCompleted TaskStatus = "COMPLETED"
	TaskStatusFailed    TaskStatus = "FAILED"
	TaskStatusCached    TaskStatus = "CACHED"
)

// TaskSource identifies where the monitor learned about a task.
type TaskSource string

const (
	TaskSourceUnknown     TaskSource = ""
	TaskSourceLiveWebhook TaskSource = "live_webhook"
	TaskSourceCachedTrace TaskSource = "cached_trace"
)

// WorkdirProvenance identifies how Task.Workdir was populated, or why it is empty.
type WorkdirProvenance string

const (
	WorkdirProvenanceUnknown       WorkdirProvenance = ""
	WorkdirProvenanceLiveWebhook   WorkdirProvenance = "live_webhook"
	WorkdirProvenanceTraceWorkdir  WorkdirProvenance = "trace_workdir"
	WorkdirProvenanceHashResolved  WorkdirProvenance = "hash_resolved"
	WorkdirProvenanceUnresolved    WorkdirProvenance = "unresolved"
	WorkdirProvenanceAmbiguousHash WorkdirProvenance = "ambiguous_hash"
)

// Trace carries task-level details, present in process_* events.
// JSON field "%cpu" requires custom unmarshalling.
type Trace struct {
	TaskID     int        `json:"task_id"`
	Status     TaskStatus `json:"status"`
	Hash       string     `json:"hash"`
	Name       string     `json:"name"`
	Process    string     `json:"process"`
	Tag        string     `json:"tag"`
	Submit     int64      `json:"submit"`
	Start      int64      `json:"start"`
	Complete   int64      `json:"complete"`
	Duration   int64      `json:"duration"`
	Realtime   int64      `json:"realtime"`
	CPUPercent float64    `json:"-"` // populated from "%cpu" via custom unmarshal
	RSS        int64      `json:"rss"`
	VMem       int64      `json:"vmem"`
	PeakRSS    int64      `json:"peak_rss"`
	CPUs       int        `json:"cpus"`
	Memory     int64      `json:"memory"`
	Exit       int        `json:"exit"`
	Workdir    string     `json:"workdir"`
	Script     string     `json:"script"`
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
// copied from the Trace on each process_* event. Source and workdir provenance are
// lightweight UI-facing metadata: live webhook tasks get live provenance, and cached
// trace rows record whether Workdir came from an explicit trace column, hash-based
// recovery, or remained unresolved.
type Task struct {
	TaskID            int
	Hash              string
	Name              string
	Process           string
	Tag               string
	Status            TaskStatus
	Source            TaskSource
	Submit            int64   // epoch millis when task was submitted
	Start             int64   // epoch millis when task started executing
	Complete          int64   // epoch millis when task finished
	Duration          int64   // wall-clock duration in milliseconds
	Realtime          int64   // actual CPU time in milliseconds
	CPUPercent        float64 // CPU usage percentage (from Trace's "%cpu")
	RSS               int64   // resident set size in bytes
	PeakRSS           int64   // peak resident set size in bytes
	Exit              int     // process exit code (0 = success)
	Workdir           string  // working directory path for this task
	WorkdirProvenance WorkdirProvenance
	WorkdirWarning    string // non-fatal cached workdir recovery warning for task detail UI
}

// CachedWorkdirResolution is the result of resolving a cached trace row's workdir.
// Exactly one hash match populates Workdir with hash-resolved provenance. Explicit
// trace workdir wins even if the path does not exist. Missing, malformed, ambiguous,
// or unavailable inputs leave Workdir empty and carry a warning/provenance instead.
type CachedWorkdirResolution struct {
	Workdir    string
	Provenance WorkdirProvenance
	Warning    string
	MatchCount int
}

// TraceImportState tracks best-effort trace discovery/import for the monitor run.
type TraceImportState struct {
	RequestedPath                   string
	ResolvedPath                    string
	Warning                         string
	Imported                        bool
	CachedTasks                     int
	CachedWorkdirsExplicit          int
	CachedWorkdirsHashResolved      int
	CachedWorkdirsUnresolved        int
	CachedWorkdirsAmbiguous         int
	CachedWorkdirResolutionWarnings []string
}

func (t *TraceImportState) appendWarning(warning string) {
	if warning == "" {
		return
	}
	if t.Warning != "" {
		t.Warning += "; " + warning
	} else {
		t.Warning = warning
	}
}

// appendCachedWorkdirWarning records a non-fatal cached workdir recovery warning.
func (t *TraceImportState) appendCachedWorkdirWarning(warning string) {
	if warning == "" {
		return
	}
	t.CachedWorkdirResolutionWarnings = append(t.CachedWorkdirResolutionWarnings, warning)
}

// liveWebhookTaskFromTrace builds a Task from a live process_* webhook trace with live provenance.
func liveWebhookTaskFromTrace(tr *Trace) *Task {
	if tr == nil {
		return nil
	}

	task := &Task{
		TaskID:     tr.TaskID,
		Hash:       tr.Hash,
		Name:       tr.Name,
		Process:    tr.Process,
		Tag:        tr.Tag,
		Status:     tr.Status,
		Source:     TaskSourceLiveWebhook,
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
	if tr.Workdir != "" {
		task.WorkdirProvenance = WorkdirProvenanceLiveWebhook
	}
	return task
}

// cachedTraceTaskFromRow builds a cached Task from a parsed trace row and resolved workdir provenance.
func cachedTraceTaskFromRow(run *Run, row traceimport.Row) *Task {
	runWorkDir := ""
	if run != nil {
		runWorkDir = run.WorkDir
	}
	workdir := resolveCachedWorkdir(runWorkDir, row)

	return &Task{
		TaskID:            row.TaskID,
		Hash:              row.Hash,
		Name:              row.Name,
		Process:           row.Process,
		Tag:               row.Tag,
		Status:            TaskStatusCached,
		Source:            TaskSourceCachedTrace,
		Submit:            row.Submit,
		Start:             row.Start,
		Complete:          row.Complete,
		Duration:          row.Duration,
		Realtime:          row.Realtime,
		CPUPercent:        row.CPUPercent,
		PeakRSS:           row.PeakRSS,
		Exit:              row.Exit,
		Workdir:           workdir.Workdir,
		WorkdirProvenance: workdir.Provenance,
		WorkdirWarning:    workdir.Warning,
	}
}

// resolveCachedWorkdir resolves a cached trace row's workdir using explicit trace workdir first, then run workDir plus hash.
func resolveCachedWorkdir(runWorkDir string, row traceimport.Row) CachedWorkdirResolution {
	if row.Workdir != "" {
		return CachedWorkdirResolution{
			Workdir:    row.Workdir,
			Provenance: WorkdirProvenanceTraceWorkdir,
		}
	}

	return resolveCachedWorkdirFromHash(runWorkDir, row.Hash)
}

func unresolvedCachedWorkdir(warning string) CachedWorkdirResolution {
	return CachedWorkdirResolution{
		Provenance: WorkdirProvenanceUnresolved,
		Warning:    warning,
	}
}

// resolveCachedWorkdirFromHash resolves Nextflow hash prefixes under runWorkDir without choosing ambiguous matches.
func resolveCachedWorkdirFromHash(runWorkDir, hash string) CachedWorkdirResolution {
	runWorkDir = strings.TrimSpace(runWorkDir)
	hash = strings.TrimSpace(hash)

	if runWorkDir == "" {
		return unresolvedCachedWorkdir("cached workdir unresolved: blank run workDir")
	}
	if hash == "" {
		return unresolvedCachedWorkdir("cached workdir unresolved: blank task hash")
	}

	parts := strings.Split(hash, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return unresolvedCachedWorkdir(fmt.Sprintf("cached workdir unresolved: malformed task hash %q; expected prefix/suffix like 88/fa3254", hash))
	}

	parent := filepath.Join(runWorkDir, parts[0])
	entries, err := os.ReadDir(parent)
	if err != nil {
		return unresolvedCachedWorkdir(fmt.Sprintf("cached workdir unresolved: cannot scan %q for hash %q: %v", parent, hash, err))
	}

	matches := make([]string, 0, 1)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), parts[1]) {
			continue
		}
		matches = append(matches, filepath.Join(parent, entry.Name()))
	}

	switch len(matches) {
	case 1:
		return CachedWorkdirResolution{
			Workdir:    matches[0],
			Provenance: WorkdirProvenanceHashResolved,
			MatchCount: 1,
		}
	case 0:
		return unresolvedCachedWorkdir(fmt.Sprintf("cached workdir unresolved: no directory under %q matched hash %q", parent, hash))
	default:
		return CachedWorkdirResolution{
			Provenance: WorkdirProvenanceAmbiguousHash,
			Warning:    fmt.Sprintf("cached workdir ambiguous: %d directories under %q matched hash %q; not choosing arbitrarily", len(matches), parent, hash),
			MatchCount: len(matches),
		}
	}
}

// recordCachedWorkdirResolution updates aggregate trace-import counters and warnings from one cached workdir resolution.
func recordCachedWorkdirResolution(traceImport *TraceImportState, resolution CachedWorkdirResolution) {
	if traceImport == nil {
		return
	}

	switch resolution.Provenance {
	case WorkdirProvenanceTraceWorkdir:
		traceImport.CachedWorkdirsExplicit++
	case WorkdirProvenanceHashResolved:
		traceImport.CachedWorkdirsHashResolved++
	case WorkdirProvenanceUnresolved:
		traceImport.CachedWorkdirsUnresolved++
	case WorkdirProvenanceAmbiguousHash:
		traceImport.CachedWorkdirsAmbiguous++
	}

	traceImport.appendCachedWorkdirWarning(resolution.Warning)
}

// RunSourceKey identifies the webhook source route for a monitor run.
type RunSourceKey struct {
	NextflowRunID string
	RunName       string
}

// Run represents one monitor execution attempt.
// ID is the monitor's stable internal execution ID used by routes/UI state.
// NextflowRunID preserves the raw webhook runId/session identity emitted by Nextflow.
// Tasks is keyed by TaskID for O(1) upsert from webhook and trace-import updates.
// CompleteTime is set when the run finishes (status "completed" or "error").
// ErrorMessage is extracted from metadata.workflow.errorMessage on "completed" (with errors)
// or "error" events. Defaults to a generic message if the error event lacks a message.
// CommandLine, SessionID, WorkDir, LaunchDir, and Params are captured from the "started"
// event metadata for resume-command reconstruction, samplesheet viewing, and trace resolution.
type Run struct {
	mu            sync.RWMutex // protects Run fields during concurrent read/write
	ID            string
	RunID         string // deprecated alias for ID retained for backward compatibility
	Attempt       int
	RunName       string
	NextflowRunID string
	ProjectName   string // pipeline name from metadata.workflow.projectName
	Status        string
	StartTime     string
	CompleteTime  string         // UTC timestamp when run finished (from "completed"/"error" event)
	ErrorMessage  string         // error message from Nextflow, empty if run succeeded
	CommandLine   string         // original command line from workflow metadata
	SessionID     string         // Nextflow session ID for -resume
	WorkDir       string         // working directory path
	LaunchDir     string         // launch directory path
	ScriptFile    string         // absolute path to the pipeline's main script file
	Params        map[string]any // pipeline parameters from workflow metadata
	IsResume      bool
	TraceImport   TraceImportState
	Tasks         map[int]*Task
}

// RLock acquires a read lock on the Run. Callers iterating Tasks or reading
// Run fields concurrently with HandleEvent must hold this lock.
func (r *Run) RLock()   { r.mu.RLock() }
func (r *Run) RUnlock() { r.mu.RUnlock() }

// MonitorID returns the canonical monitor execution ID, falling back to the
// deprecated RunID alias while tests/callers are still being migrated.
func (r *Run) MonitorID() string {
	if r.ID != "" {
		return r.ID
	}
	return r.RunID
}

func (r *Run) sessionKey() string {
	if r.SessionID != "" {
		return r.SessionID
	}
	return r.NextflowRunID
}

// Store is the concurrent state container for all monitor runs.
// Runs is keyed by monitor execution ID, not raw Nextflow runId.
// Protected by RWMutex: webhook handler takes write lock, SSE fan-out takes read lock.
type Store struct {
	mu                    sync.RWMutex
	Runs                  map[string]*Run
	routeBySource         map[RunSourceKey]string
	latestRunBySession    map[string]string
	latestRunByNextflowID map[string]string
	attemptBySession      map[string]int
	latestRunID           string // tracks the most recently updated monitor run (set by HandleEvent)
}

// ---- Constructor ----

// NewStore creates an empty Store ready to receive webhook events.
func NewStore() *Store {
	return &Store{
		Runs:                  make(map[string]*Run),
		routeBySource:         make(map[RunSourceKey]string),
		latestRunBySession:    make(map[string]string),
		latestRunByNextflowID: make(map[string]string),
		attemptBySession:      make(map[string]int),
	}
}

// ---- Methods on Store ----

// HandleEvent processes a single WebhookEvent, updating or creating Run/Task state.
// It returns the monitor execution ID for the run that handled the event, or ""
// when the event is ignored.
// Event type dispatch:
//   - "started"           → create/update Run with metadata, set status "running"
//   - "process_submitted" → upsert Task with status from trace
//   - "process_started"   → upsert Task with status from trace
//   - "process_completed" → upsert Task with status from trace
//   - "process_failed"    → upsert Task with status from trace
//   - "completed"         → set Run status "completed" and try final cached-trace import
//   - "error"             → set Run status "error"
func (s *Store) HandleEvent(event WebhookEvent) string {
	switch event.Event {
	case "started":
		s.mu.Lock()
		defer s.mu.Unlock()

		r := s.ensureRun(event)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.Status = "running"
		r.StartTime = event.UTCTime
		if event.Metadata != nil {
			r.ProjectName = event.Metadata.Workflow.ProjectName
			// For local pipelines, projectName is just the script filename
			// (e.g. "main.nf"). Prefer manifest.name when available.
			if event.Metadata.Workflow.Manifest.Name != "" {
				r.ProjectName = event.Metadata.Workflow.Manifest.Name
			}
			r.CommandLine = event.Metadata.Workflow.CommandLine
			r.SessionID = event.Metadata.Workflow.SessionID
			r.WorkDir = event.Metadata.Workflow.WorkDir
			r.LaunchDir = event.Metadata.Workflow.LaunchDir
			r.ScriptFile = event.Metadata.Workflow.ScriptFile
			r.Params = event.Metadata.Parameters
			r.IsResume = event.Metadata.Workflow.Resume
		}
		s.registerRunRouting(r)
		monitorRunID := r.MonitorID()
		s.latestRunID = monitorRunID
		return monitorRunID

	case "process_submitted", "process_started", "process_completed", "process_failed":
		if event.Trace == nil {
			return ""
		}
		s.mu.Lock()
		defer s.mu.Unlock()

		r := s.ensureRun(event)
		r.mu.Lock()
		defer r.mu.Unlock()
		tr := event.Trace
		r.Tasks[tr.TaskID] = liveWebhookTaskFromTrace(tr)
		s.registerRunRouting(r)
		monitorRunID := r.MonitorID()
		s.latestRunID = monitorRunID
		return monitorRunID

	case "completed":
		s.mu.Lock()
		defer s.mu.Unlock()
		r := s.ensureRun(event)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.CompleteTime = event.UTCTime
		if event.Metadata != nil && event.Metadata.Workflow.ErrorMessage != "" {
			r.Status = "failed"
			r.ErrorMessage = event.Metadata.Workflow.ErrorMessage
		} else {
			r.Status = "completed"
		}
		if err := s.importCachedTasksOnCompletion(r); err != nil {
			r.TraceImport.Warning = err.Error()
		}
		s.registerRunRouting(r)
		monitorRunID := r.MonitorID()
		s.latestRunID = monitorRunID
		return monitorRunID

	case "error":
		s.mu.Lock()
		defer s.mu.Unlock()
		r := s.ensureRun(event)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.Status = "failed"
		r.CompleteTime = event.UTCTime
		if event.Metadata != nil && event.Metadata.Workflow.ErrorMessage != "" {
			r.ErrorMessage = event.Metadata.Workflow.ErrorMessage
		} else {
			r.ErrorMessage = "Pipeline error (no message provided)"
		}
		s.registerRunRouting(r)
		monitorRunID := r.MonitorID()
		s.latestRunID = monitorRunID
		return monitorRunID
	}
	return ""
}

// ensureRun returns the monitor Run for this event, creating it if absent.
// Caller must hold s.mu write lock.
func (s *Store) ensureRun(event WebhookEvent) *Run {
	monitorRunID := ""
	attempt := 1
	if event.Event == "started" {
		monitorRunID, attempt = s.allocateMonitorRun(event)
	} else {
		monitorRunID = s.resolveMonitorRunID(event)
	}

	r, ok := s.Runs[monitorRunID]
	if !ok {
		r = &Run{
			ID:            monitorRunID,
			RunID:         monitorRunID,
			Attempt:       attempt,
			RunName:       event.RunName,
			NextflowRunID: event.RunID,
			IsResume:      event.Metadata != nil && event.Metadata.Workflow.Resume,
			Tasks:         make(map[int]*Task),
		}
		s.Runs[monitorRunID] = r
	}
	if event.RunName != "" {
		r.RunName = event.RunName
	}
	if r.ID == "" && r.RunID != "" {
		r.ID = r.RunID
	}
	if r.RunID == "" && r.ID != "" {
		r.RunID = r.ID
	}
	if event.RunID != "" {
		r.NextflowRunID = event.RunID
	}
	if event.Metadata != nil && event.Metadata.Workflow.Resume {
		r.IsResume = true
	}
	return r
}

// allocateMonitorRun reserves the monitor ID + attempt number for a new started event.
// The first attempt uses the base Nextflow identity; later attempts append --attempt-N.
func (s *Store) allocateMonitorRun(event WebhookEvent) (string, int) {
	attempt := s.nextAttemptNumber(event)

	baseID := event.RunID
	if baseID == "" && event.Metadata != nil && event.Metadata.Workflow.SessionID != "" {
		baseID = event.Metadata.Workflow.SessionID
	}
	if baseID == "" {
		baseID = "__nextflow-monitor-empty-run__"
	}

	if attempt == 1 {
		return baseID, attempt
	}
	return fmt.Sprintf("%s--attempt-%d", baseID, attempt), attempt
}

// nextAttemptNumber increments and returns the per-session attempt number.
// Session metadata is preferred over the raw runId so replays share a counter.
func (s *Store) nextAttemptNumber(event WebhookEvent) int {
	if s.attemptBySession == nil {
		s.attemptBySession = make(map[string]int)
	}

	sessionKey := event.RunID
	if event.Metadata != nil && event.Metadata.Workflow.SessionID != "" {
		sessionKey = event.Metadata.Workflow.SessionID
	}

	s.attemptBySession[sessionKey]++
	return s.attemptBySession[sessionKey]
}

// resolveMonitorRunID routes a non-started webhook event to the current monitor run.
// Preference order is exact (runId, runName), then latest for metadata.sessionId,
// then latest for the raw Nextflow runId. A final legacy fallback checks the
// session index keyed by runId for older/incomplete state.
func (s *Store) resolveMonitorRunID(event WebhookEvent) string {
	if monitorRunID, ok := s.routeBySource[sourceKey(event)]; ok {
		return monitorRunID
	}

	sessionID := ""
	if event.Metadata != nil {
		sessionID = event.Metadata.Workflow.SessionID
	}
	if sessionID != "" {
		if monitorRunID, ok := s.latestRunBySession[sessionID]; ok {
			return monitorRunID
		}
	}

	if monitorRunID, ok := s.latestRunByNextflowID[event.RunID]; ok {
		return monitorRunID
	}
	if monitorRunID, ok := s.latestRunBySession[event.RunID]; ok {
		return monitorRunID
	}
	return event.RunID
}

func sourceKey(event WebhookEvent) RunSourceKey {
	return RunSourceKey{NextflowRunID: event.RunID, RunName: event.RunName}
}

func (s *Store) registerRunRouting(run *Run) {
	key := RunSourceKey{NextflowRunID: run.NextflowRunID, RunName: run.RunName}
	if key.NextflowRunID != "" || key.RunName != "" {
		s.routeBySource[key] = run.MonitorID()
	}
	if run.NextflowRunID != "" {
		s.latestRunByNextflowID[run.NextflowRunID] = run.MonitorID()
	}
	if sessionKey := run.sessionKey(); sessionKey != "" {
		s.latestRunBySession[sessionKey] = run.MonitorID()
	}
}

// importCachedTasksOnCompletion resolves/parses the run trace and merges any CACHED rows.
// Trace import is best-effort: resolution/parse failures are captured as warnings
// on the run so completion webhook handling can keep the run usable.
func (s *Store) importCachedTasksOnCompletion(run *Run) error {
	if run == nil {
		return nil
	}

	resolution, err := traceimport.ResolvePath(traceimport.RunContext{
		CommandLine:  run.CommandLine,
		LaunchDir:    run.LaunchDir,
		StartTime:    run.StartTime,
		CompleteTime: run.CompleteTime,
	})

	run.TraceImport = TraceImportState{
		RequestedPath: resolution.RequestedPath,
		ResolvedPath:  resolution.ResolvedPath,
		Warning:       resolution.Warning,
	}
	if err != nil {
		run.TraceImport.appendWarning(err.Error())
		return nil
	}
	if resolution.ResolvedPath == "" {
		return nil
	}

	rows, err := traceimport.ParseFile(resolution.ResolvedPath)
	if err != nil {
		run.TraceImport.appendWarning(err.Error())
		return nil
	}

	s.mergeCachedTraceRows(run, rows)
	run.TraceImport.Imported = true
	return nil
}

// mergeCachedTraceRows merges parsed trace rows with status CACHED into run.Tasks.
func (s *Store) mergeCachedTraceRows(run *Run, rows []traceimport.Row) {
	if run == nil {
		return
	}
	if run.Tasks == nil {
		run.Tasks = make(map[int]*Task)
	}

	run.TraceImport.CachedTasks = 0
	run.TraceImport.CachedWorkdirsExplicit = 0
	run.TraceImport.CachedWorkdirsHashResolved = 0
	run.TraceImport.CachedWorkdirsUnresolved = 0
	run.TraceImport.CachedWorkdirsAmbiguous = 0
	run.TraceImport.CachedWorkdirResolutionWarnings = nil

	cachedTasks := 0
	for _, row := range rows {
		if row.Status != string(TaskStatusCached) {
			continue
		}

		existing := run.Tasks[row.TaskID]
		if existing != nil && existing.Status != "" && existing.Status != TaskStatusCached {
			continue
		}

		task := cachedTraceTaskFromRow(run, row)
		run.Tasks[row.TaskID] = task
		recordCachedWorkdirResolution(&run.TraceImport, CachedWorkdirResolution{
			Workdir:    task.Workdir,
			Provenance: task.WorkdirProvenance,
			Warning:    task.WorkdirWarning,
		})
		cachedTasks++
	}

	run.TraceImport.CachedTasks = cachedTasks
}

// GetRun returns the monitor Run for a given monitor run ID, or nil if not found. Read-locked.
func (s *Store) GetRun(runID string) *Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Runs[runID]
}

// GetLatestRunID returns the monitor run ID of the most recently updated Run, or "" if none.
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
