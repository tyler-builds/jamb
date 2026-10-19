package ui

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// The markers are colors no built-in style uses, so a part's style is found by
// its color on the node that draws it.
var (
	markBG = Hex("#0a0b0c") // the Background a part is given
	markFG = Hex("#0d0e0f") // the TextColor a part is given
)

// effectiveTextColor returns the color a text node is painted with: its own
// TextColor, or the nearest ancestor's, as paint inherits it.
func effectiveTextColor(n *node) Color {
	for a := n; a != nil; a = a.parent {
		if !a.style.TextColor.IsZero() {
			return a.style.TextColor
		}
	}
	return Color{}
}

// newThemedRig is newAccessRig with the app theme th.
func newThemedRig(th *Theme, root func(*Context) Element) *accessRig {
	r := &accessRig{now: time.Unix(1_000_000, 0)}
	r.rt = NewRuntime(root, th, nil)
	r.rt.clock = func() time.Time { return r.now }
	r.rt.SetWarnFunc(func(s string) { r.warnings = append(r.warnings, s) })
	r.rt.SetAccessWarnings(true)
	r.rt.SetViewport(500, 400, 1)
	r.rt.Frame(r.now)
	r.bridge = r.rt.AccessBridge()
	r.bridge.Activate()
	return r
}

// partNodes returns the nodes drawn for part, in no particular order.
func partNodes(rt *Runtime, part string) []*node {
	var out []*node
	for _, n := range rt.byID {
		if n.div != nil && n.div.part == part {
			out = append(out, n)
		}
	}
	return out
}

