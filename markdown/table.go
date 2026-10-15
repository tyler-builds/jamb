package markdown

import (
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// table returns a table: a grid with a column for each column of the table, as wide as
// its widest cell, and the header row first. Each cell's text is aligned as its
// column's delimiter row says. The grid is as wide as its content, and its box scrolls
// sideways when the table is wider than the view.
//
// Assistive technology reads the table as a table (a data grid with the Grid and Table
// patterns), which a screen reader moves through by row and column: the header cells
// are its column headers, and the rows of the body its rows, counted from 1. Each cell
// is named by its text, and its links stay links of their own. The table has no name,
// since the screen reader says its shape; its description is its size as it is drawn,
// the header row included, such as "Table, 3 columns, 4 rows".
func (b *builder) table(n *east.Table) ui.Element {
	cols := len(n.Alignments)
	if cols == 0 {
		return nil
	}
	tracks := make([]ui.Length, cols)
	for i := range tracks {
		tracks[i] = ui.Auto
	}
	w, line := b.st.Table.BorderWidth, b.st.Table.BorderColor
	grid := ui.Div().Role(access.RoleDataGrid).Grid(tracks...).Style(b.st.Table).AlignSelf(ui.Start).
		BorderEdges(ui.Edges{Top: w, Left: w}, line)
	rows, body := 0, 0 // every row, and the rows of the body, which are the grid's rows
	for r := n.FirstChild(); r != nil; r = r.NextSibling() {
		_, header := r.(*east.TableHeader)
		rows++
		row := 0 // the header row is not one of the grid's rows
		if !header {
			body++
			row = body
		}
		col := 0
		for c := r.FirstChild(); c != nil && col < cols; c = c.NextSibling() {
			grid.Child(b.tableCell(c, n.Alignments[col], header, row, col+1))
			col++
		}
		for ; col < cols; col++ { // a short row is filled out with empty cells
			grid.Child(b.tableCell(nil, n.Alignments[col], header, row, col+1))
		}
	}
	// No name or description: a screen reader announces the size from the Grid pattern.
	grid.AccessGrid(body, cols)
	return ui.Div().ScrollX().Child(grid)
}

// tableCell returns one cell of a table, with the lines on its right and its bottom:
// a header item of the column col, or the data item at row and col. A nil node is an
// empty cell.
func (b *builder) tableCell(n ast.Node, align east.Alignment, header bool, row, col int) ui.Element {
	st := b.st.TableCell
	if header {
		st = st.Merge(b.st.TableHeader)
	}
	w := b.st.Table.BorderWidth
	cell := ui.Div().Style(st).BorderEdges(ui.Edges{Right: w, Bottom: w}, b.st.Table.BorderColor)
	if header {
		cell.Role(access.RoleHeaderItem).AccessCell(0, col)
	} else {
		cell.Role(access.RoleDataItem).AccessCell(row, col)
	}
	if n == nil {
		return cell
	}
	a := ui.Start
	switch align {
	case east.AlignCenter:
		a = ui.Center
	case east.AlignRight:
		a = ui.End
	}
	if in := b.collect(n); !in.blank() {
		cell.Child(b.flow(in, ui.Style{}, a))
	}
	return cell
}
