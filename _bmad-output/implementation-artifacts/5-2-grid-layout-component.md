# Story 5.2: Grid Layout Component

Status: done

## Story

As a **developer viewing the dashboard**,
I want **projects displayed in an auto-sizing grid**,
So that **I can see all monitored projects at once**.

## Acceptance Criteria

### AC 5.2.1: Single project full screen
- **Given** 1 project selected
- **When** dashboard opens
- **Then** grid displays as 1x1 (full screen pane)

### AC 5.2.2: Two to three projects horizontal
- **Given** 2-3 projects selected
- **When** dashboard opens
- **Then** grid displays as 1 row with 2-3 columns

### AC 5.2.3: Four projects 2x2
- **Given** 4 projects selected
- **When** dashboard opens
- **Then** grid displays as 2x2

### AC 5.2.4: Five to six projects 2x3
- **Given** 5-6 projects selected
- **When** dashboard opens
- **Then** grid displays as 2x3

### AC 5.2.5: Seven to nine projects 3x3
- **Given** 7-9 projects selected
- **When** dashboard opens
- **Then** grid displays as 3x3

### AC 5.2.6: Terminal resize reflows grid
- **Given** terminal is resized
- **When** dashboard is visible
- **Then** panes reflow to fill available space
- **And** grid dimensions preserved (rows x cols unchanged)

### AC 5.2.7: Pane header shows project name
- **Given** a pane is displayed
- **When** viewing the dashboard
- **Then** each pane has a border and project label in the header

## Tasks / Subtasks

- [x] Task 1: Create DashboardModel structure (AC: 5.2.1-5.2.6)
  - [x] 1.1: Create `internal/tui/dashboard.go` file
  - [x] 1.2: Define `DashboardModel` struct with fields: `panes []PaneModel`, `focusIndex int`, `width int`, `height int`
  - [x] 1.3: Define `PaneModel` struct with fields: `project types.Project`, `width int`, `height int`
  - [x] 1.4: Implement `NewDashboardModel(projects []types.Project) DashboardModel` - initialize focusIndex to 0
  - [x] 1.5: Implement `Init() tea.Cmd` - return nil for now (Story 5.3 adds watcher)
  - [x] 1.6: Implement `SetSize(width, height int)` method - updates model dimensions and recalculates pane sizes

- [x] Task 2: Implement grid calculation algorithm (AC: 5.2.1-5.2.5)
  - [x] 2.1: Create `calculateGrid(count int) (rows, cols int)` function
  - [x] 2.2: Implement switch logic per PRD: 1→1x1, 2→1x2, 3→1x3, 4→2x2, 5-6→2x3, 7-9→3x3
  - [x] 2.3: Create `calculatePaneDimensions(totalWidth, totalHeight, rows, cols int) (paneWidth, paneHeight int)`
  - [x] 2.4: Account for borders (2 chars per pane width, 2 chars per pane height)
  - [x] 2.5: Create unit tests for `calculateGrid()` covering all cases (1-9 projects)

- [x] Task 3: Implement pane rendering (AC: 5.2.7)
  - [x] 3.1: Add pane border style to `styles.go` (use existing Border style as reference)
  - [x] 3.2: Add `PaneBorderStyle` with RoundedBorder() and BorderForeground(Adaptive.Subtle)
  - [x] 3.3: Add `PaneHeaderStyle` (bold, centered, Foreground(Adaptive.Primary))
  - [x] 3.4: Implement `(p PaneModel) View() string` method
  - [x] 3.5: Render pane with border and project.DisplayName in header (truncate if exceeds pane width)
  - [x] 3.6: Leave content area empty for now (Story 5.3 adds content)

