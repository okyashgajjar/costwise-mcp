#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
# CostAffective MCP — Unified Installer
#
# Installs Go (if missing), builds CostAffective from source,
# installs globally, and configures your AI coding clients.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/okyashgajjar/costaffective-mcp/main/install.sh | bash
# ═══════════════════════════════════════════════════════════════
set -euo pipefail

REPO="okyashgajjar/costaffective-mcp"
REPO_URL="https://github.com/$REPO.git"
INSTALL_DIR="/usr/local/bin"
GO_VERSION_MIN="1.25"

# ── Colors ────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()   { echo -e "${RED}[ERR]${NC}   $*"; }

echo "═══════════════════════════════════════════════"
echo "  CostAffective MCP — Unified Installer"
echo "═══════════════════════════════════════════════"
echo ""

# ═══════════════════════════════════════════════════════════════
# PHASE 1 — Check & Install Go
# ═══════════════════════════════════════════════════════════════

info "Phase 1: Checking Go toolchain..."

install_go() {
  local os arch version ver_url tar

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) err "Unsupported arch: $arch"; exit 1 ;;
  esac

  # Fetch latest Go version
  version=$(curl -fsSL https://go.dev/VERSION?m=text 2>/dev/null | head -1 || echo "go1.25.0")
  version="${version#go}"
  tar="go${version}.${os}-${arch}.tar.gz"
  url="https://go.dev/dl/$tar"

  info "Go is required to build CostAffective from source."
  info "Latest Go version: $version"
  echo ""
  read -r -p "Install Go ${version} to /usr/local/go? [Y/n] " reply
  case "$reply" in
    [nN]|[nN][oO])
      err "Go is required. Please install Go $GO_VERSION_MIN+ manually, then re-run this script."
      info "  https://go.dev/dl/"
      exit 1
      ;;
  esac

  info "Downloading $tar ..."
  curl -fsSL "$url" -o "/tmp/$tar" || {
    err "Download failed: $url"
    exit 1
  }

  info "Extracting to /usr/local/go ..."
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "/tmp/$tar"
  rm -f "/tmp/$tar"

  # Add to global PATH via profile drop-in
  if ! grep -qs '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
    echo 'export PATH="$PATH:/usr/local/go/bin"' | sudo tee /etc/profile.d/go.sh >/dev/null
    sudo chmod 0644 /etc/profile.d/go.sh
  fi

  export PATH="$PATH:/usr/local/go/bin"
  ok "Go $version installed to /usr/local/go"
}

check_go_version() {
  local ver
  ver=$(go version 2>/dev/null | sed 's/.*go\([0-9]\+\.[0-9]\+\(\.[0-9]\+\)*\).*/\1/')
  if [[ -z "$ver" ]]; then return 1; fi

  local major minor
  major="${ver%%.*}"
  minor="${ver#*.}"; minor="${minor%%.*}"
  local req_major req_minor
  req_major="${GO_VERSION_MIN%%.*}"
  req_minor="${GO_VERSION_MIN#*.}"

  if (( major > req_major )) || (( major == req_major && minor >= req_minor )); then
    return 0
  fi
  return 1
}

if command -v go &>/dev/null && check_go_version; then
  ok "Go $(go version | sed 's/.*go\([0-9.]*\).*/\1/') found"
else
  info "Go not found or below $GO_VERSION_MIN"
  install_go
fi

# ═══════════════════════════════════════════════════════════════
# PHASE 2 — Ensure Go is globally available
# ═══════════════════════════════════════════════════════════════

info "Phase 2: Ensuring Go is globally available..."

GO_BIN=""
for candidate in /usr/local/go/bin/go /usr/lib/go/bin/go /snap/go/current/bin/go; do
  if [[ -x "$candidate" ]]; then
    GO_BIN="$candidate"
    break
  fi
done

if [[ -z "$GO_BIN" ]]; then
  GO_BIN="$(command -v go 2>/dev/null || true)"
fi

if [[ -n "$GO_BIN" ]]; then
  GOROOT_BIN="$(dirname "$GO_BIN")"
  if [[ ":$PATH:" != *":$GOROOT_BIN:"* ]]; then
    export PATH="$GOROOT_BIN:$PATH"
    warn "Go was not in PATH. Adding $GOROOT_BIN for this session."
    info "A global profile entry was already added to /etc/profile.d/go.sh"
  fi
  ok "Go binary: $GO_BIN"
