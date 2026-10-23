package ui

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// lbItem is an item of the list box tests.
type lbItem struct{ key, name string }

// lbItems are the items of the list box tests, in order.
var lbItems = []lbItem{{"a", "Alpha"}, {"b", "Beta"}, {"c", "Gamma"}, {"d", "Delta"}, {"e", "Epsilon"}}

// listBoxDemo is a list box of lbItems between two buttons, so that Tab can be
// checked. The list has the key "list".
func listBoxDemo(sel *Signal[[]string], mode SelectionMode, st ListBoxStyle, onChange func([]string),
	onActivate func(lbItem), menu func(lbItem) []MenuItem) Element {
	lb := ListBox(lbItems, func(it lbItem) string { return it.key }, sel,
		func(it lbItem, s PartState) Element { return Text(it.name) }).
		Key("list").Label("Letters").Selection(mode).Styles(st).
		ItemName(func(it lbItem) string { return it.name }).Height(Px(200))
	if onChange != nil {
		lb.OnChange(onChange)
	}
	if onActivate != nil {
		lb.OnActivate(onActivate)
	}
	if menu != nil {
		lb.ContextMenu(menu)
	}
	return Div().Column().Gap(8).
		Child(Button("Before").Key("before")).
		Child(lb).
		Child(Button("After").Key("after"))
}

// newListBoxRig renders a list box of lbItems with the selection sel.
func newListBoxRig(sel *Signal[[]string], mode SelectionMode) *accessRig {
	return newAccessRig(func(*Context) Element {
		return listBoxDemo(sel, mode, ListBoxStyle{}, nil, nil, nil)
	})
}

// selectedKeys returns the keys in sel as a comma-separated list, for messages.
func selectedKeys(sel *Signal[[]string]) string { return strings.Join(sel.Peek(), ",") }

func TestListBoxIsOneTabStop(t *testing.T) {
	r := newListBoxRig(NewSignal([]string{"c"}), SelectionSingle)
	r.focusKey(t, "before")
	r.press(input.KeyTab, 0)
	if got := r.focusedKey(); got != "list" {
		t.Fatalf("Tab from the button before the list focused %q, want the list", got)
	}
	r.press(input.KeyTab, 0)
	if got := r.focusedKey(); got != "after" {
		t.Fatalf("a second Tab focused %q, want the button after the list", got)
	}
}

func TestListBoxTabReachesTheSelectedItem(t *testing.T) {
	sel := NewSignal([]string{"c"})
	r := newListBoxRig(sel, SelectionSingle)
	r.focusKey(t, "before")
	r.press(input.KeyTab, 0)
	if !r.focusedAccess(t, access.RoleListItem, "Gamma") {
		t.Fatal("Tab did not put the focus on the selected item, Gamma")
	}
	if sel.Peek()[0] != "c" {
		t.Fatalf("Tab changed the selection to %v", sel.Peek())
	}
}

func TestListBoxSingleSelectionFollowsFocus(t *testing.T) {
	sel := NewSignal([]string(nil))
	var picked []string
	r := newAccessRig(func(*Context) Element {
		return listBoxDemo(sel, SelectionSingle, ListBoxStyle{}, func(k []string) { picked = append(picked, k[0]) }, nil, nil)
	})
	r.focusKey(t, "list")
	r.press(input.KeyDown, 0)
	if got := selectedKeys(sel); got != "b" {
		t.Fatalf("Down selected %q, want b (the first item was the tab stop)", got)
	}
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnd, 0)
	if got := selectedKeys(sel); got != "e" {
		t.Fatalf("End selected %q, want e", got)
	}
	r.press(input.KeyHome, 0)
	r.press(input.KeyUp, 0) // at the top: no wrap
	if got := selectedKeys(sel); got != "a" {
		t.Fatalf("Home then Up selected %q, want a", got)
	}
	// Home then Up selects a again, but Up from a is no change, so OnChange does not run.
	if got := strings.Join(picked, ","); got != "b,c,e,a" {
		t.Fatalf("OnChange ran for %q, want b,c,e,a", got)
	}
}

