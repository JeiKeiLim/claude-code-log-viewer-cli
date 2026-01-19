# Story 5.4: New Conversation Detection

Status: done

## Story

As a **developer with active Claude sessions**,
I want **panes to auto-switch to new conversations**,
So that **I always see the latest activity**.

## Acceptance Criteria

### AC 5.4.1: Detect new conversation file creation
- **Given** a pane is watching a project's latest conversation
- **When** a new JSONL file is created in that project's `conversations/` folder
- **Then** the pane detects the new file
- **And** switches to display the new conversation

### AC 5.4.2: Visual indicator on conversation switch
- **Given** a pane switches to a new conversation
- **When** the switch occurs
- **Then** a brief visual indicator shows (e.g., header flash or "[NEW]" badge)
- **And** the indicator clears after ~1-2 seconds

### AC 5.4.3: Seamless watcher handoff
- **Given** a pane is currently watching conversation A
- **When** a new conversation B is detected and switch occurs
- **Then** watcher for conversation A is closed properly
- **And** watcher for conversation B is started
- **And** no watcher resource leaks occur

### AC 5.4.4: Directory watching
- **Given** a pane needs to detect new conversations
- **When** it initializes or switches conversations
- **Then** it watches the project's `conversations/` directory (not just the current file)
- **And** uses fsnotify Create events to detect new files

## Tasks / Subtasks

- [x] Task 1: Add directory watcher field to PaneModel (AC: 5.4.1, 5.4.4)
  - [x] 1.1: Add `dirWatcher *fsnotify.Watcher` field to PaneModel for watching project conversations directory
  - [x] 1.2: Add `watchingDir string` field to track which directory is being watched
  - [x] 1.3: Document that we need TWO watchers per pane: one for file content, one for directory
  - [x] 1.4: Add import for `"github.com/fsnotify/fsnotify"` (raw fsnotify, not watcher package wrapper)

- [x] Task 2: Implement directory watch initialization (AC: 5.4.1, 5.4.4)
  - [x] 2.1: Create `initDirectoryWatcher(paneIndex int, projectPath string) tea.Cmd` function
  - [x] 2.2: Use fsnotify to watch `{projectPath}/conversations/` directory
  - [x] 2.3: Filter for Create events only (new file creation)
  - [x] 2.4: Filter for `.jsonl` files only (ignore other files like .DS_Store)
  - [x] 2.5: Return command that waits for directory events

- [x] Task 3: Create directory watcher message types (AC: 5.4.1)
  - [x] 3.1: Define `paneDirWatcherEventMsg` struct with `paneIndex int, newFilePath string`
  - [x] 3.2: Define `paneDirWatcherInitMsg` struct with `paneIndex int, watcher *fsnotify.Watcher, watchDir string`
  - [x] 3.3: Define `paneDirWatcherErrorMsg` struct with `paneIndex int, err error`
  - [x] 3.4: Update `paneWatcherEventMsg` doc comment to clarify it's for file content events (distinct from directory events)

- [x] Task 4: Implement directory event handling (AC: 5.4.1, 5.4.3)
  - [x] 4.1: Create `waitForDirEvent(paneIndex int) tea.Cmd` in DashboardModel
  - [x] 4.2: Handle fsnotify.Create event in the wait command
  - [x] 4.3: Extract filename from event path and verify `.jsonl` extension
  - [x] 4.4: Compare new file's ModTime with current conversation's ModTime
  - [x] 4.5: If new file is newer, emit `paneNewConversationMsg`
  - [x] 4.6: Always chain next directory wait command

- [x] Task 5: Implement conversation switch logic (AC: 5.4.1, 5.4.3)
  - [x] 5.1: Handle `paneNewConversationMsg` in DashboardModel.Update()
  - [x] 5.2: Close existing file watcher for the pane (if any)
  - [x] 5.3: Reset pane state: clear entries, set loading=true
  - [x] 5.4: Return `loadPaneContentCmd(paneIndex, projectPath)` to load new conversation
  - [x] 5.5: Set flag for visual indicator (see Task 6)

