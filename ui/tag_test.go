package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// TestTagRemoveButton checks that the remove button is a focusable button named
// for the tag, and that it calls OnRemove when it is clicked, and when Enter or
// Space is pressed on it.
func TestTagRemoveButton(t *testing.T) {
	removed := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Tag("Sales").OnRemove(func() { removed++ })
	})
	accesstest.Check(t, accesstest.Find(t, r.tree(), access.RoleButton, "Remove Sales"),
		access.RoleButton, "Remove Sales", access.StateFocusable)
	clickPart(t, r, "Tag.Remove")
	if removed != 1 {
		t.Fatalf("a click calls OnRemove %d times, want 1", removed)
	}
	r.focusAccess(t, access.RoleButton, "Remove Sales")
	r.press(input.KeyEnter, 0)
	r.press(input.KeySpace, 0)
	if removed != 3 {
		t.Errorf("Enter and Space call OnRemove %d times in all, want 3", removed)
	}
}

// TestTagGroup checks that a tag is a group named by its text, and that a tag
// without OnRemove has no remove button.
func TestTagGroup(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Tag("Sales") })
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleGroup, "Sales")
	accesstest.Absent(t, tr, access.RoleButton, "Remove Sales")
}

func testTagParts(t *testing.T, applied map[string]bool) {
	applied["Tag.Tag"] = true
	t.Run("Tag.Tag", func(t *testing.T) {
		var st TagStyle
		st.Tag = Style{Background: markBG}
		r := newAccessRig(func(ctx *Context) Element { return Tag("Sales").Styles(st) })
		wantBackground(t, partNode(t, r.rt, "Tag.Tag", ""), "Tag.Tag", markBG)
	})
	applied["Tag.Label"] = true
	t.Run("Tag.Label", func(t *testing.T) {
		var st TagStyle
		st.Label = Style{TextColor: markFG}
		r := newAccessRig(func(ctx *Context) Element { return Tag("Sales").Styles(st) })
		wantTextColor(t, partNode(t, r.rt, "Tag.Label", ""), "Tag.Label", markFG)
	})
	applied["Tag.Remove"] = true
	t.Run("Tag.Remove", func(t *testing.T) {
		var st TagStyle
		st.Remove = Style{Background: markBG}
		r := newAccessRig(func(ctx *Context) Element { return Tag("Sales").OnRemove(func() {}).Styles(st) })
		wantBackground(t, partNode(t, r.rt, "Tag.Remove", ""), "Tag.Remove", markBG)
	})
}
