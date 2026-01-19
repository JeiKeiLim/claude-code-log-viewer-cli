# Story 5.3: Dashboard Pane Content Display

Status: done

## Story

As a **developer monitoring projects**,
I want **each pane to show the latest conversation content**,
So that **I can see activity across projects**.

## Acceptance Criteria

### AC 5.3.1: Load latest conversation on pane init
- **Given** a pane is initialized for a project
- **When** it loads
- **Then** it finds and displays the latest conversation file (most recently modified .jsonl)
- **And** shows project name in pane header

### AC 5.3.2: Display rendered conversation content
- **Given** a pane has content
- **When** displayed
- **Then** it shows rendered messages (not raw JSONL)
- **And** follows the same rendering style as ViewerModel (user/assistant blocks)

### AC 5.3.3: Enable file watching per pane
- **Given** a pane is watching a conversation
- **When** the conversation file changes (new entries appended)
- **Then** the pane content updates within 200ms
- **And** pane auto-scrolls to show new content

### AC 5.3.4: Truncate content to pane height
- **Given** a pane has limited height in the grid
- **When** rendering content
- **Then** content is truncated to fit the pane's available height
- **And** user can see the latest/bottom content (tail behavior)

## Tasks / Subtasks

- [x] Task 1: Extend PaneModel with conversation state (AC: 5.3.1, 5.3.2)
  - [x] 1.1: Add `conversation types.Conversation` field to PaneModel
  - [x] 1.2: Add `entries []types.LogEntry` field to PaneModel
  - [x] 1.3: Add `content string` field for pre-rendered view content
  - [x] 1.4: Add `parseErrors int` field for tracking parse failures
  - [x] 1.5: Add `watcher *watcher.Watcher` field for file monitoring
  - [x] 1.6: Add `markdownRenderer *MarkdownRenderer` field for text rendering

- [x] Task 2: Implement latest conversation discovery (AC: 5.3.1)
  - [x] 2.1: Create `findLatestConversation(projectPath string) (types.Conversation, error)` function
  - [x] 2.2: Use `scanner.ScanConversationsLazy()` to get sorted conversation list (most recent first)
  - [x] 2.3: Return first conversation from sorted list (already sorted by LastModified descending)
  - [x] 2.4: Handle edge case: project with no conversations (return empty content message)

- [x] Task 3: Implement pane content loading (AC: 5.3.1, 5.3.2)
  - [x] 3.1: Create loadPaneContentCmd function
  - [x] 3.2: Parse JSONL file using `parser.ParseJSONLFile()`
  - [x] 3.3: Store entries in `p.entries` and parseErrors in `p.parseErrors`
  - [x] 3.4: Define `paneContentLoadedMsg` struct with `paneIndex int, entries []types.LogEntry, err error`
  - [x] 3.5: Create `loadPaneContentCmd(paneIndex int, projectPath string) tea.Cmd` function

- [x] Task 4: Implement pane content rendering (AC: 5.3.2, 5.3.4)
  - [x] 4.1: Create `(p *PaneModel) renderPaneContent() string` method
  - [x] 4.2: Iterate through entries and render each using simplified viewer rendering
  - [x] 4.3: Truncate rendered content to fit pane height (innerHeight - 1 for header)
  - [x] 4.4: Use tail-style truncation (show last N lines, not first N)
  - [x] 4.5: Initialize markdown renderer in PaneModel with pane width
  - [x] 4.6: Handle showThinking=false, showToolInputs=false (collapsed by default like viewer)

- [x] Task 5: Update PaneModel.View() to display content (AC: 5.3.2)
  - [x] 5.1: Replace empty content area with `p.content` pre-rendered string
  - [x] 5.2: Ensure content fits within `innerHeight - 1` (header takes 1 line)
  - [x] 5.3: Handle loading state: show "Loading..." placeholder
  - [x] 5.4: Handle error state: show error message in muted style
  - [x] 5.5: Handle empty state: show "No conversations" message

