package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// nonModalTest is a window with an opener, a button behind the dialog and a
// non-modal dialog that build makes while open is set. A modal, built after the
// dialog, is shown while modal is set. It counts what the buttons and OnClose did.
type nonModalTest struct {
	r                       *accessRig
	open, modal             *Signal[bool]
	toasts                  bool // the window has a Toaster, for F6 to cycle to
	closes, modalCloses     int
	behind, done, modalDone int
}

func newNonModalTest(build func(c *nonModalTest) *ModalElement) *nonModalTest {
	c := &nonModalTest{open: NewSignal(false), modal: NewSignal(false)}
	c.r = newAccessRig(func(ctx *Context) Element {
		page := Div().Size(Full, Full).Column().
			Child(Button("Open").Key("opener")).
			Child(Button("Behind").Key("behind").OnClick(func() { c.behind++ }))
		if c.open.Get() {
			page.Child(build(c))
		}
		if c.modal.Get() {
			ok := Button("OK").Key("ok").Primary().OnClick(func() { c.modalDone++ })
			page.Child(Modal("Confirm", Text("Sure?")).Footer(ok).Default(ok).
				OnClose(func() { c.modalCloses++; c.modal.Set(false) }))
		}
		if c.toasts {
			page.Child(Toaster().Key("toaster"))
		}
		return page
	})
	return c
}

// show focuses the opener, opens the dialog and lets it settle.
func (c *nonModalTest) show(t *testing.T) {
	t.Helper()
	c.r.focusKey(t, "opener")
	c.open.Set(true)
	for range 12 {
		c.r.frame()
	}
}

// findPanel is a non-modal dialog with a field, and a Done button that closes it.
func findPanel(c *nonModalTest) *ModalElement {
	return Modal("Find", TextInput(NewSignal("")).Label("Find text").Key("find")).
		NonModal().Footer(Button("Done").Key("done").OnClick(func() { c.done++; c.open.Set(false) })).
		OnClose(func() { c.closes++; c.open.Set(false) })
}

// TestNonModalHasNoScrim checks that a non-modal dialog draws no scrim, and that a
// press on the window behind it reaches that window.
func TestNonModalHasNoScrim(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	if partNodeOrNil(c.r.rt, "Modal.Scrim") != nil {
		t.Fatal("a non-modal dialog has a scrim")
	}
	c.r.clickKey(t, "behind")
	if c.behind != 1 {
		t.Fatalf("the window behind was pressed %d times, want 1", c.behind)
	}
	if !c.open.Peek() {
		t.Fatal("a press on the window behind closed the dialog")
	}
}

// TestNonModalPressOutsideDoesNotClose checks that a press outside the panel
// leaves a non-modal dialog open, unless CloseOnClickOutside is set.
func TestNonModalPressOutsideDoesNotClose(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.r.clickAt(5, 390)
	if c.closes != 0 || !c.open.Peek() {
		t.Fatalf("a press outside closed the dialog %d times", c.closes)
	}
}

// TestNonModalCloseOnClickOutside checks that a press outside closes the dialog
// when it is asked to, and that the press still reaches the window behind.
func TestNonModalCloseOnClickOutside(t *testing.T) {
	c := newNonModalTest(func(c *nonModalTest) *ModalElement {
		return Modal("Find", Text("Body")).NonModal().CloseOnClickOutside().OnClose(func() {
			c.closes++
			c.open.Set(false)
		})
	})
	c.show(t)
	c.r.clickKey(t, "behind")
	if c.closes != 1 || c.open.Peek() {
		t.Fatalf("a press outside closed the dialog %d times, open %v", c.closes, c.open.Peek())
	}
	if c.behind != 1 {
		t.Fatalf("the press did not reach the window behind: %d", c.behind)
	}
}

// TestNonModalTabLeavesAndReturns checks that Tab is not trapped in a non-modal
// dialog: from its last stop, Tab goes to the first stop of the window.
func TestNonModalTabLeavesAndReturns(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.r.focusKey(t, "done")
	c.r.press(input.KeyTab, 0)
	if got := c.r.focusedKey(); got != "opener" {
		t.Fatalf("Tab from the dialog's last stop went to %q, want the window's first stop", got)
	}
}

// TestNonModalInitialFocusAndReturn checks that a non-modal dialog takes focus
// on its first field, and that focus returns to the opener when it closes with
// focus inside.
func TestNonModalInitialFocusAndReturn(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	if got := c.r.focusedKey(); got != "find" {
		t.Fatalf("initial focus is on %q, want the field", got)
	}
	c.r.clickKey(t, "done")
	if c.done != 1 {
		t.Fatalf("Done ran %d times", c.done)
	}
	if got := c.r.focusedKey(); got != "opener" {
		t.Fatalf("after closing, focus is on %q, want the opener", got)
	}
}

