// Package server provides HTTP handlers for the webhook endpoint, SSE fan-out, and static frontend.
package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/db"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// ---- Data Definition: Event Persistence ----

// EventPersister abstracts the write side of event storage.
// Implementations persist raw webhook JSON for replay on restart.
type EventPersister interface {
	Save(rawJSON []byte) error
	SaveDAG(runID, projectName string, dotText []byte) error
	LoadAllDAGs() ([]db.DAGRecord, error)
}

// ---- Data Definition: SSE Fan-Out ----

// Broker manages SSE subscriber channels. Each connected browser gets a channel.
// Webhook handler publishes HTML fragments; SSE handler streams them to subscribers.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[chan string]struct{}
}

// NewBroker creates a Broker with an empty subscriber set.
func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[chan string]struct{}),
	}
}

// Subscribe registers a new SSE client and returns its channel.
// The channel receives HTML fragment strings to send as SSE events.
func (b *Broker) Subscribe() chan string {
	ch := make(chan string, 16)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel and closes it.
// Called when the SSE connection drops (detected via r.Context().Done()).
func (b *Broker) Unsubscribe(ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

// Publish sends an HTML fragment string to all subscriber channels.
// Non-blocking: if a subscriber's channel is full, skip it (slow client).
func (b *Broker) Publish(data string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- data:
		default:
			// slow/full client — skip
		}
	}
}

// ---- Data Definition: Per-Run SSE Fan-Out ----



// ---- Data Definition: Dashboard View Model ----

// ProcessGroup is a view model for the dashboard: tasks grouped by process name
// with per-status counts and the actual task objects for rendering detail rows.
// Used by renderDashboard to build the process group list with expandable task details.
type ProcessGroup struct {
	Name      string        // process name (e.g., "sayHello", "align")
	Total     int           // total tasks in this group
	Completed int           // tasks with status COMPLETED
	Running   int           // tasks with status RUNNING
	Failed    int           // tasks with status FAILED
	Submitted int           // tasks with status SUBMITTED
	Tasks     []*state.Task // actual task objects, sorted: failed first, then by name
}

// renderGroupStatusDot returns the HTML for a process group's status indicator dot.
// Priority: failed > running > completed > pending.
func renderGroupStatusDot(g ProcessGroup) string {
	if g.Failed > 0 {
		return `<span class="group-status-indicator status-failed">●</span>`
	} else if g.Running > 0 {
		return `<span class="group-status-indicator status-running">●</span>`
	} else if g.Total > 0 && g.Completed == g.Total {
		return `<span class="group-status-indicator status-completed">●</span>`
	}
	return `<span class="group-status-indicator status-pending">●</span>`
}

// ---- Data Definition: HTTP Server ----

// Server ties together the state store, SSE broker, and HTTP routes.
// When a pipeline's DAG layout is discovered, the dashboard renders a DAG view
// instead of process group lists for that pipeline.
// The sidebar broker fans out sidebar HTML to all connected browsers.
// The sidebar broker fans out sidebar HTML to all connected browsers.
type Server struct {
	store      *state.Store
	eventStore EventPersister              // persists raw webhook JSON; nil = no persistence
	broker     *Broker                     // sidebar SSE fan-out (all clients)
	mux        *http.ServeMux
	layoutsMu  sync.RWMutex               // protects layouts
	layouts    map[string]*dag.Layout     // runID → computed layout
}

// NewServer creates a Server with routes registered on an internal ServeMux.
// Routes:
//   - POST /webhook        → handleWebhook
//   - GET  /sse/sidebar    → handleSSE (sidebar updates)
//   - GET  /sse/run/{id}   → handleRunSSE (per-run detail updates)
//   - GET  /               → handleIndex
func NewServer(store *state.Store, persist EventPersister) *Server {
	s := &Server{
		store:      store,
		eventStore: persist,
		broker:     NewBroker(),
		mux:        http.NewServeMux(),
		layouts:    make(map[string]*dag.Layout),
	}
	s.mux.HandleFunc("/webhook", s.handleWebhook)
	s.mux.HandleFunc("/sse/sidebar", s.handleSSE)
	s.mux.HandleFunc("/dashboard/{id}", s.handleDashboard)
	s.mux.HandleFunc("/tasks/{runID}/{process}", s.handleTaskPanel)
	s.mux.HandleFunc("/sse/task/{run}/{task}/logs", s.handleTaskLogs)
	s.mux.HandleFunc("/select-run/{id}", s.handleSelectRun)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
	s.mux.HandleFunc("/", s.handleIndex)
	return s
}

// SetLayout injects a pre-computed DAG layout for a specific run.
// Used by main.go to restore layouts from DB after startup.
func (s *Server) SetLayout(runID string, layout *dag.Layout) {
	s.layoutsMu.Lock()
	s.layouts[runID] = layout
	s.layoutsMu.Unlock()
}

// discoverDAG looks for a dag.dot file in the same directory as the pipeline's
// scriptFile, parses it, and returns the computed layout. Returns nil if the
// file doesn't exist or can't be parsed (non-fatal — pipeline renders without DAG).
func discoverDAG(scriptFile string) (*dag.Layout, []byte) {
	dir := filepath.Dir(scriptFile)
	dotPath := filepath.Join(dir, "dag.dot")
	dotBytes, err := os.ReadFile(dotPath)
	if err != nil {
		return nil, nil
	}
	d, err := dag.ParseDOT(bytes.NewReader(dotBytes))
	if err != nil {
		log.Printf("warning: failed to parse %s: %v", dotPath, err)
		return nil, nil
	}
	return dag.ComputeLayout(d), dotBytes
}

// ServeHTTP delegates to the internal ServeMux, making Server an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handleWebhook decodes the Nextflow webhook JSON, updates state via store.HandleEvent,
// renders an HTML fragment for the updated run, and publishes it to SSE subscribers.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Read raw body to enable diagnostics logging, then decode.
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	// Persist raw event before any processing (log-and-continue on error).
	if s.eventStore != nil {
		if err := s.eventStore.Save(bodyBytes); err != nil {
			log.Printf("event persistence failed: %v", err)
		}
	}
	var event state.WebhookEvent
	if err := json.Unmarshal(bodyBytes, &event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	// Log raw trace JSON for completed events (resource debugging).
	if event.Event == "process_completed" {
		var raw map[string]json.RawMessage
		json.Unmarshal(bodyBytes, &raw)
		if traceRaw, ok := raw["trace"]; ok {
			log.Printf("webhook RAW trace: %s", string(traceRaw))
		}
	}

	s.store.HandleEvent(event)

	// Auto-discover DAG layout when a pipeline starts.
	if event.Event == "started" && event.Metadata != nil && event.Metadata.Workflow.ScriptFile != "" {
		projectName := event.Metadata.Workflow.ProjectName
		if event.Metadata.Workflow.Manifest.Name != "" {
			projectName = event.Metadata.Workflow.Manifest.Name
		}
		scriptFile := event.Metadata.Workflow.ScriptFile
		runID := event.RunID
		if layout, dotBytes := discoverDAG(scriptFile); layout != nil {
			s.layoutsMu.Lock()
			s.layouts[runID] = layout
			s.layoutsMu.Unlock()
			// Persist the raw DOT text so we can reload on restart.
			if s.eventStore != nil && dotBytes != nil {
				if err := s.eventStore.SaveDAG(runID, projectName, dotBytes); err != nil {
					log.Printf("DAG persistence failed for %q: %v", runID, err)
				}
			}
			log.Printf("DAG loaded for %q (%d processes) from %s",
				projectName, len(layout.Nodes), filepath.Join(filepath.Dir(scriptFile), "dag.dot"))
		} else {
			log.Printf("no dag.dot found for %q in %s — using list view",
				projectName, filepath.Dir(scriptFile))
		}
	}

	// Publish sidebar + dashboard updates to all SSE subscribers.
	// Broker carries pre-formatted SSE event strings.
	latestRunID := s.store.GetLatestRunID()
	sidebarSSE := formatSSEFragment(s.renderSidebar() + renderRunSelector(latestRunID))

	// Push dashboard update targeted to clients viewing this run.
	// Uses CSS attribute selector so clients viewing a different run skip it.
	var dashboardSSE string
	runID := event.RunID
	if runID != "" {
		if run := s.store.GetRun(runID); run != nil {
			run.RLock()
			content := s.renderRunDetail(run)
			run.RUnlock()
			dashDiv := fmt.Sprintf(`<div id="dashboard" data-run="%s">%s</div>`, runID, content)
			dashboardSSE = formatSSEFragmentWithSelector(dashDiv, fmt.Sprintf(`#dashboard[data-run="%s"]`, runID))
		}
	}

	s.broker.Publish(sidebarSSE + dashboardSSE)

	w.WriteHeader(http.StatusOK)
}



