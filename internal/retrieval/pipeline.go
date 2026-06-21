package retrieval

import (
	"fmt"
	"strings"

	"github.com/okyashgajjar/costwise-mcp/internal/answertype"
	"github.com/okyashgajjar/costwise-mcp/internal/classifier"
)

type PipelineStep int

const (
	StepKnowledgeMemory PipelineStep = iota
	StepLRUCache
	StepSymbolDB
	StepReferenceCallGraph
	StepGrepGlob
	StepLLM
)

func (s PipelineStep) String() string {
	switch s {
	case StepKnowledgeMemory:
		return "memory"
	case StepLRUCache:
		return "cache"
	case StepSymbolDB:
		return "symbol_db"
	case StepReferenceCallGraph:
		return "ref_callgraph"
	case StepGrepGlob:
		return "grep_glob"
	case StepLLM:
		return "llm"
	default:
		return "unknown"
	}
}

type PipelineResult struct {
	AnswerType        answertype.Classification
	QueryClass        classifier.Classification
	Results           []RetrievalResult
	Context           string
	ContextTokens     int
	Source            PipelineStep
	MemoryHit         bool
	CacheHit          bool
	Confidence        float64
	PassedQualityGate bool
}

type PipelineGateResult struct {
	Passed bool
	Reason string
	Step   PipelineStep
}

type PipelineDiagnostics struct {
	Repository     string `json:"repository"`
	Classification string `json:"classification"`
	AnswerType     string `json:"answer_type"`
	Retriever      string `json:"retriever"`
	MemoryHit      bool   `json:"memory_hit"`
	CacheHit       bool   `json:"cache_hit"`
	ContextTokens  int    `json:"context_tokens"`
	PromptTokens   int    `json:"prompt_tokens"`
	OutputTokens   int    `json:"output_tokens"`
	LatencyMs      int64  `json:"latency_ms"`
}

type PipelineMetrics struct {
	Repository     string
	Classification string
	AnswerType     string
	Retriever      string
	MemoryHit      bool
	CacheHit       bool
	ContextTokens  int
	PromptTokens   int
	OutputTokens   int
	LatencyMs      int64
}

func CheckQualityGate(results []RetrievalResult, answerType answertype.Classification, queryClass classifier.Classification) PipelineGateResult {
	if len(results) == 0 {
		return PipelineGateResult{
			Passed: false,
			Reason: "No repository context found",
			Step:   StepLLM,
		}
	}

	topScore := results[0].Score
	if topScore < 0.15 {
		return PipelineGateResult{
			Passed: false,
			Reason: "No repository context found",
			Step:   StepLLM,
		}
	}

	if answerType.Type == answertype.YesNo || answerType.Type == answertype.Location {
		if topScore >= 0.3 {
			return PipelineGateResult{
				Passed: true,
				Reason: fmt.Sprintf("top_score=%.2f", topScore),
				Step:   StepLLM,
			}
		}
	}

	needEvidence := answerType.Type == answertype.Improvement ||
		answerType.Type == answertype.RepositoryAnalysis ||
		answerType.Type == answertype.ArchitectureReview ||
		answerType.Type == answertype.FeatureSuggestion
	if needEvidence && len(results) < 3 {
		return PipelineGateResult{
			Passed: false,
			Reason: fmt.Sprintf("Insufficient evidence: need >= 3 results, got %d", len(results)),
			Step:   StepLLM,
		}
	}

	return PipelineGateResult{
		Passed: topScore >= 0.15,
		Reason: fmt.Sprintf("top_score=%.2f", topScore),
		Step:   StepLLM,
	}
}

func GuessMaxResults(answerType answertype.Classification) int {
	switch answerType.Type {
	case answertype.YesNo:
		return 1
	case answertype.Location:
		return 3
	case answertype.Caller:
		return 5
	case answertype.Reference:
		return 5
	case answertype.Overview:
		return 8
	case answertype.Explanation:
		return 5
	case answertype.Improvement:
		return 10
	case answertype.FeatureSuggestion:
		return 8
	case answertype.ArchitectureReview:
		return 10
	case answertype.RepositoryAnalysis:
		return 15
	case answertype.Plan:
		return 10
	case answertype.Agent:
		return 10
	default:
		return 5
	}
}

func GuessContextBudget(answerType answertype.Classification) int {
	switch answerType.Type {
	case answertype.YesNo:
		return 100
	case answertype.Location:
		return 200
	case answertype.Caller:
		return 500
	case answertype.Reference:
		return 500
	case answertype.Overview:
		return 2000
	case answertype.Explanation:
		return 3000
	case answertype.Improvement:
		return 3000
	case answertype.FeatureSuggestion:
		return 2500
	case answertype.ArchitectureReview:
		return 3000
	case answertype.RepositoryAnalysis:
		return 4000
	case answertype.Plan:
		return 4000
	case answertype.Agent:
		return 4000
	default:
		return 2000
	}
}

