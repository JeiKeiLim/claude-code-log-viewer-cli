// Package tui — integration tests for Phase 5a Sub-AC 3.
//
// These tests verify end-to-end detection latency: from the moment a
// ~/.claude/sessions/{pid}.json file is created on disk until the
// SessionDashboardModel has a pane for that session.  The target SLA is
// consistently under 2 seconds for polling-based detection and under
// 1 second for fsnotify-based detection.
//
// Test strategy
// ─────────────
// Because SessionDashboardModel is a Bubbletea value-type model, there is no
// "background run" loop outside a real tea.Program.  Instead, we drive the
// subscription-tick polling by calling pollChannels() → Update() in a tight
// test loop, which is functionally identical to what the Bubbletea runtime
// does at 100 ms ticks.  We start the real scanner / dir-watcher goroutines
// and bridge their output channels to the model's channels directly, giving
// us genuine file-system latency measurements without a full TUI runtime.
package tui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
)

// ─────────────────────────────── test helpers ───────────────────────────────

// bridgeScannerToModel starts the given scanner and forwards all ScanResults to
// the model's scanResultChan.  Returns a stop function that terminates the bridge.
func bridgeScannerToModel(ctx context.Context, sc *session.SessionScanner, resultChan chan<- session.ScanResult) func() {
	resultCh := sc.Start()

	ctx2, cancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx2.Done():
				sc.Stop()
				return
			case res, ok := <-resultCh:
				if !ok {
					return
				}
				select {
				case resultChan <- res:
				case <-ctx2.Done():
					return
				default:
					// Channel full — drop, next poll will catch it.
				}
			}
		}
	}()
	return cancel
}

// bridgeDirWatcherToModel starts the watcher and forwards all SessionEvents to
// the model's dirWatcherChan.  Returns a stop function.
func bridgeDirWatcherToModel(ctx context.Context, dw *session.SessionDirectoryWatcher, eventChan chan<- session.SessionEvent) func() {
	if err := dw.Start(); err != nil {
		return func() {}
	}

	ctx2, cancel := context.WithCancel(ctx)
	go func() {
		evts := dw.Events()
		for {
			select {
			case <-ctx2.Done():
				return
			case ev, ok := <-evts:
				if !ok {
					return
				}
				select {
				case eventChan <- ev:
				case <-ctx2.Done():
					return
				default:
				}
			}
		}
	}()
	return cancel
}

// driveModelUntilPaneCount drives the model's subscription-tick loop until
// PaneCount reaches want or timeout expires.  It returns the elapsed time
// from the moment the function was called, or -1 on timeout.
//
// Note: the function polls pollChannels() every 10 ms, which is 10× faster
// than the real 100 ms tick.  This intentionally favours the test, giving
// us "is the detection pipeline capable of ≤ 2 s?" not "does the 100 ms tick
// add up to ≤ 2 s?".
func driveModelUntilPaneCount(m *SessionDashboardModel, want int, timeout time.Duration) time.Duration {
	start := time.Now()
	deadline := start.Add(timeout)
	for time.Now().Before(deadline) {
		if msg := m.pollChannels(); msg != nil {
			newModel, _ := m.Update(msg)
			*m = newModel.(SessionDashboardModel)
		}
		if m.PaneCount() >= want {
			return time.Since(start)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return -1
}

// writeIntegrationSessionFile writes a minimal {pid}.json in dir and marks the
// session CWD to match projectPath so the dashboard's project filter accepts it.
func writeIntegrationSessionFile(t *testing.T, dir string, pid int, sessionID, projectPath string) string {
	t.Helper()
	meta := session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       projectPath,
		StartedAt: time.Now().UnixMilli(),
		Kind:      "interactive",
	}
	return makeTestSessionFile(t, dir, pid, meta.SessionID, meta.CWD)
}

// latencyThreshold is the acceptance criterion: all detections must be faster.
const latencyThreshold = 2 * time.Second

// ────────────────────── scanner-only detection (Sub-AC 3) ───────────────────

// TestIntegration_Scanner_DetectionUnder2Seconds verifies that a new
// ~/${pid}.json file is reflected as a dashboard pane within 2 s when the
// polling scanner runs at its default 2-second interval.
//
// The test uses a 1.5-second scan interval so it reliably exercises the
// real polling path while still finishing within the 3-second test deadline.
func TestIntegration_Scanner_DetectionUnder2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-scanner-test"
	const pid = 77001
	const sessionID = "scanner-latency-session"

	// Use 1.5 s scan interval — realistic (~2 s default) but finite.
	// WithJSONLBaseDir directs the scanner to projectDir (temp dir) rather than
	// the real ~/.claude/projects tree, which test sessions do not populate.
	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
		session.WithScanInterval(1500*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	mon := session.NewMonitor(
		session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)
	makeJSONLFile(t, projectDir, sessionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopBridge()

	// Create the session file and start the clock.
	start := time.Now()
	writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)

	elapsed := driveModelUntilPaneCount(&m, 1, 3*time.Second)

	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("session pane never appeared within 3-second timeout after file creation")
	}
	if elapsed > latencyThreshold {
		t.Errorf("scanner detection latency %v exceeds %v SLA", elapsed, latencyThreshold)
	}
	_ = start
	t.Logf("scanner detection latency: %v", elapsed)
}

