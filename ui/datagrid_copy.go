package ui

import (
	"strings"
)

// The actions of a DataGrid's copy, which DefaultBindings binds in the grid's key
// context, "DataGrid". The actions package has them as GridCopy and
// GridCopyWithHeaders.
var (
	dataGridCopy        = NewAction("data-grid.copy", "Copy").Hide()
	dataGridCopyHeaders = NewAction("data-grid.copy-with-headers", "Copy with headers").Hide()
)

// DataGridActions lists the actions of a DataGrid: Copy, which copies the selection
// as text that a spreadsheet takes (Ctrl+C and Ctrl+Insert), and Copy with headers,
// which puts the headers of its columns first (Ctrl+Shift+C). They run on the grid
// that has focus, so they can be bound to other keys in the grid's key context,
// "DataGrid", and put in its context menu. Keymap files may name them. The actions
// package has each of them as a variable.
func DataGridActions() []*Action { return []*Action{dataGridCopy, dataGridCopyHeaders} }

// dataGridBindings are the default keys of a DataGrid's actions. Ctrl+C and
// Ctrl+Shift+C copy, as in SQL Server Management Studio's results grid, and
// Ctrl+Insert copies, as it does in Windows' text controls.
func dataGridBindings() []Binding {
	return []Binding{
		BindIn("DataGrid", "ctrl+c", dataGridCopy),
		BindIn("DataGrid", "ctrl+insert", dataGridCopy),
		BindIn("DataGrid", "ctrl+shift+c", dataGridCopyHeaders),
	}
}

// dataGridCopyMax is the most cells a copy of a DataGrid's selection makes text of by
// itself. A copy runs on the UI thread, since it reads the app's rows: 400,000 short
// cells take about 30 ms and 13 MB (BenchmarkDataGridCopy100k), so the limit is a few
// tenths of a second and some tens of MB, which the platform's clipboard then copies
// again. A larger selection is not copied, and a message is announced; CopyText lets
// an app copy more.
var dataGridCopyMax = 2_000_000

// DataGridCopy is a copy of a DataGrid's selection, which a CopyText function turns
// into the text put on the clipboard. It has the rows and the columns that have a
// selected cell, in the order they are shown: a column moved, pinned or sorted, or a
// row filtered out of the data or closed in a tree, is copied as the user sees it.
type DataGridCopy[T any] struct {
	// Rows are the rows that have a selected cell, in the order shown.
	Rows []T
	// Columns are the keys of the columns that have a selected cell, in the order shown.
	Columns []string
	// Headers is set for a copy with the headers of the columns (Ctrl+Shift+C).
	Headers  bool
	selected func(row, col int) bool
	text     func() string
}

// Selected reports whether the cell of Rows[row] in Columns[col] is selected. A
// selection that is not a rectangle, such as a row and a few cells of other rows, has
// cells in Rows and Columns that are not selected.
func (c DataGridCopy[T]) Selected(row, col int) bool { return c.selected(row, col) }

// Text returns the text the grid copies by default: the cells as tab-separated
// values (see DataGridElement.CopyText).
func (c DataGridCopy[T]) Text() string { return c.text() }

// CopyText replaces the text that a copy of the grid's selection puts on the
// clipboard with what fn returns; an empty string copies nothing, for an app that puts
// the text on the clipboard itself. fn gets the rows and columns of the selection and
// its default text (see DataGridCopy). To change only how a column's cells become text,
// set DataColumn.CopyText instead.
//
// By default a copy is the cells as tab-separated values, which a spreadsheet pastes
// into cells: a line for each row that has a selected cell, and on it a field for each
// column that has one, in the order shown, separated by tabs, with the lines separated
// by CR LF. A cell of those rows and columns that is not selected, in a selection that is
// not a rectangle, is an empty field, so the cells keep their columns. A field with a
// tab, a line break or a double quote is quoted as a spreadsheet reads it: in double
// quotes, with each double quote doubled. Ctrl+Shift+C puts a line of the columns'
// headers first. With nothing selected the copy is the focused cell. A selection of
// more than 2,000,000 cells is not copied, and a message says so; fn is still called.
func (g *DataGridElement[T]) CopyText(fn func(c DataGridCopy[T]) string) *DataGridElement[T] {
	g.copyText = fn
	return g
}

