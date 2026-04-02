package distill

import (
	"context"

	"github.com/AgentGuardHQ/llmint"
)

const defaultMinSavings = 0.0

// options holds configuration for the distill middleware.
type options struct {
	minSavings float64 // minimum fractional savings required (0.0–1.0)
}

// Option is a functional option for the distill middleware.
type Option func(*options)

// WithMinSavings sets the minimum fractional savings required before the
// distilled prompt is used. For example, 0.5 means the distilled version must
// be at least 50% shorter than the original. Default is 0.0 (always replace
// if found).
func WithMinSavings(pct float64) Option {
	return func(o *options) { o.minSavings = pct }
}

// New returns a Middleware that replaces a request's system prompt with a
// distilled equivalent looked up from lib, provided the savings percentage
// meets the minSavings threshold.
func New(lib Library, opts ...Option) llmint.Middleware {
	cfg := options{minSavings: defaultMinSavings}
	for _, o := range opts {
		o(&cfg)
	}

	return func(downstream llmint.Provider) llmint.Provider {
		return &distillProvider{
			downstream: downstream,
			lib:        lib,
			cfg:        cfg,
		}
	}
}

type distillProvider struct {
	downstream llmint.Provider
	lib        Library
	cfg        options
}

func (p *distillProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	original := req.System

	distilled, ok := p.lib.Lookup(original)
	if !ok {
		// No distilled version registered — pass through unchanged.
		return p.downstream.Complete(ctx, req)
	}

	// Check savings threshold.
	originalLen := len(original)
	distilledLen := len(distilled)

	var savings float64
	if originalLen > 0 {
		savings = float64(originalLen-distilledLen) / float64(originalLen)
	}

	if savings < p.cfg.minSavings {
		// Savings below threshold — pass through unchanged.
		return p.downstream.Complete(ctx, req)
	}

	// Replace system prompt with distilled version.
	modifiedReq := *req
	modifiedReq.System = distilled

	resp, err := p.downstream.Complete(ctx, &modifiedReq)
	if err != nil {
		return nil, err
	}

	tokensSaved := (originalLen - distilledLen) / 4
	resp.Savings = append(resp.Savings, llmint.Savings{
		TokensSaved: tokensSaved,
		Technique:   "distill",
	})

	return resp, nil
}

func (p *distillProvider) Name() string               { return p.downstream.Name() }
func (p *distillProvider) Models() []llmint.ModelInfo { return p.downstream.Models() }
