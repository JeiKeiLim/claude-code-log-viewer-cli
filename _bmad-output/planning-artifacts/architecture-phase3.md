---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
status: complete
completedAt: '2026-01-16'
inputDocuments:
  - '_bmad-output/planning-artifacts/prd-phase3.md'
  - '_bmad-output/planning-artifacts/product-brief-cclv-phase3-2026-01-16.md'
  - '_bmad-output/project-context.md'
  - 'docs/architecture.md'
workflowType: 'architecture'
project_name: 'claude-code-log-viewer-cli'
user_name: 'Jongkuk Lim'
date: '2026-01-16'
phase: 3
status: in_progress
---

# Architecture Decision Document
## cclv Phase 3 - Power Tools & Dashboard

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**

| Epic | Requirements | Architectural Impact |
|------|--------------|---------------------|
| Epic 4: Developer Power Tools | FR-401 to FR-405 | ViewerModel state machine expansion |
| Epic 5: Dashboard Mode | FR-501 to FR-506 | New DashboardModel, watcher extension, navigation context |
| Epic 6: Statistics View | FR-601 to FR-603 | New dependency (tiktoken-go), token service |

**Non-Functional Requirements:**

| NFR | Constraint | Architectural Implication |
|-----|------------|--------------------------|
| Performance | <200ms dashboard refresh | Efficient grid rendering, lazy updates |
| Performance | <50ms token calculation | Caching strategy, async calculation |
| Memory | <100MB for 9 panes | Shared viewport buffers, lazy loading |
| Compatibility | vibe-dash integration | --color=always output path maintained |

**Scale & Complexity:**

- Primary domain: CLI/TUI Application
- Complexity level: Medium (brownfield extension)
- Estimated new architectural components: 3 (DashboardModel, TokenService, CommandMode)

### Technical Constraints & Dependencies

**From project-context.md:**
- No emoji in UI (text icons only)
- Makefile build system required
- Charm stack + approved additions only
- TEA pattern for all state changes
- List component height bug workaround

**New Dependency:**
- `github.com/pkoukk/tiktoken-go` - Token counting fallback (~4MB binary impact)

### Cross-Cutting Concerns Identified

1. **Navigation Context** - AppModel must track "came from Dashboard" vs "came from ConversationList" for correct back behavior
2. **Multi-Mode State Machines** - ViewerModel now has: normal mode, raw mode, command mode, search mode
3. **File Watching at Scale** - Single watcher watching multiple project directories and files
4. **Caching Strategy** - Token calculations and potentially rendered markdown
5. **Grid Layout Responsiveness** - Terminal resize handling for dashboard panes

---

## Starter Template Evaluation

### Primary Technology Domain

**CLI/TUI Application** - Brownfield extension of existing Go + Charm stack codebase.

### Existing Foundation (Not Starter Selection)

This is Phase 3 of an established project. No starter template selection needed.

**Established Technology Stack:**

| Layer | Technology | Version | Status |
|-------|------------|---------|--------|
| Language | Go | 1.24.3 | Locked |
| TUI Framework | Bubbletea | v1.3.10 | Locked (Charm coupling) |
| Styling | Lipgloss | v1.1.1 | Locked (Charm coupling) |
| Components | Bubbles | v0.21.0 | Locked (Charm coupling) |
| Markdown | Glamour | latest | Approved |
| File Watching | fsnotify | latest | Approved |
| Token Counting | tiktoken-go | TBD | New for Phase 3 |

**Architectural Patterns Established:**

| Pattern | Implementation | Constraint |
|---------|----------------|------------|
| TEA (Elm Architecture) | Model-Update-View | All state via Update() |
| Package Layering | cmd → tui → parser → types | No circular imports |
| Styling Centralization | styles.go | No inline styles |
| Text Icons | `[U]`, `[A]`, `[T]`, `[>]` | No emoji (FR-017) |
| Build System | Makefile | No raw go build |

**Extension Strategy for Phase 3:**

New components follow existing patterns:
- `DashboardModel` mirrors `ProjectModel`, `ConversationModel`, `ViewerModel`
- `internal/token/` follows `internal/parser/` package conventions
- ViewerModel modes use existing state machine patterns

