# CostWise Architecture

## Overview

```
┌──────────────────────────────────────────────────────────────────┐
│  CLI (cmd/costwise)                                              │
│  serve | install | uninstall | doctor | skill | chat | plan     │
└─────────────────────┬────────────────────────────────────────────┘
                      │ stdio
┌─────────────────────▼────────────────────────────────────────────┐
│  MCP Server (internal/mcpserver)                                 │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │  Server + SessionCache (per-repo, process-global)         │   │
│  │  RegisterTools() → 10 tools wired to handlers             │   │
│  │  server.WithInstructions(skill.Instructions())            │   │
│  └────────────┬──────────────────────────────────────────────┘   │
└────────────────┼──────────────────────────────────────────────────┘
                 │
    ┌────────────┼───────────────────────────────┐
    │            │                               │
┌───▼────────┐ ┌▼──────────────┐ ┌──────────────▼───┐
│ Retrieval  │ │ Context-      │ │ Session Awareness │
│ Tools      │ │ Control Tools │ │ Tools             │
│            │ │               │ │                   │
│ search_code│ │ stash_context │ │ session_brief     │
│ find_symbol│ │ recall        │ │ (via ledger)      │
│ read_symbol│ │ remember      │ │                   │
│ find_refs  │ │               │ │ ledger.Append()   │
│ find_call  │ │               │ │ on every event    │
│ repo_summary│ │               │ │                   │
│ index_repo │ │               │ │                   │
└───┬────────┘ └───────┬───────┘ └───────────────────┘
    │                  │
    │           ┌──────▼──────┐
    │           │ stash.Store │
    │           │ (file-backed│
    │           │  blobs)     │
    │           └─────────────┘
    │
┌───▼──────────────────────────────────────────────────────────────┐
│  RepoSession (internal/session/repo_session.go)                  │
│  ┌──────────┬───────────┬──────────┬────────┬────────┬────────┐ │
│  │ SymbolDB │ Shared-   │ Know-    │ Cache  │ Stash  │ Know-  │ │
│  │ (SQLite) │ Indexer   │ ledge-   │ (LRU)  │ Store  │ ledge- │ │
│  │          │           │ Store    │        │        │ Memory │ │
│  │ tree-    │ parses    │ queries  │ caches │ blobs  │ facts  │ │
│  │ sitter   │ repo →    │ DB for   │ recent │ parked │ (User  │ │
│  │ indexes  │ symbols,  │ modules, │ lookups│ out of │ Notes) │ │
│  │ symbols, │ refs,     │ files,   │        │ context│        │ │
│  │ refs,    │ calls     │ summaries│        │        │        │ │
│  │ calls    │           │          │        │        │        │ │
│  └──────────┴───────────┴──────────┴────────┴────────┴────────┘ │
│                                                                  │
│  Also owns: RepoMemory (long-term), DiscMemory (discovery)       │
│             Watchdog (fsnotify auto-reindex)                     │
└──────────────────────────────────────────────────────────────────┘
```

---

## 1. Retrieval Flow (search_code, find_symbol, etc.)

### How a query reaches code

```
User asks → MCP tool handler → GetOrCreateRepoSession → retriever → results → compressor → answer
```

**search_code** (the most complex tool):

