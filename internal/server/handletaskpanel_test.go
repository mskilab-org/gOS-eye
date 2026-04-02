package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// --- handleTaskPanel one-shot HTTP tests ---

func taskPanelReq(s *Server, url, runID, process string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", url, nil)
	req.SetPathValue("runID", runID)
	req.SetPathValue("process", process)
	rec := httptest.NewRecorder()
	s.handleTaskPanel(rec, req)
	return rec
}

func TestHandleTaskPanel_ReturnsHTML(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 2, Name: "sayHello (2)", Process: "sayHello", Status: "COMPLETED"},
	})

	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/sayHello", "run-1", "sayHello")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "task-results-sayHello") {
		t.Errorf("expected task-results-sayHello div in body")
	}
	if !strings.Contains(body, "task-table-row") {
		t.Errorf("expected task-table-row in body")
	}
}

func TestHandleTaskPanel_UnknownRun_Returns404(t *testing.T) {
	store := state.NewStore()
	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/no-such-run/sayHello", "no-such-run", "sayHello")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTaskPanel_UnknownProcess_ReturnsEmpty(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "process_completed",
		Trace: &state.Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})

	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/NONEXISTENT", "run-1", "NONEXISTENT")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "task-results-NONEXISTENT") {
		t.Errorf("expected task-results-NONEXISTENT div")
	}
	if strings.Contains(body, "task-table-row") {
		t.Errorf("expected no task-table-row for unknown process")
	}
}

func TestHandleTaskPanel_DefaultPage(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID: i, Name: fmt.Sprintf("proc (%d)", i),
				Process: "proc", Status: "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/proc", "run-1", "proc")

	body := rec.Body.String()
	rowCount := strings.Count(body, "task-table-row")
	if rowCount != 10 {
		t.Errorf("expected 10 task-table-row on default page, got %d", rowCount)
	}
}

func TestHandleTaskPanel_Page2(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID: i, Name: fmt.Sprintf("proc (%d)", i),
				Process: "proc", Status: "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/proc?page=2", "run-1", "proc")

	body := rec.Body.String()
	rowCount := strings.Count(body, "task-table-row")
	if rowCount != 5 {
		t.Errorf("expected 5 task-table-row on page 2 of 15, got %d", rowCount)
	}
}

func TestHandleTaskPanel_FilterByName(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/proc?q=PATIENT", "run-1", "proc")

	body := rec.Body.String()
	rowCount := strings.Count(body, "task-table-row")
	if rowCount != 3 {
		t.Errorf("expected 3 task-table-row for q=PATIENT, got %d", rowCount)
	}
	if !strings.Contains(body, "PATIENT_001") {
		t.Errorf("expected PATIENT_001 in results")
	}
	if strings.Contains(body, "SAMPLE_A") {
		t.Errorf("expected SAMPLE_A to be filtered out")
	}
}

func TestHandleTaskPanel_FilterByStatus(t *testing.T) {
	store := populateFilterTasks()
	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/proc?status=FAILED", "run-1", "proc")

	body := rec.Body.String()
	rowCount := strings.Count(body, "task-table-row")
	if rowCount != 2 {
		t.Errorf("expected 2 task-table-row for status=FAILED, got %d", rowCount)
	}
	if !strings.Contains(body, "PATIENT_002") {
		t.Errorf("expected PATIENT_002 (FAILED) in results")
	}
	if strings.Contains(body, "PATIENT_001") {
		t.Errorf("expected PATIENT_001 (COMPLETED) to be filtered out")
	}
}

func TestHandleTaskPanel_SmallGroup_NoPagination(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 5; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID: i, Name: fmt.Sprintf("proc (%d)", i),
				Process: "proc", Status: "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/proc", "run-1", "proc")

	body := rec.Body.String()
	rowCount := strings.Count(body, "task-table-row")
	if rowCount != 5 {
		t.Errorf("expected 5 task-table-row for small group, got %d", rowCount)
	}
	if strings.Contains(body, "task-pagination") {
		t.Errorf("expected no pagination for ≤10 tasks")
	}
}

