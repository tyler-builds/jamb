package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// sizeCase is one widget in the sizing tables. def is the widget with no sizing
// method. set is the widget with its sizing methods called with the given values,
// and part is the part of its outer box, or "" when the widget's own element (the
// container's last child) is the outer box. box is the width of the container the
// widget is laid out in; auto says what Width(Auto) does in a column.
type sizeCase struct {
	name string
	def  func() Element
	set  func(w, minW, maxW Length, grow float32) Element
	part string
	box  float32
	auto autoMode
}

// autoMode is how Width(Auto) lays out a widget in a column of its own width.
type autoMode uint8

const (
	autoContent autoMode = iota // the widget's content width, as in a row
	autoFill                    // stretches to the column, as a Div does
	autoSkip                    // not compared: a floating or content-less box
)

// sizedFrame lays out el in a 400 px column (or a 500 px window for the floating
// modal) inside a container keyed "box", and returns the runtime.
func sizedFrame(box float32, el Element) *Runtime {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Width(Px(box)).Column().Child(el))
	}, 500, 400, 1)
	rt.Frame(time.Now())
	return rt
}

// rowFrame lays out el in a row of automatic width, so el takes its content width.
func rowFrame(el Element) *Runtime {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Row().Child(Div().Key("box").Row().Child(el))
	}, 500, 400, 1)
	rt.Frame(time.Now())
	return rt
}

// outerOf returns the node of a widget's outer box in rt, found by its part or as
// the last child of the container keyed "box".
func outerOf(t *testing.T, rt *Runtime, c sizeCase) *node {
	t.Helper()
	if c.part != "" {
		return partNode(t, rt, c.part, "")
	}
	box := rt.findKey(rt.tree, "box")
	if box == nil || len(box.children) == 0 {
		t.Fatalf("%s: no outer box", c.name)
	}
	return box.children[len(box.children)-1]
}

