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
    sleep 1
    echo "ERROR: segfault in libfoo.so" >&2
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
