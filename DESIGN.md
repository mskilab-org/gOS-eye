# nextflow-monitor Design Decisions

## 1. Architecture: Webhook → In-Memory State → SSE → Datastar

**Decision**: Single Go process with three responsibilities:

1. **Webhook receiver** (`POST /webhook`): Accepts Nextflow weblog JSON events, updates in-memory state
2. **SSE endpoint** (`GET /sse`): Pushes state diffs to connected browsers via Server-Sent Events
3. **Static server** (`GET /`): Serves the HTML/CSS/JS frontend (embedded in the binary)

```
Nextflow ──POST──▶ Go server ──SSE──▶ Browser (Datastar)
                      │
                   in-memory
                   run state
```

**Rationale**:
- No database — pipeline runs are ephemeral; monitoring state doesn't need persistence
- SSE (not WebSocket) because Datastar is built on SSE and the data flow is unidirectional (server→browser)
- Embedding static assets in the Go binary means deployment is a single file copy

**Tradeoff**: State is lost on server restart. Acceptable — you'd re-run the pipeline with `-with-weblog` anyway. A future iteration could add optional SQLite persistence.

## 2. Frontend: Datastar (SSE Hypermedia)

**Decision**: Use Datastar for the reactive frontend. No React, Vue, or build toolchain.

**How Datastar works**:
- Server sends SSE events containing HTML fragments or data updates
- Datastar attributes on HTML elements (`data-on-*`, `data-model`, `data-signals`) declaratively bind to server state
- DOM updates happen via SSE-driven merges — the server controls what the browser shows

**Rationale**:
- No build step (no npm, webpack, vite) — the frontend is plain HTML with `<script src="datastar.js">`
- SSE is the natural transport since Nextflow events arrive as a stream
- Hypermedia approach means the server renders HTML fragments, not JSON — simpler than maintaining a separate API + client-side rendering layer
- Tiny payload: Datastar is ~14KB

**Tradeoff**: Less ecosystem support than React/Vue. Acceptable — the UI is a single dashboard page, not a complex SPA.

## 3. Backend: Go

**Decision**: Go for the backend server.

**Rationale**:
- Single static binary — `scp` it to an HPC login node and run. No runtime, no pip, no node.
- Excellent stdlib for HTTP servers, SSE, JSON parsing, and HTML templating
- Goroutines handle concurrent webhook POSTs + SSE fan-out naturally
- `embed` package bundles static assets into the binary at compile time

**Alternatives considered**:
- Python (Flask/FastAPI): Needs Python runtime + deps on the cluster. Virtual env management is friction.
- Bun/Deno: Better than Node but still a runtime dependency. Bun's stability on Linux/HPC is uncertain.

## 4. Nextflow Weblog Event Schema

**Decision**: Target Nextflow 23.04+ weblog event format. Document the exact schema from captured events, not from (sparse) Nextflow docs.

**Workflow-level events**:
```json
{
  "runName": "happy_darwin",
  "runId": "a1b2c3d4-...",
  "event": "started",
  "utcTime": "2024-01-15T10:30:00Z",
  "metadata": {
    "workflow": {
      "projectName": "nextflow-io/hello",
      "scriptFile": "/path/to/main.nf",
      "start": "2024-01-15T10:30:00Z",
      "configFiles": ["/path/to/nextflow.config"]
    }
  }
}
```

Events: `started`, `completed`, `error`

**Task-level events**:
```json
{
  "runName": "happy_darwin",
  "runId": "a1b2c3d4-...",
  "event": "process_completed",
  "utcTime": "2024-01-15T10:30:05Z",
  "trace": {
    "task_id": 1,
    "status": "COMPLETED",
    "hash": "ab/cdef12",
    "name": "sayHello (1)",
    "process": "sayHello",
    "tag": null,
    "submit": 1705312200000,
    "start": 1705312201000,
    "complete": 1705312205000,
    "duration": 5000,
    "realtime": 4800,
    "%cpu": 95.2,
    "rss": 10485760,
    "vmem": 52428800,
    "peak_rss": 10485760,
    "cpus": 1,
    "memory": 1073741824,
    "exit": 0,
    "workdir": "/path/to/work/ab/cdef12345678",
    "script": "echo 'Hello world!'"
  }
}
```

Task events: `process_submitted`, `process_started`, `process_completed`, `process_failed`

**Important**: This schema should be validated against real captured events before coding starts. Run `nextflow run nextflow-io/hello -with-weblog http://localhost:PORT` and capture the actual POSTs. The schema above is representative but field names and types may vary by Nextflow version.

## 5. State Model

**Decision**: In-memory state organized as:

```
Server
└── runs: map[runId]Run
    └── Run
        ├── runName, runId, status, startTime, endTime
        ├── metadata (workflow info from "started" event)
        └── tasks: map[taskId]Task
            └── Task
                ├── taskId, hash, name, process, tag
                ├── status (SUBMITTED → RUNNING → COMPLETED/FAILED)
                ├── timestamps (submit, start, complete)
                ├── resources (cpus, memory, rss, vmem, duration)
                └── exit code, workdir
```

