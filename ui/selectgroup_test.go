package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// selectGroupRig is a button, a SelectAcross group of three selectable texts
// (keys a, b and c), and a lone selectable text (key lone) after the group, in
// a 500x400 window.
func selectGroupRig(t *testing.T) *editorRig {
	t.Helper()
	return newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).Child(Button("Before").Key("before")).
			Child(Div().Key("group").SelectAcross().Gap(12).
				Child(Text("Alpha one").Selectable().Key("a")).
				Child(Text("Bravo two").Selectable().Key("b")).
				Child(Text("Charlie three").Selectable().Key("c"))).
			Child(Text("Lone text").Selectable().Key("lone"))
	})
}

// pos returns the window position of byte i of the selectable text with key,
// on its first line's middle.
func (r *editorRig) pos(t *testing.T, key string, i int) (float32, float32) {
	t.Helper()
	n := r.rt.byID[r.rt.keyID(t, key)]
	l := n.st.sel.layout
	if l == nil {
		t.Fatalf("the text %q has not been laid out", key)
	}
	x, top := l.Caret(i)
	return n.abs.X + x, n.abs.Y + top + l.LineHeight/2
}

// dragKeys drags from byte i of the text k0 to byte j of the text k1.
func (r *editorRig) dragKeys(t *testing.T, k0 string, i int, k1 string, j int) {
	t.Helper()
	x0, y0 := r.pos(t, k0, i)
	x1, y1 := r.pos(t, k1, j)
	r.drag(x0, y0, x1, y1)
}

// copied returns what Ctrl+C puts on the clipboard.
func (r *editorRig) copied() string {
	r.rt.memClipboard = ""
	r.key(input.KeyC, ctrl)
	return r.rt.memClipboard
}

// selOf returns the part of the text with key that is selected.
func (r *editorRig) selOf(t *testing.T, key string) string {
	t.Helper()
	n := r.rt.byID[r.rt.keyID(t, key)]
	a, b := n.st.sel.bounds()
	return n.textEl.text[a:b]
}

// A drag from one text of a group into the next selects the tail of the first
// and the head of the second, and paints both.
func TestSelectAcrossTwoTexts(t *testing.T) {
	r := selectGroupRig(t)
	r.dragKeys(t, "a", len("Alpha "), "b", len("Bravo"))
	if got := r.copied(); got != "one\n\nBravo" {
		t.Fatalf("copied %q, want %q", got, "one\n\nBravo")
	}
	for key, want := range map[string]string{"a": "one", "b": "Bravo", "c": ""} {
		if got := r.selOf(t, key); got != want {
			t.Errorf("text %s has %q selected, want %q", key, got, want)
		}
	}
	for _, key := range []string{"a", "b"} {
		if s := r.rt.byID[r.rt.keyID(t, key)].st.sel; s.drawnGen != r.rt.paintGen {
			t.Errorf("the selection of text %s was not painted", key)
		}
	}
}

// A drag over three texts selects all of the middle one; a drag upward
// selects the same.
func TestSelectAcrossThreeTextsBothWays(t *testing.T) {
	const want = "one\n\nBravo two\n\nCharlie"
	r := selectGroupRig(t)
	r.dragKeys(t, "a", len("Alpha "), "c", len("Charlie"))
	if got := r.copied(); got != want {
		t.Fatalf("forward drag copied %q, want %q", got, want)
	}
	r.now = r.now.Add(time.Second)
	r.dragKeys(t, "c", len("Charlie"), "a", len("Alpha "))
	if got := r.copied(); got != want {
		t.Fatalf("backward drag copied %q, want %q", got, want)
	}
	if got := r.selOf(t, "b"); got != "Bravo two" {
		t.Errorf("the middle text has %q selected", got)
	}
}

// A drag that ends in the space between two texts selects to the end of the
// text above it, and one past the last text to the end of the group.
func TestSelectAcrossGapsAndEdges(t *testing.T) {
	r := selectGroupRig(t)
	b := r.rt.byID[r.rt.keyID(t, "b")]
	x0, y0 := r.pos(t, "a", len("Alpha "))
	r.drag(x0, y0, b.abs.X+5, b.abs.Bottom()+6)
	if got := r.copied(); got != "one\n\nBravo two" {
		t.Errorf("drag into the gap copied %q", got)
	}
	r.now = r.now.Add(time.Second)
	c := r.rt.byID[r.rt.keyID(t, "c")]
	r.drag(x0, y0, c.abs.Right()+40, c.abs.Y+c.abs.H/2)
	if got := r.copied(); got != "one\n\nBravo two\n\nCharlie three" {
		t.Errorf("drag past the end of the last line copied %q", got)
	}
}

