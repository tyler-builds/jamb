package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// menuItem returns the menu item named name of the open menu.
func menuItem(u *uitest.UI, name string) *uitest.Element {
	return u.ByRole(access.RoleMenuItem, name)
}

// rightClickAt presses and releases the right button at (x, y) of the window.
func rightClickAt(u *uitest.UI, x, y float32) {
	rt := u.Runtime()
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	rt.PointerUp(x, y, input.ButtonRight, 0)
	u.Frame()
}

// cardMenu is the menu of a card around the content of a test: one item, "Card action".
func cardMenu(log *[]string) func() []ui.MenuItem {
	return func() []ui.MenuItem {
		return []ui.MenuItem{{Label: "Card action", OnClick: func() { *log = append(*log, "card") }}}
	}
}

// A link's context menu opens on a right click on the link, over the menu of the card
// around it, and with Shift+F10 while the link has focus; focus comes back to the link
// when it closes. A right click on the text beside the link opens the card's menu.
func TestLinkContextMenu(t *testing.T) {
	var log []string
	linkMenu := func() []ui.MenuItem {
		return []ui.MenuItem{{Label: "Copy link", OnClick: func() { log = append(log, "copy link") }}}
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(20).Child(ui.Div().Key("card").Focusable().ContextMenu(cardMenu(&log)).Child(
			ui.Rich(ui.Link("docs", func() {}).ContextMenu(linkMenu),
				ui.Seg(" and some plain text that follows the link")).Key("text")))
	})
	// The link's access bounds are those of its whole text, so the press goes to its start.
	b := u.Get("text").Bounds()
	rightClickAt(u, b.X+4, b.Y+b.H/2)
	if u.HasRole(access.RoleMenuItem, "Card action") {
		t.Fatal("a right click on the link opened the card's menu")
	}
	menuItem(u, "Copy link").Click()
	if len(log) != 1 || log[0] != "copy link" {
		t.Fatalf("log %v after choosing Copy link, want [copy link]", log)
	}

	u.Press("tab") // the card
	u.Press("tab") // the link
	if f := u.Focused().Node(); f.Role != access.RoleLink {
		t.Fatalf("Tab: focus on the %s %q, want the link", f.Role, f.Name)
	}
	u.Press("shift+f10")
	if !u.HasRole(access.RoleMenuItem, "Copy link") {
		t.Fatal("Shift+F10 on the focused link did not open its menu")
	}
	u.Press("escape")
	if f := u.Focused().Node(); f.Role != access.RoleLink || f.Name != "docs" {
		t.Errorf("after Escape: focus on the %s %q, want the link back", f.Role, f.Name)
	}

	rightClickAt(u, b.X+b.W-4, b.Y+b.H/2)
	if !u.HasRole(access.RoleMenuItem, "Card action") {
		t.Error("a right click on the text beside the link did not open the card's menu")
	}
}

// A link made of several segments opens the menu set on any one of them.
func TestLinkSegsShareTheMenu(t *testing.T) {
	menu := func() []ui.MenuItem { return []ui.MenuItem{{Label: "Open in new window", OnClick: func() {}}} }
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(20).Child(ui.Rich(
			ui.LinkSegs(func() {}, ui.Seg("the "), ui.Seg("long bold guide").Bold().ContextMenu(menu))...).Key("text"))
	})
	b := u.Get("text").Bounds()
	rightClickAt(u, b.X+4, b.Y+b.H/2) // on "the ", which has no menu of its own
	if !u.HasRole(access.RoleMenuItem, "Open in new window") {
		t.Error("a right click on a later segment of the link did not open the link's menu")
	}
}

