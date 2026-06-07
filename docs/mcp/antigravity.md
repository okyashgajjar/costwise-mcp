# CostAffective MCP — Antigravity (Google Gemini IDE)

## Installation

### Auto-install (recommended)

```bash
costaffective install --target antigravity
```

Or let the auto-detector find it:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Antigravity IDE installed (latest version)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

## Configuration

### Via MCP Store UI

1. Open Antigravity
2. Click the **...** (Additional Options) menu in the Agent panel
3. Select **MCP Servers**
4. Click **Manage MCP Servers**
5. Click **View raw config**
6. Add the server entry

### Via config file (recommended)

Edit `~/.gemini/config/mcp_config.json`:

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

### For remote streamable HTTP (not yet available)

```json
{
  "mcpServers": {
    "costaffective": {
      "serverUrl": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer your-token-here"
      }
    }
  }
}
```

> **Note:** Antigravity uses `serverUrl` (not `url`) for remote HTTP-based MCP servers. For local stdio, use `command`/`args`.

### Config file location by OS

| OS | Path |
|----|------|
| Linux | `~/.gemini/config/mcp_config.json` |
| macOS | `~/.gemini/antigravity/mcp_config.json` |
| Windows | `C:\Users\<USERNAME>\.gemini\antigravity\mcp_config.json` |

## Verification

1. Restart Antigravity
2. In the Agent panel, the costaffective server should appear as connected
3. Ask the agent: *"Search this repo for the function CompressForAnswerType"*

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Server shows error | Binary not found in PATH | Use absolute path in `command` |
| Config not loading | Wrong file location | Check ~/.gemini/config/ vs ~/.gemini/antigravity/ |
| "command not recognized" | args formatting | Ensure `args` is a JSON array: `["serve"]` |
| Permission denied on Linux | Binary lacks execute | `chmod +x /path/to/costaffective` |

## Benchmark Setup

```bash
cd /path/to/CostAffective-CLI/CLI
# Run the Go test suite
go test ./... -bench=Benchmark -run=^$
```
