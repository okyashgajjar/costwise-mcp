package retrieval

import (
	"strings"
	"testing"

	"github.com/okyashgajjar/costwise-mcp/internal/answertype"
)

func TestNeedsCompression(t *testing.T) {
	tests := []struct {
		outputTokens int
		atype        answertype.AnswerType
		want         bool
	}{
		{5, answertype.YesNo, false},
		{21, answertype.YesNo, true},
		{100, answertype.Location, true},
		{40, answertype.Location, false},
		{0, answertype.Agent, false},
		{1000, answertype.Agent, false},
		{400, answertype.Explanation, false},
		{801, answertype.Explanation, true},
	}
	for _, tc := range tests {
		t.Run(tc.atype.String(), func(t *testing.T) {
			atc := answertype.Classification{Type: tc.atype}
			got := NeedsCompression(tc.outputTokens, atc)
			if got != tc.want {
				t.Errorf("NeedsCompression(%d, %s) = %v, want %v", tc.outputTokens, tc.atype.String(), got, tc.want)
			}
		})
	}
}

func TestBuildCompressionPrompt(t *testing.T) {
	content := "This is a long answer that needs to be shortened into something much more concise and to the point."
	prompt := BuildCompressionPrompt(content)

	if !strings.Contains(prompt, "shortest possible form") {
		t.Error("compression prompt should mention 'shortest possible form'")
	}
	if !strings.Contains(prompt, "actionable information") {
		t.Error("compression prompt should mention 'actionable information'")
	}
	if !strings.Contains(prompt, "Remove explanations") {
		t.Error("compression prompt should mention 'Remove explanations'")
	}
	if !strings.Contains(prompt, content) {
		t.Error("compression prompt should include original content")
	}
}

func TestBuildCompressionPromptEmpty(t *testing.T) {
	prompt := BuildCompressionPrompt("")
	if prompt == "" {
		t.Error("expected non-empty prompt even for empty content")
	}
}

func TestNeedsCompressionExactThreshold(t *testing.T) {
	atc := answertype.Classification{Type: answertype.Explanation}
	threshold := answertype.Explanation.MaxTokens() * 2

	got := NeedsCompression(threshold, atc)
	if got {
		t.Errorf("NeedsCompression(%d, Explanation) should be false at exact threshold %d", threshold, threshold)
	}

	got = NeedsCompression(threshold+1, atc)
	if !got {
		t.Errorf("NeedsCompression(%d, Explanation) should be true above threshold %d", threshold+1, threshold)
	}
}

func TestNeedsCompressionImprovement(t *testing.T) {
	atc := answertype.Classification{Type: answertype.Improvement}

	if NeedsCompression(400, atc) {
		t.Error("Improvement 400 tokens should not need compression (budget=200, 2x=400)")
	}
	if !NeedsCompression(401, atc) {
		t.Error("Improvement 401 tokens should need compression")
	}
}
