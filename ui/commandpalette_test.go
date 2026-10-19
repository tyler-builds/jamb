package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// The actions of the palette rig. The document handles bold and italic, the root
// handles the others; hidden is never listed.
var (
	paletteShow   = NewAction("test.palette", "Show palette").Hide()
	paletteBold   = NewAction("test.bold", "Bold text")
	paletteItalic = NewAction("test.italic", "Italic text")
	paletteNew    = NewAction("test.new", "New document")
	paletteHidden = NewAction("test.hidden", "Hidden command").Hide()
)

// paletteRig is a window with a focusable document, a second focusable element at the
// bottom and a
// command palette that Ctrl+Shift+P opens. ran records the commands that ran, with
// the key of the element that had focus when they ran.
type paletteRig struct {
	*accessRig
	open *Signal[bool]
	ran  []string
}

func newPaletteRig(t *testing.T, build func(p *CommandPaletteElement) *CommandPaletteElement) *paletteRig {
	t.Helper()
	pr := &paletteRig{open: NewSignal(false)}
	record := func(name string) func() {
		return func() { pr.ran = append(pr.ran, name+"@"+pr.focusedKey()) }
	}
	pr.accessRig = newAccessRig(func(ctx *Context) Element {
		p := CommandPalette(pr.open)
		if build != nil {
			p = build(p)
		}
		return Div().Column().Size(Full, Full).Gap(8).
			OnAction(paletteShow, func() { pr.open.Set(true) }).
			OnAction(paletteNew, record("new")).
			OnAction(paletteHidden, record("hidden")).
			Child(Div().Key("doc").Focusable().Size(Px(200), Px(30)).Label("Document").
				OnAction(paletteBold, record("bold")).
				OnAction(paletteItalic, record("italic"))).
			// The other element is at the bottom, away from the palette's panel.
			Child(Div().Grow(1)).
			Child(Div().Key("other").Focusable().Size(Px(200), Px(30)).Label("Other")).
			Child(p)
	})
	pr.rt.AddBindings(Bind("ctrl+shift+p", paletteShow), Bind("ctrl+b", paletteBold))
	pr.focusKey(t, "doc")
	return pr
}

// openPalette presses Ctrl+Shift+P and lets the field take focus.
func (pr *paletteRig) openPalette(t *testing.T) {
	t.Helper()
	pr.press(input.KeyP, input.ModCtrl|input.ModShift)
	pr.frame()
	if _, ok := pr.rt.Find("jamb.palette"); !ok {
		t.Fatal("Ctrl+Shift+P did not open the palette")
	}
}

// listed returns the names of the commands the palette lists, in order.
func (pr *paletteRig) listed() []string {
	var out []string
	list := partNodes(pr.rt, "CommandPalette.List")
	if len(list) != 1 {
		return nil
	}
	for _, c := range list[0].children {
		if c.div != nil && c.div.part == "CommandPalette.Item" {
			out = append(out, c.div.acc.name)
		}
	}
	return out
}

// highlighted returns the name of the highlighted row, or "".
func (pr *paletteRig) highlighted() string {
	for _, n := range partNodes(pr.rt, "CommandPalette.Item") {
		if n.div.acc.highlighted {
			return n.div.acc.name
		}
	}
	return ""
}

// The palette lists the commands available where focus was, without the hidden ones,
// and the field takes focus.
func TestCommandPaletteListsAvailableCommands(t *testing.T) {
	r := newPaletteRig(t, nil)
	if _, ok := r.rt.Find("jamb.palette"); ok {
		t.Fatal("the palette is open before it was asked for")
	}
	r.openPalette(t)
	if got, want := r.listed(), []string{"Bold text", "Italic text", "New document"}; !slices.Equal(got, want) {
		t.Fatalf("listed %v, want %v", got, want)
	}
	if n := r.rt.byID[r.rt.focusID]; n == nil || n.kind != kindEditor {
		t.Error("the palette's field does not have focus")
	}
	if r.highlighted() != "Bold text" {
		t.Errorf("highlighted %q, want the first command", r.highlighted())
	}
	// Focused on the other element, the document's commands are not available.
	r.press(input.KeyEscape, 0)
	r.focusKey(t, "other")
	r.openPalette(t)
	if got, want := r.listed(), []string{"New document"}; !slices.Equal(got, want) {
		t.Fatalf("listed from the other element %v, want %v", got, want)
	}
}

