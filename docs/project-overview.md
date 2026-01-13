# Project Overview: Claude Code Log Viewer CLI (cclv)

**Generated**: 2026-01-13 | **Scan Level**: Exhaustive | **Project Type**: CLI

## Executive Summary

`cclv` (Claude Code Log Viewer) is a terminal user interface (TUI) application for browsing and viewing Claude Code conversation logs stored in `~/.claude/projects/`. Built with Go and the Charm stack (Bubbletea, Lipgloss, Bubbles), it provides an interactive project browser, conversation timeline, and beautiful log viewer with vim-style navigation.

## Key Features

- **Interactive Project Browser** - Navigate all Claude Code projects from `~/.claude/projects/`
- **Conversation Timeline** - Browse conversations sorted by most recent
- **Beautiful Log Viewer** - View messages with syntax highlighting and proper formatting
- **Vim-style Navigation** - `j/k`, `gg/G`, `/search`, and more
- **Pipeline Mode** - Pipe JSONL logs directly: `cat file.jsonl | cclv`
- **Plain Text Output** - Export logs without TUI for scripting
- **Version Command** - `cclv --version` for version identification
- **Lazy Loading** - Progressive loading for large conversation lists and logs

## Technology Stack

| Category | Technology | Version |
|----------|------------|---------|
| Language | Go | 1.24.3 |
| TUI Framework | Bubbletea | v1.3.10 |
| Styling | Lipgloss | v1.1.1 |
| Components | Bubbles | v0.21.0 |
| Terminal | golang.org/x/term | v0.39.0 |

## Architecture Classification

| Attribute | Value |
|-----------|-------|
| Repository Type | Monolith |
| Project Type | CLI |
| Architecture Pattern | Standard Go CLI (cmd/ + internal/) |
| Build System | Makefile with cross-compilation |
| Distribution | Single static binary |

## Development History (Speckit)

This project was developed using Speckit with 3 feature iterations:

| Feature | Branch | Description |
|---------|--------|-------------|
| 001-claude-log-viewer | Initial | Core TUI implementation with project browser, conversation list, log viewer |
| 002-cclv-fixes-enhancements | Fixes | Plain text output mode, navigation bug fix, hyphen path handling |
| 003-ui-metadata-improvements | Current | Version command, UI decoration, token usage display, lazy loading |

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

# View specific file
cclv path/to/conversation.jsonl

# Pipeline mode
cat conversation.jsonl | cclv

# Plain text output
cclv --plain conversation.jsonl
```

## Documentation Index

- [Architecture](./architecture.md) - Detailed architecture documentation
- [Source Tree Analysis](./source-tree-analysis.md) - Annotated directory structure
- [Development Guide](./development-guide.md) - Setup, build, and testing instructions
- [Lessons Learned](./lessons-learned.md) - Technical insights and solutions

## Links

- **Repository**: https://github.com/JeiKeiLim/claude-code-log-viewer-cli
- **Go Module**: github.com/JeiKeiLim/claude-code-log-viewer-cli
