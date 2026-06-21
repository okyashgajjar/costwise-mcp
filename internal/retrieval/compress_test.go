package retrieval

import (
	"testing"

	"github.com/okyashgajjar/costwise-mcp/internal/answertype"
)

func TestCompressForAnswerTypeYesNo(t *testing.T) {
	results := []RetrievalResult{
		{File: "a.go", Score: 0.9, Snippet: "func Foo() bool { return true }", LineFrom: 10},
	}
	atc := answertype.Classification{Type: answertype.YesNo}
	compressed := CompressForAnswerType(results, atc, 100)

	if !compressed.Passed {
		t.Fatal("expected compression to pass for YesNo")
	}
	if compressed.Context == "" {
		t.Fatal("expected non-empty context for YesNo")
	}
	if !contains(compressed.Context, "a.go") {
		t.Errorf("expected file 'a.go' in compressed context, got: %s", compressed.Context)
	}
}

func TestCompressForAnswerTypeLocation(t *testing.T) {
	results := []RetrievalResult{
		{File: "internal/repomap/repomap.go", Score: 0.9, LineFrom: 45},
		{File: "internal/repomap/indexer.go", Score: 0.8, LineFrom: 12},
		{File: "internal/retrieval/pipeline.go", Score: 0.7, LineFrom: 88},
		{File: "cmd/chat.go", Score: 0.6, LineFrom: 33},
	}
	atc := answertype.Classification{Type: answertype.Location}
	compressed := CompressForAnswerType(results, atc, 200)

	if !compressed.Passed {
		t.Fatal("expected compression to pass for Location")
	}
	if compressed.Context == "" {
		t.Fatal("expected non-empty context for Location")
	}
}

func TestCompressForAnswerTypeEmptyResults(t *testing.T) {
	atc := answertype.Classification{Type: answertype.YesNo}
	compressed := CompressForAnswerType(nil, atc, 100)

	if compressed.Passed {
		t.Error("expected compression to fail for nil results")
	}
	if compressed.Context != "" {
		t.Errorf("expected empty context, got %q", compressed.Context)
	}
}

func TestCompressForAnswerTypeCaller(t *testing.T) {
	results := []RetrievalResult{
		{File: "main.go", Score: 0.9, Snippet: "func main() {\n  RepoMap.Build()\n}"},
		{File: "server.go", Score: 0.8, Snippet: "func Serve() {\n  RepoMap.Load()\n}"},
	}
	atc := answertype.Classification{Type: answertype.Caller}
	compressed := CompressForAnswerType(results, atc, 500)

	if !compressed.Passed {
		t.Fatal("expected compression to pass for Caller")
	}
	if !contains(compressed.Context, "Callers:") {
		t.Errorf("expected 'Callers:' header, got: %s", compressed.Context)
	}
}

func TestCompressForAnswerTypeReference(t *testing.T) {
	results := []RetrievalResult{
		{File: "usage.go", Score: 0.9, Snippet: "import \"github.com/okyashgajjar/costwise-mcp/internal/repomap\""},
		{File: "test_test.go", Score: 0.8, Snippet: "func TestRepoMap(t *testing.T) {}"},
	}
	atc := answertype.Classification{Type: answertype.Reference}
	compressed := CompressForAnswerType(results, atc, 500)

	if !compressed.Passed {
		t.Fatal("expected compression to pass for Reference")
	}
	if !contains(compressed.Context, "References:") {
		t.Errorf("expected 'References:' header, got: %s", compressed.Context)
	}
}

func TestCompressForAnswerTypeOverview(t *testing.T) {
	results := []RetrievalResult{
		{File: "main.go", Score: 1.0, Snippet: "package main\nfunc main() {}"},
		{File: "config.go", Score: 0.9, Snippet: "package config\nvar DefaultConfig = ..."},
	}
	atc := answertype.Classification{Type: answertype.Overview}
	compressed := CompressForAnswerType(results, atc, 2000)

	if !compressed.Passed {
		t.Fatal("expected compression to pass for Overview")
	}
	if !contains(compressed.Context, "File:") {
		t.Errorf("expected 'File:' prefix, got: %s", compressed.Context)
	}
}

func TestCompressDefault(t *testing.T) {
	results := []RetrievalResult{
		{File: "a.go", Score: 0.9, Snippet: "package a\nfunc A() {}"},
	}
	atc := answertype.Classification{Type: answertype.Explanation}
	compressed := CompressForAnswerType(results, atc, 1000)

	if !compressed.Passed {
		t.Fatal("expected compression to pass for default types")
	}
	if compressed.Context == "" {
		t.Fatal("expected non-empty context")
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/repo/internal/repomap/repomap.go", "internal/repomap/repomap.go"},
		{"/repo/cmd/chat.go", "cmd/chat.go"},
		{"/repo/pkg/utils/helper.go", "pkg/utils/helper.go"},
		{"simple.go", "simple.go"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := shortenPath(tc.path)
			if got != tc.want {
				t.Errorf("shortenPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestCompressForAnswerTypeBudgets(t *testing.T) {
	results := []RetrievalResult{
		{File: "a.go", Score: 0.9, Snippet: "line1\nline2\nline3\nline4\nline5"},
		{File: "b.go", Score: 0.8, Snippet: "line1\nline2\nline3"},
		{File: "c.go", Score: 0.8, Snippet: "line1\nline2\nline3\nline4"},
	}

	atc := answertype.Classification{Type: answertype.Overview}
	c1 := CompressForAnswerType(results, atc, 10)
	c2 := CompressForAnswerType(results, atc, 2000)

	if c1.Context == "" {
		t.Error("expected non-empty context even with small budget")
	}
	if c2.Context == "" {
		t.Error("expected non-empty context with large budget")
	}
}