// Every word of the query must occur in the name, in any order and ignoring case, and
// the matches are highlighted in the name.
func TestCommandPaletteFiltersByEveryWord(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.openPalette(t)
	r.typeText("TEXT b")
	if got, want := r.listed(), []string{"Bold text"}; !slices.Equal(got, want) {
		t.Fatalf("listed %v for \"TEXT b\", want %v", got, want)
	}
	for _, n := range r.rt.byID {
		if n.textEl != nil && n.textEl.text == "Bold text" {
			var got [][2]int
			for _, h := range n.textEl.highlights {
				got = append(got, [2]int{h.start, h.end})
			}
			if want := [][2]int{{0, 1}, {5, 9}}; !slices.Equal(got, want) {
				t.Errorf("highlights %v, want %v", got, want)
			}
		}
	}
	r.typeText("zz")
	if got := r.listed(); len(got) != 0 {
		t.Fatalf("listed %v for a query that matches nothing", got)
	}
	empty := partNode(t, r.rt, "CommandPalette.Empty", "")
	if empty.children[0].textEl.text != "No matching commands" {
		t.Errorf("empty message %q", empty.children[0].textEl.text)
	}
}

// Up and Down move the highlight and stop at the ends; PageUp and PageDown move it by
// MaxRows; typing moves it back to the first command.
func TestCommandPaletteKeysMoveTheHighlight(t *testing.T) {
	r := newPaletteRig(t, func(p *CommandPaletteElement) *CommandPaletteElement { return p.MaxRows(2) })
	r.openPalette(t)
	steps := []struct {
		key  input.Key
		want string
	}{
		{input.KeyUp, "Bold text"}, // the first row stays
		{input.KeyDown, "Italic text"},
		{input.KeyDown, "New document"},
		{input.KeyDown, "New document"}, // the last row stays
		{input.KeyPageUp, "Bold text"},
		{input.KeyPageDown, "New document"},
	}
	for _, s := range steps {
		r.press(s.key, 0)
		if got := r.highlighted(); got != s.want {
			t.Fatalf("after %v: highlighted %q, want %q", s.key, got, s.want)
		}
	}
	r.typeText("ita")
	if got := r.highlighted(); got != "Italic text" {
		t.Errorf("after typing: highlighted %q, want the first match", got)
	}
}

// Enter runs the highlighted command where focus was before the palette opened, then
// focus is back there and the palette is closed.
func TestCommandPaletteEnterRunsWhereFocusWas(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.openPalette(t)
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	if want := []string{"italic@doc"}; !slices.Equal(r.ran, want) {
		t.Fatalf("ran %v, want %v", r.ran, want)
	}
	if r.open.Peek() {
		t.Error("the palette is still open after running a command")
	}
	r.frame()
	if got := r.focusedKey(); got != "doc" {
		t.Errorf("focus on %q after the command ran, want doc", got)
	}
	if _, ok := r.rt.Find("jamb.palette"); ok {
		t.Error("the palette is still in the tree")
	}
}

// Escape closes the palette and gives focus back; a click outside closes it and
// leaves focus where the click put it.
func TestCommandPaletteEscapeAndClickOutside(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.openPalette(t)
	r.press(input.KeyEscape, 0)
	if r.open.Peek() {
		t.Fatal("Escape did not close the palette")
	}
	if got := r.focusedKey(); got != "doc" {
		t.Errorf("focus on %q after Escape, want doc", got)
	}
	if len(r.ran) != 0 {
		t.Errorf("Escape ran %v", r.ran)
	}

	r.openPalette(t)
	r.clickKey(t, "other")
	if r.open.Peek() {
		t.Fatal("a click outside did not close the palette")
	}
	if got := r.focusedKey(); got != "other" {
		t.Errorf("focus on %q after a click outside, want other", got)
	}
}

