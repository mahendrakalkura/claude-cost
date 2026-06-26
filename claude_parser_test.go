package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	lines := `{"type":"assistant","timestamp":"2026-06-26T10:00:00Z","message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":40,"cache_read_input_tokens":9,"cache_creation_input_tokens":3}}}
{"type":"user","timestamp":"2026-06-26T10:00:01Z","message":{"role":"user"}}
{"type":"assistant","timestamp":"2026-06-26T10:00:02Z","message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
not-json
`
	if err := os.WriteFile(path, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	p := &ClaudeParser{}
	records, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	// Only the first line is a non-empty assistant usage row.
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %+v", len(records), records)
	}
	r := records[0]
	if r.Agent != "claude" || r.Model != "claude-opus-4-8" {
		t.Errorf("agent/model: %+v", r)
	}
	if r.InputTokens != 100 || r.OutputTokens != 40 || r.CacheReadTokens != 9 || r.CacheWriteTokens != 3 {
		t.Errorf("token counts: %+v", r)
	}
	if r.Timestamp.IsZero() {
		t.Error("timestamp not parsed")
	}
}

func TestClaudeDiscoverEnvOverride(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "my-project", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ClaudeDirsEnv, root)

	got, err := (&ClaudeParser{}).Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("Discover with env override: got %v, want [%s]", got, want)
	}
}

func TestProjectFromDir(t *testing.T) {
	got := projectFromDir(filepath.Join("anything", "-home-user--agents"))
	want := string(filepath.Separator) + "home" + string(filepath.Separator) + "user" + string(filepath.Separator) + string(filepath.Separator) + "agents"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
