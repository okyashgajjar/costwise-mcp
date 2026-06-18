# Contributing to CostAffective

## The problem this project solves

Every AI coding session starts fast. Then it slows down. Not because the model got worse — because the context window fills up with junk.

Every file dump, every "where is X" that re-opens a whole file, every test log pasted inline — it all accumulates. Every subsequent turn re-reads and re-caches everything. You pay for the whole pile when you need one line.

A real trace: a single API call billed at $2.95, where $2.84 was a cache write re-reading ~455K tokens of resident context. The answer was $0.11.

The dominant cost in long sessions is not the model's output. It's the **prompt cache** — re-reading and re-writing the entire context window every turn. The only lever: keep tokens out of the window in the first place.

## The solution

CostAffective is an open-source MCP server that runs locally and gives AI coding agents token-budgeted access to your repository. Instead of dumping files, it answers from a pre-built Tree-sitter index. Instead of pasting large output inline, it stashes content out of context behind a tiny handle. Instead of re-deriving facts, it remembers them per-repo.

**11 MCP tools across three categories:**

**Retrieval** (index-backed, keeps tokens tiny)
| Tool | What it does |
|------|-------------|
| `search_code` | Semantic search by natural language question |
| `find_symbol` | Locate a symbol definition |
| `read_symbol` | Return a symbol's full implementation body |
| `find_references` | All usages of a symbol, precomputed |
| `find_callers` | Which functions call a given function |

**Context control** (keep large content out of the window)
| Tool | What it does |
|------|-------------|
| `remember` | Persist a durable fact per-repo |
| `stash_context` | Park a large blob behind a 20-token handle |
| `recall` | Pull back only the slice matching a query |

**Maintenance**
| Tool | What it does |
|------|-------------|
| `get_repository_summary` | Token-budgeted repo overview, drillable by module |
| `index_repository` | Manual re-index trigger |
| `deploy` | Deploy tools |

Plus a ~275-token session-awareness skill auto-delivered via MCP instructions field that teaches agents to use all of the above.

## Architecture

```
cmd/                    # Entry points
├── costaffective/
│   └── main.go         # Binary entry point
├── install.go          # Interactive install
├── uninstall.go        # Remove MCP configs
├── doctor.go           # Diagnostic checks
├── serve.go            # MCP stdio server
├── chat.go             # Chat mode
├── plan.go             # Plan mode
├── agent.go            # Agent mode
├── analyze.go          # Analyze mode
└── skill.go            # Session skill management

internal/               # Core logic
├── installer/          # Binary management, MCP config targets
│   ├── binary.go       # Binary resolution (EnsureBinary, CheckBinary)
│   ├── installer.go    # Install/uninstall/repair orchestrator
│   ├── target.go       # Target interface, BinaryPath
│   └── targets/        # Per-client targets
│       ├── claude.go
│       ├── cursor.go
│       ├── opencode.go
│       ├── codex.go
│       └── antigravity.go
├── session/            # RepoSession — owns all per-repo state
│   └── repo_session.go # NewRepoSession, RememberFact, RecallFacts, pronoun resolution
├── retrieval/          # Pipeline, compress, retrievers, knowledge store
│   ├── pipeline.go     # Pipeline ordering, system prompt builder
│   ├── compress.go     # Context compression per answer type
│   ├── compress_response.go  # Response compression when output > 2x budget
│   ├── repository_summary.go # Token-budgeted repo summary builder
│   ├── learn.go        # Auto-learning from results
│   ├── improvement.go  # Improvement struct (Impact/Effort/Confidence)
│   └── ...             # Multiple retriever implementations
├── mcpserver/          # MCP protocol handlers
│   ├── server.go       # NewServer, instructions field
│   ├── tools.go        # All 12 MCP tool handlers
│   └── session_cache.go # Process-global per-repo SessionCache
├── treesitter/         # Symbol DB (Tree-sitter SQLite index)
│   └── ...             # Parser, call extraction, reference resolution
├── answertype/         # Answer type classification (12 types with budgets)
│   └── classifier.go   # Classify(), MaxTokens budgets
├── kmemory/            # Knowledge memory (structured facts)
├── stash/              # Large-blob storage (stash_context / recall)
├── cache/              # LRU lookup cache
├── repository/         # Repository state detection (unindexed/stale/ready)
│   ├── state.go        # DetectRepositoryState, hash comparison
│   └── state_cli.go    # CLI prompts/display
├── skill/              # Session policy (embedded ~275 tokens)
├── classifier/         # Query classifier
├── contextbuilder/     # Context level builder
├── config/             # Configuration
├── doctor/             # Doctor diagnostic checks
├── watcher/            # File system watcher (auto re-index)
├── updater/            # Self-update
├── storage/            # Benchmark storage
├── benchmark/          # Benchmark suite
├── discovery_memory/   # Cross-session discovery patterns
├── repo_memory/        # Long-term symbol memory
└── architecture/       # Architecture extraction
```

