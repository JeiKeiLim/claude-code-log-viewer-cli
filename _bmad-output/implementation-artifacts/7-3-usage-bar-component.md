# Story 7.3: Usage Bar Component

Status: done

## Story

As a **cclv user**,
I want **a compact usage bar showing my limits**,
So that **I can see usage at a glance**.

## Acceptance Criteria

1. **AC-1: Basic Usage Display**
   - Given usage data is available
   - When the usage bar renders
   - Then it displays: `[5h: 35% 2h 15m] [7d: 12%]` (format: `Xh Ym` with space)
   - And fits in 1 line at top of screen

2. **AC-2: Warning Color Threshold**
   - Given 5-hour utilization > 80%
   - When displayed
   - Then percentage is styled with warning color (yellow/amber)

3. **AC-3: Critical Color Threshold**
   - Given 5-hour utilization > 95%
   - When displayed
   - Then percentage is styled with critical color (red)

4. **AC-4: Reset Time Countdown**
   - Given reset time is available
   - When displayed
   - Then shows human-readable countdown (e.g., "2h15m", "45m", "5m")

5. **AC-5: Loading State**
   - Given usage data is loading
   - When displayed
   - Then shows "Loading..." or spinner indicator

6. **AC-6: Not Logged In State**
   - Given credentials not found
   - When displayed
   - Then shows "Not logged in" with dimmed style

7. **AC-7: Stale Data Indicator**
   - Given API returned error but lastGood data available
   - When displayed
   - Then shows values with "(stale)" indicator

8. **AC-8: Error State**
   - Given API error and no lastGood data available
   - When displayed
   - Then shows appropriate error message (e.g., "Session expired - run 'claude' to re-login")

## Tasks / Subtasks

