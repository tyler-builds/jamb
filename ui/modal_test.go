package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
)

// modalOpts varies the modal the rig builds.
type modalOpts struct {
	noField       bool // the body has nothing focusable
	area          bool // the body's field is a TextArea
	backdrop      bool // CloseOnBackdrop
	disableCreate bool // the default button is disabled
	initialCreate bool // InitialFocus names the default button
}

// modalTest is a window with an opener and an outside element, and a modal that
// is shown while open is set. It counts what the buttons and OnClose did.
type modalTest struct {
	r                        *keyRig
	open                     *Signal[bool]
	name                     *Signal[string]
	opener, outside, field   *FocusHandle
	cancelH, createH         *FocusHandle
	cancels, creates, closes int
}

func newModalTest(o modalOpts) *modalTest {
	m := &modalTest{open: NewSignal(false), name: NewSignal("")}
	m.opener, m.outside = NewFocusHandle(), NewFocusHandle()
	m.field, m.cancelH, m.createH = NewFocusHandle(), NewFocusHandle(), NewFocusHandle()
	m.r = newKeyRig(func(ctx *Context) Element {
		page := Div().Size(Full, Full).Column().
			Child(Div().FocusHandle(m.opener).Child(Text("opener"))).
			Child(Div().FocusHandle(m.outside).Child(Text("outside")))
		if !m.open.Get() {
			return page
		}
		var body Element = TextInput(m.name).FocusHandle(m.field)
		switch {
		case o.noField:
			body = Text("Nothing here to type in")
		case o.area:
			body = TextArea(m.name).FocusHandle(m.field)
		}
		cancel := Button("Cancel").Secondary().OnClick(func() { m.cancels++ })
		cancel.handle = m.cancelH
		create := Button("Create").OnClick(func() { m.creates++ })
		create.handle = m.createH
		create.disabled = o.disableCreate
		mod := Modal("New project", body).Footer(cancel, create).Default(create).
			OnClose(func() { m.closes++; m.open.Set(false) })
		if o.backdrop {
			mod.CloseOnBackdrop()
		}
		if o.initialCreate {
			mod.InitialFocus(m.createH)
		}
		return page.Child(mod)
	})
	return m
}

// show opens the modal and builds it.
func (m *modalTest) show() {
	m.open.Set(true)
	m.r.frame()
}

// focusedIn names the element with focus among the modal's, or "" if none.
func (m *modalTest) focusedIn() string {
	switch {
	case m.field.Focused():
		return "field"
	case m.cancelH.Focused():
		return "cancel"
	case m.createH.Focused():
		return "create"
	case m.outside.Focused():
		return "outside"
	case m.opener.Focused():
		return "opener"
	}
	return ""
}

// Opening focuses the first field of the body. Tab then cycles through the
// modal's fields and buttons only, and Shift+Tab walks back.
func TestModalTrapsTabInside(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.opener.FocusNow()
	m.r.frame()
	m.show()
	if got := m.focusedIn(); got != "field" {
		t.Fatalf("after opening, focus is on %q; want the first field", got)
	}
	var seen []string
	for range 6 {
		m.r.key(input.KeyTab, 0)
		seen = append(seen, m.focusedIn())
	}
	want := []string{"cancel", "create", "field", "cancel", "create", "field"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("Tab order %v, want %v", seen, want)
	}
	m.r.key(input.KeyTab, input.ModShift)
	if got := m.focusedIn(); got != "create" {
		t.Errorf("Shift+Tab from the field went to %q, want create", got)
	}
}

// Escape closes the modal once, through its OnClose, as it closes an overlay.
func TestModalEscapeCloses(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.show()
	m.r.key(input.KeyEscape, 0)
	if m.closes != 1 {
		t.Fatalf("Escape ran OnClose %d times, want 1", m.closes)
	}
	if m.open.Peek() {
		t.Error("OnClose did not clear the open state")
	}
}

// Enter in a single-line field runs the default button, as it has no OnSubmit
// of its own.
func TestModalEnterRunsDefault(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.show()
	m.r.key(input.KeyEnter, 0)
	if m.creates != 1 || m.cancels != 0 {
		t.Errorf("Enter in a field: creates %d, cancels %d; want 1, 0", m.creates, m.cancels)
	}
}

// Enter on a focused button presses that button, not the default.
func TestModalEnterPressesFocusedButton(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.show()
	m.r.key(input.KeyTab, 0) // from the field to Cancel
	m.r.key(input.KeyEnter, 0)
	if m.cancels != 1 || m.creates != 0 {
		t.Errorf("Enter on Cancel: cancels %d, creates %d; want 1, 0", m.cancels, m.creates)
	}
}

// Enter in a multi-line field is a newline, and does not run the default.
func TestModalEnterInTextAreaIsNewline(t *testing.T) {
	m := newModalTest(modalOpts{area: true})
	m.show()
	m.r.rt.TextInput("a")
	m.r.key(input.KeyEnter, 0)
	m.r.rt.TextInput("b")
	m.r.frame()
	if m.creates != 0 {
		t.Errorf("Enter in a text area ran the default button %d times", m.creates)
	}
	if got := m.name.Peek(); got != "a\nb" {
		t.Errorf("text area holds %q, want a newline between a and b", got)
	}
}

