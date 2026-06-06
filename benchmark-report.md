# Benchmark Report

**Timestamp:** 2026-06-06T20:17:43+05:30

**Total Queries:** 85

## Summary

| Metric | Value |
|---|---|
| Router Accuracy | 0.0% |
| Retrieval Accuracy | 71.2% |
| Winner Accuracy | 71.2% |
| Avg Context Tokens (winner) | 712 |
| Avg Total Tokens (all retrievers) | 2666 |
| Avg Cost | $0.00000 |
| Avg Latency | 9ms |

## Accuracy Report

| Category | Queries | Accuracy | Best Retriever |
|---|---|---|
| definition | 25 | 96.0% | treesitter |
| caller | 25 | 28.0% | treesitter |
| overview | 15 | 46.7% | treesitter |

## Per-Retriever Metrics

| Retriever | Accuracy | Avg Tokens | Avg Conf | Cal Err | Acc/100Tok | Avg Latency | Wins |
|---|---|---|---|---|---|---|---|
| architecture | 32.9% | 304 | 0.52 | 19.2% | 0.11 | 6ms | 7 |
| treesitter | 62.4% | 688 | 0.99 | 36.5% | 0.09 | 7ms | 75 |
| grep | 15.3% | 1454 | 0.91 | 75.3% | 0.01 | 233ms | 2 |
| reference | 0.0% | 166 | 0.25 | 25.5% | 0.00 | 2ms | 0 |
| callgraph | 0.0% | 0 | 0.00 | 0.0% | 0.00 | 1ms | 0 |
| flowgraph | 0.0% | 53 | 0.36 | 36.0% | 0.00 | 4ms | 1 |

## Efficiency Report

| Metric | Value |
|---|---|
| Cost per correct answer | $0.00000 |
| Tokens per correct answer | 0 |
| Latency per correct answer | 30ms |