func TestListBoxCtrlArrowMovesFocusWithoutSelecting(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := newAccessRig(func(*Context) Element { return listBoxDemo(sel, SelectionMultiple, ListBoxStyle{}, nil, nil, nil) })
	r.focusKey(t, "list")
	r.press(input.KeyDown, input.ModCtrl)
	r.press(input.KeyDown, input.ModCtrl)
	if got := selectedKeys(sel); got != "" {
		t.Fatalf("Ctrl+Down selected %q, want nothing", got)
	}
	r.press(input.KeySpace, 0) // toggles the focused item, the third
	if got := selectedKeys(sel); got != "c" {
		t.Fatalf("Space after Ctrl+Down selected %q, want c", got)
	}
}

func TestListBoxMultipleSelectionKeys(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := newAccessRig(func(*Context) Element { return listBoxDemo(sel, SelectionMultiple, ListBoxStyle{}, nil, nil, nil) })
	r.focusKey(t, "list")
	r.press(input.KeySpace, 0) // a
	r.press(input.KeyDown, 0)  // a plain move selects only b
	if got := selectedKeys(sel); got != "b" {
		t.Fatalf("Down in a multiple selection selected %q, want b alone", got)
	}
	r.press(input.KeyDown, input.ModCtrl) // focus c
	r.press(input.KeySpace, 0)            // toggles c, keeps b
	if got := selectedKeys(sel); got != "b,c" {
		t.Fatalf("Space toggled to %q, want b,c", got)
	}
	r.press(input.KeyDown, input.ModShift) // from the anchor c to d
	if got := selectedKeys(sel); got != "c,d" {
		t.Fatalf("Shift+Down selected %q, want c,d", got)
	}
	r.press(input.KeyA, input.ModCtrl)
	if got := selectedKeys(sel); got != "a,b,c,d,e" {
		t.Fatalf("Ctrl+A selected %q, want every item", got)
	}
}

func TestListBoxEnterActivatesAndSpaceSelects(t *testing.T) {
	sel := NewSignal([]string(nil))
	var activated []string
	r := newAccessRig(func(*Context) Element {
		return listBoxDemo(sel, SelectionSingle, ListBoxStyle{}, nil,
			func(it lbItem) { activated = append(activated, it.name) }, nil)
	})
	r.focusKey(t, "list")
	r.press(input.KeyDown, 0) // b
	r.press(input.KeyEnter, 0)
	if got := strings.Join(activated, ","); got != "Beta" {
		t.Fatalf("Enter activated %q, want Beta", got)
	}
	r.press(input.KeyDown, input.ModCtrl) // focus c without selecting
	r.press(input.KeySpace, 0)
	if got := selectedKeys(sel); got != "c" {
		t.Fatalf("Space selected %q, want c", got)
	}
}

func TestListBoxTypeAheadMovesToTheNamedItem(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := newListBoxRig(sel, SelectionSingle)
	r.focusKey(t, "list")
	r.typeText("g")
	if got := selectedKeys(sel); got != "c" {
		t.Fatalf("typing g selected %q, want c (Gamma)", got)
	}
	r.wait(2 * time.Second) // a pause starts a new search
	r.typeText("ep")
	if got := selectedKeys(sel); got != "e" {
		t.Fatalf("typing ep selected %q, want e (Epsilon)", got)
	}
}

func TestListBoxClickAndModifiedClick(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := newAccessRig(func(*Context) Element { return listBoxDemo(sel, SelectionMultiple, ListBoxStyle{}, nil, nil, nil) })
	clickItem := func(key string, mods input.Modifiers) {
		t.Helper()
		rc := r.rt.nodeRect(r.nodeByKey(t, key))
		x, y := rc.X+rc.W/2, rc.Y+rc.H/2
		r.rt.PointerMove(x, y, 0)
		r.rt.PointerDown(x, y, input.ButtonLeft, 1, mods)
		r.rt.PointerUp(x, y, input.ButtonLeft, mods)
		r.frame()
	}
	clickItem("b", 0)
	if got := selectedKeys(sel); got != "b" {
		t.Fatalf("click selected %q, want b", got)
	}
	clickItem("d", input.ModCtrl)
	if got := selectedKeys(sel); got != "b,d" {
		t.Fatalf("Ctrl+click selected %q, want b,d", got)
	}
	// The anchor is the last Ctrl+click, d: Shift+click on a selects a through d.
	clickItem("a", input.ModShift)
	if got := selectedKeys(sel); got != "a,b,c,d" {
		t.Fatalf("Shift+click from the anchor selected %q, want a,b,c,d", got)
	}
}

