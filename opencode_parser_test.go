package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpencodeParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "msg.json")
	body := `{"role":"assistant","modelID":"claude-opus-4-8","time":{"created":1700000000000},"tokens":{"input":100,"output":20,"reasoning":5,"cache":{"read":10,"write":2}}}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := (&OpencodeParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %+v", len(records), records)
	}
	r := records[0]
	if r.Agent != "opencode" || r.Model != "claude-opus-4-8" {
		t.Errorf("agent/model: %+v", r)
	}
	if r.InputTokens != 100 || r.OutputTokens != 20 || r.ReasoningTokens != 5 || r.CacheReadTokens != 10 || r.CacheWriteTokens != 2 {
		t.Errorf("token counts: %+v", r)
	}
	if r.Timestamp.IsZero() {
		t.Error("timestamp not parsed")
	}
}

func TestOpencodeParseSkipsNonAssistant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user.json")
	body := `{"role":"user","modelID":"claude-opus-4-8","tokens":{"input":100}}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	records, err := (&OpencodeParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("user-role message should yield no records, got %+v", records)
	}
}

func TestOpencodeDiscoverEnvOverride(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "session-1", "m.json")
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(OpencodeDirsEnv, root)

	got, err := (&OpencodeParser{}).Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("Discover with env override: got %v, want [%s]", got, want)
	}
}
