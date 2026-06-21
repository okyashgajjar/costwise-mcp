package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Location string

const (
	LocationGlobal Location = "global"
	LocationLocal  Location = "local"
)

type DetectionResult struct {
	Installed         bool
	AlreadyConfigured bool
	ConfigPath        string
}

type WriteResult struct {
	Path   string
	Action string // created, updated, unchanged, removed, not-found
}

type InstallOptions struct {
	AutoAllow bool
}

type Target interface {
	ID() string
	DisplayName() string
	SupportsLocation(loc Location) bool
	Detect(loc Location) DetectionResult
	Install(loc Location, opts InstallOptions) []WriteResult
	Uninstall(loc Location) []WriteResult
	PrintConfig(loc Location) string
	DescribePaths(loc Location) []string
}

var registry []Target

func RegisterTarget(t Target) {
	registry = append(registry, t)
}

func AllTargets() []Target {
	return registry
}

func GetTarget(id string) Target {
	for _, t := range registry {
		if t.ID() == id {
			return t
		}
	}
	return nil
}

func DetectAll(loc Location) map[Target]DetectionResult {
	result := make(map[Target]DetectionResult)
	for _, t := range registry {
		result[t] = t.Detect(loc)
	}
	return result
}

var installedBinaryPath string

func SetBinaryPath(path string) {
	installedBinaryPath = path
}

func BinaryPath() string {
	if installedBinaryPath != "" {
		return installedBinaryPath
	}
	return binaryFilename()
}

func DefaultBinaryPath() string {
	return filepath.Join(HomeDir(), ".local", "bin", binaryFilename())
}

func binaryFilename() string {
	if runtime.GOOS == "windows" {
		return "costwise.exe"
	}
	return "costwise"
}

func GetMcpServerConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":    "stdio",
		"command": BinaryPath(),
		"args":    []string{"serve"},
	}
}

func ReadJSONFile(filePath string) map[string]interface{} {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return make(map[string]interface{})
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return make(map[string]interface{})
	}
	return result
}

func WriteJSONFile(filePath string, data map[string]interface{}) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(filePath, content, 0644)
}

func DeepEqual(a, b interface{}) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return home
}

func Tildify(p string) string {
	home := HomeDir()
	if len(p) >= len(home) && p[:len(home)] == home {
		return "~" + p[len(home):]
	}
	return p
}

// RemoveLegacyKeys strips old "costaffective" entries from JSON MCP configs
// so that install/repair auto-migrates users who installed before the rename.
// It handles both "mcpServers" and "mcp" top-level keys.
func RemoveLegacyKeys(cfg map[string]interface{}) (removed bool) {
	for _, key := range []string{"mcpServers", "mcp"} {
		if m, ok := cfg[key].(map[string]interface{}); ok {
			if _, exists := m["costaffective"]; exists {
				delete(m, "costaffective")
				removed = true
			}
		}
	}
	return removed
}
