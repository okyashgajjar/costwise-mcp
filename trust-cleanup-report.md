# Trust Cleanup Report

Generated during trust cleanup audit. Every file containing benchmark/accuracy/latency/performance data was inspected.

## Files Removed

| File | Reason |
|---|---|
| `token-usage-output.txt` | Private session trace: contained session ID (`ses_11fffd9cbffeja9ghWc1IAZJdB`), model name, token breakdown, tool call trace. |

## Files Archived to `docs/history/`

| File | Reason | Disclaimer |
|---|---|---|
| `benchmark-report.md` | Stale benchmark output (Campaign A — self-benchmark, 85 queries). Accuracy unreliable — 0.0% keyword coverage across all retrievers. | Added |
| `BENCHMARK_TRUST_REPORT.md` | Companion to benchmark-report.md; found 23 false negatives, 0.0% keyword coverage. | Added |
| `benchmark-excellence-report.md` | Stale benchmark output (Campaign B — measured against Aider codebase, not CostAffective). Overview accuracy (100%) conflicts with phase6 report (80%). | Added |
| `phase6-final-report.md` | Continuation of Campaign B. Overview baseline (80%) contradicts excellence report (100%). | Added |
| `costaffective-benchmark-report.md` | Extended Campaign A analysis. Identified ground-truth format loading bug. | Added |
| `architecture-validation.md` | Internal engineering analysis. Flagged FlowGraph for rejection, token estimation errors. | Added |
| `references/reference_readme.md` | **Third-party project.** README for CodeGraph by colbymchenry (npm package, Node.js). Unrelated to CostAffective. | Added |
| `reports/analysis.json` | Stale cross-repo research output (Campaign C — 5 repos × 140 queries). | Added |
| `reports/retriever_comparison.json` | Stale comparison data. | Added |
| `reports/analysis.md` | Stale analysis output. | Added |
| `reports/global_leaderboard.md` | Stale leaderboard output. | Added |
| `reports/category_leaderboard.md` | Stale category breakdown. | Added |
| `reports/repository_breakdown.md` | Stale per-repo breakdown. | Added |
| `reports/retrieval_research_v1.md` | Stale cross-repository research. | Added |
| `reports/retriever_comparison.csv` | Stale CSV data export. | Added |

## Files Edited

| File | Change |
|---|---|
| `README.md` | Removed unverifiable benchmark claims table (81.7%, 45.9%, 33.6%, 54.3%, 70%, 2.2x, "same output quality"). Replaced with honest per-retriever accuracy table from self-benchmark. |
| `README.md` | Fixed "9 MCP tools" → "10 MCP tools" (was missing `recall`). |
| `README.md` | Removed "up to 45.9%" from use cases section. |
| `docs/mcp/README.md` | Clarified "7 code intelligence tools" → "7 retrieval and maintenance tools, plus 3 context-control tools." |
| `medium-article.md` | Removed unverifiable specific percentages (45.9%, 54.3%, 42.1%, 46%) — replaced with qualitative descriptions. Swapped "$2.95/$2.84 cost breakdown" with session-level observation. |

## Files Retained (no changes needed)

| File | Reason |
|---|---|
| `AGENTS.md` | Project documentation, no accuracy claims. |
| `benchmarks/` (entire directory) | Structured test datasets (JSON with query/category/expected_fields). Essential infrastructure. |
| All `.go` source files | Implementation code. |
| All install/config files | Build and deployment scripts. |

## Claims Removed

| Claim | Location | Reason |
|---|---|---|
| 81.7% fewer tokens | README.md | No saved benchmark artifact proving this exact number. |
| 2.2x faster | README.md | No wall-time measurement code exists in benchmark suite. |
| 70% fewer tool calls | README.md | No saved output matching this number. |
| 45.9% lower tokens | README.md + medium-article.md | No saved artifact. Cross-document contradiction (42.1% vs 33.6%). |
| 33.6% fewer API calls | README.md | No saved artifact. Agent-level metric not measurable by existing code. |
| 54.3% fewer exploration loops | README.md + medium-article.md | No saved artifact. Agent-level metric. |
| 42.1% fewer tool interactions | medium-article.md | Contradicts README's 33.6% for same claimed dataset. |
| 46% cost reduction | medium-article.md (headline) | Same unsubstantiated base claim. |
| "Same output quality" | README.md | No quality-equivalence verification performed. |
| "up to 45.9%" | README.md use cases | Same unsubstantiated base claim. |

## Claims Retained (backed by evidence)

| Claim | Evidence |
|---|---|
| SQLite index queries in microseconds | Measured latency tables in benchmark archives. |
| Budgeted summaries stay small regardless of repo size | `BuildRepositorySummaryCompact` tests (50-module synthetic: 1192→216 tokens). |
| Zero-config install, auto-detects clients | Code exists in `cmd/install.go`, `internal/installer/`. |
| 100% local / no API key required | Architectural fact — all retrievers use local SQLite. |
| Pre-built binary releases exist | `.goreleaser.yaml` targets confirmed. |
| Session skill ~275 tokens | Word-count estimate from `internal/skill/policy.md` (1,273 bytes, ~193 words). |
| 10 MCP tools in 3 categories | Confirmed by inspection of `internal/mcpserver/tools.go`. |
