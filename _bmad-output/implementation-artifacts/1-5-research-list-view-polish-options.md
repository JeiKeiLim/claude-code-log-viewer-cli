# Story 1.5: Research List View Polish Options

Status: complete

## Story

As a **developer improving cclv's UI**,
I want **to research and prototype list view styling options**,
So that **we can make an informed decision on how to polish the project and conversation lists**.

## Acceptance Criteria

### AC 1.5.1: Research TUI patterns
- **Given** the need to polish list views
- **When** I research other Bubbletea/Charm applications
- **Then** I document at least 3 different styling approaches
- **And** include screenshots or descriptions of each

### AC 1.5.2: Prototype options
- **Given** the research findings
- **When** I create quick prototypes
- **Then** at least 2 viable options are demonstrated
- **And** each shows how it would look in cclv

### AC 1.5.3: Recommendation
- **Given** the prototypes
- **When** research is complete
- **Then** a recommended approach is documented
- **And** rationale explains why it fits cclv best

## Tasks / Subtasks

- [x] Task 1: Research TUI list styling patterns (AC: 1.5.1)
  - [x] 1.1: Study charm/glow list implementation and styling approach
  - [x] 1.2: Study charm/soft-serve list implementation and styling
  - [x] 1.3: Study other popular bubbles/list implementations (mods, gh-dash, etc.)
  - [x] 1.4: Document findings in research output file

- [x] Task 2: Analyze current cclv list implementation (AC: 1.5.1, 1.5.2)
  - [x] 2.1: Review `project.go` ListViewport and item rendering
  - [x] 2.2: Review `conversation.go` ListViewport and item rendering
  - [x] 2.3: Identify integration points with existing Theme/Styles
  - [x] 2.4: Note constraints from ListViewport implementation

- [x] Task 3: Create prototypes (AC: 1.5.2)
  - [x] 3.1: Prototype Option A - Enhanced selection highlighting with border
  - [x] 3.2: Prototype Option B - Alternating row backgrounds with selection
  - [x] 3.3: Create ASCII mockups showing each option
  - [x] 3.4: Document pros/cons for each approach

- [x] Task 4: Evaluate and recommend (AC: 1.5.3)
  - [x] 4.1: Compare prototypes against existing viewer polish
  - [x] 4.2: Evaluate implementation complexity
  - [x] 4.3: Select recommended approach
  - [x] 4.4: Write recommendation with rationale

- [x] Task 5: Create research output (all ACs)
  - [x] 5.1: Write research document with all findings
  - [x] 5.2: Include implementation guidance for Story 1.6
  - [x] 5.3: Save to `_bmad-output/planning-artifacts/research/list-view-polish-research-{{date}}.md`

## Dev Notes

### Story Type

**RESEARCH SPIKE** - This story produces research documentation, NOT production code.

- Output is a research document with findings and recommendations
- Prototypes are conceptual/ASCII mockups, not committed code
- Informs implementation decisions for Story 1.6

### Context: Visual Consistency Gap

From Epic 1 Retrospective:
- Epic 1 polished the **viewer** with rounded borders, adaptive colors, segmented status bar
- **Project list** and **conversation list** remain utilitarian
- Creates visual inconsistency when navigating: polished viewer vs plain lists

### Current List Implementation Analysis

**Files to analyze:**
- `internal/tui/project.go` - Project list with ListViewport
- `internal/tui/conversation.go` - Conversation list with ListViewport
- `internal/tui/listviewport.go` - Custom viewport implementation
- `internal/tui/styles.go` - Theme and Styles already available

**Current styling (from project.go:20-48):**
```go
// Current item rendering
func (i ProjectItem) Render(width int, selected bool) string {
    var prefix string
    var titleStyle, descStyle lipgloss.Style
    if selected {
        prefix = " > "
        titleStyle = Styles.Selected  // Purple background, white text, bold
        descStyle = Styles.Selected.Background(lipgloss.Color("#4C1D95"))
    } else {
        prefix = "   "
        titleStyle = Styles.Normal.Bold(true)
        descStyle = Styles.Muted
    }
    // ... renders title + description on 2 lines
}
```

**ListViewport implementation (listviewport.go):**
- Custom viewport with strict height control
- Items rendered via `Render(width, selected)` interface
- Supports scrolling, cursor movement, g/G navigation
- Does NOT use bubbles/list (avoids height bug)

### Available Theme Colors

