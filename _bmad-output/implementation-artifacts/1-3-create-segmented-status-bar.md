# Story 1.3: Create Segmented Status Bar

Status: done

## Story

As a **developer using cclv**,
I want **a visually distinct status bar with colored sections**,
So that **I can quickly see keyboard shortcuts and current position**.

## Acceptance Criteria

### AC 1.3.1: Segmented layout
- **Given** the status bar component
- **When** it renders at the bottom of the screen
- **Then** it shows distinct colored segments using JoinHorizontal()
- **And** segments have contrasting background colors

### AC 1.3.2: Keyboard shortcuts visible
- **Given** the status bar
- **When** viewing the UI
- **Then** common shortcuts are displayed (q=quit, j/k=nav, etc.)
- **And** shortcuts use muted/secondary colors

### AC 1.3.3: Position indicator
- **Given** a log file with multiple entries
- **When** viewing the status bar
- **Then** current entry index and total count are displayed
- **And** format is "Entry X of Y" or similar

### AC 1.3.4: Mode indicator (for future watch mode - Story 2.1)
- **Given** watch mode is enabled (future feature)
- **When** viewing the status bar
- **Then** a "WATCHING" or "LIVE" indicator is visible
- **And** uses accent color to stand out
- **NOTE**: Implement placeholder/hook for future Story 2.1, but display nothing when not in watch mode

## Tasks / Subtasks

- [x] Task 1: Add status bar segment styles to styles.go (AC: 1.3.1)
  - [x] 1.1: Add StatusBar.Segment struct to Styles (after line 118, inside existing struct)
  - [x] 1.2: Add Mode, Position, Shortcuts styles with contrasting backgrounds
  - [x] 1.3: Add Padding(0, 1) to each segment
  - [x] 1.4: Mode uses accentColor background + whiteColor foreground (LIVE indicator)
  - [x] 1.5: Position uses primaryColor background + whiteColor foreground
  - [x] 1.6: Shortcuts uses bgAltColor background + textColor foreground

- [x] Task 2: Refactor viewer.go footer to use segmented layout (AC: 1.3.1, 1.3.2, 1.3.3)
  - [x] 2.1: Replace current footer (View() lines 321-359) with segmented approach
  - [x] 2.2: Create buildModeSegment() - returns empty string when watchMode=false
  - [x] 2.3: Create buildPositionSegment() - shows "Entry X/Y" format
  - [x] 2.4: Create buildShortcutsSegment() - contains keyboard shortcuts
  - [x] 2.5: Join segments with lipgloss.JoinHorizontal(lipgloss.Top, ...)
  - [x] 2.6: **CRITICAL**: When m.searching=true, skip segmented footer (keep search bar behavior)

- [x] Task 3: Implement position tracking (AC: 1.3.3)
  - [x] 3.1: Calculate approximate entry position from scroll percentage
  - [x] 3.2: Display format: "Entry 1/42" using loadedCount for lazy loading scenarios
  - [x] 3.3: Edge case: Show "0/0" for empty files

- [x] Task 4: Prepare mode indicator hook (AC: 1.3.4)
  - [x] 4.1: Add ViewerModel.watchMode bool field (default false)
  - [x] 4.2: buildModeSegment() returns styled "LIVE" when watchMode=true
  - [x] 4.3: Return empty string when watchMode=false (no segment shown)

- [x] Task 5: Test and verify (all ACs)
  - [x] 5.1: Run `make test` - all tests pass
  - [x] 5.2: Run `make build` - successful
  - [x] 5.3: Visual verification: colored segments are distinct
  - [x] 5.4: Test light and dark terminal themes
  - [x] 5.5: Test search mode (/) - verify search bar still appears correctly

## Dev Notes

### Architecture Compliance

**CRITICAL**: Follow project-context.md rules exactly.

1. **Files to modify**: `internal/tui/styles.go`, `internal/tui/viewer.go`
2. **No new files**: Extend existing structures only
3. **No emoji**: Text icons `[U]`, `[A]`, `[T]`, `[>]` per FR-017
4. **Build with Make**: `make build` and `make test` - never raw go commands
5. **TEA Pattern**: All state via Update(), side effects via tea.Cmd

### Current Footer Implementation

**Location**: `internal/tui/viewer.go` - View() method, lines 321-359

