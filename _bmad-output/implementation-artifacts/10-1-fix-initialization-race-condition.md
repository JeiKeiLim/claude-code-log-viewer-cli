# Story 10.1: Fix Dashboard Initialization Race Condition

Status: done

## Story

As a **cclv user viewing a project dashboard**,
I want **the dashboard to always show the latest conversation**,
So that **I don't miss recent activity due to timing issues**.

## Acceptance Criteria

1. **AC-1: Re-scan After Watcher Ready**
   - Given dashboard enters a project
   - When directory watcher initialization completes (`paneDirWatcherInitMsg` handler)
   - Then a fresh scan for latest conversation is performed
   - And if a newer conversation exists, the pane updates to show it

2. **AC-2: Stable Sort for Equal Timestamps**
   - Given multiple conversation files with identical modification times
   - When sorting by "latest"
   - Then files are sorted by filename (descending) as tiebreaker
   - And the result is deterministic across runs

3. **AC-3: Preserve LastModified in Conversation Struct**
   - Given a conversation is loaded into a pane
   - When the Conversation struct is stored in `pane.conversation`
   - Then both `FilePath` and `LastModified` are preserved (not just FilePath)

4. **AC-4: No Visual Flicker**
   - Given the re-scan finds the same conversation
   - When comparing current vs scanned
   - Then no reload occurs (avoid unnecessary UI update)

## Tasks / Subtasks

