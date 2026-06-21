# MCP Installation Guides

This directory contains installation guides for the CostWise MCP server across all supported AI coding clients.

## What is CostWise MCP?

The CostWise MCP server (`costwise serve`) provides 7 retrieval and maintenance tools via the [Model Context Protocol](https://modelcontextprotocol.io), plus 3 context-control tools (see the main README for the full catalog):

| Tool | Description |
|------|-------------|
| `search_code` | Semantic code search with tree-sitter AST parsing |
| `find_symbol` | Find symbol definitions across the codebase |
| `read_symbol` | Return a symbol's full implementation body by name |
| `find_references` | Find all references to a symbol |
| `find_callers` | Find functions that call a given function |
| `get_repository_summary` | High-level repo overview (modules, files, languages) |
| `index_repository` | Trigger re-indexing of the codebase |

No API key is required — this is a pure retrieval server with no LLM dependency.

## Supported Clients

| Client | Guide | Config Format |
|--------|-------|---------------|
| [Claude Code](claude-code.md) | `~/.claude.json` | `mcpServers` JSON |
| [Cursor](cursor.md) | `.cursor/mcp.json` | `mcpServers` JSON |
| [OpenCode](opencode.md) | `opencode.json` | `mcp` JSON |
| [Codex CLI](codex.md) | `~/.codex/config.toml` | `mcp_servers` TOML |
| [OpenClaw](openclaw.md) | `~/.openclaw/openclaw.json` | `mcpServers` JSON |
| [Hermes Agent](hermes.md) | `~/.hermes/config.yaml` | `mcp_servers` YAML |
| [Antigravity](antigravity.md) | `~/.gemini/config/mcp_config.json` | `mcpServers` JSON |

## Quick Install

### Via CostWise (recommended)

The `costwise install` command auto-detects installed AI coding clients and writes the correct MCP config for each:

```bash
cd /path/to/CostWise-CLI/CLI
go run ./cmd/costwise/ install
```

Options:

| Flag | Description |
|------|-------------|
| `--all` | Configure all supported clients (skip detection) |
| `--target <id>` | Configure only one client (`claude`, `cursor`, `opencode`, `codex`, `antigravity`) |
| `--local` | Configure for current project only (instead of global) |
| `--dry-run` | Show what would be done without making changes |
| `--build=false` | Skip binary build (if already built) |

### Via install script (recommended for first-time setup)

```bash
bash install.sh
```

## Build From Source

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

## Verification

After installing, open your AI coding client and ask:

> *"Search this repo for the function CompressForAnswerType"*

The client should automatically invoke the appropriate MCP tools.

## Architecture

```
AI Client (MCP Host)
    │
    ├── stdio transport ──► costwise serve (MCP Server)
    │                           │
    │                           ├── search_code ───────────► tree-sitter AST match
    │                           ├── find_symbol ───────────► SymbolDB lookup
    │                           ├── find_references ───────► SymbolDB reference search
    │                           ├── find_callers ──────────► SymbolDB call graph
    │                           ├── get_repository_summary ► KnowledgeStore
    │                           └── index_repository ──────► SharedIndexer
```

## Troubleshooting

1. **Binary not found**: Ensure `costwise` is in your PATH or use an absolute path
2. **Server won't start**: Run `costwise serve` directly to see error output
3. **Tools not appearing**: Restart your IDE/CLI client after making config changes
4. **Config syntax errors**: Validate JSON with `python3 -m json.tool`
