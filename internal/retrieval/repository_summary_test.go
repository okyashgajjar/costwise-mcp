package retrieval

import (
	"fmt"
	"testing"

	"github.com/okyashgajjar/costwise-mcp/internal/treesitter"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"foo_test.go", true},
		{"foo_test.py", true},
		{"test_foo.py", true},
		{"foo_spec.rb", true},
		{"FooTest.java", true},
		{"main.go", false},
		{"app.py", false},
		{"index.js", false},
		{"helper.ts", false},
		{"README.md", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := isTestFile(tc.path)
			if got != tc.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestBuildRepositorySummaryNil(t *testing.T) {
	summary, text := BuildRepositorySummary(nil)
	if summary != nil {
		t.Error("expected nil summary for nil KnowledgeStore")
	}
	if text != "" {
		t.Errorf("expected empty text for nil KnowledgeStore, got %q", text)
	}
}

func TestArchitectureMapEntries(t *testing.T) {
	modules := []ModuleInfo{
		{
			Name:  "cmd",
			Path:  "cmd/",
			Files: []string{"cmd/main.go", "cmd/helper.go"},
		},
		{
			Name:  "internal",
			Path:  "internal/",
			Files: []string{"internal/app.go", "internal/server.go"},
		},
	}

	entries := extractArchitectureEntries(modules)
	if len(entries) == 0 {
		t.Error("expected at least one architecture entry for cmd/main.go")
	}

	found := false
	for _, e := range entries {
		if e == "cmd/main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected cmd/main.go in entries, got %v", entries)
	}
}

func TestExtractLayers(t *testing.T) {
	modules := []ModuleInfo{
		{Name: "cmd", Path: "cmd/"},
		{Name: "internal", Path: "internal/"},
		{Name: "pkg", Path: "pkg/"},
	}
	layers := extractLayers(modules)
	if len(layers) != 3 {
		t.Errorf("expected 3 layers, got %d", len(layers))
	}
}

func TestRepositorySummaryFormat(t *testing.T) {
	rs := &RepositorySummary{
		FileCount:   10,
		SymbolCount: 50,
		TestFiles:   3,
		LanguageMix: map[string]int{"Go": 7, "Python": 3},
		Modules: []ModuleSummary{
			{Name: "cmd", Path: "cmd/", FileCount: 2, SymbolCount: 10, Language: "Go"},
			{Name: "internal", Path: "internal/", FileCount: 5, SymbolCount: 30, Language: "Go"},
		},
		Architecture: ArchitectureMap{
			Layers:  []string{"cmd/", "internal/"},
			Entries: []string{"cmd/main.go"},
		},
	}

	text := rs.Format()
	if !contains(text, "Files: 10") {
		t.Errorf("expected 'Files: 10' in output, got:\n%s", text)
	}
	if !contains(text, "Symbols: 50") {
		t.Errorf("expected 'Symbols: 50' in output, got:\n%s", text)
	}
	if !contains(text, "Test Files: 3") {
		t.Errorf("expected 'Test Files: 3' in output, got:\n%s", text)
	}
	if !contains(text, "Go") {
		t.Errorf("expected 'Go' in language mix, got:\n%s", text)
	}
	if !contains(text, "cmd") || !contains(text, "internal") {
		t.Errorf("expected module names in output, got:\n%s", text)
	}
}

func TestRepositorySummaryEmpty(t *testing.T) {
	rs := &RepositorySummary{
		Modules: []ModuleSummary{},
	}
	text := rs.Format()
	if text == "" {
		t.Error("expected non-empty format even for empty summary")
	}
}

