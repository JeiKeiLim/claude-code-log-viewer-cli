package session

import "sync"

// mockPIDChecker is a shared, thread-safe test double for PIDChecker.
// It replaces the per-file duplicates (scannerMockPIDChecker,
// registryMockPIDChecker, monitorMockPIDChecker, mockPIDCheckerForPIDTest).
type mockPIDChecker struct {
	mu    sync.RWMutex
	alive map[int]bool
	calls int
}

// newMockPIDChecker creates a mockPIDChecker with optional initially-alive PIDs.
func newMockPIDChecker(alivePIDs ...int) *mockPIDChecker {
	m := &mockPIDChecker{alive: make(map[int]bool)}
	for _, pid := range alivePIDs {
		m.alive[pid] = true
	}
	return m
}

// SetAlive sets the liveness state for a given PID.
func (m *mockPIDChecker) SetAlive(pid int, alive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alive[pid] = alive
}

// IsAlive returns whether the given PID is marked alive and increments the call counter.
func (m *mockPIDChecker) IsAlive(pid int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.alive[pid]
}

// CallCount returns the number of times IsAlive has been called.
func (m *mockPIDChecker) CallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls
}
