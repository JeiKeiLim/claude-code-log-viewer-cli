---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - '_bmad-output/implementation-artifacts/epic-7-retro-2026-01-20.md'
  - '/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/external-requests/cclv-feature-streaming-plain-mode.md'
phase: 4
status: ready
createdAt: '2026-01-20'
---

# claude-code-log-viewer-cli Phase 4 - Epic 8: Plain Mode & Output Enhancements

## Overview

This document provides the epic and story breakdown for Epic 8, addressing bug fixes and feature enhancements for plain mode output and visibility flag consistency.

## Requirements Inventory

### Functional Requirements

**FR-800: Plain Mode & Output Enhancements**
- FR-801: Fix plain mode first-line empty spaces bug
- FR-802: Make `--hide-thoughts`/`--hide-tools` show collapsed indicators (consistent with TUI toggle keys)
- FR-803: Streaming plain mode for vibe-dash integration (`--watch --plain`)

### Non-Functional Requirements

**NFR-007: Output Consistency**
- Visibility flags (`--hide-thoughts`, `--hide-tools`) must behave identically to TUI toggle keys (`t`, `i`)
- Plain mode output should have no spurious whitespace

**NFR-008: Streaming Performance**
- Streaming mode must be line-buffered for real-time output
- No perceptible delay between file update and formatted output

### FR Coverage Map

| FR | Story | Description |
|----|-------|-------------|
| FR-801 | Story 8.1 | Fix plain mode first-line empty spaces |
| FR-802 | Story 8.2 | Collapsed indicators for visibility flags |
| FR-803 | Story 8.3 | Streaming plain mode |

## Epic 8: Plain Mode & Output Enhancements

Bug fixes and feature enhancements for plain mode output, visibility flag consistency, and streaming support for external tool integration.

**FRs covered:** FR-801, FR-802, FR-803
**Standalone:** Yes - independent bug fixes and feature additions
**External Request:** vibe-dash Story 17.2 (blocked by FR-803)

---

## Story 8.1: Fix Plain Mode First-Line Empty Spaces

As a **cclv user viewing logs in plain mode**,
I want **clean output without spurious whitespace**,
So that **the output is properly formatted for terminals and pipes**.

### Acceptance Criteria

1. **AC-1: No Empty Space Lines After Header**
   - Given I run `cclv --plain file.jsonl`
   - When output is rendered
   - Then the header line `=== filename ===` is followed by a blank line (just `\n`)
   - And there are no lines filled with spaces

2. **AC-2: Consistent Spacing**
   - Given plain mode output
   - When viewed with `hexdump` or `cat -v`
   - Then lines contain only intended content (no trailing space padding)

### Technical Notes

**Root Cause:**
In `internal/tui/plain.go:25-26`:
```go
header := fmt.Sprintf("=== %s ===\n\n", source)
b.WriteString(Styles.Title.Render(header))
```

The `\n\n` is inside `Styles.Title.Render()`, causing lipgloss to pad each newline character to full width with spaces.

**Fix:**
```go
header := fmt.Sprintf("=== %s ===", source)
b.WriteString(Styles.Title.Render(header) + "\n\n")
```

Move newlines outside the styled render call.

**Files to modify:**
- `internal/tui/plain.go`

**Complexity:** Low

---

## Story 8.2: Collapsed Indicators for Visibility Flags

As a **cclv user using `--hide-thoughts` or `--hide-tools`**,
I want **to see collapsed indicators instead of empty boxes**,
So that **I know content exists but is hidden (consistent with TUI toggle behavior)**.

### Acceptance Criteria

1. **AC-1: TUI `--hide-thoughts` Shows Collapsed Indicator**
   - Given I run `cclv --hide-thoughts file.jsonl` (TUI mode)
   - When an assistant message contains thinking blocks
   - Then thinking blocks display as: `[?] [thinking collapsed]`
   - And NOT as empty space or completely removed

2. **AC-2: TUI `--hide-tools` Shows Collapsed Summary**
   - Given I run `cclv --hide-tools file.jsonl` (TUI mode)
   - When an assistant message contains tool use blocks
   - Then tool blocks display as: `[>] Tool: Read (file_path: /path/to/file...)`
   - And uses the same `formatToolSummary()` as the `i` key toggle

3. **AC-3: Plain Mode `--hide-thoughts` Shows Collapsed Indicator**
   - Given I run `cclv --plain --hide-thoughts file.jsonl`
   - When an assistant message contains thinking blocks
   - Then thinking blocks display as collapsed indicator (not empty)

4. **AC-4: Plain Mode `--hide-tools` Shows Collapsed Summary**
   - Given I run `cclv --plain --hide-tools file.jsonl`
   - When an assistant message contains tool use blocks
   - Then tool blocks display as collapsed summary (not empty)

5. **AC-5: Consistent Behavior Between Flag and Toggle**
   - Given `--hide-thoughts` flag is used
   - When compared to pressing `t` key in TUI
   - Then the visual output is identical (both show collapsed indicator)
   - Same applies for `--hide-tools` vs `i` key

6. **AC-6: Message Box Not Empty**
   - Given an assistant message with ONLY thinking/tool content
   - When `--hide-thoughts --hide-tools` is used
   - Then the message box shows collapsed indicators (not empty header-only box)

### Technical Notes

**Current behavior (wrong):**
```go
// In viewer.go renderAssistantMessage()
case types.ContentTypeThinking:
    if m.renderOpts.HideThoughts {
        continue // ← Completely removes block
    }
    parts = append(parts, m.renderThinkingBlock(content))
```