- [x] Task 4: Implement grid View() assembly (AC: all)
  - [x] 4.1: Implement `(m DashboardModel) View() string`
  - [x] 4.2: Calculate grid dimensions using `calculateGrid(len(m.panes))`
  - [x] 4.3: Calculate pane dimensions using `calculatePaneDimensions()`
  - [x] 4.4: Create row slices of pane views
  - [x] 4.5: Use `lipgloss.JoinHorizontal(lipgloss.Top, ...)` for each row
  - [x] 4.6: Use `lipgloss.JoinVertical(lipgloss.Left, ...)` for all rows
  - [x] 4.7: Handle incomplete last row (e.g., 5 panes in 2x3 grid - last row has 2 panes + empty space)

- [x] Task 5: Implement Update() message handling (AC: 5.2.6)
  - [x] 5.1: Implement `(m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)`
  - [x] 5.2: Handle `tea.WindowSizeMsg` - call `SetSize()` and recalculate pane dimensions
  - [x] 5.3: Handle `tea.KeyMsg` for Escape/q - return `GoBackToProjectsFromDashboardMsg` (to be handled by AppModel)
  - [x] 5.4: Define `GoBackToProjectsFromDashboardMsg` struct in dashboard.go

- [x] Task 6: Integrate DashboardModel into AppModel
  - [x] 6.1: Add `dashboardModel DashboardModel` field to AppModel struct
  - [x] 6.2: In `DashboardSelectedMsg` handler, create `NewDashboardModel(msg.Projects)`
  - [x] 6.3: Call `m.dashboardModel.SetSize(m.width, m.height)`
  - [x] 6.4: Update `viewDashboard` state case in Update() to forward messages to dashboardModel
  - [x] 6.5: Update `viewDashboard` state case in View() to call `m.dashboardModel.View()`
  - [x] 6.6: Handle `GoBackToProjectsFromDashboardMsg` - clear selections and return to viewProjects
  - [x] 6.7: Remove `dashboardPlaceholderView()` function (replaced by DashboardModel)

- [x] Task 7: Add unit tests
  - [x] 7.1: Test `calculateGrid(0)` returns (0, 0) - edge case
  - [x] 7.2: Test `calculateGrid(1)` returns (1, 1)
  - [x] 7.3: Test `calculateGrid(2)` returns (1, 2)
  - [x] 7.4: Test `calculateGrid(3)` returns (1, 3)
  - [x] 7.5: Test `calculateGrid(4)` returns (2, 2)
  - [x] 7.6: Test `calculateGrid(5)` returns (2, 3)
  - [x] 7.7: Test `calculateGrid(6)` returns (2, 3)
  - [x] 7.8: Test `calculateGrid(7)` returns (3, 3)
  - [x] 7.9: Test `calculateGrid(8)` returns (3, 3)
  - [x] 7.10: Test `calculateGrid(9)` returns (3, 3)
  - [x] 7.11: Test `calculatePaneDimensions()` returns correct width/height for given grid
  - [x] 7.12: Test `NewDashboardModel()` creates correct number of panes with focusIndex=0
  - [x] 7.13: Test `SetSize()` updates model dimensions and pane dimensions
  - [x] 7.14: Test Escape key returns `GoBackToProjectsFromDashboardMsg`
  - [x] 7.15: Test `q` key returns `GoBackToProjectsFromDashboardMsg`
  - [x] 7.16: Test View() renders empty cell for incomplete row (e.g., 5 panes in 2x3)

- [x] Task 8: Run build, lint, and test validation
  - [x] 8.1: Run `make build` - verify binary builds successfully
  - [x] 8.2: Run `make lint` - no errors
  - [x] 8.3: Run `make test` - all tests pass

- [ ] Task 9: Manual testing (Requires user verification)
  - [ ] 9.1: Select 1 project, enter dashboard - verify full-screen single pane
  - [ ] 9.2: Select 2 projects, verify 1x2 layout (side by side)
  - [ ] 9.3: Select 3 projects, verify 1x3 layout
  - [ ] 9.4: Select 4 projects, verify 2x2 layout
  - [ ] 9.5: Select 5 projects, verify 2x3 layout (5 panes visible, bottom-right empty)
  - [ ] 9.6: Select 9 projects, verify 3x3 layout
  - [ ] 9.7: Resize terminal while in dashboard - verify panes reflow proportionally
  - [ ] 9.8: Verify each pane shows project name in header
  - [ ] 9.9: Verify Escape returns to project list with selections cleared

