// cascade demonstrates the cascade middleware: a low-confidence haiku tier
// escalates to a sonnet tier, showing model switching in the response.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/middleware/cascade"
	"github.com/chitinhq/llmint/provider/mock"
)

// shortScorer always returns a low confidence score to force escalation.
func shortScorer(resp *llmint.Response) float64 {
	if len(resp.Content) == 0 {
		return 0.0
	}
	// Responses shorter than 20 chars get a low score.
	if len(resp.Content[0].Text) < 20 {
		return 0.1
	}
	return 0.9
}

func main() {
	// Haiku tier — short response triggers escalation.
	haiku := mock.New("claude-3-haiku-20240307", "Short.")
	// Sonnet tier — longer, more complete response.
	sonnet := mock.New("claude-3-5-sonnet-20241022", "Here is a detailed and thorough answer to your question.")

	pipeline := llmint.Chain(
		cascade.New(
			[]cascade.Model{
				{Provider: haiku, Name: "claude-3-haiku-20240307", Threshold: 0.5},
				{Provider: sonnet, Name: "claude-3-5-sonnet-20241022", Threshold: 0},
			},
			cascade.WithScorer(shortScorer),
		),
	)(nil) // cascade ignores the base provider

	req := &llmint.Request{
		Model: "claude-3-haiku-20240307",
		Messages: []llmint.Message{
			{Role: "user", Content: "Explain token economics in LLMs."},
		},
		MaxTokens: 512,
	}

	resp, err := pipeline.Complete(context.Background(), req)
	if err != nil {
		log.Fatalf("cascade failed: %v", err)
	}

	text := ""
	if len(resp.Content) > 0 {
		text = resp.Content[0].Text
	}

	fmt.Printf("model used:   %s\n", resp.Model)
	fmt.Printf("cache status: %s\n", resp.CacheStatus)
	fmt.Printf("response:     %q\n", text)
	fmt.Printf("tokens:       in=%d out=%d\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
