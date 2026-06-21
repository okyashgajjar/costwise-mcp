package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/session"
)

func TestWatchdog(t *testing.T) {
	tempDir := t.TempDir()

	// Create dummy git repo
	if err := os.MkdirAll(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}

	ctx := context.Background()
	rs, err := session.NewRepoSession(ctx, tempDir, "test_session")
	if err != nil {
		t.Fatalf("Failed to create repo session: %v", err)
	}
	defer rs.Close()

	wd, err := NewWatchdog(rs)
	if err != nil {
		t.Fatalf("Failed to create watchdog: %v", err)
	}

	err = wd.Start()
	if err != nil {
		t.Fatalf("Failed to start watchdog: %v", err)
	}
	defer wd.Stop()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Watchdog has 1000ms debounce
	time.Sleep(1500 * time.Millisecond)
}
