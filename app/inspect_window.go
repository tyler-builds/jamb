package app

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/ui"
)

// The inspector's window: a toolbar with the pick toggle, the window it inspects
// and a filter for the tree; under it a split with the element tree on the left
// and the details of the selected element on the right. The details scroll as
// one pane, with the frame timings of the window pinned at the bottom.

// The look of the inspector's window: its paddings, the size of its small text,
// and the frame budget the sparkline draws, one frame at 60 Hz.
const (
	inspPad    = 16                       // the padding of the details
	inspSmall  = 12                       // the size of headings, tags and notes
	inspBudget = 16700 * time.Microsecond // the frame budget at 60 Hz
)

// inspectPick turns pick mode on or off; Ctrl+Shift+C runs it in the inspector.
var inspectPick = ui.NewAction("inspector.pick", "Pick an element")

// The box model tints of the layout diagram, the ones the inspected window
// draws over the selected element (see paintInspected in ui/inspect.go).
var (
	inspMarginTint  = ui.Hex("#f6b26b59")
	inspBorderTint  = ui.Hex("#ffe59980")
	inspPaddingTint = ui.Hex("#93c47d66")
	inspContentTint = ui.Hex("#6fa8dc59")
)

// root is the inspector's window.
func (ins *inspector) root(ctx *ui.Context) ui.Element {
	ins.tick.Get() // read the target again when the signal changes
	th := ctx.Theme()
	sel := ins.tg.Selected()
	return ui.Div().Column().Width(ui.Full).Height(ui.Full).Background(th.Panel).
		OnAction(inspectPick, ins.togglePick).Children(
		ins.toolbar(th),
		ui.Split(
			ui.Pane(ins.treePane(th)).Size(ui.Percent(40)).Min(ui.Px(220)).Label("Elements"),
			ui.Pane(ins.detailsPane(ctx, th, sel)).Min(ui.Px(320)).Label("Details"),
		).Key("inspector.split").Grow(1).MinHeight(ui.Px(0)),
	)
}

// togglePick turns pick mode on or off. The target calls changed, which sets
// the pick toggle.
func (ins *inspector) togglePick() { ins.tg.Pick(!ins.tg.Picking()) }

// toolbar is the row at the top: the pick toggle, the title of the window that
// is inspected, and the filter of the tree.
func (ins *inspector) toolbar(th *ui.Theme) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(12).PaddingX(12).PaddingY(8).Shrink(0).
		BorderEdges(ui.Edges{Bottom: 1}, th.Border).Children(
		ui.Toggle("Pick an element", ins.picking).Key("inspector.pick").Icon(icons.Search).Small().
			Tooltip("Click an element in the window to select it (Ctrl+Shift+C; Escape cancels)").
			OnChange(ins.tg.Pick),
		ui.Text(ins.title).FontSize(13).TextColor(th.TextMuted).Truncate().Grow(1),
		ui.TextInput(ins.filter).Key("inspector.filter").Small().Icon(icons.Filter).
			Label("Filter elements").Placeholder("Filter by kind, key, text or component").
			Width(ui.Px(280)),
	)
}

// treePane is the element tree. The window's root is not shown: its children
// are the top level of the tree. While the filter has text, the tree is a flat
// list of the elements that match it, in tree order.
func (ins *inspector) treePane(th *ui.Theme) ui.Element {
	tg := ins.tg
	names := tg.AccessNames()
	var nodes []ui.TreeNode
	key, empty := "inspector.tree", "The window has no elements."
	if q := strings.TrimSpace(ins.filter.Get()); q != "" {
		empty = "No element matches the filter."
		key = "inspector.matches"
		ins.matches(tg.Root(), strings.ToLower(q), names, &nodes)
	} else if root, ok := tg.Element(tg.Root()); ok {
		for _, c := range root.Children {
			nodes = append(nodes, ins.treeNode(c, names))
		}
	}
	var body ui.Element
	if len(nodes) == 0 {
		body = ui.Div().Padding(12).Child(ui.Text(empty).FontSize(13).TextColor(th.TextMuted))
	} else {
		// The row under the pointer is outlined in the window, until the pointer
		// leaves the tree.
		body = ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).OnMouseLeave(func() { tg.Hover(0) }).Child(
			ui.Tree(nodes, ins.sel).Key(key).Label("Elements").Grow(1).
				Styles(ui.TreeStyle{Label: ui.Style{Grow: 1, MinWidth: ui.Px(0)}}).
				RenderLabel(func(n ui.TreeNode, st ui.PartState) ui.Element { return ins.treeRow(th, n, st) }).
				OnChange(func(key string) {
					tg.Select(idOf(key))
					ins.bump()
				}))
	}
	return ui.Div().Column().Height(ui.Full).Gap(4).Padding(8).Children(
		ui.Div().PaddingX(8).Child(heading(th, "Elements")),
		body,
	)
}

