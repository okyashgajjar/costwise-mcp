package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okyashgajjar/costaffective-mcp/internal/answertype"
	"github.com/okyashgajjar/costaffective-mcp/internal/classifier"
	"github.com/okyashgajjar/costaffective-mcp/internal/repository"
	"github.com/okyashgajjar/costaffective-mcp/internal/retrieval"
)

// TaskResult is the scored outcome of running one Task through the live pipeline.
type TaskResult struct {
	Task          Task
	RoutedTo      string   // primary retriever our classifier selected
	RouteHit      bool     // RoutedTo == Task.ExpectedRetriever
	FileHit       bool     // an expected file appeared in the returned results
	SymHit        bool     // an expected symbol appeared in the compressed answer
	GotFiles      []string // repo-relative files we returned
	ContextTokens int      // tokens of the compressed answer (what the model sees)
	LatencyMs     int64
	Stale         bool // expected file(s) not present on disk -> excluded from accuracy
	Empty         bool // pipeline returned nothing
}

// route mirrors AutoRetriever.Retrieve's primary routing table (auto.go) so we
// can score routing without reaching into the retriever's unexported state.
var route = map[classifier.QueryClass]string{
	classifier.SymbolQuery:       "treesitter",
	classifier.TextQuery:         "fts",
	classifier.RepositoryQuery:   "grep",
	classifier.ReferenceQuery:    "reference",
	classifier.CallQuery:         "callgraph",
	classifier.ArchitectureQuery: "architecture",
	classifier.FlowQuery:         "flowgraph",
}

// RunDataset indexes repoRoot once and runs every task through the same chain as
// the search_code tool: Classify -> AutoRetriever.Retrieve -> FilterResults ->
// CompressForAnswerType.
func RunDataset(ctx context.Context, repoRoot string, tasks []Task) ([]TaskResult, error) {
	repo := &repository.RepositoryInfo{Root: repoRoot}

	auto := retrieval.NewAutoRetriever("")
	if err := auto.Initialize(ctx, repo); err != nil {
		return nil, err
	}

	results := make([]TaskResult, 0, len(tasks))
	for _, t := range tasks {
		tr := TaskResult{Task: t}

		// Routing decision (mirrors auto.go primary route).
		cl := classifier.Classify(t.Query)
		tr.RoutedTo = route[cl.Class]
		tr.RouteHit = tr.RoutedTo == t.ExpectedRetriever

		// Is the ground truth even present in this checkout? If not, the query
		// is excluded from file-hit accuracy (stale dataset path, not a miss).
		tr.Stale = len(t.Files()) > 0 && !expectedFilesExist(repoRoot, t.Files())

		start := time.Now()
		atc := answertype.Classify(t.Query, "chat")
		raw, err := auto.Retrieve(ctx, t.Query)
		if err != nil {
			tr.LatencyMs = time.Since(start).Milliseconds()
			results = append(results, tr)
			continue
		}
		filtered := retrieval.FilterResults(raw, 0.15, retrieval.GuessMaxResults(atc))
		compressed := retrieval.CompressForAnswerType(filtered, atc, retrieval.GuessContextBudget(atc))
		tr.LatencyMs = time.Since(start).Milliseconds()
		tr.ContextTokens = compressed.Tokens
		tr.Empty = len(filtered) == 0

		for _, r := range filtered {
			tr.GotFiles = append(tr.GotFiles, relPath(repoRoot, r.File))
		}
		tr.FileHit = scoreFileHit(repoRoot, t.Files(), filtered, compressed.Context)
		tr.SymHit = scoreSymHit(t.Symbols(), compressed.Context)

		results = append(results, tr)
	}
	return results, nil
}

func relPath(root, file string) string {
	f := filepath.ToSlash(file)
	r := filepath.ToSlash(root)
	f = strings.TrimPrefix(f, r)
	return strings.TrimPrefix(f, "/")
}

// scoreFileHit reports whether an expected file appears in what the model
// actually receives: either as a result's structured .File field, or embedded
// in the compressed answer text. The latter matters because the callgraph and
// reference retrievers return a single synthetic result (File "callgraph:Sym")
// and list the real files inside the snippet, which compression may truncate.
// Matching against the compressed context measures the post-truncation truth.
func scoreFileHit(root string, expected []string, results []retrieval.RetrievalResult, context string) bool {
	hay := filepath.ToSlash(context)
	for _, e := range expected {
		full := filepath.ToSlash(e)
		// 1) Structured .File field (definition/symbol results). pathSuffixMatch
		//    is bidirectional, so a result of "packages/x.ts" still matches an
		//    expected of "opencode/packages/x.ts".
		for _, r := range results {
			if pathSuffixMatch(full, relPath(root, r.File)) {
				return true
			}
		}
		// 2) Path embedded in the answer text (caller/reference/architecture).
		//    Require >=2 segments so a bare basename can't false-positive.
		for _, c := range pathCandidates(full) {
			if strings.Count(c, "/") >= 1 && strings.Contains(hay, c) {
				return true
			}
		}
	}
	return false
}

// scoreSymHit reports whether any expected symbol appears in the compressed
// answer — the file-free accuracy proxy for reference/concept/flow queries,
// whose datasets specify a symbol rather than a file.
func scoreSymHit(symbols []string, context string) bool {
	if len(symbols) == 0 {
		return false
	}
	low := strings.ToLower(context)
	for _, s := range symbols {
		if strings.Contains(low, strings.ToLower(strings.TrimSpace(s))) {
			return true
		}
	}
	return false
}

// pathCandidates returns the path as-is plus a leading-segment-stripped variant,
// since datasets sometimes prefix the repo dir name (e.g. "opencode/...").
func pathCandidates(e string) []string {
	e = filepath.ToSlash(e)
	cands := []string{e}
	if i := strings.IndexByte(e, '/'); i > 0 {
		cands = append(cands, e[i+1:])
	}
	return cands
}

// expectedFilesExist returns true if at least one expected file is on disk under
// root, trying both the path as-is and with a leading segment stripped (datasets
// sometimes prefix the repo dir name).
func expectedFilesExist(root string, expected []string) bool {
	for _, e := range expected {
		candidates := []string{e}
		if i := strings.IndexByte(filepath.ToSlash(e), '/'); i > 0 {
			candidates = append(candidates, e[i+1:])
		}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(c))); err == nil {
				return true
			}
		}
	}
	return false
}

// pathSuffixMatch reports whether the shorter path is a tail of the longer one
// at segment boundaries, so "oo.go" never matches "foo.go".
func pathSuffixMatch(a, b string) bool {
	as := strings.Split(strings.Trim(filepath.ToSlash(a), "/"), "/")
	bs := strings.Split(strings.Trim(filepath.ToSlash(b), "/"), "/")
	if len(as) > len(bs) {
		as, bs = bs, as
	}
	if len(as) == 0 || as[0] == "" {
		return false
	}
	for i := 1; i <= len(as); i++ {
		if as[len(as)-i] != bs[len(bs)-i] {
			return false
		}
	}
	return true
}
