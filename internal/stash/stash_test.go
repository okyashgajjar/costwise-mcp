package stash

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreGetRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	content := "line one\nline two\nline three\n"
	e, err := s.Store(content, "my blob")
	if err != nil {
		t.Fatal(err)
	}
	if e.Handle == "" || !isHexHandle(e.Handle) {
		t.Errorf("expected hex handle, got %q", e.Handle)
	}
	if e.Label != "my blob" || e.Bytes != len(content) {
		t.Errorf("unexpected entry metadata: %+v", e)
	}

	got, ok, err := s.Get(e.Handle)
	if err != nil || !ok {
		t.Fatalf("Get failed: ok=%v err=%v", ok, err)
	}
	if got != content {
		t.Errorf("round-trip mismatch: %q != %q", got, content)
	}
}

func TestStoreIdempotentHandle(t *testing.T) {
	s, _ := New(t.TempDir())
	a, _ := s.Store("same content", "first")
	b, _ := s.Store("same content", "second")
	if a.Handle != b.Handle {
		t.Errorf("identical content should yield identical handle: %q != %q", a.Handle, b.Handle)
	}
	if len(s.List()) != 1 {
		t.Errorf("idempotent store should not duplicate entries, got %d", len(s.List()))
	}
}

func TestQueryMatchesAndBudget(t *testing.T) {
	s, _ := New(t.TempDir())
	var sb strings.Builder
	// Filler shares no substring with the needle and carries its own token ("padding").
	for i := 0; i < 200; i++ {
		sb.WriteString("alpha padding row\n")
	}
	sb.WriteString("here is the NEEDLE we want\n")
	for i := 0; i < 200; i++ {
		sb.WriteString("beta padding row\n")
	}
	e, _ := s.Store(sb.String(), "haystack")

	// Query returns only the matching line(s).
	out, err := s.Query(e.Handle, "needle", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "NEEDLE") {
		t.Errorf("expected matching line, got:\n%s", out)
	}
	if strings.Contains(out, "padding") {
		t.Errorf("query should not return non-matching lines, got:\n%s", out)
	}

	// Budget is respected even when many lines match.
	all, _ := s.Query(e.Handle, "padding", 50) // ~200 chars
	if len(all)/4 > 50+50 {
		t.Errorf("query exceeded budget: ~%d tokens\n%s", len(all)/4, all)
	}
	if !strings.Contains(all, "more matching lines") {
		t.Errorf("expected truncation marker for over-budget matches, got:\n%s", all)
	}

	// No match is reported, not errored.
	none, err := s.Query(e.Handle, "zzz-not-present", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(none, "No lines") {
		t.Errorf("expected no-match message, got:\n%s", none)
	}
}

func TestGetRejectsPathTraversal(t *testing.T) {
	s, _ := New(t.TempDir())
	for _, bad := range []string{"../../etc/passwd", "..", "foo/bar", "abc.def"} {
		if _, ok, _ := s.Get(bad); ok {
			t.Errorf("path-traversal handle %q should not resolve", bad)
		}
	}
	if _, err := s.Query("../../etc/passwd", "x", 100); err == nil {
		t.Error("Query on traversal handle should error (not found)")
	}
}

func TestHandlePrefixAccepted(t *testing.T) {
	s, _ := New(t.TempDir())
	e, _ := s.Store("hello world", "")
	// A "stash:" / "stash://" prefixed handle should resolve the same blob.
	if _, ok, _ := s.Get("stash://" + e.Handle); !ok {
		t.Error("stash:// prefixed handle should resolve")
	}
	if _, ok, _ := s.Get("stash:" + e.Handle); !ok {
		t.Error("stash: prefixed handle should resolve")
	}
}

func TestPruneCapsEntries(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	for i := 0; i < 10; i++ {
		if _, err := s.Store(fmt.Sprintf("content number %d", i), ""); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.List()) != 10 {
		t.Fatalf("expected 10 stored entries, got %d", len(s.List()))
	}

	removed := s.Prune(4)
	if removed != 6 {
		t.Errorf("expected 6 entries pruned, got %d", removed)
	}
	if len(s.List()) != 4 {
		t.Errorf("expected 4 surviving entries, got %d", len(s.List()))
	}

	// Pruned blobs are deleted from disk, not just dropped from the manifest.
	txt, _ := filepath.Glob(filepath.Join(dir, ".mycli-fts", stashDirName, "*.txt"))
	if len(txt) != 4 {
		t.Errorf("expected 4 blob files on disk after prune, got %d", len(txt))
	}

	// A reopened store sees only the surviving entries.
	s2, _ := New(dir)
	if len(s2.List()) != 4 {
		t.Errorf("prune did not persist: reopened store has %d entries", len(s2.List()))
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s1, _ := New(dir)
	e, _ := s1.Store("durable content\nsecond line\n", "persisted")

	// Reopen at the same repo root — the manifest and blob must survive.
	s2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, _ := s2.Get(e.Handle)
	if !ok || !strings.Contains(got, "durable content") {
		t.Errorf("stash did not survive reopen: ok=%v content=%q", ok, got)
	}
	list := s2.List()
	if len(list) != 1 || list[0].Label != "persisted" {
		t.Errorf("manifest did not survive reopen: %+v", list)
	}
}
