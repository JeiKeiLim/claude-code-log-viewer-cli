# Story 4.6: Fix Position Indicator Desync

Status: review

## Story

As a **user scrolling through logs**,
I want **the position indicator to accurately show which entry I'm viewing**,
So that **I can orient myself in long conversations**.

## Bug Description

**Reported by:** Jongkuk Lim (Epic 4 Retrospective, 2026-01-19)

**Symptoms:**
1. Position indicator (Entry X/Y in status bar) doesn't match the visible box number
2. Indicator oscillates erratically while scrolling (e.g., 3800↔4000↔3800)
3. Problem is worse before lazy loading completes
4. After scrolling to bottom and back to top, behavior improves slightly but still inaccurate

**Example:**
- Log has 5080 entries
- User is looking at box 10
- Indicator shows "Entry 1680/5080"
- Scrolling causes jumps between 3800-4000 range

**User Hypothesis:** "Behaviour looks like it's using percentage of scroll I'm looking at"

**Root Cause (Suspected):**
`buildPositionSegment()` likely calculates position as:
```go
currentEntry = int(scrollPercent * float64(totalEntries))
```
This fails because:
1. With lazy loading, only N entries are rendered but totalEntries is the full count
2. Scroll percentage against partial content gives wrong entry number
3. Rounding/viewport height fluctuations cause oscillation

**Correct Approach:**
Calculate position by finding which entry is visible at the current viewport offset using `entryLinePositions` array.

## Acceptance Criteria

### AC 4.6.1: Accurate position in normal mode
- **Given** I am viewing a conversation with entries loaded
- **When** I scroll to view entry N
- **Then** the position indicator shows "Entry N/Total"
- **And** the indicator matches the topmost visible entry

### AC 4.6.2: Accurate position with lazy loading
- **Given** lazy loading is active (only partial entries rendered)
- **When** I scroll through loaded content
- **Then** position indicator accurately reflects visible entry
- **And** indicator doesn't jump erratically

### AC 4.6.3: Accurate position in raw mode
- **Given** I am in raw JSONL mode
- **When** I scroll to view line N
- **Then** the position indicator shows "Line N/Total"
- **And** the indicator matches the topmost visible line

### AC 4.6.4: No oscillation
- **Given** I am scrolling smoothly through content
- **When** the viewport moves
- **Then** position indicator increments/decrements smoothly
- **And** no erratic jumping between distant values

### AC 4.6.5: Position after mode toggle
- **Given** I toggle between normal and raw mode
- **When** viewing content after toggle
- **Then** position indicator is accurate for the current mode

## Tasks / Subtasks

- [x] Task 1: Investigate current implementation
  - [x] 1.1: Read `buildPositionSegment()` in viewer.go
  - [x] 1.2: Identify how current position is calculated
  - [x] 1.3: Trace data flow for `entryLinePositions` and `rawLinePositions`
  - [x] 1.4: Document the root cause

- [x] Task 2: Implement position lookup function
  - [x] 2.1: Create `findVisibleEntry(yOffset int) int` helper function
  - [x] 2.2: Use binary search on `entryLinePositions` to find entry at offset
  - [x] 2.3: Handle edge cases: offset before first entry, offset after last entry
  - [x] 2.4: Return 1-indexed entry number for display

- [x] Task 3: Implement raw mode position lookup
  - [x] 3.1: Create `findVisibleRawLine(yOffset int) int` helper function
  - [x] 3.2: Use binary search on `rawLinePositions`
  - [x] 3.3: Handle edge cases same as normal mode

- [x] Task 4: Update buildPositionSegment()
  - [x] 4.1: Replace scroll percentage calculation with position lookup
  - [x] 4.2: Use `m.viewport.YOffset` as input to lookup function
  - [x] 4.3: Handle normal mode vs raw mode with appropriate lookup
  - [x] 4.4: Handle case where positions array is empty or incomplete

- [x] Task 5: Handle lazy loading edge cases
  - [x] 5.1: When positions array is incomplete, show position within loaded range
  - [x] 5.2: Consider showing "Entry N/M (of Total)" during lazy load
  - [x] 5.3: Ensure position updates correctly when more content loads

- [x] Task 6: Add unit tests
  - [x] 6.1: Test findVisibleEntry() with various offsets
  - [x] 6.2: Test findVisibleEntry() edge cases (empty, single entry, boundary)
  - [x] 6.3: Test findVisibleRawLine() similarly
  - [x] 6.4: Test buildPositionSegment() returns accurate position
  - [x] 6.5: Test position accuracy during lazy loading

- [x] Task 7: Run build, lint, and test validation
  - [x] 7.1: Run `make build` - verify binary builds
  - [x] 7.2: Run `make lint` - no errors
  - [x] 7.3: Run `make test` - all tests pass

- [ ] Task 8: Manual testing (Requires user verification)
  - [ ] 8.1: Open large conversation (1000+ entries)
  - [ ] 8.2: Scroll slowly - verify indicator increments smoothly
  - [ ] 8.3: Scroll quickly - verify no erratic jumping
  - [ ] 8.4: Test with lazy loading active (before scrolling to bottom)
  - [ ] 8.5: Test after lazy loading complete (scroll to bottom, then back)
  - [ ] 8.6: Toggle raw mode, verify position accurate
  - [ ] 8.7: Use `:N` navigation, verify position updates correctly

## Dev Notes

### Current Implementation Location

`buildPositionSegment()` in `internal/tui/viewer.go`

### Key Data Structures

