//go:build js

package web

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
)

// overlayPage is a textPage on iOS, whose editable element lies over the
// field's text, with a style to read, and a canvas at 0, 0 of the page.
func overlayPage(t *testing.T, text string) *textPage {
	p := newTextPage(t, text)
	p.area.Set("style", js.Global().Get("Object").New())
	rect := js.FuncOf(func(js.Value, []js.Value) any { return map[string]any{"left": 0, "top": 0} })
	t.Cleanup(rect.Release)
	p.w.canvas.Set("getBoundingClientRect", rect)
	p.w.text.overlay = true
	return p
}

// fieldOf returns the context of the page's field, which shows its text in box,
// starting at textX, textY, with text of the size font and lines line apart,
// wrapping at wrap, laid out as runs, all in physical pixels.
func (p *textPage) fieldOf(box platform.Rect, textX, textY, font, line, wrap float32, runs []platform.TextRun,
	faces []platform.FontFace) platform.TextContext {
	c := p.app.context()
	c.Box, c.TextX, c.TextY, c.FontSize, c.LineHeight, c.WrapWidth = box, textX, textY, font, line, wrap
	c.Runs, c.Faces = runs, faces
	return c
}

// frameWith runs a frame of the app, which reports the field as c and its
// caret at caret.
func (p *textPage) frameWith(c platform.TextContext, caret platform.Rect) {
	runPosted()
	p.w.SetTextContext(c)
	p.w.SetTextInputRect(caret)
}

// plainRuns returns the runs of text laid out on one line, in face 0.
func plainRuns(text string) []platform.TextRun {
	return []platform.TextRun{{Start: 0, End: len(text), Face: 0, Width: 100}}
}

// On iOS the editable element lies over the field's text, laid out at the
// field's text size and line height, wrapping at its width, its text where the
// field has scrolled it. Elsewhere, and for a field that says nothing of its
// text, it is at the caret.
func TestElementOverField(t *testing.T) {
	p := overlayPage(t, "alpha beta\ngamma")
	p.w.scale = 2
	caret := platform.Rect{X: 100, Y: 40, W: 2, H: 36}
	c := p.fieldOf(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 22, 28, 36, 400, plainRuns(p.app.text), nil)
	p.frameWith(c, caret)
	want := placement{x: 20, y: 20, w: 200, h: 60, font: 14, line: 18, scrollX: 0, scrollY: 9, wrap: 200}
	if got := p.w.text.placed; got != want {
		t.Errorf("over the field, the element is placed %+v, want %+v", got, want)
	}

	// The caret moving, as typing moves it, leaves the element where it is.
	p.frameWith(c, platform.Rect{X: 120, Y: 40, W: 2, H: 36})
	if got := p.w.text.placed; got != want {
		t.Errorf("the caret's move placed the element at %+v", got)
	}

	// While the browser moves the element's caret, as iOS's keyboard trackpad
	// does, the field scrolling to the caret leaves the element's text where it
	// is; once the browser is done, the element follows the field.
	p.area.Call("setSelectionRange", 12, 12)
	p.w.areaSelect(12, 12, false)
	c.TextY = 4
	p.frameWith(c, platform.Rect{X: 60, Y: 76, W: 2, H: 36})
	if got := p.w.text.placed.scrollY; got != 9 {
		t.Errorf("the field scrolled while the browser moved the caret, and the element's text moved to %v", got)
	}
	if p.app.caret != 12 {
		t.Errorf("the browser's caret at 12 moved the field's to %d", p.app.caret)
	}
	p.w.text.browserSel = time.Now().Add(-time.Second)
	p.frameWith(c, platform.Rect{X: 60, Y: 76, W: 2, H: 36})
	if got := p.w.text.placed.scrollY; got != 18 {
		t.Errorf("after the browser's move the element's text is scrolled by %v, want the field's 18", got)
	}

	// A field that says nothing of where its text is, as a code editor: the
	// element is at the caret.
	p.frameWith(p.fieldOf(platform.Rect{}, 0, 0, 0, 0, 0, nil, nil), caret)
	if got := p.w.text.placed; got != (placement{x: 50, y: 20, w: 1, h: 18}) {
		t.Errorf("for a field that reports no text box, the element is placed %+v, want at the caret", got)
	}

	// Elsewhere than on iOS it is at the caret, whatever the field says.
	q := overlayPage(t, "abc")
	q.w.text.overlay = false
	q.frameWith(q.fieldOf(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 22, 28, 36, 400, plainRuns("abc"), nil), caret)
	if got := q.w.text.placed; got.font != 0 || q.w.text.laidOut != "" {
		t.Errorf("off iOS the textarea is placed %+v and laid out %q; want at the caret, and no layout", got, q.w.text.laidOut)
	}
}

