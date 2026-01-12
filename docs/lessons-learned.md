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
