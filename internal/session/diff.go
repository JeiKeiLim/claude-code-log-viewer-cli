// Package session provides multi-session detection and lifecycle management
// for Claude Code sessions (Phase 5a).
package session

import (
	"sort"
	"time"
)

// DiffResult holds the result of comparing two scan results.
// It identifies which sessions are new (opened) and which have been
// removed (closed) between consecutive scans.
type DiffResult struct {
	// Opened contains sessions present in Current but not in Previous.
	Opened []ActiveSession
	// Closed contains sessions present in Previous but not in Current.
	Closed []ActiveSession
	// Unchanged contains sessions present in both scans.
	Unchanged []ActiveSession

	// DetectedAt is the timestamp when the diff was computed,
	// used for detection latency tracking.
	DetectedAt time.Time
	// PreviousScanTime is the ScanTime of the previous ScanResult.
	PreviousScanTime time.Time
	// CurrentScanTime is the ScanTime of the current ScanResult.
	CurrentScanTime time.Time
}

// HasChanges returns true if any sessions were opened or closed.
func (d DiffResult) HasChanges() bool {
	return len(d.Opened) > 0 || len(d.Closed) > 0
}

// Events returns a slice of SessionEvent for all opened and closed sessions.
// Opened sessions come first, followed by closed sessions.
func (d DiffResult) Events() []SessionEvent {
	events := make([]SessionEvent, 0, len(d.Opened)+len(d.Closed))
	for _, s := range d.Opened {
		events = append(events, SessionEvent{
			Type:    SessionOpened,
			Session: s,
		})
	}
	for _, s := range d.Closed {
		events = append(events, SessionEvent{
			Type:    SessionClosed,
			Session: s,
		})
	}
	return events
}

// DetectionLatency returns the time between the current scan and when
// the diff was computed. This measures how quickly scan results are
// processed into events.
func (d DiffResult) DetectionLatency() time.Duration {
	return d.DetectedAt.Sub(d.CurrentScanTime)
}

// DiffSessions compares two ScanResults and identifies new and removed sessions.
// Sessions are matched by PID. A nil or zero-length previous result is treated
// as an empty baseline (all current sessions are "opened").
func DiffSessions(previous, current ScanResult) DiffResult {
	now := time.Now()
	result := DiffResult{
		DetectedAt:       now,
		PreviousScanTime: previous.ScanTime,
		CurrentScanTime:  current.ScanTime,
	}

	// Build lookup maps by PID for O(n) comparison
	prevByPID := buildSessionMap(previous.Sessions)
	currByPID := buildSessionMap(current.Sessions)

	// Find opened (in current but not in previous) and unchanged sessions
	for pid, s := range currByPID {
		if _, exists := prevByPID[pid]; exists {
			result.Unchanged = append(result.Unchanged, s)
		} else {
			result.Opened = append(result.Opened, s)
		}
	}

	// Find closed (in previous but not in current)
	for pid, s := range prevByPID {
		if _, exists := currByPID[pid]; !exists {
			result.Closed = append(result.Closed, s)
		}
	}

	// Sort all slices by PID for deterministic ordering across calls.
	// Map iteration is randomized in Go; without sorting, UI would flicker.
	sortByPID := func(s []ActiveSession) {
		sort.Slice(s, func(i, j int) bool { return s[i].Meta.PID < s[j].Meta.PID })
	}
	sortByPID(result.Opened)
	sortByPID(result.Closed)
	sortByPID(result.Unchanged)

	return result
}

// buildSessionMap creates a PID-keyed map from a session slice.
func buildSessionMap(sessions []ActiveSession) map[int]ActiveSession {
	m := make(map[int]ActiveSession, len(sessions))
	for _, s := range sessions {
		m[s.Meta.PID] = s
	}
	return m
}

// SessionDiffer tracks scan state across polls and produces diff events.
// It is designed to be used by the scanner's polling loop to emit
// SessionOpened/SessionClosed events on each scan cycle.
type SessionDiffer struct {
	previous ScanResult
	hasFirst bool
}

// NewSessionDiffer creates a new SessionDiffer with no prior state.
func NewSessionDiffer() *SessionDiffer {
	return &SessionDiffer{}
}

// Update computes the diff between the stored previous scan and the
// given current scan, updates the stored state, and returns the diff.
// On the first call, all sessions in current are treated as "opened".
func (d *SessionDiffer) Update(current ScanResult) DiffResult {
	var prev ScanResult
	if d.hasFirst {
		prev = d.previous
	}

	diff := DiffSessions(prev, current)

	d.previous = current
	d.hasFirst = true

	return diff
}

// Reset clears the stored previous state, making the next Update
// treat all sessions as new.
func (d *SessionDiffer) Reset() {
	d.previous = ScanResult{}
	d.hasFirst = false
}

// HasPrevious returns true if the differ has processed at least one scan.
func (d *SessionDiffer) HasPrevious() bool {
	return d.hasFirst
}

// Previous returns the last scan result that was processed.
// Returns a zero ScanResult if no scan has been processed.
func (d *SessionDiffer) Previous() ScanResult {
	return d.previous
}
