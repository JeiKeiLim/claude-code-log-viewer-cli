# Story 4.1: Line Numbers Display

Status: complete

## Story

As a **developer reviewing logs**,
I want **line numbers displayed in a gutter column**,
So that **I can reference specific locations when debugging**.

## Acceptance Criteria

### AC 4.1.1: Normal mode box numbers
- **Given** the viewer is displaying a conversation in normal mode
- **When** I view the content
- **Then** box numbers (1, 2, 3...) appear in a left gutter column
- **And** numbers are right-aligned and styled with dimmed text

### AC 4.1.2: Raw mode JSONL line numbers (Future - Story 4.3)
- **Given** the viewer is displaying a conversation in raw mode
- **When** I view the content
- **Then** JSONL line numbers appear in the gutter
- **And** gutter width adjusts based on max line count

**Note:** Raw mode is implemented in Story 4.3. This story implements gutter infrastructure that Story 4.3 will extend.

### AC 4.1.3: Gutter styling
- **Given** line numbers are displayed
- **When** I view the gutter
- **Then** numbers use the Theme's Dim color
- **And** gutter has consistent fixed width based on entry count
- **And** gutter is visually separated from content (subtle border or padding)

### AC 4.1.4: Gutter width adaptation
- **Given** a conversation with N entries
- **When** calculating gutter width
- **Then** width accommodates the largest number (e.g., 3 chars for 999 entries)
- **And** minimum gutter width is 3 characters (for numbers 1-99)

## Tasks / Subtasks

- [x] Task 1: Add gutter infrastructure to ViewerModel (AC: 4.1.1, 4.1.4)
  - [x] 1.1: Add `showLineNumbers bool` field to ViewerModel (default: true)
  - [x] 1.2: Add `gutterWidth int` field calculated from entry count
  - [x] 1.3: Add helper function `calculateGutterWidth(entryCount int) int` that returns max(3, len(fmt.Sprintf("%d", entryCount)))
  - [x] 1.4: Initialize gutterWidth in NewViewerModel() based on len(entries)

- [x] Task 2: Add gutter style to styles.go (AC: 4.1.3)
  - [x] 2.1: Add `GutterStyle` to Styles struct using `dimColor`
  - [x] 2.2: Style: right-aligned, fixed width placeholder (actual width set at render time)
  - [x] 2.3: Add `GutterSeparator` constant (e.g., " | " or single space with subtle styling)

- [x] Task 3: Modify updateContent() to prepend line numbers (AC: 4.1.1, 4.1.3)
  - [x] 3.1: For each rendered entry, prepend formatted line number with gutter styling
  - [x] 3.2: Format: `fmt.Sprintf("%*d", gutterWidth, lineNum)` for right-alignment
  - [x] 3.3: Apply GutterStyle to the line number string
  - [x] 3.4: Prepend gutter to first line of each rendered entry
  - [x] 3.5: Do NOT prepend gutter to lazy loading indicator ("-- X more entries --") - it's not a real entry

- [x] Task 4: Handle multi-line entries (AC: 4.1.1)
  - [x] 4.1: Only first line of each entry shows the line number
  - [x] 4.2: Subsequent lines get padding equal to gutterWidth + separator length
  - [x] 4.3: Create helper `prependGutter(entryNum int, content string, gutterWidth int) string`
  - [x] 4.4: Create static version `prependGutterStatic()` for use in `markAllMessagesLoadedCmd()` (viewer.go:509-532) which pre-renders all content for G-key bulk loading

- [x] Task 5: Update gutter on entry count changes (AC: 4.1.4)
  - [x] 5.1: In NewEntriesMsg handler (viewer.go:426-446), recalculate gutterWidth ONLY if new total crosses digit threshold (e.g., 99→100, 999→1000). Invalidate cache if width changes.
  - [x] 5.2: In FileResetMsg handler (viewer.go:448-466), recalculate gutterWidth for new entry count after reload, invalidate cache.

- [x] Task 6: Adjust content width calculation (AC: 4.1.1)
  - [x] 6.1: Reduce available content width by gutterWidth + separator width
  - [x] 6.2: Update wrapWidth calculation in renderUserMessage()
  - [x] 6.3: Update markdown renderer width to account for gutter
  - [x] 6.4: Invalidate render cache when gutter width changes (similar to resize)

- [x] Task 7: Add unit tests for gutter functionality (AC: 4.1.1, 4.1.3, 4.1.4)
  - [x] 7.1: Test calculateGutterWidth() returns correct width for various entry counts
  - [x] 7.2: Test prependGutter() formats correctly with right-alignment
  - [x] 7.3: Test multi-line entries have proper padding on continuation lines
  - [x] 7.4: Test gutter width recalculation on new entries

