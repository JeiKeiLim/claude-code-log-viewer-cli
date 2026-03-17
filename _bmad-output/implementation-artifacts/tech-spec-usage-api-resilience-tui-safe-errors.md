---
title: 'Usage API Rate-Limit Resilience & TUI-Safe Error Handling'
slug: 'usage-api-resilience-tui-safe-errors'
created: '2026-03-17'
status: 'implementation-complete'
stepsCompleted: [1, 2, 3, 4]
tech_stack: [Go 1.24.3, Bubbletea v1.3.10, Lipgloss v1.1.1, Bubbles v0.21.0]
files_to_modify:
  - internal/usage/types.go
  - internal/usage/client.go
  - internal/usage/client_test.go
  - internal/tui/app.go
  - internal/tui/app_test.go
  - internal/scanner/projects.go
  - internal/token/service.go
code_patterns:
  - Sentinel errors with errors.Is() routing (usage/types.go → tui/app.go)
  - Stale data fallback (client.lastGood returned on error)
  - tea.Cmd async fetch pattern (fetchUsage returns tea.Cmd)
  - ShowToastMsg app-level message (currently no-op at app.go:341-343)
  - ViewerModel.showToast() full toast with ID-based race prevention
  - sync.Once for one-time warnings (scanner/projects.go)
  - mockTransport for HTTP test interception
test_patterns:
  - Table-driven tests with httptest.NewServer
  - mockTransport struct for request interception in client_test.go
  - Direct model.Update(msg) testing for TUI message handling
  - Coverage gate 90% minimum (CI enforced)
---

# Tech-Spec: Usage API Rate-Limit Resilience & TUI-Safe Error Handling

**Created:** 2026-03-17

## Overview

### Problem Statement

When the Anthropic usage API returns a 429 (rate-limit) error, the TUI breaks because `log.Printf()` calls write directly to stderr, bypassing Bubbletea's terminal control. Additionally, 429 responses are treated as generic `ErrAPIError`, showing an unhelpful "Unknown" message in the usage bar with no awareness of the `Retry-After` header.

There are 6 `log.Printf`/`log.Println` calls across 3 files (`tui/app.go`, `scanner/projects.go`, `token/service.go`) that can fire during TUI runtime and corrupt the terminal display.

### Solution

1. Remove all `log.Printf` calls from code paths that execute during TUI runtime — the existing UI already communicates all user-relevant error states
2. Add `ErrRateLimited` sentinel error with `Retry-After` header parsing for HTTP 429 responses
3. Route `ErrRateLimited` in the TUI to show "Rate limited - retrying" in usage bar and schedule retry using the `Retry-After` duration

### Scope

**In Scope:**
- Remove 6 `log.Print*` calls across `tui/app.go` (4), `scanner/projects.go` (1), `token/service.go` (1)
- Add `ErrRateLimited` sentinel and `RateLimitError` type to `usage/types.go`
- Parse HTTP 429 + `Retry-After` header in `usage/client.go`
- Handle `ErrRateLimited` in TUI error routing in `tui/app.go`
- Tests for all changes

**Out of Scope:**
- Exponential backoff (existing stale data fallback + 60s cache is sufficient)
- Circuit breaker pattern
- File-based debug logging
- App-level toast UI implementation (ShowToastMsg is currently a no-op, separate story)
- Changes to usage bar rendering (bar already handles error states correctly)

## Context for Development

### Codebase Patterns

**Error Flow:**
```
fetchUsage() tea.Cmd
  → usage.GetOAuthToken()       → credential errors (ErrNoCredentials, etc.)
  → usageClient.FetchUsage()    → API errors (ErrAPIError, ErrAPITimeout, ErrTokenExpired)
  → returns usageFetchedMsg{limits, stale, err}
  → AppModel.Update() routes error via errors.Is() to usage bar states
```

