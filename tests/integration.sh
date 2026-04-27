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
#   ./tests/integration.sh resume       # interrupt mock pipeline, then rerun with -resume
#   ./tests/integration.sh pact         # just nf-pact pipeline
#   ./tests/integration.sh many_samples # mock pipeline with many samples (scale test)
#   ./tests/integration.sh -p 8080      # custom port (default: 8998)
#   ./tests/integration.sh mock -p 8080 # combine mode + port
#   ./tests/integration.sh resume -p 8998 # resume mode on selected port
#
# Environment variables:
#   PAIRS=500           # number of tumor/normal pairs for many_samples (default: 100)
#   INTERRUPT_AFTER=8   # seconds before process-group SIGINT in resume mode (default: 8)
#   INTERRUPT_GRACE=15  # seconds to wait before escalating to SIGTERM/SIGKILL (default: 15)

set -euo pipefail

PORT=8998
MODE=all

while [[ $# -gt 0 ]]; do
    case "$1" in
        -p) PORT="$2"; shift 2 ;;
        hello|mock|fail|resource|resume|pact|many_samples|all) MODE="$1"; shift ;;
        *) echo "Usage: $0 [hello|mock|fail|resource|resume|pact|many_samples|all] [-p PORT]" >&2; exit 1 ;;
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

