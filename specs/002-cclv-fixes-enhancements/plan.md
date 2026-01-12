# Implementation Plan: CCLV Fixes and Enhancements

**Branch**: `002-cclv-fixes-enhancements` | **Date**: 2026-01-12 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-cclv-fixes-enhancements/spec.md`

## Summary

This plan addresses four improvements to cclv: (1) adding plain text output mode for pipeline compatibility, (2) fixing the navigation double-skip bug, (3) correcting hyphen handling in project path decoding, and (4) adding contextual information to the viewer title. All changes are bug fixes or enhancements to existing functionality with no new dependencies required.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: Bubbletea, Lipgloss, Bubbles (existing)
**Storage**: N/A (local file reading only)
**Testing**: Go standard testing
**Target Platform**: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
**Project Type**: Single CLI application
**Performance Goals**: <100ms startup, <50ms navigation latency (per Constitution)
**Constraints**: <100MB memory, single binary distribution
**Scale/Scope**: Local tool, typical log files <10MB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Binary Distribution | PASS | No new dependencies, maintains single binary |
| II. Dual Mode Interface | PASS | Enhances pipeline mode with plain text output |
| III. Claude Log Format Fidelity | PASS | Fixes hyphen decoding for accurate path display |
| IV. Performance & Responsiveness | PASS | Bug fix improves navigation responsiveness |
| V. Simplicity & YAGNI | PASS | All changes are focused bug fixes/enhancements |

**Gate Status**: PASSED - All principles satisfied

## Project Structure

### Documentation (this feature)

```text
specs/002-cclv-fixes-enhancements/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cmd/cclv/
└── main.go              # Entry point, flag parsing, mode detection

internal/
├── tui/
│   ├── app.go           # Root application model
│   ├── project.go       # Project browser (navigation fix)
│   ├── conversation.go  # Conversation list (navigation fix)
│   ├── viewer.go        # Log viewer (title enhancement)
│   ├── plain.go         # NEW: Plain text renderer
│   └── styles.go        # Styling
├── parser/
│   └── jsonl.go         # JSONL parsing
├── scanner/
│   └── projects.go      # Path decoding fix
└── types/
    └── entry.go         # Log entry types
```

**Structure Decision**: Existing single-project CLI structure. Adding one new file (`plain.go`) for plain text rendering. All other changes are modifications to existing files.

## Complexity Tracking

No constitution violations. All changes are straightforward bug fixes and enhancements within the existing architecture.
