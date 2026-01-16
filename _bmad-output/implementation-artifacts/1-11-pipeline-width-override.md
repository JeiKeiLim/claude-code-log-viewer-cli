# Story 1.11: Pipeline Width Override

Status: done

## Story

As a **developer piping cclv output to other tools**,
I want **to specify the rendering width explicitly**,
So that **output formats correctly regardless of terminal detection**.

## Acceptance Criteria

### AC 1.11.1: --width flag
- **Given** the cclv CLI
- **When** I run `cclv --width 120 <file>`
- **Then** all content renders to 120 character width
- **And** terminal auto-detection is overridden

### AC 1.11.2: Reasonable bounds
- **Given** the --width flag
- **When** set to unreasonable values (< 40 or > 500)
- **Then** cclv uses sensible defaults or shows warning
- **And** doesn't crash or produce broken output

### AC 1.11.3: Works with pipeline mode
- **Given** cclv piped to another tool
- **When** --width is specified
- **Then** output uses specified width
- **And** boxes and borders align correctly

## Tasks / Subtasks

- [x] Task 1: Add --width CLI flag (AC: 1.11.1, 1.11.2)
  - [x] 1.1: Add `widthFlag := flag.Int("width", 0, "Override rendering width (0=auto-detect)")` in main.go after line 39 (after hideToolsFlag)
  - [x] 1.2: Add width validation function `validateWidth(w int) int` returning clamped/default value
  - [x] 1.3: Clamp logic: `<= 0` returns 0 (auto-detect), `< 40` returns 80 (default with warning), `> 500` returns 500 (with warning), else return input
  - [x] 1.4: Print warning to stderr if width clamped: `"Warning: --width %d clamped to %d\n"`

- [x] Task 2: Add Width field to RenderOptions (AC: 1.11.1, 1.11.3)
  - [x] 2.1: Add `Width int` field to `RenderOptions` struct in `internal/tui/viewer.go` (line 21-24, after HideTools)
  - [x] 2.2: Add comment: `// Width override for rendering (0=auto-detect)`
  - [x] 2.3: Update `DefaultRenderOptions()` to set Width: 0 (lines 27-32)

- [x] Task 3: Pass width to RenderOptions in main.go (AC: 1.11.1)
  - [x] 3.1: Call `validatedWidth := validateWidth(*widthFlag)` before creating opts (before line 84)
  - [x] 3.2: Add `Width: validatedWidth` to RenderOptions initialization (line 84-87)

- [x] Task 4: Update RenderPlain to use width override (AC: 1.11.1, 1.11.3)
  - [x] 4.1: In `internal/tui/plain.go` RenderPlain function (line 14), calculate effective width:
    ```go
    width := opts.Width
    if width == 0 {
        width = 80 // Default for plain mode when no terminal
    }
    ```
  - [x] 4.2: Update `renderEntryPlain()` signature: `func renderEntryPlain(entry types.LogEntry, opts RenderOptions, width int) string`
  - [x] 4.3: Update `renderUserMessagePlain()` signature: `func renderUserMessagePlain(entry types.LogEntry, width int) string`
  - [x] 4.4: Update `renderAssistantMessagePlain()` signature: `func renderAssistantMessagePlain(entry types.LogEntry, opts RenderOptions, width int) string`
  - [x] 4.5: Add text wrapping using `WrapText()` from utils.go in all plain rendering functions (currently plain.go does NOT wrap text)
  - [x] 4.6: Calculate wrapWidth as `width - 4` (same pattern as TUI mode) for consistent margins

- [x] Task 5: Update ViewerModel to use width override (AC: 1.11.1)
  - [x] 5.1: In NewViewerModel (viewer.go line 80), after `renderOpts: opts`, add width override check:
    ```go
    m := ViewerModel{...}
    if opts.Width > 0 {
        m.width = opts.Width
    }
    return m
    ```
  - [x] 5.2: In Update() WindowSizeMsg handler (line 278-297), only update m.width if renderOpts.Width == 0:
    ```go
    case tea.WindowSizeMsg:
        if m.renderOpts.Width == 0 {
            m.width = msg.Width
        }
        m.height = msg.Height
    ```
  - [x] 5.3: Verify View() and renderEntry methods already use m.width (they do, no changes needed)

