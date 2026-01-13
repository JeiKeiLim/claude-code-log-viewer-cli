# Feature Specification: UI Decoration and Metadata Improvements

**Feature Branch**: `003-ui-metadata-improvements`
**Created**: 2026-01-12
**Status**: Draft
**Input**: User description: "1. Project list and conversations list view can be decorated more (do not use emoji) 2. total token usage and other meta data can be shown. 3. Perhaps lazy loading on conversation list and log view? 4. --version command to check which version it is."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Version Command (Priority: P1)

As a user of cclv, I need to check which version of the tool I am running so that I can report issues accurately, verify I have the latest version, and ensure compatibility with my workflow.

**Why this priority**: Version identification is fundamental for debugging, support requests, and ensuring users know what features are available. It is a quick win with high value.

**Independent Test**: Run `cclv --version` and verify version information is displayed and the program exits.

**Acceptance Scenarios**:

1. **Given** the user runs `cclv --version`, **When** the command executes, **Then** the version number is displayed and the program exits
2. **Given** the user runs `cclv -v`, **When** the command executes, **Then** the version number is displayed (short flag alias)
3. **Given** the user runs `cclv --version`, **When** output is generated, **Then** it includes the program name and semantic version (e.g., "cclv v1.2.3")

---

### User Story 2 - Enhanced Project List Decoration (Priority: P2)

As a user browsing projects, I want the project list to be visually enhanced with better formatting so that I can quickly scan and identify projects, understand their status, and navigate more effectively.

**Why this priority**: Visual clarity improves usability and user experience significantly. Users spend time browsing the project list and need clear visual hierarchy.

**Independent Test**: Launch `cclv`, view the project list, and verify improved visual formatting with borders, separators, and clear headers.

**Acceptance Scenarios**:

1. **Given** the project list is displayed, **When** the user views it, **Then** projects are displayed with visual borders or separators between items
2. **Given** the project list is displayed, **When** the user views it, **Then** a header section shows the total number of projects
3. **Given** the project list is displayed, **When** the selected item changes, **Then** the selected project is clearly highlighted with distinctive styling
4. **Given** the project list is displayed, **When** the user views it, **Then** project names are formatted with consistent typography (no emojis)

---

### User Story 3 - Enhanced Conversation List Decoration (Priority: P2)

As a user browsing conversations within a project, I want the conversation list to display more visual structure so that I can quickly understand the conversation history and navigate efficiently.

**Why this priority**: Conversation lists can become long and require visual cues to navigate. Decoration improves scan-ability.

**Independent Test**: Select a project and view the conversation list, verify improved visual structure with decorations and metadata.

**Acceptance Scenarios**:

1. **Given** the conversation list is displayed, **When** the user views it, **Then** conversations are displayed with visual borders or separators
2. **Given** the conversation list is displayed, **When** the user views it, **Then** a header shows the project name and conversation count
3. **Given** the conversation list is displayed, **When** the user views it, **Then** conversation timestamps are formatted in a human-readable way
4. **Given** the conversation list is displayed, **When** the selected item changes, **Then** the selected conversation is clearly highlighted

---

### User Story 4 - Token Usage Display (Priority: P2)

As a user tracking API costs, I want to see the total token usage for each conversation so that I can understand my Claude API consumption and identify high-usage conversations.

**Why this priority**: Token usage directly relates to API costs. Users need visibility into this metric for budget management and understanding conversation complexity.

**Independent Test**: View a conversation list and verify token usage metadata is displayed for each conversation.

**Acceptance Scenarios**:

1. **Given** the conversation list is displayed, **When** the user views it, **Then** each conversation shows its total token count (input + output)
2. **Given** a conversation is selected in the log viewer, **When** metadata is displayed, **Then** token usage breakdown is visible (input tokens, output tokens)
3. **Given** multiple conversations exist, **When** viewing the list, **Then** token counts are formatted with thousands separators for readability (e.g., "12,345 tokens")
4. **Given** a conversation has no token data available, **When** displayed, **Then** it shows a placeholder or "N/A" instead of zero

---

### User Story 5 - Conversation Metadata Display (Priority: P3)

As a user analyzing my Claude usage patterns, I want to see additional metadata for each conversation such as duration, message count, and model used so that I can understand my usage patterns.

**Why this priority**: Metadata provides context but is less critical than core functionality. It enhances the power user experience.

**Independent Test**: View conversations and verify metadata like message count, duration, and model are displayed.

**Acceptance Scenarios**:

1. **Given** the conversation list is displayed, **When** the user views it, **Then** each conversation shows the number of messages/turns
2. **Given** the conversation list is displayed, **When** the user views it, **Then** conversation duration (start to end time) is displayed when available
3. **Given** the log viewer is open, **When** viewing a conversation, **Then** the model name used is displayed in the header or footer
4. **Given** metadata is not available for a field, **When** displayed, **Then** that field is gracefully omitted or shows "Unknown"

