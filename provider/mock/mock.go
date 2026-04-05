// Package mock provides a deterministic in-memory Provider for use in tests
// and pipeline development. It returns a configured response or error and
// tracks the number of times Complete has been called.
package mock

import (
	"context"
	"sync/atomic"

	"github.com/chitinhq/llmint"
)

// Provider is a mock llmint.Provider that returns a pre-configured response.
type Provider struct {
	model    string
	response string
	err      error
	calls    int64 // accessed via sync/atomic
}

// New returns a mock Provider that always succeeds with the given responseText.
func New(model, response string) *Provider {
	return &Provider{model: model, response: response}
}

// NewWithError returns a mock Provider that always returns err from Complete.
func NewWithError(model string, err error) *Provider {
	return &Provider{model: model, err: err}
}

// Complete increments the call counter and returns the configured
// response or error. Input tokens are estimated at ~4 chars/token (min 10);
// output tokens are estimated from the response text length (min 10).
func (p *Provider) Complete(_ context.Context, req *llmint.Request) (*llmint.Response, error) {
	atomic.AddInt64(&p.calls, 1)

	if p.err != nil {
		return nil, p.err
	}

	// Estimate input tokens from combined message content.
	inputChars := 0
	for _, m := range req.Messages {
		inputChars += len(m.Content)
	}
	inputTokens := max(inputChars/4, 10)

	outputTokens := max(len(p.response)/4, 10)

	return &llmint.Response{
		Content: []llmint.ContentBlock{
			{Type: "text", Text: p.response},
		},
		Usage: llmint.Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
		Model:       p.model,
		CacheStatus: llmint.CacheMiss,
	}, nil
}

// Name returns "mock".
func (p *Provider) Name() string { return "mock" }

// Models returns a single ModelInfo entry for the configured model.
func (p *Provider) Models() []llmint.ModelInfo {
	return []llmint.ModelInfo{{ID: p.model}}
}

// CallCount returns how many times Complete has been called.
func (p *Provider) CallCount() int {
	return int(atomic.LoadInt64(&p.calls))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
