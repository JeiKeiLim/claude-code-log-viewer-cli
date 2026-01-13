---
title: 'String Width & List Navigation Fixes'
slug: 'stringwidth-and-viewport-list'
created: '2026-01-13'
status: 'completed'
stepsCompleted: [1, 2, 3, 4]
implementationDate: '2026-01-13'
reviewDate: '2026-01-13'
reviewNotes: |
  - Adversarial review completed (14 findings)
  - 4 fixed (newline bug, F5 bounds check, F8 char count, F10 test assertions)
  - 1 false positive (F1 - standard Bubbletea pattern)
  - 9 skipped (noise/pre-existing/out-of-scope)
tech_stack:
  - 'Go 1.24.3'
  - 'Bubbletea v1.3.10 (tea.Model, tea.Cmd, tea.Msg)'
  - 'Lipgloss v1.1.1 (lipgloss.Width for visual width)'
  - 'Bubbles v0.21.0 (viewport.Model - reference in viewer.go)'
files_to_modify:
  - 'internal/tui/stringwidth.go (NEW)'
  - 'internal/tui/listviewport.go (NEW)'
  - 'internal/tui/conversation.go (REFACTOR)'
  - 'internal/tui/project.go (REFACTOR)'
  - 'internal/tui/utils.go (UPDATE)'
  - 'internal/tui/viewer.go (AUDIT - minor fix)'
  - 'internal/tui/app.go (AUDIT - minor fix)'
  - 'internal/tui/stringwidth_test.go (NEW)'
  - 'internal/tui/listviewport_test.go (NEW)'
code_patterns:
  - 'Elm Architecture: Model-Update-View'
  - 'Message-based state updates'
  - 'tea.Cmd for async operations'
  - 'Viewport reference: viewer.go SetSize pattern'
test_patterns:
  - 'Table-driven tests required per project-context.md'
  - '90% coverage target per project-context.md'
  - 'Test Update() state transitions, trust View() rendering'
---

# Tech-Spec: String Width & List Navigation Fixes

**Created:** 2026-01-13
**Status:** Implementation Complete

## Overview

### Problem Statement

Two interconnected bugs affecting the TUI:

1. **CJK Character Alignment Bug**: Right border misaligns when conversation preview contains Korean (or other CJK) characters. Root cause: code uses `len()` (byte count) instead of visual column width. Korean characters are 3 bytes in UTF-8 but only 2 display columns.

2. **List Navigation Bug**: With large conversation lists, pressing down/j at the visual bottom doesn't scroll immediately - requires ~10 extra presses. Root cause: `bubbles/list` component has a known bug where `list.View()` outputs more lines than `SetSize()` specifies. Manual truncation creates mismatch between list's internal pagination state and actual displayed content.

### Solution

1. **String Width System**: Create centralized `stringwidth.go` utility module using `lipgloss.Width()` for ALL visual width calculations. Audit and replace all `len()` usage for display strings.

2. **Viewport-Based List**: Replace `bubbles/list` with `bubbles/viewport` + custom item rendering. Viewport strictly respects height (no output bug). Full control over cursor, selection, scrolling, and pagination.

### Scope

**In Scope:**
- New `internal/tui/stringwidth.go` utility module
- New `internal/tui/listviewport.go` generic list component
- Refactor `ConversationModel` to use ListViewport
- Refactor `ProjectModel` to use ListViewport
- Audit all string width operations across TUI package
- Unit tests for stringwidth utilities
- Unit tests for ListViewport state transitions

**Out of Scope:**
- `viewer.go` major changes (already uses viewport correctly)
- Adding new dependencies (using existing Charm stack)
- New features beyond bug fixes
- Changes to parser/scanner packages

## Context for Development

### Codebase Patterns

**Elm Architecture (Model-Update-View):**
```go
type Model struct { /* state */ }
func (m Model) Init() tea.Cmd { return nil }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { /* handle messages */ }
func (m Model) View() string { /* render UI */ }
```

