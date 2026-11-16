//go:build js

package web

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/tyler-builds/jamb/internal/platform"
)

// The textarea that lies over a text field on iOS (see placeTextArea) lays its
// text out in the faces the field draws it with: the font files jamb holds,
// the Go fonts the page fetched and the fonts the app registered, given to the
// page as FontFaces, with nothing fetched again. So the browser puts each
// character where jamb draws it, and the keyboard's trackpad finds under the
// finger the character jamb shows there. Until a face has loaded in the page
// the textarea's text is in the next family of its list; when it has, the
// textarea is placed again.

// areaFontDefault is the textarea's font family when it lies over no field.
const areaFontDefault = "system-ui, sans-serif"

// textFonts is what the page has of the faces of text fields.
type textFonts struct {
	added map[string]bool // the faces given to the page, by key
	// of and family are the last faces areaFamily was asked for, and the
	// font-family list it made of them; wght is where on the weight axis the
	// first is drawn, or 0.
	of     []platform.FontFace
	family string
	wght   float64
}

// areaFamily returns the font-family list of the faces of a field, and where
// on its weight axis the first is drawn (0 for its default instance), and
// gives the page the faces it does not have yet. faces is the same slice while
// the field's font is (see platform.TextContext).
func (w *window) areaFamily(faces []platform.FontFace) (string, float64) {
	tf := &w.text.fonts
	if len(faces) == 0 {
		return areaFontDefault, 0
	}
	if len(tf.of) == len(faces) && &tf.of[0] == &faces[0] {
		return tf.family, tf.wght
	}
	var b strings.Builder
	for _, f := range faces {
		w.addFace(f)
		b.WriteString(strconv.Quote(f.Key))
		b.WriteString(", ")
	}
	// A character that none of the faces has: the browser draws it in a font
	// of its own, where jamb draws its missing-glyph box.
	b.WriteString(areaFontDefault)
	tf.of, tf.family, tf.wght = faces, b.String(), float64(faces[0].Weight)
	return tf.family, tf.wght
}

// addFace gives the page the face f, once, from the bytes jamb holds, and
// places the textarea again when the page has loaded it.
func (w *window) addFace(f platform.FontFace) {
	tf := &w.text.fonts
	if tf.added[f.Key] {
		return
	}
	if tf.added == nil {
		tf.added = map[string]bool{}
	}
	tf.added[f.Key] = true
	g := js.Global()
	ctor, fonts := g.Get("FontFace"), g.Get("document").Get("fonts")
	if ctor.Type() != js.TypeFunction || !fonts.Truthy() {
		return // the browser cannot be given fonts: the textarea's are its own
	}
	data := g.Get("Uint8Array").New(len(f.Data))
	js.CopyBytesToJS(data, f.Data)
	// A FontFace made from bytes reports a file it cannot read by rejecting
	// its load, never by throwing, which would end a TinyGo build.
	face := ctor.New(f.Key, data.Get("buffer"))
	fonts.Call("add", face)
	var loaded, failed js.Func
	release := func() {
		loaded.Release()
		failed.Release()
	}
	loaded = js.FuncOf(func(js.Value, []js.Value) any {
		release()
		Post(func() {
			w.text.placed = placement{} // the text is laid out again: so is its scroll
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

// setAreaFont gives the textarea the font of the placement p.
func (w *window) setAreaFont(st js.Value, p placement) {
	family, variations := areaFontDefault, "normal"
	if p.font > 0 {
		family = p.family
		if p.wght > 0 {
			variations = `"wght" ` + strconv.FormatFloat(p.wght, 'f', -1, 64)
		}
	}
	st.Set("fontFamily", family)
	st.Set("fontVariationSettings", variations)
}