**Expected behavior:**
```go
case types.ContentTypeThinking:
    if m.renderOpts.HideThoughts {
        // Show collapsed indicator (same as when showThinking=false)
        parts = append(parts, Styles.CollapsedIndicator.Render(
            fmt.Sprintf("%s [thinking collapsed]", ThinkingIcon),
        ))
        continue
    }
    if !m.showThinking {
        parts = append(parts, Styles.CollapsedIndicator.Render(
            fmt.Sprintf("%s [thinking - press 't' to expand]", ThinkingIcon),
        ))
        continue
    }
    parts = append(parts, m.renderThinkingBlock(content))
```

For tools, reuse existing `formatToolSummary()`:
```go
case types.ContentTypeToolUse:
    if m.renderOpts.HideTools {
        summary := formatToolSummary(content.ToolName, content.ToolInput)
        parts = append(parts, Styles.ToolBlock.Render(
            header + " " + Styles.CollapsedIndicator.Render(summary),
        ))
        continue
    }
    // ... existing logic
```

**Files to modify:**
- `internal/tui/viewer.go` - TUI rendering (both dynamic and static render functions)
- `internal/tui/plain.go` - Plain mode rendering

**Complexity:** Medium

---

## Story 8.3: Streaming Plain Mode

As a **developer integrating cclv with other tools (like vibe-dash)**,
I want **`--watch --plain` to stream formatted output continuously**,
So that **new log entries appear formatted in real-time**.

### Acceptance Criteria

1. **AC-1: Watch + Plain Streams Output**
   - Given I run `cclv --watch --plain file.jsonl`
   - When the command starts
   - Then existing entries are formatted and output
   - And the process continues running (does not exit)

2. **AC-2: New Entries Appear Formatted**
   - Given `cclv --watch --plain file.jsonl` is running
   - When new entries are appended to the file
   - Then new entries appear formatted on stdout
   - And formatting is consistent with initial entries

3. **AC-3: Color Flag Works**
   - Given `--color=always` is specified with `--watch --plain`
   - When output is rendered
   - Then ANSI color codes are included

4. **AC-4: Visibility Flags Work**
   - Given `--hide-thoughts` or `--hide-tools` with `--watch --plain`
   - When output is rendered
   - Then collapsed indicators are shown (per Story 8.2)

5. **AC-5: Width Flag Works**
   - Given `--width=120` with `--watch --plain`
   - When output is rendered
   - Then text wrapping uses specified width

6. **AC-6: Line-Buffered Output**
   - Given streaming mode is active
   - When new entries are formatted
   - Then output appears immediately (not buffered until process exit)

7. **AC-7: Clean Exit**
   - Given streaming mode is running
   - When SIGINT (Ctrl+C) or SIGTERM is received
   - Then process exits cleanly with code 0

8. **AC-8: Partial Line Handling**
   - Given a file with incomplete JSONL line at end
   - When streaming
   - Then incomplete line is buffered until complete
   - And no parse errors are shown for in-progress writes

### Technical Notes

**Current behavior:**
- `--watch` launches TUI with file watching
- `--plain` outputs formatted text and exits
- `--watch --plain` behaves like `--plain` (no streaming)

**Expected behavior:**
- `--watch --plain` outputs formatted text and continues watching
- Uses existing file watcher from `internal/watcher/`
- Outputs new entries as they're detected

**Implementation approach:**
```go
// In main.go, when both --watch and --plain are set:
if *watchFlag && *plainFlag {
    return runStreamingPlainMode(source, opts)
}

func runStreamingPlainMode(source string, opts tui.RenderOptions) error {
    // 1. Parse and render existing entries
    entries, _ := parser.ParseFile(source)
    fmt.Print(tui.RenderPlain(entries, source, opts))

    // 2. Start file watcher
    w, _ := watcher.New(source)

    // 3. Handle new entries
    for {
        select {
        case entries := <-w.NewEntries():
            for _, entry := range entries {
                fmt.Print(tui.RenderEntryPlain(entry, opts))
            }
        case <-signals:
            return nil
        }
    }
}
```

**Line buffering:**
```go
// Ensure stdout is line-buffered
os.Stdout.Sync() // After each entry
```

**Files to modify:**
- `cmd/cclv/main.go` - Add streaming mode detection and orchestration
- `internal/tui/plain.go` - Add `RenderEntryPlain()` for single-entry rendering

**External reference:**
- vibe-dash feature request: `/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/external-requests/cclv-feature-streaming-plain-mode.md`

**Complexity:** Medium-High

---

## Implementation Order

| Order | Story | Dependency | Effort |
|-------|-------|------------|--------|
| 1 | 8.1 | None | Low |
| 2 | 8.2 | None | Medium |
| 3 | 8.3 | 8.2 (for visibility flags) | Medium-High |

Stories 8.1 and 8.2 can be done in parallel. Story 8.3 depends on 8.2 for proper visibility flag handling in streaming mode.

---

## Definition of Done

- [ ] All stories implemented and tested
- [ ] Plain mode output has no spurious whitespace (8.1)
- [ ] `--hide-thoughts`/`--hide-tools` show collapsed indicators in both TUI and plain mode (8.2)
- [ ] `cclv --watch --plain` streams formatted output continuously (8.3)
- [ ] vibe-dash can integrate with streaming mode
- [ ] Test coverage maintained at 90%+
- [ ] Code review completed

---

## References

- [Epic 7 Retrospective](_bmad-output/implementation-artifacts/epic-7-retro-2026-01-20.md) - Action items
- [vibe-dash Feature Request](/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/external-requests/cclv-feature-streaming-plain-mode.md)
- [project-context.md](_bmad-output/project-context.md) - Critical rules
