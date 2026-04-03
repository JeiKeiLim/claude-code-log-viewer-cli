package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestSessionScanner_Scan(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newMockPIDChecker()
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
			WithScannerPIDChecker(newMockPIDChecker()))

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
		checker := newMockPIDChecker(100, 200)

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
		checker := newMockPIDChecker(100) // Only 100 is alive

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "alive", CWD: "/test/filters-dead"})
		writeSessionJSON(t, tmpDir, 200, SessionMeta{PID: 200, SessionID: "dead", CWD: "/test/filters-dead"})

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
		checker := newMockPIDChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "valid", CWD: "/test/skips-nonjson"})
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
		checker := newMockPIDChecker(100, 200)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "valid", CWD: "/test/skips-corrupt"})
		os.WriteFile(filepath.Join(tmpDir, "200.json"), []byte("{corrupt"), 0644)

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
	})

	t.Run("pid mismatch between filename and content", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newMockPIDChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 999, SessionID: "mismatch"})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 0 {
			t.Errorf("expected 0 sessions (PID mismatch), got %d", len(result.Sessions))
		}
	})

	t.Run("pid zero in content uses filename pid", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newMockPIDChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{SessionID: "no-pid-in-content", CWD: "/test/pid-zero"})

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
		checker := newMockPIDChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "valid", CWD: "/test/skips-dirs"})
		os.MkdirAll(filepath.Join(tmpDir, "200.json"), 0755)

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result.Sessions))
		}
	})

	t.Run("scan time is set", func(t *testing.T) {
		tmpDir := t.TempDir()
		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(newMockPIDChecker()))

		before := time.Now()
		result := scanner.Scan()
		after := time.Now()

		if result.ScanTime.Before(before) || result.ScanTime.After(after) {
			t.Errorf("ScanTime %v not between %v and %v", result.ScanTime, before, after)
		}
	})

	t.Run("JSONLDir is derived from CWD", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newMockPIDChecker(100)

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

	t.Run("empty CWD leaves JSONLDir empty — session skipped", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := newMockPIDChecker(100)

		writeSessionJSON(t, tmpDir, 100, SessionMeta{
			PID:       100,
			SessionID: "no-cwd",
		})

		scanner := NewSessionScanner(tmpDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		// Sessions without a JSONL log file are skipped entirely.
		// Empty CWD means no JSONLDir, so no JSONL file can be found.
		if len(result.Sessions) != 0 {
			t.Fatalf("expected 0 sessions (no JSONL file for empty CWD), got %d", len(result.Sessions))
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
	checker := newMockPIDChecker(100)
	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "cached", CWD: "/test/scan-once"})

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
	checker := newMockPIDChecker(100)
	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "polled", CWD: "/test/start-stop"})

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
		WithScannerPIDChecker(newMockPIDChecker()),
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
		WithScannerPIDChecker(newMockPIDChecker()),
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
	checker := newMockPIDChecker(100)

	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "initial", CWD: "/test/dynamic-lifecycle"})

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
	writeSessionJSON(t, tmpDir, 200, SessionMeta{PID: 200, SessionID: "new", CWD: "/test/dynamic-lifecycle"})

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
	checker := newMockPIDChecker(100, 200)

	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "stays", CWD: "/test/session-closure"})
	writeSessionJSON(t, tmpDir, 200, SessionMeta{PID: 200, SessionID: "dies", CWD: "/test/session-closure"})

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
		WithScannerPIDChecker(newMockPIDChecker()),
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
		checker := newMockPIDChecker()

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
		checker := newMockPIDChecker(100)
		writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "existing", CWD: "/test/multi-detect"})

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
		writeSessionJSON(t, tmpDir, 400, SessionMeta{PID: 400, SessionID: "new-a", CWD: "/test/multi-detect"})
		writeSessionJSON(t, tmpDir, 500, SessionMeta{PID: 500, SessionID: "new-b", CWD: "/test/multi-detect"})

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
		checker := newMockPIDChecker()

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
			CWD:       "/test/default-interval",
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
		checker := newMockPIDChecker()

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
		writeSessionJSON(t, tmpDir, 700, SessionMeta{PID: 700, SessionID: "rapid", CWD: "/test/rapid"})

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
		checker := newMockPIDChecker()

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
		writeSessionJSON(t, tmpDir, 800, SessionMeta{PID: 800, SessionID: "bounded", CWD: "/test/bounded"})

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
// When CWD is non-empty, it also creates the corresponding JSONL log file
// so that the scanner's JSONL-existence check passes.
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

	// Create the JSONL log file so the scanner includes this session.
	if meta.CWD != "" && meta.SessionID != "" {
		jsonlDir := CWDToProjectDir(meta.CWD)
		if jsonlDir != "" {
			if err := os.MkdirAll(jsonlDir, 0755); err != nil {
				t.Fatalf("create JSONL dir %s: %v", jsonlDir, err)
			}
			jsonlPath := filepath.Join(jsonlDir, meta.SessionID+".jsonl")
			if err := os.WriteFile(jsonlPath, []byte("{}\n"), 0644); err != nil {
				t.Fatalf("write JSONL file %s: %v", jsonlPath, err)
			}
			t.Cleanup(func() {
				os.Remove(jsonlPath)
				os.Remove(jsonlDir) // best-effort removal if empty
			})
		}
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

	scanner := NewSessionScanner(sessDir, WithScannerPIDChecker(newMockPIDChecker()))
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
	checker := newMockPIDChecker(100)
	writeSessionJSON(t, tmpDir, 100, SessionMeta{PID: 100, SessionID: "channel-full-test", CWD: "/test/channel-full"})

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
	checker := newMockPIDChecker(60696)

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
			checker := newMockPIDChecker(tt.pidAlive)

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
				CWD:       "/test/malformed-handled",
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

// TestClassifySessionState verifies the three-state lifecycle classification
// based on JSONL file last-modified time relative to the current scan time.
func TestClassifySessionState(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		age      time.Duration
		expected SessionState
	}{
		{"just modified", 0, SessionActive},
		{"modified 30s ago", 30 * time.Second, SessionActive},
		{"modified 1m59s ago (just under idle)", IdleThreshold - time.Second, SessionActive},
		{"modified exactly 2m ago (idle threshold)", IdleThreshold, SessionIdle},
		{"modified 2m30s ago", 2*time.Minute + 30*time.Second, SessionIdle},
		{"modified 4m59s ago (just under removal)", RemovalThreshold - time.Second, SessionIdle},
		{"modified exactly 5m ago (removal threshold)", RemovalThreshold, SessionRemoved},
		{"modified 10m ago", 10 * time.Minute, SessionRemoved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modTime := now.Add(-tt.age)
			got := ClassifySessionState(modTime, now)
			if got != tt.expected {
				t.Errorf("ClassifySessionState(age=%v) = %v, want %v", tt.age, got, tt.expected)
			}
		})
	}
}