**Sentinel Error Pattern** (`usage/types.go`):
- All error types are package-level `var` with `errors.New()`
- Wrapped errors use `fmt.Errorf("%w: detail", ErrSentinel)` pattern
- TUI routes via `errors.Is(msg.err, usage.ErrXxx)`

**Stale Data Fallback** (`usage/client.go`):
- `Client.lastGood` preserves last successful response
- On error: returns `(lastGood, stale=true, error)` if lastGood exists
- On error with no lastGood: returns `(nil, false, error)`
- TUI shows stale data with "(stale)" indicator, or "Unknown" if no data

**Log Call Inventory** (all 6 in codebase):

| Location | When | TUI Risk | Replacement |
|----------|------|----------|-------------|
| `tui/app.go:99` | `NewAppModel()` init | Low (before TUI) | Remove — usageBar shows "Not logged in" downstream |
| `tui/app.go:122` | `NewAppModelWithError()` init | Low (before TUI) | Remove — same reason |
| `tui/app.go:396` | Usage retry error (runtime) | **HIGH** | Remove — bar shows "Run 'claude' to refresh", silent retry continues |
| `tui/app.go:403` | Unknown fetch error (runtime) | **HIGH** | Remove — bar already shows "Unknown" |
| `scanner/projects.go:305` | Birthtime unavailable (runtime) | **HIGH** | Remove — sync.Once debug hint, mtime fallback is seamless, fires during loading screen |
| `token/service.go:100` | ToolInput marshal fail (runtime) | **HIGH** | Remove log, keep `continue` — non-critical edge case |

### Files to Reference

| File | Purpose | Key Lines |
| ---- | ------- | --------- |
| `internal/usage/types.go` | Sentinel errors, data types | Lines 11-19 (error vars) |
| `internal/usage/client.go` | API client, makeRequest() | Lines 122-171 (HTTP handling, 429 gap at line 155) |
| `internal/usage/client_test.go` | Client tests, mockTransport | Table-driven tests, httptest pattern |
| `internal/tui/app.go` | Error routing in Update() | Lines 364-417 (usageFetchedMsg handler) |
| `internal/tui/app_test.go` | TUI message handling tests | usageFetchedMsg test cases |
| `internal/scanner/projects.go` | Birthtime log | Line 305 (sync.Once log.Println) |
| `internal/token/service.go` | Token marshal log | Line 100 (log.Printf in CalculateTokens) |

### Technical Decisions

1. **Silent removal for all `log.Printf`**: Every log call is either redundant (usage bar already shows error state) or non-critical debug info. No toast replacement needed — the existing UI already communicates all user-relevant errors through the usage bar.

2. **`RateLimitError` type with `RetryAfter`**: Instead of just a sentinel, use a typed error that carries the parsed `Retry-After` duration. This lets the TUI schedule the next refresh at the right time.
   ```go
   type RateLimitError struct {
       RetryAfter time.Duration
   }
   func (e *RateLimitError) Error() string { ... }
   func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }
   ```

3. **`Retry-After` parsing**: HTTP spec allows both seconds (integer) and HTTP-date format. Parse seconds first (most common for APIs), fall back to `defaultRetryAfter` constant (60s) if header is missing or unparseable. The constant should be defined alongside `cacheTTL` and `apiTimeout` in `client.go` for consistency.

4. **TUI routing for rate limit**: Show stale data if available (with "(stale)"), or "Rate limited" in bar error state. Schedule next refresh using `RetryAfter` duration instead of fixed 60s interval.

5. **No exponential backoff**: The 60s cache TTL + stale fallback already prevents hammering. Adding `Retry-After` awareness is sufficient.

## Implementation Plan

### Tasks

