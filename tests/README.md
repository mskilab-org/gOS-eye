# Tests

Go convention requires `_test.go` files to live alongside the package they test. The test files are in their source directories; this directory provides a runner script and this map.

## Running Tests

```bash
./tests/run.sh              # all tests
./tests/run.sh server       # internal/server package
./tests/run.sh state        # internal/state package
./tests/run.sh cmd          # cmd package
./tests/run.sh -v           # all tests, verbose
./tests/run.sh server -v    # server tests, verbose
```

## Test Map

### `cmd/` — Entrypoint Wiring

| File | What it tests |
|---|---|
| `main_test.go` | Store + Server wire together, server accepts HTTP connections |

### `internal/state/` — State Store

| File | What it tests |
|---|---|
| `state_test.go` | `NewStore` constructor (non-nil, empty map) |
| `handleevent_test.go` | `HandleEvent` dispatch: started, process_*, completed, error events |
| `getrun_test.go` | `GetRun` lookup (existing, missing) |
| `getlatestrun_test.go` | `GetLatestRun` / `GetLatestRunID` tracking |
| `getallruns_test.go` | `GetAllRuns` returns all runs |
| `unmarshal_test.go` | `Trace.UnmarshalJSON` handles `%cpu` field |

### `internal/server/` — HTTP Handlers & Rendering

| File | What it tests |
|---|---|
| `newserver_test.go` | `NewServer` constructor, route registration |
| `servehttp_test.go` | `ServeHTTP` delegates to mux correctly |
| `handlewebhook_test.go` | POST /webhook: JSON parsing, state updates, SSE publish |
| `handlesse_test.go` | GET /sse: SSE headers, initial render, streaming |
| `handleindex_test.go` | GET /: serves index.html |
| `broker_test.go` | `Broker` subscribe/unsubscribe/publish fan-out |
| `renderdashboard_test.go` | Full dashboard render: empty state, single run, multi-run |
| `renderrundetail_test.go` | Per-run detail panel: header, progress bar, process groups |
| `renderrunselector_test.go` | Run selector panel: run list, sorting, click handlers |
| `renderrunsummary_test.go` | Run summary: duration, task counts, peak memory, errors |
| `rendertaskrows_test.go` | Task row rendering: status badges, expandable details |
| `groupprocesses_test.go` | `groupProcesses`: grouping, counting, sorting |
| `formatduration_test.go` | `formatDuration`: ms → human-readable |
| `formatbytes_test.go` | `formatBytes`: bytes → human-readable (KB/MB/GB) |
| `formattimestamp_test.go` | `formatTimestamp`: epoch ms → UTC string |
| `formatssefragment_test.go` | `formatSSEFragment`: HTML → Datastar SSE wire format |
| `formatssesignals_test.go` | `formatSSESignals`: JSON → Datastar signal patch |
| `computerunduration_test.go` | `computeRunDuration`: ISO timestamps → duration string |
