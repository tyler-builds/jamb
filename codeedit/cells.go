package codeedit

import "unicode/utf8"

// The editor draws on a grid of cells, one per column of a monospace font. A character
// takes one cell, or two when it is a wide East Asian character, and a tab moves to the next
// tab stop. Positions in the text are converted to cells for drawing and for the pointer,
// so the pointer never measures text: the paint measures one cell, and the grid does the rest.
// The vertical moves of the caret keep their goal in cells too, so a move up or down lands
// under the same cell.

// wideRanges are the code points that take two cells: the East Asian wide and fullwidth
// blocks, and the emoji pictographs. They are the common ones; a text with other wide
// characters is drawn with its own font's widths, so the grid can be off by a cell there.
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF}, {0x3400, 0x4DBF}, {0x4E00, 0x9FFF},
	{0xA000, 0xA4CF}, {0xAC00, 0xD7A3}, {0xF900, 0xFAFF}, {0xFE30, 0xFE4F}, {0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6}, {0x1F300, 0x1F64F}, {0x1F900, 0x1F9FF}, {0x20000, 0x3FFFD},
}

// runeCells returns how many cells r takes, not counting a tab.
func runeCells(r rune) int {
	if r < wideRanges[0][0] {
		return 1 // below every wide block, as most text is
	}
	for _, w := range wideRanges {
		if r >= w[0] && r <= w[1] {
			return 2
		}
	}
	return 1
}

// advance returns the cell after cell, for the rune r: a tab goes to the next tab stop.
func advance(cell int, r rune, tab int) int {
	if r == '\t' {
		return cell + tab - cell%tab
	}
	return cell + runeCells(r)
}

// cellOfCol returns the cell where the byte offset col of line starts.
func cellOfCol(line string, col, tab int) int {
	cell := 0
	for i, r := range line {
		if i >= col {
			break
		}
		cell = advance(cell, r, tab)
	}
	return cell
}

// colOfCell returns the byte offset of the boundary of line nearest to the cell. A cell
// past the end of the line is the end of the line.
func colOfCell(line string, cell, tab int) int {
	start := 0
	for i, r := range line {
		next := advance(start, r, tab)
		if cell < next {
			if cell-start < next-cell {
				return i
			}
			return i + utf8.RuneLen(r)
		}
		start = next
	}
	return len(line)
}
