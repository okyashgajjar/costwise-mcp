package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/repository"
	"github.com/okyashgajjar/costwise-mcp/internal/treesitter"
)

type CallGraphRetriever struct {
	repo    *repository.RepositoryInfo
	db      *treesitter.SymbolDB
	metrics RetrievalMetrics
}

func NewCallGraphRetriever() *CallGraphRetriever {
	return &CallGraphRetriever{}
}

func (r *CallGraphRetriever) Name() string {
	return "callgraph"
}

func (r *CallGraphRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.metrics = RetrievalMetrics{}

	db, err := treesitter.NewSymbolDB(repo.Root)
	if err != nil {
		return fmt.Errorf("callgraph DB init: %w", err)
	}
	r.db = db

	indexer := NewSharedIndexer(db, repo.Root, IndexConfig{
		ParseSymbols:    true,
		ParseReferences: true,
		ParseCalls:      true,
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
func (r *CallGraphRetriever) InitializeWithIndexer(repo *repository.RepositoryInfo, db *treesitter.SymbolDB) {
	r.repo = repo
	r.db = db
	r.metrics = RetrievalMetrics{}
}

func (r *CallGraphRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	symbolName := extractCallSymbol(query)
	if symbolName == "" {
		symbolName = query
	}

	edges, _ := r.db.SearchCallEdges(symbolName)
	if len(edges) == 0 {
		edges, _ = r.db.SearchCallEdgesLike(symbolName)
	}

	symbolMatches, _ := r.db.Search(symbolName, 5)

	if len(edges) == 0 && len(symbolMatches) == 0 {
		r.metrics = RetrievalMetrics{
			LatencyMs:  time.Since(start).Milliseconds(),
			Confidence: 0,
		}
		return nil, nil
	}

	calleeDefs := gatherCalleeDefs(symbolMatches)

	callerSections := gatherCallerSections(edges, r.repo)

	var lines []string
	if calleeDefs != "" {
		lines = append(lines, calleeDefs)
	}

	callerCount := len(callerSections)
	callSiteCount := len(edges)

	totalCallers := 0
	for _, cg := range callerSections {
		totalCallers += cg.count
	}

	if callerCount > 0 {
		summary := fmt.Sprintf("\nCall sites found (%d callers, %d calls, %d files):\n", callerCount, callSiteCount, countCallerFiles(edges))
		lines = append(lines, summary)
		for _, cg := range callerSections {
			lines = append(lines, fmt.Sprintf("\n  %s:", cg.file))
			for _, call := range cg.calls {
				ctxLine := call.context
				if ctxLine == "" {
					ctxLine = fmt.Sprintf("line %d", call.line)
				}
				lines = append(lines, fmt.Sprintf("    %s (line %d):  %s", call.caller, call.line, ctxLine))
			}
		}
	}

	snippet := strings.Join(lines, "\n")

	allFiles := make(map[string]bool)
	for _, cg := range callerSections {
		for _, call := range cg.calls {
			allFiles[call.file] = true
		}
	}
	var fileList []string
	for f := range allFiles {
		fileList = append(fileList, f)
	}
	sort.Strings(fileList)

	confidence := 0.0
	if callSiteCount > 0 || callerCount > 0 {
		confidence = 0.35 + 0.25*float64(min(callSiteCount, 10))/10.0
		if confidence > 0.60 {
			confidence = 0.60
		}
	}

	reason := fmt.Sprintf("callers=%d,calls=%d,files=%d",
		callerCount, callSiteCount, len(fileList))

	results := []RetrievalResult{
		{
			File:      fmt.Sprintf("callgraph:%s", symbolName),
			Snippet:   snippet,
			Score:     confidence,
			Tokens:    len(snippet) / 4,
			MatchHits: callSiteCount,
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
		MatchCount:               callSiteCount,
		Confidence:               confidence,
		ClassificationLabel:      "callgraph",
		ClassificationConfidence: confidence,
	}

	return results, nil
}

func (r *CallGraphRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *CallGraphRetriever) Shutdown() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

type callerGroup struct {
	file  string
	calls []callerInfo
	count int
}

type callerInfo struct {
	caller  string
	file    string
	line    int
	context string
}

type callerFileKey struct {
	file       string
	callerName string
}

func gatherCalleeDefs(symbolMatches []treesitter.SymbolMatch) string {
	if len(symbolMatches) == 0 {
		return ""
	}
	var defLines []string
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
	}
	return "Definition:\n" + strings.Join(defLines, "\n")
}

func gatherCallerSections(edges []treesitter.CallEdge, repo *repository.RepositoryInfo) []callerGroup {
	groupMap := make(map[callerFileKey][]treesitter.CallEdge)
	for _, e := range edges {
		key := callerFileKey{file: e.File, callerName: e.CallerName}
		groupMap[key] = append(groupMap[key], e)
	}

	var groups []callerGroup
	for key, edgeList := range groupMap {
		var calls []callerInfo
		for _, e := range edgeList {
			context := extractCtxLine(repo.Root, e.File, e.Line)
			calls = append(calls, callerInfo{
				caller:  key.callerName,
				file:    e.File,
				line:    e.Line,
				context: context,
			})
		}
		groups = append(groups, callerGroup{
			file:  key.file,
			calls: calls,
			count: len(calls),
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].file != groups[j].file {
			return groups[i].file < groups[j].file
		}
		if len(groups[i].calls) > 0 && len(groups[j].calls) > 0 {
			return groups[i].calls[0].caller < groups[j].calls[0].caller
		}
		return false
	})

	return groups
}

func countCallerFiles(edges []treesitter.CallEdge) int {
	files := make(map[string]bool)
	for _, e := range edges {
		files[e.File] = true
	}
	return len(files)
}

func extractCtxLine(repoRoot, filePath string, line int) string {
	fullPath := filePath
	if !filepath.IsAbs(fullPath) && repoRoot != "" {
		fullPath = filepath.Join(repoRoot, fullPath)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if line > 0 && line <= len(lines) {
		ctx := strings.TrimSpace(lines[line-1])
		if len(ctx) > 120 {
			ctx = ctx[:120] + "..."
		}
		return ctx
	}
	return ""
}

func extractCallSymbol(query string) string {
	query = strings.TrimSpace(query)
	lower := strings.ToLower(query)

	prefixes := []string{
		"who calls ", "find callers of ", "show callers of ",
		"show call sites of ", "trace ",
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
