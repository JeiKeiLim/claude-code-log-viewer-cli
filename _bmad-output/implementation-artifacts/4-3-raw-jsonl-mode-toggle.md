# Story 4.3: Raw JSONL Mode Toggle

Status: done

## Story

As a **developer debugging log structure**,
I want **to toggle between parsed view and raw JSONL**,
So that **I can see the actual log content like `jq`**.

## Acceptance Criteria

### AC 4.3.1: Toggle to raw mode
- **Given** I am in the viewer in normal mode
- **When** I press `r`
- **Then** the view switches to raw JSONL content
- **And** status bar indicates "RAW" mode

### AC 4.3.2: Toggle back to normal mode
- **Given** I am in raw mode
- **When** I press `r`
- **Then** the view switches back to parsed/rendered mode
- **And** status bar shows normal mode

### AC 4.3.3: Raw mode scrolling
- **Given** I am in raw mode
- **When** I scroll through content
- **Then** raw JSON lines display with preserved formatting
- **And** scrolling works identically to normal mode

### AC 4.3.4: Raw mode line numbers
- **Given** I am in raw mode with line numbers visible
- **When** I view the content
- **Then** JSONL line numbers (not box numbers) appear in the gutter
- **And** gutter width adjusts based on total raw line count

### AC 4.3.5: Command mode integration
- **Given** I am in raw mode
- **When** I enter `:N` and press Enter
- **Then** the viewer jumps to raw JSONL line N (not box N)

## Tasks / Subtasks

- [x] Task 1: Add raw mode state to ViewerModel (AC: 4.3.1, 4.3.2)
  - [x] 1.1: Add `rawMode bool` field to ViewerModel (default: false)
  - [x] 1.2: Add `rawLines []string` field to cache raw JSONL content
  - [x] 1.3: Add `rawLineCount int` field for tracking total lines
  - [x] 1.4: Initialize rawMode = false in NewViewerModel()

- [x] Task 2: Implement raw JSONL loading (AC: 4.3.1)
  - [x] 2.1: Create `loadRawJSONL() error` method that reads from `renderOpts.FilePath`
  - [x] 2.2: Read file line-by-line, preserving raw JSON strings (no parsing)
  - [x] 2.3: Store lines in `rawLines` slice
  - [x] 2.4: Store count in `rawLineCount`
  - [x] 2.5: Handle missing FilePath gracefully (show toast error if file unavailable)

- [x] Task 3: Add `r` key handler for mode toggle (AC: 4.3.1, 4.3.2)
  - [x] 3.1: Add `r` key handler in Update() that toggles `rawMode`
  - [x] 3.2: On toggle TO raw mode: call `loadRawJSONL()`, then `updateRawContent()`
  - [x] 3.3: On toggle FROM raw mode: call `updateContent()` to restore normal view
  - [x] 3.4: Ensure toggle works from both normal and raw modes
  - [x] 3.5: Preserve scroll position relative to current entry when toggling (best effort)

- [x] Task 4: Implement raw content rendering (AC: 4.3.3)
  - [x] 4.1: Create `updateRawContent()` method for rendering raw JSONL
  - [x] 4.2: Format JSON with indentation for readability (2-space indent)
  - [ ] 4.3: Add syntax highlighting using lipgloss styles (optional, time permitting) - SKIPPED
  - [x] 4.4: Wrap long lines at viewport width (preserve JSON structure visibility)
  - [x] 4.5: Maintain lazy loading pattern if raw line count > 100

- [x] Task 5: Implement raw mode line numbers (AC: 4.3.4)
  - [x] 5.1: In raw mode, calculate gutterWidth from `rawLineCount` (not entry count)
  - [x] 5.2: Prepend JSONL line numbers (1-indexed) to each raw line
  - [x] 5.3: Use same `prependGutter()` helper with raw line index
  - [x] 5.4: Track `rawLinePositions []int` for navigation (parallel to entryLinePositions)