// writeTSVField writes s to b as a field of tab-separated values: as it is, or in
// double quotes, with each double quote doubled, when it has a tab, a line break or a
// double quote, as Excel and the other spreadsheets read it on a paste.
func writeTSVField(b *strings.Builder, s string) {
	if !strings.ContainsAny(s, "\t\r\n\"") {
		b.WriteString(s)
		return
	}
	b.WriteByte('"')
	for {
		i := strings.IndexByte(s, '"')
		if i < 0 {
			break
		}
		b.WriteString(s[:i+1])
		b.WriteByte('"')
		s = s[i+1:]
	}
	b.WriteString(s)
	b.WriteByte('"')
}

// dgCopySource is what a copy of a grid's selection is made from: the rows shown (n of
// them, the row at position q being row(q) with the key key(q)), the columns shown,
// and the selection.
type dgCopySource[T any] struct {
	n    int
	row  func(q int) T
	key  func(q int) string
	cols []DataColumn[T]
	sel  []CellRange
}

// copyOf returns the copy of the selection of src, and the number of its cells: the
// rows and the columns that have a selected cell, in the order shown. The copy has its
// Rows when withRows is set, for a CopyText function.
func copyOf[T any](src dgCopySource[T], headers, withRows bool) (DataGridCopy[T], int) {
	idx := newDgSel(src.sel)
	nr := len(src.sel)
	// A range with no column has no cell, so it adds no row.
	live := make([]bool, nr)
	for i := range nr {
		live[i] = !src.sel[i].Columns.empty()
	}
	used := make([]bool, nr)
	var qs []int
	for q := range src.n {
		k, in := src.key(q), false
		for i := range nr {
			if live[i] && idx.rows[i].has(k) {
				used[i], in = true, true
			}
		}
		if in {
			qs = append(qs, q)
		}
	}
	var cs []int
	for c, cl := range src.cols {
		for i := range nr {
			if used[i] && idx.cols[i].has(cl.key()) {
				cs = append(cs, c)
				break
			}
		}
	}
	keys := make([]string, len(cs))
	for j, c := range cs {
		keys[j] = src.cols[c].key()
	}
	selected := func(i, j int) bool { return idx.has(src.key(qs[i]), keys[j]) }
	text := func() string {
		var b strings.Builder
		if headers {
			for j, c := range cs {
				if j > 0 {
					b.WriteByte('\t')
				}
				writeTSVField(&b, src.cols[c].Header)
			}
		}
		for i, q := range qs {
			if i > 0 || headers {
				b.WriteString("\r\n")
			}
			if i == 64 && len(qs) > 128 {
				// The text is grown once to the size the first rows foretell.
				b.Grow(b.Len() / 64 * (len(qs) - 64))
			}
			r, k := src.row(q), src.key(q)
			for j, c := range cs {
				if j > 0 {
					b.WriteByte('\t')
				}
				if nr > 1 && !idx.has(k, keys[j]) {
					continue
				}
				writeTSVField(&b, cellCopyText(src.cols[c], r))
			}
		}
		return b.String()
	}
	c := DataGridCopy[T]{Columns: keys, Headers: headers, selected: selected, text: text}
	if withRows {
		c.Rows = make([]T, len(qs))
		for i, q := range qs {
			c.Rows[i] = src.row(q)
		}
	}
	return c, len(qs) * len(cs)
}

// cellCopyText is the text of a cell of the column cl in the row r for a copy: its
// CopyText, or its Text, or nothing for a column with neither.
func cellCopyText[T any](cl DataColumn[T], r T) string {
	switch {
	case cl.CopyText != nil:
		return cl.CopyText(r)
	case cl.Text != nil:
		return cl.Text(r)
	}
	return ""
}
