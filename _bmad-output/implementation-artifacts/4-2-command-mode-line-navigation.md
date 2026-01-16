# Story 4.2: Command Mode Line Navigation

Status: done

## Story

As a **developer analyzing logs**,
I want **to jump to a specific line using `:N` syntax**,
So that **I can quickly navigate to known locations**.

## Acceptance Criteria

### AC 4.2.1: Command mode activation
- **Given** I am in the viewer (normal or raw mode)
- **When** I press `:`
- **Then** command mode activates with `:` prompt in status bar
- **And** subsequent digit keypresses are captured

### AC 4.2.2: Line navigation execution
- **Given** I am in command mode with digits entered
- **When** I press Enter
- **Then** the viewer scrolls to that line/box number
- **And** command mode exits

### AC 4.2.3: Command mode cancellation
- **Given** I am in command mode
- **When** I press Escape
- **Then** command mode cancels without navigation

### AC 4.2.4: Invalid input handling
- **Given** I enter an invalid number (out of range or non-numeric)
- **When** I press Enter
- **Then** a toast displays "Invalid line number"
- **And** command mode exits

### AC 4.2.5: Mode-specific navigation target
- **Given** I am in normal mode
- **When** I enter `:39` and press Enter
- **Then** the viewer jumps to box/entry 39

- **Given** I am in raw mode (Story 4.3 - future)
- **When** I enter `:39` and press Enter
- **Then** the viewer jumps to JSONL line 39

**Note:** Raw mode is Story 4.3. This story implements command mode infrastructure. Story 4.3 will integrate raw mode line navigation.

## Tasks / Subtasks

- [x] Task 1: Add command mode state to ViewerModel (AC: 4.2.1)
  - [x] 1.1: Define `InputMode` enum in viewer.go: `InputNone`, `InputCommand`, `InputSearch`
  - [x] 1.2: Add `inputMode InputMode` field to ViewerModel (default: InputNone)
  - [x] 1.3: Add `inputBuffer string` field for capturing digit input
  - [x] 1.4: Migrate existing `searching bool` (viewer.go:59) to use `inputMode == InputSearch` - refactor search handling at lines 282-302 to check `inputMode == InputSearch`
  - [x] 1.5: Add `entryLinePositions []int` field for tracking Y offset of each entry

- [x] Task 2: Add command mode key handling in Update() (AC: 4.2.1, 4.2.2, 4.2.3)
  - [x] 2.1: Add `:` key handler (after search mode check, around line 304) that sets `inputMode = InputCommand`, clears `inputBuffer`
  - [x] 2.2: Add command mode handling block BEFORE the existing search mode check (lines 282-302) - command mode takes priority
  - [x] 2.3: Capture digit keys (0-9) and append to `inputBuffer` (limit to 6 digits max)
  - [x] 2.4: Handle Enter key: parse number, validate 1 ≤ N ≤ len(entries), navigate, exit command mode
  - [x] 2.5: Handle Escape key: clear inputBuffer, set `inputMode = InputNone`
  - [x] 2.6: Handle Backspace: remove last character from inputBuffer (if not empty)

- [x] Task 3: Implement line navigation logic (AC: 4.2.2, 4.2.5)
  - [x] 3.1: Create `navigateToEntry(entryNum int) error` method
  - [x] 3.2: Validate entryNum is within 1 to len(entries), return error if invalid
  - [x] 3.3: Use `entryLinePositions[entryNum-1]` to get viewport Y offset
  - [x] 3.4: Call `m.viewport.SetYOffset(offset)` to scroll to target entry
  - [x] 3.5: Track entry line positions in updateContent() - count rendered lines including gutter padding

- [x] Task 4: Add toast notification system (AC: 4.2.4)
  - [x] 4.1: Add `toast string` field to ViewerModel
  - [x] 4.2: Add `toastExpiry time.Time` field to ViewerModel
  - [x] 4.3: Create `toastExpiredMsg` message type (unexported)
  - [x] 4.4: Create `showToast(message string, duration time.Duration) tea.Cmd` method - set toast and toastExpiry, return tea.Tick
  - [x] 4.5: Add `ToastDuration` constant (3 * time.Second) in styles.go
  - [x] 4.6: Handle toastExpiredMsg in Update(): clear BOTH `toast` and `toastExpiry` to prevent race conditions
  - [x] 4.7: Render toast in View() when toast is non-empty and time.Now().Before(toastExpiry)

