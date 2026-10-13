package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// Checkbox and CheckboxTri report a change by click and by Space, after the
// signal is set.
func TestCheckboxOnChange(t *testing.T) {
	on := NewSignal(false)
	var got []bool
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(Checkbox("Alerts", on).OnChange(func(c bool) {
			if c != on.Peek() {
				t.Errorf("OnChange(%v) ran before the signal was set (%v)", c, on.Peek())
			}
			got = append(got, c)
		}))
	})
	r.click(20, 20)
	r.key(input.KeySpace, 0)
	if len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("OnChange saw %v, want [true false]", got)
	}

	mixed := NewSignal(CheckMixed)
	var tri []bool
	r = newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(CheckboxTri("All", mixed).OnChange(func(c bool) { tri = append(tri, c) }))
	})
	r.click(20, 20)
	if len(tri) != 1 || tri[0] != (mixed.Peek() == CheckOn) {
		t.Fatalf("CheckboxTri OnChange saw %v with the state %v", tri, mixed.Peek())
	}
}

// NumberInput reports a change by key, by typed text and Enter, and not when
// the value stays the same.
func TestNumberInputOnChange(t *testing.T) {
	v := NewSignal(9)
	var got []int
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(NumberInput(v, 0, 10, 1).Label("Hour").OnChange(func(x int) { got = append(got, x) }))
	})
	r.click(40, 27)
	r.key(input.KeyUp, 0) // 10
	r.key(input.KeyUp, 0) // stays 10: no change
	r.key(input.KeyA, ctrl)
	r.typeText("3")
	r.key(input.KeyEnter, 0) // 3
	if len(got) != 2 || got[0] != 10 || got[1] != 3 {
		t.Fatalf("OnChange saw %v, want [10 3]", got)
	}
	if v.Peek() != 3 {
		t.Fatalf("value %d, want 3", v.Peek())
	}
}
