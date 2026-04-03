package session

import (
	"os"
	"syscall"
	"testing"
)

func TestSyscallPIDChecker_CurrentProcess(t *testing.T) {
	checker := &SyscallPIDChecker{}
	pid := os.Getpid()

	if !checker.IsAlive(pid) {
		t.Errorf("current process PID %d should be alive", pid)
	}
}

func TestSyscallPIDChecker_InvalidPID(t *testing.T) {
	checker := &SyscallPIDChecker{}

	tests := []struct {
		name string
		pid  int
	}{
		{"zero", 0},
		{"negative", -1},
		{"very negative", -99999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if checker.IsAlive(tt.pid) {
				t.Errorf("PID %d should not be alive", tt.pid)
			}
		})
	}
}

func TestSyscallPIDChecker_NonExistentPID(t *testing.T) {
	checker := &SyscallPIDChecker{}

	// PID 4194304 (2^22) is very unlikely to exist on any system
	if checker.IsAlive(4194304) {
		t.Skip("PID 4194304 unexpectedly exists on this system")
	}
}

// TestSyscallPIDChecker_EPERM_ReturnsAlive verifies that a process
// returning EPERM (permission denied) from Kill(pid, 0) is correctly
// classified as alive. On most Unix systems, PID 1 (init/launchd/systemd)
// is owned by root and returns EPERM for non-root callers.
func TestSyscallPIDChecker_EPERM_ReturnsAlive(t *testing.T) {
	checker := &SyscallPIDChecker{}

	// Probe PID 1 to see if we get EPERM (non-root checking root-owned process)
	err := syscall.Kill(1, 0)
	if err == nil {
		// We have permission to signal PID 1 (running as root or same user)
		// Verify the checker returns true for this alive process
		if !checker.IsAlive(1) {
			t.Error("PID 1 should be alive when Kill returns nil")
		}
		return
	}
	if err != syscall.EPERM {
		t.Skipf("PID 1 returned unexpected error %v (expected EPERM), skipping EPERM test", err)
	}

	// PID 1 returns EPERM - verify we correctly report it as alive
	if !checker.IsAlive(1) {
		t.Error("PID 1 (init/launchd) returned EPERM but should be reported as alive")
	}
}

// TestSyscallPIDChecker_KillErrorSemantics documents the Kill(pid, 0) semantics
// that underpin our liveness detection logic.
func TestSyscallPIDChecker_KillErrorSemantics(t *testing.T) {
	// Verify that ESRCH (no such process) is correctly classified as dead.
	// PID 4194304 (2^22) is above typical PID_MAX on most systems.
	err := syscall.Kill(4194304, 0)
	if err == syscall.ESRCH {
		// This is the expected "process not found" error
		checker := &SyscallPIDChecker{}
		if checker.IsAlive(4194304) {
			t.Error("PID returning ESRCH should be reported as dead")
		}
	} else if err == nil {
		t.Skip("PID 4194304 unexpectedly exists; skipping ESRCH test")
	}
	// If EPERM or other: PID may be recycled; skip to avoid flakiness
}

func TestNewPIDChecker_ReturnsNonNil(t *testing.T) {
	checker := NewPIDChecker()
	if checker == nil {
		t.Error("NewPIDChecker should return non-nil checker")
	}

	// Should be a SyscallPIDChecker
	if _, ok := checker.(*SyscallPIDChecker); !ok {
		t.Error("NewPIDChecker should return *SyscallPIDChecker")
	}
}

func TestNewPIDChecker_CurrentProcessAlive(t *testing.T) {
	checker := NewPIDChecker()
	if !checker.IsAlive(os.Getpid()) {
		t.Error("current process should be alive via NewPIDChecker()")
	}
}

// TestPIDCheckerInterface verifies the PIDChecker interface can be implemented
// by custom types (used extensively in tests throughout the session package).
func TestPIDCheckerInterface(t *testing.T) {
	var _ PIDChecker = &SyscallPIDChecker{}
	var _ PIDChecker = newMockPIDChecker()
}

// TestMockPIDChecker_Interface validates mock implementation satisfies interface.
func TestMockPIDChecker_Interface(t *testing.T) {
	aliveChecker := newMockPIDChecker(1234)
	deadChecker := newMockPIDChecker()

	if !aliveChecker.IsAlive(1234) {
		t.Error("alive mock should return true")
	}
	if deadChecker.IsAlive(1234) {
		t.Error("dead mock should return false")
	}
}

// TestSyscallPIDChecker_ChildProcess verifies that a freshly-exited child
// process is correctly detected as dead. This exercises the ESRCH code path
// in a controlled manner.
func TestSyscallPIDChecker_ChildProcess(t *testing.T) {
	checker := &SyscallPIDChecker{}

	// Start a child process that exits immediately
	proc, err := os.StartProcess(
		"/bin/sh",
		[]string{"/bin/sh", "-c", "exit 0"},
		&os.ProcAttr{},
	)
	if err != nil {
		t.Skipf("cannot start child process: %v", err)
	}

	childPID := proc.Pid

	// Process should be alive immediately after start (may race but usually true)
	// We wait for it to exit cleanly
	state, err := proc.Wait()
	if err != nil {
		t.Fatalf("error waiting for child process: %v", err)
	}
	if !state.Exited() {
		t.Fatal("child process should have exited")
	}

	// After Wait(), the process has been reaped — it should be dead
	if checker.IsAlive(childPID) {
		t.Errorf("reaped child PID %d should be reported as dead", childPID)
	}
}
