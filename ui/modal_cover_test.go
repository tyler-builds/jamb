package ui_test

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// The places a Modal can be declared in the window of coverApp: inside the
// toolbar, next to the button that opens it, as the root's first child, and as the
// root's last child.
const (
	inToolbar = "toolbar"
	rootFirst = "root first"
	rootLast  = "root last"
)

var placements = []string{inToolbar, rootFirst, rootLast}

// coverApp is a window with a 40 px toolbar, an editor under it and a status bar,
// and a Modal declared at place. It counts the clicks that reach the window's
// elements.
type coverApp struct {
	open                    *ui.Signal[bool]
	backdrop                bool // CloseOnBackdrop
	editorClicks, newClicks int
	barPresses              int
}

func (a *coverApp) modal() ui.Element {
	if !a.open.Get() {
		return nil
	}
	srv := ui.NewSignal("localhost")
	connect := ui.Button("Connect").OnClick(func() { a.open.Set(false) })
	m := ui.Modal("Connect to Server", ui.TextInput(srv).Label("Server name")).
		Footer(ui.Button("Cancel").Secondary().OnClick(func() { a.open.Set(false) }), connect).
		Default(connect).OnClose(func() { a.open.Set(false) }).Key("connect")
	if a.backdrop {
		m.CloseOnBackdrop()
	}
	return m
}

func (a *coverApp) root(place string) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		bar := ui.Div().Key("toolbar").Row().Gap(8).Padding(6).Height(ui.Px(40)).
			Role(access.RoleToolBar).Label("Main").
			OnMouseDown(func(*ui.MouseEvent) { a.barPresses++ }).
			Children(
				ui.Button("Connect...").Small().OnClick(func() { a.open.Set(true) }),
				ui.Button("New Query").Small().Secondary().OnClick(func() { a.newClicks++ }))
		if place == inToolbar {
			bar.Child(a.modal())
		}
		root := ui.Div().Key("root").Column()
		if place == rootFirst {
			root.Child(a.modal())
		}
		root.Children(bar,
			ui.Div().Key("editor").Grow(1).Padding(8).Background(ctx.Theme().Surface).
				OnClick(func() { a.editorClicks++ }).
				Child(ui.Text("SELECT * FROM dbo.Customers;").Mono()),
			ui.Div().Key("status").Height(ui.Px(24)).Padding(4).Child(ui.Text("Ready")))
		if place == rootLast {
			root.Child(a.modal())
		}
		return root
	}
}

// openCover builds coverApp with the modal at place, and opens it with the
// toolbar's button.
func openCover(t *testing.T, place string, backdrop bool) (*uitest.UI, *coverApp) {
	t.Helper()
	a := &coverApp{open: ui.NewSignal(false), backdrop: backdrop}
	u := uitest.New(t, a.root(place), uitest.Size(800, 500), uitest.Theme(ui.DarkTheme()))
	u.ByRole(access.RoleButton, "Connect...").Click()
	u.Settle()
	if !u.HasRole(access.RoleDialog, "Connect to Server") {
		t.Fatalf("%s: the modal did not open", place)
	}
	return u, a
}

// press presses and releases the left button at (x, y).
func press(u *uitest.UI, x, y float32) {
	rt := u.Runtime()
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	u.Frame()
}

// A Modal's backdrop covers the whole window and its panel is centered in it,
// wherever the modal is declared: inside a 40 px toolbar, as the root's first
// child, or as its last child, the documented place, which looks as it did.
func TestModalCoversTheWindowWhereverDeclared(t *testing.T) {
	shots := map[string][]byte{}
	for _, place := range placements {
		u, _ := openCover(t, place, false)
		scrim := u.Get("connect").Style()
		if want := (ui.Rect{W: 800, H: 500}); scrim.Bounds != want {
			t.Errorf("%s: the backdrop is drawn at %v, want the window %v", place, scrim.Bounds, want)
		}
		if scrim.Background.A == 0 {
			t.Errorf("%s: the backdrop has no fill", place)
		}
		p := u.ByRole(access.RoleDialog, "Connect to Server").Bounds()
		if cx, cy := p.X+p.W/2, p.Y+p.H/2; abs(cx-400) > 1 || abs(cy-250) > 1 {
			t.Errorf("%s: the panel %v is centered at (%v, %v), want the window's center (400, 250)", place, p, cx, cy)
		}
		if f := u.Focused(); f == nil || f.Name() != "Server name" {
			t.Errorf("%s: focus is not in the modal's field", place)
		}
		shots[place] = u.Screenshot().Pix
	}
	for _, place := range []string{inToolbar, rootFirst} {
		if !bytes.Equal(shots[place], shots[rootLast]) {
			t.Errorf("a modal declared %s is drawn differently from one declared as the root's last child", place)
		}
	}
}

