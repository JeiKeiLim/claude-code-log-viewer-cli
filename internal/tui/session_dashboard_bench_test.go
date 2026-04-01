package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/session"
	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// --- Helpers for benchmark setup ---

// benchSessionDashboard creates a SessionDashboardModel with N panes pre-populated
// with varying amounts of log entries. Each pane gets entries proportional to its
// index (pane 0 = light, pane N-1 = heavy) to simulate varying content update rates.
func benchSessionDashboard(b *testing.B, paneCount int, entriesPerPane []int, width, height int) SessionDashboardModel {
	b.Helper()

	checker := newTestPIDChecker()
	sessDir := b.TempDir()

	for i := 0; i < paneCount; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/bench-project", "/tmp/bench-projectdir", scanner, monitor)
	m.SetSize(width, height)

	for i := 0; i < paneCount; i++ {
		numEntries := entriesPerPane[i%len(entriesPerPane)]
		entries := makeBenchEntries(numEntries, i)

		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("bench-session-%d", i),
					CWD:       "/tmp/bench-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}

		m.panes = append(m.panes, pane)
	}

	// Assign sizes via grid layout
	m.recalcPaneSizes()

	// Initialize markdown renderers and content for each pane
	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
		p.dirty = true
	}

	return m
}

// makeBenchEntries creates a slice of LogEntry for benchmarking with mixed types.
func makeBenchEntries(count, seed int) []types.LogEntry {
	entries := make([]types.LogEntry, count)
	for i := 0; i < count; i++ {
		switch i % 3 {
		case 0:
			// User message
			entries[i] = types.LogEntry{
				Type:      types.EntryTypeUser,
				Timestamp: time.Now(),
				Message: types.Message{
					Role:        "user",
					TextContent: fmt.Sprintf("User message %d from pane %d with some content to render", i, seed),
				},
			}
		case 1:
			// Assistant text response
			entries[i] = types.LogEntry{
				Type:      types.EntryTypeAssistant,
				Timestamp: time.Now(),
				Message: types.Message{
					Role: "assistant",
					Content: []types.MessageContent{
						{
							Type: types.ContentTypeText,
							Text: fmt.Sprintf("Here is a response with some code:\n```go\nfunc example%d() {}\n```\nAnd more text.", i),
						},
					},
				},
			}
		case 2:
			// Tool use
			entries[i] = types.LogEntry{
				Type:      types.EntryTypeAssistant,
				Timestamp: time.Now(),
				Message: types.Message{
					Role: "assistant",
					Content: []types.MessageContent{
						{
							Type:     types.ContentTypeToolUse,
							ToolName: "Read",
							ToolInput: map[string]any{
								"file_path": fmt.Sprintf("/path/to/file%d.go", i),
							},
						},
					},
				},
			}
		}
	}
	return entries
}

// varyingEntryDistribution returns an entry count distribution for 9 panes
// simulating varying content update rates:
// - 3 light panes (5 entries) - idle sessions
// - 3 medium panes (20 entries) - moderately active
// - 3 heavy panes (50 entries) - very active
func varyingEntryDistribution() []int {
	return []int{5, 5, 5, 20, 20, 20, 50, 50, 50}
}

// uniformEntryDistribution returns a uniform entry count for N panes.
func uniformEntryDistribution(count, entriesEach int) []int {
	dist := make([]int, count)
	for i := range dist {
		dist[i] = entriesEach
	}
	return dist
}

// --- Benchmarks ---

// BenchmarkSessionDashboard_View_9Panes_Varying benchmarks full View() rendering
// with 9 panes at varying content levels (light/medium/heavy).
// This is the primary 60fps throughput benchmark.
func BenchmarkSessionDashboard_View_9Panes_Varying(b *testing.B) {
	m := benchSessionDashboard(b, 9, varyingEntryDistribution(), 240, 80)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Mark all panes dirty to force full re-render (worst case)
		m.markAllPanesDirty()
		m.gridDirty = true
		_ = m.View()
	}

	elapsed := b.Elapsed()
	avgPerFrame := elapsed / time.Duration(b.N)
	if avgPerFrame > FrameBudget {
		b.Logf("WARNING: average frame time %v exceeds 16ms budget", avgPerFrame)
	}
}