// TestNonModalClosedWithFocusOutsideKeepsIt checks that closing a non-modal dialog
// while focus is in the window behind leaves focus where it is.
func TestNonModalClosedWithFocusOutsideKeepsIt(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.r.clickKey(t, "behind")
	c.open.Set(false)
	c.r.frame()
	if got := c.r.focusedKey(); got != "behind" {
		t.Fatalf("after closing, focus moved to %q, want it to stay on the window", got)
	}
}

// TestNonModalKeepsAppKeysAndTabOrder checks that a non-modal dialog does not cut
// the key path or the Tab order to itself: the window's controls are still in
// both.
func TestNonModalKeepsAppKeysAndTabOrder(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	if c.r.rt.topModal() != nil {
		t.Fatal("a non-modal dialog is the top modal")
	}
	found := false
	for _, id := range c.r.rt.tabOrder() {
		if n := c.r.rt.byID[id]; n != nil && n.key == "behind" {
			found = true
		}
	}
	if !found {
		t.Fatal("the window behind is not in the Tab order")
	}
}

// TestNonModalDefaultCloseButton checks that a non-modal dialog shows its close
// button without CloseButton, and that CloseButton(false) hides it.
func TestNonModalDefaultCloseButton(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	if partNodeOrNil(c.r.rt, "Modal.Close") == nil {
		t.Fatal("a non-modal dialog has no close button")
	}
	c = newNonModalTest(func(c *nonModalTest) *ModalElement {
		return Modal("Find", Text("Body")).NonModal().CloseButton(false).OnClose(func() {})
	})
	c.show(t)
	if partNodeOrNil(c.r.rt, "Modal.Close") != nil {
		t.Fatal("CloseButton(false) on a non-modal dialog still shows one")
	}
}

// TestNonModalDrawerHasNoScrim checks that a non-modal drawer draws no scrim at
// any side, and that it is built.
func TestNonModalDrawerHasNoScrim(t *testing.T) {
	for _, side := range []DrawerSide{DrawerRight, DrawerLeft, DrawerTop, DrawerBottom} {
		open := NewSignal(false)
		r := newAccessRig(func(ctx *Context) Element {
			page := Div().Size(Full, Full).Column().Child(Button("Open").Key("opener"))
			if open.Get() {
				page.Child(Drawer("Properties", Text("Body")).Side(side).NonModal().OnClose(func() { open.Set(false) }))
			}
			return page
		})
		r.focusKey(t, "opener")
		open.Set(true)
		for range 12 {
			r.frame()
		}
		if partNodeOrNil(r.rt, "Drawer.Scrim") != nil {
			t.Errorf("side %d: a non-modal drawer has a scrim", side)
		}
		if partNodeOrNil(r.rt, "Drawer.Panel") == nil {
			t.Errorf("side %d: the drawer is not built", side)
		}
	}
}

// TestModalOverNonModalTrapsFocus checks that a modal opened over a non-modal
// dialog traps Tab, and that a press where the modal is does not reach the window
// behind it.
func TestModalOverNonModalTrapsFocus(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.modal.Set(true)
	for range 4 {
		c.r.frame()
	}
	c.r.focusKey(t, "ok")
	c.r.press(input.KeyTab, 0)
	if got := c.r.focusedKey(); got != "ok" {
		t.Fatalf("Tab in the modal went to %q, want the modal's own button", got)
	}
	c.r.clickKey(t, "ok")
	if c.modalDone != 1 {
		t.Fatalf("OK in the modal ran %d times", c.modalDone)
	}
	c.r.clickKey(t, "behind")
	if c.behind != 0 {
		t.Fatal("a press on the window behind reached it under a modal")
	}
}

// TestModalOverNonModalEscapeClosesModalFirst checks that Escape, with focus in a
// modal opened over a non-modal dialog, closes the modal and leaves the dialog.
func TestModalOverNonModalEscapeClosesModalFirst(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.modal.Set(true)
	for range 4 {
		c.r.frame()
	}
	c.r.focusKey(t, "ok")
	c.r.press(input.KeyEscape, 0)
	if c.modalCloses != 1 || c.modal.Peek() {
		t.Fatalf("Escape closed the modal %d times, open %v", c.modalCloses, c.modal.Peek())
	}
	if c.closes != 0 || !c.open.Peek() {
		t.Fatal("Escape closed the non-modal dialog too")
	}
}

// TestNonModalEscapeNeedsFocusInside checks that Escape closes a non-modal dialog
// only while focus is inside it: a press that leaves focus in the window behind
// does not close it.
func TestNonModalEscapeNeedsFocusInside(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.r.clickKey(t, "behind")
	c.r.press(input.KeyEscape, 0)
	if c.closes != 0 || !c.open.Peek() {
		t.Fatalf("Escape with focus outside closed the dialog %d times", c.closes)
	}
	c.r.focusKey(t, "find")
	c.r.press(input.KeyEscape, 0)
	if c.closes != 1 || c.open.Peek() {
		t.Fatalf("Escape with focus inside closed the dialog %d times, open %v", c.closes, c.open.Peek())
	}
}

