package retrieval

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/okyashgajjar/costaffective-mcp/internal/repository"
	"github.com/okyashgajjar/costaffective-mcp/internal/treesitter"
)

type SymbolRetriever struct {
	repo    *repository.RepositoryInfo
	db      *treesitter.SymbolDB
	metrics RetrievalMetrics
	symbols []treesitter.SymbolMatch
}

func NewSymbolRetriever() *SymbolRetriever {
	return &SymbolRetriever{}
}

func (r *SymbolRetriever) Name() string {
	return "treesitter"
}

func (r *SymbolRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.metrics = RetrievalMetrics{}

	db, err := treesitter.NewSymbolDB(repo.Root)
	if err != nil {
		return fmt.Errorf("symbol DB init: %w", err)
	}
	r.db = db

	indexer := NewSharedIndexer(db, repo.Root, IndexConfig{
		ParseSymbols:    true,
		ParseReferences: true,
		ParseCalls:      false,
	})
	result, err := indexer.Index(ctx)
	if err != nil {
		return err
	}

	r.metrics = RetrievalMetrics{
		FilesScanned: result.Total,
		FilesLoaded:  result.Changed,
		LatencyMs:    result.LatencyMs,
	}

	return nil
}

// InitializeWithIndexer uses a pre-indexed DB, skipping redundant indexing.
// Used by AutoRetriever when a SharedIndexer has already run.
func (r *SymbolRetriever) InitializeWithIndexer(repo *repository.RepositoryInfo, db *treesitter.SymbolDB) {
	r.repo = repo
	r.db = db
	r.metrics = RetrievalMetrics{}
}

func (r *SymbolRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	candidates := extractSymbolCandidates(query)
	allSymbols := make(map[string]bool)
	var matches []treesitter.SymbolMatch

	for _, candidate := range candidates {
		results, err := r.db.Search(candidate, 20)
		if err != nil {
			continue
		}
		for _, m := range results {
			key := fmt.Sprintf("%s:%s:%d", m.Symbol.Name, m.Symbol.File, m.Symbol.StartLine)
			if !allSymbols[key] {
				m.Score = adjustScore(m.Score, query, m.Symbol)
				allSymbols[key] = true
				matches = append(matches, m)
			}
		}
	}

	if len(matches) == 0 {
		results, err := r.db.Search(query, 10)
		if err == nil {
			for _, m := range results {
				key := fmt.Sprintf("%s:%s:%d", m.Symbol.Name, m.Symbol.File, m.Symbol.StartLine)
				if !allSymbols[key] {
					allSymbols[key] = true
					matches = append(matches, m)
				}
			}
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	topN := 10
	if len(matches) < topN {
		topN = len(matches)
	}
	matches = matches[:topN]

	r.symbols = matches

	var results []RetrievalResult
	totalHits := 0
	for _, m := range matches {
		snippet := buildSymbolSnippet(m)
		hits := 1
		totalHits += hits
		results = append(results, RetrievalResult{
			File:      filepath.Join(r.repo.Root, m.Symbol.File),
			Snippet:   snippet,
			Score:     m.Score,
			Tokens:    len(snippet) / 4,
			LineFrom:  m.Symbol.StartLine,
			LineTo:    m.Symbol.EndLine,
			MatchHits: hits,
			Reason:    m.Reason,
		})
	}

	confidence := 0.0
	if len(results) > 0 && totalHits > 0 {
		confidence = results[0].Score
		if confidence < 0.3 {
			confidence = 0.3
		}
	}

	r.metrics = RetrievalMetrics{
		FilesScanned:    r.metrics.FilesScanned,
		FilesLoaded:     len(results),
		Tokens:          totalTokens(results),
		LatencyMs:       time.Since(start).Milliseconds(),
		MatchedFiles:    len(results),
		MatchedSnippets: len(results),
		MatchCount:      totalHits,
		Confidence:      confidence,
	}

	return results, nil
}

func (r *SymbolRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *SymbolRetriever) Shutdown() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// DB returns the underlying symbol database for reuse.
func (r *SymbolRetriever) DB() *treesitter.SymbolDB {
	return r.db
}

func extractSymbolCandidates(query string) []string {
	query = strings.TrimSpace(query)
	words := strings.Fields(query)

	var candidates []string
	seen := make(map[string]bool)

	stopWords := map[string]bool{
		"where": true, "is": true, "the": true, "a": true, "an": true,
		"find": true, "show": true, "get": true, "all": true, "implemented": true,
		"defined": true, "located": true, "tell": true, "me": true,
		"about": true, "this": true, "that": true, "in": true, "of": true,
		"for": true, "how": true, "do": true, "does": true, "what": true,
		"are": true, "which": true, "can": true, "you": true, "please": true,
		"code": true, "repository": true, "related": true,
	}

	addCandidate := func(w string) {
		lower := strings.ToLower(w)
		if len(w) >= 2 && !seen[lower] {
			candidates = append(candidates, w)
			seen[lower] = true
		}
	}

	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()[]{}/\\")
		if len(w) < 2 || stopWords[strings.ToLower(w)] {
			continue
		}
		addCandidate(w)
		stem := simpleStem(w)
		if stem != w {
			addCandidate(stem)
		}
	}

	upperWords := filterUpperWords(words)
	for _, w := range upperWords {
		addCandidate(w)
	}

	return candidates
}

func filterUpperWords(words []string) []string {
	var upper []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()[]{}/\\")
		if len(w) >= 2 && w[0] >= 'A' && w[0] <= 'Z' {
			upper = append(upper, w)
		}
	}
	return upper
}

func adjustScore(base float64, query string, sym treesitter.Symbol) float64 {
	q := strings.ToLower(query)
	name := strings.ToLower(sym.Name)

	if name == q {
		return 1.0
	}

	if strings.Contains(name, q) || strings.Contains(q, name) {
		return 0.9 + base*0.1
	}

	queryWords := strings.Fields(q)
	matched := 0
	for _, qw := range queryWords {
		if strings.Contains(name, qw) {
			matched++
		}
	}
	if len(queryWords) > 0 {
		ratio := float64(matched) / float64(len(queryWords))
		if ratio > 0.5 {
			return 0.7 + ratio*0.3
		}
	}

	return base
}

func buildSymbolSnippet(m treesitter.SymbolMatch) string {
	var b strings.Builder
	kind := string(m.Symbol.Kind)
	name := m.Symbol.Name
	file := m.Symbol.File

	fmt.Fprintf(&b, "[%s] %s (%s:%d)\n", kind, name, file, m.Symbol.StartLine)
	if m.Symbol.Signature != "" {
		fmt.Fprintf(&b, "  %s\n", m.Symbol.Signature)
	}
	if m.Reason != "" {
		fmt.Fprintf(&b, "  reason: %s\n", m.Reason)
	}

	return b.String()
}

func skipDir(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", "node_modules", "vendor", ".venv", "venv",
		"__pycache__", ".next", "dist", "build", "target",
		".idea", ".vscode", ".DS_Store", ".mycli-fts":
		return true
	}
	return false
}