// The pointer highlights the row it is over, and a click runs that row's command
// where focus was.
func TestCommandPaletteHoverAndClick(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.openPalette(t)
	rc := r.rt.nodeRect(r.nodeByKey(t, "jamb.palette/test.new"))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	if got := r.highlighted(); got != "New document" {
		t.Errorf("highlighted %q under the pointer, want New document", got)
	}
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if want := []string{"new@doc"}; !slices.Equal(r.ran, want) {
		t.Fatalf("ran %v, want %v", r.ran, want)
	}
	if r.open.Peek() {
		t.Error("the palette is still open after a click on a command")
	}
}

// Commands replaces the available actions, still without the hidden ones; a command
// runs where focus was, so one that nothing there handles does nothing.
func TestCommandPaletteCommands(t *testing.T) {
	calls := 0
	r := newPaletteRig(t, func(p *CommandPaletteElement) *CommandPaletteElement {
		return p.Commands(func() []*Action {
			calls++
			return []*Action{paletteNew, paletteHidden, paletteBold}
		})
	})
	r.openPalette(t)
	if got, want := r.listed(), []string{"New document", "Bold text"}; !slices.Equal(got, want) {
		t.Fatalf("listed %v, want %v", got, want)
	}
	r.typeText("x")
	if calls != 1 {
		t.Errorf("Commands ran %d times while the palette was open, want once", calls)
	}
	r.press(input.KeyEscape, 0)
	r.focusKey(t, "other")
	r.openPalette(t)
	r.typeText("bold")
	r.press(input.KeyEnter, 0)
	if len(r.ran) != 0 {
		t.Errorf("ran %v from the other element, which does not handle Bold", r.ran)
	}
}

// Each row shows its key binding, which assistive technology reads as the row's
// description.
func TestCommandPaletteShowsKeys(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.openPalette(t)
	keys := partNode(t, r.rt, "CommandPalette.Keys", "")
	if keys.children[0].textEl.text != "Ctrl+B" {
		t.Errorf("keys %q, want Ctrl+B", keys.children[0].textEl.text)
	}
	row := accesstest.Find(t, r.tree(), access.RoleListItem, "Bold text")
	if row.Description != "Ctrl+B" {
		t.Errorf("row description %q, want Ctrl+B", row.Description)
	}
}

// The palette is a dialog with a combo box: the field keeps focus, and the
// highlighted row is the focus that assistive technology reports, as in a Combobox.
func TestCommandPaletteAccessibility(t *testing.T) {
	r := newPaletteRig(t, func(p *CommandPaletteElement) *CommandPaletteElement { return p.Placeholder("Search") })
	r.openPalette(t)
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleDialog, "Command palette")
	combo := accesstest.Find(t, tr, access.RoleComboBox, "Command palette")
	if !combo.States.Has(access.StateExpanded) {
		t.Error("the combo box is not expanded")
	}
	accesstest.Find(t, tr, access.RoleList, "Command palette")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleListItem, "Bold text"), access.RoleListItem, "Bold text",
		access.StateSelectable|access.StateSelected)
	if !r.focusedAccess(t, access.RoleListItem, "Bold text") {
		t.Error("the highlighted row is not the focus of the tree")
	}
	r.press(input.KeyDown, 0)
	if !r.focusedAccess(t, access.RoleListItem, "Italic text") {
		t.Error("Down did not move the tree's focus to the next row")
	}
	for _, w := range r.warnings {
		t.Errorf("warning: %s", w)
	}
	var ph string
	for _, n := range r.rt.byID {
		if n.kind == kindEditor {
			ph = n.editor.placeholder
		}
	}
	if ph != "Search" {
		t.Errorf("placeholder %q, want Search", ph)
	}
}