- [x] Task 5: Update View() to show command mode UI (AC: 4.2.1)
  - [x] 5.1: When `inputMode == InputCommand`, display `:` + inputBuffer in status bar area (similar to search bar at line 685)
  - [x] 5.2: Style command input using `Styles.SearchInput` (reuse existing style)
  - [x] 5.3: Show cursor indicator `_` at end of input buffer
  - [x] 5.4: Update `buildShortcutsSegment()` to include `:N` navigation hint

- [x] Task 6: Add unit tests for command mode (AC: 4.2.1, 4.2.2, 4.2.3, 4.2.4)
  - [x] 6.1: Test `:` key activates command mode (sets inputMode = InputCommand)
  - [x] 6.2: Test digit capture into inputBuffer (single and multiple digits)
  - [x] 6.3: Test Enter key with valid number navigates and exits command mode
  - [x] 6.4: Test Escape key cancels command mode (clears buffer, resets inputMode)
  - [x] 6.5: Test `:0` shows toast error "Invalid line number"
  - [x] 6.6: Test number > entry count shows toast error
  - [x] 6.7: Test toast expiry clears both toast and toastExpiry fields
  - [x] 6.8: Test empty conversation (0 entries) - any number shows error
  - [x] 6.9: Test `:1` navigation works for first entry
  - [x] 6.10: Test `:N` navigation works for last entry (N = len(entries))
  - [x] 6.11: Test non-numeric input handling (letters ignored in command mode)
  - [x] 6.12: Test backspace removes last digit

- [x] Task 7: Run build, lint, and test validation
  - [x] 7.1: Run `make build` - verify binary builds
  - [x] 7.2: Run `make lint` - no errors
  - [x] 7.3: Run `make test` - all tests pass, coverage maintained

- [x] Task 8: Manual testing (deferred - requires real terminal)
  - [ ] 8.1: Press `:` - verify command mode prompt appears with cursor
  - [ ] 8.2: Type digits - verify they appear in prompt
  - [ ] 8.3: Press Enter with valid number - verify navigation to correct entry
  - [ ] 8.4: Press Escape - verify cancellation (prompt disappears, no navigation)
  - [ ] 8.5: Enter `:0` - verify error toast "Invalid line number"
  - [ ] 8.6: Enter number > entry count - verify error toast
  - [ ] 8.7: Toast disappears after 3 seconds
  - [ ] 8.8: Press backspace - verify last digit removed
  - [ ] 8.9: `:1` navigates to first entry
  - [ ] 8.10: `:N` (last entry) navigates to bottom

**Note:** Manual testing items deferred to user verification in real terminal session. All automated tests pass.

## Dev Notes

### Implementation Reference

**New Types (viewer.go):**

```go
// InputMode represents the current input mode for the viewer.
type InputMode int

const (
    InputNone InputMode = iota
    InputCommand  // :N navigation
    InputSearch   // / search
)
```

**New ViewerModel Fields:**

```go
type ViewerModel struct {
    // ... existing fields (keep all current fields)

    // REMOVE: searching bool (line 59) - replaced by inputMode check
    // KEEP: searchInput, searchQuery, searchMatches, currentMatch, noResults

    // Phase 3 additions (Story 4.2)
    inputMode    InputMode  // Current input mode (replaces searching bool)
    inputBuffer  string     // Buffer for command input (:42)
    toast        string     // Toast message to display
    toastExpiry  time.Time  // When toast should disappear

    // Line position tracking for navigation
    entryLinePositions []int  // Y offset where each entry starts in viewport
}
```

**InputMode State Transitions:**

```
┌─────────────┐    ":"     ┌──────────────┐
│  InputNone  │───────────▶│ InputCommand │
└─────────────┘            └──────────────┘
      ▲                           │
      │ Esc                       │ Enter (valid)
      │ ◀─────────────────────────┘
      │                           │
      │ Esc                       │ Enter (invalid) → shows toast
      │ ◀─────────────────────────┘
      │
      │     "/"                   ┌─────────────┐
      └──────────────────────────▶│ InputSearch │
                                  └─────────────┘
                                        │
                                        │ Esc/Enter
                                        ▼
                                  ┌─────────────┐
                                  │  InputNone  │
                                  └─────────────┘
```