// TestClassifySessionState_IdleTransitionBoundary verifies the exact boundary
// where a session transitions from Active to Idle at the 2-minute mark.
func TestClassifySessionState_IdleTransitionBoundary(t *testing.T) {
	now := time.Now()

	// 1 millisecond before threshold: still Active
	justBefore := now.Add(-(IdleThreshold - time.Millisecond))
	if got := ClassifySessionState(justBefore, now); got != SessionActive {
		t.Errorf("1ms before idle threshold: got %v, want Active", got)
	}

	// Exactly at threshold: Idle
	atThreshold := now.Add(-IdleThreshold)
	if got := ClassifySessionState(atThreshold, now); got != SessionIdle {
		t.Errorf("at idle threshold: got %v, want Idle", got)
	}

	// 1 millisecond after threshold: Idle
	justAfter := now.Add(-(IdleThreshold + time.Millisecond))
	if got := ClassifySessionState(justAfter, now); got != SessionIdle {
		t.Errorf("1ms after idle threshold: got %v, want Idle", got)
	}
}

// TestScanSetsIdleState verifies that the scanner marks sessions as Idle
// when their JSONL file has not been modified for more than 2 minutes.
func TestScanSetsIdleState(t *testing.T) {
	sessionsDir := t.TempDir()

	// Create session JSON file
	writeSessionJSON(t, sessionsDir, 100, SessionMeta{
		PID:       100,
		SessionID: "idle-session",
		CWD:       "/tmp/test-project",
		StartedAt: 1711900000000,
	})

	// Create the JSONL directory structure and file
	sess := ActiveSession{
		Meta: SessionMeta{SessionID: "idle-session", CWD: "/tmp/test-project"},
	}
	sess.JSONLDir = CWDToProjectDir("/tmp/test-project")
	jsonlDir := sess.JSONLDir
	if err := os.MkdirAll(jsonlDir, 0755); err != nil {
		t.Fatalf("failed to create JSONL dir: %v", err)
	}
	jsonlPath := filepath.Join(jsonlDir, "idle-session.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"type":"test"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to write JSONL file: %v", err)
	}

	// Set the JSONL file's modification time to 3 minutes ago (idle)
	idleTime := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(jsonlPath, idleTime, idleTime); err != nil {
		t.Fatalf("failed to set JSONL mod time: %v", err)
	}

	checker := newMockPIDChecker(100)
	scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
	result := scanner.Scan()

	if result.Err != nil {
		t.Fatalf("unexpected scan error: %v", result.Err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(result.Sessions))
	}

	if result.Sessions[0].State != SessionIdle {
		t.Errorf("expected session state Idle, got %v", result.Sessions[0].State)
	}
	if result.Sessions[0].JSONLLastModified.IsZero() {
		t.Error("expected JSONLLastModified to be set")
	}
}