**Viewport Pattern (from viewer.go - REFERENCE IMPLEMENTATION):**
```go
type ViewerModel struct {
    viewport viewport.Model
    width    int
    height   int
    ready    bool
}

func (m *ViewerModel) SetSize(width, height int) {
    headerHeight := 1
    footerHeight := 1
    verticalMargins := headerHeight + footerHeight

    if !m.ready {
        m.viewport = viewport.New(width, height-verticalMargins)
        m.viewport.YPosition = headerHeight
        m.ready = true
    } else {
        m.viewport.Width = width
        m.viewport.Height = height - verticalMargins
    }
}
```

**Navigation Pattern (from viewer.go):**
- j/k: LineDown/LineUp (single line)
- d/u: HalfViewDown/HalfViewUp
- Space/pgdown: ViewDown (full page)
- g/G: GotoTop/GotoBottom (gg sequence detection for top)

### Files to Reference

| File | Purpose |
| ---- | ------- |
| `internal/tui/viewer.go` | Reference viewport implementation - copy this pattern |
| `internal/tui/styles.go` | All styles, lazy load config, text icons |
| `internal/tui/utils.go` | `addBorder()` correctly uses `lipgloss.Width()` |
| `docs/lessons-learned.md` | Documents the list height bug |
| `_bmad-output/project-context.md` | 90% test coverage, no emoji rule |

### Technical Decisions

1. **Generic ListViewport[T]**: Use Go generics for reusable component
2. **Interface-based items**: `ListItem` interface with `Render()` method
3. **Cursor in state**: Explicit cursor management (not hidden in list component)
4. **Trust viewport**: Test `Update()` logic, trust viewport for `View()` rendering
5. **lipgloss.Width everywhere**: Single source of truth for visual width

## len() Audit Results

### PROBLEMATIC (must fix - display width):

| File | Line | Code | Issue |
|------|------|------|-------|
| `conversation.go` | 91 | `availWidth - len(metaPrefix)` | Bytes vs columns |
| `conversation.go` | 95 | `if len(preview) > previewMaxLen` | Bytes vs columns |
| `project.go` | 64 | `if len(path) > availWidth` | Bytes vs columns |
| `utils.go` | 22 | `if len(s) <= maxLen` | truncateString uses bytes |
| `viewer.go` | 475 | `if len(inputStr) > 200` | Tool input truncation |
| `app.go` | 128 | `if len(modelShort) > 20` | Model name truncation |

### SAFE (counting items, not display):

- `len(projects)`, `len(conversations)`, `len(entries)` - counting items
- `len(m.searchMatches)` - counting matches
- `len(lines)` - counting lines for truncation
- `len(input)` - checking empty map

## Implementation Plan

### Tasks

#### Phase 1: String Width Foundation

- [x] **Task 1: Create `internal/tui/stringwidth.go`**
  - File: `internal/tui/stringwidth.go` (NEW)
  - Action: Create utility module with visual width functions
  - Implementation:
    ```go
    package tui

    import "github.com/charmbracelet/lipgloss"

    // VisualWidth returns the visual column width of a string.
    // CJK characters count as 2 columns, ASCII as 1.
    func VisualWidth(s string) int {
        return lipgloss.Width(s)
    }

    // TruncateToWidth truncates a string to fit within maxWidth visual columns.
    // Adds "..." suffix if truncated.
    func TruncateToWidth(s string, maxWidth int) string {
        if maxWidth <= 3 {
            return s[:min(len(s), maxWidth)]
        }
        if VisualWidth(s) <= maxWidth {
            return s
        }
        runes := []rune(s)
        for i := len(runes) - 1; i >= 0; i-- {
            candidate := string(runes[:i]) + "..."
            if VisualWidth(candidate) <= maxWidth {
                return candidate
            }
        }
        return "..."
    }

    // TruncateFromLeftToWidth truncates from the left, keeping the right portion.
    // Adds "..." prefix if truncated. Useful for paths.
    func TruncateFromLeftToWidth(s string, maxWidth int) string {
        if maxWidth <= 3 {
            return "..."
        }
        if VisualWidth(s) <= maxWidth {
            return s
        }
        runes := []rune(s)
        for i := 1; i < len(runes); i++ {
            candidate := "..." + string(runes[i:])
            if VisualWidth(candidate) <= maxWidth {
                return candidate
            }
        }
        return "..."
    }

    // PadToWidth pads a string with spaces to reach exact visual width.
    func PadToWidth(s string, width int) string {
        currentWidth := VisualWidth(s)
        if currentWidth >= width {
            return s
        }
        return s + strings.Repeat(" ", width-currentWidth)
    }
    ```

