package server

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

const tasksPerPage = 10

// parseTaskFilters extracts the common task-panel query parameters (page, q,
// status) from URL query values. page defaults to 1 if missing or invalid.
func parseTaskFilters(query url.Values) (page int, q, statusFilter string) {
	page = 1
	if p := query.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}
	q = query.Get("q")
	statusFilter = query.Get("status")
	return
}

// buildFilterQuery returns a URL query suffix (starting with "&") encoding the
// non-empty filter parameters q and statusFilter.
func buildFilterQuery(q, statusFilter string) string {
	var s string
	if q != "" {
		s += "&q=" + url.QueryEscape(q)
	}
	if statusFilter != "" {
		s += "&status=" + url.QueryEscape(statusFilter)
	}
	return s
}

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

	// Parse page, generation, and filters from URL query.
	page, q, statusFilter := parseTaskFilters(r.URL.Query())
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
	html := s.renderTaskPanelHTML(runID, process, q, statusFilter, page)
	fmt.Fprint(w, formatSSEReplaceFragment(html))
	flusher.Flush()

	for {
		select {
		case <-done:
			// A newer connection replaced us — exit cleanly.
			cleanup()
			return
		case <-ch:
			// Something changed — re-render only the results section so the
			// filter bar input keeps focus (never replaced by SSE updates).
			html := s.renderTaskResultsHTML(runID, process, q, statusFilter, page)
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
	page, q, statusFilter := parseTaskFilters(r.URL.Query())

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

	// Build filter query params for the data-init URL.
	filterQuery := buildFilterQuery(q, statusFilter)

	content := s.renderTaskPanelHTML(runID, process, q, statusFilter, page)
	wrapper := fmt.Sprintf(
		`<div id="task-panel-%s" data-ignore-morph data-init="@get('/sse/run/%s/tasks/%s?page=%d&gen=%d%s')">%s</div>`,
		process, runID, process, page, gen, filterQuery, content)

	fmt.Fprint(w, formatSSEReplaceFragment(wrapper))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// renderTaskPanelHTML builds the full task-panel div HTML for the given process
// and page number, including the filter bar and pagination controls.
// Used for initial render and handleTaskPageNav (one-shot). For live updates
// in the SSE loop, use renderTaskResultsHTML instead (avoids replacing the
// filter bar and losing input focus).
func (s *Server) renderTaskPanelHTML(runID, process, q, statusFilter string, page int) string {
	filterBarHTML := renderFilterBar(process, runID, q, statusFilter)
	resultsHTML := s.renderTaskResultsHTML(runID, process, q, statusFilter, page)
	return fmt.Sprintf(`<div id="task-content-%s">%s%s</div>`,
		process, filterBarHTML, resultsHTML)
}

// renderTaskResultsHTML builds just the results section (table + pagination)
// wrapped in <div id="task-results-{process}">. Separated from the filter bar
// so SSE live updates can replace only the results without touching the input.
func (s *Server) renderTaskResultsHTML(runID, process, q, statusFilter string, page int) string {
	run := s.store.GetRun(runID)

	var processTasks []*state.Task
	if run != nil {
		run.RLock()
		for _, task := range run.Tasks {
			if task.Process == process {
				processTasks = append(processTasks, task)
			}
		}
		run.RUnlock()
	}

	// Sort: failed first, then alphabetical by name.
	sort.Slice(processTasks, func(i, j int) bool {
		iFailed := processTasks[i].Status == "FAILED"
		jFailed := processTasks[j].Status == "FAILED"
		if iFailed != jFailed {
			return iFailed
		}
		return processTasks[i].Name < processTasks[j].Name
	})

	// Apply search/status filter.
	filtered := filterTasks(processTasks, q, statusFilter)

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

	var resultsInner string
	if total == 0 {
		resultsInner = `<div class="no-matching-tasks">No matching tasks</div>`
	} else {
		// Slice for current page.
		start := (page - 1) * tasksPerPage
		end := start + tasksPerPage
		if end > total {
			end = total
		}
		pageSlice := filtered[start:end]

		tableHTML := renderTaskTable(process, pageSlice, runID)
		paginationHTML := renderPagination(process, runID, q, statusFilter, page, totalPages, start, end, total)
		resultsInner = tableHTML + paginationHTML
	}

	return fmt.Sprintf(`<div id="task-results-%s">%s</div>`, process, resultsInner)
}

// renderPagination renders pagination controls for the task panel.
// Returns empty string when totalPages <= 1 (no pagination needed).
func renderPagination(process, runID, q, statusFilter string, page, totalPages, start, end, total int) string {
	if totalPages <= 1 {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<div class="task-pagination">`)

	// Info text: "Showing X–Y of Z"
	fmt.Fprintf(&b, `<span class="task-pagination-info">%d–%d of %d</span>`, start+1, end, total)

	// Build query suffix for filter persistence.
	filterQuery := buildFilterQuery(q, statusFilter)

	// Button helper
	writeBtn := func(label string, targetPage int, disabled bool) {
		if disabled {
			fmt.Fprintf(&b, `<button class="btn-page" disabled>%s</button>`, label)
		} else {
			fmt.Fprintf(&b,
				`<button class="btn-page" data-on:click="@get('/run/%s/tasks/%s/page?page=%d%s')">%s</button>`,
				runID, process, targetPage, filterQuery, label)
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

// filterTasks returns the subset of tasks whose name or tag contains q (case-insensitive)
// and whose status matches one of the comma-separated values in statusFilter (case-insensitive).
// Both filters are optional: empty string means "match all".
func filterTasks(tasks []*state.Task, q, statusFilter string) []*state.Task {
	qLower := strings.ToLower(q)

	// Build status allow-set from comma-separated filter.
	var statusSet map[string]bool
	if statusFilter != "" {
		statusSet = make(map[string]bool)
		for _, s := range strings.Split(statusFilter, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statusSet[strings.ToUpper(s)] = true
			}
		}
	}

	var result []*state.Task
	for _, t := range tasks {
		// Text filter: q must appear in Name or Tag (case-insensitive).
		if qLower != "" {
			nameLower := strings.ToLower(t.Name)
			tagLower := strings.ToLower(t.Tag)
			if !strings.Contains(nameLower, qLower) && !strings.Contains(tagLower, qLower) {
				continue
			}
		}
		// Status filter: task status must be in the allow-set.
		if statusSet != nil && !statusSet[strings.ToUpper(t.Status)] {
			continue
		}
		result = append(result, t)
	}
	return result
}

// renderFilterBar renders the search/filter bar HTML for the task panel.
// Includes a text input for name/tag search and a status filter dropdown.
// Uses data-ignore-morph to preserve input focus across SSE re-renders.
func renderFilterBar(process, runID, q, statusFilter string) string {
	var b strings.Builder

	fmt.Fprintf(&b, `<div id="task-filter-%s" class="task-filter-bar" data-ignore-morph>`, process)

	// Text input for name/tag search, pre-filled with q (HTML-escaped).
	fmt.Fprintf(&b,
		`<input type="text" class="task-search" placeholder="Search tasks..." value="%s" `+
			`data-bind:_task-filter `+
			`data-on:input__debounce.300ms="`+
			`@get('/run/%s/tasks/%s/page?page=1&q='+encodeURIComponent($_taskFilter)+'&status='+encodeURIComponent($_statusFilter || ''))" />`,
		html.EscapeString(q), runID, process)

	// Status dropdown, pre-filled with statusFilter via selected attribute.
	b.WriteString(`<select class="task-status-filter" data-bind:_status-filter `)
	fmt.Fprintf(&b,
		`data-on:change="@get('/run/%s/tasks/%s/page?page=1&q='+encodeURIComponent($_taskFilter || '')+'&status='+$_statusFilter)">`,
		runID, process)

	type opt struct {
		Value string
		Label string
	}
	options := []opt{
		{"", "All"},
		{"FAILED", "Failed"},
		{"RUNNING", "Running"},
		{"COMPLETED", "Completed"},
	}
	for _, o := range options {
		sel := ""
		if o.Value != "" && o.Value == statusFilter {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, o.Value, sel, o.Label)
	}

	b.WriteString(`</select>`)
	b.WriteString(`</div>`)

	return b.String()
}