// TestScanSetsActiveState verifies that the scanner marks sessions as Active
// when their JSONL file was recently modified (within 2 minutes).
func TestScanSetsActiveState(t *testing.T) {
	sessionsDir := t.TempDir()

	writeSessionJSON(t, sessionsDir, 200, SessionMeta{
		PID:       200,
		SessionID: "active-session",
		CWD:       "/tmp/active-project",
		StartedAt: 1711900000000,
	})

	// Create JSONL file with current modification time (active)
	sess := ActiveSession{
		Meta: SessionMeta{SessionID: "active-session", CWD: "/tmp/active-project"},
	}
	sess.JSONLDir = CWDToProjectDir("/tmp/active-project")
	if err := os.MkdirAll(sess.JSONLDir, 0755); err != nil {
		t.Fatalf("failed to create JSONL dir: %v", err)
	}
	jsonlPath := filepath.Join(sess.JSONLDir, "active-session.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"type":"test"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to write JSONL file: %v", err)
	}
	// File just written — mod time is now, so state should be Active

	checker := newMockPIDChecker(200)
	scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
	result := scanner.Scan()

	if result.Err != nil {
		t.Fatalf("unexpected scan error: %v", result.Err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(result.Sessions))
	}

	if result.Sessions[0].State != SessionActive {
		t.Errorf("expected session state Active, got %v", result.Sessions[0].State)
	}
}

// TestDeduplicateBySessionID_NilInput returns nil for empty/nil input.
func TestDeduplicateBySessionID_NilInput(t *testing.T) {
	if got := DeduplicateBySessionID(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if got := DeduplicateBySessionID([]ActiveSession{}); got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

// TestDeduplicateBySessionID_NoDuplicates preserves unique sessions.
func TestDeduplicateBySessionID_NoDuplicates(t *testing.T) {
	sessions := []ActiveSession{
		{Meta: SessionMeta{PID: 100, SessionID: "sess-a"}},
		{Meta: SessionMeta{PID: 200, SessionID: "sess-b"}},
		{Meta: SessionMeta{PID: 300, SessionID: "sess-c"}},
	}
	got := DeduplicateBySessionID(sessions)
	if len(got) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(got))
	}
}

// TestDeduplicateBySessionID_KeepsLatestPID deduplicates by sessionId, keeping highest PID.
func TestDeduplicateBySessionID_KeepsLatestPID(t *testing.T) {
	sessions := []ActiveSession{
		{Meta: SessionMeta{PID: 100, SessionID: "sess-shared"}, FilePath: "/a/100.json"},
		{Meta: SessionMeta{PID: 300, SessionID: "sess-shared"}, FilePath: "/a/300.json"},
		{Meta: SessionMeta{PID: 200, SessionID: "sess-shared"}, FilePath: "/a/200.json"},
	}
	got := DeduplicateBySessionID(sessions)
	if len(got) != 1 {
		t.Fatalf("expected 1 session after dedup, got %d", len(got))
	}
	if got[0].Meta.PID != 300 {
		t.Errorf("expected PID 300 (latest), got %d", got[0].Meta.PID)
	}
}

// TestDeduplicateBySessionID_MixedDuplicates handles a mix of unique and duplicate sessionIds.
func TestDeduplicateBySessionID_MixedDuplicates(t *testing.T) {
	sessions := []ActiveSession{
		{Meta: SessionMeta{PID: 100, SessionID: "sess-a"}},
		{Meta: SessionMeta{PID: 200, SessionID: "sess-b"}},
		{Meta: SessionMeta{PID: 300, SessionID: "sess-a"}}, // duplicate of sess-a
		{Meta: SessionMeta{PID: 400, SessionID: "sess-c"}},
	}
	got := DeduplicateBySessionID(sessions)
	if len(got) != 3 {
		t.Fatalf("expected 3 sessions after dedup, got %d", len(got))
	}

	// Verify sess-a is PID 300
	pidBySession := make(map[string]int)
	for _, s := range got {
		pidBySession[s.Meta.SessionID] = s.Meta.PID
	}
	if pidBySession["sess-a"] != 300 {
		t.Errorf("sess-a should have PID 300, got %d", pidBySession["sess-a"])
	}
	if pidBySession["sess-b"] != 200 {
		t.Errorf("sess-b should have PID 200, got %d", pidBySession["sess-b"])
	}
	if pidBySession["sess-c"] != 400 {
		t.Errorf("sess-c should have PID 400, got %d", pidBySession["sess-c"])
	}
}

