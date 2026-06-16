package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyAndInstructionsNonEmpty(t *testing.T) {
	if Policy() == "" {
		t.Fatal("embedded policy is empty")
	}
	if Instructions() != Policy() {
		t.Error("Instructions should return the policy body")
	}
	// Mentions the cache-reducing tools it is meant to drive.
	for _, kw := range []string{"stash_context", "recall", "remember"} {
		if !strings.Contains(Policy(), kw) {
			t.Errorf("policy should mention %q", kw)
		}
	}
}

func TestPolicyWithinTokenBudget(t *testing.T) {
	// It is a fixed per-session cost (instructions field + skill); keep it tight.
	tokens := len(Instructions()) / 4
	if tokens > 320 {
		t.Errorf("session policy too large: ~%d tokens (keep it lean)", tokens)
	}
}

func TestSkillMarkdownFrontmatter(t *testing.T) {
	md := SkillMarkdown()
	if !strings.HasPrefix(md, "---\n") {
		t.Fatalf("SKILL.md must start with frontmatter, got:\n%s", md[:min(40, len(md))])
	}
	if !strings.Contains(md, "name: "+Name) {
		t.Error("frontmatter missing name")
	}
	if !strings.Contains(md, "description: ") {
		t.Error("frontmatter missing description")
	}
	// Exactly one closing delimiter pair before the body.
	if strings.Count(md, "---\n") < 2 {
		t.Error("frontmatter not closed")
	}
}

func TestInstallUninstallRoundTrip(t *testing.T) {
	// Redirect HOME, XDG, and cwd so all targets land in temp dirs.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))
	t.Chdir(t.TempDir())

	res, err := Install(ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no install results")
	}
	// Every target's file must exist after install.
	for _, r := range res {
		if r.Action != "created" {
			t.Errorf("%s/%s: first install should be created, got %q", r.Scope, r.Target, r.Action)
		}
		if _, err := os.Stat(r.Path); err != nil {
			t.Errorf("%s/%s: file not written at %s: %v", r.Scope, r.Target, r.Path, err)
		}
	}

	// The global Claude SKILL.md lands at the expected path.
	wantClaude := filepath.Join(tmp, ".claude", "skills", Name, "SKILL.md")
	if _, err := os.Stat(wantClaude); err != nil {
		t.Errorf("global Claude SKILL.md missing at %s: %v", wantClaude, err)
	}

	// Idempotent re-install reports "unchanged" everywhere.
	res2, _ := Install(ScopeBoth)
	for _, r := range res2 {
		if r.Action != "unchanged" {
			t.Errorf("%s/%s: re-install should be unchanged, got %q", r.Scope, r.Target, r.Action)
		}
	}

	// Uninstall removes everything.
	resU, err := Uninstall(ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range resU {
		if r.Action != "removed" {
			t.Errorf("%s/%s: uninstall should remove, got %q", r.Scope, r.Target, r.Action)
		}
		if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
			t.Errorf("%s/%s: file should be gone after uninstall: %s", r.Scope, r.Target, r.Path)
		}
	}

	// Second uninstall → not-found, not an error.
	res3, err := Uninstall(ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res3 {
		if r.Action != "not-found" {
			t.Errorf("%s/%s: second uninstall should be not-found, got %q", r.Scope, r.Target, r.Action)
		}
	}
}

// A managed block must coexist with the user's own content in a shared file,
// and uninstall must strip only our block.
func TestBlockPreservesUserContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	userText := "# My project rules\n\nAlways write tests first.\n"
	if err := os.WriteFile(path, []byte(userText), 0o644); err != nil {
		t.Fatal(err)
	}

	if action, err := upsertBlock(path, skillBlock()); err != nil || action != "updated" {
		t.Fatalf("upsert into existing file: action=%q err=%v", action, err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "Always write tests first.") {
		t.Error("user content was lost")
	}
	if !strings.Contains(string(got), blockStart) || !strings.Contains(string(got), "stash_context") {
		t.Error("managed block not inserted")
	}

	// Idempotent.
	if action, _ := upsertBlock(path, skillBlock()); action != "unchanged" {
		t.Errorf("second upsert should be unchanged, got %q", action)
	}

	// Remove strips only our block, keeps user content, keeps the file.
	if action, err := removeBlock(path); err != nil || action != "removed" {
		t.Fatalf("removeBlock: action=%q err=%v", action, err)
	}
	got2, _ := os.ReadFile(path)
	if !strings.Contains(string(got2), "Always write tests first.") {
		t.Error("user content removed along with block")
	}
	if strings.Contains(string(got2), blockStart) {
		t.Error("managed block not removed")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
