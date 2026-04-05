package account_test

import (
	"context"
	"testing"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/middleware/account"
	"github.com/chitinhq/llmint/provider/mock"
)

func simpleReq() *llmint.Request {
	return &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
	}
}

func TestAccountRecordsUsage(t *testing.T) {
	m := mock.New("test-model", "some response text")
	sink := &account.SliceSink{}
	p := account.New(sink)(m)

	_, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(sink.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sink.Entries))
	}
	e := sink.Entries[0]
	if e.Model != "test-model" {
		t.Errorf("entry Model=%q, want test-model", e.Model)
	}
	if e.Usage.InputTokens == 0 {
		t.Error("expected non-zero InputTokens in entry")
	}
	if e.Duration <= 0 {
		t.Error("expected positive Duration in entry")
	}
	if e.RequestHash == "" {
		t.Error("expected non-empty RequestHash in entry")
	}
}

func TestAccountPassthroughResponse(t *testing.T) {
	m := mock.New("test-model", "passthrough content")
	sink := &account.SliceSink{}
	p := account.New(sink)(m)

	resp, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// Response must be unmodified.
	if len(resp.Content) == 0 || resp.Content[0].Text != "passthrough content" {
		t.Errorf("response content modified: %+v", resp.Content)
	}
	if resp.Model != "test-model" {
		t.Errorf("response Model changed: %q", resp.Model)
	}
}

func TestAccountRecordsMetadata(t *testing.T) {
	m := mock.New("test-model", "hello")
	sink := &account.SliceSink{}
	p := account.New(sink)(m)

	req := simpleReq()
	req.Metadata = map[string]string{"trace_id": "xyz-789", "env": "staging"}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(sink.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sink.Entries))
	}
	e := sink.Entries[0]
	if e.Metadata["trace_id"] != "xyz-789" {
		t.Errorf("metadata trace_id=%q, want xyz-789", e.Metadata["trace_id"])
	}
	if e.Metadata["env"] != "staging" {
		t.Errorf("metadata env=%q, want staging", e.Metadata["env"])
	}
}

// savingsInjector is a middleware that appends a fixed Savings entry to every response.
func savingsInjector(technique string, tokensSaved int) llmint.Middleware {
	return func(next llmint.Provider) llmint.Provider {
		return &injectorProvider{next: next, technique: technique, tokensSaved: tokensSaved}
	}
}

type injectorProvider struct {
	next        llmint.Provider
	technique   string
	tokensSaved int
}

func (p *injectorProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	resp, err := p.next.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	resp.Savings = append(resp.Savings, llmint.Savings{
		TokensSaved: p.tokensSaved,
		Technique:   p.technique,
	})
	return resp, nil
}
func (p *injectorProvider) Name() string               { return p.next.Name() }
func (p *injectorProvider) Models() []llmint.ModelInfo { return p.next.Models() }

func TestAccountRecordsSavings(t *testing.T) {
	m := mock.New("test-model", "hello")
	sink := &account.SliceSink{}

	// Chain: savingsInjector wraps mock, then account wraps the whole thing.
	chain := llmint.Chain(
		account.New(sink),
		savingsInjector("dedup", 100),
	)
	p := chain(m)

	_, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(sink.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sink.Entries))
	}
	e := sink.Entries[0]
	found := false
	for _, s := range e.Savings {
		if s.Technique == "dedup" && s.TokensSaved == 100 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected savings entry with technique=dedup,tokensSaved=100 in entry; got %+v", e.Savings)
	}
}