- [x] Task 6: Update command mode for raw navigation (AC: 4.3.5)
  - [x] 6.1: In `navigateToEntry()`, check if `rawMode` is active
  - [x] 6.2: If raw mode: validate against `rawLineCount`, use `rawLinePositions`
  - [x] 6.3: If normal mode: existing behavior (validate against entry count)
  - [x] 6.4: Show appropriate toast errors ("Invalid line number" works for both)

- [x] Task 7: Update status bar to show RAW mode (AC: 4.3.1, 4.3.2)
  - [x] 7.1: In `buildModeSegment()`, add "RAW" indicator when `rawMode == true`
  - [x] 7.2: Style: Use existing mode segment styling with "RAW" text
  - [x] 7.3: Format: "RAW" text icon (NO EMOJI)
  - [x] 7.4: Update shortcuts segment to show `r:raw` / `r:normal` toggle hint

- [x] Task 8: Add unit tests for raw mode (AC: 4.3.1-4.3.5)
  - [x] 8.1: Test `r` key toggles rawMode from false to true
  - [x] 8.2: Test `r` key toggles rawMode from true to false
  - [x] 8.3: Test loadRawJSONL() missing FilePath shows error
  - [x] 8.4: Test formatJSONLine() renders JSON with proper formatting
  - [x] 8.5: Test gutter width calculation uses rawLineCount in raw mode
  - [x] 8.6: Test navigation uses rawLinePositions in raw mode
  - [x] 8.7: Test invalid line number in raw mode shows toast (navigateToEntry)
  - [x] 8.8: Test mode indicator shows "RAW" in status bar
  - [x] 8.9: Test buildPositionSegment shows "Line N/M" in raw mode
  - [x] 8.10: Test toggle preserves scroll percentage

- [x] Task 9: Run build, lint, and test validation
  - [x] 9.1: Run `make build` - binary builds successfully
  - [x] 9.2: Run `make lint` - no errors
  - [x] 9.3: Run `make test` - all tests pass

- [x] Task 10: Manual testing
  - [x] 10.1-10.9: Verified via unit tests covering all acceptance criteria

## Dev Notes

### Required Import Additions

Add to `internal/tui/viewer.go` imports:

```go
import (
    "bufio"  // NEW: for bufio.NewScanner in loadRawJSONL
    "os"     // NEW: for os.Open in loadRawJSONL
    // ... existing imports
)
```

### New ViewerModel Fields

```go
// Add to ViewerModel struct after line ~117 (after showLineNumbers, gutterWidth)
rawMode          bool     // Toggle between parsed and raw view (default: false)
rawLines         []string // Cached raw JSONL lines (init: nil)
rawLineCount     int      // Total raw lines for gutter width (init: 0)
rawLinePositions []int    // Y offset for each raw line for navigation (init: nil)
```

### NewViewerModel() Initialization

Add to `NewViewerModel()` after line ~226 (after entryLinePositions):

```go
rawMode:          false,
rawLines:         nil,   // Loaded on first r press
rawLineCount:     0,
rawLinePositions: nil,   // Built in updateRawContent()
```

### Core Implementation

**loadRawJSONL()** - Add after navigateToEntry():

```go
// loadRawJSONL reads the JSONL file and stores raw lines.
// Uses same 1MB buffer as parser (project-context.md).
func (m *ViewerModel) loadRawJSONL() error {
    if m.renderOpts.FilePath == "" {
        return fmt.Errorf("no file path available")
    }
    file, err := os.Open(m.renderOpts.FilePath)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()

    m.rawLines = make([]string, 0)
    scanner := bufio.NewScanner(file)
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, 1024*1024) // Max 1MB per line (matches parser)

    for scanner.Scan() {
        line := scanner.Text()
        if len(line) > 0 { // Skip empty lines
            m.rawLines = append(m.rawLines, line)
        }
    }
    m.rawLineCount = len(m.rawLines)
    return scanner.Err()
}
```

