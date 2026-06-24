# Confidence & Accuracy Scoring

> The complete map of how every retrieval tool scores and ranks results — confidence ceilings, formulas, thresholds, and decision logic.

## At a glance

| Tool | Max Confidence | Key Influencers |
|------|---------------|-----------------|
| **FTS** | **0.40** | BM25 rank, match count, snippet presence, file-type boosts |
| **Tree-sitter Symbol DB** | **1.0** (exact name match) | Name match quality, camelCase overlap |
| **Grep** | **0.92** | Unique keyword ratio (80%), hit density (20%) |
| **Reference** | **0.70** | Base 0.40 + ref count (diminishing after 10) |
| **Call Graph** | **0.60** | Base 0.35 + call count (diminishing after 10) |
| **Architecture** | **0.95** | Step function of composite score (5 tiers) |
| **Flow Graph** | **0.70** | Node count tiers (3 levels) |
| **Naive** | **1.0** | Static |
| **Query Classifier** | **0.95** (capped) | Proportion of winning score to total |
| **Answer Type Classifier** | **0.90** (pattern match) | Fixed per pattern; mode override = 1.0 |
| **Quality Gate** | N/A | Thresholds: 0.15 (minimum), 0.30 (yes_no/location) |

---

## 1. FTS (Full-Text Search) — `internal/retrieval/fts.go`

### Core Score Formula (line 198)

```
rankingScore = qualityConf × boost
// clamped to 1.0
```

Where:

- **`qualityConf`** = `ftsConfidence(rank, fileMatches, len(snippet))`
- **`boost`** = `computeFileBoost(filePath, repoRoot, query)` (range: 1.0 to ~7.8)

### `ftsConfidence` function (lines 411–431)

```
base = 1.0 / (1.0 + rank / 2.0)    // rank >= 0
base = 1.0 / (1.0 - rank / 2.0)    // rank < 0  (FTS5 can return negative ranks)

quality = 0.12                      // base quality floor
if snippetLen > 0  → quality += 0.15
if matchCount > 3  → quality += 0.05
if matchCount > 10 → quality += 0.05

conf = base × quality
// clamped to max 0.40 — HARD CAP: confidence never exceeds 0.40
```

**`FTS5 rank`**: SQLite FTS5's built-in BM25-derived rank (lower = better match).

### `computeFileBoost` function (lines 356–409)

| Factor | Boost | Condition |
|--------|-------|-----------|
| Filename match | × 1.5 | Query word appears in filename |
| README | × 2.0 | Path starts with "readme" or is README.md |
| Docs directory | × 1.5 | Path contains "doc" or "docs" |
| Has functions | × 1.3 | Content contains "func " or "def " |
| Has classes | × 1.3 | Content contains "class " or "interface " |
| Has comments | × 1.2 | Content contains comment markers |

Boosts are multiplicative; max theoretical boost ≈ 1.5 × 2.0 × 1.5 × 1.3 × 1.3 × 1.2 ≈ **9.1** (but rankingScore capped to 1.0).

### Ranking (lines 217–225)

Results sorted descending by `Score`. Top 5 kept.

### Overall FTS Confidence (lines 227–230)

```
confidence = ftsConfidence(0, results[0].MatchHits, len(results[0].Snippet))
```

Uses `rank = 0` (best-case assumption). Max 0.40.

### FTS5 Query Construction (lines 246–291)

- Stopword filtered
- Long queries: `"phrase" OR word1 OR word2 OR ...` (phrase search preferred)
- Short queries: single term

---

## 2. Tree-Sitter Symbol Search — `internal/retrieval/treesitter.go` + `internal/treesitter/db.go`

### Symbol DB Search — `db.go` (lines 400–444)

**Database query ordering** (lines 411–418):

```sql
ORDER BY
  CASE
    WHEN name LIKE ? THEN 0     -- exact match
    WHEN name LIKE ? THEN 1     -- prefix match
    ELSE 2                       -- partial/substring match
  END,
  start_line ASC
```

### `computeSymbolScore` function — `db.go` (lines 558–593)

| Condition | Score | Logic |
|-----------|-------|-------|
| Exact name match (`name == query`) | **1.0** | |
| Substring match + camelCase overlap | **0.8 + 0.2 × (matched / qParts)** | If `name` contains `query` |
| CamelCase overlap only | **0.7 × (matched / qParts)** | If no direct substring |
| Substring match (no camelCase) | **0.6** | If `name` contains `query` |
| Partial match (fallback) | **0.3** | Default for LIKE match |

