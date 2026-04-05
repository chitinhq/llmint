// basic demonstrates a dedup + account pipeline using the mock provider.
// The same request is sent twice; the second call is served from the
// deduplication cache, showing a CacheHit and a non-zero token savings.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/middleware/account"
	"github.com/chitinhq/llmint/middleware/dedup"
	"github.com/chitinhq/llmint/provider/mock"
)

func main() {
	// Build pipeline: dedup → account → mock provider.
	base := mock.New("mock-model", "The answer is 42.")
	sink := account.NewStdoutSink()
	store := dedup.NewMemoryStore()

	pipeline := llmint.Chain(
		dedup.New(store),
		account.New(sink),
	)(base)

	req := &llmint.Request{
		Model: "mock-model",
		Messages: []llmint.Message{
			{Role: "user", Content: "What is the answer to life, the universe, and everything?"},
		},
		MaxTokens: 256,
	}

	ctx := context.Background()

	// First call — cache miss, hits the mock provider.
	resp1, err := pipeline.Complete(ctx, req)
	if err != nil {
		log.Fatalf("first call failed: %v", err)
	}
	printResult("Call 1", resp1, os.Stdout)

	// Second call — identical request, served from dedup cache.
	resp2, err := pipeline.Complete(ctx, req)
	if err != nil {
		log.Fatalf("second call failed: %v", err)
	}
	printResult("Call 2", resp2, os.Stdout)
}

func printResult(label string, resp *llmint.Response, w *os.File) {
	text := ""
	if len(resp.Content) > 0 {
		text = resp.Content[0].Text
	}
	fmt.Fprintf(w, "[%s] model=%s cache=%s text=%q\n",
		label, resp.Model, resp.CacheStatus, text)
	fmt.Fprintf(w, "       tokens in=%d out=%d\n",
		resp.Usage.InputTokens, resp.Usage.OutputTokens)
	for _, s := range resp.Savings {
		fmt.Fprintf(w, "       savings: %d tokens via %s\n", s.TokensSaved, s.Technique)
	}
}