From `styles.go:34-58`, already available:
- `Primary` - Purple (#5B21B6/#7C3AED)
- `Secondary` - Green (#059669/#10B981)
- `Accent` - Amber (#D97706/#F59E0B)
- `Text` - Dark/Light gray
- `Muted` - Muted gray
- `BgAlt` - Alternate background

### Research Targets

**1. charm/glow**
- Markdown reader with file browser
- Uses bubbles/list with custom delegate
- Known for polished aesthetics

**2. charm/soft-serve**
- Git server TUI
- Repository list implementation
- SSH integration styling

**3. Other references**
- mods (charm CLI tool)
- gh-dash (GitHub dashboard)
- lazygit list views

### Styling Options to Explore

**Option A: Enhanced Selection with Border**
- Add subtle border to selected item
- Use RoundedBorder like viewer message cards
- Keep unselected items borderless

**Option B: Alternating Row Backgrounds**
- Zebra striping with BgAlt color
- Clear selection highlighting
- Common TUI pattern

**Option C: Card-style Items**
- Each item in its own bordered card
- Similar to viewer message styling
- May consume more vertical space

**Option D: Minimal Enhancement**
- Keep current layout
- Add subtle color refinements
- Focus on selection visibility only

### Constraints

1. **No emoji** - Text icons only per FR-017
2. **Existing colors** - Use Theme palette, no new colors
3. **ListViewport compatibility** - Must work with existing viewport
4. **Height preservation** - 2 lines per item is current standard
5. **Performance** - No rendering in View() callbacks

### Output Location

Research document should be saved to:
```
_bmad-output/planning-artifacts/research/list-view-polish-research-2026-01-16.md
```

### Research Document Structure

```markdown
# List View Polish Research - cclv Phase 2

## Executive Summary
[Recommended approach with brief rationale]

## Research Methodology
[Tools/apps studied, evaluation criteria]

## Pattern Analysis

### Pattern 1: [Name]
- Source: [App name]
- Visual description
- Implementation approach
- Pros/Cons for cclv

### Pattern 2: [Name]
...

## Prototypes

### Option A: [Name]
[ASCII mockup]
Pros: ...
Cons: ...

### Option B: [Name]
[ASCII mockup]
Pros: ...
Cons: ...

## Recommendation

### Selected Approach: [Name]

**Rationale:**
- Why it fits cclv
- How it matches viewer polish
- Implementation complexity

### Implementation Guidance for Story 1.6
- Files to modify
- Specific changes needed
- Integration with existing Theme

## References
[Links to studied apps, documentation]
```

### Previous Story Intelligence

From Stories 1.1-1.4:
- **Theme struct working** - All adaptive colors properly initialized
- **Styles centralized** - All styles in styles.go
- **Build commands** - `make build` and `make test` always
- **Import order** - stdlib, external, internal
- **No width hacks** - Let lipgloss handle centering

### Project Structure Notes

Relevant files:
- `internal/tui/project.go` - Project list (2 lines per item)
- `internal/tui/conversation.go` - Conversation list (2 lines per item)
- `internal/tui/listviewport.go` - Custom ListViewport[T]
- `internal/tui/styles.go` - Theme, Styles, ListStyles

### References

- [Source: internal/tui/project.go:20-48] - Current item rendering
- [Source: internal/tui/conversation.go:20-70] - Current conversation item rendering
- [Source: internal/tui/listviewport.go] - Custom viewport implementation
- [Source: internal/tui/styles.go:34-58] - Theme definition
- [Source: internal/tui/styles.go:127-237] - Styles struct
- [Source: _bmad-output/implementation-artifacts/epic-1-retro-2026-01-15.md] - Visual consistency gap identified
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.5] - Acceptance criteria
- [Source: _bmad-output/project-context.md#Styling Rules] - NO EMOJI rule

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Research task, no code execution

### Completion Notes List

1. Researched charm/glow stash view styling - identified gutter indicator pattern
2. Researched charm/soft-serve selector component - confirmed delegate pattern
3. Researched bubbles default list delegate - identified three-state styling (normal, selected, dimmed)
4. Analyzed current cclv implementation in project.go, conversation.go, listviewport.go, styles.go
5. Created 4 prototype options with ASCII mockups (Card-style, Gutter Indicator, Zebra Striping, Minimal)
6. Recommended Option B: Gutter Indicator with Enhanced Selection
7. Wrote comprehensive research document with implementation guidance for Story 1.6

### File List

- `_bmad-output/planning-artifacts/research/list-view-polish-research-2026-01-16.md` (created)
