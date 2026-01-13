// Package tui provides the terminal user interface components.
package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ListItem interface for renderable list items.
type ListItem interface {
	Render(width int, selected bool) string
	FilterValue() string
}

// ListViewport is a viewport-based list with explicit cursor control.
// It provides strict height control by using bubbles/viewport instead of bubbles/list.
type ListViewport[T ListItem] struct {
	viewport   viewport.Model
	items      []T
	cursor     int
	width      int
	height     int
	itemHeight int // Lines per item (typically 2)
	ready      bool

	// gg key detection (from viewer.go pattern)
	lastKeyG     bool
	lastKeyGTime time.Time
}

// NewListViewport creates a new list viewport.
// itemHeight must be >= 1 (lines per item).
func NewListViewport[T ListItem](items []T, itemHeight int) ListViewport[T] {
	if itemHeight < 1 {
		itemHeight = 1
	}
	return ListViewport[T]{
		items:      items,
		itemHeight: itemHeight,
		cursor:     0,
	}
}

// SetItems updates the item list and re-renders content.
func (m *ListViewport[T]) SetItems(items []T) {
	m.items = items
	if m.cursor >= len(items) {
		m.cursor = max(0, len(items)-1)
	}
	m.updateContent()
}

// SetSize sets the viewport dimensions.
func (m *ListViewport[T]) SetSize(width, height int) {
	m.width = width
	m.height = height

	if !m.ready {
		m.viewport = viewport.New(width, height)
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height
	}
	m.updateContent()
}

// Cursor returns current cursor position.
func (m ListViewport[T]) Cursor() int { return m.cursor }

// SelectedItem returns the currently selected item.
func (m ListViewport[T]) SelectedItem() (T, bool) {
	if m.cursor >= 0 && m.cursor < len(m.items) {
		return m.items[m.cursor], true
	}
	var zero T
	return zero, false
}

// ItemCount returns total items.
func (m ListViewport[T]) ItemCount() int { return len(m.items) }

// Items returns all items.
func (m ListViewport[T]) Items() []T { return m.items }

// Update handles keyboard navigation.
func (m ListViewport[T]) Update(msg tea.Msg) (ListViewport[T], tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()

		// gg sequence detection
		if keyStr == "g" {
			if m.lastKeyG && time.Since(m.lastKeyGTime) < 500*time.Millisecond {
				m.cursor = 0
				m.lastKeyG = false
				m.updateContent()
				m.viewport.GotoTop()
				return m, nil
			}
			m.lastKeyG = true
			m.lastKeyGTime = time.Now()
			return m, nil
		}
		m.lastKeyG = false

		switch keyStr {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case "G":
			if len(m.items) > 0 {
				m.cursor = len(m.items) - 1
				m.updateContent()
				m.viewport.GotoBottom()
			}
		case "d", "ctrl+d":
			m.viewport.HalfPageDown()
			m.syncCursorToViewport()
		case "u", "ctrl+u":
			m.viewport.HalfPageUp()
			m.syncCursorToViewport()
		case "pgdown", " ":
			m.viewport.PageDown()
			m.syncCursorToViewport()
		case "pgup":
			m.viewport.PageUp()
			m.syncCursorToViewport()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the list.
func (m ListViewport[T]) View() string {
	if !m.ready {
		return "Loading..."
	}
	return m.viewport.View()
}

// updateContent re-renders all items into the viewport.
func (m *ListViewport[T]) updateContent() {
	if !m.ready {
		return
	}
	var content strings.Builder
	for i, item := range m.items {
		rendered := item.Render(m.width, i == m.cursor)
		content.WriteString(rendered)
		if i < len(m.items)-1 {
			content.WriteString("\n")
		}
	}
	m.viewport.SetContent(content.String())
}

// ensureCursorVisible scrolls viewport to keep cursor in view.
func (m *ListViewport[T]) ensureCursorVisible() {
	m.updateContent()
	cursorLine := m.cursor * m.itemHeight
	viewTop := m.viewport.YOffset
	viewBottom := viewTop + m.viewport.Height

	if cursorLine < viewTop {
		m.viewport.SetYOffset(cursorLine)
	} else if cursorLine+m.itemHeight > viewBottom {
		m.viewport.SetYOffset(cursorLine + m.itemHeight - m.viewport.Height)
	}
}

// syncCursorToViewport updates cursor based on viewport scroll position.
func (m *ListViewport[T]) syncCursorToViewport() {
	if len(m.items) == 0 {
		return
	}
	// Set cursor to first visible item
	firstVisible := m.viewport.YOffset / m.itemHeight
	if firstVisible >= len(m.items) {
		firstVisible = len(m.items) - 1
	}
	if firstVisible < 0 {
		firstVisible = 0
	}
	m.cursor = firstVisible
	m.updateContent()
}

// SetCursor sets the cursor position directly.
func (m *ListViewport[T]) SetCursor(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.items) {
		idx = len(m.items) - 1
	}
	if idx < 0 {
		idx = 0
	}
	m.cursor = idx
	m.ensureCursorVisible()
}
