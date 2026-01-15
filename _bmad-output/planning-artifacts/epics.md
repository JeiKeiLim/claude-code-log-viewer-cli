---
stepsCompleted: ['step-01-validate-prerequisites', 'step-02-design-epics', 'step-03-create-stories', 'step-04-review', 'step-05-complete']
inputDocuments:
  - '_bmad-output/planning-artifacts/prd.md'
  - '_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md'
  - '_bmad-output/project-context.md'
status: complete
---

# cclv Visual & Streaming Enhancements (Phase 2) - Epics & Stories

## Overview

This document provides the complete epic and story breakdown for cclv Phase 2, decomposing the PRD requirements into implementable stories organized by feature area.

## Requirements Inventory

### Functional Requirements

| ID | Description | Epic |
|----|-------------|------|
| FR-101 | Adaptive Color System | Epic 1 |
| FR-102 | Rounded Border Styling | Epic 1 |
| FR-103 | Segmented Status Bar | Epic 1 |
| FR-104 | Spinner Animation | Epic 1 |
| FR-201 | File Watch Mode Flag | Epic 2 |
| FR-202 | fsnotify Integration | Epic 2 |
| FR-203 | Auto-scroll on New Entries | Epic 2 |
| FR-301 | Glamour Integration | Epic 3 |
| FR-302 | Dynamic Word Wrap | Epic 3 |
| FR-303 | Render Caching | Epic 3 |

### Non-Functional Requirements

| ID | Description | Covered By |
|----|-------------|------------|
| NFR-001 | Performance targets | All stories (validation) |
| NFR-002 | Compatibility | Story 1.1, 3.1 |
| NFR-003 | Code Quality | All stories (90% coverage) |

### Additional Requirements

- No emoji in UI - text icons only
- Use Makefile for builds
- TEA pattern for all state changes
- Never render in View()
- New dependencies: Glamour (approved), fsnotify (to approve)

---

## Epic List

| Epic | Title | Stories | Priority | Status |
|------|-------|---------|----------|--------|
| 1 | Visual Polish | 4 | High | Done |
| 1.5 | Visual Consistency | 4 | High | Backlog |
| 2 | Real-time File Watching | 3 | High | Backlog |
| 3 | Markdown Rendering | 3 | Medium | Backlog |

---

## Epic 1: Visual Polish

**Goal:** Transform cclv's utilitarian appearance into a polished, modern TUI with theme-aware colors, rounded borders, and visual feedback.

**Requirements Covered:** FR-101, FR-102, FR-103, FR-104, NFR-002

**Dependencies:** None (foundational)

**Risk:** Low

---

### Story 1.1: Implement Adaptive Color System

**As a** developer using cclv,
**I want** colors to automatically adapt to my terminal's light/dark theme,
**So that** the UI is readable and visually consistent regardless of my terminal settings.

**Requirements:** FR-101

**Acceptance Criteria:**

**AC 1.1.1: Theme struct creation**
- **Given** the cclv codebase
- **When** I create a new Theme struct in styles.go
- **Then** it contains AdaptiveColor fields for: Primary, Secondary, Accent, Text, Muted, Background
- **And** each color has both light and dark variants

**AC 1.1.2: Light theme adaptation**
- **Given** a terminal with light background
- **When** cclv renders the UI
- **Then** text is dark and readable against the light background
- **And** accent colors are visible and distinct

**AC 1.1.3: Dark theme adaptation**
- **Given** a terminal with dark background
- **When** cclv renders the UI
- **Then** text is light and readable against the dark background
- **And** accent colors are visible and distinct

**AC 1.1.4: No hardcoded colors**
- **Given** the view rendering code
- **When** I search for hardcoded color values
- **Then** all colors reference the Theme struct
- **And** no raw hex or ANSI codes exist in view code

**Technical Notes:**
```go
type Theme struct {
    Primary    lipgloss.AdaptiveColor
    Secondary  lipgloss.AdaptiveColor
    Accent     lipgloss.AdaptiveColor
    Text       lipgloss.AdaptiveColor
    Muted      lipgloss.AdaptiveColor
    Background lipgloss.AdaptiveColor
}

var DefaultTheme = Theme{
    Primary: lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#eaeaea"},
    // ... etc
}
```

---

