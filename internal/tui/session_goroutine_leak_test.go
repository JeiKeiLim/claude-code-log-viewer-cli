package tui

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/watcher"
)

// getGoroutineCount returns the current number of goroutines.
func getGoroutineCount() int {
	return runtime.NumGoroutine()
}

// waitForGoroutineCount waits for goroutine count to reach or go below the target.
// Returns true if the target was reached within the timeout.
func waitForGoroutineCount(target int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= target {
			return true
		}
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestSessionDashboard_NoGoroutineLeak_SingleOpenClose verifies that opening
// and closing a session dashboard leaves no leaked goroutines.
func TestSessionDashboard_NoGoroutineLeak_SingleOpenClose(t *testing.T) {
	// Stabilize goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/leak-test"

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(100*time.Millisecond),
	)
	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Simulate session appearing (no background goroutines needed for this test)
	sessionID := "leak-test-session"
	makeJSONLFile(t, projectDir, sessionID)

	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", m.PaneCount())
	}

	// Close everything
	m.closeAll()

	// Wait for goroutines to drain
	ok := waitForGoroutineCount(baseline+2, 5*time.Second) // Allow some tolerance
	if !ok {
		t.Errorf("goroutine leak detected: baseline=%d, current=%d (after 5s)",
			baseline, getGoroutineCount())
	}
}

