# Story 5.1: Multi-Project Selection

Status: done

## Story

As a **developer monitoring multiple projects**,
I want **to select multiple projects from the list**,
So that **I can open them in a dashboard view**.

## Acceptance Criteria

### AC 5.1.1: Toggle selection with Space key
- **Given** I am in the project list view
- **When** I press Space on a project
- **Then** the project is marked as selected with visual indicator
- **And** I can continue selecting more projects

### AC 5.1.2: Deselect with Space key
- **Given** I have selected projects
- **When** I press Space on a selected project
- **Then** the selection is removed

### AC 5.1.3: Maximum 9 projects limit
- **Given** I have 9 projects selected
- **When** I try to select another
- **Then** selection is prevented (max 9)
- **And** a toast message indicates the limit

### AC 5.1.4: Open dashboard with Enter
- **Given** I have 1+ projects selected
- **When** I press Enter
- **Then** dashboard view opens with selected projects
- **And** selection state is passed to DashboardModel

### AC 5.1.5: Clear selections with Escape
- **Given** I am in selection mode with selected projects
- **When** I press Escape
- **Then** all selections are cleared
- **And** the view returns to normal browsing mode

### AC 5.1.6: Visual indicator for selected state
- **Given** projects are selected
- **When** viewing the project list
- **Then** selected projects show a distinct visual indicator (e.g., `[x]` prefix or highlighted background)
- **And** selection count shown in header/footer

## Tasks / Subtasks

- [x] Task 1: Add selection state to ProjectModel (AC: 5.1.1, 5.1.2)
  - [x] 1.1: Add `selected map[int]bool` field to ProjectModel struct
  - [x] 1.2: Add `selectionMode bool` field (true when any project selected)
  - [x] 1.3: Add `SelectedCount() int` method
  - [x] 1.4: Add `SelectedProjects() []types.Project` method
  - [x] 1.5: Add `ToggleSelection(index int) bool` method - returns false if limit reached
  - [x] 1.6: Add `ClearSelections()` method
  - [x] 1.7: Initialize `selected` map in `NewProjectModel()`

- [x] Task 2: Add selection constants to styles.go (AC: 5.1.3, 5.1.6)
  - [x] 2.1: Add `MaxSelectedProjects = 9` constant
  - [x] 2.2: Add `SelectionChecked = "[x]"` constant
  - [x] 2.3: Add `SelectionUnchecked = "[ ]"` constant
  - [x] 2.4: Add `SelectionStyle` for checked items (background tint)

- [x] Task 3: Update ProjectItem for selection indicator (AC: 5.1.6)
  - [x] 3.1: Add `isChecked bool` field to ProjectItem struct
  - [x] 3.2: Modify `Render()` to prepend selection indicator when in selection mode
  - [x] 3.3: Apply distinct styling for checked items (use SelectionStyle background)
  - [x] 3.4: Create helper `updateItemsWithSelection()` to sync selection state to items

- [x] Task 4: Handle Space key for toggle selection (AC: 5.1.1, 5.1.2, 5.1.3)
  - [x] 4.1: Add `" "` (space) case in ProjectModel.Update()
  - [x] 4.2: Get current selected index via `m.listViewport.SelectedIndex()`
  - [x] 4.3: Call `ToggleSelection()` - if returns false, emit toast message
  - [x] 4.4: Update `selectionMode` based on `SelectedCount() > 0`
  - [x] 4.5: Call `updateItemsWithSelection()` and `m.listViewport.SetItems()`
  - [x] 4.6: Return `toastMsg` command if selection limit reached

- [x] Task 5: Handle Enter key with selections (AC: 5.1.4)
  - [x] 5.1: Modify Enter/l handling to check `m.selectionMode`
  - [x] 5.2: If `selectionMode && SelectedCount() > 0`, emit `DashboardSelectedMsg`
  - [x] 5.3: If no selections, keep existing behavior (emit `ProjectSelectedMsg`)
  - [x] 5.4: Define `DashboardSelectedMsg` struct with `Projects []types.Project`