```
searchCodeHandler()
  │
  ├─ GetOrCreateRepoSession(repoPath)
  │   └─ newRepoSession() → initializes:
  │       ├─ treesitter.NewSymbolDB()    — opens SQLite index
  │       ├─ NewSharedIndexer()          — parses repo if needed
  │       ├─ NewKnowledgeStore()         — read-only DB queries
  │       ├─ cache.NewCache()            — LRU of recent lookups
  │       ├─ stash.New()                 — blob store
  │       └─ kmemory.NewKnowledgeMemory()— session facts
  │
  ├─ rs.ResolveQuery(query)             — replaces "it"/"this" with last symbol
  ├─ answertype.Classify(query, mode)   — patterns → yes_no / location / caller / reference / overview / explanation / plan / improvement / etc.
  │
  ├─ NewAutoRetriever().Initialize()
  │   └─ classifier.Classify(query)     — SymbolQuery / TextQuery / RepoQuery / ReferenceQuery / CallQuery / ArchitectureQuery / FlowQuery
  │   └─ routes to primary retriever:
  │       ├─ SymbolQuery       → treesitter (SymbolRetriever)
  │       ├─ TextQuery         → fts (FTSRetriever)
  │       ├─ RepositoryQuery   → grep (GrepRetriever)
  │       ├─ ReferenceQuery    → reference (ReferenceRetriever)
  │       ├─ CallQuery         → callgraph (CallGraphRetriever)
  │       ├─ ArchitectureQuery → architecture (ArchitectureRetriever)
  │       └─ FlowQuery         → flowgraph (FlowGraphRetriever)
  │   └─ falls back to other retrievers if primary confidence < 0.3
  │
  ├─ auto.Retrieve(query) → []RetrievalResult{File, Snippet, Score, LineFrom, LineTo}
  │
  ├─ FilterResults(results, 0.15, maxResults)  — score threshold + cap
  ├─ rs.StoreResult(query, results)             — saves to session memory for pronoun resolution
  └─ CompressForAnswerType(results, type, budget)
      └─ compressYesNo / compressLocation / compressCaller / compressDefault
          → CompressedContext{Context string, Tokens int}
```

**Other retrieval tools** follow the same pattern but route directly to one retriever and skip the classifier:

| Tool | Handler | Retriever | Compression |
|------|---------|-----------|-------------|
| `find_symbol` | `findSymbolHandler` | `NewSymbolRetriever()` | `compressLocation` |
| `read_symbol` | `readSymbolHandler` | `NewSymbolRetriever()` + reads line range from file | raw body |
| `find_references` | `findReferencesHandler` | `NewReferenceRetriever()` | `compressReference` |
| `find_callers` | `findCallersHandler` | `NewCallGraphRetriever()` | `compressCaller` |
| `get_repository_summary` | `repoSummaryHandler` | `BuildRepositorySummaryCompact(ks, budget, module)` | budget-capped summary |
| `index_repository` | `indexRepoHandler` | `rs.Indexer.Index()` | status report |

---

## 2. Context-Control Flow (stash / recall / remember)

### stash_context — park large blobs out of context

```
stashContextHandler()
  ├─ GetOrCreateRepoSession(repoPath)
  ├─ rs.Stash.Store(content, label)
  │   └─ internal/stash/stash.go
  │       ├─ sha256 hash → 12-char hex handle
  │       ├─ write content to .mycli-fts/stash/<handle>.txt
  │       ├─ update manifest.json
  │       └─ prune (keep last 256 by default)
  └─ ledger.Append("stash", "create", handle, tokens, summary)
```

Response: `Stashed "label" → a1b2c3 (~4500 tokens kept out of context)`

### recall — query-scoped read-back

```
recallHandler()
  ├─ GetOrCreateRepoSession(repoPath)
  │
  ├─ if source == "stash-handle" (not "facts" and not ""):
  │   └─ rs.Stash.Query(handle, query, budget)
  │       └─ reads full file from .mycli-fts/stash/<handle>.txt
  │       └─ line-by-line strings.Contains(query) match
  │       └─ returns matching lines up to budget
  │
  ├─ else:
  │   ├─ rs.RecallFacts(query)
  │   │   └─ kmemory.Search(UserNote, query) — token overlap ranking
  │   │   └─ returns "key: value" lines
  │   └─ if source == "":
  │       └─ rs.Stash.List() — show available stashes matching query
  │
  └─ ledger.Append("recall", "read", query, source)
```

### remember — persist durable facts