// partNode returns the one node drawn for part whose element has key, or the
// one node drawn for part when key is "".
func partNode(t *testing.T, rt *Runtime, part, key string) *node {
	t.Helper()
	var found []*node
	for _, n := range partNodes(rt, part) {
		if key == "" || n.key == key {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("part %s (key %q): %d nodes, want 1", part, key, len(found))
	}
	return found[0]
}

// editorNode returns the one text editor drawn in the rig, which holds the Text,
// Placeholder and Selection parts of a text field.
func editorNode(t *testing.T, rt *Runtime) *node {
	t.Helper()
	var found []*node
	for _, n := range rt.byID {
		if n.kind == kindEditor {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d text editors, want 1", len(found))
	}
	return found[0]
}

// drawnBy returns the part whose element draws the state part: a state has no
// element of its own, so its style is found on the element of the part it is a
// state of.
func drawnBy(part string) string {
	// The frame's states are drawn by the frame of their widget.
	if w, state, ok := strings.Cut(part, ".Frame"); ok && state != "" {
		return w + ".Frame"
	}
	switch part {
	case "Tabs.TabSelected", "Tabs.TabDisabled", "Tabs.TabHover", "Tabs.TabFocusVisible":
		return "Tabs.Tab"
	case "Select.TriggerHover", "Select.TriggerFocusVisible", "Select.TriggerDisabled", "Select.TriggerInvalid":
		return "Select.Trigger"
	case "Checkbox.BoxInvalid", "CheckboxTri.BoxInvalid":
		return "Checkbox.Box"
	case "RadioGroup.RingInvalid":
		return "RadioGroup.Ring"
	case "Form.SummaryItemHover", "Form.SummaryItemFocusVisible":
		return "Form.SummaryItem"
	case "Slider.TrackFocusVisible":
		return "Slider.Track"
	case "Slider.ThumbHover", "Slider.ThumbPressed":
		return "Slider.Thumb"
	case "Split.DividerHover", "Split.DividerActive", "Split.DividerFocusVisible":
		return "Split.Divider"
	case "Dock.TabHover", "Dock.TabSelected", "Dock.TabFocusVisible":
		return "Dock.Tab"
	case "Button.RootHover", "Button.RootPressed", "Button.RootDisabled", "Button.RootFocusVisible":
		return "Button.Root"
	case "NumberInput.SpinButtonHover", "NumberInput.SpinButtonPressed",
		"NumberInput.SpinButtonFocusVisible", "NumberInput.SpinButtonDisabled":
		return "NumberInput.SpinButton"
	}
	return part
}

// keyNode returns the node built for the element with key.
func keyNode(t *testing.T, rt *Runtime, key string) *node {
	t.Helper()
	for _, n := range rt.byID {
		if n.key == key {
			return n
		}
	}
	t.Fatalf("no node with key %q", key)
	return nil
}

// hoverNode moves the pointer to the center of n.
func hoverNode(r *accessRig, n *node) {
	rc := r.rt.nodeRect(n)
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
}

// pressNode presses the left button on the center of n and keeps it down.
func pressNode(r *accessRig, n *node) {
	rc := r.rt.nodeRect(n)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.frame()
}

// focusNode gives n keyboard focus, as a Tab or an arrow key would.
func focusNode(r *accessRig, n *node) {
	r.rt.setKeyboardModality(true) // as a key press does: it restyles the element that has focus
	r.rt.setFocus(n.id)
	r.frame()
}

// wantBackground fails unless n is painted with background want.
func wantBackground(t *testing.T, n *node, part string, want Color) {
	t.Helper()
	if n.style.Background != want {
		t.Errorf("%s: background %v, want %v", part, n.style.Background, want)
	}
}

// wantTextColor fails unless n has text color want.
func wantTextColor(t *testing.T, n *node, part string, want Color) {
	t.Helper()
	if n.style.TextColor != want {
		t.Errorf("%s: text color %v, want %v", part, n.style.TextColor, want)
	}
}

// ---- the parts: each part's style reaches the node that draws it ----

// TestEveryPartIsApplied sets a distinctive style on each documented part of
// each widget, puts the part's state in effect, and asserts that the style
// reaches the node that draws the part. It keeps docs/styling.md and the code in
// step: a documented part that is not applied fails here.
func TestEveryPartIsApplied(t *testing.T) {
	applied := map[string]bool{}
	t.Run("Tabs underline", func(t *testing.T) { testTabsParts(t, TabsUnderline, applied) })
	t.Run("Tabs segmented", func(t *testing.T) { testTabsParts(t, TabsSegmented, applied) })
	t.Run("Select", func(t *testing.T) { testSelectParts(t, applied) })
	t.Run("Slider", func(t *testing.T) { testSliderParts(t, applied) })
	t.Run("Split", func(t *testing.T) { testSplitParts(t, applied) })
	t.Run("Dock", func(t *testing.T) { testDockParts(t, applied) })
	t.Run("TextInput", func(t *testing.T) { testFieldParts(t, "TextInput", applied) })
	t.Run("TextArea", func(t *testing.T) { testFieldParts(t, "TextArea", applied) })
	t.Run("NumberInput inline", func(t *testing.T) { testNumberInputParts(t, SpinInline, applied) })
	t.Run("NumberInput stacked", func(t *testing.T) { testNumberInputParts(t, SpinStacked, applied) })
	t.Run("Button", func(t *testing.T) { testButtonParts(t, applied) })
	t.Run("Checkbox", func(t *testing.T) { testCheckboxParts(t, applied) })
	t.Run("RadioGroup", func(t *testing.T) { testRadioGroupParts(t, applied) })
	t.Run("Switch", func(t *testing.T) { testSwitchParts(t, applied) })
	t.Run("Collapsible", func(t *testing.T) { testCollapsibleParts(t, applied) })
	t.Run("Tree", func(t *testing.T) { testTreeParts(t, applied) })
	t.Run("Menu", func(t *testing.T) { testMenuParts(t, applied) })
	t.Run("MenuBar", func(t *testing.T) { testMenuBarParts(t, applied) })
	t.Run("Modal", func(t *testing.T) { testModalParts(t, applied) })
	t.Run("Drawer", func(t *testing.T) { testDrawerParts(t, applied) })
	t.Run("Popover", func(t *testing.T) { testPopoverParts(t, applied) })
	t.Run("Progress", func(t *testing.T) { testProgressParts(t, applied) })
	t.Run("Badge", func(t *testing.T) { testBadgeParts(t, applied) })
	t.Run("Toast", func(t *testing.T) { testToastParts(t, applied) })
	t.Run("Alert", func(t *testing.T) { testAlertParts(t, applied) })
	t.Run("Tooltip", func(t *testing.T) { testTooltipParts(t, applied) })
	t.Run("DataGrid", func(t *testing.T) { testDataGridParts(t, applied) })
	t.Run("Forms", func(t *testing.T) { testFormParts(t, applied) })
	t.Run("Combobox", func(t *testing.T) { testComboboxParts(t, applied) })
	t.Run("DatePicker", func(t *testing.T) { testDatePickerParts(t, applied) })
	t.Run("TimePicker", func(t *testing.T) { testTimePickerParts(t, applied) })
	t.Run("Avatar", func(t *testing.T) { testAvatarParts(t, applied) })
	t.Run("Tag", func(t *testing.T) { testTagParts(t, applied) })
	t.Run("Kbd", func(t *testing.T) { testKbdParts(t, applied) })
	t.Run("GroupBox", func(t *testing.T) { testGroupBoxParts(t, applied) })
	t.Run("DescriptionList", func(t *testing.T) { testDescriptionListParts(t, applied) })
	t.Run("Skeleton", func(t *testing.T) { testSkeletonParts(t, applied) })
	t.Run("Sidebar", func(t *testing.T) { testSidebarParts(t, applied) })
	t.Run("Breadcrumbs", func(t *testing.T) { testBreadcrumbsParts(t, applied) })
	t.Run("Pagination", func(t *testing.T) { testPaginationParts(t, applied) })
	t.Run("Stepper", func(t *testing.T) { testStepperParts(t, applied) })
	t.Run("DateRangePicker", func(t *testing.T) { testDateRangePickerParts(t, applied) })
	t.Run("ColorPicker", func(t *testing.T) { testColorPickerParts(t, applied) })

	// The table and docs/styling.md list the same parts, in both directions.
	documented := documentedParts(t)
	for _, p := range documented {
		if !applied[p] {
			t.Errorf("%s is listed in docs/styling.md, but no case applies it", p)
		}
	}
	for p := range applied {
		if !slices.Contains(documented, p) {
			t.Errorf("%s is applied by a case, but docs/styling.md does not list it", p)
		}
	}
}

// documentedParts returns the parts that docs/styling.md lists, as "Widget.Part".
// It reads the widget table of that document: a row whose first column names
// one or more widgets (separated by commas), whose second column names the
// parts in code spans, and whose last column is the status. A row marked
// "planned" is not yet implemented and is skipped; see
// TestDocumentedWidgetsAreDone for the rows that must not be planned.
func documentedParts(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, row := range styleTableRows(t) {
		if strings.HasPrefix(row.status, "planned") {
			continue
		}
		// The names in code spans are the odd fields of the column split by backticks.
		for i, name := range strings.Split(row.parts, "`") {
			if i%2 == 1 {
				for _, widget := range row.widgets {
					if part := widget + "." + name; !slices.Contains(out, part) {
						out = append(out, part)
					}
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no parts found in docs/styling.md")
	}
	return out
}

// styleRow is a row of the widget table of docs/styling.md.
type styleRow struct {
	widgets []string // the widget names of the first column
	parts   string   // the parts column
	status  string   // the status column, the last one
}

// styleTableRows returns the rows of the widget table of docs/styling.md, the
// table under "## Widgets and their parts": the rows that start with a
// code-span widget name and end with a status.
func styleTableRows(t *testing.T) []styleRow {
	t.Helper()
	data, err := os.ReadFile("../docs/styling.md")
	if err != nil {
		t.Fatalf("reading the styling document: %v", err)
	}
	doc := string(data)
	start := strings.Index(doc, "\n## Widgets and their parts\n")
	end := strings.Index(doc, "\n## Examples\n")
	if start < 0 || end < start {
		t.Fatalf("docs/styling.md has no widget table between its sections")
	}
	var rows []styleRow
	for _, line := range strings.Split(doc[start:end], "\n") {
		cols := strings.Split(line, "|")
		if len(cols) < 4 || !strings.HasPrefix(strings.TrimSpace(cols[1]), "`") {
			continue
		}
		var widgets []string
		for _, w := range strings.Split(cols[1], ",") {
			widgets = append(widgets, strings.Trim(strings.TrimSpace(w), "`"))
		}
		rows = append(rows, styleRow{widgets: widgets, parts: cols[2], status: strings.TrimSpace(cols[len(cols)-2])})
	}
	return rows
}

// TestDocumentedWidgetsAreDone checks that docs/styling.md has every widget
// filled in: each widget row lists its parts and none is marked planned. It
// fails while a widget row is still planned.
func TestDocumentedWidgetsAreDone(t *testing.T) {
	rows := styleTableRows(t)
	if len(rows) == 0 {
		t.Fatalf("no widget rows in docs/styling.md")
	}
	for _, row := range rows {
		if strings.HasPrefix(row.status, "planned") {
			t.Errorf("%s is marked planned in docs/styling.md", strings.Join(row.widgets, ", "))
		}
		if !strings.Contains(row.parts, "`") {
			t.Errorf("%s lists no parts in docs/styling.md", strings.Join(row.widgets, ", "))
		}
	}
}

// partTabs is a tab list of three tabs, the middle one disabled, with variant v
// and instance style st.
func partTabs(sel *Signal[string], v TabsVariant, st TabsStyle) Element {
	return Div().Column().Child(Tabs(sel,
		Tab[string]{Value: "a", Label: "Alpha", Icon: icons.Check, Panel: func() Element { return Text("Alpha panel") }},
		Tab[string]{Value: "b", Label: "Beta", Disabled: true, Panel: func() Element { return Text("Beta panel") }},
		Tab[string]{Value: "c", Label: "Gamma", Panel: func() Element { return Text("Gamma panel") }},
	).Key("settings").Label("Settings sections").Variant(v).Styles(st))
}

func testTabsParts(t *testing.T, v TabsVariant, applied map[string]bool) {
	type tabsCase struct {
		part string
		set  func(*TabsStyle, Style)
		key  string                       // the element whose node draws the part, "" for the only one
		act  func(*testing.T, *accessRig) // puts the part's state in effect
	}
	cases := []tabsCase{
		{part: "Tabs.List", set: func(s *TabsStyle, m Style) { s.List = m }},
		{part: "Tabs.Tab", set: func(s *TabsStyle, m Style) { s.Tab = m }, key: "settings/tab:c"},
		{part: "Tabs.TabSelected", set: func(s *TabsStyle, m Style) { s.TabSelected = m }, key: "settings/tab:a"},
		{part: "Tabs.TabDisabled", set: func(s *TabsStyle, m Style) { s.TabDisabled = m }, key: "settings/tab:b"},
		{part: "Tabs.TabHover", set: func(s *TabsStyle, m Style) { s.TabHover = m }, key: "settings/tab:c",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "settings/tab:c")) }},
		{part: "Tabs.TabFocusVisible", set: func(s *TabsStyle, m Style) { s.TabFocusVisible = m }, key: "settings/tab:a",
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "settings/tab:a")) }},
		{part: "Tabs.Panel", set: func(s *TabsStyle, m Style) { s.Panel = m }, key: "settings/panel"},
	}
	if v == TabsUnderline {
		cases = append(cases, tabsCase{part: "Tabs.Indicator", set: func(s *TabsStyle, m Style) { s.Indicator = m }})
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st TabsStyle
			c.set(&st, Style{Background: markBG})
			sel := NewSignal("a")
			r := newAccessRig(func(ctx *Context) Element { return partTabs(sel, v, st) })
			if c.act != nil {
				c.act(t, r)
			}
			if c.part == "Tabs.Indicator" {
				// Only the line under the selected tab is drawn in the part's color.
				marked := 0
				for _, n := range partNodes(r.rt, c.part) {
					if n.style.Background == markBG {
						marked++
					}
				}
				if marked != 1 {
					t.Fatalf("%s: %d lines in the part's color, want the selected tab's only", c.part, marked)
				}
				return
			}
			wantBackground(t, partNode(t, r.rt, drawnBy(c.part), c.key), c.part, markBG)
		})
	}
	// The drop line is drawn while a tab of a reorderable list is dragged over another.
	applied["Tabs.DropLine"] = true
	t.Run("Tabs.DropLine", func(t *testing.T) {
		st := TabsStyle{DropLine: Style{Background: markBG}}
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().Child(Tabs(NewSignal("a"),
				Tab[string]{Value: "a", Label: "Alpha"}, Tab[string]{Value: "c", Label: "Gamma"},
			).Key("settings").Variant(v).Styles(st).Reorderable())
		})
		dragHover(t, r, "settings/tab:a", "settings/tab:c")
		wantBackground(t, partNode(t, r.rt, "Tabs.DropLine", ""), "Tabs.DropLine", markBG)
	})
	if v == TabsSegmented {
		r := newAccessRig(func(ctx *Context) Element { return partTabs(NewSignal("a"), v, TabsStyle{}) })
		if got := len(partNodes(r.rt, "Tabs.Indicator")); got != 0 {
			t.Fatalf("segmented tabs draw %d indicators, want none", got)
		}
	}
}

