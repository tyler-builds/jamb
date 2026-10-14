package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// clickWithFrames clicks at x, y as a mouse does: the pointer arrives and a frame runs, the
// button goes down and a frame runs, and then it comes up. The frames between are where a
// hover or a focus change re-renders, which a click without them does not see.
func (r *accessRig) clickWithFrames(x, y float32) {
	r.rt.PointerMove(x, y, 0)
	r.frame()
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.frame()
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

// A click on a suggestion picks it, with frames between the pointer's steps: the press in
// the list must not blur the text, whose blur closes the list before the release.
func TestComboboxMouseClickPicksWithFrames(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	x, y := r.rowCenter(t, "Apricot")
	r.clickWithFrames(x, y)
	if r.text.Peek() != "Apricot" || len(r.picks) != 1 {
		t.Fatalf("clicking Apricot set %q with picks %v", r.text.Peek(), r.picks)
	}
	if r.expanded(t) {
		t.Error("the list stayed open after the pick")
	}
	if !r.editorFocused() {
		t.Errorf("the click moved focus to %q, want the text", r.focusedKey())
	}
}

// A click in an empty field opens the list of every suggestion, whether or not the field
// had focus: the empty text has not been asked for yet, so it is not a query without results.
func TestComboboxClickOnEmptyFieldOpensTheList(t *testing.T) {
	for _, focused := range []bool{false, true} {
		r := newComboRig(t, nil)
		if focused {
			r.focusCombo(t)
		}
		rc := r.rt.nodeRect(partNodeOrNil(r.rt, "Combobox.Text"))
		r.clickWithFrames(rc.X+rc.W/2, rc.Y+rc.H/2)
		if !r.expanded(t) {
			t.Errorf("focused first %v: the click did not open the list", focused)
		}
	}
}

// A click on a time in the open list picks it, with frames between the pointer's steps.
func TestTimePickerMouseClickPicksWithFrames(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 0), nil, nil)
	openTime(t, r)
	for range 60 { // the list scrolls the current time into view, smoothly; let it settle
		r.frame()
	}
	popup := partNodeOrNil(r.rt, "TimePicker.Popup")
	if popup == nil {
		t.Fatal("no TimePicker.Popup")
	}
	// The row under a point in the upper part of the list, found as the pointer finds it, so
	// the list's scroll counts.
	view := r.rt.nodeRect(popup)
	x, y := view.X+view.W/2, view.Y+view.H/4
	name := ""
	for _, n := range r.rt.hitTest(x, y) {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleListItem {
			name = n.div.acc.name
			break
		}
	}
	if name == "" || name == r.highlightedTime() {
		t.Fatalf("the point in the list is over row %q, highlighted %q", name, r.highlightedTime())
	}
	r.clickWithFrames(x, y)
	if got := formatClock(clockOf(r.value.Peek()), currentLocale()); len(r.changes) == 0 || got != name {
		t.Fatalf("clicking %s set %s, with %d changes", name, got, len(r.changes))
	}
	if r.timeExpanded(t) {
		t.Error("the list stayed open after the pick")
	}
}

// A press on a focused field's frame, outside its text, leaves the field focused: it is not
// blurred by the press and focused again a frame later, which would run its blur, such as a
// combo box closing its list or a date field committing its text.
func TestFieldFramePressKeepsFocus(t *testing.T) {
	text := NewSignal("hello")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Padding(20).Child(TextInput(text).Key("name").Label("Name").Width(Px(300)))
	})
	r.focusKey(t, "name")
	focused := r.rt.focusID
	frame := partNodeOrNil(r.rt, "TextInput.Frame")
	if frame == nil {
		t.Fatal("no TextInput.Frame")
	}
	rc := r.rt.nodeRect(frame)
	x, y := rc.X+2, rc.Y+rc.H/2 // the frame's left padding, outside the text
	r.rt.PointerMove(x, y, 0)
	r.frame()
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	if r.rt.focusID != focused {
		t.Error("the press on the frame moved focus away from the field")
	}
	r.frame()
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	if r.rt.focusID != focused {
		t.Error("the click on the frame left the field unfocused")
	}
}
