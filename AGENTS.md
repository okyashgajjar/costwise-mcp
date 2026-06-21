# Context for AI Coding Agents

> Last updated by big-pickle on 2026-06-10.

## V2 Foundation (in progress — branch `feat/v2-foundation`)

**Goal:** cut the dominant cost of long single sessions — Anthropic **prompt-cache write/read**, not model output. Evidence: a single call charged $2.95 where $2.84 was a 5-min cache *write* of ~455k tokens; output was only ~3.9k. The MCP cannot control *when/how* the client caches (breakpoints/TTL are client-owned); its only lever is **reducing how many tokens ever enter the resident context window** (the thing re-cached every turn). Every V2 piece serves that one goal.

Three steps:
1. **DONE — compact `repo_summary`.** `BuildRepositorySummaryCompact(ks, budget, module)` in `internal/retrieval/repository_summary.go`: token-budgeted (reuses `parseBudget`), top modules by symbol count + `+N more` rollup, dropped the unbounded `Layers` chain, optional `module` drill-down. Tool gained `budget`+`module` params. Legacy `BuildRepositorySummary`/`Format()` untouched. (50-module synthetic: 1192→216 tokens; capped regardless of repo size.)
2. **DONE (code) — 3 cache-reducing tools** in `internal/mcpserver/tools.go`: `remember(repo_path,key,fact)` (durable fact → `kmemory` `UserNote` + per-repo `session_facts.json`), `stash_context(repo_path,content,label?)` (park large blob out of window → tiny handle; file-backed `internal/stash` at `<repoRoot>/.mycli-fts/stash/`), `recall(repo_path,query,source?,budget?)` (query-scoped read of a stash by handle, or facts; hard-capped via step-1 budgeting). Stores wired into `RepoSession` (`Stash`, `FactsPath`, `RememberFact`, `RecallFacts`). Chosen over compact/summarize/forget because the user requires **no context drop** — stash is lossless (relocates tokens, re-fetchable). All tool outputs stay tiny. New names added to `claude.go` allow-list. Tests: `internal/stash`, `internal/session/repo_session_v2_test.go`.
3. **DONE — `costwise-session` skill (session-awareness).** Teaches the model to keep the session lean (route large content through stash/recall, remember durable facts, prefer narrow retrieval). Single embedded source of truth: `internal/skill/policy.md` (`go:embed`, ~275 tok). Delivered two ways: (a) **automatic/cross-IDE** via `server.WithInstructions(skill.Instructions())` in `internal/mcpserver/server.go` — every MCP client auto-loads it, zero install; (b) **native Claude Code SKILL.md** via `costwise skill {install,uninstall,print}` (`cmd/skill.go`) writing `~/.claude/skills/costwise-session/SKILL.md` (or `.claude/...` with `--local`). `install` writes the skill by default (opt out `--no-skill`); `uninstall` removes it. Other IDEs rely on the instructions field + `skill print` for manual placement. `internal/skill` is standalone (NOT in the Target interface). Tests in `internal/skill/skill_test.go`.

**LANDMINE:** `repo_memory`/`discovery_memory` Init with shared `os.TempDir()` paths (NOT per-repo) — a clobber risk (same class as the shared-index bug). New V2 stores MUST be per-repo (derive from `repoRoot` like `treesitter.NewSymbolDB`/`cache.NewCache`).

**Honest limit:** these tools can't evict content the client already placed in context; they only help when the model *routes new large content through them* — which is what the step-3 skill enforces. MCP server-side state persists across tool calls via a process-global per-repo `SessionCache` (`internal/mcpserver/session_cache.go`).

## CGO Requirement

**CGO is mandatory.** The project depends on:
- `github.com/mattn/go-sqlite3` — `//go:build cgo` constraint
- `github.com/smacker/go-tree-sitter` — C bindings via cgo

Builds with `CGO_ENABLED=0` will fail. Always ensure `CGO_ENABLED=1` (which is the default on most systems with a C compiler).

On Ubuntu/Debian: `sudo apt install gcc libsqlite3-dev`
On macOS: Xcode Command Line Tools (`xcode-select --install`)
On Windows: MinGW-w64 (`mingw-w64`)

