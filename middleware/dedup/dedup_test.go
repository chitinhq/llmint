package dedup_test

import (
	"context"
	"testing"
	"time"

	"github.com/AgentGuardHQ/llmint"
	"github.com/AgentGuardHQ/llmint/middleware/dedup"
	"github.com/AgentGuardHQ/llmint/provider/mock"
)

func makeReq(content string) *llmint.Request {
	return &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: content}},
	}
}

func TestDedupCachesIdenticalRequests(t *testing.T) {
	m := mock.New("test-model", "hello world")
	store := dedup.NewMemoryStore()
	p := dedup.New(store)(m)
	ctx := context.Background()
	req := makeReq("tell me something")

	// First call — should be a miss, provider called once.
	resp1, err := p.Complete(ctx, req)
	if err != nil {
		t.Fatalf("first Complete: %v", err)
	}
	if resp1.CacheStatus != llmint.CacheMiss {
		t.Errorf("expected CacheMiss on first call, got %v", resp1.CacheStatus)
	}
	if m.CallCount() != 1 {
		t.Errorf("expected provider called 1 time, got %d", m.CallCount())
	}

	// Second call — should be a hit, provider NOT called again.
	resp2, err := p.Complete(ctx, req)
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if resp2.CacheStatus != llmint.CacheHit {
		t.Errorf("expected CacheHit on second call, got %v", resp2.CacheStatus)
	}
	if m.CallCount() != 1 {
		t.Errorf("expected provider still called 1 time, got %d", m.CallCount())
	}
	// Savings should be reported.
	found := false
	for _, s := range resp2.Savings {
		if s.Technique == "dedup" {
			found = true
		}
	}
	if !found {
		t.Error("expected Savings entry with Technique=dedup")
	}
}

func TestDedupDifferentRequestsNotCached(t *testing.T) {
	m := mock.New("test-model", "response")
	store := dedup.NewMemoryStore()
	p := dedup.New(store)(m)
	ctx := context.Background()

	_, _ = p.Complete(ctx, makeReq("question one"))
	_, _ = p.Complete(ctx, makeReq("question two"))

	if m.CallCount() != 2 {
		t.Errorf("expected 2 provider calls for different requests, got %d", m.CallCount())
	}
}

func TestDedupWithTTL(t *testing.T) {
	m := mock.New("test-model", "cached response")
	store := dedup.NewMemoryStore()
	p := dedup.New(store, dedup.WithTTL(1*time.Millisecond))(m)
	ctx := context.Background()
	req := makeReq("short-lived")

	_, _ = p.Complete(ctx, req)
	if m.CallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", m.CallCount())
	}

	// Wait for TTL to expire.
	time.Sleep(5 * time.Millisecond)

	resp, err := p.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete after expiry: %v", err)
	}
	// After TTL expiry, should be a miss and provider called again.
	if resp.CacheStatus != llmint.CacheMiss {
		t.Errorf("expected CacheMiss after TTL expiry, got %v", resp.CacheStatus)
	}
	if m.CallCount() != 2 {
		t.Errorf("expected 2 provider calls after TTL expiry, got %d", m.CallCount())
	}
}

func TestDedupWithKeyPrefix(t *testing.T) {
	store := dedup.NewMemoryStore()
	ctx := context.Background()
	req := makeReq("same content")

	// Two providers sharing the same store but different prefixes.
	m1 := mock.New("model-a", "response-a")
	m2 := mock.New("model-b", "response-b")
	p1 := dedup.New(store, dedup.WithKeyPrefix("a:"))(m1)
	p2 := dedup.New(store, dedup.WithKeyPrefix("b:"))(m2)

	_, _ = p1.Complete(ctx, req)
	// p2 should NOT hit p1's cached entry because it has a different prefix.
	resp, err := p2.Complete(ctx, req)
	if err != nil {
		t.Fatalf("p2 Complete: %v", err)
	}
	if resp.CacheStatus != llmint.CacheMiss {
		t.Errorf("expected CacheMiss for different prefix, got %v", resp.CacheStatus)
	}
	if m1.CallCount() != 1 || m2.CallCount() != 1 {
		t.Errorf("expected each provider called once; m1=%d m2=%d", m1.CallCount(), m2.CallCount())
	}
}
