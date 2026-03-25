// Package server provides HTTP handlers for the webhook endpoint, SSE fan-out, and static frontend.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// ---- Data Definition: Event Persistence ----

// EventPersister abstracts the write side of event storage.
// Implementations persist raw webhook JSON for replay on restart.
type EventPersister interface {
	Save(rawJSON []byte) error
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
type Server struct {
	store      *state.Store
	eventStore EventPersister              // persists raw webhook JSON; nil = no persistence
	broker     *Broker
	mux        *http.ServeMux
	layoutsMu  sync.RWMutex               // protects layouts
	layouts    map[string]*dag.Layout     // project name → computed layout
}

// NewServer creates a Server with routes registered on an internal ServeMux.
// layout may be nil when no DAG file is provided.
// Routes:
//   - POST /webhook → handleWebhook
//   - GET  /sse     → handleSSE
//   - GET  /        → handleIndex
func NewServer(store *state.Store, persist EventPersister) *Server {
	s := &Server{
		store:      store,
		eventStore: persist,
		broker:     NewBroker(),
		mux:        http.NewServeMux(),
		layouts:    make(map[string]*dag.Layout),
	}
	s.mux.HandleFunc("/webhook", s.handleWebhook)
	s.mux.HandleFunc("/sse", s.handleSSE)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
	s.mux.HandleFunc("/", s.handleIndex)
	return s
}

// discoverDAG looks for a dag.dot file in the same directory as the pipeline's
// scriptFile, parses it, and returns the computed layout. Returns nil if the
// file doesn't exist or can't be parsed (non-fatal — pipeline renders without DAG).
func discoverDAG(scriptFile string) *dag.Layout {
	dir := filepath.Dir(scriptFile)
	dotPath := filepath.Join(dir, "dag.dot")
	f, err := os.Open(dotPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	d, err := dag.ParseDOT(f)
	if err != nil {
		log.Printf("warning: failed to parse %s: %v", dotPath, err)
		return nil
	}
	return dag.ComputeLayout(d)
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
		if layout := discoverDAG(scriptFile); layout != nil {
			s.layoutsMu.Lock()
			s.layouts[projectName] = layout
			s.layoutsMu.Unlock()
			log.Printf("DAG loaded for %q (%d processes) from %s",
				projectName, len(layout.Nodes), filepath.Join(filepath.Dir(scriptFile), "dag.dot"))
		} else {
			log.Printf("no dag.dot found for %q in %s — using list view",
				projectName, filepath.Dir(scriptFile))
		}
	}

	fragment := s.renderSidebar() + s.renderDashboard()
	s.broker.Publish(fragment)
	w.WriteHeader(http.StatusOK)
}

// handleSSE sets SSE headers (Content-Type: text/event-stream, Cache-Control: no-cache),
// subscribes to the broker, and streams HTML fragment events until the client disconnects.
// On connect, sends an initial full render of current state.
// Datastar v1 SSE format: each event is "event: datastar-patch-elements\ndata: elements <html>\n\n"
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

	// Send initial full render of current state as Datastar v1 patch-elements event.
	initial := s.renderSidebar() + s.renderDashboard()
	fmt.Fprint(w, formatSSEFragment(initial))
	flusher.Flush()

	for {
		select {
		case data := <-ch:
			fmt.Fprint(w, formatSSEFragment(data))
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

	// Sort by StartTime descending (newest first) without mutating the input slice.
	sorted := make([]*state.Run, len(runs))
	copy(sorted, runs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime > sorted[j].StartTime
	})

	var b strings.Builder
	b.WriteString(`<div id="run-list">`)

	for _, run := range sorted {
		pipelineName := run.ProjectName
		if pipelineName == "" {
			pipelineName = "Pipeline"
		}
		statusLower := strings.ToLower(run.Status)

		b.WriteString(fmt.Sprintf(`<div class="run-entry" data-on:click="$selectedRun = '%s'" data-class:active="($selectedRun || $latestRun) === '%s'">`,
			run.RunID, run.RunID))
		b.WriteString(fmt.Sprintf(`<span class="run-pipeline">%s</span>`, pipelineName))
		b.WriteString(fmt.Sprintf(`<span class="run-name">%s</span>`, run.RunName))
		b.WriteString(fmt.Sprintf(`<span class="badge status-%s">%s</span>`, statusLower, strings.ToUpper(run.Status)))
		b.WriteString(fmt.Sprintf(`<span class="run-time">%s</span>`, formatRelativeTime(run.StartTime, time.Now())))
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	return b.String()
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
				`data-on:click="$_dagSelectedProcess = $_dagSelectedProcess === '%s' ? '' : '%s'" `+
				`data-class:dag-faded="dagShouldFade($_dagHL, '%s', %s)" `+
				`data-class:dag-node-selected="$_dagSelectedProcess === '%s'">`,
			status, pos.x, pos.y, nodeWidth, nodeHeight,
			n.Name,
			n.Name, n.Name,
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
// data-show="$_dagSelectedProcess === 'PROCESS_NAME'". Reuses renderTaskRows for task row
// rendering. Contains a close button pattern (click same node to deselect).
func renderDAGTaskPanel(groups []ProcessGroup) string {
	if len(groups) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="dag-task-panel" id="dag-task-panel">`)

	for _, g := range groups {
		// Wrapper: only visible when this process is selected in the DAG
		b.WriteString(fmt.Sprintf(`<div data-show="$_dagSelectedProcess === '%s'">`, g.Name))

		b.WriteString(`<div class="dag-task-section">`)

		// Header: process name, status dot, counts
		b.WriteString(`<div class="dag-task-section-header">`)
		b.WriteString(fmt.Sprintf(`<span class="dag-task-name">%s</span>`, g.Name))

		b.WriteString(renderGroupStatusDot(g))

		b.WriteString(fmt.Sprintf(`<span class="dag-task-counts">%d/%d</span>`, g.Completed, g.Total))
		b.WriteString(`</div>`) // close dag-task-section-header

		// Task rows
		b.WriteString(`<div class="task-list">`)
		b.WriteString(renderTaskRows(g.Name, g.Tasks))
		b.WriteString(`</div>`)

		b.WriteString(`</div>`) // close dag-task-section
		b.WriteString(`</div>`) // close data-show wrapper
	}

	b.WriteString(`</div>`) // close dag-task-panel
	return b.String()
}

// renderRunDetail renders the detail panel for a single pipeline run: header (pipeline name,
// run name, status badge), progress bar (completed/total with percentage), and process group
// list with expandable task details. This is the per-run content extracted from the former
// single-run renderDashboard. The returned HTML should be wrapped in a div with data-show
// controlling visibility based on the selected run signal.
// The wrapping div and data-show attribute are added by the caller (renderDashboard).
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

	// Run header with optional start-time signal for elapsed timer
	// Run name is omitted here — it's shown in the sidebar run list.
	b.WriteString(`<div class="run-header"`)
	if run.StartTime != "" {
		b.WriteString(fmt.Sprintf(` data-signals:start-time="'%s'"`, run.StartTime))
	}
	if run.CompleteTime != "" {
		b.WriteString(fmt.Sprintf(` data-signals:complete-time="'%s'"`, run.CompleteTime))
	} else {
		b.WriteString(` data-signals:complete-time="''"`)
	}
	b.WriteString(`>`)
	b.WriteString(fmt.Sprintf(`<h1>%s</h1>`, pipelineName))
	b.WriteString(fmt.Sprintf(`<span class="badge status-%s">%s</span>`, statusLower, strings.ToUpper(run.Status)))
	b.WriteString(fmt.Sprintf(`<span class="run-name">%s</span>`, run.RunName))
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

	// Process view: DAG when layout is available, otherwise grouped process list.
	s.layoutsMu.RLock()
	layout := s.layouts[run.ProjectName]
	s.layoutsMu.RUnlock()
	if layout != nil {
		b.WriteString(renderDAG(layout, run))
		groups := groupProcesses(run.Tasks)
		b.WriteString(renderDAGTaskPanel(groups))
	} else {
		// Process groups — each is a clickable container that expands to show task rows.
		groups := groupProcesses(run.Tasks)
		for _, g := range groups {
			gpct := 0
			if g.Total > 0 {
				gpct = g.Completed * 100 / g.Total
			}

			// Group status indicator class
			groupStatusClass := ""
			if g.Failed > 0 {
				groupStatusClass = " group-has-failed"
			} else if g.Running > 0 {
				groupStatusClass = " group-has-running"
			}

			b.WriteString(fmt.Sprintf(`<div class="process-group-container%s">`, groupStatusClass))

			// Clickable header — toggles $expandedGroup signal
			b.WriteString(fmt.Sprintf(`<div class="process-group" data-on:click="$expandedGroup = $expandedGroup === '%s' ? '' : '%s'" style="cursor: pointer;">`, g.Name, g.Name))

			// Chevron indicator
			b.WriteString(fmt.Sprintf(`<span class="chevron" data-class:expanded="$expandedGroup === '%s'">▶</span>`, g.Name))

			b.WriteString(fmt.Sprintf(`<span class="process-name">%s</span>`, g.Name))

			b.WriteString(renderGroupStatusDot(g))

			b.WriteString(fmt.Sprintf(`<span class="process-counts">%d/%d</span>`, g.Completed, g.Total))
			b.WriteString(fmt.Sprintf(`<div class="mini-bar"><div class="mini-fill" style="width: %d%%"></div></div>`, gpct))
			b.WriteString(`</div>`)

			// Task list — visible when this group is expanded
			b.WriteString(fmt.Sprintf(`<div class="task-list" data-show="$expandedGroup === '%s'">`, g.Name))
			b.WriteString(renderTaskRows(g.Name, g.Tasks))
			b.WriteString(`</div>`)

			b.WriteString(`</div>`) // close process-group-container
		}
	}

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

// renderDashboard renders the main panel HTML fragment: header (pipeline name,
// run name, status), progress bar (completed/total with percentage and animated fill),
// and process group list (each group shows completed/total with status-colored indicators).
// The fragment uses Datastar-compatible ids so SSE patches update the DOM.
// The sidebar run list is rendered separately by renderRunList.
func (s *Server) renderDashboard() string {
	runs := s.store.GetAllRuns()
	latestRunID := s.store.GetLatestRunID()

	if len(runs) == 0 {
		return `<div id="dashboard"><p class="waiting">Waiting for pipeline events...</p></div>`
	}

	// Sort runs by StartTime descending for stable output order.
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].StartTime != runs[j].StartTime {
			return runs[i].StartTime > runs[j].StartTime
		}
		return runs[i].RunID < runs[j].RunID
	})

	var b strings.Builder

	// Outer dashboard div with latest-run signal for auto-follow
	b.WriteString(fmt.Sprintf(`<div id="dashboard" data-signals:latest-run="'%s'">`, latestRunID))

	// Each run gets a wrapper div with data-show for visibility toggling
	for _, run := range runs {
		b.WriteString(fmt.Sprintf(`<div data-show="($selectedRun || $latestRun) === '%s'">`, run.RunID))
		b.WriteString(s.renderRunDetail(run))
		b.WriteString(renderRunSummary(run))
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	return b.String()
}

// renderSidebar renders the sidebar run-list fragment.
// Separate from renderDashboard so the sidebar and main panel are independent morph targets.
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

// renderTaskRows renders the expandable task rows HTML for one process group.
// Takes the process name (for Datastar signal scoping) and the sorted task slice.
// Each task renders as a row showing: name, status badge, formatted duration.
// Each row is expandable (via Datastar $expandedTask signal matching task ID) to show
// a detail panel with: CPU%, RSS/peak RSS (formatted), exit code, workdir, and
// timestamps (submit, start, complete — formatted from epoch millis).
// Failed tasks (status "FAILED") are sorted to the top and highlighted with class "task-row failed".
func renderTaskRows(processName string, tasks []*state.Task) string {
	if len(tasks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, task := range tasks {
		isFailed := task.Status == "FAILED"

		// Task row (clickable)
		rowClass := "task-row"
		if isFailed {
			rowClass = "task-row failed"
		}
		fmt.Fprintf(&b, `<div class="%s" data-on:click__stop="$expandedTask = $expandedTask === %d ? 0 : %d">`, rowClass, task.TaskID, task.TaskID)
		fmt.Fprintf(&b, `<span class="task-name">%s</span>`, task.Name)
		fmt.Fprintf(&b, `<span class="badge status-%s">%s</span>`, strings.ToLower(task.Status), task.Status)
		fmt.Fprintf(&b, `<span class="task-duration">%s</span>`, formatDuration(task.Duration))
		b.WriteString(`</div>`)

		// Task detail (expandable)
		fmt.Fprintf(&b, `<div class="task-detail" data-show="$expandedTask === %d">`, task.TaskID)
		b.WriteString(`<div class="detail-grid">`)

		// CPU
		cpuVal := "—"
		if task.CPUPercent != 0 {
			cpuVal = fmt.Sprintf("%.1f%%", task.CPUPercent)
		}
		fmt.Fprintf(&b, `<span class="detail-label">CPU</span><span class="detail-value">%s</span>`, cpuVal)

		// Memory (0 means not tracked — show "—" instead of "0 B")
		memVal := "—"
		if task.RSS > 0 {
			memVal = formatBytes(task.RSS)
		}
		fmt.Fprintf(&b, `<span class="detail-label">Memory</span><span class="detail-value">%s</span>`, memVal)

		// Peak Memory (0 means not tracked — show "—" instead of "0 B")
		peakVal := "—"
		if task.PeakRSS > 0 {
			peakVal = formatBytes(task.PeakRSS)
		}
		fmt.Fprintf(&b, `<span class="detail-label">Peak Memory</span><span class="detail-value">%s</span>`, peakVal)

		// Exit Code
		exitClass := "detail-value"
		if task.Exit != 0 {
			exitClass = "detail-value exit-error"
		}
		fmt.Fprintf(&b, `<span class="detail-label">Exit Code</span><span class="%s">%d</span>`, exitClass, task.Exit)

		// Work Dir
		fmt.Fprintf(&b, `<span class="detail-label">Work Dir</span><span class="detail-value workdir">%s</span>`, task.Workdir)

		// Timestamps
		fmt.Fprintf(&b, `<span class="detail-label">Submitted</span><span class="detail-value">%s</span>`, formatTimestamp(task.Submit))
		fmt.Fprintf(&b, `<span class="detail-label">Started</span><span class="detail-value">%s</span>`, formatTimestamp(task.Start))
		fmt.Fprintf(&b, `<span class="detail-label">Completed</span><span class="detail-value">%s</span>`, formatTimestamp(task.Complete))

		b.WriteString(`</div>`)
		b.WriteString(`</div>`)
	}
	return b.String()
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

// formatSSESignals formats a signal patch for Datastar v1 SSE wire format.
// Takes a JSON-encoded signal map string.
// Returns: "event: datastar-patch-signals\ndata: signals {\"key\": value}\n\n"
func formatSSESignals(jsonSignals string) string {
	return "event: datastar-patch-signals\ndata: signals " + jsonSignals + "\n\n"
}

// Ensure imports are used.
var _ http.Handler = (*Server)(nil)
