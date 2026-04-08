package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
)

func TestNewFrameRateGovernor(t *testing.T) {
	g := NewFrameRateGovernor()

	if g == nil {
		t.Fatal("NewFrameRateGovernor returned nil")
	}
	if !g.IsActive() {
		t.Error("governor should be active on creation")
	}
	if g.AverageFrameDuration() != 0 {
		t.Errorf("initial average duration = %v, want 0", g.AverageFrameDuration())
	}
	if g.LastFrameDuration() != 0 {
		t.Errorf("initial last duration = %v, want 0", g.LastFrameDuration())
	}
}

func TestFrameRateGovernor_FrameStartEnd(t *testing.T) {
	g := NewFrameRateGovernor()

	g.FrameStart()
	// Simulate some work
	time.Sleep(1 * time.Millisecond)
	g.FrameEnd()

	if g.LastFrameDuration() == 0 {
		t.Error("last frame duration should be > 0 after FrameEnd")
	}

	stats := g.Stats()
	if stats.FramesRendered != 1 {
		t.Errorf("frames rendered = %d, want 1", stats.FramesRendered)
	}
}

func TestFrameRateGovernor_FrameEnd_NoStart(t *testing.T) {
	g := NewFrameRateGovernor()

	// FrameEnd without FrameStart should be a no-op
	g.FrameEnd()

	if g.LastFrameDuration() != 0 {
		t.Errorf("duration should be 0 without FrameStart, got %v", g.LastFrameDuration())
	}
	stats := g.Stats()
	if stats.FramesRendered != 0 {
		t.Errorf("frames rendered = %d, want 0", stats.FramesRendered)
	}
}

