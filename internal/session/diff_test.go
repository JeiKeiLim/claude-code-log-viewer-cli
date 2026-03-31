package session

import (
	"testing"
	"time"
)

// helper to create an ActiveSession with a given PID and session ID.
func makeSession(pid int, sessionID string) ActiveSession {
	return ActiveSession{
		Meta: SessionMeta{
			PID:       pid,
			SessionID: sessionID,
			CWD:       "/home/user/project",
			StartedAt: time.Now().UnixMilli(),
		},
		FilePath: "/tmp/sessions/" + sessionID + ".json",
		JSONLDir: "/home/user/.claude/projects/-home-user-project",
	}
}

func TestDiffSessions_BothEmpty(t *testing.T) {
	prev := ScanResult{ScanTime: time.Now().Add(-2 * time.Second)}
	curr := ScanResult{ScanTime: time.Now()}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 0 {
		t.Errorf("expected 0 opened, got %d", len(diff.Opened))
	}
	if len(diff.Closed) != 0 {
		t.Errorf("expected 0 closed, got %d", len(diff.Closed))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("expected 0 unchanged, got %d", len(diff.Unchanged))
	}
	if diff.HasChanges() {
		t.Error("expected no changes")
	}
}

func TestDiffSessions_AllNew(t *testing.T) {
	prev := ScanResult{ScanTime: time.Now().Add(-2 * time.Second)}
	curr := ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "session-a"),
			makeSession(200, "session-b"),
		},
		ScanTime: time.Now(),
	}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 2 {
		t.Fatalf("expected 2 opened, got %d", len(diff.Opened))
	}
	if len(diff.Closed) != 0 {
		t.Errorf("expected 0 closed, got %d", len(diff.Closed))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("expected 0 unchanged, got %d", len(diff.Unchanged))
	}
	if !diff.HasChanges() {
		t.Error("expected changes")
	}
}

func TestDiffSessions_AllRemoved(t *testing.T) {
	prev := ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "session-a"),
			makeSession(200, "session-b"),
		},
		ScanTime: time.Now().Add(-2 * time.Second),
	}
	curr := ScanResult{ScanTime: time.Now()}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 0 {
		t.Errorf("expected 0 opened, got %d", len(diff.Opened))
	}
	if len(diff.Closed) != 2 {
		t.Fatalf("expected 2 closed, got %d", len(diff.Closed))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("expected 0 unchanged, got %d", len(diff.Unchanged))
	}
	if !diff.HasChanges() {
		t.Error("expected changes")
	}
}

func TestDiffSessions_MixedChanges(t *testing.T) {
	prev := ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "stays"),
			makeSession(200, "dies"),
			makeSession(300, "also-stays"),
		},
		ScanTime: time.Now().Add(-2 * time.Second),
	}
	curr := ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "stays"),
			makeSession(300, "also-stays"),
			makeSession(400, "new-session"),
		},
		ScanTime: time.Now(),
	}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 1 {
		t.Fatalf("expected 1 opened, got %d", len(diff.Opened))
	}
	if diff.Opened[0].Meta.PID != 400 {
		t.Errorf("expected opened PID 400, got %d", diff.Opened[0].Meta.PID)
	}

	if len(diff.Closed) != 1 {
		t.Fatalf("expected 1 closed, got %d", len(diff.Closed))
	}
	if diff.Closed[0].Meta.PID != 200 {
		t.Errorf("expected closed PID 200, got %d", diff.Closed[0].Meta.PID)
	}

	if len(diff.Unchanged) != 2 {
		t.Errorf("expected 2 unchanged, got %d", len(diff.Unchanged))
	}

	if !diff.HasChanges() {
		t.Error("expected changes")
	}
}

func TestDiffSessions_NoChanges(t *testing.T) {
	sessions := []ActiveSession{
		makeSession(100, "session-a"),
		makeSession(200, "session-b"),
	}
	prev := ScanResult{
		Sessions: sessions,
		ScanTime: time.Now().Add(-2 * time.Second),
	}
	curr := ScanResult{
		Sessions: sessions,
		ScanTime: time.Now(),
	}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 0 {
		t.Errorf("expected 0 opened, got %d", len(diff.Opened))
	}
	if len(diff.Closed) != 0 {
		t.Errorf("expected 0 closed, got %d", len(diff.Closed))
	}
	if len(diff.Unchanged) != 2 {
		t.Errorf("expected 2 unchanged, got %d", len(diff.Unchanged))
	}
	if diff.HasChanges() {
		t.Error("expected no changes")
	}
}

