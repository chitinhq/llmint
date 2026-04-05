package cascade

import (
	"testing"

	"github.com/chitinhq/llmint"
)

func textResp(text string) *llmint.Response {
	return &llmint.Response{
		Content: []llmint.ContentBlock{{Type: "text", Text: text}},
	}
}

func toolResp() *llmint.Response {
	return &llmint.Response{
		Content: []llmint.ContentBlock{{Type: "tool_use", Text: ""}},
	}
}

func TestDefaultScorerHighConfidence(t *testing.T) {
	scorer := DefaultScorer()
	resp := textResp("Here is your answer. [confidence: 0.95]")
	got := scorer(resp)
	if got != 0.95 {
		t.Errorf("expected 0.95, got %v", got)
	}
}

func TestDefaultScorerLowConfidence(t *testing.T) {
	scorer := DefaultScorer()
	resp := textResp("I am not sure. [confidence: 0.3]")
	got := scorer(resp)
	if got != 0.3 {
		t.Errorf("expected 0.3, got %v", got)
	}
}

func TestDefaultScorerNoTag(t *testing.T) {
	scorer := DefaultScorer()
	resp := textResp("No confidence tag in this response at all.")
	got := scorer(resp)
	if got != 0.5 {
		t.Errorf("expected 0.5 (default), got %v", got)
	}
}

func TestCustomScorer(t *testing.T) {
	// A custom scorer that returns 1.0 only when a tool_use block is present.
	toolUseScorer := func(resp *llmint.Response) float64 {
		for _, b := range resp.Content {
			if b.Type == "tool_use" {
				return 1.0
			}
		}
		return 0.0
	}

	if got := toolUseScorer(toolResp()); got != 1.0 {
		t.Errorf("expected 1.0 for tool_use response, got %v", got)
	}
	if got := toolUseScorer(textResp("plain text")); got != 0.0 {
		t.Errorf("expected 0.0 for text response, got %v", got)
	}
}