---

## Core Architectural Decisions

### Decision Summary

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| 1 | Dashboard Pane Architecture | Slice of PaneModels | Dynamic sizing, memory scales with usage |
| 2 | Multi-File Watching | One watcher per pane | Reuses existing package, simple ownership |
| 3 | ViewerModel Mode State | Hybrid (rawMode bool + inputMode enum) | Raw is view toggle, command/search are input states |
| 4 | Navigation Context | Enum in AppModel | Simple, sufficient for current hierarchy |
| 5 | New Conversation Detection | Directory watch + timestamp compare | Detects new files, switches automatically |
| 6 | Grid Layout | Switch-based calculation | Matches PRD spec exactly |
| 7 | tiktoken Integration | internal/token/ package with cache | Isolated, cached, cl100k_base encoder |
| 8 | Toast System | AppModel toast + expiry | Reusable for path display and errors |
| 9 | Line Number Gutter | Fixed-width left column | Box index (normal) or JSONL line (raw) |
| 10 | Command Mode Input | Status bar prompt, digit capture | Standard vim-like pattern |

### Data Architecture

**No database** - CLI tool reads JSONL files directly. No persistence beyond file system.

**Caching Strategy:**
- Token calculations: in-memory map per conversation session
- Rendered markdown: existing cache in ViewerModel (from Phase 2)

### New Package: internal/token/

```go
package token

type Service struct {
    encoder tiktoken.Codec
    cache   map[string]int
}

func New() (*Service, error)           // Initialize cl100k_base encoder
func (s *Service) Calculate(text string) int
func (s *Service) ClearCache()
```

### Dashboard Component Structure

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
    conversation types.Conversation
    content      []string
    scrollOffset int
    watcher      *watcher.Watcher
}

func (m DashboardModel) Init() tea.Cmd
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m DashboardModel) View() string
```

### ViewerModel Extensions

```go
// Additions to internal/tui/viewer.go
type InputMode int
const (
    InputNone InputMode = iota
    InputCommand
    InputSearch
)

// New fields in ViewerModel
type ViewerModel struct {
    // ... existing fields

    // Phase 3 additions
    rawMode      bool
    inputMode    InputMode
    inputBuffer  string
    showLineNums bool  // always true, but configurable
}
```

### AppModel Extensions

```go
// Additions to internal/tui/app.go
type NavigationSource int
const (
    FromConversationList NavigationSource = iota
    FromDashboard
)

type AppModel struct {
    // ... existing fields

    // Phase 3 additions
    viewerSource NavigationSource
    toast        string
    toastExpiry  time.Time
}
```

### Grid Layout Algorithm

```go
func calculateGrid(count int) (rows, cols int) {
    switch {
    case count <= 1: return 1, 1
    case count <= 3: return 1, count
    case count <= 4: return 2, 2
    case count <= 6: return 2, 3
    default:         return 3, 3
    }
}
```

### Cross-Component Dependencies

```
AppModel
├── ProjectModel (multi-select for dashboard)
├── ConversationModel
├── ViewerModel (extended with raw/command modes)
├── DashboardModel (new)
│   └── []PaneModel
│       └── watcher.Watcher
└── token.Service (new)
```

---

## Implementation Patterns & Consistency Rules

### Existing Patterns (Reference)

All patterns from `project-context.md` remain in effect. Key ones for Phase 3:

| Pattern | Rule | Enforcement |
|---------|------|-------------|
| No emoji | Text icons only: `[U]`, `[A]`, `[T]`, `[>]`, `[D]` | FR-017 |
| TEA pattern | All state changes via `Update()` | Architecture |
| List truncation | Always truncate `list.View()` output | Bug workaround |
| Makefile | `make build`, `make test` | Build system |
| Error wrapping | `fmt.Errorf("context: %w", err)` | Go idiom |

### Phase 3 Naming Patterns

**Message Types:**
```go
// Pattern: {feature}{action}Msg (unexported, descriptive)
type dashboardLoadedMsg struct{ panes []PaneModel }
type paneContentUpdatedMsg struct{ index int }
type toastExpiredMsg struct{}
```

**Enum Constants:**
```go
// Pattern: {Type}{Value} grouping
const (
    InputNone InputMode = iota
    InputCommand
    InputSearch
)
```

**New Text Icons:**
```go
// Dashboard pane indicators
const (
    PaneFocusedIcon   = "[*]"
    PaneUnfocusedIcon = "[ ]"
    PaneWatchingIcon  = "[~]"  // live updating
)
```

### Structure Patterns

**New Files (follow existing structure):**
```
internal/tui/
├── dashboard.go      // DashboardModel, PaneModel
└── dashboard_test.go