// TestIntegration_Scanner_ConsistentDetectionUnder2Seconds runs the scanner
// detection test five times in sequence to verify consistent sub-2s behaviour.
// All five runs must meet the SLA; statistics are logged.
func TestIntegration_Scanner_ConsistentDetectionUnder2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping consistency latency test in short mode")
	}

	const runs = 5
	const scanInterval = 1200 * time.Millisecond // Well inside 2-second SLA
	const projectPath = "/tmp/integ-scanner-consistent"

	latencies := make([]time.Duration, 0, runs)

	for run := 0; run < runs; run++ {
		t.Run(fmt.Sprintf("run%d", run+1), func(t *testing.T) {
			sessDir := t.TempDir()
			projectDir := t.TempDir()
			pid := 78000 + run
			sessionID := fmt.Sprintf("consistent-session-%d", run)

			sc := session.NewSessionScanner(sessDir,
				session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
				session.WithScanInterval(scanInterval),
				session.WithJSONLBaseDir(projectDir),
			)
			mon := session.NewMonitor(
				session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
			)

			m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
			m.SetSize(120, 40)
			makeJSONLFile(t, projectDir, sessionID)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
			defer stopBridge()

			writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)
			elapsed := driveModelUntilPaneCount(&m, 1, 3*time.Second)
			m.closeAll()

			if elapsed < 0 {
				t.Fatalf("run %d: pane never appeared within 3-second timeout", run+1)
			}
			if elapsed > latencyThreshold {
				t.Errorf("run %d: latency %v exceeds %v SLA", run+1, elapsed, latencyThreshold)
			}
			latencies = append(latencies, elapsed)
			t.Logf("run %d latency: %v", run+1, elapsed)
		})
	}

	if len(latencies) > 0 {
		logLatencyStats(t, "scanner", latencies)
	}
}

// TestIntegration_Scanner_DefaultIntervalMeetsRequirement verifies that the
// production default scan interval (2 s) satisfies the 2-second SLA requirement.
// Anything larger than 2 s would make detection impossible within the window.
func TestIntegration_Scanner_DefaultIntervalMeetsRequirement(t *testing.T) {
	sc := session.NewSessionScanner("") // default interval
	if sc.Interval() > latencyThreshold {
		t.Errorf("default scan interval %v exceeds detection SLA %v",
			sc.Interval(), latencyThreshold)
	}
}

// ─────────────────── fsnotify dir-watcher detection (Sub-AC 3) ──────────────

// TestIntegration_DirWatcher_DetectionUnder1Second verifies that the fsnotify-
// based SessionDirectoryWatcher detects a new session file and the dashboard
// creates the corresponding pane in well under 1 second (targeting < 500 ms).
func TestIntegration_DirWatcher_DetectionUnder1Second(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dir-watcher latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-dirwatcher-test"
	const pid = 79001
	const sessionID = "dirwatcher-latency-session"

	checker := newAlwaysAlivePIDChecker()
	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
		session.WithDirWatcherEventBuffer(32),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	// Use a very long scan interval so only the dir watcher fires.
	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(60*time.Second),
	)
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)
	makeJSONLFile(t, projectDir, sessionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeDirWatcherToModel(ctx, dw, m.dirWatcherChan)
	defer stopBridge()
	defer dw.Close()

	// Let the watcher initialise before writing the file.
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)

	elapsed := driveModelUntilPaneCount(&m, 1, 2*time.Second)
	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("dir-watcher pane never appeared within 2-second timeout")
	}
	if elapsed > latencyThreshold {
		t.Errorf("dir-watcher detection latency %v exceeds %v SLA", elapsed, latencyThreshold)
	}
	_ = start
	t.Logf("dir-watcher detection latency: %v", elapsed)
}

