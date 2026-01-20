// Package usage provides OAuth credential access and usage display components.
package usage

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// UsageBarState represents the current state of the usage bar.
type UsageBarState int

const (
	StateLoading UsageBarState = iota
	StateNormal
	StateStale
	StateNotLoggedIn
	StateError
	StateRefreshing // Story 7.5: manual refresh in progress
)

// UsageBarStyles contains styles for the usage bar component.
// Defined here to allow dependency injection from tui package.
type UsageBarStyles struct {
	Container lipgloss.Style
	Label     lipgloss.Style
	Normal    lipgloss.Style
	Warning   lipgloss.Style
	Critical  lipgloss.Style
	Dimmed    lipgloss.Style
	Stale     lipgloss.Style
}

// UsageBarModel is a view component for displaying usage limits.
// It does NOT implement tea.Model - state is managed externally.
type UsageBarModel struct {
	limits *UsageLimits
	state  UsageBarState
	errMsg string
	width  int
	styles UsageBarStyles
}

// NewUsageBarModel creates a new usage bar in loading state.
// Styles must be provided via dependency injection.
func NewUsageBarModel(styles UsageBarStyles) *UsageBarModel {
	return &UsageBarModel{
		state:  StateLoading,
		styles: styles,
	}
}

// SetLoading sets the bar to loading state.
func (m *UsageBarModel) SetLoading() {
	m.state = StateLoading
	m.limits = nil
	m.errMsg = ""
}

// SetLimits updates the bar with usage data.
func (m *UsageBarModel) SetLimits(limits *UsageLimits, stale bool) {
	m.limits = limits
	if stale {
		m.state = StateStale
	} else {
		m.state = StateNormal
	}
	m.errMsg = ""
}

// SetNotLoggedIn sets the bar to not-logged-in state.
func (m *UsageBarModel) SetNotLoggedIn() {
	m.state = StateNotLoggedIn
	m.limits = nil
	m.errMsg = ""
}

// SetError sets the bar to error state with message.
func (m *UsageBarModel) SetError(msg string) {
	m.state = StateError
	m.limits = nil
	m.errMsg = msg
}

// SetRefreshing sets the bar to refreshing state (Story 7.5).
// Preserves current limits to show during refresh.
func (m *UsageBarModel) SetRefreshing() {
	// Only set if we have limits to show during refresh
	if m.limits != nil {
		m.state = StateRefreshing
	}
	// If no limits, stay in current state (loading will handle it)
}

// SetWidth sets the available width for rendering.
func (m *UsageBarModel) SetWidth(width int) {
	m.width = width
}

// Width returns the current width setting.
func (m *UsageBarModel) Width() int {
	return m.width
}

// State returns the current bar state.
func (m *UsageBarModel) State() UsageBarState {
	return m.state
}

// View renders the usage bar.
func (m *UsageBarModel) View() string {
	switch m.state {
	case StateLoading:
		return m.renderLoading()
	case StateNotLoggedIn:
		return m.renderNotLoggedIn()
	case StateError:
		return m.renderError()
	case StateRefreshing:
		return m.renderRefreshing()
	case StateNormal, StateStale:
		return m.renderUsage()
	default:
		return ""
	}
}

func (m *UsageBarModel) renderLoading() string {
	content := m.styles.Dimmed.Render("Loading usage...")
	return m.applyContainer(content)
}

func (m *UsageBarModel) renderNotLoggedIn() string {
	content := m.styles.Dimmed.Render("Not logged in")
	return m.applyContainer(content)
}

func (m *UsageBarModel) renderError() string {
	content := m.styles.Critical.Render(m.errMsg)
	return m.applyContainer(content)
}

func (m *UsageBarModel) renderRefreshing() string {
	if m.limits == nil {
		return m.renderLoading()
	}

	// Render current values with refresh indicator
	content := m.renderWindowParts()
	content += m.styles.Dimmed.Render(" [R]")

	return m.applyContainer(content)
}

func (m *UsageBarModel) renderUsage() string {
	if m.limits == nil {
		return m.renderLoading()
	}

	content := m.renderWindowParts()

	// Add stale indicator
	if m.state == StateStale {
		content += m.styles.Stale.Render(" (stale)")
	}

	return m.applyContainer(content)
}

// renderWindowParts renders the common window parts for both normal and refreshing states.
func (m *UsageBarModel) renderWindowParts() string {
	var parts []string

	if m.limits.FiveHour != nil {
		fiveHourPart := m.renderWindow("5h", m.limits.FiveHour, true)
		parts = append(parts, fiveHourPart)
	}

	if m.limits.SevenDay != nil {
		sevenDayPart := m.renderWindow("7d", m.limits.SevenDay, true)
		parts = append(parts, sevenDayPart)
	}

	content := ""
	for i, part := range parts {
		if i > 0 {
			content += m.styles.Label.Render(" ")
		}
		content += part
	}

	return content
}

func (m *UsageBarModel) renderWindow(label string, window *UsageWindow, showReset bool) string {
	// Determine style based on utilization threshold
	utilStyle := m.getUtilizationStyle(window.Utilization)

	// Format: [5h: 35% 2h 15m] or [7d: 12%]
	// All spaces must be inside styled regions to avoid background color gaps
	labelPart := m.styles.Label.Render("[" + label + ": ")
	utilPart := utilStyle.Render(fmt.Sprintf("%.0f%%", window.Utilization))

	result := labelPart + utilPart

	// Add reset time if available and requested
	if showReset && window.ResetsAt != nil {
		resetStr := formatDuration(window.ResetsAt)
		if resetStr != "" {
			result += m.styles.Normal.Render(" " + resetStr)
		}
	}

	result += m.styles.Label.Render("]")
	return result
}

func (m *UsageBarModel) getUtilizationStyle(utilization float64) lipgloss.Style {
	if utilization > 95 {
		return m.styles.Critical
	}
	if utilization > 80 {
		return m.styles.Warning
	}
	return m.styles.Normal
}

func (m *UsageBarModel) applyContainer(content string) string {
	if m.width > 0 {
		return m.styles.Container.Width(m.width).Render(content)
	}
	return m.styles.Container.Render(content)
}

// formatDuration returns a human-readable duration string until reset time.
func formatDuration(resetTime *time.Time) string {
	if resetTime == nil {
		return ""
	}

	remaining := time.Until(*resetTime)
	if remaining < 0 {
		return "" // Past reset time, omit
	}

	if remaining < time.Minute {
		return "soon"
	}

	days := int(remaining.Hours()) / 24
	hours := int(remaining.Hours()) % 24
	minutes := int(remaining.Minutes()) % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}
