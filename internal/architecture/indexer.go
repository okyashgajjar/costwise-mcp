package architecture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okyashgajjar/costaffective-mcp/internal/treesitter"
)

type Indexer struct {
	root string
	db   *DB
}

func NewIndexer(repoRoot string) (*Indexer, error) {
	db, err := NewDB(repoRoot)
	if err != nil {
		return nil, err
	}
	return &Indexer{root: repoRoot, db: db}, nil
}

func (i *Indexer) Close() error {
	return i.db.Close()
}

func (i *Indexer) DB() *DB {
	return i.db
}

func (i *Indexer) IndexFile(ctx context.Context, relPath string) (*ModuleSummary, error) {
	absPath := filepath.Join(i.root, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	content := string(data)
	h := HashContent(data)

	needs, _ := i.db.NeedsReindex(relPath, h)
	if !needs {
		return nil, nil
	}

	lang := detectLang(relPath)
	var classes, functions, exports []string
	imports := ExtractImports(content, lang)

	if treesitter.IsSupported(relPath) {
		parser, err := treesitter.NewParser(treesitter.DetectLanguage(relPath))
		if err == nil {
			symbols, err := parser.ParseFile(ctx, absPath)
			if err == nil {
				for _, s := range symbols {
					switch s.Kind {
					case treesitter.SymbolClass, treesitter.SymbolInterface, treesitter.SymbolStruct:
						if !contains(classes, s.Name) {
							classes = append(classes, s.Name)
						}
					case treesitter.SymbolFunction, treesitter.SymbolMethod:
						if !contains(functions, s.Name) {
							functions = append(functions, s.Name)
						}
					case treesitter.SymbolExport:
						if !contains(exports, s.Name) {
							exports = append(exports, s.Name)
						}
					}
				}
			}
		}
	}

	topics := ExtractTopics(classes, functions, imports, relPath)
	description := ExtractDescription(content, relPath, lang)

	summary := &ModuleSummary{
		FilePath:    relPath,
		Language:    lang,
		Classes:     classes,
		Functions:   functions,
		Imports:     imports,
		Exports:     exports,
		Topics:      topics,
		Description: description,
	}

	if err := i.db.Store(summary, h); err != nil {
		return nil, err
	}
	return summary, nil
}

func (i *Indexer) IndexRepo(ctx context.Context, progress func(file string)) (int, time.Duration, error) {
	start := time.Now()
	count := 0
	err := filepath.Walk(i.root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if fi.IsDir() || fi.Size() == 0 {
			return nil
		}
		relPath, _ := filepath.Rel(i.root, path)
		if relPath == "" || strings.HasPrefix(relPath, ".mycli/") {
			return nil
		}
		ext := filepath.Ext(relPath)
		if !supportedExt(ext) {
			return nil
		}
		if progress != nil {
			progress(relPath)
		}
		if _, err := i.IndexFile(ctx, relPath); err == nil {
			count++
		}
		return nil
	})
	return count, time.Since(start), err
}

func (i *Indexer) Stats() (string, error) {
	n, err := i.db.Count()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d modules", n), nil
}

func detectLang(relPath string) string {
	ext := filepath.Ext(relPath)
	switch ext {
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".md":
		return "markdown"
	}
	return "unknown"
}

func supportedExt(ext string) bool {
	switch ext {
	case ".py", ".go", ".js", ".jsx", ".mjs", ".ts", ".tsx":
		return true
	}
	return false
}

func shouldSkipDir(path string) bool {
	skipDirs := []string{".git", ".mycli", "node_modules", "__pycache__", "venv", ".venv", "dist", "build", ".tox", ".eggs"}
	base := filepath.Base(path)
	for _, d := range skipDirs {
		if base == d {
			return true
		}
	}
	return false
}

func contains(slice []string, s string) bool {
	for _, x := range slice {
		if x == s {
			return true
		}
	}
	return false
}
