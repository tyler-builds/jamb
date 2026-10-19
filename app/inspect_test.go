package app

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// inspSampleWindow makes a headless window to inspect: a labelled button, a
// check box and a card, with the host side the inspector reads.
func inspSampleWindow(t *testing.T) (*host, *ui.Runtime, inspect.Target) {
	t.Helper()
	checked := ui.NewSignal(false)
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		return ui.Div().Key("root").Column().Gap(4).Children(
			ui.Div().Key("card").Padding(8).Background(ui.Hex("#112233")).Child(ui.Text("Card")),
			ui.Button("Save").Key("save"),
			ui.Checkbox("Remember", checked),
		)
	}, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	return &host{rt: rt, handle: &appWindow{}, cfg: Window{Title: "Sample"}}, rt, tg
}

// inspFindKey returns the ID of the first element of the subtree at id with the key.
func inspFindKey(tg inspect.Target, id uint64, key string) (uint64, bool) {
	e, ok := tg.Element(id)
	if !ok {
		return 0, false
	}
	if e.Key == key {
		return id, true
	}
	for _, c := range e.Children {
		if f, ok := inspFindKey(tg, c, key); ok {
			return f, true
		}
	}
	return 0, false
}

// TestInspectorWindowShowsTheSelectedElementWithoutAccessibilityWarnings renders
// the inspector's own window with a sample runtime. Its controls are labelled,
// so its access tree has no warnings, and the details show the selected element.
func TestInspectorWindowShowsTheSelectedElementWithoutAccessibilityWarnings(t *testing.T) {
	h, _, tg := inspSampleWindow(t)
	ins := newInspector(h, tg)
	tg.Attach(ins.changed)
	id, ok := inspFindKey(tg, tg.Root(), "save")
	if !ok {
		t.Fatal("no element keyed save")
	}
	tg.Select(id)

	irt := ui.NewRuntime(ins.root, ui.DarkTheme(), nil)
	var warnings []string
	irt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	irt.SetAccessWarnings(true)
	irt.SetViewport(1100, 760, 1)
	irt.Frame(time.Now())

	if len(warnings) != 0 {
		t.Fatalf("the inspector's window has accessibility warnings: %v", warnings)
	}
	var dump strings.Builder
	irt.Dump(&dump, 0)
	for _, want := range []string{`text "div save`, `text "Layout"`, `text "Accessibility"`, `text "Style"`,
		"Copy as Go", "Frames of the window"} {
		if !strings.Contains(dump.String(), want) {
			t.Errorf("the inspector's window does not show %q", want)
		}
	}
	if irt.AccessTree().Find(access.RoleButton, "Pick an element") == nil {
		t.Error("the pick button is not in the inspector's access tree")
	}
}

// hoverTarget records the Hover calls that the inspector makes on its target.
type hoverTarget struct {
	inspect.Target
	hover []uint64
}

func (h *hoverTarget) Hover(id uint64) {
	h.hover = append(h.hover, id)
	h.Target.Hover(id)
}

// TestInspectorTreeHoverOutlinesTheElement checks that the pointer over a row of
// the tree asks the window to outline that row's element, and that leaving the
// tree clears the outline.
func TestInspectorTreeHoverOutlinesTheElement(t *testing.T) {
	h, _, tg := inspSampleWindow(t)
	rec := &hoverTarget{Target: tg}
	ins := newInspector(h, rec)
	tg.Attach(ins.changed)
	root, _ := tg.Element(tg.Root())
	first := root.Children[0]

	irt := ui.NewRuntime(ins.root, ui.DarkTheme(), nil)
	irt.SetViewport(1100, 760, 1)
	irt.Frame(time.Now())
	r, ok := irt.Find(keyOf(first))
	if !ok {
		t.Fatalf("no tree row keyed %s", keyOf(first))
	}
	irt.PointerMove(r.X+r.W/2, r.Y+r.H/2, 0)
	irt.Frame(time.Now())
	if n := len(rec.hover); n == 0 || rec.hover[n-1] != first {
		t.Fatalf("hovering the row of element %d called Hover with %v", first, rec.hover)
	}
	irt.PointerMove(1090, 750, 0) // out of the tree
	irt.Frame(time.Now())
	if n := len(rec.hover); rec.hover[n-1] != 0 {
		t.Fatalf("leaving the tree did not clear the hover: %v", rec.hover)
	}
}

