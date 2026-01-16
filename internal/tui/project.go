// Package tui provides the terminal user interface components.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/types"
)

// ProjectItem implements ListItem for the project list.
type ProjectItem struct {
	project types.Project
}

// Render renders the project item for display.
func (i ProjectItem) Render(width int, selected bool) string {
	// Selection indicator and styling
	var prefixStyled string
	var titleStyle, descStyle lipgloss.Style

	if selected {
		prefixStyled = Styles.ListItem.GutterSelected.Render(GutterSelected)
		titleStyle = Styles.ListItem.TitleSelected
		descStyle = Styles.ListItem.DescSelected
	} else {
		prefixStyled = GutterNormal // No styling needed for normal gutter
		titleStyle = Styles.ListItem.TitleNormal
		descStyle = Styles.ListItem.DescNormal
	}

	// Calculate available width using shared helper
	availWidth := listItemAvailWidth(width)

	// Build count string with singular/plural handling
	countStr := fmt.Sprintf("%d conversations", i.project.ConversationCount)
	if i.project.ConversationCount == 1 {
		countStr = "1 conversation"
	}

	// Title includes project name and count
	titleContent := fmt.Sprintf("%s (%s)", i.project.DisplayName, countStr)
	title := titleStyle.Render(titleContent)

	// Description shows just the path (full width available)
	path := TruncateFromLeftToWidth(i.project.DecodedPath, availWidth)
	descContent := path

	// Pad description to fill width for consistent selection background
	paddedDesc := PadToWidth(descContent, availWidth)
	desc := descStyle.Render(paddedDesc)

	// Description line also gets gutter alignment (normal gutter for visual alignment)
	return fmt.Sprintf("%s%s\n%s%s", prefixStyled, title, GutterNormal, desc)
}

// FilterValue returns the value used for filtering.
func (i ProjectItem) FilterValue() string { return i.project.DisplayName }

// ProjectModel is the Bubbletea model for the project browser.
type ProjectModel struct {
	listViewport ListViewport[ProjectItem]
	projects     []types.Project
	allItems     []ProjectItem // All items (unfiltered)
	width        int
	height       int
	filterInput  textinput.Model
	filtering    bool
	err          error
	ready        bool // Set to true after first WindowSizeMsg
}

// NewProjectModel creates a new project browser model.
func NewProjectModel(projects []types.Project) ProjectModel {
	items := make([]ProjectItem, len(projects))
	for i, p := range projects {
		items[i] = ProjectItem{project: p}
	}

	// Create viewport-based list with 2 lines per item
	listViewport := NewListViewport[ProjectItem](items, 2)

	ti := textinput.New()
	ti.Placeholder = "Filter projects..."
	ti.CharLimit = 100

	return ProjectModel{
		listViewport: listViewport,
		projects:     projects,
		allItems:     items,
		filterInput:  ti,
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

// SetSize sets the list size and marks the model as ready.
func (m *ProjectModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Calculate actual list height: total - header - footer - border (2 lines for border top/bottom)
	listHeight := height - 4
	if m.filtering {
		listHeight = height - 6 // Account for filter input too
	}
	if listHeight < 4 {
		listHeight = 4
	}
	// Width for list is total width minus 4 (2 for outer margins, 2 for border chars)
	listWidth := width - 4
	if listWidth < 10 {
		listWidth = 10
	}
	m.listViewport.SetSize(listWidth, listHeight)
	m.ready = true
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
				m.applyFilter(m.filterInput.Value())
				// Recalculate list height without filter input
				m.SetSize(m.width, m.height)
				return m, nil
			case "esc":
				m.filtering = false
				m.filterInput.SetValue("")
				m.resetFilter()
				// Recalculate list height without filter input
				m.SetSize(m.width, m.height)
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

		case "enter", "l":
			if item, ok := m.listViewport.SelectedItem(); ok {
				return m, func() tea.Msg {
					return ProjectSelectedMsg{Project: item.project}
				}
			}

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			// Recalculate list height with filter input
			m.SetSize(m.width, m.height)
			return m, textinput.Blink
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	}

	// Delegate navigation to ListViewport
	var cmd tea.Cmd
	m.listViewport, cmd = m.listViewport.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m ProjectModel) View() string {
	if m.err != nil {
		return m.renderError()
	}

	if !m.ready {
		return "Loading..."
	}

	// Header with project count
	projectCount := m.listViewport.ItemCount()
	headerText := fmt.Sprintf("Claude Code Projects %s", ListStyles.Counter.Render(fmt.Sprintf("(%d)", projectCount)))
	header := Styles.Title.Render(headerText)

	// Footer
	help := "j/k:nav • enter/l:select • /:filter • g/G:top/bottom • q:quit"
	footer := Styles.HelpText.Render(help)

	// Viewport already respects height strictly
	listView := m.listViewport.View()

	// Add manual border
	boxed := addBorder(listView, m.width-2)

	// Middle content (list + optional filter input)
	if m.filtering {
		filterInput := Styles.SearchInput.Render(m.filterInput.View())
		return fmt.Sprintf("%s\n%s\n%s\n%s", header, boxed, filterInput, footer)
	}

	return fmt.Sprintf("%s\n%s\n%s", header, boxed, footer)
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
	items := make([]ProjectItem, 0)
	for _, p := range m.projects {
		if strings.Contains(strings.ToLower(p.DisplayName), filter) ||
			strings.Contains(strings.ToLower(p.DecodedPath), filter) {
			items = append(items, ProjectItem{project: p})
		}
	}
	m.listViewport.SetItems(items)
}

// resetFilter resets the filter and shows all projects.
func (m *ProjectModel) resetFilter() {
	m.listViewport.SetItems(m.allItems)
}

// SelectedProject returns the currently selected project.
func (m ProjectModel) SelectedProject() (types.Project, bool) {
	if item, ok := m.listViewport.SelectedItem(); ok {
		return item.project, true
	}
	return types.Project{}, false
}
