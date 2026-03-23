# nextflow-monitor — Implementation Plan

## Iteration 0: Plumbing Proof (Stub)

**Goal**: Prove the end-to-end path works: Nextflow → webhook → Go → SSE → Datastar → browser update.

**Deliverables**:
- `cmd/main.go` — HTTP server with three routes: `POST /webhook`, `GET /sse`, `GET /`
- `internal/state/state.go` — Run/Task structs, event ingestion (update state from webhook JSON)
- `internal/server/server.go` — HTTP handlers, SSE fan-out to subscribers
- `web/index.html` — Single page with Datastar, shows a list of `process_name: STATUS` lines that update live
- `testdata/hello_run.json` — Captured webhook events from `nextflow run nextflow-io/hello`

**Acceptance test**: Run `nextflow-io/hello` with `-with-weblog`, see all 4 tasks go SUBMITTED → RUNNING → COMPLETED in the browser in real time.

**Scope boundary**: No styling, no grouping, no detail panel. Just a raw list that proves the plumbing.

---

## Iteration 1: Core Dashboard

**Goal**: Usable single-run monitoring dashboard.

**Deliverables**:
- **Progress bar**: Tasks completed / total, percentage, animated fill
- **Process grouping**: Tasks grouped by process name, each group shows `completed/total` count
- **Status indicators**: Color-coded (gray=pending, blue=running, green=completed, red=failed)
- **Run header**: Pipeline name, run name, overall status, elapsed time (live-updating)
- **CSS**: Clean, minimal stylesheet. Dark mode friendly (HPC terminals are often dark-background).

**Acceptance test**: Run a pipeline with 3+ process types and multiple samples. Dashboard shows grouped progress updating in real time. Visually clear which processes are done, in-flight, or pending.

**Scope boundary**: No task-level detail panel yet. No click interactions.

---

## Iteration 2: Task Detail

**Goal**: Drill into individual tasks for debugging.

**Deliverables**:
- **Click-to-expand**: Click a process group to see individual tasks
- **Task detail panel**: Status, duration, CPU%, memory (RSS/peak), exit code, work directory
- **Failed task highlighting**: Failed tasks sort to the top with error emphasis
- **Timestamps**: Submit, start, complete times in human-readable format

**Acceptance test**: Click a process group, see task list. Click a task, see resource details. Force-fail a task (e.g., `exit 1` in a test pipeline), see it highlighted red with exit code.

**Scope boundary**: No log viewing. No DAG visualization.

---

## Iteration 3: Multi-Run + Polish

**Goal**: Handle real-world usage patterns.

**Deliverables**:
- **Run selector**: If multiple runs send events, show a dropdown/list to switch between them
- **Auto-follow**: Newest run is selected by default; new runs auto-switch focus
- **Completed run summary**: When a run finishes, show summary (total time, pass/fail counts, resource peaks)
- **Error display**: For failed runs, show the error message from the `error` event
- **Responsive layout**: Works on laptop screen and a narrow SSH-forwarded window

**Acceptance test**: Start two pipelines sequentially with `-with-weblog`. Both appear in run selector. Completed run shows summary. Failed run shows error.

**Scope boundary**: No persistence across server restarts. No DAG. No log tailing.

---

## Future (Not Planned)

These are ideas that may be valuable but are explicitly out of scope for the initial iterations:

- **DAG visualization**: Render the pipeline DAG from process dependencies. Requires either parsing the Nextflow DAG output or inferring from event ordering.
- **Log tailing**: Stream task stdout/stderr from the work directory. Requires filesystem access to the work dir (only possible if server runs on the same machine).
- **SQLite persistence**: Survive server restarts. Store events to disk, replay on startup.
- **Retry/resume controls**: Trigger `-resume` from the dashboard. Requires shell-out or Nextflow API.
- **Resource charts**: Time-series plots of CPU/memory per task. Requires accumulating trace data over time.
- **Notifications**: Slack/email on pipeline completion or failure.
- **Multi-user auth**: Basic auth or token-based access for shared deployments.
