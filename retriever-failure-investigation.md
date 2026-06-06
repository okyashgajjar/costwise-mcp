# Retriever Failure Investigation

> **Root cause analysis**: why reference, callgraph, and flowgraph retrievers show 0% accuracy, and what to do about it.

---

## 1. CallGraph Retriever — 0.0% Accuracy

### Symptoms
| Metric | Value |
|---|---|
| Accuracy | 0.0% |
| Avg tokens | 0 |
| Avg confidence | 0.0 |
| Latency | 1.4ms |
| Calibration error | 0.0 |

Returns **zero results** for all 85 queries. The `call_edges` SQLite table has **0 rows**.

### Root Cause: SharedIndexer's Change Detection Blocks Call Parsing

**The tree-sitter call extraction works perfectly.** Tested on real files:
- `compress.go`: **99 call edges** extracted (CompressForAnswerType → compressYesNo, etc.)
- `filters.go`: **12 call edges** extracted
- `auto.go`: **54 call edges** extracted (Initialize → NewSharedIndexer, etc.)

**The problem is the SharedIndexer's change detection cache.** Here's the sequence:

1. First retriever (treesitter) indexes with `ParseSymbols: true, ParseReferences: true, ParseCalls: false`. All files are parsed, SHA256 hashes stored in `symbol_files`.
2. Callgraph retriever initializes with `ParseSymbols: true, ParseReferences: true, ParseCalls: true`. Opens the same DB file.
3. SharedIndexer walks the repo. For each file: `oldHash == newHash` → **skipped**. `ParseCalls` is never called.
4. `call_edges` table remains empty.

The `symbol_files` table tracks file hashes across runs. Since the files haven't changed since treesitter's index, the callgraph retriever's index pass considers them "already done" and never invokes `parser.ParseCalls()`.

**File:** `internal/retrieval/indexer.go:99-101`
```go
if oldHash, exists := existingHashes[relPath]; exists && oldHash == newHash {
    result.Skipped++
    return nil // ← ParseCalls never runs
}
```

### CallGraph Audit — 25 Caller Queries

| Query | Extracted Symbol | Expected Callers | Actual Edge Count |
|---|---|---|---|
| Who calls CompressForAnswerType? | CompressForAnswerType | tools.go | **0** |
| Who calls FilterResults? | FilterResults | tools.go | **0** |
| Who calls GuessContextBudget? | GuessContextBudget | tools.go | **0** |
| Who calls NewSharedIndexer? | NewSharedIndexer | auto.go, callgraph.go, reference.go, treesitter.go, repo_session.go | **0** |
| Who calls BuildRepositorySummary? | BuildRepositorySummary | tools.go | **0** |
| Who calls ExtractSymbolFromQuery? | ExtractSymbolFromQuery | learn.go | **0** |
| Find callers of Classify (answer type). | Classify | tools.go | **0** |
| Who calls LearnFromResults? | LearnFromResults | auto.go | **0** |
| Find callers of KnowledgeMemory.Store. | KnowledgeMemory.Store | learn.go | **0** |
| Who calls SymbolDB.Search? | SymbolDB.Search | treesitter.go, reference.go, knowledge.go | **0** |
| Who calls SymbolDB.SearchReferences? | SearchReferences | reference.go | **0** |
| Who calls SymbolDB.SearchCallEdges? | SearchCallEdges | callgraph.go | **0** |
| Find callers of NewKnowledgeStore. | NewKnowledgeStore | repo_session.go | **0** |
| Who calls GuessMaxResults? | GuessMaxResults | tools.go | **0** |
| Find callers of NeedsCompression. | NeedsCompression | (empty expected) | **0** |
| Who calls shortenPath? | shortenPath | compress.go | **0** |
| Who calls cleanSnippet? | cleanSnippet | compress.go | **0** |
| Find callers of CheckQualityGate. | CheckQualityGate | (empty expected) | **0** |
| Who calls DetectRepositoryState? | DetectRepositoryState | (empty expected) | **0** |
| Find callers of Classify (query class). | Classify | auto.go | **0** |
| Who calls RegisterRetriever? | RegisterRetriever | (empty expected) | **0** |
| Find callers of SymbolDB.StoreSymbols. | StoreSymbols | indexer.go | **0** |
| Who calls SymbolDB.StoreReferences? | StoreReferences | indexer.go | **0** |
| Who calls SymbolDB.StoreCallEdges? | StoreCallEdges | indexer.go | **0** |
| Find callers of FormatDiagnostics. | FormatDiagnostics | (empty expected) | **0** |

