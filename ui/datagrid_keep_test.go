package ui

import (
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// dgKeepRig returns a rig with a grid of the rows in rows, which a new slice
// replaces, with multiple selection in sel and the activated rows in acts.
func dgKeepRig(rows *Signal[[]dgItem], sel *Signal[[]string], acts *[]string, cols func() []DataColumn[dgItem]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, cols()...).Label("Runs").
			Selection(SelectionMultiple, sel).OnActivate(func(x dgItem) { *acts = append(*acts, x.ID) })
	})
}

// dgRowNodes returns the elements of the built rows by key. A kept row keeps its
// element; a row built again has a new one, though its node may be the same memory
// (see newNode).
func dgRowNodes(r *accessRig) map[string]*DivElement {
	out := map[string]*DivElement{}
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		out[n.div.key] = n.div
	}
	return out
}

// TestDataGridKeepsRowsThatDidNotChange checks that a grid that renders again for a
// new slice of rows keeps the nodes of the rows that did not change, builds the
// inserted one, and gives the kept rows their new places for assistive technology
// and for clicks.
func TestDataGridKeepsRowsThatDidNotChange(t *testing.T) {
	items := dgItems(30)
	rows := NewSignal(items)
	sel := NewSignal([]string(nil))
	var acts []string
	r := dgKeepRig(rows, sel, &acts, dgColumns)
	dgSettle(r)
	// A second build of the same rows makes them rows that can be kept.
	rows.Set(slices.Clone(items))
	dgSettle(r)
	before := dgRowNodes(r)

	rows.Set(slices.Insert(slices.Clone(items), 1, dgItem{ID: "new", Name: "New"}))
	r.frame()
	if b := r.rt.Stats().NodesBuilt; b > 40 {
		t.Errorf("an insert built %d nodes; want the new row, the wrappers and the header, not every row", b)
	}
	after := dgRowNodes(r)
	for _, k := range []string{"i0", "i1", "i5"} {
		if before[k] == nil || after[k] != before[k] {
			t.Errorf("row %s was built again; want its node kept", k)
		}
	}
	// i5 moved from position 6 to 7.
	if a := after["i5"].acc; a.gridRow != 7 || a.setPos != 7 || a.setSize != 31 {
		t.Errorf("kept row i5 reports row %d, %d of %d; want row 7, 7 of 31", a.gridRow, a.setPos, a.setSize)
	}
	if c := dgCell(r, 6, 0); c == nil || c.div.acc.name != items[5].Name {
		t.Errorf("the cell at row 7 is %v; want %q", c, items[5].Name)
	}
	row := accesstest.Find(t, r.tree(), access.RoleListItem, items[5].Name+", "+strconv.Itoa(items[5].N))
	if row.PosInSet != 7 || row.SetSize != 31 {
		t.Errorf("the access tree has i5 at %d of %d; want 7 of 31", row.PosInSet, row.SetSize)
	}

	// A double-click on a kept row, and a click on another, act on each at its new
	// place. The first press of the double-click selects i6, which builds that row
	// again only at the next frame, so the double-click lands on the kept row.
	dgDoubleClick(r, dgCell(r, 7, 0))
	r.frame()
	if !slices.Equal(acts, []string{"i6"}) {
		t.Errorf("a double-click on kept row i6 activated %v", acts)
	}
	dgClick(r, dgCell(r, 6, 0), 0)
	r.frame()
	if !slices.Equal(sel.Peek(), []string{"i5"}) {
		t.Errorf("a click on kept row i5 selected %v", sel.Peek())
	}
	// Assistive technology selects a kept row at its new place: i9, at row 11 since
	// the insert, was built before it and kept.
	k9 := accesstest.Find(t, r.tree(), access.RoleListItem, items[9].Name+", "+strconv.Itoa(items[9].N))
	r.rt.runAccess(k9.ID, access.Request{Action: access.ActionSelect})
	r.frame()
	if !slices.Equal(sel.Peek(), []string{"i9"}) {
		t.Errorf("selecting kept row i9 for assistive technology selected %v", sel.Peek())
	}
	k12 := accesstest.Find(t, r.tree(), access.RoleListItem, items[12].Name+", "+strconv.Itoa(items[12].N))
	r.rt.runAccess(k12.ID, access.Request{Action: access.ActionAddToSelection})
	r.frame()
	if got := slices.Sorted(slices.Values(sel.Peek())); !slices.Equal(got, []string{"i12", "i9"}) {
		t.Errorf("adding kept row i12 to the selection for assistive technology made it %v", got)
	}
}

