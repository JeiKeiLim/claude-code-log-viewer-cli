---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - '_bmad-output/planning-artifacts/prd-phase3.md'
  - '_bmad-output/planning-artifacts/architecture-phase3.md'
phase: 3
status: complete
completedAt: '2026-01-16'
---

# claude-code-log-viewer-cli Phase 3 - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for cclv Phase 3 (Power Tools & Dashboard), decomposing the requirements from the PRD and Architecture into implementable stories.

## Requirements Inventory

### Functional Requirements

**FR-400: Developer Power Tools**
- FR-401: Line Numbers Display - Show line numbers in viewer (JSONL line in raw mode, box number in normal mode)
- FR-402: Vim-Style Line Navigation - Implement `:N` command to jump to specific line/box
- FR-403: Raw JSONL Mode Toggle - Toggle between parsed view and raw JSONL content with `r` key
- FR-404: Toast Path Display - Show current file path on demand as toast notification
- FR-405: Newline Normalization Fix - Normalize 3+ consecutive newlines to 2 in markdown rendering

**FR-500: Dashboard Mode**
- FR-501: Multi-Project Selection - Allow selecting multiple projects (up to 9) with Space key
- FR-502: Grid Layout Component - Auto-sizing grid layout (1x1 to 3x3) for dashboard panes
- FR-503: Multi-Project Watch - Each pane shows and watches latest conversation from its project
- FR-504: New Conversation Detection - Auto-switch pane when new conversation starts in project
- FR-505: Pane Focus Navigation - Navigate between panes using arrow keys
- FR-506: Dashboard Navigation Hierarchy - Proper back navigation (Viewer → Dashboard → Project List)

**FR-600: Statistics View**
- FR-601: tiktoken-go Integration - Add tiktoken-go dependency for token calculation
- FR-602: Token Calculation Fallback - Calculate tokens when log `usage` field is empty
- FR-603: Statistics Display - Show token counts with source indicator (log vs estimated)

### Non-Functional Requirements

**NFR-001: Performance**
- Dashboard refresh < 200ms per pane
- tiktoken calculation < 50ms per entry
- Memory (dashboard, 9 projects) < 100MB
- Startup time < 100ms (maintained)

**NFR-002: Compatibility**
- Terminal Support: iTerm2, Terminal.app, Alacritty, etc.
- Theme Support: Light and dark terminal themes
- Platform Support: macOS, Linux
- Integration: vibe-dash compatibility (`--color=always`)

**NFR-003: Code Quality**
- Test Coverage: Maintain 90%
- Architecture: Follow TEA (Elm Architecture) pattern
- Style: Follow project-context.md conventions

### Additional Requirements (from Architecture)

**Brownfield Constraints:**
- No starter template needed - extending existing codebase
- Follow established package layering: cmd → tui → parser/scanner → types
- Use Makefile build system (`make build`, `make test`)
- No emoji in UI (text icons only)

**Architectural Decisions:**
- Dashboard uses slice of PaneModels (dynamic 1-9 panes)
- One watcher per pane (reuses existing watcher package)
- ViewerModel: `rawMode bool` + `inputMode enum` (hybrid approach)
- Navigation context via enum in AppModel
- New `internal/token/` package for tiktoken-go integration
- Toast system in AppModel with expiry time
- Fixed-width line number gutter

### FR Coverage Map

| FR | Epic | Description |
|----|------|-------------|
| FR-401 | Epic 4 | Line numbers in gutter |
| FR-402 | Epic 4 | `:N` navigation command |
| FR-403 | Epic 4 | Raw JSONL mode toggle |
| FR-404 | Epic 4 | Toast path display |
| FR-405 | Epic 4 | Newline normalization fix |
| FR-501 | Epic 5 | Multi-project selection |
| FR-502 | Epic 5 | Grid layout component |
| FR-503 | Epic 5 | Multi-project watch |
| FR-504 | Epic 5 | New conversation detection |
| FR-505 | Epic 5 | Pane focus navigation |
| FR-506 | Epic 5 | Dashboard navigation hierarchy |
| FR-601 | Epic 6 | tiktoken-go integration |
| FR-602 | Epic 6 | Token calculation fallback |
| FR-603 | Epic 6 | Statistics display |

## Epic List

### Epic 4: Developer Power Tools
Users can analyze log structure without leaving cclv - see line numbers, view raw JSONL, jump to specific lines, and display file paths.

**FRs covered:** FR-401, FR-402, FR-403, FR-404, FR-405
**Standalone:** Yes - extends existing ViewerModel

