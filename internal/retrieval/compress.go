package retrieval

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okyashgajjar/costaffective-mcp/internal/answertype"
)

type CompressedContext struct {
	Context string
	Tokens  int
	Passed  bool
}

func CompressForAnswerType(results []RetrievalResult, answerType answertype.Classification, budget int) CompressedContext {
	switch answerType.Type {
	case answertype.YesNo:
		return compressYesNo(results)
	case answertype.Location:
		return compressLocation(results)
	case answertype.Caller:
		return compressCaller(results, budget)
	case answertype.Reference:
		return compressReference(results, budget)
	case answertype.Overview:
		return compressOverview(results, budget)
	default:
		return compressDefault(results, budget)
	}
}

func compressYesNo(results []RetrievalResult) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	for _, r := range results {
		rel := shortenPath(r.File)
		if r.LineFrom > 0 {
			fmt.Fprintf(&b, "%s:%d\n", rel, r.LineFrom)
		} else {
			fmt.Fprintf(&b, "%s\n", rel)
		}
		if r.Snippet != "" {
			lines := strings.Split(r.Snippet, "\n")
			maxLines := 3
			if len(lines) > maxLines {
				lines = lines[:maxLines]
			}
			for _, l := range lines {
				fmt.Fprintf(&b, "  %s\n", l)
			}
		}
	}
	ctx := b.String()
	return CompressedContext{
		Context: ctx,
		Tokens:  len(ctx) / 4,
		Passed:  true,
	}
}

func compressLocation(results []RetrievalResult) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	for i, r := range results {
		if i >= 3 {
			break
		}
		rel := shortenPath(r.File)
		if r.LineFrom > 0 {
			fmt.Fprintf(&b, "%s:%d", rel, r.LineFrom)
			if r.LineTo > r.LineFrom {
				fmt.Fprintf(&b, "-%d", r.LineTo)
			}
			b.WriteString("\n")
		} else {
			fmt.Fprintf(&b, "%s\n", rel)
		}
	}
	ctx := b.String()
	return CompressedContext{
		Context: ctx,
		Tokens:  len(ctx) / 4,
		Passed:  true,
	}
}

func compressCaller(results []RetrievalResult, budget int) CompressedContext {
	return compressLineList(results, budget, "Callers:", "call sites")
}

func compressReference(results []RetrievalResult, budget int) CompressedContext {
	return compressLineList(results, budget, "References:", "references")
}

// compressLineList emits the unique, non-empty lines from result snippets (or
// file paths) under a header, stopping once the running token estimate reaches
// budget and appending a truncation tail. This caps pathological answers — e.g.
// "who calls <ubiquitous symbol>" — that would otherwise blow past the budget.
func compressLineList(results []RetrievalResult, budget int, header, noun string) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	b.WriteString(header + "\n")
	seen := make(map[string]bool)
	omitted := 0
	truncated := false
	for _, r := range results {
		var lines []string
		if r.Snippet != "" {
			lines = strings.Split(r.Snippet, "\n")
		} else {
			lines = []string{shortenPath(r.File)}
		}
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			if !truncated && len(b.String())/4 >= budget {
				truncated = true
			}
			if truncated {
				omitted++
				continue
			}
			fmt.Fprintf(&b, "  %s\n", trimmed)
		}
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "  ... (+%d more %s truncated to fit budget)\n", omitted, noun)
	}
	ctx := b.String()
	return CompressedContext{
		Context: ctx,
		Tokens:  len(ctx) / 4,
		Passed:  true,
	}
}

func compressOverview(results []RetrievalResult, budget int) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	totalTokens := 0
	maxResults := 8
	for i, r := range results {
		if i >= maxResults {
			break
		}
		rel := shortenPath(r.File)
		fmt.Fprintf(&b, "File: %s\n", rel)
		if r.Snippet != "" {
			snippet := cleanSnippet(r.Snippet)
			if len(snippet) > 500 {
				snippet = snippet[:500] + "..."
			}
			fmt.Fprintf(&b, "%s\n\n", snippet)
		}
		totalTokens = len(b.String()) / 4
		if totalTokens >= budget {
			break
		}
	}
	ctx := b.String()
	return CompressedContext{
		Context: ctx,
		Tokens:  len(ctx) / 4,
		Passed:  true,
	}
}

func compressDefault(results []RetrievalResult, budget int) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	totalTokens := 0
	for i, r := range results {
		if i >= 10 {
			break
		}
		rel := shortenPath(r.File)
		if r.Snippet != "" {
			snippet := cleanSnippet(r.Snippet)
			if len(snippet) > 1000 {
				snippet = snippet[:1000] + "..."
			}
			fmt.Fprintf(&b, "=== %s ===\n%s\n\n", rel, snippet)
		} else {
			fmt.Fprintf(&b, "=== %s ===\n", rel)
		}
		totalTokens = len(b.String()) / 4
		if totalTokens >= budget {
			break
		}
	}
	ctx := b.String()
	return CompressedContext{
		Context: ctx,
		Tokens:  len(ctx) / 4,
		Passed:  true,
	}
}

func shortenPath(path string) string {
	if idx := strings.Index(path, "/internal/"); idx >= 0 {
		return path[idx+1:]
	}
	if idx := strings.Index(path, "/cmd/"); idx >= 0 {
		return path[idx+1:]
	}
	if idx := strings.Index(path, "/pkg/"); idx >= 0 {
		return path[idx+1:]
	}
	return filepath.Base(path)
}

func cleanSnippet(snippet string) string {
	lines := strings.Split(snippet, "\n")
	var cleaned []string
	var importCount int
	var inImportBlock bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip consecutive empty lines
		if trimmed == "" && len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
			continue
		}

		// Skip generated headers
		if strings.HasPrefix(trimmed, "// Code generated by") || strings.HasPrefix(trimmed, "// DO NOT EDIT") {
			continue
		}

		// Collapse massive import blocks
		if strings.HasPrefix(trimmed, "import (") || strings.HasPrefix(line, "import {") {
			inImportBlock = true
			importCount++
			if importCount <= 3 {
				cleaned = append(cleaned, line)
			}
			continue
		}

		if inImportBlock {
			importCount++
			if trimmed == ")" || trimmed == "}" || trimmed == "];" || trimmed == "};" {
				inImportBlock = false
				if importCount > 3 {
					cleaned = append(cleaned, "  // ... imports truncated ...")
					cleaned = append(cleaned, line)
				} else {
					cleaned = append(cleaned, line)
				}
			} else if importCount <= 3 {
				cleaned = append(cleaned, line)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "(") {
			importCount++
			if importCount > 5 {
				continue
			}
			if importCount == 5 {
				cleaned = append(cleaned, "// ... more imports truncated ...")
				continue
			}
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}
