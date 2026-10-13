package ui

import "testing"

// A form grid: labels in an Auto column as wide as the widest, fields filling
// an Fr column, and a widget spanning both columns.
func TestGridForm(t *testing.T) {
	name, notes := NewSignal(""), NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Padding(10).Child(Div().Grid(Auto, Fr(1)).Gap(8).Key("grid").
			Child(Text("Name").Key("l1")).Child(TextInput(name).Label("Name").Key("f1")).
			Child(Text("A longer label").Key("l2")).Child(Button("Check").Key("f2")).
			Child(TextArea(notes).Label("Notes").GridSpan(2, 1).Key("f3")))
	})
	rect := func(key string) [4]float32 {
		rc := r.rt.nodeRect(r.nodeByKey(t, key))
		return [4]float32{rc.X, rc.Y, rc.W, rc.H}
	}
	grid, l1, l2 := rect("grid"), rect("l1"), rect("l2")
	f1, f2, f3 := rect("f1"), rect("f2"), rect("f3")
	if l1[2] != l2[2] {
		t.Errorf("labels are %v and %v wide; the column is as wide as the longer", l1[2], l2[2])
	}
	if f1[0] != l2[0]+l2[2]+8 || f1[0]+f1[2] != grid[0]+grid[2] {
		t.Errorf("the field %v does not fill the second column of %v", f1, grid)
	}
	if f2[0] != f1[0] || f2[1] <= f1[1] {
		t.Errorf("the button %v is not in the second row's field column", f2)
	}
	if f3[0] != grid[0] || f3[2] != grid[2] {
		t.Errorf("the spanning text area %v does not cover the grid's width %v", f3, grid)
	}
}

// A width the app sets is kept in a grid cell; a widget's default width gives
// way to the cell.
func TestGridKeepsSetWidths(t *testing.T) {
	a, b := NewSignal(""), NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Child(Div().Grid(Fr(1)).Width(Px(400)).
			Child(TextInput(a).Label("A").Width(Px(100)).Key("set")).
			Child(TextInput(b).Label("B").Key("default")))
	})
	set, def := r.rt.nodeRect(r.nodeByKey(t, "set")), r.rt.nodeRect(r.nodeByKey(t, "default"))
	if set.W != 100 || def.W != 400 {
		t.Errorf("widths %v (set to 100) and %v (default, in a 400 cell)", set.W, def.W)
	}
}