## Dev Notes

### Current Implementation State

Story 5.1 created the foundation:
- `viewDashboard` state exists in AppModel
- `selectedProjects []types.Project` field exists
- `DashboardSelectedMsg` handler transitions to viewDashboard
- Placeholder `dashboardPlaceholderView()` function exists (to be replaced)

### Architecture Reference

From `architecture-phase3.md`:

```go
// internal/tui/dashboard.go
type DashboardModel struct {
    panes      []PaneModel
    focusIndex int
    width      int
    height     int
}

type PaneModel struct {
    project      types.Project
    conversation types.Conversation  // Not used in Story 5.2
    content      []string            // Not used in Story 5.2
    scrollOffset int                 // Not used in Story 5.2
    watcher      *watcher.Watcher    // Not used in Story 5.2
}
```

**For Story 5.2, simplify PaneModel to only what's needed:**
```go
type PaneModel struct {
    project types.Project
    width   int
    height  int
}
```

### Grid Calculation Algorithm

From PRD `prd-phase3.md#FR-502`:
```go
func calculateGrid(count int) (rows, cols int) {
    switch count {
    case 0:
        return 0, 0  // Edge case: no projects
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
        return 3, 3  // 7-9 projects
    }
}
```

**Note:** Use explicit case values (not `count <= N`) to avoid confusion. Each project count maps to exactly one grid configuration.

### Pane Dimension Calculation

```go
func calculatePaneDimensions(totalWidth, totalHeight, rows, cols int) (paneWidth, paneHeight int) {
    // Guard against division by zero
    if rows == 0 || cols == 0 {
        return 0, 0
    }

    // Simple division - lipgloss borders are included in pane dimensions
    // The Border style adds 1 char on each side, accounted for in PaneModel.View()
    paneWidth = totalWidth / cols
    paneHeight = totalHeight / rows
    return paneWidth, paneHeight
}
```

**Implementation Note:** Border overhead is handled inside `PaneModel.View()` by subtracting 2 from width/height when sizing inner content, not in dimension calculation.

### Lipgloss Grid Assembly

```go
func (m DashboardModel) View() string {
    // Handle edge case: no panes
    if len(m.panes) == 0 {
        return ""
    }

    rows, cols := calculateGrid(len(m.panes))
    paneWidth, paneHeight := calculatePaneDimensions(m.width, m.height, rows, cols)

    // Update pane dimensions (NOTE: View() should be pure, move this to SetSize() in implementation)
    for i := range m.panes {
        m.panes[i].width = paneWidth
        m.panes[i].height = paneHeight
    }

    // Build rows
    var rowViews []string
    for r := 0; r < rows; r++ {
        var colViews []string
        for c := 0; c < cols; c++ {
            idx := r*cols + c
            if idx < len(m.panes) {
                colViews = append(colViews, m.panes[idx].View())
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
```

**Implementation Note:** The for-loop updating pane dimensions should ideally be in `SetSize()` to keep `View()` pure. Move dimension updates to `SetSize()` during implementation.

### Pane Rendering

```go
func (p PaneModel) View() string {
    // Guard against invalid dimensions
    if p.width < 4 || p.height < 3 {
        return ""
    }

    // Inner content width (account for left+right border = 2 chars)
    innerWidth := p.width - 2

    // Truncate project name if too long (leave room for padding)
    displayName := p.project.DisplayName
    maxNameLen := innerWidth - 2  // padding
    if len(displayName) > maxNameLen && maxNameLen > 3 {
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
```

**Key Details:**
- Guard against tiny dimensions that would cause rendering issues
- Truncate long project names with "..." to prevent overflow
- Content area height calculated as: `totalHeight - 1 (header) - 2 (borders)`

