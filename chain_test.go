package llmint

import (
	"context"
	"testing"
)

// testProvider is a minimal Provider for use in chain tests.
type testProvider struct {
	name string
}

func (p *testProvider) Complete(_ context.Context, req *Request) (*Response, error) {
	return &Response{
		Content: []ContentBlock{{Type: "text", Text: "base:" + req.System}},
		Model:   p.name,
	}, nil
}

func (p *testProvider) Name() string      { return p.name }
func (p *testProvider) Models() []ModelInfo { return nil }

// trackingMiddleware records the order in which it wraps and calls.
func trackingMiddleware(label string, log *[]string) Middleware {
	return func(next Provider) Provider {
		*log = append(*log, "wrap:"+label)
		return &wrappingProvider{
			label: label,
			next:  next,
			log:   log,
		}
	}
}

type wrappingProvider struct {
	label string
	next  Provider
	log   *[]string
}

func (w *wrappingProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	*w.log = append(*w.log, "call:"+w.label)
	return w.next.Complete(ctx, req)
}

func (w *wrappingProvider) Name() string      { return w.next.Name() }
func (w *wrappingProvider) Models() []ModelInfo { return w.next.Models() }

func TestChainEmpty(t *testing.T) {
	base := &testProvider{name: "base"}
	mw := Chain()
	wrapped := mw(base)

	resp, err := wrapped.Complete(context.Background(), &Request{System: "ping"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "base" {
		t.Errorf("expected model 'base', got %q", resp.Model)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "base:ping" {
		t.Errorf("unexpected content: %v", resp.Content)
	}
}

func TestChainSingle(t *testing.T) {
	var log []string
	base := &testProvider{name: "base"}
	mw := Chain(trackingMiddleware("A", &log))
	wrapped := mw(base)

	_, err := wrapped.Complete(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "wrap:A" recorded at chain-build time, "call:A" at invocation time
	if len(log) != 2 || log[0] != "wrap:A" || log[1] != "call:A" {
		t.Errorf("unexpected log: %v", log)
	}
}

func TestChainOrder(t *testing.T) {
	// Chain(A, B, C) → A wraps B wraps C wraps base
	// Call order should be: A → B → C → base
	var log []string
	base := &testProvider{name: "base"}
	mw := Chain(
		trackingMiddleware("A", &log),
		trackingMiddleware("B", &log),
		trackingMiddleware("C", &log),
	)
	wrapped := mw(base)

	// Clear the wrap-time entries so we only observe call order
	log = log[:0]

	_, err := wrapped.Complete(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"call:A", "call:B", "call:C"}
	if len(log) != len(want) {
		t.Fatalf("expected %d log entries, got %d: %v", len(want), len(log), log)
	}
	for i, entry := range want {
		if log[i] != entry {
			t.Errorf("log[%d] = %q, want %q", i, log[i], entry)
		}
	}
}