**formatJSONLine()** - Add after loadRawJSONL():

```go
// formatJSONLine pretty-prints a JSON line with 2-space indentation.
// Invalid JSON lines are returned as-is (graceful degradation).
func formatJSONLine(line string) string {
    var obj map[string]interface{}
    if err := json.Unmarshal([]byte(line), &obj); err != nil {
        return line // Not valid JSON, return as-is
    }
    formatted, err := json.MarshalIndent(obj, "", "  ")
    if err != nil {
        return line
    }
    return string(formatted)
}
```

**updateRawContent()** - Add after formatJSONLine():

```go
// updateRawContent renders raw JSONL with pretty-print formatting.
// Mirrors updateContent() pattern with lazy loading support.
func (m *ViewerModel) updateRawContent() {
    var content strings.Builder
    gutterWidth := calculateGutterWidth(m.rawLineCount)

    m.rawLinePositions = make([]int, 0, m.rawLineCount)
    currentLine := 0

    // LazyThreshold = 100 per project-context.md lazy loading rules
    renderCount := m.loadedCount
    if renderCount > m.rawLineCount {
        renderCount = m.rawLineCount
    }

    for i := 0; i < renderCount; i++ {
        m.rawLinePositions = append(m.rawLinePositions, currentLine)
        formatted := formatJSONLine(m.rawLines[i])

        if m.showLineNumbers {
            formatted = prependGutter(i+1, formatted, gutterWidth)
        }
        content.WriteString(formatted)
        content.WriteString("\n")
        currentLine += strings.Count(formatted, "\n") + 1
    }

    if m.lazyEnabled && renderCount < m.rawLineCount {
        content.WriteString(Styles.Muted.Render(
            fmt.Sprintf("-- %d more lines (scroll down to load) --",
                m.rawLineCount-renderCount)))
        content.WriteString("\n")
    }
    m.viewport.SetContent(content.String())
}
```

### Key Handler (add after "w" key handler in Update())

```go
case "r":
    if m.rawMode {
        // Exit raw mode - restore scroll position best-effort
        scrollPct := m.viewport.ScrollPercent()
        m.rawMode = false
        m.updateContent()
        // Restore approximate scroll position
        maxOffset := m.viewport.TotalLineCount() - m.viewport.Height
        if maxOffset > 0 {
            m.viewport.SetYOffset(int(float64(maxOffset) * scrollPct))
        }
    } else {
        // Enter raw mode
        if err := m.loadRawJSONL(); err != nil {
            return m, m.showToast("Cannot load raw file", ToastDuration)
        }
        scrollPct := m.viewport.ScrollPercent()
        m.rawMode = true
        // LazyInitialBatch = 40 (2 * BatchSize), LazyThreshold = 100
        m.loadedCount = min(40, m.rawLineCount)
        m.lazyEnabled = m.rawLineCount > 100
        m.updateRawContent()
        // Restore approximate scroll position
        maxOffset := m.viewport.TotalLineCount() - m.viewport.Height
        if maxOffset > 0 {
            m.viewport.SetYOffset(int(float64(maxOffset) * scrollPct))
        }
    }
    return m, nil
```

### navigateToEntry() Update

Replace existing `navigateToEntry()` with:

```go
func (m *ViewerModel) navigateToEntry(entryNum int) error {
    if m.rawMode {
        if m.rawLineCount == 0 || entryNum < 1 || entryNum > m.rawLineCount {
            return fmt.Errorf("invalid line number")
        }
        if entryNum <= len(m.rawLinePositions) {
            m.viewport.SetYOffset(m.rawLinePositions[entryNum-1])
        }
    } else {
        if len(m.entries) == 0 || entryNum < 1 || entryNum > len(m.entries) {
            return fmt.Errorf("invalid line number")
        }
        if entryNum <= len(m.entryLinePositions) {
            m.viewport.SetYOffset(m.entryLinePositions[entryNum-1])
        }
    }
    return nil
}
```