**styles.go Addition:**

```go
// Toast duration constant (per FR-404 spec)
const ToastDuration = 3 * time.Second
```

**Core Functions to Implement:**

```go
// navigateToEntry jumps viewport to the specified entry (1-indexed).
func (m *ViewerModel) navigateToEntry(entryNum int) error {
    if len(m.entries) == 0 {
        return fmt.Errorf("invalid line number")
    }
    if entryNum < 1 || entryNum > len(m.entries) {
        return fmt.Errorf("invalid line number")
    }

    // Use entryLinePositions to find Y offset for entry
    if entryNum <= len(m.entryLinePositions) {
        m.viewport.SetYOffset(m.entryLinePositions[entryNum-1])
    }
    return nil
}

// showToast displays a temporary toast message.
func (m *ViewerModel) showToast(message string, duration time.Duration) tea.Cmd {
    m.toast = message
    m.toastExpiry = time.Now().Add(duration)
    return tea.Tick(duration, func(t time.Time) tea.Msg {
        return toastExpiredMsg{}
    })
}

// toastExpiredMsg signals that the toast should be cleared.
type toastExpiredMsg struct{}
```

**Key Handling Pattern:**

```go
func (m ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    // Handle toast expiry message (add near other message handlers)
    case toastExpiredMsg:
        m.toast = ""
        m.toastExpiry = time.Time{} // Clear both to prevent race conditions
        return m, nil

    case tea.KeyMsg:
        // Handle command mode FIRST (before search mode check)
        // Insert this BEFORE the existing search handling at lines 282-302
        if m.inputMode == InputCommand {
            switch msg.String() {
            case "enter":
                num, err := strconv.Atoi(m.inputBuffer)
                if err != nil || num < 1 || num > len(m.entries) {
                    m.inputMode = InputNone
                    m.inputBuffer = ""
                    return m, m.showToast("Invalid line number", ToastDuration)
                }
                _ = m.navigateToEntry(num) // Validation already done above
                m.inputMode = InputNone
                m.inputBuffer = ""
                return m, nil
            case "esc":
                m.inputMode = InputNone
                m.inputBuffer = ""
                return m, nil
            case "backspace":
                if len(m.inputBuffer) > 0 {
                    m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
                }
                return m, nil
            case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
                // Limit buffer to 6 digits (supports up to 999,999 entries)
                if len(m.inputBuffer) < 6 {
                    m.inputBuffer += msg.String()
                }
                return m, nil
            default:
                // Ignore all other keys in command mode (letters, symbols, etc.)
                return m, nil
            }
        }

        // Handle search mode SECOND (existing code at lines 282-302)
        // REFACTOR: Change `if m.searching {` to `if m.inputMode == InputSearch {`
        if m.inputMode == InputSearch {
            // ... existing search handling code ...
        }

        // Normal key handling (existing code starting around line 304)
        switch msg.String() {
        case ":":
            m.inputMode = InputCommand
            m.inputBuffer = ""
            return m, nil
        case "/":
            // REFACTOR: Change `m.searching = true` to `m.inputMode = InputSearch`
            m.inputMode = InputSearch
            m.searchInput.Focus()
            return m, textinput.Blink
        // ... rest of existing key handlers ...
        }
    }
}
```

### Key Integration Points

| Location | Line Reference | Change |
|----------|----------------|--------|
| `ViewerModel` struct | viewer.go:42-97 | Add `inputMode`, `inputBuffer`, `toast`, `toastExpiry`, `entryLinePositions` fields; REMOVE `searching bool` (line 59) |
| `Update()` toastExpiredMsg | viewer.go:271 (after spinner.TickMsg) | Add toastExpiredMsg handler to clear toast and toastExpiry |
| `Update()` command mode | viewer.go:281 (BEFORE search mode) | Add command mode handling block |
| `Update()` search mode | viewer.go:282-302 | REFACTOR: Change `if m.searching {` to `if m.inputMode == InputSearch {` |
| `Update()` "/" handler | viewer.go:374-377 | REFACTOR: Change `m.searching = true` to `m.inputMode = InputSearch` |
| `View()` command mode | viewer.go:685 (similar to search bar) | Render `:` + inputBuffer when inputMode == InputCommand |
| `View()` toast | viewer.go:724 (before return) | Render toast overlay when toast is non-empty |
| `updateContent()` | viewer.go:736-767 | Track entry line positions in `m.entryLinePositions` |
| `buildShortcutsSegment()` | viewer.go:658-669 | Add `:N` to shortcuts text |
| `NewViewerModel()` | viewer.go:148-222 | Initialize `inputMode = InputNone`, `entryLinePositions = make([]int, 0)` |

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Add InputMode enum, command mode fields, navigation logic, toast system, refactor searching bool |
| `internal/tui/viewer_test.go` | Add command mode tests (12 test cases) |
| `internal/tui/styles.go` | Add `ToastDuration` constant |

### Files NOT to Modify

| File | Reason |
|------|--------|
| `internal/tui/app.go` | No app-level changes needed (toast is viewer-scoped) |
| `internal/parser/*.go` | Parser unaffected |
| `internal/scanner/*.go` | Scanner unaffected |
| `internal/watcher/*.go` | Watcher unaffected |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI** | Text icons only per project-context.md |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **Test patterns** | Table-driven tests per project-context.md |
| **TEA pattern** | All state changes via Update() |
| **Toast duration** | 3 seconds per PRD FR-404 spec |

### Previous Story Intelligence (Story 4.1)

Key learnings from Story 4.1:

1. **Line number infrastructure exists** - `gutterWidth`, `showLineNumbers` already implemented (viewer.go:95-96)
2. **Gutter width calculation** - `calculateGutterWidth()` function available (viewer.go:99-110)
3. **Static render functions** - All have gutterWidth parameter (viewer.go:914-1034)
4. **Cache invalidation patterns** - Clear `renderCache` when state changes affect rendering (viewer.go:1153-1158)
5. **prependGutter()** - Adds line numbers to rendered content (viewer.go:112-128)

**Relevant patterns established:**
- Entry counting is 1-indexed for display
- Width calculations account for gutter: `wrapWidth := m.width - 4 - gutterSpace`
- **IMPORTANT:** Entry line positions are NOT yet tracked - this must be built in this story

**Code Review Learnings from Story 4.1:**
- Static render functions must match dynamic render function signatures
- `prependGutterStatic()` exists for async rendering without lipgloss styling
- Cache invalidation happens on: resize, toggle changes, file reset, gutter width change

### Architecture Reference

From architecture-phase3.md (Decision 10):
- **Command Mode Input:** Status bar prompt, digit capture
- **Standard vim-like pattern:** `:` enters command mode, digits capture, Enter executes
- **InputMode enum:** Hybrid approach with `rawMode bool` + `inputMode enum`

**Mode Transition Diagram:**
```
Normal ─────:────→ Command ──Enter──→ Normal (with action)
   │                   │
   /                  Esc
   ↓                   ↓
Search               Normal
```

### Entry Line Position Tracking

To navigate accurately, track where each entry starts in the rendered content.

**CRITICAL:** The line counting must account for:
1. Rendered content lines
2. Gutter padding on continuation lines (handled by prependGutter)
3. The trailing newline after each entry

```go
// In updateContent() - REPLACE existing implementation at viewer.go:736-767
func (m *ViewerModel) updateContent() {
    var content strings.Builder

    // Render only loadedCount entries for lazy loading
    renderCount := m.loadedCount
    if renderCount > len(m.entries) {
        renderCount = len(m.entries)
    }

    // Reset and track entry positions for navigation (Story 4.2)
    m.entryLinePositions = make([]int, 0, renderCount)
    currentLine := 0

    for i := 0; i < renderCount; i++ {
        // Track position BEFORE rendering this entry
        m.entryLinePositions = append(m.entryLinePositions, currentLine)

        rendered := m.getCachedRender(i, m.entries[i])
        // Prepend line numbers if enabled (Story 4.1)
        if m.showLineNumbers {
            rendered = prependGutter(i+1, rendered, m.gutterWidth) // 1-indexed line numbers
        }
        content.WriteString(rendered)
        content.WriteString("\n")

        // Count lines in rendered content (including the trailing newline)
        currentLine += strings.Count(rendered, "\n") + 1
    }

    // Add loading indicator at the bottom if more content is available
    // Note: Lazy loading indicator is NOT a real entry, so no line number prepended
    if m.lazyEnabled && renderCount < len(m.entries) {
        if m.lazyLoadState == LoadingStateLoading {
            content.WriteString(ListStyles.Loading.Render("Loading more messages..."))
        } else {
            content.WriteString(Styles.Muted.Render(fmt.Sprintf("-- %d more entries (scroll down to load) --", len(m.entries)-renderCount)))
        }
        content.WriteString("\n")
    }

    m.viewport.SetContent(content.String())
}
```

**Note:** This replaces the existing updateContent() implementation entirely. The key addition is tracking `entryLinePositions` before each entry is rendered.

### Toast Rendering

Render toast inline in the footer area (simpler than overlay):

```go
// In View() - add BEFORE the final return, around line 724
// Toast replaces shortcuts segment when active
if m.toast != "" && time.Now().Before(m.toastExpiry) {
    toastStyle := lipgloss.NewStyle().
        Background(accentColor).
        Foreground(whiteColor).
        Padding(0, 1)
    toastSegment := toastStyle.Render(m.toast)

    // Replace shortcuts segment with toast
    footer = lipgloss.JoinHorizontal(lipgloss.Top, modeSegment, newEntriesSegment, posSegment, toastSegment)
}
```

**Alternative:** If toast should overlay without replacing shortcuts, use lipgloss.Place() to position it.

### Project Context Reference

From `project-context.md`:
- **TEA pattern**: State changes only in Update(), View() for rendering
- **NO EMOJI**: Text icons only - command prompt `:` is fine
- **Test patterns**: Table-driven tests required
- **Makefile**: `make build`, `make test` commands

### Git Intelligence

Recent commits:
```
2624b13 feat: add line number gutter to viewer display
07b7984 docs: add Epic 3 retrospective for Markdown Rendering
```

Suggested commit message:
```
feat: implement command mode line navigation

- Add InputMode enum for command/search modes
- Implement :N syntax for jumping to entry N
- Add toast notification system for errors
- Track entry line positions for accurate navigation

Story 4.2 of Epic 4: Developer Power Tools

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Dependencies

- **Story 4.1 (Complete)**: Provides line numbers and gutter infrastructure
- **Story 4.3 (Future)**: Will extend navigation for raw mode JSONL lines
- **Story 4.4 (Future)**: Will use toast system for path display

### References

- [Source: epics-phase3.md lines 142-168] - Story 4.2 requirements and acceptance criteria
- [Source: prd-phase3.md lines 109-118] - FR-402 Vim-Style Line Navigation
- [Source: architecture-phase3.md lines 183-198] - ViewerModel Extensions with InputMode
- [Source: architecture-phase3.md lines 316-329] - Mode transition diagram
- [Source: project-context.md lines 97-104] - TEA pattern rules
- [Source: internal/tui/viewer.go:94-97] - showLineNumbers and gutterWidth fields
- [Source: internal/tui/viewer.go:280-302] - Existing search mode handling pattern
- [Source: 4-1-line-numbers-display.md] - Previous story implementation patterns

## Implementation Checklist

Before marking story complete, verify:

**State Management:**
- [ ] `InputMode` enum defined: `InputNone`, `InputCommand`, `InputSearch`
- [ ] `inputMode InputMode` field added to ViewerModel (default: InputNone)
- [ ] `inputBuffer string` field added to ViewerModel
- [ ] `toast string` and `toastExpiry time.Time` fields added
- [ ] `entryLinePositions []int` field added for navigation tracking
- [ ] `searching bool` field REMOVED from ViewerModel (line 59)
- [ ] All `m.searching` references refactored to `m.inputMode == InputSearch`

**styles.go Changes:**
- [ ] `ToastDuration` constant added (3 * time.Second)

**Command Mode Key Handling:**
- [ ] `:` key handler enters command mode (sets inputMode = InputCommand)
- [ ] Digit keys (0-9) append to inputBuffer (max 6 digits)
- [ ] Enter key parses number, validates (1 ≤ N ≤ len(entries)), navigates
- [ ] Escape key cancels command mode (clears buffer, resets inputMode)
- [ ] Backspace removes last character from inputBuffer
- [ ] Non-digit keys ignored in command mode

**Navigation:**
- [ ] `navigateToEntry(entryNum int) error` method implemented
- [ ] Entry line positions tracked in `updateContent()` before each entry
- [ ] Boundary cases handled: entry 1 (first), entry N (last)

**Toast System:**
- [ ] `showToast(message string, duration time.Duration) tea.Cmd` implemented
- [ ] `toastExpiredMsg` message type defined
- [ ] `toastExpiredMsg` handler clears BOTH `toast` and `toastExpiry`
- [ ] Invalid input shows toast "Invalid line number"
- [ ] Out-of-range input shows toast error
- [ ] Empty conversation (0 entries) shows error for any input
- [ ] Toast renders in footer area

**View Updates:**
- [ ] Command mode UI renders `:` + inputBuffer with cursor `_`
- [ ] `buildShortcutsSegment()` includes `:N` navigation hint
- [ ] Toast renders when active and not expired

**Tests (12 test cases):**
- [ ] Test `:` key activates command mode
- [ ] Test digit capture into inputBuffer
- [ ] Test Enter with valid number navigates and exits
- [ ] Test Escape cancels command mode
- [ ] Test `:0` shows toast error
- [ ] Test number > entry count shows toast error
- [ ] Test toast expiry clears both fields
- [ ] Test empty conversation error handling
- [ ] Test `:1` navigation (first entry)
- [ ] Test `:N` navigation (last entry)
- [ ] Test non-numeric input ignored
- [ ] Test backspace removes last digit

**Build Validation:**
- [ ] `make build` succeeds
- [ ] `make lint` has no errors
- [ ] `make test` passes with no regressions
- [ ] Coverage maintained at 90%+

**Manual Testing:**
- [ ] `:` shows command prompt with cursor
- [ ] Type digits - appear in prompt
- [ ] `:42` navigates to entry 42
- [ ] Escape cancels (prompt disappears)
- [ ] `:0` shows error toast
- [ ] Number > entry count shows error
- [ ] Toast disappears after 3 seconds
- [ ] Backspace removes last digit
- [ ] `:1` navigates to first entry
- [ ] `:N` (last entry) navigates to bottom

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - All tests pass, no debug issues.

### Completion Notes List

1. Implemented `InputMode` enum with `InputNone`, `InputCommand`, `InputSearch` states
2. Added command mode state fields: `inputMode`, `inputBuffer`, `toast`, `toastExpiry`, `entryLinePositions`
3. Refactored `searching bool` to use `inputMode == InputSearch`
4. Added command mode key handling (`:`, digits, Enter, Escape, Backspace)
5. Implemented `navigateToEntry()` method for viewport navigation
6. Implemented `showToast()` method with `tea.Tick` for auto-expiry
7. Added `ToastDuration` constant (3 seconds) in styles.go
8. Updated `updateContent()` to track entry line positions
9. Added command mode UI in View() with `:` prefix and `_` cursor
10. Added toast rendering in footer area
11. Updated `buildShortcutsSegment()` with `:N:goto` hint
12. Added 18 unit tests covering all command mode functionality
13. All automated tests pass (make build, make lint, make test)
14. Manual TUI testing deferred to user verification

### Code Review Fixes Applied

1. **Toast race condition fix:** Added `toastID` field and ID-based expiry matching to prevent stale timers from clearing new toasts
2. **Bulk load navigation sync:** Added `syncEntryLinePositions()` method, called after `viewerMessagesLoadedMsg` with pre-rendered content
3. **Toast visibility in command mode:** Command mode footer now renders toast alongside command bar
4. **Magic number elimination:** Added `MaxCommandBufferDigits` constant (6) in styles.go
5. **Test improvements:** Enhanced `TestToastExpiryClearsBothFields` to use Update(), added `TestToastExpiryIgnoresMismatchedID`, `TestShowToastIncreasesToastID`, `TestSyncEntryLinePositions`

### File List

- `internal/tui/viewer.go` - Added InputMode enum, command mode handling, navigation, toast system with ID-based race protection, syncEntryLinePositions()
- `internal/tui/styles.go` - Added ToastDuration, MaxCommandBufferDigits constants
- `internal/tui/viewer_test.go` - Added 22 command mode unit tests (18 original + 4 CR fixes)