## Build & Test
```bash
# Build all packages (CGO must be enabled)
CGO_ENABLED=1 go build ./...

# Run all tests
CGO_ENABLED=1 go test ./...

# Build binary
CGO_ENABLED=1 go build -o costwise ./cmd/costwise/

# Build with version injection (for releases)
go build -ldflags="\
  -X github.com/okyashgajjar/costwise-mcp/cmd.version=v1.0.0 \
  -X github.com/okyashgajjar/costwise-mcp/cmd.commit=$(git rev-parse --short HEAD) \
  -X github.com/okyashgajjar/costwise-mcp/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o costwise ./cmd/costwise/
```

Default version (no ldflags): `dev`
Injected version example: `v1.0.0` with commit hash and build date

## Release Process

1. Tag the release: `git tag -a v1.0.0 -m "v1.0.0"`
2. Push the tag: `git push origin v1.0.0`
3. CI runs test.yml (test+lint), then release.yml builds + publishes artifacts via GoReleaser
4. GoReleaser creates GitHub Release with artifacts for all 5 targets

### Release artifact verification
```bash
./costwise --version
# Expected: costwise v1.0.0
#           commit: abc1234
#           built:  2026-06-10T00:00:00Z
```

### CGO Cross-Compilation Strategy (`.goreleaser.yaml`)
Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`

| Target            | CC                          | Package                          |
|-------------------|-----------------------------|----------------------------------|
| linux/amd64       | `gcc`                       | (native on ubuntu-latest)        |
| linux/arm64       | `aarch64-linux-gnu-gcc`     | `gcc-aarch64-linux-gnu`          |
| darwin/amd64      | `zig cc -target x86_64-macos` | zig                          |
| darwin/arm64      | `zig cc -target aarch64-macos` | zig                          |
| windows/amd64     | `x86_64-w64-mingw32-gcc`    | `gcc-mingw-w64-x86-64`           |

`CGO_ENABLED=1` set globally. Per-target `CC` via `overrides` block.

### Local snapshot build
```bash
goreleaser release --snapshot --clean
# Artifacts in ./dist/
```

### Local config validation
```bash
goreleaser check
```

## Project Structure
- `/home/mryg/Research-Architectures/CLI/` — Go module root
- `cmd/costwise/main.go` — entry point
- `cmd/install.go` — interactive install (detect → prompt → MCP config; `--build` to rebuild)
- `cmd/uninstall.go` — remove MCP configs from configured clients
- `cmd/doctor.go` — diagnostic checks (binary, PATH, MCP configs, startup, repository)
- `cmd/serve.go` — MCP stdio server
- `cmd/chat.go` — chat mode
- `cmd/plan.go` — plan mode
- `cmd/agent.go` — agent mode
- `cmd/analyze.go` — analyze mode
- `internal/installer/target.go` — Target interface, BinaryPath, GetMcpServerConfig, shared helpers
- `internal/installer/binary.go` — binary installation, verification, PATH checks, ActionableError
- `internal/installer/installer.go` — Installer orchestrator with install, uninstall, repair modes
- `internal/installer/targets/` — per-client targets (claude, cursor, opencode, codex, antigravity)
- `internal/doctor/doctor.go` — doctor checks (binary, PATH, MCP configs, startup, repository)
- `internal/repository/state.go` — 3-state index lifecycle
- `internal/repository/state_cli.go` — CLI prompts/display
- `internal/session/repo_session.go` — session with NewRepoSession / NewRepoSessionWithoutIndex

## Architecture

    mycli chat|plan|agent|analyze
      └─ DetectRepositoryState() → Unindexed | Stale | Ready
           └─ chat/plan: prompt user; agent/analyze: auto-index
                └─ NewRepoSession(with or without index)
                     └─ pipeline: AnswerType → KnowledgeMem → Cache → Retrievers → QualityGate → Compress → LLM
                     └─ Response Compression if output > 2x budget
                     └─ Learn() stores results back into KnowledgeMem

## Answer Types & Output Budgets
| Type                | Max Tokens | Evidence Required |
|---------------------|-----------|-------------------|
| yes_no              | 10        | topScore >= 0.3   |
| location            | 25        | topScore >= 0.3   |
| reference           | 50        | >= 1 result       |
| caller              | 50        | >= 1 result       |
| overview            | 150       | >= 1 result       |
| improvement         | 200       | >= 3 results      |
| feature_suggestion  | 200       | >= 3 results      |
| architecture_review | 250       | >= 3 results      |
| repository_analysis | 300       | >= 3 results      |
| explanation         | 400       | >= 1 result       |
| plan                | 500       | >= 1 result       |
| agent               | dynamic   | N/A               |

Budgets enforced via API `max_tokens`, not prompt.

## Key Files
- `internal/repository/state.go` — DetectRepositoryState(), 3-state enum, hash comparison
- `internal/repository/state_cli.go` — PromptIndex(), PromptReindex(), Show*() display funcs
- `internal/session/repo_session.go` — NewRepoSession (indexes), NewRepoSessionWithoutIndex (skips index)
- `internal/kmemory/kmemory.go` — session knowledge memory
- `internal/answertype/classifier.go` — 12 answer types with MaxTokens budgets and pattern matching
- `internal/retrieval/pipeline.go` — pipeline ordering, quality gates, system prompt builder
- `internal/retrieval/compress.go` — context compression per answer type
- `internal/retrieval/compress_response.go` — response compression when output > 2x budget
- `internal/retrieval/learn.go` — auto-learning from results
- `internal/retrieval/repository_summary.go` — RepositorySummary builder (modules, files, languages, symbols)
- `internal/retrieval/improvement.go` — Improvement struct, Impact/Effort/Confidence ranking
- `cmd/analyze.go` — repository analysis command (<300 token output)

## Response Compression
After LLM returns, if `output_tokens > MaxTokens * 2`, a second LLM call compresses with:
"Rewrite answer in shortest possible form. Keep only actionable info. Remove explanations."

## Repository Summary (for improvement/analysis types)
When user asks "improve", "analyze", "review architecture", "suggest features":
1. Build RepositorySummary from KnowledgeStore (modules, files, languages, symbols)
2. Send ONLY summary as context (never raw files)
3. Apply hard output budget
4. Compress if exceeded

## Installer Design

`internal/installer/` has three binary resolution modes:

| Mode | Entry Point | When |
|------|-------------|------|
| **Use existing** | `EnsureBinary()` | Default: copies `os.Executable()` to `~/.local/bin/costwise` |
| **Build from source** | `InstallBinary()` | `--build` flag: requires Go toolchain + `go.mod` in parent tree |
| **Repair** | `runRepair()` | `--repair`: maps to `EnsureBinary()` or `InstallBinary()` depending on `--build` |

**`EnsureBinary()` resolution order:**
1. Binary already at `DefaultBinaryPath()` and verifiable → return it
2. `os.Executable()` returns a valid path → copy to `DefaultBinaryPath()`, return it
3. `exec.LookPath("costwise")` succeeds → copy to `DefaultBinaryPath()`, return it
4. None found → return `ActionableError` suggesting `--build`

**Key invariants:**
- Default install does NOT require a Go module or `go.mod`
- `--build` is opt-in (default `false`)
- `EnsureBinary()` never calls `findGoModRoot()`
- `CheckBinary()` includes `os.Executable()` as a candidate
- Works from any directory (`/tmp`, outside repo, etc.)
- Cross-platform: `os.Executable()` supported on Windows/macOS/Linux

## Invariants
- `go build ./...` must compile with zero errors (GOPATH warning is expected)
- `go test ./...` must pass
- All 12 answer types integrate into existing pipeline without breaking it
- All 8 existing systems (SharedIndexer, SymbolDB, Auto Router, Query Classifier, retrievers, RepoSession, LRU Cache, Knowledge Store) remain as-is — new code integrates, never replaces.

<!-- costwise-session:start (managed by `costwise skill` — do not edit) -->

## costwise-session

This project is connected to the **costwise** MCP server. Its tools keep the session cheap: in long sessions the dominant cost is the prompt cache re-reading everything in context each turn, so keep the window small. Apply this for the rest of the session — no need to be reminded again.

**Route large content out of context, don't paste it inline.**
- For any large output (file, long command/test output, generated report), call `stash_context` to park it and get a short handle, then `recall(source=<handle>, query=…)` to pull back only the slice you need.
- Persist durable facts (a decision, an entrypoint, a gotcha) with `remember`; retrieve them later with `recall` instead of re-deriving or re-pasting them.

**Prefer narrow retrieval over reading whole files.** Reach for a full file read only when a targeted query genuinely can't answer it.
- Pick the tool that fits: `find_symbol` to locate, `read_symbol` to see an implementation body, `find_references`/`find_callers` for usage, `search_code` for conceptual questions, `get_repository_summary` for structure. `recall` reads remembered facts/stashes, not code. For raw regex over files, use the host's own grep.
- Default budget unless insufficient — one `large` call can add ~10k uncached tokens.

<!-- costwise-session:end -->
