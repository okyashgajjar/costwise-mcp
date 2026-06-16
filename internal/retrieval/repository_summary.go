package retrieval

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type RepositorySummary struct {
	Purpose      string          `json:"-"`
	Modules      []ModuleSummary `json:"modules"`
	FileCount    int             `json:"file_count"`
	LanguageMix  map[string]int  `json:"language_mix"`
	SymbolCount  int             `json:"symbol_count"`
	TestFiles    int             `json:"test_files"`
	Architecture ArchitectureMap `json:"architecture"`
}

type ModuleSummary struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	FileCount   int      `json:"file_count"`
	SymbolCount int      `json:"symbol_count"`
	Language    string   `json:"language"`
	TopSymbols  []string `json:"top_symbols,omitempty"`
}

type ArchitectureMap struct {
	Layers  []string `json:"layers,omitempty"`
	Entries []string `json:"entries,omitempty"`
}

func BuildRepositorySummary(ks *KnowledgeStore) (*RepositorySummary, string) {
	if ks == nil {
		return nil, ""
	}

	modules, err := ks.GetModules()
	if err != nil {
		return nil, ""
	}

	summaries, err := ks.GetAllFileSummaries()
	if err != nil {
		return nil, ""
	}

	summary := &RepositorySummary{
		Modules:     make([]ModuleSummary, 0, len(modules)),
		LanguageMix: make(map[string]int),
		Architecture: ArchitectureMap{
			Layers:  extractLayers(modules),
			Entries: extractArchitectureEntries(modules),
		},
	}

	totalSymbols := 0
	testCount := 0

	for _, mod := range modules {
		topSymbols := extractTopSymbols(ks, mod)
		m := ModuleSummary{
			Name:        mod.Name,
			Path:        mod.Path,
			FileCount:   len(mod.Files),
			SymbolCount: mod.Symbols,
			Language:    mod.Language,
			TopSymbols:  topSymbols,
		}
		summary.Modules = append(summary.Modules, m)
		totalSymbols += mod.Symbols
		if mod.Language != "" {
			summary.LanguageMix[mod.Language] += len(mod.Files)
		}
	}

	for _, s := range summaries {
		if isTestFile(s.Path) {
			testCount++
		}
	}

	summary.FileCount = len(summaries)
	summary.SymbolCount = totalSymbols
	summary.TestFiles = testCount

	sort.Slice(summary.Modules, func(i, j int) bool {
		return summary.Modules[i].SymbolCount > summary.Modules[j].SymbolCount
	})

	text := summary.Format()
	return summary, text
}

