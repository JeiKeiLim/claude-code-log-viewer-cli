# Story 7.7: Graceful Degradation

Status: done

## Story

As a **cclv user**,
I want **the app to work even if usage fetching fails**,
So that **I can still use all other features**.

## Acceptance Criteria

1. **AC-1: Missing Credentials at Startup**
   - Given credentials are not found at startup
   - When app starts
   - Then usage bar shows "Not logged in"
   - And all other features work normally (project list, conversations, viewer, dashboard)

2. **AC-2: API Timeout Handling**
   - Given API call times out (exceeds 5 seconds)
   - When usage bar renders
   - Then shows last known values (if available) with "(stale)" indicator
   - Or shows "Unknown" if this is the first call
   - And app continues functioning without blocking

3. **AC-3: Expired OAuth Token**
   - Given OAuth token is expired (API returns 401)
   - When API call is attempted
   - Then usage bar shows "Session expired"
   - And all other features work normally
   - And error is logged but not shown as dialog/popup

4. **AC-4: Network Unavailable**
   - Given network is unavailable (no internet connection)
   - When usage fetch is attempted
   - Then fetch fails silently after timeout
   - And usage bar shows appropriate fallback state
   - And no crash or panic occurs

5. **AC-5: Non-Blocking Startup**
   - Given app initialization
   - When usage fetch is triggered
   - Then it runs asynchronously (tea.Cmd pattern)
   - And app startup is not blocked
   - And project list is immediately usable

6. **AC-6: Error Logging**
   - Given any usage-related error occurs
   - When handled
   - Then error is logged via `log.Printf` for debugging
   - But NOT surfaced to user unless actionable (expired token)

## Tasks / Subtasks

