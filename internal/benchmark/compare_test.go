package benchmark

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsSafeSearchTerm guards against ripgrep argument injection: a
// dataset-controlled term must never look like a flag (e.g. "--pre=sh" would
// make rg pipe matched file contents through `sh`).
func TestIsSafeSearchTerm(t *testing.T) {
	cases := []struct {
		term string
		ok   bool
	}{
		{"Foo", true},
		{"foo_bar", true},
		{"--pre=sh", false},
		{"--no-ignore", false},
		{"-e", false},
		{"-g", false},
		{"-", true}, // lone dash: pattern, not a flag
		{"", false},
	}
	for _, c := range cases {
		if got := isSafeSearchTerm(c.term); got != c.ok {
			t.Errorf("isSafeSearchTerm(%q) = %v, want %v", c.term, got, c.ok)
		}
	}
}

// TestGrepSymbolRejectsFlagTerm verifies the end-to-end fix: a flag-like term
// passed to grepSymbol must not be interpreted as an rg option (it must not
// execute a preprocessor command), and returns no output.
func TestGrepSymbolRejectsFlagTerm(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}
	dir := t.TempDir()
	// The planted payload: if rg interpreted --pre=sh, every matched line
	// would be piped through sh and this file would be executed as shell.
	payload := "#!/bin/sh\n" + "echo PWNED > " + filepath.Join(dir, "pwned") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("var x = 1\n"+payload), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _ := grepSymbol(dir, "--pre=sh", 80)
	if strings.Contains(out, "PWNED") {
		t.Fatal("rg argument injection: preprocessor command was executed")
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("rg argument injection: payload file was created")
	}
}

// TestGrepTermStripsPunctuation keeps a plain regression check that normal
// symbol lookup still produces a usable search term.
func TestGrepTermStripsPunctuation(t *testing.T) {
	// grepTerm picks the longest non-punctuated word.
	term := grepTerm(Task{Query: "Where is RepoMap implemented?"})
	if term != "implemented" {
		t.Errorf("grepTerm = %q, want implemented", term)
	}
}