func TestDiffSessions_NilPreviousSessions(t *testing.T) {
	// nil Sessions slice in previous should be treated as empty
	prev := ScanResult{ScanTime: time.Now().Add(-time.Second)}
	curr := ScanResult{
		Sessions: []ActiveSession{makeSession(100, "new")},
		ScanTime: time.Now(),
	}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 1 {
		t.Errorf("expected 1 opened, got %d", len(diff.Opened))
	}
}

func TestDiffSessions_NilCurrentSessions(t *testing.T) {
	prev := ScanResult{
		Sessions: []ActiveSession{makeSession(100, "old")},
		ScanTime: time.Now().Add(-time.Second),
	}
	curr := ScanResult{ScanTime: time.Now()}

	diff := DiffSessions(prev, curr)

	if len(diff.Closed) != 1 {
		t.Errorf("expected 1 closed, got %d", len(diff.Closed))
	}
}

func TestDiffSessions_Timestamps(t *testing.T) {
	prevTime := time.Now().Add(-3 * time.Second)
	currTime := time.Now().Add(-1 * time.Second)

	prev := ScanResult{ScanTime: prevTime}
	curr := ScanResult{ScanTime: currTime}

	before := time.Now()
	diff := DiffSessions(prev, curr)
	after := time.Now()

	if diff.PreviousScanTime != prevTime {
		t.Errorf("PreviousScanTime = %v, want %v", diff.PreviousScanTime, prevTime)
	}
	if diff.CurrentScanTime != currTime {
		t.Errorf("CurrentScanTime = %v, want %v", diff.CurrentScanTime, currTime)
	}
	if diff.DetectedAt.Before(before) || diff.DetectedAt.After(after) {
		t.Errorf("DetectedAt %v not between %v and %v", diff.DetectedAt, before, after)
	}
}

func TestDiffSessions_DetectionLatency(t *testing.T) {
	scanTime := time.Now().Add(-500 * time.Millisecond)
	curr := ScanResult{ScanTime: scanTime}
	prev := ScanResult{ScanTime: scanTime.Add(-2 * time.Second)}

	diff := DiffSessions(prev, curr)

	latency := diff.DetectionLatency()
	if latency < 0 {
		t.Errorf("expected non-negative latency, got %v", latency)
	}
	// DetectedAt should be after scan time, so latency >= ~500ms
	if latency < 400*time.Millisecond {
		t.Errorf("expected latency >= 400ms (scan was 500ms ago), got %v", latency)
	}
	if latency > 5*time.Second {
		t.Errorf("latency unexpectedly large: %v", latency)
	}
}

func TestDiffResult_Events(t *testing.T) {
	t.Run("opened and closed events", func(t *testing.T) {
		prev := ScanResult{
			Sessions: []ActiveSession{
				makeSession(100, "dies"),
			},
			ScanTime: time.Now().Add(-2 * time.Second),
		}
		curr := ScanResult{
			Sessions: []ActiveSession{
				makeSession(200, "new"),
			},
			ScanTime: time.Now(),
		}

		diff := DiffSessions(prev, curr)
		events := diff.Events()

		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}

		// Opened events come first
		if events[0].Type != SessionOpened {
			t.Errorf("expected first event to be SessionOpened, got %d", events[0].Type)
		}
		if events[0].Session.Meta.PID != 200 {
			t.Errorf("expected opened PID 200, got %d", events[0].Session.Meta.PID)
		}

		// Then closed events
		if events[1].Type != SessionClosed {
			t.Errorf("expected second event to be SessionClosed, got %d", events[1].Type)
		}
		if events[1].Session.Meta.PID != 100 {
			t.Errorf("expected closed PID 100, got %d", events[1].Session.Meta.PID)
		}
	})

	t.Run("no events when no changes", func(t *testing.T) {
		sessions := []ActiveSession{makeSession(100, "stable")}
		prev := ScanResult{Sessions: sessions, ScanTime: time.Now().Add(-time.Second)}
		curr := ScanResult{Sessions: sessions, ScanTime: time.Now()}

		diff := DiffSessions(prev, curr)
		events := diff.Events()

		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("only opened events", func(t *testing.T) {
		prev := ScanResult{ScanTime: time.Now().Add(-time.Second)}
		curr := ScanResult{
			Sessions: []ActiveSession{makeSession(100, "new")},
			ScanTime: time.Now(),
		}

		diff := DiffSessions(prev, curr)
		events := diff.Events()

		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != SessionOpened {
			t.Errorf("expected SessionOpened, got %d", events[0].Type)
		}
	})

	t.Run("only closed events", func(t *testing.T) {
		prev := ScanResult{
			Sessions: []ActiveSession{makeSession(100, "old")},
			ScanTime: time.Now().Add(-time.Second),
		}
		curr := ScanResult{ScanTime: time.Now()}

		diff := DiffSessions(prev, curr)
		events := diff.Events()

		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Type != SessionClosed {
			t.Errorf("expected SessionClosed, got %d", events[0].Type)
		}
	})
}

func TestDiffSessions_DuplicatePIDs(t *testing.T) {
	// If somehow the same PID appears twice, map keying deduplicates
	prev := ScanResult{ScanTime: time.Now().Add(-time.Second)}
	curr := ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "first"),
			makeSession(100, "second"), // same PID, last wins
		},
		ScanTime: time.Now(),
	}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 1 {
		t.Errorf("expected 1 opened (deduplicated), got %d", len(diff.Opened))
	}
}