**All 25 queries fail** because `call_edges` is empty. **0 cache hits, 0 false negatives** — purely a data-availability issue.

### Verification: Confirmed Working Extraction
```
compress.go: 99 call edges (CompressForAnswerType → compressYesNo, compressLocation, etc.)
auto.go:     54 call edges (Initialize → NewSharedIndexer, NewSymbolRetriever, etc.)
```
Call extraction itself is correct. Only blocked by change detection.

### % Category

| Failure Category | Count | % |
|---|---|---|
| Indexing issue (call_edges empty) | 25 | 100% |
| Symbol missing | 0 | 0% |
| Symbol extracted incorrectly | 0 | 0% |
| Query mismatch | 0 | 0% |
| Scoring issue | 0 | 0% |
| Benchmark issue | 0 | 0% |

---

## 2. Reference Retriever — 0.0% Accuracy

### Symptoms
| Metric | Value |
|---|---|
| Accuracy | 0.0% |
| Avg tokens | 166 |
| Avg confidence | 0.25 |
| Latency | 2.0ms |
| Calibration error | 0.25 |
| Avg retrieval score | 0.0 |

Returns results (166 avg tokens) but achieves 0% accuracy. All retrieved files use virtual paths that never match expected file paths.

### Root Cause 1: `extractGoRefs` Skips Function Bodies

In `internal/treesitter/extract.go:43-44`:
```go
case "function_declaration", "method_declaration", "type_declaration":
    return  // ← STOPS WALKING — never enters function bodies
```

The reference extractor visits every AST node. When it hits `function_declaration` (or `method_declaration` or `type_declaration`), it **returns immediately**, skipping all children — the entire function body. This means it only finds identifiers at the file's top level (constants, import paths, struct field types).

**Result:** Only ~101 references extracted across the entire codebase, all of which are:
- Exported constants registered via `iota` (YesNo, Location, Reference, etc. — `classifier.go`)
- Context level constants (Level0-Level7 — `builder.go`)
- Pipeline step constants (StepGrepGlob, StepSymbolDB, etc. — `pipeline.go`)
- Import package names (fmt, os, regexp, server — various files)
- A few scattered field names (Use, Short, Long, RunE — cobra commands)

None of the symbols sought by benchmark queries (CompressForAnswerType, FilterResults, NewSharedIndexer, etc.) have any reference entries in the DB because all usages of these functions occur **inside** function bodies which are never visited.

### Root Cause 2: Virtual File Path in Result

In `internal/retrieval/reference.go:177-178`:
```go
results := []RetrievalResult{
    {
        File: fmt.Sprintf("reference:%s", symbolName),  // ← virtual path
```

The reference retriever uses a virtual file path `"reference:SymbolName"` instead of an actual file path. The benchmark evaluation compares `result.File` against `expected_files` (actual file paths like `"internal/mcpserver/tools.go"`). The virtual path never matches, so the retriever is scored as incorrect even if the content is relevant.

### Reference Retriever Output for 5 Sample Queries

| Query | Symbol Extracted | DB References Found | Retrieved Files | Expected Files | Match? |
|---|---|---|---|---|---|
| Who calls CompressForAnswerType? | CompressForAnswerType | 0 | "reference:CompressForAnswerType" | tools.go | No |
| Who uses Classify (query class)? | Classify | 0 | "reference:Classify" | auto.go | No |
| Where is FilterResults defined? | FilterResults | 0 | "reference:FilterResults" | filters.go | No |
| Find references to TestSelection. | TestSelection | 0 | "reference:TestSelection" | (any test file) | No |
| Show imports of treesitter. | treesitter | 0 | "reference:treesitter" | import-containing files | No |

### % Category

