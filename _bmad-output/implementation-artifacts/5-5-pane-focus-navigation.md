# Story 5.5: Pane Focus Navigation

Status: done

## Story

As a **developer viewing the dashboard**,
I want **to navigate between panes with arrow keys**,
So that **I can select a pane to interact with**.

## Acceptance Criteria

### AC 5.5.1: Arrow key navigation between panes
- **Given** I am in the dashboard view
- **When** I press arrow keys (up/down/left/right)
- **Then** focus moves between panes in that direction

### AC 5.5.2: Visual focus indicator
- **Given** a pane is focused
- **When** displayed
- **Then** it has a distinct visual style (highlighted border)

### AC 5.5.3: Enter opens viewer for focused pane
- **Given** a pane is focused
- **When** I press Enter
- **Then** the viewer opens for that pane's conversation

### AC 5.5.4: Focus wraps at grid edges
- **Given** focus is at grid edge
- **When** I press arrow toward edge
- **Then** focus wraps to opposite side

## Tasks / Subtasks

- [x] Task 1: Verify existing focus tracking state (AC: 5.5.1, 5.5.2)
  - [x] 1.1: Confirm `focusIndex int` exists in DashboardModel (dashboard.go:27) and defaults to 0 in NewDashboardModel() (dashboard.go:297-299) - **NO CODE CHANGES NEEDED**

- [x] Task 2: Implement arrow key handling in DashboardModel.Update() (AC: 5.5.1, 5.5.4)
  - [x] 2.1: Add case for "up", "k" keys - move focus up one row
  - [x] 2.2: Add case for "down", "j" keys - move focus down one row
  - [x] 2.3: Add case for "left", "h" keys - move focus left one column
  - [x] 2.4: Add case for "right", "l" keys - move focus right one column
  - [x] 2.5: Implement grid-aware navigation using calculateGrid(len(panes))
  - [x] 2.6: Implement wrap-around logic for edge cases

- [x] Task 3: Implement focus movement calculation (AC: 5.5.1, 5.5.4)
  - [x] 3.1: Create `moveFocus(direction string) int` method on DashboardModel
  - [x] 3.2: Calculate current row and column from focusIndex: `row = idx / cols, col = idx % cols`
  - [x] 3.3: Handle "up": `row = (row - 1 + rows) % rows` (wraps to bottom)
  - [x] 3.4: Handle "down": `row = (row + 1) % rows` (wraps to top)
  - [x] 3.5: Handle "left": `col = (col - 1 + cols) % cols` (wraps to right)
  - [x] 3.6: Handle "right": `col = (col + 1) % cols` (wraps to left)
  - [x] 3.7: Calculate new index: `newIdx = row * cols + col`
  - [x] 3.8: Clamp to valid pane range: `if newIdx >= len(panes) { newIdx = len(panes) - 1 }`

- [x] Task 4: Add focused pane visual style (AC: 5.5.2)
  - [x] 4.1: In styles.go (after line 414, after existing `PaneHeaderStyle`), add:
    - `PaneFocusedBorderColor = DefaultTheme.Accent`
    - `PaneUnfocusedBorderColor = DefaultTheme.Muted`
  - [x] 4.2: Create `addBorderWithStyle(content string, width int, borderColor lipgloss.Color) string` in utils.go
  - [x] 4.3: Modify existing `addBorder()` to call `addBorderWithStyle(..., PaneUnfocusedBorderColor)`
  - [x] 4.4: Add `ViewWithFocus(focused bool) string` method to PaneModel in dashboard.go
  - [x] 4.5: Modify DashboardModel.View() loop to call `pane.ViewWithFocus(idx == m.focusIndex)` instead of `pane.View()`

