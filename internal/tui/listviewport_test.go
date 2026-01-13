package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// testItem implements ListItem for testing
type testItem struct {
	title string
}

func (t testItem) Render(width int, selected bool) string {
	if selected {
		return "> " + t.title
	}
	return "  " + t.title
}

func (t testItem) FilterValue() string {
	return t.title
}

func makeTestItems(n int) []testItem {
	items := make([]testItem, n)
	for i := 0; i < n; i++ {
		items[i] = testItem{title: "Item " + string(rune('A'+i%26))}
	}
	return items
}

func TestListViewport_Navigation(t *testing.T) {
	tests := []struct {
		name       string
		numItems   int
		startPos   int
		key        string
		wantCursor int
	}{
		{"j moves down", 5, 0, "j", 1},
		{"down moves down", 5, 0, "down", 1},
		{"k moves up", 5, 2, "k", 1},
		{"up moves up", 5, 2, "up", 1},
		{"j at end stays", 5, 4, "j", 4},
		{"k at start stays", 5, 0, "k", 0},
		{"G goes to end", 5, 0, "G", 4},
		{"G on empty stays", 0, 0, "G", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := makeTestItems(tt.numItems)
			lv := NewListViewport[testItem](items, 1)
			lv.SetSize(80, 20)
			lv.SetCursor(tt.startPos)

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if tt.key == "down" {
				msg = tea.KeyMsg{Type: tea.KeyDown}
			} else if tt.key == "up" {
				msg = tea.KeyMsg{Type: tea.KeyUp}
			}

			lv, _ = lv.Update(msg)

			if lv.Cursor() != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", lv.Cursor(), tt.wantCursor)
			}
		})
	}
}

func TestListViewport_DoubleG(t *testing.T) {
	items := makeTestItems(10)
	lv := NewListViewport[testItem](items, 1)
	lv.SetSize(80, 20)
	lv.SetCursor(5) // Start in middle

	// Single g should not move cursor
	gMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	lv, _ = lv.Update(gMsg)
	if lv.Cursor() != 5 {
		t.Errorf("single g moved cursor: got %d, want 5", lv.Cursor())
	}

	// Second g quickly should move to top
	lv, _ = lv.Update(gMsg)
	if lv.Cursor() != 0 {
		t.Errorf("double g didn't move to top: got %d, want 0", lv.Cursor())
	}
}

func TestListViewport_SetItems(t *testing.T) {
	tests := []struct {
		name       string
		initial    int
		cursor     int
		newCount   int
		wantCursor int
	}{
		{"cursor valid after resize", 10, 5, 10, 5},
		{"cursor clamped after shrink", 10, 8, 5, 4},
		{"cursor stays at 0 for empty", 5, 2, 0, 0},
		{"cursor max when at end", 10, 9, 5, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := makeTestItems(tt.initial)
			lv := NewListViewport[testItem](items, 1)
			lv.SetSize(80, 20)
			lv.SetCursor(tt.cursor)

			newItems := makeTestItems(tt.newCount)
			lv.SetItems(newItems)

			if lv.Cursor() != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", lv.Cursor(), tt.wantCursor)
			}
		})
	}
}

func TestListViewport_SelectedItem(t *testing.T) {
	t.Run("returns selected item", func(t *testing.T) {
		items := makeTestItems(5)
		lv := NewListViewport[testItem](items, 1)
		lv.SetSize(80, 20)
		lv.SetCursor(2)

		item, ok := lv.SelectedItem()
		if !ok {
			t.Error("expected ok=true")
		}
		if item.title != items[2].title {
			t.Errorf("got title %q, want %q", item.title, items[2].title)
		}
	})

	t.Run("empty list returns false", func(t *testing.T) {
		lv := NewListViewport[testItem]([]testItem{}, 1)
		lv.SetSize(80, 20)

		_, ok := lv.SelectedItem()
		if ok {
			t.Error("expected ok=false for empty list")
		}
	})
}

func TestListViewport_ItemCount(t *testing.T) {
	items := makeTestItems(7)
	lv := NewListViewport[testItem](items, 1)
	if lv.ItemCount() != 7 {
		t.Errorf("ItemCount() = %d, want 7", lv.ItemCount())
	}
}

func TestListViewport_SetCursor(t *testing.T) {
	items := makeTestItems(10)
	lv := NewListViewport[testItem](items, 1)
	lv.SetSize(80, 20)

	// Normal set
	lv.SetCursor(5)
	if lv.Cursor() != 5 {
		t.Errorf("cursor = %d, want 5", lv.Cursor())
	}

	// Negative clamps to 0
	lv.SetCursor(-1)
	if lv.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0 for negative", lv.Cursor())
	}

	// Over max clamps to last
	lv.SetCursor(100)
	if lv.Cursor() != 9 {
		t.Errorf("cursor = %d, want 9 for over max", lv.Cursor())
	}
}

func TestListViewport_ViewNotReady(t *testing.T) {
	items := makeTestItems(5)
	lv := NewListViewport[testItem](items, 1)
	// Don't call SetSize

	view := lv.View()
	if view != "Loading..." {
		t.Errorf("View() = %q, want 'Loading...'", view)
	}
}

func TestListViewport_ViewReady(t *testing.T) {
	items := makeTestItems(3)
	lv := NewListViewport[testItem](items, 1)
	lv.SetSize(80, 10)

	view := lv.View()
	if view == "Loading..." {
		t.Error("View() returned loading state after SetSize")
	}
	// Should contain rendered items
	if len(view) == 0 {
		t.Error("View() returned empty string")
	}
}