// treeNode returns the tree node of the element id, named with its row text and
// the role and name that names holds for it. Its children are read when the
// node is opened, so a large tree costs only what is open.
func (ins *inspector) treeNode(id uint64, names map[uint64]string) ui.TreeNode {
	e, ok := ins.tg.Element(id)
	node := ui.TreeNode{Key: keyOf(id), Label: rowName(e, names[id])}
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

// matches appends a flat node for each element of the subtree at id, below the
// window's root, whose kind, key, part, text or component contains q.
func (ins *inspector) matches(id uint64, q string, names map[uint64]string, out *[]ui.TreeNode) {
	e, ok := ins.tg.Element(id)
	if !ok {
		return
	}
	if id != ins.tg.Root() {
		hay := strings.ToLower(strings.Join([]string{e.Kind, e.Key, e.Part, e.Text, e.Component}, "\x00"))
		if strings.Contains(hay, q) {
			*out = append(*out, ui.TreeNode{Key: keyOf(id), Label: rowName(e, names[id])})
		}
	}
	for _, c := range e.Children {
		ins.matches(c, q, names, out)
	}
}

// treeRow is the visible text of a tree row, on one line that ends with an
// ellipsis: the kind, the key in the link color, the part, the text in quotes and
// the component's short name, both muted. A hidden element is dimmed. The full
// component name and the access name are in the tooltip.
func (ins *inspector) treeRow(th *ui.Theme, n ui.TreeNode, st ui.PartState) ui.Element {
	id := idOf(n.Key)
	e, _ := ins.tg.Element(id)
	plain := st.HighContrast && st.Selected // the row's own colors, on the accent fill
	color := func(c ui.Color) ui.Color {
		if plain {
			return ui.Color{}
		}
		return c
	}
	segs := []ui.Segment{ui.Seg(e.Kind).Color(color(th.Text))}
	if e.Key != "" {
		segs = append(segs, ui.Seg(" "+e.Key).Color(color(th.Link)))
	}
	if e.Part != "" {
		segs = append(segs, ui.Seg(" "+e.Part).Color(color(th.Text)))
	}
	if e.Text != "" {
		segs = append(segs, ui.Seg(" "+strconv.Quote(e.Text)).Color(color(th.TextMuted)))
	}
	if e.Boundary {
		segs = append(segs, ui.Seg("  "+shortComponent(e.Component)).Color(color(th.TextSubtle)))
	}
	if e.Hidden {
		segs = append(segs, ui.Seg("  hidden").Color(color(th.TextSubtle)))
	}
	row := ui.Div().Row().Align(ui.Center).MinWidth(ui.Px(0)).OnMouseEnter(func() { ins.tg.Hover(id) }).
		Child(ui.Rich(segs...).FontSize(13).Truncate().Grow(1))
	if e.Hidden && !plain {
		row.Opacity(0.6)
	}
	tip := n.Label
	if e.Boundary {
		tip += "\n" + e.Component
	}
	return row.Tooltip(tip)
}

// rowName is the accessible name of an element's tree row: its kind, then its
// part, key, text, component and visibility when it has them, and the role and
// name of its access node, access.
func rowName(e inspect.Element, access string) string {
	s := elementLabel(e)
	if e.Boundary {
		s += " component " + shortComponent(e.Component)
	}
	if e.Hidden {
		s += " (hidden)"
	}
	if access != "" {
		s += " " + access
	}
	return s
}

// elementLabel names an element by its kind, part, key and text.
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
	return s
}

