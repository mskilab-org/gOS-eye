package server

import (
	"html"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestTaskCountsAsDone_KnownStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status state.TaskStatus
		want   bool
	}{
		{name: "completed", status: state.TaskStatusCompleted, want: true},
		{name: "cached", status: state.TaskStatusCached, want: true},
		{name: "failed", status: state.TaskStatusFailed, want: false},
		{name: "running", status: state.TaskStatusRunning, want: false},
		{name: "submitted", status: state.TaskStatusSubmitted, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskCountsAsDone(tt.status)
			if got != tt.want {
				t.Fatalf("taskCountsAsDone(%q) = %t, want %t", tt.status, got, tt.want)
			}
		})
	}
}

func TestTaskCountsAsDone_UnknownStatus(t *testing.T) {
	got := taskCountsAsDone(state.TaskStatus(""))
	if got {
		t.Fatalf("taskCountsAsDone(\"\") = true, want false")
	}
}

func TestTaskHasFinishedExecution_KnownStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status state.TaskStatus
		want   bool
	}{
		{name: "completed", status: state.TaskStatusCompleted, want: true},
		{name: "cached", status: state.TaskStatusCached, want: true},
		{name: "failed", status: state.TaskStatusFailed, want: true},
		{name: "running", status: state.TaskStatusRunning, want: false},
		{name: "submitted", status: state.TaskStatusSubmitted, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskHasFinishedExecution(tt.status)
			if got != tt.want {
				t.Fatalf("taskHasFinishedExecution(%q) = %t, want %t", tt.status, got, tt.want)
			}
		})
	}
}

func TestTaskHasFinishedExecution_UnknownStatus(t *testing.T) {
	got := taskHasFinishedExecution(state.TaskStatus(""))
	if got {
		t.Fatalf("taskHasFinishedExecution(\"\") = true, want false")
	}
}

func TestTaskContributesResources_KnownStatusesAndNil(t *testing.T) {
	tests := []struct {
		name string
		task *state.Task
		want bool
	}{
		{name: "nil task", task: nil, want: false},
		{name: "completed execution", task: &state.Task{Status: state.TaskStatusCompleted}, want: true},
		{name: "failed execution", task: &state.Task{Status: state.TaskStatusFailed}, want: true},
		{name: "cached history", task: &state.Task{Status: state.TaskStatusCached}, want: false},
		{name: "running", task: &state.Task{Status: state.TaskStatusRunning}, want: false},
		{name: "submitted", task: &state.Task{Status: state.TaskStatusSubmitted}, want: false},
		{name: "unknown", task: &state.Task{Status: state.TaskStatus("")}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskContributesResources(tt.task)
			if got != tt.want {
				t.Fatalf("taskContributesResources(%#v) = %t, want %t", tt.task, got, tt.want)
			}
		})
	}
}

func TestComputeResourceMetrics_ExcludesCachedFromCurrentRunMaxima(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 1, Status: state.TaskStatusCompleted, Duration: 1000, CPUPercent: 10, PeakRSS: 1024},
		{TaskID: 2, Status: state.TaskStatusFailed, Duration: 2000, CPUPercent: 20, PeakRSS: 2048},
		{TaskID: 3, Status: state.TaskStatusCached, Duration: 9999, CPUPercent: 99, PeakRSS: 9999},
		nil,
	}

	got := computeResourceMetrics(tasks)

	if !got.hasData {
		t.Fatal("expected executed completed/failed tasks to provide current-run resource data")
	}
	if got.duration != 2000 {
		t.Fatalf("duration max = %d, want 2000", got.duration)
	}
	if got.cpu != 20 {
		t.Fatalf("cpu max = %.1f, want 20.0", got.cpu)
	}
	if got.peakRSS != 2048 {
		t.Fatalf("peakRSS max = %d, want 2048", got.peakRSS)
	}
}

func TestComputeResourceMetrics_CachedOnlyHasNoCurrentRunData(t *testing.T) {
	tasks := []*state.Task{
		{TaskID: 1, Status: state.TaskStatusCached, Duration: 5000, CPUPercent: 75, PeakRSS: 2097152},
		nil,
	}

	got := computeResourceMetrics(tasks)

	if got.hasData {
		t.Fatalf("cached-only metrics should not be marked as current-run data: %#v", got)
	}
	if got.duration != 0 || got.cpu != 0 || got.peakRSS != 0 {
		t.Fatalf("cached-only metrics should not contribute maxima, got %#v", got)
	}
}

