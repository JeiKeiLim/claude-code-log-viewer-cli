package tui

import (
	"testing"
)

func TestDistributeSize(t *testing.T) {
	tests := []struct {
		name      string
		totalSize int
		count     int
		want      []int
	}{
		{"even split", 100, 2, []int{50, 50}},
		{"even split 3", 90, 3, []int{30, 30, 30}},
		{"remainder 1", 101, 2, []int{51, 50}},
		{"remainder 2 of 3", 101, 3, []int{34, 34, 33}},
		{"single slot", 100, 1, []int{100}},
		{"zero count", 100, 0, nil},
		{"negative count", 100, -1, nil},
		{"zero size", 0, 3, []int{0, 0, 0}},
		{"small size", 2, 3, []int{1, 1, 0}},
		{"all remainder", 3, 4, []int{1, 1, 1, 0}},
		{"large remainder", 10, 3, []int{4, 3, 3}},
		{"exact 9-way split", 90, 9, []int{10, 10, 10, 10, 10, 10, 10, 10, 10}},
		{"9-way with remainder", 100, 9, []int{12, 11, 11, 11, 11, 11, 11, 11, 11}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := distributeSize(tt.totalSize, tt.count)
			if tt.want == nil {
				if got != nil {
					t.Errorf("distributeSize(%d, %d) = %v, want nil", tt.totalSize, tt.count, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("distributeSize(%d, %d) = %v (len %d), want %v (len %d)",
					tt.totalSize, tt.count, got, len(got), tt.want, len(tt.want))
			}
			sum := 0
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("distributeSize(%d, %d)[%d] = %d, want %d",
						tt.totalSize, tt.count, i, v, tt.want[i])
				}
				sum += v
			}
			if sum != tt.totalSize {
				t.Errorf("distributeSize(%d, %d) sums to %d, want %d",
					tt.totalSize, tt.count, sum, tt.totalSize)
			}
		})
	}
}

func TestDistributeSizeAlwaysSumsCorrectly(t *testing.T) {
	// Property-based: sum must always equal totalSize
	for totalSize := 0; totalSize <= 200; totalSize += 7 {
		for count := 1; count <= 9; count++ {
			got := distributeSize(totalSize, count)
			sum := 0
			for _, v := range got {
				sum += v
			}
			if sum != totalSize {
				t.Errorf("distributeSize(%d, %d) sums to %d", totalSize, count, sum)
			}
		}
	}
}

func TestCalculateGridLayout_ZeroAndEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		paneCount int
		width     int
		height    int
		wantPanes int
		wantRows  int
		wantCols  int
	}{
		{"zero panes", 0, 100, 50, 0, 0, 0},
		{"negative panes", -1, 100, 50, 0, 0, 0},
		{"zero width", 3, 0, 50, 0, 0, 0},
		{"zero height", 3, 100, 0, 0, 0, 0},
		{"negative width", 3, -10, 50, 0, 0, 0},
		{"negative height", 3, 100, -10, 0, 0, 0},
		{"all zero", 0, 0, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := CalculateGridLayout(tt.paneCount, tt.width, tt.height)
			if len(layout.Panes) != tt.wantPanes {
				t.Errorf("CalculateGridLayout(%d, %d, %d) got %d panes, want %d",
					tt.paneCount, tt.width, tt.height, len(layout.Panes), tt.wantPanes)
			}
			if layout.Rows != tt.wantRows {
				t.Errorf("Rows = %d, want %d", layout.Rows, tt.wantRows)
			}
			if layout.Cols != tt.wantCols {
				t.Errorf("Cols = %d, want %d", layout.Cols, tt.wantCols)
			}
		})
	}
}

func TestCalculateGridLayout_SinglePane(t *testing.T) {
	layout := CalculateGridLayout(1, 120, 40)

	if len(layout.Panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(layout.Panes))
	}

	p := layout.Panes[0]
	if p.X != 0 || p.Y != 0 {
		t.Errorf("single pane should be at (0,0), got (%d,%d)", p.X, p.Y)
	}
	if p.Width != 120 || p.Height != 40 {
		t.Errorf("single pane should fill space: got %dx%d, want 120x40", p.Width, p.Height)
	}
	if p.Row != 0 || p.Col != 0 {
		t.Errorf("single pane should be at grid (0,0), got (%d,%d)", p.Row, p.Col)
	}
	if p.Index != 0 {
		t.Errorf("single pane index should be 0, got %d", p.Index)
	}
}

