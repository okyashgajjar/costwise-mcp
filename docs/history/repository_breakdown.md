> **Historical research artifact. Not representative of current releases.** Cross-repository retrieval research (Campaign C — 5 repos × 140 queries). Metrics here (e.g. treesitter 22.3%) differ substantially from single-repo benchmarks due to different aggregation and methodology.

# Repository-Specific Breakdown

This report shows retriever performance details for each codebase.

## Repository: Aider

| Retriever | Accuracy | Avg Tokens | Latency | Research Score | Pass Rate |
|---|---|---|---|---|---|
| **treesitter** | 23.6% | 509 | 14ms | 39.42 | 100.0% |
| **auto** | 23.6% | 1255 | 337ms | 37.73 | 100.0% |
| **architecture** | 10.0% | 1395 | 11ms | 22.02 | 100.0% |
| **grep** | 8.6% | 1519 | 839ms | 19.00 | 100.0% |
| **fts** | 10.0% | 610 | 131ms | 16.64 | 100.0% |
| **naive** | 14.3% | 2640 | 0ms | 15.38 | 100.0% |
| **reference** | 0.0% | 103 | 5ms | 14.46 | 100.0% |
| **flowgraph** | 0.0% | 42 | 7ms | 14.02 | 100.0% |
| **callgraph** | 0.0% | 187 | 10ms | 11.88 | 100.0% |

## Repository: OpenCode

| Retriever | Accuracy | Avg Tokens | Latency | Research Score | Pass Rate |
|---|---|---|---|---|---|
| **treesitter** | 18.6% | 839 | 432ms | 27.91 | 100.0% |
| **auto** | 11.4% | 659 | 810ms | 25.83 | 100.0% |
| **flowgraph** | 0.0% | 110 | 85ms | 23.30 | 100.0% |
| **architecture** | 2.9% | 1155 | 145ms | 13.18 | 100.0% |
| **reference** | 0.0% | 277 | 147ms | 10.71 | 100.0% |
| **fts** | 0.0% | 651 | 85ms | 10.27 | 100.0% |
| **grep** | 0.7% | 903 | 2102ms | 10.03 | 100.0% |
| **callgraph** | 0.0% | 551 | 130ms | 9.64 | 100.0% |
| **naive** | 0.0% | 0 | 0ms | 5.00 | 100.0% |

## Repository: Continue

| Retriever | Accuracy | Avg Tokens | Latency | Research Score | Pass Rate |
|---|---|---|---|---|---|
| **auto** | 17.1% | 1241 | 466ms | 31.46 | 100.0% |
| **treesitter** | 20.7% | 779 | 147ms | 30.74 | 100.0% |
| **flowgraph** | 0.0% | 57 | 78ms | 18.12 | 100.0% |
| **naive** | 14.3% | 2996 | 0ms | 14.92 | 100.0% |
| **architecture** | 4.3% | 1643 | 124ms | 14.48 | 100.0% |
| **reference** | 0.0% | 405 | 57ms | 12.14 | 100.0% |
| **grep** | 0.0% | 1656 | 1403ms | 10.71 | 100.0% |
| **fts** | 0.7% | 714 | 108ms | 9.78 | 100.0% |
| **callgraph** | 0.0% | 301 | 39ms | 8.93 | 100.0% |

## Repository: Claude Code

| Retriever | Accuracy | Avg Tokens | Latency | Research Score | Pass Rate |
|---|---|---|---|---|---|
| **treesitter** | 27.9% | 608 | 3ms | 44.52 | 100.0% |
| **architecture** | 19.3% | 1189 | 2ms | 42.50 | 100.0% |
| **auto** | 22.9% | 1113 | 65ms | 34.55 | 100.0% |
| **grep** | 24.3% | 2079 | 231ms | 26.84 | 100.0% |
| **fts** | 13.6% | 829 | 108ms | 19.81 | 100.0% |
| **naive** | 14.3% | 1713 | 0ms | 14.99 | 100.0% |
| **reference** | 0.0% | 130 | 1ms | 9.29 | 100.0% |
| **callgraph** | 0.0% | 151 | 2ms | 8.57 | 100.0% |
| **flowgraph** | 0.0% | 2 | 1ms | 5.89 | 100.0% |

## Repository: MyCLI

| Retriever | Accuracy | Avg Tokens | Latency | Research Score | Pass Rate |
|---|---|---|---|---|---|
| **architecture** | 22.1% | 714 | 4ms | 38.72 | 100.0% |
| **treesitter** | 20.7% | 690 | 4ms | 36.65 | 100.0% |
| **auto** | 13.6% | 1115 | 80ms | 31.91 | 100.0% |
| **grep** | 1.4% | 2463 | 263ms | 24.87 | 100.0% |
| **flowgraph** | 0.0% | 75 | 3ms | 17.32 | 100.0% |
| **reference** | 0.0% | 125 | 2ms | 11.96 | 100.0% |
| **fts** | 2.9% | 559 | 150ms | 10.73 | 100.0% |
| **callgraph** | 0.0% | 157 | 3ms | 9.64 | 100.0% |
| **naive** | 0.0% | 1373 | 0ms | 7.86 | 100.0% |

