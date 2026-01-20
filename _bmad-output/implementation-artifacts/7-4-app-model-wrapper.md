# Story 7.4: App Model Wrapper

Status: done

## Story

As a **developer refactoring the app**,
I want **the root model to wrap all views with the usage bar**,
So that **usage is visible everywhere**.

## Acceptance Criteria

1. **AC-1: Usage Bar at Top**
   - Given the app starts
   - When any view renders (project list, conversation list, viewer, dashboard)
   - Then usage bar appears at the top
   - And view content appears below

2. **AC-2: Height Allocation**
   - Given terminal height is H
   - When a view renders
   - Then usage bar gets 1 line (UsageBarHeight constant)
   - And view gets H-1 lines

3. **AC-3: No Flicker on Navigation**
   - Given user navigates between views
   - When transition occurs
   - Then usage bar remains constant (no flicker)

4. **AC-4: Dashboard Compatibility**
   - Given dashboard is displayed
   - When rendered
   - Then usage bar appears above the grid
   - And grid layout adjusts to remaining height

5. **AC-5: Usage Fetch on Startup**
   - Given app starts
   - When Init() is called
   - Then usage fetch is initiated asynchronously
   - And UI does not block waiting for usage data

6. **AC-6: Loading State Display**
   - Given usage is being fetched
   - When usage bar renders
   - Then it shows "Loading usage..." (dimmed)

7. **AC-7: Normal Usage Display**
   - Given usage data is available
   - When usage bar renders
   - Then it displays formatted usage limits

8. **AC-8: Error State Handling**
   - Given credentials are not found or API fails
   - When usage bar renders
   - Then it shows appropriate error state (not-logged-in, error message)
   - And other app functionality works normally

## Tasks / Subtasks

