package stash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadDropsPoisonedManifestHandles verifies the manifest-poisoning fix at
// the load() layer: a repo-controlled manifest with traversal handles must not
// ingest them into the index.
func TestLoadDropsPoisonedManifestHandles(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// A "victim" file outside the stash dir that a poisoned manifest would
	// target via ../../ traversal: stashdir + handle + ".txt" == victim.txt.
	victim := filepath.Join(root, "victim.txt")
	if err := os.WriteFile(victim, []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-24 * time.Hour).UTC()
	entries := make([]Entry, 0, maxEntries+1)
	for i := 0; i < maxEntries; i++ {
		entries = append(entries, Entry{
			Handle:    fmt.Sprintf("%012x", i), // 256 distinct valid hex handles
			CreatedAt: old,
		})
	}
	entries = append(entries, Entry{
		Handle:    "../../victim",                        // traversal: would resolve to root/victim.txt
		CreatedAt: time.Now().Add(-48 * time.Hour).UTC(), // oldest → pruned first
	})
	data, _ := json.MarshalIndent(entries, "", "  ")
	if err := os.WriteFile(s.manifest, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Reopen so load() ingests the poisoned manifest.
	s2, err := New(root)
	if err != nil {
		t.Fatalf("New(reopen): %v", err)
	}
	if _, ok := s2.idx["../../victim"]; ok {
		t.Fatal("traversal handle from poisoned manifest must be rejected by load()")
	}

	// Trigger pruning via Store: 257 loaded entries + new stores > cap, and the
	// poisoned handle would be the oldest entry to prune (and delete) first.
	for i := 0; i < 3; i++ {
		if _, err := s2.Store(fmt.Sprintf("content %d to push past cap", i), "t"); err != nil {
			t.Fatalf("Store: %v", err)
		}
	}

	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("victim file was deleted via poisoned manifest: %v", err)
	}
	if string(got) != "do not delete" {
		t.Fatal("victim file was modified")
	}
}

// TestPruneGuardRejectsTraversalHandle verifies pruneLocked's second layer of
// defense independently of load(): even if a traversal handle somehow reaches
// the index (future regression, direct map injection), pruning must refuse to
// delete anything outside the stash dir.
func TestPruneGuardRejectsTraversalHandle(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	victim := filepath.Join(root, "victim.txt")
	if err := os.WriteFile(victim, []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bypass load() on purpose: simulate a regression by planting the
	// traversal handle directly in the index with the oldest timestamp.
	s.idx["../../victim"] = Entry{
		Handle:    "../../victim",
		CreatedAt: time.Now().Add(-48 * time.Hour).UTC(),
	}
	for i := 0; i < maxEntries+5; i++ {
		h := fmt.Sprintf("%012x", i)
		s.idx[h] = Entry{
			Handle:    h,
			CreatedAt: time.Now().UTC(),
		}
		// pruneLocked only removes entries whose blob files exist.
		if err := os.WriteFile(filepath.Join(s.dir, h+".txt"), []byte("blob"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if n := s.Prune(maxEntries); n == 0 {
		t.Fatal("prune should have removed entries")
	}

	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("victim file deleted despite prune guard: %v", err)
	}
	if string(got) != "do not delete" {
		t.Fatal("victim file was modified")
	}
	if _, ok := s.idx["../../victim"]; !ok {
		t.Error("traversal handle should be dropped from the index without deleting anything")
	}
}
