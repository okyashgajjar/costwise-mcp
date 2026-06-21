package retrieval

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/repository"
)

type GrepRetriever struct {
	repo    *repository.RepositoryInfo
	metrics RetrievalMetrics
}

func NewGrepRetriever() *GrepRetriever {
	return &GrepRetriever{}
}

func (r *GrepRetriever) Name() string {
	return "grep"
}

func (r *GrepRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.metrics = RetrievalMetrics{}
	return nil
}

func (r *GrepRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		keywords = []string{query}
	}

	rgPath, _ := exec.LookPath("rg")
	grepPath, _ := exec.LookPath("grep")
	hasRG := rgPath != ""
	hasGrep := grepPath != ""

	if !hasRG && !hasGrep {
		return nil, fmt.Errorf("neither ripgrep nor grep found on system")
	}

	scanned := 0
	loaded := 0
	totalMatches := 0
	fileMatchMap := make(map[string]*fileMatch)

	searchInFile := func(filePath string, scoreWeight float64) {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return
		}
		scanned++

		data, err := os.ReadFile(filePath)
		if err != nil || isBinary(data) {
			return
		}
		content := string(data)
		lines := strings.Split(content, "\n")

		var fileMatches []int
		contentLower := strings.ToLower(content)
		matchedTerms := make(map[string]bool)
		for _, kw := range keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(contentLower, kwLower) {
				matchedTerms[kwLower] = true
				for i, line := range lines {
					if strings.Contains(strings.ToLower(line), kwLower) {
						fileMatches = append(fileMatches, i)
					}
				}
			}
		}

		if len(fileMatches) == 0 {
			return
		}

		fileMatches = uniqueInts(fileMatches)
		sort.Ints(fileMatches)

		relPath, _ := filepath.Rel(r.repo.Root, filePath)

		keywordDensity := float64(len(fileMatches)) / float64(maxInt(1, len(lines)))
		uniqueRatio := float64(len(matchedTerms)) / float64(maxInt(1, len(keywords)))
		recency := 0.0
		if len(fileMatches) > 0 && len(lines) > 0 {
			firstMatchRatio := float64(fileMatches[0]) / float64(len(lines))
			if firstMatchRatio <= 0.2 {
				recency = 1.0
			} else if firstMatchRatio <= 0.5 {
				recency = 0.5
			} else {
				recency = 0.2
			}
		}

		score := float64(len(fileMatches)) * scoreWeight * (1.0 + keywordDensity*2.0 + uniqueRatio*2.0 + recency)

		snippetLines := extractSnippetLines(lines, fileMatches, 3)
		snippet := strings.Join(snippetLines, "\n")

		loaded++
		totalMatches += len(fileMatches)

		fm := &fileMatch{
			filePath:       filePath,
			relPath:        relPath,
			snippet:        snippet,
			matchLines:     fileMatches,
			matchCount:     len(fileMatches),
			score:          score,
			scoreWeight:    scoreWeight,
			keywordDensity: keywordDensity,
			uniqueRatio:    uniqueRatio,
			recency:        recency,
		}
		fileMatchMap[filePath] = fm
	}

	searchInFileContent := func(cmd *exec.Cmd, scoreWeight float64) {
		output, err := cmd.Output()
		if err != nil {
			return
		}
		outStr := strings.TrimSpace(string(output))
		if outStr == "" {
			return
		}

		lines := strings.Split(outStr, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 3)
			if len(parts) < 2 {
				continue
			}
			filePath := parts[0]
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				continue
			}
			scanned++

			if _, exists := fileMatchMap[filePath]; !exists {
				data, err := os.ReadFile(filePath)
				if err != nil || isBinary(data) {
					continue
				}
				content := string(data)
				allLines := strings.Split(content, "\n")

				var fileMatches []int
				contentLower := strings.ToLower(content)
				matchedTerms := make(map[string]bool)
				for _, kw := range keywords {
					if strings.Contains(contentLower, strings.ToLower(kw)) {
						matchedTerms[strings.ToLower(kw)] = true
						for i, l := range allLines {
							if strings.Contains(strings.ToLower(l), strings.ToLower(kw)) {
								fileMatches = append(fileMatches, i)
							}
						}
					}
				}

				if len(fileMatches) == 0 {
					continue
				}

				fileMatches = uniqueInts(fileMatches)
				sort.Ints(fileMatches)
				relPath, _ := filepath.Rel(r.repo.Root, filePath)

				keywordDensity := float64(len(fileMatches)) / float64(maxInt(1, len(allLines)))
				uniqueRatio := float64(len(matchedTerms)) / float64(maxInt(1, len(keywords)))
				recency := 0.0
				if len(fileMatches) > 0 && len(allLines) > 0 {
					firstMatchRatio := float64(fileMatches[0]) / float64(len(allLines))
					if firstMatchRatio <= 0.2 {
						recency = 1.0
					} else if firstMatchRatio <= 0.5 {
						recency = 0.5
					} else {
						recency = 0.2
					}
				}

				score := float64(len(fileMatches)) * scoreWeight * (1.0 + keywordDensity*2.0 + uniqueRatio*2.0 + recency)

				snippetLines := extractSnippetLines(allLines, fileMatches, 3)
				snippet := strings.Join(snippetLines, "\n")

				loaded++
				totalMatches += len(fileMatches)

				fm := &fileMatch{
					filePath:       filePath,
					relPath:        relPath,
					snippet:        snippet,
					matchLines:     fileMatches,
					matchCount:     len(fileMatches),
					score:          score,
					scoreWeight:    scoreWeight,
					keywordDensity: keywordDensity,
					uniqueRatio:    uniqueRatio,
					recency:        recency,
				}
				fileMatchMap[filePath] = fm
			} else {
				existing := fileMatchMap[filePath]
				existing.score += scoreWeight
			}
		}
	}

	if r.repo.ReadmePath != "" {
		searchInFile(r.repo.ReadmePath, 3.0)
	}

	if r.repo.DocsDir != "" {
		_ = filepath.Walk(r.repo.DocsDir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || fi.Size() >= 1<<20 {
				return nil
			}
			if isBinaryExt(path) {
				return nil
			}
			searchInFile(path, 2.0)
			return nil
		})
	}

	for _, keyword := range keywords {
		searchFlags := []string{"-n", "--no-heading", "-i"}
		if hasRG {
			searchFlags = append(searchFlags, "-C", "2")
		} else {
			searchFlags = append(searchFlags, "-C", "2")
		}

		if hasRG {
			cmd := exec.Command(rgPath, append(searchFlags, keyword, r.repo.Root)...)
			searchInFileContent(cmd, 1.0)
		} else if hasGrep {
			cmd := exec.Command(grepPath, append(searchFlags, "-r", keyword, r.repo.Root)...)
			searchInFileContent(cmd, 1.0)
		}
	}

	filenameCMD := exec.Command("find", r.repo.Root, "-maxdepth", "3", "-iname", fmt.Sprintf("*%s*", keywords[0]), "-not", "-path", "*/.git/*", "-not", "-path", "*/node_modules/*", "-not", "-path", "*/vendor/*")
	if filenameOut, err := filenameCMD.Output(); err == nil {
		filenameStr := strings.TrimSpace(string(filenameOut))
		if filenameStr != "" {
			for _, f := range strings.Split(filenameStr, "\n") {
				if f == "" {
					continue
				}
				if fi, err := os.Stat(f); err == nil && !fi.IsDir() {
					if _, exists := fileMatchMap[f]; !exists {
						searchInFile(f, 1.5)
					} else {
						fileMatchMap[f].score += 1.5
					}
				}
			}
		}
	}

	if totalMatches == 0 && len(fileMatchMap) > 0 {
		for _, fm := range fileMatchMap {
			totalMatches += fm.matchCount
		}
	}

	var sorted []*fileMatch
	for _, fm := range fileMatchMap {
		sorted = append(sorted, fm)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	topN := 5
	if len(sorted) < topN {
		topN = len(sorted)
	}
	topFiles := sorted[:topN]

	maxScore := 1.0
	if len(topFiles) > 0 && topFiles[0].score > maxScore {
		maxScore = topFiles[0].score
	}

	var results []RetrievalResult
	for _, fm := range topFiles {
		reasons := fmt.Sprintf("matches=%d,density=%.2f,unique=%.2f,recency=%.2f,weight=%.1f",
			fm.matchCount, fm.keywordDensity, fm.uniqueRatio, fm.recency, fm.scoreWeight)
		results = append(results, RetrievalResult{
			File:      fm.filePath,
			Snippet:   fm.snippet,
			Score:     fm.score / maxScore,
			Tokens:    len(fm.snippet) / 4,
			MatchHits: fm.matchCount,
			Reason:    reasons,
		})
	}

	confidence := 0.0
	if len(results) > 0 && totalMatches > 0 {
		matchedTerms := make(map[string]bool)
		for _, kw := range keywords {
			for _, result := range results {
				if strings.Contains(strings.ToLower(result.Snippet), kw) {
					matchedTerms[kw] = true
					break
				}
			}
		}
		uniqueRatio := float64(len(matchedTerms)) / float64(len(keywords))
		cappedMatches := totalMatches
		if cappedMatches > 500 {
			cappedMatches = 500
		}
		hitDensity := float64(cappedMatches) / float64(cappedMatches+len(results))
		confidence = uniqueRatio*0.80 + hitDensity*0.20
		if confidence > 0.92 {
			confidence = 0.92
		}
	}

	r.metrics = RetrievalMetrics{
		FilesScanned:    scanned,
		FilesLoaded:     loaded,
		Tokens:          totalTokens(results),
		LatencyMs:       time.Since(start).Milliseconds(),
		MatchedFiles:    len(results),
		MatchedSnippets: len(results),
		MatchCount:      totalMatches,
		Confidence:      confidence,
	}

	return results, nil
}

