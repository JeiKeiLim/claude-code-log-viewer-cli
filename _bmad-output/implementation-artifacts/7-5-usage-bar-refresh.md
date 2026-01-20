# Story 7.5: Usage Bar Refresh

Status: done

## Story

As a **cclv user monitoring limits**,
I want **usage to refresh automatically and on-demand**,
So that **I see current values**.

## Acceptance Criteria

1. **AC-1: Automatic Periodic Refresh**
   - Given the usage bar is displayed
   - When 60 seconds pass since last fetch
   - Then usage is refreshed in background
   - And UI does not block during refresh

2. **AC-2: Manual Refresh with R Key**
   - Given I am in any view (projects, conversations, viewer, dashboard)
   - When I press `R` (shift+r)
   - Then usage refreshes immediately
   - And a brief indicator shows refresh occurred

3. **AC-3: Debounce Manual Refresh**
   - Given a refresh is in progress or was completed < 5 seconds ago
   - When another manual refresh is requested
   - Then the duplicate request is ignored

4. **AC-4: Stale Indicator on Failure**
   - Given refresh fails (API error, timeout)
   - When displayed
   - Then last known values remain with "stale" indicator
   - And app continues functioning normally

5. **AC-5: Refresh During Loading State**
   - Given usage bar is in loading state
   - When R is pressed
   - Then no additional refresh is triggered (one is already in progress)

6. **AC-6: Cache Invalidation on Manual Refresh**
   - Given cached data exists
   - When R is pressed for manual refresh
   - Then cache is invalidated before fetching
   - And fresh data is fetched from API

## Tasks / Subtasks

