# CostWise MCP — Cursor

## Installation

### Auto-install (recommended)

```bash
costwise install --target cursor
```

Or let the auto-detector find it:

```bash
costwise install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Cursor IDE installed (latest version)
- The CostWise repository cloned

### Build the binary

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

Or keep it local:

```bash
go build -o costwise ./cmd/costwise/
```

## Configuration

### Via Settings UI (easiest)

1. Open Cursor Settings (`Cmd+,` on macOS, `Ctrl+,` on Windows/Linux)
2. Navigate to **Tools & MCP** (or **Features → Model Context Protocol**)
3. Click **+ New MCP Server**
4. Fill in:
   - **Name**: `costwise`
   - **Transport Type**: `stdio`
   - **Command**: `costwise`
   - **Args**: `serve`
5. Click **Save**

### Via project config (team-shared)

Create `.cursor/mcp.json` in your project root:

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

### Via global config (single user)

Edit `~/.cursor/mcp.json`:

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

### Via Command Palette

1. Press `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Windows/Linux)
2. Type "MCP" and select **MCP: Add Server**
3. Follow the prompts with name="costwise", transport=stdio, command="costwise serve"

## Verification

- Open **Cursor Settings → Tools & MCP**
- Look for a **green dot** next to "costwise"
- Open the Cursor chat and ask: *"Search this repo for the function CompressForAnswerType"*

Cursor will call the `costwise` MCP tools automatically.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Red dot next to server | Binary not found or crashed | Run `which costwise` to confirm path; try `costwise serve` in terminal |
| Tools not in chat | MCP not loaded | Restart Cursor entirely after config change |
| Permission error on save | Config file ownership | Check `~/.cursor/mcp.json` permissions |

## Benchmark Setup

```bash
# In Cursor terminal
cd /path/to/CostWise-CLI/CLI

# Run benchmark
go test ./... -bench=Benchmark -run=^$

# Or use MCP tools directly via Cursor chat:
# "Run the costwise benchmark on this repo and report accuracy per retriever"
```
