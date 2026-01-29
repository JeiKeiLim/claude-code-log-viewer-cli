---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - '_bmad-output/planning-artifacts/research/technical-claude-code-usage-limits-research-2026-01-20.md'
phase: 4
status: ready
createdAt: '2026-01-20'
updatedAt: '2026-01-29'
---

# claude-code-log-viewer-cli Phase 4 - Epic Breakdown

## Overview

This document provides the epic and story breakdown for cclv Phase 4 (Usage Monitoring), adding persistent Claude Code subscription usage limit display across all views.

## Requirements Inventory

### Functional Requirements

**FR-700: Usage Limit Monitoring**
- FR-701: OAuth Credential Access - Read Claude Code OAuth token from platform keychain/file
- FR-702: Usage API Client - Fetch usage limits from OAuth endpoint with caching
- FR-703: Top-Level Usage Bar Component - Persistent usage bar rendered above all views
- FR-704: App Model Wrapper - Refactor to wrap all views with usage bar
- FR-705: Usage Bar Refresh - Periodic refresh and manual refresh on keypress
- FR-706: Plain Text Usage Output - `cclv --usage` for scripting (no TUI)
- FR-707: Graceful Degradation - Handle missing credentials and API errors

### Non-Functional Requirements

**NFR-004: Usage Feature Performance**
- API call latency: Handle up to 2s gracefully
- Cache duration: 60 seconds minimum
- UI impact: No perceptible delay in view rendering

**NFR-005: Cross-Platform Support**
- macOS: Keychain access via `security` command
- Linux: File-based credentials (`~/.claude/.credentials.json`)
- Windows: File-based credentials (best effort)

**NFR-006: Code Quality**
- Test Coverage: Maintain 90%
- New package: `internal/usage/`
- Follow established patterns (soft-fail, thread safety)

### Architectural Decisions

**Decision 7.1: Top-Level Wrapper Pattern**
- Root model wraps all views with persistent usage bar
- Usage state managed at app level, passed down to bar component
- Views render in remaining space after usage bar

**Decision 7.2: Credential Access Strategy**
- macOS: Shell out to `security find-generic-password`
- Linux/Windows: Read `~/.claude/.credentials.json`
- Fallback: Check `CLAUDE_CODE_OAUTH_TOKEN` env var

**Decision 7.3: Caching Strategy**
- Cache API response for 60 seconds
- Background refresh when cache expires
- Manual refresh with `R` key

**Decision 7.4: Error Handling**
- Missing credentials: Show "Not logged in" in usage bar
- API error: Show last known values with "stale" indicator
- Network timeout: Graceful degradation, don't block UI

### FR Coverage Map

| FR | Epic | Description |
|----|------|-------------|
| FR-701 | Epic 7 | OAuth credential access |
| FR-702 | Epic 7 | Usage API client |
| FR-703 | Epic 7 | Usage bar component |
| FR-704 | Epic 7 | App model wrapper |
| FR-705 | Epic 7 | Usage bar refresh |
| FR-706 | Epic 7 | Plain text output |
| FR-707 | Epic 7 | Graceful degradation |

## Epic List

### Epic 7: Usage Limit Monitoring [DONE]

Users can see their Claude Code subscription usage limits (5-hour and weekly) persistently across all views, eliminating the need to visit the web console.

**FRs covered:** FR-701, FR-702, FR-703, FR-704, FR-705, FR-706, FR-707
**Standalone:** No - requires refactoring root model to wrap views
**Research:** `_bmad-output/planning-artifacts/research/technical-claude-code-usage-limits-research-2026-01-20.md`
**Status:** Done

### Epic 8: Plain Mode & Output Enhancements [DONE]

Bug fixes and improvements for plain text output mode.

**Document:** `epics-phase4-epic8.md`
**Status:** Done

### Epic 9: Dashboard File Descriptor Leak Fix [DONE]

Fix file descriptor and goroutine leaks in dashboard watch mode.

**Document:** `epics-phase4-epic9.md`
**Status:** Done

### Epic 10: Dashboard Reliability [BACKLOG]

Fix intermittent "latest conversation not showing" bug in dashboard mode.

**Document:** `epics-phase4-epic10.md`
**Status:** Backlog - Next up

### Epic 11: Live Updates & Auto-Refresh [BACKLOG]

