#!/usr/bin/env nextflow
nextflow.enable.dsl = 2

/*
 * Resource test pipeline — processes that do real CPU/memory work.
 * Used to verify Nextflow reports resource metrics on macOS.
 */

process CPU_WORK {
    tag "cpu"

    output:
    path "cpu_result.txt"

    script:
    """
    python3 -c "
total = 0
for i in range(20_000_000):
    total += i * i
print(total)
" > cpu_result.txt
    """
}

process MEM_WORK {
    tag "mem"

    output:
    path "mem_result.txt"

    script:
    """
    python3 -c "
import time
# Allocate ~100MB and hold it
data = bytearray(100 * 1024 * 1024)
for i in range(len(data)):
    if i % 4096 == 0:
        data[i] = 42  # touch pages to force allocation
time.sleep(3)  # hold memory so polling catches it
print(len(data))
" > mem_result.txt
    """
}

process BOTH {
    tag "cpu+mem"

    input:
    path cpu_in
    path mem_in

    output:
    path "combined.txt"

    script:
    """
    python3 -c "
import time
data = bytearray(50 * 1024 * 1024)
total = 0
for i in range(10_000_000):
    total += i
    if i % 1000000 == 0:
        data[i % len(data)] = i % 256
time.sleep(2)
print(total)
" > combined.txt
    """
}

workflow {
    cpu = CPU_WORK()
    mem = MEM_WORK()
    BOTH(cpu, mem)
}
