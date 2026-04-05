package dedup

import (
	"context"
	"encoding/json"
	"time"

	"github.com/chitinhq/llmint"
)

const defaultTTL = 10 * time.Minute

// options holds configuration for the dedup middleware.
type options struct {
	ttl       time.Duration
	keyPrefix string
}

// Option is a functional option for the dedup middleware.
type Option func(*options)

// WithTTL sets the cache TTL. Default is 10 minutes.
func WithTTL(d time.Duration) Option {
	return func(o *options) { o.ttl = d }
}

// WithKeyPrefix sets a prefix that is prepended to every cache key.
func WithKeyPrefix(prefix string) Option {
	return func(o *options) { o.keyPrefix = prefix }
}

// New returns a deduplication Middleware backed by the given CacheStore.
// Identical requests (same Hash()) return the cached Response on the second
// call, populated with CacheStatus=CacheHit and a Savings entry.
func New(store CacheStore, opts ...Option) llmint.Middleware {
	cfg := options{ttl: defaultTTL}
	for _, o := range opts {
		o(&cfg)
	}

	return func(next llmint.Provider) llmint.Provider {
		return &dedupProvider{next: next, store: store, cfg: cfg}
	}
}

type dedupProvider struct {
	next  llmint.Provider
	store CacheStore
	cfg   options
}

func (p *dedupProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	key := p.cfg.keyPrefix + req.Hash()

	if data, ok := p.store.Get(ctx, key); ok {
		var resp llmint.Response
		if err := json.Unmarshal(data, &resp); err == nil {
			resp.CacheStatus = llmint.CacheHit
			resp.Savings = append(resp.Savings, llmint.Savings{
				TokensSaved: resp.Usage.InputTokens + resp.Usage.OutputTokens,
				Technique:   "dedup",
			})
			return &resp, nil
		}
	}

	resp, err := p.next.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	if data, merr := json.Marshal(resp); merr == nil {
		_ = p.store.Set(ctx, key, data, p.cfg.ttl)
	}

	resp.CacheStatus = llmint.CacheMiss
	return resp, nil
}

func (p *dedupProvider) Name() string    { return p.next.Name() }
func (p *dedupProvider) Models() []llmint.ModelInfo { return p.next.Models() }
