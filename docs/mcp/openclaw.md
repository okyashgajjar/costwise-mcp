# CostAffective MCP — OpenClaw

## Installation

### Auto-install (recommended)

OpenClaw is configured manually. Use `costaffective install` for other supported clients:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- OpenClaw CLI installed (`openclaw --version`)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

## Configuration

### CLI

```bash
openclaw mcp add costaffective \
  --command costaffective \
  --args '["serve"]'
```

### Via config file

Add to `~/.openclaw/openclaw.json`:

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

### Via McPorter (if installed)

```bash
mcporter install costaffective --target openclaw \
  --command costaffective --args '["serve"]'
```

## Verification

```bash
# List MCP servers
openclaw mcp list

# Check detailed status
openclaw mcp status --verbose

# Probe server
openclaw mcp probe costaffective

# Run diagnostic
openclaw mcp doctor costaffective --probe
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Server shows "disconnected" | Binary not found | Verify `which costaffective` or use absolute path |
| Tools not discovered | Config not reloaded | Run `openclaw mcp reload` |
| Permission error | Config ownership | Ensure `~/.openclaw/openclaw.json` is readable |
| Duplicate entry | Multiple config sources | Check both global and project config for duplicates |

## Benchmark Setup

```bash
cd /path/to/CostAffective-CLI/CLI
openclaw run "Run the costaffective benchmark on this repo and report results"
```
