package ui

import (
	"slices"
	"strings"
	"unsafe"
)

// KeySet is a set of the keys of a DataGrid's rows or of its columns: the keys in
// Keys, or, when All is set, every key but those in Keys. KeySet{All: true} is every
// row or every column, including the rows and columns that are added later.
type KeySet struct {
	// All makes the set every key but the ones in Keys.
	All bool
	// Keys are the keys of the set, or with All the keys left out of it.
	Keys []string
}

// Has reports whether key is in the set.
func (s KeySet) Has(key string) bool {
	return slices.Contains(s.Keys, key) != s.All
}

// empty reports whether the set has no key. A set with All is taken to have keys.
func (s KeySet) empty() bool { return !s.All && len(s.Keys) == 0 }

// CellRange is a block of the cells of a DataGrid: the cells whose row is in Rows and
// whose column is in Columns. A selection of cells is a list of ranges (see
// DataGridElement.CellRanges), and a cell is selected when a range of the list has it:
//
//	ui.CellRange{Rows: ui.KeySet{All: true}, Columns: ui.KeySet{All: true}}         // every cell
//	ui.CellRange{Rows: ui.KeySet{Keys: []string{"r7"}}, Columns: ui.KeySet{All: true}} // row r7
//	ui.CellRange{Rows: ui.KeySet{All: true}, Columns: ui.KeySet{Keys: []string{"Total"}}} // the Total column
//
// The ranges name rows and columns by key, so a selection follows its rows through a
// sort, as a selection of cells does.
type CellRange struct {
	// Rows are the rows of the range.
	Rows KeySet
	// Columns are the columns of the range.
	Columns KeySet
}

// Has reports whether the cell c is in the range.
func (r CellRange) Has(c CellKey) bool { return r.Rows.Has(c.Row) && r.Columns.Has(c.Column) }

// keysIn returns the set of the keys ks.
func keysIn(ks ...string) KeySet { return KeySet{Keys: ks} }

// allKeys is the set of every key.
var allKeys = KeySet{All: true}

// keyLookup returns a test of whether a key is one of ks: a scan for a few keys, and a
// map for many.
func keyLookup(ks []string) func(string) bool {
	if len(ks) <= 16 {
		return func(k string) bool { return slices.Contains(ks, k) }
	}
	m := make(map[string]struct{}, len(ks))
	for _, k := range ks {
		m[k] = struct{}{}
	}
	return func(k string) bool { _, ok := m[k]; return ok }
}

// keysWhere returns the keys of a for which keep returns true, in their order.
func keysWhere(a []string, keep func(string) bool) []string {
	var out []string
	for _, k := range a {
		if keep(k) {
			out = append(out, k)
		}
	}
	return out
}

// keysUnion returns the keys of a and then those of b that are not in a.
func keysUnion(a, b []string) []string {
	in := keyLookup(a)
	out := slices.Clip(slices.Clone(a))
	for _, k := range b {
		if !in(k) {
			out = append(out, k)
		}
	}
	return out
}

// minus returns the keys of s that are not in o.
func (s KeySet) minus(o KeySet) KeySet {
	in := keyLookup(o.Keys)
	switch {
	case s.All && o.All: // (every key but s.Keys) less (every key but o.Keys)
		mine := keyLookup(s.Keys)
		return KeySet{Keys: keysWhere(o.Keys, func(k string) bool { return !mine(k) })}
	case s.All:
		return KeySet{All: true, Keys: keysUnion(s.Keys, o.Keys)}
	case o.All:
		return KeySet{Keys: keysWhere(s.Keys, in)}
	}
	return KeySet{Keys: keysWhere(s.Keys, func(k string) bool { return !in(k) })}
}

// and returns the keys that are in both s and o.
func (s KeySet) and(o KeySet) KeySet {
	switch {
	case s.All && o.All:
		return KeySet{All: true, Keys: keysUnion(s.Keys, o.Keys)}
	case s.All:
		return o.minus(KeySet{Keys: s.Keys})
	case o.All:
		return s.minus(KeySet{Keys: o.Keys})
	}
	in := keyLookup(o.Keys)
	return KeySet{Keys: keysWhere(s.Keys, in)}
}

// minus returns the cells of r that are not in o, as at most two ranges: the rows of r
// that o does not have, in r's columns, and the rows they share, in the columns of r
// that o does not have.
func (r CellRange) minus(o CellRange) []CellRange {
	var out []CellRange
	if rows := r.Rows.minus(o.Rows); !rows.empty() {
		out = append(out, CellRange{Rows: rows, Columns: r.Columns})
	}
	if rows, cols := r.Rows.and(o.Rows), r.Columns.minus(o.Columns); !rows.empty() && !cols.empty() {
		out = append(out, CellRange{Rows: rows, Columns: cols})
	}
	return out
}

// rangesMinus returns the selection rs less the cells of o.
func rangesMinus(rs []CellRange, o CellRange) []CellRange {
	out := make([]CellRange, 0, len(rs)+1)
	for _, r := range rs {
		out = append(out, r.minus(o)...)
	}
	return out
}

// rangesHave reports whether a range of rs has the cell c.
func rangesHave(rs []CellRange, c CellKey) bool {
	for _, r := range rs {
		if r.Has(c) {
			return true
		}
	}
	return false
}

