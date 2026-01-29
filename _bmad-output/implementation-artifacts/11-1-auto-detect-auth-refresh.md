# Story 11.1: Auto-Detect Claude Auth Refresh

Status: done

## Story

As a **cclv user who has been idle (e.g., overnight)**,
I want **CCLV to automatically detect when I've refreshed my Claude auth**,
So that **I can see usage limits again without restarting the app**.

## Acceptance Criteria

1. **AC-1: Periodic Auth Check When Expired**
   - Given usage bar shows "Run 'claude' to refresh" (auth expired state)
   - When 5 minutes have passed since last check
   - Then CCLV attempts to re-fetch usage data
   - And if successful, usage bar is restored with current values

2. **AC-2: No Polling When Valid**
   - Given auth is valid and usage bar is showing normally
   - When time passes
   - Then no additional auth checks are performed (existing 60s refresh interval is sufficient)

3. **AC-3: Visual Feedback on Recovery**
   - Given auth was expired and user refreshed externally (ran `claude` command)
   - When CCLV detects valid auth on retry
   - Then toast shows "Usage limits restored" (or similar brief feedback)
   - And usage bar displays current usage values
   - Note: Toast may not display in project view (acceptable degradation)

4. **AC-4: Graceful Network Errors**
   - Given auth retry check fails due to network error
   - When retry occurs
   - Then error is logged but not shown to user (no UI change)
   - And retry polling continues at 5-minute interval

5. **AC-5: Manual Refresh Stops Retry Polling**
   - Given auth is expired and retry polling is active
   - When user presses `R` (manual refresh) and fetch succeeds
   - Then `authExpired` becomes false and retry polling stops

## Tasks / Subtasks