- [x] Task 5: Implement Enter key to open viewer (AC: 5.5.3)
  - [x] 5.1: Define `OpenViewerFromDashboardMsg` struct in dashboard.go (near line 89, after other msg types):
    ```go
    type OpenViewerFromDashboardMsg struct {
        FilePath string
        Project  types.Project
    }
    ```
  - [x] 5.2: Handle "enter" key in DashboardModel.Update() (add case in KeyMsg switch around line 334)
  - [x] 5.3: Get focused pane and check conversation exists:
    ```go
    pane := m.panes[m.focusIndex]
    if pane.conversation.FilePath == "" {
        return m, nil // Ignore Enter on empty pane
    }
    return m, func() tea.Msg { return OpenViewerFromDashboardMsg{FilePath: pane.conversation.FilePath, Project: pane.project} }
    ```

- [x] Task 6: Handle app.go integration for viewer navigation (AC: 5.5.3)
  - [x] 6.1: Add `viewerSource NavigationSource` field to AppModel (around line 40, after existing fields)
  - [x] 6.2: Define `NavigationSource` enum (if not exists, add before AppModel struct):
    ```go
    type NavigationSource int
    const (
        FromConversationList NavigationSource = iota
        FromDashboard
    )
    ```
  - [x] 6.3: Add case for `OpenViewerFromDashboardMsg` in AppModel.Update() (around line 225, after GoBackToProjectsFromDashboardMsg):
    ```go
    case OpenViewerFromDashboardMsg:
        m.loading = true
        m.selectedConversation = types.Conversation{FilePath: msg.FilePath}
        m.selectedProject = msg.Project
        m.viewerSource = FromDashboard
        return m, tea.Batch(m.spinner.Tick, m.loadConversation(msg.FilePath))
    ```
  - [x] 6.4: Modify `GoBackMsg` handler (line 204-207) to check viewerSource:
    ```go
    case GoBackMsg:
        if m.viewerSource == FromDashboard {
            m.state = viewDashboard
        } else {
            m.state = viewConversations
        }
        m.viewerSource = FromConversationList // Reset for next navigation
        return m, nil
    ```

- [x] Task 7: Handle incomplete grid and single pane edge cases (AC: 5.5.4)
  - [x] 7.1: Single pane (1x1 grid): Arrow keys have no effect, Enter still works
  - [x] 7.2: 5 projects in 2x3 grid: When calculated index >= len(panes), clamp to len(panes)-1
  - [x] 7.3: Implement clamping in moveFocus(): `if newIdx >= len(m.panes) { newIdx = len(m.panes) - 1 }`
  - [x] 7.4: Test all partial grid scenarios: 1 (1x1), 2 (1x2), 5 (2x3), 7 (3x3)

- [x] Task 8: Add unit tests
  - [x] 8.1: Test arrow key navigation in full grid (4 panes, 2x2)
  - [x] 8.2: Test wrap-around from top-left to bottom-right
  - [x] 8.3: Test wrap-around from bottom-right to top-left
  - [x] 8.4: Test horizontal wrap: left edge to right, right edge to left
  - [x] 8.5: Test vertical wrap: top edge to bottom, bottom edge to top
  - [x] 8.6: Test incomplete grid navigation (5 panes in 2x3)
  - [x] 8.7: Test Enter key emits OpenViewerFromDashboardMsg
  - [x] 8.8: Test Enter with no conversation loaded (should not emit message)
  - [x] 8.9: Test focusIndex bounds checking
  - [x] 8.10: Test PaneModel.ViewWithFocus() renders focus state correctly

- [x] Task 9: Run build, lint, and test validation
  - [x] 9.1: Run `make build` - verify binary builds successfully
  - [x] 9.2: Run `make lint` - no errors
  - [x] 9.3: Run `make test` - all tests pass

