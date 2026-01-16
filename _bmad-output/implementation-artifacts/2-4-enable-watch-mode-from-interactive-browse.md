# Story 2.4: Enable Watch Mode from Interactive Browse

Status: done

## Story

As a **developer using cclv interactively**,
I want **to enable watch mode on a conversation I browse to**,
So that **I can monitor live sessions without knowing the file path upfront**.

## Acceptance Criteria

### AC 2.4.1: Watch mode shortcut in conversation list
- **Given** the user is in the conversation list view
- **When** they press 'w' on a selected conversation
- **Then** the conversation opens in the viewer with watch mode enabled
- **And** the watcher starts monitoring the file

### AC 2.4.2: Watch mode shortcut in viewer
- **Given** the user is viewing a conversation (entered via normal Enter key)
- **When** they press 'w' to toggle watch mode
- **Then** watch mode is enabled and watcher starts
- **And** "LIVE" indicator appears in status bar

### AC 2.4.3: Toggle watch mode off
- **Given** watch mode is active in the viewer
- **When** user presses 'w' again
- **Then** watch mode is disabled and watcher stops
- **And** "LIVE" indicator disappears

### AC 2.4.4: Help documentation
- **Given** the user views keyboard shortcuts (status bar or --help)
- **When** looking for watch functionality
- **Then** 'w' shortcut is documented for watch mode toggle

## Tasks / Subtasks

- [x] Task 1: Add 'w' key handler in ConversationModel (AC: 2.4.1)
  - [x] 1.1: In `conversation.go` `Update()`, add case `"w"` in tea.KeyMsg switch (after "G" case, around line 208)
  - [x] 1.2: Get selected conversation using `m.listViewport.SelectedItem()`
  - [x] 1.3: Return `ConversationSelectedWithWatchMsg{Conversation: item.conversation}`
  - [x] 1.4: Create new message type `ConversationSelectedWithWatchMsg` in conversation.go (after line 163)

- [x] Task 2: Handle ConversationSelectedWithWatchMsg in AppModel (AC: 2.4.1)
  - [x] 2.1: In `app.go`, add case for `ConversationSelectedWithWatchMsg` in Update() (after ConversationSelectedMsg case, around line 187)
  - [x] 2.2: Set loading state, store selected conversation
  - [x] 2.3: Return tea.Batch with loadConversationWithWatch() command
  - [x] 2.4: Create `loadConversationWithWatch()` method that returns conversationLoadedWithWatchMsg
  - [x] 2.5: Handle `conversationLoadedWithWatchMsg` - create viewer with `RenderOptions{WatchMode: true, FilePath: msg.filePath}`

- [x] Task 3: Add 'w' key handler in ViewerModel for toggle (AC: 2.4.2, 2.4.3)
  - [x] 3.1: In `viewer.go` `Update()`, add case `"w"` in tea.KeyMsg switch (after "i" case, line 311)
  - [x] 3.2: If `m.watchMode` is true → disable watch mode (close watcher, set watchMode=false, clear newEntriesCount)
  - [x] 3.3: If `m.watchMode` is false AND `m.renderOpts.FilePath != ""` → enable watch mode (create watcher, set watchMode=true, return watcher.WaitForEvent())
  - [x] 3.4: If FilePath is empty → no-op (graceful degradation)

- [x] Task 4: Fix FilePath propagation for normal conversation open (AC: 2.4.2) - CRITICAL
  - [x] 4.1: Add `filePath` field to `conversationLoadedMsg` struct in app.go (line 264-268)
  - [x] 4.2: Update `loadConversation()` to include filePath in returned message
  - [x] 4.3: In `conversationLoadedMsg` handler (line 157-187), create RenderOptions with FilePath set
  - [x] 4.4: This enables watch toggle for conversations opened via Enter key

- [x] Task 5: Add watcher cleanup on back navigation (AC: 2.4.3) - CRITICAL
  - [x] 5.1: In viewer.go `Update()`, in the "h", "esc" case (line 285-289), close watcher before returning GoBackMsg
  - [x] 5.2: Pattern: `if m.watcher != nil { _ = m.watcher.Close() }`
  - [x] 5.3: This prevents resource leak when navigating back with watch mode active

