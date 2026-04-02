// Package batch provides a middleware that queues requests and flushes them
// when either a size threshold or a time window is reached. This reduces
// per-request overhead and enables future integration with bulk Batches APIs.
package batch

import (
	"context"
	"time"

	"github.com/AgentGuardHQ/llmint"
)

// options holds configuration for the batch middleware.
type options struct {
	asyncCheck func(*llmint.Request) bool
	callback   func(*llmint.Response)
}

// Option is a functional option for the batch middleware.
type Option func(*options)

// WithAsyncCheck sets a function that determines whether a request should be
// batched. If it returns false, the request bypasses the queue and goes
// directly to the provider. The default behaviour bypasses requests with
// Metadata["priority"] == "realtime".
func WithAsyncCheck(fn func(*llmint.Request) bool) Option {
	return func(o *options) { o.asyncCheck = fn }
}

// WithCallback registers a function that is called for each response after a
// batch flush.
func WithCallback(fn func(*llmint.Response)) Option {
	return func(o *options) { o.callback = fn }
}

// pendingItem holds a queued request together with the channel used to
// return the result to the blocked caller.
type pendingItem struct {
	ctx    context.Context
	req    *llmint.Request
	result chan result
}

type result struct {
	resp *llmint.Response
	err  error
}

// New returns a Middleware that batches requests until maxSize is reached or
// maxWait has elapsed since the first item in the current batch was enqueued.
func New(maxSize int, maxWait time.Duration, opts ...Option) llmint.Middleware {
	cfg := options{}
	for _, o := range opts {
		o(&cfg)
	}

	return func(downstream llmint.Provider) llmint.Provider {
		bp := &batchProvider{
			downstream: downstream,
			cfg:        cfg,
			maxSize:    maxSize,
			maxWait:    maxWait,
			enqueue:    make(chan pendingItem),
			quit:       make(chan struct{}),
		}
		go bp.run()
		return bp
	}
}

type batchProvider struct {
	downstream llmint.Provider
	cfg        options
	maxSize    int
	maxWait    time.Duration

	enqueue chan pendingItem
	quit    chan struct{}
}

// shouldBypass returns true when the request should skip the batch queue and
// go directly to the provider. The default check bypasses "realtime" priority
// requests; a custom asyncCheck (when provided) overrides this entirely.
func (p *batchProvider) shouldBypass(req *llmint.Request) bool {
	if p.cfg.asyncCheck != nil {
		// Custom check: bypass when asyncCheck returns false.
		return !p.cfg.asyncCheck(req)
	}
	// Default: bypass realtime-priority requests.
	return req.Metadata != nil && req.Metadata["priority"] == "realtime"
}

// run is the background goroutine that manages the pending queue and flushing.
func (p *batchProvider) run() {
	var (
		pending []pendingItem
		timer   *time.Timer
		timerC  <-chan time.Time
	)

	flush := func() {
		if timer != nil {
			timer.Stop()
			timer = nil
			timerC = nil
		}
		items := pending
		pending = nil

		// Process each item individually through the downstream provider.
		for _, item := range items {
			resp, err := p.downstream.Complete(item.ctx, item.req)
			if err == nil {
				resp.Savings = append(resp.Savings, llmint.Savings{
					Technique: "batch",
				})
				if p.cfg.callback != nil {
					p.cfg.callback(resp)
				}
			}
			item.result <- result{resp: resp, err: err}
			close(item.result)
		}
	}

	for {
		select {
		case item := <-p.enqueue:
			pending = append(pending, item)
			if len(pending) == 1 {
				// Start the timer on the first item in a new batch.
				timer = time.NewTimer(p.maxWait)
				timerC = timer.C
			}
			if len(pending) >= p.maxSize {
				flush()
			}
		case <-timerC:
			if len(pending) > 0 {
				flush()
			}
		case <-p.quit:
			// Drain any remaining items with a context-cancelled error.
			for _, item := range pending {
				item.result <- result{err: context.Canceled}
				close(item.result)
			}
			return
		}
	}
}

func (p *batchProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	if p.shouldBypass(req) {
		return p.downstream.Complete(ctx, req)
	}

	ch := make(chan result, 1)
	p.enqueue <- pendingItem{ctx: ctx, req: req, result: ch}
	r := <-ch
	return r.resp, r.err
}

func (p *batchProvider) Name() string               { return p.downstream.Name() }
func (p *batchProvider) Models() []llmint.ModelInfo { return p.downstream.Models() }
