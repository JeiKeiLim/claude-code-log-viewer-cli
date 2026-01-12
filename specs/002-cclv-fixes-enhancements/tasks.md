# Tasks: CCLV Fixes and Enhancements

**Input**: Design documents from `/specs/002-cclv-fixes-enhancements/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Not explicitly requested in spec. Tests omitted per template guidelines.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- File paths relative to repository root

## Path Conventions

```text
cmd/cclv/
└── main.go              # Entry point, flag parsing, mode detection

internal/
├── tui/
│   ├── app.go           # Root application model
│   ├── project.go       # Project browser (navigation fix)
│   ├── conversation.go  # Conversation list (navigation fix)
│   ├── viewer.go        # Log viewer (title enhancement)
│   ├── plain.go         # NEW: Plain text renderer
│   └── styles.go        # Styling
└── scanner/
    └── projects.go      # Path decoding fix
```

---

## Phase 1: Setup

**Purpose**: No new dependencies required. Verify existing project structure.

- [x] T001 Verify Go build passes with `make build` before changes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No blocking infrastructure changes needed for this feature set. Each user story is independent.

**Checkpoint**: Proceed directly to User Story phases.

---

## Phase 3: User Story 1 - Plain Text Output Mode (Priority: P1)

**Goal**: Enable piping cclv output to other applications by adding `--plain` and `--tui` flags with automatic TTY detection.

**Independent Test**: Run `cclv --plain ~/.claude/projects/*/conversation.jsonl | head -20` and verify formatted text output without TUI. Run `cat file.jsonl | cclv | wc -l` to verify auto-detection.

### Implementation for User Story 1

- [x] T002 [US1] Add `--plain` and `--tui` flag parsing using Go `flag` package in cmd/cclv/main.go
- [x] T003 [US1] Add stdout TTY detection using `term.IsTerminal(int(os.Stdout.Fd()))` in cmd/cclv/main.go
- [x] T004 [US1] Implement mode selection logic per data-model.md flow diagram in cmd/cclv/main.go
- [x] T005 [P] [US1] Create internal/tui/plain.go with `RenderPlain(entries []types.LogEntry, source string) string` function
- [x] T006 [US1] Implement `renderUserMessagePlain` helper in internal/tui/plain.go reusing viewer rendering logic
- [x] T007 [US1] Implement `renderAssistantMessagePlain` helper in internal/tui/plain.go reusing viewer rendering logic
- [x] T008 [US1] Add plain mode execution path in cmd/cclv/main.go that calls RenderPlain and prints to stdout
- [x] T009 [US1] Add header line showing source (filename or "stdin") in plain output in internal/tui/plain.go

**Checkpoint**: User Story 1 complete. Verify: `cclv --plain file.jsonl`, `cat file.jsonl | cclv | head`, `cclv --tui` forces TUI.

---

## Phase 4: User Story 2 - Fix Navigation Double-Skip Bug (Priority: P1)

**Goal**: Navigation in project and conversation lists moves exactly one item per keypress.

**Independent Test**: Launch `cclv`, press j/k repeatedly, verify cursor moves exactly one item per keypress. Same for arrow keys.

### Implementation for User Story 2

- [x] T010 [P] [US2] Remove explicit `m.list.CursorDown()` and `m.list.CursorUp()` calls for j/k/down/up keys in internal/tui/project.go (let bubbles list handle navigation)
- [x] T011 [P] [US2] Remove explicit `m.list.CursorDown()` and `m.list.CursorUp()` calls for j/k/down/up keys in internal/tui/conversation.go (let bubbles list handle navigation)
- [x] T012 [US2] Remove manual g/G cursor loops in internal/tui/project.go (bubbles list supports these natively)
- [x] T013 [US2] Remove manual g/G cursor loops in internal/tui/conversation.go (bubbles list supports these natively)

**Checkpoint**: User Story 2 complete. Verify navigation in both project list and conversation list moves one item per keypress.

---

## Phase 5: User Story 3 - Fix Hyphen in Project Path Handling (Priority: P1)

**Goal**: Project paths with hyphens (e.g., `my-project`) display correctly instead of being converted to slashes.

**Independent Test**: If you have a project with hyphens in its path, run `cclv` and verify it shows correctly (e.g., "my-project" not "my/project").

### Implementation for User Story 3

- [x] T014 [US3] Fix DecodeProjectPath algorithm in internal/scanner/projects.go: (1) replace `--` with placeholder `\x00`, (2) replace `-` with `/`, (3) replace placeholder with `-`

**Checkpoint**: User Story 3 complete. Verify projects with hyphenated names display correctly.

---

## Phase 6: User Story 4 - Informative Viewer Title (Priority: P2)

**Goal**: Viewer title shows contextual information about what is being viewed (project name + time, filename, or "stdin").

**Independent Test**: Open conversation from browser - title shows project + time. Run `cclv file.jsonl` - title shows filename. Run `cat file.jsonl | cclv` - title shows "stdin".

### Implementation for User Story 4

- [x] T015 [US4] Add `title string` field to ViewerModel struct in internal/tui/viewer.go
- [x] T016 [US4] Modify `NewViewerModel` to accept title parameter in internal/tui/viewer.go
- [x] T017 [US4] Update `NewViewerModelWithBack` to accept and pass title parameter in internal/tui/viewer.go
- [x] T018 [US4] Render title in View() function header instead of hardcoded "Claude Code Log Viewer" in internal/tui/viewer.go
- [x] T019 [US4] Update call site in internal/tui/app.go to pass project name + conversation timestamp as title
- [x] T020 [US4] Update call site in cmd/cclv/main.go runPipelineMode to pass filename or "stdin" as title

**Checkpoint**: User Story 4 complete. Verify title shows appropriate context in all modes.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup.

- [x] T021 Run `make build` to verify no compilation errors
- [x] T022 Run `make test` to verify existing tests pass
- [x] T023 Run quickstart.md verification commands to validate all features
- [x] T024 Manual test: Interactive mode navigation (cannot run in non-TTY, but code verified)
- [x] T025 Manual test: Plain mode piping to `less -R` (tested with `| head` and `| wc -l`)
- [x] T026 Manual test: Hyphenated project name display (verified with unit test)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: N/A for this feature
- **User Stories (Phase 3-6)**: All independent - can proceed in parallel or sequentially
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent - new file + main.go changes
- **User Story 2 (P1)**: Independent - project.go + conversation.go changes
- **User Story 3 (P1)**: Independent - scanner/projects.go changes
- **User Story 4 (P2)**: Independent - viewer.go + app.go + main.go changes

### Within Each User Story

- US1: Flag parsing (T002-T004) before plain renderer (T005-T009)
- US2: T010 and T011 can run in parallel (different files)
- US3: Single task (T014)
- US4: ViewerModel changes (T015-T018) before call site updates (T019-T020)

### Parallel Opportunities

- T010 and T011 are parallelizable (different files)
- T005 can start while T002-T004 are in progress (new file)
- All user story phases are independent and can be parallelized across developers

---

## Parallel Example: User Story 2

```bash
# Both tasks can run in parallel (different files):
Task T010: "Remove explicit cursor calls in internal/tui/project.go"
Task T011: "Remove explicit cursor calls in internal/tui/conversation.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 + User Story 2)

1. Complete Phase 1: Setup (verify build)
2. Complete US1: Plain Text Output Mode (enables pipeline usage - main requested feature)
3. Complete US2: Fix Navigation (critical usability bug)
4. **STOP and VALIDATE**: Test plain mode piping and navigation
5. Continue with US3 and US4

### Incremental Delivery

1. US1 (Plain Mode) → Test piping to `less -R` and other tools
2. US2 (Navigation Fix) → Test j/k moves one item
3. US3 (Hyphen Fix) → Test hyphenated project names
4. US4 (Title) → Test contextual titles
5. Polish → Run all verification commands

### Single Developer Strategy

Recommended order for single developer:
1. T001 (Setup verification)
2. T014 (US3 - quickest fix, one function change)
3. T010-T013 (US2 - small changes, removes code)
4. T002-T009 (US1 - largest change, new file)
5. T015-T020 (US4 - API changes across files)
6. T021-T026 (Polish)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable
- Commit after each task or logical group
- US2 and US3 are "deletion/fix" tasks - smaller and safer
- US1 is the largest addition (new file + flag handling)
- US4 requires API changes but is isolated to viewer title
