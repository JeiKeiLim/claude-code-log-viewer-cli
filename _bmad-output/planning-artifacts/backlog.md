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

### BL-002: Dashboard Single-Project Fails to Show Latest Log [PLANNED - Epic 10]

**Date Added:** 2026-01-29
**Priority:** High
**Category:** Bug (Investigation Needed)
**Status:** Planned in Epic 10 (2026-01-29)

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

---

## Priority Legend

| Priority | Description |
|----------|-------------|
| High | Critical for user experience, plan soon |
| Medium | Nice to have, plan when capacity allows |
| Low | Future consideration |

---

_Last Updated: 2026-01-29_