// handleDashboard serves one-shot text/html for GET /dashboard/{id}.
// Returns the dashboard content for the given run ID. The response includes
// a data-run attribute for CSS selector matching by SSE push events.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	run := s.store.GetRun(runID)
	if run == nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	run.RLock()
	content := s.renderRunDetail(run)
	run.RUnlock()

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div id="dashboard" data-run="%s">%s</div>`, runID, content)
}

// handleSelectRun serves GET /select-run/{id} — a one-shot SSE endpoint for run switching.
// Sends the initial run detail wrapped in a dashboard div with data-init (to start the
// persistent per-run SSE stream), patches the selectedRun signal, then closes.
// handleSelectRun is a one-shot SSE endpoint for GET /select-run/{id}.
// Returns dashboard HTML fragment + selectedRun signal patch.
// Uses text/event-stream because it needs to send both HTML and signal.
func (s *Server) handleSelectRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	runID := r.PathValue("id")
	run := s.store.GetRun(runID)
	if run == nil {
		fmt.Fprint(w, formatSSEFragment(`<div id="dashboard"><p class="error">Run not found</p></div>`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	run.RLock()
	detail := fmt.Sprintf(`<div id="dashboard" data-run="%s">%s</div>`, runID, s.renderRunDetail(run))
	run.RUnlock()
	fmt.Fprint(w, formatSSEFragment(detail))
	fmt.Fprint(w, formatSSESignals(fmt.Sprintf(`{selectedRun: '%s'}`, runID)))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// handleTaskLogs serves GET /sse/task/{run}/{task}/logs — returns one task's log content
// as a one-shot SSE fragment. Reads .command.log and .command.err from the task's workdir,
// truncated to last 50 lines. The response targets <div id="task-logs-{taskID}">.
func (s *Server) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run")
	taskIDStr := r.PathValue("task")

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	run := s.store.GetRun(runID)
	if run == nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	run.RLock()
	task, ok := run.Tasks[taskID]
	run.RUnlock()
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	var b strings.Builder
	fmt.Fprintf(&b, `<div id="task-logs-%d">`, taskID)

	if task.Workdir == "" {
		b.WriteString(`<div class="log-error-msg">workdir not set</div>`)
	} else {
		stdout, stdoutErr := readLogTail(filepath.Join(task.Workdir, ".command.log"), 50)
		stderr, stderrErr := readLogTail(filepath.Join(task.Workdir, ".command.err"), 50)
		writeLogSection(&b, ".command.log", stdout, stdoutErr)
		writeLogSection(&b, ".command.err", stderr, stderrErr)
	}

	b.WriteString(`</div>`)

	fmt.Fprint(w, formatSSEFragment(b.String()))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// handleSSE serves GET /sse/sidebar — streams sidebar updates to all browsers.
// On connect: sends sidebar HTML, an initial dashboard wrapper that triggers the
// per-run SSE connection for the latest run, and signal patches for selectedRun/latestRun.
// After initial send: streams only sidebar updates.
// handleSSE is a persistent SSE endpoint for sidebar push only.
// On connect: send sidebar HTML + latestRun signal.
// On webhook: send sidebar HTML only.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.broker.Subscribe()

	// Initial send: sidebar HTML + run-selector + latestRun signal.
	// The run-selector auto-triggers @get('/select-run/...') when $selectedRun
	// is empty, which loads the dashboard for the latest run.
	latestRunID := s.store.GetLatestRunID()
	sidebar := s.renderSidebar()
	selector := renderRunSelector(latestRunID)
	fmt.Fprint(w, formatSSEFragment(sidebar+selector))
	if latestRunID != "" {
		fmt.Fprint(w, formatSSESignals(fmt.Sprintf("{latestRun: '%s'}", latestRunID)))
	}
	flusher.Flush()

	// Stream broker messages until client disconnects.
	// Broker carries pre-formatted SSE event strings — write directly.
	for {
		select {
		case data := <-ch:
			fmt.Fprint(w, data)
			flusher.Flush()
		case <-r.Context().Done():
			s.broker.Unsubscribe(ch)
			return
		}
	}
}

// handleIndex serves the static web/index.html page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "failed to read index.html: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// groupProcesses groups tasks by Process name and returns per-group status counts
// with the actual Task objects attached. Tasks within each group are sorted:
// failed tasks first, then alphabetically by Name. Groups are sorted alphabetically.
func groupProcesses(tasks map[int]*state.Task) []ProcessGroup {
	groups := make(map[string]*ProcessGroup)
	for _, task := range tasks {
		g, ok := groups[task.Process]
		if !ok {
			g = &ProcessGroup{Name: task.Process}
			groups[task.Process] = g
		}
		g.Total++
		g.Tasks = append(g.Tasks, task)
		switch task.Status {
		case "COMPLETED":
			g.Completed++
		case "RUNNING":
			g.Running++
		case "FAILED":
			g.Failed++
		case "SUBMITTED":
			g.Submitted++
		}
	}
	result := make([]ProcessGroup, 0, len(groups))
	for _, g := range groups {
		// Sort tasks within group: FAILED first, then alphabetically by Name
		sort.Slice(g.Tasks, func(i, j int) bool {
			iFailed := g.Tasks[i].Status == "FAILED"
			jFailed := g.Tasks[j].Status == "FAILED"
			if iFailed != jFailed {
				return iFailed // failed sorts before non-failed
			}
			return g.Tasks[i].Name < g.Tasks[j].Name
		})
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// mergeDAGGroups ensures every process in the DAG layout appears in the group list,
// and sorts groups by topological order (DAG layer, then index) instead of alphabetical.
// Processes with no tasks yet get an empty ProcessGroup (0 total, no tasks).
// This ensures the process table shows all pipeline processes from the start.
func mergeDAGGroups(layout *dag.Layout, groups []ProcessGroup) []ProcessGroup {
	// Build a lookup from existing groups.
	byName := make(map[string]ProcessGroup, len(groups))
	for _, g := range groups {
		byName[g.Name] = g
	}
	// Build result in DAG node order (topological).
	result := make([]ProcessGroup, 0, len(layout.Nodes))
	for _, node := range layout.Nodes {
		if g, ok := byName[node.Name]; ok {
			result = append(result, g)
			delete(byName, node.Name)
		} else {
			result = append(result, ProcessGroup{Name: node.Name})
		}
	}
	// Append any groups not in the DAG (shouldn't happen, but be safe).
	for _, g := range groups {
		if _, ok := byName[g.Name]; ok {
			result = append(result, g)
		}
	}
	return result
}

// renderRunList renders the sidebar run list: a clickable list of all known runs.
// Each entry shows: pipeline name, run name, status badge, and start timestamp.
// Clicking a run sets $selectedRun signal to that run's ID.
// The currently active run (matching $selectedRun or $latestRun) is visually highlighted.
// Always renders (even for 0 or 1 run) since the sidebar is always visible.
// Runs are sorted by start time (newest first).
// Returns a <div id="run-list"> element for Datastar morph targeting.
func renderRunList(runs []*state.Run, latestRunID string) string {
	if len(runs) == 0 {
		return `<div id="run-list"></div>`
	}

	// Snapshot fields under per-run read lock to avoid races with HandleEvent.
	type runSnapshot struct {
		RunID, ProjectName, RunName, Status, StartTime string
	}
	snaps := make([]runSnapshot, len(runs))
	for i, run := range runs {
		run.RLock()
		snaps[i] = runSnapshot{
			RunID:       run.RunID,
			ProjectName: run.ProjectName,
			RunName:     run.RunName,
			Status:      run.Status,
			StartTime:   run.StartTime,
		}
		run.RUnlock()
	}

	// Sort by StartTime descending (newest first).
	sorted := make([]runSnapshot, len(snaps))
	copy(sorted, snaps)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartTime != sorted[j].StartTime {
			return sorted[i].StartTime > sorted[j].StartTime
		}
		return sorted[i].RunID < sorted[j].RunID // stable tiebreaker
	})

	var b strings.Builder
	b.WriteString(`<div id="run-list">`)

	for _, snap := range sorted {
		pipelineName := snap.ProjectName
		if pipelineName == "" {
			pipelineName = "Pipeline"
		}
		statusLower := strings.ToLower(snap.Status)

		b.WriteString(fmt.Sprintf(`<div class="run-entry" data-on:click="$selectedRun = '%s'; @get('/select-run/%s')" data-class:active="$selectedRun === '%s'">`,
			snap.RunID, snap.RunID, snap.RunID))
		b.WriteString(fmt.Sprintf(`<span class="run-pipeline">%s</span>`, pipelineName))
		b.WriteString(fmt.Sprintf(`<span class="run-name">%s</span>`, snap.RunName))
		b.WriteString(fmt.Sprintf(`<span class="badge status-%s">%s</span>`, statusLower, strings.ToUpper(snap.Status)))
		b.WriteString(fmt.Sprintf(`<span class="run-time">%s</span>`, formatRelativeTime(snap.StartTime, time.Now())))
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	return b.String()
}

// renderRunSelector returns a hidden div that conditionally triggers @get('/select-run/{latestRunID}')
// when $selectedRun is empty. This prevents sidebar SSE reconnects from re-triggering dashboard
// setup when the user already has a run selected.
func renderRunSelector(latestRunID string) string {
	if latestRunID == "" {
		return `<div id="run-selector" style="display:none"></div>`
	}
	return fmt.Sprintf(`<div id="run-selector" style="display:none" data-init="$selectedRun === '' && @get('/select-run/%s')"></div>`, latestRunID)
}

// renderDAG renders the DAG as absolutely-positioned HTML divs inside a relative container,
// with an SVG overlay for edge connectors. Each node shows: process name, status dot,
// task count "completed/total", and a mini progress bar. Status is derived from run.Tasks
// by matching task.Process to node.Name. When run is nil, all nodes show "pending".
// Returns HTML string with a top-level <div id="dag-view"> for Datastar morphing.
func renderDAG(layout *dag.Layout, run *state.Run) string {
	const (
		nodeWidth  = 180
		nodeHeight = 64
		spacingX   = 200
		spacingY   = 100
		paddingX   = 40
		paddingY   = 40
	)

	if layout == nil || len(layout.Nodes) == 0 {
		return `<div id="dag-view"></div>`
	}

	containerWidth := layout.MaxWidth*spacingX + paddingX*2 - (spacingX - nodeWidth)
	containerHeight := layout.LayerCount*spacingY + paddingY*2 - (spacingY - nodeHeight)

	// Count nodes per layer for centering
	layerCounts := make(map[int]int)
	for _, n := range layout.Nodes {
		layerCounts[n.Layer]++
	}

	// Build name→NodeLayout map for edge position lookup
	nodeMap := make(map[string]dag.NodeLayout, len(layout.Nodes))
	for _, n := range layout.Nodes {
		nodeMap[n.Name] = n
	}

	// Compute node positions
	type nodePos struct {
		x, y int
	}
	positions := make(map[string]nodePos, len(layout.Nodes))
	for _, n := range layout.Nodes {
		nodesInLayer := layerCounts[n.Layer]
		layerWidth := nodesInLayer*spacingX - (spacingX - nodeWidth)
		offset := (containerWidth - paddingX*2 - layerWidth) / 2
		x := paddingX + offset + n.Index*spacingX
		y := paddingY + n.Layer*spacingY
		positions[n.Name] = nodePos{x, y}
	}

	// Build neighbor map for interactive highlighting
	neighbors := make(map[string][]string)
	for _, e := range layout.Edges {
		neighbors[e.From] = append(neighbors[e.From], e.To)
		neighbors[e.To] = append(neighbors[e.To], e.From)
	}

	// Derive status and counts per process from run.Tasks
	type processStats struct {
		total     int
		completed int
		running   int
		failed    int
	}
	stats := make(map[string]*processStats)
	if run != nil {
		for _, task := range run.Tasks {
			s, ok := stats[task.Process]
			if !ok {
				s = &processStats{}
				stats[task.Process] = s
			}
			s.total++
			switch task.Status {
			case "COMPLETED":
				s.completed++
			case "RUNNING":
				s.running++
			case "FAILED":
				s.failed++
			}
		}
	}

	deriveStatus := func(name string) (string, int, int) {
		s := stats[name]
		if s == nil || s.total == 0 {
			return "pending", 0, 0
		}
		var status string
		if s.failed > 0 {
			status = "failed"
		} else if s.running > 0 {
			status = "running"
		} else if s.completed == s.total {
			status = "completed"
		} else {
			status = "submitted"
		}
		return status, s.completed, s.total
	}

	var b strings.Builder
	b.WriteString(`<div id="dag-view">`)
	b.WriteString(`<div class="dag-scroll-container">`)
	b.WriteString(fmt.Sprintf(`<div style="position:relative;width:%dpx;height:%dpx;">`, containerWidth, containerHeight))

	// Render nodes
	for _, n := range layout.Nodes {
		pos := positions[n.Name]
		status, completed, total := deriveStatus(n.Name)
		pct := 0
		if total > 0 {
			pct = completed * 100 / total
		}

		// Build JS neighbor array literal
		neighborList := neighbors[n.Name]
		jsNeighbors := "[]"
		if len(neighborList) > 0 {
			parts := make([]string, len(neighborList))
			for i, nb := range neighborList {
				parts[i] = fmt.Sprintf("'%s'", nb)
			}
			jsNeighbors = "[" + strings.Join(parts, ",") + "]"
		}

		b.WriteString(fmt.Sprintf(
			`<div class="dag-node status-%s" style="left:%dpx;top:%dpx;width:%dpx;height:%dpx;" `+
				`data-on:mouseenter="$_dagHL = '%s'" `+
				`data-on:mouseleave="$_dagHL = ''" `+
				`data-on:click="$expandedGroup = $expandedGroup === '%s' ? '' : '%s'; setTimeout(()=>document.getElementById('process-group-%s')?.scrollIntoView({behavior:'smooth',block:'nearest'}),50)" `+
				`data-class:dag-faded="dagShouldFade($_dagHL, '%s', %s)" `+
				`data-class:dag-node-selected="$expandedGroup === '%s'">`,
			status, pos.x, pos.y, nodeWidth, nodeHeight,
			n.Name,
			n.Name, n.Name, n.Name,
			n.Name, jsNeighbors,
			n.Name,
		))
		b.WriteString(fmt.Sprintf(`<span class="dag-node-name">%s</span>`, n.Name))
		b.WriteString(fmt.Sprintf(`<span class="dag-node-counts">%d/%d</span>`, completed, total))
		b.WriteString(fmt.Sprintf(`<div class="dag-node-bar"><div class="dag-node-fill" style="width:%d%%"></div></div>`, pct))
		b.WriteString(`</div>`)
	}

	// Render SVG edges
	if len(layout.Edges) > 0 {
		b.WriteString(fmt.Sprintf(
			`<svg class="dag-svg" style="position:absolute;left:0;top:0;" width="%d" height="%d">`,
			containerWidth, containerHeight,
		))
		b.WriteString(`<defs><marker id="arrowhead" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 Z" fill="var(--text-muted)" /></marker></defs>`)
		for _, e := range layout.Edges {
			srcPos := positions[e.From]
			tgtPos := positions[e.To]
			sx := srcPos.x + nodeWidth/2
			sy := srcPos.y + nodeHeight
			tx := tgtPos.x + nodeWidth/2
			ty := tgtPos.y
			cy1 := sy + (ty-sy)/2
			cy2 := ty - (ty-sy)/2
			b.WriteString(fmt.Sprintf(
				`<path class="dag-edge" d="M%d,%d C%d,%d %d,%d %d,%d" marker-end="url(#arrowhead)" `+
					`style="pointer-events:visibleStroke;cursor:pointer" `+
					`data-on:mouseenter="$_dagHL = '%s>%s'" `+
					`data-on:mouseleave="$_dagHL = ''" `+
					`data-class:dag-edge-faded="dagEdgeFade($_dagHL, '%s', '%s')" />`,
				sx, sy, sx, cy1, tx, cy2, tx, ty,
				e.From, e.To,
				e.From, e.To,
			))
		}
		b.WriteString(`</svg>`)
	}

	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	return b.String()
}