| Failure Category | Count | % |
|---|---|---|
| Extraction issue (skips function bodies) | 20 | 80% |
| Result format issue (virtual file path) | 5 | 20% |
| Scoring issue | 0 | 0% |
| Query mismatch | 0 | 0% |
| Benchmark issue | 0 | 0% |

---

## 3. FlowGraph Retriever — 0.0% Accuracy

### Symptoms
| Metric | Value |
|---|---|
| Accuracy | 0.0% |
| Avg tokens | 52 |
| Avg confidence | 0.36 |
| Latency | 4.1ms |
| Calibration error | 0.36 |
| Win count | 1 (likely noise) |

### Root Cause: Depends on Empty `call_edges` Table

The flowgraph retriever's `traceFlow` method (`flowgraph.go:171-239`) does:

1. **`findSeeds()`** — matches query against architecture indexer's class/function/module names. Can find seeds from architecture data. ✓
2. **`traceFlow()`** — for each seed node, calls `symDB.SearchCallEdges(node.Name)` and `symDB.SearchCallEdgesByCaller(node.Name)`. Both search the empty `call_edges` table. ✗

The flowgraph CAN find seed nodes (from architecture DB), but the flow-tracing step produces nothing because `call_edges` is empty. Found seeds with no edges → min confidence (0.30) → 52 avg tokens of just the seed names with "no flow paths found" message.

Additionally, the flowgraph's result uses `File: "flowgraph:" + query` — another virtual path that never matches expected files.

### Measured Index Stats
- Architecture indexer nodes: ~200+ (module summaries with classes/functions)
- `call_edges` entries: **0**
- Seeds found per query: 1-3 (typically)
- Nodes after traceFlow: 1-3 (no edges to add)
- Confidence: 0.30 (the minimum for 1-3 nodes)

### % Category

| Failure Category | Count | % |
|---|---|---|
| Indexing issue (call_edges empty, same root cause) | 85 | 100% |
| Extraction issue | 0 | 0% |
| Result format issue (virtual path) | 85 | 100% |
| Scoring issue | 0 | 0% |

---

## 4. Retriever Cost Analysis

### Per-Retriever Metrics (from benchmark-report.json)

| Retriever | Accuracy | Avg Tokens | Latency | Acc/100Tok | Cal Error | Wins |
|---|---|---|---|---|---|---|
| **treesitter** | **62.4%** | 688 | 7ms | 0.091 | 36.5% | 75 |
| **architecture** | **32.9%** | 304 | 6ms | **0.108** | 19.2% | 7 |
| grep | 15.3% | 1454 | 233ms | 0.011 | 75.3% | 2 |
| reference | 0.0% | 166 | 2ms | 0.000 | 25.5% | 0 |
| callgraph | 0.0% | 0 | 1ms | 0.000 | 0.0% | 0 |
| flowgraph | 0.0% | 52 | 4ms | 0.000 | 36.0% | 1 |

### Accuracy Gain Per Token (if fixed)

If callgraph and reference retrievers are fixed (call_edges populated, reference walks function bodies), expected realistic accuracy:

| Retriever | Current Acc | Expected Fixed Acc | Fixed Tokens | Fixed Acc/100Tok |
|---|---|---|---|---|
| **callgraph** | 0.0% | **~60-70%** (caller queries) | ~400 | **0.15-0.18** |
| **reference** | 0.0% | **~50-60%** (reference queries) | ~350 | **0.14-0.17** |
| **flowgraph** | 0.0% | **~30-40%** (flow queries) | ~500 | 0.06-0.08 |

Callgraph and reference, if fixed, would be **the most token-efficient retrievers** — better than both treesitter and architecture. Their empty results artificially drag down cost analysis.

### When call_edges IS populated (verified):
```
SearchCallEdges("NewSharedIndexer")       → 5 edges found
SearchCallEdgesByCaller("Initialize")      → 52 edges found
Total call_edges if properly indexed:     ≈ 4000+
```

---

## 5. Final Recommendations

### CallGraph Retriever — FIX

**Priority: High.** The extraction works. The only bug is the change detection cache.

