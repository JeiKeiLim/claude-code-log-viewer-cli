---
stepsCompleted: [1, 2, 3, 4, 5, 6]
inputDocuments:
  - '_bmad-output/planning-artifacts/product-brief-claude-code-log-viewer-cli-2026-01-15.md'
  - '_bmad-output/implementation-artifacts/epic-3-retro-2026-01-16.md'
  - '_bmad-output/project-context.md'
date: '2026-01-16'
author: 'Jongkuk Lim'
phase: 3
status: complete
---

# Product Brief: cclv Phase 3 - Power Tools & Dashboard

## Executive Summary

cclv Phase 3 continues the craft journey with features driven by curiosity and real usage patterns. After completing the "finished" state in Phase 2 (Visual Polish, Real-time Watching, Markdown Rendering), new ideas emerged from daily use. This phase adds developer power tools for log analysis, a multi-project dashboard for monitoring, and conversation statistics.

**Integration Context:** cclv integrates with vibe-dash, which uses `cclv --color=always` for decorated log viewing. These features enhance both standalone and integrated use cases.

---

## Core Vision

### Problem Statement

While using cclv daily, several friction points and curiosities emerged:

1. **Log Analysis Friction** - When analyzing Claude Code log structure, can't see JSONL line numbers or raw content. Must exit cclv and use external tools.

2. **Multi-Project Monitoring** - Running multiple Claude sessions across projects requires switching between them manually. No way to monitor all at once.

3. **Understanding Token Usage** - Curious about token consumption patterns, but logs have inconsistent token data. Want visibility into what Claude is actually using.

### Motivation

- **Power Tools** - Remove friction when debugging/analyzing logs
- **Dashboard** - "Mission control" for multi-project Claude sessions
- **Statistics** - Satisfy curiosity about token usage patterns

### Proposed Solution

Three feature sets, prioritized by utility:

1. **Developer Power Tools** - Line numbers, raw mode, `:N` navigation, path display
2. **Dashboard Mode** - Multi-project grid monitoring with auto-refresh
3. **Statistics View** - Token counts with tiktoken-go fallback for missing data

---

## Target Users

### Primary User

**Jongkuk Lim** - Developer who built cclv for personal use and vibe-dash integration.

**Usage Context:**
- Reviews Claude Code logs after coding sessions
- Monitors multiple Claude sessions across projects
- Integrates cclv with vibe-dash for decorated log viewing
- Occasionally needs to analyze raw log structure

**What Success Looks Like:**
- Can analyze log structure without leaving cclv
- Can monitor multiple projects simultaneously
- Can see token usage patterns at a glance

---

## Phase 3 Scope

### Epic 4: Developer Power Tools

**Goal:** Remove friction when analyzing Claude Code logs.

| Feature | Description | Priority |
|---------|-------------|----------|
| **Line Numbers** | Show JSONL line numbers in raw mode, box numbers in normal mode | High |
| **`:N` Navigation** | Vim-style jump to line/box N | High |
| **Raw JSONL Mode** | Toggle `r` to see actual JSONL content (like jq) | High |
| **Toast Path Display** | Show file path on demand as toast (not cluttering UI) | Medium |
| **Fix Newline Bug** | Normalize 3+ consecutive newlines to 2 in markdown | Low |

**Entry from:** Viewer (toggle with `r` for raw mode)

**Navigation:** `:39` jumps to line 39 in raw mode, box 39 in normal mode

### Epic 5: Dashboard Mode

**Goal:** Monitor multiple projects simultaneously with real-time updates.

| Feature | Description | Priority |
|---------|-------------|----------|
| **Multi-Select** | `Space` to select projects (up to 9), `Esc` to cancel | High |
| **Grid Layout** | Auto-sizing grid (1x1 → 3x3 based on count) | High |
| **Multi-Project Watch** | Each pane shows latest conversation from its project | High |
| **New Conversation Detection** | Auto-switch pane when new conversation starts | High |
| **Pane Focus** | Arrow keys to navigate panes, `Enter` to open in viewer | Medium |
| **Back Navigation** | Viewer → Dashboard → Project List hierarchy | Medium |

**Entry from:** Project list with multi-select

**Grid auto-sizing:**
- 1 project → 1x1
- 2 projects → 1x2
- 3 projects → 1x3
- 4 projects → 2x2
- 5-6 projects → 2x3
- 7-9 projects → 3x3

