package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/okyashgajjar/costaffective-mcp/internal/classifier"
	"github.com/okyashgajjar/costaffective-mcp/internal/repository"
	"github.com/okyashgajjar/costaffective-mcp/internal/kmemory"
	"github.com/okyashgajjar/costaffective-mcp/internal/treesitter"
)

type AutoRetriever struct {
	repo       *repository.RepositoryInfo
	storageDir string
	retrievers map[string]Retriever

	lastClass   classifier.Classification
	lastResults []RetrievalResult
	metrics     RetrievalMetrics
}

func NewAutoRetriever(storageDir string) *AutoRetriever {
	return &AutoRetriever{
		storageDir: storageDir,
		retrievers: make(map[string]Retriever),
	}
}

func (r *AutoRetriever) Name() string {
	return "auto"
}

func (r *AutoRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo

	// Run a single SharedIndexer with all parse flags enabled.
	// This replaces the separate indexing each retriever would do.
	db, err := treesitter.NewSymbolDB(repo.Root)
	if err != nil {
		return fmt.Errorf("auto retriever DB init: %w", err)
	}

	indexer := NewSharedIndexer(db, repo.Root, IndexConfig{
		ParseSymbols:    true,
		ParseReferences: true,
		ParseCalls:      true,
	})
	if _, err := indexer.Index(ctx); err != nil {
		db.Close()
		return fmt.Errorf("auto retriever index: %w", err)
	}

	// Initialize sub-retrievers with the pre-indexed DB where possible.
	// Retrievers with InitializeWithIndexer skip redundant indexing.
	tsRet := NewSymbolRetriever()
	tsRet.InitializeWithIndexer(repo, db)
	r.retrievers["treesitter"] = tsRet

	refRet := NewReferenceRetriever()
	refRet.InitializeWithIndexer(repo, db)
	r.retrievers["reference"] = refRet

	cgRet := NewCallGraphRetriever()
	cgRet.InitializeWithIndexer(repo, db)
	r.retrievers["callgraph"] = cgRet

	// Non-symbol retrievers still initialize normally
	otherCandidates := map[string]Retriever{
		"grep":         NewGrepRetriever(),
		"fts":          NewFTSRetriever(r.storageDir),
		"architecture": NewArchitectureRetriever(),
		"flowgraph":    NewFlowGraphRetriever(),
	}

	for name, ret := range otherCandidates {
		if err := ret.Initialize(ctx, repo); err != nil {
			continue
		}
		r.retrievers[name] = ret
	}

	return nil
}

func (r *AutoRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	cl := classifier.Classify(query)
	r.lastClass = cl

	route := map[classifier.QueryClass]string{
		classifier.SymbolQuery:     "treesitter",
		classifier.TextQuery:       "fts",
		classifier.RepositoryQuery: "grep",
		classifier.ReferenceQuery:  "reference",
		classifier.CallQuery:       "callgraph",
		classifier.ArchitectureQuery: "architecture",
		classifier.FlowQuery:       "flowgraph",
	}
	fallbackOrder := map[classifier.QueryClass][]string{
		classifier.SymbolQuery:     {"fts", "grep"},
		classifier.TextQuery:       {"grep", "treesitter"},
		classifier.RepositoryQuery: {"fts", "treesitter"},
		classifier.ReferenceQuery:  {"treesitter", "grep"},
		classifier.CallQuery:       {"treesitter", "reference"},
		classifier.ArchitectureQuery: {"grep", "fts"},
		classifier.FlowQuery:       {"callgraph", "treesitter"},
	}

	primaryName := route[cl.Class]
	var results []RetrievalResult

	if ret, ok := r.retrievers[primaryName]; ok {
		res, err := ret.Retrieve(ctx, query)
		if err == nil {
			results = append(results, res...)
		}
	}

	primaryConf := resultConfidence(results)
	if primaryConf < 0.3 || (len(results) < 2 && totalMatchHits(results) < 2) {
		if fallback, ok := fallbackOrder[cl.Class]; ok {
			for _, name := range fallback {
				if ret, ok2 := r.retrievers[name]; ok2 {
					res, err := ret.Retrieve(ctx, query)
					if err == nil {
						results = append(results, res...)
					}
				}
			}
		}
	}

	if len(results) == 0 {
		for _, name := range []string{"grep", "fts", "treesitter"} {
			if name == primaryName {
				continue
			}
			if ret, ok := r.retrievers[name]; ok {
				res, err := ret.Retrieve(ctx, query)
				if err == nil && len(res) > 0 {
					results = append(results, res...)
					break
				}
			}
		}
	}

	results = dedupeResults(results)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > 10 {
		results = results[:10]
	}

	r.lastResults = results

	r.metrics = RetrievalMetrics{
		FilesScanned:             countUniqueFiles(results),
		FilesLoaded:              len(results),
		Tokens:                   totalTokens(results),
		LatencyMs:                time.Since(start).Milliseconds(),
		MatchedFiles:             len(results),
		MatchedSnippets:          len(results),
		MatchCount:               totalMatchHits(results),
		Confidence:               cl.Confidence,
		ClassificationLabel:      cl.Label,
		ClassificationConfidence: cl.Confidence,
	}

	return results, nil
}

func (r *AutoRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *AutoRetriever) Shutdown() error {
	var errs []string
	for name, ret := range r.retrievers {
		if err := ret.Shutdown(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func dedupeResults(results []RetrievalResult) []RetrievalResult {
	seen := make(map[string]int)
	var deduped []RetrievalResult
	for _, r := range results {
		idx, exists := seen[r.File]
		if !exists {
			seen[r.File] = len(deduped)
			deduped = append(deduped, r)
		} else if r.Score > deduped[idx].Score {
			deduped[idx] = r
		}
	}
	return deduped
}

func totalMatchHits(results []RetrievalResult) int {
	n := 0
	for _, r := range results {
		n += r.MatchHits
	}
	return n
}

func resultConfidence(results []RetrievalResult) float64 {
	if len(results) == 0 {
		return 0
	}
	return results[0].Score
}

func (r *AutoRetriever) Learn(km *kmemory.KnowledgeMemory, query string, results []RetrievalResult) {
	if km == nil || len(results) == 0 {
		return
	}
	LearnFromResults(km, query, results)
}

func (r *AutoRetriever) LastResults() []RetrievalResult {
	return r.lastResults
}
