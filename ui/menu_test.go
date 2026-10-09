package ui

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// key presses k with mods and lets the frame run.
func (r *overlayRig) key(k input.Key, mods input.Modifiers) {
	r.rt.KeyDown(k, mods, false)
	r.rt.KeyUp(k, mods)
	r.frame()
}

// advance moves the clock on by d and lets the frame run.
func (r *overlayRig) advance(d time.Duration) {
	r.now = r.now.Add(d)
	r.frame()
}

// focusKey returns the key of the focused element, or "".
func (r *overlayRig) focusKey() string {
	if n := r.rt.byID[r.rt.focusID]; n != nil {
		return n.key
	}
	return ""
}

// focus gives keyboard focus to the element with key.
func (r *overlayRig) focus(t *testing.T, key string) {
	t.Helper()
	for _, n := range r.rt.byID {
		if n.key == key {
			r.rt.setFocus(n.id)
			r.frame()
			return
		}
	}
	t.Fatalf("no element %q to focus", key)
}

// rightClick presses and releases the right button at (x, y).
func (r *overlayRig) rightClick(x, y float32) {
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonRight, 0)
	r.frame()
}

// center returns a point inside the element with key.
func (r *overlayRig) center(t *testing.T, key string) (float32, float32) {
	t.Helper()
	rc := r.find(t, key)
	return rc.X + rc.W/2, rc.Y + rc.H/2
}

// logged returns a menu item that records its label in log when chosen.
func logged(log *[]string, label string) MenuItem {
	return MenuItem{Label: label, OnClick: func() { *log = append(*log, label) }}
}

// sampleMenu is a menu with a separator and a disabled item: Open, Delete
// (disabled), Rename. Keyboard moves from Open to Rename.
func sampleMenu(log *[]string) []MenuItem {
	return []MenuItem{
		logged(log, "Open"),
		MenuSeparator(),
		{Label: "Delete", Disabled: true, OnClick: func() { *log = append(*log, "Delete") }},
		logged(log, "Rename"),
	}
}

// dropdownRig shows a dropdown on a trigger, with an editor that can take
// focus above it.
func dropdownRig(items func() []MenuItem, focusable bool) *overlayRig {
	return newOverlayRig(func(ctx *Context) Element {
		trigger := Div().Key("trigger").Size(Px(100), Px(30))
		if focusable {
			trigger.Focusable()
		}
		return Div().Padding(20).Column().
			Child(Div().Key("editor").Size(Px(200), Px(30)).Focusable()).
			Child(DropdownMenu(trigger, items))
	}, 800, 600)
}

func TestMenuRoles(t *testing.T) {
	tests := []struct {
		item MenuItem
		want MenuRole
	}{
		{MenuItem{Label: "Open"}, RoleMenuItem},
		{MenuItem{Label: "Star", Check: MenuCheckBox, Checked: true}, RoleMenuItemCheckbox},
		{MenuItem{Label: "Active", Check: MenuCheckRadio}, RoleMenuItemRadio},
		{MenuSeparator(), RoleSeparator},
		{MenuSection("Status"), RoleGroupLabel},
	}
	for _, tc := range tests {
		if got := tc.item.Role(); got != tc.want {
			t.Errorf("%q: role %v, want %v", tc.item.Label, got, tc.want)
		}
	}
}

// Up and Down wrap and skip separators and disabled items; Home and End go to
// the ends.
func TestMenuStepSkipsUnselectable(t *testing.T) {
	items := []MenuItem{
		{Label: "A"}, MenuSeparator(), {Label: "B", Disabled: true}, {Label: "C"}, MenuSection("S"), {Label: "D"},
	}
	tests := []struct {
		from, dir, want int
	}{
		{-1, 1, 0},
		{-1, -1, 5},
		{0, 1, 3},
		{3, 1, 5},
		{5, 1, 0},
		{0, -1, 5},
		{3, -1, 0},
	}
	for _, tc := range tests {
		if got := menuStep(items, tc.from, tc.dir); got != tc.want {
			t.Errorf("menuStep(from %d, dir %d) = %d, want %d", tc.from, tc.dir, got, tc.want)
		}
	}
	if got := menuStep([]MenuItem{MenuSeparator(), {Label: "x", Disabled: true}}, -1, 1); got != -1 {
		t.Errorf("no selectable item: got %d, want -1", got)
	}
}

