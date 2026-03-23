// Package server provides HTTP handlers for the webhook endpoint, SSE fan-out, and static frontend.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	fragment := s.renderTaskList()
	s.broker.Publish(fragment)
	w.WriteHeader(http.StatusOK)
}

// handleSSE sets SSE headers (Content-Type: text/event-stream, Cache-Control: no-cache),
// subscribes to the broker, and streams HTML fragment events until the client disconnects.
// On connect, sends an initial full render of current state.
// Datastar SSE format: each event is "event: datastar-merge-fragments\ndata: fragments <html>\n\n"
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

	// Send initial full render of current state.
	initial := s.renderTaskList()
	fmt.Fprintf(w, "event: datastar-merge-fragments\ndata: fragments %s\n\n", initial)
	flusher.Flush()

	for {
		select {
		case data := <-ch:
			fmt.Fprintf(w, "event: datastar-merge-fragments\ndata: fragments %s\n\n", data)
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

// renderTaskList renders an HTML fragment showing all tasks for all runs
// as a list of "process_name: STATUS" lines. The fragment has an id="task-list"
// so Datastar can merge it into the DOM.
func (s *Server) renderTaskList() string {
	runs := s.store.GetAllRuns()

	// Collect all tasks across all runs.
	var tasks []*state.Task
	for _, r := range runs {
		for _, task := range r.Tasks {
			tasks = append(tasks, task)
		}
	}

	if len(tasks) == 0 {
		return `<div id="task-list"><p>Waiting for pipeline events...</p></div>`
	}

	var b strings.Builder
	b.WriteString(`<div id="task-list">`)
	for _, task := range tasks {
		fmt.Fprintf(&b, "<p>%s: %s</p>", task.Name, task.Status)
	}
	b.WriteString("</div>")
	return b.String()
}

// Ensure imports are used.
var _ http.Handler = (*Server)(nil)