- [x] **Task 2: Create `internal/tui/stringwidth_test.go`**
  - File: `internal/tui/stringwidth_test.go` (NEW)
  - Action: Table-driven tests for all stringwidth functions
  - Test cases:
    - ASCII only: "hello" (5 cols)
    - Korean: "안녕" (4 cols, 6 bytes)
    - Japanese: "日本語" (6 cols)
    - Mixed: "Hello안녕" (9 cols)
    - Emoji: "👍" (2 cols)
    - Empty string
    - Exactly at width
    - One over width
    - Truncation with CJK at boundary

- [x] **Task 3: Update `internal/tui/utils.go`**
  - File: `internal/tui/utils.go`
  - Action: Replace `truncateString()` with `TruncateToWidth()`, update `truncateFromLeft()` to use `TruncateFromLeftToWidth()`
  - Changes:
    - Delete `truncateString()` function (lines 20-29)
    - Update `truncateFromLeft()` to call `TruncateFromLeftToWidth()` or delete if redundant

#### Phase 2: ListViewport Component

- [x] **Task 4: Create `internal/tui/listviewport.go`**
  - File: `internal/tui/listviewport.go` (NEW)
  - Action: Create generic viewport-based list component
  - Implementation:
    ```go
    package tui

    import (
        "strings"
        "time"

        "github.com/charmbracelet/bubbles/viewport"
        tea "github.com/charmbracelet/bubbletea"
    )

    // ListItem interface for renderable list items
    type ListItem interface {
        Render(width int, selected bool) string
        FilterValue() string
    }

    // ListViewport is a viewport-based list with explicit cursor control
    type ListViewport[T ListItem] struct {
        viewport    viewport.Model
        items       []T
        cursor      int
        width       int
        height      int
        itemHeight  int  // Lines per item (typically 2)
        ready       bool

        // gg key detection (from viewer.go pattern)
        lastKeyG     bool
        lastKeyGTime time.Time
    }

    // NewListViewport creates a new list viewport
    func NewListViewport[T ListItem](items []T, itemHeight int) ListViewport[T] {
        return ListViewport[T]{
            items:      items,
            itemHeight: itemHeight,
            cursor:     0,
        }
    }

    // SetItems updates the item list
    func (m *ListViewport[T]) SetItems(items []T) {
        m.items = items
        if m.cursor >= len(items) {
            m.cursor = max(0, len(items)-1)
        }
        m.updateContent()
    }

    // SetSize sets the viewport dimensions
    func (m *ListViewport[T]) SetSize(width, height int) {
        m.width = width
        m.height = height

        if !m.ready {
            m.viewport = viewport.New(width, height)
            m.ready = true
        } else {
            m.viewport.Width = width
            m.viewport.Height = height
        }
        m.updateContent()
    }

    // Cursor returns current cursor position
    func (m ListViewport[T]) Cursor() int { return m.cursor }

    // SelectedItem returns the currently selected item
    func (m ListViewport[T]) SelectedItem() (T, bool) {
        if m.cursor >= 0 && m.cursor < len(m.items) {
            return m.items[m.cursor], true
        }
        var zero T
        return zero, false
    }

    // ItemCount returns total items
    func (m ListViewport[T]) ItemCount() int { return len(m.items) }

    // Update handles keyboard navigation
    func (m ListViewport[T]) Update(msg tea.Msg) (ListViewport[T], tea.Cmd) {
        switch msg := msg.(type) {
        case tea.KeyMsg:
            keyStr := msg.String()

            // gg sequence detection
            if keyStr == "g" {
                if m.lastKeyG && time.Since(m.lastKeyGTime) < 500*time.Millisecond {
                    m.cursor = 0
                    m.lastKeyG = false
                    m.updateContent()
                    m.viewport.GotoTop()
                    return m, nil
                }
                m.lastKeyG = true
                m.lastKeyGTime = time.Now()
                return m, nil
            }
            m.lastKeyG = false

            switch keyStr {
            case "j", "down":
                if m.cursor < len(m.items)-1 {
                    m.cursor++
                    m.ensureCursorVisible()
                }
            case "k", "up":
                if m.cursor > 0 {
                    m.cursor--
                    m.ensureCursorVisible()
                }
            case "G":
                m.cursor = len(m.items) - 1
                m.updateContent()
                m.viewport.GotoBottom()
            case "d", "ctrl+d":
                m.viewport.HalfViewDown()
                m.syncCursorToViewport()
            case "u", "ctrl+u":
                m.viewport.HalfViewUp()
                m.syncCursorToViewport()
            case "pgdown", " ":
                m.viewport.ViewDown()
                m.syncCursorToViewport()
            case "pgup":
                m.viewport.ViewUp()
                m.syncCursorToViewport()
            }
        }

        var cmd tea.Cmd
        m.viewport, cmd = m.viewport.Update(msg)
        return m, cmd
    }

    // View renders the list
    func (m ListViewport[T]) View() string {
        if !m.ready {
            return "Loading..."
        }
        return m.viewport.View()
    }

    // updateContent re-renders all items into the viewport
    func (m *ListViewport[T]) updateContent() {
        if !m.ready {
            return
        }
        var content strings.Builder
        for i, item := range m.items {
            rendered := item.Render(m.width, i == m.cursor)
            content.WriteString(rendered)
            if i < len(m.items)-1 {
                content.WriteString("\n")
            }
        }
        m.viewport.SetContent(content.String())
    }

    // ensureCursorVisible scrolls viewport to keep cursor in view
    func (m *ListViewport[T]) ensureCursorVisible() {
        m.updateContent()
        cursorLine := m.cursor * m.itemHeight
        viewTop := m.viewport.YOffset
        viewBottom := viewTop + m.viewport.Height

        if cursorLine < viewTop {
            m.viewport.SetYOffset(cursorLine)
        } else if cursorLine+m.itemHeight > viewBottom {
            m.viewport.SetYOffset(cursorLine + m.itemHeight - m.viewport.Height)
        }
    }

    // syncCursorToViewport updates cursor based on viewport scroll position
    func (m *ListViewport[T]) syncCursorToViewport() {
        if len(m.items) == 0 {
            return
        }
        // Set cursor to first visible item
        firstVisible := m.viewport.YOffset / m.itemHeight
        if firstVisible >= len(m.items) {
            firstVisible = len(m.items) - 1
        }
        if firstVisible < 0 {
            firstVisible = 0
        }
        m.cursor = firstVisible
        m.updateContent()
    }
    ```