// A text field's context menu has Undo, Redo, Cut, Copy, Paste and Select all, and wins
// over the menu of an element around it. A right click focuses the field, keeps a
// selection it lands in, and moves the caret when it lands outside.
func TestTextFieldContextMenu(t *testing.T) {
	var log []string
	v := ui.NewSignal("alpha beta")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(20).Column().Gap(8).
			Child(ui.Button("Other").Key("other")).
			Child(ui.Div().ContextMenu(cardMenu(&log)).Child(ui.TextInput(v).Label("Name").Width(ui.Px(300))))
	})
	u.Get("other").Focus()
	field := u.ByRole(access.RoleTextField, "Name")
	field.RightClick()
	if u.HasRole(access.RoleMenuItem, "Card action") {
		t.Fatal("a right click on the field opened the card's menu")
	}
	for name, disabled := range map[string]bool{
		"Undo": true, "Redo": true, "Cut": true, "Copy": true, "Paste": false, "Select all": false,
	} {
		if got := menuItem(u, name).Disabled(); got != disabled {
			t.Errorf("%s disabled = %v, want %v", name, got, disabled)
		}
	}
	menuItem(u, "Select all").Click()
	if !field.Focused() {
		t.Fatal("focus did not come back to the field after Select all")
	}

	field.RightClick() // past the end of the text, inside the selection: it stays
	if menuItem(u, "Copy").Disabled() {
		t.Fatal("Copy is disabled while the whole text is selected")
	}
	menuItem(u, "Copy").Click()
	if got := u.Clipboard(); got != "alpha beta" {
		t.Errorf("clipboard %q after Copy, want %q", got, "alpha beta")
	}

	u.Press("home")
	u.SetClipboard("!")
	field.RightClick() // outside the empty selection: the caret moves to the press, the end
	menuItem(u, "Paste").Click()
	if got := v.Peek(); got != "alpha beta!" {
		t.Fatalf("value %q after Paste, want %q", got, "alpha beta!")
	}
	field.RightClick()
	menuItem(u, "Undo").Click()
	if got := v.Peek(); got != "alpha beta" {
		t.Errorf("value %q after Undo, want %q", got, "alpha beta")
	}
	if len(log) != 0 {
		t.Errorf("the card's menu ran: %v", log)
	}
}

// A read-only field's menu has only Copy and Select all.
func TestReadOnlyFieldContextMenu(t *testing.T) {
	v := ui.NewSignal("fixed")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(20).Child(ui.TextInput(v).Label("Name").ReadOnly(true).Width(ui.Px(300)))
	})
	u.ByRole(access.RoleTextField, "Name").RightClick()
	for _, name := range []string{"Undo", "Redo", "Cut", "Paste"} {
		if u.HasRole(access.RoleMenuItem, name) {
			t.Errorf("a read-only field's menu has %s", name)
		}
	}
	if !u.HasRole(access.RoleMenuItem, "Copy") || !u.HasRole(access.RoleMenuItem, "Select all") {
		t.Error("a read-only field's menu lacks Copy or Select all")
	}
}

// Selectable text has Copy and Select all, unless an element around it has a menu of
// its own; the selection survives the menu taking focus, so Copy copies it.
func TestSelectableTextContextMenu(t *testing.T) {
	var log []string
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(20).Column().Gap(8).
			Child(ui.Text("Error 404: page not found").Selectable().Key("err")).
			Child(ui.Div().ContextMenu(cardMenu(&log)).Child(ui.Text("inside a card").Selectable().Key("card-text")))
	})
	u.Get("err").RightClick()
	if !menuItem(u, "Copy").Disabled() {
		t.Error("Copy is enabled with nothing selected")
	}
	menuItem(u, "Select all").Click()
	u.Get("err").RightClick()
	menuItem(u, "Copy").Click()
	if got := u.Clipboard(); got != "Error 404: page not found" {
		t.Errorf("clipboard %q after Copy, want the whole text", got)
	}

	u.Get("card-text").RightClick()
	if !u.HasRole(access.RoleMenuItem, "Card action") || u.HasRole(access.RoleMenuItem, "Select all") {
		t.Error("selectable text inside a card did not give way to the card's menu")
	}
}

// The element a context menu returns focus to does not see OnBlur while the menu has
// focus, only when focus goes elsewhere.
func TestContextMenuHoldsBlur(t *testing.T) {
	blurs := 0
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		// Other sits beside the row, clear of the menu, which opens below the row.
		return ui.Div().Padding(20).Row().Gap(300).
			Child(ui.Div().Key("row").Role(access.RoleListItem).Label("Row").Size(ui.Px(200), ui.Px(30)).OnBlur(func() { blurs++ }).
				ContextMenu(func() []ui.MenuItem { return []ui.MenuItem{{Label: "Open", OnClick: func() {}}} })).
			Child(ui.Button("Other").Key("other"))
	})
	u.Get("row").Focus()
	u.Press("shift+f10")
	u.Press("escape")
	if blurs != 0 || !u.Get("row").Focused() {
		t.Fatalf("after the menu closed with Escape: %d blurs, focused %v; want none, and focus back", blurs, u.Get("row").Focused())
	}
	u.Press("shift+f10")
	u.Get("other").Click()
	if blurs != 1 {
		t.Errorf("%d blurs after a click elsewhere closed the menu, want 1", blurs)
	}
}
