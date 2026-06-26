package main

import (
	"math"
	"testing"
)

func testProvider() *Provider {
	return &Provider{prices: map[string]ModelPrice{
		"claude-opus-4-8": {
			CacheCreationInputTokenCost: 3e-6,
			CacheReadInputTokenCost:     1e-7,
			InputCostPerToken:           1e-6,
			OutputCostPerToken:          2e-6,
		},
		"google/gemini-2.5-pro": {InputCostPerToken: 5e-7, OutputCostPerToken: 1e-6},
	}}
}

func TestLookupExact(t *testing.T) {
	p := testProvider()
	if _, ok := p.Lookup("claude-opus-4-8"); !ok {
		t.Error("exact match should be found")
	}
}

func TestLookupCaseInsensitive(t *testing.T) {
	p := testProvider()
	if _, ok := p.Lookup("CLAUDE-OPUS-4-8"); !ok {
		t.Error("case-insensitive match should be found")
	}
}

func TestLookupStripsProviderPrefix(t *testing.T) {
	p := testProvider()
	// Not stored exactly, but the base name matches after stripping "anthropic/".
	if _, ok := p.Lookup("anthropic/claude-opus-4-8"); !ok {
		t.Error("prefix-stripped match should be found")
	}
}

func TestLookupNotFound(t *testing.T) {
	p := testProvider()
	if _, ok := p.Lookup("no-such-model"); ok {
		t.Error("unknown model should not be found")
	}
}

func TestCost(t *testing.T) {
	price := ModelPrice{
		CacheCreationInputTokenCost: 3e-6,
		CacheReadInputTokenCost:     1e-7,
		InputCostPerToken:           1e-6,
		OutputCostPerToken:          2e-6,
	}
	// input 1.0 + output 1.0 + cacheRead 0.2 + cacheWrite 0.3 + reasoning 0.02 = 2.52
	got := Cost(price, 1_000_000, 500_000, 2_000_000, 100_000, 10_000)
	if math.Abs(got-2.52) > 1e-9 {
		t.Errorf("got %v, want 2.52", got)
	}
}

func TestCostZero(t *testing.T) {
	if got := Cost(ModelPrice{}, 1, 2, 3, 4, 5); got != 0 {
		t.Errorf("zero price should yield zero cost, got %v", got)
	}
}

func TestParsePrices(t *testing.T) {
	data := []byte(`{"good":{"input_cost_per_token":0.001},"bad":42}`)
	prices, err := parsePrices(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := prices["good"].InputCostPerToken; got != 0.001 {
		t.Errorf("good input cost: got %v, want 0.001", got)
	}
	// A non-object entry is skipped, not fatal.
	if _, ok := prices["bad"]; ok {
		t.Error("malformed entry should be skipped")
	}
}

func TestParsePricesInvalidJSON(t *testing.T) {
	if _, err := parsePrices([]byte("not json")); err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestLoadFallbackEmbedded(t *testing.T) {
	prices := loadFallback()
	if len(prices) == 0 {
		t.Fatal("embedded fallback pricing must not be empty")
	}
	if _, ok := prices["claude-opus-4-8"]; !ok {
		t.Error("expected claude-opus-4-8 in embedded fallback")
	}
}
