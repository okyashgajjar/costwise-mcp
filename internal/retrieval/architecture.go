package retrieval

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/architecture"
	"github.com/okyashgajjar/costwise-mcp/internal/repository"
)

type ArchitectureRetriever struct {
	repo    *repository.RepositoryInfo
	idx     *architecture.Indexer
	metrics RetrievalMetrics
}

type archScored struct {
	summary   *architecture.ModuleSummary
	score     float64
	topicH    float64
	symbolH   float64
	filenameH float64
	importH   float64
	reasons   []string
}

func NewArchitectureRetriever() *ArchitectureRetriever {
	return &ArchitectureRetriever{}
}

func (r *ArchitectureRetriever) Name() string {
	return "architecture"
}

func (r *ArchitectureRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.metrics = RetrievalMetrics{}

	idx, err := architecture.NewIndexer(repo.Root)
	if err != nil {
		return err
	}
	if _, _, err := idx.IndexRepo(ctx, nil); err != nil {
		idx.Close()
		return err
	}
	r.idx = idx
	return nil
}

func (r *ArchitectureRetriever) Shutdown() error {
	if r.idx != nil {
		return r.idx.Close()
	}
	return nil
}

func (r *ArchitectureRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *ArchitectureRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	all, err := r.idx.DB().LoadAll()
	if err != nil {
		return nil, err
	}

	filtered := make([]*architecture.ModuleSummary, 0, len(all))
	for _, s := range all {
		lower := strings.ToLower(s.FilePath)
		if strings.Contains(lower, "min.js") || strings.Contains(lower, ".min.") {
			continue
		}
		filtered = append(filtered, s)
	}
	all = filtered

	topicSeeds := queryToTopics(query)
	queryWords := tokenizeQuery(query)
	queryLower := strings.ToLower(query)

	scoredAll := make([]archScored, 0, len(all))
	for _, s := range all {
		topicScore := topicSimilarity(topicSeeds, s.Topics)
		symbolScore := symbolOverlap(queryWords, s.Classes, s.Functions)
		filenameScore := filenameRelevance(queryLower, s.FilePath)
		importScore := importRelevance(topicSeeds, queryWords, s.Imports)
		descScore := descriptionRelevance(queryLower, s.Description)

		score := 0.30*topicScore + 0.20*symbolScore + 0.30*filenameScore + 0.10*importScore + 0.10*descScore
		if score > 1.0 {
			score = 1.0
		}

		if score < 0.05 {
			continue
		}

		reasons := []string{}
		if topicScore > 0 {
			reasons = append(reasons, topicMatchReasons(topicSeeds, s.Topics)...)
		}
		if symbolScore > 0 {
			reasons = append(reasons, symbolMatchReasons(queryWords, s.Classes, s.Functions)...)
		}
		if filenameScore > 0 {
			reasons = append(reasons, "filename:"+filepath.Base(s.FilePath))
		}

		scoredAll = append(scoredAll, archScored{
			summary:   s,
			score:     score,
			topicH:    topicScore,
			symbolH:   symbolScore,
			filenameH: filenameScore,
			importH:   importScore,
			reasons:   reasons,
		})
	}

	sort.Slice(scoredAll, func(i, j int) bool {
		return scoredAll[i].score > scoredAll[j].score
	})

	if len(scoredAll) > 10 {
		scoredAll = scoredAll[:10]
	}

	results := make([]RetrievalResult, 0, len(scoredAll))
	totalTokens := 0
	for _, sc := range scoredAll {
		snippet := formatModuleSummary(sc.summary, sc.reasons)
		tok := tokenEstimate(snippet)
		totalTokens += tok

		results = append(results, RetrievalResult{
			File:      sc.summary.FilePath,
			Snippet:   snippet,
			Score:     sc.score,
			Tokens:    tok,
			LineFrom:  0,
			LineTo:    0,
			MatchHits: len(sc.summary.Classes) + len(sc.summary.Functions),
			Reason:    strings.Join(sc.reasons, ", "),
		})
	}

	if len(results) == 0 {
		confidence := 0.0
		r.metrics = RetrievalMetrics{
			FilesScanned: len(all),
			FilesLoaded:  0,
			Tokens:       0,
			LatencyMs:    time.Since(start).Milliseconds(),
			Confidence:   confidence,
		}
		return results, nil
	}

	confidence := computeArchitectureConfidence(scoredAll)
	r.metrics = RetrievalMetrics{
		FilesScanned: len(all),
		FilesLoaded:  len(results),
		Tokens:       totalTokens,
		LatencyMs:    time.Since(start).Milliseconds(),
		MatchedFiles: len(results),
		MatchCount:   len(results),
		Confidence:   confidence,
	}
	return results, nil
}

