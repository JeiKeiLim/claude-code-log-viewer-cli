# Story 5.7: Dashboard Polish

Status: done

## Story

As a **developer using the dashboard**,
I want **consistent UI patterns with the viewer**,
So that **the dashboard feels intuitive and discoverable**.

## Acceptance Criteria

### AC 5.7.1: Bottom shortcut guide
- **Given** I am in the dashboard view
- **When** viewing the screen
- **Then** a shortcut guide appears at the bottom
- **And** shows dashboard-specific keys: navigation, enter, escape

### AC 5.7.2: Tool short info in panes
- **Given** a pane displays tool call entries
- **When** rendered
- **Then** tool entries show short info format like collapsed viewer
- **And** displays `[T] ToolName: target/summary` (e.g., `[T] Read: /path/to/file.go`)

## Tasks / Subtasks

- [x] Task 1: Add bottom shortcut guide to dashboard (AC: 5.7.1)
  - [x] 1.1: Create `dashboardHelpText` constant in styles.go or dashboard.go
  - [x] 1.2: Format: `"h/j/k/l:nav • Enter:open • Esc:back"` or similar
  - [x] 1.3: Add help text rendering to DashboardModel.View() at bottom
  - [x] 1.4: Style consistently with viewer's help text (dimmed)

- [x] Task 2: Update tool entry rendering in panes (AC: 5.7.2)
  - [x] 2.1: Locate `renderPaneEntry()` in dashboard.go
  - [x] 2.2: Add case for `types.EntryTypeToolUse` entries
  - [x] 2.3: Format as `[T] {ToolName}: {target/summary}` - one line
  - [x] 2.4: Truncate target if too long for pane width
  - [x] 2.5: Add case for `types.EntryTypeToolResult` if needed (or skip - less important)

- [x] Task 3: Add unit tests
  - [x] 3.1: Test dashboard View() includes help text
  - [x] 3.2: Test renderPaneEntry() for tool use entries

- [x] Task 4: Run build, lint, and test validation
  - [x] 4.1: Run `make build` - verify binary builds successfully
  - [x] 4.2: Run `make lint` - no errors
  - [x] 4.3: Run `make test` - all tests pass

- [ ] Task 5: Manual testing (Requires user verification)
  - [ ] 5.1: Open dashboard - verify shortcut guide visible at bottom
  - [ ] 5.2: View pane with tool calls - verify short info format displays

## Dev Notes

### Current Implementation

From Story 5.3, `renderPaneEntry()` in dashboard.go handles:
- `types.EntryTypeUser` - Shows `[U] {first line of message}`
- `types.EntryTypeAssistant` - Shows `[A] {markdown rendered, max 3 lines}`

Missing: Tool use entries (`types.EntryTypeToolUse`)

### Viewer Pattern Reference

From viewer.go, collapsed tool display shows:
```
[T] Read: /path/to/file.go
[T] Bash: make build
[T] Write: /path/to/output.md
```

Format: `ToolIcon + " " + entry.ToolName + ": " + truncatedTarget`

### Help Text Pattern Reference

From viewer.go status bar:
```go
helpText := "j/k:scroll • t:thinking • i:inputs • r:raw • p:path • h:back"
```

Dashboard equivalent:
```go
helpText := "h/j/k/l:nav • Enter:open • Esc:back"
// Or with arrows:
helpText := "arrows:nav • Enter:open • Esc:back"
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/dashboard.go` | Add help text to View(), update renderPaneEntry() |
| `internal/tui/dashboard_test.go` | Add tests for new functionality |
| `internal/tui/styles.go` | (Optional) Add DashboardHelpStyle if needed |

### Project Context Rules

- **NO EMOJI IN UI** - Use `[T]` text icon for tools
- **TEA pattern** - View() is pure rendering
- **Use Makefile** - `make build`, `make test`

### References

- [Source: 5-3-dashboard-pane-content-display.md] - renderPaneEntry() implementation
- [Source: internal/tui/viewer.go] - Help text and collapsed tool patterns
- [Source: internal/tui/styles.go] - ToolIcon constant

## Origin

Created from Epic 5 Retrospective feedback (2026-01-19) by Jongkuk Lim:
1. Bottom shortcut guide like log viewer
2. Short tool info in panes like collapsed tool display

## Dev Agent Record

### Implementation Summary (2026-01-19)

**AC 5.7.1: Bottom shortcut guide**
- Added `dashboardHelpText` constant in `dashboard.go:22`
- Modified `View()` function to reserve 1 line for help text and render it at bottom
- Help text: `"h/j/k/l:nav • Enter:open • Esc:back"`
- Styled with `Styles.HelpText` (dimmed) for consistency with viewer

**AC 5.7.2: Tool short info in panes**
- Added `PaneToolIcon = "[T]"` constant for tool entries in panes
- Added `formatPaneToolSummary()` function supporting: Read, Write, Edit, Bash, Glob, Grep, Task, WebFetch, WebSearch
- Modified `renderPaneEntry()` to check for tool_use content first and format as `[T] ToolName: target`
- Implements truncation: long paths show just filename via `filepath.Base()`

### Files Modified
| File | Changes |
|------|---------|
| `internal/tui/dashboard.go` | Added dashboardHelpText, PaneToolIcon, formatPaneToolSummary(), updated View() and renderPaneEntry() |
| `internal/tui/dashboard_test.go` | Added 8 new tests: TestDashboardViewIncludesHelpText, TestDashboardHelpTextConstant, TestFormatPaneToolSummary, TestRenderPaneEntryToolTruncation, TestRenderPaneEntryToolNoSummary, TestRenderPaneEntryToolBeforeText; updated TestRenderPaneEntryTool |

### Tests Added
- `TestDashboardViewIncludesHelpText` - verifies help text appears in View()
- `TestDashboardHelpTextConstant` - verifies constant contains expected keys
- `TestFormatPaneToolSummary` - 13 test cases covering all supported tools
- `TestRenderPaneEntryTool` - updated to test new [T] format with file path
- `TestRenderPaneEntryToolTruncation` - verifies long paths are truncated
- `TestRenderPaneEntryToolNoSummary` - verifies unknown tools show just name
- `TestRenderPaneEntryToolBeforeText` - verifies tool_use takes priority over text

### Validation
- `make build`: PASS
- `make lint`: PASS (0 issues)
- `make test`: PASS (all tests, 50.2% coverage on tui package)

### Code Review Fixes (2026-01-19)

**Issues Fixed:**

1. **M1: DRY violation in formatPaneToolSummary()** - Combined Read/Write/Edit cases that shared identical logic
   - `dashboard.go:193` - Single combined case now

2. **M3: Missing test for unknown content types** - Added test to verify graceful handling
   - `dashboard_test.go` - Added `TestRenderPaneEntryUnknownContentType`

3. **L1: Help text now mentions arrow keys** - Updated to show both navigation options
   - `dashboard.go:22` - Changed from `"h/j/k/l:nav"` to `"arrows/hjkl:nav"`

**Post-Review Validation:**
- `make build`: PASS
- `make lint`: PASS (0 issues)
- `make test`: PASS (50.3% coverage on tui package)
