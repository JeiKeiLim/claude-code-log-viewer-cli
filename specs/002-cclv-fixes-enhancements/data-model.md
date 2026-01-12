# Data Model: CCLV Fixes and Enhancements

**Feature**: 002-cclv-fixes-enhancements
**Date**: 2026-01-12

## New Types

### OutputMode

Enumeration for display mode selection.

```
OutputMode:
  - TUI: Interactive terminal UI mode
  - Plain: Plain text output to stdout
```

**Usage**: Determined at startup based on flags and TTY detection.

### SourceContext

Information about what is being viewed for title display.

```
SourceContext:
  - Type: enum (Interactive, File, Stdin)
  - ProjectName: string (optional, for Interactive)
  - ConversationTime: timestamp (optional, for Interactive)
  - FilePath: string (optional, for File)
```

**Usage**: Passed to ViewerModel for title rendering.

## Modified Types

### ViewerModel

Add field for dynamic title:

```
ViewerModel:
  + title: string  # Display title based on source context
```

### CLI Flags

New command-line flags:

```
Flags:
  --plain: bool  # Force plain text output mode
  --tui: bool    # Force TUI mode (even if stdout not TTY)
```

## State Transitions

### Mode Selection Flow

```
Start
  │
  ├─ --plain flag? ─────────→ Plain Mode
  │
  ├─ --tui flag? ───────────→ TUI Mode
  │
  ├─ stdin is TTY?
  │   ├─ Yes + no args ─────→ Interactive Mode (TUI)
  │   └─ No or has args ────→ Pipeline Mode
  │       │
  │       └─ stdout is TTY?
  │           ├─ Yes ───────→ TUI Mode
  │           └─ No ────────→ Plain Mode
```

## Path Encoding/Decoding

### Claude Code Path Encoding

Encoding rules (for reference):
- `/` in path → `-` in encoded name
- `-` in path → `--` in encoded name

### DecodeProjectPath Algorithm

```
Input: encoded string (e.g., "-Users-me-my--project")
Output: decoded path (e.g., "/Users/me/my-project")

1. Replace all "--" with placeholder (e.g., "\x00")
2. Replace all "-" with "/"
3. Replace placeholder with "-"
4. Return result
```

**Note**: Current implementation has steps 2 and 3 inverted, causing the bug.
