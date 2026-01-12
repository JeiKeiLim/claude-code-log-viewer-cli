# Tasks: Claude Code Log Viewer CLI (cclv)

**Input**: Design documents from `/specs/001-claude-log-viewer/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Tests are NOT explicitly requested in the feature specification. Test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: `cmd/cclv/`, `internal/` at repository root
- Paths follow plan.md structure

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Initialize Go module with `go mod init github.com/JeiKeiLim/claude-code-log-viewer-cli`
- [x] T002 Create project directory structure: `cmd/cclv/`, `internal/tui/`, `internal/parser/`, `internal/types/`, `internal/scanner/`
- [x] T003 [P] Add Bubbletea dependency: `go get github.com/charmbracelet/bubbletea`
- [x] T004 [P] Add Lipgloss dependency: `go get github.com/charmbracelet/lipgloss`
- [x] T005 [P] Add Bubbles dependency: `go get github.com/charmbracelet/bubbles`
- [x] T006 [P] Add Glamour dependency: `go get github.com/charmbracelet/glamour`
- [x] T007 [P] Add golang.org/x/term dependency: `go get golang.org/x/term`
- [x] T008 Create .gitignore for Go project (binaries, vendor, .DS_Store)

**Checkpoint**: Go module initialized with all dependencies

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T009 [P] Define EntryType and ContentType enums in internal/types/entry.go
- [x] T010 [P] Define LogEntry struct with JSON tags in internal/types/entry.go
- [x] T011 [P] Define MessageContent struct (text/thinking/tool_use) in internal/types/entry.go
- [x] T012 [P] Define RawLogEntry, RawUserMessage, RawAssistantMessage structs for JSON parsing in internal/types/entry.go
- [x] T013 [P] Define Project struct in internal/types/project.go
- [x] T014 [P] Define Conversation struct in internal/types/conversation.go
- [x] T015 Define Lipgloss styles for user messages, assistant messages, thinking blocks, tool blocks in internal/tui/styles.go
- [x] T016 Create main.go skeleton with TTY detection using term.IsTerminal in cmd/cclv/main.go

**Checkpoint**: Foundation ready - all types defined, styles ready, entry point with mode detection

---

## Phase 3: User Story 1 - Pipeline Log Viewing (Priority: P1) 🎯 MVP

**Goal**: View Claude logs via pipe (`cat file.jsonl | cclv`) or file argument (`cclv file.jsonl`)

**Independent Test**: Run `cat ~/.claude/projects/.../conversation.jsonl | cclv` and verify formatted output with scrolling

### Implementation for User Story 1

- [x] T017 [P] [US1] Implement JSONL line-by-line parser with bufio.Scanner in internal/parser/jsonl.go
- [x] T018 [P] [US1] Implement LogEntry parsing from JSON with user/assistant message handling in internal/parser/entry.go
- [x] T019 [US1] Create viewerModel Bubbletea model with viewport in internal/tui/viewer.go
- [x] T020 [US1] Implement renderEntry function to format LogEntry as styled string in internal/tui/viewer.go
- [x] T021 [US1] Implement renderUserMessage with user icon and timestamp in internal/tui/viewer.go
- [x] T022 [US1] Implement renderAssistantMessage with text content rendering in internal/tui/viewer.go
- [x] T023 [US1] Implement renderThinkingBlock with collapsed indicator `[thinking - press 't']` in internal/tui/viewer.go
- [x] T024 [US1] Implement renderToolUseBlock with tool name and collapsed inputs indicator in internal/tui/viewer.go
- [x] T025 [US1] Add j/k and arrow key scrolling to viewerModel.Update in internal/tui/viewer.go
- [x] T026 [US1] Add q key to quit in viewerModel.Update in internal/tui/viewer.go
- [x] T027 [US1] Implement pipeline mode entry: read stdin, parse entries, launch viewer in cmd/cclv/main.go
- [x] T028 [US1] Implement file argument mode: read file path from os.Args, open file, parse, launch viewer in cmd/cclv/main.go
- [x] T029 [US1] Handle terminal resize in viewerModel.Update with tea.WindowSizeMsg in internal/tui/viewer.go
- [x] T030 [US1] Add malformed line skipping with parse error counter in internal/parser/jsonl.go
- [x] T031 [US1] Display parse error warning in viewer footer (e.g., "3 lines skipped") in internal/tui/viewer.go

**Checkpoint**: Pipeline viewing works - `cat file.jsonl | cclv` displays formatted, scrollable logs

---

## Phase 4: User Story 2 - Interactive Project Browser (Priority: P2)

**Goal**: Launch `cclv` with no args to see project list, navigate with vim keys

**Independent Test**: Run `cclv` and verify project list displays, j/k navigation works, Enter selects project

### Implementation for User Story 2

- [x] T032 [P] [US2] Implement project directory scanner for ~/.claude/projects/ in internal/scanner/projects.go
- [x] T033 [P] [US2] Implement DecodeProjectPath function to convert encoded names to paths in internal/scanner/projects.go
- [x] T034 [US2] Implement DisplayName disambiguation for colliding project names in internal/scanner/projects.go
- [x] T035 [US2] Create projectModel Bubbletea model with list.Model from bubbles in internal/tui/project.go
- [x] T036 [US2] Implement project list item rendering with DisplayName in internal/tui/project.go
- [x] T037 [US2] Add j/k/arrow navigation to projectModel.Update in internal/tui/project.go
- [x] T038 [US2] Add Enter/l key to select project (return selected Project) in internal/tui/project.go
- [x] T039 [US2] Add / key to open filter input using textinput from bubbles in internal/tui/project.go
- [x] T040 [US2] Implement filter matching on DisplayName in internal/tui/project.go
- [x] T041 [US2] Handle missing ~/.claude/projects/ directory with helpful message in internal/tui/project.go
- [x] T042 [US2] Create appModel root Bubbletea model with viewState enum in internal/tui/app.go
- [x] T043 [US2] Implement view routing in appModel.Update based on viewState in internal/tui/app.go
- [x] T044 [US2] Wire interactive mode entry: scan projects, launch appModel with projectModel in cmd/cclv/main.go

**Checkpoint**: Interactive project browser works - launch `cclv`, see projects, navigate with j/k

---

## Phase 5: User Story 3 - Conversation List Navigation (Priority: P3)

**Goal**: After selecting project, see conversation list sorted by date, select to view

**Independent Test**: Select project, verify conversation list shows with dates and previews, select opens viewer

### Implementation for User Story 3

- [x] T045 [P] [US3] Implement conversation file scanner for project directory in internal/scanner/projects.go
- [x] T046 [P] [US3] Get file LastModified timestamp from os.Stat in internal/scanner/projects.go
- [x] T047 [US3] Implement FirstUserMessage preview extraction (first 80 chars of first user message) in internal/scanner/projects.go
- [x] T048 [US3] Sort conversations by LastModified descending in internal/scanner/projects.go
- [x] T049 [US3] Create conversationModel Bubbletea model with list.Model in internal/tui/conversation.go
- [x] T050 [US3] Implement conversation list item rendering with date, preview, message count in internal/tui/conversation.go
- [x] T051 [US3] Add j/k/arrow navigation to conversationModel.Update in internal/tui/conversation.go
- [x] T052 [US3] Add Enter/l key to select conversation in internal/tui/conversation.go
- [x] T053 [US3] Add Escape/h key to return to project browser in internal/tui/conversation.go
- [x] T054 [US3] Handle empty project (no conversations) with message in internal/tui/conversation.go
- [x] T055 [US3] Wire conversation list into appModel view routing in internal/tui/app.go
- [x] T056 [US3] Wire selected conversation to viewer in appModel in internal/tui/app.go
- [x] T057 [US3] Add Escape/h in viewer to return to conversation list in internal/tui/viewer.go

**Checkpoint**: Full navigation flow works - projects → conversations → viewer → back

---

## Phase 6: User Story 4 - Advanced Log Viewer Features (Priority: P4)

**Goal**: Search within logs, gg/G jump, toggle thinking/tool visibility

**Independent Test**: Open viewer, verify / opens search, gg/G jumps, t/i toggles work

### Implementation for User Story 4

- [x] T058 [US4] Create searchModel overlay component with textinput in internal/tui/search.go
- [x] T059 [US4] Implement search input handling (Enter to search, Escape to cancel) in internal/tui/search.go
- [x] T060 [US4] Add / key to open search overlay in viewerModel.Update in internal/tui/viewer.go
- [x] T061 [US4] Implement search matching across entry text content in internal/tui/viewer.go
- [x] T062 [US4] Implement search result highlighting in rendered content in internal/tui/viewer.go
- [x] T063 [US4] Add n key to jump to next match in internal/tui/viewer.go
- [x] T064 [US4] Add N key to jump to previous match in internal/tui/viewer.go
- [x] T065 [US4] Display "no results" feedback when search finds nothing in internal/tui/viewer.go
- [x] T066 [US4] Implement gg key sequence detection with timeout in viewerModel.Update in internal/tui/viewer.go
- [x] T067 [US4] Implement gg action: viewport.GotoTop() in internal/tui/viewer.go
- [x] T068 [US4] Implement G key: viewport.GotoBottom() in internal/tui/viewer.go
- [x] T069 [US4] Add showThinking bool toggle state to viewerModel in internal/tui/viewer.go
- [x] T070 [US4] Implement t key to toggle showThinking and re-render in internal/tui/viewer.go
- [x] T071 [US4] Add showToolInputs bool toggle state to viewerModel in internal/tui/viewer.go
- [x] T072 [US4] Implement i key to toggle showToolInputs and re-render in internal/tui/viewer.go
- [x] T073 [US4] Implement expanded thinking block rendering in internal/tui/viewer.go
- [x] T074 [US4] Implement expanded tool input rendering with 200 char truncation in internal/tui/viewer.go

**Checkpoint**: All advanced features work - search, jump, toggle

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T075 Add Page Up/Down and Home/End key support to viewerModel in internal/tui/viewer.go
- [x] T076 [P] Implement timestamp formatting in local timezone in internal/tui/viewer.go
- [x] T077 [P] Add Ctrl+C signal handling for clean exit in cmd/cclv/main.go
- [x] T078 Verify and fix terminal resize handling across all views in internal/tui/app.go
- [ ] T079 [P] Add streaming stdin support for tail -f compatibility in internal/parser/jsonl.go
- [x] T080 Run go vet and fix any issues
- [x] T081 Run gofmt on all source files
- [ ] T082 Validate quickstart.md scenarios work end-to-end
- [x] T083 Build cross-platform binaries for linux/darwin amd64/arm64

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational - MVP, can be released standalone
- **User Story 2 (Phase 4)**: Depends on Foundational - Adds project browser
- **User Story 3 (Phase 5)**: Depends on US2 (needs project browser to navigate to conversations)
- **User Story 4 (Phase 6)**: Depends on US1 (enhances the viewer)
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational)
    ↓
    ├─→ US1 (P1) ←────────────────────┐
    │       ↓                         │
    │       └─→ US4 (P4) ────────────┤
    │                                 │
    └─→ US2 (P2)                      │
            ↓                         │
            └─→ US3 (P3) ────────────┘
                    ↓
            Phase 7 (Polish)
```

