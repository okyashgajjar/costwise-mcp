package mcpserver

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/okyashgajjar/costaffective-mcp/internal/session"
	"github.com/okyashgajjar/costaffective-mcp/internal/watcher"
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

func GetOrCreateRepoSession(ctx context.Context, repoPath string) (*session.RepoSession, error) {
	// Normalize so different spellings of the same repo (trailing slash,
	// relative path, the resolved root index_repository passes) map to one
	// session — otherwise each spelling spawns a duplicate watchdog and DB.
	key := normalizeRepoPath(repoPath)

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
		delete(globalCache.sessions, path)
	}
}
