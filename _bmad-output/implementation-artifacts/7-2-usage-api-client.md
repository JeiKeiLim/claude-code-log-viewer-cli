# Story 7.2: Usage API Client

Status: done

## Story

As a **developer building usage features**,
I want **a client to fetch usage limits from the OAuth endpoint**,
So that **I can display current utilization and reset times**.

## Acceptance Criteria

1. **AC-1: Fetch Usage with Valid Credentials**
   - Given valid OAuth credentials
   - When FetchUsage() is called
   - Then it calls `GET https://api.anthropic.com/api/oauth/usage`
   - And returns parsed UsageLimits struct

2. **AC-2: Parse API Response**
   - Given the API response is received
   - When parsed
   - Then it extracts `five_hour.utilization`, `five_hour.resets_at`
   - And extracts `seven_day.utilization`, `seven_day.resets_at`

3. **AC-3: Cache Results**
   - Given a successful API call
   - When FetchUsage() is called again within 60 seconds
   - Then cached result is returned (no API call)

4. **AC-4: Handle Errors Gracefully**
   - Given API returns error or times out
   - When FetchUsage() is called
   - Then error is returned without crashing
   - And last known good values are preserved

5. **AC-5: 401 Unauthorized Handling**
   - Given API returns 401 status
   - When FetchUsage() is called
   - Then `ErrTokenExpired` is returned (reuse from types.go)

6. **AC-6: Timeout Handling**
   - Given API does not respond within 5 seconds
   - When FetchUsage() is called
   - Then `ErrAPITimeout` is returned
   - And context cancellation is respected

## Tasks / Subtasks