// inspectorRuntime renders the inspector's window for ins, at its default
// size, with its pick shortcut, and collects its accessibility warnings.
func inspectorRuntime(ins *inspector) (*ui.Runtime, *[]string) {
	irt := ui.NewRuntime(ins.root, ui.DarkTheme(), nil)
	var warnings []string
	irt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	irt.SetAccessWarnings(true)
	irt.AddBindings(ui.Bind("ctrl+shift+c", inspectPick))
	irt.SetViewport(1100, 760, 1)
	irt.Frame(time.Now())
	return irt, &warnings
}

// accessNamed returns the first node of the access tree whose name contains s.
func accessNamed(rt *ui.Runtime, s string) *access.Node {
	var found *access.Node
	rt.AccessTree().Walk(func(n *access.Node) bool {
		if found == nil && strings.Contains(n.Name, s) {
			found = n
		}
		return found == nil
	})
	return found
}

// TestInspectorEmptyStateAndPickShortcut checks the details with nothing
// selected, which name the pick shortcut, and that the shortcut and Escape in
// the window turn pick mode on and off with the toggle following.
func TestInspectorEmptyStateAndPickShortcut(t *testing.T) {
	h, rt, tg := inspSampleWindow(t)
	ins := newInspector(h, tg)
	tg.Attach(ins.changed)
	irt, warnings := inspectorRuntime(ins)
	if len(*warnings) != 0 {
		t.Fatalf("the empty inspector has accessibility warnings: %v", *warnings)
	}
	if accessNamed(irt, "No element selected") == nil || accessNamed(irt, "(Ctrl+Shift+C)") == nil {
		t.Fatal("the empty details do not say that nothing is selected, or do not name the pick shortcut")
	}

	irt.KeyDown(input.KeyC, input.ModCtrl|input.ModShift, false)
	irt.Frame(time.Now())
	if !tg.Picking() || !ins.picking.Peek() {
		t.Fatalf("Ctrl+Shift+C: picking %v, toggle %v; want both on", tg.Picking(), ins.picking.Peek())
	}
	rt.KeyDown(input.KeyEscape, 0, false) // in the inspected window
	if tg.Picking() || ins.picking.Peek() {
		t.Fatalf("Escape: picking %v, toggle %v; want both off", tg.Picking(), ins.picking.Peek())
	}
}

// TestInspectorFilterListsTheMatches checks that the filter turns the tree into
// a flat list of the elements that match it, nested ones included.
func TestInspectorFilterListsTheMatches(t *testing.T) {
	h, _, tg := inspSampleWindow(t)
	ins := newInspector(h, tg)
	tg.Attach(ins.changed)
	save, _ := inspFindKey(tg, tg.Root(), "save")
	card, _ := inspFindKey(tg, tg.Root(), "card")
	var label uint64
	for id := save; label == 0; { // the text is the first leaf of the button
		e, _ := tg.Element(id)
		if e.Text == "Save" {
			label = id
		} else if len(e.Children) == 0 {
			t.Fatal("no text Save in the button")
		} else {
			id = e.Children[0]
		}
	}
	ins.filter.Set("SAVE")
	irt, warnings := inspectorRuntime(ins)
	if len(*warnings) != 0 {
		t.Fatalf("the filtered inspector has accessibility warnings: %v", *warnings)
	}
	for _, id := range []uint64{save, label} {
		if _, ok := irt.Find(keyOf(id)); !ok {
			t.Errorf("element %d, which matches the filter, has no row", id)
		}
	}
	if _, ok := irt.Find(keyOf(card)); ok {
		t.Error("the card, which does not match the filter, has a row")
	}
}

// TestInspectorBreadcrumbSelectsTheAncestor checks the trail of ancestors in the
// details: a click on a crumb selects that ancestor.
func TestInspectorBreadcrumbSelectsTheAncestor(t *testing.T) {
	h, _, tg := inspSampleWindow(t)
	ins := newInspector(h, tg)
	tg.Attach(ins.changed)
	save, _ := inspFindKey(tg, tg.Root(), "save")
	tg.Select(save)
	irt, _ := inspectorRuntime(ins)
	crumb := irt.AccessTree().Find(access.RoleLink, "div root")
	if crumb == nil {
		crumb = irt.AccessTree().Find(access.RoleButton, "div root")
	}
	if crumb == nil {
		t.Fatal("no crumb for the root in the details")
	}
	b := crumb.Bounds
	irt.PointerDown(b.X+b.W/2, b.Y+b.H/2, input.ButtonLeft, 1, 0)
	irt.PointerUp(b.X+b.W/2, b.Y+b.H/2, input.ButtonLeft, 0)
	irt.Frame(time.Now())
	if tg.Selected() != tg.Root() || ins.sel.Peek() != keyOf(tg.Root()) {
		t.Fatalf("after the crumb, selected %d (tree %s), want the root %d", tg.Selected(), ins.sel.Peek(), tg.Root())
	}
}