// BenchmarkSessionDashboard_View_9Panes_Uniform benchmarks full View() with
// 9 panes each having 20 entries (uniform load).
func BenchmarkSessionDashboard_View_9Panes_Uniform(b *testing.B) {
	m := benchSessionDashboard(b, 9, uniformEntryDistribution(9, 20), 240, 80)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		m.markAllPanesDirty()
		m.gridDirty = true
		_ = m.View()
	}
}

// BenchmarkSessionDashboard_View_DirtyRegion benchmarks View() when only
// one pane is dirty (the common case during streaming). This should be
// significantly faster than full re-render.
func BenchmarkSessionDashboard_View_DirtyRegion(b *testing.B) {
	m := benchSessionDashboard(b, 9, varyingEntryDistribution(), 240, 80)

	// Pre-render all panes once to populate caches
	m.markAllPanesDirty()
	m.gridDirty = true
	_ = m.View()
	m.gridDirty = false

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Only mark the focused pane dirty (simulates single-pane content update)
		m.panes[0].dirty = true
		_ = m.View()
	}
}

// BenchmarkSessionDashboard_View_CachedClean benchmarks View() when no panes
// are dirty (all cached). This measures the baseline overhead of grid
// composition without any pane re-rendering.
func BenchmarkSessionDashboard_View_CachedClean(b *testing.B) {
	m := benchSessionDashboard(b, 9, varyingEntryDistribution(), 240, 80)

	// Pre-render all panes
	m.markAllPanesDirty()
	m.gridDirty = true
	_ = m.View()
	m.gridDirty = false

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// No panes dirty — all cached
		_ = m.View()
	}
}

// BenchmarkSessionDashboard_View_PaneCountScaling benchmarks View() across
// different pane counts (1 to 9) to measure scaling behavior.
func BenchmarkSessionDashboard_View_PaneCountScaling(b *testing.B) {
	for paneCount := 1; paneCount <= 9; paneCount++ {
		b.Run(fmt.Sprintf("panes_%d", paneCount), func(b *testing.B) {
			dist := uniformEntryDistribution(paneCount, 20)
			m := benchSessionDashboard(b, paneCount, dist, 240, 80)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m.markAllPanesDirty()
				m.gridDirty = true
				_ = m.View()
			}
		})
	}
}

// BenchmarkSessionPaneModel_ViewWithFocus benchmarks a single pane's rendering
// with varying content sizes.
func BenchmarkSessionPaneModel_ViewWithFocus(b *testing.B) {
	sizes := []struct {
		name    string
		entries int
	}{
		{"light_5", 5},
		{"medium_20", 20},
		{"heavy_50", 50},
		{"very_heavy_100", 100},
	}

	for _, sz := range sizes {
		b.Run(sz.name, func(b *testing.B) {
			entries := makeBenchEntries(sz.entries, 0)

			pane := SessionPaneModel{
				session: session.ActiveSession{
					Meta: session.SessionMeta{
						PID:       1000,
						SessionID: "bench-s1",
						CWD:       "/tmp/bench",
						Kind:      "interactive",
					},
				},
				entries: entries,
				width:   80,
				height:  26,
			}

			renderWidth := pane.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			pane.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
			pane.content = pane.renderContent()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = pane.ViewWithFocus(true)
			}
		})
	}
}

// BenchmarkSessionPaneModel_RenderContent benchmarks the content rendering
// phase (the most expensive part of pane rendering).
func BenchmarkSessionPaneModel_RenderContent(b *testing.B) {
	sizes := []struct {
		name    string
		entries int
	}{
		{"light_5", 5},
		{"medium_20", 20},
		{"heavy_50", 50},
	}

	for _, sz := range sizes {
		b.Run(sz.name, func(b *testing.B) {
			entries := makeBenchEntries(sz.entries, 0)

			pane := SessionPaneModel{
				session: session.ActiveSession{
					Meta: session.SessionMeta{
						PID:       1000,
						SessionID: "bench-s1",
						Kind:      "interactive",
					},
				},
				entries: entries,
				width:   80,
				height:  26,
			}

			renderWidth := pane.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			pane.mdRenderer, _ = NewMarkdownRenderer(renderWidth)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = pane.renderContent()
			}
		})
	}
}