**CamelCase split** (lines 595–611): Splits `PascalCase` and `camelCase` into component words.

### `adjustScore` — `treesitter.go` (lines 226–253)

After DB scoring, results are further scored per query:

| Condition | Score |
|-----------|-------|
| Exact match | **1.0** |
| Substring (either direction) | **0.9 + base × 0.1** |
| Partial word match ratio > 50% | **0.7 + ratio × 0.3** |
| Fallback | **base** (from DB) |

### Overall Tree-Sitter Confidence — `treesitter.go` (lines 132–138)

```
confidence = results[0].Score
if confidence < 0.3 {
    confidence = 0.3
}
```

Floor of **0.3**. Top 10 matches kept (line 106–110).

---

## 3. Grep / Regex Retriever — `internal/retrieval/grep.go`

### Raw Score Formula (lines 94–108, 200)

```
score = matchCount × scoreWeight × (1.0 + keywordDensity × 2.0 + uniqueRatio × 2.0 + recency)
```

Where:

- **`matchCount`**: Number of unique lines with keyword matches
- **`scoreWeight`**:
  - 3.0 for README
  - 2.0 for docs dir
  - 1.5 for filename match
  - 1.0 for regular files
- **`keywordDensity`**: `matchCount / totalLines`
- **`uniqueRatio`**: `matchedKeywords / totalKeywords`
- **`recency`**: Based on position of first match:
  - First match in first 20% of file → 1.0
  - First match in first 50% of file → 0.5
  - Else → 0.2

### Normalized Score (line 313)

```
Score = rawScore / maxScore   // normalized to [0, 1]
```

Where `maxScore` is the highest raw score among top 5 results (minimum 1.0).

### Overall Grep Confidence (lines 320–341)

```
matchedRatio = matchedTerms / totalKeywords     // unique keywords found
hitDensity   = min(totalMatches, 500) / (min(totalMatches, 500) + results)

confidence = matchedRatio × 0.80 + hitDensity × 0.20
// clamped to max 0.92
```

Max confidence: **0.92**.

---

## 4. Reference Retriever — `internal/retrieval/reference.go`

### Confidence Formula (lines 164–170)

```
confidence = 0.40 + 0.30 × min(totalRefs, 10) / 10.0
// clamped to max 0.70
```

- Base of **0.40** if any definitions or references found
- Scales up to **0.70** at 10+ references (diminishing returns after 10)
- Result `Score` = confidence (single result per query)

---

## 5. Call Graph Retriever — `internal/retrieval/callgraph.go`

### Confidence Formula (lines 134–140)

```
confidence = 0.35 + 0.25 × min(callSiteCount, 10) / 10.0
// clamped to max 0.60
```

- Base of **0.35** if any call sites or callers found
- Scales up to **0.60** at 10+ call sites
- Lower ceiling than Reference (0.60 vs 0.70)
- Result `Score` = confidence

---

## 6. Architecture Retriever — `internal/retrieval/architecture.go`

### Composite Score (line 95)

```
score = 0.30 × topicScore + 0.20 × symbolScore + 0.30 × filenameScore + 0.10 × importScore + 0.10 × descScore
// clamped to max 1.0
```

| Component | Weight | Method |
|-----------|--------|--------|
| `topicScore` | **0.30** | Ratio of query topics that match module topics (topic map) |
| `symbolScore` | **0.20** | Ratio of query words matching class/function names |
| `filenameScore` | **0.30** | Direct hits (×0.5) + synonym hits (×0.7) in filename/dir path |
| `importScore` | **0.10** | Binary (0 or 1): any import matches query topics/words |
| `descScore` | **0.10** | Ratio of query words found in module description |

Min score threshold: **0.05** (lines 100–102). Top 10 kept (line 130–132).

### Overall Architecture Confidence (lines 540–558)

```
if topScore < 0.10  → 0.20
if topScore >= 0.80 → 0.95
if topScore >= 0.50 → 0.80
if topScore >= 0.30 → 0.65
else                → 0.45
```

Step-function mapping with **5 tiers**.

---

## 7. Flow Graph Retriever — `internal/retrieval/flowgraph.go`

