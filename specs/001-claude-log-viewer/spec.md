# Feature Specification: Claude Code Log Viewer CLI (cclv)

**Feature Branch**: `001-claude-log-viewer`
**Created**: 2026-01-12
**Status**: Draft
**Input**: User description: "Claude Code Log Viewer CLI (cclv) - A TUI application for viewing Claude Code conversation logs. Features: 1) Interactive mode with project browser to navigate ~/.claude/projects/, 2) Conversation list sorted by latest, 3) Beautiful log viewer with vim keybindings (hjkl, gg, G, /), 4) Pipeline mode for stdin input (cat file.jsonl | cclv), 5) Renders user messages, assistant responses (text, thinking, tool_use), and preserves conversation threading"

## Clarifications

### Session 2026-01-12

- Q: Should thinking blocks be expanded or collapsed by default? → A: Collapsed by default (hidden, toggle to show)
- Q: How should project names be displayed from encoded directory names? → A: Show last path component only; if names collide, show last 2-3 components to disambiguate
- Q: How should large tool inputs be displayed? → A: Collapsed by default; when expanded, truncate at 200 chars with "..." and char count

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Pipeline Log Viewing (Priority: P1)

A developer wants to quickly view a specific Claude Code conversation log file from the command line. They pipe a JSONL file or pass it as an argument and see a beautifully formatted, scrollable view of the conversation.

**Why this priority**: This is the simplest use case that delivers immediate value. A developer can use it right away without learning navigation - just pipe and view. It validates the core rendering engine.

**Independent Test**: Can be fully tested by running `cat conversation.jsonl | cclv` and verifying the output displays formatted messages. Delivers immediate value as a "better jq" for Claude logs.

**Acceptance Scenarios**:

1. **Given** a valid JSONL file with Claude Code logs, **When** the user runs `cat file.jsonl | cclv`, **Then** the application displays a scrollable view with formatted messages showing user prompts, assistant responses, and tool usage.
2. **Given** stdin is not a TTY and contains JSONL data, **When** the application starts, **Then** it automatically enters pipeline mode and renders the content.
3. **Given** a file path argument, **When** the user runs `cclv path/to/conversation.jsonl`, **Then** the application opens that file directly in the viewer.
4. **Given** the user is viewing logs, **When** they press `j`/`k` or arrow keys, **Then** the view scrolls down/up respectively.
5. **Given** the user is viewing logs, **When** they press `q`, **Then** the application exits cleanly.

---

### User Story 2 - Interactive Project Browser (Priority: P2)

A developer launches `cclv` without arguments and sees a list of all their Claude Code projects. They can navigate to select a project and then browse conversations within it.

**Why this priority**: This enables discovery - users don't need to know file paths. It's the "full experience" but depends on the viewer (P1) working first.

**Independent Test**: Can be tested by running `cclv` and verifying the project list displays, navigation works, and selecting a project shows its conversations.

**Acceptance Scenarios**:

1. **Given** the user has Claude Code projects in `~/.claude/projects/`, **When** they run `cclv` with no arguments and stdin is a TTY, **Then** a project browser displays listing all projects.
2. **Given** the project browser is displayed, **When** the user presses `j`/`k` or arrow keys, **Then** the selection moves down/up through the project list.
3. **Given** a project is highlighted, **When** the user presses `Enter` or `l`, **Then** the conversation list for that project is displayed.
4. **Given** the user is in conversation list view, **When** they press `Escape` or `h`, **Then** they return to the project browser.
5. **Given** the project browser is displayed, **When** the user presses `/`, **Then** a search/filter input appears to filter projects by name.

---

### User Story 3 - Conversation List Navigation (Priority: P3)

After selecting a project, the developer sees a list of conversations sorted by most recent. They can select one to view its full log content.

**Why this priority**: Builds on P2 to complete the navigation flow. Requires project browser to work first.

**Independent Test**: Can be tested by navigating to any project and verifying conversations are listed, sorted by date, and selectable.

**Acceptance Scenarios**:

1. **Given** a project with multiple conversations, **When** the user views the conversation list, **Then** conversations are sorted by last modified date (most recent first).
2. **Given** the conversation list, **When** the user presses `j`/`k` or arrow keys, **Then** the selection moves through the list.
3. **Given** a conversation is highlighted, **When** the user presses `Enter` or `l`, **Then** the log viewer opens showing that conversation's content.
4. **Given** the user is viewing a conversation, **When** they press `Escape` or `h`, **Then** they return to the conversation list.
5. **Given** a conversation list item, **When** displayed, **Then** it shows the conversation date, first user message preview, and message count.

---

### User Story 4 - Advanced Log Viewer Features (Priority: P4)

A developer viewing logs wants to search within the conversation, jump to specific positions, and toggle visibility of thinking blocks.

**Why this priority**: These are power-user features that enhance usability but aren't required for basic functionality.

**Independent Test**: Can be tested by opening any conversation and verifying search, jump commands, and toggle features work.

**Acceptance Scenarios**:

1. **Given** the log viewer is open, **When** the user presses `/`, **Then** a search input appears.
2. **Given** a search term is entered, **When** the user presses `Enter`, **Then** the view scrolls to the first match and highlights it.
3. **Given** search results exist, **When** the user presses `n`/`N`, **Then** the view jumps to the next/previous match.
4. **Given** the log viewer is open, **When** the user presses `gg`, **Then** the view jumps to the top.
5. **Given** the log viewer is open, **When** the user presses `G`, **Then** the view jumps to the bottom.
6. **Given** thinking blocks are visible, **When** the user presses `t`, **Then** thinking blocks are collapsed/hidden.
7. **Given** thinking blocks are hidden, **When** the user presses `t`, **Then** thinking blocks are expanded/shown.
8. **Given** tool inputs are collapsed, **When** the user presses `i`, **Then** tool input parameters are expanded (truncated at 200 chars).
9. **Given** tool inputs are expanded, **When** the user presses `i`, **Then** tool input parameters are collapsed.

---

### Edge Cases

- What happens when `~/.claude/projects/` directory doesn't exist? Display a helpful message explaining where Claude Code stores logs and how to use pipeline mode instead.
- What happens when a project has no conversation files? Display an empty state message "No conversations found in this project."
- What happens when a JSONL file contains malformed JSON lines? Skip malformed lines and display a warning indicator showing "X lines skipped due to parse errors."
- What happens when the terminal is resized during viewing? The view should reflow and maintain approximate scroll position.
- What happens when piped input is still streaming (e.g., `tail -f`)? Display content as it arrives and allow scrolling through received content.
- What happens when a conversation file is very large (>10MB)? Load and display content progressively without blocking the UI.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST detect whether stdin is a TTY to determine interactive vs pipeline mode.
- **FR-002**: System MUST parse Claude Code JSONL format including entry types: `user`, `assistant`, `file-history-snapshot`.
- **FR-003**: System MUST render assistant message content types: `text`, `thinking`, `tool_use`.
- **FR-004**: System MUST support vim-style navigation keys: `h`, `j`, `k`, `l`, `gg`, `G`, `/`, `n`, `N`.
- **FR-005**: System MUST support standard navigation keys: arrow keys, Page Up/Down, Home/End.
- **FR-006**: System MUST display user messages with clear visual distinction from assistant messages.
- **FR-007**: System MUST display tool_use blocks showing the tool name. Input parameters MUST be collapsed by default (expandable via `i` key). When expanded, inputs MUST be truncated at 200 characters with "..." and total character count indicator.
- **FR-008**: System MUST allow toggling visibility of thinking blocks via `t` key. Thinking blocks MUST be collapsed (hidden) by default.
- **FR-009**: System MUST scan `~/.claude/projects/` for project directories in interactive mode.
- **FR-010**: System MUST list JSONL conversation files within selected projects.
- **FR-011**: System MUST sort conversations by last modified timestamp (descending).
- **FR-012**: System MUST exit cleanly when user presses `q` or `Ctrl+C`.
- **FR-013**: System MUST handle terminal resize events gracefully.
- **FR-014**: System MUST display timestamps in local timezone.
- **FR-015**: System MUST provide visual feedback when search finds no results.
- **FR-016**: System MUST decode project directory names to display the last path component (e.g., `swealog`). If multiple projects share the same last component, system MUST show additional parent components to disambiguate (e.g., `JeiKeiLim/swealog`).

### Key Entities

- **Project**: A directory under `~/.claude/projects/` representing a workspace. Has a name (derived from path - display last path component only, e.g., `swealog`; if names collide, show last 2-3 components to disambiguate, e.g., `JeiKeiLim/swealog`), full path, and contains multiple Conversations.
- **Conversation**: A single JSONL file containing a sequence of log entries. Has a file path, last modified timestamp, and contains multiple LogEntries.
- **LogEntry**: A single line/record from the JSONL file. Has a type (`user`, `assistant`, `file-history-snapshot`), UUID, parent UUID (for threading), timestamp, and content.
- **MessageContent**: The content within an assistant LogEntry. Can be of type `text`, `thinking`, or `tool_use`. Text contains markdown, thinking contains reasoning, tool_use contains tool name and JSON input.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view a piped JSONL file within 2 seconds of running the command.
- **SC-002**: Users can navigate from application launch to viewing a specific conversation in under 5 key presses.
- **SC-003**: Scrolling through conversation logs maintains smooth visual feedback (no perceptible lag).
- **SC-004**: Users can find specific content using search within 10 seconds of initiating search.
- **SC-005**: Application handles conversation files up to 100MB without crashing or excessive memory usage.
- **SC-006**: Users unfamiliar with the tool can successfully view a conversation within 1 minute of first launch (discoverable UI).
- **SC-007**: All vim navigation commands (hjkl, gg, G, /, n, N) work as expected by users familiar with vim.

## Assumptions

- Users have Claude Code installed and have existing conversation logs in `~/.claude/projects/`.
- The JSONL format used by Claude Code follows the observed structure with `type`, `uuid`, `parentUuid`, `timestamp`, and `message` fields.
- Terminal emulators support basic ANSI colors and cursor positioning (standard modern terminals).
- Users are comfortable with keyboard-based navigation (no mouse support required for MVP).