// A press outside the panel reaches the backdrop only: not the editor under it,
// not a button of the toolbar the modal is declared in, nor the toolbar's own
// handlers. It closes the modal when CloseOnBackdrop is set, and does nothing
// otherwise.
func TestModalBackdropTakesThePresses(t *testing.T) {
	for _, place := range placements {
		for _, backdrop := range []bool{false, true} {
			u, a := openCover(t, place, backdrop)
			barBefore := a.barPresses
			u.Get("editor").Click()
			nq := u.ByRole(access.RoleButton, "New Query").Bounds()
			press(u, nq.X+nq.W/2, nq.Y+nq.H/2)
			if a.editorClicks != 0 || a.newClicks != 0 || a.barPresses != barBefore {
				t.Errorf("%s, CloseOnBackdrop %v: presses on the backdrop reached the window: editor %d, New Query %d, toolbar %d",
					place, backdrop, a.editorClicks, a.newClicks, a.barPresses-barBefore)
			}
			if a.open.Peek() == backdrop {
				t.Errorf("%s, CloseOnBackdrop %v: after a press on the backdrop the modal is open: %v", place, backdrop, a.open.Peek())
			}
		}
	}
}

// A press in the panel stays in the panel: it does not reach the toolbar the modal
// is declared in, and does not close the modal.
func TestModalPanelPressStaysInside(t *testing.T) {
	u, a := openCover(t, inToolbar, true)
	before := a.barPresses
	p := u.ByRole(access.RoleDialog, "Connect to Server").Bounds()
	press(u, p.X+10, p.Y+10)
	if a.barPresses != before || !a.open.Peek() {
		t.Errorf("a press in the panel reached the toolbar (%d) or closed the modal (open %v)", a.barPresses-before, a.open.Peek())
	}
}

// Escape closes a modal declared anywhere, and focus returns to the button that
// opened it. Tab stays inside the modal.
func TestModalFocusWhereverDeclared(t *testing.T) {
	for _, place := range placements {
		u, a := openCover(t, place, false)
		for range 5 {
			u.Press("tab")
			if f := u.Focused(); f == nil || f.Name() == "Connect..." || f.Name() == "New Query" {
				t.Fatalf("%s: Tab left the modal", place)
			}
		}
		u.Press("escape")
		u.Settle()
		if a.open.Peek() {
			t.Fatalf("%s: Escape did not close the modal", place)
		}
		if f := u.Focused(); f == nil || f.Name() != "Connect..." {
			t.Errorf("%s: focus did not return to the opener", place)
		}
	}
}

// The accessibility tree has the modal as a dialog of the window, after the
// window's other content, wherever it is declared, and the window's elements stay
// in it as they are.
func TestModalAccessWhereverDeclared(t *testing.T) {
	for _, place := range placements {
		u, _ := openCover(t, place, false)
		tree := u.Tree()
		var dialog *access.Node
		tree.Walk(func(n *access.Node) bool {
			if n.Role == access.RoleDialog {
				dialog = n
			}
			return true
		})
		if dialog == nil || dialog.Parent != access.RootID {
			t.Errorf("%s: the dialog is not a child of the window: %+v", place, dialog)
			continue
		}
		if kids := tree.Node(access.RootID).Children; kids[len(kids)-1] != dialog.ID {
			t.Errorf("%s: the dialog is not the last of the window's children: %v, dialog %v", place, kids, dialog.ID)
		}
		if !u.ByRole(access.RoleToolBar, "Main").HasRole(access.RoleButton, "New Query") {
			t.Errorf("%s: the toolbar's button left the toolbar", place)
		}
		if u.ByRole(access.RoleToolBar, "Main").HasRole(access.RoleDialog, "Connect to Server") {
			t.Errorf("%s: the dialog is inside the toolbar", place)
		}
		if !u.ByRole(access.RoleDialog, "Connect to Server").HasRole(access.RoleButton, "Connect") {
			t.Errorf("%s: the dialog lost its buttons", place)
		}
	}
}

