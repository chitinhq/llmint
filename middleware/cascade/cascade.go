package cascade

import (
	"context"
	"fmt"
	"strings"

	"github.com/AgentGuardHQ/llmint"
)

// Model is one tier in the escalation ladder.
type Model struct {
	// Provider is the backend used for this tier.
	Provider llmint.Provider
	// Name labels this tier (e.g. "haiku", "sonnet").
	Name string
	// Threshold is the minimum confidence score to accept this tier's response.
	// A value of 0 means always accept (useful for the last/most capable tier).
	Threshold float64
}

// options holds cascade configuration.
type options struct {
	scorer         Scorer
	maxEscalations int // 0 = unlimited
}

// Option is a functional option for the cascade middleware.
type Option func(*options)

// WithScorer overrides the default confidence scorer.
func WithScorer(s Scorer) Option {
	return func(o *options) { o.scorer = s }
}

// WithMaxEscalations caps the number of times the cascade will escalate.
// For example, WithMaxEscalations(1) allows at most one escalation (two tiers total).
func WithMaxEscalations(n int) Option {
	return func(o *options) { o.maxEscalations = n }
}

// New returns a cascade Middleware that iterates through models cheapest-first,
// escalating to the next tier when the scorer returns a value below the
// current tier's Threshold.
func New(models []Model, opts ...Option) llmint.Middleware {
	cfg := options{scorer: DefaultScorer()}
	for _, o := range opts {
		o(&cfg)
	}

	return func(_ llmint.Provider) llmint.Provider {
		return &cascadeProvider{models: models, cfg: cfg}
	}
}

type cascadeProvider struct {
	models []Model
	cfg    options
}

func (p *cascadeProvider) Complete(ctx context.Context, req *llmint.Request) (*llmint.Response, error) {
	escalations := 0
	for i, m := range p.models {
		resp, err := m.Provider.Complete(ctx, req)
		if err != nil {
			return nil, err
		}
		resp.Model = m.Name

		isLast := i == len(p.models)-1
		if isLast || m.Threshold == 0 {
			return resp, nil
		}

		score := p.cfg.scorer(resp)
		if score >= m.Threshold {
			return resp, nil
		}

		// Below threshold — escalate if budget allows.
		escalations++
		if p.cfg.maxEscalations > 0 && escalations >= p.cfg.maxEscalations {
			// Use next model unconditionally; fetch its response.
			next := p.models[i+1]
			resp2, err := next.Provider.Complete(ctx, req)
			if err != nil {
				return nil, err
			}
			resp2.Model = next.Name
			resp2.Savings = append(resp2.Savings, llmint.Savings{Technique: "cascade"})
			return resp2, nil
		}
		// Cascade records a savings entry on the response that ultimately wins.
		_ = resp // discard, continue to next tier
	}
	return nil, fmt.Errorf("cascade: no models configured")
}

func (p *cascadeProvider) Name() string {
	if len(p.models) == 0 {
		return "cascade()"
	}
	names := make([]string, len(p.models))
	for i, m := range p.models {
		names[i] = m.Name
	}
	return "cascade(" + strings.Join(names, "→") + ")"
}

func (p *cascadeProvider) Models() []llmint.ModelInfo {
	var out []llmint.ModelInfo
	for _, m := range p.models {
		out = append(out, m.Provider.Models()...)
	}
	return out
}
