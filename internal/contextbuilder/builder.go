package contextbuilder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/okyashgajjar/costaffective-mcp/internal/retrieval"
)

type Level int

const (
	Level0 Level = iota
	Level1
	Level2
	Level3
	Level4
	Level5
	Level6
	Level7
)

func (l Level) String() string {
	switch l {
	case Level0:
		return "file-names"
	case Level1:
		return "names+snippets"
	case Level2:
		return "expanded-snippets"
	case Level3:
		return "full-files"
	case Level4:
		return "symbols"
	case Level5:
		return "reference"
	case Level6:
		return "call-graph"
	case Level7:
		return "function-body"
	default:
		return "unknown"
	}
}

func ParseLevel(s string) Level {
	switch s {
	case "0", "file-names":
		return Level0
	case "1", "names+snippets":
		return Level1
	case "2", "expanded-snippets":
		return Level2
	case "3", "full-files":
		return Level3
	case "4", "symbols":
		return Level4
	case "5", "reference":
		return Level5
	case "6", "call-graph":
		return Level6
	case "7", "function-body":
		return Level7
	default:
		return Level1
	}
}

type ContextMetrics struct {
	FilesSelected      int     `json:"files_selected"`
	FilesDropped       int     `json:"files_dropped"`
	Tokens             int     `json:"tokens"`
	TokenBudget        int     `json:"token_budget"`
	TotalResults       int     `json:"total_results"`
	ContextEfficiency  float64 `json:"context_efficiency"`
	RetrievalPrecision float64 `json:"retrieval_precision"`
	RetrievalRecall    float64 `json:"retrieval_recall"`
	RankingScore       float64 `json:"ranking_score"`
	EmptySnippets      int     `json:"empty_snippets"`
}

type Builder struct {
	Level       Level
	TokenBudget int
}

func NewBuilder(level Level, tokenBudget int) *Builder {
	if tokenBudget <= 0 {
		tokenBudget = 3000
	}
	return &Builder{
		Level:       level,
		TokenBudget: tokenBudget,
	}
}

func (b *Builder) Build(results []retrieval.RetrievalResult) (string, ContextMetrics) {
	if len(results) == 0 {
		return "", ContextMetrics{
			FilesSelected: 0,
			FilesDropped:  0,
			Tokens:        0,
			TokenBudget:   b.TokenBudget,
			TotalResults:  0,
		}
	}

	sorted := make([]retrieval.RetrievalResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	var included []retrieval.RetrievalResult
	totalTokens := 0
	dropped := 0
	emptySnippets := 0

	for _, result := range sorted {
		if b.Level == Level0 {
			tokens := 1
			if totalTokens+tokens <= b.TokenBudget {
				included = append(included, result)
				totalTokens += tokens
			} else {
				dropped++
			}
			continue
		}

		if result.Snippet == "" {
			emptySnippets++
		}

		if b.Level >= Level3 && b.Level != Level7 && result.Snippet == "" {
			content := loadFullFile(result.File)
			if content == "" {
				dropped++
				continue
			}
			result.Snippet = content
		}

		var block string
		switch b.Level {
		case Level1:
			rel := result.File
			if result.Snippet == "" {
				block = fmt.Sprintf("%s\n", rel)
			} else {
				block = fmt.Sprintf("=== %s (confidence: %.2f) ===\n%s\n\n", rel, result.Score, result.Snippet)
			}

		case Level2:
			if result.Snippet == "" {
				dropped++
				continue
			}
			rel := result.File
			expanded := expandSnippet(result.Snippet)
			block = fmt.Sprintf("=== %s (confidence: %.2f) ===\n%s\n\n", rel, result.Score, expanded)

		case Level3:
			content := result.Snippet
			if content == "" {
				content = loadFullFile(result.File)
			}
			if content == "" {
				dropped++
				continue
			}
			rel := result.File
			block = fmt.Sprintf("=== %s ===\n%s\n\n", rel, content)

		case Level4:
			if result.Snippet == "" {
				dropped++
				continue
			}
			block = fmt.Sprintf("%s\n", result.Snippet)

		case Level5:
			if result.Snippet == "" {
				dropped++
				continue
			}
			block = fmt.Sprintf("%s\n", result.Snippet)

		case Level6:
			if result.Snippet == "" {
				dropped++
				continue
			}
			block = fmt.Sprintf("%s\n", result.Snippet)

		case Level7:
			// Function-body level: extract only the function/class/method body
			// using LineFrom/LineTo from the retrieval result.
			body := extractFunctionBody(result)
			if body == "" {
				// Fall back to Level1 behavior if we can't extract the body
				if result.Snippet != "" {
					block = fmt.Sprintf("=== %s (confidence: %.2f) ===\n%s\n\n", result.File, result.Score, result.Snippet)
				} else {
					block = fmt.Sprintf("%s\n", result.File)
				}
			} else {
				block = body
			}
		}

		tokens := countTokens(block)
		if totalTokens+tokens > b.TokenBudget {
			dropped++
			continue
		}

		included = append(included, result)
		totalTokens += tokens
	}

	var context strings.Builder
	for _, result := range included {
		switch b.Level {
		case Level0:
			context.WriteString(filepath.Base(result.File) + "\n")
		case Level1:
			if result.Snippet == "" {
				context.WriteString(result.File + "\n")
			} else {
				fmt.Fprintf(&context, "=== %s (confidence: %.2f) ===\n%s\n\n", result.File, result.Score, result.Snippet)
			}
		case Level2:
			expanded := expandSnippet(result.Snippet)
			fmt.Fprintf(&context, "=== %s (confidence: %.2f) ===\n%s\n\n", result.File, result.Score, expanded)
		case Level3:
			content := result.Snippet
			if content == "" {
				content = loadFullFile(result.File)
			}
			fmt.Fprintf(&context, "=== %s ===\n%s\n\n", result.File, content)
		case Level4:
			if result.Snippet != "" {
				fmt.Fprintf(&context, "%s\n", result.Snippet)
			}
		case Level5:
			if result.Snippet != "" {
				fmt.Fprintf(&context, "%s\n", result.Snippet)
			}

		case Level6:
			if result.Snippet != "" {
				fmt.Fprintf(&context, "%s\n", result.Snippet)
			}

		case Level7:
			body := extractFunctionBody(result)
			if body != "" {
				context.WriteString(body)
			} else if result.Snippet != "" {
				fmt.Fprintf(&context, "=== %s (confidence: %.2f) ===\n%s\n\n", result.File, result.Score, result.Snippet)
			}
		}
	}

	ctxStr := context.String()

	contextEfficiency := 0.0
	if totalTokens > 0 {
		usefulTokens := totalTokens
		if emptySnippets > 0 {
			usefulTokens = totalTokens
		}
		contextEfficiency = float64(usefulTokens) / float64(totalTokens)
	}

	retrievalPrecision := 0.0
	if len(results) > 0 {
		matched := 0
		for _, r := range results {
			if r.Score > 0 {
				matched++
			}
		}
		retrievalPrecision = float64(matched) / float64(len(results))
	}

	retrievalRecall := 0.0
	if emptySnippets < len(results) && len(results) > 0 {
		retrievalRecall = float64(len(results)-emptySnippets) / float64(len(results))
	}

	rankingScore := 0.0
	if len(sorted) > 1 {
		ideal := make([]float64, len(sorted))
		for i, r := range sorted {
			ideal[i] = r.Score
		}
		sort.Slice(ideal, func(i, j int) bool {
			return ideal[i] > ideal[j]
		})
		var dcg, idcg float64
		for i, r := range sorted {
			disc := 1.0
			if i > 0 {
				disc = log2(float64(i + 1))
			}
			dcg += (pow2(r.Score) - 1.0) / disc
		}
		for i, s := range ideal {
			disc := 1.0
			if i > 0 {
				disc = log2(float64(i + 1))
			}
			idcg += (pow2(s) - 1.0) / disc
		}
		if idcg > 0 {
			rankingScore = dcg / idcg
		} else {
			rankingScore = 1.0
		}
	} else if len(sorted) == 1 {
		rankingScore = 1.0
	}

	return ctxStr, ContextMetrics{
		FilesSelected:      len(included),
		FilesDropped:       dropped,
		Tokens:             totalTokens,
		TokenBudget:        b.TokenBudget,
		TotalResults:       len(results),
		ContextEfficiency:  contextEfficiency,
		RetrievalPrecision: retrievalPrecision,
		RetrievalRecall:    retrievalRecall,
		RankingScore:       rankingScore,
		EmptySnippets:      emptySnippets,
	}
}

func log2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	ln2 := 0.6931471805599453
	return float64(int(ln(x)/ln2*1000)) / 1000
}