- [ ] Task 10: Manual testing (Requires user verification)
  - [ ] 10.1: Open dashboard with 4 projects (2x2 grid)
  - [ ] 10.2: Verify first pane (top-left) is focused on load
  - [ ] 10.3: Press right arrow - verify focus moves to second pane
  - [ ] 10.4: Press down arrow - verify focus moves to fourth pane (bottom-right)
  - [ ] 10.5: Press right arrow - verify focus wraps to third pane (bottom-left)
  - [ ] 10.6: Press up arrow - verify focus moves to first pane (top-left)
  - [ ] 10.7: Verify focused pane has distinct border color (accent)
  - [ ] 10.8: Press Enter on focused pane - verify viewer opens
  - [ ] 10.9: Test with 5 projects (incomplete 2x3 grid)
  - [ ] 10.10: Press Escape from viewer - verify return to dashboard (Story 5.6)

## Dev Notes

### Current Implementation State (Story 5.4 Complete)

**Existing code locations to extend:**
- `DashboardModel` struct: dashboard.go:25-30 (has `focusIndex int`, defaults to 0)
- `DashboardModel.Update()`: dashboard.go:315-501 (add key handlers here)
- `DashboardModel.View()`: dashboard.go:584-621 (modify pane loop here)
- `PaneModel.View()`: dashboard.go:685-771 (add ViewWithFocus variant)
- `addBorder()`: dashboard.go (currently inline, move to styles.go or make configurable)
- `AppModel.Update()`: app.go:77-261 (add OpenViewerFromDashboardMsg handler)
- `GoBackMsg` handler: app.go:204-207 (modify for navigation source)

**What's missing for Story 5.5:**
1. Arrow key handling in DashboardModel.Update() KeyMsg switch
2. `moveFocus(direction string) int` method with wrap-around logic
3. `addBorderWithStyle()` and focus color constants in styles.go
4. `ViewWithFocus(focused bool)` method on PaneModel
5. `OpenViewerFromDashboardMsg` type and handler
6. `NavigationSource` enum and `viewerSource` field in AppModel
7. Modified `GoBackMsg` handler to check navigation source

### Architecture Reference

From `architecture-phase3.md`:
- **FR-505**: Pane Focus Navigation - Navigate between panes using arrow keys
- **Decision 4**: Navigation Context - Enum in AppModel (FromDashboard vs FromConversationList)

### Focus Navigation Algorithm

```
Grid layout for 4 panes (2x2):
  [0] [1]   row 0
  [2] [3]   row 1

Index to grid position:
  row = focusIndex / cols
  col = focusIndex % cols

Movement with wrap-around:
  Up:    row = (row - 1 + rows) % rows
  Down:  row = (row + 1) % rows
  Left:  col = (col - 1 + cols) % cols
  Right: col = (col + 1) % cols

New index:
  newIdx = row * cols + col
  if newIdx >= len(panes) { clamp to last valid pane }
```

### moveFocus() Implementation (dashboard.go, after Update method)

```go
// moveFocus calculates new focus index for given direction.
// Handles wrap-around and clamping for incomplete grids.
func (m *DashboardModel) moveFocus(direction string) int {
    if len(m.panes) <= 1 {
        return 0 // Single pane - no movement
    }

    rows, cols := calculateGrid(len(m.panes))
    row := m.focusIndex / cols
    col := m.focusIndex % cols

    switch direction {
    case "up":
        row = (row - 1 + rows) % rows
    case "down":
        row = (row + 1) % rows
    case "left":
        col = (col - 1 + cols) % cols
    case "right":
        col = (col + 1) % cols
    }

    newIdx := row*cols + col
    // Clamp to valid pane range (handles incomplete last row)
    if newIdx >= len(m.panes) {
        newIdx = len(m.panes) - 1
    }
    return newIdx
}
```

### Arrow Key Handling in Update() (dashboard.go:334+)

```go
case tea.KeyMsg:
    switch msg.String() {
    case "esc", "q":
        m.closeAllWatchers()
        return m, func() tea.Msg { return GoBackToProjectsFromDashboardMsg{} }
    case "up", "k":
        m.focusIndex = m.moveFocus("up")
        return m, nil
    case "down", "j":
        m.focusIndex = m.moveFocus("down")
        return m, nil
    case "left", "h":
        m.focusIndex = m.moveFocus("left")
        return m, nil
    case "right", "l":
        m.focusIndex = m.moveFocus("right")
        return m, nil
    case "enter":
        pane := m.panes[m.focusIndex]
        if pane.conversation.FilePath == "" {
            return m, nil // No conversation to open
        }
        return m, func() tea.Msg {
            return OpenViewerFromDashboardMsg{
                FilePath: pane.conversation.FilePath,
                Project:  pane.project,
            }
        }
    }
```