- [x] Task 1: Add usage state to AppModel (AC: #5, #6, #7, #8)
  - [x] Subtask 1.1: Import `internal/usage` package
  - [x] Subtask 1.2: Add `usageBar *usage.UsageBarModel` field to AppModel
  - [x] Subtask 1.3: Add `usageClient *usage.Client` field to AppModel
  - [x] Subtask 1.4: Initialize usageBar and usageClient in NewAppModel()
  - [x] Subtask 1.5: Initialize usageBar and usageClient in NewAppModelWithError()

- [x] Task 2: Create usage message types (AC: #5, #6, #7, #8)
  - [x] Subtask 2.1: Define `usageFetchedMsg` struct with `limits *usage.UsageLimits`, `stale bool`, `err error`
  - [x] Subtask 2.2: Create `fetchUsage()` tea.Cmd that calls usage.GetOAuthToken() and client.FetchUsage()
  - [x] Subtask 2.3: Handle ErrNoCredentials, ErrKeychainNotFound -> SetNotLoggedIn()
  - [x] Subtask 2.4: Handle ErrTokenExpired -> SetError("Session expired")
  - [x] Subtask 2.5: Handle other errors -> SetError() with appropriate message or use stale data

- [x] Task 3: Update AppModel.Init() (AC: #5, #6)
  - [x] Subtask 3.1: Add fetchUsage() to tea.Batch() in Init()
  - [x] Subtask 3.2: Ensure usageBar starts in StateLoading

- [x] Task 4: Update AppModel.Update() for usage messages (AC: #6, #7, #8)
  - [x] Subtask 4.1: Handle usageFetchedMsg - update usageBar state based on result
  - [x] Subtask 4.2: On success: SetLimits(limits, stale)
  - [x] Subtask 4.3: On ErrNoCredentials/ErrKeychainNotFound: SetNotLoggedIn()
  - [x] Subtask 4.4: On ErrTokenExpired: SetError("Session expired")
  - [x] Subtask 4.5: On other error with stale data: SetLimits(lastGood, true)
  - [x] Subtask 4.6: On other error without stale data: SetError("Usage unavailable")

- [x] Task 5: Update AppModel.Update() for WindowSizeMsg (AC: #2, #4)
  - [x] Subtask 5.1: Update usageBar.SetWidth(width)
  - [x] Subtask 5.2: Calculate viewHeight = height - UsageBarHeight
  - [x] Subtask 5.3: Pass viewHeight to child views instead of full height

- [x] Task 6: Update AppModel.View() (AC: #1, #2, #3, #4)
  - [x] Subtask 6.1: Render usageBar.View() first
  - [x] Subtask 6.2: Use lipgloss.JoinVertical() to combine usageBar + currentView
  - [x] Subtask 6.3: Ensure loadingView() also includes usage bar
  - [x] Subtask 6.4: Test that usage bar appears in all states (projects, conversations, viewer, dashboard)

- [x] Task 7: Write comprehensive tests (AC: #1-8)
  - [x] Subtask 7.1: Create or extend `internal/tui/app_test.go`
  - [x] Subtask 7.2: Test Init() triggers fetchUsage() command
  - [x] Subtask 7.3: Test usageFetchedMsg handling for all cases (success, not-logged-in, expired, error)
  - [x] Subtask 7.4: Test View() includes usage bar in all view states
  - [x] Subtask 7.5: Test WindowSizeMsg passes correct height to child views
  - [x] Subtask 7.6: Ensure 90%+ coverage on new code

## Dev Notes

### Critical Implementation Details

**AppModel Changes:**

```go
// In internal/tui/app.go

import (
    "context"
    "errors"
    "time"
    // ... existing imports
    "github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/usage"
)

type AppModel struct {
    // ... existing fields

    // Usage monitoring (Story 7.4)
    usageBar    *usage.UsageBarModel
    usageClient *usage.Client
}

func NewAppModel(projects []types.Project) AppModel {
    // ... existing initialization

    // Initialize usage bar with styles
    styles := usage.UsageBarStyles{
        Container: GetUsageBarContainer(),
        Label:     GetUsageBarLabel(),
        Normal:    GetUsageBarNormal(),
        Warning:   GetUsageBarWarning(),
        Critical:  GetUsageBarCritical(),
        Dimmed:    GetUsageBarDimmed(),
        Stale:     GetUsageBarStale(),
    }

    return AppModel{
        // ... existing fields
        usageBar:    usage.NewUsageBarModel(styles),
        usageClient: usage.NewClient(),
    }
}
```

**Message Types:**

```go
// usageFetchedMsg carries the result of a usage API fetch.
type usageFetchedMsg struct {
    limits *usage.UsageLimits
    stale  bool
    err    error
}

// fetchUsage returns a command that fetches usage asynchronously.
func (m AppModel) fetchUsage() tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        token, err := usage.GetOAuthToken()
        if err != nil {
            return usageFetchedMsg{err: err}
        }

        limits, stale, err := m.usageClient.FetchUsage(ctx, token)
        return usageFetchedMsg{limits: limits, stale: stale, err: err}
    }
}
```

**Init() Changes:**

```go
func (m AppModel) Init() tea.Cmd {
    return tea.Batch(
        m.projectModel.Init(),
        tea.WindowSize(),
        m.fetchUsage(), // Add usage fetch on startup
    )
}
```

**Update() Handler for usageFetchedMsg:**

```go
case usageFetchedMsg:
    if msg.err != nil {
        // Handle specific error types
        if errors.Is(msg.err, usage.ErrNoCredentials) ||
           errors.Is(msg.err, usage.ErrKeychainNotFound) ||
           errors.Is(msg.err, usage.ErrKeychainTimeout) {
            m.usageBar.SetNotLoggedIn()
        } else if errors.Is(msg.err, usage.ErrTokenExpired) {
            m.usageBar.SetError("Session expired")
        } else if msg.limits != nil {
            // Error but have stale data
            m.usageBar.SetLimits(msg.limits, true)
        } else {
            m.usageBar.SetError("Usage unavailable")
        }
    } else {
        m.usageBar.SetLimits(msg.limits, msg.stale)
    }
    return m, nil
```

**WindowSizeMsg Handler Changes:**

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height

    // Update usage bar width
    m.usageBar.SetWidth(msg.Width)

    // Calculate available height for child views
    viewHeight := msg.Height - UsageBarHeight
    childMsg := tea.WindowSizeMsg{Width: msg.Width, Height: viewHeight}

    // Forward adjusted size to current view
    switch m.state {
    case viewProjects:
        newModel, cmd := m.projectModel.Update(childMsg)
        m.projectModel = newModel.(ProjectModel)
        return m, cmd
    // ... similar for other views
    }
```

**View() Changes:**

```go
func (m AppModel) View() string {
    usageBarView := m.usageBar.View()

    var contentView string
    if m.loading {
        contentView = m.loadingView()
    } else {
        switch m.state {
        case viewProjects:
            contentView = m.projectModel.View()
        case viewConversations:
            contentView = m.conversationModel.View()
        case viewViewer:
            contentView = m.viewerModel.View()
        case viewDashboard:
            contentView = m.dashboardModel.View()
        default:
            contentView = m.projectModel.View()
        }
    }

    return lipgloss.JoinVertical(lipgloss.Left, usageBarView, contentView)
}
```

**loadingView() Changes:**

```go
func (m AppModel) loadingView() string {
    loadingText := m.spinner.View() + " " + ListStyles.Loading.Render("Loading...")
    // Guard against uninitialized dimensions
    if m.width == 0 || m.height == 0 {
        return loadingText
    }
    // Calculate available height for content (excluding usage bar)
    viewHeight := m.height - UsageBarHeight
    return lipgloss.Place(
        m.width, viewHeight,
        lipgloss.Center, lipgloss.Center,
        loadingText,
    )
}
```

### Package Structure

```
internal/tui/
├── app.go               # MODIFIED: Add usageBar, usageClient, fetchUsage, update View()
├── app_test.go          # MODIFIED: Add tests for usage integration
├── styles.go            # NO CHANGE: Already has UsageBarHeight and style getters
└── ...
```

### Project Structure Notes

- No new files needed - modifications only to `internal/tui/app.go`
- Import `internal/usage` package for client and bar model
- Use existing style getter functions from `internal/tui/styles.go`

### Style Injection Pattern

The UsageBarModel requires styles via dependency injection. Use the composite getter function from styles.go:

```go
stylesExport := GetUsageBarStyles()
styles := usage.UsageBarStyles{
    Container: stylesExport.Container,
    Label:     stylesExport.Label,
    Normal:    stylesExport.Normal,
    Warning:   stylesExport.Warning,
    Critical:  stylesExport.Critical,
    Dimmed:    stylesExport.Dimmed,
    Stale:     stylesExport.Stale,
}
usageBar := usage.NewUsageBarModel(styles)
```

### Error Mapping

| Error | UsageBar State | Display |
|-------|---------------|---------|
| `usage.ErrNoCredentials` | `StateNotLoggedIn` | "Not logged in" |
| `usage.ErrKeychainNotFound` | `StateNotLoggedIn` | "Not logged in" |
| `usage.ErrKeychainTimeout` | `StateNotLoggedIn` | "Not logged in" |
| `usage.ErrTokenExpired` | `StateError` | "Session expired" |
| `usage.ErrAPITimeout` with stale | `StateStale` | Last values + "(stale)" |
| `usage.ErrAPIError` with stale | `StateStale` | Last values + "(stale)" |
| Any error without stale | `StateError` | "Usage unavailable" |

### Display Format Notes

- **5-hour window:** Shows utilization percentage AND reset countdown (e.g., `[5h: 35% 2h 15m]`)
- **7-day window:** Shows utilization percentage ONLY, no reset time (e.g., `[7d: 12%]`)
- This is intentional per Story 7.3 design - the 7-day reset time would show large values like "167h 30m" which is not useful

### Height Calculation

- UsageBarHeight = 1 (constant from styles.go)
- Child view height = terminal height - UsageBarHeight
- Pass adjusted height in WindowSizeMsg to all child views
- CRITICAL: Both the initial size pass AND subsequent resize events must use adjusted height

### Testing Strategy

**Unit Tests (app_test.go):**

**Note:** The `usageBar` field is unexported in AppModel. For unit tests, either:
1. Add an exported `UsageBarState() usage.UsageBarState` getter method to AppModel for testing
2. Test behavior indirectly via View() output
3. Use the Update() message handling and verify View() contains expected strings

Recommended approach: Add a thin getter for test access and/or test via View() output.

```go
// Optional: Add to app.go for test access
func (m AppModel) UsageBarState() usage.UsageBarState {
    return m.usageBar.State()
}

func TestAppModel_Init_FetchesUsage(t *testing.T) {
    m := NewAppModel([]types.Project{})
    cmd := m.Init()
    // Verify cmd includes fetchUsage command
    // This is tricky to test directly - may need to check for batch
}

func TestAppModel_UsageFetchedMsg_Success(t *testing.T) {
    m := NewAppModel([]types.Project{})

    limits := &usage.UsageLimits{
        FiveHour: &usage.UsageWindow{Utilization: 35.0},
        SevenDay: &usage.UsageWindow{Utilization: 12.0},
    }

    newModel, _ := m.Update(usageFetchedMsg{limits: limits, stale: false})
    m = newModel.(AppModel)

    // Verify usageBar state via getter (if added) or View() output
    if m.UsageBarState() != usage.StateNormal {
        t.Errorf("expected StateNormal, got %v", m.UsageBarState())
    }
}

func TestAppModel_UsageFetchedMsg_NotLoggedIn(t *testing.T) {
    m := NewAppModel([]types.Project{})

    newModel, _ := m.Update(usageFetchedMsg{err: usage.ErrNoCredentials})
    m = newModel.(AppModel)

    if m.UsageBarState() != usage.StateNotLoggedIn {
        t.Errorf("expected StateNotLoggedIn, got %v", m.UsageBarState())
    }
}

func TestAppModel_UsageFetchedMsg_KeychainTimeout(t *testing.T) {
    m := NewAppModel([]types.Project{})

    newModel, _ := m.Update(usageFetchedMsg{err: usage.ErrKeychainTimeout})
    m = newModel.(AppModel)

    // Keychain timeout should be treated as not-logged-in
    if m.UsageBarState() != usage.StateNotLoggedIn {
        t.Errorf("expected StateNotLoggedIn for KeychainTimeout, got %v", m.UsageBarState())
    }
}

func TestAppModel_UsageFetchedMsg_TokenExpired(t *testing.T) {
    m := NewAppModel([]types.Project{})

    newModel, _ := m.Update(usageFetchedMsg{err: usage.ErrTokenExpired})
    m = newModel.(AppModel)

    if m.UsageBarState() != usage.StateError {
        t.Errorf("expected StateError, got %v", m.UsageBarState())
    }
}

func TestAppModel_UsageFetchedMsg_StaleData(t *testing.T) {
    m := NewAppModel([]types.Project{})

    limits := &usage.UsageLimits{
        FiveHour: &usage.UsageWindow{Utilization: 35.0},
    }

    newModel, _ := m.Update(usageFetchedMsg{limits: limits, stale: true, err: usage.ErrAPITimeout})
    m = newModel.(AppModel)

    if m.UsageBarState() != usage.StateStale {
        t.Errorf("expected StateStale, got %v", m.UsageBarState())
    }
}

func TestAppModel_View_IncludesUsageBar(t *testing.T) {
    m := NewAppModel([]types.Project{})
    // Set dimensions via WindowSizeMsg
    m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}).(AppModel)

    view := m.View()

    // Usage bar should be at top (in loading state initially)
    if !strings.Contains(view, "Loading") {
        t.Error("View should include usage bar loading state")
    }
}

func TestAppModel_WindowSizeMsg_AdjustsChildHeight(t *testing.T) {
    m := NewAppModel([]types.Project{})

    newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
    m = newModel.(AppModel)

    // After update, internal height tracking should show adjusted value
    // projectModel should receive height - UsageBarHeight = 23
    // This is verified by checking if the view renders correctly
    // and doesn't overflow the terminal
}
```

### Critical Rules (from project-context.md)

- NO EMOJI in any output
- Use `make test` not raw `go test`
- Table-driven tests required where applicable
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Use errors.Is() for sentinel error checking
- All styles defined in `internal/tui/styles.go` - use getter functions

### Previous Story Learnings

**From Story 7.1 (OAuth Credential Access):**
- Sentinel errors: `ErrNoCredentials`, `ErrKeychainNotFound`, `ErrTokenExpired`
- `GetOAuthToken()` returns token string or error
- Soft-fail pattern: don't crash app on credential errors

**From Story 7.2 (Usage API Client):**
- `Client.FetchUsage(ctx, token)` returns `(limits, stale, error)`
- `stale=true` when returning lastGood data after error
- Cache TTL is 60 seconds (managed by client)
- `ErrAPITimeout`, `ErrAPIError` for API failures

**From Story 7.3 (Usage Bar Component):**
- `UsageBarModel` is a pure view component (no tea.Model)
- State managed externally via: `SetLoading()`, `SetLimits()`, `SetNotLoggedIn()`, `SetError()`
- `SetWidth(int)` for responsive rendering
- `State()` returns current state enum for testing
- `View()` returns rendered string

### Anti-Patterns to Avoid

1. **DO NOT** block Init() on usage fetch - must be async
2. **DO NOT** crash app if credentials missing - graceful degradation
3. **DO NOT** make usage bar a tea.Model - it's already a view component
4. **DO NOT** pass full height to child views - subtract UsageBarHeight
5. **DO NOT** render usage bar AFTER content - must be at top
6. **DO NOT** forget to update usageBar width on WindowSizeMsg
7. **DO NOT** use emoji in any state display
8. **DO NOT** log errors to stdout - use internal logging or ignore
9. **DO NOT** update NewAppModelWithError() differently than NewAppModel() - both need usage initialization
10. **DO NOT** forget to handle ErrKeychainTimeout - treat same as ErrKeychainNotFound (not-logged-in state)

### Architectural Considerations

**Async Usage Fetch:**
The usage fetch MUST be async to avoid blocking app startup. Use tea.Cmd pattern:

```go
func (m AppModel) Init() tea.Cmd {
    return tea.Batch(
        m.projectModel.Init(),
        tea.WindowSize(),
        m.fetchUsage(), // Async - returns usageFetchedMsg when complete
    )
}
```

**Thread Safety:**
- `usage.Client` handles its own caching with sync.RWMutex
- `UsageBarModel` state is managed by single-threaded Bubbletea Update loop
- No additional synchronization needed in AppModel

**Memory:**
- Single `usageBar` instance shared across all views
- Single `usageClient` instance for caching efficiency
- Minimal memory impact (few KB)

### Expected Commit Format

```
feat: wrap all views with persistent usage bar (Story 7.4)

Refactors AppModel to display usage bar at top of all views:
- Add usageBar and usageClient to AppModel
- Fetch usage asynchronously on Init()
- Handle usageFetchedMsg with error mapping
- Adjust child view heights by UsageBarHeight
- Combine usage bar and content view with JoinVertical

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4.md#Story-7.4]
- [Source: _bmad-output/project-context.md] - Critical rules and patterns
- [Source: internal/usage/bar.go] - UsageBarModel API
- [Source: internal/usage/client.go] - Client.FetchUsage() signature
- [Source: internal/usage/types.go] - Error types and UsageLimits
- [Source: internal/usage/credentials.go] - GetOAuthToken()
- [Source: internal/tui/styles.go] - UsageBarHeight, style getters
- [Source: internal/tui/app.go] - Current AppModel structure
- [Source: _bmad-output/implementation-artifacts/7-3-usage-bar-component.md] - Previous story patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- All 7 tasks completed with all subtasks implemented
- All acceptance criteria verified through tests
- 15 new tests added covering all Story 7.4 requirements
- Build and all tests pass successfully

### Code Review Fixes Applied (2026-01-20)

- **ISSUE 1-4 [HIGH] Fixed**: Height allocation now subtracts `UsageBarHeight` in all child view SetSize() calls:
  - `conversationsLoadedMsg` handler: conversation model gets `viewHeight`
  - `conversationLoadedMsg` handler: viewer model gets `viewHeight`
  - `conversationLoadedWithWatchMsg` handler: viewer model gets `viewHeight`
  - `DashboardSelectedMsg` handler: dashboard model gets `viewHeight`
- **ISSUE 5 [MEDIUM] Fixed**: Added tests for `viewViewer` and `viewDashboard` states - `TestAppModel_ViewerState_IncludesUsageBar`, `TestAppModel_DashboardState_IncludesUsageBar`, `TestAppModel_ConversationLoadedMsg_AdjustsHeight`, `TestAppModel_DashboardSelectedMsg_AdjustsHeight`
- **ISSUE 6 [MEDIUM] Fixed**: Extracted duplicate style injection code to `newUsageBarStyles()` helper function

### File List

- `internal/tui/app.go` - Modified: Added usageBar and usageClient fields, updated Init(), Update(), View(), loadingView(), added UsageBarState() getter, added newUsageBarStyles() helper, fixed height allocation in 4 message handlers
- `internal/tui/app_test.go` - Modified: Added 19 tests for Story 7.4 covering all acceptance criteria (15 original + 4 new for height allocation)
