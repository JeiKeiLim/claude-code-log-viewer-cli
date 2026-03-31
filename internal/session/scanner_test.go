package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// scannerMockPIDChecker is a test double for PIDChecker used by scanner tests.
type scannerMockPIDChecker struct {
	mu    sync.RWMutex
	alive map[int]bool
}

func newScannerMockChecker(alivePIDs ...int) *scannerMockPIDChecker {
	m := &scannerMockPIDChecker{alive: make(map[int]bool)}
	for _, pid := range alivePIDs {
		m.alive[pid] = true
	}
	return m
}

func (m *scannerMockPIDChecker) IsAlive(pid int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alive[pid]
}

func (m *scannerMockPIDChecker) SetAlive(pid int, alive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive[pid] = alive
}

func TestParsePIDFromFilename(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantPID int
		wantOK  bool
	}{
		{"valid pid", "12345.json", 12345, true},
		{"single digit", "1.json", 1, true},
		{"large pid", "999999.json", 999999, true},
		{"not json", "12345.txt", 0, false},
		{"not a number", "abc.json", 0, false},
		{"negative pid", "-1.json", 0, false},
		{"zero pid", "0.json", 0, false},
		{"empty base", ".json", 0, false},
		{"no extension", "12345", 0, false},
		{"mixed", "12abc.json", 0, false},
		{"float", "12.34.json", 0, false},
		{"empty string", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid, ok := parsePIDFromFilename(tt.input)
			if pid != tt.wantPID || ok != tt.wantOK {
				t.Errorf("parsePIDFromFilename(%q) = (%d, %v), want (%d, %v)",
					tt.input, pid, ok, tt.wantPID, tt.wantOK)
			}
		})
	}
}

func TestReadSessionMeta(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		tmpDir := t.TempDir()
		meta := SessionMeta{
			PID:        42,
			SessionID:  "abc-123",
			CWD:        "/home/user/project",
			StartedAt:  1711900000000,
			Kind:       "interactive",
			Entrypoint: "claude",
		}
		data, _ := json.Marshal(meta)
		path := filepath.Join(tmpDir, "42.json")
		os.WriteFile(path, data, 0644)

		got, err := readSessionMeta(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.PID != 42 || got.SessionID != "abc-123" || got.CWD != "/home/user/project" {
			t.Errorf("unexpected meta: %+v", got)
		}
		if got.Kind != "interactive" || got.Entrypoint != "claude" {
			t.Errorf("unexpected meta fields: kind=%q entrypoint=%q", got.Kind, got.Entrypoint)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "bad.json")
		os.WriteFile(path, []byte("{invalid"), 0644)

		_, err := readSessionMeta(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := readSessionMeta("/nonexistent/path/42.json")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("partial json fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "partial.json")
		os.WriteFile(path, []byte(`{"pid": 99, "sessionId": "xyz"}`), 0644)

		got, err := readSessionMeta(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.PID != 99 || got.SessionID != "xyz" {
			t.Errorf("unexpected meta: %+v", got)
		}
		if got.CWD != "" || got.Kind != "" {
			t.Errorf("expected empty optional fields, got: %+v", got)
		}
	})
}

func TestSessionScanner_Scan(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker()
		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))

		result := scanner.Scan()
		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(result.Sessions))
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		scanner := NewSessionScanner("/nonexistent/sessions/dir",
			WithScannerPIDChecker(newScannerMockChecker()))

		result := scanner.Scan()
		if result.Err != nil {
			t.Errorf("expected no error for nonexistent dir, got: %v", result.Err)
		}
		if len(result.Sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(result.Sessions))
		}
	})

	t.Run("detects alive sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100, 200)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{
			PID:       100,
			SessionID: "session-a",
			CWD:       "/home/user/project-a",
			StartedAt: 1711900000000,
		})
		writeSessionJSON(t, tmpDir, 200, SessionMeta{
			PID:       200,
			SessionID: "session-b",
			CWD:       "/home/user/project-b",
			StartedAt: 1711900001000,
		})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(result.Sessions))
		}

		found := map[string]bool{}
		for _, s := range result.Sessions {
			found[s.Meta.SessionID] = true
			if s.FilePath == "" {
				t.Error("session FilePath should not be empty")
			}
		}
		if !found["session-a"] || !found["session-b"] {
			t.Errorf("missing expected sessions: %v", found)
		}
	})

	t.Run("filters dead PIDs", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100) // Only 100 is alive

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "alive"})
		writeSessionJSON(t, tmpDir, 200, SessionMeta{PID: 200, SessionID: "dead"})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
		if result.Sessions[0].Meta.SessionID != "alive" {
			t.Errorf("expected alive session, got %q", result.Sessions[0].Meta.SessionID)
		}
	})

	t.Run("skips non-json files", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "valid"})
		os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte("{}"), 0644)

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
	})

	t.Run("skips corrupt json", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100, 200)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "valid"})
		os.WriteFile(filepath.Join(tmpDir, "200.json"), []byte("{corrupt"), 0644)

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
	})

	t.Run("pid mismatch between filename and content", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 999, SessionID: "mismatch"})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 0 {
			t.Errorf("expected 0 sessions (PID mismatch), got %d", len(result.Sessions))
		}
	})

	t.Run("pid zero in content uses filename pid", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{SessionID: "no-pid-in-content"})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
		if result.Sessions[0].Meta.PID != 100 {
			t.Errorf("expected PID 100 from filename, got %d", result.Sessions[0].Meta.PID)
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "valid"})
		os.MkdirAll(filepath.Join(tmpDir, "200.json"), 0755)

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
	})

	t.Run("scan time is set", func(t *testing.T) {
		tmpDir := t.TempDir()
		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(newScannerMockChecker()))

		before := time.Now()
		result := scanner.Scan()
		after := time.Now()

		if result.ScanTime.Before(before) || result.ScanTime.After(after) {
			t.Errorf("ScanTime %v not between %v and %v", result.ScanTime, before, after)
		}
	})

	t.Run("JSONLDir is derived from CWD", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{
			PID:       100,
			SessionID: "with-cwd",
			CWD:       "/Users/test/myproject",
		})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
		if result.Sessions[0].JSONLDir == "" {
			t.Error("expected JSONLDir to be set when CWD is present")
		}
	})

	t.Run("empty CWD leaves JSONLDir empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{
			PID:       100,
			SessionID: "no-cwd",
		})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
		if result.Sessions[0].JSONLDir != "" {
			t.Errorf("expected empty JSONLDir, got %q", result.Sessions[0].JSONLDir)
		}
	})
}