### Incomplete Grid Handling

For 5 panes in 2x3 grid:
```
  [0] [1] [2]   row 0
  [3] [4] [ ]   row 1 (position 5 is empty)
```

Edge cases:
1. Right from [4]: position 5 is empty, wrap to [3]
2. Down from [2]: position 5 is empty, stay at [2] or move to [4]
3. Right from [2]: wrap to [0]

**Chosen approach:** When calculated position exceeds `len(panes)-1`, clamp to `len(panes)-1`. This is simpler and prevents navigation to non-existent panes.

### Focus Style Implementation

Two approaches:

**Option A: Pass focused flag to PaneModel.View()**
```go
// In DashboardModel.View()
pane := m.panes[idx]
paneView := pane.ViewWithFocus(idx == m.focusIndex)
```

**Option B: Modify addBorder() to accept style**
```go
// New function signature
func addBorderWithStyle(content string, width int, borderColor lipgloss.Color) string

// In PaneModel.View(), receive focused from parent
if focused {
    return addBorderWithStyle(innerContent, p.width, DefaultTheme.Accent)
} else {
    return addBorderWithStyle(innerContent, p.width, DefaultTheme.Muted)
}
```

**Chosen approach:** Option B - cleaner separation, reuses existing border logic.

### Key Bindings

| Key | Action |
|-----|--------|
| Up, k | Move focus up |
| Down, j | Move focus down |
| Left, h | Move focus left |
| Right, l | Move focus right |
| Enter | Open viewer for focused pane |
| Esc, q | Return to project list |

**Note:** Vim-style hjkl navigation aligns with viewer's existing keybindings.

### Message Flow for Enter Key

```
User presses Enter in Dashboard
    ↓
DashboardModel.Update() (dashboard.go:315)
    - case tea.KeyMsg "enter":
    - pane := m.panes[m.focusIndex]
    - if pane.conversation.FilePath == "" { return m, nil }
    - return m, func() tea.Msg { return OpenViewerFromDashboardMsg{...} }
    ↓
AppModel.Update() (app.go:77)
    - case OpenViewerFromDashboardMsg:
    - m.viewerSource = FromDashboard  // CRITICAL: Set before loading
    - m.loading = true
    - return m, tea.Batch(m.spinner.Tick, m.loadConversation(msg.FilePath))
    ↓
conversationLoadedMsg handler (app.go:171)
    - Creates ViewerModel, transitions to viewViewer
    ↓
User presses Escape in Viewer
    ↓
GoBackMsg handler (app.go:204)
    - if m.viewerSource == FromDashboard { m.state = viewDashboard }
    - else { m.state = viewConversations }
    - m.viewerSource = FromConversationList  // Reset
```

### PaneModel ViewWithFocus() Implementation (dashboard.go)

Add new method after existing `View()` method (around line 771):

```go
// ViewWithFocus renders pane with border color based on focus state.
func (p PaneModel) ViewWithFocus(focused bool) string {
    // Guard against invalid dimensions
    if p.width < 4 || p.height < 3 {
        return ""
    }

    // ... (copy existing View() logic for inner content building) ...
    // All lines from View() up to the final addBorder() call

    innerContent := strings.Join(lines, "\n")

    // Use focused or unfocused border color
    if focused {
        return addBorderWithStyle(innerContent, p.width, PaneFocusedBorderColor)
    }
    return addBorderWithStyle(innerContent, p.width, PaneUnfocusedBorderColor)
}
```

