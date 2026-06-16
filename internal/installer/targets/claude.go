package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
)

type ClaudeTarget struct{}

func (t *ClaudeTarget) ID() string          { return "claude" }
func (t *ClaudeTarget) DisplayName() string { return "Claude Code" }

func (t *ClaudeTarget) SupportsLocation(loc installer.Location) bool {
	return true
}

func (t *ClaudeTarget) mcpJSONPath(loc installer.Location) string {
	if loc == installer.LocationGlobal {
		return filepath.Join(installer.HomeDir(), ".claude.json")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".mcp.json")
}

func (t *ClaudeTarget) Detect(loc installer.Location) installer.DetectionResult {
	path := t.mcpJSONPath(loc)
	cfg := installer.ReadJSONFile(path)
	mcpServers, _ := cfg["mcpServers"].(map[string]interface{})
	_, alreadyConfigured := mcpServers["costaffective"]

	installed := installer.Exists(filepath.Join(installer.HomeDir(), ".claude")) ||
		installer.Exists(path)

	return installer.DetectionResult{
		Installed:         installed,
		AlreadyConfigured: alreadyConfigured,
		ConfigPath:        path,
	}
}

func (t *ClaudeTarget) Install(loc installer.Location, opts installer.InstallOptions) []installer.WriteResult {
	results := []installer.WriteResult{}
	results = append(results, t.writeMcpEntry(loc))

	if opts.AutoAllow {
		results = append(results, t.writePermissions(loc))
	}

	return results
}

func (t *ClaudeTarget) writeMcpEntry(loc installer.Location) installer.WriteResult {
	file := t.mcpJSONPath(loc)
	cfg := installer.ReadJSONFile(file)
	before, _ := cfg["mcpServers"].(map[string]interface{})
	beforeEntry := before["costaffective"]
	after := installer.GetMcpServerConfig()

	if installer.DeepEqual(beforeEntry, after) {
		return installer.WriteResult{Path: file, Action: "unchanged"}
	}

	action := "created"
	if installer.Exists(file) {
		action = "updated"
	}
	if cfg["mcpServers"] == nil {
		cfg["mcpServers"] = make(map[string]interface{})
	}
	cfg["mcpServers"].(map[string]interface{})["costaffective"] = after
	if err := installer.WriteJSONFile(file, cfg); err != nil {
		return installer.WriteResult{Path: file, Action: "error"}
	}
	return installer.WriteResult{Path: file, Action: action}
}

func (t *ClaudeTarget) writePermissions(loc installer.Location) installer.WriteResult {
	claudeDir := filepath.Join(installer.HomeDir(), ".claude")
	if loc == installer.LocationLocal {
		cwd, _ := os.Getwd()
		claudeDir = filepath.Join(cwd, ".claude")
	}
	file := filepath.Join(claudeDir, "settings.json")

	cfg := installer.ReadJSONFile(file)
	perms, _ := cfg["permissions"].(map[string]interface{})
	if perms == nil {
		perms = make(map[string]interface{})
	}

	allow, _ := perms["allow"].([]interface{})
	allowStr := make([]string, len(allow))
	for i, v := range allow {
		allowStr[i], _ = v.(string)
	}

	want := []string{
		"mcp__costaffective__search_code",
		"mcp__costaffective__find_symbol",
		"mcp__costaffective__read_symbol",
		"mcp__costaffective__find_references",
		"mcp__costaffective__find_callers",
		"mcp__costaffective__get_repository_summary",
		"mcp__costaffective__index_repository",
		"mcp__costaffective__remember",
		"mcp__costaffective__stash_context",
		"mcp__costaffective__recall",
	}

	changed := false
	for _, p := range want {
		found := false
		for _, existing := range allowStr {
			if existing == p {
				found = true
				break
			}
		}
		if !found {
			allowStr = append(allowStr, p)
			changed = true
		}
	}

	if !changed {
		return installer.WriteResult{Path: file, Action: "unchanged"}
	}

	allowIface := make([]interface{}, len(allowStr))
	for i, v := range allowStr {
		allowIface[i] = v
	}
	perms["allow"] = allowIface
	cfg["permissions"] = perms

	created := !installer.Exists(file)
	if err := installer.WriteJSONFile(file, cfg); err != nil {
		return installer.WriteResult{Path: file, Action: "error"}
	}
	action := "updated"
	if created {
		action = "created"
	}
	return installer.WriteResult{Path: file, Action: action}
}

func (t *ClaudeTarget) Uninstall(loc installer.Location) []installer.WriteResult {
	results := []installer.WriteResult{}

	// Remove MCP entry
	file := t.mcpJSONPath(loc)
	cfg := installer.ReadJSONFile(file)
	if mcpServers, ok := cfg["mcpServers"].(map[string]interface{}); ok {
		if _, exists := mcpServers["costaffective"]; exists {
			delete(mcpServers, "costaffective")
			if len(mcpServers) == 0 {
				delete(cfg, "mcpServers")
			}
			cfg["mcpServers"] = mcpServers
			if err := installer.WriteJSONFile(file, cfg); err != nil {
				results = append(results, installer.WriteResult{Path: file, Action: "error"})
			} else {
				results = append(results, installer.WriteResult{Path: file, Action: "removed"})
			}
		} else {
			results = append(results, installer.WriteResult{Path: file, Action: "not-found"})
		}
	} else {
		results = append(results, installer.WriteResult{Path: file, Action: "not-found"})
	}

	return results
}

func (t *ClaudeTarget) PrintConfig(loc installer.Location) string {
	path := t.mcpJSONPath(loc)
	entry := installer.GetMcpServerConfig()
	block := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"costaffective": entry,
		},
	}
	data, _ := json.MarshalIndent(block, "", "  ")
	return fmt.Sprintf("# Add to %s\n\n%s\n", path, string(data))
}

func (t *ClaudeTarget) DescribePaths(loc installer.Location) []string {
	return []string{t.mcpJSONPath(loc)}
}