func TestMenuTypeaheadWrapsAndSkipsDisabled(t *testing.T) {
	items := []MenuItem{{Label: "Apple"}, {Label: "Banana"}, {Label: "blueberry", Disabled: true}, {Label: "Blackberry"}, {Label: "Cherry"}}
	if got := menuTypeahead(items, -1, 'b'); got != 1 {
		t.Errorf("first b: %d, want 1 (Banana)", got)
	}
	if got := menuTypeahead(items, 1, 'B'); got != 3 {
		t.Errorf("next B after Banana: %d, want 3 (Blackberry; the disabled one is skipped)", got)
	}
	if got := menuTypeahead(items, 3, 'a'); got != 0 {
		t.Errorf("a after Blackberry wraps to Apple: %d, want 0", got)
	}
	if got := menuTypeahead(items, 0, 'z'); got != -1 {
		t.Errorf("no match: %d, want -1", got)
	}
}

func TestDropdownOpensOnClickAndClosesOutside(t *testing.T) {
	var log []string
	r := dropdownRig(func() []MenuItem { return sampleMenu(&log) }, false)
	if r.has("jamb.menu") {
		t.Fatal("menu open before any click")
	}
	x, y := r.center(t, "trigger")
	r.click(x, y)
	if !r.has("jamb.menu") {
		t.Fatal("click on the trigger did not open the menu")
	}
	r.click(x, y)
	if r.has("jamb.menu") {
		t.Error("second click on the trigger did not close the menu")
	}
	r.click(x, y)
	r.click(700, 500)
	if r.has("jamb.menu") {
		t.Error("a press outside did not close the menu")
	}
}

// A click on a disabled item does nothing; a click on an enabled one runs it
// and closes the menu.
func TestMenuClickRunsEnabledItemOnly(t *testing.T) {
	var log []string
	r := dropdownRig(func() []MenuItem { return sampleMenu(&log) }, false)
	x, y := r.center(t, "trigger")
	r.click(x, y)
	dx, dy := r.center(t, "jamb.menu.0.2")
	r.click(dx, dy)
	if !r.has("jamb.menu") || len(log) != 0 {
		t.Fatalf("disabled item: menu open %v, log %v; want open and nothing run", r.has("jamb.menu"), log)
	}
	ox, oy := r.center(t, "jamb.menu.0.0")
	r.click(ox, oy)
	if r.has("jamb.menu") || !slices.Equal(log, []string{"Open"}) {
		t.Errorf("enabled item: menu open %v, log %v; want closed and [Open]", r.has("jamb.menu"), log)
	}
}

// Down opens the menu with the first item highlighted, then the keys move the
// highlight, wrapping and skipping separators and disabled items. Enter runs the
// highlighted item and returns focus to the trigger.
func TestMenuKeyboardNavigation(t *testing.T) {
	var log []string
	r := dropdownRig(func() []MenuItem { return sampleMenu(&log) }, true)
	r.focus(t, "trigger")
	r.key(input.KeyDown, 0)
	if !r.has("jamb.menu") || r.focusKey() != "jamb.menu" {
		t.Fatalf("Down did not open the menu with focus in it (focus %q)", r.focusKey())
	}
	r.key(input.KeyDown, 0) // Open -> Rename, skipping the separator and Delete
	r.key(input.KeyDown, 0) // wraps to Open
	r.key(input.KeyUp, 0)   // wraps back to Rename
	r.key(input.KeyHome, 0)
	r.key(input.KeyEnd, 0)
	r.key(input.KeyEnter, 0)
	if !slices.Equal(log, []string{"Rename"}) {
		t.Errorf("chosen items %v, want [Rename]", log)
	}
	if r.has("jamb.menu") {
		t.Error("choosing an item left the menu open")
	}
	if r.focusKey() != "trigger" {
		t.Errorf("focus after the menu closed is %q, want the trigger", r.focusKey())
	}
}