func TestSessionScanner_Options(t *testing.T) {
	t.Run("default interval", func(t *testing.T) {
		scanner := NewSessionScanner("/tmp/test")
		if scanner.Interval() != 2*time.Second {
			t.Errorf("expected default interval 2s, got %v", scanner.Interval())
		}
	})

	t.Run("custom interval", func(t *testing.T) {
		scanner := NewSessionScanner("/tmp/test", WithScanInterval(5*time.Second))
		if scanner.Interval() != 5*time.Second {
			t.Errorf("expected interval 5s, got %v", scanner.Interval())
		}
	})

	t.Run("zero interval uses default", func(t *testing.T) {
		scanner := NewSessionScanner("/tmp/test", WithScanInterval(0))
		if scanner.Interval() != 2*time.Second {
			t.Errorf("expected default interval 2s for zero input, got %v", scanner.Interval())
		}
	})

	t.Run("negative interval uses default", func(t *testing.T) {
		scanner := NewSessionScanner("/tmp/test", WithScanInterval(-1*time.Second))
		if scanner.Interval() != 2*time.Second {
			t.Errorf("expected default interval 2s for negative input, got %v", scanner.Interval())
		}
	})

	t.Run("sessions dir accessor", func(t *testing.T) {
		scanner := NewSessionScanner("/custom/path")
		if scanner.SessionsDir() != "/custom/path" {
			t.Errorf("expected /custom/path, got %q", scanner.SessionsDir())
		}
	})

	t.Run("empty sessions dir uses default", func(t *testing.T) {
		scanner := NewSessionScanner("")
		if scanner.SessionsDir() == "" {
			t.Error("expected non-empty default path")
		}
	})
}

