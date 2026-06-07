#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════
# CostAffective MCP — Universal Install Script
#
# Detects installed AI coding clients and configures the
# CostAffective MCP server for each one automatically.
#
# Usage:
#   bash install-mcp.sh                # interactive (confirm each)
#   bash install-mcp.sh --all          # install for all detected clients
#   bash install-mcp.sh --client codex # install for specific client only
#   bash install-mcp.sh --dry-run      # show what would be done
# ═══════════════════════════════════════════════════════════════

# ── Config ────────────────────────────────────────────────────
MCP_COMMAND="${MCP_COMMAND:-costaffective}"
MCP_ARGS="${MCP_ARGS:-serve}"
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$REPO_DIR"
BINARY_SOURCE="$BUILD_DIR/costaffective"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
DRY_RUN=false
ALL=false
TARGET_CLIENT=""

# ── Colors ────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()   { echo -e "${RED}[ERR]${NC}   $*"; }

# ── Parse Args ────────────────────────────────────────────────
usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  --all          Install for all detected clients (no confirmations)
  --client NAME  Install only for a specific client
  --dry-run      Show what would be done without making changes
  --help         Show this help

Supported clients: claude-code, cursor, opencode, codex, openclaw, hermes, antigravity
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --all) ALL=true; shift ;;
        --client) TARGET_CLIENT="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        --help) usage ;;
        *) err "Unknown option: $1"; usage ;;
    esac
done

# ── Build Binary ──────────────────────────────────────────────
build_binary() {
    echo ""
    info "Building CostAffective MCP server..."
    info "Source directory: $BUILD_DIR"

    if $DRY_RUN; then
        info "[DRY RUN] Would run: go build -o $BINARY_SOURCE ./cmd/costaffective/"
        return
    fi

    (cd "$BUILD_DIR" && go build -o "$BINARY_SOURCE" ./cmd/costaffective/)

    if [[ ! -f "$BINARY_SOURCE" ]]; then
        err "Build failed! Binary not found at $BINARY_SOURCE"
        exit 1
    fi

    ok "Built: $BINARY_SOURCE"

    # Install to PATH
    if [[ "$INSTALL_DIR" != "" ]]; then
        mkdir -p "$INSTALL_DIR"
        cp "$BINARY_SOURCE" "$INSTALL_DIR/costaffective"
        chmod +x "$INSTALL_DIR/costaffective"
        ok "Installed to: $INSTALL_DIR/costaffective"
    fi
}

# ── Client Detection ──────────────────────────────────────────
has_cmd() { command -v "$1" &>/dev/null; }
has_file() { [[ -f "$1" ]]; }

detect_clients() {
    local detected=()

    has_cmd claude && detected+=("claude-code")
    has_cmd cursor && detected+=("cursor")
    has_cmd opencode && detected+=("opencode")
    has_cmd codex && detected+=("codex")
    has_cmd openclaw && detected+=("openclaw")
    has_cmd hermes && detected+=("hermes")
    has_file "$HOME/.gemini/config/mcp_config.json" && detected+=("antigravity")

    echo "${detected[@]}"
}

