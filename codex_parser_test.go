package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-x.jsonl")
	lines := `{"type":"turn_context","timestamp":"2026-06-26T10:00:00Z","payload":{"model":"gpt-5.2-codex"}}
{"type":"event_msg","timestamp":"2026-06-26T10:00:01Z","payload":{"info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":50,"reasoning_output_tokens":10}}}}
{"type":"event_msg","timestamp":"2026-06-26T10:00:02Z","payload":{"info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":50,"reasoning_output_tokens":10}}}}
{"type":"event_msg","timestamp":"2026-06-26T10:00:03Z","payload":{"info":null}}
`
	if err := os.WriteFile(path, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := (&CodexParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	// The identical second event is deduped; the info=null event is skipped.
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %+v", len(records), records)
	}
	r := records[0]
	if r.Agent != "codex" || r.Model != "gpt-5.2-codex" {
		t.Errorf("agent/model: %+v", r)
	}
	// input is reported full including cached, so we subtract the cached portion.
	if r.InputTokens != 800 || r.CacheReadTokens != 200 || r.OutputTokens != 50 || r.ReasoningTokens != 10 {
		t.Errorf("token split: %+v", r)
	}
}

func TestCodexDiscoverEnvOverride(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "sessions", "2026", "06", "26", "rollout-abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(CodexDirsEnv, home)

	got, err := (&CodexParser{}).Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("Discover with env override: got %v, want [%s]", got, want)
	}
}