func TestHandleTaskPanel_PaginationControls(t *testing.T) {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for i := 1; i <= 15; i++ {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID: i, Name: fmt.Sprintf("proc (%d)", i),
				Process: "proc", Status: "COMPLETED",
			},
		})
	}

	s := serverForSSE(store)
	rec := taskPanelReq(s, "/tasks/run-1/proc?page=1", "run-1", "proc")

	body := rec.Body.String()
	if !strings.Contains(body, "task-pagination") {
		t.Errorf("expected task-pagination div for >10 tasks")
	}
	if !strings.Contains(body, "1\xe2\x80\x9310 of 15") && !strings.Contains(body, "1–10 of 15") {
		t.Errorf("expected '1–10 of 15' pagination info")
	}
	if !strings.Contains(body, "page=2") {
		t.Errorf("expected next button with page=2")
	}
	if !strings.Contains(body, "disabled") {
		t.Errorf("expected disabled first/prev buttons on page 1")
	}
}

// --- filterTasks unit tests ---

func makeTasks() []*state.Task {
	return []*state.Task{
		{Name: "PATIENT_0042 (1)", Tag: "sample_A", Status: "COMPLETED"},
		{Name: "PATIENT_0042 (2)", Tag: "sample_B", Status: "FAILED"},
		{Name: "align (1)", Tag: "genome_X", Status: "RUNNING"},
		{Name: "align (2)", Tag: "genome_Y", Status: "COMPLETED"},
		{Name: "merge (1)", Tag: "final", Status: "FAILED"},
	}
}

func TestFilterTasks_ByName(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "align", "")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks matching name 'align', got %d", len(got))
	}
	for _, tk := range got {
		if !strings.Contains(strings.ToLower(tk.Name), "align") {
			t.Errorf("task %q should contain 'align'", tk.Name)
		}
	}
}

func TestFilterTasks_ByTag(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "genome", "")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks matching tag 'genome', got %d", len(got))
	}
	for _, tk := range got {
		if !strings.Contains(strings.ToLower(tk.Tag), "genome") {
			t.Errorf("task tag %q should contain 'genome'", tk.Tag)
		}
	}
}

func TestFilterTasks_ByStatus(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "", "FAILED")
	if len(got) != 2 {
		t.Fatalf("expected 2 FAILED tasks, got %d", len(got))
	}
	for _, tk := range got {
		if tk.Status != "FAILED" {
			t.Errorf("expected FAILED status, got %q", tk.Status)
		}
	}
}

func TestFilterTasks_MultiStatus(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "", "FAILED,RUNNING")
	if len(got) != 3 {
		t.Fatalf("expected 3 tasks (2 FAILED + 1 RUNNING), got %d", len(got))
	}
	for _, tk := range got {
		if tk.Status != "FAILED" && tk.Status != "RUNNING" {
			t.Errorf("expected FAILED or RUNNING, got %q", tk.Status)
		}
	}
}

func TestFilterTasks_CaseInsensitive(t *testing.T) {
	tasks := makeTasks()
	// Lowercase query should match uppercase Name
	got := filterTasks(tasks, "patient", "")
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks matching 'patient' (case-insensitive), got %d", len(got))
	}

	// Status filter should also be case-insensitive
	got2 := filterTasks(tasks, "", "failed")
	if len(got2) != 2 {
		t.Fatalf("expected 2 tasks with status 'failed' (case-insensitive), got %d", len(got2))
	}
}

func TestFilterTasks_EmptyQ(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "", "COMPLETED")
	if len(got) != 2 {
		t.Fatalf("expected 2 COMPLETED tasks when q is empty, got %d", len(got))
	}

	// Totally empty filters should return all
	all := filterTasks(tasks, "", "")
	if len(all) != len(tasks) {
		t.Fatalf("expected all %d tasks when both filters empty, got %d", len(tasks), len(all))
	}
}

