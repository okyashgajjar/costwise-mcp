package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
)

// opencodeConfigPath returns the config path for opencode, preferring .jsonc over .json.
func opencodeConfigPath(loc installer.Location) string {
	var dir string
	if loc == installer.LocationGlobal {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(installer.HomeDir(), ".config")
		}
		dir = filepath.Join(xdg, "opencode")
	} else {
		cwd, _ := os.Getwd()
		dir = cwd
	}

	jsonc := filepath.Join(dir, "opencode.jsonc")
	json := filepath.Join(dir, "opencode.json")
	if installer.Exists(jsonc) {
		return jsonc
	}
	if installer.Exists(json) {
		return json
	}
	return jsonc
}

type OpencodeTarget struct{}

func (t *OpencodeTarget) ID() string          { return "opencode" }
func (t *OpencodeTarget) DisplayName() string  { return "OpenCode" }

func (t *OpencodeTarget) SupportsLocation(loc installer.Location) bool {
	return true
}

func (t *OpencodeTarget) Detect(loc installer.Location) installer.DetectionResult {
	path := opencodeConfigPath(loc)
	cfg := installer.ReadJSONFile(path)
	mcp, _ := cfg["mcp"].(map[string]interface{})
	_, alreadyConfigured := mcp["costaffective"]

	var installed bool
	if loc == installer.LocationGlobal {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(installer.HomeDir(), ".config")
		}
		installed = installer.Exists(filepath.Join(xdg, "opencode"))
	} else {
		installed = installer.Exists(path)
	}

	return installer.DetectionResult{
		Installed:         installed,
		AlreadyConfigured: alreadyConfigured,
		ConfigPath:        path,
	}
}

func (t *OpencodeTarget) Install(loc installer.Location, opts installer.InstallOptions) []installer.WriteResult {
	return []installer.WriteResult{t.writeMcpEntry(loc)}
}

func (t *OpencodeTarget) getServerEntry() map[string]interface{} {
	return map[string]interface{}{
		"type":    "local",
		"command": []string{installer.BinaryPath(), "serve"},
		"enabled": true,
	}
}

func (t *OpencodeTarget) writeMcpEntry(loc installer.Location) installer.WriteResult {
	file := opencodeConfigPath(loc)
	cfg := installer.ReadJSONFile(file)

	// Seed minimal config if file doesn't exist
	if len(cfg) == 0 {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}

	mcp, _ := cfg["mcp"].(map[string]interface{})
	var before interface{}
	if mcp != nil {
		before = mcp["costaffective"]
	}
	after := t.getServerEntry()

	if installer.DeepEqual(before, after) {
		return installer.WriteResult{Path: file, Action: "unchanged"}
	}

	action := "created"
	if installer.Exists(file) {
		action = "updated"
	}
	if cfg["mcp"] == nil {
		cfg["mcp"] = make(map[string]interface{})
	}
	cfg["mcp"].(map[string]interface{})["costaffective"] = after

	// Add $schema if missing
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}

	installer.WriteJSONFile(file, cfg)
	return installer.WriteResult{Path: file, Action: action}
}

func (t *OpencodeTarget) Uninstall(loc installer.Location) []installer.WriteResult {
	file := opencodeConfigPath(loc)
	if !installer.Exists(file) {
		return []installer.WriteResult{{Path: file, Action: "not-found"}}
	}

	cfg := installer.ReadJSONFile(file)
	if mcp, ok := cfg["mcp"].(map[string]interface{}); ok {
		if _, exists := mcp["costaffective"]; exists {
			delete(mcp, "costaffective")
			if len(mcp) == 0 {
				delete(cfg, "mcp")
			}
			cfg["mcp"] = mcp
			installer.WriteJSONFile(file, cfg)
			return []installer.WriteResult{{Path: file, Action: "removed"}}
		}
	}
	return []installer.WriteResult{{Path: file, Action: "not-found"}}
}

func (t *OpencodeTarget) PrintConfig(loc installer.Location) string {
	path := opencodeConfigPath(loc)
	block := map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]interface{}{
			"costaffective": t.getServerEntry(),
		},
	}
	data, _ := json.MarshalIndent(block, "", "  ")
	return fmt.Sprintf("# Add to %s\n\n%s\n", path, string(data))
}

func (t *OpencodeTarget) DescribePaths(loc installer.Location) []string {
	return []string{opencodeConfigPath(loc)}
}
