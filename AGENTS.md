# Agent Notes

Findings and gotchas discovered during development, for future reference.

## Nextflow weblog resource metrics require `-with-trace`

**Problem:** `-with-weblog` alone does NOT send CPU/memory metrics (`%cpu`, `rss`, `peak_rss`, `vmem`). These fields are completely absent from the webhook JSON, and the dashboard shows "—" for all resource values.

**Root cause:** Resource metrics are collected by a bash wrapper (`nxf_trace`) that Nextflow injects into `.command.run`. This wrapper polls `/proc/<pid>/stat` and `/proc/<pid>/status` and writes results to `.command.trace`. The `TaskHandler.getTraceRecord()` method parses `.command.trace` before passing the TraceRecord to the weblog observer.

The `nxf_trace` wrapper is only injected when `Session.statsEnabled == true`. This flag is set when **any** registered TraceObserver returns `enableMetrics() == true`. The WebLogObserver (nf-weblog plugin) does **not** override `enableMetrics()`, so it defaults to `false`. Therefore `-with-weblog` alone never triggers resource collection.

**Fix:** Always pair `-with-weblog` with `-with-trace` (or set `trace { enabled = true }` in `nextflow.config`). The trace file observer returns `enableMetrics() == true`, which enables the `/proc` polling that populates resource fields in the TraceRecord.

```bash
# Correct — resource metrics will appear in weblog
nextflow run pipeline.nf -with-trace -with-weblog http://host:port/webhook

# Broken — no resource metrics in weblog
nextflow run pipeline.nf -with-weblog http://host:port/webhook
```

**Note:** `trace.fields` in `nextflow.config` only controls which columns appear in the trace CSV file. It does NOT control what the weblog sends (weblog always sends everything in `TraceRecord.getStore()`), and it does NOT enable resource collection by itself.

Source references (Nextflow source, as of 2026-03):
- `nf-weblog` plugin: `nextflow-io/nf-weblog` — `WebLogObserver.groovy` sends `payload.getStore()`
- Resource collection gate: `BashWrapperBuilder.isTraceRequired()` → checks `statsEnabled`
- `statsEnabled` set in: `Session.groovy:462` — `observersV1.any { ob -> ob.enableMetrics() }`
- `TaskHandler.getTraceRecord()` — parses `.command.trace` at line 244-246
