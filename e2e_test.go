package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cleanEnv returns the current environment with HOME removed, then sets HOME to
// the given temp dir and appends the extra KEY=VALUE entries. Removing the
// original HOME matters because syscall.Getenv returns the first match.
func cleanEnv(home string, extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOME=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+home)
	return append(env, extra...)
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "claude-cost")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// writeFixtures populates a claude and an opencode log under temp roots and
// returns the env overrides pointing the parsers at them, plus an isolated HOME.
func writeFixtures(t *testing.T) (home string, env []string) {
	t.Helper()
	home = t.TempDir()

	claudeRoot := filepath.Join(t.TempDir(), "claude")
	claudeFile := filepath.Join(claudeRoot, "proj", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(claudeFile), 0755); err != nil {
		t.Fatal(err)
	}
	claudeLine := `{"type":"assistant","timestamp":"2026-06-26T10:00:00Z","message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":1000,"output_tokens":500,"cache_read_input_tokens":200,"cache_creation_input_tokens":100}}}` + "\n"
	if err := os.WriteFile(claudeFile, []byte(claudeLine), 0644); err != nil {
		t.Fatal(err)
	}

	opencodeRoot := filepath.Join(t.TempDir(), "opencode")
	opencodeFile := filepath.Join(opencodeRoot, "sess", "m.json")
	if err := os.MkdirAll(filepath.Dir(opencodeFile), 0755); err != nil {
		t.Fatal(err)
	}
	opencodeBody := `{"role":"assistant","modelID":"claude-opus-4-8","time":{"created":1782900000000},"tokens":{"input":300,"output":60,"reasoning":0,"cache":{"read":0,"write":0}}}`
	if err := os.WriteFile(opencodeFile, []byte(opencodeBody), 0644); err != nil {
		t.Fatal(err)
	}

	// No codex fixtures: with HOME isolated to a temp dir, default codex
	// discovery globs an empty tree and contributes nothing.
	env = cleanEnv(home,
		ClaudeDirsEnv+"="+claudeRoot,
		OpencodeDirsEnv+"="+opencodeRoot,
	)
	return home, env
}

func TestEndToEndTables(t *testing.T) {
	bin := buildBinary(t)
	_, env := writeFixtures(t)

	cmd := exec.Command(bin, "--no-fetch")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	got := string(out)

	for _, want := range []string{"# By Agent", "# By Day", "# By Month", "claude", "opencode", "TOTAL", "priced:", "$"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

func TestEndToEndJSON(t *testing.T) {
	bin := buildBinary(t)
	_, env := writeFixtures(t)

	cmd := exec.Command(bin, "--no-fetch", "--json")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, `"Agents"`) || !strings.Contains(got, `"claude"`) {
		t.Errorf("json output missing expected keys\n%s", got)
	}
}

func TestEndToEndAgentOnly(t *testing.T) {
	bin := buildBinary(t)
	_, env := writeFixtures(t)

	cmd := exec.Command(bin, "--no-fetch", "--agent")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "# By Agent") {
		t.Errorf("expected agent table\n%s", got)
	}
	if strings.Contains(got, "# By Day") || strings.Contains(got, "# By Month") {
		t.Errorf("--agent should print only the agent table\n%s", got)
	}
}
