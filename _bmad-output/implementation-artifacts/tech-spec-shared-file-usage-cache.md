---
title: 'Shared File-Based Usage Cache Across cclv Instances'
slug: 'shared-file-usage-cache'
created: '2026-03-18'
status: 'completed'
stepsCompleted: [1, 2, 3, 4]
tech_stack: [Go 1.24.3]
files_to_modify:
  - internal/usage/client.go
  - internal/usage/client_test.go
code_patterns:
  - In-memory cache with cacheTTL=60s in Client struct (getCached/setCache)
  - lastGood stale fallback on error in FetchUsage
  - UsageLimits JSON round-trip works (ResetsAtRaw serialized, ResetsAt re-derived via UnmarshalJSON)
  - Atomic file write (temp + rename) for concurrency safety
test_patterns:
  - Table-driven tests with httptest.NewServer
  - mockTransport for HTTP interception
  - t.TempDir() for file-based test isolation
---

# Tech-Spec: Shared File-Based Usage Cache Across cclv Instances

**Created:** 2026-03-18

## Overview

### Problem Statement

The Anthropic usage API (`/api/oauth/usage`) has a rate limit of ~3-4 requests per short window per OAuth token. When multiple `cclv` TUI sessions run simultaneously, each polling every 60s, they collectively exceed this limit and get persistent 429 errors. The `--usage` CLI mode also creates a fresh client each time with no caching, compounding the problem.

### Solution

Add a file-based cache layer at `~/.cache/cclv/usage.json` that all `cclv` instances share. Before hitting the API, check if the cache file exists and is fresh (< 60s). If fresh, read from file. If stale, fetch from API and write back atomically. This ensures N concurrent instances generate at most ~1 API call per 60s window.

### Scope

**In Scope:**
- File-based cache at `~/.cache/cclv/usage.json` with 60s TTL
- Atomic writes (temp file + rename) to prevent corruption
- Integration into `Client.FetchUsage()` as a layer before in-memory cache
- `--usage` CLI mode benefiting from the shared cache

**Out of Scope:**
- File locking (atomic rename is sufficient for this use case)
- Cache invalidation UI/commands
- Changing the 60s poll interval
- Cross-user cache sharing

## Context for Development

### Codebase Patterns

**Cache flow in `Client.FetchUsage()`** (client.go:96-119):
```
FetchUsage(ctx, token)
  → getCached() — in-memory cache, 60s TTL
  → makeRequest(ctx, token) — HTTP call
  → on error: return lastGood (stale fallback)
  → on success: setCache(limits) — updates in-memory + lastGood
```

The file cache inserts between in-memory cache miss and API call:
```
FetchUsage(ctx, token)
  → getCached() — in-memory cache
  → readFileCache() — NEW: check ~/.cache/cclv/usage.json
  → makeRequest(ctx, token) — HTTP call
  → on success: setCache(limits) + writeFileCache(limits) — NEW
```

**JSON round-trip safety:** `UsageLimits` serializes correctly — `ResetsAtRaw` (string) is preserved in JSON, and `UnmarshalJSON` re-derives `ResetsAt` (*time.Time) from it on read. The `json:"-"` tag on `ResetsAt` means it's excluded from serialization but correctly reconstructed.

**`--usage` CLI mode** (main.go:342): Creates `usage.NewClient()` then calls `FetchUsage()`. Since file cache is in `FetchUsage()`, it benefits automatically — no changes needed in `cmd/cclv/`.

**`InvalidateCache()`**: Only clears in-memory cache. File cache is NOT deleted — the invalidating instance will make a fresh API call and update the file cache on success, benefiting other instances. See Notes section for UX tradeoff discussion.

### Files to Reference

| File | Purpose | Key Lines |
| ---- | ------- | --------- |
| `internal/usage/client.go` | API client with cache | L18-23 (constants), L64-94 (cache methods), L96-119 (FetchUsage) |
| `internal/usage/client_test.go` | Client tests | mockTransport, httptest pattern |
| `internal/usage/types.go` | UsageLimits, UsageWindow | L80-100 (JSON tags, UnmarshalJSON) |
| `cmd/cclv/main.go` | CLI `--usage` mode | L342-346 (NewClient + FetchUsage) |

### Technical Decisions

