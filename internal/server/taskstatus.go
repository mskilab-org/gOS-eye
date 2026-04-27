package server

import (
	"fmt"
	"html"
	"strings"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

type taskStatusFilterOption struct {
	Value string
	Label string
}

// taskCountsAsDone determines whether a task should count as done for progress,
// DAG completion, and process completion. Completed and cached tasks count as done.
func taskCountsAsDone(status state.TaskStatus) bool {
	return status == state.TaskStatusCompleted || status == state.TaskStatusCached
}

// taskHasFinishedExecution determines whether a task should render as finished in
// task tables. Completed, failed, and cached tasks are terminal for display.
func taskHasFinishedExecution(status state.TaskStatus) bool {
	return status == state.TaskStatusCompleted || status == state.TaskStatusFailed || status == state.TaskStatusCached
}

// taskContributesResources determines whether a task should contribute runtime/resource
// totals and bar scaling. Cached tasks are excluded because they were not executed.
func taskContributesResources(task *state.Task) bool {
	if task == nil {
		return false
	}
	return task.Status != state.TaskStatusCached && taskHasFinishedExecution(task.Status)
}

// taskStatusFilterOptions returns the statuses shown in the task-panel filter dropdown,
// including the cached-task filter.
func taskStatusFilterOptions() []taskStatusFilterOption {
	return []taskStatusFilterOption{
		{"", "All"},
		{"FAILED", "Failed"},
		{"RUNNING", "Running"},
		{"COMPLETED", "Completed"},
		{"CACHED", "Cached"},
	}
}

// cachedTaskSummaryPart renders the cached-task summary fragment for run summaries.
// Non-positive counts produce no fragment so zero cached tasks stay hidden.
func cachedTaskSummaryPart(cached int) string {
	if cached <= 0 {
		return ""
	}
	return fmt.Sprintf("%d cached", cached)
}

type countLabel struct {
	count int
	label string
}

func joinedPositiveCountLabels(parts ...countLabel) string {
	phrases := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.count <= 0 {
			continue
		}
		phrases = append(phrases, fmt.Sprintf("%d %s", part.count, part.label))
	}
	return strings.Join(phrases, ", ")
}

func pluralLabel(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func renderTraceImportNoticeBlock(kind, title, bodyHTML string) string {
	if bodyHTML == "" {
		return ""
	}
	return fmt.Sprintf(
		`<div class="trace-import-notice trace-import-%s" role="status"><strong>%s:</strong> %s</div>`,
		kind,
		html.EscapeString(title),
		bodyHTML,
	)
}

// cachedWorkdirResolutionSummary describes aggregate cached workdir recovery outcomes for run-level notices.
func cachedWorkdirResolutionSummary(traceImport state.TraceImportState) string {
	unresolved := traceImport.CachedWorkdirsUnresolved
	ambiguous := traceImport.CachedWorkdirsAmbiguous
	if unresolved <= 0 && ambiguous <= 0 {
		return ""
	}

	summary := joinedPositiveCountLabels(
		countLabel{count: unresolved, label: "unresolved"},
		countLabel{count: ambiguous, label: "ambiguous"},
	)

	explicit := traceImport.CachedWorkdirsExplicit
	hashResolved := traceImport.CachedWorkdirsHashResolved
	resolved := 0
	if explicit > 0 {
		resolved += explicit
	}
	if hashResolved > 0 {
		resolved += hashResolved
	}
	if resolved > 0 {
		detail := joinedPositiveCountLabels(
			countLabel{count: explicit, label: "explicit"},
			countLabel{count: hashResolved, label: "hash-resolved"},
		)
		if summary != "" {
			summary += ", "
		}
		summary += fmt.Sprintf("%d resolved (%s)", resolved, detail)
	}

	return "Cached workdir recovery: " + summary + "."
}

// renderCachedWorkdirResolutionNotice renders non-fatal cached workdir recovery warnings without marking the run failed.
func renderCachedWorkdirResolutionNotice(traceImport state.TraceImportState) string {
	summary := cachedWorkdirResolutionSummary(traceImport)
	warnings := make([]string, 0, len(traceImport.CachedWorkdirResolutionWarnings))
	for _, warning := range traceImport.CachedWorkdirResolutionWarnings {
		if warning == "" {
			continue
		}
		warnings = append(warnings, warning)
	}
	if summary == "" && len(warnings) == 0 {
		return ""
	}

	body := `Cached workdir recovery warnings.`
	if summary != "" {
		body = html.EscapeString(summary)
	}

	if len(warnings) > 0 {
		warningLabel := pluralLabel(len(warnings), "warning", "warnings")
		body += fmt.Sprintf(` <details><summary>%d cached workdir %s</summary><ul>`, len(warnings), warningLabel)
		for _, warning := range warnings {
			body += fmt.Sprintf(`<li>%s</li>`, html.EscapeString(warning))
		}
		body += `</ul></details>`
	}

	return renderTraceImportNoticeBlock("warning", "Trace import notice", body)
}

// renderTraceImportNotice renders trace import warnings and cached-task import notices.
// It returns an empty string when there is nothing to report.
func renderTraceImportNotice(run *state.Run) string {
	if run == nil {
		return ""
	}

	warning := run.TraceImport.Warning
	cachedTasks := run.TraceImport.CachedTasks
	workdirNotice := renderCachedWorkdirResolutionNotice(run.TraceImport)
	if warning == "" && cachedTasks <= 0 && workdirNotice == "" {
		return ""
	}

	notice := `<div class="trace-import-notices">`
	if warning != "" {
		notice += renderTraceImportNoticeBlock("warning", "Trace import warning", html.EscapeString(warning))
	}
	if cachedTasks > 0 {
		taskLabel := pluralLabel(cachedTasks, "task", "tasks")

		source := "the trace file"
		if run.TraceImport.ResolvedPath != "" {
			source = fmt.Sprintf(`trace file <code>%s</code>`, html.EscapeString(run.TraceImport.ResolvedPath))
		} else if run.TraceImport.RequestedPath != "" {
			source = fmt.Sprintf(`trace file <code>%s</code>`, html.EscapeString(run.TraceImport.RequestedPath))
		}

		notice += renderTraceImportNoticeBlock(
			"info",
			"Trace import",
			fmt.Sprintf("Imported %d cached %s from %s.", cachedTasks, taskLabel, source),
		)
	}
	notice += workdirNotice
	notice += `</div>`
	return notice
}
