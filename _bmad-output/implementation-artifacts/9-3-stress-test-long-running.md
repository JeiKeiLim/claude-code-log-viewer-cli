# Story 9.3: Add Stress Test for Long-Running Dashboard

Status: done

## Story

As a **developer preventing regressions**,
I want **automated tests that verify constant resource usage**,
So that **file descriptor leaks are caught before production**.

## Acceptance Criteria

1. **AC-1: Repeated Watch Cycle Test**
   - Given a test that calls `New()` and `Close()` 100+ times
   - When all cycles complete
   - Then final watcher is still functional
   - And no "too many open files" errors occur

2. **AC-2: Event Storm Test**
   - Given a dashboard with 2 panes
   - When 100 file events are triggered rapidly
   - Then goroutine count remains bounded (baseline + 10 max)

3. **AC-3: FD Count Stability Test**
   - Given dashboard watch mode running
   - When FD count is measured before and after 50 events
   - Then FD count delta is ≤ 5

## Tasks / Subtasks

- [x] Task 1: Add stress test for repeated watcher New()/Close() cycles (AC: #1)
  - [x] 1.1: Create `TestStressRepeatedWatchCycles` in `internal/watcher/watcher_test.go`
  - [x] 1.2: Guard with `STRESS_TESTS` environment variable check (`t.Skip` if not set)
  - [x] 1.3: Create temp directory with test file for watcher
  - [x] 1.4: Loop 100 times: `New(testFile)` → verify no error → `Close()`
  - [x] 1.5: After loop: verify final watcher creation succeeds (no "too many open files")
  - [x] 1.6: Verify final watcher can receive events (append to file, check via `EventsChan()`)

- [x] Task 2: Add event storm test for dashboard subscription model (AC: #2)
  - [x] 2.1: Create `TestStressEventStormBoundedGoroutines` in `internal/tui/dashboard_test.go`
  - [x] 2.2: Guard with `STRESS_TESTS` environment variable check
  - [x] 2.3: Record baseline goroutine count with `runtime.NumGoroutine()`
  - [x] 2.4: Create dashboard model with 2 projects (minimal setup to avoid watchers)
  - [x] 2.5: Initialize fileEventChan for each pane (buffered channel, capacity 10)
  - [x] 2.6: Send 100 events rapidly through fileEventChan (20 events total - 10 per channel)
  - [x] 2.7: Process each event via `Update(subscriptionTickMsg{})` loop
  - [x] 2.8: Call `runtime.GC()` + `time.Sleep(100ms)` to allow goroutine cleanup
  - [x] 2.9: Measure final goroutine count, assert delta ≤ 10

- [x] Task 3: Add FD count stability test (AC: #3)
  - [x] 3.1: Create `TestStressFDCountStability` in `internal/watcher/watcher_test.go`
  - [x] 3.2: Guard with `STRESS_TESTS` environment variable check
  - [x] 3.3: Create `countOpenFDs(t *testing.T) int` helper in test file that:
    - First tries `/proc/self/fd` (Linux)
    - Falls back to `/dev/fd` (macOS)
    - Falls back to `lsof` command (sandboxed environments)
    - Skips test if none available
  - [x] 3.4: Record baseline FD count
  - [x] 3.5: Create watcher with `New(testFile)`
  - [x] 3.6: Generate 50 file write events (append to file in loop)
  - [x] 3.7: For each event: read from `EventsChan()` + call `ReadNewEntries()`
  - [x] 3.8: Close watcher
  - [x] 3.9: Record final FD count, assert delta ≤ 5
  - [x] 3.10: Log actual delta for debugging: `t.Logf("FD delta: %d", delta)`

- [x] Task 4: Add stress test Makefile target (AC: #1, #2, #3)
  - [x] 4.1: Add `test-stress` target to Makefile after `test-short` target
  - [x] 4.2: Target command: `STRESS_TESTS=1 $(GOTEST) -v -race -count=1 -timeout 5m ./... -run Stress`
  - [x] 4.3: Add descriptive echo: `@echo "Running stress tests..."`
  - [x] 4.4: Add to `.PHONY` declaration
  - [x] 4.5: Add to help section under "Test targets"

- [x] Task 5: Manual verification (AC: #1, #2, #3)
  - [x] 5.1: Run `make test-stress` and verify all stress tests pass
  - [x] 5.2: Run `make test` and verify stress tests are skipped (check output for "Skipping stress test")
  - [x] 5.3: Verify tests complete within reasonable time (< 2 minutes total)
  - [x] 5.4: Verify tests pass with race detector enabled (already in make target)

## Dev Notes

### Previous Story Learnings (Story 9.1 + 9.2)

Story 9.1 fixed the immediate FD leak by adding explicit `Remove()` calls before `Close()` in the watcher cleanup. Story 9.2 fixed the goroutine accumulation by refactoring to a subscription model with:

1. **Single long-lived goroutine per watcher** - No more chained commands spawning new goroutines
2. **Context cancellation** - Graceful shutdown via `ctx.Done()`
3. **Channel-based event delivery** - Polling subscription channels instead of blocking `WaitForEvent()`
4. **`subscriptionsActive` flag** - Terminates polling chain cleanly

These tests verify the fixes work under stress conditions.

### Test Naming Convention

Per project-context.md, test names follow `Test<Function>_<Scenario>` pattern. For stress tests, use `TestStress<Feature>` prefix so the Makefile `-run Stress` filter catches them:

- `TestStressRepeatedWatchCycles` (not `TestRepeatedWatchCycles_NoLeak`)
- `TestStressEventStormBoundedGoroutines` (not `TestEventStorm_BoundedGoroutines`)
- `TestStressFDCountStability` (not `TestFDCountStability`)

### Adapted Pattern from vibe-dash Reference

Adapted from `vibe-dash/internal/adapters/filesystem/watcher_test.go:915-976` to match this project's watcher API:

```go
func TestStressRepeatedWatchCycles(t *testing.T) {
    if os.Getenv("STRESS_TESTS") == "" {
        t.Skip("Skipping stress test (set STRESS_TESTS=1 to run)")
    }

    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.jsonl")
    if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
        t.Fatalf("failed to create test file: %v", err)
    }

    // Stress test: create and close watchers 100 times
    for i := 0; i < 100; i++ {
        w, err := New(testFile)
        if err != nil {
            t.Fatalf("iteration %d: failed to create watcher: %v", i, err)
        }
        if err := w.Close(); err != nil {
            t.Fatalf("iteration %d: failed to close watcher: %v", i, err)
        }
    }

    // Verify final watcher still works
    w, err := New(testFile)
    if err != nil {
        t.Fatalf("final watcher creation failed: %v", err)
    }
    defer w.Close()

    // Verify events can still be received
    // ... (append to file and check EventsChan)
}
```

### FD Counting Helper (Cross-Platform)

```go
// countOpenFDs returns the number of open file descriptors for the current process.
// Works on macOS (/dev/fd) and Linux (/proc/self/fd).
func countOpenFDs(t *testing.T) int {
    t.Helper()

    // Try macOS path first
    entries, err := os.ReadDir("/dev/fd")
    if err == nil {
        return len(entries)
    }

    // Fallback for Linux
    entries, err = os.ReadDir("/proc/self/fd")
    if err == nil {
        return len(entries)
    }

    t.Skip("FD counting not supported on this platform")
    return 0
}
```

### Goroutine Counting Pattern

```go
func TestStressEventStormBoundedGoroutines(t *testing.T) {
    if os.Getenv("STRESS_TESTS") == "" {
        t.Skip("Skipping stress test (set STRESS_TESTS=1 to run)")
    }

    baseline := runtime.NumGoroutine()

    // ... create dashboard with 2 panes, send 100 events ...

    // Give goroutines time to stabilize
    runtime.GC()
    time.Sleep(100 * time.Millisecond)

    final := runtime.NumGoroutine()
    delta := final - baseline

    t.Logf("Goroutine count: baseline=%d, final=%d, delta=%d", baseline, final, delta)

    // Allow some tolerance for other system goroutines
    if delta > 10 {
        t.Errorf("Goroutine count increased by %d (baseline: %d, final: %d)",
            delta, baseline, final)
    }
}
```

### Makefile Target

Add after `test-short` target:

```makefile
# Run stress tests (requires STRESS_TESTS=1)
test-stress:
	@echo "Running stress tests..."
	STRESS_TESTS=1 $(GOTEST) -v -race -count=1 -timeout 5m ./... -run Stress
```

### Test File Organization

```
internal/
├── watcher/
│   ├── watcher.go
│   └── watcher_test.go        # Add TestStressRepeatedWatchCycles, TestStressFDCountStability
└── tui/
    ├── dashboard.go
    └── dashboard_test.go      # Add TestStressEventStormBoundedGoroutines
```

### Project Structure Notes

- All tests in `*_test.go` files alongside source code per project structure
- Stress tests guarded by `STRESS_TESTS` env var to avoid slowing down `make test`
- Use `TestStress` prefix so `-run Stress` captures all stress tests
- Follow naming convention per project-context.md

### Critical Don't-Miss Rules

1. **Use Makefile**: `make test-stress` target, never raw `go test` with env vars
2. **Guard stress tests**: Always check `os.Getenv("STRESS_TESTS")` and `t.Skip`
3. **Clean up resources**: Use `defer` for watcher/channel cleanup
4. **Race detection**: `make test-stress` includes `-race` flag (required per project-context.md)
5. **Platform awareness**: FD counting helper must handle both macOS and Linux
6. **Test naming**: Use `TestStress` prefix for all stress tests

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4-epic9.md#Story 9.3]
- [Source: _bmad-output/project-context.md#Testing Rules]
- [Source: internal/watcher/watcher_test.go] - Existing watcher tests (note: uses `New()` not `Watch()`)
- [Source: internal/tui/dashboard_test.go] - Story 9.2 subscription model tests
- [Pattern: vibe-dash Story 8.13 - Stress test reference (adapted for this project's API)]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - All tests pass

### Code Review Notes (2026-01-27)

**Reviewer:** Amelia (Dev Agent)

**Issues Found:** 0 High, 3 Medium, 2 Low

**Fixes Applied:**
1. **M1 (dashboard_test.go):** Fixed event count to actually send 100 events (50 per channel) as per AC-2, replacing the previous 20-event test with batched channel filling.
2. **M2 (watcher_test.go):** Added event counting and logging to TestStressFDCountStability to verify events are actually received.
3. **L1 (watcher_test.go):** Improved lsof fallback parsing to properly skip header and only count valid FD lines.
4. **L2 (Makefile):** Moved `-run Stress` flag before `./...` for conventional placement.

**Verification:** All stress tests pass with race detector enabled.

### Completion Notes List

1. **TestStressRepeatedWatchCycles** (AC #1): Created test that loops 100 times creating/closing watchers, verifies no "too many open files" error and final watcher can receive events.

2. **TestStressEventStormBoundedGoroutines** (AC #2): Created test that simulates 100 events through 2 panes, verifies goroutine count delta ≤ 10 after cleanup.

3. **TestStressFDCountStability** (AC #3): Created test with cross-platform FD counting (Linux `/proc/self/fd`, macOS `/dev/fd`, fallback to `lsof`). Generates 50 file events and verifies FD delta ≤ 5.

4. **Makefile target**: Added `test-stress` target with proper PHONY declaration and help text.

5. **All tests pass** with race detector enabled and complete in < 15 seconds total.

### File List

- `internal/watcher/watcher_test.go` (modified) - Added TestStressRepeatedWatchCycles, TestStressFDCountStability, countOpenFDs helper
- `internal/tui/dashboard_test.go` (modified) - Added TestStressEventStormBoundedGoroutines, runtime import
- `Makefile` (modified) - Added test-stress target, PHONY, and help entry