type fileMatch struct {
	filePath       string
	relPath        string
	snippet        string
	matchLines     []int
	matchCount     int
	score          float64
	scoreWeight    float64
	keywordDensity float64
	uniqueRatio    float64
	recency        float64
}

func (r *GrepRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *GrepRetriever) Shutdown() error {
	return nil
}

func extractKeywords(query string) []string {
	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true, "need": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "out": true, "off": true,
		"over": true, "under": true, "again": true, "further": true, "then": true,
		"once": true, "here": true, "there": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "each": true, "every": true,
		"both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "not": true,
		"only": true, "own": true, "same": true, "so": true, "than": true,
		"too": true, "very": true, "just": true, "because": true, "but": true,
		"and": true, "or": true, "if": true, "while": true, "about": true,
		"explain": true, "describe": true, "what": true, "which": true,
		"tell": true, "show": true, "find": true, "get": true, "please": true,
		"this": true, "that": true, "these": true, "those": true, "it": true,
		"its": true, "my": true, "your": true, "our": true, "their": true,
		"current": true, "flow": true, "work": true,
	}

	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()[]{}/\\")
		if len(w) < 2 || stopWords[w] {
			continue
		}
		keywords = append(keywords, w)
	}

	var expanded []string
	seen := make(map[string]bool)
	for _, kw := range keywords {
		if !seen[kw] {
			expanded = append(expanded, kw)
			seen[kw] = true
		}
		stem := simpleStem(kw)
		if stem != kw && !seen[stem] {
			expanded = append(expanded, stem)
			seen[stem] = true
		}
	}

	return expanded
}