- [x] **Task 1: Add `ErrRateLimited` sentinel and `RateLimitError` type**
  - File: `internal/usage/types.go`
  - Action: Add sentinel error `ErrRateLimited` alongside existing error vars (line ~19). Add `RateLimitError` struct with `RetryAfter time.Duration` field, `Error() string` method, and `Is(target error) bool` method that matches `ErrRateLimited`.
  - Notes: Follow existing sentinel pattern. The `Is()` method enables `errors.Is(err, ErrRateLimited)` to work with the typed error.
  - Example:
    ```go
    ErrRateLimited = errors.New("usage API rate limited")

    type RateLimitError struct {
        RetryAfter time.Duration
    }

    func (e *RateLimitError) Error() string {
        return fmt.Sprintf("%s (retry after %s)", ErrRateLimited.Error(), e.RetryAfter)
    }

    func (e *RateLimitError) Is(target error) bool {
        return target == ErrRateLimited
    }
    ```

- [x] **Task 2: Handle HTTP 429 with `Retry-After` parsing in `makeRequest()`**
  - File: `internal/usage/client.go`
  - Action: In `makeRequest()`, add a 429 check **before** the existing generic `>= 400` check (line 155). On 429: read `Retry-After` header, parse as seconds (integer), fall back to 60s default if missing/unparseable. Return `&RateLimitError{RetryAfter: duration}`.
  - Notes: Must be inserted before `if resp.StatusCode >= 400` so 429 isn't caught by the generic handler. Add helper `parseRetryAfter(header string) time.Duration`.
  - Example insertion point (after line 154, before line 155):
    ```go
    if resp.StatusCode == http.StatusTooManyRequests {
        retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
        return nil, &RateLimitError{RetryAfter: retryAfter}
    }
    ```
  - Add named constant alongside existing timeout constants (line ~17):
    ```go
    defaultRetryAfter = 60 * time.Second
    ```
  - Helper function:
    ```go
    func parseRetryAfter(header string) time.Duration {
        if header == "" {
            return defaultRetryAfter
        }
        seconds, err := strconv.Atoi(header)
        if err != nil || seconds <= 0 {
            return defaultRetryAfter
        }
        return time.Duration(seconds) * time.Second
    }
    ```

- [x] **Task 3: Add `rateLimitRetryMsg` and handle `ErrRateLimited` in TUI**
  - File: `internal/tui/app.go`
  - Action:
    1. Add a new message type `rateLimitRetryMsg struct{}` alongside existing `usageTickMsg` and `authRetryTickMsg`. Add a grouping comment block above the three retry/tick message types to clarify they are intentionally separate mechanisms (periodic refresh, auth recovery, rate-limit backoff).
    2. Add a `scheduleRateLimitRetry(delay time.Duration) tea.Cmd` function that uses `tea.Tick(delay, ...)` to return `rateLimitRetryMsg`.
    3. In the `usageFetchedMsg` error handler (around line 375), add a new case **before** the generic stale/unknown handling:
       ```go
       } else if errors.Is(msg.err, usage.ErrRateLimited) {
           // Show stale data if available, or "Rate limited" error
           var rateLimitErr *usage.RateLimitError
           if errors.As(msg.err, &rateLimitErr) {
               if msg.limits != nil {
                   m.usageBar.SetLimits(msg.limits, true)
               } else {
                   m.usageBar.SetError("Rate limited - retrying")
               }
               return m, scheduleRateLimitRetry(rateLimitErr.RetryAfter)
           }
       }
       ```
    4. Add handler for `rateLimitRetryMsg` in the Update switch (alongside `usageTickMsg` and `authRetryTickMsg`):
       ```go
       case rateLimitRetryMsg:
           if !m.refreshInProgress {
               m.refreshInProgress = true
               return m, m.fetchUsage()
           }
           return m, nil
       ```
  - Notes: Insert the `ErrRateLimited` check before the `m.authExpired` check (line 394) so rate limits during auth retry are handled correctly.

