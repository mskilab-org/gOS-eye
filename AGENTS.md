# General Instructions
- do not commit the .gitignore, it is ignored on purpose to keep it local
- AI-generated docs (research, plans, notes) go in `thoughts/private/` — never in tracked directories
- commit messages: subject line only, no body. Keep them opaque — don't enumerate changes.

# Project: nextflow-monitor

A lightweight, self-hosted, live-updating dashboard for any Nextflow pipeline. Consumes Nextflow's standard `-with-weblog` events via HTTP and renders a real-time Datastar-powered frontend.

## Tech Stack
- **Backend**: Go — single binary, no runtime dependencies, ideal for HPC deployment
- **Frontend**: Datastar (SSE-driven hypermedia) + vanilla HTML/CSS — no build step, no node_modules
- **Protocol**: Nextflow POSTs webhook events → Go server stores state in memory → browser receives SSE updates → Datastar patches the DOM

## Key Directories

| Path | Description |
|---|---|
| `cmd/` | Go entrypoint (`main.go`) |
| `internal/` | Go packages: `server`, `state`, `sse` |
| `web/` | HTML templates, CSS, static assets |
| `testdata/` | Captured webhook JSON fixtures for testing |
| `thoughts/private/` | AI working notes (untracked) |

## Build & Run

```bash
# Build
go build -o nextflow-monitor ./cmd

# Run (starts HTTP server on :8080)
./nextflow-monitor

# In another terminal, run any Nextflow pipeline
nextflow run nextflow-io/hello -with-weblog http://localhost:8080/webhook

# Open browser to http://localhost:8080
```

## Test Commands

```bash
go test ./...                    # unit tests
go test -run TestWebhook ./...   # specific test
go vet ./...                     # static analysis
```

## Design Principles
- **Pipeline-agnostic**: No knowledge of specific pipelines. Consumes only the standard Nextflow weblog event schema.
- **Zero config**: Run the binary, point Nextflow at it. No database, no config files, no auth (v1).
- **Single process**: One binary serves the webhook endpoint, SSE stream, and static frontend.
- **HPC-friendly**: Single static binary, runs on a login/submit node, no containers or external services needed.