func TestJoinWithinBudget(t *testing.T) {
	items := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	// Zero/negative budget yields nothing.
	if got := joinWithinBudget(items, 0); got != "" {
		t.Errorf("budget 0 should yield empty, got %q", got)
	}
	// Empty input yields nothing.
	if got := joinWithinBudget(nil, 100); got != "" {
		t.Errorf("nil items should yield empty, got %q", got)
	}
	// Generous budget keeps everything, no truncation marker.
	full := joinWithinBudget(items, 1000)
	if !contains(full, "alpha") || !contains(full, "epsilon") || contains(full, "more") {
		t.Errorf("generous budget should keep all items, got %q", full)
	}
	// Tight budget truncates and reports the remainder.
	tight := joinWithinBudget(items, 2) // ~8 chars
	if !contains(tight, "more") {
		t.Errorf("tight budget should append a '+N more' marker, got %q", tight)
	}
	if len(tight) > 2*4+len(" +9 more") {
		t.Errorf("tight budget output too long: %q", tight)
	}
}

// newTestKnowledgeStore builds an in-memory symbol DB populated with `mods`
// directories (files need not exist on disk). Used to exercise the compact
// summary's budgeting without a real repository.
func newTestKnowledgeStore(t *testing.T, mods int) *KnowledgeStore {
	t.Helper()
	db, err := treesitter.NewSymbolDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	var symbols []treesitter.Symbol
	for d := 0; d < mods; d++ {
		file := fmt.Sprintf("internal/mod%02d/file.go", d)
		for s := 0; s < 4; s++ {
			symbols = append(symbols, treesitter.Symbol{
				Name:      fmt.Sprintf("Func%02d_%d", d, s),
				Kind:      treesitter.SymbolFunction,
				Language:  "go",
				File:      file,
				StartLine: s + 1,
				EndLine:   s + 2,
			})
		}
		// GetAllFiles() reads the symbol_files table, populated during indexing.
		if err := db.MarkFileIndexed(file, "testhash"); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.StoreSymbols(symbols); err != nil {
		t.Fatal(err)
	}
	return NewKnowledgeStore(db, t.TempDir())
}

func TestBuildRepositorySummaryCompactBudget(t *testing.T) {
	ks := newTestKnowledgeStore(t, 50)

	const budget = 200
	out := BuildRepositorySummaryCompact(ks, budget, "")

	if out == "" {
		t.Fatal("expected non-empty compact summary")
	}
	estTokens := len(out) / 4
	if estTokens > budget+80 {
		t.Errorf("compact summary exceeded budget: ~%d tokens (budget %d)\n%s", estTokens, budget, out)
	}
	if !contains(out, "more modules") {
		t.Errorf("expected a '+N more modules' rollup for 50 modules at budget %d, got:\n%s", budget, out)
	}
	if !contains(out, "Files:") || !contains(out, "Symbols:") {
		t.Errorf("expected header lines, got:\n%s", out)
	}

	// The compact summary must be materially smaller than the old unbounded one,
	// which lists every module plus the per-directory "Layers" chain.
	_, oldText := BuildRepositorySummary(ks)
	oldTokens := len(oldText) / 4
	t.Logf("repo_summary tokens: old=%d new=%d (budget %d)", oldTokens, estTokens, budget)
	if estTokens >= oldTokens {
		t.Errorf("compact summary (~%d tok) should be smaller than full summary (~%d tok)", estTokens, oldTokens)
	}
}

func TestBuildRepositorySummaryCompactDrillDown(t *testing.T) {
	ks := newTestKnowledgeStore(t, 50)

	out := BuildRepositorySummaryCompact(ks, 500, "mod07")
	if !contains(out, "Module:") {
		t.Errorf("expected module detail header, got:\n%s", out)
	}
	if !contains(out, "mod07") {
		t.Errorf("expected drill-down to reference mod07, got:\n%s", out)
	}
	// Drill-down must not leak other modules' content.
	if contains(out, "mod08") || contains(out, "more modules") {
		t.Errorf("drill-down leaked other modules, got:\n%s", out)
	}

	missing := BuildRepositorySummaryCompact(ks, 500, "does-not-exist")
	if !contains(missing, "not found") {
		t.Errorf("expected 'not found' for unknown module, got:\n%s", missing)
	}
}

func TestBuildRepositorySummaryCompactNil(t *testing.T) {
	if got := BuildRepositorySummaryCompact(nil, 500, ""); got != "" {
		t.Errorf("nil knowledge store should yield empty, got %q", got)
	}
}