- [x] Task 6: Handle Escape key to clear selections (AC: 5.1.5)
  - [x] 6.1: Add `"esc"` case in ProjectModel.Update() (non-filtering mode)
  - [x] 6.2: If `m.selectionMode`, call `ClearSelections()` and stay in project list
  - [x] 6.3: If no selections, do nothing (or existing quit behavior if any)
  - [x] 6.4: Call `updateItemsWithSelection()` and `m.listViewport.SetItems()`

- [x] Task 7: Update footer/help text for selection mode (AC: 5.1.6)
  - [x] 7.1: In View(), check `m.selectionMode` to show different help text
  - [x] 7.2: Normal mode: `"j/k:nav • enter/l:select • space:multi-select • /:filter • g/G:top/bottom • q:quit"`
  - [x] 7.3: Selection mode: `"j/k:nav • space:toggle • enter:open dashboard • esc:clear • N selected"`
  - [x] 7.4: Show selection count in header: `"Claude Code Projects (N) - M selected"`

- [x] Task 8: Add AppModel handler for DashboardSelectedMsg
  - [x] 8.1: Add `viewDashboard` to viewState enum in app.go
  - [x] 8.2: Add `selectedProjects []types.Project` field to AppModel
  - [x] 8.3: Handle `DashboardSelectedMsg` in AppModel.Update()
  - [x] 8.4: Store selected projects and transition state to `viewDashboard`
  - [x] 8.5: Add placeholder View() case for viewDashboard (displays "Dashboard - N projects" until Story 5.2)

- [x] Task 9: Add unit tests
  - [x] 9.1: Test `ToggleSelection()` adds/removes correctly
  - [x] 9.2: Test `SelectedCount()` returns accurate count
  - [x] 9.3: Test `SelectedProjects()` returns correct projects in order
  - [x] 9.4: Test max 9 limit enforcement in `ToggleSelection()`
  - [x] 9.5: Test `ClearSelections()` resets state and selectionMode
  - [x] 9.6: Test Space key handler toggles selection state
  - [x] 9.7: Test Enter key with selections emits `DashboardSelectedMsg`
  - [x] 9.8: Test Enter key without selections emits `ProjectSelectedMsg`
  - [x] 9.9: Test Escape key clears selections when in selectionMode
  - [x] 9.10: Test Escape key does nothing when not in selectionMode

- [x] Task 10: Run build, lint, and test validation
  - [x] 10.1: Run `make build` - verify binary builds successfully
  - [x] 10.2: Run `make lint` - no errors
  - [x] 10.3: Run `make test` - all tests pass (coverage 41.1% for tui package)

- [ ] Task 11: Manual testing (Requires user verification)
  - [ ] 11.1: Open project list, press Space on a project - verify `[x]` indicator appears
  - [ ] 11.2: Press Space again - verify selection removed, shows `[ ]`
  - [ ] 11.3: Select 9 projects - verify 10th selection is blocked with toast
  - [ ] 11.4: Press Enter with selections - verify "Dashboard - N projects" placeholder appears
  - [ ] 11.5: Press Escape with selections - verify all cleared, indicators removed
  - [ ] 11.6: Verify help text changes between normal and selection modes
  - [ ] 11.7: Verify header shows "N selected" when in selection mode

## Dev Notes

### Current Implementation Location

- `internal/tui/project.go` - ProjectModel, ProjectItem
- `internal/tui/app.go` - AppModel, view state management
- `internal/tui/styles.go` - Style definitions

### Key Data Structures

**ProjectModel (current):**
```go
type ProjectModel struct {
    listViewport ListViewport[ProjectItem]
    projects     []types.Project
    allItems     []ProjectItem
    width        int
    height       int
    filterInput  textinput.Model
    filtering    bool
    err          error
    ready        bool
}
```

