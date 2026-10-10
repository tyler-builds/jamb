package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// focusedAccess returns the node that the access tree reports as focused, as a
// screen reader sees it: its role and its name.
func focusedAccess(rt *Runtime) (access.Role, string) {
	tr := rt.AccessTree()
	n := tr.Node(tr.Focus)
	if n == nil {
		return 0, ""
	}
	return n.Role, n.Name
}

func cardMenuRig(t *testing.T) *Runtime {
	t.Helper()
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(
			Div().Key("card").Size(Px(200), Px(60)).Focusable().OnClick(func() {}).
				ContextMenu(func() []MenuItem {
					return []MenuItem{
						{Label: "Open"},
						{Label: "Star"},
						{Label: "Status", Submenu: []MenuItem{{Label: "Paused"}}},
					}
				}).Child(Text("Card")))
	}, 400, 300, 1)
	cx, cy := center(t, rt, "card")
	rt.PointerMove(cx, cy, 0)
	rt.PointerDown(cx, cy, input.ButtonLeft, 1, 0)
	rt.PointerUp(cx, cy, input.ButtonLeft, 0)
	frame(rt)
	return rt
}

// Shift+F10 opens the context menu with its first item highlighted, and the
// focus the screen reader follows goes to that item. Down and Up move the focus
// with the highlight, one item at a time.
func TestContextMenuFocusFollowsKeyboardHighlight(t *testing.T) {
	rt := cardMenuRig(t)
	rt.KeyDown(input.KeyF10, input.ModShift, false)
	frame(rt)
	if role, name := focusedAccess(rt); role != access.RoleMenuItem || name != "Open" {
		t.Fatalf("after Shift+F10 the focus is %v %q, want the menu item Open", role, name)
	}
	for _, want := range []string{"Star", "Status"} {
		rt.KeyDown(input.KeyDown, 0, false)
		frame(rt)
		if role, name := focusedAccess(rt); role != access.RoleMenuItem || name != want {
			t.Errorf("after Down the focus is %v %q, want the menu item %s", role, name, want)
		}
	}
	rt.KeyDown(input.KeyUp, 0, false)
	frame(rt)
	if _, name := focusedAccess(rt); name != "Star" {
		t.Errorf("after Up the focus is %q, want Star", name)
	}
}

// A menu opened with the pointer has no highlight yet: the screen reader hears
// the menu by its name, and the first Down highlights the first item.
func TestContextMenuPointerOpenNamesTheMenu(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(
			Div().Key("card").Size(Px(200), Px(60)).Focusable().OnClick(func() {}).
				ContextMenu(func() []MenuItem {
					return []MenuItem{{Label: "Open"}, {Label: "Star"}}
				}).Child(Text("Card")))
	}, 400, 300, 1)
	cx, cy := center(t, rt, "card")
	rt.PointerMove(cx, cy, 0)
	rt.PointerDown(cx, cy, input.ButtonRight, 1, 0)
	rt.PointerUp(cx, cy, input.ButtonRight, 0)
	frame(rt)
	if role, name := focusedAccess(rt); role != access.RoleMenu || name != "Card" {
		t.Fatalf("after a right click the focus is %v %q, want the menu named Card", role, name)
	}
	rt.KeyDown(input.KeyDown, 0, false)
	frame(rt)
	if role, name := focusedAccess(rt); role != access.RoleMenuItem || name != "Open" {
		t.Errorf("after Down the focus is %v %q, want the menu item Open", role, name)
	}
}

// Escape closes the menu and returns the focus to the element it hung from.
func TestContextMenuEscapeReturnsFocusToElement(t *testing.T) {
	rt := cardMenuRig(t)
	rt.KeyDown(input.KeyF10, input.ModShift, false)
	frame(rt)
	rt.KeyDown(input.KeyEscape, 0, false)
	frame(rt)
	if _, name := focusedAccess(rt); name != "Card" {
		t.Errorf("after Escape the focus is %q, want the card's button Card", name)
	}
}

// A DropdownMenu opened from the keyboard moves the focus with its highlight,
// like a context menu does.
func TestDropdownFocusFollowsKeyboardHighlight(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(DropdownMenu(Button("Actions").Key("trigger"), func() []MenuItem {
			return []MenuItem{{Label: "Open"}, {Label: "Star"}}
		}))
	}, 400, 300, 1)
	cx, cy := center(t, rt, "trigger")
	rt.PointerMove(cx, cy, 0)
	rt.PointerDown(cx, cy, input.ButtonLeft, 1, 0)
	rt.PointerUp(cx, cy, input.ButtonLeft, 0)
	frame(rt)
	rt.KeyDown(input.KeyDown, 0, false)
	frame(rt)
	if _, name := focusedAccess(rt); name != "Open" {
		t.Errorf("after the dropdown opens and Down the focus is %q, want the menu item Open", name)
	}
}

// F10 by itself activates the menu bar, as a lone Alt tap does: the first label
// takes focus, Right moves along the bar, and Escape gives the focus back.
func TestF10ActivatesMenuBar(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Column().Child(MenuBar(
			TopMenu{Label: "File", Items: func() []MenuItem { return []MenuItem{{Label: "New"}} }},
			TopMenu{Label: "Edit", Items: func() []MenuItem { return []MenuItem{{Label: "Undo"}} }},
		)).Child(Div().Key("body").Size(Px(100), Px(40)))
	}, 400, 300, 1)
	rt.KeyDown(input.KeyF10, 0, false)
	frame(rt)
	if _, name := focusedAccess(rt); name != "File" {
		t.Fatalf("after F10 the focus is %q, want the first menu label File", name)
	}
	rt.KeyDown(input.KeyRight, 0, false)
	frame(rt)
	if _, name := focusedAccess(rt); name != "Edit" {
		t.Errorf("after Right the focus is %q, want Edit", name)
	}
	rt.KeyDown(input.KeyEscape, 0, false)
	frame(rt)
	if _, name := focusedAccess(rt); name == "File" || name == "Edit" {
		t.Errorf("after Escape the focus is still on the menu bar (%q)", name)
	}
}

// Without a menu bar, F10 does nothing special: the focus stays where it was.
func TestF10WithoutMenuBarLeavesFocus(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(Button("Save").Key("save"))
	}, 400, 300, 1)
	cx, cy := center(t, rt, "save")
	rt.PointerMove(cx, cy, 0)
	rt.PointerDown(cx, cy, input.ButtonLeft, 1, 0)
	rt.PointerUp(cx, cy, input.ButtonLeft, 0)
	frame(rt)
	before := rt.focusID
	rt.KeyDown(input.KeyF10, 0, false)
	frame(rt)
	if rt.focusID != before {
		t.Errorf("F10 moved the focus in a window without a menu bar")
	}
	if rt.menuReturn != nil {
		t.Error("F10 remembered a return focus without a menu bar")
	}
}
