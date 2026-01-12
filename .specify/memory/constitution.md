<!--
  Sync Impact Report
  ==================
  Version change: 0.0.0 → 1.0.0

  Modified principles: N/A (initial constitution)

  Added sections:
  - Core Principles (5 principles)
  - Technology Stack
  - Development Workflow
  - Governance

  Removed sections: N/A (initial constitution)

  Templates requiring updates:
  - .specify/templates/plan-template.md: ✅ compatible (uses generic Language/Version fields)
  - .specify/templates/spec-template.md: ✅ compatible (technology-agnostic)
  - .specify/templates/tasks-template.md: ✅ compatible (uses src/ structure)

  Follow-up TODOs: None
-->

# CCLV Constitution

## Core Principles

### I. Single Binary Distribution

All releases MUST produce a single, self-contained binary with zero runtime dependencies.

- Go's static compilation MUST be used to produce portable executables
- Cross-compilation MUST target at minimum: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- Users MUST be able to install via `go install` or direct binary download
- No configuration files required for basic operation

**Rationale**: CLI tools live or die by installation friction. A single `curl | tar` or `go install` command is the gold standard.

### II. Dual Mode Interface

The application MUST support two distinct operational modes with consistent behavior.

**Interactive Mode (TUI)**:
- Launched with no arguments or directory path: `cclv` or `cclv ~/.claude/projects/`
- Full terminal UI with project browser, conversation list, and log viewer
- Vim keybindings MUST be primary navigation (hjkl, gg, G, /, etc.)
- Arrow keys and standard keys MUST work as fallback

**Pipeline Mode (Stdin)**:
- Activated when stdin is not a TTY or explicit file argument: `cat file.jsonl | cclv` or `cclv file.jsonl`
- Renders JSONL content as formatted, scrollable output
- MUST gracefully handle streaming input (tail -f compatibility)

**Rationale**: Unix philosophy - do one thing well, but integrate seamlessly into pipelines.

### III. Claude Log Format Fidelity

The viewer MUST accurately parse and beautifully render Claude Code's JSONL log structure.

- MUST handle all entry types: `user`, `assistant`, `file-history-snapshot`
- MUST render assistant content types: `text`, `thinking`, `tool_use`
- MUST preserve conversation threading via `parentUuid` chains
- Thinking blocks SHOULD be collapsible/expandable
- Tool use MUST show tool name, inputs, and distinguish from text output
- Timestamps MUST be displayed in local timezone with configurable format

**Rationale**: The whole point is to make Claude logs readable. Fidelity to the format is non-negotiable.

### IV. Performance & Responsiveness

The application MUST feel instant and handle large log files gracefully.

- Startup time MUST be <100ms for interactive mode
- Navigation between views MUST be <50ms perceived latency
- Log files up to 100MB MUST be viewable without loading entire file into memory
- Scrolling MUST maintain 60fps even with syntax-highlighted content
- Memory usage MUST stay below 100MB for typical usage (files <10MB)

**Rationale**: Slow tools don't get used. Performance is a feature.

### V. Simplicity & YAGNI

Complexity MUST be justified. Every feature earns its place.

- No plugin system, no scripting, no configuration DSL
- Configuration limited to: color theme (light/dark/auto), keybinding preset (vim/emacs/arrows), timestamp format
- No network features - this is a local log viewer only
- Dependencies MUST be minimal: Charm stack (bubbletea, lipgloss, bubbles) + standard library
- Code MUST be readable by a Go beginner within 30 minutes

**Rationale**: Scope creep kills projects. A focused tool that works beats a flexible tool that doesn't ship.

## Technology Stack

**Language**: Go 1.21+
**TUI Framework**: Bubbletea (Charm ecosystem)
**Styling**: Lipgloss
**Components**: Bubbles (list, viewport, textinput)
**Markdown Rendering**: Glamour (for rendering Claude's markdown responses)
**Testing**: Go standard testing + testify for assertions
**Build**: Standard `go build`, goreleaser for releases

**Project Structure**:
```
cmd/cclv/           # Main entry point
internal/
  tui/              # Bubbletea models and views
    app.go          # Root application model
    project.go      # Project browser view
    conversation.go # Conversation list view
    viewer.go       # Log viewer view
  parser/           # JSONL parsing
  types/            # Shared types for log entries
```

## Development Workflow

### Code Quality Gates

- All code MUST pass `go vet` and `staticcheck`
- All code MUST be formatted with `gofmt`
- All exported functions MUST have doc comments
- Test coverage target: 70% for parser, 50% overall

### Commit Conventions

- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`
- Each commit SHOULD be independently buildable
- Breaking changes MUST be marked with `!` suffix: `feat!:`

### Release Process

- Semantic versioning: MAJOR.MINOR.PATCH
- MAJOR: Breaking CLI interface changes
- MINOR: New features, backward compatible
- PATCH: Bug fixes only
- Releases automated via goreleaser on tagged commits

## Governance

This constitution defines the boundaries of the CCLV project.

- **Scope Lock**: Features outside the core mission (viewing Claude logs) require explicit justification against Principle V
- **Dependency Approval**: New dependencies require review against Principle V
- **Amendment Process**: Constitution changes require updating this document with rationale

All implementation decisions SHOULD reference the relevant principle when non-obvious.

**Version**: 1.0.0 | **Ratified**: 2026-01-12 | **Last Amended**: 2026-01-12
