package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
)

// layerCtx returns a context of a window with the theme th and no inspector, for
// resolving a widget style in a test.
func layerCtx(th *Theme) *Context { return &Context{rt: &Runtime{theme: th}} }

// setEveryPart sets each Style field of the struct v (settable), and of the structs
// it holds, to s.
func setEveryPart(v reflect.Value, s Style) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch {
		case f.Type() == styleType:
			f.Set(reflect.ValueOf(s))
		case f.Kind() == reflect.Struct:
			setEveryPart(f, s)
		}
	}
}

// layerTheme and layerInstance are the values that the table sets in the theme and
// in the instance Styles of every part. A part shows the fields that both set, with
// the instance as the winner and the theme's value overridden below it.
var (
	layerTheme = Style{Radius: 6, Gap: 6, Padding: All(6), BorderWidth: 6, FontSize: 6,
		Background: Hex("#060606"), TextColor: Hex("#060606")}
	layerInstance = Style{Radius: 2, Gap: 2, Padding: All(2), BorderWidth: 2, FontSize: 2,
		Background: Hex("#020202"), TextColor: Hex("#020202")}
)

// layerSetByMethod lists the fields that an element sets itself, after its layers,
// with the reason. Such a field shows the element method as its winner.
var layerSetByMethod = map[string]string{
	"Avatar.Avatar Radius":          "the avatar is a circle: the element sets its radius",
	"DataGrid.Grid Padding":         "the grid sets the padding of its frame",
	"Modal.Scrim Padding":           "the scrim is a full-window layer, padded by the element",
	"Drawer.Scrim Padding":          "the scrim is a full-window layer, padded by the element",
	"AlertDialog.Scrim Padding":     "the scrim is a full-window layer, padded by the element",
	"FormField.Label Gap":           "the label's gap is set by the field, not by the layers",
	"Tabs.Indicator Background":     "the line of an unselected tab is transparent, set by the element",
	"Dock.Indicator Background":     "the line of an unselected tab is transparent, set by the element",
	"ColorPicker.Swatch Background": "the swatch is painted with the chosen color, set by the element",
}

// layerDocAlias maps the names of the parts table's widgets to the prefix of their
// parts in the code.
var layerDocAlias = map[string]string{"CheckboxTri": "Checkbox", "Accordion": "Collapsible",
	"DropdownMenu": "Menu", "ContextMenu": "Menu"}

// layerStateSuffixes are the suffixes of the parts that are states of another part,
// as docs/styling.md lists them (such as Hover, or FocusVisible). A state part is
// not drawn by an element of its own: its style is the state layer of its base part.
var layerStateSuffixes = []string{"Hover", "Pressed", "FocusVisible", "FocusWithin", "Focused", "Selected",
	"Disabled", "Highlighted", "Invalid", "Checked", "Mixed", "Open", "ReadOnly", "Sorted", "Alternate",
	"Editing", "InRange", "Today", "OutsideMonth", "Active", "Current", "Complete", "Filled", "Loading"}

// isStatePart reports whether the part p is the state of a documented base part.
func isStatePart(p string, documented []string) bool {
	for _, s := range layerStateSuffixes {
		if base, ok := strings.CutSuffix(p, s); ok && slices.Contains(documented, base) {
			return true
		}
	}
	return false
}

// layerBuilder draws one widget with the instance Styles st, a value of the
// widget's style struct. theme gives the theme's style struct of the widget, as a
// pointer, so the test can set its parts. The widget is named by its part prefix,
// as in docs/styling.md ("Checkbox" for "Checkbox.Box").
type layerBuilder struct {
	widget string
	theme  func(*Theme) any
	build  func(st any) Element
}

