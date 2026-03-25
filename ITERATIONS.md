# nextflow-monitor — Future Iterations

## Iteration 4: SQLite Persistence

**Goal**: Survive server restarts — no more losing run history when the process stops.

**Deliverables**:
- Store incoming webhook events to SQLite on disk
- Replay stored events into in-memory state on startup
- Configurable DB path (flag/env var, default `./nextflow-monitor.db`)
- Retain current in-memory model as the live read path; SQLite is write-ahead + replay source

**Acceptance test**: Start server, run a pipeline, stop server, restart — previous run(s) appear in the dashboard immediately.

---

## Iteration 5: Resource Charts

**Goal**: Time-series CPU/memory visualisation per task for spotting bottlenecks.

**Deliverables**:
- Accumulate resource trace data (CPU%, RSS, vmem) from completed task events
- Render per-task resource charts in the task detail panel
- Lightweight client-side charting (small lib or inline SVG) — no heavy JS framework

**Acceptance test**: Run a pipeline with varied resource usage. Expand a task, see CPU and memory plotted over time.

---

## Iteration 6: Per-Run SSE + Inline Logs

**Goal**: Stop rendering all runs on every event. Enable inline log viewing without payload explosion.

The current architecture renders sidebar + all runs' full HTML on every webhook event and broadcasts to all SSE subscribers. This blocks inline log content (reading logs for all tasks in all runs is too expensive) and wastes bandwidth as the number of runs grows.

### Part A: Per-Run Rendering

**Deliverables**:
- On webhook: determine affected runID, render and send only that run's detail (not all runs)
- Initial SSE connect: send sidebar + latest run detail only
- New endpoint `GET /run/{id}` returns SSE with a single run's detail fragment
- When user clicks a different run in the sidebar, `@get('/run/{id}')` fetches that run's detail on demand
- Sidebar remains a separate morph target, updated on every event (lightweight: just names + statuses)

**Acceptance test**: Start server, run two pipelines sequentially. Switch between runs in sidebar — each loads on demand. Webhook events only send the affected run's HTML.

### Part B: Inline Logs

**Deliverables**:
- Remove the separate `/logs` endpoint, AbortController signal, and floating log panel
- Read `.command.log` / `.command.err` server-side during `renderTaskTable` and include content in the task detail panel
- Truncate to last N lines for large files (tail behavior)
- Graceful degradation when workdir is inaccessible (show message instead of content)
- Logs update on every webhook re-render of the affected run

**Acceptance test**: Run a pipeline, expand a task — log content appears inline in the detail panel. No separate "View Logs" button needed. Switching tasks shows different logs immediately.

**Constraint**: Requires server to have filesystem access to the Nextflow work directory.

---

## Iteration 7: Retry/Resume Controls

**Goal**: Trigger Nextflow `-resume` from the dashboard for failed runs.

**Deliverables**:
- "Resume" button on failed runs
- Shell-out to `nextflow run ... -resume` with the original run's session ID
- Status feedback in the UI (launched / failed to launch)
- Safety: confirmation prompt, disable button while resume is in flight

**Acceptance test**: Run a pipeline that fails. Click Resume in the dashboard. Pipeline re-runs with `-resume`, only re-executing failed tasks.

**Constraint**: Requires server to be on the same machine where Nextflow runs (HPC login/submit node).

---

## Future (Not Planned)

- **Notifications**: Slack/email on pipeline completion or failure.
