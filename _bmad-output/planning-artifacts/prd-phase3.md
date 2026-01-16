---
stepsCompleted: ['step-01-init', 'step-02-discovery', 'step-03-success', 'step-04-users', 'step-05-functional', 'step-06-nonfunctional', 'step-07-constraints', 'step-08-scope', 'step-09-risks', 'step-10-dependencies', 'step-11-complete']
inputDocuments:
  - '_bmad-output/planning-artifacts/product-brief-cclv-phase3-2026-01-16.md'
  - '_bmad-output/project-context.md'
  - '_bmad-output/planning-artifacts/prd.md'
  - '_bmad-output/planning-artifacts/research/technical-tui-streaming-markdown-polish-research-2026-01-15.md'
  - '_bmad-output/planning-artifacts/research/list-view-polish-research-2026-01-16.md'
  - 'docs/architecture.md'
  - 'docs/development-guide.md'
  - 'docs/lessons-learned.md'
workflowType: 'prd'
projectType: 'brownfield'
phase: 3
status: complete
classification:
  projectType: 'CLI/TUI Application'
  domain: 'Developer Tools'
  complexity: 'Medium'
  projectContext: 'brownfield'
---

# Product Requirements Document
## cclv Phase 3 - Power Tools & Dashboard

**Author:** Jongkuk Lim
**Date:** 2026-01-16
**Version:** 1.0
**Status:** Complete

---

## 1. Executive Summary

cclv Phase 3 extends the Claude Code Log Viewer with developer power tools, multi-project dashboard monitoring, and token usage statistics. Building on the completed Phase 2 foundation (visual polish, real-time watching, markdown rendering), Phase 3 addresses friction points discovered through daily use.

**Scope:** Three epics, approximately 16 stories
**Timeline:** Self-paced personal project
**Success Criteria:** Subjective satisfaction + technical performance targets

---

## 2. Product Vision

### Problem Statement

While using cclv daily, several friction points and curiosities emerged:

1. **Log Analysis Friction** - When analyzing Claude Code log structure, can't see JSONL line numbers or raw content. Must exit cclv and use external tools like `jq`.

2. **Multi-Project Monitoring** - Running multiple Claude sessions across projects requires switching between them manually. No way to monitor all at once.

3. **Understanding Token Usage** - Curious about token consumption patterns, but logs have inconsistent token data. Want visibility into what Claude is actually using.

### Solution

Enhance cclv with three feature sets:

1. **Developer Power Tools** - Line numbers, raw mode, `:N` navigation, path display
2. **Dashboard Mode** - Multi-project grid monitoring with auto-refresh
3. **Statistics View** - Token counts with tiktoken-go fallback for missing data

### Success Criteria

- All three feature sets implemented and working
- Dashboard refresh < 200ms per pane
- tiktoken calculation < 50ms per entry
- Binary size increase < 5MB (from tiktoken)

---

## 3. Target Users

### Primary User

**Jongkuk Lim** - Developer who built and uses cclv daily

**Usage Context:**
- Reviews Claude Code conversation logs after coding sessions
- Monitors multiple Claude sessions across projects
- Integrates cclv with vibe-dash for decorated log viewing (`cclv --color=always`)
- Occasionally needs to analyze raw log structure for debugging

**What Success Looks Like:**
- Can analyze log structure without leaving cclv
- Can monitor multiple projects simultaneously
- Can see token usage patterns at a glance

### Secondary Users

Open-source users who discover cclv on GitHub. No special accommodation.

---

## 4. Functional Requirements

### FR-400: Developer Power Tools

#### FR-401: Line Numbers Display
- **Description:** Show line numbers in viewer - JSONL line numbers in raw mode, box numbers in normal mode
- **Implementation:** Add gutter column on left side of viewer content
- **Acceptance Criteria:**
  - [ ] Normal mode shows box numbers (1, 2, 3...) in gutter
  - [ ] Raw mode shows JSONL line numbers in gutter
  - [ ] Gutter styling matches Theme colors
  - [ ] Numbers right-aligned in gutter