- [x] **Task 4: Remove all `log.Printf` calls from TUI runtime paths**
  - File: `internal/tui/app.go`
  - Action: Remove 4 `log.Printf` calls:
    1. **Line 99**: Remove `log.Printf("Warning: token service initialization failed: %v", err)` — usage bar shows "Not logged in" downstream
    2. **Line 122**: Remove `log.Printf("Warning: token service initialization failed: %v", tokenErr)` — same reason
    3. **Line 396**: Remove `log.Printf("usage retry error (continuing): %v", msg.err)` — bar shows "Run 'claude' to refresh", retry continues silently
    4. **Line 403**: Remove `log.Printf("usage fetch error: %v", msg.err)` — bar already shows "Unknown"
  - Notes: After removing all log calls, verify no other `log.` usage remains in the file, then remove the `"log"` import from the import block.

- [x] **Task 5: Remove `log.Println` from scanner birthtime fallback**
  - File: `internal/scanner/projects.go`
  - Action: Remove `log.Println(...)` call at line 305. Keep the `birthtimeWarningOnce.Do()` wrapper but make it a no-op, or remove the entire `birthtimeWarningOnce.Do(func(){...})` block since it only wrapped the log call.
  - Notes: Before removing imports, explicitly verify: (1) `birthtimeWarningOnce` is the only `sync.Once` usage — grep for `sync.` in the file, (2) `log` has no other callers. Also remove `birthtimeWarningOnce` var declaration. The compiler will catch missed imports, but verifying first prevents wasted cycles.

- [x] **Task 6: Remove `log.Printf` from token service**
  - File: `internal/token/service.go`
  - Action: Remove `log.Printf("Warning: failed to marshal ToolInput for tokenization: %v", err)` at line 100. Keep the `continue` statement — the token count simply skips this content block.
  - Notes: Remove `"log"` import if no other usage remains.