// TestNonModalEscapeClosesPopoverFirst checks that a popover open inside a
// non-modal dialog takes Escape before the dialog does.
func TestNonModalEscapeClosesPopoverFirst(t *testing.T) {
	c := newNonModalTest(func(c *nonModalTest) *ModalElement {
		return Modal("Find", Popover(Button("Filters").Key("filters"), func() Element {
			return Button("Apply").Key("apply")
		})).NonModal().OnClose(func() { c.closes++; c.open.Set(false) })
	})
	c.show(t)
	c.r.clickKey(t, "filters")
	if _, ok := c.r.rt.Find("apply"); !ok {
		t.Fatal("the popover did not open")
	}
	c.r.press(input.KeyEscape, 0)
	if _, ok := c.r.rt.Find("apply"); ok || c.closes != 0 || !c.open.Peek() {
		t.Fatalf("the first Escape did not close the popover alone: dialog closed %d times", c.closes)
	}
	c.r.press(input.KeyEscape, 0)
	if c.closes != 1 {
		t.Fatalf("the second Escape closed the dialog %d times, want 1", c.closes)
	}
}

// TestF6CyclesWindowAndDialog checks that F6 moves focus from the window into the
// open non-modal dialog and back, and that Shift+F6 steps the other way.
func TestF6CyclesWindowAndDialog(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.r.clickKey(t, "behind")
	for _, want := range []string{"find", "behind", "find"} {
		c.r.press(input.KeyF6, 0)
		if got := c.r.focusedKey(); got != want {
			t.Fatalf("F6 put focus on %q, want %q", got, want)
		}
	}
	c.r.press(input.KeyF6, input.ModShift)
	if got := c.r.focusedKey(); got != "behind" {
		t.Fatalf("Shift+F6 put focus on %q, want the window", got)
	}
}

// TestF6EntersDialogAtLastFocus checks that F6 returns to the element in a
// non-modal dialog that had focus last, not to its first field.
func TestF6EntersDialogAtLastFocus(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.r.clickKey(t, "behind")
	c.r.press(input.KeyF6, 0)
	c.r.focusKey(t, "done")
	c.r.press(input.KeyF6, 0)
	c.r.press(input.KeyF6, 0)
	if got := c.r.focusedKey(); got != "done" {
		t.Fatalf("F6 back into the dialog put focus on %q, want its last element", got)
	}
}

// TestF6CyclesToastRegion checks that, with a toast shown, F6 goes from the window
// to the dialog, then to the toast region, then back to the window.
func TestF6CyclesToastRegion(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.toasts = true
	c.show(t)
	c.r.rt.toast(ToastInfo, "Saved")
	c.r.frame()
	c.r.clickKey(t, "behind")
	for _, want := range []string{"find", "jamb.toasts", "behind"} {
		c.r.press(input.KeyF6, 0)
		if got := c.r.focusedKey(); got != want {
			t.Fatalf("F6 put focus on %q, want %q", got, want)
		}
	}
}

// TestF6WithModalOpenStaysInModal checks that F6 does not leave a modal for the
// non-modal dialog under it.
func TestF6WithModalOpenStaysInModal(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.show(t)
	c.modal.Set(true)
	for range 4 {
		c.r.frame()
	}
	c.r.focusKey(t, "ok")
	c.r.press(input.KeyF6, 0)
	if got := c.r.focusedKey(); got != "ok" {
		t.Fatalf("F6 in a modal put focus on %q", got)
	}
}

// TestNonModalEnterRunsDefaultOnlyInside checks that Enter runs a non-modal
// dialog's default button only while focus is inside the dialog.
func TestNonModalEnterRunsDefaultOnlyInside(t *testing.T) {
	c := newNonModalTest(func(c *nonModalTest) *ModalElement {
		done := Button("Done").Key("done").OnClick(func() { c.done++ })
		return Modal("Find", TextInput(NewSignal("")).Label("Find text").Key("find")).
			NonModal().Footer(done).Default(done).OnClose(func() { c.closes++; c.open.Set(false) })
	})
	c.show(t)
	c.r.focusKey(t, "find")
	c.r.press(input.KeyEnter, 0)
	if c.done != 1 {
		t.Fatalf("Enter inside the dialog ran the default %d times, want 1", c.done)
	}
	c.r.focusKey(t, "behind")
	c.r.press(input.KeyEnter, 0)
	if c.done != 1 || c.behind != 1 {
		t.Fatalf("Enter in the window behind ran the default (%d) or not the button (%d)", c.done, c.behind)
	}
}
