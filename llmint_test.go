package llmint

import (
	"testing"
)

func TestRequestHash(t *testing.T) {
	req1 := &Request{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}
	req2 := &Request{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}
	req3 := &Request{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []Message{{Role: "user", Content: "world"}},
	}

	h1 := req1.Hash()
	h2 := req2.Hash()
	h3 := req3.Hash()

	if h1 == "" {
		t.Fatal("Hash() returned empty string")
	}
	if h1 != h2 {
		t.Errorf("identical requests produced different hashes: %s vs %s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("different requests produced the same hash: %s", h1)
	}
}

func TestUsageCost(t *testing.T) {
	info := ModelInfo{
		ID:                "test-model",
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheReadPerMTok:  0.30,
		CacheWritePerMTok: 3.75,
	}

	usage := Usage{
		InputTokens:      1_000_000,
		OutputTokens:     100_000,
		CacheReadTokens:  500_000,
		CacheWriteTokens: 200_000,
	}

	// Expected cost:
	// Input:       1M  * $3.00/MTok  = $3.00
	// Output:      0.1M * $15.00/MTok = $1.50
	// CacheRead:   0.5M * $0.30/MTok  = $0.15
	// CacheWrite:  0.2M * $3.75/MTok  = $0.75
	// Total = $5.40

	cost := usage.ComputeCost(info)
	expected := 5.40

	if abs(cost-expected) > 0.0001 {
		t.Errorf("expected cost %.4f, got %.4f", expected, cost)
	}
}

func TestSavingsTotal(t *testing.T) {
	savings := []Savings{
		{TokensSaved: 100, CostSaved: 0.10, Technique: "cache"},
		{TokensSaved: 200, CostSaved: 0.20, Technique: "dedup"},
		{TokensSaved: 50, CostSaved: 0.05, Technique: "trim"},
	}

	total := TotalSavings(savings)

	if total.TokensSaved != 350 {
		t.Errorf("expected 350 tokens saved, got %d", total.TokensSaved)
	}
	if abs(total.CostSaved-0.35) > 0.0001 {
		t.Errorf("expected $0.35 cost saved, got $%.4f", total.CostSaved)
	}
	if total.Technique != "total" {
		t.Errorf("expected technique 'total', got %q", total.Technique)
	}
}

func TestCacheStatusString(t *testing.T) {
	cases := []struct {
		status CacheStatus
		want   string
	}{
		{CacheMiss, "miss"},
		{CacheHit, "hit"},
		{CachePartial, "partial"},
	}

	for _, tc := range cases {
		got := tc.status.String()
		if got != tc.want {
			t.Errorf("CacheStatus(%d).String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
