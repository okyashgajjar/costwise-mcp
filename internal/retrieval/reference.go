package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/repository"
	"github.com/okyashgajjar/costwise-mcp/internal/treesitter"
)

type ReferenceRetriever struct {
	repo    *repository.RepositoryInfo
	db      *treesitter.SymbolDB
	metrics RetrievalMetrics
}

func NewReferenceRetriever() *ReferenceRetriever {
	return &ReferenceRetriever{}
}

func (r *ReferenceRetriever) Name() string {
	return "reference"
}

func (r *ReferenceRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.metrics = RetrievalMetrics{}

	db, err := treesitter.NewSymbolDB(repo.Root)
	if err != nil {
		return fmt.Errorf("reference DB init: %w", err)
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
func (r *ReferenceRetriever) InitializeWithIndexer(repo *repository.RepositoryInfo, db *treesitter.SymbolDB) {
	r.repo = repo
	r.db = db
	r.metrics = RetrievalMetrics{}
}

func (r *ReferenceRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	symbolName := extractReferenceSymbol(query)
	if symbolName == "" {
		symbolName = query
	}

	symbolMatches, _ := r.db.Search(symbolName, 5)

	refs, _ := r.db.SearchReferences(symbolName)
	if len(refs) == 0 {
		refs, _ = r.db.SearchReferencesLike(symbolName)
	}

	if len(symbolMatches) == 0 && len(refs) == 0 {
		r.metrics = RetrievalMetrics{
			LatencyMs:  time.Since(start).Milliseconds(),
			Confidence: 0,
		}
		return nil, nil
	}

	var defLines []string
	var refLines []string
	var impLines []string
	var expLines []string

	defFiles := make(map[string]bool)
	refFiles := make(map[string]bool)

	for _, sm := range symbolMatches {
		sig := sm.Symbol.Signature
		if sig == "" {
			sig = fmt.Sprintf("%s %s", sm.Symbol.Kind, sm.Symbol.Name)
		}
		line := fmt.Sprintf("[%s] %s (%s:%d)", sm.Symbol.Kind, sm.Symbol.Name, sm.Symbol.File, sm.Symbol.StartLine)
		if sig != line {
			line += "\n  " + sig
		}
		defLines = append(defLines, line)
		defFiles[sm.Symbol.File] = true
	}

	for _, ref := range refs {
		ctxLine := ref.Context
		if ctxLine == "" {
			ctxLine = fmt.Sprintf("line %d", ref.Line)
		}

		entry := fmt.Sprintf("  %s:%d  %s", ref.File, ref.Line, ctxLine)
		switch ref.RefType {
		case treesitter.RefDefinition:
			defLines = append(defLines, entry)
			defFiles[ref.File] = true
		case treesitter.RefReference:
			refLines = append(refLines, entry)
			refFiles[ref.File] = true
		case treesitter.RefImport:
			impLines = append(impLines, entry)
			refFiles[ref.File] = true
		case treesitter.RefExport:
			expLines = append(expLines, entry)
			refFiles[ref.File] = true
		}
	}

	var defs, refsOut, imports, exports string

	if len(defLines) > 0 {
		defs = "Definition:\n" + strings.Join(defLines, "\n")
	}
	if len(refLines) > 0 {
		refsOut = "\n\nReferences (" + fmt.Sprintf("%d", len(refLines)) + "):\n" + strings.Join(refLines, "\n")
	}
	if len(impLines) > 0 {
		imports = "\n\nImports (" + fmt.Sprintf("%d", len(impLines)) + "):\n" + strings.Join(impLines, "\n")
	}
	if len(expLines) > 0 {
		exports = "\n\nExports (" + fmt.Sprintf("%d", len(expLines)) + "):\n" + strings.Join(expLines, "\n")
	}

	snippet := defs + refsOut + imports + exports

	allFiles := make(map[string]bool)
	for f := range defFiles {
		allFiles[f] = true
	}
	for f := range refFiles {
		allFiles[f] = true
	}
	var fileList []string
	for f := range allFiles {
		fileList = append(fileList, f)
	}
	sort.Strings(fileList)

	totalRefs := len(refLines)
	totalImps := len(impLines)
	totalDefs := len(defLines)

	confidence := 0.0
	if totalDefs > 0 || totalRefs > 0 {
		confidence = 0.40 + 0.30*float64(min(totalRefs, 10))/10.0
		if confidence > 0.70 {
			confidence = 0.70
		}
	}

	reason := fmt.Sprintf("defs=%d,refs=%d,imports=%d,exports=%d,files=%d",
		totalDefs, totalRefs, totalImps, len(expLines), len(fileList))

	results := []RetrievalResult{
		{
			File:      fmt.Sprintf("reference:%s", symbolName),
			Snippet:   snippet,
			Score:     confidence,
			Tokens:    len(snippet) / 4,
			MatchHits: totalRefs + totalImps + totalDefs,
			Reason:    reason,
		},
	}

	r.metrics = RetrievalMetrics{
		FilesScanned:             len(fileList),
		FilesLoaded:              len(results),
		Tokens:                   len(snippet) / 4,
		LatencyMs:                time.Since(start).Milliseconds(),
		MatchedFiles:             len(fileList),
		MatchedSnippets:          len(results),
		MatchCount:               totalRefs + totalImps + totalDefs,
		Confidence:               confidence,
		ClassificationLabel:      "reference",
		ClassificationConfidence: confidence,
	}

	return results, nil
}

func (r *ReferenceRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *ReferenceRetriever) Shutdown() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

func extractReferenceSymbol(query string) string {
	query = strings.TrimSpace(query)
	lower := strings.ToLower(query)

	prefixes := []string{
		"who uses ", "find references to ", "show imports of ",
		"which files reference ", "find usages of ",
		"where is ", "show references to ", "who imports ",
		"find all references to ", "find all usages of ",
		"references to ", "usages of ", "imports of ",
		"who calls ", "where is ", "files that import ",
		"files referencing ", "dependents of ", "dependants of ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			candidate := strings.TrimSpace(query[len(prefix):])
			candidate = strings.TrimRight(candidate, "?.!")
			words := strings.Fields(candidate)
			if len(words) > 0 {
				return candidate
			}
		}
	}

	fields := strings.Fields(query)
	for _, w := range fields {
		w = strings.Trim(w, ".,!?;:'\"()")
		if len(w) >= 2 && w[0] >= 'A' && w[0] <= 'Z' {
			return w
		}
	}

	if len(fields) > 0 {
		return fields[len(fields)-1]
	}

	return ""
}