func TestTaskStatusFilterOptions_IncludesCachedAndLegacyOptions(t *testing.T) {
	got := taskStatusFilterOptions()
	want := []taskStatusFilterOption{
		{Value: "", Label: "All"},
		{Value: string(state.TaskStatusFailed), Label: "Failed"},
		{Value: string(state.TaskStatusRunning), Label: "Running"},
		{Value: string(state.TaskStatusCompleted), Label: "Completed"},
		{Value: string(state.TaskStatusCached), Label: "Cached"},
	}

	if len(got) != len(want) {
		t.Fatalf("taskStatusFilterOptions returned %d options, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("taskStatusFilterOptions()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestRenderRunDetail_CachedCountsTowardProgress(t *testing.T) {
	run := &state.Run{
		RunName: "run1",
		RunID:   "run1",
		Status:  "started",
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Name: "align (1)", Process: "align", Status: state.TaskStatusCached},
			2: {TaskID: 2, Name: "align (2)", Process: "align", Status: state.TaskStatusRunning},
		},
	}

	srv := &Server{store: state.NewStore(), broker: NewBroker()}
	got := srv.renderRunDetail(run)

	if !strings.Contains(got, "1/2 (50%)") {
		t.Errorf("expected cached task to count toward progress, got:\n%s", got)
	}
	if !strings.Contains(got, `<span class="process-table-counts">1/2</span>`) {
		t.Errorf("expected cached task to count toward process completion, got:\n%s", got)
	}
}

func TestCachedTaskSummaryPart_NonPositiveCountsReturnEmpty(t *testing.T) {
	tests := []struct {
		name   string
		cached int
	}{
		{name: "zero", cached: 0},
		{name: "negative", cached: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cachedTaskSummaryPart(tt.cached)
			if got != "" {
				t.Fatalf("cachedTaskSummaryPart(%d) = %q, want empty", tt.cached, got)
			}
		})
	}
}

func TestCachedTaskSummaryPart_PositiveCountReturnsDistinctCachedLabel(t *testing.T) {
	tests := []struct {
		name   string
		cached int
		want   string
	}{
		{name: "one", cached: 1, want: "1 cached"},
		{name: "many", cached: 3, want: "3 cached"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cachedTaskSummaryPart(tt.cached)
			if got != tt.want {
				t.Fatalf("cachedTaskSummaryPart(%d) = %q, want %q", tt.cached, got, tt.want)
			}
			if strings.Contains(got, "completed") {
				t.Fatalf("cached summary label should be distinct from completed counts, got %q", got)
			}
		})
	}
}

func TestCachedWorkdirResolutionSummary_NoActionableOutcomesReturnsEmpty(t *testing.T) {
	tests := []struct {
		name        string
		traceImport state.TraceImportState
	}{
		{name: "empty trace import state", traceImport: state.TraceImportState{}},
		{name: "only explicit workdirs", traceImport: state.TraceImportState{CachedWorkdirsExplicit: 2}},
		{name: "only hash-resolved workdirs", traceImport: state.TraceImportState{CachedWorkdirsHashResolved: 3}},
		{name: "only non-positive warning counts", traceImport: state.TraceImportState{CachedWorkdirsUnresolved: -1, CachedWorkdirsAmbiguous: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cachedWorkdirResolutionSummary(tt.traceImport)
			if got != "" {
				t.Fatalf("cachedWorkdirResolutionSummary(%#v) = %q, want empty", tt.traceImport, got)
			}
		})
	}
}

func TestCachedWorkdirResolutionSummary_ActionableOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		traceImport state.TraceImportState
		want        string
	}{
		{
			name:        "unresolved workdirs",
			traceImport: state.TraceImportState{CachedWorkdirsUnresolved: 2},
			want:        "Cached workdir recovery: 2 unresolved.",
		},
		{
			name:        "ambiguous workdir",
			traceImport: state.TraceImportState{CachedWorkdirsAmbiguous: 1},
			want:        "Cached workdir recovery: 1 ambiguous.",
		},
		{
			name: "warning outcomes with resolved context",
			traceImport: state.TraceImportState{
				CachedWorkdirsExplicit:     4,
				CachedWorkdirsHashResolved: 3,
				CachedWorkdirsUnresolved:   2,
				CachedWorkdirsAmbiguous:    1,
			},
			want: "Cached workdir recovery: 2 unresolved, 1 ambiguous, 7 resolved (4 explicit, 3 hash-resolved).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cachedWorkdirResolutionSummary(tt.traceImport)
			if got != tt.want {
				t.Fatalf("cachedWorkdirResolutionSummary(%#v) = %q, want %q", tt.traceImport, got, tt.want)
			}
		})
	}
}

