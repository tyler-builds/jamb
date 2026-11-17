//go:build js

package web

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/tyler-builds/jamb/internal/platform"
)

// On an iPhone or an iPad, a page that has the host script takes text input in
// an editable element (contenteditable, plain text only) that lies over the
// focused field, in place of the textarea, and lays the field's text out as
// the field does: built by the host script (jamb.js) from the field's runs
// (platform.TextContext.Runs), each piece of text in a span in the face the
// field draws it in, at the font's own advances with the spacing the field
// adds, the syntax that a field with marks hides in spans of no width, a tab in
// a box of its width, and lines that wrap at the field's width. So iOS's
// keyboard trackpad, which moves the caret by where the characters are on
// screen, finds under the finger the character the field draws there. The
// faces are the font files jamb holds, given to the page as FontFaces, with
// nothing fetched again.
//
// The element's text is the field's text around the caret, as a textarea's
// would be (see mirror.go): its spans hold nothing else, and the browser's
// edits of it reach the field as edits of its text. The host script splits it
// into the spans again after an edit, keeping the spans whose text is as it
// should be, so that the browser's own edits stand.

// areaFontDefault is the element's font family where it lies over no field.
const areaFontDefault = "system-ui, sans-serif"

// textFonts is what the page has of the faces of text fields.
type textFonts struct {
	added map[string]bool // the font files given to the page, by key
}

// addFont gives the page the font file data as the family key, once, from the
// bytes jamb holds, and places the element again when the page has loaded it:
// the text in it is laid out anew.
func (w *window) addFont(key string, data []byte) {
	tf := &w.text.fonts
	if tf.added[key] || len(data) == 0 {
		return
	}
	if tf.added == nil {
		tf.added = map[string]bool{}
	}
	tf.added[key] = true
	g := js.Global()
	ctor, fonts := g.Get("FontFace"), g.Get("document").Get("fonts")
	if ctor.Type() != js.TypeFunction || !fonts.Truthy() {
		return // the browser cannot be given fonts: the element's are its own
	}
	buf := g.Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	// A FontFace made from bytes reports a file it cannot read by rejecting
	// its load, never by throwing, which would end a TinyGo build.
	face := ctor.New(key, buf.Get("buffer"))
	fonts.Call("add", face)
	var loaded, failed js.Func
	release := func() {
		loaded.Release()
		failed.Release()
	}
	loaded = js.FuncOf(func(js.Value, []js.Value) any {
		release()
		Post(func() {
			w.text.placed = placement{}
			if w.text.rect != (platform.Rect{}) {
				w.placeTextArea()
			}
		})
		return nil
	})
	failed = js.FuncOf(func(js.Value, []js.Value) any {
		release()
		return nil
	})
	face.Call("load").Call("then", loaded, failed)
}

// layoutDesc describes the runs of the field's text from the byte lo to hi,
// the text the element holds, for the host script's layout (see jamb.js):
// a line with the number of faces, a line for each face with its font-family
// list and font-variation-settings, separated by a tab, and then a line for
// each run, clipped to the text, with its kind, its length in UTF-16 code
// units, its face (-1 for none), its spacing and its width, in CSS pixels at
// the scale s. It gives the page the faces it does not have yet.
func (w *window) layoutDesc(text string, runs []platform.TextRun, faces []platform.FontFace, lo, hi int, s float32) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(faces)))
	for _, f := range faces {
		w.addFont(f.Key, f.Data)
		w.addFont(f.NotdefKey, f.Notdef)
		b.WriteString("\n" + strconv.Quote(f.Key))
		if f.NotdefKey != "" {
			b.WriteString(", " + strconv.Quote(f.NotdefKey))
		}
		b.WriteString("\t")
		if f.Weight > 0 {
			b.WriteString(`"wght" ` + num(f.Weight))
		} else {
			b.WriteString("normal")
		}
	}
	for _, r := range runs {
		a, e := max(r.Start, lo), min(r.End, hi)
		if a >= e {
			continue
		}
		b.WriteString("\n" + strconv.Itoa(int(r.Kind)) + " " + strconv.Itoa(utf16Len(text[a:e])) + " " +
			strconv.Itoa(r.Face) + " " + num(r.Spacing/s) + " " + num(r.Width/s))
	}
	return b.String()
}

// num formats v with up to four decimals.
func num(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) }

// lineStart returns where the line of the field's text that the byte i is on
// starts, from the runs of the text, and the line's index: the element then
// holds whole lines, which it wraps as the field does.
func lineStart(runs []platform.TextRun, i int) (start, line int) {
	k := 0
	for k+1 < len(runs) && runs[k+1].Start <= i {
		k++
	}
	if len(runs) == 0 {
		return i, 0
	}
	line = runs[k].Line
	for k > 0 && runs[k-1].Line == line {
		k--
	}
	return min(runs[k].Start, i), line
}

// layoutArea has the host script lay the element's text out from the field's
// runs, when they changed. lo and hi are the bytes of the field's text that
// the element holds.
func (w *window) layoutArea(c platform.TextContext, lo, hi int) {
	f := w.text.field
	if !w.text.overlay || f.fontSize == 0 || len(c.Runs) == 0 {
		return
	}
	desc := w.layoutDesc(c.Text, c.Runs, c.Faces, lo, hi, w.scale)
	if desc != w.text.laidOut {
		w.text.laidOut = desc
		w.hostLayout(desc)
	}
	if c.Carets != nil && textLayoutDebug() {
		w.showCarets(c, lo, hi)
	}
}

// textLayoutDebug reports whether the page asked, with
// jambEnv.JAMB_TEXT_LAYOUT = "1", for the field's carets on the element, for
// a test that compares the element's layout with the field's.
var textLayoutDebug = func() bool { return Getenv("JAMB_TEXT_LAYOUT") == "1" }

// showCarets writes where the field puts its caret at each character boundary
// of the text the element holds to the element's data-jamb-carets attribute,
// for a test: "offset x y" for each, separated by commas, the offset in UTF-16
// code units of the element's text, and x and y, the middle of the caret's
// line, in CSS pixels from the element's top left corner.
func (w *window) showCarets(c platform.TextContext, lo, hi int) {
	f := w.text.field
	s := float64(w.scale)
	var b strings.Builder
	for i, k := range c.Carets(lo, hi) {
		if i > 0 {
			b.WriteString(",")
		}
		x := (float64(f.textX-float32(f.box.X)) + float64(k.X)) / s
		y := (float64(f.textY-float32(f.box.Y)) + (float64(k.Line)+0.5)*float64(f.lineHeight)) / s
		b.WriteString(strconv.Itoa(utf16Len(c.Text[lo:k.Offset])) + " " + strconv.FormatFloat(x, 'f', 3, 64) + " " +
			strconv.FormatFloat(y, 'f', 3, 64))
	}
	w.text.area.Call("setAttribute", "data-jamb-carets", b.String())
}

// clearArea empties the element, of the text an input method typed into a
// field that shows it no text.
func (w *window) clearArea() {
	if w.hosted {
		w.host.Call("clear")
		return
	}
	w.text.area.Set("value", "")
}
