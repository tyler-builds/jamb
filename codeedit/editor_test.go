package codeedit

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// harness runs an editor in a headless runtime of 800 by 600, with reduced motion, so each frame
// draws once. The editor is clicked once, so that it has the keyboard focus.
type harness struct {
	t   testing.TB
	rt  *ui.Runtime
	ed  *EditorElement
	doc *Document
	now time.Time
}

// newHarness returns a harness that shows a new document of text in an editor, which is
// configured by opts.
func newHarness(t testing.TB, text string, opts ...func(*EditorElement) *EditorElement) *harness {
	t.Helper()
	doc := NewDocument(text)
	ed := Editor(doc).Label("Source").Size(ui.Px(480), ui.Px(300))
	for _, o := range opts {
		ed = o(ed)
	}
	root := func(*ui.Context) ui.Element { return ui.Div().Key("wrap").Child(ed) }
	rt := ui.NewRuntime(root, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.SetSystemSettings(ui.SystemSettings{ReducedMotion: true})
	h := &harness{t: t, rt: rt, ed: ed, doc: doc, now: time.Unix(1_000_000, 0)}
	h.frame()
	r, ok := rt.Find("wrap")
	if !ok {
		t.Fatal("the editor is not in the tree")
	}
	h.click(r.X+100, r.Y+40, 0, 1)
	h.frame()
	return h
}

// frame draws one frame, a little later than the last.
func (h *harness) frame() {
	h.now = h.now.Add(16 * time.Millisecond)
	h.rt.Frame(h.now)
}

// click presses and releases the left button at (x, y) of the window.
func (h *harness) click(x, y float32, mods input.Modifiers, clicks int) {
	h.rt.PointerDown(x, y, input.ButtonLeft, clicks, mods)
	h.rt.PointerUp(x, y, input.ButtonLeft, mods)
	h.frame()
}

// key presses k with mods, and draws the result.
func (h *harness) key(k input.Key, mods input.Modifiers) {
	h.rt.Press(k, mods)
}

// typeText types s, with the clock advanced so that each typing is a new undo step.
func (h *harness) typeText(s string) {
	h.now = h.now.Add(time.Minute)
	h.rt.Type(s)
}

// lineXY returns the window point of the cell col of the visible row vi, inside the text.
func (h *harness) lineXY(vi, col int) (float32, float32) {
	g := h.ed.view.geo
	r, ok := h.rt.Find("wrap")
	if !ok || !g.painted {
		h.t.Fatal("the editor has not been painted")
	}
	x := r.X + 1 + g.textX + float32(col)*g.charW + g.charW/2
	y := r.Y + 1 + float32(vi)*g.lineH + g.lineH/2 - h.ed.view.scrollY
	return x, y
}

func TestTypingAndNewlines(t *testing.T) {
	h := newHarness(t, "")
	h.typeText("hi")
	h.key(input.KeyEnter, 0)
	h.typeText("x")
	if got := h.doc.Text(); got != "hi\nx" {
		t.Fatalf("typed text = %q, want hi LF x", got)
	}
}

func TestMovementAndSelection(t *testing.T) {
	h := newHarness(t, "one two\nthree")
	h.doc.SetCaret(0)
	h.key(input.KeyRight, 0)
	if h.doc.Caret() != 1 {
		t.Fatalf("right arrow: caret %d, want 1", h.doc.Caret())
	}
	h.key(input.KeyRight, input.ModCtrl)
	if h.doc.Caret() != 3 {
		t.Fatalf("ctrl+right: caret %d, want 3", h.doc.Caret())
	}
	h.key(input.KeyRight, input.ModShift)
	if p := h.doc.Primary(); p.Anchor != 3 || p.Head != 4 {
		t.Fatalf("shift+right selection = %+v", p)
	}
	h.key(input.KeyA, input.ModCtrl)
	if got := h.doc.Selections(); len(got) != 1 || got[0].Head != h.doc.Len() {
		t.Fatalf("ctrl+a selections = %+v", got)
	}
	h.key(input.KeyDown, 0)
	if h.doc.RowOf(h.doc.Caret()) != 1 {
		t.Fatalf("down arrow went to row %d", h.doc.RowOf(h.doc.Caret()))
	}
}

func TestPageKeysUsePageOfRows(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	h := newHarness(t, b.String())
	h.doc.SetCaret(0)
	h.key(input.KeyPageDown, 0)
	if row := h.doc.RowOf(h.doc.Caret()); row < 10 {
		t.Fatalf("page down moved %d rows, want about a page", row)
	}
}

func TestUndoRedoKeys(t *testing.T) {
	h := newHarness(t, "")
	h.typeText("abc")
	h.key(input.KeyZ, input.ModCtrl)
	if h.doc.Text() != "" {
		t.Fatalf("ctrl+z gave %q, want the text undone", h.doc.Text())
	}
	h.key(input.KeyY, input.ModCtrl)
	if h.doc.Text() != "abc" {
		t.Fatalf("ctrl+y gave %q, want the text redone", h.doc.Text())
	}
}

func TestAltClickAddsCaret(t *testing.T) {
	h := newHarness(t, "alpha\nbeta\ngamma")
	x, y := h.lineXY(1, 2)
	h.click(x, y, input.ModAlt, 1)
	if got := len(h.doc.Selections()); got != 2 {
		t.Fatalf("alt+click gave %d selections, want 2", got)
	}
}

func TestClipboardRoundTrip(t *testing.T) {
	h := newHarness(t, "copy me")
	h.doc.SetSelections(Selection{Anchor: 0, Head: 4})
	h.key(input.KeyC, input.ModCtrl)
	h.doc.SetCaret(h.doc.Len())
	h.key(input.KeyV, input.ModCtrl)
	if h.doc.Text() != "copy mecopy" {
		t.Fatalf("paste gave %q, want the copied text at the caret", h.doc.Text())
	}
	h.key(input.KeyX, input.ModCtrl) // nothing selected: cut removes nothing
	if h.doc.Text() != "copy mecopy" {
		t.Fatalf("cut with a caret changed the text: %q", h.doc.Text())
	}
}

func TestReadOnlyRefusesChanges(t *testing.T) {
	h := newHarness(t, "fixed", func(e *EditorElement) *EditorElement { return e.ReadOnly(true) })
	h.doc.SetCaret(0)
	h.typeText("x")
	h.key(input.KeyDelete, 0)
	h.key(input.KeyEnter, 0)
	h.key(input.KeyZ, input.ModCtrl)
	if h.doc.Text() != "fixed" {
		t.Fatalf("a read-only editor changed to %q", h.doc.Text())
	}
	h.key(input.KeyRight, 0)
	if h.doc.Caret() != 1 {
		t.Fatalf("a read-only editor should still move the caret: %d", h.doc.Caret())
	}
}

func TestEnterKeepsIndentAndTabIndents(t *testing.T) {
	h := newHarness(t, "\tfoo")
	h.doc.SetCaret(h.doc.Len())
	h.key(input.KeyEnter, 0)
	if h.doc.Text() != "\tfoo\n\t" {
		t.Fatalf("enter gave %q, want the indent kept", h.doc.Text())
	}
	h.key(input.KeyTab, input.ModShift)
	if h.doc.Text() != "\tfoo\n" {
		t.Fatalf("shift+tab gave %q, want the indent removed", h.doc.Text())
	}
}

func TestFindBarOpensAndCounts(t *testing.T) {
	h := newHarness(t, "foo bar foo foo")
	h.doc.SetCaret(0)
	h.key(input.KeyF, input.ModCtrl)
	if h.ed.view.bar != barFind {
		t.Fatalf("ctrl+f opened bar %d, want find", h.ed.view.bar)
	}
	h.ed.view.query.Set("foo")
	h.frame()
	if n := len(h.ed.view.matches()); n != 3 {
		t.Fatalf("matches of foo = %d, want 3", n)
	}
	h.ed.view.findStep(true)
	if got := h.doc.Primary(); got.Anchor != 0 || got.Head != 3 {
		t.Fatalf("find next from the start selected %+v", got)
	}
	h.ed.view.findStep(true)
	h.ed.view.findStep(true)
	h.ed.view.findStep(true)
	if got := h.doc.Primary(); got.Anchor != 0 {
		t.Fatalf("find next wraps to the first match, selected %+v", got)
	}
	h.ed.view.findStep(false)
	if got := h.doc.Primary(); got.Anchor != 12 {
		t.Fatalf("find previous from the first match wraps to the last, selected %+v", got)
	}
}

func TestReplaceAllOneStep(t *testing.T) {
	h := newHarness(t, "cat cot cut")
	h.rt.Dispatch(CmdReplace)
	h.ed.view.query.Set("c.t")
	h.ed.view.repl.Set("dog")
	h.ed.view.optRegex.Set(true)
	h.frame()
	h.ed.view.replaceAll()
	if h.doc.Text() != "dog dog dog" {
		t.Fatalf("replace all = %q", h.doc.Text())
	}
	h.doc.Undo()
	if h.doc.Text() != "cat cot cut" {
		t.Fatalf("one undo of replace all = %q", h.doc.Text())
	}
}

func TestInvalidPatternShowsError(t *testing.T) {
	h := newHarness(t, "text")
	h.rt.Dispatch(CmdFind)
	h.ed.view.query.Set("(")
	h.ed.view.optRegex.Set(true)
	h.frame()
	if !h.ed.view.finderErr {
		t.Fatal("an invalid regular expression did not set the error")
	}
	if got := h.ed.view.countLabel(ui.EnglishStrings()); got != ui.EnglishStrings().CodeEditorBadPattern {
		t.Fatalf("count label = %q, want the invalid pattern message", got)
	}
}

func TestGoToLineMovesCaret(t *testing.T) {
	h := newHarness(t, "a\nb\nc\nd")
	h.rt.Dispatch(CmdGoToLine)
	h.ed.view.gotoLine.Set("3")
	h.ed.view.goToLine()
	h.frame()
	if h.doc.RowOf(h.doc.Caret()) != 2 {
		t.Fatalf("go to line 3 put the caret on row %d", h.doc.RowOf(h.doc.Caret()))
	}
}

func TestFoldAndUnfoldCommands(t *testing.T) {
	h := newHarness(t, "func a() {\n\tone\n}\nnext")
	h.doc.SetCaret(0)
	h.rt.Dispatch(CmdFold)
	if h.doc.VisibleRows() != 3 {
		t.Fatalf("fold at the caret left %d rows shown, want 3", h.doc.VisibleRows())
	}
	h.rt.Dispatch(CmdUnfold)
	if h.doc.VisibleRows() != 4 {
		t.Fatalf("unfold left %d rows shown, want 4", h.doc.VisibleRows())
	}
}

func TestToggleCommentCommand(t *testing.T) {
	h := newHarness(t, "x := 1", func(e *EditorElement) *EditorElement { return e.CommentPrefix("// ") })
	h.doc.SetCaret(0)
	h.rt.Dispatch(CmdComment)
	if h.doc.Text() != "// x := 1" {
		t.Fatalf("comment command gave %q", h.doc.Text())
	}
}

func TestTwoEditorsShareOneDocument(t *testing.T) {
	doc := NewDocument("shared")
	a := Editor(doc)
	b := Editor(doc)
	root := func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Child(ui.Div().Key("a").Child(a)).Child(ui.Div().Key("b").Child(b))
	}
	rt := ui.NewRuntime(root, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.SetSystemSettings(ui.SystemSettings{ReducedMotion: true})
	rt.Frame(time.Unix(1_000_000, 0))
	v := a.view.rev.Peek()
	if err := doc.Replace(0, 0, "x"); err != nil {
		t.Fatal(err)
	}
	rt.Frame(time.Unix(1_000_000, 1))
	if a.view.rev.Peek() == v || b.view.rev.Peek() == 0 {
		t.Fatal("a change to the shared document did not reach both editors")
	}
}

// fakeHighlighter records the edits it is sent, and gives fixed spans, as a session would.
type fakeHighlighter struct {
	edits    [][3]any
	viewport [2]int
	spans    []highlight.Span
	ver      uint64
	names    []string
	closed   bool
}

func (f *fakeHighlighter) Edit(e highlight.EditRecord) error {
	f.edits = append(f.edits, [3]any{e.Start, e.OldEnd, string(e.Text)})
	return nil
}

func (f *fakeHighlighter) SetViewport(lo, hi int) error {
	f.viewport = [2]int{lo, hi}
	return nil
}

func (f *fakeHighlighter) Spans() ([]highlight.Span, uint64) { return f.spans, f.ver }

func (f *fakeHighlighter) CaptureNames() []string { return f.names }

func (f *fakeHighlighter) Close() error {
	f.closed = true
	return nil
}

// TestHighlighterFollowsEditsAndViewport checks that each change reaches the highlighter with the
// byte range it replaced, and that the viewport is the bytes of the rows shown.
func TestHighlighterFollowsEditsAndViewport(t *testing.T) {
	fake := &fakeHighlighter{names: []string{"keyword"}, spans: []highlight.Span{{Start: 0, End: 2, Capture: 0}}, ver: 1}
	h := newHarness(t, "if x", func(e *EditorElement) *EditorElement { return e.Highlighter(fake) })
	h.doc.SetCaret(0)
	h.frame()
	if fake.viewport[1] != len("if x") {
		t.Fatalf("viewport = %v, want the whole short text", fake.viewport)
	}
	h.typeText("q")
	if len(fake.edits) == 0 || fake.edits[len(fake.edits)-1] != [3]any{0, 0, "q"} {
		t.Fatalf("the highlighter got %v, want the insertion at 0", fake.edits)
	}
	h.key(input.KeyDelete, 0)
	last := fake.edits[len(fake.edits)-1]
	if last[0] != 1 || last[1] != 2 || last[2] != "" {
		t.Fatalf("delete sent %v, want [1, 2) replaced with nothing", last)
	}
	// The span colors the keyword, and the theme maps the capture to its style.
	if !h.ed.view.capStyle[0].ok {
		t.Fatal("the keyword capture has no style, so it would not be colored")
	}
}

// TestSyntaxStylesMapCaptures checks that the capture names are looked up in the theme's syntax styles
// by their closest name.
func TestSyntaxStylesMapCaptures(t *testing.T) {
	fake := &fakeHighlighter{names: []string{"string.escape.go", "unknown"}, ver: 1}
	h := newHarness(t, "x", func(e *EditorElement) *EditorElement { return e.Highlighter(fake) })
	h.frame()
	cs := h.ed.view.capStyle
	if len(cs) != 2 || !cs[0].ok || cs[1].ok {
		t.Fatalf("capture styles = %+v, want the first mapped and the second not", cs)
	}
}

// The sizing methods size the editor's box as they size a widget of the ui package: Grow
// takes the free space of a row, MaxWidth caps the width, MinHeight raises the height,
// and GridSpan covers the columns of a grid.
func TestSizingMethods(t *testing.T) {
	root := func(*ui.Context) ui.Element {
		return ui.Div().Column().
			Child(ui.Div().Row().Width(ui.Px(600)).
				Child(Editor(NewDocument("a")).Width(ui.Px(100)).Grow(1).Key("grow")).
				Child(ui.Div().Width(ui.Px(100)).Height(ui.Px(10)))).
			Child(ui.Div().Row().Width(ui.Px(600)).
				Child(Editor(NewDocument("b")).MaxWidth(ui.Px(250)).Height(ui.Px(100)).MinHeight(ui.Px(140)).Key("capped"))).
			Child(ui.Div().Grid(ui.Fr(1), ui.Fr(1)).Width(ui.Px(600)).
				Child(Editor(NewDocument("c")).Width(ui.Auto).GridSpan(2, 1).Height(ui.Px(80)).Key("span")))
	}
	rt := ui.NewRuntime(root, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Unix(1_000_000, 0))
	find := func(key string) ui.Rect {
		rc, ok := rt.Find(key)
		if !ok {
			t.Fatalf("no element with key %q", key)
		}
		return rc
	}
	if grow := find("grow"); grow.W != 500 {
		t.Errorf("a growing editor beside 100 pixels in a row of 600 is %v wide, want 500", grow.W)
	}
	if capped := find("capped"); capped.W != 250 || capped.H != 140 {
		t.Errorf("the capped editor is %v by %v, want 250 by 140", capped.W, capped.H)
	}
	if span := find("span"); span.W != 600 {
		t.Errorf("an editor spanning two columns of 600 pixels is %v wide", span.W)
	}
}

// A size the app sets wins over the Box style, whatever the order of the calls, and the
// Box style wins over the default size.
func TestSetSizeWinsOverTheBoxStyle(t *testing.T) {
	box := Style{Box: ui.Style{Width: ui.Px(200), Height: ui.Px(120)}}
	root := func(*ui.Context) ui.Element {
		return ui.Div().Column().Width(ui.Px(600)).
			Child(Editor(NewDocument("a")).Styles(box).Key("box")).
			Child(Editor(NewDocument("b")).Size(ui.Px(400), ui.Px(90)).Styles(box).Key("set")).
			Child(Editor(NewDocument("c")).Styles(box).Width(ui.Px(300)).Key("width"))
	}
	rt := ui.NewRuntime(root, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Unix(1_000_000, 0))
	for _, c := range []struct {
		key  string
		w, h float32
	}{{"box", 200, 120}, {"set", 400, 90}, {"width", 300, 120}} {
		rc, ok := rt.Find(c.key)
		if !ok {
			t.Fatalf("no element with key %q", c.key)
		}
		if rc.W != c.w || rc.H != c.h {
			t.Errorf("editor %q is %v by %v, want %v by %v", c.key, rc.W, rc.H, c.w, c.h)
		}
	}
}