func TestListBoxRightPressSelectsAndOpensItsMenu(t *testing.T) {
	sel := NewSignal([]string{"a"})
	var built []string
	menu := func(it lbItem) []MenuItem {
		built = append(built, it.name)
		return []MenuItem{{Label: "Copy name", OnClick: func() {}}}
	}
	r := newAccessRig(func(*Context) Element {
		return listBoxDemo(sel, SelectionSingle, ListBoxStyle{}, nil, nil, menu)
	})
	rc := r.rt.nodeRect(r.nodeByKey(t, "c"))
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	r.frame()
	if got := selectedKeys(sel); got != "c" {
		t.Fatalf("right-press selected %q, want c, as File Explorer does", got)
	}
	if r.rt.ctxMenu == nil || !slices.Equal(built, []string{"Gamma"}) {
		t.Fatalf("right-press built the menu for %v, want Gamma alone", built)
	}
}

func TestListBoxShiftF10OpensTheMenuOfTheFocusedItem(t *testing.T) {
	sel := NewSignal([]string{"b"})
	var built []string
	menu := func(it lbItem) []MenuItem {
		built = append(built, it.name)
		return []MenuItem{{Label: "Copy name", OnClick: func() {}}}
	}
	r := newAccessRig(func(*Context) Element {
		return listBoxDemo(sel, SelectionSingle, ListBoxStyle{}, nil, nil, menu)
	})
	r.focusKey(t, "list")
	r.press(input.KeyF10, input.ModShift)
	if r.rt.ctxMenu == nil {
		t.Fatal("Shift+F10 did not open the menu")
	}
	if !slices.Equal(built, []string{"Beta"}) {
		t.Fatalf("the menu was built for %v, want the focused item Beta", built)
	}
}

func TestListBoxFocusedItemIsTheAccessibilityFocus(t *testing.T) {
	r := newListBoxRig(NewSignal([]string{"b"}), SelectionSingle)
	r.focusKey(t, "list")
	r.press(input.KeyDown, 0)
	if !r.focusedAccess(t, access.RoleListItem, "Gamma") {
		t.Fatal("the focused item is not Gamma after Down")
	}
	if n := r.stateOf(t, access.RoleListItem, "Gamma"); !n.States.Has(access.StateSelected) {
		t.Error("Gamma is not reported as selected")
	}
}

func TestListBoxItemsAreNamedByItemName(t *testing.T) {
	r := newListBoxRig(NewSignal([]string(nil)), SelectionSingle)
	for _, it := range lbItems {
		r.stateOf(t, access.RoleListItem, it.name)
	}
	r.stateOf(t, access.RoleList, "Letters")
}

func TestListBoxRowsAreBuiltOnlyInView(t *testing.T) {
	items := make([]lbItem, 2000)
	for i := range items {
		items[i] = lbItem{key: "k" + strconv.Itoa(i), name: "Item " + strconv.Itoa(i)}
	}
	r := newAccessRig(func(*Context) Element {
		return ListBox(items, func(it lbItem) string { return it.key }, NewSignal([]string(nil)),
			func(it lbItem, s PartState) Element { return Text(it.name) }).Height(Px(200))
	})
	built := len(partNodes(r.rt, "ListBox.Item"))
	if built == 0 || built > 40 {
		t.Fatalf("built %d rows of 2000 in a 200 px list, want only the rows in view", built)
	}
}

func TestListBoxHighContrastSelectionIsTheAccentFill(t *testing.T) {
	th := HighContrastTheme(SystemColors{}, DarkTheme())
	st := resolveListBoxFor(t, true)
	if st.ItemSelected.Background != th.Accent || st.ItemSelected.TextColor != th.AccentText {
		t.Fatalf("high contrast selected item is %v on %v, want AccentText on Accent",
			st.ItemSelected.TextColor, st.ItemSelected.Background)
	}
}

