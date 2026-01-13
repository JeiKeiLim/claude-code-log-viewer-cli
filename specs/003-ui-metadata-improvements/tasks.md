# Tasks: UI Decoration and Metadata Improvements

**Input**: Design documents from `/specs/003-ui-metadata-improvements/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Not explicitly requested in spec - test tasks omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md structure:
- **CLI entry**: `cmd/cclv/main.go`
- **Version**: `internal/version/version.go` (NEW)
- **Types**: `internal/types/`
- **Parser**: `internal/parser/`
- **TUI**: `internal/tui/`
- **Scanner**: `internal/scanner/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create new package and foundational types needed across all user stories

- [x] T001 Create internal/version/version.go with Version, Commit, BuildDate variables and String()/Full() functions
- [x] T002 [P] Add TokenUsage struct to internal/types/entry.go with Total() and TotalInput() methods
- [x] T003 [P] Add RawTokenUsage struct to internal/types/entry.go for JSON parsing
- [x] T004 [P] Add formatWithCommas() helper function to internal/tui/utils.go for thousands separator formatting

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend existing types with metadata fields that multiple user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 Add Model and Usage fields to LogEntry struct in internal/types/entry.go
- [x] T006 Add Model and Usage fields to RawAssistantMessage in internal/types/entry.go
- [x] T007 Add TotalTokens, Model, Duration, TurnCount fields to Conversation struct in internal/types/conversation.go
- [x] T008 Update parseAssistantMessage() in internal/parser/entry.go to extract model and usage data
- [x] T009 Add LoadingState type and LazyLoadConfig struct to internal/tui/styles.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Version Command (Priority: P1) MVP

**Goal**: Users can check cclv version via --version or -v flag

**Independent Test**: Run `cclv --version` and verify version string is displayed and program exits

### Implementation for User Story 1

- [x] T010 [US1] Add -v and --version flag parsing to cmd/cclv/main.go
- [x] T011 [US1] Print version.String() and os.Exit(0) when version flag is set in cmd/cclv/main.go
- [x] T012 [US1] Update build instructions in README or Makefile with ldflags for version injection

**Checkpoint**: User Story 1 complete - `cclv --version` displays version and exits

---

## Phase 4: User Story 2 - Enhanced Project List Decoration (Priority: P2)

**Goal**: Project list has visual borders, separators, header with count, and improved highlighting

**Independent Test**: Launch `cclv`, verify project list has borders, header shows "(N projects)", selection is clearly highlighted

### Implementation for User Story 2

- [x] T013 [P] [US2] Add ListBorder and ListHeader styles to internal/tui/styles.go
- [x] T014 [P] [US2] Add horizontal separator style using box-drawing characters to internal/tui/styles.go
- [x] T015 [US2] Update ProjectModel.View() in internal/tui/project.go to render header with project count
- [x] T016 [US2] Update ProjectItemDelegate.Render() in internal/tui/project.go to add separators between items
- [x] T017 [US2] Enhance selected item styling in ProjectItemDelegate.Render() with border/background in internal/tui/project.go

**Checkpoint**: User Story 2 complete - Project list is visually decorated with borders and count header

---

## Phase 5: User Story 3 - Enhanced Conversation List Decoration (Priority: P2)

**Goal**: Conversation list has visual borders, header with project name and count, improved highlighting

**Independent Test**: Select a project, verify conversation list has borders, header shows "Conversations: <project> (N)", selection is highlighted

### Implementation for User Story 3

- [x] T018 [US3] Update ConversationModel.View() in internal/tui/conversation.go to render header with project name and count
- [x] T019 [US3] Update ConversationItemDelegate.Render() in internal/tui/conversation.go to add separators between items
- [x] T020 [US3] Enhance selected item styling in ConversationItemDelegate.Render() with border/background in internal/tui/conversation.go

**Checkpoint**: User Story 3 complete - Conversation list is visually decorated

---

## Phase 6: User Story 4 - Token Usage Display (Priority: P2)

**Goal**: Each conversation shows total token count formatted with thousands separators

**Independent Test**: View conversation list, verify token counts displayed (e.g., "12,345 tokens") or "N/A" if no data

### Implementation for User Story 4

- [x] T021 [US4] Update scanner.ScanConversations() in internal/scanner/projects.go to extract and sum token usage per conversation
- [x] T022 [US4] Update ConversationItem.Description() in internal/tui/conversation.go to include formatted token count
- [x] T023 [US4] Update ConversationItemDelegate.Render() in internal/tui/conversation.go to display token count with formatWithCommas()
- [x] T024 [US4] Handle zero/missing token data by displaying "N/A" in internal/tui/conversation.go

**Checkpoint**: User Story 4 complete - Token usage visible in conversation list

---

## Phase 7: User Story 5 - Conversation Metadata Display (Priority: P3)

**Goal**: Conversations show message count, duration, and model name

**Independent Test**: View conversations, verify message count and duration shown; view log, verify model name in header

### Implementation for User Story 5

- [x] T025 [US5] Update scanner.ScanConversations() in internal/scanner/projects.go to calculate duration and turn count
- [x] T026 [US5] Update ConversationItemDelegate.Render() in internal/tui/conversation.go to display duration and turn count
- [x] T027 [US5] Update ViewerModel header in internal/tui/viewer.go to display model name
- [x] T028 [US5] Handle missing metadata gracefully (omit field or show "Unknown") in internal/tui/conversation.go