// renderDAGTaskPanel renders a task panel below the DAG view, showing expandable task details
// for the process selected by clicking a DAG node. Each process gets a section controlled by
// renderRunDetail renders the complete detail panel for a single pipeline run: header
// (pipeline name, run name, status badge), progress bar, process group list with expandable
// task details, and run summary (duration, task counts, peak memory, error message).
// The run summary (renderRunSummary) is included at the end of the detail panel.
func (s *Server) renderRunDetail(run *state.Run) string {
	if run == nil {
		return ""
	}
	var b strings.Builder

	pipelineName := run.ProjectName
	if pipelineName == "" {
		pipelineName = "Pipeline"
	}
	statusLower := strings.ToLower(run.Status)

	// Run header
	b.WriteString(`<div class="run-header">`)
	b.WriteString(fmt.Sprintf(`<h1>%s</h1>`, pipelineName))
	b.WriteString(fmt.Sprintf(`<span class="badge status-%s">%s</span>`, statusLower, strings.ToUpper(run.Status)))
	b.WriteString(fmt.Sprintf(`<span class="run-name">%s</span>`, run.RunName))
	// Elapsed timer: live for running runs (uses $tick to re-evaluate each second),
	// frozen for completed runs, hidden if no start time yet.
	// Timestamps are embedded as literals — no global signals — so each run's timer
	// is independent and switching runs shows the correct value.
	if run.StartTime != "" {
		if run.CompleteTime != "" {
			// Completed: show static frozen duration
			b.WriteString(fmt.Sprintf(
				`<span class="elapsed-time" data-text="formatElapsed('%s', '%s')"></span>`,
				run.StartTime, run.CompleteTime))
		} else {
			// Running: live timer, re-evaluated each second via $tick dependency
			b.WriteString(fmt.Sprintf(
				`<span class="elapsed-time" data-text="$tick >= 0 ? formatElapsed('%s', '') : ''"></span>`,
				run.StartTime))
		}
	}
	b.WriteString(`</div>`)

	// Progress bar
	completed := 0
	total := len(run.Tasks)
	for _, task := range run.Tasks {
		if task.Status == "COMPLETED" {
			completed++
		}
	}
	pct := 0
	if total > 0 {
		pct = completed * 100 / total
	}

	fillClass := "progress-fill"
	if statusLower == "completed" {
		fillClass += " status-completed"
	} else if statusLower == "failed" {
		fillClass += " status-failed"
	}

	b.WriteString(`<div class="progress-bar">`)
	b.WriteString(fmt.Sprintf(`<div class="%s" style="width: %d%%"></div>`, fillClass, pct))
	b.WriteString(fmt.Sprintf(`<span class="progress-label">%d/%d (%d%%)</span>`, completed, total, pct))
	b.WriteString(`</div>`)

	groups := groupProcesses(run.Tasks)

	// DAG topology (when available)
	s.layoutsMu.RLock()
	layout := s.layouts[run.RunID]
	s.layoutsMu.RUnlock()
	if layout != nil {
		b.WriteString(renderDAG(layout, run))
		// Pre-seed process table with all DAG nodes (so processes appear before
		// any tasks arrive) and sort by topological order instead of alphabetical.
		groups = mergeDAGGroups(layout, groups)
	}

	// Unified process table (always rendered, whether DAG is present or not)
	b.WriteString(renderProcessTable(groups, run.RunID))

	// Run summary (duration, task counts, peak memory, error message)
	b.WriteString(renderRunSummary(run))

	// Resume command (for completed/failed runs with session ID)
	b.WriteString(renderResumeCommand(run))

	// Samplesheet viewer (when params["input"] points to an accessible file)
	b.WriteString(renderSamplesheet(run))

	return b.String()
}

