# Source Tree Analysis: cclv

**Updated**: 2026-05-13

## Directory Structure

```
claude-code-log-viewer-cli/
├── cmd/
│   └── cclv/                  # CLI entry point, flag parsing, mode routing
│
├── internal/
│   ├── agent/                 # Provider interfaces and shared agent data model
│   ├── parser/                # Claude Code JSONL parser
│   ├── providers/
│   │   ├── claudecode/        # Claude Code provider adapter
│   │   ├── codex/             # Codex rollout JSONL provider
│   │   └── opencode/          # OpenCode SQLite provider
│   ├── scanner/               # Claude Code project/conversation discovery
│   ├── session/               # Active Claude Code session detection
│   ├── token/                 # Token estimation service
│   ├── tui/                   # Bubble Tea models, dashboards, renderers
│   ├── types/                 # Legacy/core Claude Code log types
│   ├── usage/                 # Claude usage API client and usage bar
│   ├── version/               # Build-time version metadata
│   └── watcher/               # File and project watchers
│
├── docs/                      # Current documentation
├── specs/                     # Historical Speckit specifications
├── _bmad/                     # BMAD workflow assets
├── _bmad-output/              # Historical planning/implementation artifacts
├── .github/workflows/         # CI and release workflows
├── .githooks/                 # Repository-managed git hooks
├── .specify/                  # Speckit configuration
├── testdata/fixtures/         # Sample Claude Code and Codex logs
├── tests/acceptance/          # Cross-package acceptance tests
├── CLAUDE.md                  # AI-agent development guidance
├── Makefile                   # Build/test/release automation
├── README.md                  # User-facing documentation
├── go.mod
└── go.sum
```

## File Statistics

| Category | Count / Size |
|----------|--------------|
| Go source files | 56 non-test / 108 total |
| Go packages | 15 |
| Production Go LOC | ~16,700 |
| Total Go LOC | ~63,500 including tests |
| Direct dependencies | 9 |

Counts were taken from tracked Go files. Untracked local tool state such as `.agents/`, `.codex/`, `.tenet/`, and generated coverage files is not part of this source-tree summary.

## Critical Directories

### `cmd/cclv/`

**Purpose**: Application entry point and mode routing.

Key responsibilities:

- CLI flag registration and help text
- TTY-based mode detection
- provider override parsing
- file/stdin parsing path
- usage/version exit paths
- interactive Bubble Tea program startup

### `internal/agent/`

**Purpose**: Provider-neutral interfaces and types.

Important files:

- `provider.go` - `AgentProvider`, `WatchableProvider`, and `SessionWatcher`
- `types.go` - provider, project, and session metadata
- `entry.go` - normalized conversation entries and content blocks
- `detect.go` - JSONL format detection for pipeline mode

### `internal/providers/`

**Purpose**: Backend-specific discovery, parsing, and watching.

| Package | Responsibility |
|---------|----------------|
| `claudecode` | Adapts Claude Code scanner/parser into the provider interface |
| `codex` | Discovers `~/.codex/sessions`, groups rollout files by `cwd`, parses Codex JSONL |
| `opencode` | Reads OpenCode projects/sessions from SQLite and watches DB-backed sessions |

### `internal/parser/`

**Purpose**: Claude Code JSONL parsing.

The parser reads line-oriented JSON, extracts message content and usage fields, tolerates malformed lines, and returns parse-error counts alongside valid entries.

### `internal/scanner/`

**Purpose**: Claude Code project discovery.

The scanner decodes Claude Code project directory names, uses filesystem validation to recover paths, scans conversations, and extracts metadata lazily.

### `internal/session/`

**Purpose**: Active Claude Code session lifecycle.

It reads `~/.claude/sessions/{pid}.json`, checks PID liveness, maps active session metadata to JSONL files, emits opened/closed events, and supports the active-session dashboard.

### `internal/tui/`

**Purpose**: All terminal UI code.

Major components:

- `app.go` - root model and view routing
- `agent_selector.go` - provider selector
- `project.go` - project list
- `conversation.go` - conversation/session list
- `viewer.go` - log viewer
- `dashboard.go` - selected-project dashboard
- `session_dashboard.go` and `session_dashboard_viewmode.go` - active Claude Code session dashboard
- `plain.go` - non-TUI rendering
- `styles.go`, `grid_layout.go`, `stringwidth.go`, `listviewport.go` - layout and rendering support

### `internal/usage/`

**Purpose**: Claude Code usage-limit monitoring.

This package reads credentials, fetches `https://api.anthropic.com/api/oauth/usage`, caches results in `~/.cache/cclv/usage.json`, renders usage-bar state, and handles rate-limit and timeout errors.

### `internal/watcher/`

**Purpose**: Live updates for file-backed sessions.

Includes append-only file watching for new complete lines and project-directory watching for latest-conversation follow mode.

## Integration Points

### File System

| Path | Purpose | Access |
|------|---------|--------|
| `~/.claude/projects/` | Claude Code project logs | Read |
| `~/.claude/sessions/` | Active Claude Code session metadata | Read/watch |
| `~/.codex/sessions/` | Codex rollout logs | Read |
| `~/.local/share/opencode/opencode.db` | OpenCode session database | Read |
| `~/.cache/cclv/usage.json` | Usage API shared cache | Read/write |
| stdin/stdout | Pipeline and plain modes | Read/write |

### Network

The core log viewer is local-first, but usage monitoring is networked. `cclv --usage` and the TUI usage bar call Anthropic's OAuth usage endpoint and cache responses to limit repeated requests.

### GitHub Actions

| Workflow | Purpose |
|----------|---------|
| `.github/workflows/ci.yml` | fmt, vet, lint, test, build |
| `.github/workflows/release.yml` | tagged release builds and archive upload |

## Generated or Local Artifacts

The repository root may contain local binaries, coverage files, and tool state during development. These are not part of the intended architecture and should not be used as source-of-truth docs.