**ProjectModel (with selection - proposed):**
```go
type ProjectModel struct {
    listViewport  ListViewport[ProjectItem]
    projects      []types.Project
    allItems      []ProjectItem
    width         int
    height        int
    filterInput   textinput.Model
    filtering     bool
    err           error
    ready         bool
    // Phase 3 additions
    selected      map[int]bool  // Map of project indices to selected state
    selectionMode bool          // True when any project is selected
}
```

**ProjectItem (with selection - proposed):**
```go
type ProjectItem struct {
    project   types.Project
    isChecked bool  // Whether checked for dashboard (selection state)
}
```

### Selection Indicator Design

Per FR-017 (no emoji), use text-based indicators:
```go
const (
    SelectionChecked   = "[x]"
    SelectionUnchecked = "[ ]"
)
```

Selection indicators appear as prefix in list item:
- Normal mode (no selections): No prefix
- Selection mode: `[x] Project Name` or `[ ] Project Name`

### Message Types (New)

```go
// DashboardSelectedMsg is sent when multiple projects are selected for dashboard
type DashboardSelectedMsg struct {
    Projects []types.Project
}

// Note: AppModel already has toast system from Story 4.4 (ToastDuration in styles.go)
// Use existing toast infrastructure via toastMsg type
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/project.go` | Add selection state, toggle methods, key handlers, item update helper |
| `internal/tui/project_test.go` | Add tests for selection functionality (new file) |
| `internal/tui/app.go` | Add viewDashboard state, handle DashboardSelectedMsg, store selectedProjects |
| `internal/tui/styles.go` | Add MaxSelectedProjects, SelectionChecked/SelectionUnchecked constants, SelectionStyle |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **No emoji** | Use text icons `[x]` and `[ ]` per FR-017 |
| **TEA pattern** | All state changes via Update() |
| **Max 9 projects** | Hard limit per FR-501 (constant in styles.go) |
| **ListViewport** | Use existing ListViewport component (not bubbles/list) |
| **Toast system** | Reuse existing AppModel toast infrastructure from Story 4.4 |

### ProjectItem.Render Modification Strategy

The current `Render(width int, selected bool)` signature uses `selected` for cursor highlighting.
For multi-select, we need both cursor selection AND checkbox selection.

**Chosen Approach:** Store checkbox state in ProjectItem struct
```go
type ProjectItem struct {
    project    types.Project
    isChecked  bool  // Whether checked for dashboard
}

func (i ProjectItem) Render(width int, cursorSelected bool) string {
    // cursorSelected = is this the highlighted row
    // i.isChecked = is this project marked for dashboard
    ...
}
```

Use helper function to sync selection state to items:
```go
func (m *ProjectModel) updateItemsWithSelection() {
    for i := range m.allItems {
        m.allItems[i].isChecked = m.selected[i]
    }
}
```

### ListViewport Integration

`ListViewport[T]` expects items to implement:
```go
type ListItem interface {
    Render(width int, selected bool) string
    FilterValue() string
}
```

Key methods to use:
- `m.listViewport.SelectedIndex()` - get current cursor position
- `m.listViewport.SetItems(items)` - update items after selection changes

### Project Context Rules (from project-context.md)

- **NO EMOJI IN UI** - Text icons only (`[U]`, `[A]`, `[T]`, `[>]`, and now `[x]`, `[ ]`)
- **TEA pattern** - All state changes via `Update()`
- **Use Makefile** - `make build`, `make test`
- **Import order** - stdlib → external → internal

### Previous Story Learnings (Epic 4)

From Epic 4 retrospective:
1. Binary search for position lookup (Story 4.6) - demonstrates good O(log n) patterns
2. Toast messages work via AppModel with expiry time (Story 4.4) - **reuse this infrastructure**
3. Mode toggles (rawMode, inputMode) use simple bool/enum state

### Architecture Reference

