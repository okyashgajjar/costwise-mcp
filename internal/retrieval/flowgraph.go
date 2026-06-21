package retrieval

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/architecture"
	"github.com/okyashgajjar/costwise-mcp/internal/repository"
	"github.com/okyashgajjar/costwise-mcp/internal/treesitter"
)

type FlowGraphRetriever struct {
	repo    *repository.RepositoryInfo
	archIdx *architecture.Indexer
	symDB   *treesitter.SymbolDB
	metrics RetrievalMetrics
}

const (
	maxFlowDepth = 3
	maxFlowNodes = 15
)

func NewFlowGraphRetriever() *FlowGraphRetriever {
	return &FlowGraphRetriever{}
}

func (r *FlowGraphRetriever) Name() string {
	return "flowgraph"
}

func (r *FlowGraphRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
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
	r.archIdx = idx

	db, err := treesitter.NewSymbolDB(repo.Root)
	if err != nil {
		idx.Close()
		return err
	}
	r.symDB = db

	return nil
}

func (r *FlowGraphRetriever) Shutdown() error {
	var errs []string
	if r.archIdx != nil {
		if err := r.archIdx.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if r.symDB != nil {
		if err := r.symDB.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *FlowGraphRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

type flowNode struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"` // "function", "class", "module", "import"
	File     string `json:"file"`
	Line     int    `json:"line"`
	Depth    int    `json:"depth"`
	Relation string `json:"relation"` // "calls", "called_by", "imports", "imported_by", "constructs"
}

type flowGraph struct {
	Nodes []flowNode `json:"nodes"`
}

func (r *FlowGraphRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	seeds := r.findSeeds(ctx, query)
	if len(seeds) == 0 {
		r.metrics = RetrievalMetrics{
			LatencyMs:  time.Since(start).Milliseconds(),
			Confidence: 0,
		}
		return nil, nil
	}

	graph := r.traceFlow(seeds)

	snippet := formatFlowGraph(graph, query)
	toks := tokenEstimate(snippet)
	confidence := computeFlowConfidence(graph)

	result := RetrievalResult{
		File:      "flowgraph:" + query,
		Snippet:   snippet,
		Score:     confidence,
		Tokens:    toks,
		MatchHits: len(graph.Nodes),
		Reason:    fmt.Sprintf("nodes=%d,depth=%d", len(graph.Nodes), maxNodeDepth(graph)),
	}

	r.metrics = RetrievalMetrics{
		FilesScanned: len(seeds),
		FilesLoaded:  1,
		Tokens:       toks,
		LatencyMs:    time.Since(start).Milliseconds(),
		MatchedFiles: 1,
		MatchCount:   len(graph.Nodes),
		Confidence:   confidence,
	}

	return []RetrievalResult{result}, nil
}

func (r *FlowGraphRetriever) findSeeds(ctx context.Context, query string) []flowNode {
	queryLower := strings.ToLower(query)
	seeds := []flowNode{}

	if r.archIdx != nil {
		all, err := r.archIdx.DB().LoadAll()
		if err == nil {
			for _, s := range all {
				lower := strings.ToLower(s.FilePath)
				if strings.Contains(lower, "min.js") || strings.Contains(lower, ".min.") {
					continue
				}
				for _, c := range s.Classes {
					if strings.Contains(strings.ToLower(c), queryLower) {
						seeds = append(seeds, flowNode{Name: c, Kind: "class", File: s.FilePath, Depth: 0, Relation: "seed"})
					}
				}
				for _, f := range s.Functions {
					if strings.Contains(strings.ToLower(f), queryLower) {
						seeds = append(seeds, flowNode{Name: f, Kind: "function", File: s.FilePath, Depth: 0, Relation: "seed"})
					}
				}
				base := strings.ToLower(filepath.Base(s.FilePath))
				base = strings.TrimSuffix(base, filepath.Ext(base))
				if strings.Contains(base, queryLower) || strings.Contains(queryLower, base) {
					seeds = append(seeds, flowNode{Name: base, Kind: "module", File: s.FilePath, Depth: 0, Relation: "seed"})
				}
			}
		}
	}

	if len(seeds) > 5 {
		seeds = seeds[:5]
	}
	return seeds
}

func (r *FlowGraphRetriever) traceFlow(seeds []flowNode) flowGraph {
	visited := make(map[string]bool)
	var nodes []flowNode

	addUnique := func(node flowNode) bool {
		key := fmt.Sprintf("%s:%s:%s", node.Kind, node.Name, node.File)
		if visited[key] {
			return false
		}
		visited[key] = true
		nodes = append(nodes, node)
		return true
	}

	for _, s := range seeds {
		addUnique(s)
	}

	for i := 0; i < len(nodes) && len(nodes) < maxFlowNodes; i++ {
		node := nodes[i]
		if node.Depth >= maxFlowDepth {
			continue
		}

		if node.Kind == "function" || node.Kind == "class" {
			callees, _ := r.symDB.SearchCallEdgesByCaller(node.Name)
			for _, e := range callees {
				addUnique(flowNode{
					Name:     e.CalleeName,
					Kind:     "function",
					File:     e.File,
					Line:     e.Line,
					Depth:    node.Depth + 1,
					Relation: "calls",
				})
			}

			callers, _ := r.symDB.SearchCallEdges(node.Name)
			for _, e := range callers {
				addUnique(flowNode{
					Name:     e.CallerName,
					Kind:     "function",
					File:     e.CallerFile,
					Line:     e.Line,
					Depth:    node.Depth + 1,
					Relation: "called_by",
				})
			}
		}

		if node.Kind == "module" {
			if r.archIdx != nil {
				if summary, ok := r.archIdx.DB().LoadByFile(node.File); ok {
					for _, imp := range summary.Imports {
						addUnique(flowNode{
							Name:     imp,
							Kind:     "import",
							File:     node.File,
							Depth:    node.Depth + 1,
							Relation: "imports",
						})
					}
				}
			}
		}
	}

	return flowGraph{Nodes: nodes}
}

func formatFlowGraph(g flowGraph, query string) string {
	if len(g.Nodes) == 0 {
		return "No flow paths found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Flow graph for: %s\n\n", query)

	var seeds, calls, calledBy, imports []flowNode
	for _, n := range g.Nodes {
		switch n.Relation {
		case "seed":
			seeds = append(seeds, n)
		case "calls":
			calls = append(calls, n)
		case "called_by":
			calledBy = append(calledBy, n)
		case "imports":
			imports = append(imports, n)
		}
	}

	if len(seeds) > 0 {
		b.WriteString("Starting points:\n")
		for _, s := range seeds {
			fmt.Fprintf(&b, "  %s %s (%s)\n", s.Kind, s.Name, s.File)
		}
		b.WriteString("\n")
	}

	if len(calls) > 0 {
		b.WriteString("Calls:\n")
		for _, c := range calls {
			fmt.Fprintf(&b, "  %s → %s (line %d, %s)\n", c.Name, "", c.Line, c.File)
			fmt.Fprintf(&b, "    at %s line %d\n", c.File, c.Line)
		}
		b.WriteString("\n")
	}

	if len(calledBy) > 0 {
		b.WriteString("Called by:\n")
		for _, c := range calledBy {
			fmt.Fprintf(&b, "  %s ← %s (line %d, %s)\n", c.Name, "", c.Line, c.File)
		}
		b.WriteString("\n")
	}

	if len(imports) > 0 {
		b.WriteString("Imports:\n")
		for _, imp := range imports {
			fmt.Fprintf(&b, "  %s\n", imp.Name)
		}
	}

	return b.String()
}

func computeFlowConfidence(g flowGraph) float64 {
	if len(g.Nodes) == 0 {
		return 0
	}
	n := len(g.Nodes)
	if n >= 8 {
		return 0.70
	}
	if n >= 4 {
		return 0.50
	}
	return 0.30
}

func maxNodeDepth(g flowGraph) int {
	maxD := 0
	for _, n := range g.Nodes {
		if n.Depth > maxD {
			maxD = n.Depth
		}
	}
	return maxD
}
