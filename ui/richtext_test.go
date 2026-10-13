package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/text"
)

// paintNow repaints the tree and returns the scene.
func paintNow(rt *Runtime) *scene.Scene {
	rt.needPaint = true
	return rt.Frame(time.Now())
}

// richNode returns the rich text node with key.
func richNode(t *testing.T, rt *Runtime, key string) *node {
	t.Helper()
	for _, n := range rt.byID {
		if n.key == key && n.rich != nil {
			return n
		}
	}
	t.Fatalf("no rich text with key %q", key)
	return nil
}

// caretPoint returns a point just inside the text at byte offset off of the
// rich text node with key, in window coordinates.
func caretPoint(t *testing.T, rt *Runtime, key string, off int) (float32, float32) {
	t.Helper()
	n := richNode(t, rt, key)
	l := n.rich.lay
	cx, top := l.Caret(off)
	return n.abs.X + n.style.Padding.Left + cx + 1, n.abs.Y + n.style.Padding.Top + top + l.LineHeight/2
}

// paintAt returns the paint of the span containing byte offset off.
func paintAt(t *testing.T, n *node, off int) richPaint {
	t.Helper()
	for i, sp := range n.rich.spans {
		if off >= sp.Start && off < sp.End {
			return n.rich.paint[i]
		}
	}
	t.Fatalf("no span covers byte %d", off)
	return richPaint{}
}

// countQuads counts the quads of sc with background bg and height h.
func countQuads(sc *scene.Scene, bg scene.Color, h float32) int {
	n := 0
	for _, q := range sc.Quads {
		if q.Background == bg && q.Bounds.H == h {
			n++
		}
	}
	return n
}

// countGlyphs counts the glyphs of sc drawn in color c.
func countGlyphs(sc *scene.Scene, c scene.Color) int {
	n := 0
	for _, g := range sc.Glyphs {
		if g.Color == c {
			n++
		}
	}
	return n
}

// mustPanic runs fn and checks that it panics with a message containing want.
func mustPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("no panic, want one containing %q", want)
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, want) {
			t.Errorf("panic %q does not contain %q", msg, want)
		}
	}()
	fn()
}

func TestRichJoinsSegmentsIntoOneBoldSpan(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Seg("a"), Seg("b").Bold()).Key("r"))
	}, 400, 300, 1)
	n := richNode(t, rt, "r")
	if n.textEl.text != "ab" {
		t.Fatalf("text = %q, want %q", n.textEl.text, "ab")
	}
	if len(n.rich.spans) != 2 || n.rich.spans[1].Start != 1 || n.rich.spans[1].End != 2 || n.rich.spans[1].Weight != FontBold {
		t.Fatalf("spans = %+v, want a bold span over b", n.rich.spans)
	}
	st := n.textStyle()
	bold := st
	bold.Weight = text.Bold
	want := rt.text.Layout("a", st, 1e6).Width + rt.text.Layout("b", bold, 1e6).Width
	if got := n.rich.lay.Width; got < want-0.01 || got > want+0.01 {
		t.Errorf("layout width %v, want the sum of the shaped pieces %v", got, want)
	}
}

func TestHighlightsMergeLaterWinsPerField(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Seg("hello world")).
			Highlight(0, 5, Highlight{Color: red, Weight: FontBold}).
			Highlight(3, 8, Highlight{Color: blue, Italic: true}).Key("r"))
	}, 400, 300, 1)
	n := richNode(t, rt, "r")
	if p := paintAt(t, n, 1); p.hl.Color != red || p.hl.Weight != FontBold || p.hl.Italic {
		t.Errorf("[0,3) = %+v, want red bold", p.hl)
	}
	if p := paintAt(t, n, 4); p.hl.Color != blue || p.hl.Weight != FontBold || !p.hl.Italic {
		t.Errorf("[3,5) = %+v, want blue (later wins) bold italic", p.hl)
	}
	if p := paintAt(t, n, 6); p.hl.Color != blue || p.hl.Weight != 0 || !p.hl.Italic {
		t.Errorf("[5,8) = %+v, want blue italic", p.hl)
	}
	if p := paintAt(t, n, 9); p.hl != (Highlight{}) {
		t.Errorf("[8,11) = %+v, want no highlight", p.hl)
	}
}