**Alternative (simpler):** Keep existing `View()` and add parameter, but this requires updating all callers. The ViewWithFocus approach is safer.

### DashboardModel.View() Modification (dashboard.go:603-608)

Change from:
```go
paneView := pane.View()
```

To:
```go
focused := idx == m.focusIndex
paneView := pane.ViewWithFocus(focused)
```

### addBorder() Modification

Current `addBorder()` is defined inline in dashboard.go. Create configurable version:

```go
// In styles.go (after line 414):
var (
    PaneFocusedBorderColor   = DefaultTheme.Accent  // Amber for focused
    PaneUnfocusedBorderColor = DefaultTheme.Muted   // Gray for unfocused
)

// addBorderWithStyle renders box border with configurable color
func addBorderWithStyle(content string, width int, borderColor lipgloss.AdaptiveColor) string {
    lines := strings.Split(content, "\n")
    innerWidth := width - 2 // Account for left+right border chars

    // Top border
    borderStyle := lipgloss.NewStyle().Foreground(borderColor)
    top := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")

    // Content lines with side borders
    var result []string
    result = append(result, top)
    for _, line := range lines {
        // Pad line to innerWidth
        visualW := lipgloss.Width(line)
        padding := innerWidth - visualW
        if padding < 0 { padding = 0 }
        paddedLine := line + strings.Repeat(" ", padding)
        result = append(result, borderStyle.Render("│")+paddedLine+borderStyle.Render("│"))
    }

    // Bottom border
    bottom := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
    result = append(result, bottom)

    return strings.Join(result, "\n")
}

// addBorder uses default unfocused color (backward compatible)
func addBorder(content string, width int) string {
    return addBorderWithStyle(content, width, PaneUnfocusedBorderColor)
}
```

### styles.go Additions (Location: After line 414)

```go
// Pane focus styles (Story 5.5)
var (
    PaneFocusedBorderColor   = DefaultTheme.Accent  // Amber: Light #D97706, Dark #F59E0B
    PaneUnfocusedBorderColor = DefaultTheme.Muted   // Gray: Light #6B7280, Dark #9CA3AF
)
```

**Note:** `addBorderWithStyle()` and `addBorder()` functions should also be added to styles.go (see addBorder() Modification section above). Move existing `addBorder()` from dashboard.go to styles.go for consistency.

### Files to Modify

| File | Action | Location | Description |
|------|--------|----------|-------------|
| `internal/tui/dashboard.go` | MODIFY | Line 89+ | Add `OpenViewerFromDashboardMsg` struct |
| `internal/tui/dashboard.go` | MODIFY | Line 315+ | Add arrow key + enter handlers in Update() |
| `internal/tui/dashboard.go` | MODIFY | After Update() | Add `moveFocus(direction string) int` method |
| `internal/tui/dashboard.go` | MODIFY | Line 685+ | Add `ViewWithFocus(focused bool) string` method |
| `internal/tui/dashboard.go` | MODIFY | Line 603-608 | Modify View() to call ViewWithFocus() |
| `internal/tui/styles.go` | MODIFY | Line 414+ | Add `PaneFocusedBorderColor`, `PaneUnfocusedBorderColor` |
| `internal/tui/styles.go` | ADD | After colors | Add `addBorderWithStyle()` function |
| `internal/tui/app.go` | MODIFY | Line 28+ | Add `NavigationSource` enum and `viewerSource` field |
| `internal/tui/app.go` | MODIFY | Line 225+ | Add `OpenViewerFromDashboardMsg` handler |
| `internal/tui/app.go` | MODIFY | Line 204-207 | Modify `GoBackMsg` handler to check viewerSource |
| `internal/tui/dashboard_test.go` | MODIFY | EOF | Add tests for navigation, focus, Enter key |

### Project Context Rules (from project-context.md)

| Rule | Application |
|------|-------------|
| **NO EMOJI IN UI** | Focus indicator uses border color, not emoji |
| **TEA pattern** | Key events handled in Update(), state changes returned |
| **Use Makefile** | `make build`, `make test` |
| **Vim keybindings** | Support hjkl in addition to arrow keys |

