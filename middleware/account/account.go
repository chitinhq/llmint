package account

import (
	"context"
	"time"

	"github.com/chitinhq/llmint"
)

// options holds configuration for the account middleware.
type options struct{}

// Option is a functional option for the account middleware.
type Option func(*options)

// New returns an accounting Middleware that records a usage Entry to sink
// after every successful completion. The response is passed through unmodified.
func New(sink Sink, opts ...Option) llmint.Middleware {
	return func(next llmint.Provider) llmint.Provider {
		return &accountProvider{next: next, sink: sink}
	}
}

type accountProvider struct {
	next llmint.Provider
	sink Sink
}

func (p *accountProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	start := time.Now()

	resp, err := p.next.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	entry := Entry{
		Timestamp:   start,
		Model:       resp.Model,
		RequestHash: req.Hash(),
		InputTokens: resp.Usage.InputTokens,
		Usage:       resp.Usage,
		Savings:     resp.Savings,
		Duration:    time.Since(start),
		Metadata:    req.Metadata,
	}
	_ = p.sink.Record(entry)

	return resp, nil
}

func (p *accountProvider) Name() string                  { return p.next.Name() }
func (p *accountProvider) Models() []llmint.ModelInfo    { return p.next.Models() }