func TestFilterTasks_EmptyStatus(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "merge", "")
	if len(got) != 1 {
		t.Fatalf("expected 1 task matching 'merge' with empty statusFilter, got %d", len(got))
	}
	if got[0].Name != "merge (1)" {
		t.Errorf("expected 'merge (1)', got %q", got[0].Name)
	}
}

func TestFilterTasks_BothFilters(t *testing.T) {
	tasks := makeTasks()
	// q="patient" matches 2 tasks, statusFilter="FAILED" matches 2 tasks,
	// intersection is 1: PATIENT_0042 (2) with Status=FAILED
	got := filterTasks(tasks, "patient", "FAILED")
	if len(got) != 1 {
		t.Fatalf("expected 1 task matching both filters, got %d", len(got))
	}
	if got[0].Name != "PATIENT_0042 (2)" {
		t.Errorf("expected 'PATIENT_0042 (2)', got %q", got[0].Name)
	}
	if got[0].Status != "FAILED" {
		t.Errorf("expected FAILED, got %q", got[0].Status)
	}
}

func TestFilterTasks_NoMatch(t *testing.T) {
	tasks := makeTasks()
	got := filterTasks(tasks, "nonexistent", "")
	if len(got) != 0 {
		t.Fatalf("expected 0 tasks for 'nonexistent', got %d", len(got))
	}

	got2 := filterTasks(tasks, "", "CACHED")
	if len(got2) != 0 {
		t.Fatalf("expected 0 tasks for status 'CACHED', got %d", len(got2))
	}

	got3 := filterTasks(tasks, "align", "FAILED")
	if len(got3) != 0 {
		t.Fatalf("expected 0 tasks for 'align' + 'FAILED', got %d", len(got3))
	}
}

// --- Tests for renderTaskPanelHTML with filter/status support ---

// populateTaskPanel creates a store with a run and multiple tasks for the given process.
func populateTaskPanel(process string, tasks []struct {
	id      int
	name    string
	tag     string
	status  string
}) *state.Store {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	for _, tk := range tasks {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  tk.id,
				Name:    tk.name,
				Process: process,
				Tag:     tk.tag,
				Status:  tk.status,
			},
		})
	}
	return store
}

func TestRenderTaskPanelHTML_FilterApplied(t *testing.T) {
	store := populateTaskPanel("align", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "align (sample_alpha)", "sample_alpha", "COMPLETED"},
		{2, "align (sample_beta)", "sample_beta", "COMPLETED"},
		{3, "align (sample_alpha2)", "sample_alpha2", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "align", "alpha", "", 1)

	if strings.Contains(html, "sample_beta") {
		t.Error("expected 'sample_beta' task to be filtered out by q='alpha'")
	}
	if !strings.Contains(html, "sample_alpha)") {
		t.Error("expected 'sample_alpha' task to be present")
	}
	if !strings.Contains(html, "sample_alpha2)") {
		t.Error("expected 'sample_alpha2' task to be present")
	}
}

func TestRenderTaskPanelHTML_StatusFilterApplied(t *testing.T) {
	store := populateTaskPanel("proc", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "proc (item_one)", "item_one", "COMPLETED"},
		{2, "proc (item_two)", "item_two", "FAILED"},
		{3, "proc (item_three)", "item_three", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "proc", "", "FAILED", 1)

	if !strings.Contains(html, "item_two") {
		t.Error("expected FAILED task with tag 'item_two' to be present")
	}
	if strings.Contains(html, "item_one") {
		t.Error("expected COMPLETED task 'item_one' to be filtered out")
	}
	if strings.Contains(html, "item_three") {
		t.Error("expected COMPLETED task 'item_three' to be filtered out")
	}
}