### Previous Story Learnings

From Story 5.4:
1. **Two-watcher architecture**: Each pane has file + directory watcher
2. **TEA pattern for async**: Commands for blocking operations
3. **Graceful degradation**: Continue on errors

From Story 5.3:
1. **Pre-render content**: Render in Update(), cache in content field
2. **Manual border control**: Use addBorder() for reliable height

From Story 5.2:
1. **Grid calculation**: calculateGrid() returns rows, cols
2. **Dimension calculation**: calculatePaneDimensions() for pane sizing

### Git Commit Pattern

Recent commits show pattern:
- `feat: implement pane focus navigation with arrow keys (Story 5.5)`
- `feat: add focused pane visual indicator (Story 5.5)`
- `feat: open viewer from dashboard on Enter (Story 5.5)`

### OpenViewerFromDashboardMsg Definition (dashboard.go:89+)

```go
// OpenViewerFromDashboardMsg signals request to open viewer from dashboard.
// Handled by AppModel to load conversation and transition to viewer.
type OpenViewerFromDashboardMsg struct {
    FilePath string        // Full path to conversation JSONL file
    Project  types.Project // Project for building viewer title
}
```

### NavigationSource Enum Definition (app.go, before AppModel struct)

```go
// NavigationSource tracks where the viewer was opened from.
// Used by GoBackMsg handler to return to correct parent view.
type NavigationSource int

const (
    FromConversationList NavigationSource = iota // Default: viewer opened from conversation list
    FromDashboard                                // Viewer opened from dashboard pane
)
```

### AppModel Extension (app.go:28+)

```go
type AppModel struct {
    // ... existing fields ...
    viewerSource NavigationSource // Tracks where viewer was opened from (Story 5.5)
}
```

### References

