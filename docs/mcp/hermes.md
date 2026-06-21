# CostWise MCP — Hermes Agent

## Installation

### Auto-install (recommended)

Hermes is configured manually. Use `costwise install` for other supported clients:

```bash
costwise install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Hermes Agent installed (`hermes --version`)
- The CostWise repository cloned

### Build the binary

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

## Configuration

Add to `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  costwise:
    command: "costwise"
    args: ["serve"]
```

### With tool filtering (recommended)

```yaml
mcp_servers:
  costwise:
    command: "costwise"
    args: ["serve"]
    tools:
      include:
        - search_code
        - find_symbol
        - read_symbol
        - find_references
        - find_callers
        - get_repository_summary
        - index_repository
        - remember
        - stash_context
        - recall
```

### With environment variables

```yaml
mcp_servers:
  costwise:
    command: "costwise"
    args: ["serve"]
    env:
      MYCLI_LOG_DIR: "/var/log/mycli"
```

## Verification

```bash
# Start Hermes
hermes

# Reload MCP configuration
/reload-mcp

# Check available tools
/tools

# Test by asking
# "Search this repo for the function CompressForAnswerType"
```

Expected: Hermes shows MCP tools prefixed as `mcp_costwise_search_code`, `mcp_costwise_find_symbol`, etc.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Server not loaded | Config syntax error | Validate YAML: `python3 -c "import yaml; yaml.safe_load(open('~/.hermes/config.yaml'))"` |
| Tools not showing | Filter blocking | Remove `tools.include` to allow all tools |
| Command not found | Not in PATH | Use absolute path: `/usr/local/bin/costwise` |

## Benchmark Setup

```bash
cd /path/to/CostWise-CLI/CLI
hermes run "Run the costwise benchmark suite on this project"
```