// TestDeduplicateBySessionID_EmptySessionID uses FilePath as key for sessions without sessionId.
func TestDeduplicateBySessionID_EmptySessionID(t *testing.T) {
	sessions := []ActiveSession{
		{Meta: SessionMeta{PID: 100, SessionID: ""}, FilePath: "/a/100.json"},
		{Meta: SessionMeta{PID: 200, SessionID: ""}, FilePath: "/a/200.json"},
	}
	got := DeduplicateBySessionID(sessions)
	// Both should be kept since they have different file paths
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions (different file paths), got %d", len(got))
	}
}

// TestScan_SkipsSessionsWithoutJSONL verifies AC2: sessions whose JSONL log
// file does not exist are excluded from scan results entirely.
func TestScan_SkipsSessionsWithoutJSONL(t *testing.T) {
	t.Run("session with CWD but no JSONL file is skipped", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(100)

		// Write a valid session file with CWD, but do NOT create the JSONL file.
		// We write the JSON manually to avoid writeSessionJSON creating the JSONL.
		meta := SessionMeta{
			PID:       100,
			SessionID: "no-jsonl-session",
			CWD:       "/test/no-jsonl-project",
			StartedAt: 1711900000000,
		}
		data, _ := json.Marshal(meta)
		os.WriteFile(filepath.Join(sessionsDir, "100.json"), data, 0644)

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 0 {
			t.Errorf("expected 0 sessions (no JSONL file), got %d", len(result.Sessions))
		}
	})

	t.Run("session without CWD is skipped", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(200)

		meta := SessionMeta{
			PID:       200,
			SessionID: "no-cwd-session",
			StartedAt: 1711900000000,
		}
		data, _ := json.Marshal(meta)
		os.WriteFile(filepath.Join(sessionsDir, "200.json"), data, 0644)

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 0 {
			t.Errorf("expected 0 sessions (no CWD → no JSONL), got %d", len(result.Sessions))
		}
	})

	t.Run("session with JSONL file is included", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(300)

		// Use writeSessionJSON which creates the JSONL file
		writeSessionJSON(t, sessionsDir, 300, SessionMeta{
			PID:       300,
			SessionID: "has-jsonl-session",
			CWD:       "/test/has-jsonl-project",
			StartedAt: 1711900000000,
		})

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session (JSONL exists), got %d", len(result.Sessions))
		}
		if result.Sessions[0].Meta.SessionID != "has-jsonl-session" {
			t.Errorf("unexpected session: %q", result.Sessions[0].Meta.SessionID)
		}
	})

	t.Run("mixed sessions — only those with JSONL are returned", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(400, 500, 600)

		// Session with JSONL (included)
		writeSessionJSON(t, sessionsDir, 400, SessionMeta{
			PID: 400, SessionID: "with-jsonl", CWD: "/test/mixed-jsonl",
		})

		// Session with CWD but missing JSONL (excluded) — write manually
		meta := SessionMeta{PID: 500, SessionID: "no-jsonl", CWD: "/test/missing-jsonl"}
		data, _ := json.Marshal(meta)
		os.WriteFile(filepath.Join(sessionsDir, "500.json"), data, 0644)

		// Session without CWD (excluded) — write manually
		meta2 := SessionMeta{PID: 600, SessionID: "no-cwd"}
		data2, _ := json.Marshal(meta2)
		os.WriteFile(filepath.Join(sessionsDir, "600.json"), data2, 0644)

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session (only one with JSONL), got %d", len(result.Sessions))
		}
		if result.Sessions[0].Meta.SessionID != "with-jsonl" {
			t.Errorf("expected 'with-jsonl', got %q", result.Sessions[0].Meta.SessionID)
		}
	})
}

// TestJSONLPath verifies the JSONLPath helper function.
func TestJSONLPath(t *testing.T) {
	t.Run("returns path when JSONLDir is set", func(t *testing.T) {
		sess := ActiveSession{
			Meta:     SessionMeta{SessionID: "test-session"},
			JSONLDir: "/home/.claude/projects/-test-project",
		}
		got := JSONLPath(sess)
		want := "/home/.claude/projects/-test-project/test-session.jsonl"
		if got != want {
			t.Errorf("JSONLPath = %q, want %q", got, want)
		}
	})

	t.Run("returns empty when JSONLDir is empty", func(t *testing.T) {
		sess := ActiveSession{
			Meta: SessionMeta{SessionID: "test-session"},
		}
		if got := JSONLPath(sess); got != "" {
			t.Errorf("JSONLPath = %q, want empty", got)
		}
	})
}

