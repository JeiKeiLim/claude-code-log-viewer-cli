---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - '_bmad-output/planning-artifacts/backlog.md#BL-002'
  - 'Investigation analysis from BL-002'
phase: 4
status: ready
createdAt: '2026-01-29'
---

# Epic 10: Dashboard Reliability - Latest Conversation Display Fix

## Overview

Phase 4 continuation focusing on fixing the intermittent issue where dashboard (especially single-project mode) fails to display the latest conversation log.

## Problem Statement

When running cclv in dashboard mode with a single project, the dashboard sometimes fails to display the latest conversation. This is an intermittent issue caused by race conditions and timing gaps in the initialization sequence.

## Root Cause Analysis

### Primary Issue: Initialization Race Condition

**Location:** `internal/tui/dashboard.go`

```
User enters single project
     ↓
loadPaneContentCmd (async) ──┐
     ↓                       │  ← NEW CONVERSATION CREATED HERE = MISSED
paneContentLoadedMsg         │
     ↓                       │
initDirectoryWatcher() ──────┘
     ↓
Watcher ready (200ms+ total delay)
```

If a new conversation is created between content load and watcher startup, it's **invisible** with no recovery mechanism.

### Contributing Issues

| # | Issue | Location | Impact |
|---|-------|----------|--------|
| 1 | Initialization race condition | `dashboard.go:508-525` | High - conversations missed during startup |
| 2 | Timestamp loss in Conversation struct | `dashboard.go:501, 579` | Medium - stale comparison data |
| 3 | Directory watcher only watches Create | `dashboard.go:736` | Medium - misses file modifications |
| 4 | Unstable sort for equal timestamps | `scanner/projects.go:303-305` | Low - undefined order on same timestamp |
| 5 | No manual refresh mechanism | N/A | Low - no recovery option |

## Requirements Inventory

### Functional Requirements

**FR-1000: Dashboard Latest Conversation Reliability**
- FR-1001: Dashboard must show the latest conversation regardless of initialization timing
- FR-1002: User must be able to manually refresh pane content
- FR-1003: Conversation sorting must be deterministic for equal timestamps

### Non-Functional Requirements

**NFR-011: Initialization Reliability**
- Dashboard pane shows latest conversation within 500ms of entering dashboard mode
- No conversations missed due to watcher startup timing

**NFR-012: User Recovery**
- Manual refresh responds within 200ms
- Visual feedback on refresh action

### FR Coverage Map

| FR | Story | Description |
|----|-------|-------------|
| FR-1001 | Story 10.1, 10.3 | Fix initialization sequence to eliminate race condition |
| FR-1002 | Story 10.2 | Add manual refresh keybinding |
| FR-1003 | Story 10.1 | Stable sort with filename tiebreaker |
| FR-1004 | Story 10.3 | Use creation time for accurate "latest" detection |

---

## Epic Summary

Targeted fix for intermittent "latest conversation not showing" bug in dashboard mode, primarily affecting single-project dashboards.

**FRs covered:** FR-1001, FR-1002, FR-1003
**Standalone:** Yes - isolated to dashboard initialization and refresh
**Priority:** High - affects data accuracy in dashboard view

---

## Story 10.1: Fix Dashboard Initialization Race Condition

As a **cclv user viewing a project dashboard**,
I want **the dashboard to always show the latest conversation**,
So that **I don't miss recent activity due to timing issues**.

### Acceptance Criteria

1. **AC-1: Re-scan After Watcher Ready**
   - Given dashboard enters a project
   - When directory watcher initialization completes
   - Then a fresh scan for latest conversation is performed
   - And if a newer conversation exists, the pane updates to show it

2. **AC-2: Stable Sort for Equal Timestamps**
   - Given multiple conversation files with identical modification times
   - When sorting by "latest"
   - Then files are sorted by filename (descending) as tiebreaker
   - And the result is deterministic across runs

3. **AC-3: Preserve LastModified in Conversation Struct**
   - Given a conversation is loaded into a pane
   - When the Conversation struct is stored
   - Then both `FilePath` and `LastModified` are preserved (not just FilePath)

4. **AC-4: No Visual Flicker**
   - Given the re-scan finds the same conversation
   - When comparing current vs scanned
   - Then no reload occurs (avoid unnecessary UI update)

### Technical Notes

**Fix 1: Re-scan after watcher ready**

```go
// In paneDirWatcherInitMsg handler (around line 508-525)
case paneDirWatcherInitMsg:
    // ... existing watcher init code ...

    // NEW: Re-scan for latest conversation after watcher is ready
    cmds = append(cmds, m.rescanLatestCmd(msg.paneIndex))
```

**Fix 2: Stable sort with filename tiebreaker**

```go
// In scanner/projects.go ScanConversationsLazy()
sort.Slice(conversations, func(i, j int) bool {
    if conversations[i].LastModified.Equal(conversations[j].LastModified) {
        // Tiebreaker: sort by filename descending (newer UUIDs tend to be "later")
        return conversations[i].FilePath > conversations[j].FilePath
    }
    return conversations[i].LastModified.After(conversations[j].LastModified)
})
```

**Fix 3: Preserve LastModified in pane.conversation**

```go
// Line 501 - change from:
pane.conversation = types.Conversation{FilePath: msg.filePath}
// To:
pane.conversation = types.Conversation{
    FilePath:     msg.filePath,
    LastModified: msg.lastModified, // Add to paneContentLoadedMsg
}
```

