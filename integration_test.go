package llmint_test

import (
	"context"
	"testing"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/middleware/account"
	"github.com/chitinhq/llmint/middleware/cascade"
	"github.com/chitinhq/llmint/middleware/dedup"
	"github.com/chitinhq/llmint/middleware/distill"
	"github.com/chitinhq/llmint/provider/mock"
)

// TestFullPipeline verifies the happy path: dedup → distill → cascade → account.
//
// On the first request:
//   - haiku has high confidence (0.95) so cascade stays on haiku
//   - distill replaces the verbose system prompt with a short one
//   - account records the entry
//   - dedup caches the result
//
// On the second identical request:
//   - dedup returns the cached response (CacheHit)
//   - haiku is called only once total, sonnet is never called
//   - account has 2 entries, with savings recorded
func TestFullPipeline(t *testing.T) {
	verbosePrompt := "You are a helpful assistant. Please answer all questions thoroughly, " +
		"with great care and detail, providing multiple examples where appropriate, " +
		"citing sources, and explaining your reasoning step by step in a comprehensive way."
	shortPrompt := "You are a helpful assistant. Be concise."

	// Create mock providers: haiku responds with high confidence.
	haiku := mock.New("haiku", "Answer [confidence: 0.95]")
	sonnet := mock.New("sonnet", "Sonnet answer [confidence: 0.90]")

	// Build distill library.
	lib := distill.NewMemoryLibrary()
	if err := lib.Register(verbosePrompt, shortPrompt); err != nil {
		t.Fatalf("distill.Register: %v", err)
	}

	// Build accountant sink.
	sink := &account.SliceSink{}

	// Assemble the pipeline: account → dedup → distill → cascade
	// Chain applies outermost-first, so the call path is:
	//   account → dedup → distill → cascade(haiku/sonnet)
	// account is outermost so it records every call (including cache hits).
	// dedup is next so it can short-circuit the cascade on repeated requests.
	// distill rewrites the system prompt before reaching cascade.
	// cascade ignores its downstream and routes among its own model list.
	store := dedup.NewMemoryStore()
	pipeline := llmint.Chain(
		account.New(sink),
		dedup.New(store),
		distill.New(lib),
		cascade.New([]cascade.Model{
			{Provider: haiku, Name: "haiku", Threshold: 0.8},
			{Provider: sonnet, Name: "sonnet", Threshold: 0},
		}),
	)(nil)

	req := &llmint.Request{
		Model:  "haiku",
		System: verbosePrompt,
		Messages: []llmint.Message{
			{Role: "user", Content: "What is 2+2?"},
		},
	}
	ctx := context.Background()

	// --- First call ---
	resp1, err := pipeline.Complete(ctx, req)
	if err != nil {
		t.Fatalf("first Complete: %v", err)
	}
	if resp1.Model != "haiku" {
		t.Errorf("first call: expected model=haiku, got %q", resp1.Model)
	}
	if haiku.CallCount() != 1 {
		t.Errorf("first call: haiku call count = %d, want 1", haiku.CallCount())
	}
	if sonnet.CallCount() != 0 {
		t.Errorf("first call: sonnet should not be called, got %d", sonnet.CallCount())
	}

	// --- Second identical call ---
	resp2, err := pipeline.Complete(ctx, req)
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if resp2.CacheStatus != llmint.CacheHit {
		t.Errorf("second call: expected CacheHit, got %s", resp2.CacheStatus)
	}
	// haiku should still only have been called once (dedup short-circuits).
	if haiku.CallCount() != 1 {
		t.Errorf("second call: haiku call count = %d, want 1 (dedup should cache)", haiku.CallCount())
	}
	if sonnet.CallCount() != 0 {
		t.Errorf("second call: sonnet should not be called, got %d", sonnet.CallCount())
	}

	// Accountant should have 2 entries (one per call, even the dedup hit).
	if len(sink.Entries) != 2 {
		t.Errorf("expected 2 accounting entries, got %d", len(sink.Entries))
	}

	// Verify savings are recorded (dedup savings on second call).
	total := llmint.TotalSavings(resp2.Savings)
	if total.TokensSaved <= 0 {
		t.Errorf("expected savings > 0 on second (cached) call, got %d tokens saved", total.TokensSaved)
	}
}

// TestFullPipelineWithEscalation verifies that a low-confidence haiku response
// causes cascade to escalate to sonnet, and that both providers are called once.
func TestFullPipelineWithEscalation(t *testing.T) {
	// haiku responds with low confidence → cascade escalates to sonnet.
	haiku := mock.New("haiku", "I'm not sure [confidence: 0.2]")
	sonnet := mock.New("sonnet", "Definitive answer [confidence: 0.95]")

	sink := &account.SliceSink{}

	// Pipeline: account → cascade (no dedup or distill for this test).
	// account is outermost so it records the final response from cascade.
	pipeline := llmint.Chain(
		account.New(sink),
		cascade.New([]cascade.Model{
			{Provider: haiku, Name: "haiku", Threshold: 0.8},
			{Provider: sonnet, Name: "sonnet", Threshold: 0},
		}),
	)(nil)

	req := &llmint.Request{
		Model:    "haiku",
		Messages: []llmint.Message{{Role: "user", Content: "Complex question"}},
	}

	resp, err := pipeline.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Should have escalated to sonnet.
	if resp.Model != "sonnet" {
		t.Errorf("expected escalation to sonnet, got model=%q", resp.Model)
	}
	if haiku.CallCount() != 1 {
		t.Errorf("haiku call count = %d, want 1", haiku.CallCount())
	}
	if sonnet.CallCount() != 1 {
		t.Errorf("sonnet call count = %d, want 1", sonnet.CallCount())
	}

	// Account should have one entry for the winning model (sonnet).
	if len(sink.Entries) != 1 {
		t.Errorf("expected 1 accounting entry, got %d", len(sink.Entries))
	}
	if sink.Entries[0].Model != "sonnet" {
		t.Errorf("accounting entry model = %q, want sonnet", sink.Entries[0].Model)
	}
}
