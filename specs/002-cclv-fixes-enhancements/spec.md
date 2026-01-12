# Feature Specification: CCLV Fixes and Enhancements

**Feature Branch**: `002-cclv-fixes-enhancements`
**Created**: 2026-01-12
**Status**: Draft
**Input**: User description: "Plain text output mode, navigation bug fix, hyphen path handling, and viewer title improvements"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Plain Text Output Mode (Priority: P1)

As a user who wants to pipe cclv output to another TUI application or tool, I need a plain text output mode that bypasses the interactive TUI so that the formatted log content can be consumed by downstream applications.

**Why this priority**: This is a blocking issue for users who want to integrate cclv into pipelines with other TUI applications. Without this, the tool cannot be composed with other terminal tools.

**Independent Test**: Run `cat file.jsonl | cclv --plain` and verify formatted text output appears on stdout without entering TUI mode. Verify `cat file.jsonl | cclv | head` works when stdout is piped.

**Acceptance Scenarios**:

1. **Given** a JSONL log file, **When** user runs `cclv --plain file.jsonl`, **Then** formatted log content is printed to stdout without entering TUI mode
2. **Given** piped input, **When** user runs `cat file.jsonl | cclv --plain`, **Then** formatted output is printed to stdout
3. **Given** stdout is piped to another program, **When** user runs `cat file.jsonl | cclv | other-app`, **Then** cclv auto-detects piped stdout and outputs plain text (no TUI)
4. **Given** the `--plain` flag is used, **When** output is generated, **Then** ANSI colors are preserved for terminal display

---

### User Story 2 - Fix Navigation Double-Skip Bug (Priority: P1)

As a user navigating the project list, I expect each press of j/k or arrow keys to move exactly one item, but currently it appears to skip items (moving two at a time).

**Why this priority**: This is a critical usability bug that makes the application frustrating to use. Navigation is a core interaction.

**Independent Test**: Launch `cclv`, navigate project list with j/k keys, verify cursor moves exactly one item per keypress.

**Acceptance Scenarios**:

1. **Given** a project list with multiple items, **When** user presses 'j' once, **Then** cursor moves down exactly one item
2. **Given** a project list with multiple items, **When** user presses 'k' once, **Then** cursor moves up exactly one item
3. **Given** a project list, **When** user presses down arrow once, **Then** cursor moves down exactly one item
4. **Given** a project list, **When** user presses up arrow once, **Then** cursor moves up exactly one item

---

### User Story 3 - Fix Hyphen in Project Path Handling (Priority: P1)

As a user with projects that have hyphens in their directory names (e.g., `/Users/me/my-project`), I expect the project to be correctly recognized. Currently, hyphens in project names are being incorrectly interpreted as path separators.

**Why this priority**: This is a data corruption bug that causes projects to be incorrectly identified, breaking core functionality for users with hyphenated directory names.

**Independent Test**: Create a project with a hyphenated name (e.g., `my-cool-project`), run `cclv`, verify the project appears with the correct name and path.

**Acceptance Scenarios**:

1. **Given** a project at path `/Users/me/my-project`, **When** cclv scans projects, **Then** it displays as "my-project" (not "my/project")
2. **Given** a project with multiple hyphens `/Users/me/foo-bar-baz`, **When** displayed in list, **Then** shows correctly as "foo-bar-baz"
3. **Given** a deeply nested path with hyphens `/Users/me/GitHub/my-org/my-repo`, **When** decoded, **Then** preserves hyphens in each component

---

### User Story 4 - Informative Viewer Title (Priority: P2)

As a user viewing logs, I want the title bar to show which project and conversation I'm viewing so I know what context I'm looking at, both in TUI and pipeline modes.

**Why this priority**: This is a usability enhancement that provides important context. Lower priority than bugs but improves user experience significantly.

**Independent Test**: Open a conversation in TUI mode, verify title shows project name and/or conversation identifier. Run in pipeline mode, verify output header shows file being viewed.

**Acceptance Scenarios**:

1. **Given** viewing a conversation from project browser, **When** log viewer opens, **Then** title shows project name and conversation date/time
2. **Given** viewing a file via pipeline (`cclv file.jsonl`), **When** viewer opens, **Then** title shows the filename being viewed
3. **Given** viewing via stdin pipe (`cat file.jsonl | cclv`), **When** viewer opens, **Then** title indicates "stdin" or similar
4. **Given** plain text output mode, **When** output begins, **Then** a header line shows the source being displayed

---

### Edge Cases

- What happens when a project path contains only hyphens (e.g., `---`)?
- What happens when `--plain` flag is used with no input?
- What happens when stdout is redirected to a file (`cclv file.jsonl > output.txt`)?
- Plain mode outputs lines as-is without wrapping (downstream tools handle formatting)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `--plain` command-line flag to output formatted text without TUI
- **FR-002**: System MUST auto-detect when stdout is not a TTY and default to plain text output
- **FR-003**: System MUST allow `--tui` flag to force TUI mode even when stdout is piped
- **FR-004**: Plain text output MUST include ANSI color codes for terminal display
- **FR-005**: Navigation in project list MUST move exactly one item per keypress
- **FR-006**: Navigation in conversation list MUST move exactly one item per keypress
- **FR-007**: System MUST correctly decode project paths that contain hyphens in directory names
- **FR-008**: System MUST distinguish between path-separator hyphens and literal hyphens in names
- **FR-009**: Viewer title MUST display contextual information about what is being viewed
- **FR-010**: Viewer title MUST show project name when viewing from interactive browser
- **FR-011**: Viewer title MUST show filename when viewing a specific file
- **FR-012**: Plain text output MUST include a header identifying the source

### Key Entities

- **Output Mode**: Enumeration of display modes (TUI, Plain)
- **Source Context**: Information about what is being viewed (project name, file path, stdin)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can successfully pipe cclv output to other applications without TUI interference
- **SC-002**: Navigation moves exactly one item per keypress in all list views (0 double-skips)
- **SC-003**: 100% of projects with hyphens in names are correctly displayed
- **SC-004**: Users can identify what they are viewing from the title in under 2 seconds
- **SC-005**: Plain text output can be captured to a file and viewed correctly with `less -R`

## Clarifications

### Session 2026-01-12

- Q: How should plain mode handle very long lines? → A: No wrapping - output lines as-is (let terminal or downstream tool handle)

## Assumptions

- ANSI color codes are acceptable in plain text output (users can pipe through `cat` or use `--no-color` if needed in future)
- The existing Claude Code path encoding uses single hyphens for path separators and double hyphens for literal hyphens
- Viewport-based list navigation is the source of the double-skip bug (list component handles its own navigation)