**Data flow:**

```
AI Client (MCP Host)
    │
    ├── stdio transport ──► costaffective serve
    │                           │
    │                           ├── Session Manager
    │                           ├── Tree-sitter Parser
    │                           ├── Symbol Index (SQLite)
    │                           ├── Reference Index
    │                           ├── Call Graph Index
    │                           │
    │                           ├── search_code ───────► AST match
    │                           ├── find_symbol ───────► SymbolDB
    │                           ├── read_symbol ───────► line-range read
    │                           ├── find_references ───► SymbolDB
    │                           ├── find_callers ──────► CallGraph
    │                           ├── get_repository_summary ► KnowledgeStore
    │                           ├── remember ──────────► per-repo facts
    │                           ├── stash_context ─────► .mycli-fts/stash/
    │                           └── recall ────────────► query-scoped read
```

All per-repo state lives under `.mycli-fts/` in the repo root. No cloud. No API keys.

## Getting started

```bash
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test ./...
```

**Requirements:** Go 1.25+, C compiler (CGO mandatory — go-sqlite3 + tree-sitter).

| OS | C compiler install |
|----|-------------------|
| Ubuntu/Debian | `sudo apt install gcc libsqlite3-dev` |
| macOS | `xcode-select --install` |
| Windows | MinGW-w64 |

## Good first issues

These are concrete, scoped tasks where a new contributor can make a real impact:

### Add a Tree-sitter language parser

Pick a language not yet supported, add its Tree-sitter grammar, and wire it into the symbol extraction pipeline. A new grammar unlocks `find_symbol`, `read_symbol`, `find_references`, and `find_callers` for that language.

**Files involved:** `internal/treesitter/`
**Existing example:** look at how Go or Python symbols are extracted.
**Difficulty:** moderate (needs understanding of Tree-sitter AST structure).

### Fix the shared /tmp/ DB paths

`repo_memory.Init()` and `discovery_memory.Init()` use `os.TempDir()` paths, meaning they clobber when multiple repos are indexed on the same machine. They should use per-repo paths under `.mycli-fts/` instead.

**Files involved:** `internal/session/repo_session.go:101-110`, `internal/repo_memory/`, `internal/discovery_memory/`
**Difficulty:** easy to moderate.

### Improve compression for a specific answer type

Each answer type (explanation, plan, location, etc.) has a token budget and a compression strategy. Some types could return more useful content within the same budget with better heuristics.

**Files involved:** `internal/retrieval/compress.go`, `internal/answertype/`
**Difficulty:** easy.

### Add SSE/HTTP transport

The server currently only speaks stdio MCP. Adding HTTP+SSE transport would let Smithery and other platforms host it as a remote server.

**Files involved:** `internal/mcpserver/server.go`, `cmd/serve.go`
**Difficulty:** moderate.

### Write more benchmarks

The benchmark suite at `internal/benchmark/` evaluates retrieval accuracy and token efficiency. More benchmark tasks across different languages and repository sizes would help validate improvements.

**Files involved:** `internal/benchmark/`, `benchmarks/`
**Difficulty:** easy.

### Improve documentation

Better examples, clearer install guides, more use case walkthroughs. Documentation is as valuable as code.

**Files involved:** `README.md`, `docs/`
**Difficulty:** easy — great for first PRs.

### Fix a labeled "bug" issue

Check the [GitHub Issues](https://github.com/okyashgajjar/costaffective-mcp/issues) tab for open bugs.

## How to contribute

1. **Fork** the repo
2. **Create a branch:** `git checkout -b feat/my-thing`
3. **Make your changes** — keep them focused on one thing
4. **Run the tests:** `CGO_ENABLED=1 go test ./...`
5. **Run the linter:** `golangci-lint run ./...`
6. **Push and open a PR** with a clear title and description

### PR guidelines

- One PR = one change. Don't mix refactoring with features.
- Write a clear title and description explaining *why* the change exists.
- Add tests for new functionality.
- Match existing code style (gofmt, no commented-out code).
- Reference the issue number if applicable: `Closes #123`.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Be respectful, be constructive, be kind.

## Questions?

Open a [Discussion](https://github.com/okyashgajjar/costaffective-mcp/discussions) or ask in an issue. We're happy to help you find a place to start.
