# CostWise MCP — Codex CLI

## Installation

### Auto-install (recommended)

```bash
costwise install --target codex
```

Or let the auto-detector find it:

```bash
costwise install --all
```

### Prerequisites

- Go 1.25+ (`go version`)
- Codex CLI installed and authenticated (`codex --version`)
- The CostWise repository cloned

### Build the binary

```bash
cd /path/to/CostWise-CLI/CLI
go build -o /usr/local/bin/costwise ./cmd/costwise/
```

## Configuration

### CLI (experimental)

```bash
codex mcp add costwise -- costwise serve
```

For environment variables:

```bash
codex mcp add costwise --env MYCLI_LOG_DIR=/var/log -- costwise serve
```

### Via config.toml (recommended)

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.costwise]
command = "costwise"
args = ["serve"]
```

Or with environment variables:

```toml
[mcp_servers.costwise]
command = "costwise"
args = ["serve"]

[mcp_servers.costwise.env]
MYCLI_LOG_DIR = "/var/log/mycli"
```

### Project-scoped config

Create `.codex/config.toml` in your project root (trusted projects only):

```toml
[mcp_servers.costwise]
command = "costwise"
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

Expected: Codex shows the `costwise` server with 7 tools loaded.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `costwise: command not found` | Not in PATH | Use absolute path: `"/usr/local/bin/costwise"` |
| TOML parsing error | Invalid syntax | Validate with `python3 -c "import tomllib; tomllib.load(open('~/.codex/config.toml'))"` |
| Server not starting | Args format wrong | Ensure `args` is an array: `args = ["serve"]` |
| Config not found | Wrong file location | Config at `~/.codex/config.toml` by default |

## Benchmark Setup

```bash
cd /path/to/CostWise-CLI/CLI

# Run Go benchmarks
go test ./... -bench=Benchmark -run=^$

# Or use Codex
codex run "Run the costwise benchmark suite on this project"
```
