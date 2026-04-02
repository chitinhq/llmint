// Package openai provides an llmint.Provider adapter for OpenAI-compatible
// APIs, including OpenAI itself, DeepSeek, Mistral, and other providers that
// implement the OpenAI Chat Completions API.
package openai

import (
	"github.com/AgentGuardHQ/llmint"
)

// DeepSeekModels contains pricing data for DeepSeek models hosted on the
// DeepSeek API (api.deepseek.com). Prices are in USD per million tokens.
var DeepSeekModels = map[string]llmint.ModelInfo{
	"deepseek-coder": {
		ID:           "deepseek-coder",
		InputPerMTok: 0.14,
		OutputPerMTok: 0.28,
	},
	"deepseek-chat": {
		ID:           "deepseek-chat",
		InputPerMTok: 0.14,
		OutputPerMTok: 0.28,
	},
}

// ComputeCost calculates the USD cost of the given Usage for the named model
// using the provided pricing map. Returns 0 if the model is not found in the
// pricing map or if pricing is nil.
func ComputeCost(model string, usage llmint.Usage, pricing map[string]llmint.ModelInfo) float64 {
	if pricing == nil {
		return 0
	}
	info, ok := pricing[model]
	if !ok {
		return 0
	}
	return usage.ComputeCost(info)
}