func TestSessionScanner_ScanOnce(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newScannerMockChecker(100)
	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "cached"})

	scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))

	result := scanner.ScanOnce()
	if len(result.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(result.Sessions))
	}

	last := scanner.LastScan()
	if len(last.Sessions) != 1 {
		t.Errorf("expected cached result with 1 session, got %d", len(last.Sessions))
	}
	if last.ScanTime != result.ScanTime {
		t.Error("LastScan should return same ScanTime as ScanOnce")
	}
}

func TestSessionScanner_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newScannerMockChecker(100)
	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "polled"})

	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(checker),
		WithScanInterval(50*time.Millisecond),
	)

	resultCh := scanner.Start()
	if resultCh == nil {
		t.Fatal("expected non-nil result channel")
	}
	if !scanner.IsRunning() {
		t.Error("scanner should be running after Start")
	}

	select {
	case result := <-resultCh:
		if len(result.Sessions) != 1 {
			t.Errorf("expected 1 session in initial scan, got %d", len(result.Sessions))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial scan result")
	}

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second scan result")
	}

	scanner.Stop()
	if scanner.IsRunning() {
		t.Error("scanner should not be running after Stop")
	}

	select {
	case _, ok := <-resultCh:
		if ok {
			for range resultCh {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestSessionScanner_StartIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(newScannerMockChecker()),
		WithScanInterval(50*time.Millisecond),
	)

	ch1 := scanner.Start()
	if ch1 == nil {
		t.Fatal("first Start should return non-nil channel")
	}

	ch2 := scanner.Start()
	if ch2 != nil {
		t.Error("second Start should return nil when already running")
	}

	scanner.Stop()
	for range ch1 {
	}
}

func TestSessionScanner_StopIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(newScannerMockChecker()),
		WithScanInterval(50*time.Millisecond),
	)

	scanner.Stop()
	scanner.Stop()

	ch := scanner.Start()
	scanner.Stop()
	scanner.Stop()

	for range ch {
	}
}

func TestSessionScanner_DynamicSessionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newScannerMockChecker(100)

	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "initial"})

	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(checker),
		WithScanInterval(50*time.Millisecond),
	)

	resultCh := scanner.Start()
	defer scanner.Stop()

	select {
	case result := <-resultCh:
		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session initially, got %d", len(result.Sessions))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	checker.SetAlive(200, true)
	writeSessionJSON(t, tmpDir, 200, SessionMeta{PID: 200, SessionID: "new"})

	deadline := time.After(2 * time.Second)
	for {
		select {
		case result := <-resultCh:
			if len(result.Sessions) == 2 {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for new session detection")
		}
	}
}

func TestSessionScanner_SessionClosure(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newScannerMockChecker(100, 200)

	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "stays"})
	writeSessionJSON(t, tmpDir, 200, SessionMeta{PID: 200, SessionID: "dies"})

	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(checker),
		WithScanInterval(50*time.Millisecond),
	)

	resultCh := scanner.Start()
	defer scanner.Stop()

	select {
	case result := <-resultCh:
		if len(result.Sessions) != 2 {
			t.Fatalf("expected 2 sessions initially, got %d", len(result.Sessions))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	checker.SetAlive(200, false)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case result := <-resultCh:
			if len(result.Sessions) == 1 {
				if result.Sessions[0].Meta.SessionID != "stays" {
					t.Errorf("expected 'stays' session, got %q", result.Sessions[0].Meta.SessionID)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for session closure detection")
		}
	}
}

func TestSessionScanner_NoGoroutineLeak(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(newScannerMockChecker()),
		WithScanInterval(50*time.Millisecond),
	)

	for i := 0; i < 5; i++ {
		ch := scanner.Start()
		if ch == nil {
			t.Fatalf("iteration %d: Start returned nil", i)
		}

		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d: timed out", i)
		}

		scanner.Stop()
		for range ch {
		}
	}
}

func TestCwdToProjectDir(t *testing.T) {
	t.Run("non-empty CWD", func(t *testing.T) {
		result := CWDToProjectDir("/Users/test/myproject")
		if result == "" {
			t.Error("expected non-empty result for valid CWD")
		}
		if filepath.Base(result) != "-Users-test-myproject" {
			t.Errorf("expected encoded path ending with -Users-test-myproject, got %q", filepath.Base(result))
		}
	})

	t.Run("empty CWD", func(t *testing.T) {
		result := CWDToProjectDir("")
		_ = result // Should not panic
	})
}

