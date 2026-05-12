# Project Overview: cclv

**Updated**: 2026-05-13 | **Project Type**: CLI/TUI

## Executive Summary

`cclv` is a terminal user interface for browsing and viewing coding-agent sessions. It remains Claude Code-first by name and history, but the current implementation supports Claude Code, Codex, and OpenCode providers through a shared provider layer.

The app supports interactive browsing, file/stdin viewing, plain-text export, live watching for file-backed sessions, active Claude Code session dashboards, token statistics, and Claude usage-limit monitoring.

## Key Features

- **Provider Browser** - Select available Claude Code, Codex, and OpenCode data sources
- **Project and Session Navigation** - Browse projects, conversation lists, and active sessions
- **Log Viewer** - Render messages with markdown, wrapping, search, line navigation, and collapsible blocks
- **Dashboard Views** - Monitor selected projects or active Claude Code sessions in grid/single-session layouts
- **Live Watching** - Follow growing JSONL sessions and optionally follow the newest conversation
- **Plain Text Mode** - Export formatted logs for scripts, pagers, or other tools
- **Token Statistics** - Display logged or estimated message/conversation token usage
- **Usage Limit Monitor** - Fetch Claude Code subscription usage from Anthropic's OAuth usage API
- **CJK Support** - Measure and render wide characters correctly

## Provider Support

| Provider | Interactive Browse | File/stdin Pipeline | Live Behavior | Storage |
|----------|--------------------|---------------------|---------------|---------|
| Claude Code | Yes | Yes | File/project watchers plus active-session dashboard | `~/.claude/projects/` and `~/.claude/sessions/` |
| Codex | Yes | Yes | File watcher for rollout JSONL | `~/.codex/sessions/` |
| OpenCode | Yes | No | SQLite polling watcher in interactive mode | `~/.local/share/opencode/opencode.db` |

OpenCode does not support file/stdin pipeline mode because sessions are stored in SQLite, not append-only JSONL files.

## Technology Stack

| Category | Technology | Version |
|----------|------------|---------|
| Language | Go | 1.25 |
| TUI Framework | Bubble Tea | v1.3.10 |
| Styling | Lip Gloss | v1.1.1 pre-release |
| Components | Bubbles | v0.21.0 |
| Markdown | Glamour | v0.10.0 |
| File Watching | fsnotify | v1.9.0 |
| Token Counting | tiktoken-go | v0.1.8 |
| SQLite | modernc.org/sqlite | v1.50.0 |

## Architecture Classification

| Attribute | Value |
|-----------|-------|
| Repository Type | Monolith |
| Project Type | CLI/TUI |
| Architecture Pattern | `cmd/` entry point with `internal/` packages |
| Provider Model | Shared `internal/agent` interfaces with provider implementations |
| Build System | Makefile plus GitHub Actions |
| Distribution | Single binary release archives |

## Quick Start

```bash
# Install
go install github.com/JeiKeiLim/claude-code-log-viewer-cli/cmd/cclv@latest

# Or build from source
git clone https://github.com/JeiKeiLim/claude-code-log-viewer-cli.git
cd claude-code-log-viewer-cli
make build

# Run interactive mode
cclv

# View a specific file
cclv path/to/conversation.jsonl

# Force Codex parsing
cclv --agent=codex path/to/rollout.jsonl

# Plain text output
cclv --plain conversation.jsonl
```

## Documentation Index

- [Architecture](./architecture.md) - Current architecture and data flow
- [Source Tree Analysis](./source-tree-analysis.md) - Annotated directory structure
- [Development Guide](./development-guide.md) - Setup, build, and testing instructions
- [Known Issues](./known-issues.md) - Operational caveats
- [Lessons Learned](./lessons-learned.md) - Technical notes and solutions

## Links

- **Repository**: https://github.com/JeiKeiLim/claude-code-log-viewer-cli
- **Go Module**: github.com/JeiKeiLim/claude-code-log-viewer-cli
