> **Historical research artifact. Not representative of current releases.** Cross-repository retrieval research (Campaign C — 5 repos × 140 queries). Metrics here (e.g. treesitter 22.3%) differ substantially from single-repo benchmarks due to different aggregation and methodology.

# Global Retrievers Leaderboard

This report ranks all 9 retrievers across all 5 evaluation repositories.

## Research Score Leaderboard
Overall weighted rank combining Accuracy (40%), Evidence Coverage (25%), Token Efficiency (20%), Latency Efficiency (10%), and Validation Pass Rate (5%).

| Rank | Retriever | Research Score | Mean Accuracy | Mean Avg Tokens | Mean Latency | Mean Coverage | Pass Rate |
|---|---|---|---|---|---|---|---|
| 1 | **treesitter** | 35.85 | 22.3% | 685 | 120ms | 71.7% | 100.0% |
| 2 | **auto** | 32.29 | 17.7% | 1077 | 352ms | 79.0% | 100.0% |
| 3 | **architecture** | 26.18 | 11.7% | 1219 | 57ms | 51.9% | 100.0% |
| 4 | **grep** | 18.29 | 7.0% | 1724 | 968ms | 41.6% | 100.0% |
| 5 | **flowgraph** | 15.73 | 0.0% | 57 | 35ms | 42.9% | 100.0% |
| 6 | **fts** | 13.44 | 5.4% | 673 | 116ms | 24.3% | 100.0% |
| 7 | **reference** | 11.71 | 0.0% | 208 | 42ms | 26.9% | 100.0% |
| 8 | **naive** | 11.63 | 8.6% | 1744 | 0ms | 12.5% | 100.0% |
| 9 | **callgraph** | 9.73 | 0.0% | 269 | 37ms | 18.9% | 100.0% |

## Accuracy Leaderboard
| Rank | Retriever | Mean Accuracy | StdDev Accuracy | Mean Retrieval Score |
|---|---|---|---|---|
| 1 | **treesitter** | 22.29% | 3.21% | 0.325 |
| 2 | **auto** | 17.71% | 4.85% | 0.371 |
| 3 | **architecture** | 11.71% | 7.78% | 0.211 |
| 4 | **naive** | 8.57% | 7.00% | 0.163 |
| 5 | **grep** | 7.00% | 9.17% | 0.264 |
| 6 | **fts** | 5.43% | 5.39% | 0.095 |
| 7 | **flowgraph** | 0.00% | 0.00% | 0.139 |
| 8 | **reference** | 0.00% | 0.00% | 0.132 |
| 9 | **callgraph** | 0.00% | 0.00% | 0.069 |

## Token Efficiency Leaderboard (Accuracy per 100 Tokens)
| Rank | Retriever | Accuracy per 100 Tokens | Mean Avg Tokens | Mean Accuracy |
|---|---|---|---|---|
| 1 | **treesitter** | 3.42 | 685 | 22.3% |
| 2 | **auto** | 1.65 | 1077 | 17.7% |
| 3 | **architecture** | 1.19 | 1219 | 11.7% |
| 4 | **fts** | 0.78 | 673 | 5.4% |
| 5 | **grep** | 0.37 | 1724 | 7.0% |
| 6 | **naive** | 0.37 | 1744 | 8.6% |
| 7 | **flowgraph** | 0.00 | 57 | 0.0% |
| 8 | **reference** | 0.00 | 208 | 0.0% |
| 9 | **callgraph** | 0.00 | 269 | 0.0% |

## Cost Efficiency Leaderboard (Correct Retrievals per Dollar)
Assumes standard LLM context prompt pricing of $0.005 per 1,000 tokens ($5.00/M).

| Rank | Retriever | Correct Retrievals per Dollar | Mean Avg Tokens | Mean Accuracy |
|---|---|---|---|---|
| 1 | **treesitter** | 68 | 685 | 22.3% |
| 2 | **auto** | 33 | 1077 | 17.7% |
| 3 | **architecture** | 24 | 1219 | 11.7% |
| 4 | **fts** | 16 | 673 | 5.4% |
| 5 | **grep** | 7 | 1724 | 7.0% |
| 6 | **naive** | 7 | 1744 | 8.6% |
| 7 | **flowgraph** | 0 | 57 | 0.0% |
| 8 | **reference** | 0 | 208 | 0.0% |
| 9 | **callgraph** | 0 | 269 | 0.0% |

## Latency Leaderboard (Correct Retrievals per Second)
| Rank | Retriever | Correct Retrievals per Second | Mean Avg Latency | Mean Accuracy |
|---|---|---|---|---|
| 1 | **architecture** | 34.33 | 57ms | 11.7% |
| 2 | **treesitter** | 33.24 | 120ms | 22.3% |
| 3 | **auto** | 1.29 | 352ms | 17.7% |
| 4 | **fts** | 0.46 | 116ms | 5.4% |
| 5 | **grep** | 0.24 | 968ms | 7.0% |
| 6 | **naive** | 0.00 | 0ms | 8.6% |
| 7 | **flowgraph** | 0.00 | 35ms | 0.0% |
| 8 | **reference** | 0.00 | 42ms | 0.0% |
| 9 | **callgraph** | 0.00 | 37ms | 0.0% |

