# CCLV Feature Backlog

_Future improvement ideas to be planned and prioritized._

---

## Backlog Items

### BL-001: Auto-detect Claude Auth Refresh [PLANNED - Epic 11]

**Date Added:** 2026-01-28
**Priority:** Medium
**Category:** UX Improvement
**Status:** Planned in Epic 11 (2026-01-29)

**Problem:**
When Claude auth expires (e.g., user idle overnight or away at work), the TUI displays "run claude to refresh" but doesn't monitor whether the user has actually refreshed their credentials. Currently, the only way to restore Claude usage limits display is to restart CCLV entirely.

**Expected Behavior:**
CCLV should periodically check whether auth has been refreshed and automatically restore the usage limit display without requiring a full app restart.

**Considerations:**
- Auth expiration typically happens during long idle periods (sleep, work, etc.)
- Polling frequency TBD - needs to balance responsiveness vs resource usage
- Could poll every 5-10 minutes when in expired state
- Should provide visual feedback when auth is restored

**Related Components:**
- `internal/tui/usagebar.go` - Usage bar component
- `internal/usage/` - Usage API client and auth handling

### BL-002: Dashboard Single-Project Fails to Show Latest Log [DUPLICATE - See BL-005]

**Date Added:** 2026-01-29
**Priority:** High
**Category:** Bug (Investigation Needed)
**Status:** Duplicate of BL-005 (root cause identified 2026-01-29)

**Problem:**
When running dashboard mode with a single project selected, the dashboard sometimes fails to display the latest conversation log. The issue is intermittent and reproduction steps are not yet identified.

**Symptoms:**
- Latest log entry not appearing in dashboard pane
- Occurs with single-project dashboard
- Intermittent / not consistently reproducible

**Investigation Needed:**
- Determine exact reproduction steps
- Check if this is a race condition in conversation detection (`internal/tui/dashboard.go`)
- Check if file watcher events are being missed or delayed for single-project mode
- Check if sorting/ordering logic has an edge case with single project
- Compare behavior between single-project and multi-project dashboard modes

**Related Components:**
- `internal/tui/dashboard.go` - Dashboard view
- `internal/tui/dashboard_*.go` - Dashboard sub-components
- `internal/scanner/` - Log file scanning and detection

### BL-003: Watch Mode Follow-Latest-Conversation Flag [PLANNED - Epic 11]

**Date Added:** 2026-01-29
**Priority:** Medium
**Category:** Feature Enhancement
**Status:** Planned in Epic 11 (2026-01-29)

**Problem:**
In watch mode (`-w`), CCLV streams updates to a single conversation file. However, if a new conversation starts (e.g., user opens a new claude session in the same project), the viewer stays on the old conversation. Users need a way to automatically switch to the newest conversation.

**Expected Behavior:**
A new flag (e.g., `--follow-latest` or `-L`) that, when combined with watch mode, will:
1. Detect when a new conversation file is created in the project
2. Automatically switch to viewing/streaming that new conversation

**Key Distinction:**
- `-w` (live/watch): Stream updates to the **current** conversation file
- `--follow-latest`: Switch to the **newest** conversation when created

These are separate flags because:
- User may have multiple claude instances running in the same project
- User may want to watch a specific conversation without auto-switching
- Combining both flags gives "always show latest conversation with live updates"

**Considerations:**
- Need project-level file watching (not just single-file watching)
- Should show visual indicator when switching conversations
- Toast notification: "Switched to new conversation: <timestamp>"
- Edge case: rapid conversation creation - debounce or take latest?

**Related Components:**
- `internal/tui/viewer.go` - Viewer component
- `internal/tui/watcher.go` - File watcher integration
- `cmd/cclv/main.go` - Flag parsing

### BL-005: Use File Creation Time for Latest Conversation Detection [PLANNED - Epic 10]

**Date Added:** 2026-01-29
**Priority:** High
**Category:** Bug Fix
**Status:** Planned as Story 10.3 in Epic 10 (2026-01-29)

**Problem:**
Dashboard refresh ('r' key) shows "random" old conversations instead of the current one. Investigation revealed that Claude Code modifies multiple old conversation files when starting a new session (likely syncing metadata). Since cclv uses file modification time (mtime) to find the "latest" conversation, it incorrectly identifies recently-touched old files as the current conversation.

**Root Cause:**
- `ScanConversationsLazy` sorts by `info.ModTime()` (modification time)
- Claude Code updates mtime on OLD conversation files during session startup
- New conversation competes with many old files that have recent mtime
- Result: "latest" file changes unpredictably until user interacts more with current conversation

**Evidence (from debug session 2026-01-29):**
- 11 files all modified at 13:28 within seconds of each other
- The truly new conversation (`c002796b-...` with 2773 bytes) lost the mtime race
- Using file creation time (birthtime) correctly identified the new conversation

**Proposed Solution:**
Use file **creation time (birthtime)** instead of modification time for sorting:
- macOS: `syscall.Stat_t.Birthtimespec`
- Linux: May need fallback to mtime (ext4 doesn't track birthtime by default)
- Windows: `syscall.Win32FileAttributeData.CreationTime`

**Implementation Notes:**
```go
// In scanner/projects.go ScanConversationsLazy()
// Replace: info.ModTime()
// With: getBirthtime(filePath) with fallback to ModTime
```

**Acceptance Criteria:**
1. Dashboard refresh consistently shows the most recently CREATED conversation
2. Falls back to mtime on systems without birthtime support
3. No performance regression (birthtime is available from same stat call)

**Related Files:**
- `internal/scanner/projects.go` - `ScanConversationsLazy()`
- `internal/tui/dashboard.go` - `findLatestConversation()`

**Related Issues:**
- BL-002 (Dashboard Single-Project Fails to Show Latest Log) - same root cause

---

### BL-004: Direct Dashboard Mode CLI Flag (-d)

**Date Added:** 2026-01-29
**Priority:** Low
**Category:** Feature Enhancement
**Status:** Backlog

**Problem:**
Currently, to access dashboard mode, users must:
1. Run `cclv` (interactive mode)
2. Press `space` to select projects
3. Press `Enter` to open dashboard

There's no way to directly launch dashboard mode from the command line.

**Expected Behavior:**
A `-d` or `--dashboard` flag that accepts one or more project paths and opens dashboard mode directly:
```bash
cclv -d                           # Dashboard with all projects
cclv -d ~/project1 ~/project2     # Dashboard with specific projects
cclv -d .                         # Dashboard with current project only
```

**Use Cases:**
- Quick dashboard launch from terminal
- Scriptable dashboard invocation
- Shortcut for monitoring specific projects

**Considerations:**
- Should accept glob patterns? (e.g., `cclv -d ~/code/*`)
- Behavior with single project vs multiple projects
- Interaction with other flags (`--watch`, `--plain`)

**Related Components:**
- `cmd/cclv/main.go` - Flag parsing
- `internal/tui/app.go` - App initialization
- `internal/tui/dashboard.go` - Dashboard model

---

## Priority Legend

| Priority | Description |
|----------|-------------|
| High | Critical for user experience, plan soon |
| Medium | Nice to have, plan when capacity allows |
| Low | Future consideration |

---

_Last Updated: 2026-01-29_
_BL-004 added: Direct dashboard mode flag suggested during Story 10.2 code review_