// renderRunSummary renders a summary card for a completed or errored run.
// Shows: total execution time (from StartTime to CompleteTime), task counts by status
// (completed, failed, running, submitted), peak memory (max PeakRSS across all tasks),
// and error message (if run.ErrorMessage is non-empty).
// Returns empty string if the run is still in progress (status "running").
func renderRunSummary(run *state.Run) string {
	if run == nil || run.Status == "" || run.Status == "running" {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<div class="run-summary">`)
	b.WriteString(`<div class="detail-grid">`)

	// Duration
	duration := computeRunDuration(run.StartTime, run.CompleteTime)
	b.WriteString(`<span class="detail-label">Duration</span>`)
	fmt.Fprintf(&b, `<span class="detail-value">%s</span>`, duration)

	// Task counts by status
	completed, failed, running, submitted := 0, 0, 0, 0
	var peakRSS int64
	for _, task := range run.Tasks {
		switch task.Status {
		case "COMPLETED":
			completed++
		case "FAILED":
			failed++
		case "RUNNING":
			running++
		case "SUBMITTED":
			submitted++
		}
		if task.PeakRSS > peakRSS {
			peakRSS = task.PeakRSS
		}
	}

	var parts []string
	if completed > 0 {
		parts = append(parts, fmt.Sprintf("%d completed", completed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d running", running))
	}
	if submitted > 0 {
		parts = append(parts, fmt.Sprintf("%d submitted", submitted))
	}
	taskSummary := strings.Join(parts, ", ")

	b.WriteString(`<span class="detail-label">Tasks</span>`)
	fmt.Fprintf(&b, `<span class="detail-value">%s</span>`, taskSummary)

	// Peak memory
	b.WriteString(`<span class="detail-label">Peak Memory</span>`)
	fmt.Fprintf(&b, `<span class="detail-value">%s</span>`, formatBytes(peakRSS))

	b.WriteString(`</div>`) // close detail-grid

	// Error message (if present)
	if run.ErrorMessage != "" {
		fmt.Fprintf(&b, `<div class="error-message">%s</div>`, run.ErrorMessage)
	}

	b.WriteString(`</div>`) // close run-summary
	return b.String()
}

// renderSidebar renders the sidebar run-list fragment.
func (s *Server) renderSidebar() string {
	runs := s.store.GetAllRuns()
	latestRunID := s.store.GetLatestRunID()
	return renderRunList(runs, latestRunID)
}

// computeRunDuration calculates the wall-clock duration between two UTC timestamp strings
// (ISO 8601 format, e.g. "2024-01-15T10:30:00Z") and returns a human-readable duration.
// Used by renderRunSummary to show total pipeline execution time.
// Returns "—" if either timestamp is empty or unparseable.
func computeRunDuration(startTime, completeTime string) string {
	if startTime == "" || completeTime == "" {
		return "—"
	}
	start, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return "—"
	}
	end, err := time.Parse(time.RFC3339, completeTime)
	if err != nil {
		return "—"
	}
	millis := end.Sub(start).Milliseconds()
	return formatDuration(millis)
}

// barWidth computes the percentage width for a resource bar.
// Returns 0 for non-positive values, otherwise the value/colMax ratio as a
// percentage clamped to a 5 % floor so tiny-but-nonzero bars remain visible.
func barWidth(value, colMax float64) int {
	if value <= 0 || colMax <= 0 {
		return 0
	}
	pct := int(value / colMax * 100)
	if pct < 5 {
		pct = 5
	}
	return pct
}

// writeBarCell writes one bar-chart cell (bar + label) into b.
// pct is the bar width percentage (0–100); label is the text shown to the right.
func writeBarCell(b *strings.Builder, pct int, label string) {
	b.WriteString(`<div class="resource-bar-cell">`)
	fmt.Fprintf(b, `<div class="resource-bar"><div class="resource-bar-fill" style="width: %d%%"></div></div>`, pct)
	fmt.Fprintf(b, `<span class="resource-bar-label">%s</span>`, label)
	b.WriteString(`</div>`)
}

// resourceMetrics holds the three resource values used for bar-chart rendering.
// For a single task the fields hold that task's values; for a group they hold
// the per-group maxima. hasData is false when no COMPLETED/FAILED data exists.
type resourceMetrics struct {
	duration int64
	cpu      float64
	peakRSS  int64
	hasData  bool
}

// computeResourceMetrics scans tasks and returns the column-wide maxima of
// duration, CPU%, and peak RSS, considering only COMPLETED or FAILED tasks.
func computeResourceMetrics(tasks []*state.Task) resourceMetrics {
	var m resourceMetrics
	for _, t := range tasks {
		if t.Status != "COMPLETED" && t.Status != "FAILED" {
			continue
		}
		m.hasData = true
		if t.Duration > m.duration {
			m.duration = t.Duration
		}
		if t.CPUPercent > m.cpu {
			m.cpu = t.CPUPercent
		}
		if t.PeakRSS > m.peakRSS {
			m.peakRSS = t.PeakRSS
		}
	}
	return m
}

// writeResourceBars writes the three resource bar cells (duration, cpu, memory)
// into b. val holds the values to display; colMax holds the column-wide maxima
// used to scale the bars. When val.hasData is false, placeholder dots are shown.
func writeResourceBars(b *strings.Builder, val, colMax resourceMetrics) {
	if !val.hasData {
		for range 3 {
			writeBarCell(b, 0, "···")
		}
		return
	}
	writeBarCell(b, barWidth(float64(val.duration), float64(colMax.duration)), formatDuration(val.duration))
	writeBarCell(b, barWidth(val.cpu, colMax.cpu), fmt.Sprintf("%.0f%%", val.cpu))
	writeBarCell(b, barWidth(float64(val.peakRSS), float64(colMax.peakRSS)), formatBytes(val.peakRSS))
}

// formatDuration converts milliseconds to a human-readable duration string.
// Examples: 0 → "0s", 3800 → "3.8s", 135000 → "2m 15s", 3780000 → "1h 3m".
// Rules: <1s show ms, <60s show seconds with one decimal, <1h show Xm Ys, ≥1h show Xh Ym.
func formatDuration(millis int64) string {
	if millis == 0 {
		return "0s"
	}
	if millis < 1000 {
		return fmt.Sprintf("%dms", millis)
	}
	if millis < 60000 {
		tenths := millis / 100
		return fmt.Sprintf("%d.%ds", tenths/10, tenths%10)
	}
	if millis < 3600000 {
		totalSec := millis / 1000
		m := totalSec / 60
		s := totalSec % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	totalMin := millis / 60000
	h := totalMin / 60
	m := totalMin % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatBytes converts a byte count to a human-readable string with appropriate unit.
// Uses binary units: B, KB, MB, GB. One decimal place for KB/MB/GB.
// Examples: 0 → "0 B", 1024 → "1.0 KB", 10485760 → "10.0 MB", 1073741824 → "1.0 GB".
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes < kb:
		return fmt.Sprintf("%d B", bytes)
	case bytes < mb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	case bytes < gb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	default:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	}
}

// formatTimestamp converts epoch milliseconds to a human-readable local timestamp string.
// Format: "2024-01-15 15:30:01 EST". Returns "—" for zero/negative values (not yet set).
func formatTimestamp(epochMillis int64) string {
	if epochMillis <= 0 {
		return "—"
	}
	return time.UnixMilli(epochMillis).Local().Format("2006-01-02 15:04:05 MST")
}

// formatRelativeTime converts an ISO 8601 timestamp to a human-readable relative string.
// Uses `now` parameter for testability (production callers pass time.Now()).
// Examples: "just now", "3m ago", "2h ago", "Mar 24, 15:04".
// Returns the raw timestamp on parse error, empty string for empty input.
func formatRelativeTime(isoTimestamp string, now time.Time) string {
	if isoTimestamp == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, isoTimestamp)
	if err != nil {
		return isoTimestamp
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Local().Format("Jan 2, 15:04")
	}
}

// formatSSEFragment formats an HTML fragment string for Datastar v1 SSE wire format.
// Multi-line HTML: each line gets its own "data: elements " prefix.
// Returns the full SSE event string: "event: datastar-patch-elements\ndata: elements ...\n\n"
func formatSSEFragment(html string) string {
	var b strings.Builder
	b.WriteString("event: datastar-patch-elements\n")
	for _, line := range strings.Split(html, "\n") {
		b.WriteString("data: elements ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

// formatSSEFragmentWithSelector formats an HTML fragment as a Datastar SSE event
// that targets a specific element via CSS selector. Used to push dashboard updates
// only to clients viewing a specific run (via data-run attribute matching).
func formatSSEFragmentWithSelector(html, selector string) string {
	var b strings.Builder
	b.WriteString("event: datastar-patch-elements\n")
	b.WriteString("data: selector ")
	b.WriteString(selector)
	b.WriteByte('\n')
	for _, line := range strings.Split(html, "\n") {
		b.WriteString("data: elements ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

// formatSSESignals formats a signal patch for Datastar v1 SSE wire format.
// Takes a JSON-encoded signal map string.
// Returns: "event: datastar-patch-signals\ndata: signals {\"key\": value}\n\n"
func formatSSESignals(jsonSignals string) string {
	return "event: datastar-patch-signals\ndata: signals " + jsonSignals + "\n\n"
}

// readLogTail reads the last maxLines lines of a file.
// Returns empty string + nil if the file doesn't exist (os.IsNotExist).
// Returns error for permission/IO issues.
// When maxLines <= 0 or file has fewer lines, returns full content.
// writeLogSection renders one log section (e.g. .command.log or .command.err) into the builder.
// Shows the content if available, "(no log file)" if empty, or the error message on read failure.
func writeLogSection(b *strings.Builder, label string, content string, err error) {
	b.WriteString(`<div class="log-section">`)
	fmt.Fprintf(b, `<div class="log-section-label">%s</div>`, label)
	if err != nil {
		fmt.Fprintf(b, `<div class="log-error-msg">%s</div>`, html.EscapeString(err.Error()))
	} else if content == "" {
		b.WriteString(`<div class="log-content"><span style="color:var(--text-muted)">(no log file)</span></div>`)
	} else {
		fmt.Fprintf(b, `<div class="log-content">%s</div>`, html.EscapeString(content))
	}
	b.WriteString(`</div>`)
}

func readLogTail(path string, maxLines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	content := string(data)
	if maxLines <= 0 || content == "" {
		return content, nil
	}

	// Split into lines preserving trailing newline awareness.
	// We work backward from the end to find the last maxLines lines.
	lines := strings.SplitAfter(content, "\n")
	// SplitAfter produces an empty trailing element if content ends with "\n"
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= maxLines {
		return content, nil
	}
	return strings.Join(lines[len(lines)-maxLines:], ""), nil
}

// renderTaskTable renders a unified task table where each task gets one row with
// resource bars AND click-to-expand detail. Replaces the former separate chart +
// task list pattern.
func renderTaskTable(processName string, tasks []*state.Task, runID string) string {
	if len(tasks) == 0 {
		return ""
	}

	colMax := computeResourceMetrics(tasks)

	// taskLabel strips the process name prefix, returning just the parenthetical part.
	// e.g. "sayHello (1)" → "(1)". If no parens, returns the full name.
	taskLabel := func(name string) string {
		idx := strings.Index(name, "(")
		if idx < 0 {
			return name
		}
		return strings.TrimSpace(name[idx:])
	}

	var b strings.Builder
	b.WriteString(`<div class="task-table">`)

	// Header row
	b.WriteString(`<div class="task-table-header">`)
	b.WriteString(`<span class="resource-col-label"></span>`)
	b.WriteString(`<span class="resource-col-label"></span>`)
	b.WriteString(`<span class="resource-col-label">duration</span>`)
	b.WriteString(`<span class="resource-col-label">cpu</span>`)
	b.WriteString(`<span class="resource-col-label">memory</span>`)
	b.WriteString(`</div>`)

	for _, t := range tasks {
		isCompleted := t.Status == "COMPLETED" || t.Status == "FAILED"
		isFailed := t.Status == "FAILED"

		// Task row
		rowClass := "task-table-row"
		if isFailed {
			rowClass = "task-table-row failed"
		}
		fmt.Fprintf(&b, `<div class="%s" data-on:click__stop="$expandedTask = $expandedTask === %d ? 0 : %d">`, rowClass, t.TaskID, t.TaskID)

		// Chevron
		fmt.Fprintf(&b, `<span class="chevron" data-class:expanded="$expandedTask === %d">▶</span>`, t.TaskID)

		// Task name (stripped)
		fmt.Fprintf(&b, `<span class="task-table-name">%s</span>`, taskLabel(t.Name))

		// Status badge
		fmt.Fprintf(&b, `<span class="badge status-%s">%s</span>`, strings.ToLower(t.Status), t.Status)

		// Resource bars
		taskVal := resourceMetrics{
			duration: t.Duration,
			cpu:      t.CPUPercent,
			peakRSS:  t.PeakRSS,
			hasData:  isCompleted,
		}
		writeResourceBars(&b, taskVal, colMax)

		b.WriteString(`</div>`) // close task-table-row

		// Detail panel
		fmt.Fprintf(&b, `<div class="task-detail" data-show="$expandedTask === %d">`, t.TaskID)
		b.WriteString(`<div class="detail-grid">`)

		// Exit Code
		exitClass := "detail-value"
		if t.Exit != 0 {
			exitClass = "detail-value exit-error"
		}
		fmt.Fprintf(&b, `<span class="detail-label">Exit Code</span><span class="%s">%d</span>`, exitClass, t.Exit)

		// Work Dir
		fmt.Fprintf(&b, `<span class="detail-label">Work Dir</span><span class="detail-value workdir">%s</span>`, t.Workdir)

		// Timestamps
		fmt.Fprintf(&b, `<span class="detail-label">Submitted</span><span class="detail-value">%s</span>`, formatTimestamp(t.Submit))
		fmt.Fprintf(&b, `<span class="detail-label">Started</span><span class="detail-value">%s</span>`, formatTimestamp(t.Start))
		fmt.Fprintf(&b, `<span class="detail-label">Completed</span><span class="detail-value">%s</span>`, formatTimestamp(t.Complete))

		b.WriteString(`</div>`) // close detail-grid

		// Inline log sections (read from task workdir on every render for live updates)
		if t.Workdir != "" {
			stdout, stdoutErr := readLogTail(filepath.Join(t.Workdir, ".command.log"), 50)
			stderr, stderrErr := readLogTail(filepath.Join(t.Workdir, ".command.err"), 50)
			writeLogSection(&b, ".command.log", stdout, stdoutErr)
			writeLogSection(&b, ".command.err", stderr, stderrErr)
		}

		b.WriteString(`</div>`) // close task-detail
	}

	b.WriteString(`</div>`) // close task-table
	return b.String()
}

// renderProcessTable renders a unified process table with expandable task rows.
// Each process is a row with resource bars (max duration, max CPU%, max peak memory)
// that is clickable to expand a nested task table via renderTaskTable.
// renderProcessTable renders the process group table with expandable task panels.
// Task panels use data-on-interval for 1s polling via GET /tasks/{runID}/{process}.
// Click handler on process row toggles $expandedGroup signal AND triggers immediate fetch.
func renderProcessTable(groups []ProcessGroup, runID string) string {
	if len(groups) == 0 {
		return ""
	}

	// Step 1: Compute per-group max metrics from completed/failed tasks only.
	metrics := make([]resourceMetrics, len(groups))
	for i, g := range groups {
		metrics[i] = computeResourceMetrics(g.Tasks)
	}

	// Step 2: Find column-wide max across all groups.
	var colMax resourceMetrics
	for _, m := range metrics {
		if !m.hasData {
			continue
		}
		colMax.hasData = true
		if m.duration > colMax.duration {
			colMax.duration = m.duration
		}
		if m.cpu > colMax.cpu {
			colMax.cpu = m.cpu
		}
		if m.peakRSS > colMax.peakRSS {
			colMax.peakRSS = m.peakRSS
		}
	}

	// Step 3: Render HTML.
	var b strings.Builder
	b.WriteString(`<div class="process-table">`)

	// Header row: 6 columns (chevron, name, status+count, duration, cpu, memory)
	b.WriteString(`<div class="process-table-header">`)
	b.WriteString(`<span class="resource-col-label"></span>`)
	b.WriteString(`<span class="resource-col-label"></span>`)
	b.WriteString(`<span class="resource-col-label"></span>`)
	b.WriteString(`<span class="resource-col-label">duration</span>`)
	b.WriteString(`<span class="resource-col-label">cpu</span>`)
	b.WriteString(`<span class="resource-col-label">memory</span>`)
	b.WriteString(`</div>`)

	// Process group rows
	for i, g := range groups {
		m := metrics[i]

		// Group container with status highlight classes
		groupClass := "process-table-group"
		if g.Failed > 0 {
			groupClass += " group-has-failed"
		}
		if g.Running > 0 {
			groupClass += " group-has-running"
		}
		fmt.Fprintf(&b, `<div class="%s" id="process-group-%s">`, groupClass, g.Name)

		// Clickable process row — toggle expandedGroup AND immediately fetch tasks
		fmt.Fprintf(&b, `<div class="process-table-row" data-on:click="$expandedGroup = $expandedGroup === '%s' ? '' : '%s'; $expandedGroup === '%s' && @get('/tasks/%s/%s')">`, g.Name, g.Name, g.Name, runID, g.Name)

		// Chevron with expanded class binding
		fmt.Fprintf(&b, `<span class="chevron" data-class:expanded="$expandedGroup === '%s'">▶</span>`, g.Name)

		// Process name
		fmt.Fprintf(&b, `<span class="process-table-name">%s</span>`, g.Name)

		// Status dot + task count (combined in one grid cell)
		b.WriteString(`<span class="process-table-info">`)
		b.WriteString(renderGroupStatusDot(g))
		fmt.Fprintf(&b, `<span class="process-table-counts">%d/%d</span>`, g.Completed, g.Total)
		b.WriteString(`</span>`)

		// Resource bars
		writeResourceBars(&b, m, colMax)

		b.WriteString(`</div>`) // close process-table-row

		// Expandable task section (hidden by default)
		fmt.Fprintf(&b, `<div class="process-table-tasks" data-show="$expandedGroup === '%s'" style="display: none">`, g.Name)
		fmt.Fprintf(&b, `<div id="task-panel-%s" data-ignore-morph></div>`, g.Name)
		b.WriteString(`</div>`) // close process-table-tasks

		b.WriteString(`</div>`) // close process-table-group
	}

	b.WriteString(`</div>`) // close process-table
	return b.String()
}

// buildResumeCommand reconstructs a `nextflow run ... -resume` command from stored run metadata.
// tokenizeCommandLine splits a shell command string into tokens, respecting
// single-quoted strings (no escapes except '\''), double-quoted strings
// (backslash-escape \" and \\), backslash escapes in unquoted context, and
// whitespace delimiters. Returns empty slice for empty/whitespace-only input.
func tokenizeCommandLine(s string) []string {
	var tokens []string
	var cur strings.Builder
	inToken := false

	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\'':
			// Single-quoted string: consume until closing '
			inToken = true
			i++ // skip opening quote
			for i < len(s) && s[i] != '\'' {
				cur.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++ // skip closing quote
			}
		case c == '"':
			// Double-quoted string: backslash escapes \" and \\
			inToken = true
			i++ // skip opening quote
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
					cur.WriteByte(s[i+1])
					i += 2
				} else {
					cur.WriteByte(s[i])
					i++
				}
			}
			if i < len(s) {
				i++ // skip closing quote
			}
		case c == '\\' && i+1 < len(s):
			// Backslash escape in unquoted context
			inToken = true
			cur.WriteByte(s[i+1])
			i += 2
		case c == ' ' || c == '\t':
			// Whitespace: end current token
			if inToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inToken = false
			}
			i++
		default:
			inToken = true
			cur.WriteByte(c)
			i++
		}
	}
	if inToken {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// parseRuntimeFlags extracts Nextflow runtime flags (like -profile, -with-weblog, -c)
// from a tokenized command line, skipping the "nextflow run <project>" prefix,
// pipeline params (--key value), and -resume. Returns nil if tokens has < 3 elements.
func parseRuntimeFlags(tokens []string, params map[string]any) []string {
	if len(tokens) < 3 {
		return nil
	}

	var flags []string
	i := 3 // skip tokens[0]="nextflow", tokens[1]="run", tokens[2]=project
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasPrefix(tok, "--") {
			// Pipeline param: skip flag and its value
			i++ // skip --key
			if i < len(tokens) {
				i++ // skip value
			}
		} else if tok == "-resume" {
			// Skip -resume and its optional value (non-flag next token)
			i++ // skip -resume
			if i < len(tokens) && !strings.HasPrefix(tokens[i], "-") {
				i++ // skip session ID value
			}
		} else {
			// Runtime flag: keep it
			flags = append(flags, tok)
			i++
			// If next token is a value (doesn't start with -), keep it too
			if i < len(tokens) && !strings.HasPrefix(tokens[i], "-") {
				flags = append(flags, tokens[i])
				i++
			}
		}
	}
	return flags
}

// isLocalPipeline returns true when scriptFile is an absolute path NOT under
// ~/.nextflow/assets/ (i.e. a local pipeline run, not a pulled remote pipeline).
func isLocalPipeline(scriptFile string) bool {
	if !filepath.IsAbs(scriptFile) {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	assetsPrefix := filepath.Join(home, ".nextflow", "assets") + string(filepath.Separator)
	if strings.HasPrefix(scriptFile, assetsPrefix) || scriptFile == filepath.Join(home, ".nextflow", "assets") {
		return false
	}
	return true
}

// pipelineRef returns the pipeline reference to use in a resume command.
// For local pipelines (isLocalPipeline), returns the directory containing the script file.
// For remote pipelines, returns the project name.
func pipelineRef(run *state.Run) string {
	if run.ScriptFile != "" && isLocalPipeline(run.ScriptFile) {
		return filepath.Dir(run.ScriptFile)
	}
	return run.ProjectName
}

// parseSamplesheetCSV parses CSV content and returns headers and data rows.
// Returns error if content is empty or has no header row.
func parseSamplesheetCSV(content string) ([]string, [][]string, error) {
	if strings.TrimSpace(content) == "" {
		return nil, nil, fmt.Errorf("empty samplesheet content")
	}
	records, err := csv.NewReader(strings.NewReader(content)).ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("no records in samplesheet")
	}
	return records[0], records[1:], nil
}

// buildResumeCommand reconstructs a `nextflow run ... -resume <sessionID>` command.
// When run.CommandLine is available, it tokenizes the original command to extract
// runtime flags (like -profile, -with-weblog, -c) and includes them in the resume
// command. When CommandLine is empty, falls back to params + -work-dir + -resume.
// Returns empty string if SessionID or ProjectName is empty.
// writeParams appends sorted --key value flags for each pipeline parameter.
func writeParams(b *strings.Builder, params map[string]any) {
	if len(params) == 0 {
		return
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(" --")
		b.WriteString(k)
		b.WriteString(" ")
		b.WriteString(shellQuoteValue(params[k]))
	}
}

func buildResumeCommand(run *state.Run) string {
	if run.SessionID == "" || run.ProjectName == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("nextflow run ")
	b.WriteString(shellQuoteValue(pipelineRef(run)))

	if run.CommandLine != "" {
		// Full reconstruction from original command line
		tokens := tokenizeCommandLine(run.CommandLine)
		runtimeFlags := parseRuntimeFlags(tokens, run.Params)

		writeParams(&b, run.Params)

		// Append runtime flags, quoting values (non-flag tokens)
		for _, f := range runtimeFlags {
			b.WriteString(" ")
			if strings.HasPrefix(f, "-") {
				b.WriteString(f)
			} else {
				b.WriteString(shellQuoteValue(f))
			}
		}

		// Inject -work-dir if not already present in runtime flags
		if run.WorkDir != "" {
			hasWorkDir := false
			for _, f := range runtimeFlags {
				if f == "-work-dir" {
					hasWorkDir = true
					break
				}
			}
			if !hasWorkDir {
				b.WriteString(" -work-dir ")
				b.WriteString(shellQuoteValue(run.WorkDir))
			}
		}
	} else {
		// Fallback: params + -work-dir
		writeParams(&b, run.Params)

		b.WriteString(" -work-dir ")
		b.WriteString(shellQuoteValue(run.WorkDir))
	}

	b.WriteString(" -resume ")
	b.WriteString(run.SessionID)

	return b.String()
}

// shellQuoteValue formats a param value as a string and wraps it in single quotes
// if it contains shell-special characters. Internal single quotes are escaped as '\''.
func shellQuoteValue(val any) string {
	s := fmt.Sprintf("%v", val)
	if s == "" {
		return s
	}
	const specialChars = " \t'\"`$\\|&;()<>*?[]{}~!#^"
	needsQuoting := false
	for _, c := range s {
		if strings.ContainsRune(specialChars, c) {
			needsQuoting = true
			break
		}
	}
	if !needsQuoting {
		return s
	}
	// Wrap in single quotes, escaping internal single quotes as '\''
	var b strings.Builder
	b.WriteByte('\'')
	for _, c := range s {
		if c == '\'' {
			b.WriteString("'\\''")
		} else {
			b.WriteRune(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// renderResumeCommand renders an HTML section with the reconstructed resume command and a
// copy-to-clipboard button. Only renders for runs with status "failed" or "completed" that
// have a non-empty SessionID. Contains a <pre> with the command text and a button using
// navigator.clipboard.writeText. Returns empty string if conditions not met.
func renderResumeCommand(run *state.Run) string {
	if run == nil {
		return ""
	}
	if run.Status != "failed" && run.Status != "completed" {
		return ""
	}
	if run.SessionID == "" {
		return ""
	}

	cmd := buildResumeCommand(run)
	if cmd == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<div class="resume-command" data-signals:_show-resume="false">`)
	b.WriteString(`<div class="resume-command-header" data-on:click="$_showResume = !$_showResume">`)
	b.WriteString(`<span class="chevron" data-class:expanded="$_showResume">▶</span>`)
	b.WriteString(`<span class="detail-label">Resume Command</span>`)
	b.WriteString(`<button class="btn-copy" data-on:click__stop="copyText(evt.target, document.getElementById('resume-cmd').textContent)">Copy</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div data-show="$_showResume" style="display: none">`)
	b.WriteString(`<pre id="resume-cmd" class="resume-cmd-text">`)
	b.WriteString(html.EscapeString(cmd))
	b.WriteString(`</pre>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	return b.String()
}

// renderSamplesheet checks if run.Params has an "input" key; if so, reads the file at that
// path using readLogTail (unlimited lines). Renders an editable textarea with the file contents
// and a copy-to-clipboard button. The container has data-ignore-morph so user edits survive
// SSE re-renders. Returns empty string if params["input"] is missing or the file is unreadable.
func renderSamplesheet(run *state.Run) string {
	if run == nil || run.Params == nil {
		return ""
	}
	inputVal, ok := run.Params["input"]
	if !ok {
		return ""
	}
	path, ok := inputVal.(string)
	if !ok || path == "" {
		return ""
	}

	if _, err := os.Stat(path); err != nil {
		return ""
	}
	content, err := readLogTail(path, 0)
	if err != nil {
		return ""
	}

	headers, rows, csvErr := parseSamplesheetCSV(content)

	var b strings.Builder
	b.WriteString(`<div class="samplesheet-section" data-ignore-morph data-signals:_show-samplesheet="false">`)
	b.WriteString(`<div class="samplesheet-header" data-on:click="$_showSamplesheet = !$_showSamplesheet">`)
	b.WriteString(`<span class="chevron" data-class:expanded="$_showSamplesheet">▶</span>`)
	b.WriteString(`<span class="detail-label">Samplesheet</span>`)
	b.WriteString(`<span class="samplesheet-path">`)
	b.WriteString(html.EscapeString(path))
	b.WriteString(`</span>`)

	if csvErr != nil {
		// Fallback: render as textarea
		b.WriteString(`<button class="btn-copy" data-on:click__stop="copyText(evt.target, document.getElementById('samplesheet-content').value)">Copy</button>`)
		b.WriteString(`</div>`)
		b.WriteString(`<div data-show="$_showSamplesheet" style="display: none">`)
		b.WriteString(`<textarea id="samplesheet-content" class="samplesheet-textarea">`)
		b.WriteString(html.EscapeString(content))
		b.WriteString(`</textarea>`)
		b.WriteString(`</div>`)
	} else {
		// Table rendering
		b.WriteString(`<div class="samplesheet-actions">`)
		b.WriteString(`<button class="btn-ss-action btn-ss-undo" data-on:click__stop="undoSamplesheet()" title="Undo" disabled>↩</button>`)
		b.WriteString(`<button class="btn-ss-action btn-ss-redo" data-on:click__stop="redoSamplesheet()" title="Redo" disabled>↪</button>`)
		b.WriteString(`</div>`)
		b.WriteString(`<button class="btn-copy" data-on:click__stop="copySamplesheet(evt.target)">Copy CSV</button>`)
		b.WriteString(`</div>`)
		b.WriteString(`<div data-show="$_showSamplesheet" style="display: none">`)
		b.WriteString(`<div class="samplesheet-table-wrapper">`)
		b.WriteString(`<table id="samplesheet-table" class="samplesheet-table">`)
		b.WriteString(`<thead><tr>`)
		for _, h := range headers {
			b.WriteString(`<th>`)
			b.WriteString(html.EscapeString(h))
			b.WriteString(`</th>`)
		}
		b.WriteString(`<th class="col-actions"></th>`)
		b.WriteString(`</tr></thead><tbody>`)
		for _, row := range rows {
			b.WriteString(`<tr>`)
			for _, cell := range row {
				b.WriteString(`<td><input type="text" value="`)
				b.WriteString(html.EscapeString(cell))
				b.WriteString(`"></td>`)
			}
			b.WriteString(`<td class="cell-actions"><button class="btn-remove-row" onclick="removeSamplesheetRow(this)" title="Remove row">×</button></td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</tbody></table>`)
		b.WriteString(`</div>`)
		b.WriteString(`<button class="btn-add-row" data-on:click__stop="addSamplesheetRow()">+ Add Row</button>`)
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	return b.String()
}

// Ensure imports are used.
var _ http.Handler = (*Server)(nil)