- [x] Task 1: Add LastModified to paneContentLoadedMsg (AC: #3)
  - [x] 1.1: Modify `paneContentLoadedMsg` struct to include `lastModified time.Time` field
  - [x] 1.2: Update `loadPaneContentCmd()` to populate `lastModified` from file stat
  - [x] 1.3: Update `paneContentLoadedMsg` handler (line ~501) to preserve LastModified in `pane.conversation`

- [x] Task 2: Implement re-scan after watcher ready (AC: #1, #4)
  - [x] 2.1: Create `rescanLatestCmd(paneIndex int, projectPath string) tea.Cmd` function that:
    - Takes `projectPath` as parameter (captures value at call time, avoids stale closure)
    - Calls `scanner.ScanConversationsLazy(projectPath)` and takes first result
    - Returns a new message type `paneRescanResultMsg`
  - [x] 2.2: Add `paneRescanResultMsg` message type with fields: `paneIndex`, `latestConv types.Conversation`, `err error`
  - [x] 2.3: Modify `paneDirWatcherInitMsg` handler (lines 517-526) to call `rescanLatestCmd(msg.paneIndex, pane.project.DirPath)`
  - [x] 2.4: Add `paneRescanResultMsg` handler in Update() that:
    - Compares scanned `latestConv.FilePath` with current `pane.conversation.FilePath`
    - If different AND newer, triggers `paneNewConversationMsg`
    - If same, does nothing (no flicker)

- [x] Task 3: Implement stable sort with filename tiebreaker (AC: #2)
  - [x] 3.1: Modify `ScanConversationsLazy()` sort in `internal/scanner/projects.go` (line ~303-305):
    ```go
    sort.Slice(conversations, func(i, j int) bool {
        if conversations[i].LastModified.Equal(conversations[j].LastModified) {
            // Tiebreaker: filename descending (newer UUIDs/timestamps tend to sort later)
            return conversations[i].FilePath > conversations[j].FilePath
        }
        return conversations[i].LastModified.After(conversations[j].LastModified)
    })
    ```
  - [x] 3.2: Add unit test for stable sort with equal timestamps

- [x] Task 4: Add unit tests (AC: #1, #2, #3, #4)
  - [x] 4.1: Test `paneContentLoadedMsg` includes and preserves LastModified
  - [x] 4.2: Test re-scan logic: different conversation triggers reload
  - [x] 4.3: Test re-scan logic: same conversation skips reload (no flicker)
  - [x] 4.4: Test stable sort determinism with equal timestamps

- [x] Task 5: Manual verification
  - [x] 5.1: Build passes (`make build`)
  - [x] 5.2: Tests pass (`make test`)
  - [x] 5.3: Race condition test: Logic verified via unit tests (TestPaneRescanResultMsgHandlerNewerConversation, TestPaneDirWatcherInitMsgTriggersRescan)
  - [x] 5.4: No-flicker test: Logic verified via unit test (TestPaneRescanResultMsgHandlerSameConversation)

## Dev Notes

### Problem: Initialization Race Condition

**Current flow in `dashboard.go`:**

```
User enters single project
     |
loadPaneContentCmd (async) -----+
     |                          |  <-- NEW CONVERSATION CREATED HERE = MISSED
paneContentLoadedMsg            |
     |                          |
initDirectoryWatcher() ---------+
     |
Watcher ready (paneDirWatcherInitMsg) -- 200ms+ total delay
```

If a new conversation is created between content load and watcher startup, it's **invisible** with no recovery mechanism.

### Solution: Re-scan After Watcher Ready

Add a re-scan step in `paneDirWatcherInitMsg` handler:

```go
case paneDirWatcherInitMsg:
    // Handle directory watcher initialization success
    if msg.paneIndex >= 0 && msg.paneIndex < len(m.panes) {
        pane := &m.panes[msg.paneIndex]
        pane.dirWatcher = msg.watcher
        pane.watchingDir = msg.watchDir
        // Story 9.2: Start subscription goroutine
        m.startDirWatcherSubscription(m.ctx, msg.paneIndex)

        // NEW (Story 10.1): Re-scan for latest conversation after watcher is ready
        // Pass projectPath explicitly to avoid stale closure capture
        return m, rescanLatestCmd(msg.paneIndex, pane.project.DirPath)
    }
    return m, nil
```

### Key Implementation Details

**1. paneContentLoadedMsg lastModified field:**

```go
// dashboard.go line ~64-70
type paneContentLoadedMsg struct {
    paneIndex    int
    entries      []types.LogEntry
    parseErrors  int
    filePath     string
    lastModified time.Time  // ADD THIS
    err          error
}
```

**2. loadPaneContentCmd update:**

```go
// In loadPaneContentCmd(), after getting filePath:
info, _ := os.Stat(filePath)
return paneContentLoadedMsg{
    paneIndex:    paneIndex,
    entries:      result.Entries,
    parseErrors:  result.ParseErrors,
    filePath:     filePath,
    lastModified: info.ModTime(),  // ADD THIS
}
```

**3. paneContentLoadedMsg handler update:**

```go
// dashboard.go line ~501
// BEFORE:
pane.conversation = types.Conversation{FilePath: msg.filePath}

// AFTER:
pane.conversation = types.Conversation{
    FilePath:     msg.filePath,
    LastModified: msg.lastModified,
}
```

**4. New message type and command:**

```go
// paneRescanResultMsg signals result of re-scanning for latest conversation
type paneRescanResultMsg struct {
    paneIndex  int
    latestConv types.Conversation
    err        error
}

// rescanLatestCmd returns a command that re-scans for the latest conversation.
// CRITICAL: projectPath must be passed as parameter (not read from m.panes inside closure)
// because the closure executes asynchronously and m.panes may have changed.
func rescanLatestCmd(paneIndex int, projectPath string) tea.Cmd {
    return func() tea.Msg {
        conversations, err := scanner.ScanConversationsLazy(projectPath)
        if err != nil {
            return paneRescanResultMsg{paneIndex: paneIndex, err: err}
        }
        if len(conversations) == 0 {
            return paneRescanResultMsg{paneIndex: paneIndex, err: nil} // Empty project, no action needed
        }
        return paneRescanResultMsg{paneIndex: paneIndex, latestConv: conversations[0]}
    }
}
```

**5. Handler for paneRescanResultMsg:**

```go
case paneRescanResultMsg:
    if msg.err != nil || msg.paneIndex < 0 || msg.paneIndex >= len(m.panes) {
        return m, nil
    }
    pane := &m.panes[msg.paneIndex]

    // Compare: only switch if different and newer
    if msg.latestConv.FilePath != pane.conversation.FilePath {
        // Different file - check if actually newer
        if msg.latestConv.LastModified.After(pane.conversation.LastModified) {
            return m, func() tea.Msg {
                return paneNewConversationMsg{
                    paneIndex:   msg.paneIndex,
                    newFilePath: msg.latestConv.FilePath,
                }
            }
        }
    }
    // Same file or not newer - no action (prevents flicker)
    return m, nil
```

### Stable Sort with Filename Tiebreaker

```go
// scanner/projects.go line ~303-305
// BEFORE:
sort.Slice(conversations, func(i, j int) bool {
    return conversations[i].LastModified.After(conversations[j].LastModified)
})

// AFTER:
sort.Slice(conversations, func(i, j int) bool {
    if conversations[i].LastModified.Equal(conversations[j].LastModified) {
        // Tiebreaker: filename descending
        return conversations[i].FilePath > conversations[j].FilePath
    }
    return conversations[i].LastModified.After(conversations[j].LastModified)
})
```

### Project Structure Notes

- **Files to modify:**
  - `internal/tui/dashboard.go` - Re-scan logic, LastModified preservation
  - `internal/scanner/projects.go` - Stable sort fix

- **No new packages or dependencies required**

- **Note:** `errors` package is NOT needed - we use `err: nil` for empty projects (valid state)

- **Note:** `ScanConversationsLazy` has NO limit parameter - take `[0]` from result slice instead

- **Pattern alignment:**
  - Message types follow `pane{action}Msg` convention
  - Command follows `{action}Cmd` convention
  - TEA pattern maintained (commands produce messages)

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4-epic10.md#Story 10.1]
- [Source: _bmad-output/project-context.md#Bubbletea Framework Rules]
- [Source: internal/tui/dashboard.go:508-525] - paneDirWatcherInitMsg handler
- [Source: internal/tui/dashboard.go:501] - pane.conversation assignment
- [Source: internal/scanner/projects.go:303-305] - Sort logic
- [Lesson: Story 9.2] - Subscription model and context patterns

### Previous Story Learnings (Epic 9)

From Story 9.2 (Dashboard Subscription Model):
- Context and cancellation patterns are established
- Subscription goroutines with polling are working
- Channel-based event delivery is functional
- `closeAllWatchers()` shutdown sequence is solid

Key patterns to maintain:
- TEA commands produce messages, not direct state changes
- Use `tea.Batch()` for multiple commands
- Non-blocking operations in commands

### Testing Strategy

**Unit Tests:**
1. Test LastModified preservation through paneContentLoadedMsg flow
2. Test stable sort produces deterministic results with equal timestamps
3. Test rescan skips reload when same conversation detected
4. Test rescan triggers reload when newer conversation detected

**Manual Tests:**
1. Race condition test:
   - Open cclv dashboard on a project
   - Immediately start a new claude session in same project
   - Verify new conversation appears in dashboard

2. No-flicker test:
   - Open dashboard, note current conversation
   - Wait for re-scan to complete (after watcher init)
   - Verify no visual flicker/reload

### Complexity

Medium - Changes are localized to 2 files but touch core initialization flow.

## Validation Record

**Validated:** 2026-01-29 by Scrum Master (validate-create-story workflow)

**Issues Found & Fixed:**
1. ✅ `rescanLatestCmd` used method receiver but accessed `m.panes` in closure (stale data risk) → Changed to standalone function with `projectPath` parameter
2. ✅ Referenced non-existent `limit` parameter on `ScanConversationsLazy()` → Fixed to use `[0]` slice access
3. ✅ Used `errors.New()` unnecessarily → Simplified to `err: nil` for empty project case
4. ✅ Dev notes showed incomplete `paneDirWatcherInitMsg` handler → Added complete handler code

**Verified Accurate:**
- Line number references (64-70, 501, 517-526, 303-305) match current codebase
- `types.Conversation` already has `LastModified` field (line 11 of conversation.go)
- `paneContentLoadedMsg` correctly identified as missing `lastModified` field
- Sort logic location and current implementation verified

**Ready for Development:** Yes

---

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- No debug issues encountered during implementation

### Completion Notes List

1. **AC-1 (Re-scan After Watcher Ready):** Implemented `rescanLatestCmd()` function and integrated it into `paneDirWatcherInitMsg` handler. When directory watcher initialization completes, a fresh scan is triggered to catch conversations created during the initialization window.

2. **AC-2 (Stable Sort):** Modified `ScanConversationsLazy()` sort logic in `internal/scanner/projects.go` to use filename as a tiebreaker when modification times are equal. Files with "later" names sort first, ensuring deterministic ordering.

3. **AC-3 (LastModified Preservation):** Added `lastModified time.Time` field to `paneContentLoadedMsg`, populated it from file stat in `loadPaneContentCmd()`, and preserved it in `pane.conversation` in the handler.

4. **AC-4 (No Visual Flicker):** The `paneRescanResultMsg` handler compares the current conversation path and timestamp with the scanned result. If they match (same file or scanned file is not newer), no action is taken, preventing unnecessary UI updates.

5. **Tests Added:**
   - `TestScanConversationsLazyStableSort` - Verifies deterministic ordering with equal timestamps
   - `TestScanConversationsLazySortByModTime` - Verifies correct mod time sorting
   - `TestPaneContentLoadedMsgHasLastModifiedField` - Verifies struct has the field
   - `TestPaneRescanResultMsgType` - Verifies message type structure
   - `TestPaneRescanResultMsgHandlerSameConversation` - Verifies no-flicker behavior
   - `TestPaneRescanResultMsgHandlerNewerConversation` - Verifies reload trigger
   - `TestPaneRescanResultMsgHandlerOlderConversation` - Verifies older file is ignored
   - `TestPaneRescanResultMsgHandlerError` - Verifies error handling
   - `TestPaneRescanResultMsgHandlerInvalidIndex` - Verifies bounds checking
   - `TestRescanLatestCmdReturnsFunction` - Verifies command creation
   - `TestRescanLatestCmdReturnsCorrectMessage` - Verifies message content
   - `TestRescanLatestCmdEmptyProject` - Verifies empty project handling
   - `TestPaneDirWatcherInitMsgTriggersRescan` - Verifies handler triggers rescan

6. **Existing Test Updated:**
   - `TestPaneDirWatcherInitMsgHandler` - Updated expectation: now returns rescan command (Story 10.1 change)

### File List

- `internal/tui/dashboard.go` - Added `lastModified` field to `paneContentLoadedMsg`, added `paneRescanResultMsg` type and `rescanLatestCmd()` function, added `paneRescanResultMsg` handler, modified `paneDirWatcherInitMsg` handler to trigger rescan, updated `loadPaneContentCmd()` to use conv.LastModified (code review fix: removed redundant os.Stat)
- `internal/scanner/projects.go` - Modified `ScanConversationsLazy()` sort to use filename tiebreaker for equal timestamps
- `internal/tui/dashboard_test.go` - Added 14 new Story 10.1 tests (including TestPaneRescanResultMsgHandlerEmptyResult from code review), updated 1 existing test
- `internal/scanner/projects_test.go` - Added 2 new tests for stable sort

## Change Log

- 2026-01-29: Story 10.1 implementation complete - Fixed dashboard initialization race condition with re-scan after watcher ready, stable sort with filename tiebreaker, and LastModified preservation
- 2026-01-29: Code review fixes applied:
  - M1: Removed redundant os.Stat call in loadPaneContentCmd - now uses conv.LastModified from ScanConversationsLazy
  - M2: Added explicit empty FilePath check in paneRescanResultMsg handler with clarifying comment
  - L1: Improved comment for lastModified field explaining its purpose
  - Added TestPaneRescanResultMsgHandlerEmptyResult test for empty rescan case

