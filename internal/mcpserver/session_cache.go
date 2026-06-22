package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/okyashgajjar/costwise-mcp/internal/ledger"
	"github.com/okyashgajjar/costwise-mcp/internal/session"
	"github.com/okyashgajjar/costwise-mcp/internal/watcher"
)

var (
	errPathNotAllowed = errors.New("repo_path is not within any allowed directory")
)

type sessionEntry struct {
	rs       *session.RepoSession
	watchdog *watcher.Watchdog
}

type SessionCache struct {
	mu       sync.RWMutex
	sessions map[string]*sessionEntry
}

var globalCache = &SessionCache{
	sessions: make(map[string]*sessionEntry),
}

// allowedPaths holds the resolved absolute prefixes that repo_path must be
// within. Set via Init() at server startup.
var allowedPaths []string

// Init configures the path sandbox. If no paths are given the server will
// refuse every request — the serve command always provides at least $HOME.
func Init(paths []string) {
	seen := make(map[string]bool)
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				clean := filepath.Clean(abs)
				if !seen[clean] {
					allowedPaths = append(allowedPaths, clean)
					seen[clean] = true
				}
			}
		}
	}
}

// validateRepoPath checks that the resolved absolute path lives inside one
// of the allowed prefixes. If no paths have been configured (Init was never
// called) the check is skipped for backward compatibility.
func validateRepoPath(path string) error {
	if len(allowedPaths) == 0 {
		return nil
	}
	for _, prefix := range allowedPaths {
		// Ensure an exact prefix match at a dir boundary:
		//   prefix="/home/user"  path="/home/user2"  → rejected (no / after)
		//   prefix="/home/user"  path="/home/user/"  → accepted
		//   prefix="/home/user"  path="/home/user/projects/repo" → accepted
		rel, err := filepath.Rel(prefix, path)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			return nil
		}
		if err == nil && rel == "." {
			return nil
		}
	}
	return fmt.Errorf("%w (allowed: %v)", errPathNotAllowed, allowedPaths)
}

func GetOrCreateRepoSession(ctx context.Context, repoPath string) (*session.RepoSession, error) {
	// Normalize so different spellings of the same repo (trailing slash,
	// relative path, the resolved root index_repository passes) map to one
	// session — otherwise each spelling spawns a duplicate watchdog and DB.
	key := normalizeRepoPath(repoPath)

	if err := validateRepoPath(key); err != nil {
		return nil, fmt.Errorf("repo_path %s: %w", repoPath, err)
	}

	globalCache.mu.RLock()
	entry, ok := globalCache.sessions[key]
	globalCache.mu.RUnlock()

	if ok {
		return entry.rs, nil
	}

	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	// Check again in case it was created while waiting for the lock
	if entry, ok := globalCache.sessions[key]; ok {
		return entry.rs, nil
	}

	rs, err := session.NewRepoSession(ctx, key, "mcp_session")
	if err != nil {
		return nil, fmt.Errorf("failed to create repo session for %s: %w", key, err)
	}

	// Initialize and start the watchdog
	wd, err := watcher.NewWatchdog(rs)
	if err != nil {
		log.Printf("Warning: failed to initialize watchdog for %s: %v", repoPath, err)
	} else {
		if err := wd.Start(); err != nil {
			log.Printf("Warning: failed to start watchdog for %s: %v", repoPath, err)
		}
	}

	globalCache.sessions[key] = &sessionEntry{
		rs:       rs,
		watchdog: wd,
	}

	return rs, nil
}

// normalizeRepoPath canonicalizes a repo path to a stable cache key. filepath.Abs
// also cleans (resolving "." / ".." and trailing slashes), so "/repo", "/repo/",
// and a relative spelling all collapse to one key.
func normalizeRepoPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}

func CloseAllSessions() {
	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	for path, entry := range globalCache.sessions {
		if entry.watchdog != nil {
			entry.watchdog.Stop()
		}
		if entry.rs != nil {
			entry.rs.Close()
		}
		ledger.Close(path)
		delete(globalCache.sessions, path)
	}
}