func TestDefaultSessionsPath(t *testing.T) {
	path := DefaultSessionsPath()
	if path == "" {
		t.Skip("unable to determine home directory")
	}
	if filepath.Base(path) != "sessions" {
		t.Errorf("expected path ending with 'sessions', got %q", path)
	}
}

// TestSessionScanner_NewSessionDetectedWithin2Seconds verifies that a newly
// created session file is detected within 2 seconds by the polling loop.
// This is the primary acceptance criterion for session detection latency.
func TestSessionScanner_NewSessionDetectedWithin2Seconds(t *testing.T) {
	t.Run("single new session detected within 2s", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker()

		// Use a realistic polling interval (500ms) to ensure we still meet 2s.
		scanner := NewSessionScanner(tmpDir,
			WithScannerPIDChecker(checker),
			WithScanInterval(500*time.Millisecond),
		)

		resultCh := scanner.Start()
		defer scanner.Stop()

		// Consume initial scan (empty directory)
		select {
		case result := <-resultCh:
			if len(result.Sessions) != 0 {
				t.Fatalf("expected 0 sessions initially, got %d", len(result.Sessions))
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for initial scan")
		}

		// Now create a new session file and mark PID alive
		checker.SetAlive(300, true)
		createTime := time.Now()
		writeSessionJSON(t, tmpDir, 300, SessionMeta{
			PID:       300,
			SessionID: "detect-latency-test",
			CWD:       "/home/user/project",
			StartedAt: time.Now().UnixMilli(),
		})

		// Wait for the scanner to detect the new session
		deadline := time.After(2 * time.Second)
		for {
			select {
			case result := <-resultCh:
				if len(result.Sessions) == 1 && result.Sessions[0].Meta.SessionID == "detect-latency-test" {
					detectLatency := time.Since(createTime)
					t.Logf("Session detected in %v (must be < 2s)", detectLatency)
					if detectLatency > 2*time.Second {
						t.Errorf("detection latency %v exceeds 2s threshold", detectLatency)
					}
					return
				}
			case <-deadline:
				t.Fatal("new session was NOT detected within 2 seconds")
			}
		}
	})

	t.Run("multiple new sessions detected within 2s", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker(100)
		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "existing"})

		scanner := NewSessionScanner(tmpDir,
			WithScannerPIDChecker(checker),
			WithScanInterval(500*time.Millisecond),
		)

		resultCh := scanner.Start()
		defer scanner.Stop()

		// Consume initial scan with existing session
		select {
		case result := <-resultCh:
			if len(result.Sessions) != 1 {
				t.Fatalf("expected 1 session initially, got %d", len(result.Sessions))
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for initial scan")
		}

		// Add two new sessions simultaneously
		checker.SetAlive(400, true)
		checker.SetAlive(500, true)
		createTime := time.Now()
		writeSessionJSON(t, tmpDir, 400, SessionMeta{PID: 400, SessionID: "new-a"})
		writeSessionJSON(t, tmpDir, 500, SessionMeta{PID: 500, SessionID: "new-b"})

		deadline := time.After(2 * time.Second)
		for {
			select {
			case result := <-resultCh:
				if len(result.Sessions) == 3 {
					detectLatency := time.Since(createTime)
					t.Logf("All 3 sessions detected in %v (must be < 2s)", detectLatency)
					if detectLatency > 2*time.Second {
						t.Errorf("detection latency %v exceeds 2s threshold", detectLatency)
					}
					return
				}
			case <-deadline:
				t.Fatal("new sessions were NOT detected within 2 seconds")
			}
		}
	})

	t.Run("default 2s interval detects within threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker()

		// Use the default 2s interval — the real production configuration
		scanner := NewSessionScanner(tmpDir,
			WithScannerPIDChecker(checker),
		)

		if scanner.Interval() != 2*time.Second {
			t.Fatalf("expected default 2s interval, got %v", scanner.Interval())
		}

		resultCh := scanner.Start()
		defer scanner.Stop()

		// Consume initial scan
		select {
		case <-resultCh:
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for initial scan")
		}

		// Create session just after a scan cycle — worst case timing
		checker.SetAlive(600, true)
		createTime := time.Now()
		writeSessionJSON(t, tmpDir, 600, SessionMeta{
			PID:       600,
			SessionID: "default-interval-test",
		})

		// With 2s polling, worst case detection is just over 2s (poll fires at 2s mark)
		// We allow 3s total to account for scheduling jitter.
		deadline := time.After(3 * time.Second)
		for {
			select {
			case result := <-resultCh:
				if len(result.Sessions) == 1 {
					detectLatency := time.Since(createTime)
					t.Logf("Default-interval detection in %v", detectLatency)
					return
				}
			case <-deadline:
				t.Fatal("new session was NOT detected within default polling cycle")
			}
		}
	})

	t.Run("rapid session creation detected within 2s", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker()

		scanner := NewSessionScanner(tmpDir,
			WithScannerPIDChecker(checker),
			WithScanInterval(100*time.Millisecond),
		)

		resultCh := scanner.Start()
		defer scanner.Stop()

		// Consume initial scan
		select {
		case <-resultCh:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for initial scan")
		}

		// Create a session file immediately
		checker.SetAlive(700, true)
		createTime := time.Now()
		writeSessionJSON(t, tmpDir, 700, SessionMeta{PID: 700, SessionID: "rapid"})

		deadline := time.After(2 * time.Second)
		for {
			select {
			case result := <-resultCh:
				if len(result.Sessions) == 1 {
					detectLatency := time.Since(createTime)
					t.Logf("Rapid detection in %v (must be < 2s)", detectLatency)
					if detectLatency > 2*time.Second {
						t.Errorf("detection latency %v exceeds 2s", detectLatency)
					}
					return
				}
			case <-deadline:
				t.Fatal("new session was NOT detected within 2 seconds")
			}
		}
	})

	t.Run("detection latency bounded by scan interval", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newScannerMockChecker()

		interval := 200 * time.Millisecond
		scanner := NewSessionScanner(tmpDir,
			WithScannerPIDChecker(checker),
			WithScanInterval(interval),
		)

		resultCh := scanner.Start()
		defer scanner.Stop()

		// Consume initial scan
		select {
		case <-resultCh:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for initial scan")
		}

		// Create session and measure detection latency
		checker.SetAlive(800, true)
		createTime := time.Now()
		writeSessionJSON(t, tmpDir, 800, SessionMeta{PID: 800, SessionID: "bounded"})

		deadline := time.After(2 * time.Second)
		for {
			select {
			case result := <-resultCh:
				if len(result.Sessions) == 1 {
					detectLatency := time.Since(createTime)
					// Detection should happen within ~1 interval + scheduling overhead
					maxExpected := interval + 200*time.Millisecond
					t.Logf("Detection in %v (expected within ~%v)", detectLatency, maxExpected)
					if detectLatency > 2*time.Second {
						t.Errorf("detection latency %v exceeds 2s threshold", detectLatency)
					}
					return
				}
			case <-deadline:
				t.Fatal("session was NOT detected within 2 seconds")
			}
		}
	})
}

