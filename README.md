# claude-cost

A single Go CLI that scans local AI-coding-agent logs, counts tokens, applies live model pricing, and prints per-day, per-month, and per-agent cost in ASCII tables. Local-only: no web server, no account, no upload. Every run rescans your logs from disk; the only state it keeps is a cached pricing file.

## Install

```
go install github.com/mahendrakalkura/claude-cost@latest
```

Or build from source:

```
git clone https://github.com/mahendrakalkura/claude-cost
cd claude-cost
make build
```

## Usage

```
claude-cost                  # all three tables (agent, day, month)
claude-cost --agent          # per-agent only
claude-cost --day            # per-day only
claude-cost --month          # per-month only
claude-cost --day --month    # per-day then per-month
claude-cost --json           # JSON output (for scripting)
claude-cost --since 2026-01-01 --until 2026-06-30  # date range filter (inclusive)
claude-cost --no-fetch       # offline pricing only (never touch network)
claude-cost --refresh        # force re-fetch pricing now
```

The mode flags `--agent`, `--day`, and `--month` combine; with none set, all three tables print.

## Supported agents

`claude-cost` discovers session logs under your home directory automatically. No configuration is needed - parsers run for whichever agents have data on disk.

| Agent    | Source                                             |
| -------- | -------------------------------------------------- |
| claude   | `~/.claude/projects/**/*.jsonl`                    |
| codex    | `~/.codex*/sessions/**/rollout-*.jsonl`            |
| opencode | `~/.local/share/opencode/storage/message/**/*.json` |

The claude parser also reads `~/.agents/projects` (a non-default location) in addition to the standard `~/.claude/projects`. The codex parser globs every `~/.codex*` directory, so multiple Codex homes are picked up.

## Configuring log locations

If your logs live somewhere non-standard, point any parser at one or more directories with an environment variable. Multiple directories are separated by your OS path-list separator (`:` on Linux and macOS). When a variable is unset, the parser uses its built-in defaults under `$HOME`.

| Agent    | Environment variable         | Each directory holds                            |
| -------- | ---------------------------- | ----------------------------------------------- |
| claude   | `CLAUDE_COST_CLAUDE_DIRS`     | per-project subdirectories of `*.jsonl` files   |
| codex    | `CLAUDE_COST_CODEX_DIRS`      | a codex home with a `sessions/YYYY/MM/DD` tree  |
| opencode | `CLAUDE_COST_OPENCODE_DIRS`   | per-session subdirectories of `*.json` messages |

```
CLAUDE_COST_CLAUDE_DIRS=/mnt/logs/claude:/backup/claude claude-cost --agent
```

## Pricing

On each run, `claude-cost` loads model pricing from one of three sources, in priority order:

1. Disk cache at `~/.cache/claude-cost/pricing.json` (valid 24h)
2. Live fetch from the LiteLLM model prices JSON
3. Embedded offline fallback table compiled into the binary

Flags: `--no-fetch` never touches the network; `--refresh` forces an immediate re-fetch. Models with no matching price are listed in the footer and shown as `n/a` in the cost column.

## Output format

ASCII tables with `+` corners, `-` horizontals, `|` verticals, and right-aligned numeric columns. Costs are prefixed with `$`. A footer line reports how many models were priced.

```
# By Agent
+--------+------------+----------+------------+-----------+----------+
| AGENT  |      INPUT |   OUTPUT |      CACHE | REASONING | COST USD |
+--------+------------+----------+------------+-----------+----------+
| claude | 12,345,678 |  234,567 |  9,876,543 |         0 |   $42.17 |
| codex  |  1,234,567 |   45,678 |          0 |    12,345 |    $3.88 |
+--------+------------+----------+------------+-----------+----------+
| TOTAL  | 13,580,245 |  280,245 |  9,876,543 |    12,345 |   $46.05 |
+--------+------------+----------+------------+-----------+----------+

priced: 2 of 2 models
```

`--json` emits the same aggregates as indented JSON for scripting.

## Build and lint

```
make build    # go build -o main .
make run      # build + run with --no-fetch
make lint     # golangci-lint run ./...
make clean    # remove the built binary
```

## Contributing

Architecture, parser interface, and instructions for adding a new agent live in [AGENTS.md](AGENTS.md).