func TestMenuTypeaheadJumpsToItems(t *testing.T) {
	var log []string
	items := func() []MenuItem {
		return []MenuItem{logged(&log, "Apple"), logged(&log, "Banana"), logged(&log, "Blueberry"), logged(&log, "Cherry")}
	}
	r := dropdownRig(items, true)
	r.focus(t, "trigger")
	r.key(input.KeyDown, 0) // highlights Apple
	r.rt.TextInput("b")
	r.frame()
	r.rt.TextInput("b")
	r.frame()
	r.key(input.KeyEnter, 0)
	if !slices.Equal(log, []string{"Blueberry"}) {
		t.Errorf("typeahead chose %v, want [Blueberry]", log)
	}
}

// Right opens a submenu with its first item highlighted, Left closes it, and
// Escape closes one level at a time.
func TestSubmenuKeyboardAndEscapeOneLevel(t *testing.T) {
	var log []string
	items := func() []MenuItem {
		return []MenuItem{
			{Label: "Status", Submenu: []MenuItem{
				{Label: "Active", Check: MenuCheckRadio, Checked: true, OnClick: func() { log = append(log, "Active") }},
				{Label: "Paused", Check: MenuCheckRadio, OnClick: func() { log = append(log, "Paused") }},
			}},
			logged(&log, "Other"),
		}
	}
	r := dropdownRig(items, true)
	r.focus(t, "trigger")
	r.key(input.KeyDown, 0)
	r.key(input.KeyRight, 0)
	if !r.has("jamb.menu.1.0") {
		t.Fatal("Right did not open the submenu")
	}
	r.key(input.KeyLeft, 0)
	if r.has("jamb.menu.1.0") || !r.has("jamb.menu") {
		t.Fatal("Left should close the submenu and keep the menu")
	}
	r.key(input.KeyRight, 0)
	r.key(input.KeyDown, 0) // Active -> Paused
	r.key(input.KeyEnter, 0)
	if !slices.Equal(log, []string{"Paused"}) || r.has("jamb.menu") {
		t.Errorf("choosing in the submenu: log %v, menu open %v; want [Paused] and closed", log, r.has("jamb.menu"))
	}

	r.focus(t, "trigger")
	r.key(input.KeyDown, 0)
	r.key(input.KeyRight, 0)
	r.key(input.KeyEscape, 0)
	if r.has("jamb.menu.1.0") || !r.has("jamb.menu") {
		t.Fatal("first Escape should close only the submenu")
	}
	r.key(input.KeyEscape, 0)
	if r.has("jamb.menu") || r.focusKey() != "trigger" {
		t.Errorf("second Escape: menu open %v, focus %q; want closed and focus on the trigger", r.has("jamb.menu"), r.focusKey())
	}
}

// Moving the pointer onto a submenu item opens it only after submenuDelay, and
// leaving it closes the submenu only after the same delay, so the pointer can
// cross other items on its way into the submenu.
func TestSubmenuHoverDelay(t *testing.T) {
	items := func() []MenuItem {
		return []MenuItem{
			{Label: "Status", Submenu: []MenuItem{{Label: "Active"}}},
			{Label: "Other"},
		}
	}
	r := dropdownRig(items, false)
	x, y := r.center(t, "trigger")
	r.click(x, y)
	sx, sy := r.center(t, "jamb.menu.0.0")
	r.rt.PointerMove(sx, sy, 0)
	r.frame()
	if r.has("jamb.menu.1.0") {
		t.Fatal("submenu opened without a delay")
	}
	r.advance(submenuDelay - 10*time.Millisecond)
	if r.has("jamb.menu.1.0") {
		t.Fatal("submenu opened before the delay ended")
	}
	r.advance(20 * time.Millisecond)
	if !r.has("jamb.menu.1.0") {
		t.Fatal("submenu did not open after the delay")
	}
	ox, oy := r.center(t, "jamb.menu.0.1")
	r.rt.PointerMove(ox, oy, 0)
	r.frame()
	if !r.has("jamb.menu.1.0") {
		t.Fatal("submenu closed at once when the pointer moved to another item")
	}
	r.advance(submenuDelay + 10*time.Millisecond)
	if r.has("jamb.menu.1.0") {
		t.Error("submenu did not close after the pointer moved on")
	}
}

