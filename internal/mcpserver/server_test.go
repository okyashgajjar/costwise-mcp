package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatalf("NewServer returned nil")
	}
}

func TestGetOrCreateRepoSession(t *testing.T) {
	tempDir := t.TempDir()

	// mock a .git folder to make it a valid repo
	if err := os.MkdirAll(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}

	ctx := context.Background()
	rs1, err := GetOrCreateRepoSession(ctx, tempDir)
	if err != nil {
		t.Fatalf("Failed to get or create: %v", err)
	}

	rs2, err := GetOrCreateRepoSession(ctx, tempDir)
	if err != nil {
		t.Fatalf("Failed to get or create again: %v", err)
	}

	if rs1 != rs2 {
		t.Errorf("Expected identical sessions, got different instances")
	}

	CloseAllSessions()
}

// Different spellings of the same repo path (trailing slash, uncleaned) must
// resolve to a single session — otherwise each spelling spawns a duplicate
// watchdog and SQLite handle on the same repo.
func TestGetOrCreateRepoSessionNormalizesPath(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}
	ctx := context.Background()

	rs1, err := GetOrCreateRepoSession(ctx, tempDir)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	rs2, err := GetOrCreateRepoSession(ctx, tempDir+string(filepath.Separator))
	if err != nil {
		t.Fatalf("trailing-slash get: %v", err)
	}
	if rs1 != rs2 {
		t.Errorf("trailing-slash path created a duplicate session")
	}

	CloseAllSessions()
}