- [x] Task 1: Audit existing error handling in `app.go` (AC: #1, #2, #3, #4, #6)
  - [x] Subtask 1.1: Review `usageFetchedMsg` handler in `AppModel.Update()` for completeness
  - [x] Subtask 1.2: Verify all sentinel errors from `usage/types.go` are handled
  - [x] Subtask 1.3: Confirm stale data path returns `lastGood` values correctly

- [x] Task 2: Verify non-blocking async fetch (AC: #5)
  - [x] Subtask 2.1: Confirm `fetchUsage()` returns `tea.Cmd` (not blocking call)
  - [x] Subtask 2.2: Verify `Init()` returns batch with async usage fetch
  - [x] Subtask 2.3: Test that project list is interactive during usage load

- [x] Task 3: Add missing error state handling (AC: #2, #4)
  - [x] Subtask 3.1: Add "Unknown" state for first-time failures with no lastGood
  - [x] Subtask 3.2: Ensure network errors map to appropriate bar state
  - [x] Subtask 3.3: Handle context.Canceled errors gracefully

- [x] Task 4: Add error logging (AC: #6)
  - [x] Subtask 4.1: Add `log.Printf` for credential errors (debug context)
  - [x] Subtask 4.2: Add `log.Printf` for API errors (debug context)
  - [x] Subtask 4.3: Ensure logs go to stderr (not stdout)

- [x] Task 5: Write comprehensive tests (AC: #1-6)
  - [x] Subtask 5.1: Test no-credentials state renders "Not logged in"
  - [x] Subtask 5.2: Test API timeout returns stale data if available
  - [x] Subtask 5.3: Test expired token renders "Session expired"
  - [x] Subtask 5.4: Test network error doesn't block/crash app
  - [x] Subtask 5.5: Test async fetch doesn't block Init()
  - [x] Subtask 5.6: Verify 90%+ coverage maintained

## Dev Notes

### Current Implementation Analysis

The existing implementation in `app.go` already handles most graceful degradation scenarios. This story is primarily about **verification and gap-filling**, not greenfield implementation.

**Already Implemented (in `AppModel.Update()` usageFetchedMsg handler):**
```go
case usageFetchedMsg:
    // Handle usage fetch result (Story 7.4)
    if msg.err != nil {
        // Handle specific error types
        if errors.Is(msg.err, usage.ErrNoCredentials) ||
            errors.Is(msg.err, usage.ErrKeychainNotFound) ||
            errors.Is(msg.err, usage.ErrKeychainTimeout) {
            m.usageBar.SetNotLoggedIn()
        } else if errors.Is(msg.err, usage.ErrTokenExpired) {
            m.usageBar.SetError("Session expired")
        } else if msg.limits != nil {
            // Error but have stale data
            m.usageBar.SetLimits(msg.limits, true)
        } else {
            m.usageBar.SetError("Usage unavailable")
        }
    } else {
        m.usageBar.SetLimits(msg.limits, msg.stale)
    }
```

**Already Implemented (in `AppModel.fetchUsage()` method):**
```go
func (m AppModel) fetchUsage() tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        token, err := usage.GetOAuthToken()
        if err != nil {
            return usageFetchedMsg{err: err}
        }
        limits, stale, err := m.usageClient.FetchUsage(ctx, token)
        return usageFetchedMsg{limits: limits, stale: stale, err: err}
    }
}
```

### Sentinel Errors Reference (from `usage/types.go`)

```go
var (
    ErrNoCredentials      = errors.New("no Claude Code credentials found")
    ErrKeychainNotFound   = errors.New("credential not found in Keychain")
    ErrKeychainTimeout    = errors.New("keychain access timed out")
    ErrInvalidCredentials = errors.New("invalid credentials format")
    ErrTokenExpired       = errors.New("OAuth token has expired - run 'claude' to re-login")
    ErrEmptyToken         = errors.New("credentials file exists but accessToken is empty")
    ErrAPITimeout         = errors.New("usage API request timed out")
    ErrAPIError           = errors.New("usage API returned an error")
)
```

### Gaps to Address

1. **Missing Error Handling**: `ErrInvalidCredentials`, `ErrEmptyToken`, `ErrAPIError` are not explicitly handled - they fall through to "Usage unavailable" which is correct but should be verified.

2. **Error Logging**: No `log.Printf` calls for debugging - errors are silently mapped to UI states. Add logging for troubleshooting.

3. **Test Coverage**: Need explicit tests for each error path to ensure graceful degradation.

### Implementation Pattern

**DO NOT change the fundamental async pattern** - it already works. Focus on:
1. Verifying all error paths
2. Adding debug logging
3. Writing comprehensive tests

### Error Handling Decision Matrix

| Error Type | Bar State | Log Level | User Message |
|------------|-----------|-----------|--------------|
| `ErrNoCredentials` | `StateNotLoggedIn` | Debug | "Not logged in" |
| `ErrKeychainNotFound` | `StateNotLoggedIn` | Debug | "Not logged in" |
| `ErrKeychainTimeout` | `StateNotLoggedIn` | Warning | "Not logged in" |
| `ErrInvalidCredentials` | `StateNotLoggedIn` | Warning | "Not logged in" |
| `ErrEmptyToken` | `StateNotLoggedIn` | Warning | "Not logged in" |
| `ErrTokenExpired` | `StateError` | Info | "Session expired" |
| `ErrAPITimeout` (with stale) | `StateStale` | Warning | Shows stale data |
| `ErrAPITimeout` (no stale) | `StateError` | Warning | "Usage unavailable" |
| `ErrAPIError` (with stale) | `StateStale` | Warning | Shows stale data |
| `ErrAPIError` (no stale) | `StateError` | Warning | "Usage unavailable" |
| Network error | Same as timeout | Warning | Same as timeout |

### Test File Location

Tests should be added to: `internal/tui/app_test.go`

### Files to Modify

- `internal/tui/app.go` - Add logging to error handler, expand error handling for missing sentinel errors
- `internal/tui/app_test.go` - Add tests for graceful degradation scenarios (table-driven per project-context.md)

### Files to NOT Modify

- `internal/usage/client.go` - Already returns stale data on error (see `FetchUsage()` lastGood handling)
- `internal/usage/credentials.go` - Already returns clear sentinel errors
- `internal/usage/bar.go` - Already has all needed states
- `internal/usage/types.go` - Sentinel errors already defined

### Code Change: Updated usageFetchedMsg Handler

The following shows the COMPLETE replacement for the `usageFetchedMsg` case in `AppModel.Update()`:

```go
case usageFetchedMsg:
    // Update refresh state (Story 7.5)
    m.refreshInProgress = false // ALWAYS reset, even on error
    if msg.err == nil {
        m.lastRefreshTime = time.Now() // Only on success
    }

    // Handle usage fetch result (Story 7.4, 7.7)
    if msg.err != nil {
        // Log for debugging (AC-6) - goes to stderr
        log.Printf("usage fetch error: %v", msg.err)

        // Handle specific error types (expanded for Story 7.7)
        if errors.Is(msg.err, usage.ErrNoCredentials) ||
            errors.Is(msg.err, usage.ErrKeychainNotFound) ||
            errors.Is(msg.err, usage.ErrKeychainTimeout) ||
            errors.Is(msg.err, usage.ErrInvalidCredentials) ||
            errors.Is(msg.err, usage.ErrEmptyToken) {
            m.usageBar.SetNotLoggedIn()
        } else if errors.Is(msg.err, usage.ErrTokenExpired) {
            m.usageBar.SetError("Session expired")
        } else if msg.limits != nil {
            // Error but have stale data - show stale values
            m.usageBar.SetLimits(msg.limits, true)
        } else {
            // No stale data available - show unavailable
            m.usageBar.SetError("Usage unavailable")
        }
    } else {
        m.usageBar.SetLimits(msg.limits, msg.stale)
    }
    return m, nil
```

**Changes from current implementation:**
1. Added `log.Printf` for debugging (AC-6)
2. Added `ErrInvalidCredentials` to SetNotLoggedIn() branch
3. Added `ErrEmptyToken` to SetNotLoggedIn() branch
4. Added comments for clarity

### Test Cases Required

Add to `internal/tui/app_test.go`:

```go
func TestGracefulDegradation(t *testing.T) {
    tests := []struct {
        name           string
        fetchErr       error
        staleLimits    *usage.UsageLimits
        wantState      usage.UsageBarState
        wantErrMsg     string
    }{
        {
            name:       "no credentials",
            fetchErr:   usage.ErrNoCredentials,
            wantState:  usage.StateNotLoggedIn,
        },
        {
            name:       "keychain not found",
            fetchErr:   usage.ErrKeychainNotFound,
            wantState:  usage.StateNotLoggedIn,
        },
        {
            name:       "keychain timeout",
            fetchErr:   usage.ErrKeychainTimeout,
            wantState:  usage.StateNotLoggedIn,
        },
        {
            name:       "invalid credentials",
            fetchErr:   usage.ErrInvalidCredentials,
            wantState:  usage.StateNotLoggedIn,
        },
        {
            name:       "empty token",
            fetchErr:   usage.ErrEmptyToken,
            wantState:  usage.StateNotLoggedIn,
        },
        {
            name:       "token expired",
            fetchErr:   usage.ErrTokenExpired,
            wantState:  usage.StateError,
            wantErrMsg: "Session expired",
        },
        {
            name:        "api timeout with stale data",
            fetchErr:    usage.ErrAPITimeout,
            staleLimits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 50}},
            wantState:   usage.StateStale,
        },
        {
            name:       "api timeout without stale data",
            fetchErr:   usage.ErrAPITimeout,
            wantState:  usage.StateError,
            wantErrMsg: "Usage unavailable",
        },
        {
            name:        "api error with stale data",
            fetchErr:    fmt.Errorf("%w: HTTP 500", usage.ErrAPIError),
            staleLimits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 25}},
            wantState:   usage.StateStale,
        },
        {
            name:       "api error without stale data",
            fetchErr:   fmt.Errorf("%w: HTTP 500", usage.ErrAPIError),
            wantState:  usage.StateError,
            wantErrMsg: "Usage unavailable",
        },
        {
            name:        "network error with stale data",
            fetchErr:    fmt.Errorf("dial tcp: connect: network unreachable"),
            staleLimits: &usage.UsageLimits{FiveHour: &usage.UsageWindow{Utilization: 30}},
            wantState:   usage.StateStale,
        },
        {
            name:       "network error without stale data",
            fetchErr:   fmt.Errorf("dial tcp: connect: network unreachable"),
            wantState:  usage.StateError,
            wantErrMsg: "Usage unavailable",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create app model with empty projects
            m := NewAppModel([]types.Project{})

            // Simulate usageFetchedMsg
            msg := usageFetchedMsg{
                err:    tt.fetchErr,
                limits: tt.staleLimits,
                stale:  tt.staleLimits != nil,
            }

            result, _ := m.Update(msg)
            newModel := result.(AppModel)

            if newModel.UsageBarState() != tt.wantState {
                t.Errorf("got state %v, want %v", newModel.UsageBarState(), tt.wantState)
            }

            // Verify error message if applicable
            if tt.wantErrMsg != "" {
                // Check bar contains expected message
                view := newModel.usageBar.View()
                if !strings.Contains(view, tt.wantErrMsg) {
                    t.Errorf("bar view %q does not contain %q", view, tt.wantErrMsg)
                }
            }
        })
    }
}

func TestNonBlockingInit(t *testing.T) {
    // Create app model
    m := NewAppModel([]types.Project{})

    // Init() should return a tea.Batch (multiple commands)
    cmd := m.Init()
    if cmd == nil {
        t.Error("Init() returned nil, expected tea.Batch with async fetch")
    }

    // Verify the app is immediately usable (not blocked)
    // Project list should be accessible
    if m.state != viewProjects {
        t.Errorf("initial state %v, want viewProjects", m.state)
    }
}
```

### Project Structure Notes

- All changes confined to `internal/tui/app.go` and test files
- No new files required
- No new dependencies required
- Follows existing error handling patterns

### Previous Story Learnings (Stories 7.1-7.6)

From Story 7.1:
- Sentinel errors are well-defined and testable
- macOS Keychain access can timeout (5 seconds)
- File-based credentials may have malformed JSON

From Story 7.2:
- `FetchUsage()` returns `(limits, stale, error)` tuple
- `lastGood` is preserved for graceful degradation
- Cache invalidation works correctly

From Story 7.4:
- Usage bar renders at app level, not per-view
- State changes via `SetLimits()`, `SetError()`, `SetNotLoggedIn()`
- Async fetch via `tea.Cmd` pattern

From Story 7.5:
- `refreshInProgress` flag prevents duplicate fetches
- Cache invalidation triggers fresh fetch
- Debounce prevents rapid manual refreshes

### Critical Rules (from project-context.md)

- NO EMOJI in any output
- Use `make test` not raw `go test`
- Table-driven tests required
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Logging goes to stderr via `log.Printf`

### Anti-Patterns to Avoid

1. **DO NOT** add blocking calls to `Init()` or `fetchUsage()`
2. **DO NOT** show popup/dialog for usage errors (use bar state only)
3. **DO NOT** panic on any usage-related error
4. **DO NOT** retry failed fetches automatically (user can press R)
5. **DO NOT** store error details beyond what bar needs
6. **DO NOT** modify `usage/client.go` or `usage/credentials.go`

### Expected Commit Format

```
feat: add graceful degradation for usage monitoring (Story 7.7)

Ensures app functions when usage fetching fails:
- Expanded error type handling (invalid credentials, empty token)
- Added debug logging for troubleshooting
- Comprehensive tests for all degradation scenarios
- Verified non-blocking async fetch pattern

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4.md#Story-7.7]
- [Source: _bmad-output/project-context.md] - Critical rules
- [Source: internal/tui/app.go] - `AppModel.Update()` usageFetchedMsg handler
- [Source: internal/tui/app.go] - `AppModel.fetchUsage()` async pattern
- [Source: internal/usage/types.go] - Sentinel error definitions
- [Source: internal/usage/client.go] - `FetchUsage()` with stale data handling

### Dependency Notes

**Depends on (completed):**
- Story 7.1: OAuth Credential Access - error types
- Story 7.2: Usage API Client - stale data handling
- Story 7.4: App Model Wrapper - error handler location
- Story 7.5: Usage Bar Refresh - refresh state management

**No blocking dependencies on this story.**

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Implementation straightforward

### Completion Notes List

1. Audited existing `usageFetchedMsg` handler - found missing handling for `ErrInvalidCredentials` and `ErrEmptyToken`
2. Verified async fetch pattern - `fetchUsage()` correctly returns `tea.Cmd`, `Init()` uses `tea.Batch()`
3. Added `ErrInvalidCredentials` and `ErrEmptyToken` to the SetNotLoggedIn() branch in error handler
4. Added `log.Printf("usage fetch error: %v", msg.err)` for debugging (AC-6), logs go to stderr by default
5. Wrote comprehensive table-driven tests covering all 12 error scenarios
6. All tests pass, coverage maintained

**Code Review Fixes Applied (2026-01-20):**
7. Fixed AC-2 "Unknown" state - Changed error message from "Usage unavailable" to "Unknown" for first-time failures
8. Added `context.Canceled` handling (Task 3.3) - silently ignores cancelled context, preserves bar state
9. Added test `TestGracefulDegradation_ContextCanceled` verifying context.Canceled is handled gracefully
10. Added test `TestGracefulDegradation_StaleIndicatorPresent` verifying "(stale)" indicator appears in bar view

### File List

- `internal/tui/app.go` - Added `ErrInvalidCredentials`, `ErrEmptyToken`, `context.Canceled` handling; changed "Usage unavailable" to "Unknown"; added error logging
- `internal/tui/app_test.go` - Added 8 new test functions for Story 7.7 graceful degradation scenarios (2 added during code review)