// shortComponent is the short name of a component's render function: the base
// name of its package and the type or function, such as ui.TitleBar for
// github.com/tyler-builds/jamb/ui.(*TitleBarElement).expand.func1.
func shortComponent(full string) string {
	s := full[strings.LastIndex(full, "/")+1:]
	s = strings.TrimPrefix(s, "*")
	pkg, rest, ok := strings.Cut(s, ".")
	if !ok {
		return s
	}
	rest = strings.TrimPrefix(rest, "(*")
	rest = strings.TrimPrefix(rest, "(")
	name, _, _ := strings.Cut(rest, ")")
	name, _, _ = strings.Cut(name, ".")
	if n := strings.TrimSuffix(name, "Element"); n != "" {
		name = n
	}
	return pkg + "." + name
}

// detailsPane is what the inspector shows of the selected element, in one
// scrolling column, over the frame timings of the window.
func (ins *inspector) detailsPane(ctx *ui.Context, th *ui.Theme, sel uint64) ui.Element {
	body := ui.Div().Column().Gap(20).Padding(inspPad)
	e, ok := ins.tg.Element(sel)
	if sel == 0 || !ok {
		body.Grow(1).Justify(ui.Center).Align(ui.Center).Gap(8).Children(
			ui.Icon(icons.Search).Size(28).Color(th.TextSubtle),
			ui.Text("No element selected").TextColor(th.TextMuted),
			ui.Text("Select an element in the tree, or press Pick an element (Ctrl+Shift+C) and click one in the window.").
				FontSize(13).TextColor(th.TextMuted).Align(ui.Center).MaxWidth(ui.Px(360)),
		)
	} else {
		body.Children(
			ins.header(th, e),
			section(th, "Layout", ins.layoutSection(th, e)),
			section(th, "Accessibility", ins.accessSection(th, sel)),
			section(th, "Style", ins.styleSection(ctx, th, sel)),
		)
	}
	return ui.Div().Column().Height(ui.Full).BorderEdges(ui.Edges{Left: 1}, th.Border).Children(
		ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).ScrollY().Child(body),
		ins.framesSection(th),
	)
}

// header names the selected element, gives the full name of the component that
// rendered it, and the trail of its ancestors, each of which selects it.
func (ins *inspector) header(th *ui.Theme, e inspect.Element) ui.Element {
	tg := ins.tg
	title := e.Kind
	if e.Key != "" {
		title += " " + e.Key
	}
	if e.Part != "" {
		title += " " + e.Part
	}
	col := ui.Div().Column().Gap(6).Child(ui.Div().Heading(2).Child(ui.Text(title).FontSize(18).Semibold()))

	// The component is the element's own when it is the root of one, or else
	// the nearest above it.
	var trail []inspect.Element
	comp := ""
	for id := e.Parent; id != 0; {
		a, ok := tg.Element(id)
		if !ok {
			break
		}
		trail = append(trail, a)
		if comp == "" && a.Boundary {
			comp = a.Component
		}
		id = a.Parent
	}
	if e.Boundary {
		comp = e.Component
	}
	if comp != "" {
		col.Child(ui.Text(comp).Mono().FontSize(inspSmall).TextColor(th.TextMuted).Selectable())
	}
	if len(trail) > 0 {
		slices.Reverse(trail)
		var crumbs []ui.Crumb
		for _, a := range trail {
			id := a.ID
			crumbs = append(crumbs, ui.Crumb{Label: crumbName(a), OnClick: func() { tg.Select(id) }})
		}
		crumbs = append(crumbs, ui.Crumb{Label: crumbName(e)})
		col.Child(ui.Breadcrumbs(crumbs...).Label("Ancestors").MaxItems(6))
	}
	return col
}

// crumbName is the short name of an element in the trail of ancestors.
func crumbName(e inspect.Element) string {
	switch {
	case e.Key != "":
		return e.Kind + " " + e.Key
	case e.Part != "":
		return e.Part
	case e.Boundary:
		return shortComponent(e.Component)
	}
	return e.Kind
}