// A modal takes the root's text style, not that of the element it is declared in.
func TestModalTakesTheRootsLook(t *testing.T) {
	look := func(place string) ui.DrawnStyle {
		open := ui.NewSignal(true)
		m := func() ui.Element {
			return ui.Modal("Settings", ui.Text("Body").Key("body")).OnClose(func() { open.Set(false) })
		}
		u := uitest.New(t, func(ctx *ui.Context) ui.Element {
			bar := ui.Div().Height(ui.Px(40)).TextColor(ui.RGB(255, 0, 0)).FontSize(30).Opacity(0.5).Child(ui.Text("bar"))
			root := ui.Div().Column().Child(bar)
			if place == inToolbar {
				bar.Child(m())
			} else {
				root.Child(m())
			}
			return root
		}, uitest.Size(800, 500))
		u.Settle()
		return u.Get("body").Style()
	}
	in, last := look(inToolbar), look(rootLast)
	if in.TextColor != last.TextColor || in.Opacity != last.Opacity || in.Bounds != last.Bounds {
		t.Errorf("a modal in a red, 30 px, half transparent toolbar draws its body as %+v, want %+v", in, last)
	}
}

// A modal declared in a scrolled container, out of view, still covers the window,
// and opening it does not scroll the container.
func TestModalInScrolledContainer(t *testing.T) {
	open := ui.NewSignal(false)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		list := ui.Div().Key("list").Column().Height(ui.Px(200)).ScrollY()
		for range 30 {
			list.Child(ui.Div().Height(ui.Px(30)).Child(ui.Text("row")))
		}
		list.Child(ui.Div().Key("anchor").Height(ui.Px(30)).Child(
			ui.If(open.Get(), ui.Modal("Rename", ui.TextInput(ui.NewSignal("x")).Label("Name")).
				OnClose(func() { open.Set(false) }).Key("rename"))))
		return ui.Div().Column().Child(ui.Button("Rename").OnClick(func() { open.Set(true) })).Child(list)
	}, uitest.Size(800, 500))
	before := u.Get("anchor").Bounds()
	u.ByRole(access.RoleButton, "Rename").Click()
	u.Settle()
	if r := u.Get("rename").Bounds(); r != (ui.Rect{W: 800, H: 500}) {
		t.Errorf("the backdrop of a modal declared out of view is at %v, want the window", r)
	}
	p := u.ByRole(access.RoleDialog, "Rename").Bounds()
	if cy := p.Y + p.H/2; abs(cy-250) > 1 {
		t.Errorf("the panel is centered at y %v, want 250", cy)
	}
	if f := u.Focused(); f == nil || f.Name() != "Name" {
		t.Errorf("focus is not in the modal's field")
	}
	if after := u.Get("anchor").Bounds(); after != before {
		t.Errorf("opening the modal scrolled the container: the anchor moved from %v to %v", before, after)
	}
}

