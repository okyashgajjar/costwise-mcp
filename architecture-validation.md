# CostAffective Architecture Validation Report

**Generated:** 2026-06-06
**Scope:** Full architecture review, compression analysis, gap analysis, roadmap

---

## Part 1 — Validate Existing Features

### 1A. Context Compression Engine (`internal/retrieval/compress.go`)

#### Token Reduction Measurement

The engine uses `len(ctx) / 4` for token estimation. This is a critical flaw:

| Issue | Impact |
|---|---|
| `len/4` assumes English prose (4 chars/token) | Code is denser; Go averages ~3.1 chars/token, Python ~3.8, JS/TS ~3.3 |
| No per-language tokenization | Compression ratios are language-dependent but measured uniformly |
| `compressOverview` hardcodes 500-char snippet ceiling | Same absolute ceiling regardless of answer type |

**Estimated reduction by language** (based on 4-char assumption error):

| Language | Actual chars/token | Estimated Reduction | Real Reduction |
|---|---|---|---|
| JavaScript | ~3.3 | claimed by code | ~21% lower than claimed |
| TypeScript | ~3.3 | claimed by code | ~21% lower than claimed |
| Go | ~3.1 | claimed by code | ~29% lower than claimed |
| Python | ~3.8 | claimed by code | ~5% lower than claimed |

**Result:** The compression engine likely overstates its effectiveness by 5-29% depending on language because `len/4` overcounts code tokens.

#### Compression Strategy Weaknesses

1. **`shortenPath` (compress.go:231-242)** — Only handles `/internal/`, `/cmd/`, `/pkg/` prefixes. Misses:
   - `/src/`, `/lib/`, `/packages/`, `/modules/`
   - Nested monorepo paths
   - Falls back to `filepath.Base` which strips all directory context

2. **Import block truncation (cleanSnippet, compress.go:244-303)** — Correctly identifies `import (`, `import {` blocks but:
   - State machine can get confused by nested braces in Go generics
   - `importCount` variable is shared across ALL import blocks in a file (persists across blocks)
   - Truncation of imports removes valuable dependency-resolution information

3. **Duplicate compression logic** — `compressCaller` and `compressReference` are nearly identical (lines 95-161 vs 129-161, differing only in header). This is duplicated code that will drift over time.

4. **No structural compression** — The engine removes import detail but doesn't:
   - Collapse type definitions to signatures only
   - Remove comments/docstrings
   - Remove blank lines
   - Remove test assertions vs. production code
   - Preserve function signatures while removing bodies

5. **Budget overrun possible** — `compressOverview` and `compressDefault` check `totalTokens > budget` after adding each file, but a single large snippet can exceed budget by multiples.

#### Recommendations

1. **Replace `len/4` with per-language token counting** (use a tokenizer or language-specific estimator)
2. **Add structural compression modes:**
   - `collapse-bodies`: function signatures only, no bodies
   - `drop-comments`: remove all comments/docstrings
   - `drop-blank-lines`: collapse consecutive blank lines to one
   - `truncate-lines`: cap individual line length at 80 chars
3. **Fix `shortenPath`** to handle `src/`, `lib/`, `packages/*/` patterns
4. **Merge `compressCaller` and `compressReference`** into a shared formatResults helper
5. **Add language-specific compression presets** (Python: preserve docstrings; Go: preserve type definitions; JS/TS: preserve JSDoc)

---

### 1B. Budget-Aware Retrieval

#### Budget Threshold Analysis

Current budgets from `GuessContextBudget` (pipeline.go:164-193) vs. actual needs:

| Answer Type | Context Budget | Expected Output (MaxTokens) | Ratio (budget/output) |
|---|---|---|---|
| YesNo | 100 | 10 | 10x |
| Location | 200 | 25 | 8x |
| Caller | 500 | 50 | 10x |
| Reference | 500 | 50 | 10x |
| Overview | 2000 | 150 | 13x |
| Explanation | 3000 | 400 | 7.5x |
| Improvement | 3000 | 200 | 15x |
| ArchitectureReview | 3000 | 250 | 12x |
| FeatureSuggestion | 2500 | 200 | 12.5x |
| RepositoryAnalysis | 4000 | 300 | 13x |

**Finding:** Context budgets are 7-15x larger than output budgets. This is deliberately generous, but lacks empirical basis. The 0% benchmark accuracy for `reference` and `callgraph` retrievers (from benchmark-report.md) suggests budget alone doesn't fix retrieval quality.