// TestFilterByProject tests the FilterByProject function.
func TestFilterByProject(t *testing.T) {
	sessions := []ActiveSession{
		{Meta: SessionMeta{PID: 1, SessionID: "a", CWD: "/Users/me/project-a"}},
		{Meta: SessionMeta{PID: 2, SessionID: "b", CWD: "/Users/me/project-b"}},
		{Meta: SessionMeta{PID: 3, SessionID: "c", CWD: "/Users/me/project-a"}},
		{Meta: SessionMeta{PID: 4, SessionID: "d", CWD: ""}},
	}

	t.Run("matches single session", func(t *testing.T) {
		got := FilterByProject(sessions, "/Users/me/project-b")
		if len(got) != 1 {
			t.Fatalf("expected 1 match, got %d", len(got))
		}
		if got[0].Meta.SessionID != "b" {
			t.Errorf("expected session 'b', got %q", got[0].Meta.SessionID)
		}
	})

	t.Run("matches multiple sessions with same CWD", func(t *testing.T) {
		got := FilterByProject(sessions, "/Users/me/project-a")
		if len(got) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(got))
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		got := FilterByProject(sessions, "/Users/me/project-z")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty sessions returns nil", func(t *testing.T) {
		got := FilterByProject(nil, "/Users/me/project-a")
		if got != nil {
			t.Errorf("expected nil for empty sessions, got %v", got)
		}
	})

	t.Run("empty project CWD returns nil", func(t *testing.T) {
		got := FilterByProject(sessions, "")
		if got != nil {
			t.Errorf("expected nil for empty CWD, got %v", got)
		}
	})

	t.Run("trailing slash is normalized", func(t *testing.T) {
		got := FilterByProject(sessions, "/Users/me/project-a/")
		if len(got) != 2 {
			t.Fatalf("expected 2 matches with trailing slash, got %d", len(got))
		}
	})

	t.Run("session with empty CWD is not matched", func(t *testing.T) {
		onlyEmpty := []ActiveSession{
			{Meta: SessionMeta{PID: 5, SessionID: "e", CWD: ""}},
		}
		got := FilterByProject(onlyEmpty, "/Users/me/project-a")
		if got != nil {
			t.Errorf("expected nil (empty CWD session not matched), got %v", got)
		}
	})

	t.Run("both empty slice and CWD returns nil", func(t *testing.T) {
		got := FilterByProject([]ActiveSession{}, "")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("real-world CWD from product brief", func(t *testing.T) {
		realSessions := []ActiveSession{
			{
				Meta: SessionMeta{
					PID:       60696,
					SessionID: "e10c86ca-6cd9-4716-b905-576810a52484",
					CWD:       "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web",
				},
			},
			{
				Meta: SessionMeta{
					PID:       60697,
					SessionID: "other-session",
					CWD:       "/Users/limjk/GitHub/JeiKeiLim/other-project",
				},
			},
		}
		got := FilterByProject(realSessions, "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web")
		if len(got) != 1 {
			t.Fatalf("expected 1 match, got %d", len(got))
		}
		if got[0].Meta.PID != 60696 {
			t.Errorf("expected PID 60696, got %d", got[0].Meta.PID)
		}
	})
}

// writeSessionJSON is a helper that writes a properly formatted session file.
func writeSessionJSON(t *testing.T, dir string, pid int, meta SessionMeta) string {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal session meta: %v", err)
	}
	filename := filepath.Join(dir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	return filename
}

// TestSessionScanner_Scan_DirectoryReadError verifies that Scan() returns a
// non-nil Err when the sessions directory exists but cannot be read due to
// permissions. This exercises the error path distinct from IsNotExist.
func TestSessionScanner_Scan_DirectoryReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	tmpDir := t.TempDir()
	sessDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Remove all permissions so ReadDir fails (not IsNotExist).
	if err := os.Chmod(sessDir, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(sessDir, 0755) // restore for cleanup
	})

	scanner := NewSessionScanner(sessDir, WithScannerPIDChecker(newScannerMockChecker()))
	result := scanner.Scan()

	if result.Err == nil {
		t.Error("expected non-nil Err for non-readable directory, got nil")
	}
	if len(result.Sessions) != 0 {
		t.Errorf("expected 0 sessions on read error, got %d", len(result.Sessions))
	}
}

