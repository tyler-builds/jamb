package ui

import (
	"testing"
	"time"
)

// Small and Large give every sized control the same height: 28 and 40 px,
// against 34 px by default, so a row of them lines up.
func TestControlSizes(t *testing.T) {
	opts := []Option[string]{{Value: "a", Label: "A"}}
	day := time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local)
	cases := []struct {
		name, part string
		build      func(size controlSize) Element
	}{
		{"Button", "Button.Root", func(size controlSize) Element { return sizedAs[*ButtonElement](Button("Save"), size) }},
		{"TextInput", "TextInput.Frame", func(size controlSize) Element {
			return sizedAs[*TextInputElement](TextInput(NewSignal("")).Label("Name"), size)
		}},
		{"Select", "Select.Trigger", func(size controlSize) Element {
			return sizedAs[*SelectElement[string]](Select(NewSignal("a"), opts...).Label("Pick"), size)
		}},
		{"NumberInput stacked", "NumberInput.Frame", func(size controlSize) Element {
			return sizedAs[*NumberInputElement[int]](NumberInput(NewSignal(3), 0, 9, 1).Label("N").SpinButtons(SpinStacked), size)
		}},
		{"NumberInput inline", "NumberInput.Frame", func(size controlSize) Element {
			return sizedAs[*NumberInputElement[int]](NumberInput(NewSignal(3), 0, 9, 1).Label("N").SpinButtons(SpinInline), size)
		}},
		{"Combobox", "Combobox.Frame", func(size controlSize) Element {
			return sizedAs[*ComboboxElement[string]](Combobox(NewSignal(""), func(string) []Option[string] { return opts }).Label("Find"), size)
		}},
		{"DatePicker", "DatePicker.Frame", func(size controlSize) Element {
			return sizedAs[*DatePickerElement](DatePicker(NewSignal(day)).Label("Day"), size)
		}},
		{"TimePicker", "TimePicker.Frame", func(size controlSize) Element {
			return sizedAs[*TimePickerElement](TimePicker(NewSignal(day)).Label("Time"), size)
		}},
	}
	want := map[controlSize]float32{sizeDefault: 34, sizeSmall: 28, sizeLarge: 40}
	for _, c := range cases {
		for _, size := range []controlSize{sizeDefault, sizeSmall, sizeLarge} {
			r := newAccessRig(func(ctx *Context) Element { return Div().Column().Align(Start).Child(c.build(size)) })
			if got := partNode(t, r.rt, c.part, "").lay.H; got != want[size] {
				t.Errorf("%s (size %d): height %v, want %v", c.name, size, got, want[size])
			}
		}
	}
	// The last of Small and Large wins.
	r := newAccessRig(func(ctx *Context) Element { return Div().Column().Align(Start).Child(Button("Save").Large().Small()) })
	if got := partNode(t, r.rt, "Button.Root", "").lay.H; got != 28 {
		t.Errorf("Large().Small(): height %v, want 28", got)
	}
}

// sizedAs calls Small or Large on e for size, or nothing for the default.
func sizedAs[E interface {
	Element
	Small() E
	Large() E
}](e E, size controlSize) Element {
	switch size {
	case sizeSmall:
		return e.Small()
	case sizeLarge:
		return e.Large()
	}
	return e
}