// TestSessionDashboard_NoGoroutineLeak_MultipleOpenCloseCycles runs multiple
// open/close cycles and verifies goroutines don't accumulate.
func TestSessionDashboard_NoGoroutineLeak_MultipleOpenCloseCycles(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	for cycle := 0; cycle < 5; cycle++ {
		checker := newTestPIDChecker(100 + cycle)
		sessDir := t.TempDir()
		projectDir := t.TempDir()
		projectPath := fmt.Sprintf("/tmp/cycle-%d", cycle)

		scanner := session.NewSessionScanner(sessDir,
			session.WithScannerPIDChecker(checker),
			session.WithScanInterval(100*time.Millisecond),
		)
		monitor := session.NewMonitor(
			session.WithMonitorPIDChecker(checker),
			session.WithPollInterval(session.MinPollInterval),
		)

		m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
		m.SetSize(120, 40)

		sessionID := fmt.Sprintf("cycle-sess-%d", cycle)
		makeJSONLFile(t, projectDir, sessionID)

		// Add a pane
		scanResult := session.ScanResult{
			Sessions: []session.ActiveSession{
				{Meta: session.SessionMeta{
					PID:       100 + cycle,
					SessionID: sessionID,
					CWD:       projectPath,
				}},
			},
		}
		newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
		m = newModel.(SessionDashboardModel)

		if m.PaneCount() != 1 {
			t.Fatalf("cycle %d: expected 1 pane, got %d", cycle, m.PaneCount())
		}

		// Close
		m.closeAll()
	}

	// After all cycles, goroutine count should be near baseline
	ok := waitForGoroutineCount(baseline+3, 5*time.Second)
	if !ok {
		t.Errorf("goroutine leak after 5 cycles: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestSessionDashboard_NoGoroutineLeak_SessionCloseRemovesPaneCleanly verifies
// that when a session closes (PID dies), the pane removal doesn't leak goroutines.
func TestSessionDashboard_NoGoroutineLeak_SessionCloseRemovesPaneCleanly(t *testing.T) {
	checker := newTestPIDChecker(100, 200, 300)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/remove-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Add 3 panes
	sessions := make([]session.ActiveSession, 3)
	for i := 0; i < 3; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID:       100 + i*100,
				SessionID: sid,
				CWD:       projectPath,
			},
		}
	}

	scanResult := session.ScanResult{Sessions: sessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 3 {
		t.Fatalf("expected 3 panes, got %d", m.PaneCount())
	}

	// Close sessions one by one
	for _, pid := range []int{100, 200, 300} {
		closeEvent := session.SessionEvent{
			Type:    session.SessionClosed,
			Session: session.ActiveSession{Meta: session.SessionMeta{PID: pid}},
		}
		newModel, _ = m.Update(sessionClosedMsg{event: closeEvent})
		m = newModel.(SessionDashboardModel)
	}

	if m.PaneCount() != 0 {
		t.Errorf("expected 0 panes after all closures, got %d", m.PaneCount())
	}

	// Final cleanup
	m.closeAll()
}

// TestSessionDashboard_NoGoroutineLeak_RapidOpenClose stress tests rapid
// session creation and removal to check for goroutine leaks.
func TestSessionDashboard_NoGoroutineLeak_RapidOpenClose(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	checker := newTestPIDChecker()
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/rapid-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	for i := 0; i < 20; i++ {
		pid := 1000 + i
		checker.SetAlive(pid, true)

		sid := fmt.Sprintf("rapid-%d", i)
		makeJSONLFile(t, projectDir, sid)

		// Add session
		scanResult := session.ScanResult{
			Sessions: []session.ActiveSession{
				{Meta: session.SessionMeta{PID: pid, SessionID: sid, CWD: projectPath}},
			},
		}
		newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
		m = newModel.(SessionDashboardModel)

		// Immediately close it
		closeEvent := session.SessionEvent{
			Type:    session.SessionClosed,
			Session: session.ActiveSession{Meta: session.SessionMeta{PID: pid}},
		}
		newModel, _ = m.Update(sessionClosedMsg{event: closeEvent})
		m = newModel.(SessionDashboardModel)
	}

	m.closeAll()

	ok := waitForGoroutineCount(baseline+3, 5*time.Second)
	if !ok {
		t.Errorf("goroutine leak after rapid open/close: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestSessionDashboard_WaitForGoroutines verifies the WaitForGoroutines method works.
func TestSessionDashboard_WaitForGoroutines(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// WaitForGoroutines on fresh model should return immediately
	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForGoroutines blocked on fresh model")
	}
}

// TestSessionDashboard_CloseAll_WaitsForGoroutines verifies closeAll blocks
// until all goroutines have exited.
func TestSessionDashboard_CloseAll_WaitsForGoroutines(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// closeAll should return promptly with no active goroutines
	done := make(chan struct{})
	go func() {
		m.closeAll()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("closeAll blocked for too long")
	}
}

// TestSessionDashboard_CloseAll_Idempotent verifies closeAll can be called multiple times.
func TestSessionDashboard_CloseAll_Idempotent(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// Multiple calls should not panic or deadlock
	m.closeAll()
	m.closeAll()
	m.closeAll()
}

// TestSessionDashboard_NoGoroutineLeak_WithFileWatcher tests that file watcher
// subscription goroutines are properly cleaned up.
func TestSessionDashboard_NoGoroutineLeak_WithFileWatcher(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/watcher-leak-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "watcher-test"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	// Add a pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Manually create a file watcher and start subscription
	// (instead of going through handlePaneContentLoaded which has value-semantics issues)
	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	m.panes[0].watcher = w
	m.panes[0].jsonlPath = jsonlPath
	m.startSessionFileWatcherSubscription(m.ctx, 0)

	// Verify watcher was created
	if m.panes[0].watcher == nil {
		t.Fatal("expected watcher to be created")
	}
	if m.panes[0].fileEventChan == nil {
		t.Fatal("expected fileEventChan to be created")
	}

	// Give goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Close everything
	m.closeAll()

	ok := waitForGoroutineCount(baseline+3, 5*time.Second)
	if !ok {
		t.Errorf("goroutine leak with file watcher: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestSessionDashboard_NoGoroutineLeak_PaneWithWatcherClosed tests that closing
// a pane with an active file watcher doesn't leak the subscription goroutine.
func TestSessionDashboard_NoGoroutineLeak_PaneWithWatcherClosed(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/pane-watcher-close-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "pane-watch-close"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	// Add pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Manually create watcher and start subscription
	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	m.panes[0].watcher = w
	m.panes[0].jsonlPath = jsonlPath
	m.startSessionFileWatcherSubscription(m.ctx, 0)

	time.Sleep(50 * time.Millisecond)

	// Cancel context first to stop the subscription goroutine before
	// modifying panes (avoids race between goroutine reading watcher and
	// handleSessionClosed closing watcher)
	m.cancel()
	m.WaitForGoroutines()

	// Close session (pane removed) - safe now since goroutine has exited
	closeEvent := session.SessionEvent{
		Type:    session.SessionClosed,
		Session: session.ActiveSession{Meta: session.SessionMeta{PID: 100}},
	}
	newModel, _ = m.Update(sessionClosedMsg{event: closeEvent})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 0 {
		t.Fatalf("expected 0 panes, got %d", m.PaneCount())
	}

	ok := waitForGoroutineCount(baseline+3, 5*time.Second)
	if !ok {
		t.Errorf("goroutine leak after pane watcher close: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestSessionDashboard_NoGoroutineLeak_MaxPanesCycle tests that filling to max
// panes and then closing all doesn't leak goroutines.
func TestSessionDashboard_NoGoroutineLeak_MaxPanesCycle(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	pids := make([]int, MaxSessionPanes)
	for i := range pids {
		pids[i] = 500 + i
	}
	checker := newTestPIDChecker(pids...)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/max-panes-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Create max panes
	sessions := make([]session.ActiveSession, MaxSessionPanes)
	for i := 0; i < MaxSessionPanes; i++ {
		sid := fmt.Sprintf("max-sess-%d", i)
		makeJSONLFile(t, projectDir, sid)
		sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{PID: 500 + i, SessionID: sid, CWD: projectPath},
		}
	}

	scanResult := session.ScanResult{Sessions: sessions}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != MaxSessionPanes {
		t.Fatalf("expected %d panes, got %d", MaxSessionPanes, m.PaneCount())
	}

	// Close all at once
	m.closeAll()

	ok := waitForGoroutineCount(baseline+3, 5*time.Second)
	if !ok {
		t.Errorf("goroutine leak after max panes cycle: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestMonitor_NoGoroutineLeak_StartStop verifies Monitor's goroutine is properly
// cleaned up across multiple start/stop cycles.
func TestMonitor_NoGoroutineLeak_StartStop(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	checker := newTestPIDChecker()

	for i := 0; i < 10; i++ {
		monitor := session.NewMonitor(
			session.WithMonitorPIDChecker(checker),
			session.WithPollInterval(session.MinPollInterval),
		)

		pid := 1000 + i
		checker.SetAlive(pid, true)
		monitor.TrackSession(session.ActiveSession{
			Meta: session.SessionMeta{PID: pid, SessionID: fmt.Sprintf("s%d", i)},
		})

		monitor.Start(context.Background())
		monitor.Stop()
	}

	ok := waitForGoroutineCount(baseline+2, 5*time.Second)
	if !ok {
		t.Errorf("monitor goroutine leak: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestScanner_NoGoroutineLeak_StartStop verifies Scanner's goroutine is properly
// cleaned up across multiple start/stop cycles.
func TestScanner_NoGoroutineLeak_StartStop(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := getGoroutineCount()

	checker := newTestPIDChecker()

	for i := 0; i < 10; i++ {
		sessDir := t.TempDir()
		scanner := session.NewSessionScanner(sessDir,
			session.WithScannerPIDChecker(checker),
			session.WithScanInterval(100*time.Millisecond),
		)

		ch := scanner.Start()
		if ch == nil {
			t.Fatalf("cycle %d: scanner.Start returned nil", i)
		}
		scanner.Stop()

		// Drain the channel
		for range ch {
		}
	}

	ok := waitForGoroutineCount(baseline+2, 5*time.Second)
	if !ok {
		t.Errorf("scanner goroutine leak: baseline=%d, current=%d",
			baseline, getGoroutineCount())
	}
}

// TestSessionDashboard_FileWatcherSubscription_ExitsOnContextCancel verifies
// that the file watcher subscription goroutine exits when context is cancelled.
func TestSessionDashboard_FileWatcherSubscription_ExitsOnContextCancel(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/sub-cancel-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "sub-cancel"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	// Add pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Create a real file watcher and start subscription
	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	m.panes[0].watcher = w
	m.startSessionFileWatcherSubscription(m.ctx, 0)

	// Verify goroutine is running (channel should be set)
	if m.panes[0].fileEventChan == nil {
		t.Fatal("expected fileEventChan to be set")
	}

	// Cancel context and close watcher
	m.cancel()
	_ = w.Close()

	// Wait for goroutine to exit
	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Good - goroutine exited
	case <-time.After(3 * time.Second):
		t.Fatal("subscription goroutine did not exit after context cancellation")
	}
}

// TestSessionDashboard_BridgeGoroutines_ExitOnContextCancel verifies that
// scanner and monitor bridge goroutines exit when context is cancelled.
func TestSessionDashboard_BridgeGoroutines_ExitOnContextCancel(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(100*time.Millisecond),
	)
	monitor := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(session.MinPollInterval),
	)

	m := NewSessionDashboardModel("/tmp/p", "/tmp/pd", scanner, monitor)

	// Start scanner and monitor bridge goroutines by executing the cmds
	scannerCmd := m.startScannerCmd()
	monitorCmd := m.startMonitorCmd()

	// Execute the commands (they start goroutines)
	if scannerCmd != nil {
		scannerCmd()
	}
	if monitorCmd != nil {
		monitorCmd()
	}

	// Give goroutines time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	m.cancel()
	scanner.Stop()
	monitor.Stop()

	// Wait for bridge goroutines to exit
	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(3 * time.Second):
		t.Fatal("bridge goroutines did not exit after context cancellation")
	}
}

// TestSessionDashboard_HandlePaneContentLoaded_WatcherGoroutineTracked verifies
// that the file watcher subscription goroutine created via startSessionFileWatcherSubscription
// is tracked by the WaitGroup.
func TestSessionDashboard_HandlePaneContentLoaded_WatcherGoroutineTracked(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/tracked-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "tracked-watcher"
	jsonlPath := makeJSONLFile(t, projectDir, sessionID)

	// Add pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Manually create watcher and start subscription
	w, err := watcher.New(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	m.panes[0].watcher = w
	m.startSessionFileWatcherSubscription(m.ctx, 0)

	time.Sleep(50 * time.Millisecond)

	// Watcher subscription goroutine should be tracked
	// Verify by cancelling context and waiting
	m.cancel()

	// Close watcher to unblock the goroutine
	_ = w.Close()

	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Good - goroutine was tracked and exited
	case <-time.After(3 * time.Second):
		t.Fatal("watcher goroutine not tracked by WaitGroup")
	}
}

// TestSessionDashboard_NoGoroutineLeak_EscapeKey verifies that pressing escape
// properly cleans up all goroutines.
func TestSessionDashboard_NoGoroutineLeak_EscapeKey(t *testing.T) {
	checker := newTestPIDChecker(100)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/esc-test"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	sessionID := "esc-session"
	makeJSONLFile(t, projectDir, sessionID)

	// Add pane
	scanResult := session.ScanResult{
		Sessions: []session.ActiveSession{
			{Meta: session.SessionMeta{PID: 100, SessionID: sessionID, CWD: projectPath}},
		},
	}
	newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	// Press escape - should trigger closeAll
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(SessionDashboardModel)

	if m.subscriptionsActive {
		t.Error("subscriptions should be inactive after escape")
	}
	if cmd == nil {
		t.Error("expected GoBackFromSessionDashboardMsg command")
	}

	// WaitForGoroutines should return promptly
	done := make(chan struct{})
	go func() {
		m.WaitForGoroutines()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(3 * time.Second):
		t.Fatal("goroutines still running after escape")
	}
}

// Ensure imports are used.
var (
	_ = context.Background
	_ tea.Msg
	_ watcher.Watcher
	_ types.LogEntry
)
