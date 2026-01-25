---
title: 'Fix Usage API Auth Errors After Extended Runtime'
slug: 'fix-usage-api-auth-errors'
created: '2026-01-24'
completed: '2026-01-26'
status: 'complete'
stepsCompleted: [1, 2, 3, 4]
tech_stack: ['Go 1.24', 'net/http', 'net']
files_to_modify: ['internal/usage/client.go', 'internal/tui/app.go']
code_patterns: ['constructor-chaining', 'table-driven-tests']
test_patterns: ['httptest.Server', 'mockTransport']
---

# Tech-Spec: Fix Usage API Auth Errors After Extended Runtime

**Created:** 2026-01-24
**Completed:** 2026-01-26

## Overview

### Problem Statement

After running cclv for extended periods (~12 hours), the usage API calls fail with "session expired" errors. The error message was unclear and caused floating error text in the UI.

### Initial Hypothesis (Incorrect)

Initially suspected stale HTTP connections in Go's default transport pool, since restarting the app seemed to fix the issue.

### Actual Root Cause

The OAuth token from Claude CLI expires after a period of time (~12 hours). The user had coincidentally run `claude` CLI around the same time as restarting cclv, which refreshed the token. The actual fix requires running `claude` CLI to get a new token.

### Solution

Two-part fix:
1. **HTTP Transport improvement** (kept as good practice) - Custom `http.Transport` with proper connection pool settings
2. **UX improvement** (actual fix) - Clean, actionable error message telling user to run `claude` to refresh

### Scope

**In Scope:**
- Modify `internal/usage/client.go` to use custom `http.Transport` (preventive)
- Modify `internal/tui/app.go` to show actionable error message
- Remove error logging spam for expected token expiration

**Out of Scope:**
- Automatic OAuth token refresh (researched, deemed too fragile - server-side session expires independently)

## Context for Development

### Research: Auto-Refresh Feasibility

Investigated automatic token refresh:

**Endpoint exists:**
```
POST https://console.anthropic.com/api/oauth/token
{
  "grant_type": "refresh_token",
  "refresh_token": "<token>",
  "client_id": "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
}
```

**Why not implemented:**
- Server-side session expires independently (can't be prevented by refresh)
- Anthropic may change client_id, endpoints without notice
- Token refresh only extends by ~1 day before server session expires anyway
- High maintenance burden for low reliability

**Conclusion:** Manual refresh via `claude` CLI is the correct approach.

### Files Modified

| File | Change |
| ---- | ------- |
| `internal/usage/client.go` | Custom `http.Transport` with connection pool settings |
| `internal/tui/app.go` | Actionable error message, selective logging |
| `internal/tui/app_test.go` | Updated test expectation |

## Implementation Plan

### Tasks

- [x] Task 1: Add custom `http.Transport` to `client.go` (preventive improvement)
- [x] Task 2: Change error message from "Session expired" to "Run 'claude' to refresh"
- [x] Task 3: Remove `log.Printf` for token expiration (expected behavior, not an error)
- [x] Task 4: Keep logging for truly unexpected errors only
- [x] Task 5: Update tests

### Acceptance Criteria

1. **AC-1: Actionable Error Message**
   - Given the OAuth token has expired
   - When the usage bar displays an error
   - Then it shows "Run 'claude' to refresh" (not generic "Session expired")

2. **AC-2: No Floating Error Spam**
   - Given the OAuth token has expired
   - When the app periodically tries to fetch usage
   - Then no error messages are logged to stderr (no floating text)

3. **AC-3: Unexpected Errors Still Logged**
   - Given a truly unexpected error occurs (not token expiry)
   - When the error is handled
   - Then it is logged for debugging

4. **AC-4: HTTP Transport Improved (Preventive)**
   - Given the usage client is created
   - When `NewClientWithTimeout()` is called
   - Then it creates a custom `http.Transport` with proper connection settings

## Implementation Details

### Error Message Change (`app.go`)

**Before:**
```go
} else if errors.Is(msg.err, usage.ErrTokenExpired) {
    m.usageBar.SetError("Session expired")
}
```

**After:**
```go
} else if errors.Is(msg.err, usage.ErrTokenExpired) {
    // Token expired - show actionable message (no log, expected behavior)
    m.usageBar.SetError("Run 'claude' to refresh")
}
```

### Selective Logging (`app.go`)

**Before:**
```go
if msg.err != nil {
    // Log for debugging (AC-6) - goes to stderr
    log.Printf("usage fetch error: %v", msg.err)
    // ... handle errors
}
```

**After:**
```go
if msg.err != nil {
    // Handle specific error types...
    } else {
        // No stale data available - log unexpected error and show "Unknown" (AC-2)
        log.Printf("usage fetch error: %v", msg.err)
        m.usageBar.SetError("Unknown")
    }
}
```

### HTTP Transport (`client.go`)

```go
func NewClientWithTimeout(timeout time.Duration) *Client {
    transport := &http.Transport{
        MaxIdleConns:        10,
        MaxIdleConnsPerHost: 2,
        IdleConnTimeout:     30 * time.Second,
        ForceAttemptHTTP2:   true,
        DialContext: (&net.Dialer{
            Timeout:   5 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,
    }
    return &Client{
        httpClient: &http.Client{
            Timeout:   timeout,
            Transport: transport,
        },
    }
}
```

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Investigation Timeline

1. **Initial diagnosis** (2026-01-24): Suspected HTTP connection pool exhaustion
2. **First fix applied**: Custom `http.Transport` with connection settings
3. **User testing** (~12 hours): Issue persisted
4. **Root cause identified** (2026-01-26): OAuth token expiration, not HTTP connections
5. **Research conducted**: Auto-refresh feasibility (deemed too fragile)
6. **Final fix applied**: Clean error messaging + keep Transport improvement

### Lessons Learned

- Don't assume correlation implies causation (restart + token refresh happened together)
- OAuth token expiration is expected behavior, not an error to log
- Auto-refresh for OAuth is fragile when you don't control the auth server
- Clear, actionable error messages are better than generic ones

### File List

- `internal/usage/client.go` - Custom HTTP Transport (preventive)
- `internal/tui/app.go` - Actionable error message, selective logging
- `internal/tui/app_test.go` - Updated test expectation

### References

- [OAuth Token Refresh Failure Issue #2633](https://github.com/anthropics/claude-code/issues/2633)
- [Token expiration handling needed Issue #12447](https://github.com/anthropics/claude-code/issues/12447)
