package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// parseJSONConfig parses a client config file. A missing file is an empty
// config; a whitespace-only file is also treated as empty (clients sometimes
// create those). An existing, non-empty, unparseable file is an error: the
// file holds client state (history, settings) that must never be silently
// replaced.
func parseJSONConfig(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return make(map[string]interface{}), nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON (%v); refusing to overwrite it — fix or remove the file, then retry", filePath, err)
	}
	if result == nil {
		result = make(map[string]interface{})
	}
	return result, nil
}

func ReadJSONFile(filePath string) map[string]interface{} {
	result, err := parseJSONConfig(filePath)
	if err != nil {
		// Callers treat an empty map as "no config"; the destructive write is
		// blocked separately in WriteJSONFile so the invalid file survives.
		return make(map[string]interface{})
	}
	return result
}

func WriteJSONFile(filePath string, data map[string]interface{}) error {
	if fi, err := os.Lstat(filePath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink; refusing to write it", filePath)
		}
		// Never overwrite an existing unparseable config: ReadJSONFile would
		// have handed callers an empty map, and writing that back would wipe
		// the client's own state (e.g. ~/.claude.json) with no backup.
		if _, perr := parseJSONConfig(filePath); perr != nil {
			return perr
		}
	}
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	// Atomic replace: temp file + rename so a concurrent writer (the client
	// itself rewrites ~/.claude.json frequently) can never observe a torn or
	// half-written config.
	tmp, err := os.CreateTemp(dir, filepath.Base(filePath)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filePath)
}

func DeepEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
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