#### Quality Gate Thresholds

`CheckQualityGate` (pipeline.go:86-131):

- `topScore < 0.15` → fail for all types (too low; 0.15 means near-random match)
- `topScore >= 0.3` → pass for YesNo/Location (acceptable)
- `>= 3 results` → required for Improvement, RepositoryAnalysis, ArchitectureReview, FeatureSuggestion (too few; 3 results is insufficient for meaningful analysis)

**Problems:**
1. Quality gate always returns `StepLLM` as the step — the step field is meaningless
2. The 0.15 threshold captures noise; should be 0.3 minimum for all types
3. The evidence requirement for analysis types (>= 3 results) is too low — at least 5 results needed for meaningful analysis

#### Aggressive Truncation Impact

Tools that suffer most from small budgets:
1. **FlowGraphRetriever** — already 0% accuracy at 34 avg tokens. More budget won't help; the fundamental approach is broken.
2. **ReferenceRetriever** — 0% accuracy at 96 avg tokens. Budget isn't the problem; symbol extraction is.
3. **CallGraphRetriever** — 0% accuracy at 182 avg tokens. Same issue.

**Recommendations:**
- Reduce context budgets by 30-40% across all types (empirical tuning needed)
- Raise quality gate minimum from 0.15 to 0.3
- Increase evidence threshold for analysis types from 3 to 5
- Response compression (compress_response.go) thresholds are reasonable (2x MaxTokens)

---

### 1C. Retrieval Ranking (`internal/retrieval/filters.go`)

#### Current Scoring

`FilterResults` (filters.go:10-82) provides:
- Score-based descending sort
- Directory path exclusions (15 excluded dir patterns)
- Filename pattern exclusions (5 patterns)
- Result count cap

#### Missing Scoring Dimensions

| Feature | Status | Impact |
|---|---|---|
| Exact-match boost | SymbolDB has `computeSymbolScore` (db.go:546-581) but filters.go doesn't use it | Medium |
| Symbol resolution boost | Not present | High — symbols in same package should rank higher |
| Path relevance boost | Not present | High — files in same directory/module should rank higher |
| Repository structure boost | Not present | Medium — entry point files vs. utility files |
| Definition vs. reference | `references_t` has `ref_type` field but unused in scoring | High — definitions should rank above references |

#### Failure Cases

1. A utility file with many symbol matches can outrank the definition file
2. Test files are NOT excluded by filters.go (only chat-history, .gold., .snapshot., .log, benchmark-report)
3. Node_modules is excluded by indexer's `skipDir` but not enforced at query time — if a retriever bypasses the indexer, node_modules could leak

#### Recommendations

1. **Add symbol resolution boost** — If the query matches a symbol name AND the file is in the same Go package/TS module, add +0.2 score
2. **Add path relevance** — Score = base_score + (0.1 * path_depth_relevance) where files at the same directory level as common entry points get a boost
3. **Prefer definitions over references** — Files containing the matching symbol's definition get +0.15
4. **Exclude test files** from all results unless query explicitly contains "test"
5. **Move `skipDir` logic** from indexer into a shared filter so all retrievers benefit

---

### 1D. Benchmark Trust Validity

From BENCHMARK_TRUST_REPORT.md:

**Critical finding:** File Match Rate is 0.0% for ALL retrievers. The benchmark uses "strict file path equality" but the stored ground-truth paths likely don't match the retrievers' output paths (absolute vs. relative, prefixed differently). This means:
- All accuracy metrics in benchmark-report.md are based on keyword coverage, NOT file matching
- The "96% Router Accuracy" metric is untrustworthy for retrieval quality

---

## Part 2 — Benchmark Design

### Compression Quality Benchmarks

#### Token Reduction Measurement

```
BenchmarkCompressionReduction/repo_type=react
  OriginalTokens:      measurement
  CompressedTokens:    after CompressForAnswerType
  ReductionPercent:    (orig - compressed) / orig * 100
  OverheadPercent:     (len/4_estimate - actual_tokens) / actual_tokens * 100

BenchmarkCompressionReduction/repo_type=nextjs  (same structure)
BenchmarkCompressionReduction/repo_type=go       (same structure)
BenchmarkCompressionReduction/repo_type=python   (same structure)
```

Test with each answer type (YesNo, Location, Caller, Reference, Overview, Explanation, Improvement). Measure per-language.

#### Structural Compression

```
BenchmarkStructuralCompression/mode=signatures-only
BenchmarkStructuralCompression/mode=no-comments
BenchmarkStructuralCompression/mode=all
```

