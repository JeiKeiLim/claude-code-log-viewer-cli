---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - '_bmad-output/planning-artifacts/backlog.md#BL-001'
  - '_bmad-output/planning-artifacts/backlog.md#BL-003'
phase: 4
status: ready
createdAt: '2026-01-29'
---

# Epic 11: Live Updates & Auto-Refresh

## Overview

Phase 4 feature epic focused on keeping CCLV current without manual intervention. Addresses two related UX improvements: auto-detecting when Claude auth is refreshed, and automatically following the latest conversation in watch mode.

## Problem Statement

### Problem 1: Auth Refresh Requires App Restart (BL-001)

When Claude auth expires (user idle overnight or away at work), the TUI displays "run claude to refresh" but doesn't monitor whether the user has actually refreshed credentials. The only way to restore usage limit display is to restart CCLV.

### Problem 2: Watch Mode Stuck on Single Conversation (BL-003)

In watch mode (`-w`), CCLV streams updates to a single conversation file. If a new conversation starts in the same project (e.g., user opens new claude session), the viewer stays on the old conversation. Users need a way to automatically follow the newest conversation.

## Thematic Connection

Both issues share a common theme: **"Keeping CCLV current without manual intervention"**

| Issue | Current Behavior | Desired Behavior |
|-------|------------------|------------------|
| Auth expiry | Manual restart required | Auto-detect refresh, restore display |
| Watch mode | Stays on one conversation | Option to follow latest |

## Requirements Inventory

### Functional Requirements

**FR-1100: Auth Refresh Auto-Detection**
- FR-1101: When auth is expired, periodically check if user has refreshed
- FR-1102: When auth becomes valid again, restore usage bar display
- FR-1103: Provide visual feedback when auth is restored

**FR-1200: Watch Mode Follow-Latest**
- FR-1201: New flag `--follow-latest` or `-L` to enable latest-conversation following
- FR-1202: Detect new conversation files created in the project
- FR-1203: Automatically switch viewer to newest conversation
- FR-1204: Show notification when switching conversations

### Non-Functional Requirements

**NFR-013: Auth Polling Efficiency**
- Poll interval: 5-10 minutes when in expired state (not aggressive)
- No polling when auth is valid (avoid unnecessary API calls)
- Graceful handling of network errors during poll

**NFR-014: Conversation Switch UX**
- Switch to new conversation within 2 seconds of creation
- Toast notification shows timestamp of new conversation
- No content loss during switch (old content can be scrolled back via history if needed)

### FR Coverage Map

| FR | Story | Description |
|----|-------|-------------|
| FR-1101, FR-1102, FR-1103 | Story 11.1 | Auth refresh auto-detection |
| FR-1201, FR-1202, FR-1203, FR-1204 | Story 11.2 | Follow-latest conversation flag |

---

## Epic Summary

UX enhancement epic to make CCLV "self-healing" - automatically recovering from auth expiry and optionally following the latest conversation in a project.

**FRs covered:** FR-1101-1103, FR-1201-1204
**Standalone:** Yes - independent of Epic 10
**Priority:** Medium - quality of life improvements
**Depends on:** Epic 10 should be completed first (shares some dashboard/watcher patterns)

---

## Story 11.1: Auto-Detect Claude Auth Refresh

As a **cclv user who has been idle (e.g., overnight)**,
I want **CCLV to automatically detect when I've refreshed my Claude auth**,
So that **I can see usage limits again without restarting the app**.

### Acceptance Criteria

1. **AC-1: Periodic Auth Check When Expired**
   - Given usage bar shows "run claude to refresh" (auth expired)
   - When 5 minutes have passed
   - Then CCLV attempts to re-fetch usage data
   - And if successful, usage bar is restored

2. **AC-2: No Polling When Valid**
   - Given auth is valid and usage bar is showing normally
   - When time passes
   - Then no additional auth checks are performed (use existing refresh interval)

3. **AC-3: Visual Feedback on Recovery**
   - Given auth was expired and user refreshed externally
   - When CCLV detects valid auth
   - Then toast shows "Usage limits restored"
   - And usage bar displays current usage

4. **AC-4: Graceful Network Errors**
   - Given auth check fails due to network error
   - When retry occurs
   - Then error is logged but not shown to user
   - And polling continues at normal interval

### Technical Notes

**Current auth error handling (`internal/usage/client.go`):**
- When auth fails, `FetchUsage()` returns error
- Usage bar shows "run claude to refresh"
- No retry mechanism

