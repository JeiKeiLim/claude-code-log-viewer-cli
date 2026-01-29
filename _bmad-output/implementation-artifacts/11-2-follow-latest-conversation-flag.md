# Story 11.2: Watch Mode Follow-Latest Conversation Flag

Status: done

## Story

As a **cclv user watching a project**,
I want **an option to automatically switch to the newest conversation**,
So that **I always see the latest activity even when new claude sessions start**.

## Acceptance Criteria

1. **AC-1: New CLI Flag**
   - Given user runs `cclv -w -L` or `cclv -w --follow-latest`
   - When viewing a conversation
   - Then follow-latest mode is enabled

2. **AC-2: Detect New Conversation**
   - Given follow-latest is enabled
   - When a new .jsonl file is created in the project directory
   - Then CCLV detects it within 2 seconds

3. **AC-3: Automatic Switch**
   - Given new conversation is detected
   - When new conversation is confirmed newer than current (by creation time/birthtime)
   - Then viewer switches to the new conversation
   - And live streaming continues on new file

4. **AC-4: Switch Notification**
   - Given viewer switches to new conversation
   - When switch occurs
   - Then toast shows "Switched to new conversation: <timestamp>"
   - And toast auto-dismisses after 3 seconds

5. **AC-5: Flag Without Watch Mode**
   - Given user runs `cclv -L` without `-w`
   - When command executes
   - Then error: "Error: --follow-latest requires --watch mode\n"
   - And exit with code 1

6. **AC-6: Works With Interactive Browse**
   - Given user is in interactive browse mode
   - When user enters watch mode on a conversation with `w` key
   - Then they can toggle follow-latest with `L` key
   - And project watcher is started/stopped accordingly

## Tasks / Subtasks