func TestFrameRateGovernor_ShouldSkipNonEssential_UnderBudget(t *testing.T) {
	g := NewFrameRateGovernor()

	// Simulate several fast frames (well under 16ms budget)
	for i := 0; i < 5; i++ {
		g.FrameStart()
		// Record a very fast frame by directly setting duration
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-1 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	if g.ShouldSkipNonEssential() {
		t.Error("should NOT skip non-essential when under budget")
	}
}

func TestFrameRateGovernor_ShouldSkipNonEssential_OverBudget(t *testing.T) {
	g := NewFrameRateGovernor()

	// Simulate several slow frames (over 16ms budget)
	for i := 0; i < 5; i++ {
		g.FrameStart()
		// Record a slow frame
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-20 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	if !g.ShouldSkipNonEssential() {
		t.Error("should skip non-essential when over budget")
	}
}

func TestFrameRateGovernor_ShouldSkipNonEssential_NoSamples(t *testing.T) {
	g := NewFrameRateGovernor()

	// No frames recorded yet
	if g.ShouldSkipNonEssential() {
		t.Error("should NOT skip when no samples available")
	}
}

func TestFrameRateGovernor_SampleCount(t *testing.T) {
	g := NewFrameRateGovernor()

	if g.SampleCount() != 0 {
		t.Errorf("initial sample count = %d, want 0", g.SampleCount())
	}

	// Record 3 frames
	for i := 0; i < 3; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-5 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	if g.SampleCount() != 3 {
		t.Errorf("sample count after 3 frames = %d, want 3", g.SampleCount())
	}

	// Fill beyond sample size — should cap at frameSampleSize
	for i := 0; i < frameSampleSize+5; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-2 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	if g.SampleCount() != frameSampleSize {
		t.Errorf("sample count after overflow = %d, want %d", g.SampleCount(), frameSampleSize)
	}
}

func TestFrameRateGovernor_RollingAverage(t *testing.T) {
	g := NewFrameRateGovernor()

	// Record frameSampleSize frames at 10ms each
	for i := 0; i < frameSampleSize; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-10 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	avg := g.AverageFrameDuration()
	// Average should be approximately 10ms (allow for timing jitter)
	if avg < 9*time.Millisecond || avg > 12*time.Millisecond {
		t.Errorf("average frame duration = %v, want ~10ms", avg)
	}

	// Now add fast frames to bring the average down
	for i := 0; i < frameSampleSize; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-1 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	avg2 := g.AverageFrameDuration()
	if avg2 >= avg {
		t.Errorf("average should decrease with faster frames, got %v (was %v)", avg2, avg)
	}
}

func TestFrameRateGovernor_OverBudgetTracking(t *testing.T) {
	g := NewFrameRateGovernor()

	// 3 fast frames
	for i := 0; i < 3; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-1 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	// 2 slow frames
	for i := 0; i < 2; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-20 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	stats := g.Stats()
	if stats.FramesRendered != 5 {
		t.Errorf("frames rendered = %d, want 5", stats.FramesRendered)
	}
	if stats.OverBudgetFrames != 2 {
		t.Errorf("over budget frames = %d, want 2", stats.OverBudgetFrames)
	}
}

func TestFrameRateGovernor_RecordSkip(t *testing.T) {
	g := NewFrameRateGovernor()

	g.RecordSkip()
	g.RecordSkip()

	stats := g.Stats()
	if stats.FramesSkipped != 2 {
		t.Errorf("frames skipped = %d, want 2", stats.FramesSkipped)
	}
}

func TestFrameRateGovernor_Stop(t *testing.T) {
	g := NewFrameRateGovernor()

	if !g.IsActive() {
		t.Error("should be active initially")
	}

	g.Stop()

	if g.IsActive() {
		t.Error("should not be active after Stop")
	}
}

func TestFrameRateGovernor_Reset(t *testing.T) {
	g := NewFrameRateGovernor()

	// Record some data
	for i := 0; i < 3; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-10 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}
	g.RecordSkip()

	// Verify data exists
	stats := g.Stats()
	if stats.FramesRendered == 0 {
		t.Fatal("should have frames before reset")
	}

	g.Reset()

	stats = g.Stats()
	if stats.FramesRendered != 0 {
		t.Errorf("frames rendered = %d after reset, want 0", stats.FramesRendered)
	}
	if stats.FramesSkipped != 0 {
		t.Errorf("frames skipped = %d after reset, want 0", stats.FramesSkipped)
	}
	if stats.OverBudgetFrames != 0 {
		t.Errorf("over budget frames = %d after reset, want 0", stats.OverBudgetFrames)
	}
	if stats.AverageDuration != 0 {
		t.Errorf("average duration = %v after reset, want 0", stats.AverageDuration)
	}
	if stats.LastDuration != 0 {
		t.Errorf("last duration = %v after reset, want 0", stats.LastDuration)
	}
}

func TestFrameStats_OverBudgetRatio(t *testing.T) {
	tests := []struct {
		name     string
		rendered int64
		over     int64
		want     float64
	}{
		{"no frames", 0, 0, 0},
		{"no over budget", 10, 0, 0},
		{"half over budget", 10, 5, 0.5},
		{"all over budget", 10, 10, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := FrameStats{
				FramesRendered:   tt.rendered,
				OverBudgetFrames: tt.over,
			}
			got := s.OverBudgetRatio()
			if got != tt.want {
				t.Errorf("OverBudgetRatio() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestFrameTickCmd(t *testing.T) {
	cmd := frameTickCmd()
	if cmd == nil {
		t.Fatal("frameTickCmd returned nil")
	}
}

// TestFrameTickCmd_ExecutesCallback verifies that the command returned by
// frameTickCmd() executes the inner callback and delivers a frameTickMsg.
// This covers the anonymous function body inside frameTickCmd.
func TestFrameTickCmd_ExecutesCallback(t *testing.T) {
	cmd := frameTickCmd()
	if cmd == nil {
		t.Fatal("frameTickCmd returned nil")
	}

	// Execute the command — it blocks for ~16ms (FrameTickInterval) then returns.
	done := make(chan tea.Msg, 1)
	go func() {
		done <- cmd()
	}()

	select {
	case msg := <-done:
		if _, ok := msg.(frameTickMsg); !ok {
			t.Errorf("frameTickCmd callback returned %T, want frameTickMsg", msg)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("frameTickCmd callback did not fire within 200ms")
	}
}

func TestFrameBudgetConstants(t *testing.T) {
	if FrameBudget != 16*time.Millisecond {
		t.Errorf("FrameBudget = %v, want 16ms", FrameBudget)
	}
	if FrameTickInterval != 16*time.Millisecond {
		t.Errorf("FrameTickInterval = %v, want 16ms", FrameTickInterval)
	}
}

func TestFrameRateGovernor_ConcurrentAccess(t *testing.T) {
	g := NewFrameRateGovernor()

	done := make(chan struct{})

	// Concurrent readers
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = g.ShouldSkipNonEssential()
				_ = g.AverageFrameDuration()
				_ = g.LastFrameDuration()
				_ = g.Stats()
				_ = g.IsActive()
			}
		}()
	}

	// Concurrent writers
	for i := 0; i < 2; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				g.FrameStart()
				g.FrameEnd()
				g.RecordSkip()
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 6; i++ {
		<-done
	}
}

// TestSessionDashboard_FrameGovernorIntegration tests that the frame governor
// is properly created and accessible on the session dashboard model.
func TestSessionDashboard_FrameGovernorIntegration(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", "/tmp/projectdir", scanner, monitor)

	if m.FrameGovernor() == nil {
		t.Fatal("frame governor should not be nil on new model")
	}
	if !m.FrameGovernor().IsActive() {
		t.Error("frame governor should be active on new model")
	}
}

// TestSessionDashboard_FrameGovernorStopsOnClose tests that closeAll stops the governor.
func TestSessionDashboard_FrameGovernorStopsOnClose(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", "/tmp/projectdir", scanner, monitor)
	m.closeAll()

	if m.FrameGovernor().IsActive() {
		t.Error("frame governor should be stopped after closeAll")
	}
}

// TestSessionDashboard_FrameTickHandling tests that frameTickMsg is handled correctly.
func TestSessionDashboard_FrameTickHandling(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Add two panes manually
	m.panes = append(m.panes,
		SessionPaneModel{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "s1"}},
			width:   60, height: 20,
			dirty: true,
		},
		SessionPaneModel{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 200, SessionID: "s2"}},
			width:   60, height: 20,
			dirty: true,
		},
	)
	m.focusIndex = 0

	// Under budget: frameTickMsg should NOT clear any dirty flags
	newModel, cmd := m.Update(frameTickMsg{time: time.Now()})
	updated := newModel.(SessionDashboardModel)

	if cmd == nil {
		t.Error("frameTickMsg should return a new tick command")
	}
	// Both panes should remain dirty (under budget, no samples yet)
	if !updated.panes[0].dirty {
		t.Error("focused pane should remain dirty when under budget")
	}
	if !updated.panes[1].dirty {
		t.Error("unfocused pane should remain dirty when under budget")
	}
}

