package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
)

// focusedKey returns the key of the focused element, or "" when nothing has
// focus.
func focusedKey(rt *Runtime) string {
	if n := rt.byID[rt.focusID]; n != nil {
		return n.key
	}
	return ""
}

// ringQuads counts the focus-ring quads in the scene.
// isRing reports whether q is a focus ring: a hollow border around its box in
// color c.
func isRing(q scene.Quad, c Color) bool {
	return q.Background.A == 0 && q.BorderWidth > 0 && q.BorderColor == c.scene()
}

func ringQuads(rt *Runtime) int {
	n := 0
	for _, q := range rt.scene.Quads {
		if isRing(q, DarkTheme().FocusRing) {
			n++
		}
	}
	return n
}

// Tab makes input keyboard-driven and a click makes it pointer-driven; a key
// press after a click makes it keyboard-driven again.
func TestKeyboardModalityFollowsInput(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("a").Size(Px(100), Px(40)).Focusable()).
			Child(Div().Key("b").Size(Px(100), Px(40)).Focusable())
	})
	r.key(input.KeyTab, 0)
	if !r.rt.keyboardModality || focusedKey(r.rt) != "a" {
		t.Fatalf("after Tab: keyboard %v, focus %q; want true, a", r.rt.keyboardModality, focusedKey(r.rt))
	}
	r.click(50, 60) // the second element
	if r.rt.keyboardModality || focusedKey(r.rt) != "b" {
		t.Fatalf("after click: keyboard %v, focus %q; want false, b", r.rt.keyboardModality, focusedKey(r.rt))
	}
	// A modifier on its own is not a key press that makes input keyboard-driven.
	r.rt.KeyDown(input.KeyUnknown, input.ModCtrl, false)
	if r.rt.keyboardModality {
		t.Error("a modifier-only key press made input keyboard-driven")
	}
	r.key(input.KeyTab, 0)
	if !r.rt.keyboardModality || focusedKey(r.rt) != "a" {
		t.Errorf("after Tab following a click: keyboard %v, focus %q; want true, a", r.rt.keyboardModality, focusedKey(r.rt))
	}
}

// FocusVisible applies while focus is keyboard-driven and is removed by a
// click, and it comes back with the next key press.
func TestFocusVisibleStyleFollowsModality(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("a").Size(Px(100), Px(40)).FocusVisible(Style{BorderWidth: 2, BorderColor: Hex("#ff0000")}))
	})
	border := func() float32 { return r.rt.findKey(r.rt.tree, "a").style.BorderWidth }
	r.key(input.KeyTab, 0)
	if border() != 2 {
		t.Errorf("focused by Tab: border %v, want 2", border())
	}
	r.click(50, 20)
	if border() != 0 {
		t.Errorf("focused by click: border %v, want 0 (no focus-visible style)", border())
	}
	r.key(input.KeyDown, 0)
	if border() != 2 {
		t.Errorf("after a key press with focus kept: border %v, want 2", border())
	}
}

// Focused applies whatever the modality, unlike FocusVisible.
func TestFocusedStyleAppliesAfterClick(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("a").Size(Px(100), Px(40)).Focused(Style{BorderWidth: 3}))
	})
	r.click(50, 20)
	if n := r.rt.findKey(r.rt.tree, "a"); n.style.BorderWidth != 3 {
		t.Errorf("Focused after click: border %v, want 3", n.style.BorderWidth)
	}
}

// A button shows its ring only while it has keyboard focus, as a spread shadow
// (so its box does not move).
func TestButtonRingOnlyWhenKeyboardFocused(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().Child(Button("Save").Key("save"))
	})
	if got := ringQuads(r.rt); got != 0 {
		t.Fatalf("unfocused button: %d ring quads, want 0", got)
	}
	r.key(input.KeyTab, 0)
	if focusedKey(r.rt) != "save" || ringQuads(r.rt) != 1 {
		t.Errorf("focused by Tab: focus %q, %d ring quads; want save, 1", focusedKey(r.rt), ringQuads(r.rt))
	}
	r.click(5, 5)
	if focusedKey(r.rt) != "save" || ringQuads(r.rt) != 0 {
		t.Errorf("focused by click: focus %q, %d ring quads; want save, 0", focusedKey(r.rt), ringQuads(r.rt))
	}
}