- [x] Task 1: Add CLI flag (AC: #1, #5)
  - [x] 1.1: Add flags in `cmd/cclv/main.go` using existing pattern:
    ```go
    followLatestFlag := flag.Bool("follow-latest", false, "Follow to newest conversation (requires --watch)")
    followLatestShortFlag := flag.Bool("L", false, "Follow to newest conversation (requires --watch)")
    ```
  - [x] 1.2: After `flag.Parse()`, combine flags: `followLatest := *followLatestFlag || *followLatestShortFlag`
  - [x] 1.3: Add validation AFTER existing watch mode validation (line ~170):
    ```go
    if followLatest && !watchMode {
        fmt.Fprintf(os.Stderr, "Error: --follow-latest requires --watch mode\n")
        os.Exit(1)
    }
    ```
  - [x] 1.4: Derive project path: `projectPath := filepath.Dir(absPath)` when followLatest is true

- [x] Task 2: Extend RenderOptions (AC: #1)
  - [x] 2.1: Add fields in `internal/tui/viewer.go:36-43`:
    ```go
    type RenderOptions struct {
        HideThoughts bool
        HideTools    bool
        Width        int
        WatchMode    bool
        FilePath     string
        FollowLatest bool   // NEW: Enable follow-latest mode
        ProjectPath  string // NEW: Directory path for project watcher
    }
    ```
  - [x] 2.2: Pass to opts in main.go when constructing RenderOptions (line ~217)

- [x] Task 3: Create ProjectWatcher type (AC: #2)
  - [x] 3.1: Create new file `internal/watcher/project_watcher.go`:
    ```go
    type ProjectWatcher struct {
        projectPath string
        fsWatcher   *fsnotify.Watcher
        mu          sync.Mutex
        closed      bool
    }
    ```
  - [x] 3.2: Implement `NewProjectWatcher(projectPath string) (*ProjectWatcher, error)`:
    - Create fsnotify watcher
    - Add project directory to watch list
    - Return ProjectWatcher pointer
  - [x] 3.3: Implement `WaitForNewConversation() tea.Cmd`:
    - Loop waiting on `fsWatcher.Events` channel
    - Filter for `fsnotify.Create` events on `.jsonl` files
    - Get birthtime via `scanner.GetBirthtime(info)`
    - Return `NewConversationMsg{FilePath, CreationTime}`
  - [x] 3.4: **CRITICAL (macOS kqueue)** - Implement `Close() error`:
    ```go
    func (w *ProjectWatcher) Close() error {
        w.mu.Lock()
        defer w.mu.Unlock()
        if w.closed {
            return nil
        }
        w.closed = true
        if w.fsWatcher != nil {
            // MUST call Remove() before Close() on macOS (Story 9.1)
            for _, path := range w.fsWatcher.WatchList() {
                _ = w.fsWatcher.Remove(path)
            }
            return w.fsWatcher.Close()
        }
        return nil
    }
    ```
  - [x] 3.5: Implement `IsClosed() bool` with mutex protection

- [x] Task 4: Add new message types (AC: #2, #3)
  - [x] 4.1: Add to `internal/watcher/messages.go`:
    ```go
    type NewConversationMsg struct {
        FilePath     string
        CreationTime time.Time
    }
    ```

- [x] Task 5: Extend ViewerModel for follow-latest (AC: #2, #3, #6)
  - [x] 5.1: Add fields to ViewerModel struct:
    ```go
    followLatest           bool
    projectWatcher         *watcher.ProjectWatcher
    currentConversationPath string
    currentCreationTime    time.Time  // For birthtime comparison
    switchInProgress       bool       // Debounce rapid switches
    ```
  - [x] 5.2: In `NewViewerModel`, after watcher creation (line ~271):
    ```go
    if opts.FollowLatest && opts.ProjectPath != "" {
        pw, err := watcher.NewProjectWatcher(opts.ProjectPath)
        if err == nil {
            m.projectWatcher = pw
            m.followLatest = true
            m.currentConversationPath = opts.FilePath
            // Get current conversation's birthtime
            if info, err := os.Stat(opts.FilePath); err == nil {
                m.currentCreationTime = scanner.GetBirthtime(info)
                if m.currentCreationTime.IsZero() {
                    m.currentCreationTime = info.ModTime()
                }
            }
        }
    }
    ```
  - [x] 5.3: In `Init()` (line ~315), add project watcher command if active:
    ```go
    var cmds []tea.Cmd
    if m.watcher != nil {
        cmds = append(cmds, m.watcher.WaitForEvent())
    }
    if m.projectWatcher != nil {
        cmds = append(cmds, m.projectWatcher.WaitForNewConversation())
    }
    return tea.Batch(cmds...)
    ```

- [x] Task 6: Handle NewConversationMsg in ViewerModel.Update (AC: #2, #3, #4)
  - [x] 6.1: Add case in Update() (after watcher.WatcherErrorMsg case, line ~767):
    ```go
    case watcher.NewConversationMsg:
        // Skip if switch already in progress (debounce)
        if m.switchInProgress {
            if m.projectWatcher != nil {
                return m, m.projectWatcher.WaitForNewConversation()
            }
            return m, nil
        }

        // Skip if not newer than current (compare birthtimes)
        if !msg.CreationTime.After(m.currentCreationTime) {
            if m.projectWatcher != nil {
                return m, m.projectWatcher.WaitForNewConversation()
            }
            return m, nil
        }

        // Handle birthtime zero (fallback already applied in sender)
        if msg.CreationTime.IsZero() {
            // Skip - can't determine if newer
            if m.projectWatcher != nil {
                return m, m.projectWatcher.WaitForNewConversation()
            }
            return m, nil
        }

        m.switchInProgress = true

        // Close current file watcher
        if m.watcher != nil {
            _ = m.watcher.Close()
            m.watcher = nil
        }

        // Parse new conversation
        result, err := parser.ParseJSONLFile(msg.FilePath)
        if err != nil {
            m.switchInProgress = false
            if m.projectWatcher != nil {
                return m, m.projectWatcher.WaitForNewConversation()
            }
            return m, nil
        }

        // Update state (pattern from Story 11.1: capture before modify)
        m.entries = result.Entries
        m.loadedCount = len(m.entries)
        m.parseErrors = result.ParseErrors
        m.currentConversationPath = msg.FilePath
        m.currentCreationTime = msg.CreationTime
        m.renderOpts.FilePath = msg.FilePath
        m.gutterWidth = calculateGutterWidth(len(m.entries))
        m.newEntriesCount = 0

        // Recalculate tokens
        if m.tokenService != nil {
            m.conversationTokens, m.tokensEstimated = m.tokenService.CalculateConversation(m.entries)
        }

        // Exit raw mode on switch (consistent with file reset behavior)
        if m.rawMode {
            m.rawMode = false
        }

        m.invalidateRenderCache()
        m.updateContent()
        m.viewport.GotoBottom()

        // Start new file watcher
        w, err := watcher.New(msg.FilePath)
        if err == nil {
            m.watcher = w
        }

        m.switchInProgress = false

        // Format timestamp for toast
        timestamp := msg.CreationTime.Format("15:04:05")
        toastCmd := m.showToast(fmt.Sprintf("Switched to new conversation: %s", timestamp), ToastDuration)

        var cmds []tea.Cmd
        cmds = append(cmds, toastCmd)
        if m.watcher != nil {
            cmds = append(cmds, m.watcher.WaitForEvent())
        }
        if m.projectWatcher != nil {
            cmds = append(cmds, m.projectWatcher.WaitForNewConversation())
        }
        return m, tea.Batch(cmds...)
    ```

- [x] Task 7: Add `L` key toggle for follow-latest in interactive mode (AC: #6)
  - [x] 7.1: Add case in KeyMsg handler (after "w" case, line ~537):
    ```go
    case "L":
        // Only allow toggle when in watch mode
        if !m.watchMode {
            return m, m.showToast("Follow-latest requires watch mode", ToastDuration)
        }

        if m.followLatest {
            // Disable follow-latest
            if m.projectWatcher != nil {
                _ = m.projectWatcher.Close()
                m.projectWatcher = nil
            }
            m.followLatest = false
            return m, m.showToast("Follow-latest: OFF", ToastDuration)
        }

        // Enable follow-latest
        projectPath := filepath.Dir(m.renderOpts.FilePath)
        pw, err := watcher.NewProjectWatcher(projectPath)
        if err != nil {
            return m, m.showToast("Cannot start follow-latest", ToastDuration)
        }
        m.projectWatcher = pw
        m.followLatest = true
        m.currentConversationPath = m.renderOpts.FilePath

        // Get current file's birthtime
        if info, err := os.Stat(m.renderOpts.FilePath); err == nil {
            m.currentCreationTime = scanner.GetBirthtime(info)
            if m.currentCreationTime.IsZero() {
                m.currentCreationTime = info.ModTime()
            }
        }

        return m, tea.Batch(
            m.showToast("Follow-latest: ON", ToastDuration),
            m.projectWatcher.WaitForNewConversation(),
        )
    ```

- [x] Task 8: Update help/shortcuts display (AC: #6)
  - [x] 8.1: Add to `buildShortcutsSegment()` (line ~987), conditionally when watchMode:
    ```go
    if m.watchMode {
        if m.followLatest {
            parts = append(parts, "L:unfollow")
        } else {
            parts = append(parts, "L:follow")
        }
    }
    ```
  - [x] 8.2: Update `printHelp()` in main.go OPTIONS section:
    ```
    -L, --follow-latest   Follow newest conversation (requires --watch)
    ```
  - [x] 8.3: Update EXAMPLES section:
    ```
    cclv -w -L conversation.jsonl    Watch and follow latest conversation
    ```
  - [x] 8.4: Update KEYBOARD SHORTCUTS Toggles section:
    ```
    L               Toggle follow-latest (in watch mode)
    ```

- [x] Task 9: Update buildModeSegment for FOLLOW indicator (AC: #3)
  - [x] 9.1: Add to `buildModeSegment()` (line ~923, before len check):
    ```go
    if m.followLatest && m.projectWatcher != nil {
        modes = append(modes, "FOLLOW")
    }
    ```

- [x] Task 10: Cleanup on navigation back (AC: #6)
  - [x] 10.1: Update back navigation handler (line ~477) to also close project watcher:
    ```go
    case "h", "esc":
        if m.canGoBack {
            if m.watcher != nil {
                _ = m.watcher.Close()
            }
            if m.projectWatcher != nil {
                _ = m.projectWatcher.Close()
            }
            return m, func() tea.Msg { return GoBackMsg{} }
        }
    ```

- [x] Task 11: Add unit tests (AC: #1, #2, #5)
  - [x] 11.1: In `cmd/cclv/main_test.go`, add flag parsing tests:
    ```go
    func TestFollowLatestFlagValidation(t *testing.T) {
        tests := []struct {
            name    string
            args    []string
            wantErr bool
        }{
            {"L with watch", []string{"cmd", "-w", "-L", "file.jsonl"}, false},
            {"L without watch", []string{"cmd", "-L", "file.jsonl"}, true},
            {"follow-latest long form", []string{"cmd", "--watch", "--follow-latest", "file.jsonl"}, false},
        }
        // ... test implementation
    }
    ```
  - [x] 11.2: In `internal/watcher/project_watcher_test.go`, add:
    - Test NewProjectWatcher with valid/invalid paths
    - Test WaitForNewConversation returns correct message on CREATE event
    - Test Close() is idempotent (can call multiple times)
    - Test Close() removes paths before closing (macOS safety)
  - [x] 11.3: In `internal/tui/viewer_test.go`, add:
    - Test NewConversationMsg handling switches when newer
    - Test NewConversationMsg ignores when not newer
    - Test L key toggle starts/stops project watcher
    - Test L key shows error toast when not in watch mode
    - Test buildModeSegment displays "FOLLOW" when active

- [x] Task 12: Manual verification (CLI smoke test per project-context.md)
  - [x] 12.1: `make build` passes
  - [x] 12.2: `make test` passes
  - [x] 12.3: Run `cclv -w -L conversation.jsonl`, verify FOLLOW appears in mode segment
  - [x] 12.4: Start new claude session in same project, verify switch occurs with toast
  - [x] 12.5: Verify live streaming works on new conversation after switch
  - [x] 12.6: Test `cclv -L` without `-w` shows correct error message
  - [x] 12.7: Test interactive mode: enter viewer, press `w` for watch, press `L`, verify toggle and toast

## Dev Notes

### Critical Implementation Patterns

**macOS File Descriptor Leak Prevention (Story 9.1):**
The ProjectWatcher MUST call `Remove()` on all watched paths before `Close()`. On macOS, fsnotify uses kqueue which opens a file descriptor for EACH watched path. Failure to remove before close causes FD leaks.

**State Capture Pattern (Story 11.1):**
When handling NewConversationMsg, capture relevant state BEFORE modifying to avoid race conditions.

**Debouncing Concurrent Operations:**
- `switchInProgress` flag prevents rapid switch attempts
- Check `!m.switchInProgress` before processing NewConversationMsg
- Reset flag after switch completes (success or failure)

### Concurrent Watcher Lifecycle

Both watchers can be active simultaneously:
- **File Watcher**: Watches single file for WRITE events (content updates)
- **Project Watcher**: Watches directory for CREATE events (new conversations)

Cleanup order when navigating back or toggling:
1. Close file watcher first (stops content updates)
2. Close project watcher second
3. Set both to nil

### Birthtime Comparison Edge Cases

| Scenario | Handling |
|----------|----------|
| Both birthtimes valid | Compare with `After()` |
| New file birthtime zero | Skip switch (can't determine if newer) |
| Current birthtime zero | Accept switch if new birthtime > 0 |
| Both zero | Skip switch (indeterminate) |
| Equal birthtimes | Skip switch (not newer) |

### Error Handling Strategy

| Error | Handling |
|-------|----------|
| ProjectWatcher creation fails | Log, continue without follow-latest |
| File stat fails on new file | Skip this CREATE event, continue watching |
| Parse fails on new conversation | Skip switch, show toast, continue watching |
| New file watcher creation fails | Switch completes but no live updates |

### Files to Modify

| File | Changes | Key Lines |
|------|---------|-----------|
| `cmd/cclv/main.go` | Add `-L`/`--follow-latest` flags, validation, ProjectPath derivation | ~132 (flags), ~170 (validation), ~217 (opts) |
| `internal/tui/viewer.go` | Add fields, RenderOptions extension, NewConversationMsg handler, L key toggle, cleanup | ~36 (RenderOptions), ~105 (fields), ~315 (Init), ~537 (L key), ~767 (msg handler), ~913 (mode), ~969 (shortcuts) |
| `internal/watcher/project_watcher.go` | NEW FILE: ProjectWatcher type with WaitForNewConversation | N/A |
| `internal/watcher/messages.go` | Add NewConversationMsg type | ~23 |

### Imports Required

In `viewer.go`, add:
```go
import (
    "path/filepath"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
)
```

In `project_watcher.go`:
```go
import (
    "strings"
    "sync"
    "time"
    "github.com/fsnotify/fsnotify"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/scanner"
)
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4-epic11.md#Story 11.2]
- [Source: internal/watcher/watcher.go - existing watcher patterns, Close() with Remove()]
- [Source: internal/scanner/birthtime_darwin.go - GetBirthtime function]
- [Source: project-context.md - Bubbletea patterns, error handling]
- [Source: 11-1-auto-detect-auth-refresh.md - state capture pattern, toast usage]

---

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - No debug logs needed

### Completion Notes List

- Implemented `-L`/`--follow-latest` CLI flags with validation requiring `--watch` mode
- Created `ProjectWatcher` type in `internal/watcher/project_watcher.go` for directory monitoring
- Added `NewConversationMsg` message type for new conversation detection
- Extended `ViewerModel` with follow-latest state fields and initialization
- Implemented `NewConversationMsg` handler with birthtime comparison and conversation switching
- Added `L` key toggle for follow-latest in watch mode (AC-6)
- Updated help text, shortcuts display, and mode segment with FOLLOW indicator
- Added proper cleanup on quit and back navigation
- All unit tests pass (new tests for flag parsing, ProjectWatcher, and viewer model)
- Build compiles without errors

### Code Review Fixes (2026-01-29)

- Added `-w` shorthand flag for `--watch` (was missing, test assumed it existed)
- Added tests for NewConversationMsg handling (switches when newer, ignores when not)
- Added test for switchInProgress debouncing behavior

### File List

- `cmd/cclv/main.go` - Added flags (-w, -L, --follow-latest), validation, help text, ProjectPath derivation
- `cmd/cclv/main_test.go` - Added flag parsing tests
- `internal/tui/viewer.go` - Extended RenderOptions, ViewerModel, Update handler, shortcuts
- `internal/tui/viewer_test.go` - Added follow-latest tests, NewConversationMsg handler tests
- `internal/watcher/project_watcher.go` - NEW: ProjectWatcher type for directory monitoring
- `internal/watcher/project_watcher_test.go` - NEW: ProjectWatcher unit tests
- `internal/watcher/messages.go` - Added NewConversationMsg type
