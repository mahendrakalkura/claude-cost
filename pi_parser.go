package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// piEntry is one JSONL line of a pi session file. Only the fields that carry or attribute token usage are decoded.
type piEntry struct {
	CWD       string     `json:"cwd"`
	ID        string     `json:"id"`
	Message   *piMessage `json:"message"`
	Model     string     `json:"model"`
	ModelID   string     `json:"modelId"`
	Timestamp string     `json:"timestamp"`
	Type      string     `json:"type"`
	Usage     *piUsage   `json:"usage"`
}

type piMessage struct {
	Details       *piToolDetails `json:"details"`
	Model         string         `json:"model"`
	ResponseModel string         `json:"responseModel"`
	Role          string         `json:"role"`
	Usage         *piUsage       `json:"usage"`
}

// PiParser reads pi coding agent JSONL session files.
type PiParser struct{}

// piToolDetails holds the subagent metadata that subagent tools attach to their tool result.
type piToolDetails struct {
	ModelName string `json:"modelName"`
}

type piUsage struct {
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	Reasoning  int64 `json:"reasoning"`
}

// PiDirsEnv names the environment variable that overrides where the pi parser looks for session logs (OS-list-separated directories, each searched recursively for *.jsonl session files).
const PiDirsEnv = "CLAUDE_COST_PI_DIRS"

const (
	piAgentDirEnv   = "PI_CODING_AGENT_DIR"
	piEntryBranch   = "branch_summary"
	piEntryCompact  = "compaction"
	piEntryMessage  = "message"
	piEntryModel    = "model_change"
	piEntrySession  = "session"
	piEntryUsage    = "usage"
	piRoleAssistant = "assistant"
	piRoleTool      = "toolResult"
	piSessionDirEnv = "PI_CODING_AGENT_SESSION_DIR"
	piSubagentModel = "pi-subagent"
	piSuffix        = ".jsonl"
	piTasksDir      = "tasks"
)

func init() {
	Register(&PiParser{})
}

// Discover searches pi's session store, which is <agent dir>/sessions (agent dir from PI_CODING_AGENT_DIR, default ~/.pi/agent), plus PI_CODING_AGENT_SESSION_DIR when set. The walk is recursive because subagent sessions sit one level deeper, under <session>/tasks/.
func (p *PiParser) Discover() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	agentDir := filepath.Join(home, ".pi", "agent")
	if dir := os.Getenv(piAgentDirEnv); dir != "" {
		agentDir = dir
	}
	defaults := []string{filepath.Join(agentDir, "sessions")}
	if dir := os.Getenv(piSessionDirEnv); dir != "" {
		defaults = append(defaults, dir)
	}
	return discoverTree(rootsFromEnv(PiDirsEnv, defaults), piSuffix)
}

func (p *PiParser) Name() string { return "pi" }

// Parse emits one record per usage-bearing entry that pi itself counts in session totals: assistant messages, usage entries such as cache warming, the summarization usage of compaction and branch_summary entries, and the subagent usage that subagent tools report on their tool result. Tool result usage is skipped when the session has a <session>/tasks/ directory, because there the subagent sessions are stored, and counted, as their own files. Records carry the entry ID so main can drop entries that forks and clones copy into other session files.
func (p *PiParser) Parse(path string) (records []Record, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		e := f.Close()
		if e != nil && err == nil {
			err = e
		}
	}()

	currentModel := ""
	project := ""
	subagentFiles := piHasSubagentFiles(path)
	reader := bufio.NewReaderSize(f, 1024*1024)
	for {
		// ReadBytes has no line-length cap; pi lines holding tool output or images exceed any fixed scanner buffer.
		line, e := reader.ReadBytes('\n')
		if len(line) > 0 {
			entry := piEntry{}
			if json.Unmarshal(line, &entry) == nil {
				model, usage := "", (*piUsage)(nil)
				switch entry.Type {
				case piEntryBranch, piEntryCompact:
					model, usage = currentModel, entry.Usage
				case piEntryMessage:
					if entry.Message != nil && entry.Message.Role == piRoleAssistant {
						model, usage = entry.Message.ResponseModel, entry.Message.Usage
						if model == "" {
							model = entry.Message.Model
						}
						if model != "" {
							currentModel = model
						}
					}
					// The tool result names the subagent model only by display name, which pricing may not resolve.
					if entry.Message != nil && entry.Message.Role == piRoleTool && !subagentFiles {
						model, usage = piSubagentModel, entry.Message.Usage
						if entry.Message.Details != nil && entry.Message.Details.ModelName != "" {
							model = entry.Message.Details.ModelName
						}
					}
				case piEntryModel:
					if entry.ModelID != "" {
						currentModel = entry.ModelID
					}
				case piEntrySession:
					project = entry.CWD
				case piEntryUsage:
					model, usage = entry.Model, entry.Usage
				}
				record, ok := piRecord(entry, model, usage, project)
				if ok {
					records = append(records, record)
				}
			}
		}
		if errors.Is(e, io.EOF) {
			return records, nil
		}
		if e != nil {
			return records, e
		}
	}
}

// piHasSubagentFiles reports whether the session at path stores its subagent sessions as files under <session>/tasks/.
func piHasSubagentFiles(path string) bool {
	_, err := os.Stat(filepath.Join(strings.TrimSuffix(path, piSuffix), piTasksDir))
	return err == nil
}

// piRecord converts one usage-bearing entry into a Record. Pi reports reasoning as a subset of output, so it is moved out of OutputTokens to keep the cost formula from billing it twice.
func piRecord(entry piEntry, model string, usage *piUsage, project string) (Record, bool) {
	if model == "" || usage == nil {
		return Record{}, false
	}
	if usage.Input == 0 && usage.Output == 0 && usage.CacheRead == 0 && usage.CacheWrite == 0 {
		return Record{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
	if err != nil {
		return Record{}, false
	}
	reasoning := min(usage.Reasoning, usage.Output)
	record := Record{
		Agent:            "pi",
		CacheReadTokens:  usage.CacheRead,
		CacheWriteTokens: usage.CacheWrite,
		ID:               fmt.Sprintf("%s %s", entry.ID, entry.Timestamp),
		InputTokens:      usage.Input,
		Model:            model,
		OutputTokens:     usage.Output - reasoning,
		Project:          project,
		ReasoningTokens:  reasoning,
		Timestamp:        timestamp,
	}
	return record, true
}
