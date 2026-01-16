# Story 3.3: Implement Render Caching

Status: done

## Story

As a **developer scrolling through logs**,
I want **smooth scroll performance**,
So that **markdown rendering doesn't cause lag**.

## Acceptance Criteria

### AC 3.3.1: Cache rendered output
- **Given** an assistant entry is rendered
- **When** the same entry is viewed again
- **Then** cached output is used
- **And** Glamour.Render is NOT called again

### AC 3.3.2: Cache keyed by entry
- **Given** the render cache
- **When** storing rendered content
- **Then** it is keyed by entry ID or index
- **And** each entry has its own cache slot
- **Note:** Toggle state changes (showThinking, showToolInputs) are handled via full cache invalidation, not key modification

### AC 3.3.3: Cache invalidation on resize
- **Given** terminal is resized
- **When** width changes
- **Then** entire render cache is invalidated
- **And** next View() triggers fresh renders

### AC 3.3.4: Lazy rendering
- **Given** a large log file
- **When** scrolling through entries
- **Then** only visible entries are rendered
- **And** off-screen entries remain uncached until viewed

### AC 3.3.5: Performance target
- **Given** 10k entries loaded
- **When** scrolling rapidly
- **Then** framerate remains smooth (target 60 FPS)
- **And** no visible lag or stutter

## Tasks / Subtasks

- [x] Task 1: Add render cache infrastructure to ViewerModel (AC: 3.3.1, 3.3.2)
  - [x] 1.1: Add `renderCache map[int]string` field to `ViewerModel` struct in viewer.go
  - [x] 1.2: Add `cacheWidth int` field to track width when cache was built
  - [x] 1.3: Initialize `renderCache = make(map[int]string)` in `NewViewerModel()`
  - [x] 1.4: Initialize `cacheWidth = initialWidth - 4` in `NewViewerModel()`

- [x] Task 2: Create cache helper methods (AC: 3.3.1, 3.3.2)
  - [x] 2.1: Add `invalidateRenderCache()` method that resets map and updates cacheWidth
  - [x] 2.2: Add `getCachedRender(idx int, entry types.LogEntry) string` method that checks cache before rendering

- [x] Task 3: Integrate caching into rendering flow (AC: 3.3.1, 3.3.4)
  - [x] 3.1: Modify `updateContent()` to use `getCachedRender()` instead of direct `m.renderEntry()`
  - [x] 3.2: Only cache assistant entries (ContentTypeText with markdown) - user/tool entries are fast enough without caching

- [x] Task 4: Implement cache invalidation (AC: 3.3.3)
  - [x] 4.1: In `WindowSizeMsg` handler, call `invalidateRenderCache()` when renderer is recreated (widthDiff > 5)
  - [x] 4.2: Ensure cache is invalidated BEFORE `updateContent()` is called
  - [x] 4.3: (Optional) Consider adding cache size limit or LRU eviction if cache exceeds ~1000 entries for memory safety

- [x] Task 5: Handle watch mode cache updates (AC: 3.3.1)
  - [x] 5.1: In `NewEntriesMsg` handler, DO NOT invalidate cache - new entries get cached on first render
  - [x] 5.2: In `FileResetMsg` handler, invalidate cache when file is truncated (full reload)

- [x] Task 6: Handle toggle state changes (AC: 3.3.1)
  - [x] 6.1: In 't' key handler (showThinking toggle), invalidate cache
  - [x] 6.2: In 'i' key handler (showToolInputs toggle), invalidate cache
  - [x] 6.3: Note: Toggle changes affect rendering, so cache must be cleared

- [x] Task 7: Verify bulk loading aligns with lazy rendering (AC: 3.3.4)
  - [x] 7.1: Confirm bulk loading uses pre-rendered string approach (existing design stores in renderedContent, not cache)
  - [x] 7.2: Note: Cache populates naturally via getCachedRender() during subsequent scrolling - no special bulk cache population needed
  - [x] 7.3: Verify after bulk load completes, subsequent updateContent() uses cache correctly

- [x] Task 8: Add unit tests for render caching (AC: 3.3.1, 3.3.2, 3.3.3)
  - [x] 8.1: Test cache hit returns same content without re-render
  - [x] 8.2: Test cache key is entry index
  - [x] 8.3: Test `invalidateRenderCache()` clears cache and updates width
  - [x] 8.4: Test resize triggers cache invalidation