var architectureTopicHints = map[string][]string{
	"error":       {"error handling", "failures", "exceptions", "retry logic"},
	"errors":      {"error handling", "failures", "exceptions", "retry logic"},
	"exception":   {"error handling", "exceptions", "failures"},
	"exceptions":  {"error handling", "exceptions", "failures"},
	"retry":       {"retry logic", "resilience"},
	"fail":        {"failures", "error handling"},
	"failure":     {"failures", "error handling"},
	"cache":       {"caching", "performance"},
	"caching":     {"caching", "performance"},
	"cached":      {"caching", "performance"},
	"prompt":      {"prompts", "LLM"},
	"prompts":     {"prompts", "LLM"},
	"chat":        {"chat", "conversation", "messaging"},
	"repo":        {"repository", "git", "source control"},
	"repomap":     {"repository mapping", "graph", "indexing"},
	"map":         {"mapping", "graph", "indexing"},
	"model":       {"LLM", "language model"},
	"models":      {"LLM", "language model"},
	"llm":         {"LLM", "language model"},
	"edit":        {"editing", "code modification", "diffs"},
	"editing":     {"editing", "code modification", "diffs"},
	"diff":        {"diffs", "comparison", "patches"},
	"history":     {"history", "conversation log"},
	"voice":       {"voice input", "audio"},
	"analytics":   {"analytics", "telemetry"},
	"help":        {"help system", "documentation"},
	"config":      {"configuration", "settings"},
	"startup":     {"entry point", "startup", "initialization"},
	"test":        {"testing"},
	"token":       {"tokens", "tokenization"},
	"context":     {"context", "context window"},
	"coder":       {"AI coding agent", "code generation"},
	"copy":        {"clipboard"},
	"paste":       {"clipboard"},
	"version":     {"version check"},
	"architect":   {"architecture mode", "planning", "design"},
	"system":      {"system design", "architecture"},
	"workflow":    {"workflow", "lifecycle"},
	"mechanism":   {"mechanism", "design", "architecture"},
	"design":      {"design", "architecture"},
	"feature":     {"feature", "capability"},
	"integration": {"integration", "API"},
	"data":        {"data", "model", "persistence"},
	"schema":      {"schema", "data model"},
	"migration":   {"migration", "database"},
	"plugin":      {"plugin", "extension"},
	"extension":   {"plugin", "extension"},
	"command":     {"CLI", "command line"},
	"service":     {"service", "API"},
	"worker":      {"worker", "background", "concurrency"},
	"queue":       {"queue", "async", "background"},
	"async":       {"async", "background"},
	"http":        {"HTTP", "API", "REST"},
	"api":         {"API", "HTTP"},
}

func queryToTopics(query string) []string {
	q := strings.ToLower(query)
	seen := make(map[string]bool)
	var topics []string
	add := func(t string) {
		if !seen[t] {
			seen[t] = true
			topics = append(topics, t)
		}
	}
	for word, hints := range architectureTopicHints {
		if strings.Contains(q, word) {
			for _, h := range hints {
				add(h)
			}
		}
	}
	return topics
}

func tokenizeQuery(query string) []string {
	q := strings.ToLower(query)
	words := strings.FieldsFunc(q, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true, "how": true,
		"does": true, "do": true, "what": true, "explain": true, "describe": true,
		"of": true, "in": true, "to": true, "and": true, "or": true, "this": true,
		"that": true, "it": true, "be": true, "for": true, "with": true, "as": true,
	}
	var out []string
	for _, w := range words {
		if !stop[w] && len(w) > 1 {
			out = append(out, w)
		}
	}
	return out
}

func topicSimilarity(queryTopics, moduleTopics []string) float64 {
	if len(queryTopics) == 0 {
		return 0
	}
	qset := make(map[string]bool, len(queryTopics))
	for _, t := range queryTopics {
		qset[t] = true
	}
	hits := 0
	for _, t := range moduleTopics {
		if qset[t] {
			hits++
		}
	}
	return float64(hits) / float64(len(queryTopics))
}