// Of two open modals, the one opened last is on top and takes the input, even when
// it is declared before the other one.
func TestModalNewestOnTop(t *testing.T) {
	first, second := ui.NewSignal(false), ui.NewSignal(false)
	others := 0
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		bar := ui.Div().Height(ui.Px(40)).Child(ui.Button("Open").OnClick(func() { first.Set(true) }))
		// The second modal is declared in the toolbar, before the first one.
		done := ui.Button("Done").OnClick(func() { second.Set(false) })
		bar.Child(ui.If(second.Get(), ui.Modal("Second", ui.Text("second")).
			Footer(done).Default(done).OnClose(func() { second.Set(false) }).Key("second")))
		next := ui.Button("Next").OnClick(func() { second.Set(true) })
		// The first modal is wider than the second, so its Other button is
		// outside the second one's panel.
		return ui.Div().Column().Child(bar).
			Child(ui.If(first.Get(), ui.Modal("First", ui.Button("Other").OnClick(func() { others++ })).
				Width(ui.Px(760)).Footer(next).Default(next).OnClose(func() { first.Set(false) }).Key("first")))
	}, uitest.Size(800, 500))
	u.ByRole(access.RoleButton, "Open").Click()
	u.Settle()
	other := u.ByRole(access.RoleButton, "Other").Bounds()
	u.ByRole(access.RoleButton, "Next").Click()
	u.Settle()
	if f := u.Focused(); f == nil || f.Name() != "Done" {
		t.Fatalf("focus is not in the second modal")
	}
	// A press on the first modal's button lands on the second modal's backdrop,
	// which is above it, and one in the second modal's panel stays there.
	press(u, other.X+other.W/2, other.Y+other.H/2)
	p := u.ByRole(access.RoleDialog, "Second").Bounds()
	press(u, p.X+10, p.Y+10)
	if others != 0 || !second.Peek() || !first.Peek() {
		t.Fatalf("presses over the first modal reached it (%d clicks) or closed a modal: first %v, second %v",
			others, first.Peek(), second.Peek())
	}
	u.Press("escape")
	u.Settle()
	if second.Peek() || !first.Peek() {
		t.Fatalf("Escape closed the wrong modal: first %v, second %v", first.Peek(), second.Peek())
	}
	if f := u.Focused(); f == nil || f.Name() != "Next" {
		t.Errorf("focus did not return to the first modal's button")
	}
}

// A menu opened from inside a modal declared in a toolbar is drawn above the
// modal and takes the presses.
func TestModalMenuAbove(t *testing.T) {
	open := ui.NewSignal(true)
	var picked string
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		menu := ui.DropdownMenu(ui.Button("Options"), func() []ui.MenuItem {
			return []ui.MenuItem{
				{Label: "Alpha", OnClick: func() { picked = "Alpha" }},
				{Label: "Beta", OnClick: func() { picked = "Beta" }},
			}
		})
		bar := ui.Div().Height(ui.Px(40)).Child(ui.Text("bar")).
			Child(ui.If(open.Get(), ui.Modal("Pick", menu).OnClose(func() { open.Set(false) })))
		return ui.Div().Column().Child(bar).Child(ui.Div().Grow(1))
	}, uitest.Size(800, 500))
	u.Settle()
	u.ByRole(access.RoleButton, "Options").Click()
	u.Settle()
	u.ByRole(access.RoleMenuItem, "Beta").Click()
	u.Settle()
	if picked != "Beta" {
		t.Errorf("the menu item was not clicked: picked %q", picked)
	}
	if !open.Peek() {
		t.Errorf("clicking the menu closed the modal")
	}
}

