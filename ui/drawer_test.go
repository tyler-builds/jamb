package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// partDrawer is a drawer at side with a field and two buttons, with the
// instance style st.
func partDrawer(side DrawerSide, st DrawerStyle, closed *bool) *DrawerElement {
	ok := Button("Save").Key("ok").Primary()
	return Drawer("Settings", TextInput(NewSignal("")).Label("Name").Key("name")).
		Side(side).Footer(Button("Cancel"), ok).Default(ok).Styles(st).
		OnClose(func() {
			if closed != nil {
				*closed = true
			}
		})
}

// A drawer's panel is against its edge of the window, across the whole side.
func TestDrawerSides(t *testing.T) {
	for _, c := range []struct {
		side       DrawerSide
		x, y, w, h float32
	}{
		{DrawerRight, 800 - drawerWidth, 0, drawerWidth, 600},
		{DrawerLeft, 0, 0, drawerWidth, 600},
		{DrawerTop, 0, 0, 800, drawerHeight},
		{DrawerBottom, 0, 600 - drawerHeight, 800, drawerHeight},
	} {
		r := drawerRig(c.side, nil)
		p := rectOf(r, partNode(t, r.rt, "Drawer.Panel", ""))
		if p.X != c.x || p.Y != c.y || p.W != c.w || p.H != c.h {
			t.Errorf("side %d: panel %+v, want %v,%v %vx%v", c.side, p, c.x, c.y, c.w, c.h)
		}
	}
}

// A drawer is a modal dialog: focus moves to its field, Escape and a press on
// the backdrop close it.
func TestDrawerIsModal(t *testing.T) {
	closed := false
	r := drawerRig(DrawerRight, &closed)
	if !r.rt.focusWithin("name") {
		t.Fatal("focus did not move to the drawer's field")
	}
	accesstest.Find(t, r.tree(), access.RoleDialog, "Settings")
	r.press(input.KeyEscape, 0)
	if !closed {
		t.Fatal("Escape did not ask the drawer to close")
	}
	closed = false
	r = drawerRig(DrawerRight, &closed)
	r.clickAt(100, 300) // the backdrop, left of the panel
	if !closed {
		t.Fatal("a press on the backdrop did not ask the drawer to close")
	}
}

// testDrawerParts checks each part of a drawer.
func testDrawerParts(t *testing.T, applied map[string]bool) {
	cases := []struct {
		part string
		set  func(*DrawerStyle, Style)
	}{
		{"Drawer.Scrim", func(s *DrawerStyle, m Style) { s.Scrim = m }},
		{"Drawer.Panel", func(s *DrawerStyle, m Style) { s.Panel = m }},
		{"Drawer.Title", func(s *DrawerStyle, m Style) { s.Title = m }},
		{"Drawer.Body", func(s *DrawerStyle, m Style) { s.Body = m }},
		{"Drawer.Footer", func(s *DrawerStyle, m Style) { s.Footer = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st DrawerStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element { return Div().Column().Child(partDrawer(DrawerRight, st, nil)) })
			wantBackground(t, partNode(t, r.rt, c.part, ""), c.part, markBG)
		})
	}
}

// drawerRig is an 800x600 window that opens a drawer at side after its first
// frame, as an app does, once the drawer has slid in.
func drawerRig(side DrawerSide, closed *bool) *accessRig {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Button("Before")).Child(If(open.Get(), partDrawer(side, DrawerStyle{}, closed)))
	})
	r.rt.SetViewport(800, 600, 1)
	r.frame()
	open.Set(true)
	for range 12 {
		r.frame()
	}
	return r
}
