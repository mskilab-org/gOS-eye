# nextflow-monitor

A lightweight, self-hosted, live-updating dashboard for any Nextflow pipeline. Consumes Nextflow's standard `-with-weblog` events via HTTP and renders a real-time browser dashboard — no database, no config files, no build step.

## How it works

```
Nextflow  ──POST /webhook──▶  nextflow-monitor  ──SSE──▶  Browser
           (JSON events)       (Go, in-memory)          (Datastar, live DOM)
```

1. Run any Nextflow pipeline with `-with-weblog http://host:port/webhook`
2. The monitor receives webhook events and stores pipeline state in memory
3. Connected browsers receive live HTML updates via Server-Sent Events
4. [Datastar](https://data-star.dev) patches the DOM — no client-side rendering

## Features

- **Pipeline-agnostic** — works with any Nextflow pipeline, no configuration needed
- **Real-time dashboard** — progress bars, task status, resource metrics, live elapsed timer
- **Multi-run support** — run selector panel when monitoring multiple pipelines
- **Expandable task details** — CPU%, memory, peak RSS, exit codes, work directories, timestamps
- **Process grouping** — tasks grouped by process with per-group status indicators
- **Run summary** — duration, task counts, peak memory, error messages on completion
- **Single binary** — no runtime dependencies, ideal for HPC login/submit nodes
- **Zero config** — run the binary, point Nextflow at it, open a browser

## Install

### Download pre-built binary (Linux amd64)

```bash
curl -L -o nextflow-monitor https://github.com/mskilab-org/nextflow-monitor/releases/download/v0.1.0/nextflow-monitor
chmod +x nextflow-monitor
```

### Build from source

```bash
go build -o nextflow-monitor ./cmd
```

## Quick Start

```bash
# Run (starts HTTP server on :8080)
./nextflow-monitor

# In another terminal, run any Nextflow pipeline
nextflow run nextflow-io/hello -with-weblog http://localhost:8080/webhook

# Open http://localhost:8080
```

### Options

```
-host string   host to bind to (default "localhost")
-port int      port to listen on (default 8080)
```

To accept connections from other machines (e.g., Nextflow running on a compute node):

```bash
./nextflow-monitor -host 0.0.0.0 -port 8080
```

## Development

### Prerequisites

- Go 1.22+
- [Air](https://github.com/air-verse/air) (optional, for hot reload)

### Dev Server

```bash
# Air watches .go, .html, .css files and auto-rebuilds/restarts
air
# Runs on port 8998 by default (see .air.toml)
# Test with: nextflow run nextflow-io/hello -with-weblog http://localhost:8998/webhook
```

### Tests

```bash
go test ./...                        # run all tests
go test ./internal/server/...        # server package only
go test ./internal/state/...         # state package only
go test ./cmd/...                    # main wiring tests
go test -run TestFormatDuration ./...  # specific test by name
go test -v ./...                     # verbose output
```

Or use the test runner script:

```bash
./tests/run.sh          # all tests
./tests/run.sh server   # server package
./tests/run.sh state    # state package
./tests/run.sh cmd      # cmd package
```

See [`tests/README.md`](tests/README.md) for a full test map.

## Project Structure

```
cmd/                      Go entrypoint (main.go)
internal/
  server/                 HTTP handlers, SSE broker, HTML rendering
  state/                  In-memory state store, webhook event processing
web/                      Static frontend (index.html, style.css)
testdata/                 Captured webhook JSON fixtures
tests/                    Test runner script and documentation
```

## Architecture

The server is a single Go process with three responsibilities:

| Endpoint | Method | Purpose |
|---|---|---|
| `/webhook` | POST | Receives Nextflow weblog JSON events |
| `/sse` | GET | Streams live HTML fragments to browsers via SSE |
| `/` | GET | Serves the static dashboard page |

State is held in memory — no database. Each webhook event updates the in-memory store, triggers a re-render of the dashboard HTML, and publishes the fragment to all connected SSE subscribers. The browser uses [Datastar v1](https://data-star.dev) to morph received fragments into the DOM by matching element IDs.

## License

MIT