func TestRenderCachedWorkdirResolutionNotice_NoSummaryOrWarningsReturnsEmpty(t *testing.T) {
	tests := []struct {
		name        string
		traceImport state.TraceImportState
	}{
		{name: "empty trace import state", traceImport: state.TraceImportState{}},
		{name: "only explicit workdirs", traceImport: state.TraceImportState{CachedWorkdirsExplicit: 2}},
		{name: "only hash-resolved workdirs", traceImport: state.TraceImportState{CachedWorkdirsHashResolved: 3}},
		{name: "empty warning strings do not create a notice", traceImport: state.TraceImportState{CachedWorkdirResolutionWarnings: []string{"", ""}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCachedWorkdirResolutionNotice(tt.traceImport)
			if got != "" {
				t.Fatalf("renderCachedWorkdirResolutionNotice(%#v) = %q, want empty", tt.traceImport, got)
			}
		})
	}
}

func TestRenderCachedWorkdirResolutionNotice_RendersNonFatalSummaryNotice(t *testing.T) {
	traceImport := state.TraceImportState{
		CachedWorkdirsExplicit:     4,
		CachedWorkdirsHashResolved: 3,
		CachedWorkdirsUnresolved:   2,
		CachedWorkdirsAmbiguous:    1,
	}
	summary := cachedWorkdirResolutionSummary(traceImport)

	got := renderCachedWorkdirResolutionNotice(traceImport)

	if got == "" {
		t.Fatal("expected cached workdir recovery notice, got empty string")
	}
	if !strings.Contains(got, `trace-import-notice`) || !strings.Contains(got, `trace-import-warning`) {
		t.Fatalf("expected non-fatal trace import warning notice classes, got:\n%s", got)
	}
	if !strings.Contains(got, `role="status"`) {
		t.Fatalf("expected status role, got:\n%s", got)
	}
	if strings.Contains(got, `error-message`) {
		t.Fatalf("cached workdir recovery notice should not render as a run error, got:\n%s", got)
	}
	if !strings.Contains(got, html.EscapeString(summary)) {
		t.Fatalf("expected escaped summary %q, got:\n%s", html.EscapeString(summary), got)
	}
	if strings.Contains(got, `<details`) {
		t.Fatalf("summary-only notice should not add warning details, got:\n%s", got)
	}
}

func TestRenderCachedWorkdirResolutionNotice_IncludesEscapedWarningsInConciseDetails(t *testing.T) {
	warnings := []string{
		`cached workdir unresolved: cannot scan "/tmp/<work>" & gave up`,
		`cached workdir ambiguous: 2 directories matched "88/fa3254"`,
	}
	traceImport := state.TraceImportState{
		CachedWorkdirsUnresolved:        1,
		CachedWorkdirsAmbiguous:         1,
		CachedWorkdirResolutionWarnings: warnings,
	}

	got := renderCachedWorkdirResolutionNotice(traceImport)

	if !strings.Contains(got, `<details`) || !strings.Contains(got, `</details>`) {
		t.Fatalf("expected warning details to stay concise in a collapsible details block, got:\n%s", got)
	}
	if !strings.Contains(got, `2 cached workdir warnings`) {
		t.Fatalf("expected concise warning count summary, got:\n%s", got)
	}
	for _, warning := range warnings {
		escaped := html.EscapeString(warning)
		if !strings.Contains(got, escaped) {
			t.Fatalf("expected escaped warning %q, got:\n%s", escaped, got)
		}
	}
	if strings.Contains(got, `/tmp/<work>`) || strings.Contains(got, `"88/fa3254"`) {
		t.Fatalf("warning details contain unescaped warning text, got:\n%s", got)
	}
}

