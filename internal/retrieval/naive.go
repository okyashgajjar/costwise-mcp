package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okyashgajjar/costaffective-mcp/internal/repository"
)

type NaiveRetriever struct {
	repo    *repository.RepositoryInfo
	results []RetrievalResult
	metrics RetrievalMetrics
}

func NewNaiveRetriever() *NaiveRetriever {
	return &NaiveRetriever{}
}

func (r *NaiveRetriever) Name() string {
	return "naive"
}

func (r *NaiveRetriever) Initialize(ctx context.Context, repo *repository.RepositoryInfo) error {
	r.repo = repo
	r.results = nil
	r.metrics = RetrievalMetrics{}
	return r.buildIndex()
}

func (r *NaiveRetriever) buildIndex() error {
	start := time.Now()
	var results []RetrievalResult
	scanned := 0
	loaded := 0

	if r.repo.ReadmePath != "" {
		if fi, err := os.Stat(r.repo.ReadmePath); err == nil && fi.Size() < 1<<20 {
			scanned++
			data, err := os.ReadFile(r.repo.ReadmePath)
			if err == nil && !isBinary(data) {
				loaded++
				results = append(results, RetrievalResult{
					File:    r.repo.ReadmePath,
					Snippet: string(data),
					Score:   1.0,
					Tokens:  len(data) / 4,
				})
			}
		}
	}

	if r.repo.DocsDir != "" {
		filepath.Walk(r.repo.DocsDir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || fi.Size() >= 1<<20 {
				return nil
			}
			if isBinaryExt(path) {
				return nil
			}
			scanned++
			data, err := os.ReadFile(path)
			if err != nil || isBinary(data) {
				return nil
			}
			loaded++
			results = append(results, RetrievalResult{
				File:    path,
				Snippet: string(data),
				Score:   0.8,
				Tokens:  len(data) / 4,
			})
			return nil
		})
	}

	entries, err := os.ReadDir(r.repo.Root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(r.repo.Root, entry.Name())
		fi, err := os.Stat(path)
		if err != nil || fi.Size() >= 1<<20 {
			continue
		}
		if isBinaryExt(path) {
			continue
		}
		scanned++
		data, err := os.ReadFile(path)
		if err != nil || isBinary(data) {
			continue
		}
		loaded++
		results = append(results, RetrievalResult{
			File:    path,
			Snippet: string(data),
			Score:   0.5,
			Tokens:  len(data) / 4,
		})
	}

	r.results = results
	r.metrics = RetrievalMetrics{
		FilesScanned:     scanned,
		FilesLoaded:      loaded,
		Tokens:           totalTokens(results),
		LatencyMs:        time.Since(start).Milliseconds(),
		MatchedFiles:     loaded,
		MatchedSnippets:  loaded,
		MatchCount:       loaded,
		Confidence:       1.0,
	}
	return nil
}

func (r *NaiveRetriever) Retrieve(ctx context.Context, query string) ([]RetrievalResult, error) {
	return r.results, nil
}

func (r *NaiveRetriever) Metrics() RetrievalMetrics {
	return r.metrics
}

func (r *NaiveRetriever) Shutdown() error {
	r.results = nil
	return nil
}

func totalTokens(results []RetrievalResult) int {
	total := 0
	for _, res := range results {
		total += res.Tokens
	}
	return total
}

var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".ico": true, ".svg": true, ".woff": true, ".woff2": true,
	".ttf": true, ".eot": true, ".pdf": true, ".zip": true,
	".tar": true, ".gz": true, ".exe": true, ".dll": true,
	".so": true, ".dylib": true, ".bin": true, ".o": true,
	".a": true, ".pyc": true, ".class": true,
}

func isBinaryExt(path string) bool {
	return binaryExts[filepath.Ext(path)]
}

func isBinary(data []byte) bool {
	if len(data) > 1024 {
		for _, b := range data[:1024] {
			if b == 0 {
				return true
			}
		}
	} else {
		for _, b := range data {
			if b == 0 {
				return true
			}
		}
	}
	return false
}