// partSelect is a select of three options, the middle one disabled, with the
// instance style st, the value sel, and the field disabled or not.
func partSelect(sel *Signal[string], st SelectStyle, disabled bool) Element {
	return Div().Column().Child(Select(sel,
		Option[string]{Value: "a", Label: "Alpha"},
		Option[string]{Value: "b", Label: "Beta", Disabled: true},
		Option[string]{Value: "c", Label: "Gamma"},
	).Key("chan").Label("Channel").Placeholder("Choose").Disabled(disabled).Styles(st))
}

func testSelectParts(t *testing.T, applied map[string]bool) {
	type selectCase struct {
		part     string
		set      func(*SelectStyle, Style)
		value    string // the chosen value, "" for none
		open     bool   // open the list
		disabled bool
		text     bool   // the part is colored by its text color
		row      string // for an option part: the label of the row that draws it
		act      func(*testing.T, *accessRig)
	}
	cases := []selectCase{
		{part: "Select.Trigger", set: func(s *SelectStyle, m Style) { s.Trigger = m }},
		{part: "Select.TriggerHover", set: func(s *SelectStyle, m Style) { s.TriggerHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Select.Trigger", "")) }},
		{part: "Select.TriggerFocusVisible", set: func(s *SelectStyle, m Style) { s.TriggerFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Select.Trigger", "")) }},
		{part: "Select.TriggerDisabled", set: func(s *SelectStyle, m Style) { s.TriggerDisabled = m }, disabled: true},
		{part: "Select.Value", set: func(s *SelectStyle, m Style) { s.Value = m }, value: "a", text: true},
		{part: "Select.Placeholder", set: func(s *SelectStyle, m Style) { s.Placeholder = m }, text: true},
		{part: "Select.Chevron", set: func(s *SelectStyle, m Style) { s.Chevron = m }, text: true},
		{part: "Select.Popup", set: func(s *SelectStyle, m Style) { s.Popup = m }, open: true},
		{part: "Select.Option", set: func(s *SelectStyle, m Style) { s.Option = m }, open: true, row: "Gamma"},
		{part: "Select.OptionHighlighted", set: func(s *SelectStyle, m Style) { s.OptionHighlighted = m }, open: true, row: "Alpha"},
		{part: "Select.OptionSelected", set: func(s *SelectStyle, m Style) { s.OptionSelected = m }, value: "a", open: true, row: "Alpha"},
		{part: "Select.OptionDisabled", set: func(s *SelectStyle, m Style) { s.OptionDisabled = m }, open: true, row: "Beta"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st SelectStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			sel := NewSignal(c.value)
			r := newAccessRig(func(ctx *Context) Element { return partSelect(sel, st, c.disabled) })
			if c.act != nil {
				c.act(t, r)
			}
			if c.open {
				r.clickKey(t, "chan")
			}
			switch {
			case c.row != "":
				// The option rows are list items named by their labels.
				row := accessNode(t, r, access.RoleListItem, c.row)
				wantBackground(t, row, c.part, markBG)
			case c.text:
				wantTextColor(t, partNode(t, r.rt, drawnBy(c.part), ""), c.part, markFG)
			default:
				wantBackground(t, partNode(t, r.rt, drawnBy(c.part), ""), c.part, markBG)
			}
		})
	}
}

// partSplit is a split of two panes with the instance style st.
func partSplit(st SplitStyle) Element {
	return Div().Column().Child(Split(Pane(Div()), Pane(Div())).Styles(st))
}

func testSplitParts(t *testing.T, applied map[string]bool) {
	t.Helper()
	cases := []struct {
		part string
		set  func(*SplitStyle, Style)
		act  func(t *testing.T, r *accessRig)
	}{
		{part: "Split.Divider", set: func(s *SplitStyle, m Style) { s.Divider = m }},
		{part: "Split.DividerHover", set: func(s *SplitStyle, m Style) { s.DividerHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Split.Divider", "")) }},
		{part: "Split.DividerActive", set: func(s *SplitStyle, m Style) { s.DividerActive = m },
			act: func(t *testing.T, r *accessRig) { pressNode(r, partNode(t, r.rt, "Split.Handle", "")) }},
		{part: "Split.DividerFocusVisible", set: func(s *SplitStyle, m Style) { s.DividerFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Split.Handle", "")) }},
		{part: "Split.Grip", set: func(s *SplitStyle, m Style) { s.Grip = m }},
		{part: "Split.Pane", set: func(s *SplitStyle, m Style) { s.Pane = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st SplitStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element { return partSplit(st) })
			if c.act != nil {
				c.act(t, r)
			}
			if c.part == "Split.Pane" {
				wantBackground(t, splitParts(r, "Split.Pane", false)[0], c.part, markBG)
				return
			}
			wantBackground(t, partNode(t, r.rt, drawnBy(c.part), ""), c.part, markBG)
		})
	}
}

// partSlider is a slider with the instance style st.
func partSlider(v *Signal[float64], st SliderStyle) Element {
	return Div().Column().Child(Slider(v, 0, 100, 5).Label("Volume").Key("vol").Styles(st))
}

func testSliderParts(t *testing.T, applied map[string]bool) {
	type sliderCase struct {
		part string
		set  func(*SliderStyle, Style)
		act  func(*testing.T, *accessRig)
		text bool
	}
	cases := []sliderCase{
		{part: "Slider.Track", set: func(s *SliderStyle, m Style) { s.Track = m }},
		{part: "Slider.TrackFocusVisible", set: func(s *SliderStyle, m Style) { s.TrackFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Slider.Track", "")) }},
		{part: "Slider.Rail", set: func(s *SliderStyle, m Style) { s.Rail = m }},
		{part: "Slider.Fill", set: func(s *SliderStyle, m Style) { s.Fill = m }},
		{part: "Slider.Thumb", set: func(s *SliderStyle, m Style) { s.Thumb = m }},
		{part: "Slider.ThumbHover", set: func(s *SliderStyle, m Style) { s.ThumbHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Slider.Thumb", "")) }},
		{part: "Slider.ThumbPressed", set: func(s *SliderStyle, m Style) { s.ThumbPressed = m },
			act: func(t *testing.T, r *accessRig) { pressNode(r, partNode(t, r.rt, "Slider.Track", "")) }},
		{part: "Slider.ValueText", set: func(s *SliderStyle, m Style) { s.ValueText = m }, text: true},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st SliderStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			v := NewSignal(50.0)
			r := newAccessRig(func(ctx *Context) Element { return partSlider(v, st) })
			if c.act != nil {
				c.act(t, r)
			}
			n := partNode(t, r.rt, drawnBy(c.part), "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

// partNumberInput is a number input from 0 to 23 with the instance style st and
// spin buttons of placement p.
func partNumberInput(v *Signal[int], st NumberInputStyle, p SpinPlacement, disabled bool) Element {
	return Div().Column().Child(NumberInput(v, 0, 23, 1).Label("Quiet hours").Key("nf").
		SpinButtons(p).Disabled(disabled).Styles(st))
}

func testNumberInputParts(t *testing.T, p SpinPlacement, applied map[string]bool) {
	type numberCase struct {
		part     string
		set      func(*NumberInputStyle, Style)
		value    int
		disabled bool
		act      func(*testing.T, *accessRig)
	}
	invalid := func(t *testing.T, r *accessRig) {
		focusNode(r, r.keyed(t, "nf"))
		r.typeText("x")
		r.press(input.KeyEnter, 0)
	}
	cases := []numberCase{
		{part: "NumberInput.Frame", set: func(s *NumberInputStyle, m Style) { s.Frame = m }, value: 8},
		{part: "NumberInput.FrameHover", set: func(s *NumberInputStyle, m Style) { s.FrameHover = m }, value: 8,
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "NumberInput.Frame", "")) }},
		{part: "NumberInput.FrameFocusWithin", set: func(s *NumberInputStyle, m Style) { s.FrameFocusWithin = m }, value: 8,
			act: func(t *testing.T, r *accessRig) { focusNode(r, editorNode(t, r.rt)) }},
		{part: "NumberInput.FrameInvalid", set: func(s *NumberInputStyle, m Style) { s.FrameInvalid = m }, value: 8, act: invalid},
		{part: "NumberInput.FrameDisabled", set: func(s *NumberInputStyle, m Style) { s.FrameDisabled = m }, value: 8, disabled: true},
		{part: "NumberInput.Text", set: func(s *NumberInputStyle, m Style) { s.Text = m }, value: 8},
		{part: "NumberInput.Selection", set: func(s *NumberInputStyle, m Style) { s.Selection = m }, value: 8},
		{part: "NumberInput.Message", set: func(s *NumberInputStyle, m Style) { s.Message = m }, value: 8, act: invalid},
		{part: "NumberInput.SpinButtons", set: func(s *NumberInputStyle, m Style) { s.SpinButtons = m }, value: 8},
		{part: "NumberInput.SpinButton", set: func(s *NumberInputStyle, m Style) { s.SpinButton = m }, value: 8},
		{part: "NumberInput.SpinButtonHover", set: func(s *NumberInputStyle, m Style) { s.SpinButtonHover = m }, value: 8,
			act: func(t *testing.T, r *accessRig) { hoverNode(r, accessNode(t, r, access.RoleButton, "Increase")) }},
		{part: "NumberInput.SpinButtonPressed", set: func(s *NumberInputStyle, m Style) { s.SpinButtonPressed = m }, value: 8,
			act: func(t *testing.T, r *accessRig) { pressNode(r, accessNode(t, r, access.RoleButton, "Increase")) }},
		{part: "NumberInput.SpinButtonFocusVisible", set: func(s *NumberInputStyle, m Style) { s.SpinButtonFocusVisible = m }, value: 8,
			act: func(t *testing.T, r *accessRig) { focusNode(r, accessNode(t, r, access.RoleButton, "Increase")) }},
		// At the top of the range the increase button cannot step any further.
		{part: "NumberInput.SpinButtonDisabled", set: func(s *NumberInputStyle, m Style) { s.SpinButtonDisabled = m }, value: 23},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			if c.part == "NumberInput.SpinButtons" && p != SpinStacked {
				// The inline buttons are laid over the frame's edges, with no group (see
				// NumberInputStyle); the group is the stacked column's part.
				t.Skip("the inline buttons have no group")
			}
			var st NumberInputStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			v := NewSignal(c.value)
			r := newAccessRig(func(ctx *Context) Element { return partNumberInput(v, st, p, c.disabled) })
			if c.act != nil {
				c.act(t, r)
			}
			switch c.part {
			case "NumberInput.SpinButton", "NumberInput.SpinButtonHover", "NumberInput.SpinButtonPressed",
				"NumberInput.SpinButtonFocusVisible", "NumberInput.SpinButtonDisabled":
				// Every button that the part applies to is drawn in the part's color.
				// Hover, press and focus pick out the one under the pointer or focus.
				found := 0
				for _, b := range partNodes(r.rt, "NumberInput.SpinButton") {
					if b.style.Background == markBG {
						found++
					}
				}
				if found == 0 {
					t.Errorf("%s: no spin button is drawn in the part's color", c.part)
				}
			case "NumberInput.SpinButtons", "NumberInput.Message":
				wantBackground(t, partNode(t, r.rt, c.part, ""), c.part, markBG)
			case "NumberInput.Text":
				wantTextColor(t, editorNode(t, r.rt), c.part, markFG)
			case "NumberInput.Selection":
				if got := editorNode(t, r.rt).editor.selection; got != markBG {
					t.Errorf("%s: selection color %v, want %v", c.part, got, markBG)
				}
			default:
				wantBackground(t, partNode(t, r.rt, drawnBy(c.part), ""), c.part, markBG)
			}
		})
	}
}

// ---- precedence: instance over theme over defaults ----

// TestPartStylePrecedence checks that an instance's Styles win over the theme's
// style for a widget, which wins over the built-in default. Each probe reads the
// background of one part, built with no override, with the theme's, and with
// both.
func TestPartStylePrecedence(t *testing.T) {
	themeBG, instBG := Hex("#111213"), Hex("#141516")
	type probe struct {
		name     string
		part     string
		key      string
		def      Color                              // the built-in background
		build    func(th *Theme, inst bool) Element // the widget, with the instance's override when inst
		setTheme func(th *Theme, c Color)           // sets the theme's override
	}
	probes := []probe{
		{name: "Tabs", part: "Tabs.Tab", key: "settings/tab:c", def: Color{},
			build: func(th *Theme, inst bool) Element {
				var st TabsStyle
				if inst {
					st.Tab = Style{Background: instBG}
				}
				return partTabs(NewSignal("a"), TabsUnderline, st)
			},
			setTheme: func(th *Theme, c Color) { th.Tabs.Tab.Background = c }},
		{name: "Select", part: "Select.Trigger", def: selectDefaults(DarkTheme(), 220, false, sizeDefault).Trigger.Background,
			build: func(th *Theme, inst bool) Element {
				var st SelectStyle
				if inst {
					st.Trigger = Style{Background: instBG}
				}
				return partSelect(NewSignal(""), st, false)
			},
			setTheme: func(th *Theme, c Color) { th.Select.Trigger.Background = c }},
		{name: "Slider", part: "Slider.Thumb", def: DarkTheme().Accent,
			build: func(th *Theme, inst bool) Element {
				var st SliderStyle
				if inst {
					st.Thumb = Style{Background: instBG}
				}
				return partSlider(NewSignal(50.0), st)
			},
			setTheme: func(th *Theme, c Color) { th.Slider.Thumb.Background = c }},
		{name: "NumberInput", part: "NumberInput.Frame", def: DarkTheme().Surface,
			build: func(th *Theme, inst bool) Element {
				var st NumberInputStyle
				if inst {
					st.Frame = Style{Background: instBG}
				}
				return partNumberInput(NewSignal(8), st, SpinInline, false)
			},
			setTheme: func(th *Theme, c Color) { th.NumberInput.Frame.Background = c }},
		{name: "TextInput", part: "TextInput.Frame", def: DarkTheme().Surface,
			build: func(th *Theme, inst bool) Element {
				var st FieldStyle
				if inst {
					st.Frame = Style{Background: instBG}
				}
				return partTextInput(NewSignal(""), st)
			},
			setTheme: func(th *Theme, c Color) { th.Field.Frame.Background = c }},
	}
	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			bgOf := func(th *Theme, inst bool) Color {
				r := newThemedRig(th, func(ctx *Context) Element { return p.build(th, inst) })
				return partNode(t, r.rt, p.part, p.key).style.Background
			}
			plain := *DarkTheme()
			withTheme := *DarkTheme()
			p.setTheme(&withTheme, themeBG)
			if got := bgOf(&plain, false); got != p.def {
				t.Errorf("no override: background %v, want the default %v", got, p.def)
			}
			if got := bgOf(&withTheme, false); got != themeBG {
				t.Errorf("theme override: background %v, want the theme's %v", got, themeBG)
			}
			if got := bgOf(&withTheme, true); got != instBG {
				t.Errorf("instance over theme: background %v, want the instance's %v", got, instBG)
			}
		})
	}
}

// ---- high contrast ----

// TestPartOverridesUnderHighContrast checks the guardrails: under high contrast
// the colors that an app sets for a part are not used, but its sizes and
// spacing are, and the built-in focus ring and disabled colors remain.
func TestPartOverridesUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Tabs = TabsStyle{Tab: Style{Background: markBG, TextColor: markFG, Padding: All(7)}}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return partTabs(NewSignal("a"), TabsSegmented, TabsStyle{Tab: Style{Background: markBG, TextColor: markFG, Padding: All(9)}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	tab := keyNode(t, r.rt, "settings/tab:c")
	if tab.style.Background == markBG || tab.style.TextColor == markFG {
		t.Errorf("high contrast: an app color reached the tab: background %v, text %v", tab.style.Background, tab.style.TextColor)
	}
	if tab.style.Padding != All(9) {
		t.Errorf("high contrast: the instance's padding %v did not apply, got %v", All(9), tab.style.Padding)
	}
	// The selected tab is the system Highlight, and the keyboard focus ring stays.
	sel := keyNode(t, r.rt, "settings/tab:a")
	if sel.style.Background != th.Accent {
		t.Errorf("high contrast: the selected tab is %v, want Highlight %v", sel.style.Background, th.Accent)
	}
	focusNode(r, sel)
	if len(sel.style.Shadows) == 0 {
		t.Errorf("high contrast: the focused tab has no focus ring")
	}
	// The disabled tab is GrayText.
	dis := keyNode(t, r.rt, "settings/tab:b")
	if got := effectiveTextColor(dis); got != th.Disabled {
		t.Errorf("high contrast: the disabled tab's text is %v, want GrayText %v", got, th.Disabled)
	}
}

// ---- variants ----

func TestTabsSegmentedLook(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return partTabs(NewSignal("a"), TabsSegmented, TabsStyle{}) })
	th := r.rt.theme
	sel := keyNode(t, r.rt, "settings/tab:a")
	if sel.style.Background != th.Accent {
		t.Errorf("segmented: the selected segment is %v, want Accent %v", sel.style.Background, th.Accent)
	}
	if got := len(partNodes(r.rt, "Tabs.Indicator")); got != 0 {
		t.Errorf("segmented: %d indicators, want none", got)
	}
	// The unselected segment is not filled, and a hover fills it.
	gamma := keyNode(t, r.rt, "settings/tab:c")
	if gamma.style.Background == th.Accent {
		t.Errorf("segmented: an unselected segment is filled with Accent")
	}
	hoverNode(r, gamma)
	if gamma.style.Background != th.SurfaceHover {
		t.Errorf("segmented: hovered segment is %v, want SurfaceHover %v", gamma.style.Background, th.SurfaceHover)
	}
}

func TestTabsUnderlineDrawsOneLinePerTab(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return partTabs(NewSignal("a"), TabsUnderline, TabsStyle{}) })
	th := r.rt.theme
	lines := partNodes(r.rt, "Tabs.Indicator")
	if len(lines) != 3 {
		t.Fatalf("underline: %d indicators, want one per tab (3)", len(lines))
	}
	lit := 0
	for _, n := range lines {
		if n.style.Background == th.Accent {
			lit++
		}
	}
	if lit != 1 {
		t.Errorf("underline: %d lines in Accent, want the selected tab's only", lit)
	}
}

func TestNumberInputSpinButtonsByPlacement(t *testing.T) {
	// The stacked buttons are a column in the frame's flow that fills the frame
	// inside its border, so the field is 34 px high like the other fields. The two
	// buttons share it, with the gap at the column's top and bottom and twice the
	// gap between them.
	r := newAccessRig(func(ctx *Context) Element {
		return partNumberInput(NewSignal(8), NumberInputStyle{}, SpinStacked, false)
	})
	group := partNode(t, r.rt, "NumberInput.SpinButtons", "")
	if group.style.Direction != Column {
		t.Errorf("stacked: group direction %v, want Column", group.style.Direction)
	}
	if h := partNode(t, r.rt, "NumberInput.Frame", "").lay.H; h != 34 {
		t.Errorf("stacked: field height %v, want 34", h)
	}
	if h := buttonNode(t, r, "Increase").lay.H; h != 16-2*spinGap {
		t.Errorf("stacked: Increase height %v, want %v", h, 16-2*spinGap)
	}
	// The inline buttons have no group: each is laid over an edge of the frame.
	r = newAccessRig(func(ctx *Context) Element {
		return partNumberInput(NewSignal(8), NumberInputStyle{}, SpinInline, false)
	})
	if n := len(partNodes(r.rt, "NumberInput.SpinButtons")); n != 0 {
		t.Errorf("inline: %d spin button groups, want none", n)
	}
	for _, name := range []string{"Decrease", "Increase"} {
		if b := buttonNode(t, r, name); !b.style.Absolute {
			t.Errorf("inline: %s is not laid over its frame's edge", name)
		}
	}
}

// buttonNode returns the node of the button with the accessible name.
func buttonNode(t *testing.T, r *accessRig, name string) *node {
	t.Helper()
	return accessNode(t, r, access.RoleButton, name)
}

// ---- render slots ----

func TestTabsRenderTabKeepsBehaviorAndName(t *testing.T) {
	sel := NewSignal("a")
	counts := map[string]int{"a": 3, "b": 0, "c": 12}
	var calls []PartState
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Tabs(sel,
			Tab[string]{Value: "a", Label: "Alpha, 3 items"},
			Tab[string]{Value: "b", Label: "Beta, 0 items"},
			Tab[string]{Value: "c", Label: "Gamma, 12 items"},
		).Key("settings").Label("Settings sections").RenderTab(func(tb Tab[string], s PartState) Element {
			if tb.Value == "c" {
				calls = append(calls, s)
			}
			return Text(fmt.Sprintf("%s %d", tb.Value, counts[tb.Value]))
		}))
	})
	// The slot's content is drawn in the tab.
	if n := keyNode(t, r.rt, "settings/tab:c"); len(n.children) == 0 {
		t.Fatalf("the tab has no children")
	}
	found := false
	for _, n := range r.rt.byID {
		if n.kind == kindText && n.textEl != nil && n.textEl.text == "c 12" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the slot's content c 12 is not drawn")
	}
	// The tab keeps its name, and the content is not read as part of it.
	accessNode(t, r, access.RoleTab, "Gamma, 12 items")
	r.tree().Walk(func(n *access.Node) bool {
		if strings.Contains(n.Name, "c 12") {
			t.Errorf("the slot's content is in the access tree as %q", n.Name)
		}
		return true
	})
	// Selection by the keyboard works as before.
	r.focusKey(t, "settings/tab:a")
	r.press(input.KeyRight, 0)
	if sel.Peek() != "b" {
		t.Fatalf("Right selected %q, want b", sel.Peek())
	}
	// Hovering the tab re-renders the slot with the hovered state.
	before := len(calls)
	hoverNode(r, keyNode(t, r.rt, "settings/tab:c"))
	if len(calls) == before {
		t.Fatalf("the slot was not called again on hover")
	}
	if !calls[len(calls)-1].Hovered {
		t.Errorf("the slot's last state has Hovered false, want true")
	}
}

