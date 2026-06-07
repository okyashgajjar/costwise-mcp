# CostAffective MCP — Claude Code

## Installation

### Auto-install (recommended)

```bash
costaffective install --target claude
```

Or let the auto-detector find it:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Claude Code CLI installed (`claude --version`)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

Or keep it in-project:

```bash
go build -o costaffective ./cmd/costaffective/
```

## Configuration

### CLI (recommended)

```bash
claude mcp add costaffective -- costaffective serve
```

### Manual (global — applies to all projects)

Add to `~/.claude.json`:

```json
{
  "mcpServers": {
    "costaffective": {
      "command": "costaffective",
      "args": ["serve"]
    }
  }
}
```

### Manual (project-local)

Create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "costaffective": {
      "command": "costaffective",
      "args": ["serve"]
    }
  }
}
```

### Alternative locations

Claude Code reads MCP config from these locations (highest priority first):

| Path | Scope |
|------|-------|
| `.claude/settings.local.json` | Project-local (not committed) |
| `.claude/settings.json` | Project-shared |
| `~/.claude/settings.local.json` | User-local |
| `~/.claude/settings.json` | User-global |
| `~/.claude/mcp_servers.json` | Dedicated MCP file |
| `.mcp.json` | Project-local |
| `~/.claude.json` | User-global |

## Verification

```bash
# List connected MCP servers
claude mcp list

# You should see "costaffective" with status "connected"

# Test a tool
claude run --tools "Search for the function CompressForAnswerType in this repo"
```

Expected output shows the server connected with 7 tools:
- `search_code`
- `find_symbol`
- `find_references`
- `find_callers`
- `grep_code`
- `get_repository_summary`
- `index_repository`

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `costaffective: command not found` | Binary not in PATH | Run `which costaffective`; add to PATH or use absolute path |
| Server shows "disconnected" | Binary crashed or missing | Run `costaffective serve` directly to see errors |
| Tools not appearing | Config merge issue | Check `~/.claude.json` syntax with `python3 -m json.tool` |
| Permission denied | Binary lacks execute bit | `chmod +x $(which costaffective)` |

## Benchmark Setup

```bash
# Run the full benchmark suite on the current repo
claude run --tools "Run the costaffective benchmark suite on this repo"

# Or manually evaluate a specific retriever
claude run --tools "Evaluate the treesitter retriever for definition queries"
```
