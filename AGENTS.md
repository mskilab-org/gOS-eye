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

## Datastar v1 Reference (RC.8)

CDN: `https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.0-RC.8/bundles/datastar.js`
Full docs: https://data-star.dev/docs.md (single-page)

### Architecture Pattern (this project)

Backend-driven hypermedia: Go server renders HTML fragments, sends them as SSE events, Datastar morphs them into the DOM. No client-side rendering.

1. Page loads → `data-init="@get('/sse')"` → Datastar sends GET to `/sse`
2. Server responds with `Content-Type: text/event-stream`, streams `datastar-patch-elements` events
3. Datastar morphs received HTML into DOM by matching element IDs
4. On webhook → server re-renders dashboard fragment → publishes to all SSE subscribers

### SSE Wire Format (we craft this manually, no Go SDK)

**Patch Elements** — morph HTML into DOM by ID:
```
event: datastar-patch-elements
data: elements <div id="dashboard">
data: elements   <h1>Hello</h1>
data: elements </div>

```
Each line of HTML gets its own `data: elements ` prefix. Event ends with blank line (`\n\n`).
Top-level elements **must have an `id`** for morph to work.

**Patch Signals** — update signal values:
```
event: datastar-patch-signals
data: signals {foo: 1, bar: 'hello'}

```
Values use JS object notation (not strict JSON). Set to `null` to remove a signal.

**Optional data lines** for patch-elements:
- `data: selector #foo` — target element via CSS selector
- `data: mode inner` — morph inner HTML (default: `outer`)
- `data: mode append|prepend|before|after|replace|remove`

### Attribute Syntax Rules

**Key separator = colon (`:`)**, not hyphen:
```html
✅ data-on:click="..."        ✗ data-on-click="..."
✅ data-class:expanded="..."   ✗ data-class-expanded="..."
✅ data-signals:foo="1"        ✗ data-signals-foo="1"
```

**Modifier separator = double underscore (`__`)**, modifier values use dot (`.`):
```html
✅ data-on:click__stop="..."                  ✗ data-on:click.stop="..."
✅ data-on-interval__duration.1s="$tick++"     (data-on-interval is the full plugin name)
✅ data-init__delay.500ms="..."
```

**Signal casing**: `data-signals:foo-bar` → creates `$fooBar` (auto camelCase).
**Class casing**: `data-class:font-bold` → toggles class `font-bold` (auto kebab-case).

### Attributes Used in This Project

| Attribute | Purpose | Example |
|---|---|---|
| `data-signals` | Define reactive signals | `data-signals="{tick: 0, expandedGroup: ''}"` |
| `data-signals:key` | Define single signal (key→camelCase) | `data-signals:start-time="'2024-01-15T...'"` |
| `data-init` | Run expression on load / when patched into DOM | `data-init="@get('/sse')"` |
| `data-on:click` | Click handler | `data-on:click="$expandedGroup = $expandedGroup === 'foo' ? '' : 'foo'"` |
| `data-on:click__stop` | Click with stopPropagation | `data-on:click__stop="$expandedTask = $expandedTask === 1 ? 0 : 1"` |
| `data-on-interval` | Repeated timer (default 1s) | `data-on-interval__duration.1s="$tick++"` |
| `data-show` | Show/hide element | `data-show="$expandedGroup === 'foo'"` |
| `data-text` | Bind text content to expression | `data-text="$startTime ? formatElapsed($startTime) : ''"` |
| `data-class:name` | Toggle CSS class | `data-class:expanded="$expandedGroup === 'foo'"` |

### Signals

- Reactive variables, globally accessible, denoted with `$` prefix in expressions
- Defined via `data-signals`, `data-bind`, `data-computed`, or patched from backend
- Signals starting with `_` are local (not sent to backend in requests)
- Type is preserved: if defined as number `0`, stays number even when updated from input
- Expressions: full JS syntax — ternary, logical operators, function calls, semicolon-separated statements

### Morphing Behavior

- Default mode (`outer`): replaces element matching by ID, preserves unchanged attributes
- Only reapplies `data-*` attributes that actually changed in the new HTML
- **Signals survive morph** — they're global, not tied to elements
- Place `data-ignore-morph` on elements that shouldn't be touched
- To prevent flicker on `data-show` elements, add `style="display: none"` initially

### Common Gotchas

1. **No ID = no morph**: Top-level elements in SSE fragments must have `id` attributes
2. **Colon not hyphen**: `data-on:click` not `data-on-click` — wrong separator silently fails
3. **Double underscore for modifiers**: `__stop` not `.stop` — dot is for modifier values only
4. **String signals need quotes**: `data-signals:name="'hello'"` (outer quotes = attr, inner = JS string)
5. **Semicolons required**: multi-statement expressions need `;` — line breaks alone don't separate
6. **Morph preserves state**: event listeners, CSS transitions, focus survive if element IDs match
7. **`@get()` retry**: auto-retries on network errors by default; set `openWhenHidden: true` for always-on dashboards

### Go SDK Alternative (not currently used)

```go
import "github.com/starfederation/datastar-go/datastar"
sse := datastar.NewSSE(w, r)                    // sets headers, creates generator
sse.PatchElements(`<div id="foo">...</div>`)     // sends patch-elements event
sse.PatchSignals([]byte(`{bar: 'hello'}`))       // sends patch-signals event
```
The project currently formats SSE events manually in `formatSSEFragment()` and `formatSSESignals()`.

## Design Principles
- **Pipeline-agnostic**: No knowledge of specific pipelines. Consumes only the standard Nextflow weblog event schema.
- **Zero config**: Run the binary, point Nextflow at it. No database, no config files, no auth (v1).
- **Single process**: One binary serves the webhook endpoint, SSE stream, and static frontend.
- **HPC-friendly**: Single static binary, runs on a login/submit node, no containers or external services needed.