// A press outside the menu closes the whole chain, submenus included.
func TestClickOutsideClosesChain(t *testing.T) {
	items := func() []MenuItem {
		return []MenuItem{{Label: "Status", Submenu: []MenuItem{{Label: "Active"}}}}
	}
	r := dropdownRig(items, false)
	x, y := r.center(t, "trigger")
	r.click(x, y)
	sx, sy := r.center(t, "jamb.menu.0.0")
	r.rt.PointerMove(sx, sy, 0)
	r.advance(submenuDelay + 10*time.Millisecond)
	if !r.has("jamb.menu.1.0") {
		t.Fatal("submenu did not open")
	}
	r.click(700, 500)
	if r.has("jamb.menu") || r.has("jamb.menu.1.0") {
		t.Error("a press outside left part of the chain open")
	}
}

// An action item runs through the action dispatch after the menu closes, and
// focus is back on the element that had it, so the handler on its ancestors
// runs.
func TestMenuActionRunsAfterClosing(t *testing.T) {
	run := NewAction("test.run", "Run it")
	ran := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().OnAction(run, func() { ran++ }).Padding(20).Column().
			Child(Div().Key("editor").Size(Px(200), Px(30)).Focusable()).
			Child(DropdownMenu(Div().Key("trigger").Size(Px(100), Px(30)), func() []MenuItem {
				return []MenuItem{{Action: run}}
			}))
	}, 800, 600)
	r.focus(t, "editor")
	x, y := r.center(t, "trigger")
	r.click(x, y)
	ix, iy := r.center(t, "jamb.menu.0.0")
	r.click(ix, iy)
	if ran != 1 {
		t.Errorf("action ran %d times, want once", ran)
	}
	if r.has("jamb.menu") {
		t.Error("the menu is still open after running an action")
	}
	if r.focusKey() != "editor" {
		t.Errorf("focus after the action is %q, want the editor it returned to", r.focusKey())
	}
}

// The shortcut shown on an action item comes from its binding.
func TestMenuShowsShortcutFromBinding(t *testing.T) {
	newNote := NewAction("test.new", "New note")
	r := dropdownRig(func() []MenuItem { return []MenuItem{{Action: newNote}} }, false)
	r.rt.AddBindings(Bind("ctrl+n", newNote))
	x, y := r.center(t, "trigger")
	r.click(x, y)
	var buf bytes.Buffer
	r.rt.Dump(&buf, 64)
	if !strings.Contains(buf.String(), "Ctrl+N") {
		t.Errorf("the open menu does not show Ctrl+N:\n%s", buf.String())
	}
}

// A right press opens the context menu at the pointer, a press outside closes
// it, and Shift+F10 and the Menu key open it below the focused element.
func TestContextMenuOpensAtPointerAndFromKeyboard(t *testing.T) {
	var log []string
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).Child(Div().Key("row").Size(Px(300), Px(40)).Focusable().
			ContextMenu(func() []MenuItem { return sampleMenu(&log) }))
	}, 800, 600)
	rc := r.find(t, "row")
	px, py := rc.X+30, rc.Y+12
	r.rightClick(px, py)
	if !r.has("jamb.menu") {
		t.Fatal("right-click did not open the context menu")
	}
	r.advance(300 * time.Millisecond) // the menu's Enter motion settles
	m := r.find(t, "jamb.menu")
	if !near(m.X, px) || !near(m.Y, py) {
		t.Errorf("menu at (%v, %v), want the pointer (%v, %v)", m.X, m.Y, px, py)
	}
	r.key(input.KeyEscape, 0)
	if r.has("jamb.menu") {
		t.Fatal("Escape did not close the context menu")
	}

	r.focus(t, "row")
	r.key(input.KeyF10, input.ModShift)
	if !r.has("jamb.menu") {
		t.Fatal("Shift+F10 did not open the context menu")
	}
	r.advance(300 * time.Millisecond)
	m = r.find(t, "jamb.menu")
	if !near(m.X, rc.X) || !near(m.Y, rc.Y+rc.H) {
		t.Errorf("keyboard menu at (%v, %v), want below the focused row (%v, %v)", m.X, m.Y, rc.X, rc.Y+rc.H)
	}
	r.key(input.KeyEscape, 0)
	r.key(input.KeyMenu, 0)
	if !r.has("jamb.menu") {
		t.Error("the Menu key did not open the context menu")
	}
	r.rightClick(700, 500)
	if r.has("jamb.menu") {
		t.Error("a right press outside did not close the context menu")
	}
}