---

### User Story 6 - Lazy Loading for Conversation List (Priority: P3)

As a user with many conversations in a project, I want the conversation list to load progressively so that I can start browsing immediately without waiting for all conversations to load.

**Why this priority**: Performance optimization that improves UX for users with large conversation histories. Not blocking but improves perceived performance.

**Independent Test**: Open a project with many conversations and verify the list appears quickly with additional items loading as needed.

**Acceptance Scenarios**:

1. **Given** a project with many conversations, **When** opening the conversation list, **Then** initial items load within 1 second
2. **Given** a partially loaded conversation list, **When** the user scrolls to the bottom, **Then** more conversations are loaded automatically
3. **Given** lazy loading is in progress, **When** the user views the list, **Then** a loading indicator is shown for pending items
4. **Given** all conversations are loaded, **When** scrolling, **Then** no additional loading occurs

---

### User Story 7 - Lazy Loading for Log View (Priority: P3)

As a user viewing long conversation logs, I want the log view to load content progressively so that I can start reading immediately without waiting for the entire log to render.

**Why this priority**: Long conversations can have thousands of messages. Progressive loading improves perceived performance and responsiveness.

**Independent Test**: Open a long conversation log and verify content appears quickly with additional content loading as the user scrolls.

**Acceptance Scenarios**:

1. **Given** a conversation with many messages, **When** opening the log viewer, **Then** initial content loads within 1 second
2. **Given** a partially loaded log view, **When** the user scrolls down, **Then** more content is loaded and rendered
3. **Given** the user scrolls up to previously viewed content, **When** reaching that section, **Then** content is already cached and displays immediately
4. **Given** lazy loading is in progress, **When** the user views the log, **Then** a subtle loading indicator is shown

---

### Edge Cases

- What happens when version information cannot be determined (development build)? → Display "dev-<commit-hash>"
- What happens when a conversation has zero tokens recorded?
- What happens when lazy loading fails mid-scroll (file access error)?
- What happens when metadata fields contain unusually long values (truncation)?
- What happens when terminal width is too narrow for all metadata columns?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support `--version` and `-v` flags to display version information
- **FR-002**: Version output MUST include program name and semantic version number
- **FR-003**: System MUST display version and exit without entering TUI mode
- **FR-004**: Project list MUST display visual borders or separators between items
- **FR-005**: Project list MUST display a header with project count
- **FR-006**: Conversation list MUST display visual borders or separators between items
- **FR-007**: Conversation list MUST display a header with project name and conversation count
- **FR-008**: Selected items in lists MUST be highlighted with distinctive styling
- **FR-009**: System MUST display total token usage for each conversation
- **FR-010**: Token counts MUST be formatted with thousands separators
- **FR-011**: System MUST display message/turn count for each conversation
- **FR-012**: System MUST display conversation duration when timestamp data is available
- **FR-013**: Log viewer MUST display the model name used for the conversation
- **FR-014**: Conversation list MUST support lazy loading when conversation count exceeds 50
- **FR-015**: Log viewer MUST support progressive loading when message count exceeds 100
- **FR-016**: Loading indicators MUST be shown during lazy loading operations
- **FR-017**: All decorations MUST NOT use emoji characters (text-based only)

### Key Entities

- **Version Information**: Semantic version number, program name, optional build metadata
- **Token Usage**: Input token count, output token count, total token count
- **Conversation Metadata**: Message count, duration, model name, timestamps
- **Loading State**: Pending, loading, loaded, error states for lazy loading

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can identify the installed version in under 2 seconds via command flag
- **SC-002**: Users report improved visual clarity in project and conversation lists
- **SC-003**: Token usage is visible for 100% of conversations that have token data
- **SC-004**: Initial list view loads within 1 second regardless of total conversation count
- **SC-005**: Log viewer displays initial content within 1 second regardless of conversation length
- **SC-006**: Users can browse lists smoothly without perceivable lag during lazy loading
- **SC-007**: All UI elements are readable on 80-column terminals (minimum supported width)

## Clarifications

### Session 2026-01-13

- Q: What to display for development/untagged builds? → A: Show "dev" with git commit hash (e.g., "cclv dev-abc1234")
- Q: When should lazy loading activate? → A: When items exceed 50 (conversations) or 100 (log messages)

## Assumptions

- Semantic versioning (major.minor.patch) is used for version numbering
- Development builds display "dev-<commit-hash>" when version tag is unavailable
- Token usage data is available in the JSONL log format from Claude Code
- The Bubbletea framework supports efficient list virtualization for lazy loading
- Lazy loading thresholds: 50 conversations, 100 log messages (below these, load all at once)
- Border characters and separators use ASCII or box-drawing Unicode characters (not emoji)
- Metadata can be extracted from existing log entry structures without additional parsing
