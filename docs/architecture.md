# Architecture Documentation: cclv

**Updated**: 2026-05-13

## Overview

`cclv` is a Go CLI/TUI application built around Bubble Tea and a provider abstraction. The command entry point chooses between interactive mode, file/stdin pipeline mode, plain-text rendering, usage reporting, and version output.

The implementation is no longer Claude Code-only. Claude Code, Codex, and OpenCode are normalized behind `internal/agent` provider interfaces, while some capabilities remain provider-specific.

## High-Level Architecture

```
cmd/cclv/main.go
  ├─ flags and mode detection
  ├─ provider override and auto-detection
  ├─ file/stdin pipeline rendering
  ├─ usage/version exit modes
  └─ interactive Bubble Tea app

internal/agent
  └─ shared provider/session/conversation interfaces

internal/providers
  ├─ claudecode -> ~/.claude/projects JSONL
  ├─ codex      -> ~/.codex/sessions rollout JSONL
  └─ opencode   -> ~/.local/share/opencode/opencode.db

internal/tui
  ├─ agent selector
  ├─ project and conversation lists
  ├─ viewer
  ├─ project dashboard
  └─ active session dashboard
```

## Entry Point: `cmd/cclv`

`cmd/cclv/main.go` owns the command-line interface and routes execution.

Responsibilities:

- Parse flags such as `--plain`, `--tui`, `--agent`, `--watch`, `--follow-latest`, `--usage`, and `--version`
- Detect stdin/stdout TTY status
- Auto-detect Claude Code versus Codex JSONL when no `--agent` override is provided
- Reject unsupported combinations such as `--agent=opencode` with file/stdin input
- Start the Bubble Tea app for interactive mode
- Render plain text or TUI viewers for file/stdin mode

Mode detection flow:

```
1. --version or -v?       -> print version and exit
2. --usage or -u?         -> fetch usage limits and exit
3. --watch requires file  -> validate file-backed watch mode
4. --follow-latest?       -> require --watch
5. --agent=opencode?      -> allow only interactive mode
6. --watch --plain?       -> streaming plain mode
7. --plain?               -> plain mode
8. --tui?                 -> force TUI mode
9. stdin TTY + no args?   -> interactive mode
10. stdout TTY?           -> pipeline TUI mode
11. otherwise             -> pipeline plain mode
```

## Provider Layer: `internal/agent`

The provider layer defines common capabilities across agent backends:

- `AgentProvider` exposes type, display metadata, availability, project discovery, session discovery, and parsing
- `WatchableProvider` is optional for providers that can watch non-file-backed sessions
- Shared `Project`, `Session`, `ConversationEntry`, and content block types allow the TUI to render different backends consistently

### Provider Capabilities

| Provider | Project Discovery | Session Discovery | Stream Parsing | Session Watching |
|----------|-------------------|-------------------|----------------|------------------|
| Claude Code | `~/.claude/projects/` | JSONL files per encoded project directory | Yes | File/project watcher |
| Codex | `~/.codex/sessions/` grouped by `cwd` | `rollout-*.jsonl` files | Yes | File watcher |
| OpenCode | SQLite database | SQLite `session` rows | No | SQLite polling watcher |

OpenCode intentionally does not implement stream parsing because no standalone JSONL stream represents a session.

## Core Packages

### `internal/parser`

Parses Claude Code JSONL into `internal/types.LogEntry` values. It is tolerant of malformed lines and tracks parse errors rather than failing the entire file when some lines are valid.

### `internal/providers/claudecode`

Adapts existing Claude Code scanner/parser behavior to the provider interface.

### `internal/providers/codex`

Discovers Codex rollout files under `~/.codex/sessions`, reads `session_meta` for `cwd` grouping, and parses Codex JSONL events into shared conversation entries.

### `internal/providers/opencode`

Reads OpenCode projects and sessions from `~/.local/share/opencode/opencode.db` using SQLite. It supports interactive browsing and provider-level watching, but not file/stdin stream parsing.

