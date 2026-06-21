> **Historical benchmark artifact. Not representative of current releases.** This trust report was generated alongside `benchmark-report.md` (Campaign A — CostWise self-benchmark, 85 queries). It found 0.0% keyword coverage for all 6 retrievers and 23 false negatives, indicating the file-matching accuracy metrics were unreliable due to path normalization mismatches.

# Benchmark Trust Report

**Generated:** 2026-06-06T20:17:44+05:30

> **NOTE:** All metrics in this report measure **RETRIEVAL EVIDENCE**. Since no LLM is executed during benchmark runs, there are no 'answers' to evaluate. Terms like 'Coverage' and 'Accuracy' refer exclusively to whether the retriever surfaced the expected files, symbols, and keywords.

## 1. Dataset Coverage

| Category | Entries | With Files | With Symbols | With Keywords |
|---|---|---|---|---|
| definition | 25 | 0 | 0 | 0 |
| caller | 25 | 4 | 0 | 0 |
| architecture | 20 | 20 | 0 | 0 |
| repository | 15 | 15 | 0 | 0 |

## 2. Retrieval Evidence Accuracy (File Matching)

> Measures whether the retriever surfaced the exact expected file paths. Uses strict equality matching.

| Retriever | File Match Rate | Avg Retrieval Score (Weighted) |
|---|---|---|
| architecture | 32.9% | 0.131 |
| treesitter | 62.4% | 0.203 |
| grep | 15.3% | 0.050 |
| reference | 0.0% | 0.000 |
| callgraph | 0.0% | 0.000 |
| flowgraph | 0.0% | 0.000 |

## 3. Evidence Coverage (Keywords)

> Measures whether the context snippets provided by the retriever contain the expected keywords.

| Retriever | Sufficient Evidence Rate | Avg Keyword Coverage |
|---|---|---|
| architecture | 0.0% | 0.0% |
| treesitter | 0.0% | 0.0% |
| grep | 0.0% | 0.0% |
| reference | 0.0% | 0.0% |
| callgraph | 0.0% | 0.0% |
| flowgraph | 0.0% | 0.0% |

## 4. Token Accounting

> Only Context Tokens are counted during benchmark runs.

| Retriever | Avg Context Tokens |
|---|---|
| architecture | 304 |
| treesitter | 688 |
| grep | 1454 |
| reference | 166 |
| callgraph | 0 |
| flowgraph | 53 |

## 5. Validation & Anomalies

> Detects logically inconsistent benchmark results.

- **Validation Pass Rate:** 100.0%
- **False Positives Detected:** 0
- **False Negatives Detected:** 23
- **Invalid Evaluations:** 0

