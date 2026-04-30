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
	project      types.Project
	index        int  // Original index in projects slice (for selection tracking)
	isChecked    bool // Whether checked for dashboard (multi-select)
	showCheckbox bool // Whether to show checkbox (when in selection mode)
}

// Render renders the project item for display.
func (i ProjectItem) Render(width int, selected bool) string {
	// Selection indicator and styling
	var prefixStyled string
	var titleStyle, descStyle lipgloss.Style

	if selected {
		prefixStyled = Styles.ListItem.GutterSelected.Render(GutterSelected)
		if i.isChecked {
			// Checked and cursor-selected: use checked background
			titleStyle = Styles.ListItem.TitleChecked
			descStyle = Styles.ListItem.DescChecked
		} else {
			titleStyle = Styles.ListItem.TitleSelected
			descStyle = Styles.ListItem.DescSelected
		}
	} else {
		prefixStyled = GutterNormal // No styling needed for normal gutter
		if i.isChecked {
			// Checked but not cursor-selected: use checked background
			titleStyle = Styles.ListItem.TitleChecked
			descStyle = Styles.ListItem.DescChecked
		} else {
			titleStyle = Styles.ListItem.TitleNormal
			descStyle = Styles.ListItem.DescNormal
		}
	}

	// Build checkbox prefix if in selection mode
	var checkboxPrefix string
	checkboxWidth := 0
	if i.showCheckbox {
		if i.isChecked {
			checkboxPrefix = Styles.SelectionIndicator.Render(SelectionChecked) + " "
		} else {
			checkboxPrefix = SelectionUnchecked + " "
		}
		checkboxWidth = 4 // "[x] " or "[ ] " = 4 chars
	}

	// Calculate available width using shared helper, minus checkbox width
	availWidth := listItemAvailWidth(width) - checkboxWidth

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

	// Build description line gutter - align with checkbox if present
	descGutter := GutterNormal
	if i.showCheckbox {
		descGutter = GutterNormal + "    " // Align with checkbox width
	}

	// Description line also gets gutter alignment (normal gutter for visual alignment)
	return fmt.Sprintf("%s%s%s\n%s%s", prefixStyled, checkboxPrefix, title, descGutter, desc)
}

// FilterValue returns the value used for filtering.
func (i ProjectItem) FilterValue() string { return i.project.DisplayName }

// BackToAgentSelectorMsg is emitted when the user presses esc/h in the project
// list and canGoBack is true (navigated from agent selector).
type BackToAgentSelectorMsg struct{}

// ProjectModel is the Bubbletea model for the project browser.
type ProjectModel struct {
	listViewport  ListViewport[ProjectItem]
	projects      []types.Project
	allItems      []ProjectItem // All items (unfiltered)
	width         int
	height        int
	filterInput   textinput.Model
	filtering     bool
	err           error
	ready         bool         // Set to true after first WindowSizeMsg
	selected      map[int]bool // Map of project indices to selected state
	selectionMode bool         // True when any project is selected
	canGoBack     bool         // When true, esc emits BackToAgentSelectorMsg
}

// NewProjectModel creates a new project browser model.
func NewProjectModel(projects []types.Project) ProjectModel {
	items := make([]ProjectItem, len(projects))
	for i, p := range projects {
		items[i] = ProjectItem{project: p, index: i}
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
		selected:     make(map[int]bool),
	}
}

// NewProjectModelWithError creates a project model showing an error.
func NewProjectModelWithError(err error) ProjectModel {
	return ProjectModel{err: err}
}

