package retrieval

import (
	"github.com/okyashgajjar/costaffective-mcp/internal/answertype"
)

func NeedsCompression(outputTokens int, answerType answertype.Classification) bool {
	maxTokens := answerType.Type.MaxTokens()
	if maxTokens <= 0 {
		return false
	}
	return outputTokens > maxTokens*2
}

func BuildCompressionPrompt(content string) string {
	return "Rewrite answer in the shortest possible form. Keep only actionable information. Remove explanations. Remove examples.\n\nOriginal:\n" + content + "\n\nShortened:"
}
