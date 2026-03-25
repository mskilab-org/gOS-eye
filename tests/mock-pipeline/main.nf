#!/usr/bin/env nextflow
nextflow.enable.dsl = 2

/*
 * Mock nf-gos pipeline for web portal demo — paired tumor/normal mode.
 * Each process just sleeps to simulate real compute.
 * DAG mirrors the real pipeline's dependency structure.
 *
 * Input: a samplesheet CSV with columns:
 *   pair,sample,status,fastq_1,fastq_2[,purity,ploidy]
 *
 * Each pair (patient) has a tumor and a normal sample.
 * BWA_MEM, FRAGCOUNTER, DRYCLEAN run independently on both tumor and normal.
 * GRIDSS, SAGE, AMBER take paired tumor+normal BAMs.
 * CBS, PURPLE, JABBA, EVENTS_FUSIONS operate at the pair level (tumor data).
 */

params.input  = null
params.outdir = 'results'

// ── Per-sample processes (run on both tumor and normal) ──────

process BWA_MEM {
    tag "${pair}:${sample_id}"
    publishDir "${params.outdir}/${pair}/bwa_mem/${sample_id}", mode: 'copy'

    input:
    tuple val(pair), val(sample_id), val(status), val(fastq_1), val(fastq_2)

    output:
    tuple val(pair), val(sample_id), val(status), path("aligned.bam")

    script:
    """
    echo "[BWA_MEM] Aligning sample ${sample_id} (pair: ${pair})"
    echo "[BWA_MEM] Loading reference genome index..."
    sleep 2
    echo "[BWA_MEM] Mapping reads from ${fastq_1}..."
    sleep 2
    echo "[BWA_MEM] Sorting and indexing BAM..."
    sleep 1
    echo "[BWA_MEM] Done. Wrote aligned.bam"
    echo "[BWA_MEM] WARN: low mapping quality for 3.2% of reads" >&2
    echo "mock_aligned_reads_${sample_id}" > aligned.bam
    """
}

process FRAGCOUNTER {
    tag "${pair}:${sample_id}"
    publishDir "${params.outdir}/${pair}/fragcounter/${sample_id}", mode: 'copy'

    input:
    tuple val(pair), val(sample_id), val(status), path(bam)

    output:
    tuple val(pair), val(sample_id), val(status), path("frag_coverage.rds")

    script:
    """
    echo "[FRAGCOUNTER] Computing fragment coverage for ${sample_id}"
    echo "[FRAGCOUNTER] Reading BAM: ${bam}"
    sleep 1
    echo "[FRAGCOUNTER] Counting fragments in 1kb windows..."
    sleep 1
    echo "[FRAGCOUNTER] GC-correcting coverage..."
    sleep 1
    echo "[FRAGCOUNTER] Done. 2,847,312 windows processed."
    echo "mock_fragcounter_${sample_id}" > frag_coverage.rds
    """
}

process DRYCLEAN {
    tag "${pair}:${sample_id}"
    publishDir "${params.outdir}/${pair}/dryclean/${sample_id}", mode: 'copy'

    input:
    tuple val(pair), val(sample_id), val(status), path(frag_cov)

    output:
    tuple val(pair), val(sample_id), val(status), path("dryclean_coverage.rds")

    script:
    """
    sleep 3
    echo "mock_dryclean_${sample_id}" > dryclean_coverage.rds
    """
}

// ── Paired processes (tumor BAM + normal BAM) ────────────────

process GRIDSS {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/gridss", mode: 'copy'

    input:
    tuple val(pair), path('tumor.bam'), path('normal.bam')

    output:
    tuple val(pair), path("sv_calls.vcf")

    script:
    """
    echo "[GRIDSS] Structural variant calling for ${pair}"
    echo "[GRIDSS] Tumor BAM: tumor.bam"
    echo "[GRIDSS] Normal BAM: normal.bam"
    sleep 2
    echo "[GRIDSS] Extracting split reads and discordant pairs..."
    sleep 2
    echo "[GRIDSS] Assembling breakpoints..."
    echo "[GRIDSS] WARN: contig assembly timeout at chr17:43,044,295" >&2
    sleep 2
    echo "[GRIDSS] Scoring variants..."
    echo "[GRIDSS] Done. 847 SV candidates called."
    echo "mock_sv_calls_${pair}" > sv_calls.vcf
    """
}

process SAGE {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/sage", mode: 'copy'

    input:
    tuple val(pair), path('tumor.bam'), path('normal.bam')

    output:
    tuple val(pair), path("snv_calls.vcf")

    script:
    """
    sleep 5
    echo "mock_snv_calls_${pair}" > snv_calls.vcf
    """
}

process AMBER {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/amber", mode: 'copy'

    input:
    tuple val(pair), path('tumor.bam'), path('normal.bam')

    output:
    tuple val(pair), path("baf.tsv")

    script:
    """
    sleep 4
    echo "mock_baf_${pair}" > baf.tsv
    """
}

// ── Tumor / pair-level processes ─────────────────────────────

process CBS {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/cbs", mode: 'copy'

    input:
    tuple val(pair), path(coverage)

    output:
    tuple val(pair), path("segments.rds")

    script:
    """
    sleep 3
    echo "mock_segments_${pair}" > segments.rds
    """
}

