# Benchmark Analysis Report

*Generated from existing benchmark data — no retrieval executed.*

## A. Global Leaderboard

| Rank | Retriever | Research Score | Accuracy | Token Eff | Cost Eff | Latency Eff | StdDev | Pass Rate |
|------|-----------|---------------|----------|-----------|----------|-------------|--------|----------|
| 1 | **treesitter** | 35.85 | 22.3% | 3.42 | 68 | 33.24 | 3.21% | 100% |
| 2 | **auto** | 32.29 | 17.7% | 1.65 | 33 | 1.29 | 4.85% | 100% |
| 3 | **architecture** | 26.18 | 11.7% | 1.19 | 24 | 34.33 | 7.78% | 100% |
| 4 | **grep** | 18.29 | 7.0% | 0.37 | 7 | 0.24 | 9.17% | 100% |
| 5 | **flowgraph** | 15.73 | 0.0% | 0.00 | 0 | 0.00 | 0.00% | 100% |
| 6 | **fts** | 13.44 | 5.4% | 0.78 | 16 | 0.46 | 5.39% | 100% |
| 7 | **reference** | 11.71 | 0.0% | 0.00 | 0 | 0.00 | 0.00% | 100% |
| 8 | **naive** | 11.63 | 8.6% | 0.37 | 7 | 0.00 | 7.00% | 100% |
| 9 | **callgraph** | 9.73 | 0.0% | 0.00 | 0 | 0.00 | 0.00% | 100% |

## B. Repository Breakdown

| Repository | Best Retriever | Score | Worst Retriever | Score | Best Accuracy | Best Tokens | Best Latency |
|------------|---------------|-------|-----------------|-------|---------------|-------------|-------------|
| Aider | **treesitter** | 39.42 | callgraph | 11.88 | 23.6% | 42 | 4.8ms |
| Claude Code | **treesitter** | 44.52 | flowgraph | 5.89 | 27.9% | 2 | 0.6ms |
| Continue | **auto** | 31.46 | callgraph | 8.93 | 20.7% | 57 | 38.7ms |
| MyCLI | **architecture** | 38.72 | naive | 7.86 | 22.1% | 75 | 2.0ms |
| OpenCode | **treesitter** | 27.91 | naive | 5.00 | 18.6% | 110 | 84.7ms |

## C. Category Breakdown

| Category | Best Retriever | Best Accuracy | Worst Retriever | Worst Accuracy |
|----------|---------------|---------------|-----------------|----------------|
| definition | **treesitter** | 91.0% | naive | 0.0% |
| reference | **naive** | 0.0% | naive | 0.0% |
| caller | **treesitter** | 65.0% | naive | 0.0% |
| repository | **naive** | 60.0% | treesitter | 0.0% |
| architecture | **naive** | 0.0% | naive | 0.0% |
| flow | **naive** | 0.0% | naive | 0.0% |
| concept | **naive** | 0.0% | naive | 0.0% |

## D. Auto Router Audit

- **Auto Accuracy:** 17.7%
- **Best Static Retriever:** treesitter (22.3%)
- **Accuracy Gap:** -4.6%
- **Perfect Router Potential Gain:** +4.9%

## E. Oracle Analysis

*Oracle selects the best-performing retriever per category per repository after the fact.*

- **Oracle Accuracy:** 31.1%
- **Current Auto Accuracy:** 17.7%
- **Improvement Potential:** +13.4%

## F. Failure Analysis

| Rank | Failure Reason | Count | Percentage |
|------|---------------|-------|------------|
| 1 | Wrong file returned | 544 | 100.0% |
| 2 | Wrong routing (auto) | 32 | 5.9% |
| 3 | Low keyword coverage | 0 | 0.0% |
| 4 | No results returned | 0 | 0.0% |

## G. Recommendations

1. **Production Default Retriever:** `treesitter`
2. **Largest Bottleneck:** 4 of 7 categories have 0% accuracy across all retrievers (including: reference)
3. **Highest-Gain Action:** Improve auto-router to close the 13.4% gap to oracle accuracy (17.7% → 31.1%)
4. **Next Phase Focus:** `retrieval quality` — 4 categories have zero accuracy. No amount of routing or ranking improvement can fix missing retrieval coverage.
