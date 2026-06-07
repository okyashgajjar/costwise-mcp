package retrieval

import (
	"github.com/okyashgajjar/costaffective-mcp/internal/answertype"
)

const responseCompressionPrompt = `Rewrite the following answer in the shortest possible form.
Keep only actionable information.
Remove explanations.
Remove examples.
Remove filler words.
Use bullet points if helpful.
Stay factual — do NOT add anything not in the original.

Original:
%s

Shortened:`

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