- [x] Task 8: Run build, lint, and test validation
  - [x] 8.1: Run `make build` - verify binary builds
  - [x] 8.2: Run `make lint` - no errors
  - [x] 8.3: Run `make test` - all tests pass, coverage maintained

- [x] Task 9: Manual testing
  - [x] 9.1: Open conversation with < 100 entries - verify 3-char gutter (minimum width)
  - [x] 9.2: Open conversation with 100-999 entries - verify 3-char gutter
  - [x] 9.3: Open conversation with 1000+ entries - verify 4-char gutter
  - [x] 9.4: Verify line numbers are right-aligned and dimmed
  - [x] 9.5: Verify multi-line entries show number only on first line
  - [x] 9.6: Watch mode - verify gutter adjusts when many new entries arrive

## Dev Notes

### Implementation Reference

**New ViewerModel Fields:**
```go
showLineNumbers bool  // default true
gutterWidth     int   // calculated from entry count
```

**Core Functions to Implement:**

```go
// calculateGutterWidth returns max(3, digits_in(entryCount))
func calculateGutterWidth(entryCount int) int {
    if entryCount == 0 {
        return 3
    }
    width := len(fmt.Sprintf("%d", entryCount))
    if width < 3 {
        width = 3
    }
    return width
}

// prependGutter adds line number to first line, padding to subsequent lines
func prependGutter(entryNum int, content string, gutterWidth int) string {
    lines := strings.Split(content, "\n")
    var result strings.Builder

    numStr := fmt.Sprintf("%*d", gutterWidth, entryNum)
    gutterFormatted := Styles.Gutter.Render(numStr) + GutterSeparator
    result.WriteString(gutterFormatted + lines[0])

    padding := strings.Repeat(" ", gutterWidth+len(GutterSeparator))
    for i := 1; i < len(lines); i++ {
        result.WriteString("\n" + padding + lines[i])
    }
    return result.String()
}
```

**styles.go Additions:**
```go
GutterStyle: lipgloss.NewStyle().Foreground(dimColor)
const GutterSeparator = " "
```

**Width Adjustment Pattern:**
```go
// All wrapWidth calculations must subtract gutter space
wrapWidth := m.width - 4 - m.gutterWidth - len(GutterSeparator)
```

### Key Integration Points

| Location | Change |
|----------|--------|
| `NewViewerModel()` | Initialize gutterWidth = calculateGutterWidth(len(entries)) |
| `updateContent()` | Call prependGutter() for each entry (NOT for lazy loading indicator) |
| `markAllMessagesLoadedCmd()` | Use prependGutterStatic() in async rendering loop |
| `NewEntriesMsg` handler | Recalculate gutterWidth if digit threshold crossed |
| `FileResetMsg` handler | Recalculate gutterWidth after reload |
| `renderUserMessage()` | Reduce wrapWidth by gutter space |
| Markdown renderer init | Reduce width by gutter space |

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Add showLineNumbers, gutterWidth, prependGutter(), update updateContent() |
| `internal/tui/styles.go` | Add GutterStyle, GutterSeparator |
| `internal/tui/viewer_test.go` | Add gutter-related tests |

### Files NOT to Modify

| File | Reason |
|------|--------|
| `cmd/cclv/main.go` | No CLI changes for this story |
| `internal/parser/*.go` | Parser unaffected |
| `internal/scanner/*.go` | Scanner unaffected |
| `internal/watcher/*.go` | Watcher unaffected |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI** | Text icons only per project-context.md |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **Test patterns** | Table-driven tests per project-context.md |
| **Cache invalidation** | Must invalidate if gutterWidth changes |
| **TEA pattern** | All state changes via Update() |

### Previous Story Intelligence (Story 3.3)

Key learnings from Epic 3:
1. **Cache invalidation patterns** - Know when to clear renderCache
2. **Width calculation** - wrapWidth = m.width - 4 pattern
3. **Graceful degradation** - Handle edge cases (nil checks, empty data)

### Project Context Reference

From `project-context.md`:
- **Styling**: All styles defined in `internal/tui/styles.go`
- **Dim color**: `dimColor = DefaultTheme.Dim` - use for gutter numbers
- **NO EMOJI**: Text icons only - numbers are fine
- **TEA pattern**: State changes only in Update(), View() for rendering

### Git Intelligence

Recent commits:
```
07b7984 docs: add Epic 3 retrospective for Markdown Rendering
78c2bde feat: implement render caching for markdown content
aca15d1 feat: integrate Glamour markdown renderer for assistant text
```

