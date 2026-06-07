package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
)

type CursorTarget struct{}

func (t *CursorTarget) ID() string          { return "cursor" }
func (t *CursorTarget) DisplayName() string  { return "Cursor" }

func (t *CursorTarget) SupportsLocation(loc installer.Location) bool {
	return true
}

func (t *CursorTarget) mcpJSONPath(loc installer.Location) string {
	if loc == installer.LocationGlobal {
		return filepath.Join(installer.HomeDir(), ".cursor", "mcp.json")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".cursor", "mcp.json")
}

func (t *CursorTarget) Detect(loc installer.Location) installer.DetectionResult {
	path := t.mcpJSONPath(loc)
	cfg := installer.ReadJSONFile(path)
	mcpServers, _ := cfg["mcpServers"].(map[string]interface{})
	_, alreadyConfigured := mcpServers["costaffective"]

	var installed bool
	if loc == installer.LocationGlobal {
		installed = installer.Exists(filepath.Join(installer.HomeDir(), ".cursor"))
	} else {
		cwd, _ := os.Getwd()
		installed = installer.Exists(filepath.Join(cwd, ".cursor"))
	}

	return installer.DetectionResult{
		Installed:         installed,
		AlreadyConfigured: alreadyConfigured,
		ConfigPath:        path,
	}
}

func (t *CursorTarget) Install(loc installer.Location, opts installer.InstallOptions) []installer.WriteResult {
	return []installer.WriteResult{t.writeMcpEntry(loc)}
}

func (t *CursorTarget) buildMcpConfig(loc installer.Location) map[string]interface{} {
	return map[string]interface{}{
		"type":    "stdio",
		"command": installer.BinaryPath(),
		"args":    []string{"serve"},
	}
}

func (t *CursorTarget) writeMcpEntry(loc installer.Location) installer.WriteResult {
	file := t.mcpJSONPath(loc)
	cfg := installer.ReadJSONFile(file)
	mcpServers, _ := cfg["mcpServers"].(map[string]interface{})
	before := mcpServers["costaffective"]
	after := t.buildMcpConfig(loc)

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
	cfg["mcpServers"].(map[string]interface{})["costaffective"] = after
	installer.WriteJSONFile(file, cfg)
	return installer.WriteResult{Path: file, Action: action}
}

func (t *CursorTarget) Uninstall(loc installer.Location) []installer.WriteResult {
	file := t.mcpJSONPath(loc)
	cfg := installer.ReadJSONFile(file)
	if mcpServers, ok := cfg["mcpServers"].(map[string]interface{}); ok {
		if _, exists := mcpServers["costaffective"]; exists {
			delete(mcpServers, "costaffective")
			if len(mcpServers) == 0 {
				delete(cfg, "mcpServers")
			}
			cfg["mcpServers"] = mcpServers
			installer.WriteJSONFile(file, cfg)
			return []installer.WriteResult{{Path: file, Action: "removed"}}
		}
	}
	return []installer.WriteResult{{Path: file, Action: "not-found"}}
}

func (t *CursorTarget) PrintConfig(loc installer.Location) string {
	path := t.mcpJSONPath(loc)
	block := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"costaffective": t.buildMcpConfig(loc),
		},
	}
	data, _ := json.MarshalIndent(block, "", "  ")
	return fmt.Sprintf("# Add to %s\n\n%s\n", path, string(data))
}

func (t *CursorTarget) DescribePaths(loc installer.Location) []string {
	return []string{t.mcpJSONPath(loc)}
}