- [x] Task 6: Update help text in viewer status bar (AC: 2.4.4)
  - [x] 6.1: In `buildShortcutsSegment()` (line 511-522), add "w:watch" before "q:quit"
  - [x] 6.2: Result: `parts = append(parts, "t:thinking", "i:inputs", "w:watch", "q:quit")`

- [x] Task 7: Update help text in conversation list (AC: 2.4.4)
  - [x] 7.1: In ConversationModel `View()` (line 304), update help string
  - [x] 7.2: Change to: `"j/k:nav • enter/l:open • w:watch • h/esc:back • g/G:top/bottom • q:quit"`

- [x] Task 8: Update --help output (AC: 2.4.4)
  - [x] 8.1: In `cmd/cclv/main.go` help section (line 57-74), add 'w' shortcut
  - [x] 8.2: Add `                w               Toggle watch mode` under Toggles section

- [x] Task 9: Add unit tests for watch mode toggle
  - [x] 9.1: Test 'w' key in viewer enables watch mode when off with valid FilePath
  - [x] 9.2: Test 'w' key in viewer disables watch mode when on
  - [x] 9.3: Test watcher is properly closed when toggling off
  - [x] 9.4: Test watcher is created when toggling on with valid file path
  - [x] 9.5: Test no-op when toggling on without file path (empty FilePath)
  - [x] 9.6: Test watcher is closed on GoBackMsg (h/esc navigation)
  - [x] 9.7: Test ConversationSelectedWithWatchMsg creates viewer with watcher active

- [x] Task 10: Run build and lint validation
  - [x] 10.1: Run `make build` - succeeds
  - [x] 10.2: Run `make lint` - no errors (0 issues)
  - [x] 10.3: Run `make test` - all pass

- [x] Task 11: Manual testing (performed during code review)
  - [x] 11.1: Start cclv interactively: `./bin/cclv`
  - [x] 11.2: Navigate to project → conversation list
  - [x] 11.3: Press 'w' on a conversation - verify opens with LIVE indicator (AC: 2.4.1)
  - [x] 11.4: Press Escape to go back - verify no resource leak
  - [x] 11.5: Press Enter to open same conversation normally
  - [x] 11.6: Press 'w' - verify LIVE indicator appears (AC: 2.4.2)
  - [x] 11.7: Press 'w' again - verify LIVE indicator disappears (AC: 2.4.3)
  - [x] 11.8: Verify help text shows 'w' shortcut (AC: 2.4.4)

## Dev Notes

### Architecture Pattern - Message-Based State Changes

The watch mode toggle follows Bubbletea's Elm Architecture:
- User presses 'w' → Update() handles KeyMsg
- State changes: `watchMode` flag, `watcher` instance
- If enabling, return `tea.Cmd` from `watcher.WaitForEvent()` to start event loop
- If disabling, close watcher, return `nil` cmd

### Critical Implementation: FilePath Propagation

**IMPORTANT**: For watch toggle to work in viewer, `m.renderOpts.FilePath` MUST be set when creating the viewer. Currently, conversations opened via Enter key use `DefaultRenderOptions()` which has empty FilePath.

**Required Fix in app.go:**
```go
// 1. Update conversationLoadedMsg to include filePath
type conversationLoadedMsg struct {
    entries     []types.LogEntry
    parseErrors int
    err         error
    filePath    string  // ADD THIS
}

// 2. Update loadConversation() to return filePath
func (m AppModel) loadConversation(filePath string) tea.Cmd {
    return func() tea.Msg {
        result, err := parser.ParseJSONLFile(filePath)
        if err != nil {
            return conversationLoadedMsg{err: err}
        }
        return conversationLoadedMsg{
            entries:     result.Entries,
            parseErrors: result.ParseErrors,
            filePath:    filePath,  // ADD THIS
        }
    }
}

// 3. Update conversationLoadedMsg handler to use FilePath
case conversationLoadedMsg:
    m.loading = false
    if msg.err != nil {
        return m, nil
    }
    title := fmt.Sprintf("%s - %s", m.selectedProject.DisplayName, formatTimestamp(m.selectedConversation.LastModified))
    opts := RenderOptions{FilePath: msg.filePath}  // SET FilePath
    m.viewerModel = NewViewerModelWithBackNavigation(msg.entries, msg.parseErrors, title, opts)
    // ...
```

### Critical: Watcher Cleanup on Back Navigation