// layerBuilders draw the widgets of the parts table in their default states, and in
// the states that a single element can show (a placeholder, a stacked spin button,
// a dot, a removable tag, a long page run). A part that only an open popup, a menu
// or a toast draws is listed in layerNotDrawn instead.
var layerBuilders = []layerBuilder{
	{"Button", func(th *Theme) any { return &th.Button },
		func(st any) Element { return partButton(ButtonPrimary, st.(ButtonStyle), false) }},
	{"Button", func(th *Theme) any { return &th.Button },
		func(st any) Element { return partButton(ButtonPrimary, st.(ButtonStyle), true) }},
	{"SplitButton", func(th *Theme) any { return &th.SplitButton },
		func(st any) Element { return partSplitButton(st.(SplitButtonStyle), false) }},
	{"SplitButton", func(th *Theme) any { return &th.SplitButton },
		func(st any) Element { return partSplitButton(st.(SplitButtonStyle), true) }},
	{"Checkbox", func(th *Theme) any { return &th.Checkbox },
		func(st any) Element { return partCheckbox(false, CheckOn, st.(CheckboxStyle), false) }},
	{"Checkbox", func(th *Theme) any { return &th.Checkbox },
		func(st any) Element { return partCheckbox(true, CheckMixed, st.(CheckboxStyle), false) }},
	{"Checkbox", func(th *Theme) any { return &th.Checkbox },
		func(st any) Element { return partCheckbox(false, CheckOff, st.(CheckboxStyle), true) }},
	{"RadioGroup", func(th *Theme) any { return &th.RadioGroup },
		func(st any) Element { return partRadio(true, st.(RadioGroupStyle), false) }},
	{"RadioGroup", func(th *Theme) any { return &th.RadioGroup },
		func(st any) Element { return partRadio(true, st.(RadioGroupStyle), true) }},
	{"Collapsible", func(th *Theme) any { return &th.Collapsible },
		func(st any) Element { return partCollapsible(false, true, st.(CollapsibleStyle), true, nil) }},
	{"Switch", func(th *Theme) any { return &th.Switch },
		func(st any) Element { return partSwitch(true, st.(SwitchStyle), false) }},
	{"Switch", func(th *Theme) any { return &th.Switch },
		func(st any) Element { return partSwitch(false, st.(SwitchStyle), true) }},
	{"Toggle", func(th *Theme) any { return &th.Toggle },
		func(st any) Element { return partToggle(true, st.(ToggleStyle), false) }},
	{"Toggle", func(th *Theme) any { return &th.Toggle },
		func(st any) Element { return partToggle(false, st.(ToggleStyle), true) }},
	{"ToggleGroup", func(th *Theme) any { return &th.ToggleGroup },
		func(st any) Element { return partToggleGroup(st.(ToggleGroupStyle), false) }},
	{"Rating", func(th *Theme) any { return &th.Rating },
		func(st any) Element { return partRating(st.(RatingStyle), false) }},
	{"Rating", func(th *Theme) any { return &th.Rating },
		func(st any) Element { return partRating(st.(RatingStyle), true) }},
	{"Collapsible", func(th *Theme) any { return &th.Collapsible },
		func(st any) Element { return partCollapsible(false, true, st.(CollapsibleStyle), false, nil) }},
	{"Tree", func(th *Theme) any { return &th.Tree },
		func(st any) Element { return partTree(NewSignal("docs"), st.(TreeStyle), nil, nil) }},
	{"ListBox", func(th *Theme) any { return &th.ListBox },
		func(st any) Element {
			return listBoxDemo(NewSignal([]string{"c"}), SelectionSingle, st.(ListBoxStyle), nil, nil, nil)
		}},
	{"Tabs", func(th *Theme) any { return &th.Tabs },
		func(st any) Element { return partTabs(NewSignal("a"), TabsUnderline, st.(TabsStyle)) }},
	{"Tabs", func(th *Theme) any { return &th.Tabs },
		func(st any) Element { return partTabs(NewSignal("a"), TabsSegmented, st.(TabsStyle)) }},
	{"Select", func(th *Theme) any { return &th.Select },
		func(st any) Element { return partSelect(NewSignal("a"), st.(SelectStyle), false) }},
	{"Select", func(th *Theme) any { return &th.Select },
		func(st any) Element { return partSelect(NewSignal("a"), st.(SelectStyle), true) }},
	{"Slider", func(th *Theme) any { return &th.Slider },
		func(st any) Element { return partSlider(NewSignal(0.5), st.(SliderStyle)) }},
	{"RangeSlider", func(th *Theme) any { return &th.RangeSlider },
		func(st any) Element { return partRangeSlider(st.(RangeSliderStyle)) }},
	{"Split", func(th *Theme) any { return &th.Split },
		func(st any) Element { // the grip is shown, so the layer table draws it while idle
			return Div().Column().Child(Split(Pane(Div()), Pane(Div())).Grip(SplitGripShown).Styles(st.(SplitStyle)))
		}},
	{"Dock", func(th *Theme) any { return &th.Dock },
		func(st any) Element {
			layout := DockLayout{Root: DockNode{Sizes: []float64{0.5, 0.5}, Children: []DockNode{
				{Panels: []string{"notes", "log"}, Active: "notes"},
				{Panels: []string{"tree"}},
			}}}
			return Div().Column().Size(Full, Full).Child(Dock(NewSignal(layout), testDockPanels()).Styles(st.(DockStyle)))
		}},
	{"NumberInput", func(th *Theme) any { return &th.NumberInput },
		func(st any) Element { return partNumberInput(NewSignal(3), st.(NumberInputStyle), SpinInline, false) }},
	{"NumberInput", func(th *Theme) any { return &th.NumberInput },
		func(st any) Element { return partNumberInput(NewSignal(3), st.(NumberInputStyle), SpinInline, true) }},
	{"NumberInput", func(th *Theme) any { return &th.NumberInput },
		func(st any) Element { return partNumberInput(NewSignal(3), st.(NumberInputStyle), SpinStacked, true) }},
	{"DatePicker", func(th *Theme) any { return &th.DatePicker },
		func(st any) Element {
			return DatePicker(NewSignal(time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))).Disabled(true).Styles(st.(DatePickerStyle))
		}},
	{"TimePicker", func(th *Theme) any { return &th.TimePicker },
		func(st any) Element {
			return TimePicker(NewSignal(time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC))).Disabled(true).Styles(st.(TimePickerStyle))
		}},
	{"DateRangePicker", func(th *Theme) any { return &th.DateRangePicker },
		func(st any) Element {
			day := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
			return DateRangePicker(NewSignal(day), NewSignal(day.AddDate(0, 0, 2))).Disabled(true).Styles(st.(DateRangePickerStyle))
		}},
	{"NumberInput", func(th *Theme) any { return &th.NumberInput },
		func(st any) Element { return partNumberInput(NewSignal(3), st.(NumberInputStyle), SpinStacked, false) }},
	{"TextInput", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextInput", NewSignal(""), st.(FieldStyle), false, "") }},
	{"TextInput", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextInput", NewSignal(""), st.(FieldStyle), false, "disabled") }},
	{"TextInput", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextInput", NewSignal(""), st.(FieldStyle), true, "") }},
	{"TextInput", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextInput", NewSignal("x"), st.(FieldStyle), false, "buttons") }},
	{"TextArea", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextArea", NewSignal(""), st.(FieldStyle), false, "") }},
	{"TextArea", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextArea", NewSignal(""), st.(FieldStyle), false, "disabled") }},
	{"TextArea", func(th *Theme) any { return &th.Field },
		func(st any) Element { return partField("TextArea", NewSignal(""), st.(FieldStyle), true, "") }},
	{"Combobox", func(th *Theme) any { return &th.Combobox },
		func(st any) Element { return partCombobox(NewSignal(""), st.(ComboboxStyle)) }},
	{"ColorPicker", func(th *Theme) any { return &th.ColorPicker },
		func(st any) Element { return ColorPicker(NewSignal(Hex("#112233"))).Styles(st.(ColorPickerStyle)) }},
	{"ColorPicker", func(th *Theme) any { return &th.ColorPicker },
		func(st any) Element {
			return ColorPicker(NewSignal(Hex("#112233"))).Disabled(true).Styles(st.(ColorPickerStyle))
		}},
	{"DatePicker", func(th *Theme) any { return &th.DatePicker },
		func(st any) Element {
			return DatePicker(NewSignal(time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))).Styles(st.(DatePickerStyle))
		}},
	{"TimePicker", func(th *Theme) any { return &th.TimePicker },
		func(st any) Element {
			return TimePicker(NewSignal(time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC))).Styles(st.(TimePickerStyle))
		}},
	{"DateRangePicker", func(th *Theme) any { return &th.DateRangePicker },
		func(st any) Element {
			day := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
			return DateRangePicker(NewSignal(day), NewSignal(day.AddDate(0, 0, 2))).Styles(st.(DateRangePickerStyle))
		}},
	{"Calendar", func(th *Theme) any { return &th.Calendar },
		func(st any) Element {
			// The rig's today is January 12, 1970: the value is the day after, and the
			// Sundays are disabled.
			v := dayOf(time.Unix(1_000_000, 0)).AddDate(0, 0, 1)
			return Calendar(NewSignal(v)).DisableDays(func(d time.Time) bool { return d.Weekday() == time.Sunday }).
				Styles(st.(CalendarStyle))
		}},
	{"Calendar", func(th *Theme) any { return &th.Calendar },
		func(st any) Element {
			return Calendar(NewSignal(time.Time{})).Disabled(true).Styles(st.(CalendarStyle))
		}},
	{"Calendar", func(th *Theme) any { return &th.Calendar },
		func(st any) Element {
			v := dayOf(time.Unix(1_000_000, 0))
			return CalendarRange(NewSignal(v), NewSignal(v.AddDate(0, 0, 3))).Styles(st.(CalendarStyle))
		}},
	{"Carousel", func(th *Theme) any { return &th.Carousel },
		func(st any) Element { return partCarousel(st.(CarouselStyle), false) }},
	{"Carousel", func(th *Theme) any { return &th.Carousel },
		func(st any) Element { return partCarousel(st.(CarouselStyle), true) }},
	{"DataGrid", func(th *Theme) any { return &th.DataGrid },
		func(st any) Element {
			return Component(partDataGrid(3, st.(DataGridStyle), NewSignal([]string{}), NewSignal(SortState{})))
		}},
	{"Modal", func(th *Theme) any { return &th.Modal },
		func(st any) Element { return partModal(st.(ModalStyle)) }},
	{"AlertDialog", func(th *Theme) any { return &th.AlertDialog },
		func(st any) Element { return partAlertDialog(st.(AlertDialogStyle)) }},
	{"Drawer", func(th *Theme) any { return &th.Drawer },
		func(st any) Element {
			closed := false
			return partDrawer(DrawerLeft, st.(DrawerStyle), &closed)
		}},
	{"CommandPalette", func(th *Theme) any { return &th.CommandPalette },
		func(st any) Element {
			return CommandPalette(NewSignal(true)).Styles(st.(CommandPaletteStyle)).
				Commands(func() []*Action { return []*Action{NewAction("test.open", "Open file")} })
		}},
	{"CommandPalette", func(th *Theme) any { return &th.CommandPalette },
		func(st any) Element { return CommandPalette(NewSignal(true)).Styles(st.(CommandPaletteStyle)) }},
	{"Sidebar", func(th *Theme) any { return &th.Sidebar },
		func(st any) Element { return partSidebar(st.(SidebarStyle)) }},
	{"Breadcrumbs", func(th *Theme) any { return &th.Breadcrumbs },
		func(st any) Element {
			link := func() {}
			return Breadcrumbs(Crumb{Label: "Home", OnClick: link}, Crumb{Label: "Records", OnClick: link}).Styles(st.(BreadcrumbsStyle))
		}},
	{"Pagination", func(th *Theme) any { return &th.Pagination },
		func(st any) Element { return Pagination(NewSignal(10), 20).Styles(st.(PaginationStyle)) }},
	{"Stepper", func(th *Theme) any { return &th.Stepper },
		func(st any) Element { return Stepper(NewSignal(1), checkoutSteps...).Styles(st.(StepperStyle)) }},
	{"Stepper", func(th *Theme) any { return &th.Stepper },
		func(st any) Element {
			return Stepper(NewSignal(1), checkoutSteps...).OnSelect(func(int) {}).Styles(st.(StepperStyle))
		}},
	{"Stepper", func(th *Theme) any { return &th.Stepper },
		func(st any) Element {
			return Stepper(NewSignal(1), checkoutSteps...).OnSelect(func(int) {}).Disabled(true).Styles(st.(StepperStyle))
		}},
	{"Stepper", func(th *Theme) any { return &th.Stepper },
		func(st any) Element {
			return Stepper(NewSignal(1), checkoutSteps...).Vertical().Styles(st.(StepperStyle))
		}},
	{"OTPInput", func(th *Theme) any { return &th.OTPInput },
		func(st any) Element {
			return OTPInput("Code", NewSignal("12"), 4).Groups(2, 2).Styles(st.(OTPInputStyle))
		}},
	{"OTPInput", func(th *Theme) any { return &th.OTPInput },
		func(st any) Element {
			return OTPInput("Code", NewSignal("12"), 4).Masked().Disabled(true).Styles(st.(OTPInputStyle))
		}},
	{"ProgressBar", func(th *Theme) any { return &th.ProgressBar },
		func(st any) Element { return ProgressBar(0.4).Styles(st.(ProgressBarStyle)) }},
	{"ProgressRing", func(th *Theme) any { return &th.ProgressRing },
		func(st any) Element { return ProgressRing().Styles(st.(ProgressRingStyle)) }},
	{"Badge", func(th *Theme) any { return &th.Badge },
		func(st any) Element { return Badge(3).Styles(st.(BadgeStyle)) }},
	{"Badge", func(th *Theme) any { return &th.Badge },
		func(st any) Element { return Badge(3).StatusDot().Styles(st.(BadgeStyle)) }},
	{"Alert", func(th *Theme) any { return &th.Alert },
		func(st any) Element {
			return Alert("Sample").Description("Details").OnClose(func() {}).Styles(st.(AlertStyle))
		}},
	{"Avatar", func(th *Theme) any { return &th.Avatar },
		func(st any) Element { return Avatar("Sample User").Styles(st.(AvatarStyle)) }},
	{"Tag", func(th *Theme) any { return &th.Tag },
		func(st any) Element { return Tag("Sample").Styles(st.(TagStyle)) }},
	{"Tag", func(th *Theme) any { return &th.Tag },
		func(st any) Element { return Tag("Sample").OnRemove(func() {}).Styles(st.(TagStyle)) }},
	{"GroupBox", func(th *Theme) any { return &th.GroupBox },
		func(st any) Element {
			return GroupBox("Sample").Description("Details").Child(Text("Body")).Styles(st.(GroupBoxStyle))
		}},
	{"StatusBar", func(th *Theme) any { return &th.StatusBar },
		func(st any) Element {
			return StatusBar().Start(Text("Ready")).End(Text("Ln 1")).Styles(st.(StatusBarStyle))
		}},
	{"Toolbar", func(th *Theme) any { return &th.Toolbar },
		func(st any) Element {
			return Toolbar("Tools").Child(Button("A")).Child(ToolbarSeparator()).Child(Button("B")).Styles(st.(ToolbarStyle))
		}},
	{"Empty", func(th *Theme) any { return &th.Empty },
		func(st any) Element {
			return Empty("No projects").Description("Create one.").Action(Button("New project")).Styles(st.(EmptyStyle))
		}},
	{"DescriptionList", func(th *Theme) any { return &th.DescriptionList },
		func(st any) Element {
			return DescriptionList().ItemText("Owner", "Sample").Styles(st.(DescriptionListStyle))
		}},
	{"Kbd", func(th *Theme) any { return &th.Kbd },
		func(st any) Element { return Kbd("ctrl+k ctrl+t").Styles(st.(KbdStyle)) }},
	{"Skeleton", func(th *Theme) any { return &th.Skeleton },
		func(st any) Element { return Skeleton().Styles(st.(SkeletonStyle)) }},
	{"FormField", func(th *Theme) any { return &th.FormField },
		func(st any) Element {
			return Field("Name", TextInput(NewSignal("")).Key("name")).Styles(st.(FormFieldStyle))
		}},
}

// layerNotDrawn lists, by widget, the parts of the parts table that the builders do
// not draw, with the reason. Each such part is logged by the test. A widget that is
// not drawn at all is listed too.
var layerNotDrawn = map[string]string{
	"Dock":            "the drop preview (Dock.Preview) is drawn while a tab is dragged",
	"Tabs":            "the drop line (Tabs.DropLine) is drawn while a tab of a reorderable list is dragged; the bar and its buttons while the tabs overflow or have a prefix, a suffix or a menu",
	"Menu":            "the menu's parts are drawn while it is open",
	"MenuBar":         "the menu bar's parts are drawn with an open menu",
	"Popover":         "the popover is drawn while it is open",
	"HoverCard":       "the card is drawn while the pointer rests on its trigger",
	"CommandPalette":  "the match is a highlight of a command's name, not an element; the keys need a binding",
	"Toast":           "the toasts are drawn once notified",
	"Tooltip":         "the tooltip is drawn while the pointer rests on its element",
	"Form":            "the form's summary is drawn while the form has errors",
	"FormField":       "the inline label and the required mark need other field options",
	"Select":          "the placeholder, the popup and the options are drawn when the field is empty or open",
	"Combobox":        "the popup, the options and the list's states are drawn while the list is open",
	"DatePicker":      "the placeholder and the selection need an empty value; the calendar is drawn while open",
	"TimePicker":      "the placeholder and the selection need an empty value; the list is drawn while open",
	"DateRangePicker": "the placeholder and the selection need an empty range; the calendar is drawn while open",
	"ColorPicker":     "the popup, the area, the thumb, the slider and the presets are drawn while open",
	"DataGrid":        "the sort indicator, the empty message and the editor need a sorted, empty or editing grid",
	"Breadcrumbs":     "the overflow is drawn when the trail is too long for its width",
	"Tree":            "the focus state of the tree is drawn when the tree has focus",
	"TextInput":       "the placeholder and the selection need an empty field or a selected text",
	"TextArea":        "the placeholder and the selection need an empty field or a selected text",
	"NumberInput":     "the selection needs a selected text",
}

// checkLayerParts checks each base part of the widget in the tree at id: each
// field that the node shows with both the theme's value and the instance's (see
// layerTheme and layerInstance) must have the instance as its winner, with the
// theme's value overridden below it. seen collects the parts that were found.
func checkLayerParts(t *testing.T, tg inspect.Target, widget string, documented []string, seen map[string]bool) {
	t.Helper()
	var walk func(id uint64)
	walk = func(id uint64) {
		e, ok := tg.Element(id)
		if !ok {
			return
		}
		// Only the documented parts are checked: an element may draw an undocumented
		// part, such as the tree's focus ring, which has no style of its own.
		if w, _, _ := strings.Cut(e.Part, "."); w == widget && slices.Contains(documented, e.Part) && !isStatePart(e.Part, documented) {
			seen[e.Part] = true
			checkPartLayers(t, tg, id, e.Part)
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(tg.Root())
}

// checkPartLayers checks the fields of the part's node that the table set in both
// layers.
func checkPartLayers(t *testing.T, tg inspect.Target, id uint64, part string) {
	t.Helper()
	covered := 0
	for _, f := range tg.Fields(id) {
		themeText, instanceText := layerValueText(layerTheme, f.Name), layerValueText(layerInstance, f.Name)
		if themeText == "" || instanceText == "" {
			continue
		}
		theme, instance := layerAny(f, inspect.SourceTheme, themeText), layerAny(f, inspect.SourceInstance, instanceText)
		if theme == nil || instance == nil {
			continue
		}
		covered++
		if _, ok := layerSetByMethod[part+" "+f.Name]; ok && f.Source == inspect.SourceElement {
			continue // the element sets the field itself, as layerSetByMethod says
		}
		if !isInstanceWinner(f.Source) || f.Value != instanceText {
			t.Errorf("%s %s = %s (%s), want %s from the instance Styles; layers %+v", part, f.Name, f.Value, f.Source, instanceText, f.Layers)
			continue
		}
		if !theme.Overridden {
			t.Errorf("%s %s: the theme's %s is not overridden; layers %+v", part, f.Name, themeText, f.Layers)
		}
		if f.Layers[0].Source == inspect.SourceDefault && !f.Layers[0].Overridden {
			t.Errorf("%s %s: the built-in default is not overridden; layers %+v", part, f.Name, f.Layers)
		}
	}
	if covered == 0 {
		t.Errorf("%s: no field shows both the theme's and the instance's value", part)
	}
}

// layerValueText returns the value of the field name of s as the inspector shows
// it, or "" when s does not set it.
func layerValueText(s Style, name string) string {
	v := reflect.ValueOf(s).FieldByName(name)
	if v.IsZero() {
		return ""
	}
	return formatStyleValue(v)
}

// layerAny returns the layer of f from the source with the value, or nil. A layer of
// a state of the source counts too, such as "state: current (theme)" for the theme.
func layerAny(f inspect.Field, source, value string) *inspect.Layer {
	for i := range f.Layers {
		l := &f.Layers[i]
		if l.Value == value && (l.Source == source || strings.HasPrefix(l.Source, inspect.SourceState) &&
			strings.HasSuffix(l.Source, "("+source+")")) {
			return l
		}
	}
	return nil
}

// layerOf returns the layer of f from the source with the value, or nil.
func layerOf(f inspect.Field, source, value string) *inspect.Layer {
	for i := range f.Layers {
		if f.Layers[i].Source == source && f.Layers[i].Value == value {
			return &f.Layers[i]
		}
	}
	return nil
}

// TestLayersNameEveryDocumentedPart runs each builder of the parts table with the
// theme's values and the instance's, and checks that the inspector names the
// instance as the winning layer of each part's fields, with the theme's value
// overridden. The parts come from docs/styling.md, the table that
// TestEveryPartIsApplied also reads. A state part (Hover, Selected, and so on) is
// not drawn by an element of its own, so it is not checked here; the hover state is
// covered by TestLayersNameTheHoverStateAndItsLayer. A documented part that no
// builder draws fails the test unless its widget is in layerNotDrawn.
func TestLayersNameEveryDocumentedPart(t *testing.T) {
	seen := map[string]bool{}
	documented := documentedParts(t)
	for _, b := range layerBuilders {
		t.Run(b.widget, func(t *testing.T) {
			th := DarkTheme()
			tp := b.theme(th)
			setEveryPart(reflect.ValueOf(tp).Elem(), layerTheme)
			inst := reflect.New(reflect.TypeOf(tp).Elem()).Elem()
			setEveryPart(inst, layerInstance)
			_, tg := inspRuntime(t, th, func(ctx *Context) Element { return b.build(inst.Interface()) })
			checkLayerParts(t, tg, b.widget, documented, seen)
		})
	}
	for _, p := range documented {
		if isStatePart(p, documented) {
			continue
		}
		w, part, _ := strings.Cut(p, ".")
		if alias, ok := layerDocAlias[w]; ok {
			w = alias
		}
		if seen[w+"."+part] {
			continue
		}
		if !slices.ContainsFunc(layerBuilders, func(b layerBuilder) bool { return b.widget == w }) {
			if _, ok := layerNotDrawn[w]; !ok {
				t.Errorf("%s is documented, but no layer builder draws %s, and it is not in layerNotDrawn", p, w)
			}
			continue
		}
		if _, ok := layerNotDrawn[w]; !ok {
			t.Errorf("%s is documented, but no layer builder draws it; draw it or list %s in layerNotDrawn", p, w)
		} else {
			t.Logf("%s is not drawn by the layer table: %s", p, layerNotDrawn[w])
		}
	}
}

// TestLayersShowOverriddenValuesAndSameValues checks the layers of a check box's
// box: the built-in default sets the radius to 4, the theme to 6, and the instance
// to 2. A layer that sets the value of the layer below it is marked same.
func TestLayersShowOverriddenValuesAndSameValues(t *testing.T) {
	th := DarkTheme()
	th.Checkbox.Box = Style{Radius: 6}
	_, tg := inspRuntime(t, th, func(ctx *Context) Element {
		return Checkbox("Plain", NewSignal(false)).Styles(CheckboxStyle{Box: Style{Radius: 2}})
	})
	box, ok := inspFind(tg, tg.Root(), func(e inspect.Element) bool { return e.Part == "Checkbox.Box" })
	if !ok {
		t.Fatal("no Checkbox.Box")
	}
	f, _ := inspField(tg, box, "Radius")
	want := []inspect.Layer{
		{Source: inspect.SourceDefault, Value: "4", Overridden: true},
		{Source: inspect.SourceTheme, Value: "6", Overridden: true},
		{Source: inspect.SourceInstance, Value: "2"},
	}
	if f.Source != inspect.SourceInstance || f.Value != "2" || !reflect.DeepEqual(f.Layers, want) {
		t.Fatalf("Radius = %+v, want layers %+v", f, want)
	}

	// A theme that sets the default's radius again: the theme's layer is same.
	th2 := DarkTheme()
	th2.Checkbox.Box = Style{Radius: 4}
	_, tg2 := inspRuntime(t, th2, func(ctx *Context) Element { return Checkbox("Plain", NewSignal(false)) })
	box2, _ := inspFind(tg2, tg2.Root(), func(e inspect.Element) bool { return e.Part == "Checkbox.Box" })
	f2, _ := inspField(tg2, box2, "Radius")
	if len(f2.Layers) != 2 || !f2.Layers[1].Same || f2.Layers[1].Source != inspect.SourceTheme || f2.Source != inspect.SourceTheme {
		t.Fatalf("Radius with a theme of the same value = %+v, want the theme same as the default", f2)
	}
}

// TestLayersNameTheHoverStateAndItsLayer hovers a button whose theme sets the hover
// background: the background is the hover state's, from the theme.
func TestLayersNameTheHoverStateAndItsLayer(t *testing.T) {
	th := DarkTheme()
	th.Button.RootHover = Style{Background: Hex("#123456")}
	rt, tg := inspRuntime(t, th, func(ctx *Context) Element {
		return Button("Save").Key("save")
	})
	save := inspByKey(t, tg, "save")
	root, ok := inspFind(tg, save, func(e inspect.Element) bool { return e.Part == "Button.Root" })
	if !ok {
		t.Fatal("no Button.Root under the button")
	}
	r, _ := rt.Find("save")
	rt.PointerMove(r.X+r.W/2, r.Y+r.H/2, 0)
	rt.Frame(time.Now())

	f, ok := inspField(tg, root, "Background")
	if !ok || f.Source != inspect.SourceState+"hover (theme)" || f.Value != "#123456ff" {
		t.Fatalf("hovered Background = %+v, want #123456ff from the hover state of the theme", f)
	}
	if n := len(f.Layers); n == 0 || f.Layers[n-1].Overridden {
		t.Fatalf("the hover layer is not the winner: %+v", f.Layers)
	}
}

// TestLayersUnderHighContrast checks the high contrast theme: the instance's colors
// are dropped (the system colors win), its sizes and the radius still apply, and
// the theme's radius is overridden by the instance's.
func TestLayersUnderHighContrast(t *testing.T) {
	th := HighContrastTheme(HighContrastBlack(), DarkTheme())
	th.Checkbox.Box = Style{Radius: 6}
	_, tg := inspRuntime(t, th, func(ctx *Context) Element {
		return Checkbox("Plain", NewSignal(false)).Styles(CheckboxStyle{Box: Style{Radius: 9, Background: Hex("#ff0000")}})
	})
	box, _ := inspFind(tg, tg.Root(), func(e inspect.Element) bool { return e.Part == "Checkbox.Box" })
	radius, _ := inspField(tg, box, "Radius")
	if radius.Source != inspect.SourceInstance || radius.Value != "9" {
		t.Fatalf("Radius under high contrast = %+v, want 9 from the instance", radius)
	}
	if n := len(radius.Layers); n < 3 || radius.Layers[n-2].Source != inspect.SourceTheme || radius.Layers[n-2].Value != "6" {
		t.Fatalf("Radius layers under high contrast = %+v, want the theme's 6 below the instance", radius.Layers)
	}
	bg, _ := inspField(tg, box, "Background")
	if bg.Source == inspect.SourceInstance {
		t.Fatalf("Background under high contrast = %+v, want the system color, not the instance's", bg)
	}
}

// TestLayerRecordsFollowMergeSemantics merges layers of one field each and checks
// that the last record of each field is the layer and the value that Merge left,
// including a partial merge of edges, a border width that resets its edges, and a
// clear.
func TestLayerRecordsFollowMergeSemantics(t *testing.T) {
	a := tagStyle(Style{Padding: All(4), BorderEdges: Edges{Top: 1}, Background: Hex("#101010"), Radius: 3}, "a")
	b := tagStyle(Style{Padding: Edges{Right: 8}, BorderWidth: 2}, "b")
	c := tagStyle(Style{Clear: ClearRadius | ClearBackground}, "c")
	cases := []struct {
		name   string
		s      Style
		field  string
		layer  string
		value  string
		layers int // the records of the field
	}{
		{"partial edges", a.Merge(b), "Padding", "b", "4 8 4 4", 2},
		{"border width resets edges", a.Merge(b), "BorderEdges", "cleared by b", "0", 2},
		{"border width sets", a.Merge(b), "BorderWidth", "b", "2", 1},
		{"clear removes", a.Merge(c), "Radius", "cleared by c", "0", 2},
		{"clear removes a background", a.Merge(c), "Background", "cleared by c", "#00000000", 2},
		{"untouched field", a.Merge(b), "Background", "a", "#101010ff", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recs := tc.s.prov[tc.field]
			if len(recs) != tc.layers {
				t.Fatalf("%s has %d records, want %d: %+v", tc.field, len(recs), tc.layers, recs)
			}
			last := recs[len(recs)-1]
			if last.layer != tc.layer || last.value != tc.value {
				t.Fatalf("%s last record = %+v, want layer %q value %q", tc.field, last, tc.layer, tc.value)
			}
			// The last record must agree with the merged value, or a value that a
			// method of the element set later would look like a layer.
			if got := formatStyleValue(reflect.ValueOf(tc.s).FieldByName(tc.field)); got != last.value {
				t.Fatalf("%s is %q, but its last record says %q", tc.field, got, last.value)
			}
		})
	}
}

// TestNoLayerRecordsWithoutAnInspector checks that nothing is recorded while no
// inspector is attached: the resolved styles carry no records.
func TestNoLayerRecordsWithoutAnInspector(t *testing.T) {
	th := DarkTheme()
	th.Checkbox.Box = Style{Radius: 6}
	rt := NewRuntime(func(ctx *Context) Element {
		return Checkbox("Plain", NewSignal(false)).Styles(CheckboxStyle{Box: Style{Radius: 2}})
	}, th, nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	for _, n := range rt.byID {
		if n.div != nil && n.style.prov != nil {
			t.Fatalf("node %q has layer records without an inspector: %+v", n.div.part, n.style.prov)
		}
	}
}