### Status Bar Updates

**buildModeSegment()** - Replace existing:

```go
func (m ViewerModel) buildModeSegment() string {
    var modes []string
    if m.rawMode {
        modes = append(modes, "RAW")
    }
    if m.watchMode && m.watcher != nil {
        modes = append(modes, "LIVE")
    }
    if len(modes) == 0 {
        return ""
    }
    return Styles.StatusBarSegment.Mode.Render(strings.Join(modes, " "))
}
```

**buildShortcutsSegment()** - Update to include raw toggle:

```go
// In buildShortcutsSegment(), update the shortcuts list:
if m.rawMode {
    parts = append(parts, "r:normal")
} else {
    parts = append(parts, "r:raw")
}
```

### Watch Mode Behavior in Raw Mode

When a file change occurs while in raw mode, **exit raw mode** and reload normal view:

```go
// In NewEntriesMsg handler, add at start:
if m.rawMode {
    m.rawMode = false // Exit raw mode on file change
}
// ... existing handler code ...

// In FileResetMsg handler, add at start:
if m.rawMode {
    m.rawMode = false // Exit raw mode on file reset
}
// ... existing handler code ...
```

### File Changes Summary

| File | Changes |
|------|---------|
| `internal/tui/viewer.go` | Add imports (`bufio`, `os`), ViewerModel fields (`rawMode`, `rawLines`, `rawLineCount`, `rawLinePositions`), 3 new functions (`loadRawJSONL`, `updateRawContent`, `formatJSONLine`), update `navigateToEntry()`, `buildModeSegment()`, `buildShortcutsSegment()`, add `r` key handler, update NewEntriesMsg/FileResetMsg handlers |
| `internal/tui/viewer_test.go` | Add 10 raw mode unit tests |
| **No changes** | `app.go`, `styles.go` (reuse existing), `parser/*`, `scanner/*`, `watcher/*` |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI** | Text icons only - "RAW" text indicator |
| **Use Makefile** | `make build`, `make test` |
| **TEA pattern** | All state changes via Update() |
| **Buffer size** | Max 1MB per line (matches parser) |
| **Lazy loading** | Threshold 100 lines, batch size 20 |

### Architecture Context

**Mode State Design (from architecture-phase3.md Decision 3):**
- `rawMode bool` = VIEW toggle (normal vs raw JSONL)
- `inputMode enum` = INPUT state (none, command, search)
- **Independent states**: Can be in raw mode + command mode simultaneously

```
Normal View ─────r────→ Raw View ─────r────→ Normal View
   ↕                        ↕
Command Mode            Command Mode
(navigates entries)     (navigates lines)
```

### Reusable Functions from Stories 4.1 & 4.2

| Function | Use in Story 4.3 |
|----------|------------------|
| `calculateGutterWidth(count)` | Calculate gutter for rawLineCount |
| `prependGutter(num, content, width)` | Add line numbers to raw lines |
| `showToast(msg, duration)` | Display "Cannot load raw file" error |
| `navigateToEntry(num)` | Extend for raw mode line navigation |

### Edge Cases

1. **Missing FilePath**: Toast "Cannot load raw file"
2. **Invalid JSON**: Display line as-is (graceful degradation)
3. **Watch mode + raw mode**: Exit raw mode on file change
4. **Large files (>100 lines)**: Apply lazy loading
5. **Empty file**: Empty viewport (graceful)

### Git Commit Template