// dgKeyIndex is a KeySet made quick to look up: a map of its keys when it has many.
type dgKeyIndex struct {
	all bool
	in  func(string) bool
	n   int
}

func newKeyIndex(s KeySet) dgKeyIndex {
	return dgKeyIndex{all: s.All, in: keyLookup(s.Keys), n: len(s.Keys)}
}

func (x *dgKeyIndex) has(k string) bool { return x.in(k) != x.all }

// whole reports whether the set is every key, with none left out.
func (x *dgKeyIndex) whole() bool { return x.all && x.n == 0 }

// dgSel is a selection of cells made quick to look up, for the build of the rows: the
// ranges of the selection, each with its rows and columns indexed.
type dgSel struct {
	rows, cols []dgKeyIndex
}

// newDgSel indexes the ranges rs.
func newDgSel(rs []CellRange) *dgSel {
	s := &dgSel{rows: make([]dgKeyIndex, len(rs)), cols: make([]dgKeyIndex, len(rs))}
	for i, r := range rs {
		s.rows[i], s.cols[i] = newKeyIndex(r.Rows), newKeyIndex(r.Columns)
	}
	return s
}

// has reports whether the cell in the row with the key row and the column col is selected.
func (s *dgSel) has(row, col string) bool {
	if s == nil {
		return false
	}
	for i := range s.rows {
		if s.rows[i].has(row) && s.cols[i].has(col) {
			return true
		}
	}
	return false
}

// wholeRow reports whether a range of the selection has every column of the row with
// the key row.
func (s *dgSel) wholeRow(row string) bool {
	if s == nil {
		return false
	}
	for i := range s.rows {
		if s.cols[i].whole() && s.rows[i].has(row) {
			return true
		}
	}
	return false
}

// wholeColumn reports whether a range of the selection has every row of the column col.
func (s *dgSel) wholeColumn(col string) bool {
	if s == nil {
		return false
	}
	for i := range s.rows {
		if s.rows[i].whole() && s.cols[i].has(col) {
			return true
		}
	}
	return false
}

// rangesSelection returns the index of the ranges rs, made again only when rs changes.
func (sh *dgShared[T]) rangesSelection(rs []CellRange) *dgSel {
	ident := unsafe.Pointer(unsafe.SliceData(rs))
	if sh.rangeOK && sh.rangeIdent == ident && sh.rangeN == len(rs) {
		return sh.rangeSel
	}
	sh.rangeOK, sh.rangeIdent, sh.rangeN, sh.rangeSel = true, ident, len(rs), newDgSel(rs)
	return sh.rangeSel
}

// rangesOfCells returns the selection of the cells cs as ranges: the rows whose
// selected cells are in the same columns share one range, so a rectangle of cells is
// one range.
func rangesOfCells(cs []CellKey) []CellRange {
	if len(cs) == 0 {
		return nil
	}
	var rowOrder []string
	colsOf := map[string][]string{}
	for _, c := range cs {
		cols, seen := colsOf[c.Row]
		if !seen {
			rowOrder = append(rowOrder, c.Row)
		}
		if !slices.Contains(cols, c.Column) {
			colsOf[c.Row] = append(cols, c.Column)
		}
	}
	var out []CellRange
	at := map[string]int{} // the range of each set of columns
	for _, row := range rowOrder {
		cols := colsOf[row]
		sig := strings.Join(cols, "\x00")
		if i, ok := at[sig]; ok {
			out[i].Rows.Keys = append(out[i].Rows.Keys, row)
			continue
		}
		at[sig] = len(out)
		out = append(out, CellRange{Rows: keysIn(row), Columns: keysIn(cols...)})
	}
	return out
}

// cellsOfRanges returns the cells of the ranges rs, a key for each, for the
// []CellKey of CellSelection: a range of every row has the rows shown, from rowKey,
// and a range of every column the columns of colKeys. A cell in two ranges is listed
// once.
func cellsOfRanges(rs []CellRange, rowCount int, rowKey func(q int) string, colKeys []string) []CellKey {
	expand := func(s KeySet, n int, key func(int) string) []string {
		if !s.All {
			return s.Keys
		}
		in := keyLookup(s.Keys)
		out := make([]string, 0, n)
		for i := range n {
			if k := key(i); !in(k) {
				out = append(out, k)
			}
		}
		return out
	}
	colKey := func(i int) string { return colKeys[i] }
	size := 0
	parts := make([][2][]string, len(rs))
	for i, r := range rs {
		parts[i] = [2][]string{expand(r.Rows, rowCount, rowKey), expand(r.Columns, len(colKeys), colKey)}
		size += len(parts[i][0]) * len(parts[i][1])
	}
	// A cell that an earlier range has is listed there: the ranges the grid makes do
	// not overlap, so the check rarely skips one, and costs no map of the cells.
	idx := newDgSel(rs)
	earlier := func(i int, row, col string) bool {
		for j := range i {
			if idx.rows[j].has(row) && idx.cols[j].has(col) {
				return true
			}
		}
		return false
	}
	out := make([]CellKey, 0, size)
	for i, p := range parts {
		for _, row := range p[0] {
			for _, col := range p[1] {
				if i > 0 && earlier(i, row, col) {
					continue
				}
				out = append(out, CellKey{Row: row, Column: col})
			}
		}
	}
	return out
}
