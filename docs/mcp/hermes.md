# CostAffective MCP — Hermes Agent

## Installation

### Auto-install (recommended)

Hermes is configured manually. Use `costaffective install` for other supported clients:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Hermes Agent installed (`hermes --version`)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

## Configuration

Add to `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  costaffective:
    command: "costaffective"
    args: ["serve"]
```

### With tool filtering (recommended)

```yaml
mcp_servers:
  costaffective:
    command: "costaffective"
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
  costaffective:
    command: "costaffective"
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

Expected: Hermes shows MCP tools prefixed as `mcp_costaffective_search_code`, `mcp_costaffective_find_symbol`, etc.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Server not loaded | Config syntax error | Validate YAML: `python3 -c "import yaml; yaml.safe_load(open('~/.hermes/config.yaml'))"` |
| Tools not showing | Filter blocking | Remove `tools.include` to allow all tools |
| Command not found | Not in PATH | Use absolute path: `/usr/local/bin/costaffective` |

## Benchmark Setup

```bash
cd /path/to/CostAffective-CLI/CLI
hermes run "Run the costaffective benchmark suite on this project"
```