else
  err "Go is not available even after installation. Please fix manually."
  exit 1
fi

# ═══════════════════════════════════════════════════════════════
# PHASE 3 — Check CGO Dependencies
# ═══════════════════════════════════════════════════════════════

info "Phase 3: Checking CGO dependencies..."

if ! command -v gcc &>/dev/null && ! command -v clang &>/dev/null; then
  warn "No C compiler found (CGO requires gcc or clang)"
  case "$(uname -s)" in
    Linux)
      if command -v apt-get &>/dev/null; then
        info "Installing: gcc libsqlite3-dev"
        sudo apt-get update -qq && sudo apt-get install -y -qq gcc libsqlite3-dev
      elif command -v yum &>/dev/null; then
        info "Installing: gcc sqlite-devel"
        sudo yum install -y gcc sqlite-devel
      elif command -v pacman &>/dev/null; then
        info "Installing: gcc sqlite"
        sudo pacman -S --noconfirm gcc sqlite
      elif command -v zypper &>/dev/null; then
        info "Installing: gcc sqlite3-devel"
        sudo zypper install -y gcc sqlite3-devel
      else
        err "Please install gcc and sqlite3-dev manually, then re-run."
        exit 1
      fi
      ;;
    Darwin)
      if ! command -v xcode-select &>/dev/null || ! xcode-select -p &>/dev/null; then
        info "Installing Xcode Command Line Tools..."
        xcode-select --install
        info "Please re-run the script after installation completes."
        exit 0
      fi
      ;;
  esac
else
  ok "C compiler found: $(command -v gcc || command -v clang)"
fi

# Linux: verify sqlite3 headers are available
if [[ "$(uname -s)" == "Linux" ]]; then
  if ! command -v pkg-config &>/dev/null; then
    if command -v apt-get &>/dev/null; then
      info "Installing: pkg-config"
      sudo apt-get install -y -qq pkg-config
    fi
  fi
fi

# ═══════════════════════════════════════════════════════════════
# PHASE 4 — Clone & Build
# ═══════════════════════════════════════════════════════════════

info "Phase 4: Building CostAffective from source..."

BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

info "Cloning repository..."
git clone --depth 1 "$REPO_URL" "$BUILD_DIR" 2>/dev/null || {
  err "Failed to clone $REPO_URL"
  exit 1
}

info "Building binary..."
(
  cd "$BUILD_DIR"
  CGO_ENABLED=1 go build -o costaffective ./cmd/costaffective/
) 2>&1 | while IFS= read -r line; do info "  $line"; done

BINARY_PATH="$BUILD_DIR/costaffective"
if [[ ! -f "$BINARY_PATH" ]]; then
  err "Build failed — binary not found"
  exit 1
fi

ok "Binary built: $(file "$BINARY_PATH" | sed 's/.*: //')"

# ═══════════════════════════════════════════════════════════════
# PHASE 5 — Install Globally
# ═══════════════════════════════════════════════════════════════

info "Phase 5: Installing globally..."

sudo install -m755 "$BINARY_PATH" "$INSTALL_DIR/costaffective" 2>/dev/null || {
  sudo mkdir -p "$INSTALL_DIR"
  sudo cp "$BINARY_PATH" "$INSTALL_DIR/costaffective"
  sudo chmod +x "$INSTALL_DIR/costaffective"
}

export PATH="$INSTALL_DIR:$PATH"

ok "Installed: $INSTALL_DIR/costaffective"

if command -v costaffective &>/dev/null; then
  ok "Globally available: $(command -v costaffective)"
else
  warn "$INSTALL_DIR may not be in PATH"
  case "$(uname -s)" in
    Darwin)
      profile="$HOME/.zshrc"
      if [[ ! -f "$profile" ]]; then profile="$HOME/.bash_profile"; fi
      ;;
    Linux) profile="$HOME/.bashrc" ;;
    *) profile="$HOME/.profile" ;;
  esac
  echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$profile"
  ok "Added $INSTALL_DIR to $profile — restart your shell or run: source $profile"
fi

# ═══════════════════════════════════════════════════════════════
# PHASE 6 — MCP Client Configuration
# ═══════════════════════════════════════════════════════════════