func TestInspectorShortComponentNames(t *testing.T) {
	for full, want := range map[string]string{
		"github.com/tyler-builds/jamb/ui.(*TitleBarElement).expand.func1": "ui.TitleBar",
		"example.com/app/views.Card":                                      "views.Card",
		"example.com/app/views.Card.func2":                                "views.Card",
		"*ui.ButtonElement":                                               "ui.Button",
		"main.main.func1":                                                 "main.main",
		"plain":                                                           "plain",
	} {
		if got := shortComponent(full); got != want {
			t.Errorf("shortComponent(%q) = %q, want %q", full, got, want)
		}
	}
}

// TestInspectorFrameSamplesTheTiming checks that a frame of the inspected window
// records its timings, and that the inspector is asked to read the window again.
func TestInspectorFrameSamplesTheTiming(t *testing.T) {
	h, rt, tg := inspSampleWindow(t)
	ins := newInspector(h, tg)
	before := ins.tick.Peek()
	rt.SetViewport(900, 600, 1) // a new size lays the window out again
	rt.Frame(time.Now())
	ins.frame(rt, time.Now())
	ins.frame(rt, time.Now().Add(time.Millisecond))
	if n := len(ins.perf.Samples()); n == 0 {
		t.Fatal("no frame timing was recorded")
	}
	if ins.tick.Peek() == before {
		t.Fatal("the inspector was not asked to read the window again")
	}
}

// TestInspectCloseDetachesTheWindow checks that closing the inspector ends its
// pick mode and drops its live edits, and that it does nothing once the window's
// inspector is gone.
func TestInspectCloseDetachesTheWindow(t *testing.T) {
	h, rt, tg := inspSampleWindow(t)
	ins := newInspector(h, tg)
	h.insp = ins
	tg.Attach(ins.changed)
	tg.Pick(true)
	card, _ := inspFindKey(tg, tg.Root(), "root")
	if err := tg.Edit(card, "Background", "#ff0000"); err != nil {
		t.Fatal(err)
	}
	ins.close()
	if h.insp != nil {
		t.Fatal("close kept the inspector on the window")
	}
	if tg.Picking() || tg.Edited(card) {
		t.Fatal("close left pick mode or a live edit on the window")
	}
	ins.close() // a second close does nothing
	rt.Frame(time.Now())
}

// TestInspectShortcutNeedsDevTools checks that Ctrl+Shift+I is left to the window
// unless the window has the dev tools, and that other keys are never the shortcut.
func TestInspectShortcutNeedsDevTools(t *testing.T) {
	h, _, _ := inspSampleWindow(t)
	if h.inspectKey(input.KeyI, input.ModCtrl|input.ModShift) {
		t.Fatal("the shortcut opened an inspector on a window without dev tools")
	}
	h.devTools = true
	if h.inspectKey(input.KeyA, input.ModCtrl|input.ModShift) {
		t.Fatal("a key other than I was taken as the shortcut")
	}
	if h.inspectKey(input.KeyI, input.ModCtrl) {
		t.Fatal("Ctrl+I without Shift was taken as the shortcut")
	}
}

func TestInspectorOptionsAndKeys(t *testing.T) {
	if (devOptions{}).inspectOn() {
		t.Error("the inspector is on in a release run with no options")
	}
	if !(devOptions{inspect: true}).inspectOn() || !(devOptions{devStarted: 1}).inspectOn() {
		t.Error("JAMB_INSPECT=1 or the dev runner does not turn the inspector on")
	}
	for _, id := range []uint64{0, 1, 1 << 40} {
		if idOf(keyOf(id)) != id {
			t.Errorf("idOf(keyOf(%d)) = %d", id, idOf(keyOf(id)))
		}
	}
}