**Checkpoint**: User Story 5 complete - Full metadata visible

---

## Phase 8: User Story 6 - Lazy Loading for Conversation List (Priority: P3)

**Goal**: Conversation lists with >50 items load progressively

**Independent Test**: Open project with 100+ conversations, verify initial load <1s, scrolling loads more items

### Implementation for User Story 6

- [x] T029 [US6] Add lazyLoadState and loadedCount fields to ConversationModel in internal/tui/conversation.go
- [x] T030 [US6] Create loadNextBatch() command that loads next 20 conversations in internal/tui/conversation.go
- [x] T031 [US6] Update ConversationModel.Update() to trigger loadNextBatch when scrolling near bottom in internal/tui/conversation.go
- [x] T032 [US6] Add loading indicator rendering when LoadingState is Loading in internal/tui/conversation.go
- [x] T033 [US6] Implement threshold check (>50) to enable/disable lazy loading in internal/tui/conversation.go

**Checkpoint**: User Story 6 complete - Large conversation lists load progressively

---

## Phase 9: User Story 7 - Lazy Loading for Log View (Priority: P3)

**Goal**: Log views with >100 messages load progressively

**Independent Test**: Open conversation with 500+ messages, verify initial load <1s, scrolling loads more content

### Implementation for User Story 7

- [x] T034 [US7] Add lazyLoadState and loadedCount fields to ViewerModel in internal/tui/viewer.go
- [x] T035 [US7] Create loadMoreMessages() command that loads next batch of entries in internal/tui/viewer.go
- [x] T036 [US7] Update ViewerModel.Update() to trigger loadMoreMessages when scrolling near bottom in internal/tui/viewer.go
- [x] T037 [US7] Add loading indicator rendering when LoadingState is Loading in internal/tui/viewer.go
- [x] T038 [US7] Implement threshold check (>100) to enable/disable lazy loading in internal/tui/viewer.go
- [x] T039 [US7] Implement caching for previously loaded content to support scroll-up in internal/tui/viewer.go

**Checkpoint**: User Story 7 complete - Large log files load progressively

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup and validation

- [x] T040 Remove emoji icons from internal/tui/styles.go (UserIcon, AssistantIcon, etc.) if still present
- [x] T041 Verify 80-column terminal compatibility for all new UI elements
- [x] T042 Run quickstart.md validation commands to verify all features work
- [x] T043 Update README.md with new --version flag documentation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 1 (version package)
- **User Stories 2-7 (Phases 4-9)**: Depend on Phase 2 (foundational types)
- **Polish (Phase 10)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (Version)**: Independent - only needs version package from Setup
- **US2 (Project Decoration)**: Needs foundational styles
- **US3 (Conversation Decoration)**: Needs foundational styles
- **US4 (Token Usage)**: Needs TokenUsage type and parser updates from Foundational
- **US5 (Metadata)**: Needs Conversation metadata fields from Foundational
- **US6 (Lazy Loading Conv)**: Needs LoadingState from Foundational
- **US7 (Lazy Loading Log)**: Needs LoadingState from Foundational

### Within Each User Story

- Types/models before parser changes
- Parser changes before TUI changes
- Core implementation before edge case handling

### Parallel Opportunities

**Phase 1 (Setup)**: T002, T003, T004 can run in parallel
**Phase 2 (Foundational)**: Must be sequential (shared files)
**Phase 4 (US2)**: T013, T014 can run in parallel
**Cross-Story**: US2 and US3 can run in parallel (different files)

---

## Parallel Example: Setup Phase

```bash
# Launch all Setup tasks in parallel:
Task: "Create internal/version/version.go"
Task: "Add TokenUsage struct to internal/types/entry.go"
Task: "Add RawTokenUsage struct to internal/types/entry.go"
Task: "Add formatWithCommas() helper to internal/tui/utils.go"
```

## Parallel Example: User Story 2

```bash
# Launch style tasks in parallel:
Task: "Add ListBorder style to internal/tui/styles.go"
Task: "Add separator style to internal/tui/styles.go"

# Then sequential TUI updates (same file):
Task: "Update ProjectModel.View() header"
Task: "Update ProjectItemDelegate.Render() separators"
Task: "Enhance selected item styling"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 3: User Story 1 (T010-T012)
3. **STOP and VALIDATE**: `cclv --version` works
4. Deploy if ready - version command is useful standalone

### Incremental Delivery

1. MVP: US1 (Version) - Quick win, immediate value
2. P2 Stories: US2 + US3 + US4 (Decoration + Tokens) - Visual improvements
3. P3 Stories: US5 + US6 + US7 (Metadata + Lazy Loading) - Power user features
4. Polish: Final cleanup

### Recommended Order

1. T001-T004 (Setup)
2. T005-T009 (Foundational)
3. T010-T012 (US1 - Version) **MVP CHECKPOINT**
4. T013-T017 (US2 - Project Decoration)
5. T018-T020 (US3 - Conversation Decoration)
6. T021-T024 (US4 - Token Usage)
7. T025-T028 (US5 - Metadata)
8. T029-T033 (US6 - Lazy Load Conversations)
9. T034-T039 (US7 - Lazy Load Logs)
10. T040-T043 (Polish)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- No emoji in UI decorations (per spec FR-017)
