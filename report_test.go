package main

import (
	"testing"
	"time"
)

func TestComputeAggregatesAndPrices(t *testing.T) {
	p := &Provider{prices: map[string]ModelPrice{
		"priced-model": {InputCostPerToken: 0.001},
	}}
	jan := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 5, 9, 0, 0, 0, time.UTC)
	records := []Record{
		{Agent: "claude", Model: "priced-model", InputTokens: 1000, Timestamp: jan},
		{Agent: "claude", Model: "priced-model", InputTokens: 2000, Timestamp: jan},
		{Agent: "codex", Model: "unpriced-model", InputTokens: 5000, Timestamp: feb},
	}

	res := Compute(records, p)

	if res.Total != 3 || res.Priced != 2 {
		t.Errorf("priced/total: got %d/%d, want 2/3", res.Priced, res.Total)
	}

	// Two agents, alphabetical.
	if len(res.Agents) != 2 || res.Agents[0].Agent != "claude" || res.Agents[1].Agent != "codex" {
		t.Fatalf("agents: %+v", res.Agents)
	}
	if res.Agents[0].InputTokens != 3000 {
		t.Errorf("claude input tokens: got %d, want 3000", res.Agents[0].InputTokens)
	}
	if !res.Agents[0].HasCost || res.Agents[0].Cost != 3.0 {
		t.Errorf("claude cost: got %v (hasCost=%v), want 3.0", res.Agents[0].Cost, res.Agents[0].HasCost)
	}
	if res.Agents[1].HasCost {
		t.Error("codex used an unpriced model and should have HasCost=false")
	}

	// Two distinct days and two distinct months, both sorted ascending.
	if len(res.Days) != 2 || res.Days[0].Date != "2026-01-10" || res.Days[1].Date != "2026-02-05" {
		t.Errorf("days: %+v", res.Days)
	}
	if len(res.Months) != 2 || res.Months[0].Month != "2026-01" || res.Months[1].Month != "2026-02" {
		t.Errorf("months: %+v", res.Months)
	}

	if len(res.UnpricedModels) != 1 || res.UnpricedModels[0] != "unpriced-model" {
		t.Errorf("unpriced models: %v", res.UnpricedModels)
	}
}

func TestComputeEmpty(t *testing.T) {
	res := Compute(nil, &Provider{prices: map[string]ModelPrice{}})
	if res.Total != 0 || len(res.Agents) != 0 || len(res.Days) != 0 || len(res.Months) != 0 {
		t.Errorf("empty input should yield empty results: %+v", res)
	}
}
