# Story 1.1: Implement Adaptive Color System

Status: in-progress

## Story

As a **developer using cclv**,
I want **colors to automatically adapt to my terminal's light/dark theme**,
So that **the UI is readable and visually consistent regardless of my terminal settings**.

## Acceptance Criteria

### AC 1.1.1: Theme struct creation
- **Given** the cclv codebase
- **When** I create a new Theme struct in styles.go
- **Then** it contains AdaptiveColor fields for: Primary, Secondary, Accent, Text, Muted, Dim, Background, BgAlt
- **And** it contains role-specific AdaptiveColor fields for: User, Assistant, Thinking, Tool
- **And** each color has both light and dark variants

### AC 1.1.2: Light theme adaptation
- **Given** a terminal with light background
- **When** cclv renders the UI
- **Then** text is dark and readable against the light background
- **And** accent colors are visible and distinct

### AC 1.1.3: Dark theme adaptation
- **Given** a terminal with dark background
- **When** cclv renders the UI
- **Then** text is light and readable against the dark background
- **And** accent colors are visible and distinct

### AC 1.1.4: No hardcoded colors
- **Given** the view rendering code
- **When** I search for hardcoded color values
- **Then** all colors reference the Theme struct (including core, text, background, and role colors)
- **And** no raw hex or ANSI codes exist in view code
- **And** all Styles and ListStyles use DefaultTheme color references

## Tasks / Subtasks

- [x] Task 1: Create Theme struct with AdaptiveColor fields (AC: 1.1.1)
  - [x] 1.1: Define Theme struct with Primary, Secondary, Accent, Text, Muted, Background fields
  - [x] 1.2: Add role-specific colors: User, Assistant, Thinking, Tool
  - [x] 1.3: Add UI element colors: BgAlt, Dim
  - [x] 1.4: Create DefaultTheme variable with light/dark color pairs

- [x] Task 2: Replace existing color variables with Theme colors (AC: 1.1.4)
  - [x] 2.1: Update Styles struct to use DefaultTheme colors
  - [x] 2.2: Replace primaryColor, secondaryColor, accentColor references
  - [x] 2.3: Replace textColor, mutedColor, dimColor references
  - [x] 2.4: Replace bgAltColor reference
  - [x] 2.5: Replace userColor, assistantColor, thinkingColor, toolColor references

- [x] Task 3: Update ListStyles to use Theme colors (AC: 1.1.4)
  - [x] 3.1: Update ListStyles.Header to use Theme.Primary
  - [x] 3.2: Update ListStyles.Separator to use Theme.Dim
  - [x] 3.3: Update ListStyles.Counter to use Theme.Muted
  - [x] 3.4: Update ListStyles.Loading to use Theme.Accent

- [ ] Task 4: Test on light and dark terminals (AC: 1.1.2, 1.1.3)
  - [ ] 4.1: Manual verification on dark terminal (iTerm2 dark theme)
  - [ ] 4.2: Manual verification on light terminal (iTerm2 light theme)
  - [ ] 4.3: Verify all message types (User, Assistant, Thinking, Tool) are readable
  - [ ] 4.4: Verify status bar and help text are readable

- [x] Task 5: Write unit tests for Theme struct (AC: 1.1.1)
  - [x] 5.1: Test that DefaultTheme is not nil and has all 14 color fields populated
  - [x] 5.2: Test that all AdaptiveColor fields have both Light and Dark values non-empty
  - [x] 5.3: Test core colors (Primary, Secondary, Accent) are present
  - [x] 5.4: Test role colors (User, Assistant, Thinking, Tool) are present

## Dev Notes

### Architecture Compliance

**CRITICAL**: Follow project-context.md rules exactly.

1. **File Location**: All changes go in `internal/tui/styles.go`
2. **No new files**: Refactor existing styles.go only
3. **No emoji**: Continue using text icons `[U]`, `[A]`, `[T]`, `[>]`
4. **Build with Make**: Use `make build` and `make test` - never raw go commands

### Technical Implementation

**lipgloss.AdaptiveColor API**:
```go
type AdaptiveColor struct {
    Light string  // Color for light terminal backgrounds
    Dark  string  // Color for dark terminal backgrounds
}
```

The terminal background is automatically detected at runtime. Use hex colors for consistency.

**Theme Struct Design**:
```go
type Theme struct {
    // Core colors
    Primary    lipgloss.AdaptiveColor
    Secondary  lipgloss.AdaptiveColor
    Accent     lipgloss.AdaptiveColor

    // Text colors
    Text       lipgloss.AdaptiveColor
    Muted      lipgloss.AdaptiveColor
    Dim        lipgloss.AdaptiveColor

    // Background
    Background lipgloss.AdaptiveColor
    BgAlt      lipgloss.AdaptiveColor

    // Role colors (semantic)
    User       lipgloss.AdaptiveColor
    Assistant  lipgloss.AdaptiveColor
    Thinking   lipgloss.AdaptiveColor
    Tool       lipgloss.AdaptiveColor
}
```