func TestDiffSessions_LargeSessionSet(t *testing.T) {
	// Verify correctness with many sessions (stress test for map operations)
	const count = 100
	var prevSessions, currSessions []ActiveSession

	for i := 1; i <= count; i++ {
		prevSessions = append(prevSessions, makeSession(i, "prev"))
	}
	// Current: remove first 10, keep 11-90, add 101-110
	for i := 11; i <= count; i++ {
		currSessions = append(currSessions, makeSession(i, "curr"))
	}
	for i := 101; i <= 110; i++ {
		currSessions = append(currSessions, makeSession(i, "new"))
	}

	prev := ScanResult{Sessions: prevSessions, ScanTime: time.Now().Add(-time.Second)}
	curr := ScanResult{Sessions: currSessions, ScanTime: time.Now()}

	diff := DiffSessions(prev, curr)

	if len(diff.Opened) != 10 {
		t.Errorf("expected 10 opened, got %d", len(diff.Opened))
	}
	if len(diff.Closed) != 10 {
		t.Errorf("expected 10 closed, got %d", len(diff.Closed))
	}
	if len(diff.Unchanged) != 90 {
		t.Errorf("expected 90 unchanged, got %d", len(diff.Unchanged))
	}
}

// --- SessionDiffer tests ---

func TestSessionDiffer_New(t *testing.T) {
	d := NewSessionDiffer()

	if d.HasPrevious() {
		t.Error("new differ should not have previous")
	}
	prev := d.Previous()
	if len(prev.Sessions) != 0 {
		t.Error("previous should be empty")
	}
}

func TestSessionDiffer_FirstUpdate(t *testing.T) {
	d := NewSessionDiffer()

	curr := ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "first"),
			makeSession(200, "second"),
		},
		ScanTime: time.Now(),
	}

	diff := d.Update(curr)

	// First update: all sessions are "opened"
	if len(diff.Opened) != 2 {
		t.Errorf("expected 2 opened on first update, got %d", len(diff.Opened))
	}
	if len(diff.Closed) != 0 {
		t.Errorf("expected 0 closed on first update, got %d", len(diff.Closed))
	}
	if !d.HasPrevious() {
		t.Error("should have previous after first update")
	}
}