func TestHighlightOverSegments(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Seg("ab").Color(red), Seg("cd")).
			Highlight(1, 3, Highlight{Color: blue}).Key("r"))
	}, 400, 300, 1)
	n := richNode(t, rt, "r")
	if c := paintAt(t, n, 0).hl.Color; c != red {
		t.Errorf("a = %v, want red from its segment", c)
	}
	if c := paintAt(t, n, 1).hl.Color; c != blue {
		t.Errorf("b = %v, want blue from the highlight", c)
	}
	if c := paintAt(t, n, 2).hl.Color; c != blue {
		t.Errorf("c = %v, want blue from the highlight", c)
	}
	if c := paintAt(t, n, 3).hl.Color; !c.IsZero() {
		t.Errorf("d = %v, want the text color", c)
	}
}

func TestHighlightPanicsExplainOffsets(t *testing.T) {
	mustPanic(t, "offset 4 is inside the UTF-8 encoding of 'é'; use byte offsets from strings.Index", func() {
		Rich(Seg("café")).Highlight(4, 5, Highlight{Weight: FontBold})
	})
	mustPanic(t, `Highlight(0, 99) on "café": offset 99 is outside the text (5 bytes)`, func() {
		Text("café").Highlight(0, 99, Highlight{})
	})
	mustPanic(t, "start after end", func() {
		Text("abc").Highlight(2, 1, Highlight{})
	})
}

func TestRichPaintsSegmentColorsBackgroundsAndUnderlines(t *testing.T) {
	red, yellow := RGB(255, 0, 0), RGB(255, 255, 0)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(Rich(
			Seg("ab"),
			Seg("cd").Color(red),
			Seg("ef").Background(yellow),
			Seg("gh").Underline(),
		).Key("r"))
	}, 400, 300, 1)
	sc := paintNow(rt)
	textColor := DarkTheme().Text.scene()
	if got := countGlyphs(sc, red.scene()); got != 2 {
		t.Errorf("red glyphs = %d, want 2 (the cd segment)", got)
	}
	if got := countGlyphs(sc, textColor); got != 6 {
		t.Errorf("text-colored glyphs = %d, want 6 (ab, ef, gh inherit the text color)", got)
	}
	bg := 0
	for _, q := range sc.Quads {
		if q.Background == yellow.scene() {
			bg++
			if q.Bounds.W <= 0 || q.Bounds.H <= 0 {
				t.Errorf("background quad is empty: %+v", q.Bounds)
			}
		}
	}
	if bg != 1 {
		t.Errorf("background quads = %d, want 1 behind ef", bg)
	}
	if got := countQuads(sc, textColor, 1); got != 1 {
		t.Errorf("underline quads = %d, want 1 under gh", got)
	}
}

func TestZeroColorInheritsParentTextColor(t *testing.T) {
	green := RGB(0, 255, 0)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().TextColor(green).Child(Rich(Seg("ab"), Seg("cd").Color(RGB(255, 0, 0))).Key("r"))
	}, 400, 300, 1)
	sc := paintNow(rt)
	if got := countGlyphs(sc, green.scene()); got != 2 {
		t.Errorf("glyphs in the inherited color = %d, want 2", got)
	}
}

func TestLinkUsesThemeLinkColorAndFollowsSetTheme(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Link("open", func() {})).Key("r"))
	}, 400, 300, 1)
	if got := countGlyphs(paintNow(rt), DarkTheme().Link.scene()); got != 4 {
		t.Fatalf("accent glyphs = %d, want 4 in the dark theme", got)
	}
	rt.SetTheme(LightTheme())
	if got := countGlyphs(paintNow(rt), LightTheme().Link.scene()); got != 4 {
		t.Errorf("accent glyphs after SetTheme = %d, want 4 in the light theme", got)
	}
}