### Epic 5: Dashboard Mode
Users can monitor multiple projects simultaneously with a grid layout that auto-updates when conversations change.

**FRs covered:** FR-501, FR-502, FR-503, FR-504, FR-505, FR-506
**Standalone:** Yes - new DashboardModel, reuses existing watcher

### Epic 6: Statistics View
Users can see token usage patterns with calculated fallback for missing log data.

**FRs covered:** FR-601, FR-602, FR-603
**Standalone:** Yes - new token package, integrates with ViewerModel

---

## Epic 4: Developer Power Tools

Users can analyze log structure without leaving cclv - see line numbers, view raw JSONL, jump to specific lines, and display file paths.

### Story 4.1: Line Numbers Display

As a **developer reviewing logs**,
I want **line numbers displayed in a gutter column**,
So that **I can reference specific locations when debugging**.

**Acceptance Criteria:**

**Given** the viewer is displaying a conversation in normal mode
**When** I view the content
**Then** box numbers (1, 2, 3...) appear in a left gutter column
**And** numbers are right-aligned and styled with dimmed text

**Given** the viewer is displaying a conversation in raw mode
**When** I view the content
**Then** JSONL line numbers appear in the gutter
**And** gutter width adjusts based on max line count

---

### Story 4.2: Command Mode Line Navigation

As a **developer analyzing logs**,
I want **to jump to a specific line using `:N` syntax**,
So that **I can quickly navigate to known locations**.

**Acceptance Criteria:**

**Given** I am in the viewer (normal or raw mode)
**When** I press `:`
**Then** command mode activates with `:` prompt in status bar
**And** subsequent digit keypresses are captured

**Given** I am in command mode with digits entered
**When** I press Enter
**Then** the viewer scrolls to that line/box number
**And** command mode exits

**Given** I am in command mode
**When** I press Escape
**Then** command mode cancels without navigation

**Given** I enter an invalid number (out of range)
**When** I press Enter
**Then** a toast displays "Invalid line number"
**And** command mode exits

---

### Story 4.3: Raw JSONL Mode Toggle

As a **developer debugging log structure**,
I want **to toggle between parsed view and raw JSONL**,
So that **I can see the actual log content like `jq`**.

**Acceptance Criteria:**

**Given** I am in the viewer in normal mode
**When** I press `r`
**Then** the view switches to raw JSONL content
**And** status bar indicates "RAW" mode

**Given** I am in raw mode
**When** I press `r`
**Then** the view switches back to parsed/rendered mode
**And** status bar shows normal mode

**Given** I am in raw mode
**When** I scroll through content
**Then** raw JSON lines display with preserved formatting
**And** scrolling works identically to normal mode

---

### Story 4.4: Toast Path Display

As a **developer using cclv**,
I want **to see the current file path on demand**,
So that **I know which conversation file I'm viewing**.

**Acceptance Criteria:**

**Given** I am viewing a conversation
**When** I press `p`
**Then** a toast displays the full absolute file path
**And** the toast disappears after 3 seconds

**Given** a toast is displaying
**When** 3 seconds elapse
**Then** the toast fades/disappears
**And** the UI returns to normal

---

### Story 4.5: Newline Normalization Fix

As a **user viewing assistant responses**,
I want **excessive blank lines normalized**,
So that **rendered markdown doesn't have awkward spacing**.

**Acceptance Criteria:**

**Given** assistant text content with 3+ consecutive newlines
**When** the markdown is rendered
**Then** consecutive newlines are reduced to maximum 2
**And** single and double newlines are preserved

**Given** user messages or tool content
**When** displayed
**Then** newline normalization is not applied

---

## Epic 5: Dashboard Mode

Users can monitor multiple projects simultaneously with a grid layout that auto-updates when conversations change.

### Story 5.1: Multi-Project Selection

As a **developer monitoring multiple projects**,
I want **to select multiple projects from the list**,
So that **I can open them in a dashboard view**.

**Acceptance Criteria:**

**Given** I am in the project list view
**When** I press Space on a project
**Then** the project is marked as selected with visual indicator
**And** I can continue selecting more projects

**Given** I have selected projects
**When** I press Space on a selected project
**Then** the selection is removed

**Given** I have 9 projects selected
**When** I try to select another
**Then** selection is prevented (max 9)

**Given** I have 1+ projects selected
**When** I press Enter
**Then** dashboard view opens with selected projects

**Given** I am in selection mode
**When** I press Escape
**Then** all selections are cleared

---

### Story 5.2: Grid Layout Component

