// Package llmint provides foundational types and middleware primitives for
// LLM token economics — cost tracking, cache-aware usage, and provider composition.
package llmint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Provider is the core interface every LLM backend must implement.
type Provider interface {
	Complete(ctx context.Context, req *Request) (*Response, error)
	Name() string
	Models() []ModelInfo
}

// Middleware is a function that wraps a Provider with additional behavior.
// Follows the same convention as net/http middleware: the first middleware
// applied is the outermost layer.
type Middleware func(Provider) Provider

// ModelInfo describes pricing and capacity for a single model.
type ModelInfo struct {
	ID                string
	InputPerMTok      float64 // USD per million input tokens
	OutputPerMTok     float64 // USD per million output tokens
	CacheReadPerMTok  float64 // USD per million cache-read tokens
	CacheWritePerMTok float64 // USD per million cache-write tokens
	MaxContextTokens  int
}

// Message is a single turn in a conversation.
type Message struct {
	Role    string // "user" | "assistant" | "system"
	Content string
}

// Tool describes a function the model may invoke.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// Request is the canonical input sent to any Provider.
type Request struct {
	Model     string
	Messages  []Message
	Tools     []Tool
	MaxTokens int
	System    string
	Metadata  map[string]string
}

// Hash returns a deterministic SHA-256 hex digest of the request contents.
// Requests with identical fields always produce the same hash.
func (r *Request) Hash() string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	// Encode each field in a fixed order so the hash is stable.
	_ = enc.Encode(r.Model)
	_ = enc.Encode(r.Messages)
	_ = enc.Encode(r.Tools)
	_ = enc.Encode(r.MaxTokens)
	_ = enc.Encode(r.System)
	// Metadata is omitted from the hash intentionally — it carries
	// tracing/routing hints, not semantic request content.
	return hex.EncodeToString(h.Sum(nil))
}

// ContentBlock is a typed segment of model output.
type ContentBlock struct {
	Type string // "text" | "tool_use" | etc.
	Text string
}

// CacheStatus indicates how the prompt cache was utilised for a response.
type CacheStatus int

const (
	CacheMiss    CacheStatus = iota // No cache tokens used
	CacheHit                        // All input tokens served from cache
	CachePartial                    // Some input tokens served from cache
)

// String returns a human-readable label for the cache status.
func (c CacheStatus) String() string {
	switch c {
	case CacheHit:
		return "hit"
	case CachePartial:
		return "partial"
	default:
		return "miss"
	}
}

// Usage records raw token counts and computed cost for a single completion.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	Cost             float64
}

// ComputeCost calculates the USD cost of this Usage given a ModelInfo's pricing.
func (u Usage) ComputeCost(info ModelInfo) float64 {
	const perMTok = 1_000_000.0
	input := float64(u.InputTokens) / perMTok * info.InputPerMTok
	output := float64(u.OutputTokens) / perMTok * info.OutputPerMTok
	cacheRead := float64(u.CacheReadTokens) / perMTok * info.CacheReadPerMTok
	cacheWrite := float64(u.CacheWriteTokens) / perMTok * info.CacheWritePerMTok
	return input + output + cacheRead + cacheWrite
}

// Savings records the tokens and cost saved by a single optimisation technique.
type Savings struct {
	TokensSaved int
	CostSaved   float64
	Technique   string
}

// TotalSavings aggregates a slice of Savings into a single summary.
func TotalSavings(ss []Savings) Savings {
	var total Savings
	total.Technique = "total"
	for _, s := range ss {
		total.TokensSaved += s.TokensSaved
		total.CostSaved += s.CostSaved
	}
	return total
}

// Response is the canonical output from any Provider.
type Response struct {
	Content     []ContentBlock
	Usage       Usage
	Model       string
	CacheStatus CacheStatus
	Savings     []Savings
}
