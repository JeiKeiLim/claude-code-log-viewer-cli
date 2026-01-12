# Quickstart: CCLV Fixes and Enhancements

**Feature**: 002-cclv-fixes-enhancements
**Date**: 2026-01-12

## Overview

This update adds plain text output mode, fixes navigation bugs, corrects hyphen handling in paths, and adds contextual titles to the viewer.

## New Usage Patterns

### Plain Text Output

```bash
# Explicit plain mode
cclv --plain file.jsonl

# Pipe to another tool (auto-detects plain mode)
cat file.jsonl | cclv | less -R

# Force TUI even when piping
cat file.jsonl | cclv --tui
```

### Viewing with Context

When viewing logs, the title now shows:
- **Interactive mode**: "project-name - 2026-01-12 09:00"
- **File mode**: "conversation.jsonl"
- **Stdin mode**: "stdin"

## Verification Commands

### Test Plain Mode

```bash
# Should output formatted text, not enter TUI
cclv --plain ~/.claude/projects/*/conversation.jsonl | head -20

# Should work with piping
cat ~/.claude/projects/*/conversation.jsonl | cclv | wc -l
```

### Test Navigation Fix

```bash
# Launch interactive mode
cclv

# Press j/k - cursor should move exactly one item per keypress
# Press down/up arrows - same behavior
```

### Test Hyphen Handling

```bash
# If you have a project with hyphens (e.g., my-project):
cclv

# Should show "my-project" not "my/project"
```

### Test Title Display

```bash
# Interactive mode - title shows project + time
cclv

# File mode - title shows filename
cclv ~/.claude/projects/.../conversation.jsonl

# Plain mode - header shows source
cclv --plain file.jsonl | head -5
```

## Files Changed

| File | Change |
|------|--------|
| `cmd/cclv/main.go` | Add flag parsing, mode detection, plain output path |
| `internal/tui/plain.go` | NEW: Plain text renderer |
| `internal/tui/project.go` | Remove duplicate navigation handling |
| `internal/tui/conversation.go` | Remove duplicate navigation handling |
| `internal/tui/viewer.go` | Add title field and rendering |
| `internal/scanner/projects.go` | Fix DecodeProjectPath for hyphens |

## Build & Test

```bash
# Build
make build

# Run tests
make test

# Verify
./cclv --plain ~/.claude/projects/*/conversation.jsonl | head
```
