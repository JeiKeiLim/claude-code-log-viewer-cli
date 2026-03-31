// Package tui provides the terminal user interface components.
package tui

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// FrameBudget is the target frame duration for 60fps rendering (~16ms).
const FrameBudget = 16 * time.Millisecond

// FrameTickInterval is the interval for the frame-rate governor tick loop.
// Set to 16ms for ~60fps target frame rate.
const FrameTickInterval = 16 * time.Millisecond

// frameTickMsg is sent by the frame-rate governor tick loop.
type frameTickMsg struct {
	time time.Time
}

// FrameRateGovernor manages frame timing and budget tracking for the TUI.
// It tracks how long each frame takes to render and determines whether
// non-essential redraws should be skipped when rendering exceeds budget.
type FrameRateGovernor struct {
	mu sync.RWMutex

	// Frame timing
	lastFrameStart    time.Time
	lastFrameDuration time.Duration

	// Budget tracking
	framesRendered   int64
	framesSkipped    int64
	overBudgetFrames int64

	// Rolling average of last N frame durations for smooth budget decisions
	frameDurations [frameSampleSize]time.Duration
	sampleIndex    int
	sampleCount    int

	// State
	active bool
}

const frameSampleSize = 16 // Power of 2 for efficient modulo

// NewFrameRateGovernor creates a new frame-rate governor.
func NewFrameRateGovernor() *FrameRateGovernor {
	return &FrameRateGovernor{
		active: true,
	}
}

// FrameStart records the start of a new frame.
// Call this at the beginning of each render cycle.
func (g *FrameRateGovernor) FrameStart() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastFrameStart = time.Now()
}

// FrameEnd records the end of a frame and updates budget tracking.
// Call this after the render cycle completes.
func (g *FrameRateGovernor) FrameEnd() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.lastFrameStart.IsZero() {
		return
	}

	g.lastFrameDuration = time.Since(g.lastFrameStart)
	g.framesRendered++

	// Add to rolling sample
	g.frameDurations[g.sampleIndex] = g.lastFrameDuration
	g.sampleIndex = (g.sampleIndex + 1) % frameSampleSize
	if g.sampleCount < frameSampleSize {
		g.sampleCount++
	}

	if g.lastFrameDuration > FrameBudget {
		g.overBudgetFrames++
	}
}

// ShouldSkipNonEssential returns true if non-essential redraws should be
// skipped due to frame budget pressure. Non-essential redraws include:
// - Panes that are not focused and have only minor content changes
// - Cosmetic updates (border animations, indicator timeouts)
//
// Essential redraws always proceed:
// - Grid layout changes (pane add/remove, resize)
// - Focused pane content updates
// - Initial content loads
func (g *FrameRateGovernor) ShouldSkipNonEssential() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.sampleCount == 0 {
		return false // No data yet, allow all renders
	}

	avg := g.averageFrameDuration()
	return avg > FrameBudget
}

// averageFrameDuration calculates the rolling average frame duration.
// Must be called with at least a read lock held.
func (g *FrameRateGovernor) averageFrameDuration() time.Duration {
	if g.sampleCount == 0 {
		return 0
	}

	var total time.Duration
	for i := 0; i < g.sampleCount; i++ {
		total += g.frameDurations[i]
	}
	return total / time.Duration(g.sampleCount)
}

// AverageFrameDuration returns the rolling average frame duration.
// Exported for testing and metrics.
func (g *FrameRateGovernor) AverageFrameDuration() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.averageFrameDuration()
}

// LastFrameDuration returns the duration of the most recent frame.
func (g *FrameRateGovernor) LastFrameDuration() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lastFrameDuration
}

// Stats returns the current frame statistics.
func (g *FrameRateGovernor) Stats() FrameStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return FrameStats{
		FramesRendered:   g.framesRendered,
		FramesSkipped:    g.framesSkipped,
		OverBudgetFrames: g.overBudgetFrames,
		AverageDuration:  g.averageFrameDuration(),
		LastDuration:     g.lastFrameDuration,
	}
}

// RecordSkip increments the skip counter for metrics tracking.
func (g *FrameRateGovernor) RecordSkip() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.framesSkipped++
}

// SampleCount returns the number of frame duration samples collected.
// Exported for testing.
func (g *FrameRateGovernor) SampleCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sampleCount
}

// Stop deactivates the governor.
func (g *FrameRateGovernor) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active = false
}

// IsActive returns whether the governor is active.
func (g *FrameRateGovernor) IsActive() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.active
}

// Reset clears all frame statistics and timing data.
func (g *FrameRateGovernor) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastFrameStart = time.Time{}
	g.lastFrameDuration = 0
	g.framesRendered = 0
	g.framesSkipped = 0
	g.overBudgetFrames = 0
	g.sampleIndex = 0
	g.sampleCount = 0
	g.frameDurations = [frameSampleSize]time.Duration{}
}

// FrameStats contains frame timing statistics.
type FrameStats struct {
	FramesRendered   int64
	FramesSkipped    int64
	OverBudgetFrames int64
	AverageDuration  time.Duration
	LastDuration     time.Duration
}

// OverBudgetRatio returns the ratio of frames that exceeded the budget.
func (s FrameStats) OverBudgetRatio() float64 {
	if s.FramesRendered == 0 {
		return 0
	}
	return float64(s.OverBudgetFrames) / float64(s.FramesRendered)
}

// frameTickCmd returns a command that schedules the next frame tick.
func frameTickCmd() tea.Cmd {
	return tea.Tick(FrameTickInterval, func(t time.Time) tea.Msg {
		return frameTickMsg{time: t}
	})
}