```
rememberHandler()
  ├─ GetOrCreateRepoSession(repoPath)
  ├─ rs.RememberFact(key, fact)
  │   └─ kmemory.Store(UserNote, key, NewUserNote(key, fact))
  │   └─ kmemory.SaveToFile(factsPath)
  │       └─ .mycli-fts/session_facts.json — persists across restarts
  └─ ledger.Append("fact", "add", summary)
```

---

## 3. Session Awareness (ledger + session_brief)

### Ledger (internal/ledger)

Appends JSONL events to `.mycli-fts/session_events.jsonl`:

| Event Kind | Actions | When | Fields |
|-----------|---------|------|--------|
| `fact` | `add` | on `remember` | summary |
| `stash` | `create` | on `stash_context` | handle, tokens, summary |
| `recall` | `read` | on `recall` | query, source |
| `index` | `reindex` | on `index_repository` | files, trigger |
| `watch` | `auto_reindex` | on fsnotify watch trigger | changed_files |

### session_brief — catch up without re-reading

```
sessionBriefHandler()
  ├─ ledger.SessionBrief(repoPath, scope, budget)
  │   ├─ ReadAll() → parse JSONL → []Event
  │   ├─ filterByScope(ScopeLast / ScopeToday / ScopeAll)
  │   │   └─ ScopeLast: events since last 30-min idle gap
  │   ├─ renderEvents() → human-readable lines
  │   └─ trim to budget (oldest dropped first)
  └─ return rendered summary
```

### Session skill (internal/skill/policy.md)

~275 tokens of guidance delivered automatically via `server.WithInstructions()` to every MCP client on connect. Also installable as Claude Code skill. Teaches the model to use stash/recall/remember instead of pasting inline.

---

## 4. Data Flow Diagram — A Complete search_code Call

```
[AI Client]
    │ search_code(repo_path="/repo", query="where is UserService?")
    ▼
[mcpserver.tools.go: searchCodeHandler]
    │
    ├─ GetOrCreateRepoSession("/repo")
    │   └─ newRepoSession()
    │       ├─ treesitter.NewSymbolDB()      → opens .mycli-fts/symbols_*.db
    │       ├─ NewSharedIndexer()            → if unindexed, parses repo
    │       │   └─ tree-sitter AST walks per file
    │       │       ├─ Go/Python/JS/TS: bespoke extractors
    │       │       └─ Rust/Java/C/C++/C#/Ruby/PHP: generic spec-driven (langspec.go)
    │       ├─ NewKnowledgeStore()           → queries over SymbolDB
    │       ├─ cache.NewCache()              → .mycli-fts/cache.db
    │       ├─ stash.New()                   → .mycli-fts/stash/
    │       └─ kmemory.NewKnowledgeMemory()  → loads .mycli-fts/session_facts.json
    │
    ├─ answertype.Classify("where is UserService?", "chat")
    │   └─ matches locationPatterns → Type: Location, Confidence: 0.9
    │
    ├─ NewAutoRetriever().Initialize()
    │   └─ classifier.Classify("where is UserService?")
    │       └─ "where is" → SymbolQuery
    │       └─ route → SymbolRetriever (treesitter)
    │
    ├─ auto.Retrieve(query)
    │   └─ SymbolRetriever.Retrieve("UserService")
    │       └─ SymbolDB.Search("UserService")
    │           └─ SQLite: SELECT * FROM symbols WHERE name LIKE ?
    │           └─ returns: [{File: "internal/service/user.go", LineFrom: 42, ...}]
    │
    ├─ FilterResults(results, 0.15, 3)
    ├─ rs.StoreResult(query, results)  → LastResolved = "UserService"
    └─ CompressForAnswerType(results, {Type:Location}, budget=200)
        └─ compressLocation → "internal/service/user.go:42"
```

Response to AI client: `internal/service/user.go:42`

---

## 5. Key Data Stores