1. **Cache location**: `~/.cache/cclv/usage.json` — follows XDG conventions. Directory created with `os.MkdirAll` on first write.
2. **Same TTL as in-memory cache**: 60s, reusing `cacheTTL` constant.
3. **Atomic write**: Write to `usage.json.tmp` in same directory, then `os.Rename()`. Safety comes from the rename being atomic on the same filesystem — the `.tmp` file is always in the same directory as the target, guaranteeing same-filesystem rename.
4. **File cache struct**: Wraps `UsageLimits` with `FetchedAt` timestamp and a version field for forward compatibility:
   ```go
   type fileCacheEntry struct {
       Version   int          `json:"version"`
       FetchedAt time.Time    `json:"fetched_at"`
       Limits    *UsageLimits `json:"limits"`
   }
   ```
   `Version` is set to `1` on write. On read, if `Version != 1` (or missing/zero from old format), discard and re-fetch — prevents silent misinterpretation across upgrades.
5. **Graceful degradation**: File cache read/write errors are silently ignored — fall through to API call. File cache is an optimization, not a requirement.
6. **Configurable path for testing**: `Client` gets a `fileCachePath` field, defaulting to `~/.cache/cclv/usage.json`. Tests override it with `t.TempDir()`.
7. **Injectable clock for testing**: `Client` gets a `nowFunc func() time.Time` field, defaulting to `time.Now`. Tests override it to control TTL expiry deterministically without relying on wall-clock time.

## Implementation Plan

### Tasks

- [x] **Task 1: Add `fileCacheEntry` type and `fileCachePath` to Client**
  - File: `internal/usage/client.go`
  - Action: Add `fileCacheEntry` struct with `Version int`, `FetchedAt time.Time`, and `Limits *UsageLimits` fields, all with JSON tags. Add `fileCachePath string` and `nowFunc func() time.Time` fields to `Client` struct. In `NewClient()` and `NewClientWithTimeout()`, set default path to `~/.cache/cclv/usage.json` using `os.UserHomeDir()`, and set `nowFunc` to `time.Now`. Add `defaultFileCachePath()` helper that returns the path (empty string on `UserHomeDir` error — disables file cache gracefully).
  - Notes: Empty `fileCachePath` means file cache is disabled (e.g., if home dir unavailable). `nowFunc` enables deterministic TTL testing.

- [x] **Task 2: Implement `readFileCache()` method**
  - File: `internal/usage/client.go`
  - Action: Add `func (c *Client) readFileCache() *UsageLimits` method. Read `c.fileCachePath`, unmarshal into `fileCacheEntry`, check `entry.Version == 1` (discard if mismatch), check if `c.nowFunc().Sub(entry.FetchedAt) <= cacheTTL`. Return `entry.Limits` if fresh and valid, `nil` otherwise. Return `nil` on any error (file not found, parse error, version mismatch, etc.) — silent degradation.
  - Notes: Uses `c.nowFunc()` instead of `time.Now()` for testability. Uses same `cacheTTL` constant as in-memory cache.

- [x] **Task 3: Implement `writeFileCache()` method**
  - File: `internal/usage/client.go`
  - Action: Add `func (c *Client) writeFileCache(limits *UsageLimits)` method. Create `fileCacheEntry{Version: 1, FetchedAt: c.nowFunc(), Limits: limits}`, marshal to JSON, write to `c.fileCachePath + ".tmp"`, then `os.Rename()` to `c.fileCachePath`. Call `os.MkdirAll` on the parent directory before writing. Silently ignore all errors.
  - Notes: Atomic rename prevents partial reads. MkdirAll is idempotent for concurrent instances.

- [x] **Task 4: Integrate file cache into `FetchUsage()`**
  - File: `internal/usage/client.go`
  - Action: In `FetchUsage()`, after the in-memory cache check (line 100) and before `makeRequest()` (line 105), add: `if fileCached := c.readFileCache(); fileCached != nil { c.setCache(fileCached); return fileCached, false, nil }`. The second return value (`false`) is the `isBackoff` flag — file cache hits are not rate-limit backoffs. After successful API response and `setCache()` (line 119), add: `c.writeFileCache(limits)`.
  - Notes: File cache hit also populates in-memory cache via `setCache()`, so subsequent calls within the same process are fast. The `setCache` call also updates `lastGood`.

- [x] **Task 5: Add unit tests for `readFileCache` and `writeFileCache`**
  - File: `internal/usage/client_test.go`
  - Action: Add tests using `t.TempDir()` for isolation:
    1. **Write then read**: Write cache, read back, verify limits match and TTL is fresh.
    2. **Expired cache**: Use `nowFunc` override to return a time 2 minutes in the future, verify read returns nil.
    3. **Missing file**: Read from non-existent path returns nil (no error).
    4. **Corrupt file**: Write garbage to cache path, read returns nil (no error).
    5. **Missing directory**: Write to path with non-existent parent dir — creates directory and succeeds.
    6. **Atomic write**: Verify `.tmp` file doesn't persist after successful write.
    7. **Version mismatch**: Write a cache entry with `Version: 99`, verify read returns nil (discarded).
  - Notes: Override `c.fileCachePath` with temp dir path and `c.nowFunc` for deterministic TTL testing.