**Files to modify:**
- `internal/tui/dashboard.go` - Re-scan logic, conversation storage
- `internal/scanner/projects.go` - Stable sort

**Complexity:** Medium

---

## Story 10.2: Add Manual Refresh Keybinding

As a **cclv user viewing a dashboard pane**,
I want **to manually refresh the pane content**,
So that **I can recover if the latest conversation wasn't detected automatically**.

### Acceptance Criteria

1. **AC-1: Refresh Keybinding**
   - Given user is viewing dashboard
   - When user presses `r`
   - Then the focused pane reloads its content (re-scans for latest conversation)

2. **AC-2: Visual Feedback**
   - Given user presses `r`
   - When refresh starts
   - Then pane shows loading indicator
   - And toast displays "Refreshing..." briefly

3. **AC-3: Help Text Update**
   - Given user views help (?)
   - When help is displayed
   - Then `r - Refresh pane` is listed

4. **AC-4: Shift+R Refreshes All Panes**
   - Given user is viewing multi-project dashboard
   - When user presses `R` (shift+r)
   - Then all panes reload their content

### Technical Notes

**Key handler addition:**

```go
case "r":
    // Refresh focused pane
    return m, m.refreshPaneCmd(m.focusIndex)

case "R":
    // Refresh all panes
    cmds := make([]tea.Cmd, len(m.panes))
    for i := range m.panes {
        cmds[i] = m.refreshPaneCmd(i)
    }
    return m, tea.Batch(cmds...)
```

**Files to modify:**
- `internal/tui/dashboard.go` - Key handler, refresh command
- `internal/tui/dashboard_help.go` - Help text (if exists)

**Complexity:** Low

---

## Story 10.3: Use File Creation Time for Latest Conversation Detection

As a **cclv user viewing a dashboard**,
I want **the dashboard to identify the current conversation by creation time**,
So that **I always see my active conversation, not old files that Claude Code updated**.

### Background

Investigation (2026-01-29) revealed that Claude Code modifies multiple OLD conversation files when starting a new session (likely syncing metadata). This causes the "latest modified" file to be an old conversation, not the current one.

**Evidence:**
- 11 files all modified at 13:28 within seconds of each other
- The new "hi" conversation (`c002796b-...` with 2773 bytes) lost the mtime race
- Using file creation time (birthtime) correctly identified the new conversation

### Acceptance Criteria

1. **AC-1: Sort by Creation Time**
   - Given multiple conversation files exist
   - When scanning for latest conversation
   - Then files are sorted by creation time (birthtime) descending, not modification time
   - And the most recently CREATED file is returned as "latest"

2. **AC-2: Fallback for Systems Without Birthtime**
   - Given a system that doesn't support birthtime (some Linux filesystems)
   - When birthtime is unavailable (returns zero or error)
   - Then fall back to modification time
   - And log a warning on first occurrence

3. **AC-3: Consistent Behavior Across Platforms**
   - Given cclv runs on macOS, Linux, or Windows
   - When getting file creation time
   - Then use platform-appropriate syscall:
     - macOS: `syscall.Stat_t.Birthtimespec`
     - Linux: `statx` syscall with `STATX_BTIME` (kernel 4.11+)
     - Windows: `syscall.Win32FileAttributeData.CreationTime`

4. **AC-4: No Performance Regression**
   - Given a project with 1000+ conversation files
   - When scanning for latest conversation
   - Then scan completes within 500ms
   - And birthtime is obtained from same stat call (no extra syscall)

### Technical Notes

**Platform-specific implementation:**

```go
// internal/scanner/birthtime_darwin.go
//go:build darwin

func getBirthtime(info os.FileInfo) time.Time {
    stat := info.Sys().(*syscall.Stat_t)
    return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}
```

```go
// internal/scanner/birthtime_linux.go
//go:build linux

func getBirthtime(info os.FileInfo) time.Time {
    // Use statx syscall for birthtime, fallback to mtime
    // Note: Requires kernel 4.11+ and filesystem support
}
```

**Files to modify:**
- `internal/scanner/projects.go` - Use birthtime in sorting
- `internal/scanner/birthtime_darwin.go` - macOS implementation (new)
- `internal/scanner/birthtime_linux.go` - Linux implementation (new)
- `internal/scanner/birthtime_windows.go` - Windows implementation (new)

**Complexity:** Medium

---

## Implementation Order

| Order | Story | Rationale |
|-------|-------|-----------|
| 1 | Story 10.1 | Core fix - eliminates race condition |
| 2 | Story 10.2 | User safety net - manual recovery option |
| 3 | Story 10.3 | Root cause fix - correct "latest" detection |

## Validation Strategy

### Manual Testing

1. **Race condition test:**
   - Open cclv dashboard on a project
   - Immediately start a new claude session in same project
   - Verify new conversation appears in dashboard

2. **Refresh test:**
   - Open dashboard, note current conversation
   - Create new conversation externally
   - Press `r` - verify new conversation appears

3. **Single vs multi-project:**
   - Test both modes with same scenarios

### Automated Testing

- Unit test for stable sort with equal timestamps
- Unit test for LastModified preservation in Conversation struct

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Re-scan causes flicker | Compare before updating, skip if same |
| Performance impact of re-scan | Single re-scan is fast (<50ms) |
| Key conflict with `r` | Check existing keybindings first |

## Dependencies

- None - self-contained fix

## Estimated Effort

| Story | Complexity | Estimate |
|-------|------------|----------|
| 10.1 | Medium | 2-3 hours |
| 10.2 | Low | 1 hour |
| **Total** | | ~4 hours |
