---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - '_bmad-output/implementation-artifacts/fix-usage-api-auth-errors.md'
  - '/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/sprint-artifacts/stories/epic-8/8-13-fsnotify-file-handle-leak-fix.md'
  - '/Users/limjk/GitHub/JeiKeiLim/vibe-dash/internal/adapters/filesystem/watcher.go'
phase: 4
status: ready
createdAt: '2026-01-27'
---

# Epic 9: Dashboard File Descriptor Leak Fix

## Overview

Phase 4 continuation focusing on stability improvements and resource management for long-running cclv sessions. The primary issue is a file descriptor leak in dashboard watch mode that causes system resource exhaustion after extended runtime.

## Problem Statement

When running cclv in dashboard mode (multi-project grid view) with watch mode enabled, file descriptors accumulate over time due to improper fsnotify watcher lifecycle management. After ~8+ hours of runtime, the system hits the file descriptor limit (`ulimit -n`), causing failures like:

```
keychain access failed: pipe: too many open files
```

## Root Cause Analysis

### Primary Issue: Goroutine Accumulation in Command Chaining

**Location:** `internal/tui/dashboard.go`

| Function | Lines | Problem |
|----------|-------|---------|
| `waitForDirEvent()` | 624-656 | Blocking select in chained command - goroutine never terminates |
| `waitForPaneWatcher()` | 604-620 | Calls `WaitForEvent()` inside Cmd - double-blocking |
| Event chaining | 476, 497, 532, 563, 586 | Each event spawns new goroutine, old one keeps running |

### Accumulation Pattern

1. Dashboard with 2 panes × 2 watchers (file + dir) = 4 watchers
2. Each file event: +1 zombie goroutine holding FD
3. After hours: 100+ goroutines → hits `ulimit -n` → "too many open files"

### Platform-Specific Insight (macOS kqueue)

From vibe-dash learnings:
> On macOS kqueue, fsnotify opens a file descriptor for EACH watched path. `Close()` closes the kqueue FD but does NOT close individual watch FDs. You must explicitly `Remove()` all watches before `Close()`.

## Reference Implementation: vibe-dash

vibe-dash solved the identical issue in Story 8.13. Key patterns:

1. **Explicit Cleanup**: `Remove()` all paths before `Close()`
2. **Context Cancellation**: Cancel old context before creating new watcher
3. **Goroutine Lifecycle**: Each eventLoop captures watcher reference; channel closure triggers graceful exit
4. **Stress Testing**: 100+ repeated Watch() cycles to catch leaks

**Reference files:**
- `/Users/limjk/GitHub/JeiKeiLim/vibe-dash/internal/adapters/filesystem/watcher.go:105-136`
- `/Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/sprint-artifacts/stories/epic-8/8-13-fsnotify-file-handle-leak-fix.md`

## Requirements Inventory

### Functional Requirements

**FR-900: Dashboard Watch Mode Resource Management**
- FR-901: Dashboard watchers must properly cleanup file descriptors
- FR-902: Goroutine count must remain bounded regardless of event count
- FR-903: Long-running dashboard sessions must not exhaust system resources

### Non-Functional Requirements

**NFR-009: Resource Stability**
- File descriptor count must remain constant (±10) after initial startup
- Goroutine count per pane: max 2 (file watcher + dir watcher)
- 8+ hour runtime without resource exhaustion

**NFR-010: Graceful Shutdown**
- All watchers must cleanup within 5 seconds on exit
- No orphaned goroutines after dashboard close

### FR Coverage Map

| FR | Story | Description |
|----|-------|-------------|
| FR-901 | Story 9.1 | Watcher cleanup with explicit Remove() before Close() |
| FR-902 | Story 9.2 | Refactor dashboard to subscription model |
| FR-903 | Story 9.3 | Add stress test for long-running sessions |

---

## Epic Summary

Critical fix for file descriptor/goroutine leak in dashboard watch mode that prevents extended runtime usage.

**FRs covered:** FR-901, FR-902, FR-903
**Standalone:** Yes - isolated to dashboard watcher implementation
**Priority:** Critical - blocks long-term dashboard usage

---

## Story 9.1: Add Explicit Watcher Cleanup with Remove()

As a **cclv user running dashboard mode for extended periods**,
I want **watchers to properly release file descriptors**,
So that **the app doesn't crash with "too many open files" errors**.

### Acceptance Criteria

1. **AC-1: Explicit Path Removal Before Close**
   - Given a dashboard pane watcher needs to be closed
   - When `closeAllWatchers()` is called
   - Then all watched paths are removed via `watcher.Remove()` before `watcher.Close()`

2. **AC-2: macOS kqueue FD Release**
   - Given the app is running on macOS
   - When a watcher is closed
   - Then individual file descriptors for each watched path are released (not just the kqueue FD)

3. **AC-3: No Panic on Missing Paths**
   - Given a watched path no longer exists
   - When Remove() is called
   - Then errors are logged at debug level but don't cause panic

### Technical Notes

**Pattern from vibe-dash (`watcher.go:105-136`):**
```go
if w.watcher != nil {
    // Get all watched paths
    watchList := w.watcher.WatchList()

    // Remove each path explicitly (releases individual FDs on macOS)
    for _, path := range watchList {
        _ = w.watcher.Remove(path) // Ignore errors
    }

    // Now safe to close
    w.watcher.Close()
    w.watcher = nil
}
```

