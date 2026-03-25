#!/usr/bin/env nextflow
nextflow.enable.dsl = 2

/*
 * Minimal pipeline that always fails — for testing error handling and views.
 *
 * 3 processes:
 *   SETUP   → succeeds (2s sleep)
 *   COMPUTE → fails with exit code 1
 *   REPORT  → would succeed, but never runs because COMPUTE fails
 */

process SETUP {
    tag "prep"

    output:
    path "input.txt"

    script:
    """
    sleep 2
    echo "prepared data" > input.txt
    """
}

process COMPUTE {
    tag "crunch"
    errorStrategy 'terminate'

    input:
    path data

    output:
    path "result.txt"

    script:
    """
    echo "[COMPUTE] Loading input data..."
    sleep 1
    echo "[COMPUTE] Processing batch 1/3..."
    echo "ERROR: segfault in libfoo.so at 0x7fff2a3b4c5d" >&2
    echo "ERROR: stack trace:" >&2
    echo "  #0 libfoo.so:compute_scores()+0x42" >&2
    echo "  #1 main:process_batch()+0x1a" >&2
    exit 1
    """
}

process REPORT {
    tag "summary"

    input:
    path result

    output:
    path "report.html"

    script:
    """
    sleep 1
    echo "<html>done</html>" > report.html
    """
}

workflow {
    data    = SETUP()
    result  = COMPUTE(data)
    REPORT(result)
}
