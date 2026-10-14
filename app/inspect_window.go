package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/ui"
)

// root is the inspector's window: the element tree on the left, and on the right
// the selected element's layout, accessibility node and style, its live edit, and
// the frame timings of the window it inspects.
func (ins *inspector) root(ctx *ui.Context) ui.Element {
	ins.tick.Get() // read the target again when the signal changes
	sel := ins.tg.Selected()
	return ui.Div().Row().Gap(12).Padding(12).Width(ui.Full).Height(ui.Full).Children(
		ins.treeColumn(),
		ins.detailColumn(ctx, ctx.Theme(), sel),
	)
}

// treeColumn is the pick button and the element tree. The window's root is not
// shown: its children are the top level of the tree.
func (ins *inspector) treeColumn() ui.Element {
	tg := ins.tg
	pick := "Pick an element"
	if tg.Picking() {
		pick = "Stop picking"
	}
	var nodes []ui.TreeNode
	if root, ok := tg.Element(tg.Root()); ok {
		names := tg.AccessNames()
		for _, c := range root.Children {
			nodes = append(nodes, ins.treeNode(c, names))
		}
	}
	return ui.Div().Column().Gap(8).Width(ui.Px(380)).Height(ui.Full).Children(
		ui.Text("Elements"),
		ui.Button(pick).Key("inspector.pick").OnClick(func() {
			tg.Pick(!tg.Picking())
			ins.bump()
		}),
		ui.Tree(nodes, ins.sel).Key("inspector.tree").Grow(1).OnChange(func(key string) {
			tg.Select(idOf(key))
			ins.bump()
		}),
	)
}

// treeNode returns the tree row of the element id, with the role and name that
// names holds for it. Its children are read when the row is opened, so a large
// tree costs only what is open.
func (ins *inspector) treeNode(id uint64, names map[uint64]string) ui.TreeNode {
	e, ok := ins.tg.Element(id)
	label := elementLabel(e)
	if n := names[id]; n != "" {
		label += " " + n
	}
	node := ui.TreeNode{Key: keyOf(id), Label: label}
	if ok && len(e.Children) > 0 {
		node.Children = func() []ui.TreeNode {
			out := make([]ui.TreeNode, 0, len(e.Children))
			for _, c := range e.Children {
				out = append(out, ins.treeNode(c, names))
			}
			return out
		}
	}
	return node
}

// elementLabel is the text of an element's tree row: its kind, then its part,
// key, text, component and visibility, when it has them.
func elementLabel(e inspect.Element) string {
	s := e.Kind
	if e.Part != "" {
		s += " " + e.Part
	}
	if e.Key != "" {
		s += " key=" + e.Key
	}
	if e.Text != "" {
		s += " " + strconv.Quote(e.Text)
	}
	if e.Boundary {
		s += " [component " + e.Component + "]"
	}
	if e.Hidden {
		s += " (hidden)"
	}
	return s
}

// detailColumn is what the inspector shows of the selected element.
func (ins *inspector) detailColumn(ctx *ui.Context, th *ui.Theme, sel uint64) ui.Element {
	col := ui.Div().Column().Gap(8).Grow(1).Height(ui.Full)
	e, ok := ins.tg.Element(sel)
	if sel == 0 || !ok {
		col.Child(ui.Text("Select an element in the tree, or press Pick an element and click one in the window."))
	} else {
		col.Child(ui.Textf("Element: %s", elementLabel(e)))
		col.Child(ins.layoutRows(e))
		col.Child(ins.accessRows(sel))
		col.Child(ins.styleRows(sel))
		col.Child(ins.editRow(ctx, sel))
	}
	col.Child(ins.timingRows(th))
	return col
}

// layoutRows are the box, insets, content size and overflow of the element.
func (ins *inspector) layoutRows(e inspect.Element) ui.Element {
	return ui.Div().Column().Gap(2).Children(
		ui.Textf("Box: x %.0f, y %.0f, width %.0f, height %.0f", e.Box.X, e.Box.Y, e.Box.W, e.Box.H),
		ui.Textf("Padding: %s   Margin: %s   Border: %s", edgesText(e.Padding), edgesText(e.Margin), edgesText(e.Border)),
		ui.Textf("Content: %.0f x %.0f   Overflow: x %s, y %s", e.ContentW, e.ContentH, e.OverflowX, e.OverflowY),
	)
}

// accessRows are the accessibility node of the element and its warnings, as
// JAMB_A11Y_WARN reports them.
func (ins *inspector) accessRows(id uint64) ui.Element {
	a := ins.tg.Access(id)
	col := ui.Div().Column().Gap(2)
	if !a.Exposed {
		return col.Child(ui.Text("Accessibility: not in the accessibility tree"))
	}
	col.Child(ui.Textf("Accessibility: role %s, name %q", a.Role, a.Name))
	if a.Description != "" {
		col.Child(ui.Textf("Description: %s", a.Description))
	}
	if a.Value != "" {
		col.Child(ui.Textf("Value: %s", a.Value))
	}
	if len(a.States) > 0 {
		col.Child(ui.Textf("States: %v", a.States))
	}
	if len(a.Actions) > 0 {
		col.Child(ui.Textf("Actions: %v", a.Actions))
	}
	for _, w := range a.Warnings {
		col.Child(ui.Textf("Warning: %s", w))
	}
	return col
}