**IMPORTANT**: Close watcher when user navigates back via h/esc to prevent resource leaks.

```go
// In viewer.go Update(), "h", "esc" case (around line 285-289)
case "h", "esc":
    if m.canGoBack {
        // Close watcher before navigating back to prevent resource leak
        if m.watcher != nil {
            _ = m.watcher.Close()
        }
        return m, func() tea.Msg { return GoBackMsg{} }
    }
```

### Key Implementation Code

**1. ConversationSelectedWithWatchMsg (conversation.go, after line 163):**
```go
type ConversationSelectedWithWatchMsg struct {
    Conversation types.Conversation
}
```

**2. Handle 'w' in ConversationModel (conversation.go, after "G" case):**
```go
case "w":
    if item, ok := m.listViewport.SelectedItem(); ok {
        return m, func() tea.Msg {
            return ConversationSelectedWithWatchMsg{Conversation: item.conversation}
        }
    }
```

**3. Handle ConversationSelectedWithWatchMsg in AppModel (app.go, after ConversationSelectedMsg):**
```go
case ConversationSelectedWithWatchMsg:
    m.loading = true
    m.selectedConversation = msg.Conversation
    return m, tea.Batch(m.spinner.Tick, m.loadConversationWithWatch(msg.Conversation.FilePath))

// Add new method:
func (m AppModel) loadConversationWithWatch(filePath string) tea.Cmd {
    return func() tea.Msg {
        result, err := parser.ParseJSONLFile(filePath)
        if err != nil {
            return conversationLoadedWithWatchMsg{err: err}
        }
        return conversationLoadedWithWatchMsg{
            entries:     result.Entries,
            parseErrors: result.ParseErrors,
            filePath:    filePath,
        }
    }
}

// Add new message type:
type conversationLoadedWithWatchMsg struct {
    entries     []types.LogEntry
    parseErrors int
    err         error
    filePath    string
}

// Handle the new message:
case conversationLoadedWithWatchMsg:
    m.loading = false
    if msg.err != nil {
        return m, nil
    }
    title := fmt.Sprintf("%s - %s", m.selectedProject.DisplayName, formatTimestamp(m.selectedConversation.LastModified))
    opts := RenderOptions{WatchMode: true, FilePath: msg.filePath}
    m.viewerModel = NewViewerModelWithBackNavigation(msg.entries, msg.parseErrors, title, opts)
    m.viewerModel.SetSize(m.width, m.height)
    m.state = viewViewer
    return m, m.viewerModel.Init()  // Return Init() to start watcher
```

**4. Toggle 'w' in ViewerModel (viewer.go, after "i" case around line 311):**
```go
case "w":
    if m.watchMode {
        // Disable watch mode
        if m.watcher != nil {
            _ = m.watcher.Close()
            m.watcher = nil
        }
        m.watchMode = false
        m.newEntriesCount = 0
        return m, nil
    }
    // Enable watch mode
    if m.renderOpts.FilePath != "" {
        w, err := watcher.New(m.renderOpts.FilePath)
        if err == nil {
            m.watcher = w
            m.watchMode = true
            return m, m.watcher.WaitForEvent()
        }
    }
    return m, nil
```

**5. Update buildShortcutsSegment (viewer.go line 520):**
```go
parts = append(parts, "t:thinking", "i:inputs", "w:watch", "q:quit")
```

**6. Update conversation list help (conversation.go line 304):**
```go
help := "j/k:nav • enter/l:open • w:watch • h/esc:back • g/G:top/bottom • q:quit"
```

**7. Update --help (cmd/cclv/main.go, add under Toggles section):**
```
  Toggles:      t               Toggle thinking blocks
                i               Toggle tool inputs
                w               Toggle watch mode
```

### Critical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI in UI** | Text icons only: "w:watch" not watch icon |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **tea.Cmd for side effects** | Watcher event loop via tea.Cmd, never raw goroutines |
| **Import order** | stdlib -> external -> internal |
| **Idempotent watcher.Close()** | Safe to call multiple times |
| **Close watcher on back nav** | Prevent resource leaks when user presses h/esc |

### Edge Cases

