package server

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

const tasksPerPage = 10

// handleTaskPanel is a persistent SSE endpoint that streams the paginated task
// table for a single process group. Route: /sse/run/{id}/tasks/{process}
//
// Connection lifecycle: only one persistent connection per (runID, process) is
// allowed at a time. When a new connection opens, any previous one for the same
// key is killed via its done channel. A generation counter prevents stale
// Datastar auto-retries (from a closed connection's old URL) from evicting the
// current connection.
func (s *Server) handleTaskPanel(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	process := r.PathValue("process")

	run := s.store.GetRun(runID)
	if run == nil {
		http.NotFound(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Parse page and generation from URL query.
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}
	var gen int64
	if g := r.URL.Query().Get("gen"); g != "" {
		if v, err := strconv.ParseInt(g, 10, 64); err == nil {
			gen = v
		}
	}

	// ---- Connection dedup ----
	key := runID + "/" + process
	done := make(chan struct{})

	s.panelMu.Lock()
	currentGen := s.panelGen[key]
	if gen < currentGen {
		// Stale retry from an old URL — reject immediately.
		s.panelMu.Unlock()
		return
	}
	// Kill the previous connection for this panel (if any).
	if old, ok := s.panelConns[key]; ok {
		close(old.done)
	}
	s.panelConns[key] = &panelConn{done: done, gen: gen}
	s.panelMu.Unlock()

	ch := s.runBroker.Subscribe(runID)

	// Clean up on exit (from any cause).
	cleanup := func() {
		s.runBroker.Unsubscribe(runID, ch)
		s.panelMu.Lock()
		if cur, ok := s.panelConns[key]; ok && cur.done == done {
			delete(s.panelConns, key)
		}
		s.panelMu.Unlock()
	}

	// Send initial render.
	html := s.renderTaskPanelHTML(runID, process, page)
	fmt.Fprint(w, formatSSEReplaceFragment(html))
	flusher.Flush()

	for {
		select {
		case <-done:
			// A newer connection replaced us — exit cleanly.
			cleanup()
			return
		case <-ch:
			// Something changed — re-render our page.
			html := s.renderTaskPanelHTML(runID, process, page)
			fmt.Fprint(w, formatSSEReplaceFragment(html))
			flusher.Flush()
		case <-r.Context().Done():
			cleanup()
			return
		}
	}
}

// handleTaskPageNav is a one-shot SSE endpoint for pagination navigation.
// Route: /run/{id}/tasks/{process}/page?page=N
//
// It increments the generation counter for this panel (so any stale retry of
// the old URL is rejected), kills the old persistent connection, and sends back
// a replacement task-panel wrapper whose data-init points to the new page+gen.
// When Datastar inserts the new element, data-init fires → new persistent
// connection opens for the requested page.
func (s *Server) handleTaskPageNav(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	process := r.PathValue("process")
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}

	// Bump generation and kill old connection.
	key := runID + "/" + process
	s.panelMu.Lock()
	s.panelGen[key]++
	gen := s.panelGen[key]
	if old, ok := s.panelConns[key]; ok {
		close(old.done)
		delete(s.panelConns, key)
	}
	s.panelMu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	content := s.renderTaskPanelHTML(runID, process, page)
	wrapper := fmt.Sprintf(
		`<div id="task-panel-%s" data-ignore-morph data-init="@get('/sse/run/%s/tasks/%s?page=%d&gen=%d')">%s</div>`,
		process, runID, process, page, gen, content)

	fmt.Fprint(w, formatSSEReplaceFragment(wrapper))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// renderTaskPanelHTML builds the full task-panel div HTML for the given process
// and page number, including pagination controls when needed.
func (s *Server) renderTaskPanelHTML(runID, process string, page int) string {
	run := s.store.GetRun(runID)

	var filtered []*state.Task
	if run != nil {
		run.RLock()
		for _, task := range run.Tasks {
			if task.Process == process {
				filtered = append(filtered, task)
			}
		}
		run.RUnlock()
	}

	// Sort: failed first, then alphabetical by name.
	sort.Slice(filtered, func(i, j int) bool {
		iFailed := filtered[i].Status == "FAILED"
		jFailed := filtered[j].Status == "FAILED"
		if iFailed != jFailed {
			return iFailed
		}
		return filtered[i].Name < filtered[j].Name
	})

	total := len(filtered)
	totalPages := (total + tasksPerPage - 1) / tasksPerPage
	if totalPages < 1 {
		totalPages = 1
	}

	// Clamp page to valid range.
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// Slice for current page.
	start := (page - 1) * tasksPerPage
	end := start + tasksPerPage
	if end > total {
		end = total
	}
	pageSlice := filtered[start:end]

	tableHTML := renderTaskTable(process, pageSlice, runID)
	paginationHTML := renderPagination(process, runID, page, totalPages, start, end, total)

	return fmt.Sprintf(`<div id="task-content-%s" data-ignore-morph>%s%s</div>`, process, tableHTML, paginationHTML)
}

// renderPagination renders pagination controls for the task panel.
// Returns empty string when totalPages <= 1 (no pagination needed).
func renderPagination(process, runID string, page, totalPages, start, end, total int) string {
	if totalPages <= 1 {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<div class="task-pagination">`)

	// Info text: "Showing X–Y of Z"
	fmt.Fprintf(&b, `<span class="task-pagination-info">%d–%d of %d</span>`, start+1, end, total)

	// Button helper
	writeBtn := func(label string, targetPage int, disabled bool) {
		if disabled {
			fmt.Fprintf(&b, `<button class="btn-page" disabled>%s</button>`, label)
		} else {
			fmt.Fprintf(&b,
				`<button class="btn-page" data-on:click="@get('/run/%s/tasks/%s/page?page=%d')">%s</button>`,
				runID, process, targetPage, label)
		}
	}

	onFirst := page == 1
	onLast := page == totalPages

	writeBtn("«", 1, onFirst)
	writeBtn("‹", page-1, onFirst)
	writeBtn("›", page+1, onLast)
	writeBtn("»", totalPages, onLast)

	b.WriteString(`</div>`)
	return b.String()
}
