# CostWise MCP — OpenClaw

## Installation

### Auto-install (recommended)

OpenClaw is configured manually. Use `costwise install` for other supported clients:

```bash
costwise install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- OpenClaw CLI installed (`openclaw --version`)
- The CostWise repository cloned

### Build the binary

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

## Configuration

### CLI

```bash
openclaw mcp add costwise \
  --command costwise \
  --args '["serve"]'
```

### Via config file

Add to `~/.openclaw/openclaw.json`:

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

### Via McPorter (if installed)

```bash
mcporter install costwise --target openclaw \
  --command costwise --args '["serve"]'
```

## Verification

```bash
# List MCP servers
openclaw mcp list

# Check detailed status
openclaw mcp status --verbose

# Probe server
openclaw mcp probe costwise

# Run diagnostic
openclaw mcp doctor costwise --probe
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Server shows "disconnected" | Binary not found | Verify `which costwise` or use absolute path |
| Tools not discovered | Config not reloaded | Run `openclaw mcp reload` |
| Permission error | Config ownership | Ensure `~/.openclaw/openclaw.json` is readable |
| Duplicate entry | Multiple config sources | Check both global and project config for duplicates |

## Benchmark Setup

```bash
cd /path/to/CostWise-CLI/CLI
openclaw run "Run the costwise benchmark on this repo and report results"
```
