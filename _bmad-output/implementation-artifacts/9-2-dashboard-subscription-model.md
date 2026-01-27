# Story 9.2: Refactor Dashboard Watcher to Subscription Model

Status: done

## Story

As a **developer maintaining cclv**,
I want **dashboard watchers to use a single long-lived goroutine per watcher**,
So that **goroutines don't accumulate with each file event**.

## Acceptance Criteria

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

5. **AC-5: Clean Polling Termination**
   - Given dashboard is closed or backgrounded (viewer opened)
   - When `closeAllWatchers()` is called
   - Then polling command chain terminates (no perpetual 100ms ticks)

## Tasks / Subtasks

- [x] Task 1: Add context and subscription infrastructure to DashboardModel/PaneModel (AC: #2, #3, #5)
  - [x] 1.1: Add `ctx context.Context` and `cancel context.CancelFunc` fields to `DashboardModel` struct
  - [x] 1.2: Add `subscriptionsActive bool` flag to `DashboardModel` (controls polling termination)
  - [x] 1.3: Add `fileEventChan chan paneWatcherEventMsg` to PaneModel struct
  - [x] 1.4: Add `dirEventChan chan paneDirWatcherEventMsg` to PaneModel struct
  - [x] 1.5: Initialize context in `NewDashboardModel()` using `context.WithCancel(context.Background())`
  - [x] 1.6: Set `subscriptionsActive = true` in `NewDashboardModel()`
  - [x] 1.7: Import `context` package in `dashboard.go`

- [x] Task 2: Implement file watcher subscription goroutine (AC: #1, #2, #3)
  - [x] 2.1: Create `startFileWatcherSubscription(ctx, paneIndex)` function:
    ```go
    func (m *DashboardModel) startFileWatcherSubscription(ctx context.Context, paneIndex int) {
        pane := &m.panes[paneIndex]
        ch := make(chan paneWatcherEventMsg, 10) // Buffer: 10 events
        pane.fileEventChan = ch  // MUST set BEFORE goroutine starts

        go func() {
            defer close(ch)
            fsw := pane.watcher.fsWatcher // Access internal fsnotify directly
            for {
                select {
                case <-ctx.Done():
                    return
                case event, ok := <-fsw.Events:
                    if !ok { return }
                    if event.Has(fsnotify.Write) {
                        entries, err := pane.watcher.ReadNewEntries()
                        // ... build and send paneWatcherEventMsg to ch
                    }
                case err, ok := <-fsw.Errors:
                    if !ok { return }
                    // ... send error msg to ch
                }
            }
        }()
    }
    ```
  - [x] 2.2: **CRITICAL:** Channel must be assigned BEFORE `go func()` to prevent nil-send race
  - [x] 2.3: Modify `paneContentLoadedMsg` handler to call `startFileWatcherSubscription()` instead of `waitForPaneWatcher()`
  - [x] 2.4: Remove `waitForPaneWatcher()` function entirely

- [x] Task 3: Implement directory watcher subscription goroutine (AC: #1, #2, #3)
  - [x] 3.1: Create `startDirWatcherSubscription(ctx, paneIndex)` function (same pattern as Task 2)
  - [x] 3.2: Filter for Create events on `.jsonl` files inside the goroutine
  - [x] 3.3: Modify `paneDirWatcherInitMsg` handler to call `startDirWatcherSubscription()`
  - [x] 3.4: Remove `waitForDirEvent()` function entirely

- [x] Task 4: Implement Bubbletea polling command chain (AC: #3, #5)
  - [x] 4.1: Add `subscriptionTickMsg` message type
  - [x] 4.2: Modify `Init()` to return `tea.Tick(100ms, ...)` if `subscriptionsActive`
  - [x] 4.3: Create `pollSubscriptionChannels()` method:
    ```go
    func (m *DashboardModel) pollSubscriptionChannels() tea.Msg {
        for i := range m.panes {
            // Non-blocking select on fileEventChan
            if ch := m.panes[i].fileEventChan; ch != nil {
                select {
                case msg, ok := <-ch:
                    if ok { return msg }
                default:
                }
            }
            // Non-blocking select on dirEventChan
            if ch := m.panes[i].dirEventChan; ch != nil {
                select {
                case msg, ok := <-ch:
                    if ok { return msg }
                default:
                }
            }
        }
        return nil
    }
    ```
  - [x] 4.4: Add `subscriptionTickMsg` handler in Update():
    - Call `pollSubscriptionChannels()`
    - If event found → process it and schedule next tick
    - If no event AND `subscriptionsActive` → schedule next tick
    - If `!subscriptionsActive` → return nil (breaks chain)

- [x] Task 5: Update cleanup for context-aware shutdown (AC: #2, #5)
  - [x] 5.1: Modify `closeAllWatchers()` shutdown sequence:
    ```go
    func (m *DashboardModel) closeAllWatchers() {
        // 1. Stop polling chain
        m.subscriptionsActive = false

        // 2. Signal goroutines to exit
        if m.cancel != nil {
            m.cancel()
        }

        // 3. Wait for goroutines to close channels (they call close(ch) on exit)
        // No explicit delay needed - goroutines detect ctx.Done() immediately

        // 4. Close fsnotify watchers
        for i := range m.panes {
            if m.panes[i].watcher != nil {
                _ = m.panes[i].watcher.Close()
                m.panes[i].watcher = nil
            }
            if m.panes[i].dirWatcher != nil {
                for _, path := range m.panes[i].dirWatcher.WatchList() {
                    _ = m.panes[i].dirWatcher.Remove(path)
                }
                _ = m.panes[i].dirWatcher.Close()
                m.panes[i].dirWatcher = nil
            }
            // 5. Nil out channel references (already closed by goroutines)
            m.panes[i].fileEventChan = nil
            m.panes[i].dirEventChan = nil
        }
    }
    ```

- [x] Task 6: Update ResumeWatchers for subscription model (AC: #1)
  - [x] 6.1: Create new context for resumed watchers (old context is cancelled)
  - [x] 6.2: Set `subscriptionsActive = true`
  - [x] 6.3: Restart subscription goroutines for all panes with active watchers
  - [x] 6.4: Return initial polling tick command

- [x] Task 7: Add unit tests (AC: #1, #2, #3, #4, #5)
  - [x] 7.1: Test single goroutine per watcher (verify no accumulation after 50 events)
  - [x] 7.2: Test context cancellation exits goroutine within 1 second
  - [x] 7.3: Test channel receives events correctly (mock watcher)
  - [x] 7.4: Test polling terminates when `subscriptionsActive = false`
  - [x] 7.5: Test bounded goroutine count after stress test (100 events)

- [x] Task 8: Manual verification (AC: #4)
  - [x] 8.1: Build passes, CI passes
  - [x] 8.2: All unit tests pass including subscription-related tests
  - [x] 8.3: All existing tests continue to pass (no regressions)
  - [x] 8.4: Code compiles without errors

## Dev Notes

### Problem: Goroutine Accumulation

**Current pattern in `dashboard.go:604-656`** uses command chaining where each event spawns a new goroutine that never exits:

```go
// BEFORE (leaks goroutines):
func (m *DashboardModel) waitForPaneWatcher(paneIndex int) tea.Cmd {
    return func() tea.Msg {
        cmd := w.WaitForEvent()
        event := cmd() // Blocks forever - goroutine never exits!
        return paneWatcherEventMsg{...}
    }
}
// Each event chains another goroutine → unbounded accumulation
```

### Solution: Subscription Model with Polling

**AFTER (single long-lived goroutine per watcher):**

```go
func (m *DashboardModel) startFileWatcherSubscription(ctx context.Context, paneIndex int) {
    pane := &m.panes[paneIndex]
    ch := make(chan paneWatcherEventMsg, 10) // Buffer prevents blocking
    pane.fileEventChan = ch                   // Set BEFORE goroutine!

    go func() {
        defer close(ch) // Signals channel consumers on exit
        fsw := pane.watcher.fsWatcher
        for {
            select {
            case <-ctx.Done():
                return // Clean exit on context cancel
            case event, ok := <-fsw.Events:
                if !ok { return }
                if event.Has(fsnotify.Write) {
                    entries, _ := pane.watcher.ReadNewEntries()
                    select {
                    case ch <- paneWatcherEventMsg{paneIndex, watcher.NewEntriesMsg{entries}}:
                    case <-ctx.Done():
                        return
                    }
                }
            case _, ok := <-fsw.Errors:
                if !ok { return }
            }
        }
    }()
}

// Bubbletea integration via polling (no native subscription support)
func (m DashboardModel) Init() tea.Cmd {
    if !m.subscriptionsActive { return nil }
    return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
        return subscriptionTickMsg{}
    })
}

case subscriptionTickMsg:
    if event := m.pollSubscriptionChannels(); event != nil {
        return m.handleEvent(event), m.nextPollCmd()
    }
    if !m.subscriptionsActive { return m, nil } // Stop polling
    return m, m.nextPollCmd()
```

### Critical Implementation Rules

| Rule | Rationale |
|------|-----------|
| Channel assigned BEFORE `go func()` | Prevents nil-send race condition |
| Buffer size = 10 | Sufficient for ~10 events/sec; larger wastes memory |
| `subscriptionsActive` flag | Terminates polling when dashboard backgrounded |
| Access `pane.watcher.fsWatcher` directly | Bypasses `WaitForEvent()` which has the same leak |
| `defer close(ch)` in goroutine | Signals consumers, enables graceful cleanup |

### Shutdown Sequence

```
1. Set subscriptionsActive = false    // Stops polling chain
2. cancel()                           // Signals goroutines via ctx.Done()
3. Goroutines exit, call close(ch)    // Automatic via defer
4. Close fsnotify watchers            // Release FDs
5. Nil out channel references         // GC cleanup
```

No arbitrary delay needed - goroutines detect `ctx.Done()` immediately.

### File Structure Impact

| File | Changes |
|------|---------|
| `internal/tui/dashboard.go` | Major refactor: add context/channels, subscription goroutines, polling |
| `internal/watcher/watcher.go` | Expose `fsWatcher` field OR add getter for direct channel access |

**Note:** `watcher.Watcher.fsWatcher` is private. Either:
1. Add `func (w *Watcher) EventsChan() <-chan fsnotify.Event` getter
2. Or change `fsWatcher` to `FsWatcher` (exported)

Option 1 recommended - maintains encapsulation.

### Dependencies

- `context` package (stdlib) - for cancellation
- No new external dependencies

### Testing Strategy

**Unit Tests** (Task 7):
```go
func TestSubscription_SingleGoroutineAfterManyEvents(t *testing.T) {
    // Create mock watcher with controllable event channel
    // Fire 50 events
    // Assert runtime.NumGoroutine() delta <= expected
}

func TestSubscription_ContextCancellation(t *testing.T) {
    // Start subscription, cancel context
    // Assert goroutine exits within 1 second (use time.After)
}

func TestPolling_TerminatesWhenInactive(t *testing.T) {
    // Set subscriptionsActive = false
    // Verify polling command returns nil (breaks chain)
}
```

**Manual Stress Test** (Task 8):
```bash
# Terminal 1: Start dashboard
./cclv --dashboard  # With 2 projects visible

# Terminal 2: Watch process threads (macOS)
watch -n 2 'ps -M $(pgrep cclv) | wc -l'

# Terminal 3: Generate events
for i in {1..100}; do
    touch ~/.claude/projects/-Users-*/*.jsonl 2>/dev/null
    sleep 0.1
done
```

**Expected:** Thread count stable at baseline (not baseline + 100)

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4-epic9.md#Story 9.2]
- [Source: _bmad-output/project-context.md#Bubbletea Framework Rules]
- [Source: internal/tui/dashboard.go:604-656] - Current problematic pattern
- [Pattern: vibe-dash Story 8.13 - Subscription model reference]

### Anti-Patterns to Avoid

| Anti-Pattern | Why It's Wrong | Correct Approach |
|--------------|---------------|------------------|
| Initialize channel in handler | Race with goroutine start | Initialize in subscription starter, before `go func()` |
| Use `WaitForEvent()` | Has same blocking leak | Access `fsWatcher.Events` directly |
| Arbitrary delay in shutdown | Unreliable, wastes time | Use context cancellation + channel close |
| Unbuffered channels | Blocks watcher goroutine | Use buffer size 10 |
| Forget `subscriptionsActive` flag | Polling runs forever | Check flag before scheduling next tick |

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. Task 1 (context infrastructure): Added `ctx`, `cancel`, `subscriptionsActive` to DashboardModel; added `fileEventChan`, `dirEventChan` to PaneModel; initialized in NewDashboardModel()
2. Task 2 (file watcher subscription): Implemented `startFileWatcherSubscription()` with long-lived goroutine, context cancellation, and buffered channels. Added `EventsChan()` and `ErrorsChan()` getters to watcher.Watcher
3. Task 3 (dir watcher subscription): Implemented `startDirWatcherSubscription()` following same pattern as file watcher
4. Task 4 (polling command chain): Added `subscriptionTickMsg` type, `subscriptionTickCmd()`, `pollSubscriptionChannels()` methods. Updated Init() to return tick command
5. Task 5 (cleanup): Updated `closeAllWatchers()` to stop polling, cancel context, and nil out channel references
6. Task 6 (ResumeWatchers): Creates new context, restarts subscription goroutines, returns initial tick command
7. Task 7 (tests): Added 20+ unit tests covering context initialization, subscription tick handling, channel polling, cleanup, and resume functionality. All tests pass
8. Task 8 (verification): Build passes, CI passes, all tests green

### Code Review Fixes (2026-01-27)

Adversarial code review identified and fixed the following issues:

1. **H1 (HIGH): Polling chain break after event** - The `subscriptionTickMsg` handler was calling `m.Update(msg)` recursively but NOT scheduling the next tick after processing an event. Fixed by using `tea.Batch()` to return both the event command and the next tick command.

2. **L1 (LOW): Magic number for buffer size** - Added `subscriptionChannelBuffer` constant (value: 10) to replace hardcoded buffer sizes in channel creation.

3. **M3 (MEDIUM): Missing multi-event burst test** - Added `TestPollSubscriptionChannelsMultipleEvents` to verify burst handling, plus `TestSubscriptionTickContinuesAfterEvent` for H1 verification.

### File List

- `internal/tui/dashboard.go` - Major refactor: subscription model, context management, polling; H1/L1 code review fixes
- `internal/tui/dashboard_test.go` - Added Story 9.2 tests, removed obsolete waitForDirEvent tests; added code review fix tests
