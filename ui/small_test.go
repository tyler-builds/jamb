package ui

import (
	"testing"
	"time"
)

// Small makes every field-like control 28 px high, as Button.Small and
// TextInput.Small are; without it they are 34 px.
func TestSmallFieldsAreCompact(t *testing.T) {
	opts := []Option[string]{{Value: "a", Label: "A"}}
	day := time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local)
	cases := []struct {
		name, part string
		build      func(small bool) Element
	}{
		{"Select", "Select.Trigger", func(small bool) Element {
			s := Select(NewSignal("a"), opts...).Label("Pick")
			if small {
				s.Small()
			}
			return s
		}},
		{"NumberInput", "NumberInput.Frame", func(small bool) Element {
			n := NumberInput(NewSignal(3), 0, 9, 1).Label("N").SpinButtons(SpinStacked)
			if small {
				n.Small()
			}
			return n
		}},
		{"Combobox", "Combobox.Frame", func(small bool) Element {
			c := Combobox(NewSignal(""), func(string) []Option[string] { return opts }).Label("Find")
			if small {
				c.Small()
			}
			return c
		}},
		{"DatePicker", "DatePicker.Frame", func(small bool) Element {
			d := DatePicker(NewSignal(day)).Label("Day")
			if small {
				d.Small()
			}
			return d
		}},
		{"TimePicker", "TimePicker.Frame", func(small bool) Element {
			p := TimePicker(NewSignal(day)).Label("Time")
			if small {
				p.Small()
			}
			return p
		}},
	}
	for _, c := range cases {
		for _, small := range []bool{false, true} {
			r := newAccessRig(func(ctx *Context) Element { return Div().Column().Align(Start).Child(c.build(small)) })
			want := float32(34)
			if small {
				want = 28
			}
			if got := partNode(t, r.rt, c.part, "").lay.H; got != want {
				t.Errorf("%s (small %v): height %v, want %v", c.name, small, got, want)
			}
		}
	}
}
