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

// handleTaskPanel is a one-shot text/html endpoint for GET /tasks/{runID}/{process}.
// Parses runID, process, page, q, status from URL; renders task panel HTML; writes
// a text/html response. Polled by the browser every 1s via data-on-interval.
func (s *Server) handleTaskPanel(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runID")
	process := r.PathValue("process")

	if s.store.GetRun(runID) == nil {
		http.NotFound(w, r)
		return
	}

	page, q, statusFilter := parseTaskFilters(r.URL.Query())
	html := s.renderTaskPanelHTML(runID, process, q, statusFilter, page)

	// Use Datastar-Mode: inner to morph content INTO the task-panel div
	// rather than replacing the div itself. This avoids conflict with the
	// dashboard morph which sends an empty task-panel div — inner mode
	// means we target the panel's children, not the panel element itself.
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Datastar-Selector", fmt.Sprintf("#task-panel-%s", cssID(process)))
	w.Header().Set("Datastar-Mode", "inner")
	w.Write([]byte(html))
}

// renderTaskPanelHTML builds the full task-panel div HTML for the given process
// and page number, including the filter bar, task table, and pagination controls.
// Used by handleTaskPanel (one-shot text/html response).
func (s *Server) renderTaskPanelHTML(runID, process, q, statusFilter string, page int) string {
	// 1. Get run from store, filter tasks by process.
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

	// 2. Sort: failed first, then alphabetical by name.
	sort.Slice(processTasks, func(i, j int) bool {
		iFailed := processTasks[i].Status == "FAILED"
		jFailed := processTasks[j].Status == "FAILED"
		if iFailed != jFailed {
			return iFailed
		}
		return processTasks[i].Name < processTasks[j].Name
	})

	// 3. Apply search/status filter.
	filtered := filterTasks(processTasks, q, statusFilter)

	// 4. Render filter bar.
	filterBarHTML := renderFilterBar(process, runID, q, statusFilter)

	// 5. Paginate (clamp page, slice).
	total := len(filtered)
	totalPages := (total + tasksPerPage - 1) / tasksPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// 6–7. Build results inner HTML.
	var resultsInner string
	if total == 0 {
		resultsInner = `<div class="no-matching-tasks">No matching tasks</div>`
	} else {
		start := (page - 1) * tasksPerPage
		end := start + tasksPerPage
		if end > total {
			end = total
		}
		pageSlice := filtered[start:end]
		resultsInner = renderTaskTable(process, pageSlice, runID) +
			renderPagination(process, runID, q, statusFilter, page, totalPages, start, end, total)
	}

	// 8. Return two-div structure.
	// No outer task-panel wrapper — handleTaskPanel uses Datastar-Mode: inner
	// with Datastar-Selector: #task-panel-{process}, so this HTML goes INSIDE
	// the existing task-panel div. The dashboard morph sends an empty task-panel
	// div but inner mode means it replaces children, not the element itself.
	//
	// The task-results div has data-on-interval so the expanded panel auto-refreshes
	// every 1s. The condition ($expandedGroup === '{process}') stops polling when
	// the group is collapsed. The URL includes current page + filters so the view
	// stays on the same page across refreshes.
	filterQuery := buildFilterQuery(q, statusFilter)
	return fmt.Sprintf(`%s<div id="task-results-%s" data-on-interval__duration.1s="$expandedGroup === '%s' && @get('/tasks/%s/%s?page=%d%s')">%s</div>`,
		filterBarHTML, cssID(process), process, runID, process, page, filterQuery, resultsInner)
}

// renderPagination renders pagination controls for the task panel.
// Returns empty string when totalPages <= 1 (no pagination needed).
// renderPagination renders pagination controls for the task panel.
// Button URLs use /tasks/{runID}/{process}?page=N (new endpoint path).
// Returns empty string when totalPages <= 1 (no pagination needed).
func renderPagination(process, runID, q, statusFilter string, page, totalPages, start, end, total int) string {
	if totalPages <= 1 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="task-pagination">`)
	fmt.Fprintf(&b, `<span class="task-pagination-info">%d–%d of %d</span>`, start+1, end, total)
	filterQuery := buildFilterQuery(q, statusFilter)
	writeBtn := func(label string, targetPage int, disabled bool) {
		if disabled {
			fmt.Fprintf(&b, `<button class="btn-page" disabled>%s</button>`, label)
		} else {
			fmt.Fprintf(&b, `<button class="btn-page" data-on:click="@get('/tasks/%s/%s?page=%d%s')">%s</button>`,
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
// renderFilterBar renders the search/filter bar HTML for the task panel.
// @get URLs use /tasks/{runID}/{process}?page=1&q=...&status=... (new endpoint path).
func renderFilterBar(process, runID, q, statusFilter string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<div id="task-filter-%s" class="task-filter-bar" data-ignore-morph>`, cssID(process))
	fmt.Fprintf(&b,
		`<input type="text" class="task-search" placeholder="Search tasks..." value="%s" `+
			`data-bind:_task-filter `+
			`data-on:input__debounce.300ms="`+
			`@get('/tasks/%s/%s?page=1&q='+encodeURIComponent($_taskFilter)+'&status='+encodeURIComponent($_statusFilter || ''))" />`,
		html.EscapeString(q), runID, process)
	b.WriteString(`<select class="task-status-filter" data-bind:_status-filter `)
	fmt.Fprintf(&b,
		`data-on:change="@get('/tasks/%s/%s?page=1&q='+encodeURIComponent($_taskFilter || '')+'&status='+$_statusFilter)">`,
		runID, process)
	type opt struct{ Value, Label string }
	for _, o := range []opt{{"", "All"}, {"FAILED", "Failed"}, {"RUNNING", "Running"}, {"COMPLETED", "Completed"}} {
		sel := ""
		if o.Value != "" && o.Value == statusFilter {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, o.Value, sel, o.Label)
	}
	b.WriteString(`</select></div>`)
	return b.String()
}
