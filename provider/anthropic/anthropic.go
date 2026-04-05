package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/chitinhq/llmint"
)

const (
	defaultBaseURL       = "https://api.anthropic.com/v1/messages"
	defaultMaxTokens     = 1024
	anthropicVersion     = "2023-06-01"
)

// apiMessage is the Anthropic Messages API message format.
type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// apiRequest is the Anthropic Messages API request body.
type apiRequest struct {
	Model     string       `json:"model"`
	Messages  []apiMessage `json:"messages"`
	System    string       `json:"system,omitempty"`
	MaxTokens int          `json:"max_tokens"`
}

// apiContentBlock is a single block in an Anthropic response.
type apiContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// apiUsage is the usage section of an Anthropic response.
type apiUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// apiResponse is the Anthropic Messages API response body.
type apiResponse struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Role    string            `json:"role"`
	Model   string            `json:"model"`
	Content []apiContentBlock `json:"content"`
	Usage   apiUsage          `json:"usage"`
}

// apiError is the Anthropic API error response.
type apiError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// options holds optional configuration for the Provider.
type options struct {
	baseURL string
}

// Option is a functional option for Provider construction.
type Option func(*options)

// WithBaseURL overrides the Anthropic API endpoint. Useful for testing.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// Provider implements llmint.Provider against the Anthropic Messages API.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New constructs a Provider with the given API key and optional options.
func New(apiKey string, opts ...Option) *Provider {
	cfg := options{baseURL: defaultBaseURL}
	for _, o := range opts {
		o(&cfg)
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: cfg.baseURL,
		client:  &http.Client{},
	}
}

// Complete sends a request to the Anthropic Messages API and returns the response.
func (p *Provider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	messages := make([]apiMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		// Anthropic API only accepts "user" and "assistant" roles in messages.
		// System prompt is sent as the top-level "system" field.
		if m.Role == "system" {
			continue
		}
		messages = append(messages, apiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	apiReq := apiRequest{
		Model:     req.Model,
		Messages:  messages,
		System:    req.System,
		MaxTokens: maxTokens,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create http request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var apiErr apiError
		if jerr := json.Unmarshal(respBody, &apiErr); jerr == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("anthropic: api error %d: %s", httpResp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("anthropic: unexpected status %d", httpResp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("anthropic: parse response: %w", err)
	}

	content := make([]llmint.ContentBlock, 0, len(apiResp.Content))
	for _, b := range apiResp.Content {
		content = append(content, llmint.ContentBlock{
			Type: b.Type,
			Text: b.Text,
		})
	}

	usage := llmint.Usage{
		InputTokens:      apiResp.Usage.InputTokens,
		OutputTokens:     apiResp.Usage.OutputTokens,
		CacheReadTokens:  apiResp.Usage.CacheReadInputTokens,
		CacheWriteTokens: apiResp.Usage.CacheCreationInputTokens,
	}
	usage.Cost = ComputeCost(apiResp.Model, usage)

	// Determine cache status.
	cacheStatus := llmint.CacheMiss
	if usage.CacheReadTokens > 0 && usage.InputTokens == 0 {
		cacheStatus = llmint.CacheHit
	} else if usage.CacheReadTokens > 0 {
		cacheStatus = llmint.CachePartial
	}

	return &llmint.Response{
		Content:     content,
		Usage:       usage,
		Model:       apiResp.Model,
		CacheStatus: cacheStatus,
	}, nil
}

// Name returns "anthropic".
func (p *Provider) Name() string { return "anthropic" }

// Models returns all models in the pricing table.
func (p *Provider) Models() []llmint.ModelInfo {
	out := make([]llmint.ModelInfo, 0, len(Models))
	for _, m := range Models {
		out = append(out, m)
	}
	return out
}