Current behavior:
- When `m.searching=true`: Shows search bar (lines 316-319) - **PRESERVE THIS**
- When not searching: Shows `helpText` + spacing + `status` joined horizontally

```go
// CURRENT (to REPLACE when not searching):
var footerParts []string
footerParts = append(footerParts, "j/k:scroll", "gg/G:top/bottom", "/:search")
// ... more parts
helpText := Styles.HelpText.Render(strings.Join(footerParts, " • "))
status := Styles.Muted.Render(statusText)
footer := lipgloss.JoinHorizontal(lipgloss.Left, helpText, spacing, status)
```

### Target Implementation

```go
// NEW segmented approach (when NOT searching):
func (m ViewerModel) buildModeSegment() string {
    if !m.watchMode {
        return "" // Empty when not in watch mode
    }
    return Styles.StatusBar.Segment.Mode.Render(" LIVE ")
}

func (m ViewerModel) buildPositionSegment() string {
    total := len(m.entries)
    if total == 0 {
        return Styles.StatusBar.Segment.Position.Render(" 0/0 ")
    }
    // Approximate position from scroll percentage
    pos := int(float64(total)*m.viewport.ScrollPercent()) + 1
    if pos > total {
        pos = total
    }
    return Styles.StatusBar.Segment.Position.Render(fmt.Sprintf(" Entry %d/%d ", pos, total))
}

func (m ViewerModel) buildShortcutsSegment() string {
    var parts []string
    parts = append(parts, "j/k:scroll", "gg/G:top/bottom", "/:search")
    if len(m.searchMatches) > 0 {
        parts = append(parts, "n/N:next/prev")
    }
    if m.canGoBack {
        parts = append(parts, "h/esc:back")
    }
    parts = append(parts, "t:thinking", "i:inputs", "q:quit")
    return strings.Join(parts, " • ")
}

// In View(), replace footer construction:
modeSegment := m.buildModeSegment()
posSegment := m.buildPositionSegment()
shortcutsText := m.buildShortcutsSegment()

// Width calculation: shortcuts fills remaining space
modeWidth := lipgloss.Width(modeSegment)
posWidth := lipgloss.Width(posSegment)
shortcutsWidth := m.width - modeWidth - posWidth

shortcutsSegment := Styles.StatusBar.Segment.Shortcuts.
    Width(shortcutsWidth).
    Render(shortcutsText)

footer := lipgloss.JoinHorizontal(lipgloss.Top, modeSegment, posSegment, shortcutsSegment)
```

### Styles to Add (styles.go after line 118)

Add inside existing Styles struct definition:

```go
// Add new nested struct after SearchInput field (around line 118):
StatusBar struct {
    Segment struct {
        Mode      lipgloss.Style  // Accent background for LIVE indicator
        Position  lipgloss.Style  // Primary background for entry position
        Shortcuts lipgloss.Style  // BgAlt background for help text
    }
}
```

Then in the struct initialization (after SearchInput initialization):

```go
StatusBar: struct {
    Segment struct {
        Mode      lipgloss.Style
        Position  lipgloss.Style
        Shortcuts lipgloss.Style
    }
}{
    Segment: struct {
        Mode      lipgloss.Style
        Position  lipgloss.Style
        Shortcuts lipgloss.Style
    }{
        Mode: lipgloss.NewStyle().
            Background(accentColor).
            Foreground(whiteColor).
            Padding(0, 1).
            Bold(true),
        Position: lipgloss.NewStyle().
            Background(primaryColor).
            Foreground(whiteColor).
            Padding(0, 1),
        Shortcuts: lipgloss.NewStyle().
            Background(bgAltColor).
            Foreground(textColor).
            Padding(0, 1),
    },
},
```

### Previous Story Intelligence (1.2)

**Key learnings from Story 1.2:**
1. All styles are in `internal/tui/styles.go` - centralized
2. Lipgloss `Border(lipgloss.RoundedBorder())` working correctly
3. Theme colors (DefaultTheme) already adaptive for light/dark
4. `whiteColor` constant for text on colored backgrounds
5. No width constraints needed for bordered elements - let content determine width

**Recent commit:** `6971e73 feat: apply rounded border styling to message cards`

### Position Calculation Notes

