package ui

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"
)

// The scenarios of FerroUI's comparison of its TreeDataGrid with GPUI's tables
// (github.com/wieslawsoltes/FerroUI, tests/FerroUI.TreeDataGrid.GpuiComparison),
// run against jamb's DataGrid so the three can be set side by side.
//
//	go test ./ui -run xxx -bench DataGridCompare -benchtime 2s
//
// Each benchmark operation is one operation of the scenario: a change (an offset,
// an edit, a sort) and the frame that follows, or two changes and two frames for the
// bursts that undo themselves (insert and remove, move and move back, detach and
// attach), as FerroUI counts them. A frame is Runtime.Frame: render the dirty
// components, lay out and paint into the scene, which is not presented. That is
// GPUI's Window::draw on its test platform and FerroUI's layout+render column.
//
// Text is jamb's real text system (HarfBuzz over the system fonts), so these rows
// match the comparison's Text=Platform rows; its Text=Synthetic rows give the other
// two a fake shaper.
//
// The viewport is 800 x 500 as in the comparison. jamb's compact rows are 26 pixels
// high, not 24, and the grid has a header row, as GPUI's tables do.

type cmpRow struct {
	id    int
	title string
}

func cmpRows(n int, padded, descending bool) []cmpRow {
	rows := make([]cmpRow, n)
	for x := range rows {
		switch {
		case descending:
			id := n - x
			rows[x] = cmpRow{id, "Row " + strconv.Itoa(id)}
		case padded:
			rows[x] = cmpRow{x, "Item " + strconv.Itoa(x) + "xxxxxxxxxxxxxxxxxxxx"[:x%20]}
		default:
			rows[x] = cmpRow{x, "Item " + strconv.Itoa(x)}
		}
	}
	return rows
}

// cmpColumns returns the columns of VirtualizationBenchmarks.CreateGrid: an id
// column 80 pixels wide and n-1 text columns 100 pixels wide.
func cmpColumns(n int) []DataColumn[cmpRow] {
	cols := []DataColumn[cmpRow]{{Header: "ID", Width: 80,
		Text:    func(r cmpRow) string { return strconv.Itoa(r.id) },
		Compare: SortBy(func(r cmpRow) int { return r.id })}}
	for c := 1; c < n; c++ {
		suffix := "-" + strconv.Itoa(c)
		cols = append(cols, DataColumn[cmpRow]{Header: "Column " + strconv.Itoa(c), Width: 100,
			Text: func(r cmpRow) string { return r.title + suffix }})
	}
	return cols
}

// cmpFirstLayoutColumns returns the six columns of FirstLayoutBenchmarks, 124
// pixels wide each, or sharing the width (star) when grow is set.
func cmpFirstLayoutColumns(grow bool) []DataColumn[cmpRow] {
	cols := make([]DataColumn[cmpRow], 6)
	for c := range cols {
		suffix := "-" + strconv.Itoa(c)
		cols[c] = DataColumn[cmpRow]{Header: "Column " + strconv.Itoa(c), Width: 124,
			Text: func(r cmpRow) string { return r.title + suffix }}
		if grow {
			cols[c].Width, cols[c].Grow = 1, 1
		}
	}
	return cols
}

// cmpIDTitle returns the two columns of the sort and tree scenarios.
func cmpIDTitle() []DataColumn[cmpRow] {
	return []DataColumn[cmpRow]{
		{Header: "ID", Width: 80, Text: func(r cmpRow) string { return strconv.Itoa(r.id) },
			Compare: SortBy(func(r cmpRow) int { return r.id })},
		{Header: "Title", Width: 100, Text: func(r cmpRow) string { return r.title }},
	}
}

type cmpGrid struct {
	rt       *Runtime
	rows     *Signal[[]cmpRow]
	attached *Signal[bool]
	sort     *Signal[SortState]
	expanded *Signal[[]string]
	now      time.Time
}

func cmpKey(r cmpRow) string { return strconv.Itoa(r.id) }