// TestDataGridBuildsRowsThatChanged checks the rows a grid builds again: a row whose
// text changed under the same key, a row whose selection changed, the rows of
// columns whose text function changed, and every row when the width of a column
// changes. The others are kept.
func TestDataGridBuildsRowsThatChanged(t *testing.T) {
	items := dgItems(20)
	rows := NewSignal(items)
	sel := NewSignal([]string(nil))
	suffix := NewSignal("")
	widths := NewSignal(map[string]float32(nil))
	var acts []string
	cols := func() []DataColumn[dgItem] {
		c := dgColumns()
		sfx := suffix.Get()
		c[1].Text = func(x dgItem) string { return strconv.Itoa(x.N) + sfx }
		return c
	}
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, cols()...).Label("Runs").
			Selection(SelectionSingle, sel).OnActivate(func(x dgItem) { acts = append(acts, x.ID) }).
			ColumnWidths(widths)
	})
	dgSettle(r)
	rows.Set(slices.Clone(items))
	dgSettle(r)

	// The text of i3 changes under the same key.
	before := dgRowNodes(r)
	next := slices.Clone(items)
	next[3].Name = "Renamed"
	rows.Set(next)
	r.frame()
	after := dgRowNodes(r)
	if after["i3"] == before["i3"] {
		t.Error("row i3, whose text changed, was kept")
	}
	if after["i2"] != before["i2"] || after["i4"] != before["i4"] {
		t.Error("the rows next to i3 were built again")
	}
	if got := dgCellText(r, 3, 0); got != "Renamed" {
		t.Errorf("the cell of i3 shows %q, want Renamed", got)
	}

	// A selection builds the selected row again, and keeps the others.
	before = dgRowNodes(r)
	sel.Set([]string{"i2"})
	r.frame()
	after = dgRowNodes(r)
	if after["i2"] == before["i2"] || !after["i2"].acc.selected {
		t.Error("row i2 was kept, or is not selected, after it was selected")
	}
	if after["i4"] != before["i4"] {
		t.Error("row i4 was built again when i2 was selected")
	}

	// A new text function for a column, from a render of the parent, shows in every row.
	suffix.Set(" kg")
	r.frame()
	for q := range 5 {
		if got, want := dgCellText(r, q, 1), strconv.Itoa(items[q].N)+" kg"; got != want {
			t.Errorf("row %d's N cell shows %q after the column's text changed, want %q", q, got, want)
		}
	}

	// A new width of a column builds every row.
	before = dgRowNodes(r)
	widths.Set(map[string]float32{"N": 90})
	r.frame()
	after = dgRowNodes(r)
	for k, n := range after {
		if before[k] == n {
			t.Errorf("row %s was kept after a column's width changed", k)
		}
	}
}

// TestDataGridKeptTreeRowsOpenAndClose checks that the chevron of a kept tree row
// opens and closes it, and that the row reports the state it is in.
func TestDataGridKeptTreeRowsOpenAndClose(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	dgSettle(r)
	for range 2 {
		chev := (*node)(nil)
		for _, n := range partNodes(r.rt, "DataGrid.TreeChevron") {
			if n.parent != nil && n.parent.parent != nil && n.parent.parent.div.key == "g2" {
				chev = n
			}
		}
		if chev == nil {
			t.Fatal("no chevron on row g2")
		}
		dgClick(r, chev, 0)
		dgSettle(r)
	}
	if want := []string{"+g2", "-g2"}; !slices.Equal(ev, want) {
		t.Errorf("two clicks on g2's chevron gave %v, want %v", ev, want)
	}
	if len(exp.Peek()) != 0 || dgRowNode(t, r, "g2").div.acc.expanded {
		t.Errorf("after two clicks g2 is open: %v", exp.Peek())
	}
}

// TestDataGridChevronTogglesTwiceInOneFrame checks that two clicks on a tree row's
// chevron before the next frame open and then close it: each click reads the open
// rows as the last one left them, not as the row was built.
func TestDataGridChevronTogglesTwiceInOneFrame(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	dgSettle(r)
	var chev *node
	for _, n := range partNodes(r.rt, "DataGrid.TreeChevron") {
		if n.parent != nil && n.parent.parent != nil && n.parent.parent.div.key == "g2" {
			chev = n
		}
	}
	if chev == nil {
		t.Fatal("no chevron on row g2")
	}
	rc := r.rt.nodeRect(chev)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	for range 2 {
		r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	}
	dgSettle(r)
	if want := []string{"+g2", "-g2"}; !slices.Equal(ev, want) {
		t.Errorf("two clicks in one frame on g2's chevron gave %v, want %v", ev, want)
	}
}

// TestDataGridKeepsNoRowsWithCellContent checks that a grid whose column has a Cell
// builds its rows each time, since the content the Cell returns cannot be compared.
func TestDataGridKeepsNoRowsWithCellContent(t *testing.T) {
	items := dgItems(10)
	rows := NewSignal(items)
	var acts []string
	cols := func() []DataColumn[dgItem] {
		c := dgColumns()
		c[1].Cell = func(x dgItem) Element { return Text(strconv.Itoa(x.N)) }
		return c
	}
	r := dgKeepRig(rows, NewSignal([]string(nil)), &acts, cols)
	dgSettle(r)
	rows.Set(slices.Clone(items))
	dgSettle(r)
	before := dgRowNodes(r)
	rows.Set(slices.Clone(items))
	r.frame()
	for k, d := range dgRowNodes(r) {
		if before[k] == d {
			t.Errorf("row %s with Cell content was kept", k)
		}
	}
	if len(partNodes(r.rt, "DataGrid.Row")) == 0 {
		t.Fatal("no rows")
	}
}