info "Phase 6: MCP Client Configuration..."

MCP_COMMAND="$(command -v costaffective)"
MCP_ARGS="serve"

has_cmd() { command -v "$1" &>/dev/null; }
has_file() { [[ -f "$1" ]]; }

# ── Detect clients ────────────────────────────────────────────
client_label() {
  case "$1" in
    claude-code)  echo "Claude Code" ;;
    cursor)       echo "Cursor" ;;
    opencode)     echo "OpenCode" ;;
    codex)        echo "Codex CLI" ;;
    antigravity)  echo "Antigravity (Gemini CLI)" ;;
    *)            echo "$1" ;;
  esac
}

detect_clients() {
  local detected=()
  has_cmd claude && detected+=("claude-code")
  has_cmd cursor && detected+=("cursor")
  has_cmd opencode && detected+=("opencode")
  has_cmd codex && detected+=("codex")
  has_file "$HOME/.gemini/config/mcp_config.json" && detected+=("antigravity")
  echo "${detected[@]}"
}

# ── Config writers ────────────────────────────────────────────
write_config_claude_code() {
  local config_file="$HOME/.claude.json"
  if [[ -f "$config_file" ]]; then
    local tmp
    tmp=$(python3 -c "
import json, sys
with open('$config_file') as f:
    cfg = json.load(f)
cfg.setdefault('mcpServers', {})['costaffective'] = {
    'command': '$MCP_COMMAND',
    'args': ['$MCP_ARGS']
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
  ok "Claude Code: configured at $config_file"
}

write_config_cursor() {
  local config_dir="$HOME/.cursor"
  mkdir -p "$config_dir"
  cat > "$config_dir/mcp.json" <<EOF
{
  "mcpServers": {
    "costaffective": {
      "command": "${MCP_COMMAND}",
      "args": ["${MCP_ARGS}"]
    }
  }
}
EOF
  ok "Cursor: configured at $config_dir/mcp.json"
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
  local config_file="$config_dir/config.toml"
  mkdir -p "$config_dir"
  if [[ -f "$config_file" ]]; then echo "" >> "$config_file"; fi
  cat >> "$config_file" <<EOF

[mcp_servers.costaffective]
command = "${MCP_COMMAND}"
args = ["${MCP_ARGS}"]
EOF
  ok "Codex CLI: configured at $config_file"
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
    'command': '$MCP_COMMAND',
    'args': ['$MCP_ARGS']
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

configure_client() {
  local client="$1"
  info "Configuring $(client_label "$client") ..."
  case "$client" in
    claude-code) write_config_claude_code ;;
    cursor)      write_config_cursor ;;
    opencode)    write_config_opencode ;;
    codex)       write_config_codex ;;
    antigravity) write_config_antigravity ;;
    *) warn "Unknown client: $client" ;;
  esac
}

# ── Detect and prompt ─────────────────────────────────────────
DETECTED=($(detect_clients))

if [[ ${#DETECTED[@]} -eq 0 ]]; then
  warn "No supported AI coding clients detected."
  info "You can manually configure any MCP-compatible client later:"
  info "  costaffective install"
else
  echo ""
  info "Detected clients:"
  for c in "${DETECTED[@]}"; do
    echo "    $(client_label "$c")"
  done
  echo ""

  # Ask about each client
  for client in "${DETECTED[@]}"; do
    local label="$(client_label "$client")"
    echo ""
    read -r -p "  Connect CostAffective to ${label}? [Y/n] " reply
    case "$reply" in
      [nN]|[nN][oO])
        info "Skipping $label"
        ;;
      *)
        configure_client "$client"
        ;;
    esac
  done
fi

# ═══════════════════════════════════════════════════════════════
# DONE
# ═══════════════════════════════════════════════════════════════

echo ""
echo "═══════════════════════════════════════════════"
echo "  Installation Complete!"
echo "═══════════════════════════════════════════════"
echo ""
ok "CostAffective is globally available: $(command -v costaffective)"
echo ""
info "Run:  costaffective doctor     (health check)"
info "Run:  costaffective install    (reconfigure clients)"
info "Run:  costaffective serve      (start MCP server)"
echo ""
info "Need help? https://github.com/$REPO"
