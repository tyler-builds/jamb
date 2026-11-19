package ui

import (
	"fmt"
	"testing"
)

// TestKeySetAlgebra checks minus and and of key sets, and the minus of ranges, against
// the sets they stand for, over every kind of set: some keys, and every key but some.
func TestKeySetAlgebra(t *testing.T) {
	universe := []string{"a", "b", "c", "d", "e"}
	sets := []KeySet{
		{}, {Keys: []string{"a"}}, {Keys: []string{"a", "c", "e"}}, {Keys: []string{"b", "c"}},
		{All: true}, {All: true, Keys: []string{"a"}}, {All: true, Keys: []string{"b", "d"}},
	}
	members := func(s KeySet) string {
		out := ""
		for _, k := range universe {
			if s.Has(k) {
				out += k
			}
		}
		return out
	}
	for _, x := range sets {
		for _, y := range sets {
			var wantMinus, wantAnd string
			for _, k := range universe {
				if x.Has(k) && !y.Has(k) {
					wantMinus += k
				}
				if x.Has(k) && y.Has(k) {
					wantAnd += k
				}
			}
			if got := members(x.minus(y)); got != wantMinus {
				t.Errorf("%+v minus %+v = %q, want %q", x, y, got, wantMinus)
			}
			if got := members(x.and(y)); got != wantAnd {
				t.Errorf("%+v and %+v = %q, want %q", x, y, got, wantAnd)
			}
		}
	}
	cells := func(rs []CellRange) string {
		out := ""
		for _, r := range universe {
			for _, c := range universe {
				if rangesHave(rs, CellKey{Row: r, Column: c}) {
					out += r + c + " "
				}
			}
		}
		return out
	}
	for _, a := range sets[1:] {
		for _, b := range sets[1:] {
			r := CellRange{Rows: a, Columns: b}
			o := CellRange{Rows: b, Columns: a}
			want := ""
			for _, rk := range universe {
				for _, ck := range universe {
					c := CellKey{Row: rk, Column: ck}
					if r.Has(c) && !o.Has(c) {
						want += rk + ck + " "
					}
				}
			}
			if got := cells(r.minus(o)); got != want {
				t.Errorf("%+v minus %+v = %q, want %q", r, o, got, want)
			}
		}
	}
}

// TestCellsOfRangesListsEachCellOnce checks that the cells of overlapping ranges are
// listed once, and that a range of every row lists the rows shown.
func TestCellsOfRangesListsEachCellOnce(t *testing.T) {
	rows := []string{"r0", "r1", "r2"}
	rs := []CellRange{
		{Rows: KeySet{All: true}, Columns: keysIn("A")},
		{Rows: keysIn("r1"), Columns: KeySet{All: true}},
	}
	got := fmt.Sprint(cellsOfRanges(rs, len(rows), func(q int) string { return rows[q] }, []string{"A", "B"}))
	want := "[{r0 A} {r1 A} {r2 A} {r1 B}]"
	if got != want {
		t.Errorf("cells %s, want %s", got, want)
	}
	if got := rangesOfCells([]CellKey{{"r0", "A"}, {"r0", "B"}, {"r1", "A"}, {"r1", "B"}, {"r2", "A"}}); len(got) != 2 {
		t.Errorf("a rectangle and a cell made %d ranges, want 2: %+v", len(got), got)
	}
}