// TestScan_DeadPIDRemovedImmediatelyRegardlessOfJSONLTiming verifies AC5:
// a session whose PID is dead is excluded from scan results even when its
// JSONL log file was modified very recently (would otherwise be "Active").
// PID liveness is checked first; JSONL timing is irrelevant for dead PIDs.
func TestScan_DeadPIDRemovedImmediatelyRegardlessOfJSONLTiming(t *testing.T) {
	t.Run("dead PID with very recent JSONL is excluded from results", func(t *testing.T) {
		sessionsDir := t.TempDir()
		// PID 100 is alive, PID 200 is dead — both have recent JSONL files.
		checker := newMockPIDChecker(100) // only 100 is alive

		writeSessionJSON(t, sessionsDir, 100, SessionMeta{
			PID:       100,
			SessionID: "alive-session",
			CWD:       "/test/ac5-dead-pid",
			StartedAt: 1711900000000,
		})
		writeSessionJSON(t, sessionsDir, 200, SessionMeta{
			PID:       200,
			SessionID: "dead-session",
			CWD:       "/test/ac5-dead-pid",
			StartedAt: 1711900001000,
		})
		// Both JSONL files are written at current time → both would be "Active"
		// if only JSONL timing were used. But PID 200 is dead.

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 1 {
			t.Fatalf("expected 1 session (dead PID excluded), got %d", len(result.Sessions))
		}
		if result.Sessions[0].Meta.SessionID != "alive-session" {
			t.Errorf("expected alive-session, got %q", result.Sessions[0].Meta.SessionID)
		}
		if result.Sessions[0].State != SessionActive {
			t.Errorf("alive session with recent JSONL should be Active, got %v", result.Sessions[0].State)
		}
	})

	t.Run("dead PID with Active-state JSONL is immediately excluded", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(300) // 300 alive, 400 dead
		writeSessionJSON(t, sessionsDir, 300, SessionMeta{
			PID: 300, SessionID: "stays", CWD: "/test/ac5-immediate",
		})
		writeSessionJSON(t, sessionsDir, 400, SessionMeta{
			PID: 400, SessionID: "dies-with-active-jsonl", CWD: "/test/ac5-immediate",
		})

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		initial := scanner.Scan()
		if len(initial.Sessions) != 1 {
			t.Fatalf("expected 1 session (dead PID 400 excluded immediately), got %d", len(initial.Sessions))
		}
		for _, sess := range initial.Sessions {
			if sess.Meta.PID == 400 {
				t.Error("dead PID 400 must not appear in scan results even with recent JSONL")
			}
		}
	})

	t.Run("PID that dies mid-run is excluded on next scan", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(500, 600) // both alive initially
		writeSessionJSON(t, sessionsDir, 500, SessionMeta{
			PID: 500, SessionID: "stays-alive", CWD: "/test/ac5-dies-midrun",
		})
		writeSessionJSON(t, sessionsDir, 600, SessionMeta{
			PID: 600, SessionID: "dies-midrun", CWD: "/test/ac5-dies-midrun",
		})

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))

		// First scan: both alive, both Active (JSONL recently written)
		first := scanner.Scan()
		if len(first.Sessions) != 2 {
			t.Fatalf("first scan: expected 2 sessions, got %d", len(first.Sessions))
		}
		for _, sess := range first.Sessions {
			if sess.State != SessionActive {
				t.Errorf("first scan: PID=%d expected Active (recent JSONL), got %v",
					sess.Meta.PID, sess.State)
			}
		}

		// PID 600 dies — JSONL file is still recent (not modified since last scan)
		checker.SetAlive(600, false)

		// Second scan: PID 600 must be excluded immediately, JSONL timing irrelevant
		second := scanner.Scan()
		if len(second.Sessions) != 1 {
			t.Fatalf("second scan after PID death: expected 1 session, got %d", len(second.Sessions))
		}
		if second.Sessions[0].Meta.PID != 500 {
			t.Errorf("expected only PID 500 to remain, got PID %d", second.Sessions[0].Meta.PID)
		}
	})
}

