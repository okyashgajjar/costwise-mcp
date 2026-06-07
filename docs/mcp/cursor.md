# CostAffective MCP — Cursor

## Installation

### Auto-install (recommended)

```bash
costaffective install --target cursor
```

Or let the auto-detector find it:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Cursor IDE installed (latest version)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

Or keep it local:

```bash
go build -o costaffective ./cmd/costaffective/
```

## Configuration

### Via Settings UI (easiest)

1. Open Cursor Settings (`Cmd+,` on macOS, `Ctrl+,` on Windows/Linux)
2. Navigate to **Tools & MCP** (or **Features → Model Context Protocol**)
3. Click **+ New MCP Server**
4. Fill in:
   - **Name**: `costaffective`
   - **Transport Type**: `stdio`
   - **Command**: `costaffective`
   - **Args**: `serve`
5. Click **Save**

### Via project config (team-shared)

Create `.cursor/mcp.json` in your project root:

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

### Via global config (single user)

Edit `~/.cursor/mcp.json`:

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

### Via Command Palette

1. Press `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Windows/Linux)
2. Type "MCP" and select **MCP: Add Server**
3. Follow the prompts with name="costaffective", transport=stdio, command="costaffective serve"

## Verification

- Open **Cursor Settings → Tools & MCP**
- Look for a **green dot** next to "costaffective"
- Open the Cursor chat and ask: *"Search this repo for the function CompressForAnswerType"*

Cursor will call the `costaffective` MCP tools automatically.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Red dot next to server | Binary not found or crashed | Run `which costaffective` to confirm path; try `costaffective serve` in terminal |
| Tools not in chat | MCP not loaded | Restart Cursor entirely after config change |
| Permission error on save | Config file ownership | Check `~/.cursor/mcp.json` permissions |

## Benchmark Setup

```bash
# In Cursor terminal
cd /path/to/CostAffective-CLI/CLI

# Run benchmark
go test ./... -bench=Benchmark -run=^$

# Or use MCP tools directly via Cursor chat:
# "Run the costaffective benchmark on this repo and report accuracy per retriever"
```