func TestLinkClickRunsOnlyTheLink(t *testing.T) {
	links, rows := 0, 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Key("row").Padding(10).OnClick(func() { rows++ }).
			Child(Rich(Seg("Open "), Link("project", func() { links++ })).Key("r"))
	}, 400, 300, 1)
	paintNow(rt)
	x, y := caretPoint(t, rt, "r", 6) // inside "project"
	rt.PointerMove(x, y, 0)
	if rt.hostCursor() != input.CursorPointer {
		t.Errorf("cursor over the link = %v, want pointer", rt.hostCursor())
	}
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	if links != 1 || rows != 0 {
		t.Errorf("clicking the link: links=%d rows=%d; want the link only", links, rows)
	}
	x, y = caretPoint(t, rt, "r", 1) // inside "Open"
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	if links != 1 || rows != 1 {
		t.Errorf("clicking plain text: links=%d rows=%d; want the row to get the click", links, rows)
	}
}

func TestCursorIsPointerOnlyOverTheLink(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(Rich(Seg("Open "), Link("project", func() {})).Key("r"))
	}, 400, 300, 1)
	paintNow(rt)
	x, y := caretPoint(t, rt, "r", 6)
	rt.PointerMove(x, y, 0)
	if rt.hostCursor() != input.CursorPointer {
		t.Errorf("cursor over the link = %v, want pointer", rt.hostCursor())
	}
	x, y = caretPoint(t, rt, "r", 1)
	rt.PointerMove(x, y, 0)
	if rt.hostCursor() != input.CursorDefault {
		t.Errorf("cursor over plain text = %v, want default", rt.hostCursor())
	}
}

func TestLinkUnderlineShowsOnlyWhileHoveredAndDoesNotReshape(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(Rich(Seg("see "), Link("this", func() {})).Key("r"))
	}, 400, 300, 1)
	accent := DarkTheme().Link.scene()
	before := paintNow(rt)
	if got := countQuads(before, accent, 1); got != 0 {
		t.Fatalf("underline before hover = %d quads, want 0", got)
	}
	lay := richNode(t, rt, "r").rich.lay
	x, y := caretPoint(t, rt, "r", 5)
	rt.PointerMove(x, y, 0)
	hovered := paintNow(rt)
	if got := countQuads(hovered, accent, 1); got != 1 {
		t.Errorf("underline while hovered = %d quads, want 1", got)
	}
	if richNode(t, rt, "r").rich.lay != lay {
		t.Error("hovering re-shaped the text: the layout is a different object")
	}
	rt.PointerMove(1, 1, 0)
	if got := countQuads(paintNow(rt), accent, 1); got != 0 {
		t.Errorf("underline after leaving = %d quads, want 0", got)
	}
}

func TestRichWrapsAcrossSegmentsAndTruncates(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(
			Seg("alpha beta "),
			Link("gamma delta epsilon", func() {}),
			Seg(" zeta eta theta iota kappa"),
		).Width(Px(120)).Key("wrap"))
	}, 400, 300, 1)
	paintNow(rt)
	n := richNode(t, rt, "wrap")
	if len(n.rich.lay.Lines) < 3 {
		t.Errorf("lines = %d, want the text to wrap across the segments", len(n.rich.lay.Lines))
	}
	if n.rich.lay.Width > 120 {
		t.Errorf("width = %v, want at most 120", n.rich.lay.Width)
	}
	rt2 := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Seg("alpha beta "), Link("gamma delta epsilon", func() {}), Seg(" zeta eta theta")).
			Width(Px(120)).MaxLines(2).Key("cut"))
	}, 400, 300, 1)
	paintNow(rt2)
	c := richNode(t, rt2, "cut")
	if len(c.rich.lay.Lines) != 2 || !c.rich.lay.Truncated {
		t.Errorf("MaxLines(2): lines=%d truncated=%v, want 2 and true", len(c.rich.lay.Lines), c.rich.lay.Truncated)
	}
}

func TestRichDumpShowsConcatenatedText(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Seg("ab"), Seg("cd").Bold()).Key("r"))
	}, 400, 300, 1)
	var b bytes.Buffer
	rt.Dump(&b, 0)
	if !strings.Contains(b.String(), `text "abcd"`) {
		t.Errorf("dump does not show the concatenated text:\n%s", b.String())
	}
	if _, ok := rt.Find("r"); !ok {
		t.Error("Find does not locate the rich text by key")
	}
}