// NewProjectModelWithBack creates a project model that supports back navigation
// to the agent selector screen via esc/h keys.
func NewProjectModelWithBack(projects []types.Project) ProjectModel {
	m := NewProjectModel(projects)
	m.canGoBack = true
	return m
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

// DashboardSelectedMsg is sent when multiple projects are selected for dashboard.
type DashboardSelectedMsg struct {
	Projects []types.Project
}

// ShowToastMsg requests AppModel to show a toast message.
type ShowToastMsg struct {
	Message string
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
			// If in selection mode with selected projects, open dashboard
			if m.selectionMode && m.SelectedCount() > 0 {
				projects := m.SelectedProjects()
				return m, func() tea.Msg {
					return DashboardSelectedMsg{Projects: projects}
				}
			}
			// Otherwise, open single project (existing behavior)
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

		case " ":
			// Toggle selection for multi-select (Story 5.1)
			if item, ok := m.listViewport.SelectedItem(); ok {
				if !m.ToggleSelection(item.index) {
					// Selection limit reached
					return m, func() tea.Msg {
						return ShowToastMsg{Message: "Maximum 9 projects selected"}
					}
				}
				m.updateItemsWithSelection()
			}
			return m, nil

		case "esc", "h":
			// If in selection mode, clear selections and stay in project list
			if m.selectionMode {
				m.ClearSelections()
				m.updateItemsWithSelection()
				return m, nil
			}
			// If canGoBack (navigated from agent selector), go back
			if m.canGoBack {
				return m, func() tea.Msg {
					return BackToAgentSelectorMsg{}
				}
			}
			// Otherwise, do nothing (no quit on esc in project list)
			return m, nil
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

	// Header with project count and selection count
	projectCount := m.listViewport.ItemCount()
	headerText := fmt.Sprintf("Claude Code Projects %s", ListStyles.Counter.Render(fmt.Sprintf("(%d)", projectCount)))
	if m.selectionMode {
		headerText = fmt.Sprintf("Claude Code Projects %s - %s",
			ListStyles.Counter.Render(fmt.Sprintf("(%d)", projectCount)),
			Styles.SelectionIndicator.Render(fmt.Sprintf("%d selected", m.SelectedCount())))
	}
	header := Styles.Title.Render(headerText)

	// Footer - different help text in selection mode
	var help string
	if m.selectionMode {
		help = "j/k:nav • space:toggle • enter:open dashboard • esc:clear"
	} else {
		help = "j/k:nav • enter/l:select • space:multi-select • /:filter • g/G:top/bottom • q:quit"
	}
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
	for i, p := range m.projects {
		if strings.Contains(strings.ToLower(p.DisplayName), filter) ||
			strings.Contains(strings.ToLower(p.DecodedPath), filter) {
			item := ProjectItem{
				project:      p,
				index:        i,
				isChecked:    m.selected[i],
				showCheckbox: m.selectionMode,
			}
			items = append(items, item)
		}
	}
	m.listViewport.SetItems(items)
}

// resetFilter resets the filter and shows all projects.
func (m *ProjectModel) resetFilter() {
	// Sync selection state before resetting to ensure checkboxes display correctly
	m.updateItemsWithSelection()
}

// SelectedProject returns the currently selected project.
func (m ProjectModel) SelectedProject() (types.Project, bool) {
	if item, ok := m.listViewport.SelectedItem(); ok {
		return item.project, true
	}
	return types.Project{}, false
}

// SelectedCount returns the number of selected projects.
func (m ProjectModel) SelectedCount() int {
	count := 0
	for _, isSelected := range m.selected {
		if isSelected {
			count++
		}
	}
	return count
}

// SelectedProjects returns the list of selected projects in index order.
func (m ProjectModel) SelectedProjects() []types.Project {
	result := make([]types.Project, 0, len(m.selected))
	for i, p := range m.projects {
		if m.selected[i] {
			result = append(result, p)
		}
	}
	return result
}

// ToggleSelection toggles the selection state for a project index.
// Returns false if the selection limit is reached and cannot add more.
func (m *ProjectModel) ToggleSelection(index int) bool {
	if index < 0 || index >= len(m.projects) {
		return false
	}

	if m.selected[index] {
		// Deselecting - always allowed
		delete(m.selected, index)
		m.selectionMode = len(m.selected) > 0
		return true
	}

	// Selecting - check limit
	if m.SelectedCount() >= MaxSelectedProjects {
		return false
	}

	m.selected[index] = true
	m.selectionMode = true
	return true
}

// ClearSelections clears all selected projects.
func (m *ProjectModel) ClearSelections() {
	m.selected = make(map[int]bool)
	m.selectionMode = false
}

// updateItemsWithSelection syncs selection state to items and updates the viewport.
func (m *ProjectModel) updateItemsWithSelection() {
	for i := range m.allItems {
		m.allItems[i].isChecked = m.selected[i]
		m.allItems[i].showCheckbox = m.selectionMode
	}
	m.listViewport.SetItems(m.allItems)
}