**Rationale**:
- Webhook events are upserts: each task event updates the corresponding Task struct
- Multiple concurrent pipeline runs are supported (keyed by `runId`)
- No persistence — if the server restarts, state is empty until new events arrive

**Concurrency**: `sync.RWMutex` on the runs map. Webhook handler takes write lock; SSE fan-out takes read lock. At the event volumes Nextflow produces (tens of events/sec at peak), this is fine.

## 6. SSE Fan-Out Strategy

**Decision**: On each webhook event, broadcast a state diff to all connected SSE clients.

**Mechanism**:
- Each SSE client connection registers a channel in a subscriber list
- Webhook handler updates state, then sends a rendered HTML fragment to all subscriber channels
- Datastar on the client merges the fragment into the DOM

**What gets sent**: Not raw JSON — server-rendered HTML fragments (Datastar's `merge` mode). The server decides what HTML to update. This keeps the browser-side logic trivial.

**Cleanup**: When an SSE connection drops, remove its channel from the subscriber list (detect via `r.Context().Done()`).

## 7. UI Layout (v1)

**Decision**: Single-page dashboard with three panels:

```
┌─────────────────────────────────────────────┐
│  nextflow-monitor          run: happy_darwin │
├─────────────────────────────────────────────┤
│  Pipeline Progress                          │
│  ████████████░░░░░░░░  12/20 tasks  (60%)  │
├────────────────────┬────────────────────────┤
│  Processes         │  Task Detail           │
│                    │                        │
│  ✅ FASTQC (4/4)  │  FASTQC (sample_1)    │
│  🔄 ALIGN  (2/4)  │  Status: COMPLETED     │
│  ⏳ MARK   (0/4)  │  Duration: 3m 42s      │
│  ⏳ CALL   (0/4)  │  CPU: 95%              │
│                    │  Memory: 2.1 GB        │
│                    │  Exit: 0               │
│                    │  Workdir: ab/cdef12... │
└────────────────────┴────────────────────────┘
```

- **Header**: Run name, pipeline name, overall status
- **Progress bar**: Tasks completed / total tasks (completed + in-flight + pending)
- **Process list** (left): Grouped by process name, showing completion count. Color-coded by status.
- **Task detail** (right): Click a process group to expand individual tasks with resource details.

**Rationale**:
- Process-grouped view matches how pipeline authors think ("is ALIGN done?"), not task-level IDs
- Progress bar gives at-a-glance status without reading anything
- Detail panel is optional — the left panel alone is sufficient for monitoring

## 8. Deployment: Single Binary

**Decision**: Ship as a single statically-compiled Go binary. No containers, no docker-compose, no systemd.

**Usage on HPC**:
```bash
# Copy binary to cluster
scp nextflow-monitor user@hpc:~/bin/

# Run on login/submit node (where Nextflow runs)
~/bin/nextflow-monitor --port 8080 &

# Launch pipeline
nextflow run pipeline.nf -with-weblog http://localhost:8080/webhook
```

**Rationale**:
- HPC login nodes typically have no Docker/Podman
- A single binary avoids all dependency issues
- Running on the submit node means `localhost` connectivity is guaranteed (Nextflow's weblog POSTs come from the Nextflow head process, not from SLURM workers)

**Port forwarding for browser access**:
```bash
ssh -L 8080:localhost:8080 user@hpc
# Then open http://localhost:8080 in local browser
```

## 9. No Auth (v1)

**Decision**: No authentication or authorization in v1. The server binds to `localhost` by default.

**Rationale**:
- Primary use case is a single user on an HPC cluster, accessing via SSH tunnel
- Adding auth (even basic auth) means managing credentials — friction for a monitoring tool
- Binding to `localhost` (not `0.0.0.0`) means only SSH-tunneled connections can reach it

**When to revisit**: If the tool is exposed on a shared network or serves multiple users.

## 10. Testing Strategy

**Decision**: Three tiers:

1. **Unit tests**: State model (event ingestion, status transitions), SSE encoding, HTML fragment rendering. Standard `go test`.
2. **Integration tests**: Spin up the server, POST captured webhook fixtures, assert SSE output. Uses `httptest` — no external process needed.
3. **Manual smoke test**: `nextflow run nextflow-io/hello -with-weblog http://localhost:8080/webhook` + open browser.

**Fixtures**: Capture real webhook events from `nextflow-io/hello` and store in `testdata/`. These are the contract — if Nextflow changes the schema, the fixtures (and tests) update.

**No browser/E2E tests in v1**: The frontend is thin enough (Datastar declarative bindings) that server-side fragment tests cover the logic. Visual correctness is verified manually.