#### FR-402: Vim-Style Line Navigation
- **Description:** Implement `:N` command to jump to specific line/box
- **Implementation:** Add command mode triggered by `:` key, parse number input
- **Acceptance Criteria:**
  - [ ] `:` key enters command mode
  - [ ] Typing number and Enter jumps to that line/box
  - [ ] Escape cancels command mode
  - [ ] Invalid input shows error toast
  - [ ] In normal mode, `:39` jumps to box 39
  - [ ] In raw mode, `:39` jumps to JSONL line 39

#### FR-403: Raw JSONL Mode Toggle
- **Description:** Toggle between parsed view and raw JSONL content
- **Implementation:** Add `r` key toggle, render raw JSON with syntax highlighting
- **Acceptance Criteria:**
  - [ ] `r` key toggles raw mode on/off
  - [ ] Raw mode shows actual JSONL content (like `jq`)
  - [ ] Raw mode preserves JSON formatting
  - [ ] Status bar indicates current mode
  - [ ] Scrolling works in both modes

#### FR-404: Toast Path Display
- **Description:** Show current file path on demand as toast notification
- **Implementation:** Add key binding to display path as temporary toast overlay
- **Acceptance Criteria:**
  - [ ] Key binding (e.g., `p`) shows file path as toast
  - [ ] Toast disappears after 3 seconds
  - [ ] Toast doesn't clutter permanent UI
  - [ ] Full absolute path displayed

#### FR-405: Newline Normalization Fix
- **Description:** Normalize 3+ consecutive newlines to 2 in markdown rendering
- **Implementation:** Add post-processing step to Glamour output
- **Acceptance Criteria:**
  - [ ] 3+ consecutive newlines reduced to 2
  - [ ] Single and double newlines preserved
  - [ ] Fix applied only to assistant text content

### FR-500: Dashboard Mode

#### FR-501: Multi-Project Selection
- **Description:** Allow selecting multiple projects from project list
- **Implementation:** Add `Space` to toggle selection, track selected projects
- **Acceptance Criteria:**
  - [ ] `Space` toggles project selection
  - [ ] Visual indicator shows selected state
  - [ ] Maximum 9 projects can be selected
  - [ ] `Esc` cancels selection mode
  - [ ] `Enter` with selections opens dashboard

#### FR-502: Grid Layout Component
- **Description:** Auto-sizing grid layout for dashboard panes
- **Implementation:** Create new `DashboardModel` with grid rendering
- **Grid Sizing:**
  - 1 project: 1x1
  - 2 projects: 1x2
  - 3 projects: 1x3
  - 4 projects: 2x2
  - 5-6 projects: 2x3
  - 7-9 projects: 3x3
- **Acceptance Criteria:**
  - [ ] Grid auto-sizes based on project count
  - [ ] Panes fill available terminal space
  - [ ] Grid reflows on terminal resize
  - [ ] Each pane has border and project label

#### FR-503: Multi-Project Watch
- **Description:** Each pane shows latest conversation from its project
- **Implementation:** Extend watcher package for multiple file watching
- **Acceptance Criteria:**
  - [ ] Each pane watches its project's latest conversation
  - [ ] Content updates when watched file changes
  - [ ] Independent scroll position per pane (future)
  - [ ] Refresh rate < 200ms per pane

#### FR-504: New Conversation Detection
- **Description:** Auto-switch pane when new conversation starts in a project
- **Implementation:** Monitor project directory for new JSONL files
- **Acceptance Criteria:**
  - [ ] Pane detects new conversation file
  - [ ] Pane automatically switches to new conversation
  - [ ] Visual indicator when conversation switches
  - [ ] Old conversation accessible via history (future)

#### FR-505: Pane Focus Navigation
- **Description:** Navigate between panes using arrow keys
- **Implementation:** Track focused pane, highlight active pane
- **Acceptance Criteria:**
  - [ ] Arrow keys move focus between panes
  - [ ] Focused pane has distinct visual style
  - [ ] `Enter` on focused pane opens full viewer
  - [ ] Focus wraps at grid edges

#### FR-506: Dashboard Navigation Hierarchy
- **Description:** Proper back navigation between views
- **Implementation:** App tracks navigation context
- **Navigation Flow:**
  ```
  Project List --Space+Enter--> Dashboard --Enter--> Viewer
       ^                            ^                   |
       +--------h/esc---------------+-------h/esc------+
  ```
