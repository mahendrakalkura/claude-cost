package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFmtNum(t *testing.T) {
	cases := map[int64]string{
		0:        "0",
		100:      "100",
		1000:     "1,000",
		12345:    "12,345",
		1234567:  "1,234,567",
		12345678: "12,345,678",
	}
	for in, want := range cases {
		if got := fmtNum(in); got != want {
			t.Errorf("fmtNum(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFmtCost(t *testing.T) {
	if got := fmtCost(42.17, true); got != "$42.17" {
		t.Errorf("got %q, want $42.17", got)
	}
	if got := fmtCost(0, false); got != "n/a" {
		t.Errorf("unpriced should be n/a, got %q", got)
	}
}

func TestAgentTableRendersHouseStyle(t *testing.T) {
	res := &Results{
		Agents: []AgentSummary{
			{Agent: "claude", InputTokens: 12345678, OutputTokens: 234567, CacheReadTokens: 9876543, Cost: 42.17, HasCost: true},
		},
		Priced: 1, Total: 1,
	}
	var buf bytes.Buffer
	AgentTable(&buf, res)
	out := buf.String()
	// go-pretty uppercases header labels; data cells keep their case.
	for _, want := range []string{"AGENT", "COST USD", "claude", "12,345,678", "$42.17", "TOTAL", "+", "|"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q\n%s", want, out)
		}
	}
}

func TestJSONOutput(t *testing.T) {
	res := &Results{
		Agents: []AgentSummary{{Agent: "claude", InputTokens: 10, Cost: 1.5, HasCost: true}},
		Priced: 1, Total: 1,
	}
	var buf bytes.Buffer
	if err := JSON(&buf, res); err != nil {
		t.Fatal(err)
	}
	var decoded Results
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded.Agents) != 1 || decoded.Agents[0].Agent != "claude" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}

func TestFooter(t *testing.T) {
	var buf bytes.Buffer
	Footer(&buf, &Results{Priced: 2, Total: 3, UnpricedModels: []string{"mystery-model"}})
	out := buf.String()
	if !strings.Contains(out, "priced: 2 of 3 models") {
		t.Errorf("footer missing priced count: %q", out)
	}
	if !strings.Contains(out, "mystery-model") {
		t.Errorf("footer missing unpriced model: %q", out)
	}
}