func TestRenderCachedWorkdirResolutionNotice_WarningsOnlyStillRenderNotice(t *testing.T) {
	warning := `cached workdir unresolved: blank task hash`
	traceImport := state.TraceImportState{CachedWorkdirResolutionWarnings: []string{warning}}

	got := renderCachedWorkdirResolutionNotice(traceImport)

	if got == "" {
		t.Fatal("expected warning-only cached workdir recovery notice, got empty string")
	}
	if !strings.Contains(got, "Cached workdir recovery") {
		t.Fatalf("expected cached workdir recovery label for warning-only notice, got:\n%s", got)
	}
	if !strings.Contains(got, `1 cached workdir warning`) {
		t.Fatalf("expected singular warning count, got:\n%s", got)
	}
	if !strings.Contains(got, html.EscapeString(warning)) {
		t.Fatalf("expected escaped warning %q, got:\n%s", html.EscapeString(warning), got)
	}
}

func TestRenderTraceImportNotice_NoWarningOrImportedCachedTasksReturnsEmpty(t *testing.T) {
	tests := []struct {
		name string
		run  *state.Run
	}{
		{name: "nil run", run: nil},
		{name: "empty trace import state", run: &state.Run{}},
		{name: "zero cached tasks after import", run: &state.Run{TraceImport: state.TraceImportState{Imported: true, CachedTasks: 0}}},
		{name: "negative cached task count", run: &state.Run{TraceImport: state.TraceImportState{CachedTasks: -1}}},
		{name: "cached workdir outcomes that produce no notice", run: &state.Run{TraceImport: state.TraceImportState{CachedWorkdirsExplicit: 2, CachedWorkdirResolutionWarnings: []string{""}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTraceImportNotice(tt.run)
			if got != "" {
				t.Fatalf("renderTraceImportNotice() = %q, want empty", got)
			}
		})
	}
}

func TestRenderTraceImportNotice_WarningIsEscaped(t *testing.T) {
	warning := `trace <missing> & "unsafe" path`
	run := &state.Run{TraceImport: state.TraceImportState{Warning: warning}}

	got := renderTraceImportNotice(run)

	if got == "" {
		t.Fatal("expected trace import warning notice, got empty string")
	}
	if !strings.Contains(got, `trace-import-warning`) {
		t.Fatalf("expected warning notice class, got:\n%s", got)
	}
	if !strings.Contains(got, "Trace import warning") {
		t.Fatalf("expected warning label, got:\n%s", got)
	}
	if !strings.Contains(got, html.EscapeString(warning)) {
		t.Fatalf("expected escaped warning %q, got:\n%s", html.EscapeString(warning), got)
	}
	if strings.Contains(got, warning) || strings.Contains(got, "<missing>") {
		t.Fatalf("warning notice contains unescaped warning text, got:\n%s", got)
	}
}

func TestRenderTraceImportNotice_CachedTaskImportSummaryIsNonError(t *testing.T) {
	path := `/tmp/trace & cached.txt`
	run := &state.Run{TraceImport: state.TraceImportState{ResolvedPath: path, Imported: true, CachedTasks: 2}}

	got := renderTraceImportNotice(run)

	if got == "" {
		t.Fatal("expected cached task import summary, got empty string")
	}
	if !strings.Contains(got, `trace-import-info`) {
		t.Fatalf("expected non-error import summary class, got:\n%s", got)
	}
	if strings.Contains(got, `error-message`) || strings.Contains(got, `trace-import-warning`) {
		t.Fatalf("cached task summary should not render as an error/warning, got:\n%s", got)
	}
	if !strings.Contains(got, "Imported 2 cached tasks") {
		t.Fatalf("expected imported cached task count, got:\n%s", got)
	}
	if !strings.Contains(got, html.EscapeString(path)) {
		t.Fatalf("expected escaped trace path %q, got:\n%s", html.EscapeString(path), got)
	}
}

func TestRenderTraceImportNotice_CachedTaskImportSummaryUsesSingularTask(t *testing.T) {
	run := &state.Run{TraceImport: state.TraceImportState{Imported: true, CachedTasks: 1}}

	got := renderTraceImportNotice(run)

	if !strings.Contains(got, "Imported 1 cached task") {
		t.Fatalf("expected singular cached task summary, got:\n%s", got)
	}
	if strings.Contains(got, "Imported 1 cached tasks") {
		t.Fatalf("cached task summary should use singular label for one task, got:\n%s", got)
	}
}

func TestRenderTraceImportNotice_WorkdirResolutionNoticeOnlyStillUsesWrapper(t *testing.T) {
	warning := `cached workdir unresolved: cannot scan "/tmp/<work>" & gave up`
	run := &state.Run{TraceImport: state.TraceImportState{
		CachedWorkdirsUnresolved:        1,
		CachedWorkdirResolutionWarnings: []string{warning},
	}}

	got := renderTraceImportNotice(run)

	if got == "" {
		t.Fatal("expected cached workdir resolution notice, got empty string")
	}
	if !strings.HasPrefix(got, `<div class="trace-import-notices"><div class="trace-import-notice trace-import-warning"`) {
		t.Fatalf("expected cached workdir notice inside the trace import notices wrapper, got:\n%s", got)
	}
	if !strings.HasSuffix(got, `</div></div>`) {
		t.Fatalf("expected wrapper to close after cached workdir notice, got:\n%s", got)
	}
	if strings.Count(got, `trace-import-notices`) != 1 {
		t.Fatalf("expected exactly one trace import notices wrapper, got:\n%s", got)
	}
	if !strings.Contains(got, html.EscapeString(warning)) {
		t.Fatalf("expected escaped workdir warning %q, got:\n%s", html.EscapeString(warning), got)
	}
	if strings.Contains(got, `/tmp/<work>`) {
		t.Fatalf("workdir warning contains unescaped dynamic text, got:\n%s", got)
	}
}

func TestRenderTraceImportNotice_RendersWarningCachedTaskAndWorkdirNoticesTogether(t *testing.T) {
	run := &state.Run{TraceImport: state.TraceImportState{
		Warning:                    "trace path was not explicit; using discovered trace file",
		ResolvedPath:               "/tmp/trace.txt",
		Imported:                   true,
		CachedTasks:                3,
		CachedWorkdirsUnresolved:   2,
		CachedWorkdirsAmbiguous:    1,
		CachedWorkdirsHashResolved: 4,
	}}

	got := renderTraceImportNotice(run)

	want := `<div class="trace-import-notices">` +
		`<div class="trace-import-notice trace-import-warning" role="status"><strong>Trace import warning:</strong> trace path was not explicit; using discovered trace file</div>` +
		`<div class="trace-import-notice trace-import-info" role="status"><strong>Trace import:</strong> Imported 3 cached tasks from trace file <code>/tmp/trace.txt</code>.</div>` +
		`<div class="trace-import-notice trace-import-warning" role="status"><strong>Trace import notice:</strong> Cached workdir recovery: 2 unresolved, 1 ambiguous, 4 resolved (4 hash-resolved).</div>` +
		`</div>`
	if got != want {
		t.Fatalf("renderTraceImportNotice() =\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTraceImportNotice_RendersWarningAndCachedTaskSummaryTogether(t *testing.T) {
	run := &state.Run{TraceImport: state.TraceImportState{
		Warning:     "trace path was not explicit; using discovered trace file",
		Imported:    true,
		CachedTasks: 3,
	}}

	got := renderTraceImportNotice(run)

	if !strings.Contains(got, `trace-import-warning`) {
		t.Fatalf("expected warning notice, got:\n%s", got)
	}
	if !strings.Contains(got, `trace-import-info`) {
		t.Fatalf("expected cached task import summary, got:\n%s", got)
	}
	if !strings.Contains(got, "Imported 3 cached tasks") {
		t.Fatalf("expected cached task count, got:\n%s", got)
	}
}

func TestRenderDAG_StatusDerivation_CachedCountsAsCompleted(t *testing.T) {
	layout := &dag.Layout{
		Nodes:      []dag.NodeLayout{{Name: "ALIGN", Layer: 0, Index: 0}},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}

	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "ALIGN", Status: state.TaskStatusCached},
			2: {TaskID: 2, Process: "ALIGN", Status: state.TaskStatusCompleted},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-completed") {
		t.Errorf("expected cached tasks to allow DAG completion, got: %s", got)
	}
	if !strings.Contains(got, "2/2") {
		t.Errorf("expected cached task to count toward completed/total, got: %s", got)
	}
}