### Story 1.2: Apply Rounded Border Styling

**As a** developer viewing conversation logs,
**I want** message cards to have rounded borders,
**So that** the UI feels modern and polished.

**Requirements:** FR-102

**Acceptance Criteria:**

**AC 1.2.1: User message styling**
- **Given** a log entry with type "human"
- **When** it renders in the viewport
- **Then** it displays with RoundedBorder() style
- **And** uses the Theme's user message colors

**AC 1.2.2: Assistant message styling**
- **Given** a log entry with type "assistant"
- **When** it renders in the viewport
- **Then** it displays with RoundedBorder() style
- **And** uses the Theme's assistant message colors

**AC 1.2.3: Tool call styling**
- **Given** a log entry with type "tool_use" or "tool_result"
- **When** it renders in the viewport
- **Then** it displays with RoundedBorder() style
- **And** uses the Theme's tool colors (muted/secondary)

**AC 1.2.4: Border characters**
- **Given** any bordered message card
- **When** rendered in terminal
- **Then** uses Unicode rounded corners: ╭ ╮ ╰ ╯

**Technical Notes:**
```go
cardStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(theme.Primary).
    Padding(0, 1)
```

---

### Story 1.3: Create Segmented Status Bar

**As a** developer using cclv,
**I want** a visually distinct status bar with colored sections,
**So that** I can quickly see keyboard shortcuts and current position.

**Requirements:** FR-103

**Acceptance Criteria:**

**AC 1.3.1: Segmented layout**
- **Given** the status bar component
- **When** it renders at the bottom of the screen
- **Then** it shows distinct colored segments using JoinHorizontal()
- **And** segments have contrasting background colors

**AC 1.3.2: Keyboard shortcuts visible**
- **Given** the status bar
- **When** viewing the UI
- **Then** common shortcuts are displayed (q=quit, j/k=nav, etc.)
- **And** shortcuts use muted/secondary colors

**AC 1.3.3: Position indicator**
- **Given** a log file with multiple entries
- **When** viewing the status bar
- **Then** current entry index and total count are displayed
- **And** format is "Entry X of Y" or similar

**AC 1.3.4: Mode indicator**
- **Given** watch mode is enabled
- **When** viewing the status bar
- **Then** a "WATCHING" or "LIVE" indicator is visible
- **And** uses accent color to stand out

**Technical Notes:**
```go
statusBar := lipgloss.JoinHorizontal(
    lipgloss.Top,
    modeSegment,
    positionSegment,
    shortcutsSegment,
)
```

---

### Story 1.4: Add Spinner Animation

**As a** developer,
**I want** to see a spinner during loading operations,
**So that** I know the application is working.

**Requirements:** FR-104

**Acceptance Criteria:**

**AC 1.4.1: Spinner during file load**
- **Given** cclv is loading a large log file
- **When** parsing is in progress
- **Then** a spinner animation displays
- **And** spinner uses bubbles/spinner component

**AC 1.4.2: Spinner during watch init**
- **Given** watch mode is enabled
- **When** fsnotify watcher is initializing
- **Then** a spinner displays with "Watching..." text

**AC 1.4.3: Spinner stops on completion**
- **Given** a loading operation completes
- **When** data is ready to display
- **Then** spinner stops and content renders
- **And** no visual artifacts remain

**AC 1.4.4: Spinner tick command**
- **Given** the Bubbletea model
- **When** spinner is active
- **Then** spinner.Tick is returned from Update()
- **And** animation runs at smooth framerate

**Technical Notes:**
```go
type model struct {
    spinner  spinner.Model
    loading  bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.loading {
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    // ...
}
```

---

## Epic 1.5: Visual Consistency

**Goal:** Complete the visual polish across all views (project list, conversation list) and refine spacing for a cohesive, polished UI experience.

**Requirements Covered:** Retrospective finding - visual consistency gap identified in Epic 1 retro

**Dependencies:** Epic 1 (uses Theme, AdaptiveColors, styling patterns)

**Risk:** Low

**Origin:** Added from Epic 1 Retrospective (2026-01-15)

---

### Story 1.5: Research List View Polish Options

**As a** developer improving cclv's UI,
**I want** to research and prototype list view styling options,
**So that** we can make an informed decision on how to polish the project and conversation lists.

