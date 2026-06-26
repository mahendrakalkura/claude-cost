package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func init() {
	Register(&CodexParser{})
}

// CodexParser reads Codex CLI rollout JSONL session files.
type CodexParser struct{}

func (p *CodexParser) Name() string { return "codex" }

// CodexDirsEnv names the environment variable that overrides where the codex
// parser looks for session logs (OS-list-separated codex home directories, each
// holding a sessions/YYYY/MM/DD/rollout-*.jsonl tree).
const CodexDirsEnv = "CLAUDE_COST_CODEX_DIRS"

func (p *CodexParser) Discover() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var defaults []string
	matches, _ := filepath.Glob(filepath.Join(home, ".codex*"))
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil || !fi.IsDir() {
			continue
		}
		defaults = append(defaults, m)
	}
	return discover(rootsFromEnv(CodexDirsEnv, defaults), filepath.Join("sessions", "*", "*", "*", "rollout-*.jsonl"))
}

type codexEvent struct {
	Payload   json.RawMessage `json:"payload"`
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
}

type codexTurnContext struct {
	Model string `json:"model"`
}

type codexTokenCountInfo struct {
	Info *codexTokenCountPayload `json:"info"`
}

type codexTokenCountPayload struct {
	LastTokenUsage *codexTokenUsage `json:"last_token_usage"`
}

type codexTokenUsage struct {
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

func (p *CodexParser) Parse(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var (
		currentModel string
		lastUsage    *codexTokenUsage
		records      []Record
	)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var evt codexEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		switch evt.Type {
		case "turn_context":
			var tc codexTurnContext
			if json.Unmarshal(evt.Payload, &tc) == nil && tc.Model != "" {
				currentModel = tc.Model
			}
		case "event_msg":
			var msg codexTokenCountInfo
			if json.Unmarshal(evt.Payload, &msg) != nil {
				continue
			}
			if msg.Info == nil || msg.Info.LastTokenUsage == nil {
				continue
			}
			lu := msg.Info.LastTokenUsage
			// Dedup identical consecutive token counts.
			if lastUsage != nil &&
				lastUsage.InputTokens == lu.InputTokens &&
				lastUsage.CachedInputTokens == lu.CachedInputTokens &&
				lastUsage.OutputTokens == lu.OutputTokens &&
				lastUsage.ReasoningOutputTokens == lu.ReasoningOutputTokens {
				continue
			}
			lastUsage = lu
			if currentModel == "" {
				continue
			}
			ts, err := time.Parse(time.RFC3339Nano, evt.Timestamp)
			if err != nil {
				ts, err = time.Parse(time.RFC3339, evt.Timestamp)
				if err != nil {
					continue
				}
			}
			records = append(records, Record{
				Agent:           "codex",
				CacheReadTokens: lu.CachedInputTokens,
				InputTokens:     lu.InputTokens - lu.CachedInputTokens,
				Model:           currentModel,
				OutputTokens:    lu.OutputTokens,
				Project:         "",
				ReasoningTokens: lu.ReasoningOutputTokens,
				Timestamp:       ts,
			})
		}
	}
	return records, scanner.Err()
}