// resolveListBoxFor returns the defaults of the list box for the dark theme, or the
// high contrast theme when hc is set.
func resolveListBoxFor(t *testing.T, hc bool) ListBoxStyle {
	t.Helper()
	th := DarkTheme()
	if hc {
		th = HighContrastTheme(SystemColors{}, DarkTheme())
	}
	return listBoxDefaults(th)
}

func TestListBoxDefaultsDoNotChangeFontWeight(t *testing.T) {
	for _, st := range []ListBoxStyle{resolveListBoxFor(t, false), resolveListBoxFor(t, true)} {
		for name, s := range map[string]Style{"Item": st.Item, "ItemHover": st.ItemHover,
			"ItemSelected": st.ItemSelected, "ItemFocusVisible": st.ItemFocusVisible} {
			if s.FontWeight != 0 {
				t.Errorf("%s sets a font weight, which a state must not change", name)
			}
		}
	}
}

func testListBoxParts(t *testing.T, applied map[string]bool) {
	type listBoxCase struct {
		part  string
		set   func(*ListBoxStyle, Style)
		sel   []string
		key   string // the element whose node draws the part
		drawn string // the part whose node draws it
		act   func(*testing.T, *accessRig)
	}
	cases := []listBoxCase{
		{part: "ListBox.List", set: func(s *ListBoxStyle, m Style) { s.List = m }, key: "list", drawn: "ListBox.List"},
		{part: "ListBox.Item", set: func(s *ListBoxStyle, m Style) { s.Item = m }, key: "b", drawn: "ListBox.Item"},
		{part: "ListBox.ItemHover", set: func(s *ListBoxStyle, m Style) { s.ItemHover = m }, key: "b", drawn: "ListBox.Item",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "b")) }},
		{part: "ListBox.ItemSelected", set: func(s *ListBoxStyle, m Style) { s.ItemSelected = m }, sel: []string{"c"},
			key: "c", drawn: "ListBox.Item"},
		{part: "ListBox.ItemFocusVisible", set: func(s *ListBoxStyle, m Style) { s.ItemFocusVisible = m }, key: "a",
			drawn: "ListBox.Item", act: func(t *testing.T, r *accessRig) {
				// The list is the node that draws ListBox.List: a key can name more than one node.
				focusNode(r, partNode(t, r.rt, "ListBox.List", ""))
				// The list measures and rebuilds its rows over a few frames, so the focused
				// row is restyled after them.
				for range 3 {
					r.frame()
				}
			}},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ListBoxStyle
			c.set(&st, Style{Background: markBG})
			sel := NewSignal(c.sel)
			r := newAccessRig(func(*Context) Element {
				return listBoxDemo(sel, SelectionSingle, st, nil, nil, nil)
			})
			if c.act != nil {
				c.act(t, r)
			}
			wantBackground(t, partNode(t, r.rt, c.drawn, c.key), c.part, markBG)
		})
	}
}

// SelectedKey keeps a single selection in a string signal: the keys move it.
func TestListBoxSelectedKey(t *testing.T) {
	cur := NewSignal("b")
	var picked []string
	r := newAccessRig(func(*Context) Element {
		return ListBox([]string{"a", "b", "c"}, func(s string) string { return s }, nil,
			func(s string, _ PartState) Element { return Text(s) }).
			Key("list").Label("Letters").SelectedKey(cur).
			OnChange(func(keys []string) { picked = keys })
	})
	r.focusKey(t, "list") // the selected item, b, is the Tab stop
	r.press(input.KeyDown, 0)
	if cur.Peek() != "c" || strings.Join(picked, ",") != "c" {
		t.Fatalf("Down from b set the key to %q and OnChange to %v, want c", cur.Peek(), picked)
	}
}

// A list box with neither a keys signal nor SelectedKey keeps its selection itself.
func TestListBoxWithoutASignalKeepsItsOwnSelection(t *testing.T) {
	var picked []string
	r := newAccessRig(func(*Context) Element {
		return ListBox([]string{"a", "b"}, func(s string) string { return s }, nil,
			func(s string, _ PartState) Element { return Text(s) }).
			Key("list").Label("Letters").
			OnChange(func(keys []string) { picked = keys })
	})
	r.focusKey(t, "list")
	r.press(input.KeyDown, 0)
	if strings.Join(picked, ",") != "b" {
		t.Fatalf("Down selected %v, want b", picked)
	}
}