process PURPLE {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/purple", mode: 'copy'

    input:
    tuple val(pair), path(coverage), path(sv_vcf), path(snv_vcf), path(baf), val(purity_hint), val(ploidy_hint)

    output:
    tuple val(pair), path("purity_ploidy.txt")

    script:
    """
    if [ -n "${purity_hint}" ] && [ -n "${ploidy_hint}" ]; then
        printf "purity=${purity_hint}\\nploidy=${ploidy_hint}\\n" > purity_ploidy.txt
    else
        sleep 4
        printf "purity=0.65\\nploidy=2.1\\n" > purity_ploidy.txt
    fi
    """
}

process JABBA {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/jabba", mode: 'copy'

    input:
    tuple val(pair), path(segments), path(purity_ploidy), path(sv_vcf), path(coverage), path(snv_vcf)

    output:
    tuple val(pair), path("jabba_gg.rds")

    script:
    """
    echo "[JABBA] Building genome graph for ${pair}"
    echo "[JABBA] Inputs: segments, purity/ploidy, SVs, coverage, SNVs"
    sleep 1
    echo "[JABBA] Fitting junction-balanced genome graph..."
    sleep 2
    echo "[JABBA] Optimizing with 15 iterations..."
    echo "[JABBA] WARN: loose end at chr8:128,750,000 (telomere?)" >&2
    echo "[JABBA] WARN: high-CN segment at chr17:37,800,000-38,200,000" >&2
    sleep 2
    echo "[JABBA] Final graph: 34 segments, 12 junctions"
    echo "[JABBA] Done."
    echo "mock_genome_graph_${pair}" > jabba_gg.rds
    """
}

process EVENTS_FUSIONS {
    tag "${pair}"
    publishDir "${params.outdir}/${pair}/events_fusions", mode: 'copy'

    input:
    tuple val(pair), path(ggraph)

    output:
    tuple val(pair), path("events.rds"), path("fusions.rds")

    script:
    """
    sleep 3
    echo "mock_events_${pair}"  > events.rds
    echo "mock_fusions_${pair}" > fusions.rds
    """
}

/*
 * DAG (per tumor/normal pair):
 *
 *   BWA_MEM(T) ─┬─ FRAGCOUNTER(T) ── DRYCLEAN(T) ──┬── CBS ──────────────────┐
 *               │                                    ├── PURPLE (+ AMBER) ─────┤
 *               ├── GRIDSS ──────(T+N BAMs)──────────┤                         ├── JABBA ── EVENTS_FUSIONS
 *               ├── SAGE ────────(T+N BAMs)──────────┤                         │
 *               └── AMBER ───────(T+N BAMs)──────────┘                         │
 *                                                                              │
 *   BWA_MEM(N) ─┬─ FRAGCOUNTER(N) ── DRYCLEAN(N)                              │
 *               ├── GRIDSS ────────────────────────────────────────────────────┘
 *               ├── SAGE
 *               └── AMBER
 */
workflow {
    // Parse samplesheet CSV
    Channel
        .fromPath(params.input, checkIfExists: true)
        .splitCsv(header: true)
        .map { row ->
            tuple(row.pair, row.sample, row.status,
                  row.fastq_1, row.fastq_2,
                  row.purity ?: '', row.ploidy ?: '')
        }
        .branch {
            tumor:  it[2] == 'tumor'
            normal: it[2] == 'normal'
        }
        .set { rows }

    // Split tumor rows into fastq inputs and purity/ploidy hints
    rows.tumor
        .multiMap { pair, sample, status, fq1, fq2, pur, plo ->
            fq:    tuple(pair, sample, status, fq1, fq2)
            hints: tuple(pair, pur, plo)
        }
        .set { tumor_ch }

    normal_fq = rows.normal.map { pair, sample, status, fq1, fq2, pur, plo ->
        tuple(pair, sample, status, fq1, fq2)
    }

    // ── Per-sample: BWA_MEM → FRAGCOUNTER → DRYCLEAN (both T and N)
    all_fq  = tumor_ch.fq.mix(normal_fq)
    all_bam = BWA_MEM(all_fq)
    all_frag = FRAGCOUNTER(all_bam)
    all_cov  = DRYCLEAN(all_frag)

    // ── Branch BAMs by tumor/normal for paired processes
    all_bam
        .branch {
            tumor:  it[2] == 'tumor'
            normal: it[2] == 'normal'
        }
        .set { bam_split }

    tumor_bam_keyed  = bam_split.tumor.map  { pair, sample, status, bam -> tuple(pair, bam) }
    normal_bam_keyed = bam_split.normal.map { pair, sample, status, bam -> tuple(pair, bam) }
    paired_bams = tumor_bam_keyed.join(normal_bam_keyed)  // → (pair, tumor_bam, normal_bam)

    // ── Paired somatic callers
    sv  = GRIDSS(paired_bams)
    snv = SAGE(paired_bams)
    baf = AMBER(paired_bams)

    // ── Branch coverage for tumor-only downstream
    all_cov
        .branch {
            tumor:  it[2] == 'tumor'
            normal: it[2] == 'normal'
        }
        .set { cov_split }

    tumor_cov_keyed = cov_split.tumor.map { pair, sample, status, cov -> tuple(pair, cov) }

    // ── CBS: tumor coverage only
    seg = CBS(tumor_cov_keyed)

    // ── PURPLE: tumor coverage + SV + SNV + BAF + hints
    tumor_cov_keyed.join(sv).join(snv).join(baf).join(tumor_ch.hints).set { purple_in }
    pp = PURPLE(purple_in)

    // ── JABBA: segments + purity_ploidy + SV + coverage + SNV
    seg.join(pp).join(sv).join(tumor_cov_keyed).join(snv).set { jabba_in }
    gg = JABBA(jabba_in)

    EVENTS_FUSIONS(gg)
}