// A non-modal dialog and a drawer declared in a toolbar are placed in the window,
// as they are when declared last in the root.
func TestDialogVariantsInToolbar(t *testing.T) {
	cases := []struct {
		name  string
		build func(close func()) ui.Element
		check func(t *testing.T, p ui.Rect)
	}{
		{"non-modal", func(close func()) ui.Element {
			return ui.Modal("Find", ui.Text("find")).NonModal().OnClose(close)
		}, func(t *testing.T, p ui.Rect) {
			if cx, cy := p.X+p.W/2, p.Y+p.H/2; abs(cx-400) > 1 || abs(cy-250) > 1 {
				t.Errorf("the non-modal dialog %v is not centered in the window", p)
			}
		}},
		{"drawer", func(close func()) ui.Element {
			return ui.Drawer("Filters", ui.Text("filters")).OnClose(close)
		}, func(t *testing.T, p ui.Rect) {
			if p.X+p.W != 800 || p.Y != 0 || p.H != 500 {
				t.Errorf("the drawer %v is not against the window's right edge, full height", p)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			open := ui.NewSignal(true)
			var clicks int
			u := uitest.New(t, func(ctx *ui.Context) ui.Element {
				bar := ui.Div().Height(ui.Px(40)).Child(ui.Text("bar")).
					Child(ui.If(open.Get(), c.build(func() { open.Set(false) })))
				return ui.Div().Column().Child(bar).
					Child(ui.Div().Key("page").Grow(1).OnClick(func() { clicks++ }))
			}, uitest.Size(800, 500))
			u.Settle()
			var dialog *access.Node
			u.Tree().Walk(func(n *access.Node) bool {
				if n.Role == access.RoleDialog {
					dialog = n
				}
				return true
			})
			if dialog == nil {
				t.Fatal("no dialog")
			}
			c.check(t, u.ByRole(access.RoleDialog, dialog.Name).Bounds())
			if c.name == "non-modal" {
				// A press beside a non-modal dialog reaches the window behind it.
				press(u, 50, 400)
				if clicks != 1 {
					t.Errorf("a press beside the non-modal dialog did not reach the page: %d clicks", clicks)
				}
			}
		})
	}
}

// A modal declared in a toolbar enters and leaves with its motion, as one declared
// last in the root does: the backdrop fades in over the window and fades out after
// it closes, and nothing pops or jumps.
func TestModalMotionWhereverDeclared(t *testing.T) {
	for _, place := range []string{inToolbar, rootLast} {
		a := &coverApp{open: ui.NewSignal(false)}
		u := uitest.New(t, a.root(place), uitest.Size(800, 500))
		rec := u.Record()
		u.ByRole(access.RoleButton, "Connect...").Click()
		u.Advance(40 * time.Millisecond)
		if s := u.Get("connect").Style(); s.Opacity <= 0 || s.Opacity >= 1 || s.Bounds != (ui.Rect{W: 800, H: 500}) {
			t.Errorf("%s: 40 ms after opening, the backdrop is drawn at %v with opacity %v, want the window, fading in",
				place, s.Bounds, s.Opacity)
		}
		u.Settle()
		u.Press("escape")
		u.Advance(30 * time.Millisecond)
		if a.open.Peek() {
			t.Fatalf("%s: Escape did not close the modal", place)
		}
		closing := u.Screenshot().Pix
		u.Settle()
		if bytes.Equal(closing, u.Screenshot().Pix) {
			t.Errorf("%s: the modal disappeared at once instead of fading out", place)
		}
		rec.Check(t)
	}
}

// Under the default title bar, a modal declared in a toolbar covers the app's part of
// the window, as one declared last in the root does, and leaves the bar usable.
func TestModalUnderDefaultTitleBar(t *testing.T) {
	for _, place := range []string{inToolbar, rootLast} {
		a := &coverApp{open: ui.NewSignal(false)}
		u := uitest.New(t, a.root(place), uitest.Size(800, 500))
		u.Runtime().SetDefaultTitleBar(true, "SQL")
		u.Frame()
		u.ByRole(access.RoleButton, "Connect...").Click()
		u.Settle()
		top := float32(ui.DefaultTitleBarHeight)
		if r := u.Get("connect").Bounds(); r != (ui.Rect{Y: top, W: 800, H: 500 - top}) {
			t.Errorf("%s: the backdrop is at %v, want the window under the title bar", place, r)
		}
		p := u.ByRole(access.RoleDialog, "Connect to Server").Bounds()
		if cy := p.Y + p.H/2; abs(cy-(top+(500-top)/2)) > 1 {
			t.Errorf("%s: the panel is centered at y %v, want the middle of the window under the title bar", place, cy)
		}
		// The dialog is read after the app's content and before the title bar's
		// buttons, as a modal declared last in the root was.
		var order []string
		tree := u.Tree()
		for _, id := range tree.Node(access.RootID).Children {
			if n := tree.Node(id); n.Name == "Ready" || n.Name == "Connect to Server" || n.Name == "Minimize" {
				order = append(order, n.Name)
			}
		}
		if want := []string{"Ready", "Connect to Server", "Minimize"}; !slices.Equal(order, want) {
			t.Errorf("%s: the window's children are in the order %q, want %q", place, order, want)
		}
	}
}