// The ring is a spread shadow, added to the button's own shadows, so the
// button's box and its layout are unchanged.
func TestButtonRingKeepsLayoutAndShadows(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().Child(Button("Save").Key("save"))
	})
	before := r.rt.findKey(r.rt.tree, "save").lay.H
	r.key(input.KeyTab, 0)
	n := r.rt.findKey(r.rt.tree, "save")
	if n.lay.H != before {
		t.Errorf("height changed from %v to %v when focused", before, n.lay.H)
	}
	if len(n.style.Shadows) != 2 || n.style.Shadows[1].Spread != 2 {
		t.Errorf("shadows %+v, want the button's shadow and a spread-2 ring", n.style.Shadows)
	}
}

// Enter and Space run the OnClick of the focused element, and only without
// modifiers and not for repeats.
func TestEnterAndSpaceActivateFocusedElement(t *testing.T) {
	clicks, buttons := 0, 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("card").Size(Px(100), Px(40)).Focusable().OnClick(func() { clicks++ })).
			Child(Button("Go").Key("go").OnClick(func() { buttons++ }))
	})
	r.key(input.KeyTab, 0)
	r.key(input.KeyEnter, 0)
	r.key(input.KeySpace, 0)
	if clicks != 2 {
		t.Fatalf("Enter and Space on a focused div: %d clicks, want 2", clicks)
	}
	r.key(input.KeyEnter, input.ModCtrl)
	r.key(input.KeySpace, input.ModShift)
	r.key(input.KeyEnter, input.ModAlt)
	if clicks != 2 {
		t.Errorf("modified Enter or Space clicked: %d clicks, want 2", clicks)
	}
	r.rt.KeyDown(input.KeyEnter, 0, true) // auto-repeat
	r.rt.KeyUp(input.KeyEnter, 0)
	if clicks != 2 {
		t.Errorf("a repeated Enter clicked: %d clicks, want 2", clicks)
	}
	r.key(input.KeyTab, 0)
	r.key(input.KeyEnter, 0)
	r.key(input.KeySpace, 0)
	if focusedKey(r.rt) != "go" || buttons != 2 {
		t.Errorf("focused button: focus %q, %d clicks; want go, 2", focusedKey(r.rt), buttons)
	}
}

// A binding for Enter runs instead of the click, as bindings run first.
func TestBindingWinsOverEnterActivation(t *testing.T) {
	act := NewAction("test.confirm", "Confirm")
	ran, clicks := 0, 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().OnAction(act, func() { ran++ }).
			Child(Div().Key("card").Size(Px(100), Px(40)).Focusable().OnClick(func() { clicks++ }))
	})
	r.rt.AddBindings(Bind("enter", act))
	r.key(input.KeyTab, 0)
	r.key(input.KeyEnter, 0)
	if ran != 1 || clicks != 0 {
		t.Errorf("Enter with a binding: action ran %d times, clicks %d; want 1, 0", ran, clicks)
	}
}

// Enter and Space in a text field are typing and submit keys, never a click on
// an element that contains the field.
func TestEnterAndSpaceLeaveTextFieldsAlone(t *testing.T) {
	v := NewSignal("")
	clicks := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().OnClick(func() { clicks++ }).
			Child(TextInput(v).Key("field"))
	})
	r.click(50, 20)
	clicks = 0
	r.key(input.KeyEnter, 0)
	r.key(input.KeySpace, 0)
	if n := r.rt.byID[r.rt.focusID]; n == nil || n.kind != kindEditor {
		t.Fatal("the text field did not keep focus")
	}
	if clicks != 0 {
		t.Errorf("Enter or Space in a text field clicked the container %d times", clicks)
	}
}