**New behavior:**
```go
// In usagebar model or wrapper
type UsageBarModel struct {
    // ... existing fields
    authExpired     bool
    lastAuthCheck   time.Time
    authRetryTicker *time.Ticker  // Only active when expired
}

// When auth expires, start retry ticker
func (m *UsageBarModel) startAuthRetry() tea.Cmd {
    return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
        return authRetryTickMsg{}
    })
}

// On retry tick, attempt to fetch usage
case authRetryTickMsg:
    if m.authExpired {
        return m, m.fetchUsageCmd()
    }
    return m, nil
```

**Files to modify:**
- `internal/tui/usagebar.go` - Retry logic, state tracking
- `internal/usage/client.go` - Possibly expose auth validity check

**Complexity:** Medium

---

## Story 11.2: Watch Mode Follow-Latest Conversation Flag

As a **cclv user watching a project**,
I want **an option to automatically switch to the newest conversation**,
So that **I always see the latest activity even when new claude sessions start**.

### Acceptance Criteria

1. **AC-1: New CLI Flag**
   - Given user runs `cclv -w -L` or `cclv -w --follow-latest`
   - When viewing a conversation
   - Then follow-latest mode is enabled

2. **AC-2: Detect New Conversation**
   - Given follow-latest is enabled
   - When a new .jsonl file is created in the project
   - Then CCLV detects it within 2 seconds

3. **AC-3: Automatic Switch**
   - Given new conversation is detected
   - When new conversation is confirmed newer than current
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
   - Then error: "--follow-latest requires --watch mode"

6. **AC-6: Works With Interactive Browse**
   - Given user is in interactive browse mode
   - When user enters watch mode on a conversation with `w` key
   - Then they can toggle follow-latest with `L` key

### Technical Notes

**Key distinction:**
- `-w` (watch): Stream updates to current conversation file
- `-L` / `--follow-latest`: Switch to newest conversation when created
- Combined: Always show latest conversation with live updates

**Implementation approach:**

1. **Add CLI flag:**
```go
// cmd/cclv/main.go
followLatest := flag.Bool("L", false, "Follow latest conversation (requires -w)")
flag.BoolVar(followLatest, "follow-latest", false, "Follow latest conversation (requires -w)")
```

2. **Project-level file watcher:**
```go
// Watch project directory, not just single file
// Filter for .jsonl file creation events
func watchProjectForNewConversations(projectPath string) <-chan string {
    // Return channel that emits new conversation paths
}
```

3. **Viewer integration:**
```go
type ViewerModel struct {
    // ... existing fields
    followLatest    bool
    projectPath     string        // Need to know project for watching
    projectWatcher  *fsnotify.Watcher
}
```

**Files to modify:**
- `cmd/cclv/main.go` - New flag
- `internal/tui/viewer.go` - Follow-latest logic, project watcher
- `internal/tui/app.go` - Pass flag through to viewer

**Complexity:** Medium-High

---

## Implementation Order

| Order | Story | Rationale |
|-------|-------|-----------|
| 1 | Story 11.1 | Simpler, self-contained, immediate value |
| 2 | Story 11.2 | More complex, requires viewer + watcher changes |

## Validation Strategy

### Story 11.1 Manual Testing

1. Run cclv, let auth expire (or manually invalidate token)
2. Verify "run claude to refresh" message
3. Run `claude` in another terminal to refresh auth
4. Wait up to 5 minutes
5. Verify usage bar restores automatically

### Story 11.2 Manual Testing

1. Start `cclv -w -L /path/to/conversation.jsonl`
2. In another terminal, start new claude session in same project
3. Verify cclv switches to new conversation
4. Verify toast notification appears
5. Verify live streaming works on new conversation

### Automated Testing

- Unit test for auth retry ticker lifecycle
- Unit test for follow-latest flag validation
- Integration test for conversation switch detection

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Auth polling too aggressive | 5-minute interval is conservative |
| Race condition on conversation switch | Compare timestamps before switching |
| File watcher resource usage | Reuse existing watcher patterns from Epic 9/10 |

## Dependencies

- **Epic 10** should be completed first (shares dashboard/watcher patterns that may inform implementation)
- Story 11.2 depends on watcher infrastructure already in codebase

## Estimated Effort

| Story | Complexity | Estimate |
|-------|------------|----------|
| 11.1 | Medium | 2-3 hours |
| 11.2 | Medium-High | 4-5 hours |
| **Total** | | ~7 hours |
