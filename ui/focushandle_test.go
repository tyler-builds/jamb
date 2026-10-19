package ui

import (
	"testing"
	"time"
)

// Every interactive control takes a FocusHandle, and focusing the handle puts
// the keyboard focus on the control.
func TestEveryControlTakesAFocusHandle(t *testing.T) {
	opts := []Option[string]{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}
	type row struct{ ID string }
	cases := []struct {
		name  string
		build func(h *FocusHandle) Element
	}{
		{"Button", func(h *FocusHandle) Element { return Button("Save").FocusHandle(h) }},
		{"Checkbox", func(h *FocusHandle) Element { return Checkbox("On", NewSignal(false)).FocusHandle(h) }},
		{"Switch", func(h *FocusHandle) Element { return Switch("On", NewSignal(false)).FocusHandle(h) }},
		{"Toggle", func(h *FocusHandle) Element { return Toggle("Bold", NewSignal(false)).FocusHandle(h) }},
		{"ToggleGroup", func(h *FocusHandle) Element { return ToggleGroup(NewSignal("b"), opts...).Label("Pick").FocusHandle(h) }},
		{"Slider", func(h *FocusHandle) Element { return Slider(NewSignal(5.0), 0, 10, 1).Label("Level").FocusHandle(h) }},
		{"RadioGroup", func(h *FocusHandle) Element { return RadioGroup(NewSignal("b"), opts...).Label("Pick").FocusHandle(h) }},
		{"Select", func(h *FocusHandle) Element { return Select(NewSignal("a"), opts...).Label("Pick").FocusHandle(h) }},
		{"Combobox", func(h *FocusHandle) Element {
			return Combobox(NewSignal(""), func(string) []Option[string] { return opts }).Label("Find").FocusHandle(h)
		}},
		{"NumberInput", func(h *FocusHandle) Element { return NumberInput(NewSignal(3), 0, 9, 1).Label("N").FocusHandle(h) }},
		{"DatePicker", func(h *FocusHandle) Element {
			return DatePicker(NewSignal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local))).Label("Day").FocusHandle(h)
		}},
		{"TimePicker", func(h *FocusHandle) Element {
			return TimePicker(NewSignal(time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local))).Label("Time").FocusHandle(h)
		}},
		{"DateRangePicker", func(h *FocusHandle) Element {
			day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)
			return DateRangePicker(NewSignal(day), NewSignal(day.AddDate(0, 0, 2))).Label("Days").FocusHandle(h)
		}},
		{"ColorPicker", func(h *FocusHandle) Element {
			return ColorPicker(NewSignal(Hex("#3B82F6"))).Label("Fill").FocusHandle(h)
		}},
		{"TextInput", func(h *FocusHandle) Element { return TextInput(NewSignal("")).Label("Name").FocusHandle(h) }},
		{"TextArea", func(h *FocusHandle) Element { return TextArea(NewSignal("")).Label("Notes").FocusHandle(h) }},
		{"Tabs", func(h *FocusHandle) Element {
			return Tabs(NewSignal("a"), Tab[string]{Value: "a", Label: "A"}, Tab[string]{Value: "b", Label: "B"}).Label("Views").FocusHandle(h)
		}},
		{"Tree", func(h *FocusHandle) Element {
			return Tree([]TreeNode{{Key: "a", Label: "A"}, {Key: "b", Label: "B"}}, NewSignal("a")).Label("Files").FocusHandle(h)
		}},
		{"DataGrid", func(h *FocusHandle) Element {
			return DataGrid([]row{{"1"}, {"2"}}, func(r row) string { return r.ID },
				DataColumn[row]{Header: "ID", Text: func(r row) string { return r.ID }}).Label("Rows").FocusHandle(h)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewFocusHandle()
			r := newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(Button("Before")).Child(c.build(h))
			})
			h.Focus()
			r.frame()
			if !h.Focused() {
				t.Fatalf("%s: focusing its handle did not focus the control", c.name)
			}
		})
	}
}
