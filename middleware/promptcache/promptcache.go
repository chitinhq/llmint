// Package promptcache provides a middleware that signals provider-side prompt
// caching by annotating requests with cache-control metadata when the system
// prompt prefix meets a minimum token threshold.
package promptcache

import (
	"context"
	"sync"
	"time"

	"github.com/AgentGuardHQ/llmint"
)

const (
	defaultMinPrefixTokens = 100
	charsPerToken          = 4
)

// options holds configuration for the promptcache middleware.
type options struct {
	minPrefixTokens int
	extractor       func(*llmint.Request) string
}

// Option is a functional option for the promptcache middleware.
type Option func(*options)

// WithMinPrefixTokens sets the minimum token count required before cache-control
// metadata is added to the request. Default is 100 tokens (~400 chars).
func WithMinPrefixTokens(n int) Option {
	return func(o *options) { o.minPrefixTokens = n }
}

// WithPrefixExtractor sets a custom function to extract the cacheable prefix
// from a request. When set, the extracted value is also stored in
// req.Metadata["cache_prefix"].
func WithPrefixExtractor(fn func(*llmint.Request) string) Option {
	return func(o *options) { o.extractor = fn }
}

// New returns a Middleware that annotates requests with prompt-cache hints
// when the system prompt prefix is long enough. window controls how long
// seen prefixes are tracked before being evicted.
func New(window time.Duration, opts ...Option) llmint.Middleware {
	cfg := options{minPrefixTokens: defaultMinPrefixTokens}
	for _, o := range opts {
		o(&cfg)
	}

	return func(downstream llmint.Provider) llmint.Provider {
		return &promptCacheProvider{
			downstream: downstream,
			cfg:        cfg,
			window:     window,
			seen:       make(map[string]time.Time),
		}
	}
}

type promptCacheProvider struct {
	downstream llmint.Provider
	cfg        options
	window     time.Duration

	mu   sync.Mutex
	seen map[string]time.Time
}

// extractPrefix returns the cacheable prefix string for a request.
// If a custom extractor is configured it is used; otherwise the system prompt is used.
func (p *promptCacheProvider) extractPrefix(req *llmint.Request) string {
	if p.cfg.extractor != nil {
		return p.cfg.extractor(req)
	}
	return req.System
}

// estimateTokens estimates the number of tokens in s (~4 chars per token).
func estimateTokens(s string) int {
	return len(s) / charsPerToken
}

// cleanExpired removes stale entries from the seen map. Must be called with mu held.
func (p *promptCacheProvider) cleanExpired(now time.Time) {
	for k, ts := range p.seen {
		if now.Sub(ts) > p.window {
			delete(p.seen, k)
		}
	}
}

func (p *promptCacheProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	prefix := p.extractPrefix(req)
	tokens := estimateTokens(prefix)

	if tokens >= p.cfg.minPrefixTokens {
		// Ensure Metadata map exists.
		if req.Metadata == nil {
			req.Metadata = make(map[string]string)
		}
		req.Metadata["cache_control"] = "ephemeral"
		if p.cfg.extractor != nil {
			req.Metadata["cache_prefix"] = prefix
		}

		// Track the prefix so we can reason about cache hits later.
		p.mu.Lock()
		now := time.Now()
		p.cleanExpired(now)
		p.seen[prefix] = now
		p.mu.Unlock()
	}

	resp, err := p.downstream.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	// If the provider reported cache-read tokens, record a savings entry.
	if resp.Usage.CacheReadTokens > 0 {
		resp.Savings = append(resp.Savings, llmint.Savings{
			TokensSaved: resp.Usage.CacheReadTokens,
			Technique:   "prompt-cache",
		})
	}

	return resp, nil
}

func (p *promptCacheProvider) Name() string               { return p.downstream.Name() }
func (p *promptCacheProvider) Models() []llmint.ModelInfo { return p.downstream.Models() }