// heading is the small, muted heading of a section.
func heading(th *ui.Theme, title string) ui.Element {
	return ui.Div().Heading(3).Child(ui.Text(title).FontSize(inspSmall).Semibold().TextColor(th.TextMuted))
}

// section is a part of the details: a divider, its heading, and its content.
func section(th *ui.Theme, title string, content ui.Element) ui.Element {
	return ui.Div().Column().Gap(10).PaddingEdges(ui.Edges{Top: 12}).BorderEdges(ui.Edges{Top: 1}, th.Border).
		Children(heading(th, title), content)
}

// layoutSection draws the box model of the element, with its insets, as the
// browser's developer tools do, beside its position, size, content size and
// overflow.
func (ins *inspector) layoutSection(th *ui.Theme, e inspect.Element) ui.Element {
	content := ui.Div().Padding(6).Background(inspContentTint).Border(1, th.Border).Radius(2).
		Child(ui.Textf("%s × %s", num(contentW(e)), num(contentH(e))).FontSize(inspSmall).Mono())
	model := boxLayer(th, "padding", e.Padding, inspPaddingTint, content)
	model = boxLayer(th, "border", e.Border, inspBorderTint, model)
	model = boxLayer(th, "margin", e.Margin, inspMarginTint, model)

	list := ui.DescriptionList().Styles(ui.DescriptionListStyle{DescriptionList: ui.Style{Gap: 6}}).
		ItemText("Position", fmt.Sprintf("x %s, y %s", num(e.Box.X), num(e.Box.Y))).
		ItemText("Size", fmt.Sprintf("%s × %s", num(e.Box.W), num(e.Box.H))).
		ItemText("Content", fmt.Sprintf("%s × %s", num(e.ContentW), num(e.ContentH))).
		ItemText("Overflow", fmt.Sprintf("x %s, y %s", e.OverflowX, e.OverflowY))
	return ui.Div().Row().Wrap().Gap(20).Align(ui.Start).Children(
		ui.Div().MinWidth(ui.Px(280)).MaxWidth(ui.Px(400)).Grow(1).Child(model),
		ui.Div().MinWidth(ui.Px(220)).Grow(1).Child(list),
	)
}

// contentW and contentH are the size of the element's content box: its border
// box less its border and padding.
func contentW(e inspect.Element) float32 {
	return max(0, e.Box.W-e.Border.Left-e.Border.Right-e.Padding.Left-e.Padding.Right)
}

func contentH(e inspect.Element) float32 {
	return max(0, e.Box.H-e.Border.Top-e.Border.Bottom-e.Padding.Top-e.Padding.Bottom)
}

// boxLayer is one box of the box model diagram: its name at the top left, its
// four insets around inner, on its tint.
func boxLayer(th *ui.Theme, name string, e inspect.Edges, tint ui.Color, inner ui.Element) ui.Element {
	side := func(v float32) ui.Element {
		return ui.Text(edgeNum(v)).FontSize(11).Mono().Align(ui.Center).Width(ui.Px(30))
	}
	return ui.Div().Column().Align(ui.Center).Gap(2).Padding(4).Background(tint).Border(1, th.Border).Radius(2).Children(
		ui.Div().Row().Width(ui.Full).Children(
			ui.Text(name).FontSize(10).TextColor(th.TextMuted).Width(ui.Px(50)),
			ui.Div().Grow(1).Row().Justify(ui.Center).Child(side(e.Top)),
			ui.Div().Width(ui.Px(50)),
		),
		ui.Div().Row().Align(ui.Center).Width(ui.Full).Children(
			side(e.Left),
			ui.Div().Grow(1).Column().Align(ui.Center).Child(inner),
			side(e.Right),
		),
		side(e.Bottom),
	)
}

// edgeNum is an inset in the diagram: its number, or a dash for none.
func edgeNum(v float32) string {
	if v == 0 {
		return "–"
	}
	return num(v)
}

// num is a number of logical pixels as text, rounded to the hundredth.
func num(v float32) string { return strconv.FormatFloat(math.Round(float64(v)*100)/100, 'f', -1, 64) }