- [x] **Task 5: Create `internal/tui/listviewport_test.go`**
  - File: `internal/tui/listviewport_test.go` (NEW)
  - Action: Table-driven tests for ListViewport state transitions
  - Test cases:
    - Cursor navigation: j/k at bounds
    - Cursor at 0, press k → stays at 0
    - Cursor at last, press j → stays at last
    - G goes to end
    - gg goes to start
    - SetItems preserves cursor if valid
    - SetItems clamps cursor if out of bounds

#### Phase 3: Refactor Conversation List

- [x] **Task 6: Refactor `internal/tui/conversation.go`**
  - File: `internal/tui/conversation.go`
  - Action: Replace `list.Model` with `ListViewport[ConversationItem]`
  - Changes:
    1. Remove `bubbles/list` import
    2. Change `list list.Model` to `listViewport ListViewport[ConversationItem]`
    3. Update `ConversationItem.Render(width int, selected bool) string` method
    4. Use `TruncateToWidth()` for preview (fixes CJK bug)
    5. Use `VisualWidth()` for width calculations
    6. Remove `truncateConvLines()` - viewport handles height
    7. Remove `ConversationItemDelegate` - rendering in item itself
    8. Update `SetSize()` to call `listViewport.SetSize()`
    9. Update `Update()` to delegate to `listViewport.Update()`
    10. Update `View()` to use `listViewport.View()` + header/footer

