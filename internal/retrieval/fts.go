package retrieval

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/okyashgajjar/costaffective-mcp/internal/repository"
)

type FTSRetriever struct {
	repo    *repository.RepositoryInfo
	store   *FTSStore
	metrics RetrievalMetrics
}

type FTSStore struct {
	db *sql.DB
}

func NewFTSRetriever(storageDir string) *FTSRetriever {
	return &FTSRetriever{
		store: &FTSStore{},
	}
}

func (r *FTSRetriever) Name() string {
	return "fts"
}

func (r *FTSRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.metrics = RetrievalMetrics{}

	hash := sha256.Sum256([]byte(repo.Root))
	dbName := fmt.Sprintf("fts_%x.db", hash[:8])
	dbPath := filepath.Join(repo.Root, ".mycli-fts", dbName)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("failed to open FTS database: %w", err)
	}

	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS file_fts USING fts5(
			file_path UNINDEXED,
			content,
			tokenize='porter unicode61'
		)
	`); err != nil {
		db.Close()
		return fmt.Errorf("failed to create FTS5 table: %w", err)
	}

	if _, err := db.Exec("DELETE FROM file_fts"); err != nil {
		db.Close()
		return fmt.Errorf("failed to clear FTS index: %w", err)
	}

	r.store.db = db
	return r.indexRepo()
}

func (r *FTSRetriever) indexRepo() error {
	start := time.Now()

	tx, err := r.store.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO file_fts (file_path, content) VALUES (?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	scanned := 0

	filepath.Walk(r.repo.Root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if fi.IsDir() {
			return nil
		}
		if isBinaryExt(path) {
			return nil
		}
		if fi.Size() > 1<<20 {
			return nil
		}
		if fi.Size() == 0 {
			return nil
		}

		relPath, _ := filepath.Rel(r.repo.Root, path)
		data, err := os.ReadFile(path)
		if err != nil || isBinary(data) {
			return nil
		}
		content := string(data)
		if strings.TrimSpace(content) == "" {
			return nil
		}

		summary := extractFileSummary(relPath, content)
		summaryLine := buildSummaryLine(summary)
		indexedContent := summaryLine + content

		scanned++
		if _, err := stmt.Exec(relPath, indexedContent); err != nil {
			return nil
		}
		return nil
	})

	if err := tx.Commit(); err != nil {
		return err
	}

	r.metrics = RetrievalMetrics{
		FilesScanned: scanned,
		FilesLoaded:  scanned,
		LatencyMs:    time.Since(start).Milliseconds(),
	}
	return nil
}

func (r *FTSRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	start := time.Now()

	if query == "" {
		return []RetrievalResult{}, nil
	}

	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return []RetrievalResult{}, nil
	}

	rows, err := r.store.db.Query(`
		SELECT file_path, content, rank
		FROM file_fts
		WHERE file_fts MATCH ?
		ORDER BY rank
		LIMIT 50
	`, ftsQuery)
	if err != nil {
		return nil, fmt.Errorf("FTS query failed: %w", err)
	}
	defer rows.Close()

	var results []RetrievalResult
	totalMatches := 0

	for rows.Next() {
		var filePath, content string
		var rank float64
		if err := rows.Scan(&filePath, &content, &rank); err != nil {
			continue
		}

		fullPath := filepath.Join(r.repo.Root, filePath)
		fullContent, err := os.ReadFile(fullPath)
		var contentLines []string
		if err == nil {
			contentLines = strings.Split(string(fullContent), "\n")
		} else {
			contentLines = strings.Split(content, "\n")
		}

		snippet := extractAllSnippets(contentLines, query, 4)
		fileMatches := countMatchesInContent(contentLines, query)
		totalMatches += fileMatches
		boost, boostReasons := computeFileBoost(filePath, r.repo.Root, query)
		qualityConf := ftsConfidence(rank, fileMatches, len(snippet))
		rankingScore := qualityConf * boost
		if rankingScore > 1.0 {
			rankingScore = 1.0
		}

		results = append(results, RetrievalResult{
			File:      fullPath,
			Snippet:   snippet,
			Score:     rankingScore,
			Tokens:    len(snippet) / 4,
			MatchHits: fileMatches,
			Reason:    boostReasons,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	topN := 5
	if len(results) < topN {
		topN = len(results)
	}
	results = results[:topN]

	confidence := 0.0
	if len(results) > 0 {
		confidence = ftsConfidence(0, results[0].MatchHits, len(results[0].Snippet))
	}

	r.metrics = RetrievalMetrics{
		FilesScanned:     r.metrics.FilesScanned,
		FilesLoaded:      len(results),
		Tokens:           totalTokens(results),
		LatencyMs:        time.Since(start).Milliseconds(),
		MatchedFiles:     len(results),
		MatchedSnippets:  len(results),
		MatchCount:       totalMatches,
		Confidence:       confidence,
	}

	return results, nil
}

func buildFTSQuery(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return ""
	}

	var filtered []string
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "have": true,
		"has": true, "had": true, "do": true, "did": true,
		"will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "and": true, "or": true,
		"but": true, "not": true, "this": true, "that": true, "it": true,
		"its": true, "my": true, "your": true, "our": true, "their": true,
		"explain": true, "describe": true, "what": true, "which": true,
		"how": true, "tell": true, "show": true, "find": true,
		"current": true, "flow": true, "please": true, "just": true,
		"get": true, "work": true,
	}

	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()[]{}/\\?")
		w = strings.ToLower(w)
		if len(w) < 2 || stopWords[w] {
			continue
		}
		filtered = append(filtered, w)
	}

	if len(filtered) == 0 {
		return ""
	}

	if len(filtered) == 1 {
		return filtered[0]
	}

	phrase := "\"" + strings.Join(filtered, " ") + "\""
	orTerms := strings.Join(filtered, " OR ")

	return phrase + " OR " + orTerms
}

const maxSnippetLines = 10

func extractAllSnippets(lines []string, query string, contextLines int) string {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	matchLines := make(map[int]bool)
	for i, line := range lines {
		lineLower := strings.ToLower(line)
		for _, qw := range queryWords {
			qw = strings.Trim(qw, ".,!?;:'\"()[]{}/\\")
			if qw != "" && strings.Contains(lineLower, qw) {
				matchLines[i] = true
				break
			}
		}
	}

	if len(matchLines) == 0 {
		return ""
	}

	var matchPositions []int
	for ml := range matchLines {
		matchPositions = append(matchPositions, ml)
	}
	sort.Ints(matchPositions)

	snippetLines := extractSnippetLines(lines, matchPositions, contextLines)
	if len(snippetLines) > maxSnippetLines {
		snippetLines = snippetLines[:maxSnippetLines]
	}
	return strings.Join(snippetLines, "\n")
}

func countMatchesInContent(lines []string, query string) int {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)
	count := 0
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		for _, qw := range queryWords {
			qw = strings.Trim(qw, ".,!?;:'\"()[]{}/\\")
			if qw != "" && strings.Contains(lineLower, qw) {
				count++
				break
			}
		}
	}
	return count
}

func (r *FTSRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *FTSRetriever) Shutdown() error {
	if r.store != nil && r.store.db != nil {
		return r.store.db.Close()
	}
	return nil
}

func computeFileBoost(relPath, repoRoot, query string) (float64, string) {
	boost := 1.0
	var reasons []string

	base := strings.ToLower(filepath.Base(relPath))
	relLower := strings.ToLower(relPath)
	queryWords := strings.Fields(strings.ToLower(query))

	filenameMatch := false
	for _, qw := range queryWords {
		qw = strings.Trim(qw, ".,!?;:'\"()[]{}/\\?")
		if qw != "" && strings.Contains(base, qw) {
			filenameMatch = true
			break
		}
	}
	if filenameMatch {
		boost *= 1.5
		reasons = append(reasons, "filename_match")
	}

	if strings.HasPrefix(relLower, "readme") || relPath == "README.md" || relPath == "README" {
		boost *= 2.0
		reasons = append(reasons, "readme")
	}

	if strings.Contains(relLower, "doc") || strings.Contains(relLower, "docs") {
		boost *= 1.5
		reasons = append(reasons, "docs")
	}

	fullPath := filepath.Join(repoRoot, relPath)
	if data, err := os.ReadFile(fullPath); err == nil {
		contentStr := string(data)
		if strings.Contains(contentStr, "func ") || strings.Contains(contentStr, "def ") {
			boost *= 1.3
			reasons = append(reasons, "has_functions")
		}
		if strings.Contains(contentStr, "class ") || strings.Contains(contentStr, "interface ") {
			boost *= 1.3
			reasons = append(reasons, "has_classes")
		}
		if strings.Contains(contentStr, "// ") || strings.Contains(contentStr, "# ") ||
			strings.Contains(contentStr, "/* ") || strings.Contains(contentStr, "///") {
			boost *= 1.2
			reasons = append(reasons, "has_comments")
		}
	}

	if len(reasons) == 0 {
		return 1.0, "base"
	}
	return boost, strings.Join(reasons, ",")
}

func ftsConfidence(rank float64, matchCount int, snippetLen int) float64 {
	base := 1.0 / (1.0 + rank/2.0)
	if rank < 0 {
		base = 1.0 / (1.0 - rank/2.0)
	}
	quality := 0.12
	if snippetLen > 0 {
		quality += 0.15
	}
	if matchCount > 3 {
		quality += 0.05
	}
	if matchCount > 10 {
		quality += 0.05
	}
	conf := base * quality
	if conf > 0.40 {
		conf = 0.40
	}
	return conf
}

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", "node_modules", "vendor", ".venv", "venv",
		"__pycache__", ".next", "dist", "build", "target",
		".idea", ".vscode", ".DS_Store", ".mycli-fts":
		return true
	}
	return false
}