From architecture-phase3.md:
- Dashboard uses slice of PaneModels (dynamic 1-9 panes)
- Navigation context via enum in AppModel
- New `viewDashboard` state will be added to viewState enum

### References

- [Source: epics-phase3.md#Story-5.1] - Acceptance criteria
- [Source: prd-phase3.md#FR-501] - Multi-project selection requirements
- [Source: architecture-phase3.md#Core-Architectural-Decisions] - Decision #1 (Dashboard Pane Architecture)
- [Source: internal/tui/project.go] - Current ProjectModel implementation
- [Source: internal/tui/app.go] - Current AppModel and view state management
- [Source: internal/tui/styles.go] - Style definitions, ToastDuration constant

## Implementation Checklist

Before marking story complete, verify:

**State Management:**
- [x] `selected map[int]bool` field added to ProjectModel
- [x] `selected` map initialized in `NewProjectModel()`
- [x] `ToggleSelection()` correctly adds/removes indices and respects limit
- [x] `ClearSelections()` resets map and selectionMode
- [x] `SelectedProjects()` returns correct projects in index order

**UI Updates:**
- [x] Selection indicators `[x]`/`[ ]` appear when in selection mode
- [x] Selected items have distinct visual styling (CheckedBg background)
- [x] Selection count displayed in header
- [x] Help text updates for selection mode

**Key Handlers:**
- [x] Space toggles selection
- [x] Enter with selections emits DashboardSelectedMsg
- [x] Enter without selections emits ProjectSelectedMsg (existing behavior)
- [x] Escape clears selections (doesn't quit if selections exist)

**Limits:**
- [x] Max 9 selection limit enforced
- [x] Toast message shown when limit reached (uses existing toast system)

**Testing:**
- [x] Unit tests for all selection methods
- [x] Unit tests for key handlers
- [x] Unit test for filter with selections preservation
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes

**Manual Verification (Requires User):**
- [ ] Selection toggle works visually
- [ ] Selection limit prevents 10th selection with toast
- [ ] Enter with selections shows dashboard placeholder
- [ ] Escape clears all selections
- [ ] Help text reflects current mode

## Dev Agent Record

### Agent Model Used

claude-opus-4-5-20251101

### Debug Log References

N/A

### Completion Notes List

1. Implemented multi-project selection with Space key toggle
2. Added `selected map[int]bool` and `selectionMode bool` to ProjectModel
3. Added `index` field to ProjectItem for tracking original position during filtering
4. Created constants: `MaxSelectedProjects=9`, `SelectionChecked="[x]"`, `SelectionUnchecked="[ ]"`
5. Added `SelectionIndicator` style with green foreground
6. Added `CheckedBg` adaptive color to theme (subtle green)
7. Implemented `ToggleSelection()`, `SelectedCount()`, `SelectedProjects()`, `ClearSelections()`
8. Added `updateItemsWithSelection()` helper to sync selection state to items
9. Modified `Render()` to show checkbox prefix when in selection mode
10. Added Space key handler with limit enforcement (emits ShowToastMsg on limit)
11. Modified Enter key to emit `DashboardSelectedMsg` when selections exist
12. Added Escape key handler to clear selections when in selection mode
13. Updated help text to show different shortcuts based on mode
14. Header shows "N selected" when in selection mode
15. Added `viewDashboard` state and placeholder view in AppModel
16. Dashboard placeholder shows list of selected projects with esc/q navigation
17. All 10 unit tests pass covering selection methods and key handlers

### File List

- `internal/tui/project.go` - Added selection state, methods, key handlers, ProjectItem fields, fixed resetFilter to sync selection state, added checked item styling
- `internal/tui/project_test.go` - NEW: 11 unit tests for selection functionality (added filter preservation test)
- `internal/tui/app.go` - Added viewDashboard state, DashboardSelectedMsg handler, placeholder view
- `internal/tui/styles.go` - Added MaxSelectedProjects, Selection constants, CheckedBg, SelectionIndicator style, TitleChecked/DescChecked styles
