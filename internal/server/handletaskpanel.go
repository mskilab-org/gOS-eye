package server

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// handleTaskPanel is a one-shot SSE endpoint that returns the task table HTML
// for a single process group. Route: /sse/run/{id}/tasks/{process}
func (s *Server) handleTaskPanel(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	process := r.PathValue("process")

	run := s.store.GetRun(runID)
	if run == nil {
		http.NotFound(w, r)
		return
	}

	// Filter tasks by process name
	var filtered []*state.Task
	for _, task := range run.Tasks {
		if task.Process == process {
			filtered = append(filtered, task)
		}
	}

	// Sort: failed first, then alphabetical by name
	sort.Slice(filtered, func(i, j int) bool {
		iFailed := filtered[i].Status == "FAILED"
		jFailed := filtered[j].Status == "FAILED"
		if iFailed != jFailed {
			return iFailed
		}
		return filtered[i].Name < filtered[j].Name
	})

	tableHTML := renderTaskTable(process, filtered, runID)
	html := fmt.Sprintf(`<div id="task-panel-%s">%s</div>`, process, tableHTML)

	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, formatSSEFragment(html))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