// accessSection is the accessibility node of the element and its warnings, as
// JAMB_A11Y_WARN reports them.
func (ins *inspector) accessSection(th *ui.Theme, id uint64) ui.Element {
	a := ins.tg.Access(id)
	if !a.Exposed {
		return ui.Text("Not in the accessibility tree").FontSize(13).TextColor(th.TextMuted)
	}
	orNone := func(s string) string {
		if s == "" {
			return "–"
		}
		return s
	}
	list := ui.DescriptionList().Styles(ui.DescriptionListStyle{DescriptionList: ui.Style{Gap: 6}}).
		ItemText("Role", a.Role).
		ItemText("Name", orNone(a.Name)).
		ItemText("Description", orNone(a.Description)).
		ItemText("Value", orNone(a.Value)).
		ItemText("States", orNone(strings.Join(a.States, ", "))).
		ItemText("Actions", orNone(strings.Join(a.Actions, ", ")))
	col := ui.Div().Column().Gap(8)
	for _, w := range a.Warnings {
		col.Child(ui.Alert(w).Warning())
	}
	return col.Child(list)
}

// styleSection is the live edit of the element, and the set fields of its
// resolved style in a table: each field, its winning value and the layer it came
// from, with the values of the layers it overrides under it, struck through,
// from the highest layer down.
func (ins *inspector) styleSection(ctx *ui.Context, th *ui.Theme, id uint64) ui.Element {
	grid := ui.Div().Grid(ui.Px(130), ui.Fr(1), ui.Auto).Gap(6).Align(ui.Center)
	mono := func(s string, c ui.Color) *ui.TextElement {
		return ui.Text(s).Mono().FontSize(inspSmall).TextColor(c).Truncate()
	}
	for _, f := range ins.tg.Fields(id) {
		src := f.Source
		if len(f.Layers) > 0 && f.Layers[len(f.Layers)-1].Same {
			src += ", same as below"
		}
		grid.Children(mono(f.Name, th.TextMuted), mono(f.Value, th.Text).Selectable(), ui.Tag(src))
		for i := len(f.Layers) - 2; i >= 0; i-- {
			l := f.Layers[i]
			src := l.Source
			if l.Same {
				src += ", same as below"
			}
			v := mono(l.Value, th.TextSubtle)
			if l.Value != "" {
				v.Highlight(0, len(l.Value), ui.Highlight{Strikethrough: true})
			}
			grid.Children(ui.Div(), v, ui.Text("overridden: "+src).FontSize(inspSmall).TextColor(th.TextSubtle).Truncate())
		}
	}
	return ui.Div().Column().Gap(12).Children(ins.editRow(ctx, th, id), grid)
}

// editRow sets one style field of the element, copies the live edits as Go, and
// says what the last action did.
func (ins *inspector) editRow(ctx *ui.Context, th *ui.Theme, sel uint64) ui.Element {
	tg := ins.tg
	var options []ui.Option[string]
	for _, name := range tg.EditableFields() {
		options = append(options, ui.Option[string]{Value: name, Label: name})
	}
	apply := func() {
		if err := tg.Edit(sel, ins.field.Peek(), ins.value.Peek()); err != nil {
			ins.status.Set(err.Error())
		} else {
			ins.status.Set("Applied to the window")
		}
		ins.bump()
	}
	edited := tg.Edited(sel)
	row := ui.Div().Row().Wrap().Align(ui.Center).Gap(6).Children(
		ui.Select(ins.field, options...).Label("Style field").Small().Width(ui.Px(150)),
		ui.TextInput(ins.value).Label("Value").Placeholder("Value").Small().Width(ui.Px(130)).
			OnSubmit(func(string) { apply() }),
		ui.Button("Apply").Key("inspector.apply").Small().OnClick(apply),
		ui.Button("Clear edits").Key("inspector.clear").Small().Ghost().Disabled(!edited).OnClick(func() {
			tg.ClearEdit(sel)
			ins.status.Set("Edits cleared")
			ins.bump()
		}),
		ui.Button("Copy as Go").Key("inspector.copy").Small().Ghost().Icon(icons.Copy).Disabled(!edited).OnClick(func() {
			ctx.SetClipboard(tg.GoLiteral(sel))
			ins.status.Set("Copied the edits as a ui.Style literal")
		}),
	)
	col := ui.Div().Column().Gap(4).Child(row)
	if s := ins.status.Get(); s != "" {
		col.Child(ui.Text(s).FontSize(inspSmall).TextColor(th.TextMuted))
	}
	return col
}