// Shift+click extends the selection into another text of the group, and
// Ctrl+A selects the whole group.
func TestSelectAcrossShiftClickAndSelectAll(t *testing.T) {
	r := selectGroupRig(t)
	x, y := r.pos(t, "a", 2)
	r.click(x, y)
	x2, y2 := r.pos(t, "c", 3)
	r.rt.PointerDown(x2, y2, input.ButtonLeft, 1, input.ModShift)
	r.rt.PointerUp(x2, y2, input.ButtonLeft, input.ModShift)
	frame(r.rt)
	if got := r.copied(); got != "pha one\n\nBravo two\n\nCha" {
		t.Fatalf("Shift+click copied %q", got)
	}
	// Shift+click back above the anchor selects from it upward.
	r.now = r.now.Add(time.Second)
	x3, y3 := r.pos(t, "a", 0)
	r.rt.PointerDown(x3, y3, input.ButtonLeft, 1, input.ModShift)
	r.rt.PointerUp(x3, y3, input.ButtonLeft, input.ModShift)
	frame(r.rt)
	if got := r.copied(); got != "Al" {
		t.Fatalf("Shift+click before the anchor copied %q, want Al", got)
	}
	r.key(input.KeyA, ctrl)
	if got := r.copied(); got != "Alpha one\n\nBravo two\n\nCharlie three" {
		t.Fatalf("Ctrl+A copied %q", got)
	}
	if got := r.selOf(t, "lone"); got != "" {
		t.Errorf("Ctrl+A in the group selected %q of the lone text", got)
	}
}

// A double-click selects a word and a triple-click a paragraph, within one
// text of the group.
func TestSelectAcrossWordAndParagraph(t *testing.T) {
	r := selectGroupRig(t)
	x, y := r.pos(t, "b", 2)
	r.click(x, y)
	r.click(x, y)
	if got := r.copied(); got != "Bravo" && got != "Bravo " {
		t.Fatalf("double-click copied %q, want the word", got)
	}
	r.click(x, y)
	if got := r.copied(); got != "Bravo two" {
		t.Fatalf("triple-click copied %q, want the paragraph", got)
	}
}

// A press on another text of the group starts a new selection there; focus
// leaving the group clears the selection of all its texts.
func TestSelectAcrossFocus(t *testing.T) {
	r := selectGroupRig(t)
	r.dragKeys(t, "a", 0, "c", 4)
	r.now = r.now.Add(time.Second)
	r.dragKeys(t, "b", 0, "b", len("Bravo"))
	if got := r.copied(); got != "Bravo" {
		t.Fatalf("a new drag in the group copied %q", got)
	}
	if got := r.selOf(t, "a") + r.selOf(t, "c"); got != "" {
		t.Errorf("the old selection kept %q", got)
	}
	r.key(input.KeyA, ctrl)
	r.rt.setFocus(r.rt.keyID(t, "before"))
	frame(r.rt)
	for _, key := range []string{"a", "b", "c"} {
		if got := r.selOf(t, key); got != "" {
			t.Errorf("text %s kept %q selected after focus moved away", key, got)
		}
	}
}

// A lone selectable text beside a group is unchanged: a drag from it into the
// group selects only within it, and Ctrl+A selects only it.
func TestSelectableOutsideGroupUnchanged(t *testing.T) {
	r := selectGroupRig(t)
	lone := r.rt.byID[r.rt.keyID(t, "lone")]
	if selectGroup(lone) != nil {
		t.Fatal("the lone text is in a group")
	}
	x0, y0 := r.pos(t, "lone", len("Lone "))
	x1, y1 := r.pos(t, "b", 2)
	r.drag(x0, y0, x1, y1)
	// The pointer above the lone text, at the x of byte 2 of b, is at byte 2
	// of the lone text's line, as it was before groups.
	if got := r.copied(); got != "ne " {
		t.Errorf("a drag from the lone text up into the group copied %q, want %q", got, "ne ")
	}
	for _, key := range []string{"a", "b", "c"} {
		if got := r.selOf(t, key); got != "" {
			t.Errorf("text %s of the group has %q selected", key, got)
		}
	}
	r.key(input.KeyA, ctrl)
	if got := r.copied(); got != "Lone text" {
		t.Errorf("Ctrl+A copied %q", got)
	}
}