// The name, the placeholder and the empty message follow the window's language.
func TestCommandPaletteStrings(t *testing.T) {
	r := newPaletteRig(t, nil)
	r.rt.SetStrings(StringsFor("it"))
	r.openPalette(t)
	r.typeText("zz")
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleDialog, "Riquadro comandi")
	empty := partNode(t, r.rt, "CommandPalette.Empty", "")
	if got := empty.children[0].textEl.text; got != "Nessun comando corrispondente" {
		t.Errorf("empty message %q", got)
	}
}

// testCommandPaletteParts checks the palette's parts.
func testCommandPaletteParts(t *testing.T, applied map[string]bool) {
	type paletteCase struct {
		part  string
		set   func(*CommandPaletteStyle, Style)
		query string // the text typed
		key   string // the row that draws the part, for a row's part
	}
	cases := []paletteCase{
		{part: "CommandPalette.Panel", set: func(s *CommandPaletteStyle, m Style) { s.Panel = m }},
		{part: "CommandPalette.Input", set: func(s *CommandPaletteStyle, m Style) { s.Input = m }},
		{part: "CommandPalette.List", set: func(s *CommandPaletteStyle, m Style) { s.List = m }},
		{part: "CommandPalette.Item", set: func(s *CommandPaletteStyle, m Style) { s.Item = m }, key: "jamb.palette/test.italic"},
		{part: "CommandPalette.ItemHighlighted", set: func(s *CommandPaletteStyle, m Style) { s.ItemHighlighted = m },
			key: "jamb.palette/test.bold"},
		{part: "CommandPalette.Label", set: func(s *CommandPaletteStyle, m Style) { s.Label = m }},
		{part: "CommandPalette.Keys", set: func(s *CommandPaletteStyle, m Style) { s.Keys = m }},
		{part: "CommandPalette.Empty", set: func(s *CommandPaletteStyle, m Style) { s.Empty = m }, query: "zz"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st CommandPaletteStyle
			c.set(&st, Style{Background: markBG})
			r := newPaletteRig(t, func(p *CommandPaletteElement) *CommandPaletteElement { return p.Styles(st) })
			r.openPalette(t)
			if c.query != "" {
				r.typeText(c.query)
			}
			if c.key != "" {
				wantBackground(t, r.nodeByKey(t, c.key), c.part, markBG)
				return
			}
			nodes := partNodes(r.rt, c.part)
			if len(nodes) == 0 {
				t.Fatalf("no %s", c.part)
			}
			wantBackground(t, nodes[0], c.part, markBG)
		})
	}
	// The match is drawn in the names: the highlight of the match has the Match
	// background.
	applied["CommandPalette.Match"] = true
	t.Run("CommandPalette.Match", func(t *testing.T) {
		st := CommandPaletteStyle{Match: Style{Background: markBG}}
		r := newPaletteRig(t, func(p *CommandPaletteElement) *CommandPaletteElement { return p.Styles(st) })
		r.openPalette(t)
		r.typeText("bold")
		for _, n := range r.rt.byID {
			if n.textEl != nil && n.textEl.text == "Bold text" {
				if len(n.textEl.highlights) == 0 || n.textEl.highlights[0].hl.Background != markBG {
					t.Fatalf("the match in Bold text is not drawn with the Match background")
				}
				return
			}
		}
		t.Fatal("no Bold text label")
	})
}

// The defaults never change the weight of the text for the highlighted row.
func TestCommandPaletteHighlightKeepsTheWeight(t *testing.T) {
	for _, th := range []*Theme{DarkTheme(), LightTheme(), HighContrastTheme(HighContrastBlack(), DarkTheme())} {
		d := commandPaletteDefaults(th)
		if d.ItemHighlighted.FontWeight != 0 || d.Item.FontWeight != 0 {
			t.Errorf("the defaults set a font weight on a row: %v, %v", d.Item.FontWeight, d.ItemHighlighted.FontWeight)
		}
	}
}
