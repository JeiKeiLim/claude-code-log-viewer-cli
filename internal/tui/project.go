// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ProjectItem implements list.Item for the project list.
type ProjectItem struct {
	project types.Project
}

func (i ProjectItem) Title() string       { return i.project.DisplayName }
func (i ProjectItem) Description() string { return i.project.DecodedPath }
func (i ProjectItem) FilterValue() string { return i.project.DisplayName }

// ProjectItemDelegate is a custom delegate for rendering project items.
type ProjectItemDelegate struct{}

func (d ProjectItemDelegate) Height() int                             { return 2 }
func (d ProjectItemDelegate) Spacing() int                            { return 0 }
func (d ProjectItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d ProjectItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(ProjectItem)
	if !ok {
		return
	}

	var style lipgloss.Style
	if index == m.Index() {
		style = Styles.Selected
	} else {
		style = Styles.Normal
	}

	title := style.Render(i.project.DisplayName)
	desc := Styles.Muted.Render(i.project.DecodedPath)

	fmt.Fprintf(w, "  %s\n  %s\n", title, desc)
}

// ProjectModel is the Bubbletea model for the project browser.
type ProjectModel struct {
	list        list.Model
	projects    []types.Project
	width       int
	height      int
	filterInput textinput.Model
	filtering   bool
	err         error
}

// NewProjectModel creates a new project browser model.
func NewProjectModel(projects []types.Project) ProjectModel {
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = ProjectItem{project: p}
	}

	delegate := ProjectItemDelegate{}
	l := list.New(items, delegate, 0, 0)
	l.Title = "Claude Code Projects"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = Styles.Title

	ti := textinput.New()
	ti.Placeholder = "Filter projects..."
	ti.CharLimit = 100

	return ProjectModel{
		list:        l,
		projects:    projects,
		filterInput: ti,
	}
}

// NewProjectModelWithError creates a project model showing an error.
func NewProjectModelWithError(err error) ProjectModel {
	return ProjectModel{err: err}
}

// Init implements tea.Model.
func (m ProjectModel) Init() tea.Cmd {
	return nil
}

// ProjectSelectedMsg is sent when a project is selected.
type ProjectSelectedMsg struct {
	Project types.Project
}

// Update implements tea.Model.
func (m ProjectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		// In error state, only allow quit
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "q" || keyMsg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter":
				m.filtering = false
				m.list.SetFilteringEnabled(false)
				m.applyFilter(m.filterInput.Value())
				return m, nil
			case "esc":
				m.filtering = false
				m.filterInput.SetValue("")
				m.list.SetFilteringEnabled(false)
				m.resetFilter()
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter(m.filterInput.Value())
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j", "down":
			m.list.CursorDown()

		case "k", "up":
			m.list.CursorUp()

		case "enter", "l":
			if item, ok := m.list.SelectedItem().(ProjectItem); ok {
				return m, func() tea.Msg {
					return ProjectSelectedMsg{Project: item.project}
				}
			}

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "g":
			m.list.CursorUp()
			for m.list.Index() > 0 {
				m.list.CursorUp()
			}

		case "G":
			m.list.CursorDown()
			for m.list.Index() < len(m.list.Items())-1 {
				m.list.CursorDown()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m ProjectModel) View() string {
	if m.err != nil {
		return m.renderError()
	}

	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(Styles.SearchInput.Render(m.filterInput.View()))
		b.WriteString("\n")
	}

	// Help text
	help := "j/k:nav • enter/l:select • /:filter • g/G:top/bottom • q:quit"
	b.WriteString(Styles.HelpText.Render(help))

	return b.String()
}

// renderError renders the error state.
func (m ProjectModel) renderError() string {
	title := Styles.Title.Render("Claude Code Log Viewer")
	errMsg := Styles.Muted.Render(m.err.Error())

	help := "\nClaude Code stores conversation logs in ~/.claude/projects/\n\n"
	help += "To use this tool:\n"
	help += "  • Run 'cclv <file.jsonl>' to view a specific log file\n"
	help += "  • Or 'cat file.jsonl | cclv' to pipe content\n"
	help += "\nPress 'q' to quit"

	return fmt.Sprintf("%s\n\n%s\n%s", title, errMsg, Styles.HelpText.Render(help))
}

// applyFilter filters the project list.
func (m *ProjectModel) applyFilter(filter string) {
	if filter == "" {
		m.resetFilter()
		return
	}

	filter = strings.ToLower(filter)
	items := make([]list.Item, 0)
	for _, p := range m.projects {
		if strings.Contains(strings.ToLower(p.DisplayName), filter) ||
			strings.Contains(strings.ToLower(p.DecodedPath), filter) {
			items = append(items, ProjectItem{project: p})
		}
	}
	m.list.SetItems(items)
}

// resetFilter resets the filter and shows all projects.
func (m *ProjectModel) resetFilter() {
	items := make([]list.Item, len(m.projects))
	for i, p := range m.projects {
		items[i] = ProjectItem{project: p}
	}
	m.list.SetItems(items)
}

// SelectedProject returns the currently selected project.
func (m ProjectModel) SelectedProject() (types.Project, bool) {
	if item, ok := m.list.SelectedItem().(ProjectItem); ok {
		return item.project, true
	}
	return types.Project{}, false
}