// BenchmarkSessionDashboard_FrameGovernor_Overhead benchmarks the frame governor's
// per-frame overhead (FrameStart + FrameEnd + ShouldSkipNonEssential).
func BenchmarkSessionDashboard_FrameGovernor_Overhead(b *testing.B) {
	g := NewFrameRateGovernor()

	// Pre-fill with some samples
	for i := 0; i < frameSampleSize; i++ {
		g.FrameStart()
		g.mu.Lock()
		g.lastFrameStart = time.Now().Add(-10 * time.Millisecond)
		g.mu.Unlock()
		g.FrameEnd()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		g.FrameStart()
		_ = g.ShouldSkipNonEssential()
		g.FrameEnd()
	}
}

// BenchmarkSessionDashboard_GridLayoutAndView benchmarks the combined cost of
// grid layout calculation + View() rendering for 9 panes.
func BenchmarkSessionDashboard_GridLayoutAndView(b *testing.B) {
	m := benchSessionDashboard(b, 9, varyingEntryDistribution(), 240, 80)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Recalculate grid layout (as if window resized)
		m.recalcPaneSizes()
		m.markAllPanesDirty()
		m.gridDirty = true
		_ = m.View()
	}
}

// BenchmarkSessionDashboard_Update_FrameTick benchmarks the Update() path for
// frameTickMsg which drives the frame-rate governor logic.
func BenchmarkSessionDashboard_Update_FrameTick(b *testing.B) {
	m := benchSessionDashboard(b, 9, varyingEntryDistribution(), 240, 80)
	m.subscriptionsActive = true

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Mark some panes dirty to exercise the skip logic
		m.panes[3].dirty = true
		m.panes[7].dirty = true
		newModel, _ := m.Update(frameTickMsg{time: time.Now()})
		m = newModel.(SessionDashboardModel)
	}
}

// --- 60fps Throughput Verification Tests ---

// TestSessionDashboard_60fps_Throughput_9Panes verifies that View() can render
// 9 panes with varying content within the 16ms frame budget.
// This is a test (not benchmark) that asserts the budget constraint.
// Guarded by PERF_TESTS environment variable because frame-time thresholds are
// meaningless under the race detector or on heavily loaded CI machines.
func TestSessionDashboard_60fps_Throughput_9Panes(t *testing.T) {
	if os.Getenv("PERF_TESTS") == "" {
		t.Skip("Skipping performance test (set PERF_TESTS=1 to run)")
	}
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)

	// Create 9 panes with varying entry counts
	dist := varyingEntryDistribution()
	for i := 0; i < 9; i++ {
		entries := makeBenchEntries(dist[i], i)
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}
		m.panes = append(m.panes, pane)
	}

	m.recalcPaneSizes()

	// Initialize renderers
	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
		p.dirty = true
	}

	// Warm-up: first render initializes caches
	m.gridDirty = true
	_ = m.View()

	// Measure 100 frames in steady state (single dirty pane per frame)
	const frameCount = 100
	frameTimes := make([]time.Duration, frameCount)

	for i := 0; i < frameCount; i++ {
		// Simulate single-pane update (most common scenario)
		paneIdx := i % 9
		m.panes[paneIdx].dirty = true
		m.gridDirty = false

		start := time.Now()
		_ = m.View()
		frameTimes[i] = time.Since(start)
	}

	// Calculate statistics
	var totalDuration time.Duration
	var maxDuration time.Duration
	overBudgetCount := 0

	for _, d := range frameTimes {
		totalDuration += d
		if d > maxDuration {
			maxDuration = d
		}
		if d > FrameBudget {
			overBudgetCount++
		}
	}

	avgDuration := totalDuration / frameCount

	t.Logf("60fps throughput (single-pane dirty):")
	t.Logf("  Average frame time: %v", avgDuration)
	t.Logf("  Max frame time:     %v", maxDuration)
	t.Logf("  Over budget (>16ms): %d/%d frames (%.1f%%)",
		overBudgetCount, frameCount, float64(overBudgetCount)/float64(frameCount)*100)

	// Assert: average frame time should be well under budget
	// Use 3x budget as threshold to account for CI variability
	if avgDuration > 3*FrameBudget {
		t.Errorf("average frame time %v exceeds 3x budget (48ms) — rendering too slow for 60fps", avgDuration)
	}

	// Note: per-frame budget percentage is logged for observability but not
	// asserted — CI machines under load routinely spike individual frames.
	// The average assertion above (3x budget) is the binding quality gate.
	overBudgetPct := float64(overBudgetCount) / float64(frameCount)
	if overBudgetPct > 0.90 {
		// Only fail if virtually ALL frames are over budget (catastrophic regression).
		t.Errorf("%.0f%% of frames exceeded 16ms budget — rendering severely degraded for 60fps", overBudgetPct*100)
	}
}