- [x] Task 6: Add visual indicator for new conversation (AC: 5.4.2)
  - [x] 6.1: Add `showNewIndicator bool` field to PaneModel
  - [x] 6.2: Add `newIndicatorExpiry time.Time` field to track indicator timeout
  - [x] 6.3: Set `showNewIndicator = true` and expiry when switching to new conversation
  - [x] 6.4: Create `paneIndicatorExpiredMsg` struct with `paneIndex int`
  - [x] 6.5: Create `paneIndicatorTimeoutCmd(paneIndex int, duration time.Duration) tea.Cmd`
  - [x] 6.6: Return timeout command when setting indicator

- [x] Task 7: Update PaneModel.View() for new indicator (AC: 5.4.2)
  - [x] 7.1: Check `p.showNewIndicator` flag in View()
  - [x] 7.2: If true, append "[NEW]" badge to project name in header
  - [x] 7.3: Style badge with accent color for visibility
  - [x] 7.4: Handle `paneIndicatorExpiredMsg` in Update() to clear flag

- [x] Task 8: Update DashboardModel initialization (AC: 5.4.4)
  - [x] 8.1: In `paneContentLoadedMsg` handler (NOT constructor), start directory watcher after content loads successfully
  - [x] 8.2: Return directory watch command in the batch along with file watcher command
  - [x] 8.3: Ensure directory watcher starts even if no conversations exist (create folder if needed per Task 10.4)
  - [x] 8.4: IMPORTANT: Directory watcher initialization happens in Update(), not NewDashboardModel() - follows TEA pattern like file watcher

- [x] Task 9: Update DashboardModel cleanup (AC: 5.4.3)
  - [x] 9.1: Extend `closeAllWatchers()` to also close directory watchers
  - [x] 9.2: Set both `pane.watcher = nil` and `pane.dirWatcher = nil` after close
  - [x] 9.3: Update any error handling to prevent panics on nil watchers

- [x] Task 10: Handle edge cases (AC: all)
  - [x] 10.1: Empty project (no conversations) - start directory watcher, wait for first file
  - [x] 10.2: Multiple rapid file creations - compare timestamps to switch to newest
  - [x] 10.3: File created but immediately deleted - check `os.Stat()` before switching, ignore if error
  - [x] 10.4: Project directory doesn't have `conversations/` folder yet:
    - Check if conversations dir exists: `os.Stat(conversationsDir)`
    - If not exists, create it: `os.MkdirAll(conversationsDir, 0755)`
    - Then start the directory watcher
    - This ensures watcher always has a valid directory to watch

- [x] Task 11: Add unit tests
  - [x] 11.1: Test `waitForDirEvent()` returns nil with nil watcher
  - [x] 11.2: Test `paneNewConversationMsg` handler resets pane state
  - [x] 11.3: Test `paneNewConversationMsg` handler returns batch command
  - [x] 11.4: Test `showNewIndicator` is set on conversation switch
  - [x] 11.5: Test `paneIndicatorExpiredMsg` clears the indicator
  - [x] 11.6: Test `closeAllWatchers()` handles both file and directory watchers
  - [x] 11.7: Test PaneModel.View() displays [NEW] badge
  - [x] 11.8: Test directory watcher init message stores watchDir
  - [x] 11.9: Test initDirectoryWatcher returns a command

- [x] Task 12: Run build, lint, and test validation
  - [x] 12.1: Run `make build` - verify binary builds successfully
  - [x] 12.2: Run `make lint` - no errors
  - [x] 12.3: Run `make test` - all tests pass

- [ ] Task 13: Manual testing (Requires user verification)
  - [ ] 13.1: Open dashboard with 2 projects
  - [ ] 13.2: Start a new Claude session in one of the projects (creates new JSONL file)
  - [ ] 13.3: Verify pane auto-switches to the new conversation
  - [ ] 13.4: Verify "[NEW]" indicator appears briefly
  - [ ] 13.5: Verify indicator clears after ~1-2 seconds
  - [ ] 13.6: Verify old conversation watcher was properly closed (no errors in logs)
  - [ ] 13.7: Verify new conversation shows live updates
  - [ ] 13.8: Press Escape - verify clean exit with no watcher errors

## Dev Notes

### Current Implementation State

Story 5.3 implemented:
- PaneModel with `watcher *watcher.Watcher` for file content watching
- `loadPaneContentCmd()` for loading conversation content
- `waitForPaneWatcher()` for chaining file watcher events
- `closeAllWatchers()` for cleanup on exit

**Key observation**: Current implementation only watches the *conversation file*. To detect *new* conversations, we need to also watch the *project directory*.

### Architecture Reference