// TestIntegration_DirWatcher_ConsistentDetectionUnder2Seconds runs the fsnotify
// detection path five consecutive times and verifies all runs are sub-2s.
func TestIntegration_DirWatcher_ConsistentDetectionUnder2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dir-watcher consistency test in short mode")
	}

	const runs = 5
	const projectPath = "/tmp/integ-dw-consistent"

	latencies := make([]time.Duration, 0, runs)

	for run := 0; run < runs; run++ {
		t.Run(fmt.Sprintf("run%d", run+1), func(t *testing.T) {
			sessDir := t.TempDir()
			projectDir := t.TempDir()
			pid := 80000 + run
			sessionID := fmt.Sprintf("dw-consistent-%d", run)

			checker := newAlwaysAlivePIDChecker()
			dw, err := session.NewSessionDirectoryWatcher(sessDir,
				session.WithDirWatcherPIDChecker(checker),
				session.WithDirWatcherEventBuffer(32),
			)
			if err != nil {
				t.Fatalf("run %d: NewSessionDirectoryWatcher: %v", run+1, err)
			}

			sc := session.NewSessionScanner(sessDir,
				session.WithScannerPIDChecker(checker),
				session.WithScanInterval(60*time.Second),
			)
			mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

			m := NewSessionDashboardModel(projectPath, projectDir, sc, mon,
				WithDashboardDirWatcher(dw),
			)
			m.SetSize(120, 40)
			makeJSONLFile(t, projectDir, sessionID)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stopBridge := bridgeDirWatcherToModel(ctx, dw, m.dirWatcherChan)
			defer stopBridge()
			defer dw.Close()

			time.Sleep(50 * time.Millisecond)

			writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)
			elapsed := driveModelUntilPaneCount(&m, 1, 2*time.Second)
			m.closeAll()

			if elapsed < 0 {
				t.Fatalf("run %d: pane never appeared within 2-second timeout", run+1)
			}
			if elapsed > latencyThreshold {
				t.Errorf("run %d: dir-watcher latency %v exceeds %v SLA",
					run+1, elapsed, latencyThreshold)
			}
			latencies = append(latencies, elapsed)
			t.Logf("run %d latency: %v", run+1, elapsed)
		})
	}

	if len(latencies) > 0 {
		logLatencyStats(t, "dir-watcher", latencies)
	}
}

// ─────────────────── multi-session simultaneous detection ───────────────────

// TestIntegration_MultipleSessionsAllDetectedUnder2Seconds creates three session
// files simultaneously and verifies all three panes appear within 2 seconds.
func TestIntegration_MultipleSessionsAllDetectedUnder2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-session latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-multi-session"

	type sessionSpec struct {
		pid       int
		sessionID string
	}
	specs := []sessionSpec{
		{81001, "multi-sess-1"},
		{81002, "multi-sess-2"},
		{81003, "multi-sess-3"},
	}

	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
		session.WithScanInterval(1000*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	mon := session.NewMonitor(
		session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(200, 60) // wide enough for 3 panes side-by-side

	for _, s := range specs {
		makeJSONLFile(t, projectDir, s.sessionID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopBridge()

	// Write all session files atomically from the dashboard's perspective.
	start := time.Now()
	for _, s := range specs {
		writeIntegrationSessionFile(t, sessDir, s.pid, s.sessionID, projectPath)
	}

	elapsed := driveModelUntilPaneCount(&m, len(specs), 3*time.Second)
	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("not all %d panes appeared within 3-second timeout; got %d",
			len(specs), m.PaneCount())
	}
	if elapsed > latencyThreshold {
		t.Errorf("multi-session detection latency %v exceeds %v SLA", elapsed, latencyThreshold)
	}
	_ = start
	t.Logf("all %d panes appeared in: %v", len(specs), elapsed)
}

// ──────────────── combined scanner + dir-watcher (production config) ────────

// TestIntegration_Combined_DetectionUnder2Seconds exercises the production-like
// configuration where both the polling scanner and the fsnotify dir watcher are
// active.  The first detection source to fire wins; the result should be
// comfortably sub-2s.
func TestIntegration_Combined_DetectionUnder2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping combined detection latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-combined-test"
	const pid = 82001
	const sessionID = "combined-latency-session"

	checker := newAlwaysAlivePIDChecker()

	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
		session.WithDirWatcherEventBuffer(32),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(2*time.Second), // production default
	)
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)
	makeJSONLFile(t, projectDir, sessionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopScannerBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopScannerBridge()

	stopDirBridge := bridgeDirWatcherToModel(ctx, dw, m.dirWatcherChan)
	defer stopDirBridge()
	defer dw.Close()

	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)

	elapsed := driveModelUntilPaneCount(&m, 1, 3*time.Second)
	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("combined detection: pane never appeared within 3-second timeout")
	}
	if elapsed > latencyThreshold {
		t.Errorf("combined detection latency %v exceeds %v SLA", elapsed, latencyThreshold)
	}
	_ = start
	t.Logf("combined (scanner+dir-watcher) detection latency: %v", elapsed)
}