**Files to modify:**
- `internal/tui/dashboard.go` - `closeAllWatchers()` function
- `internal/watcher/watcher.go` - Add `Close()` method with explicit Remove()

**Complexity:** Low

---

## Story 9.2: Refactor Dashboard Watcher to Subscription Model

As a **developer maintaining cclv**,
I want **dashboard watchers to use a single long-lived goroutine per watcher**,
So that **goroutines don't accumulate with each file event**.

### Acceptance Criteria

1. **AC-1: Single Goroutine Per Watcher**
   - Given a dashboard pane with file watching enabled
   - When multiple file events occur
   - Then exactly 1 goroutine handles all events (not 1 per event)

2. **AC-2: Context Cancellation**
   - Given a dashboard pane's watcher needs to stop
   - When context is cancelled
   - Then the watcher goroutine exits gracefully within 1 second

3. **AC-3: Channel-Based Event Delivery**
   - Given file events occur
   - When the watcher goroutine processes them
   - Then events are sent via channel to Bubbletea (not via chained commands)

4. **AC-4: Bounded Goroutine Count**
   - Given a 2-pane dashboard running for 1 hour with 100+ file events
   - When goroutine count is measured
   - Then count is ≤ baseline + 4 (2 panes × 2 watcher types)

### Technical Notes

**Current (problematic) pattern:**
```go
// Each event spawns a new goroutine that chains to the next
func (m *DashboardModel) waitForDirEvent(paneIndex int) tea.Cmd {
    return func() tea.Msg {
        for {
            select {
            case event := <-w.Events:
                return dirEventMsg{event}  // Goroutine continues running!
            }
        }
    }
}
```

**New pattern (subscription model):**
```go
// Single goroutine per watcher, sends to channel
func (m *DashboardModel) startDirWatcher(ctx context.Context, paneIndex int) <-chan dirEventMsg {
    ch := make(chan dirEventMsg)
    go func() {
        defer close(ch)
        fsWatcher := m.panes[paneIndex].dirWatcher
        for {
            select {
            case <-ctx.Done():
                return
            case event, ok := <-fsWatcher.Events:
                if !ok {
                    return  // Watcher closed, exit gracefully
                }
                ch <- dirEventMsg{paneIndex, event}
            }
        }
    }()
    return ch
}
```

**Bubbletea integration:**
Use `tea.Sub` or periodic polling from the subscription channel.

**Files to modify:**
- `internal/tui/dashboard.go` - Refactor `waitForDirEvent()`, `waitForPaneWatcher()`
- Add context management to DashboardModel

**Complexity:** Medium-High

---

## Story 9.3: Add Stress Test for Long-Running Dashboard

As a **developer preventing regressions**,
I want **automated tests that verify constant resource usage**,
So that **file descriptor leaks are caught before production**.

### Acceptance Criteria

1. **AC-1: Repeated Watch Cycle Test**
   - Given a test that calls Watch() 100+ times
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

### Technical Notes

**Pattern from vibe-dash (`watcher_test.go:915-976`):**
```go
func TestRepeatedCalls_NoLeak(t *testing.T) {
    if os.Getenv("STRESS_TESTS") == "" {
        t.Skip("Skipping stress test (set STRESS_TESTS=1 to run)")
    }

    w := NewWatcher()
    defer w.Close()

    for i := 0; i < 100; i++ {
        ctx, cancel := context.WithCancel(context.Background())
        _, err := w.Watch(ctx, []string{tmpDir})
        require.NoError(t, err)
        cancel()
    }

    // Verify final watcher still works
    ctx := context.Background()
    ch, err := w.Watch(ctx, []string{tmpDir})
    require.NoError(t, err)
    // ... verify events still received
}
```

**Test location:**
- `internal/tui/dashboard_test.go` - New stress tests
- `internal/watcher/watcher_test.go` - Repeated Watch() test

**Complexity:** Medium

---

## Implementation Order

| Order | Story | Rationale |
|-------|-------|-----------|
| 1 | Story 9.1 | Quick win - fixes FD leak on cleanup |
| 2 | Story 9.2 | Core fix - prevents goroutine accumulation |
| 3 | Story 9.3 | Verification - ensures fix works and prevents regression |

## Validation Strategy

### Manual Testing

Monitor FD count during extended dashboard session:
```bash
# Get cclv PID
PID=$(pgrep -f "cclv")

# Watch FD count over time
watch -n 10 "lsof -p $PID 2>/dev/null | wc -l"
```

**Expected behavior:**
- FD count stable after initial startup (±10)
- After 10+ minutes: same as baseline
- After 1+ hour: same as baseline

### Automated Testing

```bash
# Run stress tests
STRESS_TESTS=1 make test
```

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Bubbletea subscription complexity | Reference vibe-dash implementation pattern |
| Race conditions in cleanup | Use mutex for watcher access, context for cancellation |
| Platform-specific behavior | Test on both macOS and Linux |

## References

- [vibe-dash Story 8.13](file:///Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/sprint-artifacts/stories/epic-8/8-13-fsnotify-file-handle-leak-fix.md)
- [vibe-dash watcher.go](file:///Users/limjk/GitHub/JeiKeiLim/vibe-dash/internal/adapters/filesystem/watcher.go)
- [vibe-dash Epic 8 Retrospective](file:///Users/limjk/GitHub/JeiKeiLim/vibe-dash/docs/sprint-artifacts/retrospectives/epic-8-retro-2025-12-31.md)
