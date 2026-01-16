# Story 1.10: Project Conversation Count

Status: done

## Story

As a **developer browsing projects**,
I want **to see how many conversations each project has**,
So that **I can quickly identify active projects**.

## Acceptance Criteria

### AC 1.10.1: Count displayed in project list
- **Given** the project list view
- **When** it renders
- **Then** each project shows "X conversations" in the description
- **And** format matches existing conversation list metadata style

### AC 1.10.2: Count accuracy
- **Given** a project with multiple conversation files
- **When** the count is displayed
- **Then** it accurately reflects the number of .jsonl files in conversations/
- **And** updates when conversations are added/removed (on refresh)

### AC 1.10.3: Lazy loading compatible
- **Given** lazy loading is enabled
- **When** browsing projects
- **Then** conversation counts load with project metadata
- **And** no performance degradation on large project lists

## Tasks / Subtasks

- [x] Task 1: Add ConversationCount field to Project struct (AC: 1.10.1, 1.10.2, 1.10.3)
  - [x] 1.1: Add `ConversationCount int` field to `types.Project` struct in `internal/types/project.go` (after DirPath field, line 13-14)
  - [x] 1.2: Add comment documenting the field purpose: `// ConversationCount is the number of .jsonl files in the project`

- [x] Task 2: Count conversations during project scan (AC: 1.10.2, 1.10.3)
  - [x] 2.1: In `scanner.ScanProjects()`, after creating Project struct (lines 60-64), call `countConversations(project.DirPath)` and assign to `project.ConversationCount`
  - [x] 2.2: Add unexported helper `countConversations(projectPath string) int` after `assignDisplayNames` function (after line 229)
  - [x] 2.3: Helper uses `os.ReadDir` and counts only files ending with `.jsonl` (no subdirectory traversal)
  - [x] 2.4: Return 0 on any error (graceful degradation per project-context.md error handling rules)

- [x] Task 3: Display count in project list item (AC: 1.10.1)
  - [x] 3.1: Update `ProjectItem.Render()` in `internal/tui/project.go` (lines 21-49)
  - [x] 3.2: Build count string with singular/plural handling: `"1 conversation"` vs `"X conversations"`
  - [x] 3.3: Format description as `"{count} • {path}"` with path truncated to fit remaining width
  - [x] 3.4: Calculate path width = `availWidth - lipgloss.Width(countStr) - 3` (3 chars for " • " separator)
  - [x] 3.5: Use existing `Styles.ListItem.DescNormal/DescSelected` for consistent styling

- [x] Task 4: Add tests and validation (AC: all)
  - [x] 4.1: Add `TestCountConversations` in `internal/scanner/projects_test.go` using table-driven tests
  - [x] 4.2: Test case: empty directory returns 0
  - [x] 4.3: Test case: directory with only .jsonl files returns correct count
  - [x] 4.4: Test case: mixed file types (.jsonl, .txt, .md) counts only .jsonl
  - [x] 4.5: Test case: non-existent directory returns 0 (not error)
  - [x] 4.6: Test case: directory with subdirectories containing .jsonl (should NOT count nested files)
  - [x] 4.7: Run `make test` - all tests pass with >90% coverage
  - [x] 4.8: Run `make lint` - no lint errors
  - [x] 4.9: Run `make build` - build succeeds

## Dev Notes

### Implementation Approach

This is a simple data-enrichment story. The conversation count should be calculated during the initial project scan in `scanner.ScanProjects()` since we're already reading directory entries. This avoids additional I/O later.

### Key Files to Modify

| File | Purpose | Lines of Interest |
|------|---------|-------------------|
| `internal/types/project.go` | Add ConversationCount field | Line 5-14 (Project struct) |
| `internal/scanner/projects.go` | Count conversations during scan | Lines 49-66 (ScanProjects), add helper after line 229 |
| `internal/tui/project.go` | Display count in Render() | Lines 21-49 (Render method) |
| `internal/scanner/projects_test.go` | Add tests | New test function |

### Counting Logic

```go
// Add to internal/scanner/projects.go after assignDisplayNames (line 229)

// countConversations counts .jsonl files in a project directory.
// Returns 0 on any error (graceful degradation).
// Does NOT traverse subdirectories - only counts files at the top level.
func countConversations(projectPath string) int {
    entries, err := os.ReadDir(projectPath)
    if err != nil {
        return 0 // Graceful degradation per project-context.md
    }

    count := 0
    for _, entry := range entries {
        if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
            count++
        }
    }
    return count
}
```

### ScanProjects Integration

```go
// In ScanProjects() after line 64 (project struct creation):
project := types.Project{
    EncodedName: name,
    DecodedPath: DecodeProjectPath(name),
    DirPath:     filepath.Join(projectsPath, name),
    ConversationCount: countConversations(filepath.Join(projectsPath, name)),
}
```

### Display Format

```go
// In ProjectItem.Render(), after calculating availWidth (around line 37-38):

// Build count string with singular/plural handling
countStr := fmt.Sprintf("%d conversations", i.project.ConversationCount)
if i.project.ConversationCount == 1 {
    countStr = "1 conversation"
}

// Path truncation needs to account for count prefix
// Available width calculation:
// - availWidth already accounts for gutter
// - Remove " • " separator (3 chars)
// - Remove count string rendered width
countWidth := lipgloss.Width(countStr)
pathWidth := availWidth - countWidth - 3 // 3 = len(" • ")
if pathWidth < 10 {
    pathWidth = 10 // Minimum path width
}
path := TruncateFromLeftToWidth(i.project.DecodedPath, pathWidth)
descContent := fmt.Sprintf("%s • %s", countStr, path)

// Pad and style as before
paddedDesc := PadToWidth(descContent, availWidth)
desc := descStyle.Render(paddedDesc)
```

