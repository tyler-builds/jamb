//go:build js

package web

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
)

// overlayPage is a textPage on iOS, whose textarea lies over the field's text,
// with a style to read, and a canvas at 0, 0 of the page.
func overlayPage(t *testing.T, text string) *textPage {
	p := newTextPage(t, text)
	p.area.Set("style", js.Global().Get("Object").New())
	rect := js.FuncOf(func(js.Value, []js.Value) any { return map[string]any{"left": 0, "top": 0} })
	t.Cleanup(rect.Release)
	p.w.canvas.Set("getBoundingClientRect", rect)
	p.w.text.overlay = true
	return p
}

// style returns the textarea's style property name.
func (p *textPage) style(name string) string { return p.area.Get("style").Get(name).String() }

// placed runs a frame of the app, whose field shows its text in box, starting
// at textX, textY, with text of the size font and lines line apart, and whose
// caret is at caret, all in physical pixels.
func (p *textPage) placed(box platform.Rect, textX, textY, font, line float32, caret platform.Rect) {
	runPosted()
	c := p.app.context()
	c.Box, c.TextX, c.TextY, c.FontSize, c.LineHeight = box, textX, textY, font, line
	p.w.SetTextContext(c)
	p.w.SetTextInputRect(caret)
}

// On iOS the textarea lies over the field's text, laid out as the field lays it
// out, and takes the touches there; its text is 16 px at least, scaled down to
// the field's. Elsewhere, and for a field that says nothing of its text, it is
// at the caret, and takes no touches.
func TestTextareaOverField(t *testing.T) {
	p := overlayPage(t, "alpha beta\ngamma")
	p.w.scale = 2
	caret := platform.Rect{X: 100, Y: 40, W: 2, H: 36}
	// The field's text is 28 px (14 CSS px), its lines 36 apart, scrolled up by
	// half a line.
	p.placed(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 22, 28, 36, caret)
	for name, want := range map[string]string{
		"left": "20.00px", "top": "20.00px", "width": "228.57px", "height": "68.57px", "fontSize": "16.00px",
		"lineHeight": "20.57px", "transform": "scale(0.8750)", "whiteSpace": "pre-wrap", "pointerEvents": "auto",
	} {
		if got := p.style(name); got != want {
			t.Errorf("over the field, the textarea's %s is %s, want %s", name, got, want)
		}
	}
	if top := p.area.Get("scrollTop").Float(); top < 10.28 || top > 10.29 {
		t.Errorf("the textarea is scrolled to %v, want the field's 9 CSS px at its scale, 10.29", top)
	}

	// The caret moving, as typing moves it, leaves the textarea where it is.
	p.area.Get("style").Set("left", "moved")
	p.placed(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 22, 28, 36, platform.Rect{X: 120, Y: 40, W: 2, H: 36})
	if got := p.style("left"); got != "moved" {
		t.Errorf("the caret's move placed the textarea again, at %s", got)
	}

	// While the browser moves the textarea's caret, as iOS's keyboard trackpad
	// does, the field scrolling to the caret leaves the textarea's scroll.
	p.area.Set("scrollTop", 3)
	p.area.Call("setSelectionRange", 12, 12)
	p.w.areaSelect(12, 12, false)
	p.placed(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 4, 28, 36, platform.Rect{X: 60, Y: 76, W: 2, H: 36})
	if top := p.area.Get("scrollTop").Float(); top != 3 {
		t.Errorf("the field scrolled while the browser moved the caret, and the textarea was scrolled to %v", top)
	}
	if p.app.caret != 12 {
		t.Errorf("the browser's caret at 12 moved the field's to %d", p.app.caret)
	}
	// Once the browser is done, the textarea follows the field's scroll.
	p.w.text.browserSel = time.Now().Add(-time.Second)
	p.placed(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 4, 28, 36, platform.Rect{X: 60, Y: 76, W: 2, H: 36})
	if top := p.area.Get("scrollTop").Float(); top < 20.57 || top > 20.58 {
		t.Errorf("after the browser's move the textarea is scrolled to %v, want the field's 18 CSS px, 20.57", top)
	}

	// A field that says nothing of where its text is, as a code editor: the
	// textarea is at the caret.
	p.placed(platform.Rect{}, 0, 0, 0, 0, caret)
	for name, want := range map[string]string{
		"left": "50.00px", "top": "20.00px", "width": "1.00px", "height": "18.00px", "fontSize": "16.00px",
		"transform": "none", "whiteSpace": "pre", "pointerEvents": "none",
	} {
		if got := p.style(name); got != want {
			t.Errorf("at the caret, the textarea's %s is %s, want %s", name, got, want)
		}
	}

	// Elsewhere than on iOS it is at the caret, whatever the field says.
	q := overlayPage(t, "abc")
	q.w.text.overlay = false
	q.placed(platform.Rect{X: 40, Y: 40, W: 400, H: 120}, 40, 22, 28, 36, caret)
	if got := q.style("width"); got != "2.00px" || q.style("pointerEvents") != "none" {
		t.Errorf("off iOS the textarea is %s wide, and takes pointer events %s; want at the caret, taking none",
			got, q.style("pointerEvents"))
	}
}

// The textarea goes over the field only while it holds the field's text: a
// masked field's caret has it, with no text.
func TestTextareaAtCaretForMaskedField(t *testing.T) {
	p := overlayPage(t, "secret")
	p.app.noText = true
	p.placed(platform.Rect{X: 40, Y: 40, W: 400, H: 36}, 40, 40, 28, 36, platform.Rect{X: 100, Y: 40, W: 2, H: 36})
	if got := p.style("pointerEvents"); got != "none" {
		t.Errorf("a masked field's textarea takes pointer events %s, want none: it is at the caret", got)
	}
}
