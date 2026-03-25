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
#   ./tests/integration.sh fail         # just the fail pipeline (tests error handling)
#   ./tests/integration.sh resource     # resource test (verifies CPU/mem reporting)
#   ./tests/integration.sh pact         # just nf-pact pipeline
#   ./tests/integration.sh -p 8080      # custom port (default: 8998)
#   ./tests/integration.sh mock -p 8080 # combine mode + port

set -euo pipefail

PORT=8998
MODE=all

while [[ $# -gt 0 ]]; do
    case "$1" in
        -p) PORT="$2"; shift 2 ;;
        hello|mock|fail|resource|pact|all) MODE="$1"; shift ;;
        *) echo "Usage: $0 [hello|mock|fail|pact|all] [-p PORT]" >&2; exit 1 ;;
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
      nextflow run nextflow-io/hello -with-trace -with-weblog "$URL" 2>&1 | sed 's/^/  [hello] /') &
}

run_mock() {
    local label="${1:-mock}"
    echo "[$label] Starting nf-gos-mock ..."
    (cd "$WORKDIR" && mkdir -p "$label" && cd "$label" && \
      nextflow run "$MOCK" --input "$MOCK/assets/samplesheet.csv" \
        -with-trace -with-weblog "$URL" 2>&1 | sed "s/^/  [$label] /") &
}

FAIL="$ROOT/tests/fail-pipeline"
RESOURCE="$ROOT/tests/resource-pipeline"

run_fail() {
    echo "[fail] Starting fail-pipeline (expects COMPUTE to fail) ..."
    (cd "$WORKDIR" && mkdir -p fail && cd fail && \
      nextflow run "$FAIL" \
        -with-trace -with-weblog "$URL" 2>&1 | sed 's/^/  [fail] /') &
}

run_resource() {
    echo "[resource] Starting resource-test (CPU/mem work) ..."
    (cd "$WORKDIR" && mkdir -p resource && cd resource && \
      nextflow run "$RESOURCE" \
        -with-trace -with-weblog "$URL" 2>&1 | sed 's/^/  [resource] /') &
}

PACT_DIR="/gpfs/home/diders01/Projects/nf-pact"

run_pact() {
    echo "[pact] Starting nf-pact ..."
    (cd "$WORKDIR" && mkdir -p pact && cd pact && \
      nextflow run "$PACT_DIR/main.nf" \
        -c "$PACT_DIR/conf/test_integration.config" \
        --input "$PACT_DIR/tests/samplesheets/NGS-26-5073.csv" \
        --outdir results \
        -with-trace -with-weblog "$URL" 2>&1 | sed 's/^/  [pact] /') &
}

case "$MODE" in
    hello)
        run_hello
        ;;
    mock)
        run_mock mock
        ;;
    fail)
        run_fail
        ;;
    resource)
        run_resource
        ;;
    pact)
        run_pact
        ;;
    all)
        run_hello
        run_mock mock-a
        sleep 2
        run_mock mock-b
        run_fail
        ;;
esac

echo ""
echo "==> Pipelines running. Watch the dashboard at http://localhost:${PORT}"
echo "==> Waiting for all to finish..."

wait
echo ""
echo "==> All pipelines complete."