- [x] Task 6: Implement per-pane file watching (AC: 5.3.3)
  - [x] 6.1: Store watcher reference in PaneModel after successful content load
  - [x] 6.2: Create `waitForPaneWatcher(paneIndex int) tea.Cmd` in DashboardModel
  - [x] 6.3: Define `paneWatcherEventMsg` wrapper struct with `paneIndex int, event tea.Msg`
  - [x] 6.4: Create watcher wait command that wraps events with pane index
  - [x] 6.5: Handle `paneWatcherEventMsg` in DashboardModel.Update() - unwrap and route
  - [x] 6.6: Handle `watcher.NewEntriesMsg` - append entries, re-render, chain next wait
  - [x] 6.7: Handle `watcher.FileResetMsg` - reload full conversation via loadPaneContentCmd
  - [x] 6.8: Handle `watcher.WatcherErrorMsg` - continue watching (graceful degradation)

- [x] Task 7: Update DashboardModel for content management (AC: all)
  - [x] 7.1: Change `NewDashboardModel()` signature to return `(DashboardModel, tea.Cmd)` for TEA pattern
  - [x] 7.2: Return batch of `loadPaneContentCmd` for all panes from constructor
  - [x] 7.3: Handle `paneContentLoadedMsg` in Update() - update correct pane's content and start watcher
  - [x] 7.4: Handle `paneWatcherEventMsg` in Update() - unwrap and route to pane handler
  - [x] 7.5: Implement pane-specific watcher event handlers (NewEntries, FileReset, Error)
  - [x] 7.6: Chain watcher wait commands after handling watcher messages
  - [x] 7.7: Update `SetSize()` to propagate resize to panes and trigger re-render
  - [x] 7.8: Clean up watchers on dashboard exit (close all pane watchers in GoBackToProjectsFromDashboardMsg handler)

- [x] Task 8: Integrate DashboardModel changes with AppModel (AC: all)
  - [x] 8.1: Update `DashboardSelectedMsg` handler to capture command from `NewDashboardModel()` return
  - [x] 8.2: Forward `paneWatcherEventMsg` and `paneContentLoadedMsg` to dashboard in Update()
  - [x] 8.3: Ensure `GoBackToProjectsFromDashboardMsg` closes all pane watchers before transition
  - [x] 8.4: Forward `tea.WindowSizeMsg` to dashboard when in viewDashboard state

- [x] Task 9: Add pane rendering helper functions (AC: 5.3.2)
  - [x] 9.1: Create `renderPaneEntry(entry types.LogEntry, width int, mdRenderer *MarkdownRenderer) string`
  - [x] 9.2: Simplify entry rendering for pane context (no line numbers, minimal chrome)
  - [x] 9.3: Render user messages with UserIcon and wrapped text
  - [x] 9.4: Render assistant messages with AssistantIcon and markdown
  - [x] 9.5: Render thinking/tool blocks as collapsed indicators only (space-efficient)

- [x] Task 10: Implement tail truncation utility (AC: 5.3.4)
  - [x] 10.1: Create `truncateFromTop(content string, maxLines int) string` function
  - [x] 10.2: Count lines, remove excess lines from the beginning
  - [x] 10.3: Return last `maxLines` lines of content
  - [x] 10.4: Added to `dashboard.go`

- [x] Task 11: Add unit tests
  - [x] 11.1: Test `truncateFromTop()` with content longer than max
  - [x] 11.2: Test `truncateFromTop()` with content shorter than max (no-op)
  - [x] 11.3: Test `renderPaneEntry()` for user message format
  - [x] 11.4: Test `renderPaneEntry()` for assistant message format
  - [x] 11.5: Test `paneContentLoadedMsg` handler updates correct pane
  - [x] 11.6: Test NewDashboardModel returns batch command for all panes
  - [x] 11.7: Test pane loading, error, and empty states
  - [x] 11.8: Test extractPaneTextContent for text extraction
  - [x] 11.9: Test closeAllWatchers safety with nil watchers

- [x] Task 12: Run build, lint, and test validation
  - [x] 12.1: Run `make build` - verify binary builds successfully
  - [x] 12.2: Run `make lint` - no errors
  - [x] 12.3: Run `make test` - all tests pass

- [ ] Task 13: Manual testing (Requires user verification)
  - [ ] 13.1: Select 1 project with conversations - verify pane shows latest conversation content
  - [ ] 13.2: Select 2 projects - verify both panes load and display content independently
  - [ ] 13.3: Select project with no conversations - verify "No conversations" message
  - [ ] 13.4: With dashboard open, add new entry to active conversation - verify auto-update within 200ms
  - [ ] 13.5: Verify content shows newest messages at bottom (tail behavior)
  - [ ] 13.6: Verify pane header still shows project name
  - [ ] 13.7: Press Escape to exit - verify clean exit (no watcher errors)