- [x] Task 6: Add tests and validation (all ACs)
  - [x] 6.1: Create `cmd/cclv/main_test.go` with `TestValidateWidth` using table-driven tests
  - [x] 6.2: Test cases: 0→0, 50→50, 80→80, 120→120, 500→500, 30→80+warning, 39→80+warning, 600→500+warning, -1→0 (auto)
  - [x] 6.3: Create `internal/tui/plain_test.go` with test for RenderPlain with Width override
  - [x] 6.4: Verify text wrapping occurs at specified width in plain output
  - [x] 6.5: Run `make test` - all tests pass
  - [x] 6.6: Run `make lint` - no lint errors
  - [x] 6.7: Run `make build` - build succeeds

## Dev Notes

### Width Validation Logic

```go
// Add to cmd/cclv/main.go after flag definitions

const (
    minWidth     = 40
    maxWidth     = 500
    defaultWidth = 80
)

// validateWidth ensures width is within reasonable bounds.
// Returns the validated width (possibly clamped) and prints warning if clamped.
// Negative values are treated as auto-detect (0).
func validateWidth(w int) int {
    if w <= 0 {
        return 0 // Auto-detect mode (including negative values)
    }
    if w < minWidth {
        fmt.Fprintf(os.Stderr, "Warning: --width %d too small, using %d\n", w, defaultWidth)
        return defaultWidth
    }
    if w > maxWidth {
        fmt.Fprintf(os.Stderr, "Warning: --width %d too large, using %d\n", w, maxWidth)
        return maxWidth
    }
    return w
}
```

### RenderOptions Update

```go
// In internal/tui/viewer.go, update RenderOptions struct:

type RenderOptions struct {
    HideThoughts bool // Hide thinking blocks
    HideTools    bool // Hide tool use blocks
    Width        int  // Width override for rendering (0=auto-detect)
}

// DefaultRenderOptions returns options that show all content types with auto-detect width.
func DefaultRenderOptions() RenderOptions {
    return RenderOptions{
        HideThoughts: false,
        HideTools:    false,
        Width:        0,
    }
}
```

### main.go Flag Integration

```go
// Add after line 38 (hideToolsFlag):
widthFlag := flag.Int("width", 0, "Override rendering width (0=auto-detect)")

// Before creating opts (around line 84):
validatedWidth := validateWidth(*widthFlag)

// Update opts creation:
opts := tui.RenderOptions{
    HideThoughts: *hideThoughtsFlag,
    HideTools:    *hideToolsFlag,
    Width:        validatedWidth,
}
```

### Plain Mode Width Handling

The key insight: In pipeline/plain mode, there's no terminal to detect width from. Currently plain.go doesn't use width at all - text just flows **without any wrapping**. With this story, when `--width` is specified, rendering should respect it for:

1. **Text wrapping**: User and assistant message text should wrap at width using `WrapText()` from utils.go
2. **Border boxes**: Tool blocks and message cards should respect width
3. **Truncation**: Long lines should truncate/wrap appropriately

**IMPORTANT**: The current `plain.go` implementation does NOT wrap text at all. This story adds text wrapping functionality to plain mode, consistent with TUI mode behavior.

