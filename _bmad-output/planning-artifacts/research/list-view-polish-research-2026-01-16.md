# List View Polish Research - cclv Phase 2

**Date:** 2026-01-16
**Story:** 1.5 - Research List View Polish Options
**Status:** Complete

## Executive Summary

**Recommended Approach: Option B - Gutter Indicator with Enhanced Selection**

This approach combines Glow's clean gutter indicator pattern with cclv's existing color system. It provides clear visual hierarchy, minimal vertical space usage, and seamless integration with the current ListViewport implementation.

Key benefits:
- Clear visual selection without increasing item height
- Matches Charm ecosystem patterns (familiar to users of glow, soft-serve)
- Works within existing 2-line item constraint
- Uses current Theme colors with no additions needed

## Research Methodology

### Tools/Apps Studied

1. **charm/glow** - Markdown reader with file browser (stash view)
2. **charm/soft-serve** - Git server TUI (file/repo lists)
3. **charmbracelet/bubbles** - Default list delegate implementation
4. **Current cclv** - Existing project/conversation list styling

### Evaluation Criteria

1. Visual clarity of selection state
2. Vertical space efficiency (2-line item constraint)
3. Integration with existing ListViewport
4. Consistency with viewer polish (rounded borders, adaptive colors)
5. Implementation complexity

## Pattern Analysis

### Pattern 1: Gutter Indicator (Glow)

**Source:** charm/glow `ui/stashitem.go`

**Visual description:**
- Vertical line gutter on left side of selected item
- Selected: `│` (U+2502) prefix
- Unselected: ` ` (space) prefix
- Color changes for selected (fuchsia/green) vs normal (muted)
- Two-line items with icon, separator, title on line 1; date on line 2

**Implementation approach:**
```
│ 2024-01-15 14:32                    <- Selected (colored gutter + bright text)
│ 15 msgs • 45m • How do I implement...
  2024-01-14 09:15                    <- Normal (no gutter, muted text)
  8 msgs • 12m • Fix the bug in...
```

**Pros for cclv:**
- Clean, minimal visual change
- No extra height per item
- Familiar pattern in Charm ecosystem
- Easy to implement with current Render() signature

**Cons for cclv:**
- Subtler than current selection (no background)
- May need color tuning for visibility

---

### Pattern 2: Background Selection with Border Accent (Bubbles Default)

**Source:** charmbracelet/bubbles `list/defaultitem.go`

