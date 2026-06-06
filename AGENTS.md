# Context for AI Coding Agents

> Last updated by big-pickle on 2026-06-05.

## Build & Test
```bash
go build ./...   # Build all packages (expects "warning: GOPATH set to GOROOT" only)
go test ./...    # Run all tests

# Build binary
go build -o costaffective ./cmd/mycli/
```

## Project Structure
- `/home/mryg/CostAffective-CLI/CLI/` — Go module root
- `cmd/mycli/main.go` — entry point
- `cmd/install.go` — interactive install (build → detect → prompt → MCP config)
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

## Invariants
- `go build ./...` must compile with zero errors (GOPATH warning is expected)
- `go test ./...` must pass
- All 12 answer types integrate into existing pipeline without breaking it
- All 8 existing systems (SharedIndexer, SymbolDB, Auto Router, Query Classifier, retrievers, RepoSession, LRU Cache, Knowledge Store) remain as-is — new code integrates, never replaces.