func TestSelectRenderOptionAndValue(t *testing.T) {
	sel := NewSignal("a")
	var valueCalls, optionCalls int
	var highlighted bool
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Select(sel,
			Option[string]{Value: "a", Label: "Alpha"},
			Option[string]{Value: "b", Label: "Beta"},
		).Key("chan").Label("Channel").
			RenderValue(func(o Option[string], s PartState) Element {
				valueCalls++
				return Text("value " + o.Label)
			}).
			RenderOption(func(o Option[string], s PartState) Element {
				optionCalls++
				if o.Value == "b" && s.Focused {
					highlighted = true
				}
				return Text("row " + o.Label)
			}))
	})
	if valueCalls == 0 {
		t.Errorf("RenderValue was not called for the chosen option")
	}
	// The field keeps its value, and each row keeps its name.
	if v := r.stateOf(t, access.RoleComboBox, "Channel").Value; v != "Alpha" {
		t.Errorf("the field's value is %q, want Alpha", v)
	}
	r.clickKey(t, "chan")
	accessNode(t, r, access.RoleListItem, "Alpha")
	accessNode(t, r, access.RoleListItem, "Beta")
	// The highlight moves with the keys, and the slot is told the row is highlighted.
	r.press(input.KeyDown, 0)
	if !highlighted {
		t.Errorf("RenderOption was not called with Focused for the highlighted row")
	}
	r.press(input.KeyEnter, 0)
	if sel.Peek() != "b" {
		t.Fatalf("Enter chose %q, want b", sel.Peek())
	}
	if optionCalls == 0 {
		t.Errorf("RenderOption was not called")
	}
}

