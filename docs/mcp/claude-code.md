# CostWise MCP — Claude Code

## Installation

### Auto-install (recommended)

```bash
costwise install --target claude
```

Or let the auto-detector find it:

```bash
costwise install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Claude Code CLI installed (`claude --version`)
- The CostWise repository cloned

### Build the binary

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

Or keep it in-project:

```bash
go build -o costwise ./cmd/costwise/
```

## Configuration

### CLI (recommended)

```bash
claude mcp add costwise -- costwise serve
```

### Manual (global — applies to all projects)

Add to `~/.claude.json`:

```json
{
  "mcpServers": {
    "costwise": {
      "command": "costwise",
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
    "costwise": {
      "command": "costwise",
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

# You should see "costwise" with status "connected"

# Test a tool
claude run --tools "Search for the function CompressForAnswerType in this repo"
```

Expected output shows the server connected with these tools:
- `search_code`
- `find_symbol`
- `read_symbol`
- `find_references`
- `find_callers`
- `get_repository_summary`
- `index_repository`
- `remember`
- `stash_context`
- `recall`

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `costwise: command not found` | Binary not in PATH | Run `which costwise`; add to PATH or use absolute path |
| Server shows "disconnected" | Binary crashed or missing | Run `costwise serve` directly to see errors |
| Tools not appearing | Config merge issue | Check `~/.claude.json` syntax with `python3 -m json.tool` |
| Permission denied | Binary lacks execute bit | `chmod +x $(which costwise)` |

## Benchmark Setup

```bash
# Run the full benchmark suite on the current repo
claude run --tools "Run the costwise benchmark suite on this repo"

# Or manually evaluate a specific retriever
claude run --tools "Evaluate the treesitter retriever for definition queries"
```
