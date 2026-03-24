#!/usr/bin/env bash
# Manual integration test — launches Nextflow pipelines against a running
# nextflow-monitor instance.
#
# Prerequisites:
#   - nextflow-monitor (or air) running on the target port
#   - nextflow on PATH
#
# Usage:
#   ./tests/integration.sh              # all pipelines (hello + 2× mock)
#   ./tests/integration.sh hello        # just the hello pipeline
#   ./tests/integration.sh mock         # just one mock-nf-gos run
#   ./tests/integration.sh -p 8080      # custom port (default: 8998)
#   ./tests/integration.sh mock -p 8080 # combine mode + port

set -euo pipefail

PORT=8998
MODE=all

while [[ $# -gt 0 ]]; do
    case "$1" in
        -p) PORT="$2"; shift 2 ;;
        hello|mock|all) MODE="$1"; shift ;;
        *) echo "Usage: $0 [hello|mock|all] [-p PORT]" >&2; exit 1 ;;
    esac
done

URL="http://localhost:${PORT}/webhook"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MOCK="$ROOT/tests/mock-pipeline"
WORKDIR=$(mktemp -d)

trap 'echo "Work dirs in: $WORKDIR"; wait' EXIT

echo "==> Mode: $MODE"
echo "==> Targeting $URL"
echo "==> Work directory: $WORKDIR"
echo ""

run_hello() {
    echo "[hello] Starting nextflow-io/hello ..."
    (cd "$WORKDIR" && mkdir -p hello && cd hello && \
      nextflow run nextflow-io/hello -with-weblog "$URL" 2>&1 | sed 's/^/  [hello] /') &
}

run_mock() {
    local label="${1:-mock}"
    echo "[$label] Starting nf-gos-mock ..."
    (cd "$WORKDIR" && mkdir -p "$label" && cd "$label" && \
      nextflow run "$MOCK" --input "$MOCK/assets/samplesheet.csv" \
        -with-weblog "$URL" 2>&1 | sed "s/^/  [$label] /") &
}

case "$MODE" in
    hello)
        run_hello
        ;;
    mock)
        run_mock mock
        ;;
    all)
        run_hello
        run_mock mock-a
        sleep 2
        run_mock mock-b
        ;;
esac

echo ""
echo "==> Pipelines running. Watch the dashboard at http://localhost:${PORT}"
echo "==> Waiting for all to finish..."

wait
echo ""
echo "==> All pipelines complete."