- **Acceptance Criteria:**
  - [ ] `h` or `Esc` from Viewer returns to Dashboard (if came from Dashboard)
  - [ ] `h` or `Esc` from Viewer returns to Conversation List (if came from there)
  - [ ] `h` or `Esc` from Dashboard returns to Project List
  - [ ] Navigation context preserved correctly

### FR-600: Statistics View

#### FR-601: tiktoken-go Integration
- **Description:** Add tiktoken-go dependency for token calculation
- **Implementation:** Add `github.com/pkoukk/tiktoken-go` to go.mod
- **Acceptance Criteria:**
  - [ ] Dependency added and builds successfully
  - [ ] Token encoder initialized for Claude models
  - [ ] Binary size increase < 5MB

#### FR-602: Token Calculation Fallback
- **Description:** Calculate tokens when log `usage` field is empty
- **Implementation:** Create token calculation service with caching
- **Acceptance Criteria:**
  - [ ] Detect entries with empty/missing `usage` data
  - [ ] Calculate tokens using tiktoken-go
  - [ ] Cache calculated results
  - [ ] Calculation time < 50ms per entry

#### FR-603: Statistics Display
- **Description:** Show token counts with source indicator
- **Implementation:** Add statistics section to viewer or dedicated panel
- **Display Format:**
  - When data available: "Tokens: 1,234 (from log)"
  - When calculated: "Tokens: ~1,200 (estimated)"
- **Acceptance Criteria:**
  - [ ] Token counts displayed in viewer
  - [ ] Source indicator (log vs estimated) shown
  - [ ] Total tokens for conversation summarized
  - [ ] Tool usage breakdown available

---

## 5. Non-Functional Requirements

### NFR-001: Performance

| Metric | Target | Measurement |
|--------|--------|-------------|
| Dashboard refresh | < 200ms per pane | Bubbletea message timing |
| tiktoken calculation | < 50ms per entry | Benchmark test |
| Memory (dashboard, 9 projects) | < 100MB | `go tool pprof` |
| Startup time | < 100ms (maintained) | `time cclv` |

### NFR-002: Compatibility

- **Terminal Support:** Works in standard terminals (iTerm2, Terminal.app, Alacritty, etc.)
- **Theme Support:** Adapts to light and dark terminal themes (existing)
- **Platform Support:** macOS, Linux (existing platforms)
- **Integration:** Maintains compatibility with vibe-dash (`--color=always` output)

### NFR-003: Code Quality

- **Test Coverage:** Maintain 90% coverage (existing requirement)
- **Architecture:** Follow TEA (Elm Architecture) pattern
- **Style:** Follow existing project conventions from project-context.md

---

## 6. Technical Constraints

### From project-context.md

| Constraint | Requirement |
|------------|-------------|
| No emoji in UI | Text icons only: `[U]`, `[A]`, `[T]`, `[>]`, etc. |
| Build system | Use Makefile (`make build`, `make test`) |
| Dependencies | Charm stack + approved additions |
| List component | Use ListViewport (not bubbles/list) |
| Import order | stdlib -> external -> internal |
| TEA pattern | All state changes via `Update()` |

### New Dependencies

| Package | Purpose | Binary Impact |
|---------|---------|---------------|
| `github.com/pkoukk/tiktoken-go` | Token counting | ~4MB |

### Architecture Considerations

**Dashboard Mode:**
- New `DashboardModel` in `internal/tui/dashboard.go`
- Extends existing `watcher` package for multi-file watching
- Parent (`app.go`) tracks navigation context for correct back behavior
- Grid layout uses `lipgloss.JoinHorizontal`/`JoinVertical`

**Raw Mode:**
- Toggle state in `ViewerModel`
- Different rendering path for raw JSONL vs parsed entries
- Line numbers in gutter (left margin)
- Share viewport component between modes

**Command Mode (`:N` navigation):**
- New state in `ViewerModel`: `commandMode bool`, `commandInput string`
- Intercept key events when in command mode
- Parse and validate input on Enter

---

## 7. Implementation Phases

| Epic | Features | Risk | Dependencies |
|------|----------|------|--------------|
| **Epic 4** | Developer Power Tools (line numbers, raw mode, navigation) | Low-Medium | None |
| **Epic 5** | Dashboard Mode (multi-select, grid, multi-watch) | Medium | Extends watcher |
| **Epic 6** | Statistics View (tiktoken, token display) | Low | None |