func ln(x float64) float64 {
	if x <= 0 {
		return 0
	}
	n := (x - 1) / (x + 1)
	n2 := n * n
	return 2 * n * (1 + n2/3 + n2*n2/5 + n2*n2*n2/7 + n2*n2*n2*n2/9)
}

func pow2(x float64) float64 {
	if x <= 0 {
		return 1
	}
	r := 1.0
	base := 2.0
	exp := int(x * 100)
	for i := 0; i < exp; i++ {
		r *= base
	}
	return r
}

func expandSnippet(snippet string) string {
	lines := strings.Split(snippet, "\n")
	if len(lines) < 3 {
		return snippet
	}

	seenContent := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "...") && !strings.HasPrefix(trimmed, ">") {
			seenContent = true
			break
		}
	}

	if !seenContent {
		return snippet
	}

	return snippet
}

func loadFullFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func countTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// extractFunctionBody extracts the function/class/method body from a source file
// using the LineFrom/LineTo range from the retrieval result.
// This is the core of Context Level 7: instead of sending full files or metadata,
// only the relevant code range is included in the context.
func extractFunctionBody(result retrieval.RetrievalResult) string {
	if result.LineFrom <= 0 || result.LineTo <= 0 || result.LineFrom > result.LineTo {
		return ""
	}

	// Cap function body at 100 lines to prevent overly large snippets
	maxLines := 100
	endLine := result.LineTo
	if endLine-result.LineFrom+1 > maxLines {
		endLine = result.LineFrom + maxLines - 1
	}

	filePath := result.File
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	if result.LineFrom > len(lines) {
		return ""
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Extract the line range (1-indexed to 0-indexed)
	extracted := lines[result.LineFrom-1 : endLine]

	rel := filePath
	if idx := strings.LastIndex(rel, "/internal/"); idx >= 0 {
		rel = rel[idx+1:]
	} else if idx := strings.LastIndex(rel, "/cmd/"); idx >= 0 {
		rel = rel[idx+1:]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== %s:%d-%d (confidence: %.2f) ===\n", rel, result.LineFrom, endLine, result.Score)
	for i, line := range extracted {
		fmt.Fprintf(&b, "%d: %s\n", result.LineFrom+i, line)
	}
	b.WriteString("\n")
	return b.String()
}