- [x] **Task 7: Add tests for 429 handling and `RateLimitError`**
  - File: `internal/usage/client_test.go`
  - Action: Add test cases to the existing `TestFetchUsage` table:
    1. **429 without Retry-After**: Server returns 429, no header → error is `ErrRateLimited`, `RetryAfter` is 60s default
    2. **429 with Retry-After: 120**: Server returns 429 + header → error is `ErrRateLimited`, `RetryAfter` is 120s
    3. **429 with invalid Retry-After**: Server returns 429 + `"abc"` header → error is `ErrRateLimited`, `RetryAfter` is 60s default
    4. **429 with stale data**: Populate `lastGood` first, then 429 → returns stale data + `ErrRateLimited`
  - Also add unit test for `parseRetryAfter()` helper:
    - Empty string → 60s
    - "30" → 30s
    - "0" → 60s (intentionally treated as invalid — we don't want to retry immediately even though HTTP spec allows it)
    - "-5" → 60s (invalid)
    - "abc" → 60s (invalid)
  - Also add unit test for `RateLimitError`:
    - `errors.Is(&RateLimitError{...}, ErrRateLimited)` returns true
    - `Error()` string contains retry duration

- [x] **Task 8: Add tests for `ErrRateLimited` TUI routing**
  - File: `internal/tui/app_test.go`
  - Action: Add test cases following existing `usageFetchedMsg` test patterns:
    1. **Rate limited with stale data**: Send `usageFetchedMsg{limits: staleData, stale: true, err: &RateLimitError{RetryAfter: 120s}}` → bar shows stale data, returns `scheduleRateLimitRetry` cmd
    2. **Rate limited without stale data**: Send `usageFetchedMsg{err: &RateLimitError{RetryAfter: 60s}}` → bar shows "Rate limited - retrying", returns `scheduleRateLimitRetry` cmd
    3. **Rate limit retry triggers fetch**: Send `rateLimitRetryMsg{}` when not refreshing → sets `refreshInProgress`, returns fetch cmd
    4. **Rate limit retry no-op when refreshing**: Send `rateLimitRetryMsg{}` when `refreshInProgress=true` → returns nil cmd
    5. **usageTickMsg during rate-limit backoff**: Set `refreshInProgress=true` (simulating active rate-limit retry), then send `usageTickMsg{}` → tick is skipped (not refreshing), reschedules tick. This locks in the correct interaction between the two mechanisms.

- [x] **Task 9: Verify no `log.Print*` calls remain in TUI-runtime paths**
  - Action: Run `grep -rn "log\.\(Print\|Fatal\|Panic\)" internal/` and confirm zero results, or only results in test files.
  - Notes: This is a verification step, not a code change. Run as part of the validation.

- [x] **Task 10: Run `make ci` and validate**
  - Action: Run `make ci` to ensure all checks pass: tests, race detection, coverage >= 90%, linting.
  - Notes: If coverage drops, add additional tests as needed.

### Acceptance Criteria

- [x] **AC-1**: Given a TUI is running, when the usage API returns HTTP 429, then the usage bar shows stale data with "(stale)" indicator (if previous data exists) or "Rate limited - retrying" text (if no previous data), and no stderr output corrupts the terminal display.

- [x] **AC-2**: Given a TUI is running, when the usage API returns HTTP 429 with `Retry-After: 120` header, then the next usage refresh is scheduled in 120 seconds (not the default 60s).

- [x] **AC-3**: Given a TUI is running, when the usage API returns HTTP 429 without a `Retry-After` header, then the next usage refresh is scheduled in 60 seconds (default fallback).

- [x] **AC-4**: Given a TUI is running, when any `log.Printf` was previously called during runtime (usage errors, birthtime fallback, token marshal), then no output appears on stderr — the TUI display remains clean.

- [x] **AC-5**: Given the codebase, when searching for `log.Print*` in `internal/` (excluding test files), then zero matches are found — all runtime logging has been removed.

- [x] **AC-6**: Given `errors.Is(err, usage.ErrRateLimited)` is called with a `*RateLimitError`, then it returns `true`, and `errors.As(err, &rateLimitErr)` extracts the `RetryAfter` duration.

- [x] **AC-7**: Given `make ci` is run, then all tests pass, race detector is clean, and coverage is >= 90%.

## Additional Context

### Dependencies

- No new external dependencies. All changes use standard library (`net/http`, `strconv`, `time`, `errors`) and existing Charm stack.
- Depends on existing `ErrAPIError` sentinel pattern in `usage/types.go`.
- Depends on existing `usageFetchedMsg` routing in `tui/app.go`.

### Testing Strategy

**Unit Tests:**
- `internal/usage/client_test.go`: 429 handling (with/without `Retry-After`, with stale data), `parseRetryAfter()` helper, `RateLimitError` type behavior
- `internal/tui/app_test.go`: `ErrRateLimited` routing (stale vs no data), `rateLimitRetryMsg` handling

**Integration/Verification:**
- `grep` scan for `log.Print*` confirms zero runtime log calls remain
- `make ci` full validation (tests + race detection + coverage + lint)

**Manual Testing:**
- Run `cclv` in TUI mode, verify usage bar displays normally
- If possible, simulate 429 by temporarily modifying test to confirm bar shows "Rate limited" (or verify via unit tests)

### Notes

- **Risk: `"log"` import removal** — After removing all `log.Printf` calls from a file, the `"log"` import must also be removed or Go won't compile. Check each file for remaining `log` usage before removing the import.
- **Risk: `sync` import in scanner** — `scanner/projects.go` uses `sync.Once` for the birthtime warning. If the `sync.Once` block is removed entirely, check whether `sync` is still needed for other uses in the file.
- **Future consideration**: The `ShowToastMsg` app-level toast is currently a no-op (TODO at `app.go:341-343`). Implementing app-level toast UI would enable richer error communication across all views, but is a separate story.
- **Future consideration**: If Anthropic's usage API starts returning 429 frequently, exponential backoff could be added to the client. The `RateLimitError` type already provides the foundation — a future enhancement would track `consecutiveErrors` on the `Client` struct and multiply the `RetryAfter` duration.
