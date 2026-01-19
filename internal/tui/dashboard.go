// Package tui provides the terminal user interface components.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// GoBackToProjectsFromDashboardMsg signals return to project list from dashboard.
type GoBackToProjectsFromDashboardMsg struct{}

// DashboardModel represents the multi-project dashboard view.
type DashboardModel struct {
	panes      []PaneModel
	focusIndex int
	width      int
	height     int
}

// PaneModel represents a single pane in the dashboard grid.
type PaneModel struct {
	project types.Project
	width   int
	height  int
}

// NewDashboardModel creates a new dashboard model with the given projects.
func NewDashboardModel(projects []types.Project) DashboardModel {
	panes := make([]PaneModel, len(projects))
	for i, p := range projects {
		panes[i] = PaneModel{project: p}
	}
	return DashboardModel{
		panes:      panes,
		focusIndex: 0,
	}
}

// Init implements tea.Model.
func (m DashboardModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return GoBackToProjectsFromDashboardMsg{} }
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m DashboardModel) View() string {
	// Handle edge case: no panes
	if len(m.panes) == 0 {
		return ""
	}

	rows, cols := calculateGrid(len(m.panes))
	if rows == 0 || cols == 0 {
		return ""
	}

	paneWidth, paneHeight := calculatePaneDimensions(m.width, m.height, rows, cols)

	// Build rows
	var rowViews []string
	for r := 0; r < rows; r++ {
		var colViews []string
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			if idx < len(m.panes) {
				// Render pane with calculated dimensions
				pane := m.panes[idx]
				pane.width = paneWidth
				pane.height = paneHeight
				colViews = append(colViews, pane.View())
			} else {
				// Empty cell for incomplete last row - render blank space matching pane dimensions
				colViews = append(colViews, lipgloss.NewStyle().
					Width(paneWidth).
					Height(paneHeight).
					Render(""))
			}
		}
		rowViews = append(rowViews, lipgloss.JoinHorizontal(lipgloss.Top, colViews...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rowViews...)
}

// SetSize updates the dashboard dimensions and recalculates pane sizes.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if len(m.panes) == 0 {
		return
	}

	rows, cols := calculateGrid(len(m.panes))
	paneWidth, paneHeight := calculatePaneDimensions(width, height, rows, cols)

	for i := range m.panes {
		m.panes[i].width = paneWidth
		m.panes[i].height = paneHeight
	}
}

// calculateGrid returns the grid dimensions (rows, cols) for a given project count.
// Grid mapping per PRD FR-502:
//   - 0: 0x0 (edge case)
//   - 1: 1x1
//   - 2: 1x2
//   - 3: 1x3
//   - 4: 2x2
//   - 5-6: 2x3
//   - 7+: 3x3 (max 9 per Story 5.1)
func calculateGrid(count int) (rows, cols int) {
	switch count {
	case 0:
		return 0, 0
	case 1:
		return 1, 1
	case 2:
		return 1, 2
	case 3:
		return 1, 3
	case 4:
		return 2, 2
	case 5, 6:
		return 2, 3
	default:
		return 3, 3 // 7+ projects (max 9 per Story 5.1)
	}
}

// calculatePaneDimensions returns the dimensions for each pane given total space and grid size.
func calculatePaneDimensions(totalWidth, totalHeight, rows, cols int) (paneWidth, paneHeight int) {
	// Guard against division by zero
	if rows == 0 || cols == 0 {
		return 0, 0
	}

	// Simple division - borders are handled in PaneModel.View()
	paneWidth = totalWidth / cols
	paneHeight = totalHeight / rows
	return paneWidth, paneHeight
}

// View renders a single pane with border and project name header.
func (p PaneModel) View() string {
	// Guard against invalid dimensions
	if p.width < 4 || p.height < 3 {
		return ""
	}

	// Inner content width (account for left+right border = 2 chars)
	innerWidth := p.width - 2

	// Truncate project name if too long (leave room for padding)
	displayName := p.project.DisplayName
	maxNameLen := innerWidth - 2 // padding
	if maxNameLen > 3 && len(displayName) > maxNameLen {
		displayName = displayName[:maxNameLen-3] + "..."
	}

	// Header with project name
	header := PaneHeaderStyle.
		Width(innerWidth).
		Render(displayName)

	// Content area (empty for Story 5.2)
	// Height calculation: total - header (1 line) - top/bottom border (2 lines)
	contentHeight := p.height - 3
	if contentHeight < 0 {
		contentHeight = 0
	}
	content := lipgloss.NewStyle().
		Width(innerWidth).
		Height(contentHeight).
		Render("")

	// Combine header and content
	inner := lipgloss.JoinVertical(lipgloss.Left, header, content)

	// Wrap with border
	return PaneBorderStyle.
		Width(p.width).
		Height(p.height).
		Render(inner)
}
