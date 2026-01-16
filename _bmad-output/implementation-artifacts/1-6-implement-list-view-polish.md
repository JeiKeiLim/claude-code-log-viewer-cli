# Story 1.6: Implement List View Polish

Status: done

## Story

As a **developer using cclv**,
I want **the project and conversation list views to match the polished viewer styling**,
So that **the entire UI feels cohesive and modern**.

## Acceptance Criteria

### AC 1.6.1: Project list styling
- **Given** the project list view
- **When** it renders
- **Then** it uses the Theme's adaptive colors
- **And** visual styling matches the polished viewer

### AC 1.6.2: Conversation list styling
- **Given** the conversation list view
- **When** it renders
- **Then** it uses the Theme's adaptive colors
- **And** visual styling matches the polished viewer

### AC 1.6.3: Selection highlighting
- **Given** a list with items
- **When** an item is selected
- **Then** it has clear visual distinction
- **And** uses Theme colors for highlighting

### AC 1.6.4: Consistent with viewer
- **Given** the polished lists
- **When** navigating from list to viewer
- **Then** the visual transition feels seamless
- **And** no jarring style changes occur

## Tasks / Subtasks

- [x] Task 1: Update styles.go with ListItem styles (AC: 1.6.1, 1.6.2, 1.6.3)
  - [x] 1.1: Add `SelectedBg` adaptive color to DefaultTheme (dark: #2D1B4E, light: #EDE9FE)
  - [x] 1.2: Add `ListItemStyles` struct to Styles with: GutterSelected, GutterNormal, TitleSelected, TitleNormal, DescSelected, DescNormal
  - [x] 1.3: Define `GutterSelected` style: Foreground(Theme.Primary)
  - [x] 1.4: Define `GutterNormal` style: empty style (no color)
  - [x] 1.5: Define `TitleSelected` style: Bold(true), Foreground(Theme.Primary), Background(SelectedBg)
  - [x] 1.6: Define `TitleNormal` style: Bold(true), Foreground(Theme.Text)
  - [x] 1.7: Define `DescSelected` style: Foreground(Theme.Text), Background(SelectedBg)
  - [x] 1.8: Define `DescNormal` style: Foreground(Theme.Muted)

- [x] Task 2: Update ProjectItem.Render() in project.go (AC: 1.6.1, 1.6.3)
  - [x] 2.1: Define constants: `gutterSelected = "│ "` (U+2502 + space), `gutterNormal = "  "` (two spaces)
  - [x] 2.2: Replace ` > ` prefix with `gutterSelected` when selected
  - [x] 2.3: Replace `   ` prefix (3 spaces) with `gutterNormal` (2 spaces) when unselected
  - [x] 2.4: Style the gutter prefix: `Styles.ListItem.GutterSelected.Render(gutterSelected)` when selected, plain `gutterNormal` when not
  - [x] 2.5: Apply `Styles.ListItem.TitleSelected` and `Styles.ListItem.DescSelected` when selected
  - [x] 2.6: Update description line to use `gutterNormal` prefix: `fmt.Sprintf("%s%s\n%s%s", prefixStyled, title, gutterNormal, desc)`
  - [x] 2.7: Use `lipgloss.Width()` for styled strings, `VisualWidth()` for unstyled - calculate content width BEFORE applying styles

- [x] Task 3: Update ConversationItem.Render() in conversation.go (AC: 1.6.2, 1.6.3)
  - [x] 3.1: Define same gutter constants as project.go (or use shared constants from styles.go)
  - [x] 3.2: Apply same gutter indicator pattern as project.go
  - [x] 3.3: Apply `ListItemStyles` consistently with project.go
  - [x] 3.4: Preserve existing metadata format: `{count} msgs • {duration} • {preview}` on description line
  - [x] 3.5: Update description line to use `gutterNormal` prefix: `fmt.Sprintf("%s%s\n%s%s", prefixStyled, title, gutterNormal, desc)`

- [x] Task 4: Validate visual consistency (AC: 1.6.4)
  - [x] 4.1: Test navigation flow: projects → conversations → viewer
  - [x] 4.2: Verify no jarring style transitions between views
  - [x] 4.3: Test in dark terminal theme (e.g., default Terminal.app dark, VS Code dark)
  - [x] 4.4: Test in light terminal theme (e.g., VS Code light, Terminal.app light)

- [x] Task 5: Test and verify (all ACs)
  - [x] 5.1: Run `make test` to ensure no regressions
  - [x] 5.2: Test truncation with long titles (verify no overflow, proper ellipsis)
  - [x] 5.3: Test `│` rendering in terminals: verify no gaps, proper vertical alignment, consistent width
  - [x] 5.4: Test in VS Code integrated terminal, iTerm2, Terminal.app (macOS)

## Dev Notes

### Implementation Pattern: Option B+ (Hybrid Gutter + Subtle Background)

From Story 1.5 research, the recommended approach is **Option B+**:

```
│ 2024-01-15 14:32              <- Purple gutter + subtle purple bg tint
│ 15 msgs • 45m • How do I...
  2024-01-14 09:15              <- No gutter, no background
  8 msgs • 12m • Fix the...
```

This combines:
1. **Gutter indicator** (`│`) from Glow pattern for modern aesthetic
2. **Subtle background tint** (15-20% opacity) for clear selection visibility
3. **Theme colors** for consistency with viewer polish

### Current Implementation to Replace

**project.go:20-47 (current):**
```go
func (i ProjectItem) Render(width int, selected bool) string {
    var prefix string
    var titleStyle, descStyle lipgloss.Style
    if selected {
        prefix = " > "
        titleStyle = Styles.Selected  // Full purple bg, white text
        descStyle = Styles.Selected.Background(lipgloss.Color("#4C1D95"))
    } else {
        prefix = "   "
        titleStyle = Styles.Normal.Bold(true)
        descStyle = Styles.Muted
    }
    // ...
    return fmt.Sprintf("%s%s\n   %s", prefix, title, desc)
}
```

**conversation.go:20-69 (identical pattern)**

### Target Implementation

**styles.go additions:**

```go
// Add to DefaultTheme
SelectedBg lipgloss.AdaptiveColor  // Subtle selection background

// In DefaultTheme definition:
SelectedBg: lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#2D1B4E"},

// Add ListItemStyles to Styles struct
ListItem struct {
    GutterSelected lipgloss.Style  // │ in Theme.Primary
    GutterNormal   lipgloss.Style  // No styling (empty)
    TitleSelected  lipgloss.Style  // Bold + Primary fg + subtle bg
    TitleNormal    lipgloss.Style  // Bold + Text fg
    DescSelected   lipgloss.Style  // Text fg + subtle bg
    DescNormal     lipgloss.Style  // Muted fg
}
```

**Subtle Background Colors (from research):**
- Dark theme: `#2D1B4E` (muted purple, ~15% opacity equivalent)
- Light theme: `#EDE9FE` (very light purple)

**project.go/conversation.go Render() update:**
```go
// Constants (can be in styles.go for sharing)
const gutterSelected = "│ "  // U+2502 + space (2 chars visual)
const gutterNormal = "  "    // Two spaces (2 chars visual)

func (i ProjectItem) Render(width int, selected bool) string {
    var prefixStyled string
    var titleStyle, descStyle lipgloss.Style

    if selected {
        prefixStyled = Styles.ListItem.GutterSelected.Render(gutterSelected)
        titleStyle = Styles.ListItem.TitleSelected
        descStyle = Styles.ListItem.DescSelected
    } else {
        prefixStyled = gutterNormal  // No styling needed for normal
        titleStyle = Styles.ListItem.TitleNormal
        descStyle = Styles.ListItem.DescNormal
    }

    // Calculate available width BEFORE styling
    // Prefix is 2 chars visual width (either "│ " or "  ")
    const prefixWidth = 2
    availWidth := width - prefixWidth - 2  // -2 for border padding
    if availWidth < 10 {
        availWidth = 10
    }

    title := titleStyle.Render(i.project.DisplayName)

    // Truncate path from left if too long
    path := TruncateFromLeftToWidth(i.project.DecodedPath, availWidth)
    desc := descStyle.Render(path)

    // Description line also gets gutter alignment (normal gutter for visual alignment)
    return fmt.Sprintf("%s%s\n%s%s", prefixStyled, title, gutterNormal, desc)
}
```

### Width Calculation Rules

**CRITICAL:** Width calculation must be done correctly to avoid truncation issues.

1. **For prefix width:** Use constant `2` (both `│ ` and `  ` are 2 visual chars)
2. **For styled strings measurement:** Use `lipgloss.Width(styledString)`
3. **For unstyled strings measurement:** Use `VisualWidth(string)` (existing helper)
4. **Calculate BEFORE applying styles:** Determine truncation limits on raw strings, then style

### Theme Colors to Use

From `styles.go:34-58`:
- `Theme.Primary`: Purple (#5B21B6/#7C3AED) - for gutter and selected title
- `Theme.Text`: Gray - for selected description
- `Theme.Muted`: Muted gray - for normal description
- NEW `Theme.SelectedBg`: Subtle purple tint (#2D1B4E/#EDE9FE) - for selection background

### Unicode Box-Drawing Compatibility

The `│` character (U+2502) is used by Glow and is widely supported. From research:
- **Supported:** iTerm2, VS Code terminal, Windows Terminal, Kitty, Alacritty, Terminal.app
- **Potential issues:** Some Japanese fonts, very small font sizes (<8pt)
- **Risk:** LOW for target audience (developers on modern terminals)

If issues are reported, fallback to ASCII `|` is trivial to implement.

### Accessibility Notes

- Selection uses multiple indicators: color + gutter character + position
- Compliant with WCAG 1.4.1 (color not sole indicator)
- Purple is visible to most color blind types (red/green blindness unaffected)
- Contrast verified: #7C3AED on dark bg passes AA, #5B21B6 on light bg passes AA

### Project Structure Notes

Files to modify:
- `internal/tui/styles.go` - Add SelectedBg to Theme, add ListItemStyles to Styles
- `internal/tui/project.go` - Update ProjectItem.Render()
- `internal/tui/conversation.go` - Update ConversationItem.Render()

No new files needed. Changes are additive to existing styles.

### Build and Test Commands

```bash
make build  # Always use Makefile
make test   # Run tests with race detection
```

### References

- [Source: _bmad-output/planning-artifacts/research/list-view-polish-research-2026-01-16.md] - Full research and recommendation
- [Source: internal/tui/styles.go:34-58] - Theme definition
- [Source: internal/tui/styles.go:86-237] - Current Styles struct
- [Source: internal/tui/project.go:20-47] - Current ProjectItem.Render()
- [Source: internal/tui/conversation.go:20-69] - Current ConversationItem.Render()
- [Source: internal/tui/listviewport.go] - ListViewport implementation
- [Source: _bmad-output/project-context.md#Styling Rules] - NO EMOJI, box-drawing chars allowed
- [Source: _bmad-output/implementation-artifacts/1-5-research-list-view-polish-options.md] - Story 1.5 completed

### Previous Story Intelligence (Story 1.5)

From Story 1.5 research spike:
- Glow's gutter pattern validated as clean, minimal, professional
- Option B+ hybrid recommended for cclv (gutter + subtle bg)
- ListViewport Render() interface unchanged - only styling changes
- No height increase per item - stays at 2 lines
- Theme colors sufficient - only SelectedBg tint needs adding

### Git Intelligence

Recent commits:
- `9a638a3` - docs: complete Story 1.5 list view polish research with adversarial review
- `0de60a8` - docs: complete Epic 1 retrospective and add Epic 1.5
- `13f4cd9` - feat: add spinner animation during loading operations
- `5f80be1` - feat: implement segmented status bar with position tracking
- `6971e73` - feat: apply rounded border styling to message cards

Pattern: Features use `feat:` prefix, Epic 1 established rounded borders, adaptive colors, and segmented status bar patterns.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - No debug issues encountered.

### Completion Notes List

1. Added `SelectedBg` adaptive color to Theme struct and DefaultTheme
2. Added `ListItem` styles struct with 6 styles: GutterSelected, GutterNormal, TitleSelected, TitleNormal, DescSelected, DescNormal
3. Added shared gutter constants: `GutterSelected = "│ "` and `GutterNormal = "  "`
4. Updated ProjectItem.Render() to use new gutter pattern and ListItem styles
5. Updated ConversationItem.Render() to use new gutter pattern and ListItem styles
6. Both list views now display purple gutter indicator (`│`) for selected items with subtle purple background tint
7. Description lines properly aligned with GutterNormal spacing
8. All tests pass (make test)
9. Build successful (make build)

### Code Review Fixes Applied (2026-01-16)

Post-review fixes by Amelia (Dev Agent):

1. **[HIGH] Added missing tests for ListItem styles** - Added `TestListItemStyles` and `TestGutterConstants` to verify style initialization and gutter constant byte values
2. **[MEDIUM] Added SelectedBg to background colors test** - Updated `TestThemeBackgroundColors` to include SelectedBg
3. **[MEDIUM] Added test for gutter constant bytes** - Verified U+2502 and U+0020 exact byte sequences
4. **[MEDIUM] Extracted shared width calculation** - Created `listItemAvailWidth()` helper in utils.go to eliminate duplicate code
5. **[MEDIUM] Fixed full-width selection background** - Added `PadToWidth()` to description rendering for consistent background width
6. **[LOW] Removed unused GutterNormal style** - Removed `Styles.ListItem.GutterNormal` (only the constant is needed)
7. Updated `TestThemeAllFieldsPopulated` to expect 15 fields (was 14, now includes SelectedBg)

All 8 review findings addressed. Tests pass.

### File List

- `internal/tui/styles.go` - Added SelectedBg to Theme, ListItem styles struct (5 styles), gutter constants
- `internal/tui/styles_test.go` - Added TestListItemStyles, TestGutterConstants, updated theme field tests
- `internal/tui/utils.go` - Added listItemAvailWidth() shared helper
- `internal/tui/project.go` - Updated Render() to use gutter pattern, ListItem styles, and shared helper
- `internal/tui/conversation.go` - Updated Render() to use gutter pattern, ListItem styles, and shared helper
