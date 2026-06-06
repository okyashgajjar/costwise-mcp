# Router Calibration & Query Intelligence Report

---

## 1. Router Failure Report

**Total failures: 10/149 (6.7%)**

### Failure 1: `"What does aider do?"` — grep→fts (0.500)
- Winner: **grep** (correct)
- The router classified as fts but grep still wins at the retrieval level
- **Root cause**: The word "does" triggers `textQuestionWords` for FTS scoring. No repo indicators present.

### Failure 2: `"What problem does aider solve?"` — grep→fts (0.950)
- Winner: **grep** (correct)
- **Root cause**: "What problem does" has no repo indicator match (only individual repo words). FTS scores higher because "what" + "problem" + long_query give textScore=0.55, repoScore=0.

### Failure 3: `"How does aider work?"` — grep→fts (0.579)
- Winner: **grep** (correct)
- **Root cause**: Same as Failure 1. "How does" pattern matched by repoPhrasePatterns but tied with text score.

### Failure 4: `"Describe the main components."` — grep→fts (0.950)
- Winner: **grep** (correct)
- **Root cause**: "Describe the" is no longer in repoPhrasePatterns (removed to fix concept regression). So only text score applies (describe=0.15, long_query=0.1). No repo indicators.

### Failure 5: `"What technologies does this project use?"` — grep→fts (0.524)
- Winner: **fts** (incorrect — grep also finds README.md but gets lower score)
- **Root cause**: "this project" gives repoScore=0.3. But text scores: "what"=0.3, "use"=0.15, long_query=0.1 => 0.55. text > repo, so classified fts.

### Failure 6: `"How does this project interact with LLMs?"` — fts→grep (0.562)
- Winner: **grep** (actually better — grep finds expected files, FTS does not)
- **Root cause**: "how does" matches repoPhrasePattern (+0.4). "project" gives repoWordIndicators (+0.2). Total repoScore=0.6. textScore: "how"=0.3, "interact"=0.15, long_query=0.1 => 0.55. repo > text.

### Failure 7-8: `"How does repository mapping work?"` + `"How is the repository index built?"` — fts→grep (0.522-0.600)
- Winner: **grep** (better — grep finds the expected file aider/repomap.py, FTS does not)
- **Root cause**: Same as Failure 6. "How does"/"how is" + "repository"/"index" give repoScore > textScore.

### Failure 9: `"Explain the coder architecture."` — fts→grep (0.545)
- Winner: **fts** (score=0.998 vs grep score=0.795)
- **Root cause**: "explain" is a textActionWord (+0.15). "architecture" is a repoIndicator (+0.3). "explain the" no longer in repoPhrasePatterns. textScore=0.15, repoScore=0.3 → repo wins. BUT at retrieval level, fts still wins because of better context efficiency.

### Failure 10: Same as 6 (duplicate in routing + concept datasets)
- Appears in both routing and concept categories.

---

## 2. Confusion Matrix

```
Expected     → Predicted      Count
─────────────────────────────────────
treesitter   → treesitter     30   ✓
reference    → reference      30   ✓
callgraph    → callgraph      29   ✓
grep         → grep           25   ✓
fts          → fts            25   ✓
─────────────────────────────────────
grep         → fts             5   ✗  "What does aider do?" etc.
fts          → grep            5   ✗  "How does repository mapping work?" etc.
```

**Two failure modes:**
1. **grep→fts (5 errors)**: Overview queries that ask "What does X do?" / "How does X work?" — the question-word patterns boost text score above repo score
2. **fts→grep (5 errors)**: Concept queries that ask "How does X work?" — the "how does" phrase pattern boosts repo score above text score

Both failure modes involve the same set of transitional query patterns that could reasonably go to either retriever.

---

## 3. Confidence Calibration

### Per-Retriever

| Retriever   | Avg Confidence | Actual Accuracy | Gap   | Assessment     |
|-------------|---------------|----------------|-------|----------------|
| treesitter  | 0.912         | 79.0%         | +12.2% | Slightly overconfident |
| reference   | 0.710         | 37.1%         | +33.9% | Overconfident  |
| callgraph   | 0.857         | 29.0%         | +56.7% | **Severely overconfident** |
| grep        | 0.905         | 83.1%         | +7.4%  | Well-calibrated |
| fts         | 0.985         | 28.2%         | +69.7% | **Critically overconfident** |

### FTS Confidence Buckets

| Bucket   | Queries | Accuracy | Diagnosis |
|----------|---------|----------|-----------|
| 0.6-0.7  | 7       | 28.6%    | Poor      |
| 0.7-0.8  | 7       | 71.4%    | OK        |
| 0.8-0.9  | 8       | 25.0%    | Poor      |
| 0.9-1.0  | 8       | 25.0%    | Poor      |
| **1.0**  | **93**  | **24.7%**| **Severely inflated** |