| Case | Handling |
|------|----------|
| Press 'w' on conversation with missing file | FilePath from types.Conversation will be valid |
| Toggle watch on when already on | Toggles off (standard toggle behavior) |
| Toggle watch off twice | Idempotent - watcher.Close() is safe |
| FilePath empty in viewer | No-op, graceful degradation |
| File deleted during watch | fsnotify will send error, handled by WatcherErrorMsg |
| Navigate back with watch active | Close watcher before returning GoBackMsg |
| Quit with watch active | Watcher closed in "q", "ctrl+c" case |

### Testing Strategy

**Unit Tests (viewer_test.go, conversation_test.go):**
```go
func TestWatchModeToggleOn(t *testing.T) {
    // Given: viewer with watchMode=false, valid FilePath in renderOpts
    // When: 'w' key pressed
    // Then: watchMode=true, watcher != nil, returns WaitForEvent cmd
}

func TestWatchModeToggleOff(t *testing.T) {
    // Given: viewer with watchMode=true, watcher active
    // When: 'w' key pressed
    // Then: watchMode=false, watcher=nil, newEntriesCount=0
}

func TestWatchModeNoFilePathNoOp(t *testing.T) {
    // Given: viewer with watchMode=false, FilePath="" in renderOpts
    // When: 'w' key pressed
    // Then: watchMode remains false (no-op)
}

func TestWatcherClosedOnBackNavigation(t *testing.T) {
    // Given: viewer with watchMode=true, watcher active, canGoBack=true
    // When: 'h' or 'esc' key pressed
    // Then: watcher.Close() called, GoBackMsg returned
}

func TestConversationWatchMsgCreatesWatchViewer(t *testing.T) {
    // Given: AppModel in viewConversations state
    // When: ConversationSelectedWithWatchMsg received
    // Then: ViewerModel created with watchMode=true, watcher initialized
}
```

**Manual Tests:**
```bash
# Test AC 2.4.1 - Watch from conversation list
./bin/cclv
# Navigate to project, press 'w' on conversation
# Verify: LIVE indicator shows, new entries auto-scroll

# Test AC 2.4.2 - Toggle on in viewer
./bin/cclv
# Navigate to project, press Enter on conversation
# Press 'w'
# Verify: LIVE indicator appears

# Test AC 2.4.3 - Toggle off in viewer
# With LIVE active, press 'w' again
# Verify: LIVE indicator disappears

# Test resource cleanup - Back navigation
./bin/cclv
# Navigate to conversation, press 'w' to enable watch
# Press 'h' or 'esc' to go back
# Verify: No resource leak (watcher closed)

# Test AC 2.4.4 - Help documentation
./bin/cclv --help
# Verify: 'w' shortcut documented in keyboard shortcuts
```

### Git Intelligence

Recent commits from Epic 2:
```
e217df7 feat: implement smart auto-scroll for live watch mode
bbbf70a feat: implement fsnotify file watching for live log updates
82ac5ae feat: add --watch and --live CLI flags for file watching mode
```

This story completes Epic 2 by adding interactive watch mode capability on top of the existing CLI flag-based watch mode.