Measure: tokens before → after, semantic preservation (symbols/definitions retained).

### Retrieval Accuracy Benchmarks

```
BenchmarkRetrievalTopK/k=1, category=symbol_lookup
BenchmarkRetrievalTopK/k=3, category=symbol_lookup
BenchmarkRetrievalTopK/k=5, category=symbol_lookup
BenchmarkRetrievalTopK/k=1, category=caller_lookup
BenchmarkRetrievalTopK/k=3, category=caller_lookup
BenchmarkRetrievalTopK/k=5, category=caller_lookup
BenchmarkRetrievalTopK/k=1, category=repository_search
BenchmarkRetrievalTopK/k=3, category=repository_search
BenchmarkRetrievalTopK/k=5, category=repository_search
```

**Implementation:** Use the existing `benchmarks/definitions.json`, `benchmarks/callers.json`, `benchmarks/concepts.json` datasets with path-normalized ground truth.

**Fix:** Normalize paths to relative before comparison.

### Budget Performance Benchmarks

```
BenchmarkBudgetSuccess/size=small   (50 tokens)
BenchmarkBudgetSuccess/size=medium  (200 tokens)
BenchmarkBudgetSuccess/size=large   (1000 tokens)
```

Measure for each answer type: Does retrieval succeed (pass quality gate) within budget?

**Find minimum token budget** by binary search for each answer type + language combination.

### MCP Performance Benchmarks

```
BenchmarkMCPPerformance/files=10000
BenchmarkMCPPerformance/files=50000
BenchmarkMCPPerformance/files=100000
```

Measure:
- Index time (seconds)
- Query latency (ms) for symbol lookup, reference lookup, grep
- Memory usage (MB RSS)
- Database growth (MB on disk)

Use synthetic repos with generated files at each scale.

---

## Part 3 — Gap Analysis

### Unsolved Retrieval Problems

Ranked by (1) Retrieval Quality Impact, (2) Token Reduction Impact, (3) Implementation Cost.
Scale: 1 (low) to 5 (high)

| Gap | Quality Impact | Token Reduction | Implementation Cost | Priority Score |
|---|---|---|---|---|
| **Alias imports** (TS paths, Go replace) | 5 | 4 | 3 | 12 |
| **Monorepo workspace resolution** | 5 | 4 | 4 | 13 |
| **Path resolution** (relative → absolute) | 4 | 2 | 2 | 8 |
| **Framework routing** (Next.js pages, React Router) | 3 | 3 | 4 | 10 |
| **Symbol tracing** (cross-module) | 4 | 3 | 5 | 12 |
| **Type-aware retrieval** (interface impl detection) | 3 | 2 | 5 | 10 |
| **Cross-language references** (Python → C extensions) | 2 | 1 | 5 | 8 |

#### Detailed Gap Analysis

**1. Alias Imports (Priority: Critical)**
- TypeScript `paths` in tsconfig.json resolve `@/components/Button` to `src/components/Button.tsx`
- Go `replace` directives in go.mod redirect module paths
- Current system stores import strings as-is; no resolution layer
- Missing alias → symbol lookup fails → LLM gets no context → hallucination
- **Fix cost:** moderate (parse tsconfig/go.mod, build resolution table)

**2. Monorepo Workspace Resolution (Priority: Critical)**
- No understanding of yarn workspaces, pnpm workspaces, Go workspaces
- Cross-package symbol lookup (e.g., `@scope/package/src/util`) fails
- Indexer walks entire monorepo but doesn't track package boundaries
- **Fix cost:** moderate (detect workspace root, per-package index)

**3. Framework Route Awareness (Priority: High)**
- Next.js App Router (`app/page.tsx`, `app/api/route.ts`)
- File-based routing maps to URL paths
- LLM answering "how does the /api/users endpoint work?" needs route → file mapping
- **Fix cost:** moderate (parse directory conventions, generate route map)

**4. Symbol Tracing — Cross-Module (Priority: High)**
- Current call graph is intra-file only (walk per-file AST)
- No cross-module call chain tracing
- "What calls RepoMap.Build()?" only finds direct calls in same file
- **Fix cost:** high (requires building full call graph across all files)

**5. Type-Aware Retrieval (Priority: Medium)**
- Interface implementation not tracked
- Can't answer "what implements the Retriever interface?"
- TypeScript interface/type resolution not available
- **Fix cost:** high (requires type checker integration)

