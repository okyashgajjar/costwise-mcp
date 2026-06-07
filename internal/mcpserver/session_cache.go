package mcpserver

import (
	"context"
	"fmt"
	"log"
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
	globalCache.mu.RLock()
	entry, ok := globalCache.sessions[repoPath]
	globalCache.mu.RUnlock()

	if ok {
		return entry.rs, nil
	}

	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	// Check again in case it was created while waiting for the lock
	if entry, ok := globalCache.sessions[repoPath]; ok {
		return entry.rs, nil
	}

	rs, err := session.NewRepoSession(ctx, repoPath, "mcp_session")
	if err != nil {
		return nil, fmt.Errorf("failed to create repo session for %s: %w", repoPath, err)
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

	globalCache.sessions[repoPath] = &sessionEntry{
		rs:       rs,
		watchdog: wd,
	}

	return rs, nil
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