### Recommended Implementation Order

**Epic 4: Developer Power Tools** (foundational)
1. Story 4.1 - Line numbers + gutter
2. Story 4.2 - `:N` navigation (command mode)
3. Story 4.3 - Raw JSONL mode toggle
4. Story 4.4 - Toast path display
5. Story 4.5 - Fix newline bug

**Epic 5: Dashboard Mode** (builds on watcher)
1. Story 5.1 - Multi-select in project list
2. Story 5.2 - Grid layout component
3. Story 5.3 - Multi-project watcher
4. Story 5.4 - New conversation detection
5. Story 5.5 - Pane focus navigation
6. Story 5.6 - Dashboard <-> Viewer back navigation

**Epic 6: Statistics View** (independent)
1. Story 6.1 - Add tiktoken-go dependency
2. Story 6.2 - Token calculation service
3. Story 6.3 - Statistics display in viewer

---

## 8. Out of Scope

- Export functionality (noted for future)
- Conversation diff/compare
- Bookmarks
- Quick copy to clipboard (SSH limitation)
- Pane independent scrolling (deferred - can add later)
- Custom themes/configuration files
- Remote file support

---

## 9. Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| tiktoken-go encoding mismatch | Medium | Low | Use cl100k_base encoding, validate against known Claude outputs |
| Dashboard performance with 9 panes | Medium | Medium | Limit refresh rate, lazy render off-screen panes |
| Command mode conflicts with vim keys | Low | Low | Clear mode indicator, Esc always exits |
| Grid layout complexity | Medium | Low | Start with fixed ratios, optimize later |
| Multi-file watcher resource usage | Medium | Medium | Use single fsnotify watcher with multiple paths |

---

## 10. Dependencies

### External Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Bubbletea | v1.3.10 | TUI framework (existing) |
| Lipgloss | v1.1.1 | Styling (existing) |
| Bubbles | v0.21.0 | UI components (existing) |
| Glamour | latest | Markdown rendering (existing) |
| fsnotify | latest | File watching (existing) |
| tiktoken-go | latest | Token counting (new) |

### Internal Dependencies

| Component | Dependency |
|-----------|------------|
| Dashboard | watcher package (extends) |
| Raw mode | parser package (bypass for display) |
| Statistics | types.TokenUsage (existing) |
| Command mode | viewer keybinding system |

---

## 11. Acceptance Criteria Summary

### Phase 3 Complete When:

- [ ] **Developer Power Tools (Epic 4)**
  - [ ] Line numbers visible in viewer (box # or JSONL line #)
  - [ ] `:N` navigation works in both modes
  - [ ] `r` toggles raw JSONL mode
  - [ ] Path displayed as toast on demand
  - [ ] Newline bug fixed

- [ ] **Dashboard Mode (Epic 5)**
  - [ ] Can multi-select up to 9 projects
  - [ ] Grid layout auto-sizes correctly
  - [ ] Each pane shows latest conversation
  - [ ] Panes auto-switch on new conversation
  - [ ] Navigation: Dashboard <-> Viewer works correctly

- [ ] **Statistics View (Epic 6)**
  - [ ] Token counts displayed in viewer
  - [ ] Fallback calculation works for empty usage
  - [ ] Source indicator (log vs estimated) shown

- [ ] **Quality**
  - [ ] All tests pass
  - [ ] 90% test coverage maintained
  - [ ] Dashboard refresh < 200ms per pane
  - [ ] tiktoken calculation < 50ms per entry
  - [ ] Binary size increase < 5MB

---

## 12. Reference Documents

- **Phase 3 Product Brief:** `_bmad-output/planning-artifacts/product-brief-cclv-phase3-2026-01-16.md`
- **Phase 2 PRD:** `_bmad-output/planning-artifacts/prd.md`
- **Project Context:** `_bmad-output/project-context.md`
- **Architecture:** `docs/architecture.md`

---

## Next Steps

1. **Architecture** - `/bmad:bmm:workflows:create-architecture` - Technical decisions for dashboard, raw mode, command mode
2. **Epics & Stories** - `/bmad:bmm:workflows:create-epics-and-stories` - Break into implementation tasks

---

*PRD Complete - 2026-01-16*
*Phase 3: Power Tools & Dashboard*