**Requirements:** Retrospective finding

**Acceptance Criteria:**

**AC 1.5.1: Research TUI patterns**
- **Given** the need to polish list views
- **When** I research other Bubbletea/Charm applications
- **Then** I document at least 3 different styling approaches
- **And** include screenshots or descriptions of each

**AC 1.5.2: Prototype options**
- **Given** the research findings
- **When** I create quick prototypes
- **Then** at least 2 viable options are demonstrated
- **And** each shows how it would look in cclv

**AC 1.5.3: Recommendation**
- **Given** the prototypes
- **When** research is complete
- **Then** a recommended approach is documented
- **And** rationale explains why it fits cclv best

**Technical Notes:**
- This is a research spike - output is a recommendation, not production code
- Look at: charm/glow, charm/soft-serve, other bubbles/list implementations
- Consider: rounded borders on items, alternating row colors, selection highlighting

---

### Story 1.6: Implement List View Polish

**As a** developer using cclv,
**I want** the project and conversation list views to match the polished viewer styling,
**So that** the entire UI feels cohesive and modern.

**Requirements:** Retrospective finding

**Acceptance Criteria:**

**AC 1.6.1: Project list styling**
- **Given** the project list view
- **When** it renders
- **Then** it uses the Theme's adaptive colors
- **And** visual styling matches the polished viewer

**AC 1.6.2: Conversation list styling**
- **Given** the conversation list view
- **When** it renders
- **Then** it uses the Theme's adaptive colors
- **And** visual styling matches the polished viewer

**AC 1.6.3: Selection highlighting**
- **Given** a list with items
- **When** an item is selected
- **Then** it has clear visual distinction
- **And** uses Theme colors for highlighting

**AC 1.6.4: Consistent with viewer**
- **Given** the polished lists
- **When** navigating from list to viewer
- **Then** the visual transition feels seamless
- **And** no jarring style changes occur

**Technical Notes:**
- Modify `internal/tui/project.go` and `internal/tui/conversation.go`
- Use existing Theme and Styles from styles.go
- May need to customize bubbles/list delegate styling

---

### Story 1.7: Adjust Spacing and Margins

**As a** developer viewing conversation logs,
**I want** tighter spacing between message cards,
**So that** screen space is used efficiently while maintaining readability.

**Requirements:** Retrospective finding

**Acceptance Criteria:**

**AC 1.7.1: Reduced message margins**
- **Given** the viewer with message cards
- **When** messages render
- **Then** MarginBottom is reduced from 1 to 0
- **And** rounded borders provide sufficient visual separation

**AC 1.7.2: Visual separation maintained**
- **Given** reduced margins
- **When** viewing consecutive messages
- **Then** each message is still clearly distinct
- **And** readability is not compromised

**AC 1.7.3: Consistent spacing**
- **Given** all message types (User, Assistant, Thinking, Tool)
- **When** they render
- **Then** spacing is consistent across all types
- **And** the layout feels balanced

**Technical Notes:**
- Modify `internal/tui/styles.go`
- Change `MarginBottom(1)` to `MarginBottom(0)` for message styles
- Test with various message combinations to ensure readability

---

### Story 1.8: Implement Overlay Spinner

**As a** developer navigating large lists,
**I want** a spinner overlay during bulk loading operations,
**So that** I get visual feedback without losing context of the current view.

**Requirements:** Retrospective finding

**Acceptance Criteria:**

**AC 1.8.1: Overlay rendering**
- **Given** a bulk loading operation (e.g., pressing 'G' on large list)
- **When** loading is in progress
- **Then** a spinner displays overlaid on the current view
- **And** the underlying list remains visible

**AC 1.8.2: Non-blocking display**
- **Given** the overlay spinner
- **When** it renders
- **Then** it appears centered over the list
- **And** does NOT replace the entire view

**AC 1.8.3: Clean disappearance**
- **Given** loading completes
- **When** data is ready
- **Then** spinner overlay disappears
- **And** no visual artifacts remain

**AC 1.8.4: Consistent styling**
- **Given** the overlay spinner
- **When** it displays
- **Then** it uses the same spinner style as Story 1.4
- **And** matches the Theme's accent colors