internal/token/
├── service.go        // Service struct, Calculate()
└── service_test.go
```

**No new packages beyond:**
- `internal/token/` - token calculation service

### Process Patterns

**Toast Messages:**
- Duration: 1-3 seconds based on importance
- Format: Short, no trailing period
- Examples: "Path: /foo/bar.jsonl", "Invalid line number", "Jumped to line 42"

**Mode Transitions:**
```
Normal ─────r────→ Raw ─────r────→ Normal
   │                 │
   :                 :
   ↓                 ↓
Command ──Enter──→ Normal (with action)
   │                 │
   Esc               Esc
   ↓                 ↓
Normal              Raw
```

**Grid Resize Handling:**
- Recalculate grid on `tea.WindowSizeMsg`
- Preserve focus index if still valid
- Clamp focus if pane count reduced

### Anti-Patterns to Avoid

| Don't | Do |
|-------|-----|
| `dashboard_pane.go` | `dashboard.go` (keep related types together) |
| `type pane struct` | `type PaneModel struct` (explicit Model suffix) |
| Inline styles in View() | Use `styles.go` |
| `panic()` on parse error | Return error, increment `ParseErrors` |
| `go build` directly | `make build` |

---

## Project Structure & Boundaries

### Current Structure (Brownfield)

```
claude-code-log-viewer-cli/
├── cmd/
│   └── cclv/
│       └── main.go              # Entry point, mode detection
├── internal/
│   ├── parser/
│   │   ├── entry.go             # LogEntry parsing
│   │   ├── entry_test.go
│   │   ├── jsonl.go             # JSONL file parsing
│   │   └── jsonl_test.go
│   ├── scanner/
│   │   ├── projects.go          # Project/conversation discovery
│   │   └── projects_test.go
│   ├── tui/
│   │   ├── app.go               # AppModel, view routing
│   │   ├── project.go           # ProjectModel
│   │   ├── conversation.go      # ConversationModel
│   │   ├── viewer.go            # ViewerModel
│   │   ├── styles.go            # All styles, colors, icons
│   │   ├── utils.go             # Formatting helpers
│   │   └── plain.go             # Non-TUI output
│   ├── types/
│   │   ├── entry.go             # LogEntry, Message types
│   │   ├── conversation.go      # Conversation type
│   │   └── project.go           # Project type
│   ├── version/
│   │   └── version.go           # Build-time version
│   └── watcher/
│       └── watcher.go           # File watching (Phase 2)
├── docs/
├── Makefile
├── go.mod
└── go.sum
```

### Phase 3 Additions

```
internal/
├── tui/
│   ├── dashboard.go         # NEW: DashboardModel, PaneModel
│   ├── dashboard_test.go    # NEW: Dashboard tests
│   ├── app.go               # MODIFIED: +NavigationSource, +toast
│   ├── viewer.go            # MODIFIED: +rawMode, +inputMode, +lineNums
│   └── styles.go            # MODIFIED: +dashboard styles, +pane icons
├── token/                   # NEW PACKAGE
│   ├── service.go           # Token calculation service
│   └── service_test.go      # Token service tests
└── watcher/
    └── watcher.go           # UNCHANGED (reused per-pane)
```

### Architectural Boundaries

**Package Dependencies (allowed):**
```
cmd/cclv/main.go
    → internal/tui (AppModel, modes)
    → internal/version

internal/tui
    → internal/parser
    → internal/scanner
    → internal/types
    → internal/watcher
    → internal/token (NEW)

internal/parser
    → internal/types

internal/scanner
    → internal/types
    → internal/parser (for metadata extraction)

internal/token
    → (external: tiktoken-go)

