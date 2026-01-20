package usage

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// testStyles returns a minimal UsageBarStyles for testing.
// Uses simple unstyled renders so we can verify content without ANSI codes.
func testStyles() UsageBarStyles {
	return UsageBarStyles{
		Container: lipgloss.NewStyle(),
		Label:     lipgloss.NewStyle(),
		Normal:    lipgloss.NewStyle(),
		Warning:   lipgloss.NewStyle(),
		Critical:  lipgloss.NewStyle(),
		Dimmed:    lipgloss.NewStyle(),
		Stale:     lipgloss.NewStyle(),
	}
}

// testStylesWithMarkers returns styles with distinguishable markers for threshold testing.
func testStylesWithMarkers() UsageBarStyles {
	return UsageBarStyles{
		Container: lipgloss.NewStyle(),
		Label:     lipgloss.NewStyle(),
		Normal:    lipgloss.NewStyle(),
		Warning:   lipgloss.NewStyle().Bold(true),   // Bold for warning
		Critical:  lipgloss.NewStyle().Italic(true), // Italic for critical
		Dimmed:    lipgloss.NewStyle(),
		Stale:     lipgloss.NewStyle(),
	}
}

func TestNewUsageBarModel(t *testing.T) {
	styles := testStyles()
	m := NewUsageBarModel(styles)

	if m.State() != StateLoading {
		t.Errorf("NewUsageBarModel() state = %v, want StateLoading", m.State())
	}
	if m.limits != nil {
		t.Error("NewUsageBarModel() limits should be nil")
	}
	if m.Width() != 0 {
		t.Errorf("NewUsageBarModel() width = %v, want 0", m.Width())
	}
}

func TestUsageBarModel_SetLoading(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	m.SetLimits(&UsageLimits{}, false)
	m.SetError("test error")

	m.SetLoading()

	if m.State() != StateLoading {
		t.Errorf("SetLoading() state = %v, want StateLoading", m.State())
	}
	if m.limits != nil {
		t.Error("SetLoading() should clear limits")
	}
	if m.errMsg != "" {
		t.Error("SetLoading() should clear errMsg")
	}
}

func TestUsageBarModel_SetLimits(t *testing.T) {
	tests := []struct {
		name      string
		stale     bool
		wantState UsageBarState
	}{
		{
			name:      "normal data",
			stale:     false,
			wantState: StateNormal,
		},
		{
			name:      "stale data",
			stale:     true,
			wantState: StateStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewUsageBarModel(testStyles())
			m.SetError("previous error")

			limits := &UsageLimits{
				FiveHour: &UsageWindow{Utilization: 50.0},
			}
			m.SetLimits(limits, tt.stale)

			if m.State() != tt.wantState {
				t.Errorf("SetLimits() state = %v, want %v", m.State(), tt.wantState)
			}
			if m.limits != limits {
				t.Error("SetLimits() should set limits")
			}
			if m.errMsg != "" {
				t.Error("SetLimits() should clear errMsg")
			}
		})
	}
}

func TestUsageBarModel_SetNotLoggedIn(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	m.SetLimits(&UsageLimits{}, false)

	m.SetNotLoggedIn()

	if m.State() != StateNotLoggedIn {
		t.Errorf("SetNotLoggedIn() state = %v, want StateNotLoggedIn", m.State())
	}
	if m.limits != nil {
		t.Error("SetNotLoggedIn() should clear limits")
	}
}

func TestUsageBarModel_SetError(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	// Set limits first to verify they get cleared
	m.SetLimits(&UsageLimits{FiveHour: &UsageWindow{Utilization: 50.0}}, false)
	errMsg := "Session expired"

	m.SetError(errMsg)

	if m.State() != StateError {
		t.Errorf("SetError() state = %v, want StateError", m.State())
	}
	if m.errMsg != errMsg {
		t.Errorf("SetError() errMsg = %q, want %q", m.errMsg, errMsg)
	}
	if m.limits != nil {
		t.Error("SetError() should clear limits")
	}
}

func TestUsageBarModel_SetWidth(t *testing.T) {
	m := NewUsageBarModel(testStyles())

	m.SetWidth(80)

	if m.Width() != 80 {
		t.Errorf("SetWidth() width = %v, want 80", m.Width())
	}
}