// newCmpGrid creates the grid in an 800 x 500 window and brings it to its first frame.
func newCmpGrid(rows []cmpRow, cols []DataColumn[cmpRow], children func(cmpRow) []cmpRow) *cmpGrid {
	g := &cmpGrid{
		rows:     NewSignal(rows),
		attached: NewSignal(true),
		sort:     NewSignal(SortState{}),
		expanded: NewSignal([]string(nil)),
		now:      time.Unix(1_000_000, 0),
	}
	g.rt = NewRuntime(func(ctx *Context) Element {
		root := Div().Height(Full)
		if !g.attached.Get() {
			return root
		}
		grid := DataGrid(g.rows.Get(), cmpKey, cols...).Density(DataGridCompact).
			Sort(g.sort).Grow(1)
		if children != nil {
			grid.Children(children).Expanded(g.expanded)
		}
		return root.Child(grid)
	}, DarkTheme(), benchText)
	g.rt.SetViewport(800, 500, 1)
	g.frame()
	return g
}

func (g *cmpGrid) frame() {
	g.now = g.now.Add(16 * time.Millisecond)
	g.rt.Frame(g.now)
}

func (g *cmpGrid) body() *node {
	ns := partNodes(g.rt, "DataGrid.Grid")
	if len(ns) != 1 {
		panic(fmt.Sprintf("want one grid body, have %d", len(ns)))
	}
	return ns[0]
}

// setOffset scrolls the rows to x, y at once, as a precise wheel does, without a frame.
func (g *cmpGrid) setOffset(body *node, x, y float32) {
	st := body.st
	st.targetX, st.targetY, st.scrollX, st.scrollY = x, y, x, y
	g.rt.needPaint = true
	g.rt.scrolled = true
	g.rt.requestFrame()
}

// cmpEdits edits the rows of a grid as jamb asks: each change is a new slice, since
// rows are compared by slice identity. It keeps two buffers and writes each change
// into the one the grid is not showing, so the benchmark measures the grid, not
// the allocation of a copy of every row.
type cmpEdits struct {
	g     *cmpGrid
	other []cmpRow
}

func newCmpEdits(g *cmpGrid) *cmpEdits {
	return &cmpEdits{g: g, other: make([]cmpRow, 0, len(g.rows.Peek())+1)}
}

// set copies the rows into the other buffer, changes them with fn and shows them.
func (e *cmpEdits) set(fn func([]cmpRow) []cmpRow) {
	cur := e.g.rows.Peek()
	next := fn(append(e.other[:0], cur...))
	e.other = cur
	e.g.rows.Set(next)
}

