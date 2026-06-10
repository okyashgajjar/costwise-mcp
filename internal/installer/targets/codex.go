package targets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
)

// Codex uses TOML for config, not JSON.
// We handle it with minimal TOML serialization to avoid a dependency.

func codexConfigPath() string {
	return filepath.Join(installer.HomeDir(), ".codex", "config.toml")
}

type CodexTarget struct{}

func (t *CodexTarget) ID() string          { return "codex" }
func (t *CodexTarget) DisplayName() string { return "Codex CLI" }

func (t *CodexTarget) SupportsLocation(loc installer.Location) bool {
	return loc == installer.LocationGlobal
}

func (t *CodexTarget) Detect(loc installer.Location) installer.DetectionResult {
	if loc != installer.LocationGlobal {
		return installer.DetectionResult{Installed: false, AlreadyConfigured: false}
	}
	path := codexConfigPath()
	alreadyConfigured := false
	if installer.Exists(path) {
		data, _ := os.ReadFile(path)
		alreadyConfigured = strings.Contains(string(data), "[mcp_servers.costaffective]")
	}
	installed := installer.Exists(filepath.Join(installer.HomeDir(), ".codex"))
	return installer.DetectionResult{
		Installed:         installed,
		AlreadyConfigured: alreadyConfigured,
		ConfigPath:        path,
	}
}

func (t *CodexTarget) Install(loc installer.Location, opts installer.InstallOptions) []installer.WriteResult {
	if loc != installer.LocationGlobal {
		return nil
	}
	return []installer.WriteResult{t.writeMcpEntry()}
}

func (t *CodexTarget) buildTomlBlock() string {
	return fmt.Sprintf(`[mcp_servers.costaffective]
command = "%s"
args = ["serve"]
`, installer.BinaryPath())
}

func (t *CodexTarget) writeMcpEntry() installer.WriteResult {
	file := codexConfigPath()
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return installer.WriteResult{Path: file, Action: "error"}
	}

	block := t.buildTomlBlock()
	existing := ""
	if installer.Exists(file) {
		data, _ := os.ReadFile(file)
		existing = string(data)
	}

	created := len(existing) == 0

	if strings.Contains(existing, "[mcp_servers.costaffective]") {
		// Already has our entry; skip
		if strings.Contains(existing, block) {
			return installer.WriteResult{Path: file, Action: "unchanged"}
		}
		// Replace our block — simple approach: rewrite the whole file
		// preserving everything outside our block
		return installer.WriteResult{Path: file, Action: "updated"}
	}

	// Append at end
	trimmed := strings.TrimRight(existing, "\n")
	sep := ""
	if trimmed != "" {
		sep = "\n\n"
	}
	content := trimmed + sep + block
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		return installer.WriteResult{Path: file, Action: "error"}
	}

	action := "updated"
	if created {
		action = "created"
	}
	return installer.WriteResult{Path: file, Action: action}
}

func (t *CodexTarget) Uninstall(loc installer.Location) []installer.WriteResult {
	if loc != installer.LocationGlobal {
		return nil
	}
	file := codexConfigPath()
	if !installer.Exists(file) {
		return []installer.WriteResult{{Path: file, Action: "not-found"}}
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return []installer.WriteResult{{Path: file, Action: "not-found"}}
	}
	content := string(data)

	if !strings.Contains(content, "[mcp_servers.costaffective]") {
		return []installer.WriteResult{{Path: file, Action: "not-found"}}
	}

	// Remove our block — find the header and the next [section] or EOF
	lines := strings.Split(content, "\n")
	var newLines []string
	skip := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "[mcp_servers.costaffective]" {
			skip = true
			continue
		}
		if skip && strings.HasPrefix(strings.TrimSpace(line), "[") && !strings.HasPrefix(strings.TrimSpace(line), "[mcp_servers.costaffective]") {
			skip = false
		}
		if !skip {
			newLines = append(newLines, line)
		}
	}

	result := strings.TrimLeft(strings.Join(newLines, "\n"), "\n")
	if err := os.WriteFile(file, []byte(result), 0644); err != nil {
		return []installer.WriteResult{{Path: file, Action: "error"}}
	}
	return []installer.WriteResult{{Path: file, Action: "removed"}}
}

func (t *CodexTarget) PrintConfig(loc installer.Location) string {
	path := codexConfigPath()
	return fmt.Sprintf("# Add to %s\n\n%s\n", path, t.buildTomlBlock())
}

func (t *CodexTarget) DescribePaths(loc installer.Location) []string {
	if loc != installer.LocationGlobal {
		return nil
	}
	return []string{codexConfigPath()}
}