| Store | Location | Format | Purpose |
|-------|----------|--------|---------|
| Symbol index | `.mycli-fts/symbols_*.db` | SQLite (tree-sitter) | Definitions, references, call edges |
| FTS index | `.mycli-fts/fts_*.db` | SQLite | Full-text search |
| LRU cache | `.mycli-fts/cache*.db` | SQLite | Recent lookups |
| Stash blobs | `.mycli-fts/stash/<handle>.txt` | Plain text | Large content parked out of context |
| Stash manifest | `.mycli-fts/stash/manifest.json` | JSON | Metadata for stashed blobs |
| Session facts | `.mycli-fts/session_facts.json` | JSON | Durable user facts (remember tool) |
| Event ledger | `.mycli-fts/session_events.jsonl` | JSONL | Session event log (session_brief) |
| Long-term memory | `/tmp/repo_memory.db` | SQLite | Cross-session symbol learning (⚠️ shared) |
| Discovery memory | `/tmp/discovery_memory.db` | SQLite | Cross-session discovery patterns (⚠️ shared) |

---

## 6. Component Dependency Map

```
cmd/costwise/main.go
  └─ cmd.Execute()
      ├─ cmd/serve.go → mcpserver.NewServer() → server.ServeStdio()
      │   └─ internal/mcpserver/
      │       ├─ server.go — NewServer() with skill instructions
      │       ├─ session_cache.go — GetOrCreateRepoSession() (process-global cache)
      │       │   └─ internal/session/repo_session.go — RepoSession (owns everything)
      │       │       ├─ internal/treesitter/ — SymbolDB, Language, Symbol, langspec
      │       │       ├─ internal/retrieval/  — retrievers, pipeline, compress, summary
      │       │       ├─ internal/cache/      — LRU cache
      │       │       ├─ internal/stash/      — blob store
      │       │       ├─ internal/kmemory/    — knowledge memory
      │       │       ├─ internal/repo_memory/— long-term symbol memory
      │       │       ├─ internal/discovery_memory/ — discovery patterns
      │       │       └─ internal/watcher/    — fsnotify watchdog
      │       ├─ tools.go — 10 tool handlers (wired by RegisterTools())
      │       │   ├─ internal/answertype/     — classify answer type from query
      │       │   ├─ internal/classifier/     — classify query class for routing
      │       │   ├─ internal/retrieval/      — filters, compression
      │       │   └─ internal/ledger/         — event logging
      │       └─ internal/skill/              — session-awareness policy
      │
      ├─ cmd/install.go → internal/installer/  — detect, build, configure clients
      ├─ cmd/uninstall.go  → internal/installer/targets/ — remove MCP configs
      ├─ cmd/doctor.go     → internal/doctor/  — validation checks
      ├─ cmd/skill.go      → internal/skill/   — install/uninstall/print skill
      ├─ cmd/chat.go       → internal/retrieval/ — chat pipeline
      ├─ cmd/plan.go       → internal/retrieval/ — plan pipeline
      └─ cmd/agent.go      → internal/retrieval/ — agent pipeline
```

---

## 7. Pipeline Steps (in order)

When an AI client calls `search_code`:

1. **Query Classification** (`answertype.Classify`) — yes_no / location / caller / reference / overview / explanation / plan / agent / improvement / repository_analysis / architecture_review / feature_suggestion
2. **Query Routing** (`classifier.Classify` → `AutoRetriever`) — SymbolQuery / TextQuery / RepositoryQuery / ReferenceQuery / CallQuery / ArchitectureQuery / FlowQuery
3. **Primary Retrieval** — route to the best retriever for the query class
4. **Fallback Retrieval** — if primary confidence < 0.3, try secondary retrievers
5. **Deduplication** — merge results from multiple retrievers, keep highest score per file
6. **Quality Gate** (`CheckQualityGate`) — score threshold (0.15), minimum evidence checks
7. **Answer-Type Compression** (`CompressForAnswerType`) — format results per answer type within budget
8. **Response** — compressed context returned to AI client