// A disabled default button does not run on Enter.
func TestModalDisabledDefaultDoesNothing(t *testing.T) {
	m := newModalTest(modalOpts{disableCreate: true})
	m.show()
	m.r.key(input.KeyEnter, 0)
	if m.creates != 0 {
		t.Errorf("Enter ran a disabled default button %d times", m.creates)
	}
}

// A press on the backdrop closes the modal only when CloseOnBackdrop is set.
func TestModalBackdropClick(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.show()
	m.r.click(5, 5)
	if m.closes != 0 {
		t.Errorf("a backdrop press closed a modal without CloseOnBackdrop")
	}

	m = newModalTest(modalOpts{backdrop: true})
	m.show()
	m.r.click(5, 5)
	if m.closes != 1 {
		t.Errorf("a backdrop press ran OnClose %d times, want 1", m.closes)
	}
}

// A press on the panel is not a press on the backdrop, even with
// CloseOnBackdrop. The panel is centered, so its center is inside it.
func TestModalPanelPressKeepsItOpen(t *testing.T) {
	m := newModalTest(modalOpts{backdrop: true})
	m.show()
	m.r.click(250, 200)
	if m.closes != 0 {
		t.Errorf("a press on the panel ran OnClose %d times", m.closes)
	}
	if got := m.focusedIn(); got == "outside" || got == "opener" {
		t.Errorf("a press on the panel left the modal: focus on %q", got)
	}
}

// Closing the modal returns focus to the element that had it when it opened.
func TestModalReturnsFocusOnClose(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.opener.FocusNow()
	m.r.frame()
	m.show()
	if m.focusedIn() != "field" {
		t.Fatalf("focus %q after opening, want the field", m.focusedIn())
	}
	m.open.Set(false)
	m.r.frame()
	if !m.opener.Focused() {
		t.Errorf("after closing, focus is on %q, want the opener", m.focusedIn())
	}
}

// Focus that has moved away from the modal when it closes is left where it is.
func TestModalCloseKeepsFocusMovedAway(t *testing.T) {
	m := newModalTest(modalOpts{})
	m.opener.FocusNow()
	m.r.frame()
	m.show()
	m.outside.FocusNow() // the app moved focus out of the modal
	m.r.frame()
	m.open.Set(false)
	m.r.frame()
	if !m.outside.Focused() {
		t.Errorf("closing the modal moved focus from the outside element to %q", m.focusedIn())
	}
}

// Without a focusable field in the body, the default button takes focus.
func TestModalDefaultButtonFocusedWithoutFields(t *testing.T) {
	m := newModalTest(modalOpts{noField: true})
	m.opener.FocusNow()
	m.r.frame()
	m.show()
	if got := m.focusedIn(); got != "create" {
		t.Errorf("with no field in the body, focus is on %q; want the default button", got)
	}
}

// InitialFocus names the element that takes focus when the modal opens.
func TestModalInitialFocusOption(t *testing.T) {
	m := newModalTest(modalOpts{initialCreate: true})
	m.show()
	if got := m.focusedIn(); got != "create" {
		t.Errorf("InitialFocus named the default button; focus is on %q", got)
	}
}

// The panel is centered over its anchor, and stays inside the view.
func TestOverCenterPlacement(t *testing.T) {
	view := scene.Rect{X: 0, Y: 0, W: 500, H: 400}
	x, y := placeOverlay(OverCenter, 0, view, 420, 200, view)
	if x != 40 || y != 100 {
		t.Errorf("centered panel at (%v, %v), want (40, 100)", x, y)
	}
	x, y = placeOverlay(OverCenter, 0, view, 600, 500, view)
	if x != overlayMargin || y != overlayMargin {
		t.Errorf("oversized panel at (%v, %v), want the margin (%v, %v)", x, y, overlayMargin, overlayMargin)
	}
}

// While a modal is open, the app's bindings and key handlers outside it don't
// run behind it, but typing in its field still does. Once it closes, they run
// again.
func TestModalBlocksShortcutsOutsideIt(t *testing.T) {
	newAct := NewAction("test.new", "New")
	open, name := NewSignal(false), NewSignal("")
	hits, outsideKeys := 0, 0
	r := newKeyRig(func(ctx *Context) Element {
		page := Div().Size(Full, Full).
			OnAction(newAct, func() { hits++ }).
			OnKeyDown(func(*KeyEvent) { outsideKeys++ })
		if !open.Get() {
			return page
		}
		return page.Child(Modal("New project", TextInput(name)).
			OnClose(func() { open.Set(false) }))
	})
	r.rt.AddBindings(Bind("ctrl+n", newAct))
	r.frame()
	open.Set(true)
	r.frame()

	r.key(input.KeyN, input.ModCtrl)
	r.key(input.KeyX, 0)
	r.rt.TextInput("ab")
	r.frame()
	if hits != 0 || outsideKeys != 0 {
		t.Errorf("with the modal open: action ran %d times, outside OnKeyDown %d times; want 0, 0", hits, outsideKeys)
	}
	if name.Peek() != "ab" {
		t.Errorf("field inside the modal = %q; want typing to reach it (%q)", name.Peek(), "ab")
	}

	r.key(input.KeyEscape, 0)
	r.frame()
	if open.Peek() {
		t.Fatal("Escape did not close the modal")
	}
	r.key(input.KeyN, input.ModCtrl)
	if hits != 1 {
		t.Errorf("after closing: action ran %d times; want 1", hits)
	}
}