- [x] Task 1: Add Client types to types.go (AC: #1, #2)
  - [x] Subtask 1.1: Add `UsageLimits` struct with `FiveHour` and `SevenDay` fields
  - [x] Subtask 1.2: Add `UsageWindow` struct with `Utilization` (float64) and `ResetsAt` (*time.Time)
  - [x] Subtask 1.3: Add new sentinel errors: `ErrAPITimeout`, `ErrAPIError`
  - [x] Subtask 1.4: Write unit tests for types JSON unmarshaling

- [x] Task 2: Create client.go with Client struct (AC: #1, #3)
  - [x] Subtask 2.1: Define `Client` struct with `http.Client`, `cache`, `cacheLock`, `lastGood` fields
  - [x] Subtask 2.2: Implement `NewClient()` constructor with configurable timeout (default 5s)
  - [x] Subtask 2.3: Add `cacheTTL` constant (60 seconds)
  - [x] Subtask 2.4: Implement cache check logic in private `getCached()` method

- [x] Task 3: Implement FetchUsage() core logic (AC: #1, #2, #4)
  - [x] Subtask 3.1: Create HTTP request with required headers (Authorization, anthropic-beta, User-Agent)
  - [x] Subtask 3.2: Parse JSON response into UsageLimits struct
  - [x] Subtask 3.3: Handle `resets_at` string-to-time.Time conversion (RFC3339)
  - [x] Subtask 3.4: Store successful response in cache with timestamp
  - [x] Subtask 3.5: Store successful response in `lastGood` for stale fallback
  - [x] Subtask 3.6: Write table-driven tests for response parsing

- [x] Task 4: Implement error handling (AC: #4, #5, #6)
  - [x] Subtask 4.1: Handle HTTP status 401 -> return `ErrTokenExpired`
  - [x] Subtask 4.2: Handle HTTP status 4xx/5xx -> return `ErrAPIError` with wrapped status
  - [x] Subtask 4.3: Handle context timeout -> return `ErrAPITimeout`
  - [x] Subtask 4.4: Return `lastGood` values (if available) on error for graceful degradation
  - [x] Subtask 4.5: Write table-driven tests for all error paths

- [x] Task 5: Implement caching logic (AC: #3)
  - [x] Subtask 5.1: Use `sync.RWMutex` for thread-safe cache access
  - [x] Subtask 5.2: Check cache age before making API call
  - [x] Subtask 5.3: Implement `InvalidateCache()` for manual refresh (Story 7.5 prep)
  - [x] Subtask 5.4: Write tests for cache hit/miss scenarios
  - [x] Subtask 5.5: Write concurrent access test with race detector (multiple goroutines reading/writing simultaneously)

- [x] Task 6: Integration with credentials (AC: #1)
  - [x] Subtask 6.1: Accept token as parameter to `FetchUsage(ctx, token string)`
  - [x] Subtask 6.2: Do NOT call `GetOAuthToken()` internally - caller provides token (see Anti-Patterns section for rationale)
  - [x] Subtask 6.3: Write integration test that uses `GetOAuthToken()` + `FetchUsage()` together

## Dev Notes

### API Endpoint Details

**Endpoint:**
```
GET https://api.anthropic.com/api/oauth/usage
```

**Required Headers:**
| Header | Value | Notes |
|--------|-------|-------|
| `Authorization` | `Bearer {token}` | OAuth token from Story 7.1 |
| `anthropic-beta` | `oauth-2025-04-20` | Required beta flag |
| `User-Agent` | `claude-code/cclv-{version}` | Use version from internal/version |
| `Accept` | `application/json` | Standard |
| `Content-Type` | `application/json` | Standard |

**Example Response:**
```json
{
  "five_hour": {
    "utilization": 35.0,
    "resets_at": "2026-01-20T18:00:00Z"
  },
  "seven_day": {
    "utilization": 12.0,
    "resets_at": "2026-01-27T00:00:00Z"
  },
  "seven_day_opus": {
    "utilization": 0.0,
    "resets_at": null
  },
  "seven_day_oauth_apps": null,
  "iguana_necktie": null
}
```

**Note:** The API may include additional fields like `seven_day_oauth_apps` and `iguana_necktie` (both typically null). These should be ignored for forward compatibility - use `omitempty` tags and don't fail on unknown fields.

### Type Definitions (Add to types.go)

```go
// UsageLimits represents the API response from /api/oauth/usage.
type UsageLimits struct {
    FiveHour *UsageWindow `json:"five_hour"`
    SevenDay *UsageWindow `json:"seven_day"`
    // Opus-specific limits (optional)
    SevenDayOpus *UsageWindow `json:"seven_day_opus,omitempty"`
}

// UsageWindow represents a single usage window (5-hour or 7-day).
type UsageWindow struct {
    Utilization float64    `json:"utilization"` // 0-100 percentage
    ResetsAt    *time.Time `json:"-"`           // Parsed from string
    ResetsAtRaw *string    `json:"resets_at"`   // Raw API response
}

// Custom UnmarshalJSON for UsageWindow to handle resets_at string parsing
func (w *UsageWindow) UnmarshalJSON(data []byte) error {
    type alias UsageWindow
    aux := &struct {
        ResetsAtRaw *string `json:"resets_at"`
        *alias
    }{
        alias: (*alias)(w),
    }
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }
    if aux.ResetsAtRaw != nil && *aux.ResetsAtRaw != "" {
        t, err := time.Parse(time.RFC3339, *aux.ResetsAtRaw)
        if err == nil {
            w.ResetsAt = &t
        }
    }
    return nil
}
```

### New Sentinel Errors (Add to types.go)

```go
var (
    // ... existing errors from Story 7.1

    ErrAPITimeout = errors.New("usage API request timed out")
    ErrAPIError   = errors.New("usage API returned an error")
)
```

### Client Structure (client.go)

```go
package usage

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version"
)

const (
    usageAPIURL = "https://api.anthropic.com/api/oauth/usage"
    cacheTTL    = 60 * time.Second
    apiTimeout  = 5 * time.Second
)

// Client fetches usage limits from the Claude API.
type Client struct {
    httpClient *http.Client

    cache      *UsageLimits
    cacheTime  time.Time
    cacheLock  sync.RWMutex

    lastGood   *UsageLimits  // Preserved for graceful degradation
}

// NewClient creates a new usage API client.
func NewClient() *Client {
    return &Client{
        httpClient: &http.Client{Timeout: apiTimeout},
    }
}

// FetchUsage retrieves usage limits, using cache if available.
// Returns (limits, stale, error) where stale=true means returned from lastGood.
func (c *Client) FetchUsage(ctx context.Context, token string) (*UsageLimits, bool, error) {
    // 1. Check cache first
    // 2. Make API request if cache miss
    // 3. On error, return lastGood if available
    // 4. Update cache and lastGood on success
}
```

### HTTP Request Construction

```go
func (c *Client) makeRequest(ctx context.Context, token string) (*UsageLimits, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageAPIURL, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("anthropic-beta", "oauth-2025-04-20")
    req.Header.Set("User-Agent", "claude-code/cclv-"+version.Version)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    // ... handle response
}
```

### Project Structure Notes

```
internal/usage/
├── types.go           # MODIFY: Add UsageLimits, UsageWindow, new errors
├── credentials.go     # UNCHANGED (from Story 7.1)
├── credentials_test.go # UNCHANGED
├── credentials_darwin_test.go # UNCHANGED
├── client.go          # NEW: Client struct, FetchUsage(), caching
└── client_test.go     # NEW: Table-driven tests
```

### Critical Implementation Rules

1. **NO calls to GetOAuthToken() inside FetchUsage()** - Token is passed as parameter
2. **Context propagation** - All HTTP calls use provided context for cancellation
3. **Thread-safe caching** - Use `sync.RWMutex` for concurrent access
4. **Graceful degradation** - Return `lastGood` on error if available
5. **Return signature** - `(limits *UsageLimits, stale bool, err error)` for UI to show stale indicator

### Testing Requirements

**Unit Tests (client_test.go):**
- Table-driven tests for all response parsing scenarios
- Mock HTTP responses using `httptest.Server`
- Cache hit/miss verification
- Error handling for all HTTP status codes
- Timeout handling with context cancellation
- Concurrent access test (run with `go test -race`)
  - Spawn 10+ goroutines calling `FetchUsage()` simultaneously
  - Verify no data races with `-race` flag
  - Verify cache is populated correctly under contention

**Test Patterns:**
```go
func TestFetchUsage(t *testing.T) {
    tests := []struct {
        name        string
        response    string
        statusCode  int
        wantErr     error
        wantStale   bool
        wantFiveHr  float64
    }{
        // ... test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Use httptest.Server to mock API
        })
    }
}
```

### Previous Story Learnings (Story 7.1)

From Story 7.1 implementation:
- Used testable command executor pattern for Keychain - **apply similar pattern for HTTP client**
- Sentinel errors defined in types.go - **add new errors there, not in client.go**
- Table-driven tests with edge cases - **continue this pattern**
- 95.6% coverage achieved - **maintain 90%+ coverage**
- Platform-specific build tags used - **no platform-specific code needed for client**

### Anti-Patterns to Avoid

1. **DO NOT** call `GetOAuthToken()` inside client - caller provides token
   - **Reason:** Separation of concerns. Credential retrieval is a separate operation that may have platform-specific errors (Keychain timeout, file not found). The caller handles credential errors distinctly from API errors.
   - **Example:**
     ```go
     // WRONG - mixes credential and API concerns
     func (c *Client) FetchUsage(ctx context.Context) (*UsageLimits, error) {
         token, err := GetOAuthToken()  // BAD
         // ...
     }

     // CORRECT - caller provides token
     func (c *Client) FetchUsage(ctx context.Context, token string) (*UsageLimits, bool, error)
     ```
2. **DO NOT** block on API call - always respect context timeout
3. **DO NOT** panic on parse errors - return error
4. **DO NOT** modify cache without lock - use `sync.RWMutex`
5. **DO NOT** ignore lastGood on error - return it for graceful degradation
6. **DO NOT** hardcode version - use `internal/version.Version`
7. **DO NOT** skip error wrapping - always wrap with context
8. **DO NOT** fail on unknown JSON fields - use struct tags that tolerate extras for forward compatibility

### Expected Commit Format

```
feat: add usage API client with caching (Story 7.2)

Implements HTTP client for Claude OAuth usage endpoint:
- FetchUsage() with 60-second cache TTL
- Thread-safe caching with sync.RWMutex
- Graceful degradation with lastGood fallback
- Proper error handling (401, timeout, etc.)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4.md#Story-7.2]
- [Source: _bmad-output/planning-artifacts/research/technical-claude-code-usage-limits-research-2026-01-20.md#3-OAuth-Usage-API-Discovery]
- [Source: _bmad-output/project-context.md] - Critical rules and patterns
- [Source: internal/usage/types.go] - Existing types from Story 7.1
- [Source: internal/usage/credentials.go] - Reference for testable patterns
- [Source: https://codelynx.dev/posts/claude-code-usage-limits-statusline] - API endpoint details

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

N/A

### Completion Notes List

- All 6 tasks completed with all subtasks
- 94.6% test coverage on usage package (exceeds 90% requirement)
- Concurrent access test passes with race detector
- Table-driven tests for all scenarios
- FetchUsage accepts token as parameter (separation of concerns)
- InvalidateCache() implemented for Story 7.5 prep
- Custom UnmarshalJSON for time parsing
- Forward-compatible: unknown JSON fields ignored

### Code Review Fixes Applied (2026-01-20)

- **H1 Fixed**: Added `isTimeoutError()` helper for robust HTTP client timeout detection
- **H2 Fixed**: `TestFetchUsage_Timeout` now verifies `ErrAPITimeout` sentinel error
- **H3 Fixed**: `TestFetchUsage_ContextDeadlineExceeded` now verifies `ErrAPITimeout` sentinel error
- **M2 Fixed**: Added `TestFetchUsage_EmptyToken` test for empty token edge case
- **L2 Fixed**: Removed dead code (unused variable warning suppression)
- Coverage maintained at 90.6%

### File List

- `internal/usage/types.go` - Added UsageLimits, UsageWindow structs, ErrAPITimeout, ErrAPIError
- `internal/usage/types_test.go` - NEW: Unit tests for types JSON unmarshaling
- `internal/usage/client.go` - NEW: Client struct with FetchUsage, caching, graceful degradation; added isTimeoutError() helper
- `internal/usage/client_test.go` - NEW: Comprehensive tests including concurrent access; added ErrAPITimeout assertions and empty token test