From Story 4.2:
```go
entryLinePositions []int  // Y offset where each entry starts in viewport
```

From Story 4.3:
```go
rawLinePositions []int    // Y offset where each raw line starts
```

### Binary Search Implementation

```go
// findVisibleEntry returns the 1-indexed entry number visible at yOffset.
// Uses binary search on entryLinePositions for O(log n) lookup.
func (m *ViewerModel) findVisibleEntry(yOffset int) int {
    positions := m.entryLinePositions
    if len(positions) == 0 {
        return 1
    }

    // Binary search for largest position <= yOffset
    lo, hi := 0, len(positions)-1
    result := 0

    for lo <= hi {
        mid := (lo + hi) / 2
        if positions[mid] <= yOffset {
            result = mid
            lo = mid + 1
        } else {
            hi = mid - 1
        }
    }

    return result + 1 // 1-indexed for display
}
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Add findVisibleEntry(), findVisibleRawLine(), update buildPositionSegment() |
| `internal/tui/viewer_test.go` | Add tests for position lookup functions |

### Technical Constraints

| Constraint | Requirement |
|------------|-------------|
| **Performance** | Binary search O(log n), not linear scan |
| **Accuracy** | Position must match topmost visible entry |
| **Lazy loading** | Must work with partial positions array |
| **Both modes** | Normal and raw mode must be accurate |

### References

- [Source: Epic 4 Retrospective 2026-01-19] - Bug report
- [Source: internal/tui/viewer.go] - buildPositionSegment(), entryLinePositions
- [Source: 4-2-command-mode-line-navigation.md] - entryLinePositions implementation
- [Source: 4-3-raw-jsonl-mode-toggle.md] - rawLinePositions implementation

## Implementation Checklist

Before marking story complete, verify:

**Investigation:**
- [x] Root cause documented and confirmed
- [x] Current calculation method identified

**Implementation:**
- [x] `findVisibleEntry()` implemented with binary search
- [x] `findVisibleRawLine()` implemented with binary search
- [x] `buildPositionSegment()` updated to use lookup
- [x] Edge cases handled (empty, boundary, incomplete)

**Testing:**
- [x] Unit tests for position lookup functions
- [x] Unit tests for buildPositionSegment accuracy
- [x] `make build` succeeds
- [x] `make lint` has no errors
- [x] `make test` passes

**Manual Verification (Requires User):**
- [ ] Large conversation scrolling is smooth
- [ ] No oscillation/jumping
- [ ] Lazy loading doesn't break accuracy
- [ ] Raw mode position accurate
- [ ] `:N` navigation updates position correctly

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. **Root Cause Confirmed**: `buildPositionSegment()` at viewer.go:902-924 was using `viewport.ScrollPercent() * totalEntries` to calculate position. This scroll percentage approach failed because:
   - Variable-height entries (markdown, tool blocks) mean scroll position != entry position
   - Lazy loading renders partial content but calculation used full count
   - Rounding/viewport fluctuations caused oscillation

2. **Implementation**: Added two binary search functions:
   - `findVisibleEntry(yOffset int) int` - searches `entryLinePositions` for visible entry
   - `findVisibleRawLine(yOffset int) int` - searches `rawLinePositions` for visible raw line
   - Both return 1-indexed position, handle empty arrays by returning 1

3. **buildPositionSegment() Updated**: Now uses `m.viewport.YOffset` with binary search lookup instead of scroll percentage. Shows "Entry N/M (of Total)" format during lazy loading.

4. **Tests Added**:
   - `TestFindVisibleEntry` - 11 test cases covering empty, single, multiple, boundary, large array
   - `TestFindVisibleRawLine` - 6 test cases
   - `TestBuildPositionSegmentWithAccuratePosition` - 4 test cases for output format
   - `TestBuildPositionSegmentLazyLoading` - 1 test case for lazy loading format

5. **All CI Passes**: `make build`, `make lint`, `make test` all succeed

### File List

| File | Change |
|------|--------|
| `internal/tui/viewer.go` | Added `findPositionByOffset()` helper, `findVisibleEntry()`, `findVisibleRawLine()`, updated `buildPositionSegment()` |
| `internal/tui/viewer_test.go` | Added tests for Story 4.6 position functions |

### Change Log

- 2026-01-19: Fixed position indicator desync by replacing scroll percentage calculation with binary search on position arrays

## Senior Developer Review (AI)

**Reviewer:** Amelia (Dev Agent) | **Date:** 2026-01-19 | **Model:** Claude Opus 4.5

### Review Outcome: ✅ APPROVED (with fixes applied)

### Issues Found and Fixed

| # | Severity | Issue | Resolution |
|---|----------|-------|------------|
| 1 | MEDIUM | Missing test case for negative yOffset | Added `negative offset returns first entry` and `negative offset returns first line` test cases |
| 2 | MEDIUM | Duplicate binary search implementation | Refactored to shared `findPositionByOffset()` helper function |
| 3 | MEDIUM | Test comment "offset before first entry" was unclear | Improved comment to explain binary search behavior |
| 4 | LOW | Comment typo (potential `/ Note:` issue) | Verified correct - no fix needed |

### Verification After Fixes

| Check | Status |
|-------|--------|
| `make build` | ✓ PASS |
| `make lint` | ✓ 0 issues |
| `make test` | ✓ All pass (including new negative offset tests) |

### Summary

Implementation is solid. Core fix correctly replaces scroll percentage with binary search O(log n) lookup. Code quality improved by eliminating duplication. Test coverage expanded to include edge cases (negative offset). Story ready for manual testing (Task 8).
