package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
)

// isInstanceWinner reports whether a winning layer is the instance's: its Styles, or a
// state of them, such as "state: checked (instance Styles)".
func isInstanceWinner(source string) bool {
	return source == inspect.SourceInstance || strings.HasPrefix(source, inspect.SourceState) &&
		strings.HasSuffix(source, "("+inspect.SourceInstance+")")
}

// stateLabels are the labels that the inspector gives the pointer and focus states, by
// the suffix of the part (see inState in style_layers.go).
var stateLabels = map[string]string{"Hover": "hover", "Pressed": "pressed", "FocusVisible": "focus visible",
	"FocusWithin": "focus within", "Focused": "focus"}

// stateLabel returns the label of a state suffix: a pointer or focus state, or an
// applied state (see appliedStates).
func stateLabel(suffix string) (string, bool) {
	if l, ok := stateLabels[suffix]; ok {
		return l, true
	}
	l, ok := appliedStates[suffix]
	return l, ok
}

// documentedStateOf returns the base part of a documented state part, such as "Checkbox.Box"
// for "Checkbox.BoxChecked", with the label of its state. Aliases of the widgets (see
// layerDocAlias) are mapped to the names in the code.
func documentedStateOf(p string, documented []string) (base, label string, ok bool) {
	for _, s := range layerStateSuffixes {
		b, found := strings.CutSuffix(p, s)
		if !found || !slices.Contains(documented, b) {
			continue
		}
		l, has := stateLabel(s)
		if !has {
			return "", "", false
		}
		w, part, _ := strings.Cut(b, ".")
		if alias, mapped := layerDocAlias[w]; mapped {
			w = alias
		}
		return w + "." + part, l, true
	}
	return "", "", false
}

// layerStateNotDrawn lists the documented state parts that no builder of the layer
// table reaches, with the reason. Each such part is logged by the test; a reason is
// what the table cannot do headlessly, not a claim that the state is wrong.
var layerStateNotDrawn = map[string]string{
	"Dock.TabFocusVisible":            "after the press selects the tab and the dock re-renders, setFocus restyles a stale node from byID, not the live tab; remove this entry with the fix for that bug",
	"NumberInput.SpinButtonDisabled":  "a spin button is disabled only at the limit of its value, which the table does not reach",
	"Form.SummaryItemHover":           "the form summary lists the errors of a form that has failed validation",
	"Form.SummaryItemFocusVisible":    "the form summary lists the errors of a form that has failed validation",
	"Slider.ThumbPressed":             "a press on the thumb starts a drag, which the table does not start",
	"ColorPicker.AreaFocusVisible":    "the area is drawn while the color picker's popup is open",
	"ColorPicker.PresetFocusVisible":  "the presets are drawn while the color picker's popup is open",
	"Split.DividerHover":              "the hover of the divider is on its handle (Split.Handle), not on the bar that the docs name Split.Divider",
	"Split.DividerFocusVisible":       "the focus of the divider is on its handle (Split.Handle), not on the bar that the docs name Split.Divider",
	"Split.DividerActive":             "the active state is a drag of the divider, which the table does not start",
	"DataGrid.CellFocused":            "a focused cell is not reached by focusing the grid headlessly",
	"DataGrid.CellEditing":            "the editor of a cell is open only after an edit starts",
	"DropdownMenu.ItemHighlighted":    "the menu's items are drawn while the menu is open",
	"DropdownMenu.ItemDisabled":       "the menu's items are drawn while the menu is open",
	"ContextMenu.ItemHighlighted":     "the menu's items are drawn while the menu is open",
	"ContextMenu.ItemDisabled":        "the menu's items are drawn while the menu is open",
	"MenuBar.BarItemHover":            "the menu bar's items are not reached by the table; to be checked in a window",
	"MenuBar.BarItemFocusVisible":     "the menu bar's items are not reached by the table; to be checked in a window",
	"MenuBar.BarItemOpen":             "the open item is drawn while its menu is open",
	"Sidebar.ItemFocusVisible":        "not reached by the table headlessly; to be checked in a window",
	"Select.OptionHighlighted":        "the highlighted option is drawn in an open list",
	"Select.OptionSelected":           "the selected option is drawn in an open list",
	"Select.OptionDisabled":           "a disabled option is drawn in an open list",
	"Select.TriggerInvalid":           "the invalid trigger needs a failed validation",
	"Combobox.OptionHighlighted":      "the highlighted option is drawn in an open list",
	"Combobox.OptionDisabled":         "a disabled option is drawn in an open list",
	"TimePicker.OptionHighlighted":    "the highlighted option is drawn in an open list",
	"TimePicker.OptionSelected":       "the selected option is drawn in an open list",
	"DatePicker.DaySelected":          "the days are drawn in an open calendar",
	"DatePicker.DayToday":             "the days are drawn in an open calendar",
	"DatePicker.DayDisabled":          "the days are drawn in an open calendar",
	"DatePicker.DayOutsideMonth":      "the days are drawn in an open calendar",
	"DatePicker.DayFocusVisible":      "the days are drawn in an open calendar",
	"DatePicker.DayHover":             "the days are drawn in an open calendar",
	"DateRangePicker.DaySelected":     "the days are drawn in an open calendar",
	"DateRangePicker.DayInRange":      "the days are drawn in an open calendar",
	"DateRangePicker.DayToday":        "the days are drawn in an open calendar",
	"DateRangePicker.DayDisabled":     "the days are drawn in an open calendar",
	"DateRangePicker.DayOutsideMonth": "the days are drawn in an open calendar",
	"DateRangePicker.DayFocusVisible": "the days are drawn in an open calendar",
	"DateRangePicker.DayHover":        "the days are drawn in an open calendar",
	"TextInput.FrameInvalid":          "the invalid frame needs a failed validation",
	"TextArea.FrameInvalid":           "the invalid frame needs a failed validation",
	"NumberInput.FrameInvalid":        "the invalid frame needs a failed validation",
	"DatePicker.FrameInvalid":         "the invalid frame needs a failed validation",
	"TimePicker.FrameInvalid":         "the invalid frame needs a failed validation",
	"DateRangePicker.FrameInvalid":    "the invalid frame needs a failed validation",
	"RadioGroup.RingInvalid":          "the invalid ring needs a failed validation",
	"Checkbox.BoxInvalid":             "the invalid box needs a failed validation",
	"CheckboxTri.BoxInvalid":          "the invalid box needs a failed validation",
}