### Confidence Formula (lines 297–309)

```
if nodeCount >= 8  → 0.70
if nodeCount >= 4  → 0.50
else               → 0.30
```

Based purely on number of flow graph nodes discovered. Result `Score` = confidence.

---

## 8. Naive Retriever — `internal/retrieval/naive.go`

Scores are **static**:

- README files: **1.0** (line 49)
- Docs directory files: **0.8** (line 73)
- Root-level files: **0.5** (line 109)
- Overall confidence: **1.0** (line 123) — always fully confident since all results are pre-loaded

---

## 9. Auto Retriever (Fusion / Routing) — `internal/retrieval/auto.go`

### Retrieval Routing (lines 95–112)

Based on query classifier:

| Query Class | Primary | Fallbacks |
|-------------|---------|-----------|
| `SymbolQuery` | treesitter | fts, grep |
| `TextQuery` | fts | grep, treesitter |
| `RepositoryQuery` | grep | fts, treesitter |
| `ReferenceQuery` | reference | treesitter, grep |
| `CallQuery` | callgraph | treesitter, reference |
| `ArchitectureQuery` | architecture | grep, fts |
| `FlowQuery` | flowgraph | callgraph, treesitter |

### Fallback Trigger (lines 124–136)

```
primaryConf < 0.3 || (len(results) < 2 && totalMatchHits < 2)
```

### Deduplication (lines 198–211)

When same file found by multiple retrievers, keeps the one with **higher Score**.

### Result Fusion (lines 153–161)

Results from multiple retrievers are concatenated, deduplicated, sorted by Score descending, and capped to **10**.

### Auto Retriever Confidence (line 173)

```
metrics.Confidence = cl.Confidence  // uses query classifier confidence, not retrieval quality
```

### `resultConfidence` helper (lines 221–226)

```
return results[0].Score  // top result's score, or 0 if empty
```

---

## 10. Query Classifier — `internal/classifier/classifier.go`

### Classification by Score Competition (lines 170–256)

Each class gets a raw score via dedicated functions. Total is summed:

```
total = symScore + txtScore + repScore + archScore + flowScore
```

**Class scores:**

| Class | Score Function | Scoring Basis |
|-------|---------------|---------------|
| **CallQuery** | `scoreCall()` | Pattern match + action word + uppercase symbol (max ~1.2) |
| **ReferenceQuery** | `scoreReference()` | Pattern match + action word + uppercase symbol (max ~1.2) |
| **SymbolQuery** | `scoreSymbol()` | PascalCase (0.3–0.7) + snake_case (0.3+) + ALL_CAPS + camelCase + keyword indicators |
| **TextQuery** | `scoreText()` | Question words (×0.3) + action words (×0.15) + long query (0.1) |
| **RepositoryQuery** | `scoreRepo()` | Repository indicators (×0.3) + phrase patterns (×0.4) + word markers (×0.2) |
| **ArchitectureQuery** | `scoreArchitecture()` | Architecture keywords + explain/describe + optional symbol |
| **FlowQuery** | `scoreFlow()` | Flow/trace/pipeline/lifecycle words (0.4–0.6) |

**Winner selection**: highest-scoring class wins, with:

```
confidence = winningScore / total  // proportional confidence
// clamped to max 0.95
```

**CallQuery and ReferenceQuery have priority bypass** (lines 140–168): if their score > 0, they win immediately without competing against other classes.

---

## 11. Answer Type Classifier — `internal/answertype/classifier.go`

### Classification by Pattern Matching (lines 243–431)

Fixed confidence values assigned per matched pattern:

| Answer Type | Confidence | Condition |
|-------------|-----------|-----------|
| YesNo | **0.85** | yes_no pattern + ≤6 words |
| Location | **0.90** | location prefix pattern |
| Caller | **0.90** | caller prefix pattern |
| Reference | **0.90** | reference prefix pattern |
| Overview | **0.90** | overview prefix pattern |
| Improvement | **0.85** | improvement phrase |
| RepositoryAnalysis | **0.85** | analysis phrase |
| ArchitectureReview | **0.85** | architecture phrase |
| FeatureSuggestion | **0.80** | feature phrase |
| Plan | **0.85** | plan prefix pattern |
| Explanation | **0.70** | explanation prefix pattern |
| Plan (from agent patterns) | **0.70** | implementation/agent style |
| Improvement (keyword) | **0.65** | keyword "improve" etc. |
| Explanation (question) | **0.60** | ends with "?" |
| Location (short query) | **0.65** | ≤4 words with uppercase |
| Explanation (default) | **0.50** | fallback |