func TestSelectRenderValueIsNotCalledWithoutAChoice(t *testing.T) {
	calls := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Select(NewSignal(""),
			Option[string]{Value: "a", Label: "Alpha"},
		).Key("chan").Label("Channel").Placeholder("Choose").
			RenderValue(func(o Option[string], s PartState) Element {
				calls++
				return Text("value")
			}))
	})
	if calls != 0 {
		t.Errorf("RenderValue was called %d times with no option chosen", calls)
	}
	found := false
	for _, n := range r.rt.byID {
		if n.kind == kindText && n.textEl != nil && n.textEl.text == "Choose" {
			found = true
		}
	}
	if !found {
		t.Errorf("the placeholder is not drawn when no option is chosen")
	}
}

// ---- keys ----

// TestTabKeysAreDisjointFromPanelKeys checks the collision fix: a tab whose value
// formats as "panel" must not share a key with the default panel.
func TestTabKeysAreDisjointFromPanelKeys(t *testing.T) {
	sel := NewSignal("panel")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Tabs(sel,
			Tab[string]{Value: "panel", Label: "Panel tab", Panel: func() Element { return Text("the panel") }},
			Tab[string]{Value: "x", Label: "Other", Panel: func() Element { return Text("other") }},
		).Key("settings").Label("Settings sections"))
	})
	tabNode := keyNode(t, r.rt, "settings/tab:panel")
	if tabNode.div.access().role != access.RoleTab {
		t.Errorf("settings/tab:panel is not the tab")
	}
	panelNode := keyNode(t, r.rt, "settings/panel")
	if panelNode.div.access().role != access.RoleTabPanel {
		t.Errorf("settings/panel is not the panel")
	}
	count := 0
	for _, n := range r.rt.byID {
		if n.key == "settings/panel" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%d nodes have the key settings/panel, want 1", count)
	}
}

