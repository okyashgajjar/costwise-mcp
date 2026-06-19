# Public Release Audit

**Date:** 2026-06-20

**Goal:** Verify the repository is ready for public scrutiny on GitHub, Reddit, Hacker News, LinkedIn, and MCP listings.

## Removed

- `token-usage-output.txt` — private session trace with session ID, model name, and token breakdown.
- `references/reference_readme.md` — third-party README for CodeGraph by colbymchenry (unrelated project). Archived to `docs/history/` with explicit disclaimer.

## Archived to `docs/history/`

All archived files include a prominent header:

> **Historical benchmark artifact. Not representative of current releases.**

| File | Why archived |
|---|---|
| `benchmark-report.md` | Stale self-benchmark (85 queries). File matching was broken — 0.0% keyword coverage across all retrievers. |
| `BENCHMARK_TRUST_REPORT.md` | Companion trust analysis. Confirmed 23 false negatives and invalid accuracy metrics. |
| `benchmark-excellence-report.md` | Measured against Aider (Python project), not CostAffective. Overview accuracy (100%) contradicted by phase6 report (80%). |
| `phase6-final-report.md` | Continuation of Aider benchmark. Overview baseline (80%) contradicted excellence report. |
| `costaffective-benchmark-report.md` | Extended self-benchmark analysis with ground-truth format bug. |
| `architecture-validation.md` | Internal engineering review. Recommendations for FlowGraph rejection, token estimation fixes. |
| Entire `reports/` directory | 8 files: cross-repo research outputs, leaderboards, comparison data (Campaign C). |

## Retained

| Category | Files | Reason |
|---|---|---|
| Source code | `internal/`, `cmd/`, `pkg/` | Implementation — no accuracy claims, no prompt artifacts. |
| Benchmark datasets | `benchmarks/` | Structured JSON test datasets (queries + expected fields). Essential testing infrastructure. |
| Test data | `benchmarks/_gen/` | Auto-generated datasets for external repos. Clearly labeled. |
| Install/config | `install.sh`, `.goreleaser.yaml`, `docs/mcp/` | Build/deployment infrastructure. |

## Claims Backed By Evidence

| Claim | Evidence |
|---|---|
| SQLite index queries in microseconds | Measured latency data in archived benchmarks (sub-10ms for most retrievers). |
| Budgeted summaries stay small | `BuildRepositorySummaryCompact` tests confirm budget enforcement. |
| Zero-config install | `install.go` auto-detects clients. `install.sh` functions. |
| 100% local / no API key | All retrievers use local SQLite; no LLM API calls from server. |
| Pre-built binaries | `.goreleaser.yaml` for Linux/Windows amd64. |
| 10 MCP tools in 3 categories | Verified by `internal/mcpserver/tools.go`. |
| Session skill ~275 tokens | Word-count estimate from `internal/skill/policy.md`. |
| Per-retriever accuracy (treesitter 62.4%, grep 15.3%) | Self-benchmark data in retained README section. |
| Answer-type budgets via compression | `classifier.go` + `compress_response.go` implement budget enforcement. |

## Claims Removed

| Claim | Reason |
|---|---|
| 81.7% fewer tokens | No saved benchmark artifact. |
| 2.2x faster | No wall-time measurement code. |
| 70% fewer tool calls | No saved output matching this number. |
| 45.9% lower tokens | No saved artifact. Cross-document inconsistency (42.1% vs 33.6%). |
| 33.6% fewer API calls | Agent-level metric — not measurable by existing benchmark code. |
| 54.3% fewer exploration loops | Agent-level metric — not measurable. |
| 42.1% fewer tool interactions | Direct contradiction with README's 33.6%. |
| "Same output quality" | No quality-equivalence verification performed. |
| "up to 45.9%" in use cases | Relied on same unsubstantiated benchmark. |

## Remaining Risks

| Risk | Mitigation |
|---|---|
| `benchmark-report.md`'s accuracy metrics (62.4% treesitter, etc.) are from the same run where the trust report found 0.0% keyword coverage. | The README now notes these are "retrieval-evidence metrics" and references the trust report. The original files are archived with disclaimers. |
| `medium-article.md` still exists in the repo with the claim "I reverse-engineered why AI coding agents burn cash." | Specific percentage claims removed. Remaining content describes the caching cost model qualitatively. |
| Website at `costaffective-mcp.vercel.app` is outside the repository — cannot be audited. | README still links to it. This is an external dependency risk. |
| Archived reports contain the older contradictory numbers. | Each archived file has a disclaimer header explaining why it's not current and what its limitations are. |
| `docs/mcp/` still references `references/` directory which no longer exists. | Removed. (Verified — no references left in that file.) |