Auto-detect auth refresh and follow-latest conversation flag for watch mode.

**Document:** `epics-phase4-epic11.md`
**Status:** Backlog - After Epic 10

---

## Epic 7: Usage Limit Monitoring (Details)

Users can see their Claude Code subscription usage limits (5-hour and weekly) persistently across all views, eliminating the need to visit the web console.

### Story 7.1: OAuth Credential Access

As a **cclv user with a Claude Code subscription**,
I want **cclv to read my existing Claude Code credentials**,
So that **I don't need to log in separately**.

**Acceptance Criteria:**

**Given** I am on macOS and logged into Claude Code
**When** cclv needs credentials
**Then** it retrieves the OAuth token from Keychain (`Claude Code-credentials`)

**Given** I am on Linux and logged into Claude Code
**When** cclv needs credentials
**Then** it reads the OAuth token from `~/.claude/.credentials.json`

**Given** `CLAUDE_CODE_OAUTH_TOKEN` env var is set
**When** cclv needs credentials
**Then** it uses the env var value (takes precedence on Linux)

**Given** no credentials are found
**When** cclv tries to fetch usage
**Then** it returns a clear error without crashing

**Technical Notes:**
- New package: `internal/usage/credentials.go`
- macOS: Use `os/exec` to call `security find-generic-password -s "Claude Code-credentials" -w`
- Linux: Parse JSON from `~/.claude/.credentials.json`
- Extract `claudeAiOauth.accessToken` from JSON response

---

### Story 7.2: Usage API Client

As a **developer building usage features**,
I want **a client to fetch usage limits from the OAuth endpoint**,
So that **I can display current utilization and reset times**.

**Acceptance Criteria:**

**Given** valid OAuth credentials
**When** FetchUsage() is called
**Then** it calls `GET https://api.anthropic.com/api/oauth/usage`
**And** returns parsed UsageLimits struct

**Given** the API response is received
**When** parsed
**Then** it extracts `five_hour.utilization`, `five_hour.resets_at`
**And** extracts `seven_day.utilization`, `seven_day.resets_at`

**Given** a successful API call
**When** FetchUsage() is called again within 60 seconds
**Then** cached result is returned (no API call)

**Given** API returns error or times out
**When** FetchUsage() is called
**Then** error is returned without crashing
**And** last known good values are preserved

**Technical Notes:**
- New file: `internal/usage/client.go`
- Headers: `Authorization: Bearer {token}`, `anthropic-beta: oauth-2025-04-20`
- User-Agent: `claude-code/cclv-{version}`
- Timeout: 5 seconds
- Cache: `sync.RWMutex` protected map with TTL

**Types:**
```go
type UsageLimits struct {
    FiveHour *UsageWindow `json:"five_hour"`
    SevenDay *UsageWindow `json:"seven_day"`
}

type UsageWindow struct {
    Utilization float64 `json:"utilization"`
    ResetsAt    *string `json:"resets_at"`
}
```

---

### Story 7.3: Usage Bar Component

As a **cclv user**,
I want **a compact usage bar showing my limits**,
So that **I can see usage at a glance**.

**Acceptance Criteria:**

**Given** usage data is available
**When** the usage bar renders
**Then** it displays: `[5h: 35% 2h15m] [7d: 12%]`
**And** fits in 1 line at top of screen

**Given** 5-hour utilization > 80%
**When** displayed
**Then** percentage is styled with warning color (yellow/orange)

**Given** 5-hour utilization > 95%
**When** displayed
**Then** percentage is styled with critical color (red)

**Given** reset time is available
**When** displayed
**Then** shows human-readable countdown (e.g., "2h15m", "45m", "5m")

**Given** usage data is loading
**When** displayed
**Then** shows "Loading..." or spinner

**Given** credentials not found
**When** displayed
**Then** shows "Not logged in" with dimmed style

**Technical Notes:**
- New file: `internal/usage/bar.go`
- Use lipgloss for styling
- Width: Full terminal width
- Height: 1 line
- Format time remaining with `formatDuration()` helper

---

### Story 7.4: App Model Wrapper

As a **developer refactoring the app**,
I want **the root model to wrap all views with the usage bar**,
So that **usage is visible everywhere**.

**Acceptance Criteria:**

**Given** the app starts
**When** any view renders (project list, conversation list, viewer, dashboard)
**Then** usage bar appears at the top
**And** view content appears below