---

## Part 4 — Phase 2 Design Review

### Proposed Resolver Architecture

The proposed `internal/resolver/` package with `alias.go`, `workspace.go`, `framework.go`, `symbol_tracer.go`:

#### Architecture Quality Assessment

**Strengths:**
- Separate package is correct — resolution is a cross-cutting concern
- Each resolver handles one domain (good separation)
- Resolvers can be composed into a pipeline

**Weaknesses:**
- 4 separate files = 4 interfaces and 4 implementations = more code than needed
- `framework.go` is too broad — framework routing and framework component patterns are different problems
- `symbol_tracer.go` overlaps with existing callgraph + flowgraph retrievers

#### Unnecessary Abstractions

1. **`framework.go`** — Should be two separate resolvers: `route_resolver.go` (file-based routing) and `component_resolver.go` (component tree). Or better, one resolver with two phases.

2. **`symbol_tracer.go`** — The existing `CallGraphRetriever` and `FlowGraphRetriever` already attempt this with 0% accuracy. A new tracer would duplicate this. Instead, fix the existing ones.

#### Missing Components

1. **`import_resolver.go`** — Resolves import paths to physical files (alias resolution, module resolution, path normalization). This is the #1 gap and not in the proposal.

2. **`monorepo.go`** — Detects workspace structure, maps packages, enables cross-package queries.

#### Simplification Recommendations

**Merge into 2 files instead of 4:**

```
internal/resolver/
  resolver.go       — Interface + pipeline composition
  imports.go        — Alias resolution + workspace + module resolution (all path-based)
  routes.go         — Framework routing detection (Next.js, Express, etc.)
```

**Remove `symbol_tracer.go` entirely** — Existing `CallGraphRetriever` + `FlowGraphRetriever` should be fixed instead.

#### Rationale

The proposed 4-file design has too many abstractions for what is fundamentally a path-resolution problem:
- Alias resolution: path → path (1:1 mapping)
- Workspace resolution: package name → path (1:1 mapping)
- Framework routing: URL pattern → path (1:1 mapping)
- Symbol tracing: symbol → callers/callees (already exists in callgraph.go)

None of these need separate files. The first three can share a `ResolveImport(ctx, importPath) → []ResolvedPath` interface.

---

## Part 5 — Specific Questions

### Language Scope: V0.2

**Recommendation: Option B — TypeScript, JavaScript, Go, Python immediately.**

**Reasoning:**
- Tree-sitter already supports all four languages (treesitter/language.go:10-15 confirms this)
- All four are already implemented in extractors (extract.go), callgraph (callgraph.go), reference parsing (reference.go)
- The `IsSupported` function (language.go:33-35) already accepts `.go`, `.py`, `.js`, `.jsx`, `.mjs`, `.ts`, `.tsx`
- Restricting to TS/JS only would leave Go and Python users with no retrieval — grep-only fallback
- Four languages add negligible complexity since the AST extraction patterns are already written
- Token efficiency argument: Go and Python tend to be more verbose → more token reduction opportunity

### Framework Awareness

**Recommendation: Option B — Remain internal and influence retrieval scoring only.**

**Justification (token efficiency):**
1. Framework metadata adds ~50-200 tokens per query to the context if returned to MCP clients
2. Internal scoring adds 0 tokens — it's a simple weight adjustment
3. Clients don't need framework information; they need accurate symbol locations
4. The mission is "smaller context with equal or better answer quality" — adding framework info to output violates the primary objective

### Database Design: `references_t` vs. `imports_t`

**Recommendation: Extend `references_t` with a `source_module` column. Do NOT create a new table.**

**Rationale (lowest-complexity design):**
- `references_t` already has `symbol_name`, `file`, `line`, `col`, `ref_type`, `context`
- `ref_type` already distinguishes `import`, `reference`, `definition`, `export`
- Adding a `source_module TEXT` column captures the importing module's package/module name
- Adding a `resolved_path TEXT` column captures the resolved file path after alias/workspace resolution
- A new `imports_t` table would duplicate all `references_t` columns plus add workspace/alias fields
- Two tables means JOIN queries for common patterns ("find symbols imported by this file")
- One table with proper indexing is simpler, faster, and maintains referential integrity

**Schema change:**
```sql
ALTER TABLE references_t ADD COLUMN source_module TEXT NOT NULL DEFAULT '';
ALTER TABLE references_t ADD COLUMN resolved_path TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_refs_module ON references_t(source_module);
CREATE INDEX IF NOT EXISTS idx_refs_resolved ON references_t(resolved_path);
```