### Style Definitions to Add

```go
// In styles.go

// Dashboard pane styles
var (
    PaneBorderStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(Adaptive.Subtle)

    PaneHeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Align(lipgloss.Center).
        Foreground(Adaptive.Primary)
)
```

### Message Types

```go
// GoBackToProjectsFromDashboardMsg signals return to project list from dashboard
type GoBackToProjectsFromDashboardMsg struct{}
```

### Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/dashboard.go` | CREATE | DashboardModel, PaneModel, grid calculation |
| `internal/tui/dashboard_test.go` | CREATE | Unit tests for grid calculation and DashboardModel |
| `internal/tui/app.go` | MODIFY | Add dashboardModel field, update handlers |
| `internal/tui/styles.go` | MODIFY | Add PaneBorderStyle, PaneHeaderStyle |

### Project Context Rules (from project-context.md)

| Rule | Application |
|------|-------------|
| **NO EMOJI IN UI** | Text icons only - no emoji in pane headers |
| **TEA pattern** | DashboardModel implements tea.Model interface |
| **Use Makefile** | `make build`, `make test` |
| **Import order** | stdlib → external → internal |
| **Lipgloss styling** | All styles in styles.go |
| **Box-drawing borders** | Use RoundedBorder() for panes |

### Previous Story 5.1 Patterns to Follow

1. **Message struct naming**: Use `{Feature}Msg` pattern (e.g., `GoBackToProjectsFromDashboardMsg`)
2. **Style organization**: Add new styles to styles.go with descriptive names
3. **Model creation**: Use `New{Model}(params) {Model}` constructor pattern
4. **SetSize method**: Follow existing `SetSize(width, height int)` pattern from other models
5. **Update routing**: AppModel forwards messages to child model based on state

### Key Implementation Decisions

1. **PaneModel is minimal for Story 5.2**: Only `project`, `width`, `height` fields. Story 5.3 adds watcher, content, scrolling.

2. **Focus navigation deferred to Story 5.5**: This story only implements grid layout. `focusIndex` field exists but no arrow key handling yet.

3. **Empty pane content**: Panes show only header with project name. Content area is blank. Story 5.3 adds conversation content.

4. **Incomplete row handling**: For 5 projects in 2x3 grid, bottom-right cell renders as empty space (same dimensions, no border).

### Testing Strategy

**Unit tests focus on:**
- Grid calculation correctness (all 9 cases)
- Pane dimension calculation
- DashboardModel construction
- Message handling (Escape key)

**Manual tests focus on:**
- Visual layout correctness at different project counts
- Terminal resize behavior
- Navigation back to project list

### References

