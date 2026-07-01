# AGENTS.md

Guidance for AI agents and human contributors working on this codebase.

## What this project is

A Go CLI that scans local AI-coding-agent session logs and reports token usage and cost. It answers one question: "how much did each agent cost me?" The tool is stateless - every run rescans logs from disk and prints. The only persistent file is the cached LiteLLM pricing JSON at `~/.cache/claude-cost/pricing.json`.

The Go module path is `github.com/mahendrakalkura/claude-cost`, so the binary installs via `go install github.com/mahendrakalkura/claude-cost@latest`.

## Commands

```
make build    # go build -o main .
make run      # build + run with --no-fetch (uses cached/fallback pricing)
make lint     # golangci-lint run ./...
make clean    # remove the built binary (via gio trash)
```

Always run `make build` and `make lint` before declaring a change done.

## Layout

All Go files live in the repository root under `package main`. There are no subdirectories.

```
main.go               CLI entry, flag parsing, parallel file parse, dispatch to views
agent.go              Record type, Parser interface, global registry
claude_parser.go      Claude Code JSONL parser (~/.claude*/projects, ~/.agents/projects, $CLAUDE_CONFIG_DIR)
codex_parser.go       Codex CLI rollout JSONL parser (~/.codex*/sessions)
opencode_parser.go    Opencode JSON message parser (~/.local/share/opencode)
pricing.go            ModelPrice, Provider, Lookup, Cost, cache logic
litellm.go            Live fetch from the LiteLLM model prices JSON
fallback.go           //go:embed embedded offline pricing table
pricing_data.json     Embedded pricing data (compiled into the binary)
report.go             Aggregation: by-agent, by-day, by-month summaries
render.go             ASCII table rendering via go-pretty, JSON output
```

## Key types

- `Record` - normalized per-message token usage (agent, model, input/output/cache/reasoning tokens, timestamp, project)
- `ModelPrice` - per-token costs for a model (input, output, cache read, cache write)
- `Provider` - pricing source with cache, fetch, and fallback chain
- `Results` - aggregate views (Agents, Days, Months summaries plus priced/total counts)

## Parser interface

Each agent implements:

```go
type Parser interface {
    Name() string                          // agent name, used as the registry key
    Discover() ([]string, error)           // find candidate files on disk
    Parse(path string) ([]Record, error)   // extract records from a file
}
```

A parser registers itself in `init()` by calling `Register(&fooParser{})`. The global `All()` returns every registered parser sorted by name. `main.go` discovers all files across all parsers, then parses them in parallel (one goroutine per file, bounded by `runtime.NumCPU()`), writing into per-job slots so the merged output stays deterministic.

## Log discovery and configuration

`Discover()` builds its search roots from two sources via the shared helpers in `agent.go`:

- `rootsFromEnv(name, fallback)` returns the OS-list-separated directories in the named environment variable, or `fallback` when it is unset or empty.
- `discover(roots, tail)` globs the `tail` pattern under each root and returns every matching file, de-duplicated by resolved real path (`filepath.EvalSymlinks`) so roots that symlink to the same store are not counted twice.

Each parser exposes an environment variable so a user can point it at custom or multiple log locations without code changes (entries are separated by the OS path-list separator, `:` on Unix):

+----------+----------------------------+-------------------------------------------------+
| Agent    | Env var                    | What each directory should contain              |
+----------+----------------------------+-------------------------------------------------+
| claude   | CLAUDE_COST_CLAUDE_DIRS    | per-project subdirectories of *.jsonl files     |
| codex    | CLAUDE_COST_CODEX_DIRS      | a codex home with a sessions/YYYY/MM/DD tree    |
| opencode | CLAUDE_COST_OPENCODE_DIRS  | per-session subdirectories of *.json messages   |
+----------+----------------------------+-------------------------------------------------+

When a variable is unset, the parser falls back to its built-in default roots under `$HOME`. The constants are `ClaudeDirsEnv`, `CodexDirsEnv`, and `OpencodeDirsEnv`. The claude parser builds its defaults by globbing every `~/.claude*` home (plus `~/.agents`) and appending `projects`, and additionally honors `CLAUDE_CONFIG_DIR` by adding `$CLAUDE_CONFIG_DIR/projects`. Because homes often symlink to a shared store, `discover()` de-duplicates by real path.

## Adding a new agent

1. Create `<agent>_parser.go` in the repository root.
2. Implement the `Parser` interface (`Name`, `Discover`, `Parse`).
3. Register it in `init()` with `Register(&<Agent>Parser{})`.
4. In `Discover`, compute default roots under `os.UserHomeDir()`, expose a `<Agent>DirsEnv` constant, and return `discover(rootsFromEnv(<Agent>DirsEnv, defaults), tail)` - never hardcode an absolute path or username.
5. In `Parse`, emit one `Record` per assistant message that has non-zero usage and a non-empty model. Skip non-assistant entries and zero-usage rows.
6. Rebuild with `make build` and confirm the new agent appears in the by-agent table.

## Pricing flow

1. `NewProvider(refresh, noFetch)` tries: cache -> live fetch -> cache again -> embedded fallback.
2. `Lookup(modelID)` tries: exact match -> case-insensitive -> strip provider prefix (e.g. `google/gemini-2.5-pro` -> `gemini-2.5-pro`).
3. `Cost(price, tokens...)` sums each token bucket against its per-token cost.

Cost formula:

```
cost = input_tokens        * input_cost_per_token
     + output_tokens       * output_cost_per_token
     + cache_read_tokens   * cache_read_input_token_cost
     + cache_write_tokens  * cache_creation_input_token_cost
     + reasoning_tokens    * output_cost_per_token
```

## Token counting policy

Trust the provider-reported usage counts from agent logs. Never re-tokenize text. Re-tokenizing cannot recover cache hits and would disagree with the actual bill.

## Code conventions

- Alphabetical ordering for all declarations (types, constants, variables, functions) and for imports within a group.
- Stdlib only, except `github.com/jedib0t/go-pretty/v6` for table rendering.
- ASCII-only output: `+` corners, `-` horizontals, `|` verticals. No Unicode box-drawing characters.
- Right-align numeric columns; prefix costs with `$`.
- One paragraph is one line. Never hard-wrap text in source comments, docs, or output.
- Never hardcode a home directory or username; always resolve paths relative to `os.UserHomeDir()`.

## Dependencies

- `github.com/jedib0t/go-pretty/v6` - ASCII table rendering
- Go standard library otherwise (`net/http`, `encoding/json`, `embed`)