// framesSection is pinned under the details: tiles with the frame timings and
// the counts of the window's last frame, and a sparkline of the CPU time per
// frame.
func (ins *inspector) framesSection(th *ui.Theme) ui.Element {
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
	tiles := ui.Div().Row().Wrap().Gap(6).Children(
		tile(th, "Last frame", ms(last.Total())),
		tile(th, "Average", ms(avg)),
		tile(th, "Peak", ms(ins.perf.MaxTotal())),
		tile(th, "Nodes", strconv.Itoa(last.Nodes)),
		tile(th, "Components", strconv.Itoa(last.Rendered)),
		tile(th, "Quads", strconv.Itoa(last.Quads)),
		tile(th, "Glyphs", strconv.Itoa(last.Glyphs)),
	)
	note := fmt.Sprintf("Last frame: build %s, layout %s, paint %s. Average and peak over %d frames.",
		ms(last.Build), ms(last.Layout), ms(last.Paint), len(samples))
	return ui.Div().Column().Gap(10).PaddingX(inspPad).PaddingY(12).BorderEdges(ui.Edges{Top: 1}, th.Border).Children(
		heading(th, "Frames of the window"),
		tiles,
		ui.Text(note).FontSize(inspSmall).TextColor(th.TextMuted),
		ins.sparkline(th, samples),
	)
}

// tile is a small stat: its name over its value.
func tile(th *ui.Theme, name, value string) ui.Element {
	return ui.Div().Column().Gap(2).PaddingX(8).PaddingY(6).Grow(1).
		Background(th.Surface).Border(1, th.Border).Radius(th.Radius).Children(
		ui.Text(name).FontSize(11).TextColor(th.TextMuted),
		ui.Text(value).FontSize(14).Mono(),
	)
}

// ms is a duration in milliseconds, for display.
func ms(d time.Duration) string { return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond)) }

// sparkline draws the CPU time of each kept frame, with the frame budget as a
// line. It is scaled to the larger of the peak and the budget, so that one slow
// frame does not flatten the others below the budget.
func (ins *inspector) sparkline(th *ui.Theme, samples []inspect.Sample) ui.Element {
	peak := ins.perf.MaxTotal()
	low := peak
	for _, s := range samples {
		low = min(low, s.Total())
	}
	top := max(peak, inspBudget)
	return ui.Canvas(func(p *ui.Painter, w, h float32) {
		const pad = 4
		y := func(d time.Duration) float32 { return h - pad - (h-2*pad)*float32(d)/float32(top) }
		label := ui.TextFormat{FontSize: 10, Color: th.TextMuted}
		by := y(inspBudget)
		p.Stroke(ui.NewPath().MoveTo(0, by).LineTo(w, by), ui.StrokeStyle{Width: 1, Color: th.Warning.Alpha(0.7)})
		p.Text(w-2, by-2, "16.7 ms", ui.TextFormat{FontSize: 10, Color: th.TextMuted, Align: ui.End, VAlign: ui.End})
		if len(samples) < 2 {
			return
		}
		line, area := ui.NewPath(), ui.NewPath().MoveTo(0, h)
		for i, s := range samples {
			x := w * float32(i) / float32(len(samples)-1)
			if i == 0 {
				line.MoveTo(x, y(s.Total()))
			} else {
				line.LineTo(x, y(s.Total()))
			}
			area.LineTo(x, y(s.Total()))
		}
		area.LineTo(w, h).Close()
		p.Fill(area, th.Accent.Alpha(0.15))
		p.Stroke(line, ui.StrokeStyle{Width: 1.5, Color: th.Accent})
		p.Text(2, 2, "peak "+ms(peak), label)
		p.Text(2, h-2, "min "+ms(low), ui.TextFormat{FontSize: 10, Color: th.TextMuted, VAlign: ui.End})
	}).Width(ui.Full).Height(ui.Px(52)).Label("CPU time per frame of the window, as a line, with the 16.7 ms budget")
}