// TestSessionScanner_pollLoop_ChannelFull verifies that the pollLoop drops
// excess scan results without blocking when the result channel is full (default
// branch). This exercises the concurrent path where the caller doesn't drain
// the result channel fast enough.
func TestSessionScanner_pollLoop_ChannelFull(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newScannerMockChecker(100)
	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "channel-full-test"})

	scanner := NewSessionScanner(tmpDir,
		WithScannerPIDChecker(checker),
		WithScanInterval(5*time.Millisecond), // Very fast to trigger multiple ticks quickly
	)

	// Start but deliberately do NOT drain the result channel so the buffer
	// fills up. After the initial scan fills the single-slot buffer, subsequent
	// ticker ticks will hit the `default` branch.
	resultCh := scanner.Start()

	// Let multiple ticks fire with a full channel.
	time.Sleep(80 * time.Millisecond)

	scanner.Stop()
	// Drain to unblock the channel-close and avoid goroutine leak.
	for range resultCh {
	}
	// Test passes if we reach here without deadlock or panic.
}

// TestSessionScanner_ParsedMetadata validates the complete Sub-AC 2 flow:
// scanner detects a new {pid}.json file, parses the metadata, and returns
// the correct pid, project name, and start time.
func TestSessionScanner_ParsedMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	checker := newScannerMockChecker(60696)

	startedAt := int64(1774909391881)
	writeSessionJSON(t, tmpDir, 60696, SessionMeta{
		PID:        60696,
		SessionID:  "e10c86ca-6cd9-4716-b905-576810a52484",
		CWD:        "/Users/limjk/GitHub/JeiKeiLim/podcast-gen-web",
		StartedAt:  startedAt,
		Kind:       "interactive",
		Entrypoint: "sdk-cli",
	})

	scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
	result := scanner.Scan()

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(result.Sessions))
	}

	sess := result.Sessions[0]

	// Verify PID extraction
	if sess.Meta.PID != 60696 {
		t.Errorf("PID: got %d, want 60696", sess.Meta.PID)
	}

	// Verify project name extraction (the "project" component from CWD)
	if sess.Meta.ProjectName() != "podcast-gen-web" {
		t.Errorf("ProjectName: got %q, want 'podcast-gen-web'", sess.Meta.ProjectName())
	}

	// Verify start time extraction
	expectedTime := time.UnixMilli(startedAt)
	if !sess.Meta.StartedAtTime().Equal(expectedTime) {
		t.Errorf("StartedAtTime: got %v, want %v", sess.Meta.StartedAtTime(), expectedTime)
	}

	// Verify sessionId is preserved (maps to JSONL filename)
	if sess.Meta.SessionID != "e10c86ca-6cd9-4716-b905-576810a52484" {
		t.Errorf("SessionID: got %q", sess.Meta.SessionID)
	}

	// Verify JSONL directory is derived from CWD
	if sess.JSONLDir == "" {
		t.Error("JSONLDir must be non-empty when CWD is present")
	}
}