From `architecture-phase3.md`:
- **FR-504**: New Conversation Detection - Auto-switch pane when new conversation starts in project
- **Decision 5**: New Conversation Detection - Directory watch + timestamp compare

The architecture specifies:
```
New Conversation Detection: Directory watch + timestamp compare
Detects new files, switches automatically
```

### Two-Watcher Architecture

Each pane needs TWO watchers:
1. **File watcher** (`watcher.Watcher`) - watches current conversation file for content updates
2. **Directory watcher** (`fsnotify.Watcher`) - watches project's conversations/ directory for new files

```go
// PaneModel extension
type PaneModel struct {
    // Existing
    watcher      *watcher.Watcher    // File content watcher

    // New for Story 5.4
    dirWatcher       *fsnotify.Watcher   // Directory watcher for new files
    watchingDir      string              // Directory being watched
    showNewIndicator bool                // Visual indicator (cleared via tea.Tick timeout)
}
```

### Directory Watch Pattern

```go
// Import required at top of dashboard.go:
import "github.com/fsnotify/fsnotify"

// initDirectoryWatcher creates and stores the directory watcher, returns wait command
// Called from paneContentLoadedMsg handler, NOT from constructor
func (m *DashboardModel) initDirectoryWatcher(paneIndex int, projectPath string) tea.Cmd {
    return func() tea.Msg {
        convsDir := filepath.Join(projectPath, "conversations")

        // Ensure directory exists (Task 10.4)
        if _, err := os.Stat(convsDir); os.IsNotExist(err) {
            if err := os.MkdirAll(convsDir, 0755); err != nil {
                return paneDirWatcherErrorMsg{paneIndex: paneIndex, err: err}
            }
        }

        w, err := fsnotify.NewWatcher()
        if err != nil {
            return paneDirWatcherErrorMsg{paneIndex: paneIndex, err: err}
        }

        if err := w.Add(convsDir); err != nil {
            _ = w.Close()
            return paneDirWatcherErrorMsg{paneIndex: paneIndex, err: err}
        }

        // Return init success message with watcher reference
        return paneDirWatcherInitMsg{paneIndex: paneIndex, watcher: w, watchDir: convsDir}
    }
}

// waitForDirEvent blocks on directory watcher events (runs in goroutine via tea.Cmd)
func (m *DashboardModel) waitForDirEvent(paneIndex int) tea.Cmd {
    if paneIndex >= len(m.panes) || m.panes[paneIndex].dirWatcher == nil {
        return nil
    }
    w := m.panes[paneIndex].dirWatcher
    return func() tea.Msg {
        // This runs in a goroutine - blocking is safe
        for {
            select {
            case event := <-w.Events:
                if event.Op&fsnotify.Create != 0 {
                    if strings.HasSuffix(event.Name, ".jsonl") {
                        // Verify file still exists (Task 10.3)
                        if _, err := os.Stat(event.Name); err == nil {
                            return paneDirWatcherEventMsg{paneIndex: paneIndex, newFilePath: event.Name}
                        }
                    }
                }
            case err := <-w.Errors:
                return paneDirWatcherErrorMsg{paneIndex: paneIndex, err: err}
            }
        }
    }
}
```

### Complete State Transition Diagram

```
                    NewDashboardModel()
                           ↓
              loadPaneContentCmd(i, projectPath)
                           ↓
                  paneContentLoadedMsg
                    /              \
                   ↓                ↓
    initDirectoryWatcher()    startFileWatcher()
                   ↓                ↓
      paneDirWatcherInitMsg   waitForPaneWatcher()
                   ↓                ↓
          Store dirWatcher    [File watcher loop]
                   ↓
         waitForDirEvent()
                   ↓
    [Directory watcher loop] ←─────────────────────┐
                   ↓                               │
       paneDirWatcherEventMsg (new .jsonl)         │
                   ↓                               │
       DashboardModel.Update()                     │
         - Close old file watcher                  │
         - Set showNewIndicator = true             │
         - Return batch:                           │
           • loadPaneContentCmd(i)                 │
           • paneIndicatorTimeoutCmd(i, 2s)        │
           • waitForDirEvent(i) ───────────────────┘
                   ↓
         paneContentLoadedMsg
                   ↓
      Start new file watcher for new conversation
                   ↓
         paneIndicatorExpiredMsg (after 2s)
                   ↓
      Clear showNewIndicator flag
```