**Visual description:**
- Selected items have colored background + border accent
- Magenta border on left side (#EE6FF8)
- Title in bright color, description inherits
- Dimmed state during filtering

**Implementation approach:**
```
▌ 2024-01-15 14:32                   <- Selected (border accent + bg)
▌ 15 msgs • 45m • How do I implement...
  2024-01-14 09:15                   <- Normal (left padding only)
  8 msgs • 12m • Fix the bug in...
```

**Pros for cclv:**
- Very high visibility selection
- Familiar bubbles/list pattern
- Filter match highlighting built-in

**Cons for cclv:**
- More complex styling with background
- May clash with outer list border

---

### Pattern 3: Delegate with Injected Styles (Soft-serve)

**Source:** charm/soft-serve `pkg/ui/components/selector/selector.go`

**Visual description:**
- Clean separation of state logic from presentation
- ItemDelegate interface handles all rendering
- Styles injected via common config object
- Minimal selector component, delegate does heavy lifting

**Implementation approach:**
- Not a visual pattern per se, but an architecture pattern
- cclv already uses similar approach with ListItem interface

**Relevance for cclv:**
- Confirms current architecture is sound
- Validates Render(width, selected) interface approach

---

### Pattern 4: Current cclv Implementation

**Source:** `internal/tui/project.go`, `internal/tui/conversation.go`

**Current styling:**
```go
if selected {
    prefix = " > "
    titleStyle = Styles.Selected  // Purple bg, white text, bold
    descStyle = Styles.Selected.Background("#4C1D95")
} else {
    prefix = "   "
    titleStyle = Styles.Normal.Bold(true)
    descStyle = Styles.Muted
}
```

**Current visual:**
```
 > 2024-01-15 14:32          <- Selected (purple bg, "> " prefix)
   15 msgs • 45m • How do...
   2024-01-14 09:15          <- Normal (no bg, "   " prefix)
   8 msgs • 12m • Fix the...
```

**Issues identified:**
- " > " arrow doesn't align with viewer polish style
- Purple background spans full width (can feel heavy)
- No visual connection to rounded-border viewer cards
- Inconsistent with the polished viewer aesthetic

## Prototypes

### Option A: Card-Style Items (Full Border Per Item)

```
┌────────────────────────────────────────┐
│ > 2024-01-15 14:32                     │  <- Selected (purple border)
│   15 msgs • 45m • How do I implement...│
└────────────────────────────────────────┘
  2024-01-14 09:15                          <- Normal (no border)
  8 msgs • 12m • Fix the bug in...

  2024-01-13 16:45
  3 msgs • 5m • Quick question about...
```

**Pros:**
- Matches viewer message card style
- Very clear selection state
- Consistent visual language with viewer

**Cons:**
- Adds 2 lines per item (top/bottom border)
- Significantly reduces visible items
- Heavy visual weight for list context
- May feel "boxy" for navigation

**Verdict: Not recommended** - Too much vertical space consumption

---

### Option B: Gutter Indicator with Enhanced Selection (RECOMMENDED)

```
│ 2024-01-15 14:32                        <- Selected (purple gutter, bright text)
│ 15 msgs • 45m • How do I implement...
  2024-01-14 09:15                        <- Normal (no gutter, standard text)
  8 msgs • 12m • Fix the bug in...
  2024-01-13 16:45
  3 msgs • 5m • Quick question about...
```

**Implementation details:**
- Selected: `│ ` (vertical line + space) prefix
- Unselected: `  ` (two spaces) prefix
- Selected title: `Styles.Selected` (purple bg removed, keep bold + primary color)
- Selected desc: `Styles.Normal` with accent color
- Gutter colored with `Theme.Primary`

**Pros:**
- No extra height per item
- Clean, minimal, professional
- Matches Glow's proven pattern
- Easy implementation (prefix change only)
- Works with current ListViewport

**Cons:**
- Subtler than current heavy selection
- Users may need adjustment period

**Verdict: Recommended** - Best balance of polish and practicality

---

### Option C: Alternating Row Backgrounds (Zebra Striping)

```
  2024-01-15 14:32              <- Row 0 (BgAlt background)
  15 msgs • 45m • How do I...
  ────────────────────────────
  2024-01-14 09:15              <- Row 1 (no background)
  8 msgs • 12m • Fix the...
  ────────────────────────────
> 2024-01-13 16:45              <- Row 2 + Selected (BgAlt + purple accent)
  3 msgs • 5m • Quick...
```

**Pros:**
- Clear row separation
- Common pattern in table UIs
- Helps track across wide content

**Cons:**
- Busy visual with existing outer border
- Selection highlighting competes with stripes
- Harder to implement (needs row index tracking in Render)
- Not typical in Charm ecosystem

**Verdict: Not recommended** - Too busy, complicates implementation

---

### Option D: Minimal Enhancement (Current + Refinement)

```
 > 2024-01-15 14:32               <- Selected (keep current, refine colors)
   15 msgs • 45m • How do I...
   2024-01-14 09:15               <- Normal (unchanged)
   8 msgs • 12m • Fix the...
```

**Changes:**
- Keep " > " prefix
- Reduce purple background width (not full width)
- Add subtle left border instead of arrow

**Pros:**
- Minimal code change
- Users already familiar

**Cons:**
- Doesn't address visual inconsistency with viewer
- Still feels utilitarian vs polished
- Arrow prefix is dated pattern

**Verdict: Acceptable fallback** - If Option B rejected

## Recommendation

### Selected Approach: Option B - Gutter Indicator with Enhanced Selection

**Rationale:**

1. **Matches viewer polish level** - Clean lines, not heavy backgrounds
2. **Proven pattern** - Used successfully in Glow, familiar to Charm users
3. **Zero height increase** - Stays within 2-line item constraint
4. **Simple implementation** - Prefix change + style adjustment only
5. **Consistent with modern TUI aesthetics** - Subtle, professional

### Implementation Guidance for Story 1.6

**Files to modify:**
1. `internal/tui/project.go` - ProjectItem.Render()
2. `internal/tui/conversation.go` - ConversationItem.Render()
3. `internal/tui/styles.go` - Add ListItem styles (optional)

**Specific changes:**

1. **Update prefix logic:**
```go
// Before
if selected {
    prefix = " > "
} else {
    prefix = "   "
}

// After
if selected {
    prefix = "│ "  // Vertical line (U+2502) + space
} else {
    prefix = "  "  // Two spaces
}
```

2. **Refine selection styling:**
```go
// Before
titleStyle = Styles.Selected  // Purple bg, white text

// After (Option B)
if selected {
    titleStyle = Styles.Normal.Foreground(Theme.Primary).Bold(true)
    descStyle = Styles.Normal.Foreground(Theme.Text)
    prefixStyle = lipgloss.NewStyle().Foreground(Theme.Primary)
} else {
    titleStyle = Styles.Normal.Bold(true)
    descStyle = Styles.Muted
    prefixStyle = lipgloss.NewStyle()
}
prefix = prefixStyle.Render(prefixChar)
```

3. **Optional: Add to Styles struct:**
```go
ListItem struct {
    SelectedPrefix lipgloss.Style
    SelectedTitle  lipgloss.Style
    SelectedDesc   lipgloss.Style
    NormalTitle    lipgloss.Style
    NormalDesc     lipgloss.Style
}
```

**Integration with existing Theme:**
- Use `Theme.Primary` for gutter color
- Use `Theme.Text` for selected description
- Use `Theme.Muted` for normal description
- No new colors needed

**Constraints respected:**
- No emoji (using box-drawing character U+2502)
- Existing Theme palette only
- ListViewport compatible (Render interface unchanged)
- 2 lines per item maintained

### Alternative: Option D Fallback

If stakeholder feedback prefers keeping the arrow indicator, implement Option D with these refinements:
- Change " > " to " * " or " + " for less dated appearance
- Reduce background color intensity
- Add left border to selected item container

---

## Adversarial Review Addendum

**Review Date:** 2026-01-16
**Reviewer:** Dev Agent (Adversarial Mode)

This section documents issues identified during adversarial review and additional research conducted to address gaps.

### Issues Identified

#### ISSUE 1: Glow Source Verified - Additional Context Needed

**Original concern:** Claims to study Glow but no evidence of actual code review.

**Verification performed:** Fetched actual `charm/glow ui/stashitem.go` from GitHub.

**Verified findings:**
- Glow uses `│` (U+2502) vertical line as gutter - CONFIRMED
- Glow applies **different colors per state**:
  - Normal selected: Dull fuchsia/magenta gutter + fuchsia title
  - Stashing selected: Green gutter + green title
  - Unselected: Space prefix, dimmed text
- Glow items include **icons and separators** (e.g., file icon + `·` separator)
- Two-line structure: Line 1 = gutter + icon + separator + title, Line 2 = gutter + date

**Impact on cclv:** The gutter pattern is validated, but cclv lacks icons/separators that provide additional visual structure in Glow. This makes the gutter more "orphaned" in cclv's plain-text items.

**Mitigation for Story 1.6:** Consider adding a subtle separator character (e.g., `·`) between metadata elements to provide more visual rhythm.

---

#### ISSUE 2: Unicode Box-Drawing Character Compatibility

**Original concern:** U+2502 rendering untested across terminals.

**Research findings from [Box-drawing characters - Wikipedia](https://en.wikipedia.org/wiki/Box-drawing_characters) and [Microsoft Terminal Issues](https://github.com/microsoft/terminal/issues/577):**

**Known problems:**
- U+2502 may not fill line height properly in some fonts (gap issues)
- Japanese fonts may render box-drawing characters incorrectly or make them disappear
- Small font sizes (< 8pt in Consolas) can make light-width characters invisible
- Font hinting/fitting causes alignment issues at various zoom levels

**Modern terminal solutions:**
- Kitty generates box-drawing characters programmatically (first to do so)
- Windows Terminal now fills cells properly at all zoom levels
- Most modern terminals (iTerm2, Windows Terminal, Kitty, Alacritty) handle U+2502 correctly

**Risk assessment:** LOW for modern terminals, MEDIUM for legacy/SSH environments.

**Mitigation for Story 1.6:**
1. Document terminal requirements (modern terminal with Unicode support)
2. Test in: macOS Terminal.app, iTerm2, VS Code integrated terminal, Windows Terminal
3. Fallback: If issues reported, provide config option to use ASCII `|` instead

---

#### ISSUE 3: Color Contrast and Visibility

**Original concern:** Purple gutter visibility unverified in dark/light themes.

**Research findings from [WCAG Contrast Guidelines](https://webaim.org/articles/contrast/) and [Section508.gov](https://www.section508.gov/create/making-color-usage-accessible/):**

**WCAG Requirements:**
- Normal text: 4.5:1 contrast ratio minimum (AA)
- Large text/UI components: 3:1 contrast ratio minimum
- Level AAA: 7:1 for normal text

**cclv Theme colors to verify:**
| Color | Hex (Dark) | Hex (Light) | Use |
|-------|------------|-------------|-----|
| Primary | #7C3AED | #5B21B6 | Gutter, selected text |
| Text | Light gray | Dark gray | Selected description |
| Muted | Muted gray | Muted gray | Normal description |

**Manual contrast checks needed:**
- #7C3AED on #1a1a1a (dark bg) - likely passes AA (bright purple on dark)
- #5B21B6 on #ffffff (light bg) - likely passes AA (dark purple on white)
- #7C3AED on #4C1D95 (purple-on-purple) - FAILS (this was old selection bg)

**Mitigation for Story 1.6:**
1. Use [WebAIM Contrast Checker](https://webaim.org/resources/contrastchecker/) to verify exact ratios before implementation
2. Ensure gutter color is NEVER same hue as background
3. Consider using `Theme.Secondary` (green) as alternative for higher contrast

---

#### ISSUE 4: Removing Background May Hurt Selection Clarity

**Original concern:** Proposed change removes solid purple background.

**Analysis:**

Current selection: **VERY HIGH visibility**
- Full-width purple background (#7C3AED)
- White text on purple
- Impossible to miss

Proposed selection: **MODERATE visibility**
- Single `│` character in purple
- Bold text in primary color
- No background differentiation

**Research insight from Glow:**
Glow uses colored text (fuchsia/green) WITHOUT background for selected items. However, Glow has:
- More visual structure (icons, separators)
- Single-purpose UI (file browser)
- Users familiar with the pattern

**Risk:** Users accustomed to cclv's current "heavy" selection may find gutter-only selection unclear.

**Mitigation for Story 1.6:**
1. **Hybrid approach (REVISED RECOMMENDATION):** Keep subtle background tint + add gutter
   ```
   │ 2024-01-15 14:32              <- Gutter + light purple tint bg
   │ 15 msgs • 45m • How do I...
   ```
2. Background: Use 10-20% opacity of Primary, not full saturation
3. This preserves both: modern gutter aesthetic + clear selection state

---

#### ISSUE 5: Accessibility - Color Blindness and Screen Readers

**Original concern:** Zero accessibility consideration.

**Research findings from [Color Blind Accessibility Guidelines](https://www.a11y-collective.com/blog/color-blind-accessibility-guidelines/) and [Section508.gov](https://www.section508.gov/create/making-color-usage-accessible/):**

**Color blindness considerations:**
- Purple (#7C3AED) is generally visible to most color blind types
- Red/green color blindness (most common) does NOT affect purple perception
- Blue/yellow blindness (tritanopia) may see purple differently but still distinguishable

**WCAG 1.4.1 requirement:**
> "Color is not used as the only visual means of conveying information"

**Current cclv compliance:**
- Selection indicated by: color change + prefix change (` > `) + position
- Multiple indicators = compliant

**Proposed Option B compliance:**
- Selection indicated by: color change + gutter character (`│`) + position
- Still multiple indicators = compliant

**Screen reader considerations:**
- Box-drawing characters may be announced as "box drawings light vertical" or skipped
- The `│` character provides no semantic meaning to screen readers
- However, TUI apps generally have limited screen reader support

**Mitigation for Story 1.6:**
1. Ensure selection is NEVER indicated by color alone (keep structural indicator)
2. Document that cclv is visual-first TUI, limited screen reader support
3. Purple is a safe color choice for color blindness

---

#### ISSUE 6: Width Calculation Risk

**Original concern:** Styled prefix may affect content width calculation.

**Analysis of `listviewport.go`:**
The `Render(width int, selected bool)` interface passes total available width. The implementation must account for prefix characters.

**Current code (project.go):**
```go
prefix = " > "  // 3 characters
// Title truncated to: width - len(prefix)
```

**Proposed code:**
```go
prefix = "│ "   // 2 characters (styled separately)
// Title truncated to: width - 2
```

**Potential issue:** If prefix is styled with ANSI codes, `len(prefix)` will include escape sequences, causing over-truncation.

**Mitigation for Story 1.6:**
1. Use `lipgloss.Width()` not `len()` for styled strings
2. Calculate content width BEFORE applying styles
3. Test truncation with long titles to verify

---

#### ISSUE 7: No User Testing

**Status:** Acknowledged limitation.

**Mitigation:** This is a research spike, not final implementation. Story 1.6 implementation can gather user feedback. Consider:
1. Screenshot before/after comparison in PR
2. Request feedback from users in release notes
3. Be prepared to iterate based on feedback

---

#### ISSUE 8: Option D Dismissed Hastily

**Reconsideration:**

The current ` > ` prefix was likely chosen intentionally for universal clarity. "Dated" is subjective.

**Arguments FOR keeping current style refined:**
- Zero learning curve
- Works in any terminal, any font
- Universally understood selection indicator
- The "problem" is aesthetic, not functional

**Revised position:** Option D remains valid fallback. If user feedback after 1.6 is negative, revert to refined Option D.

---

#### ISSUE 9: No Rollback Plan

**Mitigation for Story 1.6:**
1. Keep current implementation in git history (trivial to revert)
2. Single commit for list styling changes (easy to cherry-pick out)
3. No feature flag needed - changes are pure cosmetic
4. Success metric: No user complaints within 2 releases

---

#### ISSUE 10: Soft-Serve Architecture Confirmed

**Verification performed:** Fetched `charm/soft-serve pkg/ui/components/selector/selector.go`.

**Findings:**
- Uses delegate pattern wrapping `list.ItemDelegate`
- Styling injected via common styles system
- Confirms cclv's `Render(width, selected)` interface is architecturally sound
- Actual visual styling is NOT in selector.go - delegated to ItemDelegate implementations

**Impact:** Validates cclv's current architecture. No changes needed.

---

### Revised Recommendation

Based on adversarial review, the recommendation is **MODIFIED**:

#### Primary: Option B+ (Hybrid Gutter + Subtle Background)

```
│ 2024-01-15 14:32              <- Purple gutter + 15% purple bg tint
│ 15 msgs • 45m • How do I...
  2024-01-14 09:15              <- No gutter, no background
  8 msgs • 12m • Fix the...
```

**Changes from original Option B:**
1. ADD subtle background tint (not full saturation) to selected items
2. KEEP gutter indicator for modern aesthetic
3. REMOVE the heavy full-width purple background
4. VERIFY contrast ratios before implementation

**Implementation adjustments:**
```go
if selected {
    prefix = "│ "
    prefixStyle = lipgloss.NewStyle().Foreground(Theme.Primary)
    // Add subtle bg: 15% opacity approximation
    titleStyle = Styles.Normal.
        Foreground(Theme.Primary).
        Background(lipgloss.Color("#2D1B4E")).  // Dark theme: muted purple
        Bold(true)
    descStyle = Styles.Normal.
        Background(lipgloss.Color("#2D1B4E"))
} else {
    prefix = "  "
    titleStyle = Styles.Normal.Bold(true)
    descStyle = Styles.Muted
}
```

**For light theme:**
```go
Background(lipgloss.Color("#EDE9FE"))  // Light theme: very light purple
```

#### Fallback: Option D (Refined Current)

If hybrid approach receives negative feedback:
- Keep ` > ` prefix
- Reduce background saturation
- This is low-risk, proven approach

---

### Pre-Implementation Checklist for Story 1.6

Before implementing, Story 1.6 must:

- [ ] Verify #7C3AED contrast ratio on dark backgrounds using [WebAIM](https://webaim.org/resources/contrastchecker/)
- [ ] Verify #5B21B6 contrast ratio on light backgrounds
- [ ] Test `│` rendering in: macOS Terminal.app, iTerm2, VS Code terminal
- [ ] Determine exact background tint hex values for dark/light themes
- [ ] Update `styles.go` with new ListItem styles
- [ ] Test title truncation with styled prefix
- [ ] Create before/after screenshots for PR

---

### Research Sources

**Verified primary sources:**
- [charm/glow ui/stashitem.go](https://github.com/charmbracelet/glow) - Gutter pattern implementation
- [charm/soft-serve selector.go](https://github.com/charmbracelet/soft-serve) - Delegate architecture
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Adaptive color support

**Terminal compatibility:**
- [Box-drawing characters - Wikipedia](https://en.wikipedia.org/wiki/Box-drawing_characters)
- [Microsoft Terminal Issue #577](https://github.com/microsoft/cascadia-code/issues/577) - Line height gaps
- [Microsoft Terminal Issue #13527](https://github.com/microsoft/terminal/issues/13527) - Japanese font issues
- [Kitty Discussion #7680](https://github.com/kovidgoyal/kitty/discussions/7680) - Programmatic generation

**Accessibility:**
- [WebAIM Contrast Checker](https://webaim.org/resources/contrastchecker/)
- [WCAG 2.1 Contrast Guidelines](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
- [Color Blind Accessibility - A11Y Collective](https://www.a11y-collective.com/blog/color-blind-accessibility-guidelines/)
- [Section508.gov Color Usage](https://www.section508.gov/create/making-color-usage-accessible/)

---

## References

- [Source: charm/glow stashitem.go] - Gutter indicator pattern (VERIFIED)
- [Source: charmbracelet/bubbles list/defaultitem.go] - Default delegate styling
- [Source: charm/soft-serve selector.go] - Delegate architecture (VERIFIED)
- [Source: internal/tui/project.go:20-48] - Current cclv item rendering
- [Source: internal/tui/styles.go:34-58] - Theme definition
- [Source: _bmad-output/implementation-artifacts/epic-1-retro-2026-01-15.md] - Visual consistency gap