func BuildSystemPrompt(answerType answertype.Classification, query string) string {
	switch answerType.Type {
	case answertype.YesNo:
		return "You are a code intelligence assistant. Answer YES or NO based on the repository context. Respond with ONLY 'Yes.' or 'No.' Do NOT add any explanation."
	case answertype.Location:
		return "You are a code intelligence assistant. Answer with the exact file path and line number where the symbol is defined. Format: `file/path.go:line`. Do NOT add any explanation."
	case answertype.Caller:
		return "You are a code intelligence assistant. List the files that call the given symbol. Do NOT include source code. Do NOT add any explanation."
	case answertype.Reference:
		return "You are a code intelligence assistant. List the files that reference, import, or depend on the given symbol. Do NOT include source code. Do NOT add any explanation."
	case answertype.Overview:
		return "You are a code intelligence assistant. Provide a brief overview based on the repository context. Be concise."
	case answertype.Explanation:
		return "You are a code intelligence assistant. Explain the requested concept based on the repository context. Be concise."
	case answertype.Improvement:
		return "You are a code intelligence assistant. The user wants suggestions for improving this repository. Based ONLY on the repository summary provided below, suggest concrete improvements. Each improvement must include:\n- Title (short)\n- Impact (High/Medium/Low)\n- Effort (High/Medium/Low)\n- Confidence (0.0-1.0)\n\nSort: High Impact, Low Effort first. Maximum 5 items. No generic advice. No explanations beyond the ranked list."
	case answertype.FeatureSuggestion:
		return "You are a code intelligence assistant. The user wants feature suggestions for this repository. Based ONLY on the repository summary, suggest specific features that align with the codebase. Each suggestion must include:\n- Title\n- Impact (High/Medium/Low)\n- Effort (High/Medium/Low)\n- Confidence (0.0-1.0)\n\nMaximum 5 items. No generic advice."
	case answertype.ArchitectureReview:
		return "You are a code intelligence assistant. Review the architecture of this repository based on the provided context. Each finding must include:\n- Area (e.g. module/file name)\n- Observation (specific, actionable)\n- Severity (High/Medium/Low)\n\nMaximum 5 items. Focus on concrete patterns in the codebase. No generic advice."
	case answertype.RepositoryAnalysis:
		return "You are a code intelligence assistant. Analyze this repository and provide:\n- Purpose (1 sentence)\n- Architecture (2-3 sentences)\n- Strengths (2-3 bullet points)\n- Weaknesses (2-3 bullet points)\n- Top 5 improvements (sorted by impact)\n\nBudget: <300 output tokens. Base EVERYTHING on the repository context below. No generic statements."
	case answertype.Plan:
		return "You are a code intelligence assistant. Generate a step-by-step implementation plan based on the repository context. Focus on what files need to change."
	case answertype.Agent:
		return "You are an autonomous code intelligence agent. You have tools available to read, search, and modify the repository."
	default:
		return "You are a code intelligence assistant. Answer based on the provided repository context. Be concise."
	}
}

func FormatDiagnostics(diag PipelineDiagnostics) string {
	var b strings.Builder
	b.WriteString("Repository: ")
	b.WriteString(diag.Repository)
	b.WriteString("\n")
	b.WriteString("Classification: ")
	b.WriteString(diag.Classification)
	b.WriteString("\n")
	b.WriteString("Retriever: ")
	b.WriteString(diag.Retriever)
	b.WriteString("\n")
	b.WriteString("Memory Hit: ")
	fmt.Fprintf(&b, "%t", diag.MemoryHit)
	b.WriteString("\n")
	b.WriteString("Cache Hit: ")
	fmt.Fprintf(&b, "%t", diag.CacheHit)
	b.WriteString("\n")
	b.WriteString("Context Tokens: ")
	fmt.Fprintf(&b, "%d", diag.ContextTokens)
	b.WriteString("\n")
	b.WriteString("Prompt Tokens: ")
	fmt.Fprintf(&b, "%d", diag.PromptTokens)
	b.WriteString("\n")
	b.WriteString("Output Tokens: ")
	fmt.Fprintf(&b, "%d", diag.OutputTokens)
	b.WriteString("\n")
	b.WriteString("Latency: ")
	fmt.Fprintf(&b, "%d ms", diag.LatencyMs)
	return b.String()
}

func ExtractSymbolFromQuery(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Fields(query)
	for _, w := range words {
		w = strings.TrimRight(w, "?,.:;!")
		if len(w) < 2 {
			continue
		}
		if w[0] >= 'A' && w[0] <= 'Z' {
			hasInternalUpper := false
			for _, c := range w[1:] {
				if c >= 'A' && c <= 'Z' || c == '_' {
					hasInternalUpper = true
					break
				}
			}
			if hasInternalUpper {
				return w
			}
		}
	}

	if len(words) > 0 {
		return words[len(words)-1]
	}
	return ""
}

func IsLikelySymbol(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] >= 'A' && s[0] <= 'Z' {
		for _, c := range s[1:] {
			if c >= 'A' && c <= 'Z' || c == '_' {
				return true
			}
		}
	}
	return strings.Contains(s, "_") && len(s) >= 3
}
