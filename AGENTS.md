# General Instructions
- do not commit the .gitignore, it is ignored on purpose to keep it local
- AI-generated docs (research, plans, notes) go in `thoughts/private/` — never in tracked directories
- commit messages: subject line only, no body. Keep them opaque — don't enumerate changes.
- don't run servers yourself, just give me the command I'll run it on a separate tmux window
- don't use browser tool for manual verificaiton, just tell me and i will do it myself

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
# Go binary (shell `go` is shadowed by a custom function)
GO=/opt/homebrew/bin/go

# Build
$GO build -o nextflow-monitor ./cmd

# Run (starts HTTP server on :8080)
./nextflow-monitor

# In another terminal, run any Nextflow pipeline
nextflow run nextflow-io/hello -with-weblog http://localhost:8080/webhook

# Open browser to http://localhost:8080
```

## Dev Server (Hot Reload)

```bash
# Air watches .go, .html, .css files and auto-rebuilds/restarts
~/go/bin/air
# Config in .air.toml — runs on port 8998 by default
# Test with: nextflow run nextflow-io/hello -with-weblog http://localhost:8998/webhook
```

## Test Commands

```bash
GO=/opt/homebrew/bin/go
$GO test ./...                    # unit tests
$GO test -run TestWebhook ./...   # specific test
$GO vet ./...                     # static analysis
```

## HtDP Settings

```
htdp.transparent: false
```

## Design Principles
- **Pipeline-agnostic**: No knowledge of specific pipelines. Consumes only the standard Nextflow weblog event schema.
- **Zero config**: Run the binary, point Nextflow at it. No database, no config files, no auth (v1).
- **Single process**: One binary serves the webhook endpoint, SSE stream, and static frontend.
- **HPC-friendly**: Single static binary, runs on a login/submit node, no containers or external services needed.
