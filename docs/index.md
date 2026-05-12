# cclv Documentation Index

**Updated**: 2026-05-13 | **Scope**: Current source and user-facing docs

## Project Overview

| Attribute | Value |
|-----------|-------|
| **Type** | CLI/TUI |
| **Primary Language** | Go 1.25 |
| **Framework** | Charm stack: Bubble Tea, Lip Gloss, Bubbles, Glamour |
| **Architecture** | Go CLI with provider-backed internal packages |

## Quick Reference

| Metric | Value |
|--------|-------|
| **Go Files** | 56 non-test / 108 total |
| **Packages** | 15 |
| **Production LOC** | ~16,700 |
| **Total Go LOC** | ~63,500 including tests |
| **Direct Dependencies** | 9 |
| **Entry Point** | `cmd/cclv/main.go` |

### Key Commands

```bash
# Interactive mode: browse available agent providers and projects
cclv

# View a Claude Code or auto-detected JSONL file
cclv path/to/conversation.jsonl

# Force Codex JSONL parsing
cclv --agent=codex path/to/rollout.jsonl

# Pipeline mode
cat file.jsonl | cclv

# Plain text output
cclv --plain file.jsonl

# Usage limits
cclv --usage

# Version
cclv --version
```

## Provider Support

| Provider | Interactive Browse | File/stdin Pipeline | Storage |
|----------|--------------------|---------------------|---------|
| Claude Code | Yes | Yes | `~/.claude/projects/` |
| Codex | Yes | Yes | `~/.codex/sessions/` |
| OpenCode | Yes | No | `~/.local/share/opencode/opencode.db` |

OpenCode is SQLite-backed, so it is selected through interactive mode. File/stdin parsing is for Claude Code and Codex JSONL sessions.

## Generated Documentation

| Document | Description |
|----------|-------------|
| [Project Overview](./project-overview.md) | Summary, features, providers, tech stack |
| [Architecture](./architecture.md) | Current packages, data flow, runtime behavior |
| [Source Tree Analysis](./source-tree-analysis.md) | Annotated directory structure |
| [Development Guide](./development-guide.md) | Setup, build, test, and release guidance |

## Existing Documentation

| Document | Description |
|----------|-------------|
| [README.md](../README.md) | User-facing install and usage guide |
| [CLAUDE.md](../CLAUDE.md) | Development guidelines for AI agents |
| [Known Issues](./known-issues.md) | Operational issues such as usage API rate limits |
| [Lessons Learned](./lessons-learned.md) | Technical notes and debugging history |

## Feature Specifications

The `specs/` and `_bmad-output/` directories are historical planning artifacts. They are useful for context, but this `docs/` set and the source code should be treated as the current reference.

| Feature | Status |
|---------|--------|
| [001-claude-log-viewer](../specs/001-claude-log-viewer/) | Complete |
| [002-cclv-fixes-enhancements](../specs/002-cclv-fixes-enhancements/) | Complete |
| [003-ui-metadata-improvements](../specs/003-ui-metadata-improvements/) | Complete |

## Architecture At-a-Glance

```
cmd/cclv/                 CLI flags, mode selection, provider selection
internal/agent/           Provider interfaces and shared entry/session types
internal/providers/       Claude Code, Codex, and OpenCode implementations
internal/parser/          Claude Code JSONL parser
internal/scanner/         Claude Code project and conversation discovery
internal/session/         Active Claude Code session detection
internal/tui/             Bubble Tea views, dashboards, rendering
internal/token/           Token estimation service
internal/usage/           Claude usage API client and usage bar
internal/version/         Build-time version metadata
internal/watcher/         File and project watchers for live updates
```

## Navigation

| I want to... | Go to... |
|--------------|----------|
| Understand the project | [Project Overview](./project-overview.md) |
| Learn the architecture | [Architecture](./architecture.md) |
| Explore the codebase | [Source Tree Analysis](./source-tree-analysis.md) |
| Set up development | [Development Guide](./development-guide.md) |
| Read user instructions | [README.md](../README.md) |
