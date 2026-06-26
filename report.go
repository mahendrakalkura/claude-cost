package main

import (
	"math"
	"sort"
)

// AgentSummary holds aggregated usage and cost for one agent.
type AgentSummary struct {
	Agent           string
	CacheReadTokens int64
	CacheWriteTokens int64
	Cost            float64
	HasCost         bool
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
}

// DaySummary holds aggregated usage and cost for one day.
type DaySummary struct {
	Date            string
	CacheReadTokens int64
	CacheWriteTokens int64
	Cost            float64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
}

// MonthSummary holds aggregated usage and cost for one month.
type MonthSummary struct {
	Month           string
	CacheReadTokens int64
	CacheWriteTokens int64
	Cost            float64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
}

// Results holds all aggregated views.
type Results struct {
	Agents []AgentSummary
	Days   []DaySummary
	Months []MonthSummary
	Priced int
	Total  int
	UnpricedModels []string
}

// Compute aggregates records into agent, day, and month views.
func Compute(records []Record, p *Provider) *Results {
	agentMap := map[string]*AgentSummary{}
	dayMap := map[string]*DaySummary{}
	monthMap := map[string]*MonthSummary{}
	priced := 0
	total := 0
	unpricedSet := map[string]struct{}{}

	for _, r := range records {
		total++
		key := r.Agent

		as := agentMap[key]
		if as == nil {
			as = &AgentSummary{Agent: key}
			agentMap[key] = as
		}
		as.CacheReadTokens += r.CacheReadTokens
		as.CacheWriteTokens += r.CacheWriteTokens
		as.InputTokens += r.InputTokens
		as.OutputTokens += r.OutputTokens
		as.ReasoningTokens += r.ReasoningTokens

		dayKey := r.Timestamp.Format("2006-01-02")
		ds := dayMap[dayKey]
		if ds == nil {
			ds = &DaySummary{Date: dayKey}
			dayMap[dayKey] = ds
		}
		ds.CacheReadTokens += r.CacheReadTokens
		ds.CacheWriteTokens += r.CacheWriteTokens
		ds.InputTokens += r.InputTokens
		ds.OutputTokens += r.OutputTokens
		ds.ReasoningTokens += r.ReasoningTokens

		monthKey := r.Timestamp.Format("2006-01")
		ms := monthMap[monthKey]
		if ms == nil {
			ms = &MonthSummary{Month: monthKey}
			monthMap[monthKey] = ms
		}
		ms.CacheReadTokens += r.CacheReadTokens
		ms.CacheWriteTokens += r.CacheWriteTokens
		ms.InputTokens += r.InputTokens
		ms.OutputTokens += r.OutputTokens
		ms.ReasoningTokens += r.ReasoningTokens

		price, ok := p.Lookup(r.Model)
		if ok {
			cost := Cost(price, r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens, r.ReasoningTokens)
			as.Cost += cost
			as.HasCost = true
			ds.Cost += cost
			ms.Cost += cost
			priced++
		} else {
			unpricedSet[r.Model] = struct{}{}
		}
	}

	// Round costs to cents.
	for _, as := range agentMap {
		as.Cost = math.Round(as.Cost*100) / 100
	}
	for _, ds := range dayMap {
		ds.Cost = math.Round(ds.Cost*100) / 100
	}
	for _, ms := range monthMap {
		ms.Cost = math.Round(ms.Cost*100) / 100
	}

	res := &Results{Priced: priced, Total: total}
	for _, as := range agentMap {
		res.Agents = append(res.Agents, *as)
	}
	sort.Slice(res.Agents, func(i, j int) bool { return res.Agents[i].Agent < res.Agents[j].Agent })

	for _, ds := range dayMap {
		res.Days = append(res.Days, *ds)
	}
	sort.Slice(res.Days, func(i, j int) bool { return res.Days[i].Date < res.Days[j].Date })

	for _, ms := range monthMap {
		res.Months = append(res.Months, *ms)
	}
	sort.Slice(res.Months, func(i, j int) bool { return res.Months[i].Month < res.Months[j].Month })

	for m := range unpricedSet {
		res.UnpricedModels = append(res.UnpricedModels, m)
	}
	sort.Strings(res.UnpricedModels)

	return res
}
