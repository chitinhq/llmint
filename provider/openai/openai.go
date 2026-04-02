package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/AgentGuardHQ/llmint"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-4o"
)

// apiMessage is the OpenAI Chat Completions API message format.
type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// apiRequest is the OpenAI Chat Completions API request body.
type apiRequest struct {
	Model     string       `json:"model"`
	Messages  []apiMessage `json:"messages"`
	MaxTokens int          `json:"max_tokens,omitempty"`
}

// apiChoice is a single choice in an OpenAI response.
type apiChoice struct {
	Message apiMessage `json:"message"`
	Index   int        `json:"index"`
}

// apiUsage is the usage section of an OpenAI response.
type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// apiResponse is the OpenAI Chat Completions API response body.
type apiResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Model   string      `json:"model"`
	Choices []apiChoice `json:"choices"`
	Usage   apiUsage    `json:"usage"`
}

// apiError is the OpenAI API error response.
type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// options holds optional configuration for the Provider.
type options struct {
	baseURL    string
	model      string
	pricing    map[string]llmint.ModelInfo
	httpClient *http.Client
}

// Option is a functional option for Provider construction.
type Option func(*options)

// WithBaseURL overrides the API base URL. Useful for pointing at DeepSeek,
// Mistral, or other OpenAI-compatible providers, and for testing.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// WithModel sets the default model to use when the Request does not specify one.
func WithModel(model string) Option {
	return func(o *options) { o.model = model }
}

// WithPricing sets the pricing table used to compute costs on each response.
// Pass DeepSeekModels or a custom map for your provider.
func WithPricing(pricing map[string]llmint.ModelInfo) Option {
	return func(o *options) { o.pricing = pricing }
}

// WithHTTPClient replaces the default http.Client. Useful for testing or for
// injecting transport-level middleware (retries, tracing, etc.).
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) { o.httpClient = client }
}

// Provider implements llmint.Provider against any OpenAI-compatible Chat
// Completions API.
type Provider struct {
	apiKey  string
	baseURL string
	model   string
	pricing map[string]llmint.ModelInfo
	client  *http.Client
}

// New constructs a Provider with the given API key and optional options.
func New(apiKey string, opts ...Option) *Provider {
	cfg := options{
		baseURL:    defaultBaseURL,
		model:      defaultModel,
		httpClient: &http.Client{},
	}
	for _, o := range opts {
		o(&cfg)
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: cfg.baseURL,
		model:   cfg.model,
		pricing: cfg.pricing,
		client:  cfg.httpClient,
	}
}

// Complete sends a request to the OpenAI Chat Completions API and returns the
// response. The system prompt (if any) is sent as the first message with role
// "system", following the OpenAI convention.
func (p *Provider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Build messages: system prompt first (OpenAI format), then conversation.
	var messages []apiMessage
	if req.System != "" {
		messages = append(messages, apiMessage{
			Role:    "system",
			Content: req.System,
		})
	}
	for _, m := range req.Messages {
		messages = append(messages, apiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	apiReq := apiRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	endpoint := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create http request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var apiErr apiError
		if jerr := json.Unmarshal(respBody, &apiErr); jerr == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("openai: api error %d: %s", httpResp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("openai: unexpected status %d", httpResp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("openai: parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("openai: response contained no choices")
	}

	content := []llmint.ContentBlock{
		{
			Type: "text",
			Text: apiResp.Choices[0].Message.Content,
		},
	}

	usage := llmint.Usage{
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
	}
	usage.Cost = ComputeCost(apiResp.Model, usage, p.pricing)

	return &llmint.Response{
		Content: content,
		Usage:   usage,
		Model:   apiResp.Model,
	}, nil
}

// Name returns "openai".
func (p *Provider) Name() string { return "openai" }

// Models returns all models in the provider's pricing table.
// Returns an empty slice if no pricing table was configured.
func (p *Provider) Models() []llmint.ModelInfo {
	out := make([]llmint.ModelInfo, 0, len(p.pricing))
	for _, m := range p.pricing {
		out = append(out, m)
	}
	return out
}