func TestRenderTaskPanelHTML_FilterAndPaginate(t *testing.T) {
	var tasks []struct {
		id      int
		name    string
		tag     string
		status  string
	}
	for i := 1; i <= 15; i++ {
		tasks = append(tasks, struct {
			id      int
			name    string
			tag     string
			status  string
		}{i, fmt.Sprintf("proc (match_%02d)", i), "match", "COMPLETED"})
	}
	for i := 16; i <= 20; i++ {
		tasks = append(tasks, struct {
			id      int
			name    string
			tag     string
			status  string
		}{i, fmt.Sprintf("proc (other_%02d)", i), "other", "COMPLETED"})
	}
	store := populateTaskPanel("proc", tasks)
	s := serverForSSE(store)

	html1 := s.renderTaskPanelHTML("run-1", "proc", "match", "", 1)
	if !strings.Contains(html1, "task-pagination") {
		t.Error("expected pagination controls on page 1 of filtered results")
	}
	if strings.Contains(html1, "other_") {
		t.Error("expected 'other' tasks to be filtered out")
	}

	html2 := s.renderTaskPanelHTML("run-1", "proc", "match", "", 2)
	if !strings.Contains(html2, "task-pagination") {
		t.Error("expected pagination controls on page 2")
	}
	if !strings.Contains(html2, "11–15 of 15") {
		t.Errorf("expected '11–15 of 15' pagination info on page 2, got:\n%s", html2)
	}
}

func TestRenderTaskPanelHTML_NoResults(t *testing.T) {
	store := populateTaskPanel("proc", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "proc (a)", "a", "COMPLETED"},
		{2, "proc (b)", "b", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "proc", "nonexistent", "", 1)

	if !strings.Contains(html, "No matching tasks") {
		t.Error("expected 'No matching tasks' message when filter matches nothing")
	}
	if strings.Contains(html, "task-table") {
		t.Error("expected no task-table when filter matches nothing")
	}
}

func TestRenderTaskPanelHTML_IncludesFilterBar(t *testing.T) {
	store := populateTaskPanel("sayHello", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "sayHello (1)", "tag1", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "sayHello", "", "", 1)

	if !strings.Contains(html, `task-filter-sayHello`) {
		t.Error("expected task-filter-bar div with process-specific ID")
	}
	if !strings.Contains(html, `task-filter-bar`) {
		t.Error("expected task-filter-bar class in output")
	}
}

func TestRenderTaskPanelHTML_ResultsDiv(t *testing.T) {
	store := populateTaskPanel("sayHello", []struct {
		id      int
		name    string
		tag     string
		status  string
	}{
		{1, "sayHello (1)", "tag1", "COMPLETED"},
	})
	s := serverForSSE(store)

	html := s.renderTaskPanelHTML("run-1", "sayHello", "", "", 1)

	if !strings.Contains(html, `id="task-results-sayHello"`) {
		t.Errorf("expected task-results-sayHello div, got:\n%s", html)
	}
	// Response should NOT contain outer task-panel wrapper (inner mode targets it)
	if strings.Contains(html, `id="task-panel-`) {
		t.Errorf("response should not contain task-panel wrapper (inner mode), got:\n%s", html)
	}
}

// --- handleTaskPanel filter integration helpers ---

func populateFilterTasks() *state.Store {
	store := state.NewStore()
	store.HandleEvent(state.WebhookEvent{
		RunName: "test_run", RunID: "run-1", Event: "started",
		UTCTime: "2024-01-01T00:00:00Z",
	})
	tasks := []struct {
		id     int
		name   string
		tag    string
		status string
	}{
		{1, "proc (PATIENT_001)", "PATIENT_001", "COMPLETED"},
		{2, "proc (PATIENT_002)", "PATIENT_002", "FAILED"},
		{3, "proc (PATIENT_003)", "PATIENT_003", "RUNNING"},
		{4, "proc (SAMPLE_A)", "SAMPLE_A", "COMPLETED"},
		{5, "proc (SAMPLE_B)", "SAMPLE_B", "FAILED"},
		{6, "proc (CONTROL_X)", "CONTROL_X", "COMPLETED"},
	}
	for _, tk := range tasks {
		store.HandleEvent(state.WebhookEvent{
			RunName: "test_run", RunID: "run-1", Event: "process_completed",
			Trace: &state.Trace{
				TaskID:  tk.id,
				Name:    tk.name,
				Process: "proc",
				Tag:     tk.tag,
				Status:  tk.status,
			},
		})
	}
	return store
}