// The context menu's Copy copies the whole selection, and a right press
// inside the selection keeps it.
func TestSelectAcrossContextMenu(t *testing.T) {
	r := selectGroupRig(t)
	r.dragKeys(t, "a", len("Alpha "), "c", len("Charlie"))
	x, y := r.pos(t, "b", 2)
	r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonRight, 0)
	frame(r.rt)
	if r.rt.ctxMenu == nil {
		t.Fatal("no context menu opened")
	}
	r.rt.memClipboard = ""
	items := r.rt.selectableMenu(r.rt.keyID(t, "b"))
	if len(items) != 2 || items[0].Action != textCopy || items[0].Disabled {
		t.Fatalf("menu items %+v, want an enabled Copy first", items)
	}
	r.rt.copySelection(r.rt.keyID(t, "b"))
	if got := r.rt.memClipboard; got != "one\n\nBravo two\n\nCharlie" {
		t.Errorf("the menu's Copy copied %q", got)
	}
}

// Copied text separates the items of a list and the rows of a table by a
// line break, and the cells of a row by a tab.
func TestSelectAcrossBreaks(t *testing.T) {
	r := newEditorRig(t, func(ctx *Context) Element {
		item := func(s string) Element { return Div().Role(access.RoleListItem).Child(Text(s).Selectable()) }
		cell := func(s string) Element { return Div().Width(Px(120)).Child(Text(s).Selectable()) }
		return Div().Padding(10).SelectAcross().Gap(10).
			Child(Text("Intro").Selectable().Key("first")).
			Child(Div().Role(access.RoleList).Child(item("one")).Child(item("two"))).
			Child(Div().Role(access.RoleDataGrid).Grid(Auto, Auto).
				Child(cell("h1")).Child(cell("h2")).Child(cell("d1")).Child(cell("d2"))).
			Child(Text("End").Selectable().Key("last"))
	})
	r.dragKeys(t, "first", 0, "last", len("End"))
	want := "Intro\n\none\ntwo\n\nh1\th2\nd1\td2\n\nEnd"
	if got := r.copied(); got != want {
		t.Errorf("copied %q, want %q", got, want)
	}
}

// OnTextSelect around a group reports the whole selection, with bounds around
// all of it.
func TestSelectAcrossOnTextSelect(t *testing.T) {
	var got []TextSelection
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).OnTextSelect(func(s TextSelection) { got = append(got, s) }).
			Child(Div().SelectAcross().Gap(12).
				Child(Text("Alpha one").Selectable().Key("a")).
				Child(Text("Bravo two").Selectable().Key("b")))
	})
	r.dragKeys(t, "a", len("Alpha "), "b", len("Bravo"))
	s := lastSel(t, got)
	if s.Text != "one\n\nBravo" {
		t.Fatalf("reported %q", s.Text)
	}
	a, b := r.rt.byID[r.rt.keyID(t, "a")], r.rt.byID[r.rt.keyID(t, "b")]
	if s.Bounds.H < b.abs.Bottom()-a.abs.Y-1 {
		t.Errorf("bounds %+v do not cover both texts", s.Bounds)
	}
	if s.Last.Y < b.abs.Y-a.abs.Y {
		t.Errorf("last line %+v is not in the second text", s.Last)
	}
}

// A drag past the bottom of a scroll container scrolls it, so the selection
// grows into the texts below, as a drag in a text area does.
func TestSelectAcrossScrollsAtEdge(t *testing.T) {
	r := newEditorRig(t, func(ctx *Context) Element {
		g := Div().SelectAcross().Gap(10)
		for i := range 30 {
			g.Child(Text("Paragraph " + string(rune('A'+i))).Selectable().Key(fmt.Sprintf("p%d", i)))
		}
		return Div().Padding(10).Child(Div().Key("scroller").Height(Px(200)).ScrollY().Child(g))
	})
	sc := r.rt.byID[r.rt.keyID(t, "scroller")]
	x0, y0 := r.pos(t, "p0", 0)
	r.rt.PointerMove(x0, y0, 0)
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	for range 5 {
		r.rt.PointerMove(x0+20, sc.abs.Bottom()+30, 0)
		frame(r.rt)
	}
	r.rt.PointerUp(x0+20, sc.abs.Bottom()+30, input.ButtonLeft, 0)
	frame(r.rt)
	if sc.st.scrollY <= 0 {
		t.Fatal("the scroll container did not scroll")
	}
	if got := r.selOf(t, "p8"); got == "" {
		t.Errorf("the selection did not grow past the first view; p8 has none")
	}
}