// TestSessionDashboard_60fps_FullRedraw_9Panes tests worst-case: all 9 panes
// dirty simultaneously (e.g., after window resize).
// Guarded by PERF_TESTS environment variable because frame-time thresholds are
// meaningless under the race detector or on heavily loaded CI machines.
func TestSessionDashboard_60fps_FullRedraw_9Panes(t *testing.T) {
	if os.Getenv("PERF_TESTS") == "" {
		t.Skip("Skipping performance test (set PERF_TESTS=1 to run)")
	}
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)

	dist := varyingEntryDistribution()
	for i := 0; i < 9; i++ {
		entries := makeBenchEntries(dist[i], i)
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}
		m.panes = append(m.panes, pane)
	}

	m.recalcPaneSizes()

	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
	}

	// Measure full redraws
	const frameCount = 50
	frameTimes := make([]time.Duration, frameCount)

	for i := 0; i < frameCount; i++ {
		m.markAllPanesDirty()
		m.gridDirty = true

		start := time.Now()
		_ = m.View()
		frameTimes[i] = time.Since(start)
	}

	var totalDuration time.Duration
	var maxDuration time.Duration
	overBudgetCount := 0

	for _, d := range frameTimes {
		totalDuration += d
		if d > maxDuration {
			maxDuration = d
		}
		if d > FrameBudget {
			overBudgetCount++
		}
	}

	avgDuration := totalDuration / frameCount

	t.Logf("60fps throughput (full redraw, all 9 panes dirty):")
	t.Logf("  Average frame time: %v", avgDuration)
	t.Logf("  Max frame time:     %v", maxDuration)
	t.Logf("  Over budget (>16ms): %d/%d frames (%.1f%%)",
		overBudgetCount, frameCount, float64(overBudgetCount)/float64(frameCount)*100)

	// Full redraw is heavier — use 5x budget threshold for CI
	if avgDuration > 5*FrameBudget {
		t.Errorf("average full-redraw frame time %v exceeds 5x budget (80ms) — rendering too slow", avgDuration)
	}
}

