package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/index"
	"github.com/okyashgajjar/costwise-mcp/internal/treesitter"
)

// IndexConfig controls which parse stages the SharedIndexer runs.
type IndexConfig struct {
	ParseSymbols    bool
	ParseReferences bool
	ParseCalls      bool
}

// IndexResult holds the counts from an indexing run.
type IndexResult struct {
	Changed   int   `json:"changed"`
	Skipped   int   `json:"skipped"`
	Deleted   int   `json:"deleted"`
	Total     int   `json:"total"`
	LatencyMs int64 `json:"latency_ms"`
}

// maxIndexedFileSize caps per-file reads during indexing so a malicious or
// bloated repo can't OOM the indexer. Mirrors grep.go's 1 MB content cap.
const maxIndexedFileSize = 1 << 20 // 1 MiB

// SharedIndexer provides a single indexing pass for all retrievers,
// replacing the duplicate indexRepo() methods in treesitter.go, reference.go,
// and callgraph.go. It uses proper SHA256 hashing for file change detection
// and supports incremental updates.
type SharedIndexer struct {
	db       *treesitter.SymbolDB
	semantic *index.SemanticIndexer
	repoRoot string
	config   IndexConfig
}

// NewSharedIndexer creates a new SharedIndexer.
func NewSharedIndexer(db *treesitter.SymbolDB, repoRoot string, cfg IndexConfig) *SharedIndexer {
	semIdx, _ := index.NewSemanticIndexer(filepath.Join(repoRoot, ".mycli-fts"))
	return &SharedIndexer{
		db:       db,
		semantic: semIdx,
		repoRoot: repoRoot,
		config:   cfg,
	}
}

// Close releases resources.
func (idx *SharedIndexer) Close() {
	if idx.semantic != nil {
		idx.semantic.Close()
	}
}

// hashFile computes a proper SHA256 hash of file content.
func hashFile(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Index performs an incremental index of the repository.
// Files whose SHA256 hash matches the stored hash are skipped.
// Files that are new or changed are parsed and stored.
// Files that were previously indexed but no longer exist are cleaned up.
func (idx *SharedIndexer) Index(ctx context.Context) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{}

	// Load existing file hashes for comparison
	existingHashes, err := idx.db.GetFilesByHash()
	if err != nil {
		existingHashes = make(map[string]string)
	}

	// Track which files we see on disk so we can detect deletions
	seenFiles := make(map[string]bool)

	err = filepath.Walk(idx.repoRoot, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if fi.IsDir() && skipDir(path) {
			return filepath.SkipDir
		}
		// Only regular files are indexed. os.ReadFile would happily follow a
		// file symlink, so a malicious repo could plant `evil.go -> /dev/zero`
		// (hang/OOM) or `notes.md -> ~/.aws/credentials` (secret content gets
		// hashed into the semantic index and surfaced by retrieval).
		if !fi.Mode().IsRegular() {
			return nil
		}
		if fi.IsDir() || fi.Size() == 0 {
			return nil
		}
		// Cap per-file read size so a huge file can't OOM the indexer; larger
		// files contribute little retrieval value. Mirrors grep.go's 1 MB cap.
		if fi.Size() > maxIndexedFileSize {
			return nil
		}
		if !treesitter.IsSupported(path) {
			return nil
		}

		relPath, _ := filepath.Rel(idx.repoRoot, path)
		seenFiles[relPath] = true
		result.Total++

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		newHash := hashFile(data)

		// Check if file is unchanged
		if oldHash, exists := existingHashes[relPath]; exists && oldHash == newHash {
			result.Skipped++
			return nil
		}

		// File is new or changed — parse and store
		lang := treesitter.DetectLanguage(path)
		parser, err := treesitter.NewParser(lang)
		if err != nil {
			return nil
		}

		if idx.config.ParseSymbols {
			symbols, err := parser.ParseFile(ctx, path)
			if err == nil {
				for i := range symbols {
					symbols[i].File = relPath
				}
				if err := idx.db.ClearFile(relPath); err != nil {
					return err
				}
				if len(symbols) > 0 {
					if err := idx.db.StoreSymbols(symbols); err != nil {
						return err
					}
				}
			}
		}

		if idx.config.ParseReferences {
			refs, err := parser.ParseReferences(ctx, path)
			if err == nil && len(refs) > 0 {
				for i := range refs {
					refs[i].File = relPath
				}
				if err := idx.db.ClearFileReferences(relPath); err != nil {
					return err
				}
				if err := idx.db.StoreReferences(refs); err != nil {
					return err
				}
			}
		}

		if idx.config.ParseCalls {
			calls, err := parser.ParseCalls(ctx, path)
			if err == nil && len(calls) > 0 {
				for i := range calls {
					calls[i].File = relPath
					calls[i].CallerFile = relPath
				}
				if err := idx.db.ClearFileCallEdges(relPath); err != nil {
					return err
				}
				if err := idx.db.StoreCallEdges(calls); err != nil {
					return err
				}
			}
		}

		if err := idx.db.MarkFileIndexed(relPath, newHash); err != nil {
			return err
		}

		if idx.semantic != nil {
			// Basic AST extraction for docstrings
			docstrings := ""
			// Removed sym.Doc as treesitter.Symbol doesn't support it yet.
			_ = idx.semantic.IndexFile(relPath, string(data), docstrings)
		}

		result.Changed++
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("indexing walk failed: %w", err)
	}

	// Clean up files that were previously indexed but no longer exist on disk
	for oldFile := range existingHashes {
		if !seenFiles[oldFile] {
			if err := idx.db.ClearFile(oldFile); err != nil {
				return result, err
			}
			if err := idx.db.ClearFileReferences(oldFile); err != nil {
				return result, err
			}
			if err := idx.db.ClearFileCallEdges(oldFile); err != nil {
				return result, err
			}
			// Remove from symbol_files tracking
			if err := idx.db.MarkFileIndexed(oldFile, ""); err != nil {
				return result, err
			}
			if idx.semantic != nil {
				_ = idx.semantic.RemoveFile(oldFile)
			}
			result.Deleted++
		}
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	return result, nil
}

// DB returns the underlying SymbolDB for reuse by retrievers.
func (idx *SharedIndexer) DB() *treesitter.SymbolDB {
	return idx.db
}

// RepoRoot returns the repository root path.
func (idx *SharedIndexer) RepoRoot() string {
	return idx.repoRoot
}
