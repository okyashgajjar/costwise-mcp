# CostAffective MCP — OpenCode

## Installation

### Auto-install (recommended)

```bash
costaffective install --target opencode
```

Or let the auto-detector find it:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- OpenCode CLI installed (`opencode --version`)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

## Configuration

### CLI (recommended)

```bash
opencode mcp add
# Follow the interactive wizard:
# - name: costaffective
# - type: local
# - command: costaffective serve
```

### Via config file (global)

Edit `~/.config/opencode/opencode.json` or `opencode.json` in your project root:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "costaffective": {
      "type": "local",
      "command": ["costaffective", "serve"],
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
    "costaffective": {
      "type": "local",
      "command": ["costaffective", "serve"]
    }
  }
}
```

### With environment variables

```jsonc
{
  "mcp": {
    "costaffective": {
      "type": "local",
      "command": ["costaffective", "serve"],
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

# You should see "costaffective" in the list

# Test in a session
opencode run "Search for the function CompressForAnswerType in this repo"
```

Expected: OpenCode loads 7 tools from the `costaffective` MCP server.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `costaffective: command not found` | Not in PATH | Use absolute path in `command`: `"/usr/local/bin/costaffective"` |
| Server not loading | Config syntax error | Validate JSON: `python3 -m json.tool opencode.json` |
| Tools not available in chat | MCP disabled | Ensure `"enabled": true` |
| `error: unknown command` | Wrong command format | OpenCode uses array for `command`, not string |

## Benchmark Setup

```bash
cd /path/to/CostAffective-CLI/CLI
opencode run "Run the costaffective benchmark suite on this project"
```

Or directly with Go:

```bash
go test ./... -bench=Benchmark -run=^$
```