### Message Flow for New Conversation (Detailed)

```
Directory watcher detects Create event for new .jsonl
    ↓
paneDirWatcherEventMsg{paneIndex: i, newFilePath: "/path/to/new.jsonl"}
    ↓
DashboardModel.Update():
    1. Verify file exists with os.Stat() (Task 10.3)
    2. Close existing file watcher: pane.watcher.Close(), pane.watcher = nil
    3. Set pane.loading = true (clear old content)
    4. Set pane.showNewIndicator = true
    5. Set pane.newIndicatorExpiry = time.Now().Add(2 * time.Second)
    6. Update pane.conversation to point to new file path
    7. Return tea.Batch(
        loadPaneContentCmd(i, projectPath),     // Load new conversation
        paneIndicatorTimeoutCmd(i, 2*time.Second), // Auto-clear indicator
        waitForDirEvent(i),                     // Continue watching directory
      )
    ↓
paneContentLoadedMsg → loads new conversation, starts new file watcher
    ↓
paneIndicatorExpiredMsg → clears showNewIndicator flag
```

### Visual Indicator Approach

Simple approach: Append "[NEW]" badge to project name header

```go
// In PaneModel.View()
displayName := p.project.DisplayName
if p.showNewIndicator {
    badge := lipgloss.NewStyle().
        Foreground(Styles.Theme.Accent).
        Bold(true).
        Render(" [NEW]")
    displayName = displayName + badge
}
```

### Conversation Directory Location

Claude Code stores conversations in:
```
~/.claude/projects/{encoded-project-path}/conversations/*.jsonl
```

**Critical Path Resolution:**

The `types.Project` struct (used in PaneModel) has `DirPath` which is the **full absolute path** to the project folder:
```
DirPath = "/Users/username/.claude/projects/Users-username-GitHub-myproject"
```

The conversations subfolder is always:
```go
conversationsDir := filepath.Join(pane.project.DirPath, "conversations")
// Result: "/Users/username/.claude/projects/Users-username-GitHub-myproject/conversations"
```

**Note:** `pane.project.DirPath` is NOT the source project path (e.g., `/Users/username/GitHub/myproject`). It's the Claude metadata path. This is already correct in the existing `findLatestConversation()` function from Story 5.3.

### Edge Cases to Handle

1. **Empty project**: Directory watcher should still start, waiting for first conversation
2. **Rapid file creation**: If Claude rapidly creates multiple files, always switch to newest by ModTime
3. **File deletion after creation**: fsnotify may fire Create then Remove - ignore if file doesn't exist when we check
4. **No conversations folder**: Some project folders may not have `conversations/` yet - handle mkdir or wait

### Performance Consideration

- Directory watching with fsnotify is lightweight (one goroutine per watcher)
- 9 panes = max 18 watchers (9 file + 9 directory) - acceptable
- Event rate is low (new files created infrequently)

### Existing Types to Reuse

From `internal/watcher/watcher.go`:
- This package wraps file-level watching with entry parsing
- For directory watching, use raw `fsnotify.Watcher` directly (simpler, no parsing needed)

From `internal/tui/dashboard.go` (Story 5.3):
- `closeAllWatchers()` - extend to close directory watchers too
- `paneContentLoadedMsg` - reuse for loading new conversation
- `loadPaneContentCmd()` - reuse for loading new conversation content

### Files to Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/dashboard.go` | MODIFY | Add directory watcher fields, new message types, event handlers |
| `internal/tui/dashboard_test.go` | MODIFY | Add tests for new conversation detection |
| `go.mod` | CHECK | fsnotify should already be a dependency (used by watcher package) |

### Required Imports for dashboard.go

```go
import (
    "os"                              // NEW: for os.Stat, os.MkdirAll
    "path/filepath"                   // Already present, verify
    "strings"                         // Already present
    "time"                            // NEW: for time.Time, time.Second

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/fsnotify/fsnotify"    // NEW: raw fsnotify for directory watching

    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/parser"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"  // For file content watching
)
```

### Project Context Rules (from project-context.md)

| Rule | Application |
|------|-------------|
| **NO EMOJI IN UI** | Use text badge `[NEW]` not emoji indicator |
| **TEA pattern** | Directory events via Cmd, state in Update() |
| **Use Makefile** | `make build`, `make test` |
| **Watcher cleanup** | Close BOTH watchers on dashboard exit |
| **Error wrapping** | `fmt.Errorf("start dir watch: %w", err)` |