# ── Config Writers ────────────────────────────────────────────
write_config_claude_code() {
    local config_file="$HOME/.claude.json"
    local entry

    entry=$(cat <<EOF
{
  "mcpServers": {
    "costaffective": {
      "command": "${MCP_COMMAND}",
      "args": ["${MCP_ARGS}"]
    }
  }
}
EOF
)

    if [[ -f "$config_file" ]]; then
        # Merge into existing config
        local tmp
        tmp=$(python3 -c "
import json, sys
with open('$config_file') as f:
    cfg = json.load(f)
cfg.setdefault('mcpServers', {})['costaffective'] = {
    'command': '${MCP_COMMAND}',
    'args': ['${MCP_ARGS}']
}
json.dump(cfg, sys.stdout, indent=2)
")
        echo "$tmp" > "$config_file"
    else
        echo "$entry" > "$config_file"
    fi
    ok "Claude Code: configured at $config_file"
}

write_config_cursor() {
    local config_dir="$HOME/.cursor"
    mkdir -p "$config_dir"
    local config_file="$config_dir/mcp.json"

    cat > "$config_file" <<EOF
{
  "mcpServers": {
    "costaffective": {
      "command": "${MCP_COMMAND}",
      "args": ["${MCP_ARGS}"]
    }
  }
}
EOF
    ok "Cursor: configured at $config_file"
}

write_config_opencode() {
    local config_file="${XDG_CONFIG_HOME:-$HOME/.config}/opencode/opencode.json"
    mkdir -p "$(dirname "$config_file")"

    cat > "$config_file" <<EOF
{
  "mcp": {
    "costaffective": {
      "type": "local",
      "command": ["${MCP_COMMAND}", "${MCP_ARGS}"],
      "enabled": true
    }
  }
}
EOF
    ok "OpenCode: configured at $config_file"
}

write_config_codex() {
    local config_dir="$HOME/.codex"
    mkdir -p "$config_dir"
    local config_file="$config_dir/config.toml"

    # Append or create
    if [[ -f "$config_file" ]]; then
        echo "" >> "$config_file"
    fi
    cat >> "$config_file" <<EOF

[mcp_servers.costaffective]
command = "${MCP_COMMAND}"
args = ["${MCP_ARGS}"]
EOF
    ok "Codex CLI: configured at $config_file"
}

write_config_openclaw() {
    local config_dir="$HOME/.openclaw"
    mkdir -p "$config_dir"
    local config_file="$config_dir/openclaw.json"

    if [[ -f "$config_file" ]]; then
        local tmp
        tmp=$(python3 -c "
import json, sys
with open('$config_file') as f:
    cfg = json.load(f)
cfg.setdefault('mcpServers', {})['costaffective'] = {
    'command': '${MCP_COMMAND}',
    'args': ['${MCP_ARGS}']
}
json.dump(cfg, sys.stdout, indent=2)
")
        echo "$tmp" > "$config_file"
    else
        cat > "$config_file" <<EOF
{
  "mcpServers": {
    "costaffective": {
      "command": "${MCP_COMMAND}",
      "args": ["${MCP_ARGS}"]
    }
  }
}
EOF
    fi
    ok "OpenClaw: configured at $config_file"
}

write_config_hermes() {
    local config_dir="$HOME/.hermes"
    mkdir -p "$config_dir"
    local config_file="$config_dir/config.yaml"

    cat >> "$config_file" <<EOF

mcp_servers:
  costaffective:
    command: "${MCP_COMMAND}"
    args: ["${MCP_ARGS}"]
EOF
    ok "Hermes Agent: configured at $config_file"
}

write_config_antigravity() {
    local config_dir="$HOME/.gemini/config"
    mkdir -p "$config_dir"
    local config_file="$config_dir/mcp_config.json"

    if [[ -f "$config_file" ]]; then
        local tmp
        tmp=$(python3 -c "
import json, sys
with open('$config_file') as f:
    cfg = json.load(f)
cfg.setdefault('mcpServers', {})['costaffective'] = {
    'command': '${MCP_COMMAND}',
    'args': ['${MCP_ARGS}']
}
json.dump(cfg, sys.stdout, indent=2)
")
        echo "$tmp" > "$config_file"
    else
        cat > "$config_file" <<EOF
{
  "mcpServers": {
    "costaffective": {
      "command": "${MCP_COMMAND}",
      "args": ["${MCP_ARGS}"]
    }
  }
}
EOF
    fi
    ok "Antigravity: configured at $config_file"
}

# ── Installation ──────────────────────────────────────────────
install_for_client() {
    local client="$1"

    if $DRY_RUN; then
        info "[DRY RUN] Would configure $client"
        return
    fi

    info "Configuring $client..."
    case "$client" in
        claude-code)  write_config_claude_code ;;
        cursor)       write_config_cursor ;;
        opencode)     write_config_opencode ;;
        codex)        write_config_codex ;;
        openclaw)     write_config_openclaw ;;
        hermes)       write_config_hermes ;;
        antigravity)  write_config_antigravity ;;
        *)            warn "Unknown client: $client" ;;
    esac
}

prompt_client() {
    local client="$1"
    local label=""

    case "$client" in
        claude-code) label="Claude Code (detected)" ;;
        cursor)      label="Cursor (detected)" ;;
        opencode)    label="OpenCode (detected)" ;;
        codex)       label="Codex CLI (detected)" ;;
        openclaw)    label="OpenClaw (detected)" ;;
        hermes)      label="Hermes Agent (detected)" ;;
        antigravity) label="Antigravity (config file found)" ;;
    esac

    echo ""
    read -r -p "Install CostAffective MCP for ${label}? [Y/n] " reply
    case "$reply" in
        [nN]|[nN][oO]) return 1 ;;
        *) return 0 ;;
    esac
}

# ── Main ──────────────────────────────────────────────────────
main() {
    echo "═══════════════════════════════════════════════"
    echo "  CostAffective MCP — Universal Installer"
    echo "═══════════════════════════════════════════════"

    # Build binary
    if ! $DRY_RUN; then
        build_binary
    fi

    # Detect clients
    local detected
    detected=($(detect_clients))

    if [[ ${#detected[@]} -eq 0 ]]; then
        warn "No supported AI coding clients detected."
        info "You can still build the binary and manually configure any client."
        info "See docs/mcp/ for per-client guides."
        exit 0
    fi

    echo ""
    info "Detected clients: ${detected[*]}"

    # Install
    for client in "${detected[@]}"; do
        # Filter by target if specified
        if [[ -n "$TARGET_CLIENT" && "$client" != "$TARGET_CLIENT" ]]; then
            continue
        fi

        if $ALL || prompt_client "$client"; then
            install_for_client "$client"
        else
            info "Skipping $client"
        fi
    done

    echo ""
    if $DRY_RUN; then
        info "Dry run complete. No changes made."
    else
        ok "Installation complete!"

        # Show next steps
        echo ""
        echo "═══════════════════════════════════════════════"
        echo "  Next Steps"
        echo "═══════════════════════════════════════════════"
        echo ""
        echo "  1. Restart your AI coding client(s)"
        echo "  2. Ask: \"Search this repo for the function CompressForAnswerType\""
        echo ""
        echo "  For troubleshooting, see: docs/mcp/README.md"
        echo "═══════════════════════════════════════════════"
    fi
}

main