### `internal/scanner`

Handles Claude Code project discovery, encoded path decoding, birthtime detection, and conversation metadata extraction.

### `internal/session`

Detects active Claude Code sessions from `~/.claude/sessions/{pid}.json`, verifies PID liveness, maps sessions back to JSONL files, and emits lifecycle events for the active-session dashboard.

### `internal/tui`

Contains all Bubble Tea models and rendering code:

- `AgentSelectorModel`
- `ProjectModel`
- `ConversationModel`
- `ViewerModel`
- `DashboardModel`
- `SessionDashboardModel`
- plain text rendering helpers
- markdown, layout, string width, styles, and usage bar integration

### `internal/usage`

Fetches Claude Code usage limits from `https://api.anthropic.com/api/oauth/usage`, reads Claude credentials, caches results in memory and `~/.cache/cclv/usage.json`, and handles rate-limit/backoff behavior.

### `internal/watcher`

Provides append-only file watching and project-directory watching used by TUI live mode and streaming plain mode.

### `internal/token`

Estimates token usage when logs do not contain exact usage values.

### `internal/version`

Stores build-time metadata injected through Go `-ldflags`:

- `Version`
- `Commit`
- `BuildDate`

`make build`, CI, and release builds should all inject these values.

## Data Flow

### Interactive Mode

```
main.go
  -> interactiveProviders()
  -> AgentSelectorModel
  -> selected provider discovers projects
  -> ProjectModel
  -> provider discovers sessions
  -> ConversationModel or SessionDashboardModel
  -> provider/parser loads entries
  -> ViewerModel
```

Claude Code project selection opens the active-session dashboard by default. Users can press `c` in that dashboard to view the conversation list. `--no-multi-session` skips the session dashboard and goes directly to conversations.

### File/stdin Pipeline Mode

```
stdin/file
  -> --agent override OR detect first JSONL sample
  -> Claude Code parser or Codex provider parser
  -> ViewerModel or RenderPlain()
```

OpenCode is rejected in this mode because it is SQLite-backed.

### Usage Mode

```
--usage
  -> credential lookup
  -> shared cache check
  -> Anthropic OAuth usage API
  -> RenderUsagePlain()
```

## Key Design Decisions

### Provider Abstraction

Providers keep storage-specific discovery and parsing out of the TUI. This lets the UI render Claude Code, Codex, and OpenCode sessions through shared entry types while still allowing provider-specific limitations.

### File-Backed Live Mode

Claude Code and Codex sessions can be watched as append-only files. OpenCode uses provider-level watching because its source of truth is SQLite.

### Active Session Dashboard

Claude Code exposes active session metadata through `~/.claude/sessions/{pid}.json`. The dashboard combines metadata scanning, PID liveness checks, directory watching, and file watching to show currently active sessions.

### Tolerant Parsing

Log files may contain malformed or non-conversation lines. Parsers skip recoverable failures and retain valid entries.

## External Integration Points

| Integration | Purpose |
|-------------|---------|
| `~/.claude/projects/` | Claude Code project logs |
| `~/.claude/sessions/` | Active Claude Code session metadata |
| `~/.codex/sessions/` | Codex rollout logs |
| `~/.local/share/opencode/opencode.db` | OpenCode session database |
| `~/.cache/cclv/usage.json` | Shared usage API cache |
| `https://api.anthropic.com/api/oauth/usage` | Claude usage-limit endpoint |

## Error Handling

| Scenario | Handling |
|----------|----------|
| Provider storage unavailable | Hide provider from selector or show provider-specific error |
| Malformed JSONL lines | Skip line and track parse errors |
| Empty valid session | Render empty state without treating it as fatal |
| OpenCode in file/stdin mode | Fail early with a clear interactive-only message |
| Usage API rate limited | Reuse cache/last-good data when available and back off |
| Watch file truncated | Recover and continue watching |