// The element is laid out from the field's runs: each run clipped to the text
// the element holds, with its length in UTF-16 code units, its face, its
// spacing and width in CSS pixels; and each face as a font-family list with its
// missing-glyph font.
func TestElementLayoutDesc(t *testing.T) {
	p := overlayPage(t, "")
	text := "T😀o **b**\tz\nx"
	runs := []platform.TextRun{
		{Start: 0, End: 1, Face: 0, Width: 20, Spacing: -2},              // a kerned T
		{Start: 1, End: 7, Face: 0, Width: 60},                           // 😀o and a space
		{Start: 7, End: 9, Face: -1, Kind: platform.RunNone},             // hidden **
		{Start: 9, End: 10, Face: 1, Width: 18},                          // b, bold
		{Start: 10, End: 12, Face: -1, Kind: platform.RunNone},           // hidden **
		{Start: 12, End: 13, Face: -1, Kind: platform.RunTab, Width: 21}, // a tab
		{Start: 13, End: 14, Face: 0, Width: 16},                         // z
		{Start: 14, End: 15, Face: -1, Kind: platform.RunBreak},          // the line break
		{Start: 15, End: 16, Face: 0, Line: 1, Width: 16},                // x
	}
	faces := []platform.FontFace{{Key: "jamb-face-1", NotdefKey: "jamb-notdef-1"}, {Key: "jamb-face-2", Weight: 700}}
	got := p.w.layoutDesc(text, runs, faces, 0, len(text), 2)
	want := strings.Join([]string{"2",
		"\"jamb-face-1\", \"jamb-notdef-1\"\tnormal", "\"jamb-face-2\"\t\"wght\" 700",
		"0 1 0 -1 10", "0 4 0 0 30", "3 2 -1 0 0", "0 1 1 0 9", "3 2 -1 0 0", "1 1 -1 0 10.5", "0 1 0 0 8",
		"2 1 -1 0 0", "0 1 0 0 8"}, "\n")
	if got != want {
		t.Errorf("the layout of %q is\n%s\nwant\n%s", text, got, want)
	}
	// Clipped to the text from the bold b to the tab.
	if got := p.w.layoutDesc(text, runs, faces, 9, 13, 2); !strings.HasSuffix(got, "\"wght\" 700\n0 1 1 0 9\n3 2 -1 0 0\n1 1 -1 0 10.5") {
		t.Errorf("the layout of the bytes 9 to 13 is\n%s", got)
	}
}

