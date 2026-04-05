package promptcache_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/middleware/promptcache"
)

// capturingProvider records a copy of every *Request passed to Complete.
type capturingProvider struct {
	mu       sync.Mutex
	captured []*llmint.Request
	response *llmint.Response
}

func newCapturing(cacheReadTokens int) *capturingProvider {
	return &capturingProvider{
		response: &llmint.Response{
			Content: []llmint.ContentBlock{{Type: "text", Text: "ok"}},
			Usage: llmint.Usage{
				InputTokens:     10,
				OutputTokens:    5,
				CacheReadTokens: cacheReadTokens,
			},
			Model: "test-model",
		},
	}
}

func (c *capturingProvider) Complete(_ context.Context, req *llmint.Request) (*llmint.Response, error) {
	// Capture a shallow copy so we preserve the metadata at call time.
	copy := *req
	metaCopy := make(map[string]string, len(req.Metadata))
	for k, v := range req.Metadata {
		metaCopy[k] = v
	}
	copy.Metadata = metaCopy

	c.mu.Lock()
	c.captured = append(c.captured, &copy)
	c.mu.Unlock()

	return c.response, nil
}

func (c *capturingProvider) Name() string               { return "capturing" }
func (c *capturingProvider) Models() []llmint.ModelInfo { return nil }

func (c *capturingProvider) Last() *llmint.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.captured) == 0 {
		return nil
	}
	return c.captured[len(c.captured)-1]
}

func (c *capturingProvider) All() []*llmint.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*llmint.Request, len(c.captured))
	copy(out, c.captured)
	return out
}

// buildLongSystem returns a system prompt of at least minChars characters.
func buildLongSystem(minChars int) string {
	repeat := minChars/len("You are a helpful assistant. ") + 2
	return strings.Repeat("You are a helpful assistant. ", repeat)
}

// TestPromptCacheStabilizesPrefix verifies that a system prompt >= 100 tokens
// causes cache_control="ephemeral" to be set on the forwarded request.
func TestPromptCacheStabilizesPrefix(t *testing.T) {
	cp := newCapturing(0)
	mw := promptcache.New(5 * time.Minute)
	p := mw(cp)

	// 100 tokens = 400 chars minimum; use 500 to be safe.
	sys := buildLongSystem(500)

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   sys,
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	captured := cp.Last()
	if captured == nil {
		t.Fatal("no request captured")
	}
	if captured.Metadata["cache_control"] != "ephemeral" {
		t.Errorf("expected cache_control=ephemeral, got %q", captured.Metadata["cache_control"])
	}

	// Second request with same system prompt — should also get cache_control set.
	req2 := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "world"}},
		System:   sys,
	}
	_, err = p.Complete(context.Background(), req2)
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	all := cp.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 captured requests, got %d", len(all))
	}
	if all[1].Metadata["cache_control"] != "ephemeral" {
		t.Errorf("second request: expected cache_control=ephemeral, got %q", all[1].Metadata["cache_control"])
	}
}

// TestPromptCacheSkipsShortPrefix verifies that a short system prompt does NOT
// get cache-control metadata when WithMinPrefixTokens sets a high threshold.
func TestPromptCacheSkipsShortPrefix(t *testing.T) {
	cp := newCapturing(0)
	mw := promptcache.New(5*time.Minute, promptcache.WithMinPrefixTokens(1000))
	p := mw(cp)

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   "Short system prompt.",
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	captured := cp.Last()
	if captured == nil {
		t.Fatal("no request captured")
	}
	if v, ok := captured.Metadata["cache_control"]; ok {
		t.Errorf("expected no cache_control metadata for short prompt, got %q", v)
	}
}

// TestPromptCacheCustomExtractor verifies that a custom prefix extractor results
// in cache_prefix being set in metadata.
func TestPromptCacheCustomExtractor(t *testing.T) {
	cp := newCapturing(0)

	extractor := func(req *llmint.Request) string {
		// Extract the first message content as the cache prefix.
		if len(req.Messages) > 0 {
			return req.Messages[0].Content
		}
		return ""
	}

	// Build a long first message so it exceeds 100 tokens.
	longMsg := buildLongSystem(500)

	mw := promptcache.New(5*time.Minute, promptcache.WithPrefixExtractor(extractor))
	p := mw(cp)

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: longMsg}},
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	captured := cp.Last()
	if captured == nil {
		t.Fatal("no request captured")
	}
	if captured.Metadata["cache_control"] != "ephemeral" {
		t.Errorf("expected cache_control=ephemeral, got %q", captured.Metadata["cache_control"])
	}
	if captured.Metadata["cache_prefix"] == "" {
		t.Error("expected cache_prefix to be set when custom extractor is used")
	}
}

// TestPromptCacheReportsSavings verifies that when CacheReadTokens > 0 in the
// response, a Savings entry with technique "prompt-cache" is appended.
func TestPromptCacheReportsSavings(t *testing.T) {
	cp := newCapturing(50) // provider returns 50 cache-read tokens
	mw := promptcache.New(5 * time.Minute)
	p := mw(cp)

	sys := buildLongSystem(500)
	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   sys,
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	found := false
	for _, s := range resp.Savings {
		if s.Technique == "prompt-cache" {
			found = true
			if s.TokensSaved != 50 {
				t.Errorf("expected TokensSaved=50, got %d", s.TokensSaved)
			}
		}
	}
	if !found {
		t.Error("expected Savings entry with Technique=prompt-cache")
	}
}
