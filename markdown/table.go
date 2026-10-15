package markdown

import (
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/tyler-builds/jamb/ui"
)

// table returns a table: a grid with a column for each column of the table, as wide as
// its widest cell, and the header row first. Each cell's text is aligned as its
// column's delimiter row says. The grid is as wide as its content, and its box scrolls
// sideways when the table is wider than the view.
//
// Assistive technology reads the table as a group named by its size, such as "Table,
// 3 columns, 4 rows", with the cells' text in order, row by row.
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
	grid := ui.Div().Grid(tracks...).Style(b.st.Table).AlignSelf(ui.Start).
		BorderEdges(ui.Edges{Top: w, Left: w}, line)
	rows := 0
	for r := n.FirstChild(); r != nil; r = r.NextSibling() {
		_, header := r.(*east.TableHeader)
		rows++
		col := 0
		for c := r.FirstChild(); c != nil && col < cols; c = c.NextSibling() {
			grid.Child(b.tableCell(c, n.Alignments[col], header))
			col++
		}
		for ; col < cols; col++ { // a short row is filled out with empty cells
			grid.Child(b.tableCell(nil, n.Alignments[col], header))
		}
	}
	grid.Label(b.s.MarkdownTable(cols, rows))
	return ui.Div().ScrollX().Child(grid)
}

// tableCell returns one cell of a table, with the lines on its right and its bottom.
// A nil node is an empty cell.
func (b *builder) tableCell(n ast.Node, align east.Alignment, header bool) ui.Element {
	st := b.st.TableCell
	if header {
		st = st.Merge(b.st.TableHeader)
	}
	w := b.st.Table.BorderWidth
	cell := ui.Div().Style(st).BorderEdges(ui.Edges{Right: w, Bottom: w}, b.st.Table.BorderColor)
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