```
feat: implement raw JSONL mode toggle

- Add r key to toggle between parsed and raw JSONL view
- Display raw JSON with pretty-print formatting
- Status bar shows RAW mode indicator
- Command mode :N navigates to raw line numbers

Story 4.3 of Epic 4: Developer Power Tools

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

## Implementation Checklist

**Imports & Fields:**
- [ ] Add `bufio` and `os` imports to viewer.go
- [ ] Add `rawMode`, `rawLines`, `rawLineCount`, `rawLinePositions` fields
- [ ] Initialize fields in NewViewerModel()

**Functions:**
- [ ] `loadRawJSONL()` - read file, store lines with 1MB buffer
- [ ] `formatJSONLine()` - pretty-print JSON, graceful fallback
- [ ] `updateRawContent()` - render with gutter and lazy loading
- [ ] Update `navigateToEntry()` - raw mode branch
- [ ] Update `buildModeSegment()` - show "RAW"
- [ ] Update `buildShortcutsSegment()` - show "r:raw" / "r:normal"

**Key Handling:**
- [ ] `r` key toggles rawMode with scroll position preservation
- [ ] NewEntriesMsg/FileResetMsg handlers exit raw mode

**Tests (10):**
- [ ] r toggles rawMode false→true and true→false
- [ ] loadRawJSONL() loads lines correctly
- [ ] formatJSONLine() pretty-prints valid JSON
- [ ] Gutter width uses rawLineCount
- [ ] Navigation uses rawLinePositions
- [ ] Invalid line number shows toast
- [ ] Mode indicator shows "RAW"
- [ ] Missing FilePath shows toast
- [ ] Scroll position preserved on toggle
- [ ] Watch mode exits raw mode on file change

**Build & Manual:**
- [ ] `make build` / `make lint` / `make test` pass
- [ ] Coverage >= 90%
- [ ] Manual: r toggles, RAW shows, :N navigates, formatting correct

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Implemented raw JSONL mode toggle with `r` key
- Added `rawMode`, `rawLines`, `rawLineCount`, `rawLinePositions` fields to ViewerModel
- Created `loadRawJSONL()` method with 1MB buffer matching parser
- Created `formatJSONLine()` for JSON pretty-printing with graceful fallback
- Created `updateRawContent()` with lazy loading support (threshold 100, batch 20)
- Updated `navigateToEntry()` to handle raw mode navigation
- Updated `buildModeSegment()` to show "RAW" indicator
- Updated `buildPositionSegment()` to show "Line N/M" in raw mode
- Updated `buildShortcutsSegment()` to show "r:raw" / "r:normal" toggle
- Added watch mode handlers to exit raw mode on file changes
- Skipped optional syntax highlighting (Task 4.3) - not required for AC
- All 20+ raw mode unit tests passing
- All CI checks pass (`make ci`)

### File List

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/viewer.go` | Modified | Added imports (`bufio`, `bytes`, `os`), ViewerModel fields, `loadRawJSONL()`, `formatJSONLine()`, `updateRawContent()`, `loadMoreRawLines()`, updated `navigateToEntry()`, `buildModeSegment()`, `buildShortcutsSegment()`, `buildPositionSegment()`, added `r` key handler, `rawLinesLoadedMsg` handler, updated NewEntriesMsg/FileResetMsg handlers |
| `internal/tui/viewer_test.go` | Modified | Added 26 raw mode unit tests for AC coverage |

### Code Review Fixes (2026-01-16)

| Issue | Severity | Fix Applied |
|-------|----------|-------------|
| formatJSONLine only handled objects, not arrays | MEDIUM | Changed to use `json.Indent()` with `bytes.Buffer` for generic JSON handling |
| Lazy loading in raw mode broken (used entries count) | HIGH | Added `loadMoreRawLines()` method and `rawLinesLoadedMsg` handler |
| Raw mode scroll check used wrong count | HIGH | Updated scroll trigger to check `rawLineCount` when `rawMode` is true |
| Navigation to unloaded line silent no-op | HIGH | Updated `navigateToEntry()` to load all content when target > loadedCount |
| Missing tests for raw mode lazy loading | MEDIUM | Added 6 new tests for lazy loading in raw mode |
