# Implementation Plan: UI Decoration and Metadata Improvements

**Branch**: `003-ui-metadata-improvements` | **Date**: 2026-01-13 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-ui-metadata-improvements/spec.md`

## Summary

This feature enhances the cclv CLI tool with: (1) `--version` flag for version identification, (2) improved visual decoration for project and conversation lists using box-drawing characters, (3) token usage and metadata display extracted from Claude Code JSONL logs, and (4) lazy loading for large conversation lists (>50 items) and log views (>100 messages).

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: Bubbletea, Lipgloss, Bubbles (existing Charm stack)
**Storage**: N/A (local file reading only)
**Testing**: Go standard testing + testify for assertions
**Target Platform**: Cross-platform CLI (darwin/linux amd64/arm64)
**Project Type**: Single CLI application
**Performance Goals**: <100ms startup, <50ms navigation, 60fps scrolling (per Constitution IV)
**Constraints**: <100MB memory, single binary, no emojis in decorations
**Scale/Scope**: Support log files up to 100MB, projects with 1000+ conversations

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design Check (Phase 0)

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Binary Distribution | PASS | No new dependencies, version embedded at build time |
| II. Dual Mode Interface | PASS | Version flag exits before TUI/pipeline mode decision |
| III. Claude Log Format Fidelity | PASS | Token usage extracted from existing `message.usage` field |
| IV. Performance & Responsiveness | PASS | Lazy loading thresholds (50/100) prevent memory bloat |
| V. Simplicity & YAGNI | PASS | No new config options, decorations use existing Lipgloss styles |

**Gate Result**: PASS - All principles satisfied. No violations requiring justification.

### Post-Design Check (Phase 1)

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Binary Distribution | PASS | New `internal/version` package uses only std library; no external deps added |
| II. Dual Mode Interface | PASS | Version flag handled in main.go before mode detection; both modes benefit from metadata |
| III. Claude Log Format Fidelity | PASS | Data model correctly captures `message.usage` and `message.model` fields |
| IV. Performance & Responsiveness | PASS | Lazy loading config (50/100 thresholds) documented in data-model.md |
| V. Simplicity & YAGNI | PASS | Custom comma formatter avoids golang.org/x/text dependency; no config DSL |

**Post-Design Gate Result**: PASS - Design artifacts align with all constitutional principles.

## Project Structure

### Documentation (this feature)

```text
specs/003-ui-metadata-improvements/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no APIs)
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/cclv/
└── main.go              # Add --version flag handling

internal/
├── version/
│   └── version.go       # NEW: Version info with build-time variables
├── types/
│   ├── entry.go         # Add Usage struct, Model field
│   └── conversation.go  # Add TokenUsage, Model, Duration fields
├── parser/
│   ├── entry.go         # Parse usage and model from assistant messages
│   └── jsonl.go         # Add metadata extraction during parse
├── tui/
│   ├── styles.go        # Add border/separator styles, list decoration styles
│   ├── project.go       # Add borders, header with count, enhanced delegate
│   ├── conversation.go  # Add borders, header with count, token display, lazy loading
│   └── viewer.go        # Add model display in header, lazy loading for messages
└── scanner/
    └── projects.go      # Add conversation metadata extraction
```

**Structure Decision**: Single project structure maintained. New `internal/version` package added for clean version management. All other changes enhance existing packages.

## Complexity Tracking

> No violations requiring justification. All changes align with existing architecture.

| Area | Complexity | Justification |
|------|------------|---------------|
| Version package | Low | Standard Go pattern for build-time version injection |
| Lazy loading | Medium | Required for performance (Constitution IV), uses existing Bubbletea patterns |
| Border decoration | Low | Uses existing Lipgloss border primitives |