- [Source: epics-phase3.md#Story-5.2] - Acceptance criteria
- [Source: prd-phase3.md#FR-502] - Grid layout requirements
- [Source: architecture-phase3.md#Dashboard-Component-Structure] - DashboardModel design
- [Source: architecture-phase3.md#Grid-Layout-Algorithm] - calculateGrid() spec
- [Source: 5-1-multi-project-selection.md] - Previous story patterns
- [Source: internal/tui/app.go] - Current AppModel with dashboard placeholder

## Implementation Checklist

Before marking story complete, verify:

**Structure:**
- [ ] `dashboard.go` file created in `internal/tui/`
- [ ] `DashboardModel` struct with required fields (panes, focusIndex, width, height)
- [ ] `PaneModel` struct with required fields (project, width, height)
- [ ] `NewDashboardModel()` constructor initializes focusIndex=0
- [ ] `SetSize()` method updates model AND pane dimensions
- [ ] `GoBackToProjectsFromDashboardMsg` struct defined

**Grid Calculation:**
- [ ] `calculateGrid(0)` returns (0, 0)
- [ ] `calculateGrid()` returns correct rows/cols for all 1-9 cases
- [ ] `calculatePaneDimensions()` correctly divides space
- [ ] `calculatePaneDimensions()` guards against division by zero

**Rendering:**
- [ ] `PaneModel.View()` renders border and header
- [ ] `PaneModel.View()` truncates long project names with "..."
- [ ] `PaneModel.View()` guards against tiny dimensions
- [ ] `DashboardModel.View()` assembles grid correctly
- [ ] `DashboardModel.View()` handles 0 panes edge case
- [ ] Incomplete rows render empty cells (same dimensions, no border)
- [ ] `PaneBorderStyle` and `PaneHeaderStyle` defined in styles.go

**Integration:**
- [ ] AppModel has `dashboardModel DashboardModel` field
- [ ] `DashboardSelectedMsg` handler creates and sizes DashboardModel
- [ ] `viewDashboard` state forwards messages to dashboardModel.Update()
- [ ] `viewDashboard` state renders dashboardModel.View()
- [ ] `GoBackToProjectsFromDashboardMsg` clears selections and returns to viewProjects
- [ ] `dashboardPlaceholderView()` function removed

**Testing:**
- [ ] Edge case: `calculateGrid(0)` test
- [ ] All grid calculation unit tests pass (1-9 projects)
- [ ] All dimension calculation tests pass
- [ ] Test Escape and q key return correct message
- [ ] Test incomplete row rendering
- [ ] `make build` succeeds
- [ ] `make lint` has no errors
- [ ] `make test` passes

**Manual Verification (Requires User):**
- [ ] 1 project → 1x1 layout (full screen)
- [ ] 2 projects → 1x2 layout (side by side)
- [ ] 3 projects → 1x3 layout
- [ ] 4 projects → 2x2 layout
- [ ] 5 projects → 2x3 layout (bottom-right empty)
- [ ] 9 projects → 3x3 layout
- [ ] Terminal resize reflows panes proportionally
- [ ] Pane headers show project names (truncated if long)
- [ ] Escape/q returns to project list with selections cleared

## Validation Record

### Validation Performed By
Scrum Master (SM Agent) - 2026-01-19

### INVEST Criteria Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| Independent | ✅ | Depends only on Story 5.1 (done) |
| Negotiable | ✅ | Implementation details flexible |
| Valuable | ✅ | Delivers grid layout for dashboard |
| Estimable | ✅ | Clear scope, well-defined tasks |
| Small | ✅ | Single component, manageable |
| Testable | ✅ | Clear ACs with Given/When/Then |

### Improvements Applied

1. **Task 1**: Added explicit focusIndex=0 initialization and clarified SetSize responsibility
2. **Task 3**: Fixed style names to match actual implementation (PaneBorderStyle, PaneHeaderStyle)
3. **Task 5.3**: Fixed message name inconsistency (was GoBackToDashboardMsg, now GoBackToProjectsFromDashboardMsg)
4. **Task 7**: Added edge case tests (0 projects, q key, incomplete row rendering)
5. **Grid Algorithm**: Changed from `count <= N` to explicit case values to prevent confusion
6. **Pane Dimension**: Added division-by-zero guard and clarified border handling
7. **Lipgloss Grid Assembly**: Added 0-pane edge case and TEA pattern note
8. **Pane Rendering**: Added dimension guards, name truncation logic, height calculation clarification
9. **Implementation Checklist**: Expanded with edge cases, guards, and detailed verification items

### AC Coverage Verification

| AC | Task Coverage | Status |
|----|---------------|--------|
| 5.2.1 (1x1) | Task 2, Task 7.2 | ✅ |
| 5.2.2 (1x2, 1x3) | Task 2, Task 7.3-7.4 | ✅ |
| 5.2.3 (2x2) | Task 2, Task 7.5 | ✅ |
| 5.2.4 (2x3) | Task 2, Task 7.6-7.7 | ✅ |
| 5.2.5 (3x3) | Task 2, Task 7.8-7.10 | ✅ |
| 5.2.6 (resize) | Task 5, Task 6 | ✅ |
| 5.2.7 (header) | Task 3 | ✅ |

### Risks Identified

- **Low**: Terminal size below minimum dimensions (4x3) - handled with guards
- **Low**: Project names exceeding pane width - handled with truncation

---

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - no debug issues encountered

### Completion Notes List

- Created `internal/tui/dashboard.go` with DashboardModel and PaneModel structs
- Implemented `calculateGrid()` function with explicit case values (0-9 projects)
- Implemented `calculatePaneDimensions()` with division-by-zero guards
- Added `PaneBorderStyle` and `PaneHeaderStyle` to `internal/tui/styles.go`
- PaneModel.View() handles dimension guards and long project name truncation with "..."
- DashboardModel.View() assembles grid using lipgloss.JoinHorizontal/JoinVertical
- Empty cells rendered for incomplete rows (e.g., 5 panes in 2x3 grid)
- Integrated DashboardModel into AppModel replacing dashboardPlaceholderView()
- GoBackToProjectsFromDashboardMsg handled to clear selections and return to viewProjects
- Created comprehensive unit tests covering all grid calculations, model construction, key handling, and edge cases
- All 17 dashboard-related tests pass
- Build, lint, and test validation all pass

### File List

- `internal/tui/dashboard.go` (CREATED) - DashboardModel, PaneModel, grid calculation functions
- `internal/tui/dashboard_test.go` (CREATED) - Unit tests for dashboard functionality
- `internal/tui/styles.go` (MODIFIED) - Added PaneBorderStyle, PaneHeaderStyle
- `internal/tui/app.go` (MODIFIED) - Added dashboardModel field, updated handlers, removed dashboardPlaceholderView()

### Change Log

- 2026-01-19: Implemented Story 5.2 Grid Layout Component - all automated tasks complete, ready for manual testing
- 2026-01-19: Code review completed - 4 MEDIUM, 3 LOW issues found and fixed:
  - M3 FIXED: Updated calculateGrid comment from "7-9 projects" to "7+ projects (max 9 per Story 5.1)"
  - L2 FIXED: Added tests for calculateGrid(10) and calculateGrid(12) edge cases
  - L3 FIXED: Changed fragile string(rune) conversion to fmt.Sprintf for project name generation in tests
  - M1, M2, M4, L1: Documented but not changed (acceptable patterns, no functional issues)

### Senior Developer Review (AI)

**Reviewer:** Dev Agent (Amelia) - Claude Opus 4.5
**Date:** 2026-01-19

**Outcome:** APPROVED

**Review Summary:**
- All Acceptance Criteria implemented correctly
- All tasks marked [x] verified as complete
- Git changes match story File List
- 19 dashboard-related tests pass
- Build, lint, and full test suite pass

**Issues Found and Resolved:**
| Severity | Issue | Resolution |
|----------|-------|------------|
| MEDIUM | M1: Value vs pointer receiver inconsistency in Update/SetSize | Documented - works correctly |
| MEDIUM | M2: View() recalculates pane dimensions | Acceptable - local copy only |
| MEDIUM | M3: calculateGrid comment misleading for count > 9 | FIXED - updated comment |
| MEDIUM | M4: AppModel.Init() doesn't init dashboard | Acceptable - flow guards this |
| LOW | L1: dimColor vs Adaptive.Subtle naming | Functionally equivalent |
| LOW | L2: Missing test for count > 9 | FIXED - added tests |
| LOW | L3: Fragile string conversion in test | FIXED - use fmt.Sprintf |

### Post-Review Fix (AI)

**Date:** 2026-01-19
**Issue:** First row title clipped with top border line
**Root Cause:** `lipgloss.Height()` is unreliable (documented in docs/lessons-learned.md)
**Fix:** Replaced `PaneBorderStyle.Height(p.height).Render(inner)` with manual border drawing using `addBorder()` utility function for reliable height control
