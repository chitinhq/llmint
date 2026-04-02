package anthropic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AgentGuardHQ/llmint"
	"github.com/AgentGuardHQ/llmint/provider/anthropic"
)

func TestComputeCostHaiku(t *testing.T) {
	// 1000 input tokens at $0.80/MTok + 500 output tokens at $4.00/MTok
	// = 0.000800 + 0.002000 = $0.002800
	usage := llmint.Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	}
	cost := anthropic.ComputeCost("claude-3-haiku-20240307", usage)
	expected := 0.0028
	if diff := cost - expected; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("ComputeCostHaiku: expected %.6f, got %.6f", expected, cost)
	}
}

func TestComputeCostWithCache(t *testing.T) {
	// Sonnet: 500 input @ $3/MTok + 100 output @ $15/MTok
	//       + 200 cache-read @ $0.30/MTok + 50 cache-write @ $3.75/MTok
	// = 0.0015 + 0.0015 + 0.00006 + 0.0001875 = 0.0032375
	usage := llmint.Usage{
		InputTokens:      500,
		OutputTokens:     100,
		CacheReadTokens:  200,
		CacheWriteTokens: 50,
	}
	cost := anthropic.ComputeCost("claude-3-5-sonnet-20241022", usage)
	expected := 0.0032375
	if diff := cost - expected; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("ComputeCostWithCache: expected %.7f, got %.7f", expected, cost)
	}
}

func TestAnthropicCompleteWithMockServer(t *testing.T) {
	// Build a mock response that mimics the Anthropic Messages API.
	mockResp := map[string]interface{}{
		"id":   "msg_01test",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-haiku-20240307",
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Hello from mock!",
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":               float64(10),
			"output_tokens":              float64(5),
			"cache_read_input_tokens":    float64(0),
			"cache_creation_input_tokens": float64(0),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate required headers.
		if r.Header.Get("x-api-key") != "test-key" {
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			http.Error(w, "bad version", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad content type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := &llmint.Request{
		Model:    "claude-3-haiku-20240307",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(resp.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if resp.Content[0].Text != "Hello from mock!" {
		t.Errorf("expected text %q, got %q", "Hello from mock!", resp.Content[0].Text)
	}
	if resp.Model != "claude-3-haiku-20240307" {
		t.Errorf("expected model %q, got %q", "claude-3-haiku-20240307", resp.Model)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestAnthropicName(t *testing.T) {
	p := anthropic.New("key")
	if p.Name() != "anthropic" {
		t.Errorf("expected name %q, got %q", "anthropic", p.Name())
	}
}

func TestAnthropicModels(t *testing.T) {
	p := anthropic.New("key")
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
	// Verify haiku, sonnet, and opus are all present.
	found := make(map[string]bool)
	for _, m := range models {
		found[m.ID] = true
	}
	for _, expected := range []string{
		"claude-3-haiku-20240307",
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
	} {
		if !found[expected] {
			t.Errorf("expected model %q not found in Models()", expected)
		}
	}
}

func TestAnthropicErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		errResp := map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		}
		_ = json.NewEncoder(w).Encode(errResp)
	}))
	defer srv.Close()

	p := anthropic.New("bad-key", anthropic.WithBaseURL(srv.URL))
	req := &llmint.Request{
		Model:    "claude-3-haiku-20240307",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}