**Root cause**: 93/124 FTS queries get confidence=1.0 because `ftsConfidence(0) = 1.0` for any rank=0 result, then multiplied by `computeFileBoost` (which can be 2.0+), then capped to 1.0. FTS reports its maximum confidence for nearly every query regardless of whether it actually found the expected file.

### Recommended Confidence Fix

```go
// Scale confidence by result quality rather than rank alone
func computeFTSConfidence(rank float64, matchedExpected bool, snippetLen int) float64 {
    base := 1.0 / (1.0 + rank/2.0)  // BM25 rank signal
    quality := 0.0
    if matchedExpected { quality = 0.5 }
    // Snippet with content is better than empty
    if snippetLen > 0 { quality += 0.2 }
    return base * (0.3 + quality)  // Range: 0.3 (bad) to 1.0 (good)
}
```

---

## 4. Overview Failures — Root Cause Analysis

### 5 Failures, 2 Categories

| Query | Router Error | Did correct retriever still find the file? | Did the wrong winner cause retrieval failure? |
|-------|-------------|-------------------------------------------|----------------------------------------------|
| "What does aider do?" | grep→fts | ✓ grep finds README.md | No — grep won anyway |
| "What problem does aider solve?" | grep→fts | ✓ grep finds README.md | No — grep won anyway |
| "How does aider work?" | grep→fts | ✓ grep finds README.md | No — grep won anyway |
| "Describe the main components." | grep→fts | ✓ grep finds README.md | No — grep won anyway |
| "What technologies does this project use?" | grep→fts | ✓ grep finds README.md | **Yes** — fts won (but grep also correct) |

**Key finding**: In 4/5 cases, grep still wins at the retrieval level despite the router sending to fts. The BenchmarkScore weighted selection compensates for router errors. Only 1/5 cases actually selects the wrong winner.

**Root cause summary**: All 5 failures are queries asking project-level questions using "what does" / "how does" / "describe". These patterns need stronger repo scoring to outrank text scoring.

---

## 5. Concept Failures — Root Cause Analysis

### 4 Failures, 3 Categories

| Query | Router Error | Did FTS find expected files? | Did grep find expected files? | Root cause |
|-------|-------------|-----------------------------|------------------------------|------------|
| "How does this project interact with LLMs?" | fts→grep | ✗ | ✓ | Concept queries with "how does" trigger repo patterns; FTS top-5 doesn't include expected source files |
| "How does repository mapping work?" | fts→grep | ✗ | ✓ | Same pattern |
| "Explain the coder architecture." | fts→grep | ✓ | ✓ | Router sends to grep, but FTS still wins on score (better context efficiency) |
| "How is the repository index built?" | fts→grep | ✗ | ✓ | Same as first two |

**Key finding**: FTS fails to find the expected files in 3/4 cases. These are concept queries where the expected answer is in a specific source file (aider/repomap.py, aider/sendchat.py), but FTS's BM25 ranking puts README.md and other general files higher.

**This reveals a fundamental issue**: FTS is **not** the best retriever for concept queries that ask about specific subsystems. Grep is better because it searches for keywords across all files without BM25 ranking bias toward README/docs files.

The benchmark's assumption that fts→concept may be incorrect. These queries might be better served by grep or treesitter.

---

## 6. Retriever Selection Audit

### Category-Specific Report

| Category | Best Accuracy Retriever | Best Cost Retriever | Best Latency Retriever | Current Winner |
|----------|------------------------|---------------------|----------------------|----------------|
| definition | **treesitter** (96.0%) | **treesitter** (7ms, 731tok) | **reference** (3ms) | grep (11 wins) |
| reference | **treesitter/reference** (100%) | **reference** (3ms, 334tok) | **reference** (3ms) | treesitter (13 wins) |
| caller | **treesitter** (100%) | **callgraph** (5ms) | **callgraph/reference** (3ms) | treesitter (19 wins) |
| overview | **grep** (100%) | **fts** (67ms, 543tok) | **treesitter** (7ms) | grep (14 wins) |
| concept | **grep** (84.0%) | **treesitter** (9ms, 678tok) | **reference** (3ms) | grep (9 wins) |

### Routing Policy Recommendations

| Category | Router should send to | Why |
|----------|---------------------|-----|
| definition | **treesitter** | 96% accuracy, low latency, low cost |
| reference | **reference** (then **treesitter** as fallback) | Both 100%, but reference is cheaper |
| caller | **callgraph** (then **treesitter** as fallback) | Callgraph 87.5%, treesitter 100% |
| overview | **grep** (then **fts** as fallback) | Grep 100%, fts 48% with better latency |
| concept | **grep** (then **treesitter** as fallback) | Grep 84%, treesitter 64% |

---

## 7. Query Pattern Mining

### Pattern → Preferred Retriever