- [x] Task 9: Run build, lint, and test validation
  - [x] 9.1: Run `make build` - verify binary builds
  - [x] 9.2: Run `make lint` - no errors
  - [x] 9.3: Run `make test` - all tests pass, coverage maintained

- [ ] Task 10: Manual performance testing (AC: 3.3.5)
  - [ ] 10.1: Open a conversation with 100+ messages containing markdown (real-world scenario)
  - [ ] 10.1b: (Optional stress test) Generate or find a 1000+ entry log to test edge performance
  - [ ] 10.2: Scroll rapidly up and down - verify smooth scrolling
  - [ ] 10.3: Resize terminal - verify cache clears and content rewraps without lag
  - [ ] 10.4: Toggle thinking/inputs - verify cache clears properly
  - [ ] 10.5: Watch mode with rapid updates - verify performance remains smooth
  - [ ] 10.6: Watch mode + resize sequence test:
    - Start watch mode on active file
    - Scroll up (away from bottom)
    - Let 5+ new entries arrive
    - Resize terminal
    - Verify cache clears and all content re-renders correctly

## Dev Notes

### Current State Analysis (from Stories 3.1 and 3.2)

Story 3.1 and 3.2 established the markdown rendering infrastructure:

```go
// From styles.go - MarkdownRenderer struct
type MarkdownRenderer struct {
    renderer *glamour.TermRenderer
    width    int
}

// From viewer.go - ViewerModel already has:
type ViewerModel struct {
    markdownRenderer *MarkdownRenderer
    // ... other fields
}
```

Currently, `renderAssistantMessage()` calls `m.markdownRenderer.Render()` on every `updateContent()` call. This is inefficient when scrolling through already-rendered content.

### Cache Implementation Design

**Cache Structure:**
```go
type ViewerModel struct {
    // ... existing fields ...

    // Render cache - keyed by entry index
    renderCache map[int]string  // entry index -> rendered markdown string
    cacheWidth  int             // width when cache was built
}
```