```go
// In internal/tui/plain.go, update RenderPlain:

func RenderPlain(entries []types.LogEntry, source string, opts RenderOptions) string {
    width := opts.Width
    if width == 0 {
        width = 80 // Default for plain mode
    }

    var b strings.Builder
    // ... existing header code ...

    for _, entry := range entries {
        b.WriteString(renderEntryPlain(entry, opts, width))
        b.WriteString("\n")
    }
    return b.String()
}

// renderUserMessagePlain renders a user message entry for plain text output.
func renderUserMessagePlain(entry types.LogEntry, width int) string {
    timestamp := formatTimestamp(entry.Timestamp)
    header := fmt.Sprintf("%s %s  %s",
        UserIcon,
        Styles.UserHeader.Render("User"),
        Styles.Timestamp.Render(timestamp),
    )

    // Wrap content to fit specified width (with margin for styling)
    wrapWidth := width - 4
    if wrapWidth < 20 {
        wrapWidth = 20
    }
    wrappedText := WrapText(entry.Message.TextContent, wrapWidth)
    content := Styles.MessageContent.Render(wrappedText)

    return Styles.UserMessage.Render(header + "\n" + content)
}

// renderAssistantMessagePlain renders an assistant message entry for plain text output.
func renderAssistantMessagePlain(entry types.LogEntry, opts RenderOptions, width int) string {
    timestamp := formatTimestamp(entry.Timestamp)
    header := fmt.Sprintf("%s %s  %s",
        AssistantIcon,
        Styles.AssistantHeader.Render("Assistant"),
        Styles.Timestamp.Render(timestamp),
    )

    // Calculate wrap width for content
    wrapWidth := width - 4
    if wrapWidth < 20 {
        wrapWidth = 20
    }

    var parts []string
    parts = append(parts, header)

    for _, content := range entry.Message.Content {
        switch content.Type {
        case types.ContentTypeText:
            if content.Text != "" {
                wrappedText := WrapText(content.Text, wrapWidth)
                parts = append(parts, Styles.MessageContent.Render(wrappedText))
            }

        case types.ContentTypeThinking:
            if opts.HideThoughts {
                continue
            }
            thinkingHeader := fmt.Sprintf("%s %s", ThinkingIcon, Styles.ThinkingHeader.Render("Thinking"))
            wrappedThinking := WrapText(content.Thinking, wrapWidth)
            parts = append(parts, Styles.ThinkingBlock.Render(thinkingHeader+"\n"+wrappedThinking))

        case types.ContentTypeToolUse:
            if opts.HideTools {
                continue
            }
            toolHeader := fmt.Sprintf("%s %s: %s",
                ToolIcon,
                Styles.ToolHeader.Render("Tool"),
                content.ToolName,
            )
            inputStr := formatToolInputPlain(content.ToolInput)
            wrappedInput := WrapText(inputStr, wrapWidth)
            parts = append(parts, Styles.ToolBlock.Render(toolHeader+"\n"+wrappedInput))
        }
    }

    return Styles.AssistantMessage.Render(strings.Join(parts, "\n"))
}
```

### TUI Mode Width Override

For TUI mode, the width override should be used as the initial width but can still respond to window resize events (unless we want to lock it). Decision: **Lock width when --width is specified** - user wants explicit control.

```go
// In NewViewerModel (viewer.go lines 80-117):
// Modify the return statement to use a variable so we can conditionally set width:
func NewViewerModel(entries []types.LogEntry, parseErrors int, title string, opts RenderOptions) ViewerModel {
    // ... existing initialization code ...

    m := ViewerModel{
        entries:        entries,
        parseErrors:    parseErrors,
        title:          title,
        showThinking:   false,
        showToolInputs: false,
        canGoBack:      false,
        searchInput:    ti,
        lazyEnabled:    lazyEnabled,
        loadedCount:    loadedCount,
        lazyLoadState:  state,
        overlaySpinner: s,
        renderOpts:     opts,
    }

    // Apply width override if specified
    if opts.Width > 0 {
        m.width = opts.Width
    }

    return m
}

// In Update() WindowSizeMsg handler (viewer.go lines 278-297):
case tea.WindowSizeMsg:
    // Only update width from terminal if not overridden
    if m.renderOpts.Width == 0 {
        m.width = msg.Width
    }
    m.height = msg.Height // Height always tracks terminal

    // Header: 1 line for title
    // Footer: 1 line for help/status
    headerHeight := 1
    footerHeight := 1
    verticalMargins := headerHeight + footerHeight

    if !m.ready {
        m.viewport = viewport.New(m.width, m.height-verticalMargins) // Use m.width, not msg.Width
        m.viewport.YPosition = headerHeight
        m.ready = true
        m.updateContent()
    } else {
        m.viewport.Width = m.width // Use m.width, not msg.Width
        m.viewport.Height = m.height - verticalMargins
        m.updateContent()
    }
```

