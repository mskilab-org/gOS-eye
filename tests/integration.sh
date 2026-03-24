#!/usr/bin/env bash
# Manual integration test — launches multiple Nextflow pipelines concurrently
# against a running nextflow-monitor instance.
#
# Prerequisites:
#   - nextflow-monitor (or air) running on the target port
#   - nextflow on PATH
#
# Usage:
#   ./tests/integration.sh              # default: localhost:8998
#   ./tests/integration.sh 8080         # custom port

set -euo pipefail

PORT="${1:-8998}"
URL="http://localhost:${PORT}/webhook"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MOCK="$ROOT/tests/mock-pipeline"
WORKDIR=$(mktemp -d)

trap 'echo "Work dirs in: $WORKDIR"; wait' EXIT

echo "==> Targeting $URL"
echo "==> Work directory: $WORKDIR"
echo ""

# 1. hello pipeline
echo "[1/3] Starting nextflow-io/hello ..."
(cd "$WORKDIR" && mkdir hello && cd hello && \
  nextflow run nextflow-io/hello -with-weblog "$URL" 2>&1 | sed 's/^/  [hello] /') &

# 2. mock pipeline — patient set 1 (all 3 patients)
echo "[2/3] Starting mock-nf-gos (run A) ..."
(cd "$WORKDIR" && mkdir mock-a && cd mock-a && \
  nextflow run "$MOCK" --input "$MOCK/assets/samplesheet.csv" \
    -with-weblog "$URL" 2>&1 | sed 's/^/  [mock-a] /') &

# small stagger so run names differ
sleep 2

# 3. mock pipeline — patient set 2 (same sheet, second run)
echo "[3/3] Starting mock-nf-gos (run B) ..."
(cd "$WORKDIR" && mkdir mock-b && cd mock-b && \
  nextflow run "$MOCK" --input "$MOCK/assets/samplesheet.csv" \
    -with-weblog "$URL" 2>&1 | sed 's/^/  [mock-b] /') &

echo ""
echo "==> 3 pipelines running. Watch the dashboard at http://localhost:${PORT}"
echo "==> Waiting for all to finish..."

wait
echo ""
echo "==> All pipelines complete."