**Technical Notes:**
```go
// Conceptual approach for overlay
func (m Model) View() string {
    listView := m.list.View()
    if m.bulkLoading {
        overlay := lipgloss.Place(m.width, m.height,
            lipgloss.Center, lipgloss.Center,
            m.spinner.View() + " Loading...")
        return overlayViews(listView, overlay)
    }
    return listView
}
```
- May need to implement `overlayViews()` helper or use lipgloss compositing
- Trigger on 'G' key when list has many items and needs to load metadata

---

## Epic 2: Real-time File Watching

**Goal:** Enable cclv to automatically detect and display new log entries as Claude writes them, without requiring restart.

**Requirements Covered:** FR-201, FR-202, FR-203

**Dependencies:** Epic 1 (spinner for watch init)

**Risk:** Medium (goroutine management, channel patterns)

---

### Story 2.1: Add Watch Mode CLI Flag

**As a** developer,
**I want** to enable watch mode via CLI flag,
**So that** I can monitor live Claude sessions.

**Requirements:** FR-201

**Acceptance Criteria:**

**AC 2.1.1: --watch flag**
- **Given** the cclv CLI
- **When** I run `cclv --watch <file>`
- **Then** watch mode is enabled
- **And** file monitoring starts after initial load

**AC 2.1.2: --live alias**
- **Given** the cclv CLI
- **When** I run `cclv --live <file>`
- **Then** it behaves identically to --watch

**AC 2.1.3: Help documentation**
- **Given** I run `cclv --help`
- **When** viewing the output
- **Then** --watch and --live flags are documented
- **And** description explains real-time monitoring

**AC 2.1.4: Flag stored in model**
- **Given** watch mode is enabled via flag
- **When** the Bubbletea model initializes
- **Then** a `watchMode bool` field is set to true
- **And** this triggers watcher initialization

**Technical Notes:**
```go
var watchFlag bool
var liveFlag bool

func init() {
    rootCmd.Flags().BoolVar(&watchFlag, "watch", false, "Watch file for changes")
    rootCmd.Flags().BoolVar(&liveFlag, "live", false, "Alias for --watch")
}
```

---

### Story 2.2: Implement fsnotify File Watching

**As a** developer in watch mode,
**I want** new log entries to appear automatically,
**So that** I can monitor Claude's work in real-time.

**Requirements:** FR-202

**Acceptance Criteria:**

**AC 2.2.1: fsnotify watcher setup**
- **Given** watch mode is enabled
- **When** cclv starts
- **Then** fsnotify watcher is created for the log file
- **And** watcher runs in a managed goroutine

**AC 2.2.2: File change detection**
- **Given** the watcher is running
- **When** the log file is modified
- **Then** a Write event is detected
- **And** new content is read and parsed

**AC 2.2.3: Channel to Bubbletea**
- **Given** new entries are parsed
- **When** they are ready for display
- **Then** they are sent via channel to Bubbletea
- **And** tea.Cmd chaining pattern is used

**AC 2.2.4: File truncation handling**
- **Given** the log file is truncated (new session)
- **When** detected by watcher
- **Then** entry list is cleared and reloaded
- **And** no crash or error occurs

**AC 2.2.5: Watcher cleanup**
- **Given** the user quits cclv
- **When** the application exits
- **Then** fsnotify watcher is properly closed
- **And** no goroutine leaks occur

**Technical Notes:**
```go
func watchFile(path string, entriesChan chan<- []Entry) tea.Cmd {
    return func() tea.Msg {
        watcher, _ := fsnotify.NewWatcher()
        watcher.Add(path)
        // ... watch loop
        return newEntriesMsg{entries}
    }
}
```

---

### Story 2.3: Implement Smart Auto-scroll

**As a** developer watching live logs,
**I want** the view to auto-scroll when I'm at the bottom,
**So that** I see new entries without manual scrolling.

**Requirements:** FR-203

**Acceptance Criteria:**

**AC 2.3.1: Auto-scroll when at bottom**
- **Given** the user is viewing the last entry
- **When** new entries arrive
- **Then** the view automatically scrolls to show them
- **And** the newest entry is visible

**AC 2.3.2: No auto-scroll when scrolled up**
- **Given** the user has scrolled up to view history
- **When** new entries arrive
- **Then** the view does NOT auto-scroll
- **And** user's scroll position is preserved

