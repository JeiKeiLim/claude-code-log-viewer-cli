# Implementation Plan: Claude Code Log Viewer CLI (cclv)

**Branch**: `001-claude-log-viewer` | **Date**: 2026-01-12 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-claude-log-viewer/spec.md`

## Summary

Build a TUI application for viewing Claude Code conversation logs with dual-mode operation: interactive project browser for navigating `~/.claude/projects/` and pipeline mode for stdin/file input. Uses Go + Bubbletea for a single-binary distribution with vim-style navigation and beautiful rendering of user messages, assistant responses (text, thinking, tool_use).

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: Bubbletea (TUI), Lipgloss (styling), Bubbles (components), Glamour (markdown)
**Storage**: N/A (read-only file access to `~/.claude/projects/`)
**Testing**: Go standard testing + testify for assertions
**Target Platform**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
**Project Type**: Single CLI application
**Performance Goals**: <100ms startup, <50ms navigation, 60fps scrolling, handle 100MB files
**Constraints**: <100MB memory for typical usage, single binary, no config required
**Scale/Scope**: Local tool, single user, ~1000s of conversation files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Requirement | Status |
|-----------|-------------|--------|
| I. Single Binary Distribution | Go static compilation, cross-platform targets | ✅ PASS |
| II. Dual Mode Interface | Interactive TUI + Pipeline stdin modes | ✅ PASS |
| III. Claude Log Format Fidelity | Parse user/assistant/file-history-snapshot, render text/thinking/tool_use | ✅ PASS |
| IV. Performance & Responsiveness | <100ms startup, <50ms nav, 60fps scroll, 100MB files | ✅ PASS |
| V. Simplicity & YAGNI | Minimal deps (Charm stack only), no plugins/scripting/network | ✅ PASS |

**Result**: All gates pass. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/001-claude-log-viewer/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no API)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cmd/cclv/
└── main.go              # Entry point, mode detection

internal/
├── tui/
│   ├── app.go           # Root Bubbletea model, view routing
│   ├── project.go       # Project browser view
│   ├── conversation.go  # Conversation list view
│   ├── viewer.go        # Log viewer view
│   ├── search.go        # Search overlay component
│   └── styles.go        # Lipgloss style definitions
├── parser/
│   ├── jsonl.go         # JSONL line-by-line parsing
│   └── entry.go         # Log entry type parsing
├── types/
│   ├── project.go       # Project type
│   ├── conversation.go  # Conversation type
│   └── entry.go         # LogEntry, MessageContent types
└── scanner/
    └── projects.go      # ~/.claude/projects/ directory scanner

tests/
├── parser/              # Parser unit tests
├── scanner/             # Scanner unit tests
└── integration/         # End-to-end TUI tests
```

**Structure Decision**: Single project structure per constitution. Go's standard `cmd/` + `internal/` layout. No external API contracts needed (local file viewer only).

## Complexity Tracking

> No violations. All requirements align with constitution principles.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none) | - | - |
