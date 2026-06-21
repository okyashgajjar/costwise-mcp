package targets

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/okyashgajjar/costwise-mcp/internal/installer"
)

// Antigravity (Google Gemini IDE) uses ~/.gemini/config/mcp_config.json.
// Also handles Gemini CLI which shares ~/.gemini/settings.json.

func antigravityConfigPath() string {
	// Check unified path first
	unified := filepath.Join(installer.HomeDir(), ".gemini", "config", "mcp_config.json")
	if installer.Exists(filepath.Join(installer.HomeDir(), ".gemini", "config", ".migrated")) {
		return unified
	}
	if installer.Exists(unified) {
		return unified
	}
	// Legacy path
	return filepath.Join(installer.HomeDir(), ".gemini", "antigravity", "mcp_config.json")
}

type AntigravityTarget struct{}

func (t *AntigravityTarget) ID() string          { return "antigravity" }
func (t *AntigravityTarget) DisplayName() string { return "Antigravity / Gemini" }

func (t *AntigravityTarget) SupportsLocation(loc installer.Location) bool {
	return loc == installer.LocationGlobal
}

func (t *AntigravityTarget) Detect(loc installer.Location) installer.DetectionResult {
	if loc != installer.LocationGlobal {
		return installer.DetectionResult{Installed: false, AlreadyConfigured: false}
	}
	path := antigravityConfigPath()
	cfg := installer.ReadJSONFile(path)
	mcpServers, _ := cfg["mcpServers"].(map[string]interface{})
	_, alreadyConfigured := mcpServers["costwise"]

	installed := installer.Exists(filepath.Join(installer.HomeDir(), ".gemini"))
	return installer.DetectionResult{
		Installed:         installed,
		AlreadyConfigured: alreadyConfigured,
		ConfigPath:        path,
	}
}

func (t *AntigravityTarget) Install(loc installer.Location, opts installer.InstallOptions) []installer.WriteResult {
	if loc != installer.LocationGlobal {
		return nil
	}
	return []installer.WriteResult{t.writeMcpEntry()}
}

func (t *AntigravityTarget) buildEntry() map[string]interface{} {
	return map[string]interface{}{
		"command": installer.BinaryPath(),
		"args":    []string{"serve"},
	}
}

func (t *AntigravityTarget) writeMcpEntry() installer.WriteResult {
	file := antigravityConfigPath()

	cfg := installer.ReadJSONFile(file)
	installer.RemoveLegacyKeys(cfg)
	mcpServers, _ := cfg["mcpServers"].(map[string]interface{})
	before := mcpServers["costwise"]
	after := t.buildEntry()

	if installer.DeepEqual(before, after) {
		return installer.WriteResult{Path: file, Action: "unchanged"}
	}

	action := "created"
	if installer.Exists(file) {
		action = "updated"
	}
	if cfg["mcpServers"] == nil {
		cfg["mcpServers"] = make(map[string]interface{})
	}
	cfg["mcpServers"].(map[string]interface{})["costwise"] = after
	if err := installer.WriteJSONFile(file, cfg); err != nil {
		return installer.WriteResult{Path: file, Action: "error"}
	}
	return installer.WriteResult{Path: file, Action: action}
}

func (t *AntigravityTarget) Uninstall(loc installer.Location) []installer.WriteResult {
	if loc != installer.LocationGlobal {
		return nil
	}

	results := []installer.WriteResult{}

	// Remove from preferred path
	preferred := antigravityConfigPath()
	results = append(results, t.removeFromFile(preferred))

	// Also sweep the other path (legacy or unified)
	var other string
	if preferred == filepath.Join(installer.HomeDir(), ".gemini", "config", "mcp_config.json") {
		other = filepath.Join(installer.HomeDir(), ".gemini", "antigravity", "mcp_config.json")
	} else {
		other = filepath.Join(installer.HomeDir(), ".gemini", "config", "mcp_config.json")
	}
	if preferred != other {
		r := t.removeFromFile(other)
		if r.Action == "removed" {
			results = append(results, r)
		}
	}

	return results
}

func (t *AntigravityTarget) removeFromFile(file string) installer.WriteResult {
	if !installer.Exists(file) {
		return installer.WriteResult{Path: file, Action: "not-found"}
	}
	cfg := installer.ReadJSONFile(file)
	if mcpServers, ok := cfg["mcpServers"].(map[string]interface{}); ok {
		if _, exists := mcpServers["costwise"]; exists {
			delete(mcpServers, "costwise")
			if len(mcpServers) == 0 {
				delete(cfg, "mcpServers")
			}
			cfg["mcpServers"] = mcpServers
			if err := installer.WriteJSONFile(file, cfg); err != nil {
				return installer.WriteResult{Path: file, Action: "error"}
			}
			return installer.WriteResult{Path: file, Action: "removed"}
		}
	}
	return installer.WriteResult{Path: file, Action: "not-found"}
}

func (t *AntigravityTarget) PrintConfig(loc installer.Location) string {
	path := antigravityConfigPath()
	block := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"costwise": t.buildEntry(),
		},
	}
	data, _ := json.MarshalIndent(block, "", "  ")
	return fmt.Sprintf("# Add to %s\n\n%s\n", path, string(data))
}

func (t *AntigravityTarget) DescribePaths(loc installer.Location) []string {
	if loc != installer.LocationGlobal {
		return nil
	}
	return []string{antigravityConfigPath()}
}
