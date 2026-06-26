package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func init() {
	Register(&OpencodeParser{})
}

// OpencodeParser reads opencode storage/message JSON files.
type OpencodeParser struct{}

func (p *OpencodeParser) Name() string { return "opencode" }

// OpencodeDirsEnv names the environment variable that overrides where the
// opencode parser looks for messages (OS-list-separated directories, each holding
// per-session subdirectories of *.json message files).
const OpencodeDirsEnv = "CLAUDE_COST_OPENCODE_DIRS"

func (p *OpencodeParser) Discover() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	defaults := []string{filepath.Join(home, ".local", "share", "opencode", "storage", "message")}
	return discover(rootsFromEnv(OpencodeDirsEnv, defaults), filepath.Join("*", "*.json"))
}

type opencodeMessage struct {
	ModelID string       `json:"modelID"`
	Role    string       `json:"role"`
	Time    *opencodeTime `json:"time"`
	Tokens  *opencodeTokens `json:"tokens"`
}

type opencodeTime struct {
	Created int64 `json:"created"`
}

type opencodeTokens struct {
	Cache      *opencodeCache `json:"cache"`
	Input      int64          `json:"input"`
	Output     int64          `json:"output"`
	Reasoning  int64          `json:"reasoning"`
}

type opencodeCache struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

func (p *OpencodeParser) Parse(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var msg opencodeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if msg.Role != "assistant" {
		return nil, nil
	}
	if msg.Tokens == nil {
		return nil, nil
	}
	if msg.Tokens.Input == 0 && msg.Tokens.Output == 0 && msg.Tokens.Reasoning == 0 {
		return nil, nil
	}
	model := msg.ModelID
	if model == "" {
		return nil, nil
	}
	var ts time.Time
	if msg.Time != nil {
		ts = time.UnixMilli(msg.Time.Created)
	} else {
		ts = time.Time{}
	}
	var cacheRead, cacheWrite int64
	if msg.Tokens.Cache != nil {
		cacheRead = msg.Tokens.Cache.Read
		cacheWrite = msg.Tokens.Cache.Write
	}
	return []Record{{
		Agent:            "opencode",
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
		InputTokens:      msg.Tokens.Input,
		Model:            model,
		OutputTokens:     msg.Tokens.Output,
		Project:          "",
		ReasoningTokens:  msg.Tokens.Reasoning,
		Timestamp:        ts,
	}}, nil
}