- [x] Task 1: Add auth expired state tracking to AppModel (AC: #1, #2)
  - [x] 1.1: Add `authExpired bool` field to AppModel struct
  - [x] 1.2: Update `usageFetchedMsg` handler to set `authExpired = true` when `ErrTokenExpired` received
  - [x] 1.3: Update `usageFetchedMsg` handler to set `authExpired = false` when successful fetch occurs

- [x] Task 2: Implement auth retry ticker mechanism (AC: #1, #5)
  - [x] 2.1: Create `authRetryTickMsg` message type (follows `{action}Msg` convention)
  - [x] 2.2: Create `scheduleAuthRetryTick()` function returning `tea.Cmd` (5-minute interval)
  - [x] 2.3: Add `authRetryTickMsg` handler in AppModel.Update() that:
    - Checks `authExpired == true` AND `!m.refreshInProgress`
    - If true, sets `refreshInProgress = true` and returns `m.fetchUsage()`
    - Does NOT reschedule (rescheduling happens in `usageFetchedMsg` handler)
  - [x] 2.4: Trigger first retry tick when `ErrTokenExpired` is detected

- [x] Task 3: Handle recovery with visual feedback (AC: #3)
  - [x] 3.1: In `usageFetchedMsg`, capture `wasExpired := m.authExpired` BEFORE modifying state
  - [x] 3.2: On successful fetch when `wasExpired == true`, emit `ShowToastMsg{Message: "Usage limits restored"}`
  - [x] 3.3: Toast is already handled (line 337-340) - no additional work needed

- [x] Task 4: Handle network errors during retry silently (AC: #4)
  - [x] 4.1: In `usageFetchedMsg`, when `authExpired == true` AND error is NOT `ErrTokenExpired`:
    - Log error: `log.Printf("usage retry error (continuing): %v", msg.err)`
    - Keep `authExpired = true` (do not reset)
    - Return `scheduleAuthRetryTick()` to continue polling
  - [x] 4.2: Ensure `ErrAPITimeout`, `ErrAPIError` don't reset expired state

- [x] Task 5: Add unit tests (AC: #1, #2, #3, #4, #5)
  - [x] 5.1: Test `authExpired` becomes true on `ErrTokenExpired`
  - [x] 5.2: Test `authExpired` becomes false on successful fetch
  - [x] 5.3: Test `authRetryTickMsg` only triggers fetch when `authExpired && !refreshInProgress`
  - [x] 5.4: Test recovery from expired state triggers `ShowToastMsg`
  - [x] 5.5: Test network errors during retry don't change `authExpired` state
  - [x] 5.6: Test manual refresh (`R` key) success clears `authExpired`

- [x] Task 6: Manual verification (AC: CLI smoke test per project-context.md)
  - [x] 6.1: `make build` passes
  - [x] 6.2: `make test` passes
  - [x] 6.3: Start cclv, verify usage bar displays normally
  - [x] 6.4: Clear `~/.claude/.credentials.json` (or wait for natural expiry)
  - [x] 6.5: Verify "Run 'claude' to refresh" message appears
  - [x] 6.6: Run `claude` in another terminal to re-authenticate
  - [x] 6.7: Wait up to 5 minutes, verify usage bar restores automatically

## Dev Notes

### Current Implementation (internal/tui/app.go:352-385)

Current `usageFetchedMsg` handler at line 352:
- Sets `m.refreshInProgress = false`
- On `ErrTokenExpired`: calls `m.usageBar.SetError("Run 'claude' to refresh")`
- Returns `nil` - **no retry mechanism exists**

### Required Changes to `usageFetchedMsg` Handler

```go
case usageFetchedMsg:
    wasExpired := m.authExpired  // Capture for recovery detection
    m.refreshInProgress = false

    if msg.err != nil {
        if errors.Is(msg.err, usage.ErrTokenExpired) {
            m.usageBar.SetError("Run 'claude' to refresh")
            if !m.authExpired {
                m.authExpired = true  // First time expired
            }
            return m, scheduleAuthRetryTick()  // Schedule retry
        }
        // Network error during retry - keep polling
        if m.authExpired {
            log.Printf("usage retry error (continuing): %v", msg.err)
            return m, scheduleAuthRetryTick()
        }
        // Normal error handling for non-expired state (existing code)
        // ...
    } else {
        // Success
        m.usageBar.SetLimits(msg.limits, msg.stale)
        var cmds []tea.Cmd
        if wasExpired {
            m.authExpired = false
            cmds = append(cmds, func() tea.Msg {
                return ShowToastMsg{Message: "Usage limits restored"}
            })
        }
        return m, tea.Batch(cmds...)
    }
```

### New Message Type and Ticker

```go
// authRetryTickMsg triggers retry when auth is expired (Story 11.1)
type authRetryTickMsg struct{}

// scheduleAuthRetryTick schedules auth retry (only called when expired)
func scheduleAuthRetryTick() tea.Cmd {
    return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
        return authRetryTickMsg{}
    })
}
```

### Handler for `authRetryTickMsg`

```go
case authRetryTickMsg:
    if m.authExpired && !m.refreshInProgress {
        m.refreshInProgress = true
        return m, m.fetchUsage()
    }
    return m, nil  // Recovered or refresh in progress - stop polling
```

### Files to Modify

| File | Changes |
|------|---------|
| `internal/tui/app.go` | Add `authExpired bool` field; Add `authRetryTickMsg` type; Add `scheduleAuthRetryTick()` func; Modify `usageFetchedMsg` handler; Add `authRetryTickMsg` handler |
| `internal/tui/app_test.go` | Add tests for auth retry lifecycle, recovery detection, network error handling |

### Key API References

| Location | API | Purpose |
|----------|-----|---------|
| `internal/usage/bar.go:19` | `StateError` | Usage bar error state enum |
| `internal/usage/bar.go:79-84` | `SetError(msg)` | Set error message on bar |
| `internal/usage/types.go:16` | `ErrTokenExpired` | Sentinel error for expired auth |
| `internal/tui/app.go:34-36` | `refreshInterval`, `refreshDebounce` | Existing timing constants |
| `internal/tui/app.go:70` | `refreshInProgress bool` | Debounce flag (respect this!) |
| `internal/tui/project.go:177` | `ShowToastMsg` | Toast message type |

### Patterns from Previous Stories

**From Story 10.1 (Race Condition Fix):**
- Capture state before modification: `wasExpired := m.authExpired`
- TEA commands produce messages, not direct state changes
- Use `tea.Batch()` for multiple commands

**From Story 10.2 (Manual Refresh):**
- Respect `refreshInProgress` flag for debouncing
- Pattern: check `!m.refreshInProgress` before starting fetch

### Project Context Compliance

- **Bubbletea pattern:** Uses `tea.Tick` for retry scheduling (matches `scheduleUsageTick`)
- **Error handling:** Log network errors but don't surface to user
- **No new dependencies:** Uses existing Bubbletea tick pattern
- **Message naming:** Follows `{action}Msg` convention

---

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Added `authExpired bool` field to AppModel struct (line 73)
- Created `authRetryTickMsg` message type and `scheduleAuthRetryTick()` function with 5-minute interval (lines 542-553)
- Added `authRetryTickMsg` handler that checks `authExpired && !refreshInProgress` before triggering fetch (lines 355-362)
- Modified `usageFetchedMsg` handler:
  - Captures `wasExpired := m.authExpired` before state changes (line 366)
  - Sets `authExpired = true` and schedules retry on `ErrTokenExpired` (lines 387-390)
  - Silently logs network errors during retry and continues polling (lines 394-397)
  - Emits `ShowToastMsg{Message: "Usage limits restored"}` on recovery (lines 410-414)
- Added 18 comprehensive unit tests covering all acceptance criteria in app_test.go (lines 1314-1509)
- All tests pass with race detection enabled
- Build succeeds with `make build`

### File List

- `internal/tui/app.go` - Modified: Added authExpired field, authRetryTickMsg type, scheduleAuthRetryTick function, authRetryTickMsg handler, updated usageFetchedMsg handler
- `internal/tui/app_test.go` - Modified: Added 18 Story 11.1 tests for auth retry lifecycle

## Change Log

- 2026-01-29: Story 11.1 implementation complete - Auto-detect auth refresh with 5-minute polling when expired
- 2026-01-29: Code review fixes applied:
  - Added test `TestAppModel_CredentialsRemovedDuringRetry` for edge case when credentials removed during retry polling
  - Fixed incorrect comment reference in context.Canceled handler (removed misleading AC reference)