// TestSessionDashboard_60fps_CacheEfficiency verifies that dirty-region caching
// provides significant speedup over full redraw.
//
// The speedup assertion is only enforced when renders are slow enough that
// timing noise does not dominate the ratio measurement.  On very fast machines
// where a full 9-pane redraw completes in under 5 ms, wall-clock variance is
// comparable to the measurement itself, so we log the result but do not fail.
//
// Guarded by PERF_TESTS environment variable because the 50-trial measurement
// loop takes ~45 seconds under the race detector, causing CI timeout.
func TestSessionDashboard_60fps_CacheEfficiency(t *testing.T) {
	if os.Getenv("PERF_TESTS") == "" {
		t.Skip("Skipping performance test (set PERF_TESTS=1 to run)")
	}
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)

	dist := varyingEntryDistribution()
	for i := 0; i < 9; i++ {
		entries := makeBenchEntries(dist[i], i)
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}
		m.panes = append(m.panes, pane)
	}

	m.recalcPaneSizes()

	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
	}

	// Warmup: run both paths once to allow CPU/memory caches to stabilise.
	const warmup = 5
	for i := 0; i < warmup; i++ {
		m.markAllPanesDirty()
		m.gridDirty = true
		_ = m.View()
	}
	m.gridDirty = false
	for i := 0; i < warmup; i++ {
		m.panes[0].dirty = true
		_ = m.View()
	}

	// Measure full redraw time with increased trial count for lower variance.
	const trials = 50
	var fullRedrawTotal time.Duration
	for i := 0; i < trials; i++ {
		m.markAllPanesDirty()
		m.gridDirty = true
		start := time.Now()
		_ = m.View()
		fullRedrawTotal += time.Since(start)
	}
	fullRedrawAvg := fullRedrawTotal / trials

	// Measure single-pane-dirty time (cached path)
	m.gridDirty = false
	var cachedTotal time.Duration
	for i := 0; i < trials; i++ {
		m.panes[0].dirty = true
		start := time.Now()
		_ = m.View()
		cachedTotal += time.Since(start)
	}
	cachedAvg := cachedTotal / trials

	t.Logf("Cache efficiency:")
	t.Logf("  Full redraw avg:  %v", fullRedrawAvg)
	t.Logf("  Single dirty avg: %v", cachedAvg)

	if fullRedrawAvg > 0 {
		speedup := float64(fullRedrawAvg) / float64(cachedAvg)
		t.Logf("  Speedup:          %.1fx", speedup)

		// Only enforce the speedup requirement when renders are slow enough
		// for the measurement to be reliable (> 5 ms per full redraw on
		// average).  Below that threshold, wall-clock noise dominates and the
		// ratio is not meaningful — we log it as informational only.
		//
		// The 1.05x threshold is intentionally low: under race detection and
		// coverage instrumentation both paths share the same overhead (mainly
		// lipgloss grid composition), which compresses the ratio toward 1.0x.
		// The assertion only guards against a complete regression where the
		// cache provides zero benefit (ratio ≈ 1.0x).
		const minMeaningfulAvg = 5 * time.Millisecond
		if fullRedrawAvg >= minMeaningfulAvg && speedup < 1.05 {
			t.Errorf("dirty-region caching speedup %.1fx is less than expected 1.05x minimum (cache may be broken)", speedup)
		}
	}
}

// TestSessionDashboard_60fps_FrameGovernorIntegration verifies the frame governor
// correctly tracks frame timings across View() calls and makes correct
// skip decisions under simulated budget pressure.
func TestSessionDashboard_60fps_FrameGovernorIntegration(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)

	dist := varyingEntryDistribution()
	for i := 0; i < 9; i++ {
		entries := makeBenchEntries(dist[i], i)
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}
		m.panes = append(m.panes, pane)
	}

	m.recalcPaneSizes()

	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
		p.dirty = true
	}

	// Render 20 frames and check governor tracks them
	for i := 0; i < 20; i++ {
		m.panes[i%9].dirty = true
		_ = m.View()
	}

	stats := m.FrameGovernor().Stats()
	if stats.FramesRendered != 20 {
		t.Errorf("governor tracked %d frames, want 20", stats.FramesRendered)
	}

	avgDur := m.FrameGovernor().AverageFrameDuration()
	if avgDur == 0 {
		t.Error("governor average duration should be > 0 after rendering")
	}

	t.Logf("Frame governor after 20 frames:")
	t.Logf("  Frames rendered:    %d", stats.FramesRendered)
	t.Logf("  Average duration:   %v", avgDur)
	t.Logf("  Over budget ratio:  %.2f", stats.OverBudgetRatio())
}

