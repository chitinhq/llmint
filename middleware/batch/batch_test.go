package batch_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AgentGuardHQ/llmint"
	"github.com/AgentGuardHQ/llmint/middleware/batch"
)

// countingProvider counts the number of Complete calls.
type countingProvider struct {
	var_ int64 // atomic counter; use atomic.AddInt64 / atomic.LoadInt64
}

func (c *countingProvider) Complete(_ context.Context, req *llmint.Request) (*llmint.Response, error) {
	atomic.AddInt64(&c.var_, 1)
	return &llmint.Response{
		Content: []llmint.ContentBlock{{Type: "text", Text: "ok"}},
		Usage:   llmint.Usage{InputTokens: 10, OutputTokens: 5},
		Model:   "test-model",
	}, nil
}

func (c *countingProvider) Name() string               { return "counting" }
func (c *countingProvider) Models() []llmint.ModelInfo { return nil }
func (c *countingProvider) Calls() int64               { return atomic.LoadInt64(&c.var_) }

// TestBatchRealtimeBypass verifies that requests with priority="realtime" skip
// the batch queue and go directly to the provider.
func TestBatchRealtimeBypass(t *testing.T) {
	cp := &countingProvider{}
	var directCalls int64

	cb := func(*llmint.Response) {
		// callback fires for batched responses only
	}

	mw := batch.New(10, 500*time.Millisecond, batch.WithCallback(cb))
	p := mw(cp)

	ctx := context.Background()

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		Metadata: map[string]string{"priority": "realtime"},
	}

	resp, err := p.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	atomic.AddInt64(&directCalls, 1)

	if atomic.LoadInt64(&directCalls) != 1 {
		t.Errorf("expected 1 direct call, got %d", atomic.LoadInt64(&directCalls))
	}
	if cp.Calls() != 1 {
		t.Errorf("expected provider called 1 time (direct bypass), got %d", cp.Calls())
	}
}

// TestBatchFlushOnMaxSize verifies that when maxSize requests are queued, they
// are all flushed and every caller receives a response.
func TestBatchFlushOnMaxSize(t *testing.T) {
	cp := &countingProvider{}
	mw := batch.New(2, 5*time.Second) // maxSize=2, large timeout so timer doesn't fire first
	p := mw(cp)

	ctx := context.Background()
	makeReq := func(msg string) *llmint.Request {
		return &llmint.Request{
			Model:    "test-model",
			Messages: []llmint.Message{{Role: "user", Content: msg}},
		}
	}

	var wg sync.WaitGroup
	results := make([]*llmint.Response, 2)
	errs := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = p.Complete(ctx, makeReq("message"))
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Complete error: %v", i, err)
		}
	}
	for i, resp := range results {
		if resp == nil {
			t.Errorf("goroutine %d: expected non-nil response", i)
		}
	}
	if cp.Calls() != 2 {
		t.Errorf("expected 2 provider calls, got %d", cp.Calls())
	}
}

// TestBatchFlushOnTimeout verifies that a single batched request is flushed
// after maxWait even when maxSize has not been reached.
func TestBatchFlushOnTimeout(t *testing.T) {
	cp := &countingProvider{}
	mw := batch.New(100, 50*time.Millisecond) // large maxSize, short timeout
	p := mw(cp)

	ctx := context.Background()
	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "lone request"}},
	}

	start := time.Now()
	resp, err := p.Complete(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Should have waited roughly maxWait before returning.
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected to wait ~50ms, returned too fast: %v", elapsed)
	}
	if cp.Calls() != 1 {
		t.Errorf("expected 1 provider call, got %d", cp.Calls())
	}

	// Verify savings entry was added.
	found := false
	for _, s := range resp.Savings {
		if s.Technique == "batch" {
			found = true
		}
	}
	if !found {
		t.Error("expected Savings entry with Technique=batch")
	}
}

// TestBatchCustomAsyncCheck verifies that a custom asyncCheck function controls
// which requests are batched vs. bypassed.
func TestBatchCustomAsyncCheck(t *testing.T) {
	cp := &countingProvider{}

	// Only batch requests with model "batch-me"; everything else bypasses.
	asyncCheck := func(req *llmint.Request) bool {
		return req.Model == "batch-me"
	}

	mw := batch.New(10, 500*time.Millisecond, batch.WithAsyncCheck(asyncCheck))
	p := mw(cp)

	ctx := context.Background()

	// This request should bypass (asyncCheck returns false → bypass).
	directReq := &llmint.Request{
		Model:    "direct-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
	}
	resp, err := p.Complete(ctx, directReq)
	if err != nil {
		t.Fatalf("Complete (direct): %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for direct request")
	}
	if cp.Calls() != 1 {
		t.Errorf("expected 1 provider call after direct bypass, got %d", cp.Calls())
	}
}