// TestSameStyleCoversStyle checks that every field of Style is a value sameStyle can
// compare by its memory, or one of the slices it compares by their elements, and
// that it tells apart styles that differ in any field.
func TestSameStyleCoversStyle(t *testing.T) {
	slicesOK := map[string]bool{"GridColumns": true, "GridRows": true, "Shadows": true}
	// prov is the inspector's record of the layers, nil without an inspector. It is
	// compared by identity: a new record builds the row again, which is right.
	slicesOK["prov"] = true
	var check func(path string, ty reflect.Type)
	check = func(path string, ty reflect.Type) {
		switch ty.Kind() {
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.String:
		case reflect.Array:
			check(path+"[]", ty.Elem())
		case reflect.Struct:
			for i := range ty.NumField() {
				f := ty.Field(i)
				if path == "Style" && (f.Type.Kind() == reflect.Slice || f.Type.Kind() == reflect.Map) && slicesOK[f.Name] {
					continue
				}
				check(path+"."+f.Name, f.Type)
			}
		default:
			t.Errorf("%s is a %s, which sameStyle cannot compare by its memory", path, ty.Kind())
		}
	}
	check("Style", reflect.TypeFor[Style]())

	// A change of any field, top-level or nested, makes the styles different.
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		for i := range v.NumField() {
			f := v.Field(i)
			if !f.CanSet() {
				continue
			}
			switch f.Kind() {
			case reflect.Struct:
				walk(f)
				continue
			case reflect.Bool:
				f.SetBool(true)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				f.SetInt(1)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				f.SetUint(1)
			case reflect.Float32, reflect.Float64:
				f.SetFloat(1)
			case reflect.String:
				f.SetString("x")
			case reflect.Slice:
				f.Set(reflect.MakeSlice(f.Type(), 1, 1))
			default:
				continue // an array: covered by the check above
			}
		}
	}
	zero := Style{}
	typ := reflect.TypeFor[Style]()
	for i := range typ.NumField() {
		var s Style
		f := reflect.ValueOf(&s).Elem().Field(i)
		if !f.CanSet() {
			continue
		}
		switch f.Kind() {
		case reflect.Struct:
			walk(f)
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(1)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			f.SetUint(1)
		case reflect.Float32, reflect.Float64:
			f.SetFloat(1)
		case reflect.String:
			f.SetString("x")
		case reflect.Slice:
			f.Set(reflect.MakeSlice(f.Type(), 1, 1))
		default:
			continue
		}
		if sameStyle(&s, &zero) {
			t.Errorf("sameStyle takes a style whose %s is set as the same as the zero style", typ.Field(i).Name)
		}
	}
	a := Style{Shadows: []Shadow{{Blur: 4}}, GridColumns: []Length{Px(10)}}
	b := Style{Shadows: []Shadow{{Blur: 4}}, GridColumns: []Length{Px(10)}}
	if !sameStyle(&a, &b) {
		t.Error("sameStyle takes equal slices in different memory as different")
	}
}

// TestSameDataGridStyleCoversItsParts checks that styles lists every Style of
// DataGridStyle, so that a part added later is compared.
func TestSameDataGridStyleCoversItsParts(t *testing.T) {
	ty := reflect.TypeFor[DataGridStyle]()
	n := 0
	for i := range ty.NumField() {
		if ty.Field(i).Type == reflect.TypeFor[Style]() {
			n++
		} else if ty.Field(i).Name != "Editor" {
			t.Errorf("DataGridStyle.%s is a %s: compare it in sameDataGridStyle", ty.Field(i).Name, ty.Field(i).Type)
		}
	}
	var s DataGridStyle
	if got := len(s.styles()); got != n {
		t.Fatalf("styles lists %d parts, DataGridStyle has %d", got, n)
	}
	seen := map[*Style]bool{}
	for _, p := range s.styles() {
		if seen[p] {
			t.Error("styles lists a part twice")
		}
		seen[p] = true
	}
	for i := range ty.NumField() {
		if ty.Field(i).Type != reflect.TypeFor[Style]() {
			continue
		}
		var x DataGridStyle
		reflect.ValueOf(&x).Elem().Field(i).Set(reflect.ValueOf(Style{Opacity: 0.5}))
		if sameDataGridStyle(&x, &DataGridStyle{}) {
			t.Errorf("sameDataGridStyle misses a change of %s", ty.Field(i).Name)
		}
	}
}