**IMPORTANT**: `viewport.ScrollPercent()` returns percentage of scroll position within the **rendered content**, not direct entry index. This is an approximation:

```go
// scrollPct=0.0 → first entry visible
// scrollPct=1.0 → last entry visible
pos := int(float64(len(m.entries)) * scrollPct) + 1
```

For lazy loading scenarios, use `len(m.entries)` (total count) not `m.loadedCount` (rendered count) to show accurate position within full file.

### Search Mode Handling

**CRITICAL**: When `m.searching=true`, the View() method (lines 316-319) returns early with the search bar. The segmented footer code should ONLY execute when NOT searching:

```go
// Existing search mode handling - DO NOT CHANGE:
if m.searching {
    searchBar := Styles.SearchInput.Render("/" + m.searchInput.View())
    return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), searchBar)
}

// Segmented footer code goes AFTER this check
```

### Files to Modify

| File | Changes |
|------|---------|
| `internal/tui/styles.go` | Add StatusBar.Segment struct with Mode, Position, Shortcuts styles |
| `internal/tui/viewer.go` | Add watchMode field, add buildXSegment() methods, refactor View() footer |

### Files NOT to Modify

- `app.go`, `project.go`, `conversation.go` - Different views, different footers
- `styles_test.go` - No new testable logic (styles are visual)
- `parser/`, `scanner/`, `types/` - No changes needed

### Common Pitfalls

1. **DON'T** use emoji or icons in status bar - text only
2. **DON'T** break search mode - search bar must still appear when m.searching=true
3. **DON'T** modify other views' footers - only viewer.go
4. **DON'T** use `strings.Repeat(" ", ...)` for spacing - use lipgloss Width()
5. **DO** test on both light and dark terminals
6. **DO** test search mode (press /) after implementing

### Testing Strategy

1. **Unit Tests**: `make test` - existing tests must pass (no logic changes)
2. **Manual Testing**: Essential - verify segments visually
3. **Verification**:
   - Status bar shows distinct colored segments
   - Position updates when scrolling
   - Shortcuts are readable
   - Works on light and dark themes
   - LIVE indicator hidden when watchMode=false
   - Search bar still appears when pressing `/`

### Edge Cases

- **Empty log file**: Position shows "0/0"
- **Single entry**: Position shows "1/1"
- **Lazy loading active**: Position based on total entries, not loaded count
- **Search active**: Segmented footer NOT shown (search bar shown instead)

### References

- [internal/tui/viewer.go:316-359] - Current footer implementation
- [internal/tui/styles.go:180-183] - Existing StatusBar style (reference pattern)
- [_bmad-output/project-context.md#Styling Rules] - Color and icon conventions
- [_bmad-output/planning-artifacts/epics.md#Story 1.3] - Acceptance criteria
- [_bmad-output/planning-artifacts/prd.md#FR-103] - Segmented Status Bar requirements
- [internal/tui/styles.go:56-57] - whiteColor for text on colored backgrounds

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

N/A

### Completion Notes List

1. Added `StatusBarSegment` struct to `Styles` with Mode, Position, and Shortcuts styles
2. Mode segment: accentColor background + whiteColor foreground + Bold + Padding(0,1)
3. Position segment: primaryColor background + whiteColor foreground + Padding(0,1)
4. Shortcuts segment: bgAltColor background + textColor foreground + Padding(0,1)
5. Added `watchMode` bool field to ViewerModel (default false for future Story 2.1)
6. Created `buildModeSegment()` - returns "LIVE" when watchMode=true, empty otherwise
7. Created `buildPositionSegment()` - shows "Entry X/Y" using scroll percentage approximation
8. Created `buildShortcutsSegment()` - returns keyboard shortcuts joined by " • "
9. Refactored View() footer to use segmented layout with lipgloss.JoinHorizontal(lipgloss.Top, ...)
10. Preserved search mode behavior - when m.searching=true, search bar shown instead of segmented footer
11. Status info (search results, lazy loading, parse errors) appended to shortcuts segment as suffix

### File List

- `internal/tui/styles.go` - Added StatusBarSegment struct with Mode, Position, Shortcuts styles
- `internal/tui/viewer.go` - Added watchMode field, buildModeSegment(), buildPositionSegment(), buildShortcutsSegment() methods, refactored View() footer