// TestSessionDashboard_FrameTickSkipsNonEssential tests that under budget pressure,
// non-essential (unfocused, non-grid-dirty) pane redraws are skipped.
func TestSessionDashboard_FrameTickSkipsNonEssential(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Simulate over-budget frames in governor
	gov := m.frameGovernor
	for i := 0; i < frameSampleSize; i++ {
		gov.FrameStart()
		gov.mu.Lock()
		gov.lastFrameStart = time.Now().Add(-20 * time.Millisecond) // Over 16ms budget
		gov.mu.Unlock()
		gov.FrameEnd()
	}

	// Add two panes: index 0 is focused, index 1 is not
	m.panes = append(m.panes,
		SessionPaneModel{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "s1"}},
			width:   60, height: 20,
			dirty: true,
		},
		SessionPaneModel{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 200, SessionID: "s2"}},
			width:   60, height: 20,
			dirty: true,
		},
	)
	m.focusIndex = 0
	m.gridDirty = false // Clear initial grid dirty — simulating steady-state

	newModel, cmd := m.Update(frameTickMsg{time: time.Now()})
	updated := newModel.(SessionDashboardModel)

	if cmd == nil {
		t.Error("frameTickMsg should return a new tick command")
	}

	// Focused pane should remain dirty (essential)
	if !updated.panes[0].dirty {
		t.Error("focused pane (index 0) should remain dirty - essential redraw")
	}

	// Unfocused pane should remain dirty (skip defers render, doesn't discard)
	if !updated.panes[1].dirty {
		t.Error("unfocused pane (index 1) should remain dirty - render deferred, not discarded")
	}

	// Governor should have recorded the skip
	stats := updated.FrameGovernor().Stats()
	if stats.FramesSkipped == 0 {
		t.Error("governor should have recorded at least one skip")
	}
}

