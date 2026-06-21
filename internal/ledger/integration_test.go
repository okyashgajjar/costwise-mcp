package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationWithRepoSession(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	// This test exercises the full flow: repoSession operations → ledger.append → sessionBrief
	// It verifies the two systems integrate correctly.

	// Simulate remember → ledger (as the rememberHandler does)
	if err := Append(dir, Event{
		Kind:    "fact",
		Action:  "add",
		Summary: "auth uses JWT refresh rotation, 15min expiry",
	}); err != nil {
		t.Fatal(err)
	}

	// Simulate stash_context → ledger (as the stashContextHandler does)
	if err := Append(dir, Event{
		Kind:    "stash",
		Action:  "create",
		Handle:  "stash_abc123",
		Tokens:  512,
		Summary: "large schema dump",
	}); err != nil {
		t.Fatal(err)
	}

	// Simulate index_repository → ledger (as the indexHandler does)
	if err := Append(dir, Event{
		Kind:    "index",
		Action:  "reindex",
		Files:   47,
		Trigger: "manual",
	}); err != nil {
		t.Fatal(err)
	}

	// Simulate recall → ledger (as the recallHandler does)
	if err := Append(dir, Event{
		Kind:   "recall",
		Action: "read",
		Query:  "jwt",
		Source: "facts",
	}); err != nil {
		t.Fatal(err)
	}

	// Simulate watcher auto-reindex → ledger (as the watcher does)
	if err := Append(dir, Event{
		Kind:         "watch",
		Action:       "auto_reindex",
		ChangedFiles: []string{"go.mod", "go.sum"},
	}); err != nil {
		t.Fatal(err)
	}

	// Now call sessionBrief with ScopeAll — the primary integration contract
	brief, err := SessionBrief(dir, ScopeAll, 300)
	if err != nil {
		t.Fatalf("SessionBrief: %v", err)
	}

	// Verify each event kind appears in the rendered output
	checks := []struct {
		label string
		fn    func() bool
	}{
		{"fact", func() bool { return strings.Contains(brief, "refresh") }},
		{"stash handle", func() bool { return strings.Contains(brief, "stash_abc123") }},
		{"stash tokens", func() bool { return strings.Contains(brief, "512 tok") }},
		{"index files", func() bool { return strings.Contains(brief, "47 files") }},
		{"recall query", func() bool { return strings.Contains(brief, "jwt") }},
		{"watch changed_files", func() bool { return strings.Contains(brief, "go.mod") }},
		{"kind counts header", func() bool { return strings.Contains(brief, "1 fact") }},
		{"recall kind count", func() bool { return strings.Contains(brief, "1 recall") }},
	}

	for _, c := range checks {
		if !c.fn() {
			t.Errorf("integration: missing %s in output:\n%s", c.label, brief)
		}
	}

	// Test ScopeLast — should include all events (gap < 30min)
	briefLast, err := SessionBrief(dir, ScopeLast, 300)
	if err != nil {
		t.Fatalf("SessionBrief(last): %v", err)
	}
	if !strings.Contains(briefLast, "47 files") {
		t.Errorf("ScopeLast should include all events since they're within idle threshold:\n%s", briefLast)
	}

	// Verify close/reopen
	Close(dir)
	// After close, SessionBrief should still work (ReadAll reads from disk)
	briefReopen, err := SessionBrief(dir, ScopeAll, 300)
	if err != nil {
		t.Fatalf("SessionBrief after close: %v", err)
	}
	if !strings.Contains(briefReopen, "47 files") {
		t.Errorf("SessionBrief after close/reopen should find events:\n%s", briefReopen)
	}
}
