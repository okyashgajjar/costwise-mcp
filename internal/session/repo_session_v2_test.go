package session

import (
	"context"
	"strings"
	"testing"
)

func TestRepoSessionRememberRecallFacts(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	rs, err := NewRepoSession(ctx, dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	if err := rs.RememberFact("auth-entrypoint", "auth starts in server/auth.go Login()"); err != nil {
		t.Fatal(err)
	}

	got := rs.RecallFacts("auth")
	if len(got) == 0 || !strings.Contains(got[0], "auth.go") {
		t.Errorf("expected to recall the remembered fact, got %v", got)
	}

	// Persistence: a fresh session at the same repo root reloads the fact
	// (per-repo facts file, not os.TempDir).
	rs2, err := NewRepoSession(ctx, dir, "test2")
	if err != nil {
		t.Fatal(err)
	}
	defer rs2.Close()

	if got2 := rs2.RecallFacts("auth-entrypoint"); len(got2) == 0 {
		t.Errorf("remembered fact did not persist across sessions, got %v", got2)
	}
}

func TestRepoSessionStashRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	rs, err := NewRepoSession(ctx, dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	e, err := rs.Stash.Store("alpha\nbeta needle gamma\ndelta\n", "blob")
	if err != nil {
		t.Fatal(err)
	}
	out, err := rs.Stash.Query(e.Handle, "needle", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "needle") || strings.Contains(out, "alpha") {
		t.Errorf("query-scoped recall returned wrong slice: %q", out)
	}
}