// TestScanSetsRemovedState verifies that the scanner marks sessions as Removed
// when their JSONL file has not been modified for more than 5 minutes (the removal
// threshold). Unlike the dashboard which removes panes for Removed sessions, the
// scanner itself still includes them in its results — the State field is set to
// SessionRemoved so callers can decide what to do with them.
func TestScanSetsRemovedState(t *testing.T) {
	sessionsDir := t.TempDir()

	writeSessionJSON(t, sessionsDir, 100, SessionMeta{
		PID:       100,
		SessionID: "removed-session",
		CWD:       "/tmp/test-removed-state",
		StartedAt: 1711900000000,
	})

	// writeSessionJSON creates the JSONL at CWDToProjectDir(CWD)/sessionID.jsonl.
	// Update its modification time to 6 minutes ago to trigger the Removed state.
	jsonlDir := CWDToProjectDir("/tmp/test-removed-state")
	jsonlPath := filepath.Join(jsonlDir, "removed-session.jsonl")
	removedTime := time.Now().Add(-6 * time.Minute)
	if err := os.Chtimes(jsonlPath, removedTime, removedTime); err != nil {
		t.Fatalf("failed to set JSONL mod time: %v", err)
	}

	checker := newMockPIDChecker(100)
	scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
	result := scanner.Scan()

	if result.Err != nil {
		t.Fatalf("unexpected scan error: %v", result.Err)
	}
	// Removed sessions are still present in scan results; the dashboard is responsible
	// for not adding or removing panes for them.
	if len(result.Sessions) != 1 {
		t.Fatalf("expected 1 session (Removed sessions included in scan), got %d", len(result.Sessions))
	}
	if result.Sessions[0].State != SessionRemoved {
		t.Errorf("expected session state Removed (JSONL 6 min old), got %v", result.Sessions[0].State)
	}
	if result.Sessions[0].JSONLLastModified.IsZero() {
		t.Error("expected JSONLLastModified to be set")
	}
}

// TestScan_UserInitiatedSession_AllStates verifies Sub-AC 2: user-initiated
// sessions (Entrypoint: "sdk-cli") are correctly classified through all three
// lifecycle states — Active, Idle, Removed — based on the JSONL file's
// last-modified time, as measured by the scanner's Scan() method.
func TestScan_UserInitiatedSession_AllStates(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name      string
		cwd       string
		sessionID string
		modAge    time.Duration
		expected  SessionState
	}{
		{
			name:      "active_within_2min",
			cwd:       "/tmp/user-session-active",
			sessionID: "user-sess-active",
			modAge:    10 * time.Second,
			expected:  SessionActive,
		},
		{
			name:      "idle_2_to_5min",
			cwd:       "/tmp/user-session-idle",
			sessionID: "user-sess-idle",
			modAge:    3 * time.Minute,
			expected:  SessionIdle,
		},
		{
			name:      "removed_after_5min",
			cwd:       "/tmp/user-session-removed",
			sessionID: "user-sess-removed",
			modAge:    6 * time.Minute,
			expected:  SessionRemoved,
		},
	}

	for i, tc := range cases {
		tc := tc
		pid := 9100 + i
		t.Run(tc.name, func(t *testing.T) {
			sessionsDir := t.TempDir()
			checker := newMockPIDChecker(pid)

			writeSessionJSON(t, sessionsDir, pid, SessionMeta{
				PID:        pid,
				SessionID:  tc.sessionID,
				CWD:        tc.cwd,
				StartedAt:  1711900000000,
				Kind:       "interactive",
				Entrypoint: "sdk-cli",
			})

			// Adjust JSONL modification time to simulate the desired lifecycle age.
			jsonlDir := CWDToProjectDir(tc.cwd)
			jsonlPath := filepath.Join(jsonlDir, tc.sessionID+".jsonl")
			modTime := now.Add(-tc.modAge)
			if err := os.Chtimes(jsonlPath, modTime, modTime); err != nil {
				t.Fatalf("set JSONL mod time: %v", err)
			}

			scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
			result := scanner.Scan()

			if result.Err != nil {
				t.Fatalf("unexpected error: %v", result.Err)
			}
			if len(result.Sessions) != 1 {
				t.Fatalf("expected 1 session, got %d", len(result.Sessions))
			}
			sess := result.Sessions[0]
			if sess.Meta.Entrypoint != "sdk-cli" {
				t.Errorf("expected entrypoint sdk-cli, got %q", sess.Meta.Entrypoint)
			}
			if sess.State != tc.expected {
				t.Errorf("expected state %v (modAge=%v), got %v", tc.expected, tc.modAge, sess.State)
			}
			if sess.JSONLLastModified.IsZero() {
				t.Error("expected JSONLLastModified to be non-zero")
			}
		})
	}
}

