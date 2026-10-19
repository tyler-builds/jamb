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
	for _, want := range []string{"key=save", "Style, set fields only", "Copy as Go", "Frames of the window"} {
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