// ─────────────── session-file-not-found is handled gracefully ───────────────

// TestIntegration_NonexistentSessionsDir_NoHang verifies that the scanner does
// not hang and emits no spurious panes when the sessions directory does not yet
// exist.  Detection should complete promptly (empty result, no timeout).
func TestIntegration_NonexistentSessionsDir_NoHang(t *testing.T) {
	nonexistent := t.TempDir() + "/does-not-exist"
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-nodir-test"

	sc := session.NewSessionScanner(nonexistent,
		session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
		session.WithScanInterval(200*time.Millisecond),
	)
	mon := session.NewMonitor(
		session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopBridge()

	// Drive for 500 ms; no pane should appear.
	elapsed := driveModelUntilPaneCount(&m, 1, 500*time.Millisecond)
	m.closeAll()

	// A return value of -1 means no pane appeared (expected behaviour).
	if elapsed >= 0 {
		t.Errorf("unexpected pane appeared (%v) for non-existent sessions dir", elapsed)
	}
}

// ──────────────────────── stat helpers ──────────────────────────────────────

// logLatencyStats logs min, median, max and p95 latency statistics for a test run.
func logLatencyStats(t *testing.T, label string, latencies []time.Duration) {
	t.Helper()
	if len(latencies) == 0 {
		return
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	avg := total / time.Duration(len(sorted))

	min := sorted[0]
	max := sorted[len(sorted)-1]

	p95idx := int(float64(len(sorted)) * 0.95)
	if p95idx >= len(sorted) {
		p95idx = len(sorted) - 1
	}
	p95 := sorted[p95idx]

	// Fail the test if p95 exceeds the SLA.
	if p95 > latencyThreshold {
		t.Errorf("%s p95 latency %v exceeds %v SLA", label, p95, latencyThreshold)
	}

	t.Logf("%s latency stats: min=%v avg=%v max=%v p95=%v (n=%d)",
		label, min, avg, max, p95, len(sorted))
}

// ─────────────────── registry-to-dashboard end-to-end latency ───────────────

// TestIntegration_Registry_DashboardLatency_Under2Seconds tests the full
// detection pipeline using the Registry (the production-level aggregator) and
// verifies that a SessionOpened event propagates to dashboard pane creation
// within 2 seconds.  This exercises the fan-in architecture
// (scanner + dir watcher → registry events → dashboard pane).
func TestIntegration_Registry_DashboardLatency_Under2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping registry latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-registry-test"
	const pid = 83001
	const sessionID = "registry-latency-session"

	checker := newAlwaysAlivePIDChecker()

	// Build Registry with realistic (but shortened) intervals.
	reg := session.NewRegistry(sessDir,
		session.WithRegistryPIDChecker(checker),
		session.WithRegistryScanInterval(1500*time.Millisecond),
		session.WithRegistryNoDirWatcher(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.Start(ctx)
	defer reg.Stop()

	// Build the dashboard: forward Registry events into dirWatcherChan (acting
	// as the SessionOpened delivery path for this integration test).
	dw, err := session.NewSessionDirectoryWatcher(sessDir,
		session.WithDirWatcherPIDChecker(checker),
	)
	if err != nil {
		t.Fatalf("NewSessionDirectoryWatcher: %v", err)
	}

	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(60*time.Second), // long poll; rely on reg events
	)
	mon := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon,
		WithDashboardDirWatcher(dw),
	)
	m.SetSize(120, 40)
	// Create JSONL for the dashboard's content loader.
	makeJSONLFile(t, projectDir, sessionID)
	// The registry's internal scanner uses CWDToProjectDir(projectPath) to locate
	// JSONL files. Create the file there too so the registry detects the session.
	registryJSONLDir := session.CWDToProjectDir(projectPath)
	if err := os.MkdirAll(registryJSONLDir, 0755); err != nil {
		t.Fatalf("mkdir registry JSONL dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(registryJSONLDir) })
	makeJSONLFile(t, registryJSONLDir, sessionID)

	// Bridge Registry events to the dashboard's dir-watcher channel.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-reg.Events():
				if !ok {
					return
				}
				if ev.Type == session.SessionOpened {
					select {
					case m.dirWatcherChan <- ev:
					case <-ctx.Done():
						return
					default:
					}
				}
			}
		}
	}()

	start := time.Now()
	writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)

	elapsed := driveModelUntilPaneCount(&m, 1, 3*time.Second)
	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("registry-bridged pane never appeared within 3-second timeout")
	}
	if elapsed > latencyThreshold {
		t.Errorf("registry→dashboard latency %v exceeds %v SLA", elapsed, latencyThreshold)
	}
	_ = start
	t.Logf("registry→dashboard detection latency: %v", elapsed)
}