func TestCalculateGridLayout_TwoPanes(t *testing.T) {
	layout := CalculateGridLayout(2, 100, 50)

	if layout.Rows != 1 || layout.Cols != 2 {
		t.Errorf("2 panes should be 1x2, got %dx%d", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(layout.Panes))
	}

	// First pane at (0,0)
	p0 := layout.Panes[0]
	if p0.X != 0 || p0.Y != 0 {
		t.Errorf("pane 0 at (%d,%d), want (0,0)", p0.X, p0.Y)
	}
	if p0.Width != 50 {
		t.Errorf("pane 0 width = %d, want 50", p0.Width)
	}

	// Second pane at (50,0)
	p1 := layout.Panes[1]
	if p1.X != 50 || p1.Y != 0 {
		t.Errorf("pane 1 at (%d,%d), want (50,0)", p1.X, p1.Y)
	}
	if p1.Width != 50 {
		t.Errorf("pane 1 width = %d, want 50", p1.Width)
	}
}

func TestCalculateGridLayout_TwoPanesOddWidth(t *testing.T) {
	layout := CalculateGridLayout(2, 101, 50)

	if len(layout.Panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(layout.Panes))
	}

	// Remainder pixel goes to first pane
	p0 := layout.Panes[0]
	p1 := layout.Panes[1]

	if p0.Width+p1.Width != 101 {
		t.Errorf("widths %d+%d = %d, want 101", p0.Width, p1.Width, p0.Width+p1.Width)
	}
	if p0.Width != 51 || p1.Width != 50 {
		t.Errorf("got widths %d,%d - want 51,50", p0.Width, p1.Width)
	}
	// X positions must be contiguous
	if p1.X != p0.Width {
		t.Errorf("pane 1 X=%d, should be %d", p1.X, p0.Width)
	}
}

