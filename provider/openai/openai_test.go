package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/provider/openai"
)

func TestComputeCostDeepSeek(t *testing.T) {
	// deepseek-chat: 1000 input @ $0.14/MTok + 500 output @ $0.28/MTok
	// = 0.00014 + 0.00014 = $0.00028
	usage := llmint.Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	}
	cost := openai.ComputeCost("deepseek-chat", usage, openai.DeepSeekModels)
	expected := 0.00028
	if diff := cost - expected; diff < -0.000001 || diff > 0.000001 {
		t.Errorf("ComputeCostDeepSeek: expected %.6f, got %.6f", expected, cost)
	}
}

func TestComputeCostUnknownModel(t *testing.T) {
	usage := llmint.Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	}
	cost := openai.ComputeCost("unknown-model-xyz", usage, openai.DeepSeekModels)
	if cost != 0 {
		t.Errorf("ComputeCostUnknownModel: expected 0, got %.6f", cost)
	}
}

func TestCompleteWithMockServer(t *testing.T) {
	mockResp := map[string]interface{}{
		"id":     "chatcmpl-test123",
		"object": "chat.completion",
		"model":  "deepseek-chat",
		"choices": []interface{}{
			map[string]interface{}{
				"index": float64(0),
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello from DeepSeek mock!",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(10),
			"completion_tokens": float64(7),
			"total_tokens":      float64(17),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate Authorization header.
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "missing or invalid authorization", http.StatusUnauthorized)
			return
		}
		// Validate path.
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
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

	p := openai.New("test-key",
		openai.WithBaseURL(srv.URL),
		openai.WithModel("deepseek-chat"),
		openai.WithPricing(openai.DeepSeekModels),
	)

	req := &llmint.Request{
		Model:    "deepseek-chat",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
		System:   "You are a helpful assistant.",
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(resp.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if resp.Content[0].Text != "Hello from DeepSeek mock!" {
		t.Errorf("expected text %q, got %q", "Hello from DeepSeek mock!", resp.Content[0].Text)
	}
	if resp.Model != "deepseek-chat" {
		t.Errorf("expected model %q, got %q", "deepseek-chat", resp.Model)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 7 {
		t.Errorf("expected 7 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestName(t *testing.T) {
	p := openai.New("key")
	if p.Name() != "openai" {
		t.Errorf("expected name %q, got %q", "openai", p.Name())
	}
}

func TestModels(t *testing.T) {
	p := openai.New("key", openai.WithPricing(openai.DeepSeekModels))
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("expected at least one model in pricing table")
	}
	found := make(map[string]bool)
	for _, m := range models {
		found[m.ID] = true
	}
	for _, expected := range []string{"deepseek-coder", "deepseek-chat"} {
		if !found[expected] {
			t.Errorf("expected model %q not found in Models()", expected)
		}
	}
}

func TestErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		errResp := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Rate limit exceeded",
				"type":    "rate_limit_error",
				"code":    "rate_limit_exceeded",
			},
		}
		_ = json.NewEncoder(w).Encode(errResp)
	}))
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))
	req := &llmint.Request{
		Model:    "gpt-4o",
		Messages: []llmint.Message{{Role: "user", Content: "hello"}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}
}
