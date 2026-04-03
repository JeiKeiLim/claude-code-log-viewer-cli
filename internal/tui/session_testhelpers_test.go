package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
)

// ─────────────────────────────────────────────────────────────────────────────
// Shared session-file creation helpers for TUI tests
// ─────────────────────────────────────────────────────────────────────────────

// makeTestSessionFile creates a {pid}.json session file in dir.
// When cwd and sessionID are both non-empty it also creates the corresponding
// JSONL log file in the scanner-derived location so the scanner's
// JSONL-existence check passes.
func makeTestSessionFile(t *testing.T, dir string, pid int, sessionID, cwd string) string {
	t.Helper()
	meta := session.SessionMeta{
		PID:       pid,
		SessionID: sessionID,
		CWD:       cwd,
		StartedAt: time.Now().UnixMilli(),
		Kind:      "interactive",
	}
	filePath := writeTestSessionJSON(t, dir, pid, meta)

	// Create the JSONL log file in the scanner-derived location so the
	// scanner includes this session (JSONL existence is required since AC2).
	if cwd != "" && sessionID != "" {
		jsonlDir := session.CWDToProjectDir(cwd)
		if jsonlDir != "" {
			if mkErr := os.MkdirAll(jsonlDir, 0755); mkErr == nil {
				jsonlPath := filepath.Join(jsonlDir, sessionID+".jsonl")
				_ = os.WriteFile(jsonlPath, []byte("{}\n"), 0644)
				t.Cleanup(func() {
					_ = os.Remove(jsonlPath)
					_ = os.Remove(jsonlDir) // best-effort; only removes if empty
				})
			}
		}
	}

	return filePath
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared mock PID checker for TUI tests
// ─────────────────────────────────────────────────────────────────────────────

// testPIDChecker is a controllable PID checker for tests.
// It is safe for concurrent use: IsAlive and SetAlive hold a mutex.
// When allAlive is true, IsAlive always returns true regardless of the map.
type testPIDChecker struct {
	mu       sync.RWMutex
	alive    map[int]bool
	allAlive bool
}

func newTestPIDChecker(pids ...int) *testPIDChecker {
	c := &testPIDChecker{alive: make(map[int]bool)}
	for _, pid := range pids {
		c.alive[pid] = true
	}
	return c
}

// newAlwaysAlivePIDChecker returns a testPIDChecker that reports every PID as alive.
func newAlwaysAlivePIDChecker() *testPIDChecker {
	return &testPIDChecker{alive: make(map[int]bool), allAlive: true}
}

func (c *testPIDChecker) IsAlive(pid int) bool {
	if c.allAlive {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.alive[pid]
}

func (c *testPIDChecker) SetAlive(pid int, alive bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.alive[pid] = alive
}

// makeJSONLFile creates a minimal JSONL file for a session.
func makeJSONLFile(t *testing.T, dir, sessionID string) string {
	t.Helper()
	filePath := filepath.Join(dir, sessionID+".jsonl")
	content := `{"type":"user","message":{"role":"user","content":"hello"},"timestamp":"2026-03-31T00:00:00Z"}` + "\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return filePath
}

// writeTestSessionJSON writes a {pid}.json session file to dir and returns its path.
// This is the lower-level helper that only writes the JSON metadata file
// without creating the companion JSONL file.
func writeTestSessionJSON(t *testing.T, dir string, pid int, meta session.SessionMeta) string {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal session meta: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	return path
}

// scanMissThreshold matches the constant in session_dashboard.go.
// Tests that need to trigger pane removal must send this many consecutive
// scans with the PID absent.
const testScanMissThreshold = 3

// applyScanResultNTimes sends the same scan result msg N times through
// the dashboard's Update loop. Used to satisfy the grace-period miss
// counter before a pane is actually removed.
func applyScanResultNTimes(t *testing.T, m SessionDashboardModel, msg sessionScanResultMsg, n int) SessionDashboardModel {
	t.Helper()
	for i := 0; i < n; i++ {
		newModel, _ := m.Update(msg)
		m = newModel.(SessionDashboardModel)
	}
	return m
}