## Dev Notes

### Current Implementation State

Story 5.2 created the foundation:
- `DashboardModel` exists with `panes []PaneModel` and grid layout
- `PaneModel` has minimal fields: `project`, `width`, `height`
- Grid layout calculation and rendering works
- Empty content area placeholder in PaneModel.View()
- `addBorder()` utility for reliable height control

### Architecture Reference

From `architecture-phase3.md` (adapted - architecture shows `content []string` but `string` is correct for pre-rendered view):

```go
// internal/tui/dashboard.go - EXTENDED
type PaneModel struct {
    project      types.Project
    conversation types.Conversation  // NEW in Story 5.3
    entries      []types.LogEntry    // NEW in Story 5.3
    content      string              // NEW: pre-rendered view content (string, not []string)
    parseErrors  int                 // NEW: parse error count
    watcher      *watcher.Watcher    // NEW in Story 5.3
    width        int
    height       int
    mdRenderer   *MarkdownRenderer   // NEW: reuse existing type from styles.go
}

// Constructor signature change for TEA pattern
func NewDashboardModel(projects []types.Project) (DashboardModel, tea.Cmd)
```

### Message Flow for Content Loading

```
DashboardModel.Init()
    ↓
[batch: loadPaneContentCmd(0), loadPaneContentCmd(1), ...]
    ↓
paneContentLoadedMsg{paneIndex: 0, entries: [...]}
    ↓
DashboardModel.Update() → panes[0].content = renderContent()
                        → panes[0].watcher = New(path)
                        → return panes[0].startWatching()
    ↓
[watcher.NewEntriesMsg wrapped in paneWatcherEventMsg]
    ↓
DashboardModel.Update() → panes[i].entries = append(...)
                        → panes[i].content = renderContent()
                        → return panes[i].watcher.WaitForEvent()
```

### Pane Content Rendering Strategy

**Space efficiency is critical** - panes may be 1/9th of screen height. Strategy:

1. **Tail-style display**: Show most recent content, not oldest
2. **Collapsed blocks**: Thinking and tool blocks always collapsed (space)
3. **Minimal chrome**: No line numbers, minimal borders
4. **Pre-render on update**: Cache rendered content, don't render in View()

```go
func (p *PaneModel) renderContent() string {
    var lines []string
    for _, entry := range p.entries {
        rendered := renderPaneEntry(entry, p.width-2, p.mdRenderer)
        entryLines := strings.Split(rendered, "\n")
        lines = append(lines, entryLines...)
    }
    // Tail truncation: show last N lines
    contentHeight := p.height - 3  // borders + header
    return truncateFromTop(strings.Join(lines, "\n"), contentHeight)
}
```

### Watcher Routing Pattern

Multiple panes = multiple watchers. Route watcher events to correct pane using index wrapper:

```go
// Wrapper message to identify source pane
type paneWatcherEventMsg struct {
    paneIndex int
    event     tea.Msg  // watcher.NewEntriesMsg, watcher.FileResetMsg, etc.
}

// Create watcher wait command that wraps events with pane index
// This lives in DashboardModel, not PaneModel
func (m *DashboardModel) waitForPaneWatcher(paneIndex int) tea.Cmd {
    if paneIndex >= len(m.panes) || m.panes[paneIndex].watcher == nil {
        return nil
    }
    w := m.panes[paneIndex].watcher
    return func() tea.Msg {
        // Reuse existing watcher.WaitForEvent() but wrap result
        cmd := w.WaitForEvent()
        event := cmd()  // Execute the blocking wait
        if event == nil {
            return nil  // Watcher closed
        }
        return paneWatcherEventMsg{paneIndex: paneIndex, event: event}
    }
}
```

**Note:** Watcher message types (`NewEntriesMsg`, `FileResetMsg`, `WatcherErrorMsg`) are already defined in `internal/watcher/watcher.go` - reuse them.

### Simplified Entry Rendering for Panes

Panes need compact rendering different from full ViewerModel:

