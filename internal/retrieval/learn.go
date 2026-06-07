package retrieval

import (
	"path/filepath"
	"strings"

	"github.com/okyashgajjar/costaffective-mcp/internal/kmemory"
)

func LearnFromResults(km *kmemory.KnowledgeMemory, query string, results []RetrievalResult) {
	symbol := ExtractSymbolFromQuery(query)
	if symbol == "" && len(results) > 0 {
		base := filepath.Base(results[0].File)
		if idx := strings.LastIndexByte(base, '.'); idx > 0 {
			symbol = base[:idx]
		}
	}

	if symbol == "" {
		return
	}

	var files []string
	var callers []string
	var refFiles []string
	fileSet := make(map[string]bool)
	callerSet := make(map[string]bool)

	for _, r := range results {
		rel := r.File
		if !fileSet[rel] {
			fileSet[rel] = true
			files = append(files, rel)
		}

		if strings.Contains(r.Reason, "caller") || strings.Contains(r.Reason, "calls") {
			callerName := extractCallerFromSnippet(r.Snippet)
			if callerName != "" && !callerSet[callerName] {
				callerSet[callerName] = true
				callers = append(callers, callerName)
			}
			refFiles = append(refFiles, rel)
		}

		if strings.Contains(r.Reason, "ref") || strings.Contains(r.Reason, "import") {
			refFiles = append(refFiles, rel)
		}
	}

	if len(files) > 0 {
		primaryFile := files[0]
		definition := ""
		for _, r := range results {
			if r.Snippet != "" {
				firstLine := strings.SplitN(r.Snippet, "\n", 2)[0]
				definition = firstLine
				break
			}
		}

		km.Store(kmemory.SymbolKnowledge, symbol, kmemory.NewSymbolEntry(symbol, primaryFile, definition, callers))
	}

	if len(callers) > 0 {
		km.Store(kmemory.CallerKnowledge, symbol, kmemory.NewCallerEntry(symbol, callers))
	}

	if len(refFiles) > 0 {
		km.Store(kmemory.ReferenceKnowledge, symbol, kmemory.NewReferenceEntry(symbol, refFiles))
	}

	for _, r := range results {
		rel := r.File
		owner := extractOwnershipFromRel(rel)
		if owner != "" {
			km.Store(kmemory.FileOwnership, rel, &kmemory.KnowledgeEntry{
				Type:       kmemory.FileOwnership,
				Key:        rel,
				Value:      owner,
				File:       rel,
				Confidence: 0.8,
			})
		}
	}
}

func LearnGrepResult(km *kmemory.KnowledgeMemory, query string, files []string) {
	if len(files) == 0 {
		return
	}

	km.Store(kmemory.GrepKnowledge, query, &kmemory.KnowledgeEntry{
		Type:       kmemory.GrepKnowledge,
		Key:        query,
		Value:      strings.Join(files, ", "),
		Metadata:   files,
		File:       "",
		Confidence: 0.7,
	})
}

func LearnGlobResult(km *kmemory.KnowledgeMemory, pattern string, files []string) {
	if len(files) == 0 {
		return
	}

	km.Store(kmemory.GlobKnowledge, pattern, &kmemory.KnowledgeEntry{
		Type:       kmemory.GlobKnowledge,
		Key:        pattern,
		Value:      strings.Join(files, ", "),
		Metadata:   files,
		File:       "",
		Confidence: 0.7,
	})
}

func extractCallerFromSnippet(snippet string) string {
	if snippet == "" {
		return ""
	}
	lines := strings.Split(snippet, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "func") || strings.HasPrefix(trimmed, "def") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				name := strings.TrimRight(parts[1], "(:")
				return name
			}
		}
	}
	return ""
}

func extractOwnershipFromRel(rel string) string {
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 3 {
		return parts[len(parts)-3]
	}
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

func symbolFromResults(results []RetrievalResult) string {
	symbolMap := make(map[string]int)
	for _, r := range results {
		if r.Snippet != "" {
			firstLine := strings.SplitN(r.Snippet, "\n", 2)[0]
			for _, tok := range strings.Fields(firstLine) {
				tok = strings.Trim(tok, "[]:()")
				if len(tok) >= 2 && tok[0] >= 'A' && tok[0] <= 'Z' {
					symbolMap[tok]++
				}
			}
		}
	}
	best := ""
	bestCount := 0
	for sym, count := range symbolMap {
		if count > bestCount {
			bestCount = count
			best = sym
		}
	}
	return best
}
