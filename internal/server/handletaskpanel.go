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
// It subscribes to runBroker and re-renders on each webhook notification.
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

	// Parse page param (default 1, clamp later per filtered task count).
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}

	ch := s.runBroker.Subscribe(runID)

	// Send initial render. Use "replace" mode to bypass Datastar's morph
	// ancestor check — the target element is inside a data-ignore-morph parent
	// (which protects it from the main run SSE), but replace mode uses
	// replaceWith() directly, so the content still gets updated.
	html := s.renderTaskPanelHTML(runID, process, page)
	fmt.Fprint(w, formatSSEReplaceFragment(html))
	flusher.Flush()

	for {
		select {
		case <-ch:
			// Something changed — ignore the payload, re-render our own view.
			html := s.renderTaskPanelHTML(runID, process, page)
			fmt.Fprint(w, formatSSEReplaceFragment(html))
			flusher.Flush()
		case <-r.Context().Done():
			s.runBroker.Unsubscribe(runID, ch)
			return
		}
	}
}

// handleTaskPageNav is a one-shot SSE endpoint for pagination navigation.
// Route: /run/{id}/tasks/{process}/page?page=N
// It replaces the outer task-panel-{process} wrapper (which carries data-init for
// the persistent SSE stream). By replacing the wrapper, Datastar tears down the
// old persistent SSE connection and fires data-init with the new page URL,
// establishing a fresh persistent connection for the requested page.
func (s *Server) handleTaskPageNav(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	process := r.PathValue("process")
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	content := s.renderTaskPanelHTML(runID, process, page)
	wrapper := fmt.Sprintf(`<div id="task-panel-%s" data-ignore-morph data-init="@get('/sse/run/%s/tasks/%s?page=%d')">%s</div>`,
		process, runID, process, page, content)

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
