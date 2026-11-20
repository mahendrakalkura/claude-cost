package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiDiscoverEnvOverride(t *testing.T) {
	root := t.TempDir()
	session := filepath.Join(root, "--home-user-project--", "2026-06-26T10-00-00-000Z_a.jsonl")
	task := filepath.Join(root, "--home-user-project--", "2026-06-26T10-00-00-000Z_a", "tasks", "2026-06-26T10-01-00-000Z_b.jsonl")
	index := filepath.Join(root, "--home-user-project--", ".session-index.json")
	for _, path := range []string{session, task, index} {
		err := os.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(path, []byte("{}"), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(PiDirsEnv, root)

	got, err := (&PiParser{}).Discover()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{session: true, task: true}
	if len(got) != len(want) {
		t.Fatalf("Discover: got %v, want %v", got, want)
	}
	for _, path := range got {
		if !want[path] {
			t.Errorf("Discover returned unexpected %s", path)
		}
	}
}

func TestPiDiscoverSessionDirEnv(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "s.jsonl")
	err := os.WriteFile(want, []byte("{}"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(piAgentDirEnv, t.TempDir())
	t.Setenv(piSessionDirEnv, root)

	got, err := (&PiParser{}).Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("Discover with %s: got %v, want [%s]", piSessionDirEnv, got, want)
	}
}

func TestPiParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	lines := []string{
		`{"type":"session","version":3,"id":"s1","timestamp":"2026-06-26T10:00:00.000Z","cwd":"/home/user/project"}`,
		`{"type":"model_change","id":"m1","parentId":null,"timestamp":"2026-06-26T10:00:00.100Z","provider":"anthropic","modelId":"claude-opus-4-8"}`,
		`{"type":"message","id":"u1","parentId":"m1","timestamp":"2026-06-26T10:00:01.000Z","message":{"role":"user","content":"hi","timestamp":1}}`,
		`{"type":"message","id":"a1","parentId":"u1","timestamp":"2026-06-26T10:00:02.000Z","message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input":100,"output":40,"cacheRead":9,"cacheWrite":3,"reasoning":10,"totalTokens":152,"cost":{"total":1}}}}`,
		`{"type":"message","id":"a2","parentId":"a1","timestamp":"2026-06-26T10:00:03.000Z","message":{"role":"assistant","model":"virtual","responseModel":"claude-sonnet-4-5","usage":{"input":1,"output":2,"cacheRead":0,"cacheWrite":0}}}`,
		`{"type":"message","id":"a3","parentId":"a2","timestamp":"2026-06-26T10:00:04.000Z","message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0}}}`,
		`{"type":"message","id":"t1","parentId":"a3","timestamp":"2026-06-26T10:00:05.000Z","message":{"role":"toolResult","toolName":"Agent","details":{"modelName":"deepseek v4.1 flash"},"usage":{"input":500,"output":50,"cacheRead":0,"cacheWrite":0}}}`,
		`{"type":"message","id":"t2","parentId":"t1","timestamp":"2026-06-26T10:00:05.500Z","message":{"role":"toolResult","toolName":"get_subagent_result","usage":{"input":600,"output":60,"cacheRead":0,"cacheWrite":0}}}`,
		`{"type":"message","id":"t3","parentId":"t2","timestamp":"2026-06-26T10:00:05.600Z","message":{"role":"toolResult","toolName":"bash","content":[]}}`,
		`{"type":"usage","id":"w1","parentId":"t1","timestamp":"2026-06-26T10:00:06.000Z","kind":"cache_warm","provider":"anthropic","model":"claude-opus-4-8","usage":{"input":2,"output":1,"cacheRead":5000,"cacheWrite":0}}`,
		`{"type":"compaction","id":"c1","parentId":"w1","timestamp":"2026-06-26T10:00:07.000Z","summary":"s","firstKeptEntryId":"a1","tokensBefore":1,"usage":{"input":700,"output":70,"cacheRead":0,"cacheWrite":0}}`,
		`not-json`,
		`{"type":"message","id":"big","parentId":"c1","timestamp":"2026-06-26T10:00:08.000Z","message":{"role":"assistant","model":"claude-opus-4-8","content":"` + strings.Repeat("x", 12*1024*1024) + `","usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0}}}`,
	}
	err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		t.Fatal(err)
	}

	records, err := (&PiParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		cacheRead, cacheWrite, input, output, reasoning int64
		id, model                                       string
	}
	wants := []want{
		{9, 3, 100, 30, 10, "a1 2026-06-26T10:00:02.000Z", "claude-opus-4-8"},
		{0, 0, 1, 2, 0, "a2 2026-06-26T10:00:03.000Z", "claude-sonnet-4-5"},
		{0, 0, 500, 50, 0, "t1 2026-06-26T10:00:05.000Z", "deepseek v4.1 flash"},
		{0, 0, 600, 60, 0, "t2 2026-06-26T10:00:05.500Z", piSubagentModel},
		{5000, 0, 2, 1, 0, "w1 2026-06-26T10:00:06.000Z", "claude-opus-4-8"},
		// Compaction usage carries no model, so it takes the model of the latest assistant message (a3).
		{0, 0, 700, 70, 0, "c1 2026-06-26T10:00:07.000Z", "claude-opus-4-8"},
		{0, 0, 1, 1, 0, "big 2026-06-26T10:00:08.000Z", "claude-opus-4-8"},
	}
	if len(records) != len(wants) {
		t.Fatalf("got %d records, want %d: %+v", len(records), len(wants), records)
	}
	for i, w := range wants {
		r := records[i]
		if r.Agent != "pi" || r.Project != "/home/user/project" || r.ID != w.id || r.Model != w.model || r.Timestamp.IsZero() {
			t.Errorf("record %d metadata: %+v", i, r)
		}
		if r.CacheReadTokens != w.cacheRead || r.CacheWriteTokens != w.cacheWrite || r.InputTokens != w.input || r.OutputTokens != w.output || r.ReasoningTokens != w.reasoning {
			t.Errorf("record %d tokens: %+v, want %+v", i, r, w)
		}
	}
}