// sizeCases lists every widget that takes the sizing methods, with its default and
// its sized builders. Modal is floating, so its box is the 500 px window.
func sizeCases() []sizeCase {
	opts := []Option[string]{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	tabs := []Tab[string]{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	return []sizeCase{
		{name: "Button", part: "Button.Root", box: 400, auto: autoContent,
			def: func() Element { return Button("Save") },
			set: func(w, minW, maxW Length, g float32) Element {
				return Button("Save").Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "TextInput", part: "TextInput.Frame", box: 400, auto: autoFill,
			def: func() Element { return TextInput(NewSignal("")).Placeholder("Search") },
			set: func(w, minW, maxW Length, g float32) Element {
				return TextInput(NewSignal("")).Placeholder("Search").Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "TextArea", part: "TextArea.Frame", box: 400, auto: autoFill,
			def: func() Element { return TextArea(NewSignal("")) },
			set: func(w, minW, maxW Length, g float32) Element {
				return TextArea(NewSignal("")).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "NumberInput", box: 400, auto: autoFill,
			def: func() Element { return NumberInput(NewSignal(3), 0, 10, 1) },
			set: func(w, minW, maxW Length, g float32) Element {
				return NumberInput(NewSignal(3), 0, 10, 1).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Select", part: "Select.Trigger", box: 400, auto: autoFill,
			def: func() Element { return Select(NewSignal("a"), opts...) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Select(NewSignal("a"), opts...).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Slider", box: 400, auto: autoFill,
			def: func() Element { return Slider(NewSignal(50.0), 0, 100, 5) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Slider(NewSignal(50.0), 0, 100, 5).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "ProgressBar", box: 400, auto: autoFill,
			def: func() Element { return ProgressBar(0.5) },
			set: func(w, minW, maxW Length, g float32) Element {
				return ProgressBar(0.5).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "IndeterminateProgressBar", box: 400, auto: autoFill,
			def: func() Element { return IndeterminateProgressBar() },
			set: func(w, minW, maxW Length, g float32) Element {
				return IndeterminateProgressBar().Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Checkbox", part: "Checkbox.Control", box: 400, auto: autoContent,
			def: func() Element { return Checkbox("Include", NewSignal(false)) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Checkbox("Include", NewSignal(false)).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "RadioGroup", part: "RadioGroup.Group", box: 400, auto: autoContent,
			def: func() Element { return RadioGroup(NewSignal("a"), opts...) },
			set: func(w, minW, maxW Length, g float32) Element {
				return RadioGroup(NewSignal("a"), opts...).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Switch", part: "Switch.Track", box: 400, auto: autoSkip,
			def: func() Element { return Switch("Sync", NewSignal(false)) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Switch("Sync", NewSignal(false)).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Badge", part: "Badge.Badge", box: 400, auto: autoContent,
			def: func() Element { return Badge(5) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Badge(5).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Tabs", part: "Tabs.List", box: 400, auto: autoFill,
			def: func() Element { return Tabs(NewSignal("a"), tabs...) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Tabs(NewSignal("a"), tabs...).Width(w).MinWidth(minW).MaxWidth(maxW)
			}},
		{name: "Collapsible", part: "Collapsible.Section", box: 400, auto: autoFill,
			def: func() Element { return Collapsible("Advanced", NewSignal(false), Text("body")) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Collapsible("Advanced", NewSignal(false), Text("body")).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Accordion", box: 400, auto: autoFill,
			def: func() Element {
				return Accordion(Collapsible("Advanced", NewSignal(false), Text("body")))
			},
			set: func(w, minW, maxW Length, g float32) Element {
				return Accordion(Collapsible("Advanced", NewSignal(false), Text("body"))).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Text", box: 400, auto: autoFill,
			def: func() Element { return Text("Hello") },
			set: func(w, minW, maxW Length, g float32) Element {
				return Text("Hello").Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Field", part: "FormField.Root", box: 400, auto: autoFill,
			def: func() Element { return Field("Name", TextInput(NewSignal("")).Label("Name")) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Field("Name", TextInput(NewSignal("")).Label("Name")).Width(w).MinWidth(minW).MaxWidth(maxW).Grow(g)
			}},
		{name: "Modal", part: "Modal.Panel", box: 500, auto: autoSkip,
			def: func() Element { return Modal("Settings", Text("Body")) },
			set: func(w, minW, maxW Length, g float32) Element {
				return Modal("Settings", Text("Body")).Width(w).MinWidth(minW).MaxWidth(maxW)
			}},
	}
}

// outerWidth returns the outer width of the widget build returns, in its container
// of width c.box. A modal's panel is measured in the window, which is c.box wide.
func outerWidth(t *testing.T, c sizeCase, build func() Element) float32 {
	t.Helper()
	rt := sizedFrame(c.box, build())
	return outerOf(t, rt, c).lay.W
}

// TestEveryWidgetTakesTheSizingMethods lays out every sized widget at Px, Percent,
// Full and Auto widths, and checks the width of its outer box in a column.
func TestEveryWidgetTakesTheSizingMethods(t *testing.T) {
	for _, c := range sizeCases() {
		t.Run(c.name, func(t *testing.T) {
			box := c.box
			check := func(label string, w Length, want float32) {
				t.Helper()
				got := outerWidth(t, c, func() Element { return c.set(w, Auto, Auto, 0) })
				if got != want {
					t.Errorf("%s: outer width %v, want %v", label, got, want)
				}
			}
			check("Px(200)", Px(200), 200)
			check("Percent(50)", Percent(50), box/2)
			check("Full", Full, box)
			switch c.auto {
			case autoFill:
				check("Auto", Auto, box)
			case autoContent:
				want := outerOf(t, rowFrame(c.set(Auto, Auto, Auto, 0)), c).lay.W
				check("Auto", Auto, want)
				if want >= box {
					t.Errorf("Auto: content width %v should be narrower than the box %v", want, box)
				}
			}
		})
	}
}

// TestSizingMinMaxAndGrow checks the limits and the share of a row that a widget
// takes, for every widget that takes the methods.
func TestSizingMinMaxAndGrow(t *testing.T) {
	for _, c := range sizeCases() {
		t.Run(c.name, func(t *testing.T) {
			// The default width is the reference for the limits.
			def := outerWidth(t, c, c.def)
			if got := outerWidth(t, c, func() Element { return c.set(Px(50), Px(def+60), Auto, 0) }); got != def+60 {
				t.Errorf("MinWidth(%v) above a 50 px width: outer width %v, want %v", def+60, got, def+60)
			}
			if got := outerWidth(t, c, func() Element { return c.set(Full, Auto, Px(def/2), 0) }); got != def/2 {
				t.Errorf("MaxWidth(%v) under Full: outer width %v, want %v", def/2, got, def/2)
			}
			switch c.name {
			case "Modal", "Tabs":
				return // a floating panel is not in a row, and the tab list has no Grow
			}
			// Grow takes the spare width of a row: the widget starts 50 px wide, and a
			// 100 px sibling is beside it in a 400 px row, so it grows to 300 px.
			rt := newTestRuntime(func(ctx *Context) Element {
				return Div().Column().Child(Div().Key("box").Row().Width(Px(400)).
					Child(Div().Width(Px(100))).
					Child(c.set(Px(50), Auto, Auto, 1)))
			}, 500, 400, 1)
			rt.Frame(time.Now())
			row := rt.findKey(rt.tree, "box")
			if c.part == "" {
				// Without a part, the widget is the container's last child.
				if got := row.children[len(row.children)-1].lay.W; got != 300 {
					t.Errorf("Grow(1) in a row: %v, want 300", got)
				}
				return
			}
			if got := partNode(t, rt, c.part, "").lay.W; got != 300 {
				t.Errorf("Grow(1) in a row: %v, want 300", got)
			}
		})
	}
}

// TestSizingDefaults checks the default outer width of each widget at the Text
// size of 100 percent, as the README and the sizing section of styling.md state.
func TestSizingDefaults(t *testing.T) {
	want := map[string]float32{
		"TextInput":                textFieldWidth,
		"TextArea":                 textFieldWidth,
		"NumberInput":              numberInputWidth,
		"Select":                   selectWidth,
		"Slider":                   400,
		"ProgressBar":              400,
		"IndeterminateProgressBar": 400,
		"Switch":                   38,
		"Tabs":                     400,
		"Collapsible":              400,
		"Accordion":                400,
		"Text":                     400,
		"Field":                    400,
		"Modal":                    modalWidth,
	}
	for _, c := range sizeCases() {
		t.Run(c.name, func(t *testing.T) {
			got := outerWidth(t, c, c.def)
			if w, ok := want[c.name]; ok {
				if got != w {
					t.Errorf("default outer width %v, want %v", got, w)
				}
				return
			}
			// The content-sized widgets are narrower than their container.
			if got >= c.box {
				t.Errorf("default outer width %v, want the content width, narrower than %v", got, c.box)
			}
		})
	}
}

// TestSizingDefaultsScaleWithText checks that a default width grows with the Text
// size, as a field's text does, so the controls keep their proportions at 225 percent.
func TestSizingDefaultsScaleWithText(t *testing.T) {
	for _, c := range sizeCases() {
		switch c.name {
		case "TextInput", "TextArea", "NumberInput", "Select":
		default:
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			base := outerWidth(t, c, c.def)
			rt := newTestRuntime(func(ctx *Context) Element {
				return Div().Column().Child(Div().Key("box").Width(Px(400)).Column().Child(c.def()))
			}, 500, 400, 1)
			rt.SetSystemSettings(SystemSettings{TextScale: 225})
			rt.Frame(time.Now())
			if got := outerOf(t, rt, c).lay.W; got != base*2.25 {
				t.Errorf("at 225 percent the default is %v, want %v", got, base*2.25)
			}
		})
	}
}

// TestSizingPrecedence checks the layers of a widget's outer size. A default is
// overridden by the theme, the theme by the Styles of the instance, and all of them
// by a sizing method, whatever the order of the calls.
func TestSizingPrecedence(t *testing.T) {
	type layers struct {
		name string
		part string
		// theme sets the theme's width of the outer part.
		theme func(th *Theme, w Length)
		// inst returns the widget with its instance Styles width set to w.
		inst func(w Length) Element
		// method returns the widget with its sizing method set to w.
		method func(w Length) Element
	}
	cases := []layers{
		{name: "Button", part: "Button.Root",
			theme:  func(th *Theme, w Length) { th.Button.Root.Width = w },
			inst:   func(w Length) Element { return Button("Save").Styles(ButtonStyle{Root: Style{Width: w}}) },
			method: func(w Length) Element { return Button("Save").Width(w) }},
		{name: "TextInput", part: "TextInput.Frame",
			theme:  func(th *Theme, w Length) { th.Field.Frame.Width = w },
			inst:   func(w Length) Element { return TextInput(NewSignal("")).Styles(FieldStyle{Frame: Style{Width: w}}) },
			method: func(w Length) Element { return TextInput(NewSignal("")).Width(w) }},
		{name: "TextArea", part: "TextArea.Frame",
			theme:  func(th *Theme, w Length) { th.Field.Frame.Width = w },
			inst:   func(w Length) Element { return TextArea(NewSignal("")).Styles(FieldStyle{Frame: Style{Width: w}}) },
			method: func(w Length) Element { return TextArea(NewSignal("")).Width(w) }},
		{name: "Select", part: "Select.Trigger",
			theme: func(th *Theme, w Length) { th.Select.Trigger.Width = w },
			inst: func(w Length) Element {
				return Select(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).Styles(SelectStyle{Trigger: Style{Width: w}})
			},
			method: func(w Length) Element { return Select(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).Width(w) }},
		{name: "Checkbox", part: "Checkbox.Control",
			theme: func(th *Theme, w Length) { th.Checkbox.Control.Width = w },
			inst: func(w Length) Element {
				return Checkbox("Include", NewSignal(false)).Styles(CheckboxStyle{Control: Style{Width: w}})
			},
			method: func(w Length) Element { return Checkbox("Include", NewSignal(false)).Width(w) }},
		{name: "RadioGroup", part: "RadioGroup.Group",
			theme: func(th *Theme, w Length) { th.RadioGroup.Group.Width = w },
			inst: func(w Length) Element {
				return RadioGroup(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).Styles(RadioGroupStyle{Group: Style{Width: w}})
			},
			method: func(w Length) Element {
				return RadioGroup(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).Width(w)
			}},
		{name: "Badge", part: "Badge.Badge",
			theme:  func(th *Theme, w Length) { th.Badge.Badge.Width = w },
			inst:   func(w Length) Element { return Badge(5).Styles(BadgeStyle{Badge: Style{Width: w}}) },
			method: func(w Length) Element { return Badge(5).Width(w) }},
		{name: "Collapsible", part: "Collapsible.Section",
			theme: func(th *Theme, w Length) { th.Collapsible.Section.Width = w },
			inst: func(w Length) Element {
				return Collapsible("Advanced", NewSignal(false), Text("body")).Styles(CollapsibleStyle{Section: Style{Width: w}})
			},
			method: func(w Length) Element {
				return Collapsible("Advanced", NewSignal(false), Text("body")).Width(w)
			}},
		{name: "Modal", part: "Modal.Panel",
			theme: func(th *Theme, w Length) { th.Modal.Panel.Width = w },
			inst: func(w Length) Element {
				return Modal("Settings", Text("Body")).Styles(ModalStyle{Panel: Style{Width: w}})
			},
			method: func(w Length) Element { return Modal("Settings", Text("Body")).Width(w) }},
	}
	layoutOf := func(t *testing.T, th *Theme, part string, el Element) float32 {
		t.Helper()
		r := newThemedRig(th, func(ctx *Context) Element {
			return Div().Column().Child(Div().Key("box").Width(Px(400)).Column().Child(el))
		})
		return partNode(t, r.rt, part, "").lay.W
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			th := *DarkTheme
			c.theme(&th, Px(300))
			// The theme's width replaces the default.
			if got := layoutOf(t, &th, c.part, c.inst(Auto)); got != 300 {
				t.Errorf("theme width: outer width %v, want 300", got)
			}
			// The instance's Styles win over the theme.
			if got := layoutOf(t, &th, c.part, c.inst(Px(250))); got != 250 {
				t.Errorf("instance over theme: outer width %v, want 250", got)
			}
			// A sizing method wins over the instance's Styles and the theme, in either order.
			if got := layoutOf(t, &th, c.part, c.method(Px(120))); got != 120 {
				t.Errorf("method over theme: outer width %v, want 120", got)
			}
		})
	}
}

// TestSizingMethodWinsOverStylesWhateverTheOrder checks that a sizing method wins
// over the Width of the instance's Styles, whichever is called first.
func TestSizingMethodWinsOverStylesWhateverTheOrder(t *testing.T) {
	styled := Style{Width: Px(250)}
	for name, el := range map[string]Element{
		"method first": Button("Save").Width(Px(120)).Styles(ButtonStyle{Root: styled}),
		"styles first": Button("Save").Styles(ButtonStyle{Root: styled}).Width(Px(120)),
	} {
		rt := sizedFrame(400, el)
		if got := partNode(t, rt, "Button.Root", "").lay.W; got != 120 {
			t.Errorf("%s: width %v, want the method's 120", name, got)
		}
	}
}

// TestSelectListIsAtLeastAsWideAsItsField checks that the list of a select is as
// wide as the field it opens from, whatever the field's width, and not narrower.
func TestSelectListIsAtLeastAsWideAsItsField(t *testing.T) {
	for _, w := range []Length{Px(120), Px(400), Full, Auto} {
		sel := NewSignal("a")
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().Child(Div().Key("box").Width(Px(400)).Column().Child(
				Select(sel, Option[string]{Value: "a", Label: "Alpha"}, Option[string]{Value: "b", Label: "Beta"}).
					Label("Channel").Width(w)))
		})
		r.frame()
		r.focusAccess(t, access.RoleComboBox, "Channel")
		r.press(input.KeyEnter, 0) // opens the list
		field := partNode(t, r.rt, "Select.Trigger", "").lay.W
		list := partNode(t, r.rt, "Select.Popup", "").lay.W
		if list < field {
			t.Errorf("width %v: list is %v wide, narrower than the field's %v", w, list, field)
		}
	}
}

// A progress bar in a data grid cell fills the cell's width, less its padding, as
// its Full width says: the cell's content takes the cell's width when the cell is
// aligned to the start.
func TestProgressBarInGridCellFillsTheColumn(t *testing.T) {
	type row struct {
		name string
		p    float64
	}
	rows := []row{{"a", 0.7}}
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Column().Width(Px(600)).Height(Px(200)).Child(DataGrid(rows, func(r row) string { return r.name },
			DataColumn[row]{Header: "Name", Key: "name", Text: func(r row) string { return r.name }, Grow: 2},
			DataColumn[row]{Header: "Progress", Key: "progress", Text: func(r row) string { return "x" },
				Cell: func(r row) Element { return ProgressBar(r.p).Label("bar") }, Width: 200},
		))
	}, 600, 400, 1)
	rt.Frame(time.Now())
	for _, n := range partNodes(rt, "DataGrid.Cell") {
		if n.lay.W == 200 {
			if bar := n.children[0].children[0]; bar.lay.W != 184 {
				t.Errorf("progress bar in a 200 px cell is %v wide, want 184 (less the 8 px padding each side)", bar.lay.W)
			}
			return
		}
	}
	t.Fatal("no 200 px cell found")
}

// A progress bar and a slider keep a default minimum, so they stay visible where
// their container has no width of its own, and MinWidth replaces the minimum.
func TestBarsKeepAMinimumWhereTheirContainerHasNoWidth(t *testing.T) {
	for _, c := range []struct {
		name string
		def  func() Element
		min  float32
	}{
		{"ProgressBar", func() Element { return ProgressBar(0.5) }, progressBarMinWidth},
		{"Slider", func() Element { return Slider(NewSignal(50.0), 0, 100, 5) }, sliderMinWidth},
	} {
		if got := outerOf(t, rowFrame(c.def()), sizeCase{name: c.name}).lay.W; got != c.min {
			t.Errorf("%s in an auto-width row is %v wide, want its default minimum %v", c.name, got, c.min)
		}
	}
	rt := rowFrame(ProgressBar(0.5).MinWidth(Px(60)))
	if got := partNode(t, rt, "ProgressBar.Track", "").parent.lay.W; got > 60 {
		t.Errorf("ProgressBar with MinWidth(60) is %v wide, want at most 60", got)
	}
}

// A field fills its container, and its control keeps its own width. A control whose
// width is Auto or Full fills the field; a fixed one keeps its size, and a content-
// sized one stays as wide as its content.
func TestFieldControlsFillOnlyWhenTheirWidthSaysSo(t *testing.T) {
	const box = 400
	cases := []struct {
		name    string
		control func() Element
		part    string
		want    float32 // 0 means content-sized: narrower than the field
	}{
		{"TextInput at its default", func() Element { return TextInput(NewSignal("")) }, "TextInput.Frame", textFieldWidth},
		{"TextInput Full", func() Element { return TextInput(NewSignal("")).Width(Full) }, "TextInput.Frame", box},
		{"TextInput Auto", func() Element { return TextInput(NewSignal("")).Width(Auto) }, "TextInput.Frame", box},
		{"Select Full", func() Element {
			return Select(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).Width(Full)
		}, "Select.Trigger", box},
		{"Button content", func() Element { return Button("Save") }, "Button.Root", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rt := sizedFrame(box, Field("Name", c.control()))
			got := partNode(t, rt, c.part, "").lay.W
			switch {
			case c.want == 0 && got >= box:
				t.Errorf("content-sized control is %v wide in a %v field; it should stay its content width", got, box)
			case c.want != 0 && got != c.want:
				t.Errorf("control is %v wide in the field, want %v", got, c.want)
			}
		})
	}
}
