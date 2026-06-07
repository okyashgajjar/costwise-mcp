package retrieval

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okyashgajjar/costaffective-mcp/internal/answertype"
)

type CompressedContext struct {
	Context   string
	Tokens    int
	Passed    bool
}

func CompressForAnswerType(results []RetrievalResult, answerType answertype.Classification, budget int) CompressedContext {
	switch answerType.Type {
	case answertype.YesNo:
		return compressYesNo(results)
	case answertype.Location:
		return compressLocation(results)
	case answertype.Caller:
		return compressCaller(results)
	case answertype.Reference:
		return compressReference(results)
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
			b.WriteString(fmt.Sprintf("%s:%d\n", rel, r.LineFrom))
		} else {
			b.WriteString(fmt.Sprintf("%s\n", rel))
		}
		if r.Snippet != "" {
			lines := strings.Split(r.Snippet, "\n")
			maxLines := 3
			if len(lines) > maxLines {
				lines = lines[:maxLines]
			}
			for _, l := range lines {
				b.WriteString(fmt.Sprintf("  %s\n", l))
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
			b.WriteString(fmt.Sprintf("%s:%d", rel, r.LineFrom))
			if r.LineTo > r.LineFrom {
				b.WriteString(fmt.Sprintf("-%d", r.LineTo))
			}
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("%s\n", rel))
		}
	}
	ctx := b.String()
	return CompressedContext{
		Context: ctx,
		Tokens:  len(ctx) / 4,
		Passed:  true,
	}
}

func compressCaller(results []RetrievalResult) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	b.WriteString("Callers:\n")
	seen := make(map[string]bool)
	for _, r := range results {
		if r.Snippet != "" {
			lines := strings.Split(r.Snippet, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !seen[trimmed] {
					seen[trimmed] = true
					b.WriteString(fmt.Sprintf("  %s\n", trimmed))
				}
			}
		} else {
			rel := shortenPath(r.File)
			if !seen[rel] {
				seen[rel] = true
				b.WriteString(fmt.Sprintf("  %s\n", rel))
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

func compressReference(results []RetrievalResult) CompressedContext {
	if len(results) == 0 {
		return CompressedContext{Context: "", Tokens: 0, Passed: false}
	}

	var b strings.Builder
	b.WriteString("References:\n")
	seen := make(map[string]bool)
	for _, r := range results {
		if r.Snippet != "" {
			lines := strings.Split(r.Snippet, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !seen[trimmed] {
					seen[trimmed] = true
					b.WriteString(fmt.Sprintf("  %s\n", trimmed))
				}
			}
		} else {
			rel := shortenPath(r.File)
			if !seen[rel] {
				seen[rel] = true
				b.WriteString(fmt.Sprintf("  %s\n", rel))
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
		b.WriteString(fmt.Sprintf("File: %s\n", rel))
		if r.Snippet != "" {
			snippet := cleanSnippet(r.Snippet)
			if len(snippet) > 500 {
				snippet = snippet[:500] + "..."
			}
			b.WriteString(fmt.Sprintf("%s\n\n", snippet))
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
			b.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", rel, snippet))
		} else {
			b.WriteString(fmt.Sprintf("=== %s ===\n", rel))
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