// The element holds whole lines of the field's text, which it wraps as the
// field does: the start of the text it holds goes back to the start of its
// line, and the element's first line is that line.
func TestElementHoldsWholeLines(t *testing.T) {
	runs := []platform.TextRun{
		{Start: 0, End: 10, Line: 0}, {Start: 10, End: 11, Line: 0, Kind: platform.RunBreak},
		{Start: 11, End: 30, Line: 1}, {Start: 30, End: 50, Line: 2}, {Start: 50, End: 60, Line: 2},
	}
	for _, c := range []struct{ i, start, line int }{{0, 0, 0}, {5, 0, 0}, {11, 11, 1}, {29, 11, 1}, {55, 30, 2}, {60, 30, 2}} {
		if start, line := lineStart(runs, c.i); start != c.start || line != c.line {
			t.Errorf("lineStart(%d) = %d, %d; want %d, %d", c.i, start, line, c.start, c.line)
		}
	}

	// A long text: the element holds from the start of the line that the
	// window around the caret starts in.
	long := strings.Repeat("word ", 1000) // 5000 bytes
	p := overlayPage(t, long)
	var lines []platform.TextRun
	for i := 0; i < len(long); i += 50 {
		lines = append(lines, platform.TextRun{Start: i, End: i + 50, Line: i / 50})
	}
	c := p.fieldOf(platform.Rect{X: 0, Y: 0, W: 400, H: 100}, 0, -3000, 28, 36, 400, lines, nil)
	p.frameWith(c, platform.Rect{X: 10, Y: 10, W: 2, H: 36})
	m := p.w.text.mirror
	if m.base%50 != 0 || m.base == 0 || m.base+len(m.text) != len(long) {
		t.Errorf("the element holds the bytes %d to %d; want from the start of a line to the end", m.base, m.base+len(m.text))
	}
	if top := p.w.text.field.top; top != float32(m.base/50)*36 {
		t.Errorf("the element's first line is %v below the text's top, want line %d's %v", top, m.base/50, float32(m.base/50)*36)
	}
}

// The page is given each face, and its missing-glyph font, once, from the
// bytes jamb holds; when a face has loaded, the element is placed again.
func TestElementFonts(t *testing.T) {
	p := overlayPage(t, "abc")
	g := js.Global()
	g.Call("eval", `globalThis.FontFace = function (family, data) {
		this.family = family; this.size = data.byteLength; this.load = () => Promise.resolve(this);
	}`)
	t.Cleanup(func() { g.Delete("FontFace") })
	fonts := g.Call("eval", `({ list: [], add(f) { this.list.push(f) } })`)
	g.Get("document").Set("fonts", fonts)
	faces := []platform.FontFace{{Key: "jamb-face-7", Data: []byte{1, 2, 3}, NotdefKey: "jamb-notdef-7", Notdef: []byte{4}}}
	c := p.fieldOf(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 40, 28, 36, 400, plainRuns("abc"), faces)
	caret := platform.Rect{X: 100, Y: 40, W: 2, H: 36}
	p.frameWith(c, caret)
	list := fonts.Get("list")
	if n := list.Length(); n != 2 || list.Index(0).Get("family").String() != "jamb-face-7" ||
		list.Index(0).Get("size").Int() != 3 || list.Index(1).Get("family").String() != "jamb-notdef-7" {
		t.Fatalf("the page was given %d fonts, want the face and its missing-glyph font, with their bytes", n)
	}
	if !strings.Contains(p.w.text.laidOut, `"jamb-face-7", "jamb-notdef-7"`) {
		t.Errorf("the element's layout is %q, want its runs in the face", p.w.text.laidOut)
	}
	// The faces loading place the element again, as its text lays out anew.
	p.w.text.placed.x = -1
	time.Sleep(10 * time.Millisecond) // the faces' load promises
	runPosted()
	if p.w.text.placed.x != 40 {
		t.Errorf("after the faces loaded the element is at x %v, want placed again at 40", p.w.text.placed.x)
	}
	// Frames with the same faces give the page none again.
	p.frameWith(c, caret)
	p.frameWith(c, caret)
	if n := list.Length(); n != 2 {
		t.Errorf("the page was given %d fonts after more frames, want the two only", n)
	}
}

// The element goes over the field only while it holds the field's text: a
// masked field's caret has it, with no text.
func TestElementAtCaretForMaskedField(t *testing.T) {
	p := overlayPage(t, "secret")
	p.app.noText = true
	c := p.fieldOf(platform.Rect{X: 40, Y: 40, W: 400, H: 36}, 40, 40, 28, 36, 0, plainRuns("secret"), nil)
	c.HasText = false
	p.frameWith(c, platform.Rect{X: 100, Y: 40, W: 2, H: 36})
	if got := p.w.text.placed; got.font != 0 {
		t.Errorf("a masked field's element is placed %+v, want at the caret", got)
	}
}
