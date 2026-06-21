package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create the .mycli-fts directory to match real setup
	if err := os.MkdirAll(filepath.Join(dir, ".mycli-fts"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func closeLedger(t *testing.T, repoPath string) {
	t.Helper()
	Close(repoPath)
}

func TestAppendAndReadAllRoundTrip(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	events := []Event{
		{
			Kind:    "fact",
			Action:  "add",
			Summary: "auth uses JWT refresh rotation, 15min expiry",
		},
		{
			Kind:    "stash",
			Action:  "create",
			Handle:  "stash_a91f",
			Tokens:  4200,
			Summary: "large schema dump",
		},
		{
			Kind:    "index",
			Action:  "reindex",
			Files:   12,
			Trigger: "dependency_bump",
		},
		{
			Kind:   "recall",
			Action: "read",
			Query:  "auth flow",
			Source: "facts",
		},
		{
			Kind:         "watch",
			Action:       "auto_reindex",
			ChangedFiles: []string{"go.sum", "package.json"},
		},
	}

	// Append each event
	for _, e := range events {
		if err := Append(repo, e); err != nil {
			t.Fatalf("Append(%q): %v", e.Kind, err)
		}
	}

	// Read back
	got, err := ReadAll(repo)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != len(events) {
		t.Fatalf("ReadAll returned %d events, want %d", len(got), len(events))
	}

	for i, g := range got {
		w := events[i]
		if g.Kind != w.Kind {
			t.Errorf("event %d Kind=%q, want %q", i, g.Kind, w.Kind)
		}
		if g.Action != w.Action {
			t.Errorf("event %d Action=%q, want %q", i, g.Action, w.Action)
		}
		if g.Summary != w.Summary {
			t.Errorf("event %d Summary=%q, want %q", i, g.Summary, w.Summary)
		}
		if g.Handle != w.Handle {
			t.Errorf("event %d Handle=%q, want %q", i, g.Handle, w.Handle)
		}
		if g.Tokens != w.Tokens {
			t.Errorf("event %d Tokens=%d, want %d", i, g.Tokens, w.Tokens)
		}
		if g.Files != w.Files {
			t.Errorf("event %d Files=%d, want %d", i, g.Files, w.Files)
		}
		if g.Trigger != w.Trigger {
			t.Errorf("event %d Trigger=%q, want %q", i, g.Trigger, w.Trigger)
		}
		if g.Query != w.Query {
			t.Errorf("event %d Query=%q, want %q", i, g.Query, w.Query)
		}
		if g.Source != w.Source {
			t.Errorf("event %d Source=%q, want %q", i, g.Source, w.Source)
		}
		if len(g.ChangedFiles) != len(w.ChangedFiles) {
			t.Errorf("event %d ChangedFiles length mismatch", i)
		} else {
			for j := range g.ChangedFiles {
				if g.ChangedFiles[j] != w.ChangedFiles[j] {
					t.Errorf("event %d ChangedFiles[%d]=%q, want %q", i, j, g.ChangedFiles[j], w.ChangedFiles[j])
				}
			}
		}
		// Verify TS is set (Append should set it)
		if g.TS.IsZero() {
			t.Errorf("event %d TS is zero (Append should set it)", i)
		}
	}
}

func TestMalformedLines(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	// Write some valid events and a malformed line directly
	dir := filepath.Join(repo, ".mycli-fts")
	path := filepath.Join(dir, ledgerFile)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"ts":"2026-06-21T14:02:11Z","kind":"fact","action":"add","summary":"test"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`this is not json\n` + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"ts":"2026-06-21T15:00:00Z","kind":"unknown_new_kind","action":"whatever","extra":"future-field"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"ts":"2026-06-21T15:01:00Z","kind":"stash","action":"create"` + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"ts":"2026-06-21T15:02:00Z","kind":"fact","action":"add","summary":"after truncated line"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	events, err := ReadAll(repo)
	if err != nil {
		t.Fatalf("ReadAll should not error on malformed lines: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("ReadAll with malformed lines: got %d events, want 3 (valid ones)", len(events))
	}
	if events[0].Summary != "test" {
		t.Errorf("first event summary=%q, want %q", events[0].Summary, "test")
	}
	if events[1].Kind != "unknown_new_kind" {
		t.Errorf("second event kind=%q, want %q", events[1].Kind, "unknown_new_kind")
	}
	if events[2].Summary != "after truncated line" {
		t.Errorf("third event summary=%q, want %q", events[2].Summary, "after truncated line")
	}
}

func TestAppendNoRepo(t *testing.T) {
	repo := t.TempDir() // no .mycli-fts directory
	defer closeLedger(t, repo)

	if err := Append(repo, Event{Kind: "fact", Action: "add", Summary: "test"}); err != nil {
		t.Fatalf("Append should create .mycli-fts if missing: %v", err)
	}

	events, err := ReadAll(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestAppendConcurrency(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	var wg sync.WaitGroup
	n := 50
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Append(repo, Event{
				Kind:    "fact",
				Action:  "add",
				Summary: "concurrent test",
			}); err != nil {
				t.Errorf("concurrent Append: %v", err)
			}
		}(i)
	}
	wg.Wait()

	events, err := ReadAll(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != n {
		t.Fatalf("expected %d events under concurrency, got %d", n, len(events))
	}

	// Verify no line corruption — every line must be valid JSON
	data, err := os.ReadFile(filepath.Join(repo, ".mycli-fts", ledgerFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Quick check: each line should start with { and be valid enough
		if line[0] != '{' {
			t.Fatalf("corrupted line: %q", line)
		}
	}
}

func TestSessionBriefEmpty(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	out, err := SessionBrief(repo, ScopeAll, 300)
	if err != nil {
		t.Fatalf("SessionBrief on empty ledger: %v", err)
	}
	if !strings.Contains(out, "No session events") {
		t.Errorf("unexpected output for empty ledger: %q", out)
	}
}

func TestSessionBriefAll(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	events := []Event{
		{Kind: "fact", Action: "add", Summary: "first fact"},
		{Kind: "stash", Action: "create", Handle: "abc", Tokens: 100, Summary: "data"},
		{Kind: "index", Action: "reindex", Files: 5, Trigger: "manual"},
	}
	for _, e := range events {
		if err := Append(repo, e); err != nil {
			t.Fatal(err)
		}
	}

	out, err := SessionBrief(repo, ScopeAll, 300)
	if err != nil {
		t.Fatalf("SessionBrief: %v", err)
	}
	if !strings.Contains(out, `"first fact"`) {
		t.Errorf("output missing fact: %s", out)
	}
	if !strings.Contains(out, "abc") {
		t.Errorf("output missing stash handle: %s", out)
	}
	if !strings.Contains(out, "5 files") {
		t.Errorf("output missing index count: %s", out)
	}
	if !strings.Contains(out, "1 fact") || !strings.Contains(out, "1 stash") || !strings.Contains(out, "1 index") {
		t.Errorf("output missing kind counts: %s", out)
	}
	if !strings.Contains(out, "+") {
		t.Errorf("expected '+' prefix for add/create actions: %s", out)
	}
}

func TestSessionBriefBudget(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	// Write enough events to exceed a small budget
	for i := 0; i < 100; i++ {
		if err := Append(repo, Event{
			Kind:    "fact",
			Action:  "add",
			Summary: "event number " + itoa(i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// SessionBrief with a very tight budget
	out, err := SessionBrief(repo, ScopeAll, 1) // ~4 chars
	if err != nil {
		t.Fatalf("SessionBrief: %v", err)
	}
	if !strings.Contains(out, "omitted") {
		t.Errorf("expected omitted-events marker for tight budget: %s", out)
	}
}

func TestSessionBriefScope(t *testing.T) {
	repo := setupTestRepo(t)
	defer closeLedger(t, repo)

	// Write an event, then check scope "last" returns it
	if err := Append(repo, Event{Kind: "fact", Action: "add", Summary: "recent fact"}); err != nil {
		t.Fatal(err)
	}

	out, err := SessionBrief(repo, ScopeLast, 300)
	if err != nil {
		t.Fatalf("SessionBrief last: %v", err)
	}
	if !strings.Contains(out, `"recent fact"`) {
		t.Errorf("scope=last should include recent event: %s", out)
	}
}

func TestRenderEvent(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{
			event: Event{Kind: "fact", Action: "add", Summary: "test fact"},
			want:  `+ fact:   remembered: "test fact"`,
		},
		{
			event: Event{Kind: "recall", Action: "read", Query: "auth", Source: ""},
			want:  `~ recall: recalled "auth" from facts`,
		},
		{
			event: Event{Kind: "recall", Action: "read", Query: "config", Source: "stash_abc"},
			want:  `~ recall: recalled "config" from stash_abc`,
		},
		{
			event: Event{Kind: "stash", Action: "create", Handle: "h12", Tokens: 500, Summary: "data dump"},
			want:  `+ stash:  stashed data dump (handle: h12, ~500 tok)`,
		},
		{
			event: Event{Kind: "index", Action: "reindex", Files: 7, Trigger: "manual"},
			want:  `~ index:  reindexed 7 files (trigger: manual)`,
		},
		{
			event: Event{Kind: "watch", Action: "auto_reindex", ChangedFiles: []string{"a.go", "b.go"}},
			want:  `~ watch:  auto-reindexed after file change: a.go, b.go`,
		},
		{
			event: Event{Kind: "unknown", Action: "x", Summary: "y"},
			want:  `+ unknown: x y`,
		},
	}

	for _, tt := range tests {
		got := renderEvent(tt.event)
		if got != tt.want {
			t.Errorf("renderEvent(%s/%s) =\n  %q\nwant:\n  %q", tt.event.Kind, tt.event.Action, got, tt.want)
		}
	}
}

func TestFilterByScope(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-2 * time.Hour)
	old := now.Add(-48 * time.Hour)

	events := []Event{
		{TS: old, Kind: "fact", Action: "add"},
		{TS: earlier, Kind: "fact", Action: "add"},
		{TS: now, Kind: "fact", Action: "add"},
	}

	// ScopeAll — all events, newest first
	out := filterByScope(events, ScopeAll)
	if len(out) != 3 {
		t.Fatalf("ScopeAll: got %d, want 3", len(out))
	}
	if !out[0].TS.Equal(now) {
		t.Errorf("ScopeAll: expected newest first")
	}

	// ScopeToday — only today's events, newest first
	out = filterByScope(events, ScopeToday)
	for _, e := range out {
		if e.TS.Equal(old) {
			t.Errorf("ScopeToday should exclude events from other days")
		}
	}
}

func TestLatestSession(t *testing.T) {
	base := time.Now().UTC().Add(-1 * time.Hour)

	events := []Event{
		{TS: base, Kind: "fact", Action: "add"},
		{TS: base.Add(5 * time.Minute), Kind: "fact", Action: "add"},
		{TS: base.Add(45 * time.Minute), Kind: "fact", Action: "add"}, // gap > 30 min from next
		{TS: base.Add(50 * time.Minute), Kind: "fact", Action: "add"},
	}

	out := latestSession(events)
	if len(out) != 2 {
		t.Fatalf("latestSession: got %d, want 2 (events after the gap)", len(out))
	}
	// Should be newest first
	if out[0].TS.Sub(events[3].TS) != 0 {
		t.Errorf("latestSession[0] should be the most recent event")
	}
}

func TestCloseAll(t *testing.T) {
	repo1 := setupTestRepo(t)
	repo2 := setupTestRepo(t)

	_ = Append(repo1, Event{Kind: "fact", Action: "add", Summary: "r1"})
	_ = Append(repo2, Event{Kind: "fact", Action: "add", Summary: "r2"})

	CloseAll()

	// After close, appending to either should work (reopens)
	if err := Append(repo1, Event{Kind: "fact", Action: "add", Summary: "after close"}); err != nil {
		t.Fatalf("Append after CloseAll: %v", err)
	}

	Close(repo1)
	Close(repo2)
}

// itoa is a strconv.Itoa replacement to avoid importing strconv in test
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
