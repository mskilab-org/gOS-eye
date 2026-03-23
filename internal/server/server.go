// Package server provides HTTP handlers for the webhook endpoint, SSE fan-out, and static frontend.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

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
// with per-status counts. Used by renderDashboard to build the process group list.
type ProcessGroup struct {
	Name      string // process name (e.g., "sayHello", "align")
	Total     int    // total tasks in this group
	Completed int    // tasks with status COMPLETED
	Running   int    // tasks with status RUNNING
	Failed    int    // tasks with status FAILED
	Submitted int    // tasks with status SUBMITTED
}

// ---- Data Definition: HTTP Server ----

// Server ties together the state store, SSE broker, and HTTP routes.
type Server struct {
	store  *state.Store
	broker *Broker
	mux    *http.ServeMux
}

// NewServer creates a Server with routes registered on an internal ServeMux.
// Routes:
//   - POST /webhook → handleWebhook
//   - GET  /sse     → handleSSE
//   - GET  /        → handleIndex
func NewServer(store *state.Store) *Server {
	s := &Server{
		store:  store,
		broker: NewBroker(),
		mux:    http.NewServeMux(),
	}
	s.mux.HandleFunc("/webhook", s.handleWebhook)
	s.mux.HandleFunc("/sse", s.handleSSE)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
	s.mux.HandleFunc("/", s.handleIndex)
	return s
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
	var event state.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	s.store.HandleEvent(event)
	fragment := s.renderDashboard()
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
	initial := s.renderDashboard()
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

// groupProcesses groups tasks by Process name and returns per-group status counts.
// Returns a sorted slice of ProcessGroup for stable rendering order.
func groupProcesses(tasks map[int]*state.Task) []ProcessGroup {
	groups := make(map[string]*ProcessGroup)
	for _, task := range tasks {
		g, ok := groups[task.Process]
		if !ok {
			g = &ProcessGroup{Name: task.Process}
			groups[task.Process] = g
		}
		g.Total++
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
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// renderDashboard renders the full dashboard HTML fragment: header (pipeline name,
// run name, status), progress bar (completed/total with percentage and animated fill),
// and process group list (each group shows completed/total with status-colored indicators).
// The fragment uses Datastar-compatible ids so SSE patches update the DOM.
func (s *Server) renderDashboard() string {
	run := s.store.GetLatestRun()
	if run == nil {
		return `<div id="dashboard"><p class="waiting">Waiting for pipeline events...</p></div>`
	}
	var b strings.Builder

	// Root element with optional start-time signal (Datastar v1 colon syntax;
	// data-signals:start-time → camelCase → $startTime on client)
	b.WriteString(`<div id="dashboard"`)
	if run.StartTime != "" {
		b.WriteString(fmt.Sprintf(` data-signals:start-time="'%s'"`, run.StartTime))
	}
	b.WriteByte('>')

	// Run header
	pipelineName := run.ProjectName
	if pipelineName == "" {
		pipelineName = "Pipeline"
	}
	statusLower := strings.ToLower(run.Status)
	b.WriteString(`<div class="run-header">`)
	b.WriteString(fmt.Sprintf(`<h1>%s</h1>`, pipelineName))
	b.WriteString(fmt.Sprintf(`<span class="run-name">%s</span>`, run.RunName))
	b.WriteString(fmt.Sprintf(`<span class="badge status-%s">%s</span>`, statusLower, strings.ToUpper(run.Status)))
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
	} else if statusLower == "error" {
		fillClass += " status-failed"
	}

	b.WriteString(`<div class="progress-bar">`)
	b.WriteString(fmt.Sprintf(`<div class="%s" style="width: %d%%"></div>`, fillClass, pct))
	b.WriteString(fmt.Sprintf(`<span class="progress-label">%d/%d (%d%%)</span>`, completed, total, pct))
	b.WriteString(`</div>`)

	// Process groups
	groups := groupProcesses(run.Tasks)
	for _, g := range groups {
		gpct := 0
		if g.Total > 0 {
			gpct = g.Completed * 100 / g.Total
		}
		b.WriteString(`<div class="process-group">`)
		b.WriteString(fmt.Sprintf(`<span class="process-name">%s</span>`, g.Name))
		b.WriteString(fmt.Sprintf(`<span class="process-counts">%d/%d</span>`, g.Completed, g.Total))
		b.WriteString(fmt.Sprintf(`<div class="mini-bar"><div class="mini-fill" style="width: %d%%"></div></div>`, gpct))
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
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
