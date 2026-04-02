// Package anthropic provides an llmint.Provider adapter for the Anthropic
// Messages API, with built-in pricing data for current Claude models.
package anthropic

import (
	"github.com/AgentGuardHQ/llmint"
)

// Models contains pricing data for supported Claude models.
// Prices are in USD per million tokens.
var Models = map[string]llmint.ModelInfo{
	"claude-3-haiku-20240307": {
		ID:                "claude-3-haiku-20240307",
		InputPerMTok:      0.80,
		OutputPerMTok:     4.00,
		CacheReadPerMTok:  0.08,
		CacheWritePerMTok: 1.00,
		MaxContextTokens:  200_000,
	},
	"claude-3-5-sonnet-20241022": {
		ID:                "claude-3-5-sonnet-20241022",
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheReadPerMTok:  0.30,
		CacheWritePerMTok: 3.75,
		MaxContextTokens:  200_000,
	},
	"claude-3-opus-20240229": {
		ID:                "claude-3-opus-20240229",
		InputPerMTok:      15.00,
		OutputPerMTok:     75.00,
		CacheReadPerMTok:  1.50,
		CacheWritePerMTok: 18.75,
		MaxContextTokens:  200_000,
	},
}

// ComputeCost calculates the USD cost of the given Usage for the named model.
// Returns 0 if the model is not in the pricing table.
func ComputeCost(model string, usage llmint.Usage) float64 {
	info, ok := Models[model]
	if !ok {
		return 0
	}
	return usage.ComputeCost(info)
}