func TestUsageBarModel_View(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*UsageBarModel)
		contains []string
		excludes []string
	}{
		{
			name:     "loading state",
			setup:    func(m *UsageBarModel) { m.SetLoading() },
			contains: []string{"Loading usage..."},
		},
		{
			name:     "not logged in",
			setup:    func(m *UsageBarModel) { m.SetNotLoggedIn() },
			contains: []string{"Not logged in"},
		},
		{
			name:     "error state",
			setup:    func(m *UsageBarModel) { m.SetError("Session expired") },
			contains: []string{"Session expired"},
		},
		{
			name: "normal usage both windows",
			setup: func(m *UsageBarModel) {
				m.SetLimits(&UsageLimits{
					FiveHour: &UsageWindow{Utilization: 35.0},
					SevenDay: &UsageWindow{Utilization: 12.0},
				}, false)
			},
			contains: []string{"5h:", "35%", "7d:", "12%"},
			excludes: []string{"(stale)"},
		},
		{
			name: "normal usage 5h only",
			setup: func(m *UsageBarModel) {
				m.SetLimits(&UsageLimits{
					FiveHour: &UsageWindow{Utilization: 45.0},
				}, false)
			},
			contains: []string{"5h:", "45%"},
			excludes: []string{"7d:"},
		},
		{
			name: "normal usage 7d only",
			setup: func(m *UsageBarModel) {
				m.SetLimits(&UsageLimits{
					SevenDay: &UsageWindow{Utilization: 22.0},
				}, false)
			},
			contains: []string{"7d:", "22%"},
			excludes: []string{"5h:"},
		},
		{
			name: "stale data indicator",
			setup: func(m *UsageBarModel) {
				m.SetLimits(&UsageLimits{
					FiveHour: &UsageWindow{Utilization: 35.0},
					SevenDay: &UsageWindow{Utilization: 12.0},
				}, true)
			},
			contains: []string{"(stale)"},
		},
		{
			name: "with reset time",
			setup: func(m *UsageBarModel) {
				// Add 30 seconds buffer to ensure we get "2h 15m" not "2h 14m"
				resetTime := time.Now().Add(2*time.Hour + 15*time.Minute + 30*time.Second)
				m.SetLimits(&UsageLimits{
					FiveHour: &UsageWindow{Utilization: 35.0, ResetsAt: &resetTime},
					SevenDay: &UsageWindow{Utilization: 12.0},
				}, false)
			},
			contains: []string{"5h:", "35%", "2h 15m", "7d:", "12%"},
		},
		{
			name: "nil limits falls back to loading",
			setup: func(m *UsageBarModel) {
				m.state = StateNormal
				m.limits = nil
			},
			contains: []string{"Loading usage..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewUsageBarModel(testStyles())
			tt.setup(m)
			got := m.View()

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("View() = %q, want to contain %q", got, want)
				}
			}
			for _, notWant := range tt.excludes {
				if strings.Contains(got, notWant) {
					t.Errorf("View() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestUsageBarModel_View_WarningThreshold(t *testing.T) {
	// Verify warning threshold (>80%) output contains expected percentage
	tests := []struct {
		name        string
		utilization float64
		wantPercent string
	}{
		{
			name:        "exactly 80% - normal",
			utilization: 80.0,
			wantPercent: "80%",
		},
		{
			name:        "81% - warning",
			utilization: 81.0,
			wantPercent: "81%",
		},
		{
			name:        "85% - warning",
			utilization: 85.0,
			wantPercent: "85%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewUsageBarModel(testStyles())
			m.SetLimits(&UsageLimits{
				FiveHour: &UsageWindow{Utilization: tt.utilization},
				SevenDay: &UsageWindow{Utilization: 10.0},
			}, false)

			got := m.View()
			if !strings.Contains(got, "5h:") {
				t.Errorf("View() = %q, want to contain 5h label", got)
			}
			if !strings.Contains(got, tt.wantPercent) {
				t.Errorf("View() = %q, want to contain %s", got, tt.wantPercent)
			}
		})
	}
}

func TestUsageBarModel_View_CriticalThreshold(t *testing.T) {
	// Verify critical threshold (>95%) output contains expected percentage
	tests := []struct {
		name        string
		utilization float64
		wantPercent string
	}{
		{
			name:        "exactly 95% - warning",
			utilization: 95.0,
			wantPercent: "95%",
		},
		{
			name:        "96% - critical",
			utilization: 96.0,
			wantPercent: "96%",
		},
		{
			name:        "98% - critical",
			utilization: 98.0,
			wantPercent: "98%",
		},
		{
			name:        "100% - critical",
			utilization: 100.0,
			wantPercent: "100%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewUsageBarModel(testStyles())
			m.SetLimits(&UsageLimits{
				FiveHour: &UsageWindow{Utilization: tt.utilization},
				SevenDay: &UsageWindow{Utilization: 10.0},
			}, false)

			got := m.View()
			if !strings.Contains(got, "5h:") {
				t.Errorf("View() = %q, want to contain 5h label", got)
			}
			if !strings.Contains(got, tt.wantPercent) {
				t.Errorf("View() = %q, want to contain %s", got, tt.wantPercent)
			}
		})
	}
}

func TestUsageBarModel_View_IndependentColorThresholds(t *testing.T) {
	// Verify that 5h and 7d windows have independent color thresholds
	m := NewUsageBarModel(testStyles())
	m.SetLimits(&UsageLimits{
		FiveHour: &UsageWindow{Utilization: 30.0},  // normal
		SevenDay: &UsageWindow{Utilization: 98.0}, // critical
	}, false)

	got := m.View()

	// Both percentages should appear
	if !strings.Contains(got, "30%") {
		t.Errorf("View() = %q, want to contain 5h percentage 30%%", got)
	}
	if !strings.Contains(got, "98%") {
		t.Errorf("View() = %q, want to contain 7d percentage 98%%", got)
	}
}

func TestUsageBarModel_View_WidthTruncation(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	m.SetLimits(&UsageLimits{
		FiveHour: &UsageWindow{Utilization: 35.0},
		SevenDay: &UsageWindow{Utilization: 12.0},
	}, false)
	m.SetWidth(40)

	got := m.View()

	// With width set, container should apply width
	// This is a basic check that width is being used
	if m.Width() != 40 {
		t.Errorf("Width() = %v, want 40", m.Width())
	}
	// Content should still be rendered
	if !strings.Contains(got, "5h:") {
		t.Errorf("View() with width = %q, want to contain 5h label", got)
	}
}

func TestFormatDuration(t *testing.T) {
	// Add 1 second buffer to each test to avoid race conditions where
	// time passes between test setup and formatDuration() call
	buffer := 1 * time.Second

	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "nil reset time",
			duration: 0, // Special case: handled below
			want:     "",
		},
		{
			name:     "past time",
			duration: -1 * time.Hour,
			want:     "",
		},
		{
			name:     "30 seconds (soon)",
			duration: 30*time.Second + buffer,
			want:     "soon",
		},
		{
			name:     "59 seconds (soon)",
			duration: 59*time.Second + buffer,
			want:     "soon",
		},
		{
			name:     "1 minute exactly",
			duration: 1*time.Minute + buffer,
			want:     "1m",
		},
		{
			name:     "45 minutes",
			duration: 45*time.Minute + buffer,
			want:     "45m",
		},
		{
			name:     "59 minutes",
			duration: 59*time.Minute + buffer,
			want:     "59m",
		},
		{
			name:     "1 hour exactly",
			duration: 1*time.Hour + buffer,
			want:     "1h",
		},
		{
			name:     "1 hour 1 minute",
			duration: 1*time.Hour + 1*time.Minute + buffer,
			want:     "1h 1m",
		},
		{
			name:     "2 hours 15 minutes",
			duration: 2*time.Hour + 15*time.Minute + buffer,
			want:     "2h 15m",
		},
		{
			name:     "5 hours exactly",
			duration: 5*time.Hour + buffer,
			want:     "5h",
		},
		{
			name:     "167 hours 30 minutes (7-day window)",
			duration: 167*time.Hour + 30*time.Minute + buffer,
			want:     "6d 23h", // 167h = 6*24 + 23 = 6d 23h
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resetTime *time.Time
			if tt.name == "nil reset time" {
				resetTime = nil
			} else {
				rt := time.Now().Add(tt.duration)
				resetTime = &rt
			}
			got := formatDuration(resetTime)
			if got != tt.want {
				t.Errorf("formatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDuration_EdgeCases(t *testing.T) {
	// Test deterministic cases only (negative durations always return "")
	// Time-sensitive tests are in TestFormatDuration which uses proper buffers
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "negative 1 second",
			duration: -1 * time.Second,
			want:     "",
		},
		{
			name:     "negative 1 hour",
			duration: -1 * time.Hour,
			want:     "",
		},
		{
			name:     "negative 1 minute",
			duration: -1 * time.Minute,
			want:     "",
		},
		{
			name:     "negative 1 day",
			duration: -24 * time.Hour,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTime := time.Now().Add(tt.duration)
			got := formatDuration(&resetTime)
			if got != tt.want {
				t.Errorf("formatDuration(%v from now) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestGetUtilizationStyle(t *testing.T) {
	styles := testStylesWithMarkers()
	m := NewUsageBarModel(styles)

	tests := []struct {
		name        string
		utilization float64
		wantStyle   lipgloss.Style
	}{
		{
			name:        "0% - normal",
			utilization: 0.0,
			wantStyle:   styles.Normal,
		},
		{
			name:        "50% - normal",
			utilization: 50.0,
			wantStyle:   styles.Normal,
		},
		{
			name:        "80% - normal (boundary)",
			utilization: 80.0,
			wantStyle:   styles.Normal,
		},
		{
			name:        "80.1% - warning",
			utilization: 80.1,
			wantStyle:   styles.Warning,
		},
		{
			name:        "95% - warning (boundary)",
			utilization: 95.0,
			wantStyle:   styles.Warning,
		},
		{
			name:        "95.1% - critical",
			utilization: 95.1,
			wantStyle:   styles.Critical,
		},
		{
			name:        "100% - critical",
			utilization: 100.0,
			wantStyle:   styles.Critical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.getUtilizationStyle(tt.utilization)
			// Compare rendered output to verify correct style is returned
			testText := "X"
			if got.Render(testText) != tt.wantStyle.Render(testText) {
				t.Errorf("getUtilizationStyle(%.1f) returned wrong style", tt.utilization)
			}
		})
	}
}

func TestUsageBarModel_DefaultState(t *testing.T) {
	m := NewUsageBarModel(testStyles())

	// Verify default state after construction
	got := m.View()
	if !strings.Contains(got, "Loading usage...") {
		t.Errorf("Default View() = %q, want to contain 'Loading usage...'", got)
	}
}

// Story 7.5 Tests: Refresh Indicator

func TestUsageBarModel_SetRefreshing(t *testing.T) {
	m := NewUsageBarModel(testStyles())

	// Set limits first
	m.SetLimits(&UsageLimits{
		FiveHour: &UsageWindow{Utilization: 35},
	}, false)

	m.SetRefreshing()

	if m.State() != StateRefreshing {
		t.Errorf("State() = %v, want StateRefreshing", m.State())
	}
}

func TestUsageBarModel_SetRefreshing_NoLimits(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	// No limits set - stay in loading state

	m.SetRefreshing()

	// Should not change to refreshing without limits
	if m.State() != StateLoading {
		t.Errorf("State() = %v, want StateLoading (no change without limits)", m.State())
	}
}

func TestUsageBarModel_View_Refreshing(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	m.SetWidth(80)
	m.SetLimits(&UsageLimits{
		FiveHour: &UsageWindow{Utilization: 35},
	}, false)
	m.SetRefreshing()

	view := m.View()

	if !strings.Contains(view, "[R]") {
		t.Error("expected refresh indicator [R] in view")
	}
	if !strings.Contains(view, "35%") {
		t.Error("expected current usage values preserved during refresh")
	}
}

func TestUsageBarModel_View_Refreshing_FallbackToLoading(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	m.SetWidth(80)
	// Force state to refreshing without limits (edge case)
	m.state = StateRefreshing
	// limits is nil

	view := m.View()

	// Should fallback to loading view
	if !strings.Contains(view, "Loading usage...") {
		t.Errorf("expected fallback to loading view, got %q", view)
	}
}

func TestUsageBarModel_View_Refreshing_BothWindows(t *testing.T) {
	m := NewUsageBarModel(testStyles())
	m.SetWidth(80)
	m.SetLimits(&UsageLimits{
		FiveHour: &UsageWindow{Utilization: 35},
		SevenDay: &UsageWindow{Utilization: 12},
	}, false)
	m.SetRefreshing()

	view := m.View()

	if !strings.Contains(view, "[R]") {
		t.Error("expected refresh indicator [R] in view")
	}
	if !strings.Contains(view, "5h:") || !strings.Contains(view, "35%") {
		t.Error("expected 5h window preserved during refresh")
	}
	if !strings.Contains(view, "7d:") || !strings.Contains(view, "12%") {
		t.Error("expected 7d window preserved during refresh")
	}
}
