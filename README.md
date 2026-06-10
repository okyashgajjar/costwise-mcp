<div align="center">

<p align="center">
  <img
    src="https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/logo(1).png"
    alt="CostAffective Logo"
    width="500"
  />
</p>

**45.9% fewer tokens · 54.3% fewer exploration loops · 42.1% fewer tool interactions · 100% Local** <br>

[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](#installation)
[![Windows](https://img.shields.io/badge/Windows-supported-blue.svg)](#supported-platforms)
[![macOS](https://img.shields.io/badge/macOS-temporarily_unavailable-lightgrey.svg)](#supported-platforms)
[![Linux](https://img.shields.io/badge/Linux-supported-blue.svg)](#supported-platforms)
[![Claude Code](https://img.shields.io/badge/Claude_Code-supported-blueviolet.svg)](#supported-clients)
[![Cursor](https://img.shields.io/badge/Cursor-supported-blueviolet.svg)](#supported-clients)
[![OpenCode](https://img.shields.io/badge/OpenCode-supported-blueviolet.svg)](#supported-clients)
[![Codex CLI](https://img.shields.io/badge/Codex_CLI-supported-blueviolet.svg)](#supported-clients)
[![Antigravity](https://img.shields.io/badge/Antigravity-supported-blueviolet.svg)](#supported-clients)

---

**INSTALLATION**
```
curl -fsSL https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/install.sh | bash
```

** 📦Download ** 
[![Linux (amd64)](https://github.com/okyashgajjar/costaffective-mcp/releases)]
[![Windows (amd64)](https://github.com/okyashgajjar/costaffective-mcp/releases)]

Latest binaries are available on the Releases page.
** Scroll Down for MacOS installation**

⭐ **Star this repo** — it helps others find CostAffective and keeps us motivated!

</div>

## What is CostAffective?

CostAffective is a local MCP server that helps AI coding assistants understand large repositories without repeatedly exploring the same code.

Instead of sending large amounts of code into the model context, CostAffective builds a local repository index and provides fast access to:

* Symbol definitions
* References
* Call relationships
* Repository summaries
* Architecture information
* Semantic code search

**Everything runs locally.** No API keys. No cloud indexing. No code leaves your machine.

---

## Why CostAffective?

Modern coding assistants often spend a large portion of their context window rediscovering code that already exists.

CostAffective reduces this overhead by providing repository-aware retrieval.

### Benefits

* **Faster** repository understanding
* **Smaller** context usage
* **Fewer** tool calls
* **Better** navigation of large codebases
* **Local-first** architecture — your code never leaves your machine
* **Automatic** repository updates — no manual re-indexing

---

## Works With

| Client                 | Supported |
| ---------------------- | --------- |
| Claude Code            | ✅        |
| Cursor                 | ✅        |
| OpenCode               | ✅        |
| Codex CLI              | ✅        |
| Antigravity            | ✅        |
| MCP-Compatible Clients | ✅        |
---

## Repository Intelligence

CostAffective combines multiple repository analysis techniques:

* Tree-sitter AST parsing
* Symbol indexing
* Reference indexing
* Call graph analysis
* Architecture extraction
* Repository summaries
* Context compression
* Incremental indexing

This provides significantly better repository understanding than simple text search.

---

## Automatic Incremental Indexing

CostAffective continuously watches your repository and updates its index automatically.

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
<summary><strong>📦 MCP Tools</strong> — click to expand</summary>

### search_code

Semantic repository search powered by Tree-sitter.

> **Example:** `Where is caching implemented?`

### find_symbol

Find where a symbol is defined.

> **Example:** `Find UserService`

### find_references

Find every usage of a symbol.

> **Example:** `Where is UserService used?`

### find_callers

Find which functions call another function.

> **Example:** `What calls processPayment()?`

### grep_code

Regex and full-text fallback search.

### get_repository_summary

Generate repository-level summaries. Includes languages, modules, architecture, and key symbols.

### index_repository

Refresh or rebuild repository indexes.

</details>

<details>
<summary><strong>🏗️ Architecture</strong> — click to expand</summary>

```text
AI Client (MCP Host)
    │
    ├── stdio transport ──► costaffective serve (MCP Server)
    │                           │
    │                           ├── Session Manager
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
    │                           ├── find_references ───────► SymbolDB reference search
    │                           ├── find_callers ──────────► SymbolDB call graph
    │                           ├── grep_code ─────────────► full-text search fallback
    │                           ├── get_repository_summary ► KnowledgeStore
    │                           └── index_repository ──────► SharedIndexer
```

</details>

<details>
<summary><strong>⚙️ Commands</strong> — click to expand</summary>

| Command                        | Description                    |
| ------------------------------ | ------------------------------ |
| `costaffective install`        | Interactive installation       |
| `costaffective install --all`  | Configure all detected clients |
| `costaffective install --target` | Configure a specific client  |
| `costaffective install --build` | Build from source before installing |
| `costaffective install --repair` | Repair installation          |
| `costaffective doctor`         | Validate installation          |
| `costaffective serve`          | Start MCP server               |
| `costaffective uninstall`      | Remove MCP configuration       |

</details>

<details>
<summary><strong>💻 Installation</strong> — click to expand</summary>

### Windows Installation

Current recommended installation:

1. Download `costaffective_windows_amd64.zip` from GitHub Releases.
2. Extract `costaffective.exe`.
3. Add this directory to `PATH`:

```powershell
C:\Users\<user>\AppData\Local\Programs\CostAffective\
```

Verify:

```powershell
costaffective --version
```

### macOS

```bash
brew install go sqlite
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
go install ./cmd/costaffective
costaffective --version
```

### Linux

Download `costaffective_linux_amd64.zip` from GitHub Releases.

#### Ubuntu/Debian

```bash
unzip costaffective_linux_amd64.zip
chmod +x costaffective
sudo mv costaffective /usr/local/bin/
costaffective --version
```

#### Arch

```bash
unzip costaffective_linux_amd64.zip
chmod +x costaffective
sudo install -m755 costaffective /usr/local/bin/
costaffective doctor
```

#### Fedora

```bash
unzip costaffective_linux_amd64.zip
chmod +x costaffective
sudo mv costaffective /usr/local/bin/
costaffective --version
```

#### openSUSE

```bash
unzip costaffective_linux_amd64.zip
chmod +x costaffective
sudo mv costaffective /usr/local/bin/
costaffective --version
```

</details>

<details>
<summary><strong>🗑️ Uninstall</strong> — click to expand</summary>

1. Remove MCP client configs:

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
<summary><strong>📊 Benchmarks</strong> — click to expand</summary>

### Continue OSS Repository

| Metric            | CostAffective | Alternative   |
| ----------------- | ------------- | ------------- |
| Tokens            | 4.7M          | 8.7M          |
| API Calls         | 89            | 134           |
| Exploration Calls | 43            | 94            |

### Improvement

| Metric            | Result        |
| ----------------- | ------------- |
| Token Usage       | **45.9%** lower |
| Exploration Loops | **54.3%** lower |
| Tool Interactions | **42.1%** lower |

</details>

<details>
<summary><strong>🛠️ Development</strong> — click to expand</summary>

```bash
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
go build ./...
go test ./...
```

</details>

<details>
<summary><strong>📋 Supported Clients & Config</strong> — click to expand</summary>

| Client       | Config Surface                                      |
| ------------ | --------------------------------------------------- |
| Claude Code  | `~/.claude.json`, `.mcp.json`, or settings files    |
| Cursor       | `~/.cursor/mcp.json` or workspace MCP settings      |
| OpenCode     | `opencode.json`                                     |
| Codex CLI    | `~/.codex/config.toml`                              |
| Antigravity  | `~/.gemini/config/mcp_config.json`                  |
| MCP-compatible | stdio transport                                   |

</details>

---

## Repository State

CostAffective tracks three states for your repository index:

| State        | Meaning                                        |
| ------------ | ---------------------------------------------- |
| `unindexed`  | No usable index exists yet                     |
| `stale`      | Files changed after the last index             |
| `ready`      | Index is in sync with the working tree         |

Agent mode can auto-index when needed; interactive modes prompt first.

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

- Linux amd64
- Windows amd64

Linux arm64 and macOS release binaries are temporarily unavailable for v1.0.1 and will return in a future release. Users on those platforms can still build from source with a local Go and C toolchain.

---

## License

MIT

---

<div align="center">
  <sub>Built with ❤️ for developers who believe AI coding tools should be <strong>fast, local, and open</strong>.</sub>
  <br>
  <sub>**Save tokens. Buy Coffee.** ☕</sub>
</div>