// TestPartsFollowTextScale checks that the text of the tabs, the select and the
// slider readout grows with the Text size setting, as the other text does.
func TestPartsFollowTextScale(t *testing.T) {
	heights := func(scale int) map[string]float32 {
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().Gap(12).
				Child(partTabs(NewSignal("a"), TabsUnderline, TabsStyle{})).
				Child(partSelect(NewSignal("a"), SelectStyle{}, false)).
				Child(partSlider(NewSignal(50.0), SliderStyle{}))
		})
		if scale != 100 {
			r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
			r.frame()
		}
		out := map[string]float32{}
		for _, n := range r.rt.byID {
			if n.kind == kindText && n.textEl != nil {
				switch n.textEl.text {
				case "Gamma", "Alpha", "50":
					out[n.textEl.text] = n.lay.H
				}
			}
		}
		return out
	}
	base, big := heights(100), heights(225)
	for _, label := range []string{"Gamma", "Alpha", "50"} {
		if big[label] <= base[label] {
			t.Errorf("%q is %v px tall at 225%%, %v px at 100%%: it does not grow", label, big[label], base[label])
		}
	}
}

// TestTabsRequestNoFramesUnderReducedMotion checks that a tab's line eases in
// only when reduced motion is off: under the setting the change is instant.
func TestTabsRequestNoFramesUnderReducedMotion(t *testing.T) {
	for _, reduced := range []bool{true, false} {
		sel := NewSignal("a")
		r := newAccessRig(func(ctx *Context) Element { return partTabs(sel, TabsUnderline, TabsStyle{}) })
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()
		r.clickKey(t, "settings/tab:c")
		eased := r.rt.NeedsFrame()
		if reduced && eased {
			t.Fatalf("under reduced motion the tab change asks for frames")
		}
		if !reduced && !eased {
			t.Fatalf("without reduced motion the tab change asks for no frames; the line must ease")
		}
	}
}
