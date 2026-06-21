package retrieval

import (
	"context"

	"github.com/okyashgajjar/costwise-mcp/internal/repository"
)

type MatchedFile struct {
	FilePath     string   `json:"file_path"`
	LineFrom     int      `json:"line_from"`
	LineTo       int      `json:"line_to"`
	MatchCount   int      `json:"match_count"`
	Snippet      string   `json:"snippet"`
	MatchedLines []string `json:"matched_lines"`
}

type RetrievalResult struct {
	File      string  `json:"file"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
	Tokens    int     `json:"tokens"`
	LineFrom  int     `json:"line_from"`
	LineTo    int     `json:"line_to"`
	MatchHits int     `json:"match_hits"`
	Reason    string  `json:"reason"`
}

type RetrievalDiagnostics struct {
	RawResults     int                `json:"raw_results"`
	AfterRanking   int                `json:"after_ranking"`
	PassedFilter   int                `json:"passed_filter"`
	FilesScanned   int                `json:"files_scanned"`
	FilesLoaded    int                `json:"files_loaded"`
	TotalMatchHits int                `json:"total_match_hits"`
	TopScores      map[string]float64 `json:"top_scores,omitempty"`
	Provenance     string             `json:"provenance"`
}

type RetrievalMetrics struct {
	FilesScanned             int     `json:"files_scanned"`
	FilesLoaded              int     `json:"files_loaded"`
	Tokens                   int     `json:"tokens"`
	LatencyMs                int64   `json:"latency_ms"`
	MatchedFiles             int     `json:"matched_files"`
	MatchedSnippets          int     `json:"matched_snippets"`
	MatchCount               int     `json:"match_count"`
	Confidence               float64 `json:"confidence"`
	ClassificationLabel      string  `json:"classification_label,omitempty"`
	ClassificationConfidence float64 `json:"classification_confidence,omitempty"`
}

type Retriever interface {
	Name() string
	Initialize(ctx context.Context, repo *repository.RepositoryInfo) error
	Retrieve(ctx context.Context, query string) ([]RetrievalResult, error)
	Metrics() RetrievalMetrics
	Shutdown() error
}

type RetrievalSummary struct {
	FilesScanned int     `json:"files_scanned"`
	FilesLoaded  int     `json:"files_loaded"`
	Tokens       int     `json:"tokens"`
	LatencyMs    int64   `json:"latency_ms"`
	Results      int     `json:"results"`
	Confidence   float64 `json:"confidence"`
}