- [Source: epics-phase3.md#Story-5.5] - Acceptance criteria (lines 346-369)
- [Source: prd-phase3.md#FR-505] - Pane focus navigation requirements
- [Source: architecture-phase3.md#Decision-4] - Navigation context enum (line 124)
- [Source: internal/tui/dashboard.go] - Current dashboard implementation
  - DashboardModel struct: lines 25-30
  - Update(): lines 315-501
  - View(): lines 584-621
  - PaneModel.View(): lines 685-771
  - calculateGrid(): lines 650-667
- [Source: internal/tui/app.go] - App routing
  - viewState enum: lines 17-24
  - AppModel struct: lines 26-40
  - GoBackMsg handler: lines 204-207
  - GoBackToProjectsFromDashboardMsg handler: lines 218-224
- [Source: internal/tui/styles.go] - Styling patterns
  - DefaultTheme: lines 49-78
  - Pane styles: lines 402-414
- [Source: 5-4-new-conversation-detection.md] - Previous story patterns
- [Source: project-context.md] - Code rules and patterns

## Implementation Checklist

Before marking story complete, verify:

**Focus Tracking:**
- [ ] `focusIndex` used for tracking focused pane (already exists at dashboard.go:27)
- [ ] Default focus is 0 (top-left pane) (already implemented at dashboard.go:299)

**Arrow Key Navigation:**
- [ ] Up/k moves focus up with wrap
- [ ] Down/j moves focus down with wrap
- [ ] Left/h moves focus left with wrap
- [ ] Right/l moves focus right with wrap
- [ ] Navigation respects grid layout (uses calculateGrid())
- [ ] `moveFocus(direction string) int` method implemented

**Focus Visual:**
- [ ] `PaneFocusedBorderColor` defined in styles.go (Accent color)
- [ ] `PaneUnfocusedBorderColor` defined in styles.go (Muted color)
- [ ] `addBorderWithStyle()` function created
- [ ] `ViewWithFocus(focused bool)` method on PaneModel
- [ ] DashboardModel.View() calls ViewWithFocus() with correct focused state

**Enter Key:**
- [ ] `OpenViewerFromDashboardMsg` struct defined in dashboard.go
- [ ] "enter" key handled in DashboardModel.Update()
- [ ] Empty pane (no conversation) silently ignored (no crash)
- [ ] Message emitted with FilePath and Project

**App Integration:**
- [ ] `NavigationSource` enum defined in app.go
- [ ] `viewerSource` field added to AppModel
- [ ] `OpenViewerFromDashboardMsg` handled in AppModel.Update()
- [ ] `viewerSource = FromDashboard` set before loading
- [ ] `GoBackMsg` handler checks viewerSource
- [ ] Returns to Dashboard when viewerSource == FromDashboard
- [ ] Returns to ConversationList when viewerSource == FromConversationList
- [ ] viewerSource reset to FromConversationList after navigation

**Edge Cases:**
- [ ] Single pane (1x1): Arrow keys no-op, Enter works
- [ ] Incomplete grid (5 panes in 2x3): Focus clamped to valid indices
- [ ] Focus never exceeds len(panes)-1

**Testing:**
- [ ] Unit tests for moveFocus() all directions
- [ ] Unit tests for wrap-around at edges
- [ ] Unit tests for incomplete grid clamping
- [ ] Unit tests for Enter key with/without conversation
- [ ] Unit tests for single pane navigation
- [ ] `make build` succeeds
- [ ] `make lint` has no errors
- [ ] `make test` passes

**Manual Verification (Requires User):**
- [ ] Arrow navigation works correctly
- [ ] Focus wraps at edges
- [ ] Focused pane border is accent color (amber)
- [ ] Unfocused panes have muted border (gray)
- [ ] Enter opens viewer
- [ ] Escape from viewer returns to Dashboard (not conversation list)
- [ ] Works with 1, 2, 4, 5, and 9 pane configurations

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

None

### Completion Notes List

1. **Task 1-3**: Focus tracking and movement logic implemented in `dashboard.go`. The `moveFocus()` method handles wrap-around navigation and clamping for incomplete grids.

2. **Task 4**: Focus visual style implemented via `addBorderWithStyle()` in `utils.go` with color constants in `styles.go`. Focused pane uses amber (Accent), unfocused uses gray (Muted).

3. **Task 5-6**: Enter key handling and app.go integration complete. `OpenViewerFromDashboardMsg` triggers viewer load, `NavigationSource` enum tracks origin for GoBackMsg routing.

4. **Task 7**: Edge cases handled - single pane returns 0 for all navigation, incomplete grids clamp to last valid index.

5. **Task 8**: Comprehensive unit tests added in `dashboard_test.go` (Story 5.5 section) and `app_test.go` (navigation source tests).

6. **Build/Test Validation**: `make build` successful, `make lint` 0 issues, `make test` all pass.

### File List

| File | Changes |
|------|---------|
| `internal/tui/dashboard.go` | Added `OpenViewerFromDashboardMsg` struct, arrow/enter key handlers in Update(), `moveFocus()` method, `ViewWithFocus()` method on PaneModel, modified View() to use ViewWithFocus() |
| `internal/tui/styles.go` | Added `PaneFocusedBorderColor` and `PaneUnfocusedBorderColor` constants |
| `internal/tui/utils.go` | Added `addBorderWithStyle()` function, modified `addBorder()` to use it |
| `internal/tui/app.go` | Added `NavigationSource` enum, `viewerSource` field in AppModel, `OpenViewerFromDashboardMsg` handler, modified `GoBackMsg` handler |
| `internal/tui/dashboard_test.go` | Added Story 5.5 tests: moveFocus, arrow/vim navigation, Enter key, ViewWithFocus, bounds checking, addBorder delegation tests |
| `internal/tui/app_test.go` | **NEW FILE** - Tests for NavigationSource enum and navigation routing |
| `_bmad-output/implementation-artifacts/sprint-status.yaml` | Sprint status tracking (modified during review) |
