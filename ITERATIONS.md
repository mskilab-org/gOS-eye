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

## Iteration 7: Resume Command + Samplesheet Viewer

**Goal**: Give operators the info they need to resume failed runs and inspect/edit samplesheets — without requiring the server to have execution permissions.

### Part A: Capture Run Metadata

- Extend `WorkflowInfo` to parse `commandLine`, `sessionId`, `workDir`, `launchDir`, and `params` (as `map[string]any`) from the webhook `started` event
- Store these on the `Run` struct so they're available for rendering
- Update test fixtures to include these fields

### Part B: Resume Command (Copy-to-Clipboard)

- On failed runs, display a "Resume Command" section with the reconstructed command
- Reconstruct from stored parts: project name, params, `-work-dir`, `-resume <sessionId>`, etc. (For now this reproduces the original invocation; later iterations can let users modify params before copying.)
- Copy-to-clipboard button (JS `navigator.clipboard.writeText()`) — no server-side execution

### Part C: Samplesheet Viewer/Editor

- Read samplesheet path from `params.input` (the standard nf-core convention)
- If the path is found and the file is accessible from the server, read its content and display in an editable text area in the run detail panel
- Copy-to-clipboard button for the (possibly edited) samplesheet content
- Graceful degradation: if `input` param is missing or file is inaccessible, show a message instead of the editor

**Acceptance test**: Run a pipeline that fails. See the resume command displayed — copy it, paste in terminal, pipeline resumes correctly. See the samplesheet content — edit it, copy to clipboard, save to disk for the next run.

**Constraint**: Samplesheet viewing requires server filesystem access to the samplesheet path. Resume command generation works regardless.

---

## Future (Not Planned)

- **Notifications**: Slack/email on pipeline completion or failure.
