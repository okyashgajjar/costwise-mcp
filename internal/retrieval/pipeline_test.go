package retrieval

import (
	"testing"

	"github.com/okyashgajjar/costwise-mcp/internal/answertype"
	"github.com/okyashgajjar/costwise-mcp/internal/classifier"
)

func TestGuessMaxResults(t *testing.T) {
	tests := []struct {
		atype answertype.AnswerType
		want  int
	}{
		{answertype.YesNo, 1},
		{answertype.Location, 3},
		{answertype.Caller, 5},
		{answertype.Reference, 5},
		{answertype.Overview, 8},
		{answertype.Explanation, 5},
		{answertype.Improvement, 10},
		{answertype.FeatureSuggestion, 8},
		{answertype.ArchitectureReview, 10},
		{answertype.RepositoryAnalysis, 15},
		{answertype.Plan, 10},
		{answertype.Agent, 10},
	}
	for _, tc := range tests {
		t.Run(tc.atype.String(), func(t *testing.T) {
			got := GuessMaxResults(answertype.Classification{Type: tc.atype})
			if got != tc.want {
				t.Errorf("GuessMaxResults(%s) = %d, want %d", tc.atype.String(), got, tc.want)
			}
		})
	}
}

func TestGuessContextBudget(t *testing.T) {
	tests := []struct {
		atype answertype.AnswerType
		want  int
	}{
		{answertype.YesNo, 100},
		{answertype.Location, 200},
		{answertype.Caller, 500},
		{answertype.Reference, 500},
		{answertype.Overview, 2000},
		{answertype.Explanation, 3000},
		{answertype.Improvement, 3000},
		{answertype.FeatureSuggestion, 2500},
		{answertype.ArchitectureReview, 3000},
		{answertype.RepositoryAnalysis, 4000},
		{answertype.Plan, 4000},
		{answertype.Agent, 4000},
	}
	for _, tc := range tests {
		t.Run(tc.atype.String(), func(t *testing.T) {
			got := GuessContextBudget(answertype.Classification{Type: tc.atype})
			if got != tc.want {
				t.Errorf("GuessContextBudget(%s) = %d, want %d", tc.atype.String(), got, tc.want)
			}
		})
	}
}

func TestCheckQualityGateEmptyResults(t *testing.T) {
	at := answertype.Classification{Type: answertype.Explanation}
	qc := classifier.Classification{Class: classifier.TextQuery}
	gate := CheckQualityGate(nil, at, qc)
	if gate.Passed {
		t.Error("expected quality gate to fail for nil results")
	}
	if gate.Reason != "No repository context found" {
		t.Errorf("expected reason 'No repository context found', got %q", gate.Reason)
	}
}

func TestCheckQualityGateLowScore(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.05, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.Explanation}
	qc := classifier.Classification{Class: classifier.TextQuery}
	gate := CheckQualityGate(results, at, qc)
	if gate.Passed {
		t.Error("expected quality gate to fail for low score")
	}
}

func TestCheckQualityGateYesNo(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.3, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.YesNo}
	qc := classifier.Classification{Class: classifier.SymbolQuery}
	gate := CheckQualityGate(results, at, qc)
	if !gate.Passed {
		t.Errorf("expected quality gate to pass for YesNo with score>=0.3, got reason=%q", gate.Reason)
	}
}

func TestCheckQualityGateLocation(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.3, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.Location}
	qc := classifier.Classification{Class: classifier.SymbolQuery}
	gate := CheckQualityGate(results, at, qc)
	if !gate.Passed {
		t.Errorf("expected quality gate to pass for Location with score>=0.3, got reason=%q", gate.Reason)
	}
}

func TestCheckQualityGateImprovementNeedEvidence(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.8, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.Improvement}
	qc := classifier.Classification{Class: classifier.TextQuery}
	gate := CheckQualityGate(results, at, qc)
	if gate.Passed {
		t.Error("expected quality gate to fail for Improvement with <3 results")
	}
	if !contains(gate.Reason, "Insufficient evidence") {
		t.Errorf("expected reason about insufficient evidence, got %q", gate.Reason)
	}
}

func TestCheckQualityGateImprovementPass(t *testing.T) {
	results := []RetrievalResult{
		{File: "a.go", Score: 0.8, Snippet: "test1"},
		{File: "b.go", Score: 0.7, Snippet: "test2"},
		{File: "c.go", Score: 0.6, Snippet: "test3"},
	}
	at := answertype.Classification{Type: answertype.Improvement}
	qc := classifier.Classification{Class: classifier.TextQuery}
	gate := CheckQualityGate(results, at, qc)
	if !gate.Passed {
		t.Errorf("expected quality gate to pass for Improvement with >=3 results, got reason=%q", gate.Reason)
	}
}