- [x] **Task 7: Update ConversationItem.Render()**
  - File: `internal/tui/conversation.go`
  - Action: Move delegate rendering logic to item method
  - Implementation:
    ```go
    func (i ConversationItem) Render(width int, selected bool) string {
        var prefix string
        var titleStyle, descStyle lipgloss.Style
        if selected {
            prefix = " > "
            titleStyle = Styles.Selected
            descStyle = Styles.Selected.Background(lipgloss.Color("#4C1D95"))
        } else {
            prefix = "   "
            titleStyle = Styles.Normal.Bold(true)
            descStyle = Styles.Muted
        }

        availWidth := width - VisualWidth(prefix) - 2  // -2 for border

        timestamp := formatTimestamp(i.conversation.LastModified)
        title := titleStyle.Render(timestamp)

        // Build description with proper width calculation
        preview := i.conversation.FirstUserMessage
        if preview == "" {
            preview = "(no preview)"
        }

        durationStr := formatDuration(i.conversation.Duration)
        if i.conversation.Duration == 0 {
            durationStr = "<1m"
        }

        metaPrefix := fmt.Sprintf("%d msgs • %s • ", i.conversation.MessageCount, durationStr)
        metaWidth := VisualWidth(metaPrefix)
        previewMaxWidth := availWidth - metaWidth

        if previewMaxWidth < 10 {
            previewMaxWidth = 10
        }
        preview = TruncateToWidth(preview, previewMaxWidth)

        desc := descStyle.Render(metaPrefix + preview)

        return fmt.Sprintf("%s%s\n   %s", prefix, title, desc)
    }
    ```

#### Phase 4: Refactor Project List

- [x] **Task 8: Refactor `internal/tui/project.go`**
  - File: `internal/tui/project.go`
  - Action: Replace `list.Model` with `ListViewport[ProjectItem]`
  - Changes: Same pattern as conversation.go
    1. Remove `bubbles/list` import
    2. Change to `listViewport ListViewport[ProjectItem]`
    3. Update `ProjectItem.Render()` method
    4. Use `TruncateFromLeftToWidth()` for paths
    5. Use `VisualWidth()` for width calculations
    6. Remove `truncateToLines()`
    7. Remove `ProjectItemDelegate`
    8. Keep filter functionality (separate from list)

- [x] **Task 9: Update ProjectItem.Render()**
  - File: `internal/tui/project.go`
  - Action: Move delegate rendering logic to item method
  - Implementation:
    ```go
    func (i ProjectItem) Render(width int, selected bool) string {
        var prefix string
        var titleStyle, descStyle lipgloss.Style
        if selected {
            prefix = " > "
            titleStyle = Styles.Selected
            descStyle = Styles.Selected.Background(lipgloss.Color("#4C1D95"))
        } else {
            prefix = "   "
            titleStyle = Styles.Normal.Bold(true)
            descStyle = Styles.Muted
        }

        availWidth := width - VisualWidth(prefix) - 2

        title := titleStyle.Render(i.project.DisplayName)
        path := TruncateFromLeftToWidth(i.project.DecodedPath, availWidth)
        desc := descStyle.Render(path)

        return fmt.Sprintf("%s%s\n   %s", prefix, title, desc)
    }
    ```

#### Phase 5: Fix Remaining len() Issues

- [x] **Task 10: Fix `internal/tui/viewer.go` tool input truncation**
  - File: `internal/tui/viewer.go`
  - Line: 475-476
  - Action: Replace byte-based truncation with visual width
  - Change:
    ```go
    // Before:
    if len(inputStr) > 200 {
        inputStr = inputStr[:200] + fmt.Sprintf("... (%d chars total)", len(inputStr))
    }

    // After:
    if VisualWidth(inputStr) > 200 {
        inputStr = TruncateToWidth(inputStr, 197) + fmt.Sprintf(" (%d chars)", len(inputStr))
    }
    ```

