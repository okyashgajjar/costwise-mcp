package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// dynamicAllowedPaths holds paths the user approved at runtime via the allow_dir
// tool. These are checked alongside the static Init()-time prefixes.
var (
	dynamicAllowedPaths []string
	dynamicMu           sync.RWMutex
)

// AllowPath validates, normalises, and adds path to the dynamic allowlist so
// subsequent tool calls on that path (or any subdirectory) pass validation.
// Returns a descriptive error if the path cannot be resolved or is not a
// directory; when successful the caller should retry the original tool call.
func AllowPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve path %q: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("cannot access %q: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", abs)
	}
	clean := filepath.Clean(abs)

	dynamicMu.Lock()
	defer dynamicMu.Unlock()

	for _, p := range dynamicAllowedPaths {
		if p == clean {
			return fmt.Errorf("%q is already allowed", clean)
		}
	}
	dynamicAllowedPaths = append(dynamicAllowedPaths, clean)
	return nil
}

// isPathDynamicallyAllowed checks whether the resolved path lives under any
// dynamically-added prefix.
func isPathDynamicallyAllowed(path string) bool {
	dynamicMu.RLock()
	defer dynamicMu.RUnlock()

	for _, prefix := range dynamicAllowedPaths {
		rel, err := filepath.Rel(prefix, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}