run_resume() {
    local interrupt_after="${INTERRUPT_AFTER:-8}"
    local interrupt_grace="${INTERRUPT_GRACE:-15}"
    local launch="$WORKDIR/resume-launch"
    local work="$WORKDIR/resume-work"
    local trace_first="$launch/trace-first.txt"
    local trace_resume="$launch/trace-resume.txt"
    local log_first="$launch/nextflow-first.log"
    local log_resume="$launch/nextflow-resume.log"

    mkdir -p "$launch" "$work"

    echo "[resume] Starting mock pipeline in resume manual-test mode ..."
    echo "[resume] Launch directory: $launch"
    echo "[resume] Work directory:   $work"
    echo "[resume] Interrupt after:  ${interrupt_after}s"
    echo "[resume] Interrupt grace:  ${interrupt_grace}s"
    echo "[resume] First trace:      $trace_first"
    echo "[resume] Resume trace:     $trace_resume"
    echo ""

    echo "[resume] First run: nextflow run $MOCK ..."
    echo "[resume] First run helper sends SIGINT to the Nextflow process group after ${interrupt_after}s."
    (
        cd "$launch"
        exec python3 - "$interrupt_after" "$interrupt_grace" -- \
            nextflow run "$MOCK" --input "$MOCK/assets/samplesheet.csv" \
            -work-dir "$work" \
            -with-trace "$trace_first" \
            -with-weblog "$URL" <<'PY'
import math
import os
import signal
import subprocess
import sys

HELPER_ERROR_EXIT = 125
TERM_GRACE_SECONDS = 5.0


def log(message):
    print(f"[resume] {message}", flush=True)


def fail(message):
    print(f"[resume] ERROR: {message}", file=sys.stderr, flush=True)
    sys.exit(HELPER_ERROR_EXIT)


def parse_seconds(value, name):
    try:
        seconds = float(value)
    except ValueError:
        fail(f"{name} must be a number of seconds, got {value!r}")
    if not math.isfinite(seconds) or seconds < 0:
        fail(f"{name} must be a non-negative number of seconds, got {value!r}")
    return seconds


def format_seconds(seconds):
    if seconds == int(seconds):
        return str(int(seconds))
    return str(seconds)


def shell_status(returncode):
    if returncode < 0:
        return 128 + (-returncode)
    return returncode


def status_text(returncode):
    if returncode < 0:
        signal_number = -returncode
        try:
            signal_name = signal.Signals(signal_number).name
        except ValueError:
            signal_name = f"signal {signal_number}"
        return f"{signal_name} (shell status {shell_status(returncode)})"
    return f"exit status {returncode}"


def exit_with_child_status(returncode, message):
    log(f"{message} with {status_text(returncode)}.")
    sys.exit(shell_status(returncode))


def send_group_signal(pgid, sig, signal_name):
    try:
        os.killpg(pgid, sig)
    except ProcessLookupError:
        log(f"Process group {pgid} already exited before {signal_name} could be delivered.")
        return False
    except PermissionError as exc:
        fail(f"could not send {signal_name} to process group {pgid}: {exc}")
    log(f"Sent {signal_name} to Nextflow process group {pgid}.")
    return True


if len(sys.argv) < 5 or sys.argv[3] != "--":
    fail("internal usage error: expected INTERRUPT_AFTER INTERRUPT_GRACE -- command ...")

interrupt_after = parse_seconds(sys.argv[1], "INTERRUPT_AFTER")
sigint_grace = parse_seconds(sys.argv[2], "INTERRUPT_GRACE")
command = sys.argv[4:]
if not command:
    fail("internal usage error: missing Nextflow command")

try:
    proc = subprocess.Popen(command, stdin=subprocess.DEVNULL, start_new_session=True)
except FileNotFoundError as exc:
    fail(f"could not start {command[0]!r}: {exc}")
except OSError as exc:
    fail(f"could not start first Nextflow run: {exc}")

try:
    pgid = os.getpgid(proc.pid)
except ProcessLookupError:
    pgid = proc.pid

log(f"Started first Nextflow run PID {proc.pid} in process group {pgid}.")
log(f"Waiting {format_seconds(interrupt_after)}s before process-group SIGINT.")

try:
    returncode = proc.wait(timeout=interrupt_after)
except subprocess.TimeoutExpired:
    returncode = None

if returncode is not None:
    exit_with_child_status(
        returncode,
        f"First run finished before {format_seconds(interrupt_after)}s elapsed; no SIGINT sent",
    )

if not send_group_signal(pgid, signal.SIGINT, "SIGINT"):
    exit_with_child_status(proc.wait(), "First run finished")

try:
    returncode = proc.wait(timeout=sigint_grace)
except subprocess.TimeoutExpired:
    returncode = None

if returncode is not None:
    exit_with_child_status(returncode, "First run stopped after SIGINT")

log(
    f"SIGINT did not stop process group {pgid} within "
    f"{format_seconds(sigint_grace)}s; sending SIGTERM."
)
if not send_group_signal(pgid, signal.SIGTERM, "SIGTERM"):
    exit_with_child_status(proc.wait(), "First run finished")

try:
    returncode = proc.wait(timeout=TERM_GRACE_SECONDS)
except subprocess.TimeoutExpired:
    returncode = None

if returncode is not None:
    exit_with_child_status(returncode, "First run stopped after SIGTERM")

log(
    f"SIGTERM did not stop process group {pgid} within "
    f"{format_seconds(TERM_GRACE_SECONDS)}s; sending SIGKILL."
)
if not send_group_signal(pgid, signal.SIGKILL, "SIGKILL"):
    exit_with_child_status(proc.wait(), "First run finished")

exit_with_child_status(proc.wait(), "First run stopped after SIGKILL")
PY
    ) > >(tee "$log_first" | sed 's/^/  [resume:first] /') 2>&1 &
    local first_pid=$!

    echo "[resume] First run interrupt helper PID: $first_pid"

    local first_status=0
    if wait "$first_pid"; then
        first_status=0
    else
        first_status=$?
    fi
    echo "[resume] First run exit status: $first_status (ignored unless the interrupt helper failed)"
    if [[ "$first_status" -eq 125 ]]; then
        echo "==> First run interrupt helper failed; aborting before -resume." >&2
        return "$first_status"
    fi
    if [[ "$first_status" -eq 0 ]]; then
        echo "[resume] Hint: first run completed before SIGINT, so this may be a full-cache resume."
        echo "[resume] Hint: for a true interrupted-resume test, rerun with a smaller INTERRUPT_AFTER value (currently ${interrupt_after}s)."
    fi
    echo ""

    echo "[resume] Resume run: nextflow run $MOCK ... -resume"
    (
        cd "$launch"
        exec nextflow run "$MOCK" --input "$MOCK/assets/samplesheet.csv" \
            -work-dir "$work" \
            -with-trace "$trace_resume" \
            -with-weblog "$URL" \
            -resume
    ) > >(tee "$log_resume" | sed 's/^/  [resume:resume] /') 2>&1 &
    local resume_pid=$!

    local resume_status=0
    if wait "$resume_pid"; then
        resume_status=0
    else
        resume_status=$?
    fi

    echo ""
    echo "==> Resume manual-test paths"
    echo "    Work dir:     $work"
    echo "    Launch dir:   $launch"
    echo "    First trace:  $trace_first"
    echo "    Resume trace: $trace_resume"
    echo "    Logs:"
    echo "      First run:  $log_first"
    echo "      Resume run: $log_resume"

    if [[ -f "$trace_resume" ]]; then
        if command -v rg >/dev/null 2>&1; then
            if rg -q '\bCACHED\b' "$trace_resume"; then
                local cached_count
                cached_count=$(rg -c '\bCACHED\b' "$trace_resume")
                echo "==> Resume trace contains CACHED rows: yes ($cached_count matching rows)"
            else
                echo "==> Resume trace contains CACHED rows: no"
            fi
        else
            echo "==> rg not found; inspect cached tasks with: grep -n 'CACHED' '$trace_resume'"
        fi
    else
        echo "==> Resume trace not found: $trace_resume"
    fi

    if [[ "$resume_status" -ne 0 ]]; then
        echo "==> Resume run failed with exit status $resume_status" >&2
        return "$resume_status"
    fi

    echo "==> Resume run completed successfully."
}