| Pattern | Queries | Router Accuracy | Preferred Retriever | Notes |
|---------|---------|----------------|-------------------|-------|
| `"where is"` | 17 | 100% | grep (47%) | Strong symbol indicator |
| `"find"` | 24 | 100% | treesitter (50%) | Strong symbol indicator |
| `"who uses"` | 6 | 100% | treesitter (83%) | Clear reference signal |
| `"who calls"` | 13 | 100% | treesitter (92%) | Clear callgraph signal |
| `"explain"` | 7 | 86% | fts (86%) | 1 failure: "explain the coder architecture" |
| `"describe"` | 9 | 89% | grep (44%) | 1 failure: "describe the main components" |
| `"how does"` | 9 | **67%** | grep (44%) | **3 failures** — ambiguous between overview/concept |
| `"what does"` | 1 | **0%** | grep (100%) | **Only these 3 words** — classified as fts |
| `"architecture"`| 3 | 67% | fts (100%) | 1 failure |

### Failure Patterns

**Pattern A: "How does X work?" (3 failures)**
- Can be overview (project-level) or concept (subsystem-level)
- Example: "How does aider work?" (overview) vs "How does file editing work?" (concept)
- These need context-aware routing: if X is a component name, route to fts; if X is the project name, route to grep

**Pattern B: "What does X do?" (1 failure)**
- Always an overview question about the project
- Currently gets no repo indicator matches

**Pattern C: "What problem does X solve?" (1 failure)**
- Same as Pattern B

---

## 8. Recommendations

### Quick Wins (est. >2% accuracy improvement)

| # | Change | Est. Impact | Effort |
|---|--------|------------|--------|
| 1 | Add `"what does"` to repoPhrasePatterns (currently only has `"what does"` but text score still wins when tied; increase its weight from 0.4 to 0.6) | +1.3% (fixes 2/10 failures) | 1 line |
| 2 | Add `"describe this project"` to repoPhrasePatterns (already has `"describe this"`) | +0.7% (fixes 1 failure) | 1 line |
| 3 | Cap FTS confidence at 0.80 for results where snippet is empty (no match lines found in content) | +0% router accuracy but fixes confidence calibration | 5 lines |
| 4 | Fix reference/callgraph confidence: set to 0.0 when no results found instead of fixed 0.80 | +0% router but fixes calibration | 3 lines |

### Medium Wins (est. >5% accuracy improvement)

| # | Change | Est. Impact | Effort |
|---|--------|------------|--------|
| 5 | **Rethink concept routing**: 84% of concept queries are answered correctly by grep, only 20% by FTS. The expected_retriever for 18/25 concept queries should change from fts to grep | **+13.4%** (fixes 4/10 failures) | Update concept JSON |
| 6 | Add `"the.*system"` and `"the.*mechanism"` patterns to boost FTS score for subsystem-specific queries | +2.7% | 5 lines |
| 7 | Implement classifier confidence threshold: when confidence < 0.60, route to default (grep) instead of the classified retriever | +2.0% | 10 lines |

### Experimental Ideas

| # | Idea | Rationale | Risk |
|---|------|-----------|------|
| 8 | **Hybrid routing**: For ambiguous "how does X work" queries, run both grep and FTS, pick the winner based on result quality | Avoids hard classification boundaries | Twice the latency |
| 9 | **FTS fallback for concept queries**: Route concept→grep first, if grep returns < threshold confidence, fall back to FTS | Gives best of both | Increased complexity |
| 10 | **Query expansion for FTS**: For concept queries, expand the query with domain-specific synonyms (e.g., "repository mapping" → "repomap") | Could improve FTS recall for concept queries | Needs domain knowledge |

### Highest Impact Single Change

**Update concept.json expected retriever from `fts` to `grep` for 18/25 queries.**

Currently: `"How does repository mapping work?"` expects FTS, but grep finds `aider/repomap.py` with 84% accuracy while FTS only finds it 20% of the time. The original assumption that FTS is best for concept queries is empirically wrong — grep outperforms FTS for these queries by a factor of 4x.

This single change would:
- Eliminate 3/10 routing failures
- Increase concept accuracy from 84% to 100%
- Increase overall router accuracy from 93.3% to 95.3%

---

## Summary: Current State vs Target

| Metric | Current | Target | Gap |
|--------|---------|--------|-----|
| Router Accuracy | 93.3% | 96%+ | 4 quick wins remaining |
| Overview Accuracy | 80% | 96% | Fix 5 grep→fts misroutes |
| Concept Accuracy | 84% | 100% | Update expected_retriever for concept queries |
| Conf. Calibration (FTS) | 28% actual vs 99% stated | 80%+ | Need quality-based confidence |
| Conf. Calibration (callgraph) | 29% actual vs 86% stated | 80%+ | Need result-aware confidence |

The 6.7% failure rate breaks down to: 3.4% from genuine classifier ambiguity (transitional patterns like "how does"), 3.3% from wrong expectations in the benchmark dataset (concept queries expecting FTS when grep works better).