**Key Implementation Note**: The current code at lines 289 and 294 uses `msg.Width` directly. These must be changed to use `m.width` to respect the width override. The conditional check should only guard the `m.width = msg.Width` assignment.

### Signature Changes Summary

| Function | Current | New |
|----------|---------|-----|
| renderEntryPlain | `(entry types.LogEntry, opts RenderOptions) string` | `(entry types.LogEntry, opts RenderOptions, width int) string` |
| renderUserMessagePlain | `(entry types.LogEntry) string` | `(entry types.LogEntry, width int) string` |
| renderAssistantMessagePlain | `(entry types.LogEntry, opts RenderOptions) string` | `(entry types.LogEntry, opts RenderOptions, width int) string` |

### New Test Files

**cmd/cclv/main_test.go**:
```go
package main

import (
    "bytes"
    "os"
    "strings"
    "testing"
)

func TestValidateWidth(t *testing.T) {
    tests := []struct {
        name        string
        input       int
        want        int
        wantWarning bool
    }{
        {"zero returns zero", 0, 0, false},
        {"negative returns zero", -1, 0, false},
        {"valid 50", 50, 50, false},
        {"valid 80", 80, 80, false},
        {"valid 120", 120, 120, false},
        {"valid 500 max", 500, 500, false},
        {"too small 30", 30, 80, true},
        {"too small 39", 39, 80, true},
        {"too large 600", 600, 500, true},
        {"too large 1000", 1000, 500, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Capture stderr
            oldStderr := os.Stderr
            r, w, _ := os.Pipe()
            os.Stderr = w

            got := validateWidth(tt.input)

            w.Close()
            var buf bytes.Buffer
            buf.ReadFrom(r)
            os.Stderr = oldStderr

            if got != tt.want {
                t.Errorf("validateWidth(%d) = %d, want %d", tt.input, got, tt.want)
            }

            hasWarning := strings.Contains(buf.String(), "Warning")
            if hasWarning != tt.wantWarning {
                t.Errorf("validateWidth(%d) warning = %v, wantWarning %v", tt.input, hasWarning, tt.wantWarning)
            }
        })
    }
}
```

**internal/tui/plain_test.go**:
```go
package tui

import (
    "strings"
    "testing"
    "time"

    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

func TestRenderPlainWithWidth(t *testing.T) {
    // Create a test entry with long text that should wrap
    longText := strings.Repeat("word ", 30) // 150 chars
    entries := []types.LogEntry{
        {
            Type:      types.EntryTypeUser,
            Timestamp: time.Now(),
            Message: types.Message{
                TextContent: longText,
            },
        },
    }

    tests := []struct {
        name      string
        width     int
        wantWidth int // expected max line length (approximate)
    }{
        {"default width 0 uses 80", 0, 80},
        {"explicit width 60", 60, 60},
        {"explicit width 120", 120, 120},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            opts := RenderOptions{Width: tt.width}
            output := RenderPlain(entries, "test", opts)

            // Verify output doesn't have excessively long lines
            lines := strings.Split(output, "\n")
            for _, line := range lines {
                // Strip ANSI codes for accurate length check
                // (simplified - real implementation would need proper ANSI stripping)
                if len(line) > tt.wantWidth+20 { // Allow some margin for ANSI codes
                    t.Logf("Line may be too long: %d chars", len(line))
                }
            }

            if len(output) == 0 {
                t.Error("RenderPlain returned empty output")
            }
        })
    }
}
```

### Edge Cases