run_many_samples() {
    local pairs="${PAIRS:-100}"
    local total_samples=$((pairs * 2))
    local total_tasks=$((pairs * 13))
    local label="many-samples"
    local sheet="$WORKDIR/samplesheet_${pairs}pairs.csv"

    echo "[many-samples] Generating samplesheet: $pairs pairs ($total_samples samples) ..."
    echo "[many-samples] Expected: $total_tasks tasks across 10 processes"

    echo "pair,sample,status,fastq_1,fastq_2,purity,ploidy" > "$sheet"
    for i in $(seq 1 "$pairs"); do
        pad=$(printf "%04d" "$i")
        pur="0.$(( (i * 13 % 90) + 10 ))"
        plo="$(( (i % 3) + 2 )).$(( i % 10 ))"
        echo "PATIENT_${pad},SAMPLE_T${pad},tumor,/data/fastq/T${pad}_R1.fastq.gz,/data/fastq/T${pad}_R2.fastq.gz,${pur},${plo}"
        echo "PATIENT_${pad},SAMPLE_N${pad},normal,/data/fastq/N${pad}_R1.fastq.gz,/data/fastq/N${pad}_R2.fastq.gz,,"
    done >> "$sheet"

    echo "[many-samples] Samplesheet: $sheet ($total_samples rows)"
    echo "[many-samples] Note: mock processes sleep 1-6s each. With local executor"
    echo "[many-samples]   this will take a while. Use PAIRS=N to adjust scale."
    echo "[many-samples] Starting nf-gos-mock ..."

    (cd "$WORKDIR" && mkdir -p "$label" && cd "$label" && \
      nextflow run "$MOCK" --input "$sheet" \
        -with-trace -with-weblog "$URL" 2>&1 | sed "s/^/  [$label] /") &
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
    resume)
        if run_resume; then
            exit 0
        else
            exit $?
        fi
        ;;
    pact)
        run_pact
        ;;
    many_samples)
        run_many_samples
        ;;
    all)
        run_hello
        run_mock mock-a
        sleep 2
        run_mock mock-b
        run_fail
        ;;
esac

TLS_PORT=$((PORT + 1))
echo ""
echo "==> Pipelines running. Watch the dashboard at https://localhost:${TLS_PORT}"
echo "==> Webhooks posting to http://localhost:${PORT}/webhook"
echo "==> Waiting for all to finish..."

wait
echo ""
echo "==> All pipelines complete."