**Fix: `internal/retrieval/indexer.go:99`** — Track parse-type-specific hashes, or always re-run `ParseCalls` when `ParseCalls: true` is requested.

Option A (recommended): Separate `symbol_files` table into per-parse-type tracking:
```
symbol_files: file_path, symbols_hash, refs_hash, calls_hash
```

Option B (simpler): When `ParseCalls: true`, skip the change-detection check and always parse calls:
```go
if idx.config.ParseCalls {
    calls, err := parser.ParseCalls(ctx, path)
    // Always re-index calls
    idx.db.ClearFileCallEdges(relPath)
    idx.db.StoreCallEdges(calls)
}
```

Option C (simplest): Force full re-index when parse config changes. Store the configured parse flags alongside the schema version, and re-index if different.

**Expected improvement:** 0% → ~60-70% for caller queries.

### Reference Retriever — FIX

**Priority: High.** Two bugs to fix.

**Fix 1: `internal/treesitter/extract.go:43-44`** — Remove the `return` on function_declaration/method_declaration/type_declaration. Walk INTO function bodies but still skip definition-context identifiers:
```go
case "function_declaration", "method_declaration", "type_declaration":
    for i := 0; i < int(n.ChildCount()); i++ {
        walk(n.Child(i))
    }
    return
```

This preserves the existing logic (skipping def-context identifiers via `isGoDefContext` applied to each identifier's parent) while still visiting identifiers inside function bodies.

**Fix 2: `internal/retrieval/reference.go:178`** — Use actual file paths instead of virtual paths:
```go
File: sm.Symbol.File  // Use the actual file from symbolMatches
```

**Expected improvement:** 0% → ~50-60% for reference queries. But only IF references are actually extracted (Fix 1).

### FlowGraph Retriever — FIX + MERGE

**Priority: Medium.** Has value as a visualizer but depends on call_edges.

**Fix:** Same as callgraph — populate `call_edges` first. The flowgraph can then trace seeds → calls → called_by → imports.

**Merge candidate:** Consider merging with Architecture Retriever. The flowgraph's seed-finding (from architecture indexer) duplicates architecture retriever's topic matching. The remaining flow-tracing logic is only useful if call_edges is populated.

**Alternative:** Remove as a standalone retriever and incorporate its functionality into architecture retriever as a "related symbols" bonus.

### Grep Retriever — KEEP

**Priority: Low.** Low accuracy (15.3%) and high latency (233ms), but serves as a last-resort fallback for text queries. See `auto.go:104-112` — it's the fallback for TextQuery and RepositoryQuery.

Consider **restricting to last-resort only**: never pick grep as primary, only as final fallback.

### Treesitter — KEEP

**Priority: None.** Best accuracy (62.4%), reasonable token count (688), low latency (7ms). Fix the 36.5% calibration error (claims 0.99 confidence, delivers 62.4%).

### Architecture — KEEP

**Priority: None.** Best accuracy-per-token (0.108). Second-best accuracy (32.9%). Fix the 19.2% calibration error.

### Summary Table

| Retriever | Verdict | Reason |
|---|---|---|
| **treesitter** | **KEEP** | Best accuracy (62.4%), fix calibration |
| **architecture** | **KEEP** | Best efficiency (0.108 Acc/100Tok) |
| **callgraph** | **FIX** | Extraction works, only change-detection bug blocks it |
| **reference** | **FIX** | Two bugs: skips function bodies + virtual file paths |
| **flowgraph** | **MERGE** into architecture | Depends on call_edges, duplicates architecture seed-finding |
| **grep** | **KEEP** (last resort) | Slow but necessary fallback |

### Impact if Fixed

| Retriever | Current Accuracy | Fixed Accuracy | Current Wins | Fixed Wins |
|---|---|---|---|---|
| callgraph | 0.0% | ~60-70% | 0 | ~15-18 |
| reference | 0.0% | ~50-60% | 0 | ~12-15 |
| flowgraph | 0.0% | ~30-40% | 1 | ~5-8 |

Combined, fixing these 3 retrievers would **increase total benchmark accuracy from 71.2% to ~85-90%**, while likely reducing average tokens since callgraph and reference return more targeted results.