Suggested commit message:
```
feat: enable watch mode from interactive browse

- Add 'w' key in conversation list to open with watch mode
- Add 'w' key in viewer to toggle watch mode on/off
- Create ConversationSelectedWithWatchMsg for watch mode selection
- Update help text in viewer, conversation list, and --help output
- Properly manage watcher lifecycle on toggle

Completes Epic 2: Real-time File Watching

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Performance Considerations

- Watcher creation is O(1) - single fsnotify.Add() call
- Watcher close is O(1) - single fsnotify.Close() call
- Toggle operation is instant - no file I/O until events occur
- No impact on scrolling performance in viewer

### Dependencies

- **Story 2.1** (done): Provides `--watch`/`--live` flags, `watchMode` field, `RenderOptions.WatchMode`
- **Story 2.2** (done): Provides `watcher.Watcher`, `watcher.New()`, message types
- **Story 2.3** (done): Provides smart auto-scroll, `newEntriesCount`, "LIVE" indicator in status bar

### Project Structure Notes

**Files to modify:**
- `internal/tui/conversation.go` - Add 'w' key handler, new message type (lines 163, 208)
- `internal/tui/viewer.go` - Add 'w' key handler for toggle, close watcher on back nav (lines 285, 311, 520)
- `internal/tui/app.go` - Handle ConversationSelectedWithWatchMsg, fix FilePath propagation (lines 151, 264)
- `cmd/cclv/main.go` - Update --help output (line 67)

**Alignment with unified project structure:**
- All changes in existing files - no new files needed
- Follows established TEA patterns from Stories 2.1-2.3
- Uses existing watcher package infrastructure
- Import watcher package only in viewer.go (already imported)

### Existing Infrastructure to Reuse

| Component | Location | Use |
|-----------|----------|-----|
| `watcher.New()` | internal/watcher/watcher.go:29 | Create file watcher |
| `watcher.WaitForEvent()` | internal/watcher/watcher.go:55 | Start event loop |
| `watcher.Close()` | internal/watcher/watcher.go:133 | Clean up resources |
| `NewEntriesMsg` | internal/watcher/messages.go:7 | New entries from file |
| `WatcherErrorMsg` | internal/watcher/messages.go:12 | Watcher error |
| `FileResetMsg` | internal/watcher/messages.go:17 | File truncation |
| `RenderOptions.WatchMode` | internal/tui/viewer.go:27 | Enable watch mode |
| `RenderOptions.FilePath` | internal/tui/viewer.go:28 | File path for watcher |
| `ViewerModel.watchMode` | internal/tui/viewer.go:80 | Current watch state |
| `ViewerModel.watcher` | internal/tui/viewer.go:81 | Watcher instance |
| `ViewerModel.newEntriesCount` | internal/tui/viewer.go:82 | Indicator count |

### References

- [Source: epics.md lines 808-875] - Story 2.4 requirements and acceptance criteria
- [Source: project-context.md] - Architecture constraints, TEA pattern, no emoji rule
- [Source: internal/tui/viewer.go:42-86] - ViewerModel struct with watchMode, watcher, renderOpts
- [Source: internal/tui/viewer.go:242-312] - KeyMsg handler in viewer (add 'w' case at line 311)
- [Source: internal/tui/viewer.go:285-289] - Back navigation handler (add watcher cleanup)
- [Source: internal/tui/conversation.go:182-228] - KeyMsg handler in conversation (add 'w' case at line 208)
- [Source: internal/tui/app.go:151-187] - ConversationSelectedMsg handler (pattern for new handler)
- [Source: internal/tui/app.go:264-268] - conversationLoadedMsg struct (add filePath field)
- [Source: internal/watcher/watcher.go:29-51] - watcher.New() for creating file watcher
- [Source: internal/watcher/watcher.go:133-141] - watcher.Close() for cleanup

## Implementation Checklist

Before marking story complete, verify:

- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes with no regressions
- [x] FilePath propagates to viewer in both Enter and 'w' flows
- [x] Watcher closes on back navigation (h/esc)
- [x] Watcher closes on quit (q/ctrl+c)
- [x] LIVE indicator shows/hides correctly
- [x] Help text updated in all three locations (viewer, conversation list, --help)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

N/A - Implementation straightforward with no debugging issues.

### Completion Notes List

1. Added `ConversationSelectedWithWatchMsg` message type for watch mode selection from conversation list
2. Implemented 'w' key handler in ConversationModel to open conversation with watch mode
3. Added `loadConversationWithWatch()` and `conversationLoadedWithWatchMsg` for async loading with watch mode
4. Implemented 'w' key toggle in ViewerModel to enable/disable watch mode dynamically
5. Fixed FilePath propagation in normal conversation open flow (via Enter key) to enable watch toggle
6. Added watcher cleanup on back navigation (h/esc) to prevent resource leaks
7. Updated help text in viewer status bar, conversation list, and --help output
8. Added comprehensive unit tests for watch mode toggle functionality

### Code Review Fixes Applied

9. Extracted duplicate title-building code in `app.go` to `buildConversationTitle()` helper method (DRY fix)
10. Added test for watch mode toggle with non-existent file path (edge case coverage)

### File List

- `internal/tui/conversation.go` - Added ConversationSelectedWithWatchMsg, 'w' key handler, updated help text
- `internal/tui/app.go` - Added conversationLoadedWithWatchMsg, handlers for watch mode messages, filePath field, buildConversationTitle helper
- `internal/tui/viewer.go` - Added 'w' key toggle, watcher cleanup on back navigation, updated shortcuts
- `cmd/cclv/main.go` - Added 'w' shortcut to --help output
- `internal/tui/viewer_test.go` - Added unit tests for watch mode toggle functionality, non-existent file test