- [x] **Task 6: Add integration test for cross-instance cache sharing**
  - File: `internal/usage/client_test.go`
  - Action: Create two `Client` instances sharing the same `fileCachePath` (in temp dir). First client fetches (hits mock API), second client fetches (should read file cache, NOT hit API). Verify API was called exactly once.
  - Notes: This is the core value test — proves the file cache actually prevents redundant API calls.

- [x] **Task 6b: Add negative test for `InvalidateCache()` preserving file cache**
  - File: `internal/usage/client_test.go`
  - Action: Client fetches usage (populates file cache), then calls `InvalidateCache()`. Verify the file cache file still exists on disk and is readable. Then create a second client with the same `fileCachePath` — it should still read from the file cache without hitting the API.
  - Notes: Protects the intentional design decision that `InvalidateCache()` only clears in-memory state. Prevents future regressions where someone "fixes" InvalidateCache to also delete the file.

- [x] **Task 7: Run `make ci` and validate**
  - Action: Run `make ci` to ensure all checks pass: tests, race detection, coverage >= 90%, linting.

### Acceptance Criteria

- [ ] **AC-1**: Given two `cclv` instances sharing the same file cache path, when the first fetches usage successfully and the second fetches within 60s, then only one API call is made — the second reads from `~/.cache/cclv/usage.json`.

- [ ] **AC-2**: Given a `cclv` instance, when it fetches usage and the file cache is older than 60s, then a fresh API call is made and the file cache is updated.

- [ ] **AC-3**: Given `cclv --usage` is run from the CLI, when a fresh file cache exists from a TUI session, then the CLI reads the cached data without making an API call.

- [ ] **AC-4**: Given `cclv --usage` is run from the CLI with no existing cache, when the API call succeeds, then the file cache is created so subsequent TUI or CLI sessions can read it.

- [ ] **AC-5**: Given the file cache is missing, corrupt, or unreadable, when `cclv` fetches usage, then it falls through to the API call without errors — the file cache degrades gracefully.

- [ ] **AC-6**: Given two `cclv` instances write to the file cache simultaneously, when both complete, then the file is not corrupted — atomic write (temp + rename) ensures integrity.

- [ ] **AC-7**: Given `make ci` is run, then all tests pass, race detector is clean, and coverage is >= 90%.

## Additional Context

### Dependencies

- No new external dependencies. Uses `os`, `path/filepath`, `encoding/json` from stdlib.
- Depends on existing `cacheTTL` constant and `UsageLimits` JSON serialization.

### Testing Strategy

**Unit Tests:**
- `internal/usage/client_test.go`: `readFileCache` / `writeFileCache` isolation tests (fresh, expired, missing, corrupt, directory creation, atomic write)

**Integration Tests:**
- `internal/usage/client_test.go`: Two-client shared cache test proving single API call

**Verification:**
- `make ci` full validation (tests + race detection + coverage + lint)

### Notes

- **In-memory cache is NOT replaced** — file cache is an additional layer. In-memory is checked first (fastest), then file, then API.
- **`InvalidateCache()` (R key manual refresh)** clears both in-memory and file cache (deletes the file). This ensures the next `FetchUsage` call makes a fresh API request rather than re-reading stale file-cached data. Other instances will also make a fresh API call on their next poll since the file is gone.
- **Race between instances**: Two instances may both see stale file cache and both hit the API. This is acceptable — the rate limit allows ~3-4 requests, and the file cache prevents the sustained hammering that causes problems.
- **Future consideration**: If the 60s TTL proves too aggressive, it can be increased without code changes by modifying the `cacheTTL` constant (affects both in-memory and file cache).

## Review Notes
- Adversarial review completed
- Findings: 13 total, 6 fixed, 7 skipped (noise/not applicable)
- Resolution approach: auto-fix
- Fixed: F1 (ResetsAt round-trip test), F3 (unique temp files via CreateTemp), F4 (0700/0600 permissions), F6 (InvalidateCache deletes file cache), F10 (concurrent write test), F13 (temp file cleanup on failure)
- Skipped: F2 (Windows N/A), F5 (thundering herd acceptable), F7 (XDG user-dismissed), F8 (clock skew theoretical), F9 (race benign), F11/F12 (noise)