// A menu that opens from the pointer returns focus to the element focused before
// the press.
func TestMenuReturnsFocusToBeforeThePress(t *testing.T) {
	var log []string
	r := dropdownRig(func() []MenuItem { return sampleMenu(&log) }, false)
	ex, ey := r.center(t, "editor")
	r.click(ex, ey)
	if r.focusKey() != "editor" {
		t.Fatalf("editor not focused, focus %q", r.focusKey())
	}
	x, y := r.center(t, "trigger")
	r.click(x, y)
	r.key(input.KeyEscape, 0)
	if r.has("jamb.menu") || r.focusKey() != "editor" {
		t.Errorf("after Escape: menu open %v, focus %q; want closed and focus on the editor", r.has("jamb.menu"), r.focusKey())
	}
}

// menuBarRig is a menu bar of File (New, Exit) and Edit (Undo, Redo, Cut) over an
// editor.
func menuBarRig() *overlayRig {
	var sink []string
	return newOverlayRig(func(ctx *Context) Element {
		return Div().Column().
			Child(MenuBar(
				TopMenu{Label: "File", Items: func() []MenuItem { return sampleFile(&sink) }},
				TopMenu{Label: "Edit", Items: func() []MenuItem { return sampleEdit(&sink) }},
			)).
			Child(Div().Key("editor").Size(Px(300), Px(200)).Focusable())
	}, 800, 600)
}

func sampleFile(sink *[]string) []MenuItem {
	return []MenuItem{logged(sink, "New"), logged(sink, "Exit")}
}

func sampleEdit(sink *[]string) []MenuItem {
	return []MenuItem{logged(sink, "Undo"), logged(sink, "Redo"), logged(sink, "Cut")}
}

// A click opens a top-level menu; Left and Right move to the neighboring menus,
// and hovering another label while a menu is open switches to it.
func TestMenuBarSwitchesWithKeysAndHover(t *testing.T) {
	r := menuBarRig()
	x, y := r.center(t, "jamb.menubar.0")
	r.click(x, y)
	if !r.has("jamb.menu") || r.has("jamb.menu.0.2") {
		t.Fatal("clicking File did not open File")
	}
	r.key(input.KeyRight, 0)
	if !r.has("jamb.menu.0.2") {
		t.Fatal("Right did not open Edit")
	}
	r.key(input.KeyLeft, 0)
	if r.has("jamb.menu.0.2") || !r.has("jamb.menu.0.1") {
		t.Fatal("Left did not return to File")
	}
	r.key(input.KeyLeft, 0) // wraps to Edit
	if !r.has("jamb.menu.0.2") {
		t.Fatal("Left from File did not wrap to Edit")
	}
	// Edit is open. Hovering File switches to File, and hovering Edit back. The
	// pointer first leaves the bar, so that entering File is a new hover.
	r.rt.PointerMove(700, 500, 0)
	r.frame()
	fx, fy := r.center(t, "jamb.menubar.0")
	r.rt.PointerMove(fx, fy, 0)
	r.frame()
	if r.has("jamb.menu.0.2") || !r.has("jamb.menu.0.1") {
		t.Fatal("hovering File with Edit open did not switch to File")
	}
	mx, my := r.center(t, "jamb.menubar.1")
	r.rt.PointerMove(mx, my, 0)
	r.frame()
	if !r.has("jamb.menu.0.2") {
		t.Error("hovering Edit with File open did not switch to Edit")
	}
}

