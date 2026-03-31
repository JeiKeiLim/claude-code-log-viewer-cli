// Package tui provides the terminal user interface components.
package tui

// MinPaneWidth is the minimum width a pane can have before being hidden.
const MinPaneWidth = 10

// MinPaneHeight is the minimum height a pane can have before being hidden.
const MinPaneHeight = 5

// PaneLayout describes the computed position and size of a single pane in the grid.
type PaneLayout struct {
	Index  int // Pane index (0-based)
	Row    int // Grid row (0-based)
	Col    int // Grid column (0-based)
	X      int // Horizontal offset in terminal columns
	Y      int // Vertical offset in terminal rows
	Width  int // Pane width in columns
	Height int // Pane height in rows
}

// GridLayout describes the full computed layout for all panes.
type GridLayout struct {
	Rows        int          // Number of grid rows
	Cols        int          // Number of grid columns
	Panes       []PaneLayout // Layout for each pane
	TotalWidth  int          // Total available width
	TotalHeight int          // Total available height
	Overflow    bool         // True if panes were dropped due to insufficient space
}

// CalculateGridLayout computes an adaptive grid layout for the given number of panes
// within the specified terminal dimensions. It distributes remainder pixels across
// panes and supports non-uniform last rows (e.g., 5 panes in a 2x3 grid where the
// last row's 2 panes are wider).
//
// Parameters:
//   - paneCount: number of panes to lay out (1-9)
//   - totalWidth: available terminal width in columns
//   - totalHeight: available terminal height in rows
//
// Returns a GridLayout with position and size information for each pane.
func CalculateGridLayout(paneCount, totalWidth, totalHeight int) GridLayout {
	layout := GridLayout{
		TotalWidth:  totalWidth,
		TotalHeight: totalHeight,
	}

	if paneCount <= 0 || totalWidth <= 0 || totalHeight <= 0 {
		return layout
	}

	// Cap at maximum 9 panes
	if paneCount > MaxSessionPanes {
		paneCount = MaxSessionPanes
	}

	rows, cols := calculateGrid(paneCount)
	layout.Rows = rows
	layout.Cols = cols

	if rows == 0 || cols == 0 {
		return layout
	}

	// Check if minimum size constraints can be satisfied
	maxCols := totalWidth / MinPaneWidth
	maxRows := totalHeight / MinPaneHeight
	if maxCols <= 0 || maxRows <= 0 {
		layout.Overflow = true
		return layout
	}

	// Effective grid dimensions (clamp to what fits)
	effectiveCols := cols
	effectiveRows := rows
	if effectiveCols > maxCols {
		effectiveCols = maxCols
		layout.Overflow = true
	}
	if effectiveRows > maxRows {
		effectiveRows = maxRows
		layout.Overflow = true
	}

	// Count panes in last row (may be fewer than cols)
	lastRowPaneCount := paneCount - (effectiveRows-1)*effectiveCols
	if lastRowPaneCount <= 0 {
		lastRowPaneCount = effectiveCols
	}
	if lastRowPaneCount > effectiveCols {
		lastRowPaneCount = effectiveCols
	}

	// Calculate row heights with remainder distribution
	rowHeights := distributeSize(totalHeight, effectiveRows)

	// Calculate column widths per row
	// Full rows use effectiveCols; last row may have fewer (wider) panes
	layout.Panes = make([]PaneLayout, 0, paneCount)

	yOffset := 0
	paneIdx := 0

	for r := 0; r < effectiveRows && paneIdx < paneCount; r++ {
		rowHeight := rowHeights[r]

		// Determine how many panes in this row
		panesInRow := effectiveCols
		if r == effectiveRows-1 {
			panesInRow = lastRowPaneCount
		}

		// Distribute width across panes in this row
		colWidths := distributeSize(totalWidth, panesInRow)

		xOffset := 0
		for c := 0; c < panesInRow && paneIdx < paneCount; c++ {
			colWidth := colWidths[c]

			layout.Panes = append(layout.Panes, PaneLayout{
				Index:  paneIdx,
				Row:    r,
				Col:    c,
				X:      xOffset,
				Y:      yOffset,
				Width:  colWidth,
				Height: rowHeight,
			})

			xOffset += colWidth
			paneIdx++
		}
		yOffset += rowHeight
	}

	return layout
}

// distributeSize divides totalSize evenly across count slots, distributing
// any remainder one pixel at a time to the first slots. This ensures all
// pixels are accounted for and no space is wasted.
//
// Example: distributeSize(101, 3) => [34, 34, 33]
func distributeSize(totalSize, count int) []int {
	if count <= 0 {
		return nil
	}
	base := totalSize / count
	remainder := totalSize % count

	sizes := make([]int, count)
	for i := 0; i < count; i++ {
		sizes[i] = base
		if i < remainder {
			sizes[i]++
		}
	}
	return sizes
}

// GridLayoutForPaneCount is a convenience function that returns just the
// PaneLayout slice for a given pane count and terminal dimensions.
// This is useful for direct integration with existing View() rendering loops.
func GridLayoutForPaneCount(paneCount, totalWidth, totalHeight int) []PaneLayout {
	return CalculateGridLayout(paneCount, totalWidth, totalHeight).Panes
}

// PaneLayoutAt returns the layout for a specific pane index, or a zero-value
// PaneLayout if the index is out of range.
func (g GridLayout) PaneLayoutAt(index int) PaneLayout {
	if index < 0 || index >= len(g.Panes) {
		return PaneLayout{}
	}
	return g.Panes[index]
}

// FitsMinimumSize returns true if all panes in the layout meet minimum size requirements.
func (g GridLayout) FitsMinimumSize() bool {
	for _, p := range g.Panes {
		if p.Width < MinPaneWidth || p.Height < MinPaneHeight {
			return false
		}
	}
	return len(g.Panes) > 0
}

// EffectivePaneCount returns the number of panes that actually fit in the layout.
func (g GridLayout) EffectivePaneCount() int {
	return len(g.Panes)
}

// CoversTotalArea returns true if the pane layouts collectively cover the full
// available width on each row (no gaps). Useful for testing.
func (g GridLayout) CoversTotalArea() bool {
	if len(g.Panes) == 0 {
		return false
	}

	// Check each row covers full width
	rowWidths := make(map[int]int)
	for _, p := range g.Panes {
		rowWidths[p.Row] += p.Width
	}
	for _, w := range rowWidths {
		if w != g.TotalWidth {
			return false
		}
	}

	// Check total height is covered
	rowHeights := make(map[int]int)
	for _, p := range g.Panes {
		if existing, ok := rowHeights[p.Row]; ok {
			if existing != p.Height {
				return false // inconsistent height in same row
			}
		} else {
			rowHeights[p.Row] = p.Height
		}
	}
	totalH := 0
	for r := 0; r < g.Rows; r++ {
		totalH += rowHeights[r]
	}
	return totalH == g.TotalHeight
}
