# CostWise MCP — OpenCode

## Installation

### Auto-install (recommended)

```bash
costwise install --target opencode
```

Or let the auto-detector find it:

```bash
costwise install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- OpenCode CLI installed (`opencode --version`)
- The CostWise repository cloned

### Build the binary

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

## Configuration

### CLI (recommended)

```bash
opencode mcp add
# Follow the interactive wizard:
# - name: costwise
# - type: local
# - command: costwise serve
```

### Via config file (global)

Edit `~/.config/opencode/opencode.json` or `opencode.json` in your project root:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "costwise": {
      "type": "local",
      "command": ["costwise", "serve"],
      "enabled": true
    }
  }
}
```

### Via config file (project-scoped)

Create `opencode.json` in your project root:

```json
{
  "mcp": {
    "costwise": {
      "type": "local",
      "command": ["costwise", "serve"]
    }
  }
}
```

### With environment variables

```jsonc
{
  "mcp": {
    "costwise": {
      "type": "local",
      "command": ["costwise", "serve"],
      "env": {
        "MYCLI_LOG_DIR": "/var/log/mycli"
      }
    }
  }
}
```

## Verification

```bash
# List connected MCP servers
opencode mcp list

# You should see "costwise" in the list

# Test in a session
opencode run "Search for the function CompressForAnswerType in this repo"
```

Expected: OpenCode loads 7 tools from the `costwise` MCP server.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `costwise: command not found` | Not in PATH | Use absolute path in `command`: `"/usr/local/bin/costwise"` |
| Server not loading | Config syntax error | Validate JSON: `python3 -m json.tool opencode.json` |
| Tools not available in chat | MCP disabled | Ensure `"enabled": true` |
| `error: unknown command` | Wrong command format | OpenCode uses array for `command`, not string |

## Benchmark Setup

```bash
cd /path/to/CostWise-CLI/CLI
opencode run "Run the costwise benchmark suite on this project"
```

Or directly with Go:

```bash
go test ./... -bench=Benchmark -run=^$
```