- [x] Task 1: Create UsageBarModel in `internal/usage/bar.go` (AC: #1, #5, #6, #7, #8)
  - [x] Subtask 1.1: Define `UsageBarStyles` struct for dependency injection
  - [x] Subtask 1.2: Define `UsageBarModel` struct with `limits *UsageLimits`, `state UsageBarState`, `errMsg string`, `width int`, `styles UsageBarStyles`
  - [x] Subtask 1.3: Define `UsageBarState` enum: `StateLoading`, `StateNormal`, `StateStale`, `StateNotLoggedIn`, `StateError`
  - [x] Subtask 1.4: Implement `NewUsageBarModel(styles UsageBarStyles)` constructor
  - [x] Subtask 1.5: Implement `SetLoading()`, `SetLimits(*UsageLimits, stale bool)`, `SetNotLoggedIn()`, `SetError(string)`, `SetWidth(int)` methods

- [x] Task 2: Implement `View()` rendering method (AC: #1, #2, #3, #4, #5, #6, #7, #8)
  - [x] Subtask 2.1: Render loading state: dimmed "Loading usage..."
  - [x] Subtask 2.2: Render not-logged-in state: dimmed "Not logged in"
  - [x] Subtask 2.3: Render error state: red error message
  - [x] Subtask 2.4: Render normal/stale state with utilization percentages
  - [x] Subtask 2.5: Apply warning (amber) color when utilization > 80%
  - [x] Subtask 2.6: Apply critical (red) color when utilization > 95%
  - [x] Subtask 2.7: Add "(stale)" indicator when data is stale

- [x] Task 3: Implement `formatDuration()` helper (AC: #4)
  - [x] Subtask 3.1: Create `formatDuration(resetTime *time.Time) string` helper
  - [x] Subtask 3.2: Handle nil resetTime (return empty string)
  - [x] Subtask 3.3: Format as "Xh Ym" for hours+minutes (e.g., "2h 15m")
  - [x] Subtask 3.4: Format as "Xm" for <1 hour (e.g., "45m")
  - [x] Subtask 3.5: Return "soon" for <1 minute remaining
  - [x] Subtask 3.6: Handle negative duration (past reset time) gracefully

- [x] Task 4: Add usage bar styles to `internal/tui/styles.go` (AC: #2, #3, #6)
  - [x] Subtask 4.1: Add `WarningColor` adaptive color (amber) to Theme
  - [x] Subtask 4.2: Add `CriticalColor` adaptive color (red) to Theme
  - [x] Subtask 4.3: Add `UsageBarStyles` struct with `Container`, `Label`, `Normal`, `Warning`, `Critical`, `Dimmed`, `Stale` styles
  - [x] Subtask 4.4: Add `UsageBarHeight` constant (value: 1)
  - [x] Subtask 4.5: Export styles so `internal/usage/bar.go` can import and use them (via function or direct access)

- [x] Task 5: Write comprehensive tests (AC: #1-8)
  - [x] Subtask 5.1: Create `internal/usage/bar_test.go`
  - [x] Subtask 5.2: Table-driven tests for `View()` all states (loading, normal, stale, not-logged-in, error)
  - [x] Subtask 5.3: Table-driven tests for `formatDuration()` edge cases (nil, past, <1min, minutes, hours+minutes, hours only)
  - [x] Subtask 5.4: Test color threshold application (80%, 95%) via style selection verification
  - [x] Subtask 5.5: Test width truncation behavior
  - [x] Subtask 5.6: Ensure 90%+ coverage on new code (achieved: 94.1%)

## Dev Notes

### Critical Implementation Details

**UsageBarModel Design (Pure View Component):**

The UsageBarModel is a **pure view component** - it does NOT implement `tea.Model`. State is managed externally (by AppModel in Story 7.4) and passed to the bar via setter methods.

```go
package usage

import (
    "fmt"
    "time"

    "github.com/charmbracelet/lipgloss"
)

// UsageBarState represents the current state of the usage bar.
type UsageBarState int

const (
    StateLoading UsageBarState = iota
    StateNormal
    StateStale
    StateNotLoggedIn
    StateError
)

// UsageBarModel is a view component for displaying usage limits.
// It does NOT implement tea.Model - state is managed externally.
type UsageBarModel struct {
    limits *UsageLimits
    state  UsageBarState
    errMsg string
    width  int
    styles UsageBarStyles // Injected at construction
}

// NewUsageBarModel creates a new usage bar in loading state.
// Styles must be provided via dependency injection.
func NewUsageBarModel(styles UsageBarStyles) *UsageBarModel {
    return &UsageBarModel{
        state:  StateLoading,
        styles: styles,
    }
}

// SetLoading sets the bar to loading state.
func (m *UsageBarModel) SetLoading() {
    m.state = StateLoading
    m.limits = nil
    m.errMsg = ""
}

// SetLimits updates the bar with usage data.
func (m *UsageBarModel) SetLimits(limits *UsageLimits, stale bool) {
    m.limits = limits
    if stale {
        m.state = StateStale
    } else {
        m.state = StateNormal
    }
    m.errMsg = ""
}

// SetNotLoggedIn sets the bar to not-logged-in state.
func (m *UsageBarModel) SetNotLoggedIn() {
    m.state = StateNotLoggedIn
    m.limits = nil
    m.errMsg = ""
}

// SetError sets the bar to error state with message.
func (m *UsageBarModel) SetError(msg string) {
    m.state = StateError
    m.errMsg = msg
}

// SetWidth sets the available width for rendering.
// Used to truncate/pad output to fit available terminal width.
func (m *UsageBarModel) SetWidth(width int) {
    m.width = width
}

// Width returns the current width setting.
func (m *UsageBarModel) Width() int {
    return m.width
}
```

**Display Format:**

```
[5h: 35% 2h 15m] [7d: 12%]
```

- `5h:` label for 5-hour window
- `35%` utilization percentage
- `2h 15m` time until reset (omit if nil)
- `7d:` label for 7-day window
- Separator: single space between windows

**Color Thresholds:**

| Utilization | Style |
|-------------|-------|
| 0-80% | Normal (DefaultTheme.Text) |
| >80% | Warning (amber/yellow) |
| >95% | Critical (red) |

Apply color thresholds to BOTH 5-hour and 7-day windows independently.

**Duration Formatting:**

```go
func formatDuration(resetTime *time.Time) string {
    if resetTime == nil {
        return ""
    }

    remaining := time.Until(*resetTime)
    if remaining < 0 {
        return "" // Past reset time, omit
    }

    if remaining < time.Minute {
        return "soon"
    }

    hours := int(remaining.Hours())
    minutes := int(remaining.Minutes()) % 60

    if hours > 0 {
        if minutes > 0 {
            return fmt.Sprintf("%dh %dm", hours, minutes)
        }
        return fmt.Sprintf("%dh", hours)
    }
    return fmt.Sprintf("%dm", minutes)
}
```

**Note:** For 7-day reset time, this will show large values like "167h 30m" which is fine - Story 7.5 may add special handling for day-level display if needed.

### Styles to Add (styles.go)

```go
// Usage bar colors
var (
    WarningColor  = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#F59E0B"} // Amber
    CriticalColor = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#EF4444"} // Red
)

// UsageBarHeight is the height of the usage bar in lines.
const UsageBarHeight = 1

// GetUsageBarStyles returns styles for the usage bar component.
// Exported as a function to allow dependency injection into internal/usage package.
func GetUsageBarStyles() usage.UsageBarStyles {
    return usage.UsageBarStyles{
        Container: lipgloss.NewStyle().
            Background(bgAltColor).
            Padding(0, 1),
        Label: lipgloss.NewStyle().
            Foreground(mutedColor),
        Normal: lipgloss.NewStyle().
            Foreground(textColor),
        Warning: lipgloss.NewStyle().
            Foreground(WarningColor).
            Bold(true),
        Critical: lipgloss.NewStyle().
            Foreground(CriticalColor).
            Bold(true),
        Dimmed: lipgloss.NewStyle().
            Foreground(dimColor).
            Italic(true),
        Stale: lipgloss.NewStyle().
            Foreground(mutedColor).
            Italic(true),
    }
}

// ALTERNATIVE: If circular import is an issue, define styles struct locally:
// var usageBarStyles = struct { ... }{...}
// And have bar.go define its own UsageBarStyles type that tui populates.
```

**Note:** The UsageBarStyles struct is defined in `internal/usage/bar.go` (the consumer), and `internal/tui/styles.go` provides a function that returns populated styles. This avoids circular imports.

**Previous (DEPRECATED):**
```go
// UsageBarStyles contains styles for the usage bar component.
var UsageBarStyles = struct {
    Container lipgloss.Style
    Label     lipgloss.Style
    Normal    lipgloss.Style
    Warning   lipgloss.Style
    Critical  lipgloss.Style
    Dimmed    lipgloss.Style
    Stale     lipgloss.Style
}
// ... same field assignments as above
```
(This deprecated pattern kept for reference - use GetUsageBarStyles() function instead)

### View() Implementation Pattern

```go
// View renders the usage bar.
func (m *UsageBarModel) View() string {
    switch m.state {
    case StateLoading:
        return m.renderLoading()
    case StateNotLoggedIn:
        return m.renderNotLoggedIn()
    case StateError:
        return m.renderError()
    case StateNormal, StateStale:
        return m.renderUsage()
    default:
        return ""
    }
}

func (m *UsageBarModel) renderLoading() string {
    // Use Dimmed style: "Loading usage..."
}

func (m *UsageBarModel) renderNotLoggedIn() string {
    // Use Dimmed style: "Not logged in"
}

func (m *UsageBarModel) renderError() string {
    // Use Critical style for error message
}

func (m *UsageBarModel) renderUsage() string {
    // Build: [5h: XX% Xh Xm] [7d: XX%]
    // Apply color based on threshold
    // Add "(stale)" if StateStale
    // Truncate/pad to m.width if set (>0)
}
```

### Package Structure

```
internal/usage/
├── types.go           # UsageLimits, UsageWindow, errors (existing)
├── credentials.go     # GetOAuthToken (existing)
├── client.go          # Client, FetchUsage (existing)
├── bar.go             # NEW: UsageBarModel, formatDuration
├── bar_test.go        # NEW: Tests for bar component
└── ..._test.go        # Existing tests
```

### Project Structure Notes

- `bar.go` belongs in `internal/usage/` package (co-located with types and client)
- Styles added to `internal/tui/styles.go` (centralized styling pattern)
- No new packages created

### Styling Approach (IMPORTANT)

**Decision: Dependency Injection Pattern**

To avoid circular imports between `internal/usage/` and `internal/tui/`:

1. **Styles defined in `internal/tui/styles.go`** - centralized as per project convention
2. **`bar.go` receives styles via dependency injection** - pass styles at construction time

```go
// In internal/usage/bar.go
type UsageBarStyles struct {
    Container lipgloss.Style
    Label     lipgloss.Style
    Normal    lipgloss.Style
    Warning   lipgloss.Style
    Critical  lipgloss.Style
    Dimmed    lipgloss.Style
    Stale     lipgloss.Style
}

func NewUsageBarModel(styles UsageBarStyles) *UsageBarModel {
    return &UsageBarModel{
        state:  StateLoading,
        styles: styles,
    }
}
```

```go
// In internal/tui/app.go (Story 7.4)
usageBar := usage.NewUsageBarModel(tui.GetUsageBarStyles())
```

This pattern:
- Keeps style definitions centralized in tui/styles.go
- Avoids circular imports (tui imports usage, usage does NOT import tui)
- Allows bar.go to use lipgloss directly for rendering

### Alternative: Content-Only Pattern (NOT RECOMMENDED)

An alternative would be returning structured data without styling:

```go
// UsageBarContent represents the content to be styled.
type UsageBarContent struct { /* ... */ }
func (m *UsageBarModel) Content() *UsageBarContent { /* ... */ }
```

**Decision:** Use dependency injection pattern instead. This keeps bar.go self-contained while avoiding circular imports.

### Error Message Mapping

| Error | Display Message |
|-------|-----------------|
| `ErrNoCredentials` | "Not logged in" |
| `ErrKeychainNotFound` | "Not logged in" |
| `ErrTokenExpired` | "Session expired" |
| `ErrAPITimeout` | "(timeout)" with last good data |
| `ErrAPIError` | "(error)" with last good data |
| Other | "Usage unavailable" |

### Testing Strategy

**Unit Tests (bar_test.go):**

```go
// testStyles returns a minimal UsageBarStyles for testing.
func testStyles() UsageBarStyles {
    return UsageBarStyles{
        Container: lipgloss.NewStyle(),
        Label:     lipgloss.NewStyle(),
        Normal:    lipgloss.NewStyle(),
        Warning:   lipgloss.NewStyle().SetString("[WARN]"), // Marker for verification
        Critical:  lipgloss.NewStyle().SetString("[CRIT]"), // Marker for verification
        Dimmed:    lipgloss.NewStyle(),
        Stale:     lipgloss.NewStyle(),
    }
}

func TestUsageBarModel_View(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(*UsageBarModel)
        contains []string
        excludes []string
    }{
        {
            name: "loading state",
            setup: func(m *UsageBarModel) { m.SetLoading() },
            contains: []string{"Loading"},
        },
        {
            name: "not logged in",
            setup: func(m *UsageBarModel) { m.SetNotLoggedIn() },
            contains: []string{"Not logged in"},
        },
        {
            name: "normal usage",
            setup: func(m *UsageBarModel) {
                m.SetLimits(&UsageLimits{
                    FiveHour: &UsageWindow{Utilization: 35.0},
                    SevenDay: &UsageWindow{Utilization: 12.0},
                }, false)
            },
            contains: []string{"5h:", "35%", "7d:", "12%"},
            excludes: []string{"(stale)"},
        },
        {
            name: "stale data",
            setup: func(m *UsageBarModel) {
                m.SetLimits(&UsageLimits{
                    FiveHour: &UsageWindow{Utilization: 35.0},
                    SevenDay: &UsageWindow{Utilization: 12.0},
                }, true)
            },
            contains: []string{"(stale)"},
        },
        {
            name: "error state",
            setup: func(m *UsageBarModel) { m.SetError("Session expired") },
            contains: []string{"Session expired"},
        },
        {
            name: "warning threshold 5h >80%",
            setup: func(m *UsageBarModel) {
                m.SetLimits(&UsageLimits{
                    FiveHour: &UsageWindow{Utilization: 85.0},
                    SevenDay: &UsageWindow{Utilization: 12.0},
                }, false)
            },
            contains: []string{"85%"}, // Verify warning style applied
        },
        {
            name: "critical threshold 5h >95%",
            setup: func(m *UsageBarModel) {
                m.SetLimits(&UsageLimits{
                    FiveHour: &UsageWindow{Utilization: 98.0},
                    SevenDay: &UsageWindow{Utilization: 12.0},
                }, false)
            },
            contains: []string{"98%"}, // Verify critical style applied
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := NewUsageBarModel(testStyles())
            tt.setup(m)
            got := m.View()
            for _, want := range tt.contains {
                if !strings.Contains(got, want) {
                    t.Errorf("View() = %q, want to contain %q", got, want)
                }
            }
            for _, notWant := range tt.excludes {
                if strings.Contains(got, notWant) {
                    t.Errorf("View() = %q, should not contain %q", got, notWant)
                }
            }
        })
    }
}

func TestFormatDuration(t *testing.T) {
    now := time.Now()
    tests := []struct {
        name   string
        reset  *time.Time
        want   string
    }{
        {"nil reset", nil, ""},
        {"past time", ptr(now.Add(-1 * time.Hour)), ""},
        {"30 seconds", ptr(now.Add(30 * time.Second)), "soon"},
        {"59 seconds", ptr(now.Add(59 * time.Second)), "soon"},
        {"1 minute exactly", ptr(now.Add(1 * time.Minute)), "1m"},
        {"45 minutes", ptr(now.Add(45 * time.Minute)), "45m"},
        {"1h exactly", ptr(now.Add(1 * time.Hour)), "1h"},
        {"1h 1m", ptr(now.Add(1*time.Hour + 1*time.Minute)), "1h 1m"},
        {"2h 15m", ptr(now.Add(2*time.Hour + 15*time.Minute)), "2h 15m"},
        {"5h exactly", ptr(now.Add(5 * time.Hour)), "5h"},
        {"167h 30m (7-day window)", ptr(now.Add(167*time.Hour + 30*time.Minute)), "167h 30m"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := formatDuration(tt.reset)
            if got != tt.want {
                t.Errorf("formatDuration() = %q, want %q", got, tt.want)
            }
        })
    }
}

func ptr(t time.Time) *time.Time {
    return &t
}
```

### Critical Rules (from project-context.md)

- NO EMOJI in any output
- Use `make test` not raw `go test`
- Table-driven tests required
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Keep functions focused - single responsibility
- All styles defined in `internal/tui/styles.go`
- Use Lipgloss for ALL visual styling

### Previous Story Learnings (Stories 7.1 and 7.2)

From Story 7.1:
- Sentinel errors defined in types.go - reuse existing errors (`ErrNoCredentials`, `ErrTokenExpired`)
- Table-driven tests with edge cases - continue this pattern
- 95.6% coverage achieved - maintain 90%+ coverage

From Story 7.2:
- FetchUsage returns `(limits, stale, error)` - use `stale` flag for "(stale)" indicator
- `lastGood` pattern for graceful degradation - bar should handle this case
- Custom UnmarshalJSON for time parsing - already handled in types.go
- Client.InvalidateCache() available for Story 7.5 manual refresh

### Anti-Patterns to Avoid

1. **DO NOT** implement `tea.Model` interface - this is a view-only component
2. **DO NOT** make API calls from bar.go - state comes from external caller
3. **DO NOT** use emoji - text indicators only ("(stale)", "Loading...", "Not logged in")
4. **DO NOT** create circular imports between `internal/usage/` and `internal/tui/`
5. **DO NOT** hardcode colors - use centralized styles from styles.go
6. **DO NOT** panic on nil limits - handle gracefully with loading/error state
7. **DO NOT** show "0%" when data is nil - show loading or error state instead

### Expected Commit Format

```
feat: add usage bar component for TUI display (Story 7.3)

Implements compact usage bar view component:
- UsageBarModel with state management methods
- Warning (>80%) and critical (>95%) color thresholds
- Human-readable reset time countdown
- Loading, not-logged-in, stale, and error states

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4.md#Story-7.3]
- [Source: _bmad-output/project-context.md] - Critical rules and patterns
- [Source: internal/usage/types.go] - UsageLimits, UsageWindow structs
- [Source: internal/usage/client.go] - Client.FetchUsage() return signature
- [Source: internal/tui/styles.go] - Existing style patterns
- [Source: _bmad-output/implementation-artifacts/7-1-oauth-credential-access.md] - Sentinel errors
- [Source: _bmad-output/implementation-artifacts/7-2-usage-api-client.md] - API client patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Implemented UsageBarModel as a pure view component (no tea.Model) in `internal/usage/bar.go`
- Added UsageBarStyles struct for dependency injection to avoid circular imports
- Added WarningColor and CriticalColor adaptive colors for 80%/95% thresholds
- Added UsageBarHeight constant and style getter functions in `internal/tui/styles.go`
- Display format: `[5h: XX% Xh Xm] [7d: XX%]` with optional "(stale)" indicator
- formatDuration handles: nil (empty), negative (empty), <1min ("soon"), minutes ("Xm"), hours ("Xh" or "Xh Xm")
- Tests cover all states: loading, normal, stale, not-logged-in, error
- Tests cover color thresholds: normal (<=80%), warning (>80%), critical (>95%)
- Coverage achieved: 94.1% on internal/usage package

### File List

- `internal/usage/bar.go` - NEW: UsageBarModel view component
- `internal/usage/bar_test.go` - NEW: Comprehensive tests for bar component
- `internal/tui/styles.go` - MODIFIED: Added WarningColor, CriticalColor, UsageBarHeight, usageBarStyles, getter functions, and GetUsageBarStyles() composite function

## Code Review Record

### Reviewer
Claude Opus 4.5 (claude-opus-4-5-20251101) - Code Review Agent

### Review Date
2026-01-20

### Issues Found and Fixed

| Severity | Issue | Fix |
|----------|-------|-----|
| HIGH | Missing `GetUsageBarStyles()` composite function for dependency injection | Added `UsageBarStylesExport` struct and `GetUsageBarStyles()` function to `styles.go` |
| HIGH | Unused `ptr()` helper function in tests (dead code) | Removed unused function from `bar_test.go` |
| MEDIUM | `TestGetUtilizationStyle` didn't verify style selection | Rewrote test with `testStylesWithMarkers()` to verify correct style returned |
| MEDIUM | Warning/Critical threshold tests had dead code | Removed unused variables, added percentage verification |
| MEDIUM | `SetError()` didn't clear limits (inconsistent with other setters) | Added `m.limits = nil` to `SetError()`, added test coverage |

### Post-Review Coverage
94.2% on internal/usage package (up from 94.1%)