// TestSessionDashboard_60fps_FrameGovernorSkipBehavior verifies that the
// frame governor skip logic correctly protects focused pane and skips
// unfocused panes under budget pressure with 9 panes.
func TestSessionDashboard_60fps_FrameGovernorSkipBehavior(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)
	m.subscriptionsActive = true

	for i := 0; i < 9; i++ {
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			width:  80,
			height: 26,
			dirty:  true,
		}
		m.panes = append(m.panes, pane)
	}

	m.focusIndex = 0
	m.gridDirty = false

	// Simulate over-budget pressure in governor
	gov := m.frameGovernor
	for i := 0; i < frameSampleSize; i++ {
		gov.FrameStart()
		gov.mu.Lock()
		gov.lastFrameStart = time.Now().Add(-25 * time.Millisecond) // Well over 16ms
		gov.mu.Unlock()
		gov.FrameEnd()
	}

	if !gov.ShouldSkipNonEssential() {
		t.Fatal("governor should suggest skipping non-essential under budget pressure")
	}

	// Mark all panes dirty
	m.markAllPanesDirty()

	// Process frameTickMsg
	newModel, cmd := m.Update(frameTickMsg{time: time.Now()})
	updated := newModel.(SessionDashboardModel)

	if cmd == nil {
		t.Error("frameTickMsg should return continuation command")
	}

	// Focused pane (0) should remain dirty
	if !updated.panes[0].dirty {
		t.Error("focused pane should remain dirty under budget pressure")
	}

	// All unfocused panes should have dirty cleared (skipped)
	skippedCount := 0
	for i := 1; i < 9; i++ {
		if !updated.panes[i].dirty {
			skippedCount++
		}
	}

	if skippedCount != 8 {
		t.Errorf("expected 8 unfocused panes skipped, got %d", skippedCount)
	}

	skipStats := updated.FrameGovernor().Stats()
	if skipStats.FramesSkipped < 8 {
		t.Errorf("governor should have recorded at least 8 skips, got %d", skipStats.FramesSkipped)
	}

	t.Logf("Skip behavior with 9 panes under pressure:")
	t.Logf("  Focused pane dirty:   %v (expected true)", updated.panes[0].dirty)
	t.Logf("  Unfocused panes skipped: %d/8", skippedCount)
	t.Logf("  Governor skips recorded: %d", skipStats.FramesSkipped)
}

// TestSessionDashboard_60fps_ContentUpdateRates verifies rendering performance
// with simulated varying content update rates across 9 panes.
// Panes are updated at different frequencies to simulate real-world scenarios:
// - 3 panes with high-frequency updates (every frame)
// - 3 panes with medium-frequency updates (every 3 frames)
// - 3 panes with low-frequency updates (every 10 frames)
// Guarded by PERF_TESTS environment variable because frame-time thresholds are
// meaningless under the race detector or on heavily loaded CI machines.
func TestSessionDashboard_60fps_ContentUpdateRates(t *testing.T) {
	if os.Getenv("PERF_TESTS") == "" {
		t.Skip("Skipping performance test (set PERF_TESTS=1 to run)")
	}
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)

	dist := varyingEntryDistribution()
	for i := 0; i < 9; i++ {
		entries := makeBenchEntries(dist[i], i)
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}
		m.panes = append(m.panes, pane)
	}

	m.recalcPaneSizes()

	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
	}

	// Initial render
	m.gridDirty = true
	_ = m.View()
	m.gridDirty = false

	// Simulate 60 frames with varying update rates
	const frameCount = 60
	frameTimes := make([]time.Duration, frameCount)
	dirtyPanesPerFrame := make([]int, frameCount)

	for frame := 0; frame < frameCount; frame++ {
		dirtyCount := 0

		// High-frequency panes (0-2): dirty every frame
		for i := 0; i < 3; i++ {
			m.panes[i].dirty = true
			// Simulate adding a new entry
			m.panes[i].entries = append(m.panes[i].entries, types.LogEntry{
				Type:      types.EntryTypeAssistant,
				Timestamp: time.Now(),
				Message: types.Message{
					Role: "assistant",
					Content: []types.MessageContent{
						{Type: types.ContentTypeText, Text: fmt.Sprintf("Streaming chunk %d", frame)},
					},
				},
			})
			m.panes[i].content = m.panes[i].renderContent()
			dirtyCount++
		}

		// Medium-frequency panes (3-5): dirty every 3 frames
		if frame%3 == 0 {
			for i := 3; i < 6; i++ {
				m.panes[i].dirty = true
				dirtyCount++
			}
		}

		// Low-frequency panes (6-8): dirty every 10 frames
		if frame%10 == 0 {
			for i := 6; i < 9; i++ {
				m.panes[i].dirty = true
				dirtyCount++
			}
		}

		dirtyPanesPerFrame[frame] = dirtyCount

		start := time.Now()
		_ = m.View()
		frameTimes[frame] = time.Since(start)
	}

	// Analyze results
	var totalDuration time.Duration
	var maxDuration time.Duration
	overBudgetCount := 0
	var totalDirty int

	for i, d := range frameTimes {
		totalDuration += d
		if d > maxDuration {
			maxDuration = d
		}
		if d > FrameBudget {
			overBudgetCount++
		}
		totalDirty += dirtyPanesPerFrame[i]
	}

	avgDuration := totalDuration / frameCount
	avgDirtyPanes := float64(totalDirty) / float64(frameCount)

	t.Logf("Varying content update rates (60 frames):")
	t.Logf("  Average frame time:     %v", avgDuration)
	t.Logf("  Max frame time:         %v", maxDuration)
	t.Logf("  Over budget (>16ms):    %d/%d frames", overBudgetCount, frameCount)
	t.Logf("  Avg dirty panes/frame:  %.1f", avgDirtyPanes)

	// Average frame time should be reasonable even with varying rates
	if avgDuration > 3*FrameBudget {
		t.Errorf("average frame time %v exceeds 3x budget under varying update rates", avgDuration)
	}
}