// styleRows lists the set fields of the resolved style. Each field shows its
// winning value and the layer it came from. The values of the layers that it
// overrides follow, struck through and marked overridden, lowest layer last. A
// layer that sets the same value as the layer below it is marked so.
func (ins *inspector) styleRows(id uint64) ui.Element {
	fields := ins.tg.Fields(id)
	col := ui.Div().Column().Gap(6)
	for _, f := range fields {
		row := ui.Div().Column().Child(ui.Textf("%s = %s   (%s)", f.Name, f.Value, f.Source))
		if len(f.Layers) > 0 && f.Layers[len(f.Layers)-1].Same {
			row.Child(ui.Text("   same value as the layer below"))
		}
		for i := len(f.Layers) - 2; i >= 0; i-- {
			l := f.Layers[i]
			text := fmt.Sprintf("   overridden: %s = %s   (%s)", f.Name, l.Value, l.Source)
			if l.Same {
				text += ", same value as the layer below"
			}
			row.Child(ui.Text(text))
		}
		col.Child(row)
	}
	return ui.Div().Column().Gap(4).Children(
		ui.Text("Style, set fields only: each value, its layer, and the values it overrides"),
		ui.Div().Height(ui.Px(260)).ScrollY().Child(col))
}

// editRow sets one style field of the element, copies the live edits as Go, and
// says what the last action did.
func (ins *inspector) editRow(ctx *ui.Context, sel uint64) ui.Element {
	tg := ins.tg
	var options []ui.Option[string]
	for _, name := range tg.EditableFields() {
		options = append(options, ui.Option[string]{Value: name, Label: name})
	}
	row := ui.Div().Row().Gap(8).Children(
		ui.Select(ins.field, options...).Label("Style field").Width(ui.Px(180)),
		ui.TextInput(ins.value).Label("Value").Width(ui.Px(140)),
		ui.Button("Apply").Key("inspector.apply").Disabled(sel == 0).OnClick(func() {
			if err := tg.Edit(sel, ins.field.Peek(), ins.value.Peek()); err != nil {
				ins.status.Set(err.Error())
			} else {
				ins.status.Set("Applied to the window")
			}
			ins.bump()
		}),
		ui.Button("Clear edits").Key("inspector.clear").Disabled(!tg.Edited(sel)).OnClick(func() {
			tg.ClearEdit(sel)
			ins.status.Set("Edits cleared")
			ins.bump()
		}),
		ui.Button("Copy as Go").Key("inspector.copy").Disabled(!tg.Edited(sel)).OnClick(func() {
			ctx.SetClipboard(tg.GoLiteral(sel))
			ins.status.Set("Copied the edits as a ui.Style literal")
		}),
	)
	return ui.Div().Column().Gap(4).Children(row, ui.Text(ins.status.Get()))
}

// timingRows are the frame timings of the window: the last frame, the average,
// and a sparkline of the CPU time per frame.
func (ins *inspector) timingRows(th *ui.Theme) ui.Element {
	last := ins.perf.Last()
	samples := ins.perf.Samples()
	var sum time.Duration
	for _, s := range samples {
		sum += s.Total()
	}
	var avg time.Duration
	if len(samples) > 0 {
		avg = sum / time.Duration(len(samples))
	}
	spark := ins.sparkline(th, samples)
	return ui.Div().Column().Gap(4).Children(
		ui.Text("Frames of the window"),
		ui.Textf("Last frame: build %v, layout %v, paint %v", round(last.Build), round(last.Layout), round(last.Paint)),
		ui.Textf("Nodes %d, rendered components %d, quads %d, glyphs %d", last.Nodes, last.Rendered, last.Quads, last.Glyphs),
		ui.Textf("Average over %d frames: %v, peak %v", len(samples), round(avg), round(ins.perf.MaxTotal())),
		spark,
	)
}

// sparkline draws the CPU time of each kept frame, scaled to the largest.
func (ins *inspector) sparkline(th *ui.Theme, samples []inspect.Sample) ui.Element {
	peak := ins.perf.MaxTotal()
	return ui.Canvas(func(p *ui.Painter, w, h float32) {
		if len(samples) < 2 || peak <= 0 {
			return
		}
		path := ui.NewPath()
		for i, s := range samples {
			x := w * float32(i) / float32(len(samples)-1)
			y := h - h*float32(s.Total())/float32(peak)
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		p.Stroke(path, ui.StrokeStyle{Width: 1.5, Color: th.Accent})
	}).Width(ui.Full).Height(ui.Px(60)).Label("CPU time per frame of the window, as a line")
}

// round rounds a duration to the microsecond, for display.
func round(d time.Duration) time.Duration { return d.Round(time.Microsecond) }

// edgesText is the top, right, bottom and left insets of e, as text.
func edgesText(e inspect.Edges) string {
	return fmt.Sprintf("%g %g %g %g", e.Top, e.Right, e.Bottom, e.Left)
}