```go
func renderPaneEntry(entry types.LogEntry, width int, mdRenderer *MarkdownRenderer) string {
    switch entry.Type {
    case types.EntryTypeUser:
        // [U] <first line of message, truncated>
        header := UserIcon + " "
        content := TruncateToWidth(entry.Message.TextContent, width-4)
        content = strings.Split(content, "\n")[0]  // First line only
        return header + content

    case types.EntryTypeAssistant:
        // [A] <rendered markdown, wrapped>
        header := AssistantIcon + " "
        text := extractTextContent(entry)
        rendered := mdRenderer.Render(text)
        // Take first few lines only
        lines := strings.Split(rendered, "\n")
        if len(lines) > 3 {
            lines = lines[:3]
            lines = append(lines, "...")
        }
        return header + strings.Join(lines, "\n    ")  // Indent continuation
    }
    return ""
}
```

### Latest Conversation Discovery

```go
func findLatestConversation(projectPath string) (types.Conversation, error) {
    convs, err := scanner.ScanConversationsLazy(projectPath)
    if err != nil {
        return types.Conversation{}, err
    }
    if len(convs) == 0 {
        return types.Conversation{}, nil  // Empty is valid
    }
    // ScanConversationsLazy already sorts by LastModified descending
    return convs[0], nil
}
```

### Performance Consideration (<200ms Update)

Per NFR-001, pane updates must complete within 200ms:

1. **Incremental updates**: On `NewEntriesMsg`, append entries, don't re-parse full file
2. **Pre-render content**: Render in Update(), not View()
3. **Limit displayed lines**: Truncate to visible height, don't render invisible content
4. **Reuse renderer**: Keep markdown renderer instance, don't recreate

### Files to Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/dashboard.go` | MODIFY | Extend PaneModel, add content loading, watcher integration |
| `internal/tui/dashboard_test.go` | MODIFY | Add tests for content loading and rendering |
| `internal/tui/app.go` | MODIFY | Update dashboard init handling |
| `internal/tui/utils.go` | MODIFY | Add `truncateFromTop()` helper |

### Project Context Rules (from project-context.md)

| Rule | Application |
|------|-------------|
| **NO EMOJI IN UI** | Use text icons `[U]`, `[A]` in pane content |
| **TEA pattern** | Content loading via Cmd, state in Update() |
| **Use Makefile** | `make build`, `make test` |
| **Parser resilience** | Skip malformed lines, track `parseErrors` |
| **Watcher cleanup** | Close watchers on dashboard exit |

### Previous Story Learnings

From Story 5.2:
1. **Manual border control**: Use `addBorder()` not `lipgloss.Height()` for reliable pane height
2. **Pre-computed dimensions**: Calculate pane dimensions in SetSize(), not View()
3. **Empty cell handling**: Grid handles incomplete rows with empty space

From Epic 2 (File Watching):
1. **Chain watcher commands**: After handling watcher event, return `watcher.WaitForEvent()`
2. **Graceful degradation**: On watcher error, continue waiting
3. **Auto-scroll behavior**: Scroll to bottom on new content

### Existing Types to Reuse

From `internal/watcher/watcher.go`:
- `watcher.New(filePath string) (*Watcher, error)` - Create new watcher
- `watcher.WaitForEvent() tea.Cmd` - Returns blocking command
- `watcher.NewEntriesMsg{Entries []types.LogEntry}` - New entries appended
- `watcher.FileResetMsg{}` - File truncated/reset
- `watcher.WatcherErrorMsg{Err error}` - Watcher error
- `watcher.Close()` - Cleanup

From `internal/tui/styles.go`:
- `MarkdownRenderer` - Reuse for pane content rendering
- `NewMarkdownRenderer(width int) (*MarkdownRenderer, error)` - Constructor

### Git Commit Pattern

Recent commits show pattern:
- `feat: implement <description> (Story N.M)`
- `fix: <description> (Story N.M)`

### References