// BuildRepositorySummaryCompact returns a token-budgeted repository overview
// (or a single-module drill-down when module != ""). Unlike BuildRepositorySummary,
// it never emits unbounded output: modules are capped to fit the budget, the
// per-directory "Layers" chain is dropped, and entry points are capped. This keeps
// the session-opening summary tiny so it doesn't bloat the cached context.
//
// budget is an approximate token ceiling (len/4, the repo-wide convention).
func BuildRepositorySummaryCompact(ks *KnowledgeStore, budget int, module string) string {
	if ks == nil {
		return ""
	}
	if budget <= 0 {
		budget = 500
	}

	modules, err := ks.GetModules()
	if err != nil || len(modules) == 0 {
		return ""
	}

	if module != "" {
		return formatModuleDetail(ks, modules, module, budget)
	}

	// Header stats — derived from GetModules data only (no per-file disk reads).
	fileCount := 0
	testCount := 0
	totalSymbols := 0
	langMix := make(map[string]int)
	for _, mod := range modules {
		totalSymbols += mod.Symbols
		if mod.Language != "" {
			langMix[mod.Language] += len(mod.Files)
		}
		for _, f := range mod.Files {
			fileCount++
			if isTestFile(f) {
				testCount++
			}
		}
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Symbols > modules[j].Symbols
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Files: %d\n", fileCount)
	fmt.Fprintf(&b, "Symbols: %d\n", totalSymbols)
	fmt.Fprintf(&b, "Test Files: %d\n", testCount)
	if len(langMix) > 0 {
		langs := make([]string, 0, len(langMix))
		for lang, count := range langMix {
			langs = append(langs, fmt.Sprintf("%s:%d", lang, count))
		}
		sort.Strings(langs)
		fmt.Fprintf(&b, "Languages: %s\n", strings.Join(langs, ", "))
	}

	b.WriteString("Modules:\n")
	shown := 0
	for i, mod := range modules {
		// Always show at least one module; otherwise stop once over budget and
		// roll the remainder into a single line.
		if shown > 0 && b.Len()/4 >= budget {
			moreFiles, moreSymbols := 0, 0
			for _, rest := range modules[i:] {
				moreFiles += len(rest.Files)
				moreSymbols += rest.Symbols
			}
			fmt.Fprintf(&b, "  +%d more modules (%d files, %d symbols)\n", len(modules)-shown, moreFiles, moreSymbols)
			break
		}
		fmt.Fprintf(&b, "  %s (%s) - %d files, %d symbols\n", mod.Name, mod.Language, len(mod.Files), mod.Symbols)
		// Compute top symbols only for displayed modules.
		top := extractTopSymbols(ks, mod)
		if len(top) > 3 {
			top = top[:3]
		}
		if len(top) > 0 {
			fmt.Fprintf(&b, "    Symbols: %s\n", strings.Join(top, ", "))
		}
		shown++
	}

	// Entry points, capped.
	entries := extractArchitectureEntries(modules)
	if len(entries) > 0 {
		const maxEntries = 5
		head := entries
		if len(head) > maxEntries {
			head = head[:maxEntries]
		}
		fmt.Fprintf(&b, "Entry Points: %s", strings.Join(head, ", "))
		if len(entries) > maxEntries {
			fmt.Fprintf(&b, " +%d more", len(entries)-maxEntries)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// formatModuleDetail renders a single module's files and symbols within budget.
// Match is by module name, full directory path, or path basename.
func formatModuleDetail(ks *KnowledgeStore, modules []ModuleInfo, module string, budget int) string {
	var target *ModuleInfo
	for i := range modules {
		m := &modules[i]
		if m.Name == module || m.Path == module || filepath.Base(m.Path) == module {
			target = m
			break
		}
	}
	if target == nil {
		return fmt.Sprintf("Module %q not found. Call without a module for the repository overview.", module)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Module: %s (%s)\n", target.Path, target.Language)
	fmt.Fprintf(&b, "Files: %d, Symbols: %d\n", len(target.Files), target.Symbols)

	b.WriteString("Files:\n")
	const maxFiles = 20
	shownFiles := 0
	for _, f := range target.Files {
		if shownFiles >= maxFiles || b.Len()/4 >= budget {
			break
		}
		fmt.Fprintf(&b, "  %s\n", f)
		shownFiles++
	}
	if shownFiles < len(target.Files) {
		fmt.Fprintf(&b, "  +%d more files\n", len(target.Files)-shownFiles)
	}

	// Gather symbols from the displayed files, then fit them to the budget.
	var symbols []string
	for i, f := range target.Files {
		if i >= maxFiles {
			break
		}
		fs, err := ks.GetFileSummary(f)
		if err != nil {
			continue
		}
		symbols = append(symbols, fs.Functions...)
		symbols = append(symbols, fs.Classes...)
	}
	if line := joinWithinBudget(symbols, budget-b.Len()/4); line != "" {
		fmt.Fprintf(&b, "Symbols: %s\n", line)
	}

	return b.String()
}

// joinWithinBudget joins items with ", " up to an approximate token budget
// (len/4, the repo-wide convention), appending "+N more" when it truncates.
func joinWithinBudget(items []string, tokenBudget int) string {
	if tokenBudget <= 0 || len(items) == 0 {
		return ""
	}
	maxChars := tokenBudget * 4
	var b strings.Builder
	n := 0
	for _, it := range items {
		addition := len(it)
		if n > 0 {
			addition += 2 // ", "
		}
		if b.Len()+addition > maxChars {
			fmt.Fprintf(&b, " +%d more", len(items)-n)
			break
		}
		if n > 0 {
			b.WriteString(", ")
		}
		b.WriteString(it)
		n++
	}
	return b.String()
}

func (rs *RepositorySummary) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Files: %d\n", rs.FileCount)
	fmt.Fprintf(&b, "Symbols: %d\n", rs.SymbolCount)
	fmt.Fprintf(&b, "Test Files: %d\n", rs.TestFiles)

	if len(rs.LanguageMix) > 0 {
		var langs []string
		for lang, count := range rs.LanguageMix {
			langs = append(langs, fmt.Sprintf("%s:%d", lang, count))
		}
		sort.Strings(langs)
		fmt.Fprintf(&b, "Languages: %s\n", strings.Join(langs, ", "))
	}

	b.WriteString("Modules:\n")
	for _, m := range rs.Modules {
		fmt.Fprintf(&b, "  %s (%s) - %d files, %d symbols\n", m.Name, m.Language, m.FileCount, m.SymbolCount)
		if len(m.TopSymbols) > 0 {
			top := m.TopSymbols
			if len(top) > 3 {
				top = top[:3]
			}
			fmt.Fprintf(&b, "    Symbols: %s\n", strings.Join(top, ", "))
		}
	}

	if len(rs.Architecture.Layers) > 0 {
		fmt.Fprintf(&b, "Layers: %s\n", strings.Join(rs.Architecture.Layers, " -> "))
	}
	if len(rs.Architecture.Entries) > 0 {
		fmt.Fprintf(&b, "Entry Points: %s\n", strings.Join(rs.Architecture.Entries, ", "))
	}

	return b.String()
}

func extractLayers(modules []ModuleInfo) []string {
	var layers []string
	seen := make(map[string]bool)
	for _, m := range modules {
		dir := m.Path
		if !seen[dir] {
			seen[dir] = true
			layers = append(layers, dir)
		}
	}
	return layers
}

func extractArchitectureEntries(modules []ModuleInfo) []string {
	var entries []string
	for _, m := range modules {
		for _, f := range m.Files {
			base := strings.ToLower(filepath.Base(f))
			if base == "main.go" || base == "main.py" || base == "index.js" || base == "app.go" || base == "cmd.go" || strings.HasPrefix(base, "main.") {
				entries = append(entries, f)
			}
		}
	}
	return entries
}

func extractTopSymbols(ks *KnowledgeStore, mod ModuleInfo) []string {
	var symbols []string
	for _, f := range mod.Files {
		fs, err := ks.GetFileSummary(f)
		if err != nil {
			continue
		}
		symbols = append(symbols, fs.Functions...)
		symbols = append(symbols, fs.Classes...)
	}
	if len(symbols) > 5 {
		symbols = symbols[:5]
	}
	return symbols
}

func isTestFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") || strings.HasSuffix(base, "_test.js") || strings.HasSuffix(base, "_test.ts") || strings.HasSuffix(base, "test.java") || strings.HasSuffix(base, "spec.rb") || strings.Contains(base, "test_") || strings.Contains(base, "_spec.")
}