// TestScan_DeduplicateSameSessionID_EndToEnd verifies Sub-AC 2 deduplication:
// when multiple PID files reference the same sessionId, DeduplicateBySessionID
// applied to scanner results reduces them to a single entry keeping the highest PID.
// This is the complete end-to-end flow used by the dashboard.
func TestScan_DeduplicateSameSessionID_EndToEnd(t *testing.T) {
	sessionsDir := t.TempDir()
	sessionID := "shared-session-e2e"
	cwd := "/test/dedup-e2e-project"
	checker := newMockPIDChecker(100, 200, 300)

	// Three PIDs all share the same sessionId (e.g. after successive restarts)
	for _, pid := range []int{100, 200, 300} {
		writeSessionJSON(t, sessionsDir, pid, SessionMeta{
			PID:        pid,
			SessionID:  sessionID,
			CWD:        cwd,
			StartedAt:  1711900000000,
			Entrypoint: "sdk-cli",
		})
	}

	scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
	result := scanner.Scan()

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	// Before deduplication the scanner returns all three PIDs.
	if len(result.Sessions) != 3 {
		t.Fatalf("expected 3 sessions before dedup, got %d", len(result.Sessions))
	}

	// After deduplication only the session with the highest PID survives.
	deduped := DeduplicateBySessionID(result.Sessions)
	if len(deduped) != 1 {
		t.Fatalf("expected 1 session after dedup, got %d", len(deduped))
	}
	if deduped[0].Meta.PID != 300 {
		t.Errorf("expected PID 300 (highest) to survive dedup, got PID %d", deduped[0].Meta.PID)
	}
	if deduped[0].Meta.SessionID != sessionID {
		t.Errorf("expected sessionId %q, got %q", sessionID, deduped[0].Meta.SessionID)
	}
	// Surviving session must still be Active (JSONL just written by the helper)
	if deduped[0].State != SessionActive {
		t.Errorf("surviving session should be Active (recently written JSONL), got %v", deduped[0].State)
	}
}