- [Source: epics-phase3.md#Story-5.3] - Acceptance criteria
- [Source: prd-phase3.md#FR-503] - Multi-project watch requirements
- [Source: architecture-phase3.md#Dashboard-Component-Structure] - PaneModel design
- [Source: 5-2-grid-layout-component.md] - Previous story implementation
- [Source: internal/tui/dashboard.go] - Current dashboard implementation
- [Source: internal/tui/viewer.go] - Entry rendering patterns to adapt
- [Source: internal/watcher/watcher.go] - File watcher integration

## Implementation Checklist

Before marking story complete, verify:

**PaneModel Extension:**
- [x] `conversation` field added to PaneModel (type: `types.Conversation`)
- [x] `entries` field added to PaneModel (type: `[]types.LogEntry`)
- [x] `content` field added for pre-rendered view (type: `string`)
- [x] `watcher` field added for file monitoring (type: `*watcher.Watcher`)
- [x] `mdRenderer` field added for markdown (type: `*MarkdownRenderer`)

**DashboardModel Changes:**
- [x] `NewDashboardModel()` returns `(DashboardModel, tea.Cmd)`
- [x] Init batch command loads all panes in parallel

**Content Loading:**
- [x] `findLatestConversation()` returns most recent .jsonl file
- [x] `loadPaneContentCmd()` parses file and returns entries
- [x] `paneContentLoadedMsg` handled in DashboardModel.Update()
- [x] Pane content updated and re-rendered after load

**Content Rendering:**
- [x] `renderPaneEntry()` handles user messages
- [x] `renderPaneEntry()` handles assistant messages
- [x] `renderPaneContent()` aggregates all entry renders
- [x] `truncateFromTop()` shows tail content
- [x] Thinking/tool blocks collapsed for space efficiency

**File Watching:**
- [x] Watcher created per pane after content load
- [x] `paneWatcherEventMsg` routes events to correct pane
- [x] `watcher.NewEntriesMsg` appends entries and re-renders
- [x] `watcher.FileResetMsg` triggers full reload
- [x] Watcher commands chained correctly via `waitForPaneWatcher()`
- [x] All watchers closed on dashboard exit (closeAllWatchers called in Update before returning GoBackToProjectsFromDashboardMsg)

**View Integration:**
- [x] PaneModel.View() displays `p.content`
- [x] Loading state shows "Loading..." placeholder
- [x] Error state shows error message
- [x] Empty state shows "No conversations"
- [x] Content fits within pane height

**Testing:**
- [x] Unit tests for `truncateFromTop()`
- [x] Unit tests for `renderPaneEntry()`
- [x] Unit tests for message handlers
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes

**Manual Verification (Requires User):**
- [ ] Single pane shows latest conversation
- [ ] Multiple panes load independently
- [ ] Empty project shows appropriate message
- [ ] Live updates work within 200ms
- [ ] Tail content visible (newest at bottom)
- [ ] Clean exit with no watcher errors

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. Extended PaneModel with all required fields: conversation, entries, content, parseErrors, watcher, mdRenderer, loading, errMsg
2. Implemented findLatestConversation() using scanner.ScanConversationsLazy() which already sorts by LastModified descending
3. Implemented loadPaneContentCmd() to parse JSONL files asynchronously with pane index routing
4. Created renderPaneEntry() for simplified pane rendering - user messages show first line, assistant messages show markdown with max 3 lines
5. Implemented truncateFromTop() for tail-style content truncation (shows newest content)
6. Updated PaneModel.View() to handle loading, error, empty, and content states with proper padding and truncation
7. Implemented per-pane file watching with paneWatcherEventMsg wrapper for correct routing
8. Updated DashboardModel.Update() to handle paneContentLoadedMsg (content load) and paneWatcherEventMsg (live updates)
9. Changed NewDashboardModel() signature to return (DashboardModel, tea.Cmd) for TEA pattern compliance
10. Updated app.go DashboardSelectedMsg handler to capture and forward the batch command
11. Added closeAllWatchers() to clean up resources on dashboard exit
12. Added comprehensive unit tests for all new functionality
13. All builds pass, lint passes, tests pass

### Code Review Fixes Applied (2026-01-19)

14. **H3 FIX**: Fixed renderPaneEntry() user message truncation to use VisualWidth() and TruncateToWidth() instead of byte slicing - prevents multi-byte UTF-8 character corruption (CJK, emoji)
15. **M1 FIX**: Fixed WindowSizeMsg handler to recreate mdRenderer with updated width - ensures proper word wrap after resize
16. Added unit tests: TestRenderPaneEntryUserWithCJK, TestRenderPaneEntryUserWithEmoji, TestWindowSizeMsgUpdatesMarkdownRenderer

### File List

- `internal/tui/dashboard.go` - Extended PaneModel, added content loading, rendering, and watcher integration
- `internal/tui/dashboard_test.go` - Added Story 5.3 unit tests
- `internal/tui/app.go` - Updated DashboardSelectedMsg handler for new NewDashboardModel signature