func BenchmarkDataGridCompare(b *testing.B) {
	b.Run("create_and_layout_grid/Rows=1000,Columns=12", func(b *testing.B) {
		rows, cols := cmpRows(1000, false, false), cmpColumns(12)
		b.ReportAllocs()
		for b.Loop() {
			newCmpGrid(rows, cols, nil)
		}
	})
	for _, l := range []struct {
		name string
		grow bool
	}{{"Fixed", false}, {"Star", true}} {
		b.Run("create_and_first_layout/Rows=1000,Columns=6,Layout="+l.name, func(b *testing.B) {
			rows, cols := cmpRows(1000, true, false), cmpFirstLayoutColumns(l.grow)
			b.ReportAllocs()
			for b.Loop() {
				newCmpGrid(rows, cols, nil)
			}
		})
	}

	// The scrolls: op(i) is the offset of the i-th operation, i counting from 0.
	scrolls := []struct {
		name   string
		rows   []int
		cols   int
		ops    int
		start  [2]float32
		offset func(i int) (x, y float32)
	}{
		{"vertical_small_scrolls", []int{10_000, 100_000}, 12, 10_000, [2]float32{},
			func(i int) (float32, float32) { return 0, float32(i + 1) }},
		{"vertical_row_scrolls", []int{10_000, 100_000}, 12, 10_000, [2]float32{},
			func(i int) (float32, float32) { return 0, float32((i + 1) * 24) }},
		{"vertical_jitter_scrolls", []int{10_000, 100_000}, 12, 10_000, [2]float32{},
			func(i int) (float32, float32) { return 0, float32(100 * (1 - i&1)) }},
		{"forced_stationary_vertical_layouts", []int{10_000}, 12, 25_000, [2]float32{},
			func(int) (float32, float32) { return 0, 0 }},
		{"horizontal_small_scrolls", []int{1000}, 200, 6_000, [2]float32{},
			func(i int) (float32, float32) { return float32(i + 1), 0 }},
		{"horizontal_column_scrolls", []int{1000}, 200, 1_000, [2]float32{},
			func(i int) (float32, float32) { return float32(100 + 100*(i&1)), 0 }},
		{"horizontal_far_column_scrolls", []int{1000}, 1000, 1_000, [2]float32{90_000, 0},
			func(i int) (float32, float32) { return float32(90_100 - 100*(i&1)), 0 }},
	}
	for _, s := range scrolls {
		for _, n := range s.rows {
			b.Run(fmt.Sprintf("%s/Rows=%d,Columns=%d", s.name, n, s.cols), func(b *testing.B) {
				g := newCmpGrid(cmpRows(n, false, false), cmpColumns(s.cols), nil)
				body := g.body()
				if s.start != [2]float32{} {
					g.setOffset(body, s.start[0], s.start[1])
					g.frame()
				}
				i := 0
				b.ReportAllocs()
				for b.Loop() {
					x, y := s.offset(i % s.ops)
					g.setOffset(body, x, y)
					if s.name == "forced_stationary_vertical_layouts" || s.name == "vertical_row_scrolls" {
						g.rt.needLayout = true // the invalidated measure of the rows
					}
					g.frame()
					i++
				}
			})
		}
	}

	b.Run("sort_thousand_rows_with_frame/Rows=1000,Columns=2", func(b *testing.B) {
		g := newCmpGrid(cmpRows(1000, false, true), cmpIDTitle(), nil)
		desc := false
		b.ReportAllocs()
		for b.Loop() {
			desc = !desc
			g.sort.Set(SortState{Key: "ID", Desc: desc})
			g.frame()
		}
	})

	for _, n := range []int{10_000, 100_000} {
		b.Run(fmt.Sprintf("collection_insert_remove_burst/Rows=%d,Columns=12", n), func(b *testing.B) {
			g := newCmpGrid(cmpRows(n, false, false), cmpColumns(12), nil)
			i := 0
			e := newCmpEdits(g)
			b.ReportAllocs()
			for b.Loop() {
				e.set(func(rs []cmpRow) []cmpRow {
					// -i in FerroUI; jamb keys rows by id, so the ids stay unique.
					return slices.Insert(rs, 2, cmpRow{-i - 1, "Inserted " + strconv.Itoa(i)})
				})
				g.frame()
				e.set(func(rs []cmpRow) []cmpRow { return slices.Delete(rs, 2, 3) })
				g.frame()
				i++
			}
		})
	}
	for _, n := range []int{10_000, 100_000} {
		b.Run(fmt.Sprintf("collection_move_burst/Rows=%d,Columns=12", n), func(b *testing.B) {
			g := newCmpGrid(cmpRows(n, false, false), cmpColumns(12), nil)
			e := newCmpEdits(g)
			move := func(from, to int) {
				e.set(func(rs []cmpRow) []cmpRow {
					r := rs[from]
					return slices.Insert(slices.Delete(rs, from, from+1), to, r)
				})
			}
			b.ReportAllocs()
			for b.Loop() {
				move(2, 12)
				g.frame()
				move(12, 2)
				g.frame()
			}
		})
	}

	b.Run("detach_reattach/Rows=10000,Columns=50", func(b *testing.B) {
		g := newCmpGrid(cmpRows(10_000, false, false), cmpColumns(50), nil)
		b.ReportAllocs()
		for b.Loop() {
			g.attached.Set(false)
			g.frame()
			g.attached.Set(true)
			g.frame()
		}
	})

	b.Run("expand_collapse_thousand_rows_with_frames/Rows=1000,Columns=2", func(b *testing.B) {
		kids := cmpRows(1001, false, false)[1:]
		root := cmpRow{0, "Root"}
		g := newCmpGrid([]cmpRow{root}, cmpIDTitle(), func(r cmpRow) []cmpRow {
			if r.id == 0 {
				return kids
			}
			return nil
		})
		open := []string{"0"}
		b.ReportAllocs()
		for b.Loop() {
			g.expanded.Set(open)
			g.frame()
			g.expanded.Set(nil)
			g.frame()
		}
	})
}