- [x] **Task 11: Fix `internal/tui/app.go` model name truncation**
  - File: `internal/tui/app.go`
  - Line: 128
  - Action: Replace byte-based check with visual width
  - Change:
    ```go
    // Before:
    if len(modelShort) > 20 {

    // After:
    if VisualWidth(modelShort) > 20 {
    ```

#### Phase 6: Cleanup & Documentation

- [x] **Task 12: Update `docs/lessons-learned.md`**
  - File: `docs/lessons-learned.md`
  - Action: Add section documenting the viewport-based list solution
  - Content: Document why we moved from `bubbles/list` to viewport

- [x] **Task 13: Run `make test` and fix any issues**
  - Action: Ensure all tests pass
  - Command: `make test`

- [x] **Task 14: Run `make build` and manual verification**
  - Action: Build and manually test
  - Verification:
    1. Navigate project list with Korean project names
    2. Navigate conversation list with Korean preview text
    3. Verify borders align correctly
    4. Test large list navigation (100+ items) - no stuck cursor

### Acceptance Criteria

#### String Width (Bug 1 Fix)

- [ ] **AC1**: Given a conversation with Korean preview text "안녕하세요 테스트", when displayed in the list, then the right border aligns correctly with other items.

- [ ] **AC2**: Given a project with a path containing Korean characters, when displayed in the list, then the path truncation respects visual width and border aligns.

- [ ] **AC3**: Given `VisualWidth("안녕")`, when called, then returns 4 (not 6).

- [ ] **AC4**: Given `TruncateToWidth("Hello안녕World", 10)`, when called, then returns a string with visual width <= 10 ending in "...".

#### List Navigation (Bug 2 Fix)

- [ ] **AC5**: Given a conversation list with 100+ items, when cursor is at the visual bottom and user presses 'j', then the list scrolls immediately to show next item.

- [ ] **AC6**: Given a project list with 50+ items, when user presses 'G', then cursor moves to last item and it's visible.

- [ ] **AC7**: Given a conversation list, when user presses 'gg' quickly, then cursor moves to first item.

- [ ] **AC8**: Given a list with items, when user navigates with j/k, then exactly one item is highlighted at all times.

#### Regression

- [ ] **AC9**: Given the existing viewer (viewer.go), when viewing a conversation, then all existing functionality (search, toggle thinking, navigation) works unchanged.

- [ ] **AC10**: Given project filtering with '/', when user types filter text, then projects are filtered correctly.

## Additional Context

### Dependencies

- No new dependencies required
- Uses existing Charm stack: Bubbletea, Lipgloss, Bubbles (viewport)
- `lipgloss.Width()` internally uses `go-runewidth` (transitive dependency)

### Testing Strategy

**Unit Tests:**
- `stringwidth_test.go`: 95%+ coverage target
  - All functions with ASCII, CJK, mixed, empty, edge cases
  - Table-driven tests per project-context.md

- `listviewport_test.go`: 90%+ coverage on Update()
  - Cursor bounds (0, middle, end)
  - Navigation keys (j, k, G, gg)
  - SetItems cursor preservation
  - View() excluded from coverage (trust viewport)

**Manual Testing:**
1. Create/find project with Korean name
2. Create conversation with Korean first message
3. Navigate lists and verify border alignment
4. Test with 100+ conversations - verify no navigation sticking

### Notes

**High-Risk Items:**
- ListViewport cursor sync with viewport scroll position
- Filter functionality in ProjectModel (needs careful integration)

**Known Limitations:**
- Emoji rendering varies by terminal (not fixing, out of scope)
- Exact CJK width depends on font (lipgloss.Width is best effort)

**Future Considerations:**
- Could add fuzzy search to lists (out of scope)
- Could add vim-style / search in lists (out of scope)