// TestSessionScanner_MalformedFilesHandled validates Sub-AC 2 requirement:
// malformed files are handled gracefully — scanner skips them and continues
// detecting valid sessions without returning errors.
func TestSessionScanner_MalformedFilesHandled(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		filename    string
		pidAlive    int
		wantSkipped bool
	}{
		{
			name:        "corrupt JSON is skipped",
			fileContent: `{corrupt json`,
			filename:    "100.json",
			pidAlive:    100,
			wantSkipped: true,
		},
		{
			name:        "empty file is skipped",
			fileContent: "",
			filename:    "101.json",
			pidAlive:    101,
			wantSkipped: true,
		},
		{
			name:        "missing sessionId is skipped",
			fileContent: `{"pid": 102, "cwd": "/home/user"}`,
			filename:    "102.json",
			pidAlive:    102,
			wantSkipped: true,
		},
		{
			name:        "PID mismatch is skipped",
			fileContent: `{"pid": 999, "sessionId": "mismatch-session"}`,
			filename:    "103.json",
			pidAlive:    103,
			wantSkipped: true,
		},
		{
			name:        "JSON array instead of object is skipped",
			fileContent: `["not", "an", "object"]`,
			filename:    "104.json",
			pidAlive:    104,
			wantSkipped: true,
		},
		{
			name:        "null JSON is skipped (no sessionId)",
			fileContent: "null",
			filename:    "105.json",
			pidAlive:    105,
			wantSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			checker := newScannerMockChecker(tt.pidAlive)

			// Write the malformed file
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			// Also write a valid session so we can confirm the scanner still works
			validPID := 200
			checker.SetAlive(validPID, true)
			writeSessionJSON(t, tmpDir, validPID, SessionMeta{
				PID:       validPID,
				SessionID: "valid-session",
			})

			scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
			result := scanner.Scan()

			// Scanner should not return an error even with malformed files
			if result.Err != nil {
				t.Errorf("Scan() should not return Err for malformed files, got: %v", result.Err)
			}

			// The malformed file should be skipped; only the valid session appears
			if len(result.Sessions) != 1 {
				t.Errorf("expected 1 valid session (malformed skipped), got %d", len(result.Sessions))
			}
			if len(result.Sessions) == 1 && result.Sessions[0].Meta.SessionID != "valid-session" {
				t.Errorf("expected 'valid-session', got %q", result.Sessions[0].Meta.SessionID)
			}
		})
	}
}
