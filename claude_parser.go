package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	Register(&ClaudeParser{})
}

// ClaudeParser reads Claude Code JSONL session files.
type ClaudeParser struct{}

func (p *ClaudeParser) Name() string { return "claude" }

// ClaudeDirsEnv names the environment variable that overrides where the claude
// parser looks for session logs (OS-list-separated directories, each holding
// per-project subdirectories of *.jsonl files).
const ClaudeDirsEnv = "CLAUDE_COST_CLAUDE_DIRS"

func (p *ClaudeParser) Discover() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	defaults := []string{
		filepath.Join(home, ".agents", "projects"),
		filepath.Join(home, ".claude", "projects"),
	}
	return discover(rootsFromEnv(ClaudeDirsEnv, defaults), filepath.Join("*", "*.jsonl"))
}

// claudeLine represents one JSONL line from a Claude session file.
type claudeLine struct {
	CWD       string         `json:"cwd"`
	Message   *claudeMessage `json:"message"`
	SessionID string         `json:"sessionId"`
	Timestamp string         `json:"timestamp"`
	Type      string         `json:"type"`
}

type claudeMessage struct {
	Model string       `json:"model"`
	Role  string       `json:"role"`
	Usage *claudeUsage `json:"usage"`
}

type claudeUsage struct {
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
}

func (p *ClaudeParser) Parse(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	proj := projectFromDir(filepath.Dir(path))

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var entry claudeLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" || entry.Message == nil || entry.Message.Role != "assistant" {
			continue
		}
		if entry.Message.Usage == nil {
			continue
		}
		u := entry.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
			continue
		}
		model := entry.Message.Model
		if model == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, entry.Timestamp)
			if err != nil {
				continue
			}
		}
		records = append(records, Record{
			Agent:            "claude",
			CacheReadTokens:  u.CacheReadInputTokens,
			CacheWriteTokens: u.CacheCreationInputTokens,
			InputTokens:      u.InputTokens,
			Model:            model,
			OutputTokens:     u.OutputTokens,
			Project:          proj,
			Timestamp:        ts,
		})
	}
	return records, scanner.Err()
}

func projectFromDir(dir string) string {
	name := filepath.Base(dir)
	// Claude project dirs encode the path with dashes, e.g. "-home-user--agents" means ~/.agents.
	name = strings.ReplaceAll(name, "-", string(filepath.Separator))
	if strings.HasPrefix(name, string(filepath.Separator)) {
		return name
	}
	return string(filepath.Separator) + name
}