- [x] Task 1: Add refresh-related state to AppModel (AC: #1, #2, #3)
  - [x] Subtask 1.1: Add `lastRefreshTime time.Time` field to AppModel
  - [x] Subtask 1.2: Add `refreshInProgress bool` field to AppModel
  - [x] Subtask 1.3: Define constants: `refreshInterval = 60 * time.Second`, `refreshDebounce = 5 * time.Second`

- [x] Task 2: Implement automatic periodic refresh (AC: #1)
  - [x] Subtask 2.1: Create `usageTickMsg` message type
  - [x] Subtask 2.2: Create `scheduleUsageTick() tea.Cmd` that returns `tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return usageTickMsg{} })`
  - [x] Subtask 2.3: Add `scheduleUsageTick()` to Init() tea.Batch after initial fetchUsage
  - [x] Subtask 2.4: Handle `usageTickMsg` in Update(): trigger fetchUsage and reschedule tick

- [x] Task 3: Implement manual refresh handler (AC: #2, #3, #5, #6)
  - [x] Subtask 3.1: Create `usageRefreshMsg` message type for manual refresh trigger
  - [x] Subtask 3.2: Handle `tea.KeyMsg` for "R" key in AppModel Update()
  - [x] Subtask 3.3: In R key handler: check debounce (time since lastRefreshTime > 5s)
  - [x] Subtask 3.4: In R key handler: check refreshInProgress is false
  - [x] Subtask 3.5: In R key handler: check usageBar state is not StateLoading
  - [x] Subtask 3.6: If checks pass: call `usageClient.InvalidateCache()`, set refreshInProgress=true, return fetchUsage()
  - [x] Subtask 3.7: If checks fail: do nothing (ignore the request)

- [x] Task 4: Update usageFetchedMsg handler (AC: #1, #4)
  - [x] Subtask 4.1: Set `refreshInProgress = false` after handling usageFetchedMsg (ALWAYS, even on error)
  - [x] Subtask 4.2: Set `lastRefreshTime = time.Now()` on successful fetch only
  - [x] Subtask 4.3: Existing error handling already sets stale state via SetLimits(limits, true) - no change needed
  - [x] Subtask 4.4: Ensure StateRefreshing transitions to StateNormal on success (via SetLimits with stale=false)

- [x] Task 5: Implement refresh indicator (AC: #2)
  - [x] Subtask 5.1: Add `StateRefreshing UsageBarState` constant to bar.go
  - [x] Subtask 5.2: Add `SetRefreshing()` method to UsageBarModel
  - [x] Subtask 5.3: Add `renderRefreshing()` method that shows current values + "[R]" indicator
  - [x] Subtask 5.4: Update View() to handle StateRefreshing
  - [x] Subtask 5.5: In AppModel, call SetRefreshing() before manual refresh fetch

- [x] Task 6: Write comprehensive tests (AC: #1-6)
  - [x] Subtask 6.1: Test Init() includes scheduleUsageTick command
  - [x] Subtask 6.2: Test usageTickMsg triggers fetchUsage and reschedules tick
  - [x] Subtask 6.3: Test R key triggers manual refresh when conditions met
  - [x] Subtask 6.4: Test R key ignored during refreshInProgress
  - [x] Subtask 6.5: Test R key ignored within debounce window
  - [x] Subtask 6.6: Test R key ignored during StateLoading
  - [x] Subtask 6.7: Test usageFetchedMsg updates lastRefreshTime and refreshInProgress
  - [x] Subtask 6.8: Test cache invalidation on manual refresh
  - [x] Subtask 6.9: Test StateRefreshing renders correctly with indicator
  - [x] Subtask 6.10: Test StateRefreshing → StateNormal on successful fetch
  - [x] Subtask 6.11: Test StateRefreshing → StateStale on error with cached data
  - [x] Subtask 6.12: Test usageTickMsg reschedules tick even when refresh is skipped

## Dev Notes

### Critical Implementation Details

**Required Imports in app.go:**

Note: `time` is already imported in app.go. Verify it's present:

```go
import (
    "time" // Already present - used for context.WithTimeout
    // ... existing imports
)
```

**New Constants in app.go:**

```go
const (
    refreshInterval = 60 * time.Second
    refreshDebounce = 5 * time.Second
)
```

**AppModel Changes:**

```go
type AppModel struct {
    // ... existing fields (usageBar, usageClient)

    // Usage refresh state (Story 7.5)
    lastRefreshTime   time.Time
    refreshInProgress bool
}
```

**Message Types:**

```go
// usageTickMsg triggers periodic usage refresh
type usageTickMsg struct{}

// scheduleUsageTick schedules the next periodic refresh
func scheduleUsageTick() tea.Cmd {
    return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
        return usageTickMsg{}
    })
}
```

**Init() Changes:**

```go
func (m AppModel) Init() tea.Cmd {
    return tea.Batch(
        m.projectModel.Init(),
        tea.WindowSize(),
        m.fetchUsage(),
        scheduleUsageTick(), // Story 7.5: start periodic refresh
    )
}
```

**Update() Additions:**

```go
case usageTickMsg:
    // Periodic refresh trigger (Story 7.5)
    // Only refresh if not already in progress
    if !m.refreshInProgress && m.usageBar.State() != usage.StateLoading {
        m.refreshInProgress = true
        return m, tea.Batch(m.fetchUsage(), scheduleUsageTick())
    }
    // Reschedule even if skipped
    return m, scheduleUsageTick()

case tea.KeyMsg:
    // Handle R key for manual refresh (Story 7.5)
    if msg.String() == "R" {
        return m.handleManualRefresh()
    }
    // ... forward to child views

case usageFetchedMsg:
    // Update refresh state (Story 7.5)
    m.refreshInProgress = false // ALWAYS reset, even on error
    if msg.err == nil {
        m.lastRefreshTime = time.Now() // Only on success
    }
    // ... existing error handling (sets StateNormal or StateStale via SetLimits)
```

**Manual Refresh Handler:**

```go
func (m AppModel) handleManualRefresh() (tea.Model, tea.Cmd) {
    // Skip if already refreshing
    if m.refreshInProgress {
        return m, nil
    }

    // Skip if in loading state
    if m.usageBar.State() == usage.StateLoading {
        return m, nil
    }

    // Skip if within debounce window
    if time.Since(m.lastRefreshTime) < refreshDebounce {
        return m, nil
    }

    // Trigger manual refresh
    m.usageClient.InvalidateCache() // Force fresh fetch
    m.refreshInProgress = true
    m.usageBar.SetRefreshing() // Show indicator
    return m, m.fetchUsage()
}
```

### UsageBarModel Additions (bar.go)

```go
// Add to UsageBarState constants
const (
    StateLoading UsageBarState = iota
    StateNormal
    StateStale
    StateNotLoggedIn
    StateError
    StateRefreshing // Story 7.5: manual refresh in progress
)

// SetRefreshing sets the bar to refreshing state (preserves current limits).
func (m *UsageBarModel) SetRefreshing() {
    // Only set if we have limits to show during refresh
    if m.limits != nil {
        m.state = StateRefreshing
    }
    // If no limits, stay in current state (loading will handle it)
}

func (m *UsageBarModel) renderRefreshing() string {
    if m.limits == nil {
        return m.renderLoading()
    }

    // Render current values with refresh indicator
    var parts []string

    if m.limits.FiveHour != nil {
        fiveHourPart := m.renderWindow("5h", m.limits.FiveHour, true)
        parts = append(parts, fiveHourPart)
    }

    if m.limits.SevenDay != nil {
        sevenDayPart := m.renderWindow("7d", m.limits.SevenDay, true)
        parts = append(parts, sevenDayPart)
    }

    content := ""
    for i, part := range parts {
        if i > 0 {
            content += m.styles.Label.Render(" ")
        }
        content += part
    }

    // Add refresh indicator
    content += m.styles.Dimmed.Render(" [R]")

    return m.applyContainer(content)
}

// Update View() switch statement
func (m *UsageBarModel) View() string {
    switch m.state {
    case StateLoading:
        return m.renderLoading()
    case StateNotLoggedIn:
        return m.renderNotLoggedIn()
    case StateError:
        return m.renderError()
    case StateRefreshing:
        return m.renderRefreshing()
    case StateNormal, StateStale:
        return m.renderUsage()
    default:
        return ""
    }
}
```

### Key Handler Integration

The R key must be captured in AppModel.Update() BEFORE forwarding to child views to ensure global handling. Add at the top of the Update switch:

```go
case tea.KeyMsg:
    // Global key handlers (Story 7.5)
    if msg.String() == "R" {
        return m.handleManualRefresh()
    }
    // Forward to current view...
```

### Package Structure

```
internal/tui/
├── app.go               # MODIFIED: Add refresh state, usageTickMsg, handleManualRefresh
├── app_test.go          # MODIFIED: Add tests for refresh behavior
└── ...

internal/usage/
├── bar.go               # MODIFIED: Add StateRefreshing, SetRefreshing, renderRefreshing
├── bar_test.go          # MODIFIED: Add tests for refreshing state
└── client.go            # NO CHANGE: InvalidateCache already exists
```

### Project Structure Notes

- Modifications only to `internal/tui/app.go` and `internal/usage/bar.go`
- No new files needed
- Use existing `InvalidateCache()` method on usage.Client
- tea.Tick pattern from Bubbletea for periodic refresh

### Timing Considerations

**tea.Tick Behavior:**
- Returns a tea.Cmd that sends a message after duration
- Must be rescheduled after each tick (not automatic repeat)
- Safe to call multiple times (multiple ticks won't cause issues)

**Race Condition Prevention:**
- `refreshInProgress` flag prevents overlapping fetches
- Set to true BEFORE returning fetchUsage command
- Set to false in usageFetchedMsg handler (always, even on error)

**Debounce Logic:**
- Check `time.Since(lastRefreshTime) < refreshDebounce`
- 5 seconds prevents spam while allowing reasonable refresh rate
- lastRefreshTime updated on EVERY successful fetch (auto or manual)

### Testing Strategy

**Unit Tests (app_test.go):**

```go
func TestAppModel_Init_SchedulesUsageTick(t *testing.T) {
    m := NewAppModel([]types.Project{})
    cmd := m.Init()
    // Verify batch includes scheduleUsageTick
    // This is indirect - check that usageTickMsg is eventually produced
}

func TestAppModel_UsageTickMsg_TriggersRefresh(t *testing.T) {
    m := NewAppModel([]types.Project{})
    m.refreshInProgress = false

    newModel, cmd := m.Update(usageTickMsg{})
    m = newModel.(AppModel)

    if !m.refreshInProgress {
        t.Error("expected refreshInProgress to be true after tick")
    }
    if cmd == nil {
        t.Error("expected cmd to include fetchUsage")
    }
}

func TestAppModel_ManualRefresh_R_Key(t *testing.T) {
    m := NewAppModel([]types.Project{})
    // Simulate successful initial fetch
    m, _ = m.Update(usageFetchedMsg{
        limits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 35}},
    }).(AppModel)
    // Wait for debounce
    m.lastRefreshTime = time.Now().Add(-10 * time.Second)

    // Note: Bubbletea KeyMsg for "R" (shift+r) has Type: tea.KeyRunes
    // and msg.String() returns "R". Create the message correctly:
    keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
    // Verify: keyMsg.String() == "R"

    newModel, cmd := m.Update(keyMsg)
    m = newModel.(AppModel)

    if !m.refreshInProgress {
        t.Error("expected manual refresh to trigger")
    }
    if cmd == nil {
        t.Error("expected fetchUsage command")
    }
}

func TestAppModel_ManualRefresh_Debounce(t *testing.T) {
    m := NewAppModel([]types.Project{})
    m, _ = m.Update(usageFetchedMsg{limits: &usage.UsageLimits{}}).(AppModel)
    m.lastRefreshTime = time.Now() // Just refreshed

    newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
    m = newModel.(AppModel)

    if m.refreshInProgress {
        t.Error("expected refresh to be blocked by debounce")
    }
    if cmd != nil {
        t.Error("expected no command when debounced")
    }
}

func TestAppModel_ManualRefresh_IgnoredDuringLoading(t *testing.T) {
    m := NewAppModel([]types.Project{})
    // Initial state is loading (no usageFetchedMsg received)

    newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
    m = newModel.(AppModel)

    if cmd != nil {
        t.Error("expected no refresh during loading state")
    }
}

func TestAppModel_ManualRefresh_IgnoredWhenInProgress(t *testing.T) {
    m := NewAppModel([]types.Project{})
    m.refreshInProgress = true
    m.lastRefreshTime = time.Now().Add(-10 * time.Second)

    newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
    m = newModel.(AppModel)

    if cmd != nil {
        t.Error("expected no additional refresh when already in progress")
    }
}
```

**Unit Tests (bar_test.go):**

```go
func TestUsageBarModel_SetRefreshing(t *testing.T) {
    styles := usage.UsageBarStyles{/* ... */}
    m := usage.NewUsageBarModel(styles)

    // Set limits first
    m.SetLimits(&usage.UsageLimits{
        FiveHour: &usage.UsageWindow{Utilization: 35},
    }, false)

    m.SetRefreshing()

    if m.State() != usage.StateRefreshing {
        t.Errorf("expected StateRefreshing, got %v", m.State())
    }
}

func TestUsageBarModel_View_Refreshing(t *testing.T) {
    styles := usage.UsageBarStyles{/* ... */}
    m := usage.NewUsageBarModel(styles)
    m.SetWidth(80)
    m.SetLimits(&usage.UsageLimits{
        FiveHour: &usage.UsageWindow{Utilization: 35},
    }, false)
    m.SetRefreshing()

    view := m.View()

    if !strings.Contains(view, "[R]") {
        t.Error("expected refresh indicator [R] in view")
    }
}

func TestAppModel_StateRefreshing_ToNormal_OnSuccess(t *testing.T) {
    m := NewAppModel([]types.Project{})
    // Set to refreshing state
    m.usageBar.SetLimits(&usage.UsageLimits{
        FiveHour: &usage.UsageWindow{Utilization: 35},
    }, false)
    m.usageBar.SetRefreshing()
    m.refreshInProgress = true

    // Simulate successful fetch
    newModel, _ := m.Update(usageFetchedMsg{
        limits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 40}},
        stale:  false,
    })
    m = newModel.(AppModel)

    if m.usageBar.State() != usage.StateNormal {
        t.Errorf("expected StateNormal after successful fetch, got %v", m.usageBar.State())
    }
    if m.refreshInProgress {
        t.Error("expected refreshInProgress to be false after fetch")
    }
}

func TestAppModel_StateRefreshing_ToStale_OnError(t *testing.T) {
    m := NewAppModel([]types.Project{})
    // Set to refreshing state with existing limits
    existingLimits := &usage.UsageLimits{
        FiveHour: &usage.UsageWindow{Utilization: 35},
    }
    m.usageBar.SetLimits(existingLimits, false)
    m.usageBar.SetRefreshing()
    m.refreshInProgress = true

    // Simulate error with stale cached data
    newModel, _ := m.Update(usageFetchedMsg{
        limits: existingLimits,
        stale:  true,
        err:    fmt.Errorf("network error"),
    })
    m = newModel.(AppModel)

    if m.usageBar.State() != usage.StateStale {
        t.Errorf("expected StateStale after error with cached data, got %v", m.usageBar.State())
    }
}

func TestAppModel_UsageTickMsg_ReschedulesEvenWhenSkipped(t *testing.T) {
    m := NewAppModel([]types.Project{})
    m.refreshInProgress = true // Already refreshing

    newModel, cmd := m.Update(usageTickMsg{})
    m = newModel.(AppModel)

    // Should still return a command (the reschedule tick)
    if cmd == nil {
        t.Error("expected reschedule tick command even when refresh is skipped")
    }
}
```

### Critical Rules (from project-context.md)

- NO EMOJI in any output (use `[R]` text indicator)
- Use `make test` not raw `go test`
- Table-driven tests required where applicable
- Error wrapping: `fmt.Errorf("context: %w", err)`
- All styles defined in `internal/tui/styles.go` - use getter functions

### Previous Story Learnings

**From Story 7.4 (App Model Wrapper):**
- `usageFetchedMsg` is already handled with error mapping
- `fetchUsage()` tea.Cmd pattern is established
- `usageBar.State()` getter exists for state checking
- Height adjustment already working for all views

**From Story 7.3 (Usage Bar Component):**
- `UsageBarModel` is a pure view component (state managed externally)
- State methods: `SetLoading()`, `SetLimits()`, `SetNotLoggedIn()`, `SetError()`
- `State()` returns current state enum
- `View()` switch handles all states

**From Story 7.2 (Usage API Client):**
- `Client.InvalidateCache()` method already exists
- `FetchUsage(ctx, token)` returns `(limits, stale, error)`
- Cache TTL is 60 seconds (matching our refresh interval)

### Anti-Patterns to Avoid

1. **DO NOT** use goroutines directly - use tea.Cmd for all async operations
2. **DO NOT** forget to reschedule tick after handling usageTickMsg
3. **DO NOT** block UI during refresh - always return immediately with command
4. **DO NOT** use emoji for refresh indicator - use `[R]` text
5. **DO NOT** refresh if already refreshing (check refreshInProgress)
6. **DO NOT** refresh during StateLoading (initial fetch in progress)
7. **DO NOT** forget to update lastRefreshTime on successful fetch
8. **DO NOT** forget to set refreshInProgress=false in usageFetchedMsg handler

### Architectural Considerations

**Tea.Tick vs Goroutine:**
- Use tea.Tick for periodic operations (integrates with Bubbletea event loop)
- Never use goroutines directly for timed operations
- tea.Tick is cancelable by returning nil from Update

**Refresh State Machine:**
```
Init → fetchUsage + scheduleUsageTick
         ↓
    usageFetchedMsg → refreshInProgress=false, lastRefreshTime=now
         ↓
    [wait 60s]
         ↓
    usageTickMsg → if not refreshing: fetchUsage + reschedule
         ↓
    (repeat)

Manual refresh (R key):
    Check: !refreshInProgress && state != Loading && time > debounce
    → InvalidateCache() + SetRefreshing() + fetchUsage()
         ↓
    usageFetchedMsg → refreshInProgress=false, state=Normal/Stale
```

### Expected Commit Format

```
feat: add automatic and manual usage bar refresh (Story 7.5)

Implements periodic and on-demand usage refresh:
- Add 60-second automatic refresh via tea.Tick
- Add R key handler for manual refresh
- Add 5-second debounce for manual refresh
- Add StateRefreshing with [R] indicator
- Invalidate cache on manual refresh for fresh data

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4.md#Story-7.5]
- [Source: _bmad-output/project-context.md] - Critical rules and patterns
- [Source: internal/usage/bar.go] - UsageBarModel API
- [Source: internal/usage/client.go] - Client.InvalidateCache() exists
- [Source: internal/usage/types.go] - Error types
- [Source: internal/tui/app.go] - Current AppModel with usage integration
- [Source: _bmad-output/implementation-artifacts/7-4-app-model-wrapper.md] - Previous story patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - All tests passing

### Completion Notes List

- Added `refreshInterval` (60s) and `refreshDebounce` (5s) constants to app.go
- Added `lastRefreshTime` and `refreshInProgress` fields to AppModel
- Implemented `usageTickMsg` and `scheduleUsageTick()` for periodic refresh via tea.Tick
- Implemented R key handler for manual refresh with debounce, in-progress, and loading state checks
- Updated `usageFetchedMsg` handler to reset `refreshInProgress` and update `lastRefreshTime`
- Added `StateRefreshing` state and `SetRefreshing()` method to UsageBarModel
- Added `renderRefreshing()` method showing current values with `[R]` indicator
- All acceptance criteria covered with comprehensive tests (table-driven where applicable)

### File List

- internal/tui/app.go (MODIFIED: refresh state, tick msg, handlers)
- internal/tui/app_test.go (MODIFIED: comprehensive 7.5 tests, added scheduleUsageTick and lowercase 'r' tests)
- internal/usage/bar.go (MODIFIED: StateRefreshing, SetRefreshing, renderRefreshing, refactored renderWindowParts)
- internal/usage/bar_test.go (MODIFIED: refreshing state tests, added fallback test)
- _bmad-output/implementation-artifacts/sprint-status.yaml (MODIFIED: story status tracking)
