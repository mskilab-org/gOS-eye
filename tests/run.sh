#!/usr/bin/env bash
# Test runner for nextflow-monitor.
# Usage:
#   ./tests/run.sh              # all tests
#   ./tests/run.sh server       # internal/server package
#   ./tests/run.sh state        # internal/state package
#   ./tests/run.sh cmd          # cmd package
#   ./tests/run.sh -v           # all tests, verbose
#   ./tests/run.sh server -v    # server package, verbose

set -euo pipefail

GO="${GO:-go}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Parse args: optional package selector + optional flags
PKG=""
FLAGS=()

for arg in "$@"; do
    case "$arg" in
        server) PKG="./internal/server/..." ;;
        state)  PKG="./internal/state/..." ;;
        cmd)    PKG="./cmd/..." ;;
        *)      FLAGS+=("$arg") ;;
    esac
done

if [ -z "$PKG" ]; then
    PKG="./..."
fi

cd "$ROOT"
exec "$GO" test "${FLAGS[@]+"${FLAGS[@]}"}" "$PKG"