// Tab visits elements with a positive TabIndex first, in ascending order with
// ties in tree order, then the rest in tree order. Negative indexes and
// TabStop(false) are skipped. Shift+Tab walks the same order backwards.
func TestTabIndexOrder(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("a").Size(Px(100), Px(20)).Focusable()).
			Child(Div().Key("b").Size(Px(100), Px(20)).TabIndex(2)).
			Child(Div().Key("c").Size(Px(100), Px(20)).TabIndex(1)).
			Child(Div().Key("d").Size(Px(100), Px(20)).TabIndex(0)).
			Child(Div().Key("e").Size(Px(100), Px(20)).TabIndex(2)).
			Child(Div().Key("f").Size(Px(100), Px(20)).TabIndex(-1)).
			Child(Div().Key("g").Size(Px(100), Px(20)).TabIndex(0).TabStop(false))
	})
	var forward []string
	for range 5 {
		r.key(input.KeyTab, 0)
		forward = append(forward, focusedKey(r.rt))
	}
	if want := []string{"c", "b", "e", "a", "d"}; !slices.Equal(forward, want) {
		t.Errorf("Tab order %v, want %v", forward, want)
	}
	var backward []string
	for range 5 {
		r.key(input.KeyTab, input.ModShift)
		backward = append(backward, focusedKey(r.rt))
	}
	if want := []string{"a", "e", "b", "c", "d"}; !slices.Equal(backward, want) {
		t.Errorf("Shift+Tab order %v, want %v", backward, want)
	}
}

// TabStop(false) takes an element out of Tab navigation, but a click still
// focuses it.
func TestTabStopFalseSkipsButStillFocusable(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("a").Size(Px(100), Px(40)).Focusable()).
			Child(Div().Key("b").Size(Px(100), Px(40)).Focusable().TabStop(false)).
			Child(Div().Key("c").Size(Px(100), Px(40)).Focusable())
	})
	var seq []string
	for range 3 {
		r.key(input.KeyTab, 0)
		seq = append(seq, focusedKey(r.rt))
	}
	if want := []string{"a", "c", "a"}; !slices.Equal(seq, want) {
		t.Errorf("Tab sequence %v, want %v (b skipped)", seq, want)
	}
	r.click(50, 60)
	if focusedKey(r.rt) != "b" {
		t.Errorf("click on a TabStop(false) element: focus %q, want b", focusedKey(r.rt))
	}
}

// TabIndex implies focusable, and an element inside an overlay takes its place
// in the order from the anchor's position in the tree.
func TestTabOrderIncludesOverlayInTreeOrder(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("a").Size(Px(100), Px(20)).Focusable()).
			Child(Div().Key("anchor").Size(Px(100), Px(20)).TabIndex(0).
				Child(Div().Key("menu").Overlay(BelowStart).Size(Px(100), Px(40)).
					Child(Div().Key("m1").Size(Px(100), Px(20)).Focusable()))).
			Child(Div().Key("c").Size(Px(100), Px(20)).Focusable())
	})
	var seq []string
	for range 4 {
		r.key(input.KeyTab, 0)
		seq = append(seq, focusedKey(r.rt))
	}
	if want := []string{"a", "anchor", "m1", "c"}; !slices.Equal(seq, want) {
		t.Errorf("Tab sequence %v, want %v", seq, want)
	}
}

// Tab scrolls the element it moves to into view, in its scroll container.
func TestTabRevealsOffScreenElement(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element {
		list := Div().Key("list").Size(Px(300), Px(200)).ScrollY()
		for i := range 20 {
			list.Child(Div().Keyf("row%d", i).Height(Px(40)).Shrink(0).Focusable())
		}
		return list
	})
	for range 15 {
		r.key(input.KeyTab, 0)
	}
	if focusedKey(r.rt) != "row14" {
		t.Fatalf("focus %q, want row14", focusedKey(r.rt))
	}
	list := r.rt.findKey(r.rt.tree, "list")
	row := r.rt.findKey(r.rt.tree, "row14")
	if row.abs.Y < list.abs.Y || row.abs.Bottom() > list.abs.Bottom() {
		t.Errorf("row14 at y %v..%v is outside the list at %v..%v",
			row.abs.Y, row.abs.Bottom(), list.abs.Y, list.abs.Bottom())
	}
	if list.st.targetY <= 0 {
		t.Errorf("the list did not scroll (offset %v)", list.st.targetY)
	}
}

// A component can ask whether it has keyboard focus, and it re-renders when
// that changes.
func TestKeyboardFocusReadByComponent(t *testing.T) {
	var last bool
	r := newKeyRig(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			last = ctx.KeyboardFocus()
			return Div().Key("custom").Size(Px(100), Px(40)).Focusable()
		})
	})
	r.key(input.KeyTab, 0)
	if !last {
		t.Error("KeyboardFocus false after Tab")
	}
	r.click(50, 20)
	if last {
		t.Error("KeyboardFocus true after a click")
	}
}
