// Package cascade provides a model escalation middleware that routes requests
// through increasingly capable (and expensive) models based on confidence scores.
package cascade

import (
	"regexp"
	"strconv"

	"github.com/chitinhq/llmint"
)

// Scorer is a function that evaluates the quality of a response.
// It returns a float64 in [0, 1] where 1.0 is maximum confidence.
type Scorer func(*llmint.Response) float64

var confidenceTagRe = regexp.MustCompile(`\[confidence:\s*([0-9]*\.?[0-9]+)\]`)

// DefaultScorer parses a [confidence: X.XX] tag from the first text content
// block of the response. Returns 0.5 if no tag is found. Clamps to [0, 1].
func DefaultScorer() Scorer {
	return func(resp *llmint.Response) float64 {
		for _, b := range resp.Content {
			if b.Type != "text" {
				continue
			}
			m := confidenceTagRe.FindStringSubmatch(b.Text)
			if m == nil {
				continue
			}
			v, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				continue
			}
			return clamp(v, 0, 1)
		}
		return 0.5
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