**Mode override** (lines 248–263): If mode is "plan" or "agent", returns **1.0** confidence.

### Max Tokens per Answer Type

| Type | Max Tokens | Evidence Required |
|------|-----------|-------------------|
| yes_no | 10 | topScore ≥ 0.3 |
| location | 25 | topScore ≥ 0.3 |
| reference | 50 | ≥ 1 result |
| caller | 50 | ≥ 1 result |
| overview | 150 | ≥ 1 result |
| explanation | 400 | ≥ 1 result |
| improvement | 200 | ≥ 3 results |
| feature_suggestion | 200 | ≥ 3 results |
| architecture_review | 250 | ≥ 3 results |
| repository_analysis | 300 | ≥ 3 results |
| plan | 500 | ≥ 1 result |
| agent | dynamic (0 = no limit) | N/A |

---

## 12. Quality Gate — `internal/retrieval/pipeline.go`

### `CheckQualityGate` function (lines 86–131)

| Condition | Gate Result | Threshold |
|-----------|-------------|-----------|
| No results | **FAIL** — "No repository context found" | `len(results) == 0` |
| Top score < 0.15 | **FAIL** — "No repository context found" | `topScore < 0.15` |
| YesNo/Location + topScore ≥ 0.3 | **PASS** | `topScore >= 0.3` |
| Improvement/RepoAnalysis/ArchReview/FeatureSugg + fewer than 3 results | **FAIL** — "Insufficient evidence" | `len(results) < 3` |
| Default gate | **PASS** if `topScore >= 0.15` | `topScore >= 0.15` |

**Key thresholds:**

- **0.15**: absolute minimum score floor for any result
- **0.30**: higher bar for yes_no/location types

---

## 13. Knowledge Memory — `internal/kmemory/kmemory.go`

### Entry Confidence Values

| Entry Type | Default Confidence |
|------------|-------------------|
| `NewSymbolEntry` | **1.0** (line 307) |
| `NewReferenceEntry` | **0.8** (line 317) |
| `NewCallerEntry` | **0.85** (line 328) |
| `NewUserNote` | **1.0** (line 337) |
| File ownership (in learn.go) | **0.8** (line 81) |
| Grep knowledge (in learn.go) | **0.7** (line 98) |
| Glob knowledge (in learn.go) | **0.7** (line 113) |

### Search Scoring — `matchScore` (lines 154–168)

```
key match   → +2 points
value match → +1 point
file match  → +1 point
```

Results ranked by descending match score, ties broken by most recently used.

---

## 14. Improvement Ranking — `internal/retrieval/improvement.go`

### `scoreImprovement` (lines 68–71)

```
impactScore  = float64(ImpactLevel)         // High=2, Medium=1, Low=0
effortScore  = 2.0 - float64(EffortLevel)   // Low=2, Medium=1, High=0

score = (impactScore × 2.0 + effortScore × 1.5) × Confidence
```

Max 5 items kept. Sorted descending by score.

---

## 15. Filter Results — `internal/retrieval/filters.go`

`FilterResults(results, minScore, maxResults)` (lines 10–82):

- Removes results below `minScore` (parameterizable)
- Removes results from excluded directories (fixtures, testdata, examples, vendor, dist, etc.)
- Removes results from excluded filename patterns (chat-history, *.gold.*, *.snapshot.*, *.log)
- Sorts by Score descending
- Caps to `maxResults`

---

## 16. Response Compression Trigger — `internal/retrieval/compress_response.go`

```
needsCompression = outputTokens > MaxTokens × 2
```

Line 12.

---

## 17. Context Budget by Answer Type — `internal/retrieval/pipeline.go` (lines 164–193)

| Answer Type | Token Budget |
|-------------|-------------|
| YesNo | 100 |
| Location | 200 |
| Caller | 500 |
| Reference | 500 |
| Overview | 2000 |
| Explanation | 3000 |
| Improvement | 3000 |
| FeatureSuggestion | 2500 |
| ArchitectureReview | 3000 |
| RepositoryAnalysis | 4000 |
| Plan | 4000 |
| Agent | 4000 |