### Parallel Opportunities

**Phase 1 (Setup)**:
- T003, T004, T005, T006, T007 can run in parallel (independent go get commands)

**Phase 2 (Foundational)**:
- T009-T014 can run in parallel (independent type definitions)

**Phase 3 (US1)**:
- T017, T018 can run in parallel (parser components)

**Phase 4 (US2)**:
- T032, T033 can run in parallel (scanner components)

**Phase 5 (US3)**:
- T045, T046 can run in parallel (file scanning components)

---

## Parallel Example: User Story 1

```bash
# Launch parser components in parallel:
Task T017: "Implement JSONL line-by-line parser in internal/parser/jsonl.go"
Task T018: "Implement LogEntry parsing in internal/parser/entry.go"

# Then implement viewer components sequentially:
Task T019: "Create viewerModel in internal/tui/viewer.go"
Task T020-T031: Sequential viewer implementation
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: `cat file.jsonl | cclv` works
5. Release v0.1.0 - Pipeline mode only

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add User Story 1 → v0.1.0 (MVP - pipeline viewing)
3. Add User Story 2 → v0.2.0 (project browser)
4. Add User Story 3 → v0.3.0 (conversation navigation)
5. Add User Story 4 → v0.4.0 (search and toggles)
6. Polish → v1.0.0 (stable release)

---

## Task Summary

| Phase | Tasks | Parallel Tasks |
|-------|-------|----------------|
| Phase 1: Setup | 8 | 5 |
| Phase 2: Foundational | 8 | 6 |
| Phase 3: US1 - Pipeline | 15 | 2 |
| Phase 4: US2 - Projects | 13 | 2 |
| Phase 5: US3 - Conversations | 13 | 2 |
| Phase 6: US4 - Advanced | 17 | 0 |
| Phase 7: Polish | 9 | 3 |
| **Total** | **83** | **20** |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- US1 is the MVP - can be released before other stories are complete