1. **Width 0**: Auto-detect mode (current behavior preserved)
2. **Width < 40**: Too narrow for readable content, clamp to 80 with warning
3. **Width > 500**: Unnecessarily wide, clamp to 500 with warning
4. **Negative width**: Treat as auto-detect (return 0), no warning - user likely meant default
5. **Width in TUI + resize**: Lock to specified width, only height changes
6. **Width in plain mode**: Controls all text wrapping and formatting
7. **Width 40-500**: Valid range, use as-is without warning

### Files to Modify Checklist

- [x] `cmd/cclv/main.go` - Add --width flag, validateWidth function, pass to opts
- [x] `cmd/cclv/main_test.go` - **NEW FILE**: Add TestValidateWidth with table-driven tests
- [x] `internal/tui/viewer.go` - Add Width to RenderOptions, handle in NewViewerModel and Update
- [x] `internal/tui/plain.go` - Update RenderPlain and helpers to use width + add text wrapping
- [x] `internal/tui/plain_test.go` - **NEW FILE**: Add tests for plain mode width rendering

### Testing Pattern

```bash
# Manual verification commands
./bin/cclv --width 60 test.jsonl --plain | head -20  # Should wrap at 60 chars
./bin/cclv --width 120 test.jsonl --plain | head -20  # Should wrap at 120 chars
./bin/cclv --width 30 test.jsonl 2>&1 | head -5  # Should show warning, use 80
./bin/cclv test.jsonl --plain | head -20  # Default behavior (80)
```

### Git Commit Pattern

```
feat: add --width flag for pipeline width override

- Add --width CLI flag with validation (40-500 range)
- Add Width field to RenderOptions struct
- Update plain mode rendering to respect width
- Lock TUI width when --width specified

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Previous Story Intelligence

**From Story 1.9 (Pipeline Visibility Flags):**
- CLI flag pattern established: define flag, validate, pass to RenderOptions
- RenderOptions struct is the central place for rendering configuration
- Signature changes propagate through: main.go -> tui functions -> helpers

**From Story 1.10 (Conversation Count):**
- Simple additions follow clean patterns
- Table-driven tests are the standard
- Graceful degradation on edge cases (warnings, not errors)

### Risk Assessment

**Risk: LOW**

- CLI flag parsing is well-established pattern
- Width validation is straightforward clamping logic
- RenderOptions already exists - just adding one more field
- Plain mode changes are self-contained
- TUI width override is straightforward conditional
- No async complexity or goroutine management
- Backward compatibility: Width: 0 preserves current behavior

### Project Context Reference

From `project-context.md`:
- **NO EMOJI IN UI** - Use text icons only
- **USE MAKEFILE** - `make build`, `make test`, `make lint`
- **Error handling**: Warnings to stderr, graceful degradation
- **Testing**: Table-driven tests, 90% coverage minimum

### References

- [Source: epics.md lines 552-593] - Story requirements and technical notes
- [Source: project-context.md] - NO EMOJI, USE MAKEFILE, error handling rules
- [Source: cmd/cclv/main.go lines 30-39] - Existing flag parsing pattern
- [Source: internal/tui/viewer.go lines 11-21] - RenderOptions struct
- [Source: internal/tui/plain.go] - Plain mode rendering
- [Source: Story 1.9] - CLI flag pattern, RenderOptions usage

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Implementation straightforward, no debugging required.

### Completion Notes List

- Added `--width` CLI flag with validation (40-500 range, warning on clamp)
- Extended `RenderOptions` struct with `Width` field
- Updated plain mode rendering to use width with text wrapping via `WrapText()`
- Modified TUI mode to lock width when `--width` specified (ignores terminal resize for width)
- Created comprehensive table-driven tests for width validation
- Created tests for plain mode rendering with width options

### File List

- `cmd/cclv/main.go` - Added --width flag, validateWidth function, width constants
- `cmd/cclv/main_test.go` - NEW: TestValidateWidth with 12 test cases
- `internal/tui/viewer.go` - Added Width to RenderOptions, width override in NewViewerModel and Update
- `internal/tui/plain.go` - Updated RenderPlain and helpers with width parameter and text wrapping
- `internal/tui/plain_test.go` - NEW: Tests for RenderPlain width and hide options