**Given** terminal height is H
**When** a view renders
**Then** usage bar gets 1 line
**And** view gets H-1 lines

**Given** user navigates between views
**When** transition occurs
**Then** usage bar remains constant (no flicker)

**Given** dashboard is displayed
**When** rendered
**Then** usage bar appears above the grid
**And** grid layout adjusts to remaining height

**Technical Notes:**
- Modify `internal/tui/app.go` (or equivalent root model)
- Store `usageState` at app level
- View method: `lipgloss.JoinVertical(usageBar.View(), currentView.View())`
- Pass available height to child views: `height - usageBarHeight`

---

### Story 7.5: Usage Bar Refresh

As a **cclv user monitoring limits**,
I want **usage to refresh automatically and on-demand**,
So that **I see current values**.

**Acceptance Criteria:**

**Given** the usage bar is displayed
**When** 60 seconds pass since last fetch
**Then** usage is refreshed in background

**Given** I am in any view
**When** I press `R` (shift+r)
**Then** usage refreshes immediately
**And** a brief indicator shows refresh occurred

**Given** a refresh is in progress
**When** another refresh is requested
**Then** the duplicate request is ignored

**Given** refresh fails
**When** displayed
**Then** last known values remain with "stale" indicator

**Technical Notes:**
- Use Bubble Tea `tea.Tick` for periodic refresh
- Debounce manual refresh (min 5 seconds between)
- Show subtle refresh indicator (e.g., brief flash or icon change)

---

### Story 7.6: Plain Text Usage Output

As a **scripter or terminal user**,
I want **to check usage without entering TUI**,
So that **I can use it in scripts or quick checks**.

**Acceptance Criteria:**

**Given** I run `cclv --usage` or `cclv -u`
**When** executed
**Then** prints usage to stdout and exits (no TUI)

**Output format:**
```
Claude Code Usage
  5-hour:  35% (resets in 2h 15m)
  7-day:   12%
```

**Given** credentials are not found
**When** `cclv --usage` runs
**Then** prints error message to stderr
**And** exits with non-zero code

**Given** API call fails
**When** `cclv --usage` runs
**Then** prints error message to stderr
**And** exits with non-zero code

**Technical Notes:**
- Add `--usage` / `-u` flag to CLI
- Skip TUI initialization entirely
- Use `--color` flag compatibility (auto/always/never)

---

### Story 7.7: Graceful Degradation

As a **cclv user**,
I want **the app to work even if usage fetching fails**,
So that **I can still use all other features**.

**Acceptance Criteria:**

**Given** credentials are not found at startup
**When** app starts
**Then** usage bar shows "Not logged in"
**And** all other features work normally

**Given** API call times out
**When** usage bar renders
**Then** shows last known values (or "Unknown" if first call)
**And** app continues functioning

**Given** OAuth token is expired
**When** API returns 401
**Then** usage bar shows "Session expired - run 'claude' to re-login"
**And** all other features work normally

**Given** network is unavailable
**When** usage fetch is attempted
**Then** fetch fails silently after timeout
**And** usage bar shows appropriate fallback state

**Technical Notes:**
- Never block app startup on usage fetch
- Fetch usage asynchronously after app initializes
- Log errors but don't surface to user unless actionable

---

## Implementation Order

| Order | Story | Dependency | Effort |
|-------|-------|------------|--------|
| 1 | 7.1 | None | Medium |
| 2 | 7.2 | 7.1 | Medium |
| 3 | 7.3 | 7.2 | Medium |
| 4 | 7.4 | 7.3 | Medium-High |
| 5 | 7.5 | 7.4 | Low |
| 6 | 7.6 | 7.2 | Low |
| 7 | 7.7 | 7.4 | Low |

**Critical Path:** 7.1 → 7.2 → 7.3 → 7.4

Stories 7.5, 7.6, and 7.7 can be done in parallel after 7.4.

---

## Definition of Done

- [ ] All stories implemented and tested
- [ ] Usage bar visible in all 4 views (project list, conversation list, viewer, dashboard)
- [ ] `cclv --usage` works for scripting
- [ ] macOS and Linux credential access tested
- [ ] Graceful handling of missing/expired credentials
- [ ] Test coverage maintained at 90%+
- [ ] Code review completed
- [ ] README updated with usage feature documentation