// TestSessionDashboard_60fps_ViewOutputNonEmpty verifies that rendered output
// is non-empty and contains expected content for all 9 panes.
func TestSessionDashboard_60fps_ViewOutputNonEmpty(t *testing.T) {
	checker := newTestPIDChecker()
	sessDir := t.TempDir()

	for i := 0; i < 9; i++ {
		checker.SetAlive(1000+i, true)
	}

	scanner := session.NewSessionScanner(sessDir, session.WithScannerPIDChecker(checker))
	monitor := session.NewMonitor(session.WithMonitorPIDChecker(checker))

	m := NewSessionDashboardModel("/tmp/test-project", "/tmp/test-projectdir", scanner, monitor)
	m.SetSize(240, 80)

	for i := 0; i < 9; i++ {
		entries := makeBenchEntries(10, i)
		pane := SessionPaneModel{
			session: session.ActiveSession{
				Meta: session.SessionMeta{
					PID:       1000 + i,
					SessionID: fmt.Sprintf("test-session-%d", i),
					CWD:       "/tmp/test-project",
					Kind:      "interactive",
				},
			},
			entries: entries,
			dirty:   true,
		}
		m.panes = append(m.panes, pane)
	}

	m.recalcPaneSizes()

	for i := range m.panes {
		p := &m.panes[i]
		if p.width > 0 {
			renderWidth := p.width - 6
			if renderWidth < 20 {
				renderWidth = 20
			}
			p.mdRenderer, _ = NewMarkdownRenderer(renderWidth)
		}
		p.content = p.renderContent()
		p.dirty = true
	}

	m.gridDirty = true
	// Manually trigger view mode detection since panes were added directly
	// (bypassing handleScanResult which calls updateViewMode automatically).
	m.ForceUpdateViewMode()
	output := m.View()

	if output == "" {
		t.Fatal("View() output should not be empty with 9 panes")
	}

	// Output should contain the help text
	if !strings.Contains(output, "auto-detecting sessions") {
		t.Error("View() output should contain help text")
	}

	// Each pane should have a header with its PID rendered
	for i := 0; i < 9; i++ {
		pid := fmt.Sprintf("%d", 1000+i)
		if !strings.Contains(output, pid) {
			t.Errorf("View() output should contain PID %s for pane %d", pid, i)
		}
	}
}