Suggested commit message:
```
feat: add line number gutter to viewer display

- Add gutterWidth calculation based on entry count
- Prepend right-aligned line numbers to each entry
- Multi-line entries show number only on first line
- Adjust content width to accommodate gutter

Story 4.1 of Epic 4: Developer Power Tools

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Dependencies

- **Epic 3 (Complete)**: Provides render cache that needs invalidation awareness
- **Story 4.3 (Future)**: Will extend gutter for raw mode JSONL line numbers
- **Story 4.2 (Future)**: Will use line numbers for `:N` navigation target

### References

- [Source: epics-phase3.md lines 122-139] - Story 4.1 requirements and acceptance criteria
- [Source: prd-phase3.md lines 99-107] - FR-401 Line Numbers Display
- [Source: architecture-phase3.md lines 126-129] - Decision 9: Line Number Gutter
- [Source: project-context.md lines 156-158] - NO EMOJI rule, styling rules
- [Source: internal/tui/viewer.go:650-678] - updateContent() to modify
- [Source: internal/tui/styles.go:40-67] - Theme colors including dimColor
- [Source: 3-3-implement-render-caching.md] - Cache invalidation patterns

## Implementation Checklist

Before marking story complete, verify:

- [x] `showLineNumbers bool` added to ViewerModel (default true)
- [x] `gutterWidth int` added to ViewerModel
- [x] `calculateGutterWidth()` function implemented
- [x] `GutterStyle` added to styles.go with dimColor
- [x] `GutterSeparator` constant defined
- [x] `prependGutter()` helper function implemented
- [x] updateContent() prepends line numbers to entries
- [x] Multi-line entries show number only on first line
- [x] Content width adjusted for gutter space
- [x] Cache invalidation on gutterWidth change
- [x] NewEntriesMsg recalculates gutterWidth if needed
- [x] Unit tests added for gutter functionality
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes with no regressions
- [x] Manual: line numbers visible and right-aligned
- [x] Manual: dimmed styling matches theme
- [x] Manual: multi-line entries formatted correctly
- [x] Manual: watch mode gutter adjusts properly

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. Added `showLineNumbers bool` (default true) and `gutterWidth int` fields to ViewerModel
2. Implemented `calculateGutterWidth()` function returning max(3, digit count of entry count)
3. Added `GutterStyle` to styles.go using dimColor for dimmed appearance
4. Added `GutterSeparator` constant as single space " "
5. Implemented `prependGutter()` for styled output and `prependGutterStatic()` for async rendering
6. Modified `updateContent()` to prepend line numbers to each entry (1-indexed)
7. Multi-line entries show number only on first line, continuation lines get padding
8. Updated width calculations in all render functions to account for gutter space
9. Added cache invalidation when gutterWidth changes (NewEntriesMsg, FileResetMsg handlers)
10. Added 6 unit tests: calculateGutterWidth, prependGutter, gutter width recalculation, initialization, showLineNumbers default

### Code Review Fixes (2026-01-16)

**Reviewer:** Amelia (Dev Agent) - Claude Opus 4.5

**Issues Fixed:**
- **H1 (HIGH):** Static render functions now account for gutter width - added `gutterWidth` parameter to `renderEntryStatic()`, `renderUserMessageStatic()`, `renderAssistantMessageStatic()`, `renderThinkingBlockStatic()`, `renderToolUseBlockStatic()`
- **M2 (MEDIUM):** Added `TestPrependGutterStatic` test to verify static gutter rendering without ANSI escape codes
- **M3 (MEDIUM):** Moved `GutterSeparator` constant from viewer.go to styles.go for consistency

**Deferred Issues:**
- **M1:** `prependGutterStatic()` styling difference (no dimmed colors in async rendering) - acceptable for performance, creates minor visual inconsistency during bulk load only
- **M4:** Markdown renderer width update on showLineNumbers toggle - deferred as toggle doesn't exist yet
- **M5:** Test coverage for continuation line content preservation - low priority

### File List

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Added showLineNumbers, gutterWidth fields; calculateGutterWidth(), prependGutter(), prependGutterStatic() functions; updated updateContent(), markAllMessagesLoadedCmd(), NewEntriesMsg/FileResetMsg handlers; [Code Review Fix] added gutterWidth param to all static render functions |
| `internal/tui/styles.go` | Added Gutter style with dimColor; [Code Review Fix] moved GutterSeparator constant here |
| `internal/tui/viewer_test.go` | Added TestCalculateGutterWidth, TestPrependGutter, TestGutterWidthRecalculationOnDigitThreshold, TestNewViewerModelGutterWidthInitialization, TestShowLineNumbersDefaultTrue; [Code Review Fix] added TestPrependGutterStatic; Updated TestRenderCacheInitialization |