**AC 2.3.3: New entries indicator**
- **Given** user is scrolled up and new entries arrive
- **When** viewing the status bar
- **Then** an indicator shows "X new entries" or similar
- **And** indicator uses accent color

**AC 2.3.4: Jump to bottom**
- **Given** new entries indicator is visible
- **When** user presses 'G' (end) or designated key
- **Then** view jumps to the newest entry
- **And** indicator clears

**Technical Notes:**
```go
func (m model) isAtBottom() bool {
    return m.viewport.AtBottom() ||
           m.selectedIndex >= len(m.entries)-1
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case newEntriesMsg:
        wasAtBottom := m.isAtBottom()
        m.entries = append(m.entries, msg.entries...)
        if wasAtBottom {
            m.selectedIndex = len(m.entries) - 1
        } else {
            m.newEntriesCount += len(msg.entries)
        }
    }
}
```

---

## Epic 3: Markdown Rendering

**Goal:** Render assistant text content as formatted markdown with syntax highlighting, proper word wrap, and performance optimization.

**Requirements Covered:** FR-301, FR-302, FR-303, NFR-001

**Dependencies:** None (can parallelize with Epic 2)

**Risk:** Medium (render performance, cache invalidation)

---

### Story 3.1: Integrate Glamour Markdown Renderer

**As a** developer viewing assistant responses,
**I want** markdown to be rendered with formatting,
**So that** code blocks, headers, and lists are readable.

**Requirements:** FR-301

**Acceptance Criteria:**

**AC 3.1.1: Glamour dependency**
- **Given** the go.mod file
- **When** I add Glamour
- **Then** `go mod tidy` succeeds
- **And** `make build` produces working binary

**AC 3.1.2: Renderer initialization**
- **Given** the Bubbletea model
- **When** it initializes
- **Then** a Glamour renderer is created with WithAutoStyle()
- **And** renderer adapts to terminal theme

**AC 3.1.3: Assistant text rendering**
- **Given** a log entry with type "assistant"
- **When** the text content is displayed
- **Then** it passes through Glamour renderer
- **And** markdown formatting is applied

**AC 3.1.4: Code block syntax highlighting**
- **Given** assistant text with code blocks
- **When** rendered
- **Then** code has syntax highlighting
- **And** language-specific coloring when specified

**AC 3.1.5: Non-assistant content unchanged**
- **Given** user messages or tool calls
- **When** displayed
- **Then** they do NOT pass through Glamour
- **And** render as plain text with card styling

**Technical Notes:**
```go
renderer, _ := glamour.NewTermRenderer(
    glamour.WithAutoStyle(),
    glamour.WithWordWrap(width),
)

func (m model) renderAssistantText(text string) string {
    rendered, _ := m.renderer.Render(text)
    return rendered
}
```

---

### Story 3.2: Implement Dynamic Word Wrap

**As a** developer resizing my terminal,
**I want** markdown content to rewrap correctly,
**So that** text remains readable at any width.

**Requirements:** FR-302

**Acceptance Criteria:**

**AC 3.2.1: Initial word wrap**
- **Given** cclv starts
- **When** markdown renders
- **Then** it wraps to current terminal width
- **And** no horizontal scrolling needed

**AC 3.2.2: Resize detection**
- **Given** the user resizes their terminal
- **When** tea.WindowSizeMsg is received
- **Then** the new width is captured
- **And** re-render is triggered

**AC 3.2.3: Re-render on resize**
- **Given** terminal width changes
- **When** markdown content re-renders
- **Then** it uses the new width for word wrap
- **And** display updates smoothly

**AC 3.2.4: Renderer recreation**
- **Given** terminal width changes significantly
- **When** re-rendering is needed
- **Then** Glamour renderer is recreated with new width
- **And** WithWordWrap uses updated value

**Technical Notes:**
```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.renderer = m.createRenderer(msg.Width)
        m.invalidateRenderCache()
    }
}
```

---

### Story 3.3: Implement Render Caching

**As a** developer scrolling through logs,
**I want** smooth scroll performance,
**So that** markdown rendering doesn't cause lag.

**Requirements:** FR-303, NFR-001

**Acceptance Criteria:**