### Current Color Mapping (for migration reference)

Existing hardcoded colors to replace:

| Variable | Current Value | Purpose |
|----------|--------------|---------|
| primaryColor | `#7C3AED` (Purple) | Titles, primary UI elements |
| secondaryColor | `#10B981` (Green) | Subtitles |
| accentColor | `#F59E0B` (Amber) | Search highlights, loading |
| textColor | `#E5E7EB` (Light gray) | Primary text content |
| mutedColor | `#9CA3AF` (Muted gray) | Secondary text, collapsed indicators |
| dimColor | `#6B7280` (Dim gray) | Help text, timestamps |
| bgAltColor | `#374151` | Status bar background |
| userColor | `#3B82F6` (Blue) | User message styling |
| assistantColor | `#10B981` (Green) | Assistant message styling |
| thinkingColor | `#8B5CF6` (Purple) | Thinking block styling |
| toolColor | `#F59E0B` (Amber) | Tool use styling |

### Recommended Light/Dark Color Pairs

**For Light Theme** - Need darker colors for readability:

| Purpose | Light Theme Color | Dark Theme Color (current) |
|---------|------------------|---------------------------|
| Primary | `#5B21B6` (darker purple) | `#7C3AED` |
| Secondary | `#059669` (darker green) | `#10B981` |
| Accent | `#D97706` (darker amber) | `#F59E0B` |
| Text | `#1F2937` (dark gray) | `#E5E7EB` |
| Muted | `#6B7280` | `#9CA3AF` |
| Dim | `#9CA3AF` | `#6B7280` |
| User | `#2563EB` (darker blue) | `#3B82F6` |
| Assistant | `#059669` | `#10B981` |
| Thinking | `#7C3AED` | `#8B5CF6` |
| Tool | `#D97706` | `#F59E0B` |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/tui/styles.go` | Add Theme struct, DefaultTheme var, update Styles and ListStyles |
| `internal/tui/styles_test.go` | Add unit tests for Theme struct completeness (Task 5) |

### Files NOT to Modify

- `viewer.go` - Should require NO changes if styles.go is done correctly
- `app.go`, `project.go`, `conversation.go` - Same, styles are referenced through the global Styles variable

### Testing Strategy

1. **Unit Tests**: Add `styles_test.go` with tests for Theme completeness
2. **Manual Testing**: Essential - must verify visual appearance on both light and dark terminals
3. **Coverage**: Styles code doesn't need high coverage (visual), focus on struct completeness tests

### Common Pitfalls to Avoid

1. **DON'T** change Styles struct field types - they must remain `lipgloss.Style`
2. **DON'T** add imports to viewer.go or other files - all color logic stays in styles.go
3. **DON'T** modify View() methods - they should work unchanged
4. **DON'T** use ANSI codes - use hex colors for AdaptiveColor consistency
5. **DON'T** forget to update ListStyles - it also has hardcoded colors

### Project Structure Notes

- **Alignment**: Changes confined to `internal/tui/styles.go` per existing structure
- **No conflicts**: This is a refactor of existing code, no new modules

### References

- [Source: _bmad-output/project-context.md#Technology Stack] - Lipgloss v1.1.1
- [Source: _bmad-output/project-context.md#Styling Rules] - NO EMOJI, use Lipgloss for ALL styling
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.1] - Acceptance criteria
- [Source: _bmad-output/planning-artifacts/prd.md#FR-101] - Adaptive Color System requirements
- [Source: pkg.go.dev/lipgloss#AdaptiveColor] - API documentation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

### Completion Notes List

1. **Theme struct created** with 14 AdaptiveColor fields (12 adaptive + 2 constant for text on colored backgrounds)
2. **DefaultTheme variable** initialized with light/dark color pairs per recommended mapping in Dev Notes
3. **All existing color variables** now reference DefaultTheme adaptive colors
4. **Styles struct unchanged** - uses same variable names which now resolve to adaptive colors
5. **ListStyles unchanged** - same pattern, references adaptive color variables
6. **Two hardcoded hex values replaced** with whiteColor and blackColor (Selected foreground, SearchMatch foreground)
7. **Unit tests created** in `styles_test.go` with table-driven tests for all color categories
8. **All tests pass** including race detection
9. **Build succeeds** with `make build`
10. **Task 4 (manual verification) pending** - requires user to verify on light/dark terminal themes
11. **[Code Review Fix]** Renamed ContrastLight/ContrastDark to White/Black for clarity (constant colors, not adaptive)

### File List

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/styles.go` | Modified | Added Theme struct, DefaultTheme, refactored color variables to use adaptive colors |
| `internal/tui/styles_test.go` | Created | Added unit tests for Theme struct completeness |