func symbolOverlap(queryWords, classes, functions []string) float64 {
	if len(queryWords) == 0 {
		return 0
	}
	hits := 0
	seen := make(map[string]bool)
	for _, q := range queryWords {
		qLower := strings.ToLower(q)
		if len(qLower) < 3 {
			continue
		}
		matched := false
		for _, c := range classes {
			cLower := strings.ToLower(c)
			if cLower == qLower || strings.Contains(cLower, qLower) || strings.Contains(qLower, cLower) {
				if !seen["class:"+c] {
					seen["class:"+c] = true
					matched = true
				}
			}
		}
		for _, f := range functions {
			fLower := strings.ToLower(f)
			if fLower == qLower || strings.Contains(fLower, qLower) || strings.Contains(qLower, fLower) {
				if !seen["func:"+f] {
					seen["func:"+f] = true
					matched = true
				}
			}
		}
		if matched {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(queryWords))
}

func filenameRelevance(queryLower, filePath string) float64 {
	base := strings.ToLower(filepath.Base(filePath))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	dir := strings.ToLower(filepath.Dir(filePath))
	combined := base + " " + dir

	qWords := strings.FieldsFunc(queryLower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	directHits := 0
	synonymHits := 0
	for _, w := range qWords {
		if len(w) <= 2 {
			continue
		}
		if strings.Contains(combined, w) {
			directHits++
			continue
		}
		matched := false
		for related, primary := range filenameSynonyms {
			if w == primary && strings.Contains(combined, related) {
				matched = true
				break
			}
			if w == related && strings.Contains(combined, primary) {
				matched = true
				break
			}
		}
		if matched {
			synonymHits++
		}
	}
	if directHits == 0 && synonymHits == 0 {
		return 0
	}
	score := 0.5*float64(directHits) + 0.7*float64(synonymHits)
	return score
}

func descriptionRelevance(queryLower, description string) float64 {
	if description == "" {
		return 0
	}
	descLower := strings.ToLower(description)
	qWords := strings.FieldsFunc(queryLower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	hits := 0
	for _, w := range qWords {
		if len(w) <= 2 {
			continue
		}
		if strings.Contains(descLower, w) {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(qWords))
}

var filenameSynonyms = map[string]string{
	"error":      "exception",
	"errors":     "exception",
	"exception":  "error",
	"exceptions": "error",
	"fail":       "exception",
	"failure":    "exception",
	"cache":      "cached",
	"cached":     "cache",
	"repo":       "repository",
	"repository": "repo",
	"auth":       "login",
	"login":      "auth",
	"db":         "database",
	"database":   "db",
	"ui":         "interface",
	"interface":  "ui",
	"kafka":      "queue",
	"queue":      "kafka",
	"http":       "api",
	"api":        "http",
	"user":       "account",
	"account":    "user",
}

func importRelevance(queryTopics, queryWords, imports []string) float64 {
	if len(imports) == 0 {
		return 0
	}
	hit := false
	for _, imp := range imports {
		impLower := strings.ToLower(imp)
		for _, qt := range queryTopics {
			if strings.Contains(impLower, qt) || strings.Contains(qt, impLower) {
				hit = true
			}
		}
		for _, qw := range queryWords {
			if strings.Contains(impLower, qw) || strings.Contains(qw, impLower) {
				hit = true
			}
		}
	}
	if !hit {
		return 0
	}
	return 1.0
}

func topicMatchReasons(queryTopics, moduleTopics []string) []string {
	qset := make(map[string]bool, len(queryTopics))
	for _, t := range queryTopics {
		qset[t] = true
	}
	var out []string
	seen := make(map[string]bool)
	for _, t := range moduleTopics {
		if qset[t] && !seen[t] {
			seen[t] = true
			out = append(out, "topic:"+t)
		}
	}
	return out
}

func symbolMatchReasons(queryWords []string, classes, functions []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, q := range queryWords {
		qLower := strings.ToLower(q)
		for _, c := range classes {
			cLower := strings.ToLower(c)
			if (cLower == qLower || strings.Contains(cLower, qLower) || strings.Contains(qLower, cLower)) && !seen[c] {
				seen[c] = true
				out = append(out, "class:"+c)
			}
		}
		for _, f := range functions {
			fLower := strings.ToLower(f)
			if (fLower == qLower || strings.Contains(fLower, qLower) || strings.Contains(qLower, fLower)) && !seen[f] {
				seen[f] = true
				out = append(out, "func:"+f)
			}
		}
	}
	return out
}

func formatModuleSummary(s *architecture.ModuleSummary, reasons []string) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(s.FilePath)
	b.WriteString("\n")
	if s.Description != "" {
		b.WriteString(s.Description)
		b.WriteString("\n")
	}
	if len(s.Classes) > 0 {
		b.WriteString("Classes: ")
		b.WriteString(strings.Join(s.Classes, ", "))
		b.WriteString("\n")
	}
	if len(s.Functions) > 0 {
		funcs := s.Functions
		if len(funcs) > 6 {
			funcs = funcs[:6]
		}
		b.WriteString("Functions: ")
		b.WriteString(strings.Join(funcs, ", "))
		if len(s.Functions) > 6 {
			b.WriteString(", ...")
		}
		b.WriteString("\n")
	}
	if len(s.Imports) > 0 {
		imps := s.Imports
		if len(imps) > 5 {
			imps = imps[:5]
		}
		b.WriteString("Imports: ")
		b.WriteString(strings.Join(imps, ", "))
		b.WriteString("\n")
	}
	if len(s.Topics) > 0 {
		topics := s.Topics
		if len(topics) > 5 {
			topics = topics[:5]
		}
		b.WriteString("Topics: ")
		b.WriteString(strings.Join(topics, ", "))
		b.WriteString("\n")
	}
	if len(reasons) > 0 {
		rs := reasons
		if len(rs) > 3 {
			rs = rs[:3]
		}
		b.WriteString("Matched: ")
		b.WriteString(strings.Join(rs, "; "))
		b.WriteString("\n")
	}
	return b.String()
}

func computeArchitectureConfidence(scored []archScored) float64 {
	if len(scored) == 0 {
		return 0
	}
	top := scored[0]
	if top.score < 0.10 {
		return 0.2
	}
	if top.score >= 0.80 {
		return 0.95
	}
	if top.score >= 0.50 {
		return 0.80
	}
	if top.score >= 0.30 {
		return 0.65
	}
	return 0.45
}

func tokenEstimate(s string) int {
	return len(strings.Fields(s))
}
