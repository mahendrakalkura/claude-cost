package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ModelPrice holds per-token costs for a single model.
type ModelPrice struct {
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
}

// Provider resolves model names to prices, updating live when possible.
type Provider struct {
	mu       sync.RWMutex
	noFetch  bool
	prices   map[string]ModelPrice
}

// NewProvider loads pricing, preferring cache then live fetch then embedded fallback.
func NewProvider(refresh, noFetch bool) (*Provider, error) {
	p := &Provider{noFetch: noFetch}

	if !refresh && !noFetch {
		if cached, ok := loadCache(); ok {
			p.prices = cached
			return p, nil
		}
	}
	if !noFetch {
		if fetched, err := fetchLive(); err == nil {
			p.prices = fetched
			return p, nil
		}
	}
	if cached, ok := loadCache(); ok {
		p.prices = cached
		return p, nil
	}
	p.prices = loadFallback()
	return p, nil
}

// Lookup finds the best matching price for a model ID. Returns the price
// and true if found, zero price and false otherwise.
func (p *Provider) Lookup(modelID string) (ModelPrice, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if price, ok := p.prices[modelID]; ok {
		return price, true
	}
	lower := strings.ToLower(modelID)
	for key, price := range p.prices {
		if strings.EqualFold(key, modelID) {
			return price, true
		}
		if strings.EqualFold(key, lower) {
			return price, true
		}
	}
	// Try without provider prefix (e.g. "google/gemini-2.5-pro" → "gemini-2.5-pro").
	if _, after, ok := strings.Cut(modelID, "/"); ok {
		return p.Lookup(after)
	}
	return ModelPrice{}, false
}

// Cost computes the USD cost for token counts at the given price.
func Cost(price ModelPrice, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, reasoningTokens int64) float64 {
	cost := float64(inputTokens)*price.InputCostPerToken +
		float64(outputTokens)*price.OutputCostPerToken +
		float64(cacheReadTokens)*price.CacheReadInputTokenCost +
		float64(cacheWriteTokens)*price.CacheCreationInputTokenCost +
		float64(reasoningTokens)*price.OutputCostPerToken
	// Reasonable rounding: all models have at most 6 decimal places, but
	// float accumulation can drift, so round to nanodollar precision.
	return math.Round(cost*1e9) / 1e9
}

// cachePath returns the path to the pricing cache file.
func cachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".cache", "claude-cost")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "pricing.json")
}

func loadCache() (map[string]ModelPrice, bool) {
	cp := cachePath()
	if cp == "" {
		return nil, false
	}
	fi, err := os.Stat(cp)
	if err != nil {
		return nil, false
	}
	if time.Since(fi.ModTime()) > 24*time.Hour {
		return nil, false
	}
	data, err := os.ReadFile(cp)
	if err != nil {
		return nil, false
	}
	prices, err := parsePrices(data)
	if err != nil {
		return nil, false
	}
	return prices, true
}

func parsePrices(data []byte) (map[string]ModelPrice, error) {
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	prices := make(map[string]ModelPrice, len(raw))
	for modelID, entry := range raw {
		var mp ModelPrice
		if err := json.Unmarshal(entry, &mp); err != nil {
			continue
		}
		prices[modelID] = mp
	}
	return prices, nil
}