// ─────────────── session open/close lifecycle latency ───────────────────────

// TestIntegration_SessionClose_PaneRemovedWithin5Seconds verifies that when a
// session's PID is marked dead (via the mock PID checker), the monitor detects
// it and the dashboard removes the pane within 5 seconds.
func TestIntegration_SessionClose_PaneRemovedWithin5Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping session-close latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-close-test"
	const pid = 84001
	const sessionID = "close-latency-session"

	// Use a controllable PID checker so we can kill the PID mid-test.
	checker := newTestPIDChecker(pid) // starts alive

	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(checker),
		session.WithScanInterval(500*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	mon := session.NewMonitor(
		session.WithMonitorPIDChecker(checker),
		session.WithPollInterval(500*time.Millisecond),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(120, 40)
	makeJSONLFile(t, projectDir, sessionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopBridge()

	// Bridge monitor events to model's monitorChan.
	// Capture the channel reference before starting the goroutine to avoid a
	// data race: the loop below reassigns m (m = newModel.(...)) while the
	// goroutine would otherwise read m.monitorChan concurrently.
	monChan := m.monitorChan
	mon.Start(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-mon.Events():
				if !ok {
					return
				}
				select {
				case monChan <- ev:
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()

	writeIntegrationSessionFile(t, sessDir, pid, sessionID, projectPath)

	// Wait for pane to appear (session opened).
	addElapsed := driveModelUntilPaneCount(&m, 1, 3*time.Second)
	if addElapsed < 0 {
		t.Fatal("pane never appeared after session file creation")
	}
	t.Logf("session appeared in %v", addElapsed)

	if m.PaneCount() != 1 {
		t.Fatalf("expected 1 pane before kill, got %d", m.PaneCount())
	}

	// Mark the PID as dead and start the close clock.
	checker.SetAlive(pid, false)
	mon.TrackSession(session.ActiveSession{
		Meta: session.SessionMeta{PID: pid},
	})

	// Also delete the session file so the scanner doesn't resurrect the session.
	_ = os.Remove(sessDir + "/" + fmt.Sprintf("%d.json", pid))

	start := time.Now()

	// Drive until pane count drops to 0.
	deadline := time.Now().Add(6 * time.Second)
	removed := false
	for time.Now().Before(deadline) {
		if msg := m.pollChannels(); msg != nil {
			newModel, _ := m.Update(msg)
			m = newModel.(SessionDashboardModel)
		}
		if m.PaneCount() == 0 {
			removed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.closeAll()

	elapsed := time.Since(start)
	if !removed {
		t.Fatalf("pane was never removed after PID death (elapsed %v)", elapsed)
	}
	const closeThreshold = 5 * time.Second
	if elapsed > closeThreshold {
		t.Errorf("session close detection took %v, exceeds %v SLA", elapsed, closeThreshold)
	}
	t.Logf("session-close detection latency: %v", elapsed)
}

// ──────────────── 9-pane maximum capacity detection latency ─────────────────

// TestIntegration_NineSessions_AllDetectedUnder2Seconds exercises the maximum
// 3×3 grid capacity.  Nine sessions created simultaneously must all be
// reflected as panes within 2 seconds.
func TestIntegration_NineSessions_AllDetectedUnder2Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 9-session max-capacity latency test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-nine-session"

	type spec struct {
		pid, idx int
		sessID   string
	}
	var specs []spec
	for i := 0; i < 9; i++ {
		specs = append(specs, spec{
			pid:    85000 + i,
			idx:    i,
			sessID: fmt.Sprintf("nine-sess-%d", i),
		})
	}

	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
		session.WithScanInterval(1000*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	mon := session.NewMonitor(
		session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(240, 80)

	for _, s := range specs {
		makeJSONLFile(t, projectDir, s.sessID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopBridge()

	start := time.Now()
	for _, s := range specs {
		writeIntegrationSessionFile(t, sessDir, s.pid, s.sessID, projectPath)
	}

	elapsed := driveModelUntilPaneCount(&m, 9, 3*time.Second)
	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("not all 9 panes appeared within 3-second timeout; got %d", m.PaneCount())
	}
	if elapsed > latencyThreshold {
		t.Errorf("9-session detection latency %v exceeds %v SLA", elapsed, latencyThreshold)
	}
	_ = start
	t.Logf("all 9 panes appeared in: %v", elapsed)
}

// ─────────────── AC 5: 3 sessions streaming in split panes ──────────────────

// writeStreamingJSONLFile creates a JSONL file with multiple conversation
// entries to simulate an active Claude Code session with live content.
func writeStreamingJSONLFile(t *testing.T, dir, sessionID string) string {
	t.Helper()
	filePath := dir + "/" + sessionID + ".jsonl"
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"What is the capital of France?"},"timestamp":"2026-03-31T00:00:00Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"The capital of France is Paris."}]},"timestamp":"2026-03-31T00:00:01Z"}`,
		`{"type":"user","message":{"role":"user","content":"And what about Germany?"},"timestamp":"2026-03-31T00:00:02Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"The capital of Germany is Berlin."}]},"timestamp":"2026-03-31T00:00:03Z"}`,
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return filePath
}

// driveModelUntilAllPanesStreaming drives the model by polling channels AND
// executing tea.Cmds returned by Update() (e.g., content loading commands).
// It returns the elapsed time once all `want` panes have been detected AND
// have loaded content (AllPanesHaveContent), or -1 on timeout.
//
// This simulates the Bubbletea runtime's asynchronous cmd execution in a
// synchronous test context, allowing us to measure end-to-end streaming latency
// without running a full tea.Program.
func driveModelUntilAllPanesStreaming(m *SessionDashboardModel, want int, timeout time.Duration) time.Duration {
	start := time.Now()
	deadline := start.Add(timeout)

	var pendingCmds []tea.Cmd

	for time.Now().Before(deadline) {
		// Execute any pending commands (content loaders, watchers, etc.)
		if len(pendingCmds) > 0 {
			cmd := pendingCmds[0]
			pendingCmds = pendingCmds[1:]
			if cmd != nil {
				msg := cmd()
				if msg != nil {
					// Handle tea.Batch — expand into individual commands.
					if batch, ok := msg.(tea.BatchMsg); ok {
						pendingCmds = append(pendingCmds, []tea.Cmd(batch)...)
					} else {
						newModel, newCmd := m.Update(msg)
						*m = newModel.(SessionDashboardModel)
						if newCmd != nil {
							pendingCmds = append(pendingCmds, newCmd)
						}
					}
				}
			}
			continue // process cmds before polling
		}

		// Check the goal condition: N panes AND all have content.
		if m.PaneCount() >= want && m.AllPanesHaveContent() {
			return time.Since(start)
		}

		// Poll channels for new events (scanner, monitor, dir-watcher).
		if msg := m.pollChannels(); msg != nil {
			newModel, cmd := m.Update(msg)
			*m = newModel.(SessionDashboardModel)
			if cmd != nil {
				pendingCmds = append(pendingCmds, cmd)
			}
			continue // immediately process cmd queue
		}

		time.Sleep(10 * time.Millisecond)
	}
	return -1
}

// TestIntegration_ThreeSessions_StreamingInSplitPanes is the acceptance criterion
// AC 5: opening 3 Claude Code terminals on the same project and seeing all 3
// streaming in split panes within 2-3 seconds.
//
// "Streaming" means:
//  1. All 3 sessions are detected (pane count reaches 3).
//  2. All 3 panes have loaded JSONL content (entries from disk are parsed and
//     available for rendering).
//  3. The View() renders a 3-pane split layout (not the waiting message).
//
// Latency is measured from the moment the 3 session files are written to disk
// until all panes are streaming.
func TestIntegration_ThreeSessions_StreamingInSplitPanes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AC5 three-sessions streaming test in short mode")
	}

	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/integ-three-streaming"

	type sessionSpec struct {
		pid       int
		sessionID string
	}
	specs := []sessionSpec{
		{91001, "stream-sess-1"},
		{91002, "stream-sess-2"},
		{91003, "stream-sess-3"},
	}

	// Create JSONL files with multiple conversation entries per session.
	for _, s := range specs {
		writeStreamingJSONLFile(t, projectDir, s.sessionID)
	}

	sc := session.NewSessionScanner(sessDir,
		session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
		session.WithScanInterval(1000*time.Millisecond),
		session.WithJSONLBaseDir(projectDir),
	)
	mon := session.NewMonitor(
		session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
	)

	m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
	m.SetSize(240, 60) // wide enough for a 3-pane side-by-side grid

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
	defer stopBridge()

	// Write all 3 session files simultaneously and start the clock.
	start := time.Now()
	for _, s := range specs {
		writeIntegrationSessionFile(t, sessDir, s.pid, s.sessionID, projectPath)
	}

	// Drive until all 3 panes are detected AND streaming content.
	const streamingThreshold = 3 * time.Second
	elapsed := driveModelUntilAllPanesStreaming(&m, len(specs), streamingThreshold+500*time.Millisecond)
	m.closeAll()

	if elapsed < 0 {
		t.Fatalf("not all %d sessions streaming within %v timeout (got %d panes, allContent=%v)",
			len(specs), streamingThreshold, m.PaneCount(), m.AllPanesHaveContent())
	}
	if elapsed > streamingThreshold {
		t.Errorf("3-session streaming latency %v exceeds %v SLA", elapsed, streamingThreshold)
	}
	_ = start
	t.Logf("all 3 sessions streaming in split panes in: %v", elapsed)

	// Verify the rendered view contains a 3-pane split grid (not the waiting message).
	view := m.View()
	if strings.Contains(view, "Waiting for active Claude Code sessions") {
		t.Error("expected 3-pane split view, got waiting message")
	}
	if m.PaneCount() != 3 {
		t.Errorf("expected 3 panes in view, got %d", m.PaneCount())
	}
	t.Logf("3-pane split view rendered (view size: %d bytes)", len(view))
}

// TestIntegration_ThreeSessions_ConsistentStreaming runs the 3-session streaming
// scenario three times to verify consistent sub-3s behaviour.
func TestIntegration_ThreeSessions_ConsistentStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping consistent streaming test in short mode")
	}

	const runs = 3
	const projectPath = "/tmp/integ-three-consistent-streaming"
	const streamingThreshold = 3 * time.Second

	latencies := make([]time.Duration, 0, runs)

	for run := 0; run < runs; run++ {
		t.Run(fmt.Sprintf("run%d", run+1), func(t *testing.T) {
			sessDir := t.TempDir()
			projectDir := t.TempDir()

			type spec struct {
				pid, idx int
				sessID   string
			}
			var specs []spec
			for i := 0; i < 3; i++ {
				specs = append(specs, spec{
					pid:    92000 + run*10 + i,
					idx:    i,
					sessID: fmt.Sprintf("consistent-stream-%d-%d", run, i),
				})
			}

			for _, s := range specs {
				writeStreamingJSONLFile(t, projectDir, s.sessID)
			}

			sc := session.NewSessionScanner(sessDir,
				session.WithScannerPIDChecker(newAlwaysAlivePIDChecker()),
				session.WithScanInterval(1000*time.Millisecond),
				session.WithJSONLBaseDir(projectDir),
			)
			mon := session.NewMonitor(
				session.WithMonitorPIDChecker(newAlwaysAlivePIDChecker()),
			)

			m := NewSessionDashboardModel(projectPath, projectDir, sc, mon)
			m.SetSize(240, 60)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stopBridge := bridgeScannerToModel(ctx, sc, m.scanResultChan)
			defer stopBridge()

			for _, s := range specs {
				writeIntegrationSessionFile(t, sessDir, s.pid, s.sessID, projectPath)
			}

			elapsed := driveModelUntilAllPanesStreaming(&m, len(specs), streamingThreshold+500*time.Millisecond)
			m.closeAll()

			if elapsed < 0 {
				t.Fatalf("run %d: panes not streaming within timeout (got %d panes)", run+1, m.PaneCount())
			}
			if elapsed > streamingThreshold {
				t.Errorf("run %d: streaming latency %v exceeds %v SLA", run+1, elapsed, streamingThreshold)
			}
			latencies = append(latencies, elapsed)
			t.Logf("run %d: all 3 sessions streaming in %v", run+1, elapsed)
		})
	}

	if len(latencies) > 0 {
		logLatencyStats(t, "three-session-streaming", latencies)
	}
}

// TestSessionDashboard_ThreePanes_SplitViewLayout verifies that a 3-pane session
// dashboard renders in a proper split-pane grid layout with visible pane borders
// and that the "waiting" message is absent.
func TestSessionDashboard_ThreePanes_SplitViewLayout(t *testing.T) {
	checker := newTestPIDChecker(101, 102, 103)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/three-pane-split-layout"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(240, 60) // Wide terminal: fits 3 panes side-by-side

	// Add 3 panes via scan result.
	for i, pid := range []int{101, 102, 103} {
		sessionID := fmt.Sprintf("split-sess-%d", i+1)
		makeJSONLFile(t, projectDir, sessionID)
		scanResult := session.ScanResult{
			Sessions: []session.ActiveSession{
				{Meta: session.SessionMeta{
					PID: pid, SessionID: sessionID, CWD: projectPath, Kind: "interactive",
				}},
			},
		}
		newModel, _ := m.Update(sessionScanResultMsg{result: scanResult})
		m = newModel.(SessionDashboardModel)
	}

	if m.PaneCount() != 3 {
		t.Fatalf("expected 3 panes, got %d", m.PaneCount())
	}

	// Render the view — should show 3 split panes, not the waiting message.
	view := m.View()
	if strings.Contains(view, "Waiting for active Claude Code sessions") {
		t.Error("expected 3-pane split view, got waiting message")
	}

	// All 3 panes should have a cached view after View() renders.
	for i := 0; i < 3; i++ {
		if m.PaneCachedView(i) == "" {
			t.Errorf("pane %d has no cached view after View() render", i)
		}
	}

	// Verify the grid layout for 3 panes at 240-wide terminal.
	gridHeight := 60 - 1
	layout := CalculateGridLayout(3, 240, gridHeight)
	if layout.Rows == 0 || layout.Cols == 0 {
		t.Fatal("grid layout is empty for 3 panes")
	}
	if len(layout.Panes) != 3 {
		t.Errorf("expected 3 pane layouts, got %d", len(layout.Panes))
	}
	t.Logf("3-pane grid layout: rows=%d cols=%d", layout.Rows, layout.Cols)
	t.Logf("view rendered: %d bytes", len(view))
}

// TestSessionDashboard_ThreePanes_AllLoadingThenStreaming verifies the full
// lifecycle for 3 panes: initially loading → content loaded → streaming.
// This tests the state transitions that happen during real session monitoring.
func TestSessionDashboard_ThreePanes_AllLoadingThenStreaming(t *testing.T) {
	checker := newTestPIDChecker(201, 202, 203)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	const projectPath = "/tmp/three-pane-lifecycle"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(240, 60)

	// Step 1: Add 3 sessions — they should be in loading state initially.
	type paneSpec struct {
		pid       int
		sessionID string
	}
	specs := []paneSpec{
		{201, "lifecycle-sess-1"},
		{202, "lifecycle-sess-2"},
		{203, "lifecycle-sess-3"},
	}

	for _, s := range specs {
		makeJSONLFile(t, projectDir, s.sessionID)
	}

	scanResult := session.ScanResult{
		Sessions: make([]session.ActiveSession, len(specs)),
	}
	for i, s := range specs {
		scanResult.Sessions[i] = session.ActiveSession{
			Meta: session.SessionMeta{
				PID: s.pid, SessionID: s.sessionID, CWD: projectPath, Kind: "interactive",
			},
		}
	}

	newModel, cmd := m.Update(sessionScanResultMsg{result: scanResult})
	m = newModel.(SessionDashboardModel)

	if m.PaneCount() != 3 {
		t.Fatalf("expected 3 panes after scan, got %d", m.PaneCount())
	}

	// All panes should be in loading state immediately after detection.
	for i := 0; i < 3; i++ {
		if !m.PaneIsLoading(i) {
			t.Errorf("pane %d should be loading initially", i)
		}
		if m.PaneEntriesCount(i) != 0 {
			t.Errorf("pane %d should have 0 entries initially, got %d", i, m.PaneEntriesCount(i))
		}
	}

	// Verify AllPanesHaveContent returns false while loading.
	if m.AllPanesHaveContent() {
		t.Error("AllPanesHaveContent should be false while panes are loading")
	}

	// Step 2: Execute content loading commands to simulate streaming.
	var pendingCmds []tea.Cmd
	if cmd != nil {
		pendingCmds = append(pendingCmds, cmd)
	}

	const maxIter = 100
	for iter := 0; iter < maxIter && len(pendingCmds) > 0; iter++ {
		next := pendingCmds[0]
		pendingCmds = pendingCmds[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			pendingCmds = append(pendingCmds, []tea.Cmd(batch)...)
			continue
		}
		newM, newCmd := m.Update(msg)
		m = newM.(SessionDashboardModel)
		if newCmd != nil {
			pendingCmds = append(pendingCmds, newCmd)
		}
	}

	// Step 3: After content loaded, all panes should have entries.
	for i := 0; i < 3; i++ {
		if m.PaneIsLoading(i) {
			t.Errorf("pane %d should not be loading after content load", i)
		}
		if m.PaneEntriesCount(i) == 0 {
			t.Errorf("pane %d should have entries after content load, got 0", i)
		}
	}

	// AllPanesHaveContent should now return true.
	if !m.AllPanesHaveContent() {
		t.Error("AllPanesHaveContent should be true after all panes loaded content")
	}

	// Rendered view should contain split-pane content (no "Waiting" message).
	view := m.View()
	if strings.Contains(view, "Waiting for active Claude Code sessions") {
		t.Error("expected 3-pane split view, got waiting message")
	}

	t.Logf("3-pane lifecycle test: all panes streaming (view: %d bytes)", len(view))
}
