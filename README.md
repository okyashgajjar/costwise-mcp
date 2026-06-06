<div align="center">

# CostAffective

### Local repository intelligence for Claude Code, Cursor, OpenCode, Codex CLI, and Antigravity

**100% local · no cloud · no code uploads**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](#installation)
[![Windows](https://img.shields.io/badge/Windows-supported-blue.svg)](#supported-platforms)
[![macOS](https://img.shields.io/badge/macOS-supported-blue.svg)](#supported-platforms)
[![Linux](https://img.shields.io/badge/Linux-supported-blue.svg)](#supported-platforms)

[![Claude Code](https://img.shields.io/badge/Claude_Code-supported-blueviolet.svg)](#supported-clients)
[![Cursor](https://img.shields.io/badge/Cursor-supported-blueviolet.svg)](#supported-clients)
[![OpenCode](https://img.shields.io/badge/OpenCode-supported-blueviolet.svg)](#supported-clients)
[![Codex CLI](https://img.shields.io/badge/Codex_CLI-supported-blueviolet.svg)](#supported-clients)
[![Antigravity](https://img.shields.io/badge/Antigravity-supported-blueviolet.svg)](#supported-clients)

</div>

## What It Is

CostAffective is a local MCP server that helps AI coding assistants understand large codebases without sending your code anywhere.

It provides:

- repository-aware retrieval
- symbol and reference lookup
- caller discovery
- repository summaries
- fast re-indexing
- client installation and diagnostics

No API key is required.

## Installation

### macOS / Linux / Windows with Go installed

```bash
go install github.com/okyashgajjar/costaffective-mcp@latest
```

This works on all three platforms. After installation, open a new terminal if `costaffective` is not yet on your `PATH`.

### Build from source

```bash
# macOS / Linux
go build -o costaffective ./cmd/mycli/

# Windows
go build -o costaffective.exe ./cmd/mycli/
```

If you build on Windows, run the binary as `.\costaffective.exe`.

### Connect your clients

```bash
costaffective install --all
```

On Windows, use:

```powershell
.\costaffective.exe install --all
```

The installer detects supported clients and writes the correct MCP config for each one.

### macOS / Linux shell installer

```bash
bash install-mcp.sh
```

## Supported Platforms

- Windows
- macOS
- Linux

## Supported Clients

| Client | Config Surface |
|--------|----------------|
| Claude Code | `~/.claude.json`, `.mcp.json`, or settings files |
| Cursor | `~/.cursor/mcp.json` or workspace MCP settings |
| OpenCode | `opencode.json` |
| Codex CLI | `~/.codex/config.toml` |
| Antigravity | `~/.gemini/config/mcp_config.json` |
| MCP-compatible clients | stdio transport |

## MCP Tools

| Tool | What it does |
|------|--------------|
| `search_code` | Semantic code search backed by tree-sitter parsing |
| `find_symbol` | Find where a symbol is defined |
| `find_references` | Find every use of a symbol |
| `find_callers` | Find functions that call a target function |
| `grep_code` | Regex and text search fallback |
| `get_repository_summary` | Summarize modules, files, languages, and architecture |
| `index_repository` | Rebuild or refresh the repository index |

## Commands

| Command | Description |
|---------|-------------|
| `costaffective install` | Interactive installation |
| `costaffective install --all` | Configure every detected client |
| `costaffective install --target <name>` | Configure one client only |
| `costaffective install --repair` | Repair the binary and MCP configuration |
| `costaffective doctor` | Validate installation and startup |
| `costaffective uninstall` | Remove MCP configs from clients |
| `costaffective serve` | Start the MCP stdio server |

## Quick Start

```bash
# Install the binary
go install github.com/okyashgajjar/costaffective-mcp@latest

# Connect the supported clients
costaffective install --all

# Check the install
costaffective doctor
```

## Doctor

`costaffective doctor` checks:

- binary existence and permissions
- PATH visibility
- MCP configuration for each client
- server startup
- repository state

## Repository State

CostAffective keeps track of the repository index and the working tree:

- `unindexed` means no usable index exists yet
- `stale` means files changed after indexing
- `ready` means the repository is aligned with the index

Agent mode can auto-index when needed; other modes can prompt first.

## Architecture

```text
AI Client (MCP Host)
    │
    ├── stdio transport ──► costaffective serve (MCP Server)
    │                           │
    │                           ├── search_code ───────────► tree-sitter AST match
    │                           ├── find_symbol ───────────► SymbolDB lookup
    │                           ├── find_references ───────► SymbolDB reference search
    │                           ├── find_callers ──────────► SymbolDB call graph
    │                           ├── grep_code ─────────────► full-text search fallback
    │                           ├── get_repository_summary ► KnowledgeStore
    │                           └── index_repository ──────► SharedIndexer
```

## Why CostAffective

Modern coding agents waste context by repeatedly rediscovering the same code paths.

CostAffective keeps the repository index local and gives the model smaller, more relevant context so it can spend tokens on reasoning instead of discovery.

## Benchmark Highlights

### Continue OSS Repository

| Metric | Value |
|--------|-------|
| Files | 3,203 |
| Source Files | 1,985 |

### CostAffective

| Metric | Value |
|--------|-------|
| Tokens | 4.7M |
| API Calls | 89 |
| Exploration Calls | 43 |

### Alternative Semantic Code Intelligence Benchmark

| Metric | Value |
|--------|-------|
| Tokens | 8.7M |
| API Calls | 134 |
| Exploration Calls | 94 |

### Observed Results

| Metric | Improvement |
|--------|-------------|
| Token Usage | 45.9% lower |
| Exploration Loops | 54.3% lower |
| Tool Interactions | 42.1% lower |

## Development

```bash
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
go build ./...
go test ./...
```

## License

MIT