### Performance Consideration

The `countConversations` helper performs a single `os.ReadDir` per project. Since `ScanProjects` already iterates through projects, this adds one directory read per project. For typical usage (< 100 projects), this is negligible. If performance becomes an issue with many projects, consider:
1. Lazy loading counts on-demand (not implemented in this story)
2. Caching counts (overkill for current use case)

### Project Structure Alignment

- Adding field to `types.Project` follows existing pattern (4 existing fields, this becomes 5th)
- Helper function in scanner follows existing pattern (`pathExists`, `findValidPath`)
- Render modification follows existing description rendering pattern

### Testing Pattern

Follow table-driven tests per project conventions:

```go
func TestCountConversations(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(dir string) // Setup function for more complex cases
        files    []string         // Simple file names to create (for basic cases)
        want     int
    }{
        {
            name:  "empty directory",
            files: nil,
            want:  0,
        },
        {
            name:  "only jsonl files",
            files: []string{"a.jsonl", "b.jsonl"},
            want:  2,
        },
        {
            name:  "mixed file types",
            files: []string{"a.jsonl", "b.txt", "c.jsonl", "d.md"},
            want:  2,
        },
        {
            name:  "single jsonl file",
            files: []string{"only.jsonl"},
            want:  1, // Tests singular path
        },
        {
            name: "nested jsonl not counted",
            setup: func(dir string) {
                // Create a subdirectory with .jsonl files
                subdir := filepath.Join(dir, "subdir")
                os.Mkdir(subdir, 0755)
                os.WriteFile(filepath.Join(dir, "top.jsonl"), []byte{}, 0644)
                os.WriteFile(filepath.Join(subdir, "nested.jsonl"), []byte{}, 0644)
            },
            want: 1, // Only counts top-level
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dir := t.TempDir()
            if tt.setup != nil {
                tt.setup(dir)
            } else {
                for _, f := range tt.files {
                    os.WriteFile(filepath.Join(dir, f), []byte{}, 0644)
                }
            }
            got := countConversations(dir)
            if got != tt.want {
                t.Errorf("countConversations() = %d, want %d", got, tt.want)
            }
        })
    }
}

func TestCountConversationsNonExistent(t *testing.T) {
    // Non-existent directory should return 0, not error
    got := countConversations("/nonexistent/path/that/does/not/exist")
    if got != 0 {
        t.Errorf("countConversations(nonexistent) = %d, want 0", got)
    }
}
```

### References

- [Source: epics.md lines 512-549] - Story requirements
- [Source: project-context.md] - NO EMOJI, USE MAKEFILE, test coverage rules
- [Source: internal/types/project.go] - Project struct definition
- [Source: internal/scanner/projects.go lines 26-77] - ScanProjects implementation
- [Source: internal/tui/project.go lines 21-49] - ProjectItem.Render implementation
- [Source: docs/architecture.md lines 92-101] - Scanner package responsibilities

### Previous Story Intelligence

From Story 1.9 (pipeline visibility flags):
- Simple data additions (RenderOptions struct) follow clean patterns
- Table-driven tests are the standard
- Minor signature changes propagate cleanly through callers
- formatToolSummary pattern shows how to handle display formatting

From Epic 1.5 and 1.6 patterns:
- Lazy loading threshold is 50 conversations - conversation count is computed eagerly during scan, not lazy-loaded
- ListItem.Render() pattern is well-established with gutter, title, description

### Git Commit Pattern

```
feat: add conversation count to project list

- Add ConversationCount field to types.Project
- Count .jsonl files during project scan
- Display "X conversations" in project list description

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Risk Assessment

**Risk: LOW**

- Adding a field to struct is trivial
- Counting files in directory is well-tested pattern in scanner
- Display formatting follows existing patterns in project.go
- No async complexity or state management
- Performance impact is minimal (one ReadDir per project)
- No breaking changes to existing APIs

### Edge Cases

1. **Project with 0 conversations**: Display "0 conversations" (not hidden)
2. **Project directory doesn't exist**: countConversations returns 0 (no error)
3. **Permission denied on directory**: countConversations returns 0 (graceful degradation)
4. **Very long count string**: Not an issue - max realistic is ~1000 conversations (13 chars)
5. **Singular/plural**: "1 conversation" vs "X conversations"
6. **Subdirectories with .jsonl files**: NOT counted - only top-level .jsonl files count
7. **Filtering active**: Counts remain accurate since they're stored on Project struct
8. **Very narrow terminal**: pathWidth has minimum of 10 chars to prevent layout issues

### Files to Modify Checklist

- [x] `internal/types/project.go` - Add ConversationCount field
- [x] `internal/scanner/projects.go` - Add countConversations helper, call in ScanProjects
- [x] `internal/tui/project.go` - Update Render() to display count
- [x] `internal/scanner/projects_test.go` - Add TestCountConversations

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

- Added `ConversationCount int` field to `types.Project` struct
- Added `countConversations()` helper in scanner package using `os.ReadDir`
- Updated `ProjectItem.Render()` with "X conversations" display format
- Created `internal/scanner/projects_test.go` with 6 test cases
- All tests pass, lint clean, build succeeds

### File List

- `internal/types/project.go` - Added ConversationCount field
- `internal/scanner/projects.go` - Added countConversations helper, integrated into ScanProjects
- `internal/tui/project.go` - Updated Render() to display count
- `internal/scanner/projects_test.go` - New test file with TestCountConversations

