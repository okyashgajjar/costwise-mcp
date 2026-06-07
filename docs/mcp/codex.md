# CostAffective MCP — Codex CLI

## Installation

### Auto-install (recommended)

```bash
costaffective install --target codex
```

Or let the auto-detector find it:

```bash
costaffective install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Codex CLI installed and authenticated (`codex --version`)
- The CostAffective repository cloned

### Build the binary

```bash
cd /path/to/CostAffective-CLI/CLI
go build -o /usr/local/bin/costaffective ./cmd/costaffective/
```

## Configuration

### CLI (experimental)

```bash
codex mcp add costaffective -- costaffective serve
```

For environment variables:

```bash
codex mcp add costaffective --env MYCLI_LOG_DIR=/var/log -- costaffective serve
```

### Via config.toml (recommended)

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.costaffective]
command = "costaffective"
args = ["serve"]
```

Or with environment variables:

```toml
[mcp_servers.costaffective]
command = "costaffective"
args = ["serve"]

[mcp_servers.costaffective.env]
MYCLI_LOG_DIR = "/var/log/mycli"
```

### Project-scoped config

Create `.codex/config.toml` in your project root (trusted projects only):

```toml
[mcp_servers.costaffective]
command = "costaffective"
args = ["serve"]
```

## Verification

```bash
# Start a Codex session
codex

# Check active MCP servers
/mcp

# Test by asking
# "Search this repo for the function CompressForAnswerType"
```

Expected: Codex shows the `costaffective` server with 7 tools loaded.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `costaffective: command not found` | Not in PATH | Use absolute path: `"/usr/local/bin/costaffective"` |
| TOML parsing error | Invalid syntax | Validate with `python3 -c "import tomllib; tomllib.load(open('~/.codex/config.toml'))"` |
| Server not starting | Args format wrong | Ensure `args` is an array: `args = ["serve"]` |
| Config not found | Wrong file location | Config at `~/.codex/config.toml` by default |

## Benchmark Setup

```bash
cd /path/to/CostAffective-CLI/CLI

# Run Go benchmarks
go test ./... -bench=Benchmark -run=^$

# Or use Codex
codex run "Run the costaffective benchmark suite on this project"
```
