package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/agent"
)

// AgentSelectedMsg is emitted when the user selects an agent provider.
type AgentSelectedMsg struct {
	Provider agent.AgentProvider
	Projects []agent.Project
}

// agentInfo holds pre-computed display data for a visible provider row.
type agentInfo struct {
	provider   agent.AgentProvider
	projectSet []agent.Project
	projects   int
	sessions   int
	lastActive string
}

// AgentSelectorModel lets the user choose which agent backend to browse.
type AgentSelectorModel struct {
	providers []agent.AgentProvider
	visible   []agentInfo
	cursor    int
	width     int
	height    int
	loading   bool

	// style caches
	titleStyle  lipgloss.Style
	headerStyle lipgloss.Style
	rowStyle    lipgloss.Style
	selectedRow lipgloss.Style
	borderStyle lipgloss.Style
	helpStyle   lipgloss.Style
}

// NewAgentSelectorModel creates the agent selector screen.
// If only one provider is available (has projects), it immediately
// emits an AgentSelectedMsg via the returned Init command (single-agent shortcut).
func NewAgentSelectorModel(providers []agent.AgentProvider) AgentSelectorModel {
	m := newAgentSelectorModel(providers)
	m.refreshVisible()
	return m
}

// NewAgentSelectorModelLoading creates the selector without scanning providers.
// The app uses this so expensive discovery can run after the first TUI frame.
func NewAgentSelectorModelLoading(providers []agent.AgentProvider) AgentSelectorModel {
	m := newAgentSelectorModel(providers)
	m.loading = true
	return m
}

func newAgentSelectorModel(providers []agent.AgentProvider) AgentSelectorModel {
	m := AgentSelectorModel{
		providers: providers,
		titleStyle: lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true),
		headerStyle: lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(dimColor),
		rowStyle: lipgloss.NewStyle().
			Foreground(textColor),
		selectedRow: lipgloss.NewStyle().
			Foreground(whiteColor).
			Background(primaryColor).
			Bold(true),
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			Padding(0, 1),
		helpStyle: lipgloss.NewStyle().
			Foreground(dimColor),
	}
	return m
}

// refreshVisible rebuilds the visible provider list, hiding those
// that are unavailable or have no projects.
func (m *AgentSelectorModel) refreshVisible() {
	m.visible = nil
	for _, p := range m.providers {
		if !p.IsAvailable() {
			continue
		}
		projects, err := p.DiscoverProjects()
		if err != nil || len(projects) == 0 {
			continue
		}
		info := agentInfo{
			provider:   p,
			projectSet: append([]agent.Project(nil), projects...),
			projects:   len(projects),
			lastActive: "-",
		}

		// Use project-level session counts only. Session discovery can parse a
		// large number of logs for some providers, so keep it lazy until a
		// project is opened.
		for _, proj := range projects {
			info.sessions += proj.SessionCount
		}

		// Hide agents with zero sessions per spec requirement.
		if info.sessions == 0 {
			continue
		}
		m.visible = append(m.visible, info)
	}
	if m.cursor >= len(m.visible) {
		m.cursor = max(0, len(m.visible)-1)
	}
}

// VisibleProviders returns the currently visible provider infos (for testing).
func (m AgentSelectorModel) VisibleProviders() []agentInfo {
	return m.visible
}

// Cursor returns the current cursor position (for testing).
func (m AgentSelectorModel) Cursor() int {
	return m.cursor
}

// SingleProvider returns the sole visible provider, or nil if 0 or 2+.
func (m AgentSelectorModel) SingleProvider() agent.AgentProvider {
	if len(m.visible) == 1 {
		return m.visible[0].provider
	}
	return nil
}

// Init implements tea.Model.
func (m AgentSelectorModel) Init() tea.Cmd {
	if m.loading {
		return nil
	}
	if len(m.visible) == 1 {
		info := m.visible[0]
		return func() tea.Msg {
			return AgentSelectedMsg{Provider: info.provider, Projects: info.projectSet}
		}
	}
	return tea.WindowSize()
}

// Update implements tea.Model.
func (m AgentSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if m.cursor < len(m.visible) {
				info := m.visible[m.cursor]
				return m, func() tea.Msg {
					return AgentSelectedMsg{Provider: info.provider, Projects: info.projectSet}
				}
			}
			return m, nil
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m AgentSelectorModel) View() string {
	if m.loading {
		return m.borderStyle.Render(m.titleStyle.Render("Loading agents") + "\n\n" +
			m.helpStyle.Render("Scanning agent data directories..."))
	}

	if len(m.visible) == 0 {
		return m.borderStyle.Render(m.titleStyle.Render("No agents found") + "\n\n" +
			m.helpStyle.Render("No agent data directories detected. Press q to quit."))
	}

	var b strings.Builder

	// Title
	b.WriteString(m.titleStyle.Render("Select Agent"))
	b.WriteString("\n\n")

	// Table header
	header := m.formatRow("Agent", "Projects", "Sessions", "Last Active", "Status", 0)
	b.WriteString(m.headerStyle.Render(header))
	b.WriteString("\n")

	// Rows
	for i, info := range m.visible {
		row := m.formatRow(
			fmt.Sprintf("%s %s", info.provider.Badge(), info.provider.DisplayName()),
			fmt.Sprintf("%d", info.projects),
			fmt.Sprintf("%d", info.sessions),
			info.lastActive,
			statusText(info),
			i,
		)
		if i == m.cursor {
			b.WriteString(m.selectedRow.Render(row))
		} else {
			b.WriteString(m.rowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.helpStyle.Render("j/k: navigate  enter: select  q: quit"))

	return m.borderStyle.Render(b.String())
}

// formatRow builds a fixed-width table row.
func (m AgentSelectorModel) formatRow(agentName, projects, sessions, lastActive, status string, _ int) string {
	const (
		agentColW  = 20
		projColW   = 10
		sessColW   = 10
		activeColW = 14
		statusColW = 10
	)
	pad := func(s string, w int) string {
		if VisualWidth(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-VisualWidth(s))
	}

	return pad(agentName, agentColW) +
		pad(projects, projColW) +
		pad(sessions, sessColW) +
		pad(lastActive, activeColW) +
		status
}

// statusText returns a short status string for a provider row.
func statusText(info agentInfo) string {
	if info.sessions > 0 {
		return "ready"
	}
	return "no sessions"
}