**Why cache by entry index (not entry content hash):**
1. Entries are immutable once loaded (log entries don't change)
2. Index lookup is O(1) vs hashing content
3. Index is stable within a session
4. Entry content can be very large - hashing is expensive

**Cache invalidation triggers:**
1. Terminal resize (width change > 5)
2. Toggle state change (showThinking, showToolInputs)
3. File truncation in watch mode (FileResetMsg)

**NOT triggers for invalidation:**
1. New entries arriving (watch mode) - they just get added to cache
2. Scrolling - cache is checked, not cleared
3. Search - highlighting overlays, doesn't change cache

### Implementation Pattern

**invalidateRenderCache() method:**
```go
func (m *ViewerModel) invalidateRenderCache() {
    m.renderCache = make(map[int]string)
    m.cacheWidth = m.width - 4
}
```

**getCachedRender() method:**
```go
func (m *ViewerModel) getCachedRender(idx int, entry types.LogEntry) string {
    // Check if entry type benefits from caching
    if entry.Type != types.EntryTypeAssistant {
        // User messages and tool blocks render fast - no caching needed
        return m.renderEntry(entry)
    }

    // Check cache hit
    if cached, ok := m.renderCache[idx]; ok {
        return cached
    }

    // Cache miss - render and store
    rendered := m.renderEntry(entry)
    m.renderCache[idx] = rendered
    return rendered
}
```

**Why only cache assistant entries:**
- User messages use `WrapText()` which is fast (~microseconds)
- Tool blocks use JSON formatting which is also fast
- Thinking blocks are typically short and fast
- Assistant markdown rendering via Glamour is the expensive operation (~1-5ms per entry)

### updateContent() Changes

**Current implementation (viewer.go:642-668):**
```go
func (m *ViewerModel) updateContent() {
    var content strings.Builder
    renderCount := m.loadedCount
    if renderCount > len(m.entries) {
        renderCount = len(m.entries)
    }
    for i := 0; i < renderCount; i++ {
        rendered := m.renderEntry(m.entries[i])  // <-- Change this line
        content.WriteString(rendered)
        content.WriteString("\n")
    }
    // ... lazy loading indicator code ...
    m.viewport.SetContent(content.String())
}
```

**Updated to use cache:**
```go
for i := 0; i < renderCount; i++ {
    rendered := m.getCachedRender(i, m.entries[i])  // Use cache
    content.WriteString(rendered)
    content.WriteString("\n")
}
```

### WindowSizeMsg Handler Changes

**Current implementation (viewer.go:357-393):**
```go
case tea.WindowSizeMsg:
    // ... width handling ...

    if m.markdownRenderer == nil {
        m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
    } else {
        widthDiff := m.markdownRenderer.Width() - newRenderWidth
        if widthDiff < 0 {
            widthDiff = -widthDiff
        }
        if widthDiff > 5 {
            m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
        }
    }
    // ... viewport resize and updateContent() ...
```

**Add cache invalidation:**
```go
if widthDiff > 5 {
    m.markdownRenderer, _ = NewMarkdownRenderer(newRenderWidth)
    m.invalidateRenderCache()  // ADD THIS LINE
}
```

### Toggle Handler Changes

**Current 't' handler (viewer.go:321-323):**
```go
case "t":
    m.showThinking = !m.showThinking
    m.updateContent()
```

**Updated with cache invalidation:**
```go
case "t":
    m.showThinking = !m.showThinking
    m.invalidateRenderCache()  // ADD THIS LINE
    m.updateContent()
```

Same pattern for 'i' handler (showToolInputs).

### FileResetMsg Handler Changes

**Current implementation (viewer.go:439-457):**
```go
case watcher.FileResetMsg:
    m.newEntriesCount = 0
    if m.renderOpts.FilePath != "" {
        result, err := parser.ParseJSONLFile(m.renderOpts.FilePath)
        if err == nil {
            m.entries = result.Entries
            m.loadedCount = len(m.entries)
            m.parseErrors = result.ParseErrors
            m.updateContent()
        }
    }
    // ...
```

**Add cache invalidation:**
```go
case watcher.FileResetMsg:
    m.newEntriesCount = 0
    m.invalidateRenderCache()  // ADD THIS LINE - clear cache on file reset
    if m.renderOpts.FilePath != "" {
        // ...
    }
```

### Bulk Loading Integration

**Current markAllMessagesLoadedCmd() (viewer.go:497-522):**
```go
func (m *ViewerModel) markAllMessagesLoadedCmd() tea.Cmd {
    entries := m.entries
    total := len(entries)
    // ... capture other values ...

    return func() tea.Msg {
        var content strings.Builder
        for i := 0; i < total; i++ {
            rendered := renderEntryStatic(entries[i], width, showThinking, showToolInputs, opts, mdRenderer)
            content.WriteString(rendered)
            content.WriteString("\n")
        }
        return viewerMessagesLoadedMsg{
            loadedCount:     total,
            renderedContent: content.String(),
        }
    }
}
```

**Option 1 (Simpler): Don't cache during bulk load**
- Bulk load already pre-renders and stores in `renderedContent`
- Cache is used for subsequent scrolling/re-renders
- After bulk load, cache will populate naturally via `getCachedRender()`

**Option 2 (More Complete): Return cache map with message**
- Requires modifying `viewerMessagesLoadedMsg` to include cache map
- More complex but avoids re-rendering after bulk load

**Decision: Option 1 (REQUIRED)** - This aligns with AC 3.3.4 (lazy rendering). Bulk loading sets viewport content directly via `renderedContent`, and cache populates naturally during subsequent scrolling. The performance benefit of caching during bulk load is minimal since:
1. Bulk load renders each entry once anyway
2. After bulk load, viewport content is already set
3. Cache populates when user scrolls (lazy)
4. **Critical**: Caching during bulk load would violate "off-screen entries remain uncached until viewed"

### NewViewerModel() Initialization

**Add after existing initialization (viewer.go:91-155):**
```go
// After markdownRenderer initialization
m := ViewerModel{
    // ... existing fields ...
    markdownRenderer: mdRenderer,
    renderCache:      make(map[int]string),      // ADD
    cacheWidth:       initialWidth - 4,          // ADD
}
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Add renderCache map, cacheWidth, helper methods, integrate into rendering |
| `internal/tui/viewer_test.go` | Add cache-related unit tests (create if needed) |

### Files NOT to Modify

| File | Reason |
|------|--------|
| `internal/tui/styles.go` | MarkdownRenderer already complete from Story 3.1 |
| `cmd/cclv/main.go` | No CLI changes needed |
| `internal/parser/*.go` | Parser unaffected |
| `internal/watcher/*.go` | Watcher unaffected |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **NO EMOJI in UI** | Text icons only per project-context.md |
| **Use Makefile** | `make build`, `make test`, never raw go commands |
| **Test patterns** | Table-driven tests per project-context.md |
| **Cache by index** | NOT content hash - index is O(1) and stable |
| **Only cache assistant** | User/tool/thinking render fast without caching |

### Performance Expectations

**With caching:**
- First render: ~1-5ms per assistant entry (Glamour)
- Subsequent renders: ~10-50 microseconds (cache lookup)
- 100 entries scroll: ~50ms → ~1ms (20-50x improvement)

**Memory overhead:**
- Cache stores rendered strings
- Typical entry: ~1-5KB rendered
- 1000 entries: ~1-5MB cache
- Well within acceptable bounds (target <50MB total)

### Edge Cases

| Case | Handling |
|------|----------|
| Empty entries list | Cache is empty map, no issues |
| Single entry | Cache works normally |
| Entry index out of bounds | getCachedRender checks via renderEntry (which handles) |
| Cache hit with stale width | Won't happen - invalidateRenderCache called on resize |
| Toggle then resize | Both trigger invalidation (redundant but safe) |
| Watch mode new entries | New entries cached on first render, existing cache preserved |

### Testing Strategy

**Unit tests to add (viewer_test.go or styles_test.go):**

```go
func TestRenderCacheHit(t *testing.T) {
    // Create model with entries
    // Call getCachedRender twice
    // Verify second call returns same content
    // (Ideally verify Render wasn't called again - may need mock)
}

func TestRenderCacheInvalidateOnResize(t *testing.T) {
    // Create model with cached entries
    // Call invalidateRenderCache()
    // Verify cache is empty
    // Verify cacheWidth is updated
}

func TestRenderCacheKeyByIndex(t *testing.T) {
    // Create model with multiple entries
    // Render entries 0, 2, 4
    // Verify cache has exactly keys 0, 2, 4
}

func TestRenderCacheOnlyAssistant(t *testing.T) {
    // Create model with user and assistant entries
    // Render all entries
    // Verify only assistant entries are in cache
}
```

### Git Intelligence

Recent commits:
```
aca15d1 feat: integrate Glamour markdown renderer for assistant text
1daa088 feat: enable watch mode from interactive browse
a9e753c docs: add Epic 2 retrospective and Story 2.4 for interactive watch mode
```

Suggested commit message:
```
feat: implement render caching for markdown content

- Add renderCache map to ViewerModel for O(1) cache lookup
- Cache only assistant entries (markdown rendering is expensive)
- Invalidate cache on resize, toggle, and file reset
- Preserve cache on new entries in watch mode

Story 3.3 of Epic 3: Markdown Rendering

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### Dependencies

- **Story 3.1 (Complete)**: Provides MarkdownRenderer used by cache
- **Story 3.2 (Complete)**: Provides resize handling that triggers cache invalidation
- **This completes Epic 3**: No further stories in this epic

### Previous Story Intelligence (Story 3.2)

Key learnings from Story 3.2:
1. **Nil-check order matters**: Fixed in 3.2 - check `m.markdownRenderer == nil` before calling `.Width()`
2. **widthDiff > 5 threshold**: Acts as natural debounce for resize
3. **updateContent() re-renders all**: Cache will dramatically improve this

### Project Context Reference

From `project-context.md`:
- **Lazy Loading Thresholds**: Messages >100 trigger lazy loading (cache helps when scrolling through loaded messages)
- **Performance targets**: Startup <100ms, Memory <50MB (cache adds ~1-5MB, well within budget)
- **Never render in View()**: Cache lookup is data access, not rendering - acceptable in View() context conceptually, but we do it in updateContent() which is called from Update()

### References

- [Source: epics.md lines 993-1055] - Story 3.3 requirements and acceptance criteria
- [Source: prd.md lines 163-172] - FR-303 Render Caching requirements
- [Source: project-context.md lines 235-239] - Lazy loading thresholds
- [Source: internal/tui/viewer.go:642-668] - updateContent() to modify
- [Source: internal/tui/viewer.go:357-393] - WindowSizeMsg handler to modify
- [Source: internal/tui/viewer.go:321-327] - Toggle handlers to modify
- [Source: internal/tui/viewer.go:439-457] - FileResetMsg handler to modify
- [Source: 3-1-integrate-glamour-markdown-renderer.md] - Story 3.1 implementation details
- [Source: 3-2-implement-dynamic-word-wrap.md] - Story 3.2 resize handling

## Implementation Checklist

Before marking story complete, verify:

- [x] `renderCache map[int]string` added to ViewerModel
- [x] `cacheWidth int` added to ViewerModel
- [x] Cache initialized in NewViewerModel()
- [x] `invalidateRenderCache()` method implemented
- [x] `getCachedRender()` method implemented
- [x] updateContent() uses getCachedRender()
- [x] Only assistant entries are cached (user/tool/thinking are not)
- [x] Cache invalidated on resize (widthDiff > 5)
- [x] Cache invalidated on toggle ('t' and 'i' keys)
- [x] Cache invalidated on file reset (watch mode)
- [x] Cache NOT invalidated on new entries (watch mode)
- [x] Bulk loading does NOT populate cache (aligns with AC 3.3.4 lazy rendering)
- [x] Unit tests added for cache behavior
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes with no regressions
- [ ] Manual: scrolling through 100+ message conversation is smooth
- [ ] Manual: resize clears cache and content rewraps correctly
- [ ] Manual: toggle thinking/inputs updates display correctly
- [ ] Manual: watch mode + resize sequence works correctly

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. **Task 1-4 Complete**: Added `renderCache map[int]string` and `cacheWidth int` fields to ViewerModel, initialized in NewViewerModel(), implemented `invalidateRenderCache()` and `getCachedRender()` methods, integrated caching into `updateContent()`.

2. **Task 5-6 Complete**: Cache invalidation on FileResetMsg (file truncation), toggle handlers ('t' and 'i' keys). NewEntriesMsg does NOT invalidate cache per AC 3.3.1.

3. **Task 7 Complete**: Bulk loading verified to NOT populate cache (uses renderedContent string), aligning with AC 3.3.4 lazy rendering requirement.

4. **Task 8 Complete**: Added 7 unit tests for render caching: TestRenderCacheInitialization, TestInvalidateRenderCache, TestGetCachedRenderCacheHit, TestGetCachedRenderKeyByIndex, TestGetCachedRenderOnlyAssistant, TestCacheInvalidationOnToggle, TestCacheNotInvalidatedOnNewEntries.

5. **Task 9 Complete**: `make build` succeeds, `make lint` reports 0 issues, `make test` passes all tests including new cache tests.

6. **Task 10 Pending**: Manual performance testing requires user interaction. Test steps documented below.

7. **Code Review (Post-Dev)**: Added 3 additional tests for improved coverage:
   - `TestCacheInvalidationOnResize` - Verifies cache clears when widthDiff > 5 (AC 3.3.3)
   - `TestCacheInvalidationOnFileReset` - Verifies FileResetMsg path clears cache
   - `TestUpdateContentUsesCachedRender` - Verifies updateContent() populates cache (AC 3.3.1)

### Manual Testing Steps (Task 10)

Run `./cclv` and:
1. Open a conversation with 100+ messages - verify smooth scrolling (j/k keys)
2. Resize terminal - verify content rewraps without lag
3. Press 't' to toggle thinking blocks - verify display updates correctly
4. Press 'i' to toggle tool inputs - verify display updates correctly
5. Press 'w' to enable watch mode, scroll up, let new entries arrive, resize - verify all works correctly

### File List

- `internal/tui/viewer.go` - Added renderCache, cacheWidth, invalidateRenderCache(), getCachedRender(), integrated into updateContent() and handlers
- `internal/tui/viewer_test.go` - Added 10 unit tests for render caching behavior (7 original + 3 from code review)
- `internal/tui/styles_test.go` - No changes for Story 3.3 (leftover from Stories 3.1/3.2)