func TestSessionDiffer_SequentialUpdates(t *testing.T) {
	d := NewSessionDiffer()

	// Scan 1: sessions 100, 200
	diff1 := d.Update(ScanResult{
		Sessions: []ActiveSession{
			makeSession(100, "a"),
			makeSession(200, "b"),
		},
		ScanTime: time.Now(),
	})
	if len(diff1.Opened) != 2 {
		t.Fatalf("scan 1: expected 2 opened, got %d", len(diff1.Opened))
	}

	// Scan 2: sessions 200, 300 (100 closed, 300 opened)
	diff2 := d.Update(ScanResult{
		Sessions: []ActiveSession{
			makeSession(200, "b"),
			makeSession(300, "c"),
		},
		ScanTime: time.Now(),
	})
	if len(diff2.Opened) != 1 {
		t.Errorf("scan 2: expected 1 opened, got %d", len(diff2.Opened))
	}
	if len(diff2.Closed) != 1 {
		t.Errorf("scan 2: expected 1 closed, got %d", len(diff2.Closed))
	}
	if len(diff2.Unchanged) != 1 {
		t.Errorf("scan 2: expected 1 unchanged, got %d", len(diff2.Unchanged))
	}

	// Scan 3: no sessions (all closed)
	diff3 := d.Update(ScanResult{ScanTime: time.Now()})
	if len(diff3.Closed) != 2 {
		t.Errorf("scan 3: expected 2 closed, got %d", len(diff3.Closed))
	}
	if len(diff3.Opened) != 0 {
		t.Errorf("scan 3: expected 0 opened, got %d", len(diff3.Opened))
	}

	// Scan 4: no sessions still (no changes)
	diff4 := d.Update(ScanResult{ScanTime: time.Now()})
	if diff4.HasChanges() {
		t.Error("scan 4: expected no changes")
	}
}

func TestSessionDiffer_Reset(t *testing.T) {
	d := NewSessionDiffer()

	d.Update(ScanResult{
		Sessions: []ActiveSession{makeSession(100, "a")},
		ScanTime: time.Now(),
	})

	if !d.HasPrevious() {
		t.Error("should have previous before reset")
	}

	d.Reset()

	if d.HasPrevious() {
		t.Error("should not have previous after reset")
	}

	// After reset, next update treats all as new
	diff := d.Update(ScanResult{
		Sessions: []ActiveSession{makeSession(100, "a")},
		ScanTime: time.Now(),
	})
	if len(diff.Opened) != 1 {
		t.Errorf("expected 1 opened after reset, got %d", len(diff.Opened))
	}
}

func TestSessionDiffer_Previous(t *testing.T) {
	d := NewSessionDiffer()

	scanTime := time.Now()
	d.Update(ScanResult{
		Sessions: []ActiveSession{makeSession(100, "a")},
		ScanTime: scanTime,
	})

	prev := d.Previous()
	if len(prev.Sessions) != 1 {
		t.Errorf("expected 1 session in previous, got %d", len(prev.Sessions))
	}
	if prev.ScanTime != scanTime {
		t.Error("previous scan time should match")
	}
}

func TestSessionDiffer_EmptyFirstThenPopulate(t *testing.T) {
	d := NewSessionDiffer()

	// First scan with no sessions
	diff1 := d.Update(ScanResult{ScanTime: time.Now()})
	if diff1.HasChanges() {
		t.Error("expected no changes on first empty scan")
	}

	// Second scan with sessions
	diff2 := d.Update(ScanResult{
		Sessions: []ActiveSession{makeSession(100, "a")},
		ScanTime: time.Now(),
	})
	if len(diff2.Opened) != 1 {
		t.Errorf("expected 1 opened, got %d", len(diff2.Opened))
	}
}

func TestBuildSessionMap(t *testing.T) {
	sessions := []ActiveSession{
		makeSession(100, "a"),
		makeSession(200, "b"),
		makeSession(300, "c"),
	}

	m := buildSessionMap(sessions)

	if len(m) != 3 {
		t.Fatalf("expected map size 3, got %d", len(m))
	}
	for _, s := range sessions {
		if got, ok := m[s.Meta.PID]; !ok {
			t.Errorf("missing PID %d", s.Meta.PID)
		} else if got.Meta.SessionID != s.Meta.SessionID {
			t.Errorf("PID %d: expected sessionID %q, got %q", s.Meta.PID, s.Meta.SessionID, got.Meta.SessionID)
		}
	}
}

func TestBuildSessionMap_Empty(t *testing.T) {
	m := buildSessionMap(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got size %d", len(m))
	}
}

func TestBuildSessionMap_DuplicatePID(t *testing.T) {
	sessions := []ActiveSession{
		makeSession(100, "first"),
		makeSession(100, "second"), // same PID
	}

	m := buildSessionMap(sessions)
	if len(m) != 1 {
		t.Errorf("expected map size 1 (deduplicated), got %d", len(m))
	}
	// Last one wins
	if m[100].Meta.SessionID != "second" {
		t.Errorf("expected last-wins semantics, got %q", m[100].Meta.SessionID)
	}
}