// TestSessionDashboard_FrameTickGridDirtyOverridesSkip tests that when gridDirty
// is true, no panes are skipped even under budget pressure.
func TestSessionDashboard_FrameTickGridDirtyOverridesSkip(t *testing.T) {
	checker := newTestPIDChecker(100, 200)
	sessDir := t.TempDir()
	projectDir := t.TempDir()
	projectPath := "/tmp/test-project"

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel(projectPath, projectDir, scanner, monitor)
	m.SetSize(120, 40)

	// Simulate over-budget frames
	gov := m.frameGovernor
	for i := 0; i < frameSampleSize; i++ {
		gov.FrameStart()
		gov.mu.Lock()
		gov.lastFrameStart = time.Now().Add(-20 * time.Millisecond)
		gov.mu.Unlock()
		gov.FrameEnd()
	}

	// Add panes with gridDirty = true
	m.panes = append(m.panes,
		SessionPaneModel{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 100, SessionID: "s1"}},
			width:   60, height: 20,
			dirty: true,
		},
		SessionPaneModel{
			session: session.ActiveSession{Meta: session.SessionMeta{PID: 200, SessionID: "s2"}},
			width:   60, height: 20,
			dirty: true,
		},
	)
	m.focusIndex = 0
	m.gridDirty = true // Grid layout changed — all panes essential

	newModel, _ := m.Update(frameTickMsg{time: time.Now()})
	updated := newModel.(SessionDashboardModel)

	// Both panes should remain dirty since gridDirty overrides skip
	if !updated.panes[0].dirty {
		t.Error("pane 0 should remain dirty when gridDirty is true")
	}
	if !updated.panes[1].dirty {
		t.Error("pane 1 should remain dirty when gridDirty is true")
	}
}

// TestSessionDashboard_FrameTickInactive tests that frameTickMsg is a no-op
// when subscriptions are not active.
func TestSessionDashboard_FrameTickInactive(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", "/tmp/projectdir", scanner, monitor)
	m.subscriptionsActive = false

	_, cmd := m.Update(frameTickMsg{time: time.Now()})

	if cmd != nil {
		t.Error("frameTickMsg should return nil cmd when subscriptions inactive")
	}
}

// TestSessionDashboard_ViewTracksFrameBudget tests that View() records
// frame start/end for budget tracking.
func TestSessionDashboard_ViewTracksFrameBudget(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/project", "/tmp/projectdir", scanner, monitor)
	m.SetSize(80, 24)

	// Call View() (no panes — quick render)
	_ = m.View()

	stats := m.FrameGovernor().Stats()
	if stats.FramesRendered != 1 {
		t.Errorf("after one View(), frames rendered = %d, want 1", stats.FramesRendered)
	}
	if stats.LastDuration == 0 {
		t.Error("after View(), last duration should be > 0")
	}
}