---

## Part 6 — Updated Roadmap

### V0.2 — Foundation Fixes (0-2 months)

**Must Have:**
1. Fix token counting: replace `len/4` with per-language estimation
2. Fix `shortenPath`: support `src/`, `lib/`, `packages/*/` patterns
3. Raise quality gate min score from 0.15 to 0.3
4. Raise evidence threshold for analysis types from 3 to 5
5. Normalize benchmark paths (relative) for accurate measurement
6. Exclude test files from default results

**Should Have:**
7. Import resolution: parse tsconfig paths, go.mod replace directives
8. Structural compression: signature-only mode for code bodies
9. Add `source_module` + `resolved_path` to `references_t`

**Nice to Have:**
10. Monorepo workspace detection
11. Route-aware retrieval (Next.js pages→files)

**Reject:**
- FlowGraph enhancements (0% accuracy, high complexity)
- Symbol tracer (fix existing callgraph instead)
- Framework metadata in MCP output (adds tokens, no quality gain)

### V0.3 — Compression Optimization (2-4 months)

**Must Have:**
1. Language-specific compression presets (Go: keep types; Python: keep docstrings; JS/TS: keep JSdoc)
2. Budget reduction: lower context budgets 30% with empirical validation
3. Remove duplicate code in compressCaller/compressReference

**Should Have:**
4. Response compression for all answer types (currently only checks 2x threshold)
5. Cache compression results in KnowledgeStore

**Nice to Have:**
6. Adaptive budget: increase budget only when quality gate fails
7. Compression quality metrics dashboard

**Reject:**
- Image/video compression (out of scope)

### V0.4 — Ranking & Quality (4-6 months)

**Must Have:**
1. Symbol resolution boost in scoring (definitions rank above references)
2. Path relevance — same-module files get score boost
3. Type-aware scoring: definitions > imports > references
4. Merge `filters.go` and `skipDir` into shared filtering

**Should Have:**
5. Cross-module call graph (full-resolution)
6. Interface implementation tracking (implements keyword/parent type)

**Nice to Have:**
7. Semantic similarity scoring (embedding-based reranking)
8. Query intent disambiguation (short vs. long queries)

**Reject:**
- Full type checker integration (too expensive, marginal benefit)

### V0.5 — Cross-Language & Monorepo (6-8 months)

**Must Have:**
1. Monorepo detection (workspace root, package boundaries)
2. Cross-package symbol resolution
3. Alias resolution integrated into pipeline

**Should Have:**
4. Framework route extraction (Next.js, Express)
5. Cross-language reference resolution (JS→TS, Python→C)

**Nice to Have:**
6. Dynamic budget per-package (complex packages get more budget)
7. Incremental monorepo re-indexing

**Reject:**
- Full dependency graph visualization (not retrieval focused)

### V0.6 — Production Hardening (8-10 months)

**Must Have:**
1. Performance benchmarks at 10k/50k/100k files
2. Memory optimization for large repos
3. Database compaction and WAL management
4. Error recovery (partial index, retry on parse failure)

**Should Have:**
5. Compression quality validation (round-trip: compress → decompress → compare)
6. Cache hit ratio monitoring

**Nice to Have:**
7. Custom compression strategies per repository
8. Query cost estimation before execution

**Reject:**
- Graph database backend (over-engineered for token reduction)

---

## Summary of Critical Issues

| Issue | File | Severity | Fix Complexity |
|---|---|---|---|
| Token counting uses `len/4`, wrong for code | compress.go:61,89,124,157,193,227 | High | Low |
| `shortenPath` misses common path patterns | compress.go:231-242 | Medium | Low |
| Duplicate compression logic | compress.go:95-161, 129-161 | Low | Low |
| Quality gate min score too low (0.15) | pipeline.go:96 | Medium | Low |
| Benchmark file matching is broken | all benchmarks | Critical | Medium |
| No import/alias resolution | — | High | Medium |
| No monorepo support | — | High | Medium |
| Test files not excluded in filters | filters.go | Medium | Low |
| Budget not empirically validated | pipeline.go:164-193 | Medium | Medium |
| FlowGraphRetriever 0% accuracy | flowgraph.go | Low (reject) | — |

**Primary Recommendation:** Fix the token counting first. Everything else builds on accurate measurement. Without knowing true token counts, all compression ratios, budget decisions, and quality metrics are unreliable.