As a **developer viewing the dashboard**,
I want **projects displayed in an auto-sizing grid**,
So that **I can see all monitored projects at once**.

**Acceptance Criteria:**

**Given** 1 project selected
**When** dashboard opens
**Then** grid displays as 1x1 (full screen pane)

**Given** 2-3 projects selected
**When** dashboard opens
**Then** grid displays as 1 row with 2-3 columns

**Given** 4 projects selected
**When** dashboard opens
**Then** grid displays as 2x2

**Given** 5-6 projects selected
**When** dashboard opens
**Then** grid displays as 2x3

**Given** 7-9 projects selected
**When** dashboard opens
**Then** grid displays as 3x3

**Given** terminal is resized
**When** dashboard is visible
**Then** panes reflow to fill available space

---

### Story 5.3: Dashboard Pane Content Display

As a **developer monitoring projects**,
I want **each pane to show the latest conversation**,
So that **I can see activity across projects**.

**Acceptance Criteria:**

**Given** a pane is initialized for a project
**When** it loads
**Then** it displays the latest conversation content
**And** shows project name in pane header

**Given** a pane is watching a conversation
**When** the conversation file changes
**Then** the pane content updates within 200ms

**Given** a pane has content
**When** displayed
**Then** it shows rendered messages (not raw JSONL)

---

### Story 5.4: New Conversation Detection

As a **developer with active Claude sessions**,
I want **panes to auto-switch to new conversations**,
So that **I always see the latest activity**.

**Acceptance Criteria:**

**Given** a pane is watching a project's latest conversation
**When** a new JSONL file is created in that project
**Then** the pane detects the new file
**And** switches to display the new conversation

**Given** a pane switches to a new conversation
**When** the switch occurs
**Then** a visual indicator briefly shows (e.g., flash or icon)

---

### Story 5.5: Pane Focus Navigation

As a **developer viewing the dashboard**,
I want **to navigate between panes with arrow keys**,
So that **I can select a pane to interact with**.

**Acceptance Criteria:**

**Given** I am in the dashboard view
**When** I press arrow keys
**Then** focus moves between panes in that direction

**Given** a pane is focused
**When** displayed
**Then** it has a distinct visual style (highlighted border)

**Given** a pane is focused
**When** I press Enter
**Then** the viewer opens for that pane's conversation

**Given** focus is at grid edge
**When** I press arrow toward edge
**Then** focus wraps to opposite side

---

### Story 5.6: Dashboard Navigation Hierarchy

As a **developer navigating between views**,
I want **back navigation to work correctly**,
So that **I return to the right parent view**.

**Acceptance Criteria:**

**Given** I opened Viewer from Dashboard
**When** I press `h` or Escape
**Then** I return to Dashboard (not Conversation List)

**Given** I opened Viewer from Conversation List
**When** I press `h` or Escape
**Then** I return to Conversation List (existing behavior)

**Given** I am in Dashboard view
**When** I press `h` or Escape
**Then** I return to Project List

---

## Epic 6: Statistics View

Users can see token usage patterns with calculated fallback for missing log data.

### Story 6.1: tiktoken-go Integration

As a **developer building token statistics**,
I want **tiktoken-go dependency added**,
So that **tokens can be calculated for entries missing usage data**.

**Acceptance Criteria:**

**Given** the project dependencies
**When** tiktoken-go is added
**Then** `make build` succeeds
**And** binary size increases by < 5MB

**Given** the token service initializes
**When** encoder is created
**Then** cl100k_base encoding is used (Claude-compatible)

---

### Story 6.2: Token Calculation Service

As a **developer building statistics**,
I want **a token calculation service with caching**,
So that **tokens can be calculated efficiently**.

**Acceptance Criteria:**

**Given** text content
**When** Calculate() is called
**Then** token count is returned

**Given** the same text is calculated twice
**When** Calculate() is called the second time
**Then** cached result is returned (no recalculation)

**Given** a calculation request
**When** processed
**Then** calculation completes in < 50ms

---

### Story 6.3: Statistics Display

As a **developer reviewing logs**,
I want **to see token counts in the viewer**,
So that **I understand token consumption patterns**.

**Acceptance Criteria:**

**Given** an entry has `usage` data in the log
**When** displayed
**Then** shows "Tokens: 1,234 (from log)"

**Given** an entry has no `usage` data
**When** displayed
**Then** calculates tokens and shows "Tokens: ~1,200 (estimated)"

**Given** a conversation is loaded
**When** viewing statistics
**Then** total tokens for conversation is summarized