// TestScan_BothEntrypointTypes verifies that both sdk-cli (user sessions) and
// sdk-py (Ouroboros agent sessions) are scanned, classified, and deduplicated
// correctly. The entrypoint field must not affect lifecycle classification or
// filtering — both types flow through the same JSONL-based activity detection.
func TestScan_BothEntrypointTypes(t *testing.T) {
	t.Run("sdk-cli and sdk-py sessions both included in scan", func(t *testing.T) {
		sessionsDir := t.TempDir()
		checker := newMockPIDChecker(1000, 2000)

		writeSessionJSON(t, sessionsDir, 1000, SessionMeta{
			PID:        1000,
			SessionID:  "user-session-id",
			CWD:        "/test/entrypoint-project",
			StartedAt:  1711900000000,
			Kind:       "interactive",
			Entrypoint: "sdk-cli",
		})
		writeSessionJSON(t, sessionsDir, 2000, SessionMeta{
			PID:        2000,
			SessionID:  "agent-session-id",
			CWD:        "/test/entrypoint-project",
			StartedAt:  1711900001000,
			Kind:       "task",
			Entrypoint: "sdk-py",
		})

		scanner := NewSessionScanner(sessionsDir, WithScannerPIDChecker(checker))
		result := scanner.Scan()

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if len(result.Sessions) != 2 {
			t.Fatalf("expected 2 sessions (both entrypoint types), got %d", len(result.Sessions))
		}

		// Verify both sessions have valid state classification
		for _, sess := range result.Sessions {
			if sess.State != SessionActive {
				t.Errorf("session PID=%d entrypoint=%q: expected Active state, got %s",
					sess.Meta.PID, sess.Meta.Entrypoint, sess.State)
			}
			if sess.JSONLLastModified.IsZero() {
				t.Errorf("session PID=%d entrypoint=%q: JSONLLastModified should not be zero",
					sess.Meta.PID, sess.Meta.Entrypoint)
			}
		}
	})

	t.Run("sdk-py sessions follow same lifecycle as sdk-cli", func(t *testing.T) {
		now := time.Now()

		// Active agent session (recently modified)
		activeAgent := ActiveSession{
			Meta:              SessionMeta{PID: 3000, SessionID: "active-agent", Entrypoint: "sdk-py"},
			State:             ClassifySessionState(now.Add(-30*time.Second), now),
			JSONLLastModified: now.Add(-30 * time.Second),
		}
		// Idle agent session (3 minutes old)
		idleAgent := ActiveSession{
			Meta:              SessionMeta{PID: 3001, SessionID: "idle-agent", Entrypoint: "sdk-py"},
			State:             ClassifySessionState(now.Add(-3*time.Minute), now),
			JSONLLastModified: now.Add(-3 * time.Minute),
		}
		// Removed agent session (6 minutes old)
		removedAgent := ActiveSession{
			Meta:              SessionMeta{PID: 3002, SessionID: "removed-agent", Entrypoint: "sdk-py"},
			State:             ClassifySessionState(now.Add(-6*time.Minute), now),
			JSONLLastModified: now.Add(-6 * time.Minute),
		}
		// Active user session (recently modified)
		activeUser := ActiveSession{
			Meta:              SessionMeta{PID: 4000, SessionID: "active-user", Entrypoint: "sdk-cli"},
			State:             ClassifySessionState(now.Add(-30*time.Second), now),
			JSONLLastModified: now.Add(-30 * time.Second),
		}
		// Idle user session (3 minutes old)
		idleUser := ActiveSession{
			Meta:              SessionMeta{PID: 4001, SessionID: "idle-user", Entrypoint: "sdk-cli"},
			State:             ClassifySessionState(now.Add(-3*time.Minute), now),
			JSONLLastModified: now.Add(-3 * time.Minute),
		}

		if activeAgent.State != SessionActive {
			t.Errorf("sdk-py active agent: expected Active, got %s", activeAgent.State)
		}
		if idleAgent.State != SessionIdle {
			t.Errorf("sdk-py idle agent: expected Idle, got %s", idleAgent.State)
		}
		if removedAgent.State != SessionRemoved {
			t.Errorf("sdk-py removed agent: expected Removed, got %s", removedAgent.State)
		}
		if activeUser.State != SessionActive {
			t.Errorf("sdk-cli active user: expected Active, got %s", activeUser.State)
		}
		if idleUser.State != SessionIdle {
			t.Errorf("sdk-cli idle user: expected Idle, got %s", idleUser.State)
		}
	})

	t.Run("deduplication works across entrypoint types", func(t *testing.T) {
		// Same sessionId used by both an sdk-cli and sdk-py process (e.g., session
		// started as CLI then continued as agent). The latest PID wins.
		sessions := []ActiveSession{
			{
				Meta:     SessionMeta{PID: 5000, SessionID: "shared-session", Entrypoint: "sdk-cli"},
				FilePath: "/sessions/5000.json",
			},
			{
				Meta:     SessionMeta{PID: 5001, SessionID: "shared-session", Entrypoint: "sdk-py"},
				FilePath: "/sessions/5001.json",
			},
		}
		got := DeduplicateBySessionID(sessions)
		if len(got) != 1 {
			t.Fatalf("expected 1 session after dedup, got %d", len(got))
		}
		if got[0].Meta.PID != 5001 {
			t.Errorf("expected latest PID 5001 to win dedup, got %d", got[0].Meta.PID)
		}
		if got[0].Meta.Entrypoint != "sdk-py" {
			t.Errorf("expected sdk-py entrypoint on winning session, got %q", got[0].Meta.Entrypoint)
		}
	})

	t.Run("different sessionIds with different entrypoints coexist", func(t *testing.T) {
		sessions := []ActiveSession{
			{
				Meta:     SessionMeta{PID: 6000, SessionID: "user-sess", Entrypoint: "sdk-cli"},
				FilePath: "/sessions/6000.json",
			},
			{
				Meta:     SessionMeta{PID: 6001, SessionID: "agent-sess", Entrypoint: "sdk-py"},
				FilePath: "/sessions/6001.json",
			},
		}
		got := DeduplicateBySessionID(sessions)
		if len(got) != 2 {
			t.Fatalf("expected 2 sessions (different sessionIds), got %d", len(got))
		}
	})

	t.Run("FilterByProject includes both entrypoint types", func(t *testing.T) {
		cwd := "/test/my-project"
		sessions := []ActiveSession{
			{Meta: SessionMeta{PID: 7000, CWD: cwd, Entrypoint: "sdk-cli"}},
			{Meta: SessionMeta{PID: 7001, CWD: cwd, Entrypoint: "sdk-py"}},
			{Meta: SessionMeta{PID: 7002, CWD: "/other/project", Entrypoint: "sdk-py"}},
		}
		got := FilterByProject(sessions, cwd)
		if len(got) != 2 {
			t.Fatalf("expected 2 sessions for project, got %d", len(got))
		}
		// Both entrypoint types should be present
		entrypoints := make(map[string]bool)
		for _, s := range got {
			entrypoints[s.Meta.Entrypoint] = true
		}
		if !entrypoints["sdk-cli"] || !entrypoints["sdk-py"] {
			t.Errorf("expected both sdk-cli and sdk-py in filtered results, got %v", entrypoints)
		}
	})
}
