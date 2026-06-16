<div align="center">

**CostAffective - MCP**

<p align="center">
  <img
    src="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/logo(1).png"
    alt="CostAffective Logo"
    width="500"
  />
</p>

**45.9% fewer tokens · 54.3% fewer exploration loops · 42.1% fewer tool interactions · 100% Local**

[![Website](https://img.shields.io/badge/Website-costaffective--mcp.vercel.app-0066CC)](https://costaffective-mcp.vercel.app)
[![GitHub Stars](https://img.shields.io/github/stars/okyashgajjar/costaffective-mcp?style=social)](https://github.com/okyashgajjar/costaffective-mcp)
[![GitHub Forks](https://img.shields.io/github/forks/okyashgajjar/costaffective-mcp?style=social)](https://github.com/okyashgajjar/costaffective-mcp)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](#installation)
[![Windows](https://img.shields.io/badge/Windows-supported-blue.svg)](#supported-platforms)
[![macOS](https://img.shields.io/badge/macOS-supported-blue.svg)](#supported-platforms)
[![Linux](https://img.shields.io/badge/Linux-supported-blue.svg)](#supported-platforms)
[![Claude Code](https://img.shields.io/badge/Claude_Code-supported-blueviolet.svg)](#supported-clients--config)
[![Cursor](https://img.shields.io/badge/Cursor-supported-blueviolet.svg)](#supported-clients--config)
[![OpenCode](https://img.shields.io/badge/OpenCode-supported-blueviolet.svg)](#supported-clients--config)
[![Codex CLI](https://img.shields.io/badge/Codex_CLI-supported-blueviolet.svg)](#supported-clients--config)
[![Antigravity](https://img.shields.io/badge/Antigravity-supported-blueviolet.svg)](#supported-clients--config)

---

**INSTALLATION** [ Linux & macOS & Windows ]
```
curl -fsSL https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/install.sh | bash
```
Detects OS, installs Go if missing, builds from source, installs globally, and configures your AI coding clients. No manual steps needed.

**Star this repo** — it helps others find CostAffective.

**Learn more at [costaffective-mcp.vercel.app](https://costaffective-mcp.vercel.app)** — interactive benchmarks, docs, AST sandbox, and editor configurator.

</div>

---

## The Story: Why CostAffective Exists

Every section below states not just *what* a feature does, but *why* it exists. If you only read one part, read this one — it is the reasoning the rest of the project is built on.

### The problem

An AI coding assistant working in a real repository spends most of its budget on two things, and neither of them is "thinking":

1. **Rediscovery.** The model reads the same files over and over to answer questions it has effectively already answered ("where is this defined", "who calls this", "what does this module do"). Each read pushes thousands of tokens into the context window.

2. **The prompt cache.** Providers cache the conversation so repeated context is cheaper to resend. But the cache is not free:
   - Every turn pays to **read** the entire resident context (everything currently in the window).
   - Any change to earlier context, or a short idle gap, invalidates the cache and forces a full **rewrite** of everything resident.

In a long session this compounds. A real measured example: a single API call was billed at **\$2.95**, of which **\$2.84 was the cache *write*** of roughly **455,000 tokens** of resident context. The model's actual output that turn was under 4,000 tokens. The expensive part was not the answer — it was the size of the context being carried and re-cached.

### The insight

A tool that connects to an editor over MCP **cannot control how or when the client caches** — cache breakpoints and time-to-live are decided by the client, not the server. There is exactly one lever a server *does* control:

> **How many tokens ever enter the resident context window in the first place.**

Shrink that, and both costs fall at the same time: a smaller window is cheaper to read every turn *and* cheaper to rewrite when the cache is invalidated. Every design decision in CostAffective serves this one goal — **keep tokens out of the window without losing information.**

### The approach

CostAffective is a local MCP server that does three things in service of that goal:

1. **Answer from a local index instead of from raw files.** It parses your repository once with Tree-sitter, stores symbols, references, and call edges in a local SQLite index, and answers navigation questions from that index in a few tokens instead of by dumping files. (Original feature set.)

2. **Give the model explicit tools to keep large content *out* of the window.** Stash large output to disk behind a tiny handle, recall only the slice that a query needs, and remember durable facts so they are not repeated every turn. (Added in V2.)

3. **Make the model actually use all of the above, automatically.** A small session-awareness skill, delivered to every connected editor, teaches the model the lean workflow once per session so you do not have to ask for it. (Added in V2.)

**Everything runs locally.** No API keys. No cloud indexing. No code leaves your machine.

---

## What is CostAffective?

CostAffective is a local MCP server that helps AI coding assistants understand large repositories without repeatedly exploring the same code, and without carrying large blobs of context they do not need.

Instead of sending large amounts of code into the model context, it builds a local repository index and provides fast, token-cheap access to:

* Symbol definitions
* References
* Call relationships
* Repository summaries (token-budgeted)
* Architecture information
* Semantic code search
* A durable per-repository memory and an out-of-context stash (V2)

---

## Works With

| Client                 | Supported |
| ---------------------- | --------- |
| Claude Code            | Yes       |
| Cursor                 | Yes       |
| OpenCode               | Yes       |
| Codex CLI              | Yes       |
| Antigravity            | Yes       |
| MCP-Compatible Clients | Yes       |

---

## Repository Intelligence

CostAffective combines multiple repository analysis techniques. The reasoning is the same throughout: each technique exists to answer a question precisely, so the model never has to read a whole file to find a small fact.

* **Tree-sitter AST parsing** — understands code structurally, not as text, so a "definition" is an actual definition and not a string match.
* **Symbol indexing** — "where is X defined" becomes a single index lookup returning a location, not a file dump.
* **Reference indexing** — "where is X used" is precomputed instead of grepped live.
* **Call graph analysis** — "what calls X" is answered from stored call edges.
* **Architecture extraction** — module and layer structure without reading every file.
* **Repository summaries** — a budgeted, high-level map of the repo (see the V2 changes below).
* **Context compression** — results are returned as compressed scopes sized to the kind of question asked.
* **Incremental indexing** — only changed files are reprocessed, so the index stays fresh cheaply.

This provides significantly better repository understanding than simple text search, at a fraction of the tokens.

---

## Automatic Incremental Indexing

**Why it exists:** a stale index is useless and a full rebuild on every change is too slow to keep current. CostAffective watches the repository and updates only what changed, so the index is always fresh without ever paying for a full rebuild during a session.

When files change:

```text
File Change
     ↓
Watchdog
     ↓
Hash Comparison
     ↓
Re-index Changed Files Only
     ↓
Cache Invalidation
```

No full repository rebuild is required. Only modified files are reprocessed. This keeps indexing fast even on large repositories.

---

<details>
<summary><strong>MCP Tools — full catalog and the reasoning behind each</strong></summary>

<br>

> Full interactive tool catalog with schemas and examples: [costaffective-mcp.vercel.app/tools](https://costaffective-mcp.vercel.app/tools)

The tools fall into three groups. The first five are **retrieval** — answer a question in a few tokens instead of by reading files. The next two are **maintenance**. The last three (V2) are **context control** — keep large content and durable facts out of the window entirely.

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

*Why:* this is usually the first call of a session, so it is also the first thing that lands in the cached context and stays there. The earlier version emitted one line per directory plus a full per-directory chain with no limit — on a large monorepo that was tens of thousands of tokens, cached for the entire session. It is now hard-capped: the output stays small no matter how large the repository is, and details are pulled on demand via `module`. (See the V2 changes section for the before/after.)

#### index_repository

Refresh or rebuild repository indexes manually. Usually unnecessary because the watchdog re-indexes automatically.

### Context-control tools (V2)

These exist because of the cache cost described in the story. They let the model keep large content and durable facts *out* of the resident window, losslessly.

#### remember

Persist a small durable fact — a decision, an entrypoint, a gotcha — to a per-repository store, so it does not have to be repeated inline in the conversation every time it is relevant.

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

<details>
<summary><strong>The costaffective-session skill — making the model use all of this automatically</strong></summary>

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

This section ties the pieces together. In long sessions the dominant cost is usually the **prompt cache** re-reading and re-writing everything resident in the context window every turn — not the model's output. CostAffective reduces this by keeping tokens out of the window:

* **Answer from the index, not from files** — the retrieval tools return scopes and locations measured in tens of tokens, instead of whole files measured in thousands.
* **Budgeted summaries** — `get_repository_summary` is hard-capped and supports drill-down via `module`, so it never dumps a giant tree into the cached context at session start.
* **Stash instead of paste** — `stash_context` moves large output out of the window and returns a tiny handle; `recall` brings back only the matching slice. This is lossless: the full content stays on disk.
* **Remember instead of repeat** — `remember` persists durable facts per repository; `recall` brings them back without re-deriving or re-pasting them.
* **The session skill** — makes the model do all of the above by default, in every editor.

Why not just summarize or delete old context? Because that loses information. Stashing **relocates** tokens rather than discarding them, so nothing is dropped — you can always recall the full content. That was a hard design constraint: reduce the window without ever losing context.

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
<summary><strong>Benchmarks</strong></summary>

<br>

> Full benchmark suite with the global retriever leaderboard: [costaffective-mcp.vercel.app/benchmarks](https://costaffective-mcp.vercel.app/benchmarks)

### Continue OSS Repository

| Metric            | CostAffective | Alternative |
| ----------------- | ------------- | ----------- |
| Tokens            | 4.7M          | 8.7M        |
| API Calls         | 89            | 134         |
| Exploration Calls | 43            | 94          |

### Improvement

| Metric            | Result          |
| ----------------- | --------------- |
| Token Usage       | **45.9%** lower |
| Exploration Loops | **54.3%** lower |
| Tool Interactions | **42.1%** lower |

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
<summary><strong>Supported Clients & Config</strong></summary>

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

---

## Repository State

**Why it exists:** the model should know whether the index it is querying is trustworthy. CostAffective tracks three states and behaves accordingly.

| State       | Meaning                                  |
| ----------- | ---------------------------------------- |
| `unindexed` | No usable index exists yet               |
| `stale`     | Files changed after the last index       |
| `ready`     | Index is in sync with the working tree   |

Agent mode can auto-index when needed; interactive modes prompt first.

---

## Use Cases

> Explore detailed use case studies: [costaffective-mcp.vercel.app/use-cases](https://costaffective-mcp.vercel.app/use-cases)

* **AI coding agents** — reduce token spend by up to 45.9% with compressed, scope-level lookups, and keep long sessions cheap by parking large content out of context.
* **Large monorepos** — fast SQLite index queries in microseconds instead of disk scans, and budgeted summaries that stay small regardless of repo size.
* **Code reviews** — trace caller hierarchies to audit the impact of incoming changes.
* **Repository audits** — generate summaries of file distribution, language splits, and structure.
* **MCP development** — a reference implementation for the stdio protocol, fsnotify watchers, tree-sitter mapping, and the MCP instructions field.

---

## Doctor

`costaffective doctor` checks:

- Binary existence and permissions
- PATH visibility
- MCP configuration for each client
- Server startup
- Repository state

---

## Supported Platforms

All platforms with Go 1.25+ and a C compiler are supported via the install script (`install.sh`), which handles toolchain setup automatically:

- Linux amd64 / arm64
- macOS amd64 / arm64 (Intel and Apple Silicon)
- Windows amd64

Pre-built release binaries are available for Linux amd64 and Windows amd64. All other platforms are built from source by the install script.

---

## Learn More

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

---

## License

MIT

---

<div align="center">
  <sub>Built for developers who believe AI coding tools should be <strong>fast, local, and open</strong>.</sub>
  <br>
  <sub><strong>Save tokens. Buy Coffee.</strong></sub>
</div>