// Pressing and releasing Alt by itself focuses the bar. Down opens the menu,
// Escape closes it, and focus goes back to where it was before Alt.
func TestAltFocusesMenuBar(t *testing.T) {
	r := menuBarRig()
	r.focus(t, "editor")
	r.rt.KeyDown(input.KeyUnknown, input.ModAlt, false)
	r.rt.KeyUp(input.KeyUnknown, 0)
	r.frame()
	if r.focusKey() != "jamb.menubar.0" {
		t.Fatalf("Alt did not focus the bar: focus %q", r.focusKey())
	}
	r.key(input.KeyRight, 0)
	if r.focusKey() != "jamb.menubar.1" {
		t.Errorf("Right on the bar: focus %q, want Edit", r.focusKey())
	}
	r.key(input.KeyLeft, 0)
	r.key(input.KeyDown, 0)
	if !r.has("jamb.menu.0.1") {
		t.Fatal("Down on the bar did not open File")
	}
	r.key(input.KeyEscape, 0)
	if r.has("jamb.menu") || r.focusKey() != "editor" {
		t.Errorf("Escape: menu open %v, focus %q; want closed and focus back on the editor", r.has("jamb.menu"), r.focusKey())
	}
}

// Alt with another key is not a tap, and Alt pressed alone without a menu bar
// does nothing.
func TestAltTapNeedsNothingElse(t *testing.T) {
	r := menuBarRig()
	r.focus(t, "editor")
	r.rt.KeyDown(input.KeyUnknown, input.ModAlt, false)
	r.rt.KeyDown(input.KeyF, input.ModAlt, false)
	r.rt.KeyUp(input.KeyF, input.ModAlt)
	r.rt.KeyUp(input.KeyUnknown, 0)
	r.frame()
	if r.focusKey() != "editor" {
		t.Errorf("Alt+F moved focus to %q; want it to stay on the editor", r.focusKey())
	}
}

// A press outside the bar leaves it, so a later menu does not return focus to a
// stale place.
func TestPressOutsideLeavesMenuBar(t *testing.T) {
	r := menuBarRig()
	r.focus(t, "editor")
	r.rt.KeyDown(input.KeyUnknown, input.ModAlt, false)
	r.rt.KeyUp(input.KeyUnknown, 0)
	r.frame()
	if r.rt.menuReturn == nil {
		t.Fatal("Alt did not remember where focus was")
	}
	ex, ey := r.center(t, "editor")
	r.click(ex, ey)
	if r.rt.menuReturn != nil {
		t.Error("a press outside the bar kept its return focus")
	}
}

// A submenu opens to the right of its row, and flips to the left when there is
// no room there.
func TestSubmenuFlipsLeftAtWindowEdge(t *testing.T) {
	items := func() []MenuItem {
		return []MenuItem{{Label: "Status", Submenu: []MenuItem{{Label: "Active"}}}}
	}
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Row().Children(
			Div().Size(Px(640), Px(10)),
			DropdownMenu(Div().Key("trigger").Size(Px(100), Px(30)), items))
	}, 800, 600)
	x, y := r.center(t, "trigger")
	r.click(x, y)
	sx, sy := r.center(t, "jamb.menu.0.0")
	r.rt.PointerMove(sx, sy, 0)
	r.advance(submenuDelay + 10*time.Millisecond)
	if !r.has("jamb.menu.1.0") {
		t.Fatal("submenu did not open")
	}
	menu, sub := r.find(t, "jamb.menu"), r.find(t, "jamb.menu.1.0")
	if sub.X+sub.W > menu.X {
		t.Errorf("submenu at x %v..%v overlaps the menu at x %v; want it to the left", sub.X, sub.X+sub.W, menu.X)
	}
}

// near reports whether a and b differ by less than a pixel.
func near(a, b float32) bool { return a-b < 1 && b-a < 1 }