func simpleStem(word string) string {
	if strings.HasSuffix(word, "tion") {
		base := strings.TrimSuffix(word, "tion")
		return base + "t"
	}
	if strings.HasSuffix(word, "ations") {
		return strings.TrimSuffix(word, "ations")
	}
	if strings.HasSuffix(word, "ation") {
		return strings.TrimSuffix(word, "ation")
	}
	if strings.HasSuffix(word, "ments") {
		return strings.TrimSuffix(word, "s")
	}
	if strings.HasSuffix(word, "ment") {
		return word
	}
	if strings.HasSuffix(word, "ing") {
		return strings.TrimSuffix(word, "ing")
	}
	if strings.HasSuffix(word, "ings") {
		return strings.TrimSuffix(word, "ings")
	}
	if strings.HasSuffix(word, "ed") {
		base := strings.TrimSuffix(word, "ed")
		return base
	}
	if strings.HasSuffix(word, "es") {
		return strings.TrimSuffix(word, "es")
	}
	if strings.HasSuffix(word, "s") && len(word) > 3 {
		return strings.TrimSuffix(word, "s")
	}
	return word
}

func extractSnippetLines(lines []string, matchLines []int, context int) []string {
	if len(matchLines) == 0 {
		return nil
	}

	seen := make(map[int]bool)
	var result []int
	for _, ml := range matchLines {
		for i := ml - context; i <= ml+context; i++ {
			if i >= 0 && i < len(lines) && !seen[i] {
				seen[i] = true
				result = append(result, i)
			}
		}
	}
	sort.Ints(result)

	var snippet []string
	prev := -2
	for _, lineNum := range result {
		if lineNum > prev+1 {
			snippet = append(snippet, "...")
		}
		isMatch := false
		for _, ml := range matchLines {
			if ml == lineNum {
				isMatch = true
				break
			}
		}
		prefix := " "
		if isMatch {
			prefix = ">"
		}
		snippet = append(snippet, fmt.Sprintf("%s%d: %s", prefix, lineNum+1, lines[lineNum]))
		prev = lineNum
	}

	return snippet
}

func uniqueInts(ints []int) []int {
	seen := make(map[int]bool)
	var result []int
	for _, i := range ints {
		if !seen[i] {
			seen[i] = true
			result = append(result, i)
		}
	}
	return result
}

func countUniqueFiles(results []RetrievalResult) int {
	seen := make(map[string]bool)
	for _, r := range results {
		seen[r.File] = true
	}
	return len(seen)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