internal/watcher
    → (external: fsnotify)
```

**No circular imports allowed.**

### Epic to Structure Mapping

| Epic | Primary Files | Modified Files |
|------|---------------|----------------|
| Epic 4: Power Tools | - | `viewer.go`, `styles.go`, `app.go` |
| Epic 5: Dashboard | `dashboard.go` | `app.go`, `project.go`, `styles.go` |
| Epic 6: Statistics | `token/service.go` | `viewer.go`, `types/entry.go` |

---

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
- All decisions use existing Go + Charm stack patterns
- New packages (`token/`) follow existing conventions
- TEA pattern maintained throughout

**Pattern Consistency:**
- Naming follows `project-context.md` rules
- Message types use established `{feature}{action}Msg` pattern
- Enum constants grouped by type

**Structure Alignment:**
- New files placed in established directories
- No new top-level packages beyond `internal/token/`
- Dependency flow preserved

### Requirements Coverage Validation ✅

**Epic 4: Developer Power Tools**
- FR-401 (Line Numbers): ViewerModel + gutter rendering ✅
- FR-402 (`:N` Navigation): InputMode + command parsing ✅
- FR-403 (Raw Mode): rawMode bool + rendering path ✅
- FR-404 (Toast Path): Toast system in AppModel ✅
- FR-405 (Newline Fix): Post-processing in markdown render ✅

**Epic 5: Dashboard Mode**
- FR-501 (Multi-Select): ProjectModel selection state ✅
- FR-502 (Grid Layout): calculateGrid() algorithm ✅
- FR-503 (Multi-Watch): PaneModel + watcher per pane ✅
- FR-504 (New Conversation): Directory watching ✅
- FR-505 (Pane Focus): focusIndex + arrow navigation ✅
- FR-506 (Navigation): NavigationSource enum ✅

**Epic 6: Statistics View**
- FR-601 (tiktoken-go): internal/token/ package ✅
- FR-602 (Token Fallback): Service.Calculate() ✅
- FR-603 (Statistics Display): Viewer integration ✅

**NFR Coverage:**
- Performance (<200ms, <50ms): Caching strategies defined ✅
- Memory (<100MB): Lightweight PaneModel design ✅
- Compatibility: Existing patterns maintained ✅

### Implementation Readiness Validation ✅

**Decision Completeness:**
- 10 architectural decisions documented
- All with rationale and code examples
- Versions locked (Charm stack coupling)

**Structure Completeness:**
- All new files specified
- Package boundaries defined
- Dependency flow documented

**Pattern Completeness:**
- Naming patterns for Phase 3 additions
- Mode transition diagram
- Anti-patterns listed

### Gap Analysis Results

**No Critical Gaps** - All requirements have architectural support.

**Minor Enhancements (Future):**
- Pane independent scrolling (deferred per PRD)
- Token calculation caching persistence (not needed)

---

## Architecture Completion Summary

### Workflow Completion

**Architecture Decision Workflow:** COMPLETED ✅
**Total Steps Completed:** 8
**Date Completed:** 2026-01-16
**Document Location:** `_bmad-output/planning-artifacts/architecture-phase3.md`

### Final Architecture Deliverables

| Deliverable | Status |
|-------------|--------|
| Project Context Analysis | ✅ |
| Technology Stack (brownfield) | ✅ |
| 10 Architectural Decisions | ✅ |
| Implementation Patterns | ✅ |
| Project Structure | ✅ |
| Validation | ✅ |

### Implementation Handoff

**For AI Agents:**
This architecture document is your guide for implementing cclv Phase 3. Follow all decisions, patterns, and structures exactly as documented.

**Implementation Sequence:**
1. Epic 4: Developer Power Tools (foundational viewer changes)
2. Epic 5: Dashboard Mode (new component + watcher extension)
3. Epic 6: Statistics View (new package + viewer integration)

**First Implementation Priority:**
Story 4.1 - Line numbers + gutter (smallest scope, validates ViewerModel changes)

---

**Architecture Status:** READY FOR IMPLEMENTATION ✅

**Next Phase:** Create Epics & Stories using `/bmad:bmm:workflows:create-epics-and-stories`