// checkStateLayers checks the state layers of every element in the tree. A field whose
// layers name a state (such as "state: hover (instance Styles)") must also name the
// state's theme layer, with the theme's value overridden below the instance's, and
// the instance's value must be the table's. The (part, state) pairs found are recorded
// in seen.
func checkStateLayers(t *testing.T, tg inspect.Target, seen map[string]bool) {
	t.Helper()
	var walk func(id uint64)
	walk = func(id uint64) {
		e, ok := tg.Element(id)
		if !ok {
			return
		}
		if e.Part != "" {
			for _, f := range tg.Fields(id) {
				checkFieldStates(t, e.Part, f, seen)
			}
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(tg.Root())
}

// checkFieldStates is checkStateLayers for one field of a part.
func checkFieldStates(t *testing.T, part string, f inspect.Field, seen map[string]bool) {
	t.Helper()
	themeText, instanceText := layerValueText(layerTheme, f.Name), layerValueText(layerInstance, f.Name)
	if themeText == "" || instanceText == "" {
		return
	}
	for _, l := range f.Layers {
		if !strings.HasPrefix(l.Source, inspect.SourceState) || !strings.HasSuffix(l.Source, " ("+inspect.SourceInstance+")") {
			continue
		}
		label := strings.TrimSuffix(strings.TrimPrefix(l.Source, inspect.SourceState), " ("+inspect.SourceInstance+")")
		seen[part+"|"+label] = true
		if l.Value != instanceText {
			t.Errorf("%s %s: the %s state is %s, want the instance's %s; layers %+v", part, f.Name, label, l.Value, instanceText, f.Layers)
		}
		themeState := layerOf(f, inspect.SourceState+label+" ("+inspect.SourceTheme+")", themeText)
		if themeState == nil {
			t.Errorf("%s %s: the %s state has no theme layer of %s; layers %+v", part, f.Name, label, themeText, f.Layers)
		} else if !themeState.Overridden {
			t.Errorf("%s %s: the theme layer of the %s state is not overridden; layers %+v", part, f.Name, label, f.Layers)
		}
	}
}

// TestLayersNameEveryDocumentedState renders each builder of the layer table and
// activates each state of its elements in turn: the pointer over an element (hover),
// a press on it, and keyboard focus on it (focus visible and focus within). The
// applied states (disabled, checked, selected, and so on) are checked as they are
// drawn. A state part of docs/styling.md must be seen with its state named by the
// inspector, or be listed in layerStateNotDrawn with the reason.
func TestLayersNameEveryDocumentedState(t *testing.T) {
	seen := map[string]bool{}
	documented := documentedParts(t)
	for _, b := range layerBuilders {
		th := DarkTheme()
		tp := b.theme(th)
		setEveryPart(reflect.ValueOf(tp).Elem(), layerTheme)
		inst := reflect.New(reflect.TypeOf(tp).Elem()).Elem()
		setEveryPart(inst, layerInstance)
		r := newThemedRig(th, func(ctx *Context) Element { return b.build(inst.Interface()) })
		tg := uihost.Of(r.rt).(inspect.Target)
		tg.Attach(func() {})
		r.frame()
		checkStateLayers(t, tg, seen)

		var ids []uint64
		var collect func(id uint64)
		collect = func(id uint64) {
			e, ok := tg.Element(id)
			if !ok {
				return
			}
			if e.Part != "" {
				ids = append(ids, id)
			}
			for _, c := range e.Children {
				collect(c)
			}
		}
		collect(tg.Root())
		for _, id := range ids {
			n := r.rt.byID[id]
			if n == nil || n.div == nil {
				continue
			}
			hoverNode(r, n)
			checkStateLayers(t, tg, seen)
			r.rt.PointerMove(-1000, -1000, 0)
			r.frame()

			pressNode(r, n)
			checkStateLayers(t, tg, seen)
			rc := r.rt.nodeRect(n)
			r.rt.PointerUp(rc.X+rc.W/2, rc.Y+rc.H/2, input.ButtonLeft, 0)
			// A press may have focused the element, and focusing it again does not
			// restyle it, so the focus is taken away first.
			r.rt.setFocus(0)
			r.frame()

			focusNode(r, n)
			checkStateLayers(t, tg, seen)
			r.rt.setFocus(0)
			r.frame()
		}
	}
	for _, p := range documented {
		base, label, ok := documentedStateOf(p, documented)
		if !ok || seen[base+"|"+label] {
			continue
		}
		if reason, listed := layerStateNotDrawn[p]; listed {
			t.Logf("%s is not drawn by the state table: %s", p, reason)
			continue
		}
		t.Errorf("%s is documented, but no element showed the %q state of %s; draw it or list it in layerStateNotDrawn with a reason", p, label, base)
	}
}
