package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// popoverRig is a popover with a field and a button in its panel, an outside
// button, and the state of the popover's OnChange.
func popoverRig(t *testing.T, changes *[]bool) *accessRig {
	t.Helper()
	q := NewSignal("")
	return newAccessRig(func(ctx *Context) Element {
		return Div().Row().Padding(20).Gap(20).Align(Start).
			Child(Popover(Button("Filters").Key("trigger"), func() Element {
				return Div().Column().Gap(8).
					Child(TextInput(q).Label("Name").Key("name")).
					Child(Button("Apply").Key("apply"))
			}).OnChange(func(open bool) { *changes = append(*changes, open) })).
			Child(Button("Elsewhere").Key("outside"))
	})
}

// A click on the trigger opens the panel and moves focus to its first field;
// Escape closes it and returns focus to the trigger.
func TestPopoverOpensAndClosesWithFocus(t *testing.T) {
	var changes []bool
	r := popoverRig(t, &changes)
	r.clickKey(t, "trigger")
	if _, ok := r.rt.Find("name"); !ok {
		t.Fatal("the panel did not open")
	}
	if f := r.rt.byID[r.rt.focusID]; f == nil || f.editor == nil || f.editor.name != "Name" {
		t.Fatal("focus did not move to the panel's first field")
	}
	tr := accesstest.Find(t, r.tree(), access.RoleButton, "Filters")
	if !tr.States.Has(access.StateExpanded) {
		t.Error("the trigger is not reported expanded")
	}
	accesstest.Find(t, r.tree(), access.RoleDialog, "Filters")

	r.press(input.KeyEscape, 0)
	if _, ok := r.rt.Find("name"); ok {
		t.Fatal("Escape did not close the panel")
	}
	if !r.rt.focusWithin("trigger") {
		t.Fatal("focus did not return to the trigger")
	}
	if len(changes) != 2 || !changes[0] || changes[1] {
		t.Fatalf("OnChange saw %v, want [true false]", changes)
	}
}

// A click outside closes the panel and leaves focus where the click put it.
func TestPopoverClosesOnClickOutside(t *testing.T) {
	var changes []bool
	r := popoverRig(t, &changes)
	r.clickKey(t, "trigger")
	r.clickKey(t, "outside")
	if _, ok := r.rt.Find("name"); ok {
		t.Fatal("a click outside did not close the panel")
	}
	if r.rt.focusWithin("trigger") {
		t.Fatal("a click outside moved focus back to the trigger")
	}
}

// Enter on the focused trigger opens the panel.
func TestPopoverOpensFromTheKeyboard(t *testing.T) {
	var changes []bool
	r := popoverRig(t, &changes)
	r.press(input.KeyTab, 0)
	r.press(input.KeyEnter, 0)
	if _, ok := r.rt.Find("name"); !ok {
		t.Fatal("Enter on the trigger did not open the panel")
	}
}

// testPopoverParts checks the popover's one part.
func testPopoverParts(t *testing.T, applied map[string]bool) {
	applied["Popover.Panel"] = true
	t.Run("Popover.Panel", func(t *testing.T) {
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Padding(20).Child(Popover(Button("Open").Key("trigger"), func() Element { return Text("Inside") }).
				Styles(PopoverStyle{Panel: Style{Background: markBG}}))
		})
		r.clickKey(t, "trigger")
		wantBackground(t, partNode(t, r.rt, "Popover.Panel", ""), "Popover.Panel", markBG)
	})
}