func TestCalculateGridLayout_FourPanes2x2(t *testing.T) {
	layout := CalculateGridLayout(4, 100, 50)

	if layout.Rows != 2 || layout.Cols != 2 {
		t.Errorf("4 panes should be 2x2, got %dx%d", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 4 {
		t.Fatalf("expected 4 panes, got %d", len(layout.Panes))
	}

	// Each pane 50x25
	for i, p := range layout.Panes {
		if p.Width != 50 || p.Height != 25 {
			t.Errorf("pane %d: %dx%d, want 50x25", i, p.Width, p.Height)
		}
		if p.Index != i {
			t.Errorf("pane %d: index=%d", i, p.Index)
		}
	}

	// Verify positions
	expectPositions := [][2]int{{0, 0}, {50, 0}, {0, 25}, {50, 25}}
	for i, exp := range expectPositions {
		p := layout.Panes[i]
		if p.X != exp[0] || p.Y != exp[1] {
			t.Errorf("pane %d at (%d,%d), want (%d,%d)", i, p.X, p.Y, exp[0], exp[1])
		}
	}
}

func TestCalculateGridLayout_FivePanes_LastRowWider(t *testing.T) {
	// 5 panes in 2x3 grid: row 0 has 3 panes, row 1 has 2 wider panes
	layout := CalculateGridLayout(5, 120, 50)

	if layout.Rows != 2 || layout.Cols != 3 {
		t.Errorf("5 panes should be 2x3, got %dx%d", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 5 {
		t.Fatalf("expected 5 panes, got %d", len(layout.Panes))
	}

	// Row 0: 3 panes, each 40 wide
	for i := 0; i < 3; i++ {
		p := layout.Panes[i]
		if p.Width != 40 {
			t.Errorf("pane %d (row 0) width = %d, want 40", i, p.Width)
		}
		if p.Row != 0 {
			t.Errorf("pane %d row = %d, want 0", i, p.Row)
		}
	}

	// Row 1: 2 panes, each 60 wide (120/2)
	for i := 3; i < 5; i++ {
		p := layout.Panes[i]
		if p.Width != 60 {
			t.Errorf("pane %d (row 1) width = %d, want 60", i, p.Width)
		}
		if p.Row != 1 {
			t.Errorf("pane %d row = %d, want 1", i, p.Row)
		}
	}
}

func TestCalculateGridLayout_SevenPanes_LastRowFewer(t *testing.T) {
	// 7 panes in 3x3 grid: rows 0,1 have 3 panes each, row 2 has 1 pane
	layout := CalculateGridLayout(7, 120, 60)

	if layout.Rows != 3 || layout.Cols != 3 {
		t.Errorf("7 panes should be 3x3, got %dx%d", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 7 {
		t.Fatalf("expected 7 panes, got %d", len(layout.Panes))
	}

	// Row 2 has 1 pane that fills full width
	p6 := layout.Panes[6]
	if p6.Width != 120 {
		t.Errorf("last row single pane width = %d, want 120", p6.Width)
	}
	if p6.Row != 2 || p6.Col != 0 {
		t.Errorf("pane 6 at grid (%d,%d), want (2,0)", p6.Row, p6.Col)
	}
}

func TestCalculateGridLayout_EightPanes_LastRowTwo(t *testing.T) {
	layout := CalculateGridLayout(8, 120, 60)

	if len(layout.Panes) != 8 {
		t.Fatalf("expected 8 panes, got %d", len(layout.Panes))
	}

	// Last row has 2 panes at 60 each
	p6 := layout.Panes[6]
	p7 := layout.Panes[7]
	if p6.Width != 60 || p7.Width != 60 {
		t.Errorf("last row panes: %d, %d - want 60, 60", p6.Width, p7.Width)
	}
}

func TestCalculateGridLayout_NinePanes_Full3x3(t *testing.T) {
	layout := CalculateGridLayout(9, 120, 60)

	if layout.Rows != 3 || layout.Cols != 3 {
		t.Errorf("9 panes should be 3x3, got %dx%d", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 9 {
		t.Fatalf("expected 9 panes, got %d", len(layout.Panes))
	}

	// All panes 40x20
	for i, p := range layout.Panes {
		if p.Width != 40 || p.Height != 20 {
			t.Errorf("pane %d: %dx%d, want 40x20", i, p.Width, p.Height)
		}
	}

	if !layout.CoversTotalArea() {
		t.Error("9 pane layout should cover total area")
	}
}

func TestCalculateGridLayout_RemainderPixels(t *testing.T) {
	// 121 width / 3 cols = 40 + remainder 1
	layout := CalculateGridLayout(3, 121, 41)

	if len(layout.Panes) != 3 {
		t.Fatalf("expected 3 panes, got %d", len(layout.Panes))
	}

	totalW := 0
	for _, p := range layout.Panes {
		totalW += p.Width
	}
	if totalW != 121 {
		t.Errorf("total width = %d, want 121", totalW)
	}

	// Height should also sum correctly
	if layout.Panes[0].Height != 41 {
		t.Errorf("pane height = %d, want 41 (single row)", layout.Panes[0].Height)
	}
}

func TestCalculateGridLayout_RemainderPixels2x3(t *testing.T) {
	// 5 panes, 101x51 terminal
	layout := CalculateGridLayout(5, 101, 51)

	// Row heights: 51/2 = [26, 25]
	// Row 0 widths (3 panes): 101/3 = [34, 34, 33]
	// Row 1 widths (2 panes): 101/2 = [51, 50]

	if len(layout.Panes) != 5 {
		t.Fatalf("expected 5 panes, got %d", len(layout.Panes))
	}

	// Verify row 0 widths sum to 101
	row0width := 0
	for i := 0; i < 3; i++ {
		row0width += layout.Panes[i].Width
	}
	if row0width != 101 {
		t.Errorf("row 0 total width = %d, want 101", row0width)
	}

	// Verify row 1 widths sum to 101
	row1width := 0
	for i := 3; i < 5; i++ {
		row1width += layout.Panes[i].Width
	}
	if row1width != 101 {
		t.Errorf("row 1 total width = %d, want 101", row1width)
	}

	// Verify height sums
	h0 := layout.Panes[0].Height
	h1 := layout.Panes[3].Height
	if h0+h1 != 51 {
		t.Errorf("heights %d+%d = %d, want 51", h0, h1, h0+h1)
	}
}

func TestCalculateGridLayout_CapsAtMaxPanes(t *testing.T) {
	layout := CalculateGridLayout(15, 300, 200)

	if len(layout.Panes) != MaxSessionPanes {
		t.Errorf("expected max %d panes, got %d", MaxSessionPanes, len(layout.Panes))
	}
}

func TestCalculateGridLayout_TinyTerminal(t *testing.T) {
	// Terminal too small for minimum pane sizes
	layout := CalculateGridLayout(4, 15, 8)

	// 2x2 grid would need 20x10, but we only have 15x8
	// Should set overflow
	if !layout.Overflow {
		t.Error("should set Overflow for tiny terminal")
	}
}

func TestCalculateGridLayout_MinimumViableTerminal(t *testing.T) {
	// Just barely fits 1 pane
	layout := CalculateGridLayout(1, MinPaneWidth, MinPaneHeight)

	if len(layout.Panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(layout.Panes))
	}
	if layout.Overflow {
		t.Error("should not overflow for single pane at min size")
	}
}

func TestCalculateGridLayout_ContiguousPositions(t *testing.T) {
	// Verify X positions are contiguous within each row for all pane counts
	for count := 1; count <= 9; count++ {
		layout := CalculateGridLayout(count, 200, 100)

		// Group panes by row
		rows := make(map[int][]PaneLayout)
		for _, p := range layout.Panes {
			rows[p.Row] = append(rows[p.Row], p)
		}

		for rowIdx, panes := range rows {
			expectedX := 0
			for _, p := range panes {
				if p.X != expectedX {
					t.Errorf("count=%d row=%d pane col=%d: X=%d, want %d",
						count, rowIdx, p.Col, p.X, expectedX)
				}
				expectedX += p.Width
			}
			// Row must sum to total width
			if expectedX != 200 {
				t.Errorf("count=%d row=%d: total width = %d, want 200",
					count, rowIdx, expectedX)
			}
		}
	}
}

func TestCalculateGridLayout_ContiguousYPositions(t *testing.T) {
	// Verify Y positions are contiguous across rows
	for count := 1; count <= 9; count++ {
		layout := CalculateGridLayout(count, 200, 100)

		if len(layout.Panes) == 0 {
			continue
		}

		// Get unique rows and their Y + Height
		rowY := make(map[int]int)
		rowH := make(map[int]int)
		for _, p := range layout.Panes {
			rowY[p.Row] = p.Y
			rowH[p.Row] = p.Height
		}

		expectedY := 0
		for r := 0; r < layout.Rows; r++ {
			if rowY[r] != expectedY {
				t.Errorf("count=%d row=%d: Y=%d, want %d", count, r, rowY[r], expectedY)
			}
			expectedY += rowH[r]
		}
		if expectedY != 100 {
			t.Errorf("count=%d: total height = %d, want 100", count, expectedY)
		}
	}
}

func TestCalculateGridLayout_CoversTotalArea(t *testing.T) {
	// For all pane counts with evenly divisible dimensions, should cover total area
	for count := 1; count <= 9; count++ {
		layout := CalculateGridLayout(count, 180, 90)
		if !layout.CoversTotalArea() {
			t.Errorf("count=%d: layout does not cover total area", count)
		}
	}
}

func TestCalculateGridLayout_AllPaneCountsWithRemainder(t *testing.T) {
	// Test all pane counts 1-9 with non-divisible terminal size
	for count := 1; count <= 9; count++ {
		layout := CalculateGridLayout(count, 173, 97)

		if len(layout.Panes) != count {
			t.Errorf("count=%d: got %d panes", count, len(layout.Panes))
		}

		// Every layout must cover the full area
		if !layout.CoversTotalArea() {
			t.Errorf("count=%d: does not cover 173x97", count)
		}
	}
}

func TestGridLayout_PaneLayoutAt(t *testing.T) {
	layout := CalculateGridLayout(4, 100, 50)

	// Valid index
	p := layout.PaneLayoutAt(2)
	if p.Index != 2 {
		t.Errorf("PaneLayoutAt(2) index = %d", p.Index)
	}

	// Out of range
	zero := layout.PaneLayoutAt(10)
	if zero.Width != 0 || zero.Height != 0 {
		t.Error("PaneLayoutAt(10) should return zero value")
	}

	// Negative
	zero = layout.PaneLayoutAt(-1)
	if zero.Width != 0 {
		t.Error("PaneLayoutAt(-1) should return zero value")
	}
}

func TestGridLayout_FitsMinimumSize(t *testing.T) {
	// Large terminal - should fit
	layout := CalculateGridLayout(9, 180, 90)
	if !layout.FitsMinimumSize() {
		t.Error("9 panes in 180x90 should fit minimum size")
	}

	// Single pane at exact minimum
	layout = CalculateGridLayout(1, MinPaneWidth, MinPaneHeight)
	if !layout.FitsMinimumSize() {
		t.Error("1 pane at minimum dimensions should fit")
	}

	// Empty layout
	layout = CalculateGridLayout(0, 100, 50)
	if layout.FitsMinimumSize() {
		t.Error("empty layout should not fit minimum size")
	}
}

func TestGridLayout_EffectivePaneCount(t *testing.T) {
	tests := []struct {
		count int
		want  int
	}{
		{0, 0},
		{1, 1},
		{5, 5},
		{9, 9},
		{15, 9}, // Capped
	}

	for _, tt := range tests {
		layout := CalculateGridLayout(tt.count, 200, 100)
		if layout.EffectivePaneCount() != tt.want {
			t.Errorf("count=%d: EffectivePaneCount() = %d, want %d",
				tt.count, layout.EffectivePaneCount(), tt.want)
		}
	}
}

func TestGridLayoutForPaneCount(t *testing.T) {
	panes := GridLayoutForPaneCount(4, 100, 50)
	if len(panes) != 4 {
		t.Errorf("expected 4 panes, got %d", len(panes))
	}

	// Should be identical to CalculateGridLayout
	layout := CalculateGridLayout(4, 100, 50)
	for i, p := range panes {
		exp := layout.Panes[i]
		if p != exp {
			t.Errorf("pane %d: got %+v, want %+v", i, p, exp)
		}
	}
}

func TestGridLayoutForPaneCount_Zero(t *testing.T) {
	panes := GridLayoutForPaneCount(0, 100, 50)
	if len(panes) != 0 {
		t.Errorf("expected 0 panes, got %d", len(panes))
	}
}

func TestCalculateGridLayout_RowColAssignment(t *testing.T) {
	// Verify Row/Col assignments for 6 panes (2x3 grid)
	layout := CalculateGridLayout(6, 120, 60)

	expected := [][2]int{
		{0, 0}, {0, 1}, {0, 2}, // Row 0
		{1, 0}, {1, 1}, {1, 2}, // Row 1
	}

	for i, exp := range expected {
		p := layout.Panes[i]
		if p.Row != exp[0] || p.Col != exp[1] {
			t.Errorf("pane %d: got (%d,%d), want (%d,%d)", i, p.Row, p.Col, exp[0], exp[1])
		}
	}
}

func TestCalculateGridLayout_ThreePanes(t *testing.T) {
	layout := CalculateGridLayout(3, 120, 40)

	if layout.Rows != 1 || layout.Cols != 3 {
		t.Errorf("3 panes: got %dx%d, want 1x3", layout.Rows, layout.Cols)
	}

	for i, p := range layout.Panes {
		if p.Width != 40 || p.Height != 40 {
			t.Errorf("pane %d: %dx%d, want 40x40", i, p.Width, p.Height)
		}
	}
}

func TestCalculateGridLayout_SixPanes(t *testing.T) {
	layout := CalculateGridLayout(6, 120, 60)

	if layout.Rows != 2 || layout.Cols != 3 {
		t.Errorf("6 panes: got %dx%d, want 2x3", layout.Rows, layout.Cols)
	}
	if len(layout.Panes) != 6 {
		t.Fatalf("expected 6 panes, got %d", len(layout.Panes))
	}

	// All panes should be 40x30
	for i, p := range layout.Panes {
		if p.Width != 40 || p.Height != 30 {
			t.Errorf("pane %d: %dx%d, want 40x30", i, p.Width, p.Height)
		}
	}
}

func TestCalculateGridLayout_OverflowClampsCols(t *testing.T) {
	// 3 panes in 1x3, but only 25 wide -> max 2 cols
	layout := CalculateGridLayout(3, 25, 50)

	if !layout.Overflow {
		t.Error("should overflow: can only fit 2 cols at width 25")
	}
}

func TestCalculateGridLayout_OverflowClampsRows(t *testing.T) {
	// 4 panes in 2x2, but only 8 tall -> max 1 row
	layout := CalculateGridLayout(4, 100, 8)

	if !layout.Overflow {
		t.Error("should overflow: can only fit 1 row at height 8")
	}
}

func TestGridLayout_CoversTotalArea_Empty(t *testing.T) {
	layout := GridLayout{}
	if layout.CoversTotalArea() {
		t.Error("empty layout should not cover total area")
	}
}

func TestCalculateGridLayout_LargeTerminal(t *testing.T) {
	// Realistic large terminal: 240x80
	layout := CalculateGridLayout(9, 240, 80)

	// 3x3 grid: 80x26/27
	if len(layout.Panes) != 9 {
		t.Fatalf("expected 9 panes, got %d", len(layout.Panes))
	}

	if !layout.CoversTotalArea() {
		t.Error("should cover total area")
	}

	for _, p := range layout.Panes {
		if p.Width < MinPaneWidth || p.Height < MinPaneHeight {
			t.Errorf("pane %d too small: %dx%d", p.Index, p.Width, p.Height)
		}
	}
}

func TestCalculateGridLayout_StandardTerminal80x24(t *testing.T) {
	// Standard 80x24 terminal with 4 panes
	layout := CalculateGridLayout(4, 80, 24)

	if len(layout.Panes) != 4 {
		t.Fatalf("expected 4 panes, got %d", len(layout.Panes))
	}

	// 2x2 grid: 40x12 each
	for i, p := range layout.Panes {
		if p.Width != 40 || p.Height != 12 {
			t.Errorf("pane %d: %dx%d, want 40x12", i, p.Width, p.Height)
		}
	}
}

func TestCalculateGridLayout_TotalWidthAndHeight(t *testing.T) {
	layout := CalculateGridLayout(5, 150, 80)

	if layout.TotalWidth != 150 {
		t.Errorf("TotalWidth = %d, want 150", layout.TotalWidth)
	}
	if layout.TotalHeight != 80 {
		t.Errorf("TotalHeight = %d, want 80", layout.TotalHeight)
	}
}

func TestCalculateGridLayout_NoOverlapBetweenPanes(t *testing.T) {
	// For all pane counts, verify no panes overlap
	for count := 1; count <= 9; count++ {
		layout := CalculateGridLayout(count, 200, 100)

		for i := 0; i < len(layout.Panes); i++ {
			for j := i + 1; j < len(layout.Panes); j++ {
				pi := layout.Panes[i]
				pj := layout.Panes[j]

				// Check horizontal overlap on same row
				if pi.Row == pj.Row {
					iRight := pi.X + pi.Width
					jRight := pj.X + pj.Width
					if pi.X < jRight && pj.X < iRight {
						// Same row, check if actually overlapping
						if pi.X < pj.X+pj.Width && pj.X < pi.X+pi.Width {
							if pi.Col != pj.Col {
								// Adjacent panes should not overlap
								if pi.X+pi.Width > pj.X && pi.Col < pj.Col {
									t.Errorf("count=%d: panes %d and %d overlap horizontally", count, i, j)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestGridLayout_FitsMinimumSize_SmallPanes(t *testing.T) {
	// Construct a layout where panes are below minimum
	layout := GridLayout{
		Panes: []PaneLayout{
			{Width: 5, Height: 3}, // Below MinPaneWidth and MinPaneHeight
		},
	}
	if layout.FitsMinimumSize() {
		t.Error("should not fit minimum size with small panes")
	}
}

func TestCalculateGridLayout_ClampedLastRowEdgeCases(t *testing.T) {
	// Test where effective rows are clamped such that lastRowPaneCount > effectiveCols
	// 9 panes = 3x3 grid, but terminal only fits 2 cols (width=20) and 2 rows (height=10)
	layout := CalculateGridLayout(9, 20, 10)

	if !layout.Overflow {
		t.Error("should overflow: 3x3 grid in 20x10 terminal")
	}
	// Should still produce valid panes
	for _, p := range layout.Panes {
		if p.Width <= 0 || p.Height <= 0 {
			t.Errorf("pane %d has non-positive dimensions: %dx%d", p.Index, p.Width, p.Height)
		}
	}
}

func TestCalculateGridLayout_CoversTotalArea_WithIncompleteLastRow(t *testing.T) {
	// 8 panes in 3x3: last row has 2 panes
	layout := CalculateGridLayout(8, 180, 90)
	if !layout.CoversTotalArea() {
		t.Error("8 panes in 180x90 should cover total area")
	}
}

func TestGridLayout_CoversTotalArea_InconsistentHeight(t *testing.T) {
	// Construct layout with inconsistent heights in same row - should fail
	layout := GridLayout{
		Rows:        1,
		Cols:        2,
		TotalWidth:  100,
		TotalHeight: 50,
		Panes: []PaneLayout{
			{Row: 0, Col: 0, Width: 50, Height: 25},
			{Row: 0, Col: 1, Width: 50, Height: 30}, // Different height
		},
	}
	if layout.CoversTotalArea() {
		t.Error("inconsistent heights in same row should not cover total area")
	}
}

// Benchmark the layout engine
func BenchmarkCalculateGridLayout(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateGridLayout(9, 240, 80)
	}
}

func BenchmarkCalculateGridLayout_AllCounts(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for count := 1; count <= 9; count++ {
			CalculateGridLayout(count, 240, 80)
		}
	}
}