### Previous Story Learnings

From Story 5.3:
1. **Two-stage loading**: Content loads async via command, not in constructor
2. **Watcher chaining**: After handling watcher event, return next wait command
3. **Graceful degradation**: On watcher error, continue watching
4. **Pre-render content**: Render in Update(), cache result, display in View()

From Story 5.2:
1. **Manual border control**: Use `addBorder()` for reliable height
2. **Pre-computed dimensions**: Calculate in SetSize(), not View()

### Git Commit Pattern

Recent commits show pattern:
- `feat: implement new conversation detection in dashboard (Story 5.4)`
- `fix: close directory watchers on dashboard exit (Story 5.4)`

### References

- [Source: epics-phase3.md#Story-5.4] - Acceptance criteria
- [Source: prd-phase3.md#FR-504] - New conversation detection requirements
- [Source: architecture-phase3.md#Decision-5] - Directory watch + timestamp compare approach
- [Source: internal/tui/dashboard.go] - Current dashboard implementation with file watcher
- [Source: 5-3-dashboard-pane-content-display.md] - Previous story implementation
- [Source: project-context.md] - Code rules and patterns

## Implementation Checklist

Before marking story complete, verify:

**PaneModel Extension:**
- [x] `dirWatcher` field added (type: `*fsnotify.Watcher`)
- [x] `watchingDir` field added (type: `string`)
- [x] `showNewIndicator` field added (type: `bool`)
- [x] ~~`newIndicatorExpiry` field removed (dead code - indicator cleared via tea.Tick)~~

**Directory Watching:**
- [x] `initDirectoryWatcher()` creates fsnotify watcher for conversations/ folder
- [x] Conversations directory created if missing (`os.MkdirAll`)
- [x] Watches for Create events only
- [x] Filters for `.jsonl` files only
- [x] `waitForDirEvent()` wraps events with pane index
- [x] File existence verified before switching (`os.Stat`)

**Conversation Switch:**
- [x] `paneNewConversationMsg` handled in Update()
- [x] Old file watcher closed before loading new
- [x] New conversation loaded via `loadPaneContentCmd()`
- [x] Directory watch continues after switch

**Visual Indicator:**
- [x] `[NEW]` badge appended to header on switch
- [x] Badge uses accent color styling
- [x] `paneIndicatorTimeoutCmd()` returns timeout command
- [x] `paneIndicatorExpiredMsg` clears `showNewIndicator`

**Cleanup:**
- [x] `closeAllWatchers()` closes both file and directory watchers
- [x] No watcher leaks on dashboard exit

**Edge Cases:**
- [x] Empty project handled (directory watch starts, waits for first file)
- [x] Multiple rapid creates handled (switches to newest)
- [x] File filter works (ignores .DS_Store, etc.)

**Testing:**
- [x] Unit tests for directory watcher commands
- [x] Unit tests for conversation switch logic
- [x] Unit tests for indicator lifecycle
- [x] Unit tests for directory watcher init failure
- [x] Unit tests for conversations directory creation
- [x] Unit tests for file existence check before switch
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes

**Manual Verification (Requires User):**
- [ ] New conversation detected on file creation
- [ ] Pane auto-switches to new conversation
- [ ] "[NEW]" indicator appears and clears
- [ ] Old watcher cleaned up properly
- [ ] New content shows live updates
- [ ] Clean exit with no watcher errors

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Implemented two-watcher architecture: file watcher (content) + directory watcher (new files)
- Added 5 new message types for directory watching lifecycle
- Directory watcher starts after content loads (follows TEA pattern like file watcher)
- Visual [NEW] badge uses accent color, clears after 2 seconds via tea.Tick
- All edge cases handled: empty project, missing conversations dir, rapid file creation, deleted files
- 14 new unit tests added for Story 5.4 functionality (TestPaneNewConversationMsgHandler through TestPaneDirWatcherEventMsgHandlerFileOlder)
- Code review fixes applied: removed dead `newIndicatorExpiry` field, fixed visual width truncation for CJK/emoji
- Build, lint, and all tests pass

### File List

- internal/tui/dashboard.go (MODIFIED)
- internal/tui/dashboard_test.go (MODIFIED)
