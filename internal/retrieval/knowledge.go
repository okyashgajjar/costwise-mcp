package retrieval

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/okyashgajjar/costaffective-mcp/internal/treesitter"
)

// FileSummary provides a summary of a single file's code structure.
type FileSummary struct {
	Path       string   `json:"path"`
	Language   string   `json:"language"`
	Lines      int      `json:"lines"`
	Symbols    int      `json:"symbols"`
	Functions  []string `json:"functions"`
	Classes    []string `json:"classes"`
	Imports    int      `json:"imports"`
	References int      `json:"references"`
}

// ModuleInfo describes a logical module (directory) in the repository.
type ModuleInfo struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Files    []string `json:"files"`
	Symbols  int      `json:"symbols"`
	Language string   `json:"language"`
}

// KnowledgeStore provides a read-only view over the symbol DB
// for building repository-level context. It groups symbols by
// file and directory to produce summaries without re-parsing.
// Used by Chat, Plan, and Agent modes to understand repo structure.
type KnowledgeStore struct {
	db       *treesitter.SymbolDB
	repoRoot string
}

// NewKnowledgeStore creates a new knowledge store over an existing symbol DB.
func NewKnowledgeStore(db *treesitter.SymbolDB, repoRoot string) *KnowledgeStore {
	return &KnowledgeStore{db: db, repoRoot: repoRoot}
}

// GetFileSummary returns a summary for a single file.
func (ks *KnowledgeStore) GetFileSummary(path string) (*FileSummary, error) {
	symbols, err := ks.db.GetFileSymbols(path)
	if err != nil {
		return nil, err
	}

	summary := &FileSummary{
		Path:    path,
		Symbols: len(symbols),
	}

	for _, sym := range symbols {
		if summary.Language == "" && sym.Language != "" {
			summary.Language = sym.Language
		}
		switch sym.Kind {
		case treesitter.SymbolFunction, treesitter.SymbolMethod:
			summary.Functions = append(summary.Functions, sym.Name)
		case treesitter.SymbolClass, treesitter.SymbolStruct, treesitter.SymbolInterface:
			summary.Classes = append(summary.Classes, sym.Name)
		case treesitter.SymbolImport:
			summary.Imports++
		}
	}

	// Count lines from file on disk
	fullPath := filepath.Join(ks.repoRoot, path)
	if data, err := os.ReadFile(fullPath); err == nil {
		summary.Lines = strings.Count(string(data), "\n") + 1
	}

	return summary, nil
}

// GetAllFileSummaries returns summaries for all indexed files.
func (ks *KnowledgeStore) GetAllFileSummaries() ([]FileSummary, error) {
	files, err := ks.db.GetAllFiles()
	if err != nil {
		return nil, err
	}

	var summaries []FileSummary
	for _, f := range files {
		s, err := ks.GetFileSummary(f)
		if err != nil {
			continue
		}
		summaries = append(summaries, *s)
	}
	return summaries, nil
}

// GetSymbolOwner returns the file path that defines a given symbol.
func (ks *KnowledgeStore) GetSymbolOwner(symbolName string) (string, error) {
	matches, err := ks.db.Search(symbolName, 1)
	if err != nil || len(matches) == 0 {
		return "", err
	}
	return matches[0].Symbol.File, nil
}

// GetModules returns module-level information grouped by directory.
func (ks *KnowledgeStore) GetModules() ([]ModuleInfo, error) {
	files, err := ks.db.GetAllFiles()
	if err != nil {
		return nil, err
	}

	// Group files by directory
	dirFiles := make(map[string][]string)
	for _, f := range files {
		dir := filepath.Dir(f)
		dirFiles[dir] = append(dirFiles[dir], f)
	}

	var modules []ModuleInfo
	for dir, fileList := range dirFiles {
		mod := ModuleInfo{
			Name:  filepath.Base(dir),
			Path:  dir,
			Files: fileList,
		}

		// Count symbols and detect primary language
		langCount := make(map[string]int)
		for _, f := range fileList {
			symbols, err := ks.db.GetFileSymbols(f)
			if err != nil {
				continue
			}
			mod.Symbols += len(symbols)
			for _, sym := range symbols {
				if sym.Language != "" {
					langCount[sym.Language]++
				}
			}
		}

		// Pick most common language
		maxCount := 0
		for lang, count := range langCount {
			if count > maxCount {
				maxCount = count
				mod.Language = lang
			}
		}

		modules = append(modules, mod)
	}

	return modules, nil
}

// GetModuleForFile returns the module info for the directory containing a file.
func (ks *KnowledgeStore) GetModuleForFile(path string) (*ModuleInfo, error) {
	dir := filepath.Dir(path)
	modules, err := ks.GetModules()
	if err != nil {
		return nil, err
	}
	for _, mod := range modules {
		if mod.Path == dir {
			return &mod, nil
		}
	}
	return nil, nil
}
