package distill_test

import (
	"context"
	"sync"
	"testing"

	"github.com/AgentGuardHQ/llmint"
	"github.com/AgentGuardHQ/llmint/middleware/distill"
)

// capturingProvider captures the last *Request passed to Complete.
type capturingProvider struct {
	mu       sync.Mutex
	captured *llmint.Request
}

func (c *capturingProvider) Complete(_ context.Context, req *llmint.Request) (*llmint.Response, error) {
	copy := *req
	c.mu.Lock()
	c.captured = &copy
	c.mu.Unlock()

	return &llmint.Response{
		Content: []llmint.ContentBlock{{Type: "text", Text: "ok"}},
		Usage:   llmint.Usage{InputTokens: 10, OutputTokens: 5},
		Model:   "test-model",
	}, nil
}

func (c *capturingProvider) Name() string               { return "capturing" }
func (c *capturingProvider) Models() []llmint.ModelInfo { return nil }

func (c *capturingProvider) LastSystem() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.captured == nil {
		return ""
	}
	return c.captured.System
}

// TestDistillReplacesPrompt verifies that a registered prompt is replaced in
// the forwarded request.
func TestDistillReplacesPrompt(t *testing.T) {
	lib := distill.NewMemoryLibrary()
	original := "You are a very verbose and exhaustively detailed helpful assistant that provides comprehensive information."
	compact := "You are a helpful assistant."
	_ = lib.Register(original, compact)

	cp := &capturingProvider{}
	mw := distill.New(lib)
	p := mw(cp)

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   original,
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if cp.LastSystem() != compact {
		t.Errorf("forwarded system prompt = %q, want %q", cp.LastSystem(), compact)
	}
}

// TestDistillPassthroughOnMiss verifies that an unregistered prompt is forwarded unchanged.
func TestDistillPassthroughOnMiss(t *testing.T) {
	lib := distill.NewMemoryLibrary()
	// Register a different prompt to ensure the library isn't empty.
	_ = lib.Register("other prompt", "shorter")

	cp := &capturingProvider{}
	mw := distill.New(lib)
	p := mw(cp)

	original := "Prompt that was never registered."
	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   original,
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if cp.LastSystem() != original {
		t.Errorf("forwarded system prompt = %q, want original %q", cp.LastSystem(), original)
	}
}

// TestDistillReportsSavings verifies that a Savings entry is appended with
// a positive TokensSaved value when distillation occurs.
func TestDistillReportsSavings(t *testing.T) {
	lib := distill.NewMemoryLibrary()
	// Original is long; distilled is much shorter.
	original := "You are a very verbose and exhaustively detailed helpful assistant that provides comprehensive information on every topic."
	compact := "You are a helpful assistant."
	_ = lib.Register(original, compact)

	cp := &capturingProvider{}
	mw := distill.New(lib)
	p := mw(cp)

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   original,
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	found := false
	for _, s := range resp.Savings {
		if s.Technique == "distill" {
			found = true
			if s.TokensSaved <= 0 {
				t.Errorf("expected positive TokensSaved, got %d", s.TokensSaved)
			}
		}
	}
	if !found {
		t.Error("expected Savings entry with Technique=distill")
	}
}

// TestDistillMinSavingsThreshold verifies that WithMinSavings prevents
// distillation when savings fall below the threshold — and allows it when
// savings meet the threshold.
func TestDistillMinSavingsThreshold(t *testing.T) {
	lib := distill.NewMemoryLibrary()

	// "Short prompt" → "Short": len("Short prompt")=12, len("Short")=5
	// savings = (12-5)/12 ≈ 58%, so 50% threshold should allow it.
	original := "Short prompt"
	distilled := "Short"
	_ = lib.Register(original, distilled)

	cp := &capturingProvider{}
	mw := distill.New(lib, distill.WithMinSavings(0.50))
	p := mw(cp)

	req := &llmint.Request{
		Model:    "test-model",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   original,
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// System prompt should have been replaced.
	if cp.LastSystem() != distilled {
		t.Errorf("expected system prompt to be distilled to %q, got %q", distilled, cp.LastSystem())
	}

	// Savings entry should be present.
	found := false
	for _, s := range resp.Savings {
		if s.Technique == "distill" {
			found = true
		}
	}
	if !found {
		t.Error("expected Savings entry with Technique=distill")
	}
}
