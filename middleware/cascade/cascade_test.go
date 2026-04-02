package cascade_test

import (
	"context"
	"testing"

	"github.com/AgentGuardHQ/llmint"
	"github.com/AgentGuardHQ/llmint/middleware/cascade"
	"github.com/AgentGuardHQ/llmint/provider/mock"
)

func simpleReq() *llmint.Request {
	return &llmint.Request{
		Model:    "any",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
	}
}

func TestCascadeStaysOnCheapModel(t *testing.T) {
	haiku := mock.New("haiku", "High confidence answer [confidence: 0.95]")
	sonnet := mock.New("sonnet", "Sonnet answer [confidence: 0.90]")

	models := []cascade.Model{
		{Provider: haiku, Name: "haiku", Threshold: 0.8},
		{Provider: sonnet, Name: "sonnet", Threshold: 0},
	}
	p := cascade.New(models)(nil)

	resp, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Model != "haiku" {
		t.Errorf("expected model=haiku, got %q", resp.Model)
	}
	if haiku.CallCount() != 1 {
		t.Errorf("haiku called %d times, expected 1", haiku.CallCount())
	}
	if sonnet.CallCount() != 0 {
		t.Errorf("sonnet should not have been called, got %d calls", sonnet.CallCount())
	}
}

func TestCascadeEscalates(t *testing.T) {
	haiku := mock.New("haiku", "Low confidence answer [confidence: 0.3]")
	sonnet := mock.New("sonnet", "Sonnet definitive answer [confidence: 0.90]")

	models := []cascade.Model{
		{Provider: haiku, Name: "haiku", Threshold: 0.8},
		{Provider: sonnet, Name: "sonnet", Threshold: 0},
	}
	p := cascade.New(models)(nil)

	resp, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Model != "sonnet" {
		t.Errorf("expected escalation to sonnet, got %q", resp.Model)
	}
	if haiku.CallCount() != 1 {
		t.Errorf("haiku called %d times, expected 1", haiku.CallCount())
	}
	if sonnet.CallCount() != 1 {
		t.Errorf("sonnet called %d times, expected 1", sonnet.CallCount())
	}
}

func TestCascadeMaxEscalations(t *testing.T) {
	haiku := mock.New("haiku", "Low [confidence: 0.1]")
	sonnet := mock.New("sonnet", "Still low [confidence: 0.2]")
	opus := mock.New("opus", "Best answer [confidence: 0.99]")

	models := []cascade.Model{
		{Provider: haiku, Name: "haiku", Threshold: 0.8},
		{Provider: sonnet, Name: "sonnet", Threshold: 0.8},
		{Provider: opus, Name: "opus", Threshold: 0},
	}
	// Allow at most 1 escalation — should stop at sonnet.
	p := cascade.New(models, cascade.WithMaxEscalations(1))(nil)

	resp, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Model != "sonnet" {
		t.Errorf("expected model=sonnet (capped at 1 escalation), got %q", resp.Model)
	}
	if opus.CallCount() != 0 {
		t.Errorf("opus should not have been called, got %d calls", opus.CallCount())
	}
}

func TestCascadeCustomScorer(t *testing.T) {
	// Scorer that only accepts tool_use responses.
	toolUseScorer := cascade.Scorer(func(resp *llmint.Response) float64 {
		for _, b := range resp.Content {
			if b.Type == "tool_use" {
				return 1.0
			}
		}
		return 0.0
	})

	// haiku returns plain text — scorer returns 0.0, below threshold 0.5 → escalate.
	haiku := mock.New("haiku", "plain text no tool")
	sonnet := mock.New("sonnet", "sonnet also plain text")

	models := []cascade.Model{
		{Provider: haiku, Name: "haiku", Threshold: 0.5},
		{Provider: sonnet, Name: "sonnet", Threshold: 0},
	}
	p := cascade.New(models, cascade.WithScorer(toolUseScorer))(nil)

	resp, err := p.Complete(context.Background(), simpleReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Model != "sonnet" {
		t.Errorf("expected escalation to sonnet via custom scorer, got %q", resp.Model)
	}
}