func TestCheckQualityGateFeatureSuggestionNeedEvidence(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.8, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.FeatureSuggestion}
	gate := CheckQualityGate(results, at, classifier.Classification{})
	if gate.Passed {
		t.Error("expected quality gate to fail for FeatureSuggestion with <3 results")
	}
}

func TestCheckQualityGateArchitectureReviewNeedEvidence(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.8, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.ArchitectureReview}
	gate := CheckQualityGate(results, at, classifier.Classification{})
	if gate.Passed {
		t.Error("expected quality gate to fail for ArchitectureReview with <3 results")
	}
}

func TestCheckQualityGateRepoAnalysisNeedEvidence(t *testing.T) {
	results := []RetrievalResult{{File: "a.go", Score: 0.8, Snippet: "test"}}
	at := answertype.Classification{Type: answertype.RepositoryAnalysis}
	gate := CheckQualityGate(results, at, classifier.Classification{})
	if gate.Passed {
		t.Error("expected quality gate to fail for RepositoryAnalysis with <3 results")
	}
}

func TestBuildSystemPromptYesNo(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.YesNo}, "test")
	if !contains(p, "YES") || !contains(p, "NO") {
		t.Errorf("YesNo prompt should mention YES/NO, got: %s", p)
	}
}

func TestBuildSystemPromptLocation(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.Location}, "test")
	if !contains(p, "file path") {
		t.Errorf("Location prompt should mention file path, got: %s", p)
	}
}

func TestBuildSystemPromptImprovement(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.Improvement}, "test")
	if !contains(p, "Impact") || !contains(p, "Effort") || !contains(p, "Confidence") {
		t.Errorf("Improvement prompt should mention Impact, Effort, Confidence, got: %s", p)
	}
	if !contains(p, "Maximum 5") {
		t.Errorf("Improvement prompt should mention max 5, got: %s", p)
	}
}

func TestBuildSystemPromptRepoAnalysis(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.RepositoryAnalysis}, "test")
	if !contains(p, "Purpose") || !contains(p, "Architecture") || !contains(p, "300 output tokens") {
		t.Errorf("RepositoryAnalysis prompt should mention Purpose/Architecture/300 tokens, got: %s", p)
	}
}

func TestBuildSystemPromptArchitectureReview(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.ArchitectureReview}, "test")
	if !contains(p, "Severity") {
		t.Errorf("ArchitectureReview prompt should mention Severity, got: %s", p)
	}
}

func TestBuildSystemPromptPlan(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.Plan}, "test")
	if !contains(p, "step-by-step") {
		t.Errorf("Plan prompt should mention step-by-step, got: %s", p)
	}
}

func TestBuildSystemPromptDefault(t *testing.T) {
	p := BuildSystemPrompt(answertype.Classification{Type: answertype.Overview}, "test")
	if p == "" {
		t.Error("expected non-empty prompt for Overview")
	}
}

func TestPipelineStepString(t *testing.T) {
	tests := []struct {
		step PipelineStep
		want string
	}{
		{StepKnowledgeMemory, "memory"},
		{StepLRUCache, "cache"},
		{StepSymbolDB, "symbol_db"},
		{StepReferenceCallGraph, "ref_callgraph"},
		{StepGrepGlob, "grep_glob"},
		{StepLLM, "llm"},
		{PipelineStep(99), "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.step.String()
			if got != tc.want {
				t.Errorf("(%d).String() = %q, want %q", tc.step, got, tc.want)
			}
		})
	}
}

func TestExtractSymbolFromQuery(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"Where is RepoMap implemented?", "RepoMap"},
		{"Who calls SymbolRetriever?", "SymbolRetriever"},
		{"Explain how KnowledgeStore works", "KnowledgeStore"},
		{"Show callers of ParseLevel", "ParseLevel"},
		{"hello world", "world"},
		{"", ""},
		{"a b c", "c"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			got := ExtractSymbolFromQuery(tc.query)
			if got != tc.want {
				t.Errorf("ExtractSymbolFromQuery(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestIsLikelySymbol(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"RepoMap", true},
		{"SymbolRetriever", true},
		{"ParseLevel", true},
		{"hello", false},
		{"a", false},
		{"F", false},
		{"HasInternal_Upper", true},
	}
	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			got := IsLikelySymbol(tc.s)
			if got != tc.want {
				t.Errorf("IsLikelySymbol(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
