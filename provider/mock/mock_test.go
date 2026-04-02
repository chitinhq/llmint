package mock_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/AgentGuardHQ/llmint"
	"github.com/AgentGuardHQ/llmint/provider/mock"
)

func TestMockName(t *testing.T) {
	p := mock.New("claude-3-5-sonnet-20241022", "hello")
	if p.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", p.Name())
	}
}

func TestMockModels(t *testing.T) {
	p := mock.New("claude-3-5-sonnet-20241022", "hello")
	models := p.Models()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ID != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model ID 'claude-3-5-sonnet-20241022', got %q", models[0].ID)
	}
}

func TestMockComplete(t *testing.T) {
	const responseText = "Hello from mock!"
	p := mock.New("claude-3-5-sonnet-20241022", responseText)

	req := &llmint.Request{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []llmint.Message{
			{Role: "user", Content: "Say something"},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if resp.Content[0].Text != responseText {
		t.Errorf("expected %q, got %q", responseText, resp.Content[0].Text)
	}

	// Token estimation: ~4 chars/token, min 10
	// "Say something" = 13 chars → 13/4 = 3 → max(3, 10) = 10
	if resp.Usage.InputTokens < 10 {
		t.Errorf("expected input tokens >= 10, got %d", resp.Usage.InputTokens)
	}
	// Output tokens: len(responseText)/4, min 10
	// "Hello from mock!" = 16 chars → 4 → max(4, 10) = 10
	if resp.Usage.OutputTokens < 10 {
		t.Errorf("expected output tokens >= 10, got %d", resp.Usage.OutputTokens)
	}
}

func TestMockCallCount(t *testing.T) {
	p := mock.New("claude-3-5-sonnet-20241022", "ok")
	req := &llmint.Request{Model: "claude-3-5-sonnet-20241022"}

	for i := 0; i < 3; i++ {
		_, _ = p.Complete(context.Background(), req)
	}

	if p.CallCount() != 3 {
		t.Errorf("expected 3 calls, got %d", p.CallCount())
	}
}

func TestMockWithError(t *testing.T) {
	sentinel := errors.New("mock: rate limit exceeded")
	p := mock.NewWithError("claude-3-5-sonnet-20241022", sentinel)

	req := &llmint.Request{Model: "claude-3-5-sonnet-20241022"}
	_, err := p.Complete(context.Background(), req)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}

	// CallCount should still increment even on error
	if p.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", p.CallCount())
	}

	// Suppress "fmt imported and not used" — use it for a sanity print
	_ = fmt.Sprintf("error was: %v", err)
}
