<div align="center">

**CostAffective — MCP**

<p align="center">
  <img
    src="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/logo(1).png"
    alt="CostAffective Logo"
    width="500"
  />
</p>

Coding agents that explore less, remember more, and carry less context.

[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](#installation)
[![Linux](https://img.shields.io/badge/Linux-supported-blue.svg)](#supported-platforms)
[![macOS](https://img.shields.io/badge/macOS-supported-blue.svg)](#supported-platforms)
[![Windows](https://img.shields.io/badge/Windows-supported-blue.svg)](#supported-platforms)
[![Claude Code](https://img.shields.io/badge/Claude_Code-supported-blueviolet.svg)](#supported-clients--config)
[![Cursor](https://img.shields.io/badge/Cursor-supported-blueviolet.svg)](#supported-clients--config)
[![OpenCode](https://img.shields.io/badge/OpenCode-supported-blueviolet.svg)](#supported-clients--config)
[![GitHub Stars](https://img.shields.io/github/stars/okyashgajjar/costaffective-mcp?style=social)](https://github.com/okyashgajjar/costaffective-mcp)

```
curl -fsSL https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/install.sh | bash
```

One command on Linux, macOS, or Windows (via WSL). Detects your OS, installs Go if needed, builds from source, and connects your AI coding client.

**Star this repo** — it helps others find CostAffective.

<table>
  <tr>
    <td align="center">
      <a href="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/proofs/without-mcp-smallrepo-opencode.png">
        <img src="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/proofs/without-mcp-smallrepo-opencode.png" width="400" alt="Without CostAffective">
      </a>
      <br><strong>Without CostAffective</strong><br><a href="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/proofs/opencode-without-costaffective.webm">▶ watch video</a>
    </td>
    <td align="center">
      <a href="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/proofs/with-mcp-smallrepo-opencode.png">
        <img src="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/proofs/with-mcp-smallrepo-opencode.png" width="400" alt="With CostAffective">
      </a>
      <br><strong>With CostAffective</strong><br><a href="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/proofs/opencode-with-costaffective.webm">▶ watch video</a>
    </td>
  </tr>
</table>

</div>

---

## Coding agents should behave like experienced engineers

An experienced engineer walks into a codebase and does **not**:

- Re-read the same files every time they answer a question.
- Re-discover symbols they already found ten minutes ago.
- Keep a 5,000-line build log in their head because they might need it later.
- Carry every detail of every conversation around forever.

They remember where things live. They open only what they need. They hold facts, not files.

Most AI coding agents do the opposite. They re-explore the same repository over and over. They dump whole files into context. They grow the conversation window until every turn becomes expensive — not because the answer is hard, but because *everything else* in the window has to be re-read and re-cached.

CostAffective is a local MCP server that makes coding agents behave more like that experienced engineer. It gives them fast, token-budgeted access to your repository — so they stop reading whole files to find small facts.

---

## Benchmarks

### Small repository

| Metric | Without | With | Change |
|---|---|---|---|
| Tokens consumed | 162,544 | 76,757 | **52.8% fewer** |
| Wall time | 84s | 19.6s | **4.3x faster** |
| Tool calls | 54 | 4 | **92.6% fewer** |

*Same analysis task, same model, same output quality. The difference is index-backed tools instead of file dumps.*

### Continue OSS (large repository)

| Metric | CostAffective | Alternative | Change |
|---|---|---|---|
| Tokens | 4.7M | 8.7M | **45.9% lower** |
| API Calls | 89 | 134 | **33.6% fewer** |
| Exploration Calls | 43 | 94 | **54.3% fewer** |

> Full benchmark suite: [costaffective-mcp.vercel.app/benchmarks](https://costaffective-mcp.vercel.app/benchmarks)

---

## Why long sessions get expensive

The problem is not the model's output. The problem is everything around it.

### Repositories get re-explored

Ask your coding agent "where is X defined" and it reads the file. Ask again five minutes later — it reads the same file again. Each read puts thousands of tokens into the context window. Over the course of a session this adds up to hundreds of thousands of tokens for information the model has already seen.

### Context grows without being useful

A typical session starts small. Then the model dumps a file to answer a question. Then it dumps a test output. Then a build log. None of these leave — they accumulate in the resident context window, making every subsequent turn more expensive.

### The prompt cache makes it worse

Providers cache the conversation so repeated context is cheaper to resend. But caching has a cost:

- Every turn pays to **read** the entire resident context (everything currently in the window).
- Any change to earlier context, or a short idle gap, invalidates the cache and forces a full **rewrite** of everything resident.

In a long session this compounds. A real measured example: a single API call billed at **\$2.95**, of which **\$2.84 was the cache *write*** of roughly **455,000 tokens** of resident context. The model's actual output that turn was under 4,000 tokens. The expensive part was not the answer — it was the size of the context being carried and re-cached.

---

## How CostAffective fixes it

A tool that connects over MCP cannot control how or when the client caches. Cache breakpoints and TTLs are the client's decision. There is exactly one lever the server controls:

> **How many tokens ever enter the resident context window in the first place.**

Shrink that, and both costs fall: a smaller window is cheaper to read every turn *and* cheaper to rewrite when the cache is invalidated. CostAffective does four things in service of that goal.

### 1. Answer from a local index, not from files

CostAffective parses your repository once with Tree-sitter and stores symbols, references, and call edges in a local SQLite index. Navigation questions — "where is this defined," "who calls this," "what references this" — are answered from the index in a few tokens instead of by dumping source files.

Results are compressed scopes sized to the kind of question asked. The model gets the location, not the file; the implementation body, not the whole module; the caller list, not the grep output.

### 2. Remember facts instead of repeating them

The `remember` tool persists a small durable fact — a decision, an entry point, a gotcha — to a per-repository store. The `recall` tool retrieves it later. Facts the model would otherwise re-derive or re-paste each turn are written down once and read back only when relevant.

### 3. Stash large output instead of pasting it inline

The `stash_context` tool parks a large blob — a file, a command output, a test log — out of the conversation and returns a short handle. The full content stays on disk, recoverable. The `recall` tool pulls back only the slice that matches a query, within a token budget.

A 5,000-line build log pasted inline is re-read and potentially re-written to the cache every turn for the rest of the session. Stashed, it costs about 20 tokens (the handle) and is pulled back only in the slice you need.

### 4. A session-awareness skill that makes the model use all of the above

A small piece of guidance (about 275 tokens) is delivered to every connected editor through the MCP protocol's `instructions` field. It teaches the model the lean workflow once per session: route large output through `stash_context`, persist durable facts with `remember`, prefer narrow retrieval over file reads.

It is also installable as a native Claude Code skill, AGENTS.md entry, or rules file for any editor that reads them. The same canonical source backs all delivery paths.

---

## Installation

<details>
<summary><strong>Installation</strong></summary>

<br>

> Full installation guide with platform-specific variants: [costaffective-mcp.vercel.app/docs/install](https://costaffective-mcp.vercel.app/docs/install)

### Quick Install (Linux / macOS / Windows via WSL)

The recommended way — one command:

```bash
curl -fsSL https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/install.sh | bash
```

The script does everything:
1. On Windows, detects Git Bash / MSYS and routes through WSL automatically
2. Checks for Go and installs it if missing
3. Checks for a C compiler (a CGO dependency) and installs it if missing
4. Clones the repo and builds from source
5. Installs to `/usr/local/bin/costaffective`
6. Detects AI coding clients and asks which to connect
7. Configures MCP for the selected clients (and installs the session skill unless `--no-skill`)

### Windows (Native PowerShell)

Not recommended unless you already have Go and gcc. Build manually:

```powershell
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
$env:CGO_ENABLED=1
go build -o costaffective.exe ./cmd/costaffective/
```

Or use the recommended path — install WSL (Windows 10 2004+ / Windows 11):

```powershell
# In PowerShell as Administrator:
wsl --install

# Then in WSL:
curl -fsSL https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/install.sh | bash
```

### macOS / Linux (Manual Build)

Requires Go 1.25+ and a C compiler (CGO is mandatory — see the build notes below).

```bash
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
CGO_ENABLED=1 go build -o costaffective ./cmd/costaffective/
sudo mv costaffective /usr/local/bin/
costaffective --version
```

</details>

---

## What you get

CostAffective provides **9 MCP tools** that fall into three categories.

**Retrieval tools** answer questions from a pre-built index instead of by reading files:

- `search_code` — semantic search by natural language question.
- `find_symbol` — locate where a symbol is defined.
- `read_symbol` — return a symbol's full implementation body.
- `find_references` — every usage of a symbol, precomputed.
- `find_callers` — which functions call a given function.

**Maintenance tools** keep the index in sync:

- `get_repository_summary` — token-budgeted overview of the repo, drillable by module.
- `index_repository` — manual re-index trigger (auto-watcher normally handles this).

**Context-control tools** keep large content and durable facts out of the resident window:

- `remember` — persist a fact once instead of repeating it inline.
- `stash_context` — park a large blob out of context behind a tiny handle.
- `recall` — pull back only the slice that matches a query, within a budget.

<details>
<summary><strong>Full tool catalog — why each tool exists and how to use it</strong></summary>

<br>

> Interactive tool catalog with schemas and examples: [costaffective-mcp.vercel.app/tools](https://costaffective-mcp.vercel.app/tools)

### Retrieval tools

#### search_code

Semantic repository search powered by Tree-sitter.

*Why:* a natural-language question ("where is caching implemented?") should return the relevant scopes directly, not a list of files for the model to open one by one.

> Example: `Where is caching implemented?`

#### find_symbol

Find where a symbol is defined.

*Why:* "where is X defined" is the single most common navigation question. It should cost a location, not a file.

> Example: `Find UserService`

#### read_symbol

Return a symbol's full implementation body by name.

*Why:* "show me how X works" should cost one indexed line-range read, not a whole-file dump or an agent looping through search to reconstruct the body.

> Example: `Show the body of GetOrCreateRepoSession`

#### find_references

Find every usage of a symbol.

*Why:* impact analysis ("what will this change break?") needs every usage, precomputed, without grepping the tree live.

> Example: `Where is UserService used?`

#### find_callers

Find which functions call another function.

*Why:* understanding a call chain should read from stored call edges, not from the model reconstructing it by reading callers' files.

> Example: `What calls processPayment()?`

*Note:* `search_code` already routes an exact-text/full-text strategy internally, so a literal match is covered without a separate tool. For raw regex over files, use the host's native file search.

### Maintenance tools

#### get_repository_summary

A token-budgeted overview of the repository: languages, the top modules by symbol count, and key symbols. Pass `module` to drill into one directory, and `budget` (`small` / `medium` / `large`) to cap the output.

*Why:* this is usually the first call of a session, so it is also the first thing that lands in the cached context and stays there. The earlier version emitted one line per directory plus a full per-directory chain with no limit — on a large monorepo that was tens of thousands of tokens, cached for the entire session. It is now hard-capped: the output stays small no matter how large the repository is, and details are pulled on demand via `module`.

#### index_repository

Refresh or rebuild repository indexes manually. Usually unnecessary because the watchdog re-indexes automatically.

### Context-control tools (V2)

These exist because of the cache cost described above. They let the model keep large content and durable facts *out* of the resident window, losslessly.

#### remember

Persist a small durable fact — a decision, an entry point, a gotcha — to a per-repository store, so it does not have to be repeated inline in the conversation every time it is relevant.

*Why:* facts the model re-derives or re-pastes each turn are pure cache overhead. Write them down once; read them back when needed.

#### stash_context

Park a large blob (a whole file, a long command or test output, a generated report) **out of the conversation** and get back a short handle. Nothing is lost — the full content is written to disk and remains re-fetchable.

*Why:* this is the most direct lever on window size. A 5,000-line log pasted inline is re-read and potentially re-written to the cache every turn for the rest of the session. Stashed, it costs about 20 tokens (the handle) and is pulled back only in the slice you actually need.

> Example: stash a 5,000-line log, keep roughly 20 tokens in context.

#### recall

Take back **only what you need**: the budgeted slice of a stashed blob (by handle), or matching remembered facts — instead of re-reading the whole thing.

*Why:* "take output by necessary query" is the read side of the loop. Combined with `stash_context` it becomes: stash the monster, then recall only the lines that match your query, within a token budget.

**The loop:** `stash_context` (park it) → `recall` (pull back only the slice) → `remember` (keep the durable conclusion). The content is always recoverable; the window stays small.

</details>

---

<details>
<summary><strong>How the session skill makes the model use all of this automatically</strong></summary>

<br>

**Why it exists:** the context-control tools above only help if the model actually reaches for them. Left to its defaults, a model will happily paste a whole file inline. The `costaffective-session` skill is a small piece of session-awareness guidance that teaches the model the lean workflow **once per session**, after which it applies automatically — route large output through `stash_context` / `recall`, persist durable facts with `remember`, and prefer narrow retrieval over reading whole files.

It is deliberately tiny (about 275 tokens). That is a fixed, one-time cost per session, and it pays for itself the first time it prevents a single large blob from entering the window.

It is delivered two ways, so it works everywhere with no ongoing effort from you:

1. **Automatically, in every editor.** The MCP server advertises the guidance through the protocol's `instructions` field. Every MCP client loads it on connect — Claude Code, Cursor, Codex, OpenCode, Antigravity, and any other MCP-compatible client. No setup, no per-editor files.

2. **As a native Claude Code skill.** Running `costaffective install` also writes `~/.claude/skills/costaffective-session/SKILL.md` (opt out with `--no-skill`). You can manage it directly:

   ```bash
   costaffective skill install      # write the skill (global)
   costaffective skill install --local   # write it into the current project only
   costaffective skill uninstall    # remove it
   costaffective skill print        # print the guidance for manual placement in any tool
   ```

For editors that read their own rules or instructions files, `costaffective skill print` outputs the guidance to paste in. The same single source of truth backs both delivery paths.

</details>

<details>
<summary><strong>Lower session cost — the cache-aware design in one place</strong></summary>

<br>

This section ties the pieces together. In long sessions the dominant cost is usually the **prompt cache** — re-reading and re-writing everything resident in the context window every turn — not the model's output. CostAffective reduces this by keeping tokens out of the window:

* **Answer from the index, not from files** — the retrieval tools return scopes and locations measured in tens of tokens, instead of whole files measured in thousands.
* **Budgeted summaries** — `get_repository_summary` is hard-capped and supports drill-down via `module`, so it never dumps a giant tree into the cached context at session start.
* **Stash instead of paste** — `stash_context` moves large output out of the window and returns a tiny handle; `recall` brings back only the matching slice. This is lossless: the full content stays on disk.
* **Remember instead of repeat** — `remember` persists durable facts per repository; `recall` brings them back without re-deriving or re-pasting them.
* **The session skill** — makes the model do all of the above by default, in every editor.

Why not just summarize or delete old context? Because that loses information. Stashing **relocates** tokens rather than discarding them, so nothing is dropped — you can always recall the full content. That was a hard design constraint: reduce the window without ever losing context.

</details>

<details>
<summary><strong>The honest truth: skills are suggestions, not commands</strong></summary>

<br>

The session skill sends about 275 tokens of guidance at the start of every session. It helps, but it is not a guarantee.

Here is what we cannot control. AI models decide what to pay attention to. A short set of instructions competes with everything else in the conversation the model is managing. Even the best-crafted skill can be ignored when the model is deep in a task. The model's weights and training determine its behavior, not the text in the system prompt. That is simply how these systems work today.

Here is what we can control:

- **Keep the skill short.** At 275 tokens it is one of the shortest and most direct instructions the model sees. That makes it more likely to be followed than a long document full of rules.
- **Use AGENTS.md.** Models treat repository-local instruction files with higher priority than global skills. If the session guidance matters for your project, put it in AGENTS.md as well.
- **Remind the model directly.** When you see it paste a large blob inline instead of using `stash_context`, one reminder is usually enough to correct course for the rest of the session.
- **Deliver through the MCP protocol.** The `instructions` field is automatic on every connection. Unlike a skill file that must be installed, this works everywhere with zero setup.
- **Build tools that do not require cooperation.** Tools like `stash_context` and `recall` do not need the model to remember the rules. They just need to be called.

The honest answer is that we are building tools for systems that do not reliably follow instructions. That is not a flaw in the skill. It is a constraint of the technology. The best we can do is make the right path the easiest path, keep our guidance direct, and accept that sometimes the model will do its own thing.

</details>

<details>
<summary><strong>Architecture</strong></summary>

<br>

> Interactive architecture diagram with component deep-dives: [costaffective-mcp.vercel.app/architecture](https://costaffective-mcp.vercel.app/architecture)

```text
AI Client (MCP Host)
    │
    ├── stdio transport ──► costaffective serve (MCP Server)
    │                           │   advertises session guidance via the MCP instructions field
    │                           │
    │                           ├── Session Manager (per-repo, persistent across tool calls)
    │                           ├── Repository State Manager
    │                           ├── Watchdog
    │                           ├── Shared Indexer
    │                           │
    │                           ├── Tree-sitter Parser
    │                           ├── Symbol Index
    │                           ├── Reference Index
    │                           ├── Call Graph Index
    │                           │
    │                           ├── search_code ───────────► tree-sitter AST match
    │                           ├── find_symbol ───────────► SymbolDB lookup
    │                           ├── read_symbol ───────────► SymbolDB body read
    │                           ├── find_references ───────► SymbolDB reference search
    │                           ├── find_callers ──────────► SymbolDB call graph
    │                           ├── get_repository_summary ► KnowledgeStore (token-budgeted)
    │                           ├── index_repository ──────► SharedIndexer
    │                           ├── remember ──────────────► per-repo durable facts
    │                           ├── stash_context ─────────► large blobs parked out of context
    │                           └── recall ────────────────► query-scoped read-back
```

All per-repository state (index, stash, facts) lives under the repository's local index directory, so separate repositories never clobber each other.

</details>

<details>
<summary><strong>Commands</strong></summary>

<br>

| Command                          | Description                              |
| -------------------------------- | ---------------------------------------- |
| `costaffective install`          | Interactive installation                 |
| `costaffective install --all`    | Configure all detected clients           |
| `costaffective install --target` | Configure a specific client              |
| `costaffective install --build`  | Build from source before installing      |
| `costaffective install --repair` | Repair installation                      |
| `costaffective install --no-skill` | Install without the session skill      |
| `costaffective skill install`    | Install the costaffective-session skill  |
| `costaffective skill uninstall`  | Remove the session skill                 |
| `costaffective skill print`      | Print the guidance for manual setup      |
| `costaffective doctor`           | Validate installation                    |
| `costaffective serve`            | Start the MCP server                     |
| `costaffective uninstall`        | Remove MCP configuration                 |

</details>

<details>
<summary><strong>Uninstall</strong></summary>

<br>

1. Remove MCP client configs (this also removes the session skill):

```bash
costaffective uninstall --all
```

2. Delete the binary:

**Linux / macOS:**

```bash
rm -f "$(command -v costaffective)"
```

**Windows:**

```powershell
Remove-Item (Get-Command costaffective).Source -Force
```

</details>



<details>
<summary><strong>Storage locations — where everything lives on disk</strong></summary>

<br>

All CostAffective data lives in two places. Here is where to find and delete it if needed.

### Per-repository storage (`.mycli-fts/`)

Created at the root of every indexed repository. Contains everything that is specific to that repo:

| Path | What it stores | Safe to delete? |
|------|---------------|-----------------|
| `.mycli-fts/stash/` | Stashed blobs from `stash_context` + `manifest.json` | Yes. Lose saved stashes. |
| `.mycli-fts/session_facts.json` | Durable facts from `remember` tool | Yes. Lose saved facts. |
| `.mycli-fts/symbols_*.db` | Tree-sitter symbol index (definitions, references, calls) | Yes. Triggers re-index on next session. |
| `.mycli-fts/fts_*.db` | Full-text search index | Yes. Triggers re-index on next session. |
| `.mycli-fts/cache*.db` | LRU cache of recent lookups | Yes. Temporary performance cache. |

To wipe everything for a repository:

```bash
rm -rf /path/to/your/repo/.mycli-fts
```

The index will rebuild automatically on the next session.

### Global storage (`/tmp/`)

Shared across all repositories. A known landmine — these are not per-repo and can clobber:

| Path | What it stores | Safe to delete? |
|------|---------------|-----------------|
| `/tmp/repo_memory.db` | Long-term symbol memory across sessions | Yes. Lose learned symbols. |
| `/tmp/discovery_memory.db` | Cross-session discovery patterns | Yes. Lose learned patterns. |

```bash
rm -f /tmp/repo_memory.db /tmp/discovery_memory.db
```

</details>

<details>
<summary><strong>Development</strong></summary>

<br>

CGO is mandatory. The project depends on `go-sqlite3` and `go-tree-sitter`, both of which use C bindings; builds with `CGO_ENABLED=0` will fail.

```bash
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test ./...
```

On Ubuntu/Debian: `sudo apt install gcc libsqlite3-dev`. On macOS: Xcode Command Line Tools. On Windows: MinGW-w64.

</details>

<details>
<summary><strong>Supported clients and config surfaces</strong></summary>

<br>

| Client         | Config Surface                                   |
| -------------- | ------------------------------------------------ |
| Claude Code    | `~/.claude.json`, `.mcp.json`, or settings files |
| Cursor         | `~/.cursor/mcp.json` or workspace MCP settings    |
| OpenCode       | `opencode.json`                                  |
| Codex CLI      | `~/.codex/config.toml`                           |
| Antigravity    | `~/.gemini/config/mcp_config.json`              |
| MCP-compatible | stdio transport                                 |

</details>

<details>
<summary><strong>Repository state</strong></summary>

<br>

**Why it exists:** the model should know whether the index it is querying is trustworthy. CostAffective tracks three states and behaves accordingly.

| State       | Meaning                                  |
| ----------- | ---------------------------------------- |
| `unindexed` | No usable index exists yet               |
| `stale`     | Files changed after the last index       |
| `ready`     | Index is in sync with the working tree   |

Agent mode can auto-index when needed; interactive modes prompt first.

</details>

<details>
<summary><strong>Use cases</strong></summary>

<br>

> Explore detailed use case studies: [costaffective-mcp.vercel.app/use-cases](https://costaffective-mcp.vercel.app/use-cases)

* **AI coding agents** — reduce token spend by up to 45.9% with compressed, scope-level lookups, and keep long sessions cheap by parking large content out of context.
* **Large monorepos** — fast SQLite index queries in microseconds instead of disk scans, and budgeted summaries that stay small regardless of repo size.
* **Code reviews** — trace caller hierarchies to audit the impact of incoming changes.
* **Repository audits** — generate summaries of file distribution, language splits, and structure.
* **MCP development** — a reference implementation for the stdio protocol, fsnotify watchers, tree-sitter mapping, and the MCP instructions field.

</details>

<details>
<summary><strong>Doctor</strong></summary>

<br>

`costaffective doctor` checks:

- Binary existence and permissions
- PATH visibility
- MCP configuration for each client
- Server startup
- Repository state

</details>

<details>
<summary><strong>Supported platforms</strong></summary>

<br>

All platforms with Go 1.25+ and a C compiler are supported via the install script (`install.sh`), which handles toolchain setup automatically:

- Linux amd64 / arm64
- macOS amd64 / arm64 (Intel and Apple Silicon)
- Windows amd64

Pre-built release binaries are available for Linux amd64 and Windows amd64. All other platforms are built from source by the install script.

</details>

<details>
<summary><strong>Learn more</strong></summary>

<br>

| Resource | Link |
| -------- | ---- |
| Website and interactive tools | [costaffective-mcp.vercel.app](https://costaffective-mcp.vercel.app) |
| Full benchmark suite | [costaffective-mcp.vercel.app/benchmarks](https://costaffective-mcp.vercel.app/benchmarks) |
| Developer documentation | [costaffective-mcp.vercel.app/docs/install](https://costaffective-mcp.vercel.app/docs/install) |
| MCP tool catalog | [costaffective-mcp.vercel.app/tools](https://costaffective-mcp.vercel.app/tools) |
| Architecture guide | [costaffective-mcp.vercel.app/architecture](https://costaffective-mcp.vercel.app/architecture) |
| FAQ | [costaffective-mcp.vercel.app/faq](https://costaffective-mcp.vercel.app/faq) |
| Blog and research | [costaffective-mcp.vercel.app/blog](https://costaffective-mcp.vercel.app/blog) |
| Compare with alternatives | [costaffective-mcp.vercel.app/compare/codegraph](https://costaffective-mcp.vercel.app/compare/codegraph) |

</details>

---

## License

MIT

---

<div align="center">
  <sub>Built for developers who believe AI coding tools should be <strong>fast, local, and open</strong>.</sub>
  <br>
  <sub><strong>Save tokens. Buy Coffee.</strong></sub>
</div>