**AC 3.3.1: Cache rendered output**
- **Given** an assistant entry is rendered
- **When** the same entry is viewed again
- **Then** cached output is used
- **And** Glamour.Render is NOT called again

**AC 3.3.2: Cache keyed by entry**
- **Given** the render cache
- **When** storing rendered content
- **Then** it is keyed by entry ID or index
- **And** each entry has its own cache slot

**AC 3.3.3: Cache invalidation on resize**
- **Given** terminal is resized
- **When** width changes
- **Then** entire render cache is invalidated
- **And** next View() triggers fresh renders

**AC 3.3.4: Lazy rendering**
- **Given** a large log file
- **When** scrolling through entries
- **Then** only visible entries are rendered
- **And** off-screen entries remain uncached until viewed

**AC 3.3.5: Performance target**
- **Given** 10k entries loaded
- **When** scrolling rapidly
- **Then** framerate remains smooth (target 60 FPS)
- **And** no visible lag or stutter

**Technical Notes:**
```go
type model struct {
    renderCache map[int]string  // entry index -> rendered string
    cacheWidth  int             // width when cache was built
}

func (m *model) invalidateRenderCache() {
    m.renderCache = make(map[int]string)
    m.cacheWidth = m.width
}

func (m *model) getRenderedContent(idx int, content string) string {
    if cached, ok := m.renderCache[idx]; ok {
        return cached
    }
    rendered := m.renderAssistantText(content)
    m.renderCache[idx] = rendered
    return rendered
}
```

---

## FR Coverage Map

| Requirement | Epic | Story | Status |
|-------------|------|-------|--------|
| FR-101 | 1 | 1.1 | Done |
| FR-102 | 1 | 1.2 | Done |
| FR-103 | 1 | 1.3 | Done |
| FR-104 | 1 | 1.4 | Done |
| Retro-001 | 1.5 | 1.5 | Planned |
| Retro-002 | 1.5 | 1.6 | Planned |
| Retro-003 | 1.5 | 1.7 | Planned |
| Retro-004 | 1.5 | 1.8 | Planned |
| FR-201 | 2 | 2.1 | Planned |
| FR-202 | 2 | 2.2 | Planned |
| FR-203 | 2 | 2.3 | Planned |
| FR-301 | 3 | 3.1 | Planned |
| FR-302 | 3 | 3.2 | Planned |
| FR-303 | 3 | 3.3 | Planned |

---

## Implementation Order

**Recommended sequence:**

### Epic 1: Visual Polish (DONE)
1. ~~**Story 1.1** - Adaptive Color System~~ ✅
2. ~~**Story 1.2** - Rounded Border Styling~~ ✅
3. ~~**Story 1.3** - Segmented Status Bar~~ ✅
4. ~~**Story 1.4** - Spinner Animation~~ ✅

### Epic 1.5: Visual Consistency (NEXT)
5. **Story 1.5** - Research List View Polish Options (spike)
6. **Story 1.6** - Implement List View Polish (depends on 1.5)
7. **Story 1.7** - Adjust Spacing and Margins (independent)
8. **Story 1.8** - Implement Overlay Spinner (depends on 1.4 spinner patterns)

### Epic 2: Real-time File Watching
9. **Story 2.1** - Watch Mode CLI Flag (independent)
10. **Story 2.2** - fsnotify Integration (depends on 2.1, uses 1.4/1.8 spinner)
11. **Story 2.3** - Smart Auto-scroll (depends on 2.2, uses 1.3 status bar)

### Epic 3: Markdown Rendering
12. **Story 3.1** - Glamour Integration (can parallelize with Epic 2)
13. **Story 3.2** - Dynamic Word Wrap (depends on 3.1)
14. **Story 3.3** - Render Caching (depends on 3.1, 3.2)

---

## Next Steps

1. **Sprint Planning** - `/bmad:bmm:workflows:sprint-planning` - Regenerate sprint-status.yaml with Epic 1.5
2. **Create Story 1.5** - `/bmad:bmm:agents:sm` then `[CS]` - Prepare research spike for list view polish
3. **Execute Epic 1.5** - Complete all 4 stories for visual consistency
4. **Then Epic 2** - Real-time File Watching

---

*Epics & Stories - Initial: 2026-01-15 | Updated: 2026-01-15 (Epic 1.5 added from retrospective)*
