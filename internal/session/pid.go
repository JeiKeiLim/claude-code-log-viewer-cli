package session

import (
	"errors"
	"syscall"
)

// PIDChecker defines the interface for checking process liveness.
// This abstraction enables testing without real process dependencies.
type PIDChecker interface {
	// IsAlive returns true if the process with the given PID is running.
	IsAlive(pid int) bool
}

// SyscallPIDChecker checks PID liveness using syscall.Kill(pid, 0).
//
// On Unix systems (Linux, macOS), sending signal 0 to a PID probes for
// process existence without actually delivering a signal:
//   - nil error   → process exists and we have permission to signal it
//   - EPERM error → process exists but we lack permission (still alive!)
//   - ESRCH error → process does not exist
//
// The EPERM case is critical for correctness: cross-user processes (e.g.,
// root-owned processes checked by a non-root user) return EPERM rather than
// nil, but the process is still alive and must be tracked.
type SyscallPIDChecker struct{}

// IsAlive checks if a process with the given PID exists using syscall.Kill(pid, 0).
// Returns true if the process is alive (nil or EPERM from Kill syscall).
// Returns false for invalid PIDs (<=0) or if the process does not exist (ESRCH).
func (c *SyscallPIDChecker) IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	// nil: process exists with permission to signal
	// EPERM: process exists but we lack permission to signal it (still alive!)
	// ESRCH: process does not exist
	return err == nil || errors.Is(err, syscall.EPERM)
}

// NewPIDChecker creates a new SyscallPIDChecker for production use.
func NewPIDChecker() PIDChecker {
	return &SyscallPIDChecker{}
}
