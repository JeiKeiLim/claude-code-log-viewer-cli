# Lessons Learned

## Bubbletea List Component Height Issue

**Date**: 2026-01-12
**Project**: claude-code-log-viewer-cli

### Problem

The bubbles `list` component's `SetSize(width, height)` does not strictly control the output height of `list.View()`.

### Symptoms

- Top of the screen gets clipped
- Content appears to scroll off the top of the terminal
- Issue only affects views using `list` component, not `viewport` component

### Root Cause

When calling `list.SetSize(width, height)`:
- Expected: `list.View()` outputs exactly `height` lines
- Actual: `list.View()` outputs MORE lines than specified

Example from debugging:
```
Terminal height: 65
List SetSize: 63 (height - 2 for header/footer)
Actual list output: 74 lines (11 extra!)
Total output: 76 lines
Result: Top 11 lines clipped
```

### Why Viewport Works

The `viewport` component strictly respects the height parameter. When you set `viewport.Height = N`, it outputs exactly N lines.

### Solutions

**Option 1: Manual truncation (recommended)**
```go
func truncateToLines(s string, n int) string {
    if n <= 0 {
        return ""
    }
    lines := strings.Split(s, "\n")
    if len(lines) <= n {
        return s
    }
    return strings.Join(lines[:n], "\n")
}

// Usage
listView := truncateToLines(m.list.View(), m.height - 2)
```

**Option 2: Use viewport instead of list**

Wrap your content in a viewport for strict height control.

**Option 3: lipgloss.Height() (unreliable)**
```go
// This didn't work reliably in our testing
listView := lipgloss.NewStyle().Height(listHeight).Render(m.list.View())
```

### Additional Notes

- When managing your own header/footer outside the list, disable the list's built-in chrome:
  ```go
  l.SetShowTitle(false)
  l.SetShowStatusBar(false)
  l.SetShowHelp(false)
  ```

- Always verify actual output height during debugging:
  ```go
  listLines := strings.Count(listView, "\n") + 1
  footer := fmt.Sprintf("h:%d list:%d", m.height, listLines)
  ```

### References

- Bubbles list: https://github.com/charmbracelet/bubbles/tree/master/list
- Bubbles viewport: https://github.com/charmbracelet/bubbles/tree/master/viewport

---

## Viewport-Based ListViewport Solution

**Date**: 2026-01-13
**Project**: claude-code-log-viewer-cli

### Problem

The manual truncation approach (Option 1 above) caused navigation issues: the cursor position in the list component would desync from the visible truncated output, making navigation appear "stuck" when scrolling large lists.

### Solution: ListViewport Component

We replaced `bubbles/list` entirely with a custom `ListViewport[T]` generic component that wraps `bubbles/viewport`:

```go
// ListItem interface for renderable items
type ListItem interface {
    Render(width int, selected bool) string
    FilterValue() string
}

// ListViewport wraps viewport for strict height control
type ListViewport[T ListItem] struct {
    viewport   viewport.Model
    items      []T
    cursor     int
    itemHeight int // Lines per item (typically 2)
    // ...
}
```

### Benefits

1. **Strict height control**: Viewport respects `SetSize()` exactly
2. **Explicit cursor management**: No hidden state desync
3. **Full navigation control**: j/k/G/gg all work correctly
4. **Generic**: Works with any item type implementing `ListItem`

### Implementation Pattern

```go
// In your model
type MyModel struct {
    listViewport ListViewport[MyItem]
}

// SetSize
func (m *MyModel) SetSize(width, height int) {
    listHeight := height - 4 // header + footer + borders
    m.listViewport.SetSize(width-4, listHeight)
}

// Update delegates navigation
func (m MyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.listViewport, cmd = m.listViewport.Update(msg)
    return m, cmd
}

// View uses viewport directly
func (m MyModel) View() string {
    listView := m.listViewport.View()
    return header + addBorder(listView, width) + footer
}
```

### CJK Character Width

Additionally, we created `stringwidth.go` utilities to handle CJK character width correctly:

```go
// VisualWidth returns display columns (not bytes)
func VisualWidth(s string) int {
    return lipgloss.Width(s)  // Uses go-runewidth internally
}

// TruncateToWidth truncates to visual columns
func TruncateToWidth(s string, maxWidth int) string
```

Key insight: Korean characters are 3 bytes in UTF-8 but only 2 display columns. Using `len()` for display calculations causes border misalignment.

### Files Changed

- `internal/tui/stringwidth.go` (NEW)
- `internal/tui/listviewport.go` (NEW)
- `internal/tui/conversation.go` (refactored to use ListViewport)
- `internal/tui/project.go` (refactored to use ListViewport)

---

## Dashboard Pane Border Clipping

**Date**: 2026-01-19
**Project**: claude-code-log-viewer-cli

### Problem

Dashboard grid panes had their first row title clipped and merged with the top border line.

### Symptoms

- First row title not visible
- Title text overlaps with top border character
- Only affects bordered components using `lipgloss.Height()`

### Root Cause

Using `lipgloss.Style.Height(n).Render(content)` on a style with borders does not reliably control output height. This is the same underlying issue as "Option 3" in the first lesson above.

```go
// UNRELIABLE - causes clipping
return PaneBorderStyle.
    Width(p.width).
    Height(p.height).
    Render(inner)
```

### Solution

Use manual border drawing with `addBorder()` utility function instead of lipgloss borders with Height():

```go
// RELIABLE - manual border control
func (p PaneModel) View() string {
    innerWidth := p.width - 2
    innerHeight := p.height - 2  // account for top/bottom borders

    // Build content with exact line count
    var lines []string
    lines = append(lines, header)
    for i := 0; i < innerHeight-1; i++ {
        lines = append(lines, strings.Repeat(" ", innerWidth))
    }
    innerContent := strings.Join(lines, "\n")

    // Manual border drawing
    return addBorder(innerContent, p.width)
}
```

### Key Insight

When you need strict height control with borders:
1. Never use `lipgloss.Height()` on bordered styles
2. Build inner content with exact line count manually
3. Use `addBorder()` for reliable border wrapping

### Files Changed

- `internal/tui/dashboard.go` (fixed PaneModel.View())