**Navigation:**
```
Project List ──Space+Enter──> Dashboard ──Enter──> Viewer
     ↑                            ↑                   │
     └────────h/esc───────────────┴───────h/esc───────┘
```

### Epic 6: Statistics View

**Goal:** Understand token usage patterns in conversations.

| Feature | Description | Priority |
|---------|-------------|----------|
| **tiktoken-go Integration** | Add dependency for token calculation | High |
| **Token Fallback** | Calculate tokens when log `usage` field is empty | High |
| **Statistics Panel** | Show token counts, tool usage breakdown | Medium |

**Token display:**
- When data available: "Tokens: 1,234 (from log)"
- When calculated: "Tokens: ~1,200 (estimated)"

**Technical approach:**
1. Use parsed `usage` data when present (already implemented)
2. Add tiktoken-go as fallback for entries with empty usage
3. Display with indicator showing data source

---

## Technical Constraints

### From project-context.md

| Constraint | Requirement |
|------------|-------------|
| No emoji in UI | Text icons only |
| Build system | Use Makefile |
| Dependencies | Charm stack + approved additions |
| TEA pattern | All state changes via `Update()` |

### New Dependencies

| Package | Purpose | Binary Impact |
|---------|---------|---------------|
| `github.com/pkoukk/tiktoken-go` | Token counting | ~4MB |

### Architecture Considerations

**Dashboard Mode:**
- New `DashboardModel` in `internal/tui/dashboard.go`
- Extends existing `watcher` package for multi-file watching
- Parent (app.go) tracks navigation context for correct back behavior

**Raw Mode:**
- Toggle state in `ViewerModel`
- Different rendering path for raw JSONL vs parsed entries
- Line numbers in gutter (left margin)

---

## Success Criteria

### Phase 3 Complete When:

- [ ] **Developer Power Tools**
  - [ ] Line numbers visible in viewer (box # or JSONL line #)
  - [ ] `:N` navigation works
  - [ ] `r` toggles raw JSONL mode
  - [ ] Path displayed as toast on demand
  - [ ] Newline bug fixed

- [ ] **Dashboard Mode**
  - [ ] Can multi-select up to 9 projects
  - [ ] Grid layout auto-sizes correctly
  - [ ] Each pane shows latest conversation
  - [ ] Panes auto-switch on new conversation
  - [ ] Navigation: Dashboard ↔ Viewer works

- [ ] **Statistics View**
  - [ ] Token counts displayed
  - [ ] Fallback calculation works for empty usage
  - [ ] Source indicator (log vs estimated)

### Technical Targets

| Metric | Target |
|--------|--------|
| Dashboard refresh | < 200ms per pane |
| tiktoken calculation | < 50ms per entry |
| Binary size increase | < 5MB (from tiktoken) |

---

## Out of Scope

- Export functionality (noted for future)
- Conversation diff/compare
- Bookmarks
- Quick copy to clipboard (SSH limitation)
- Pane independent scrolling (deferred)

---

## Implementation Order

### Recommended Sequence

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
6. Story 5.6 - Dashboard ↔ Viewer back navigation

**Epic 6: Statistics View** (independent)
1. Story 6.1 - Add tiktoken-go dependency
2. Story 6.2 - Token calculation service
3. Story 6.3 - Statistics display in viewer

---

## Reference Documents

- **Phase 2 Product Brief:** `_bmad-output/planning-artifacts/product-brief-claude-code-log-viewer-cli-2026-01-15.md`
- **Phase 2 PRD:** `_bmad-output/planning-artifacts/prd.md`
- **Epic 3 Retrospective:** `_bmad-output/implementation-artifacts/epic-3-retro-2026-01-16.md`
- **Project Context:** `_bmad-output/project-context.md`

---

## Next Steps

1. **PRD** - `/bmad:bmm:workflows:prd` - Detailed requirements
2. **Architecture** - `/bmad:bmm:workflows:create-architecture` - Technical decisions for dashboard, raw mode
3. **Stories** - `/bmad:bmm:workflows:create-epics-and-stories` - Implementation tasks

---

*Product Brief Complete - 2026-01-16*
*Phase 3: Power Tools & Dashboard*
