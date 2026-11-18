//go:build js

package web

import (
	"image"
	"strconv"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
)

// Text input goes through a hidden <textarea>. While a text field of the app
// has focus, the textarea has the page's focus, at the field's caret: a phone
// shows its keyboard for it, an input method composes in it and places its
// window by it, and its input events carry the text that keys, a phone's
// keyboard, an input method or a paste type. It holds the field's text around
// the caret, which a phone's keyboard reads to correct and suggest words (see
// mirror.go), or is empty between events for a field that shows its text to no
// input method. The canvas has the page's focus otherwise.
//
// On an iPhone or an iPad, in a page that has the host script, an editable
// element takes the textarea's place, and lies over the focused field's text,
// laid out as the field lays it out (see editable.go). Its text and its caret
// are transparent, and it takes the touches on it, which go to the app as
// those on the canvas do. iOS moves the caret by where the text is on screen:
// its keyboard's trackpad (a long press of the space bar) finds the position
// at the finger's point in the focused element's text, hit-tested as the page
// is, and a textarea that took no touches, a pixel wide at the caret, as
// elsewhere, left it nothing to find, so the selection went out of the
// textarea, which then reads its selection as 0. Elsewhere the textarea is at
// the field's caret, where an input method puts its window.
//
// A phone shows its keyboard only for a focus that a tap's own event makes. So
// a tap is handled while its event lasts (see runNow): the app takes it and
// draws a frame, which reports where text input is (SetTextInputRect), and the
// event then focuses the textarea.

// keyNames maps KeyboardEvent.key to jamb's keys, for the presses that have no
// code, as some of a phone keyboard's have not.
var keyNames = map[string]input.Key{
	"Backspace": input.KeyBackspace, "Delete": input.KeyDelete, "Enter": input.KeyEnter, "Tab": input.KeyTab,
	"Escape": input.KeyEscape, "ArrowLeft": input.KeyLeft, "ArrowRight": input.KeyRight, "ArrowUp": input.KeyUp,
	"ArrowDown": input.KeyDown, "Home": input.KeyHome, "End": input.KeyEnd, "PageUp": input.KeyPageUp,
	"PageDown": input.KeyPageDown,
	// The Pause key, which a browser on Windows names "Cancel" with Ctrl held.
	"Pause": input.KeyPause, "Cancel": input.KeyPause,
}

// recentKey is how soon after a key press an edit counts as that press's,
// rather than one that a keyboard made without a press.
const recentKey = 100 * time.Millisecond

// textState is the window's text input.
type textState struct {
	area      js.Value      // the hidden textarea, or the editable element (see editable.go)
	rect      platform.Rect // where text input is, in device pixels; zero for none
	focused   bool          // the textarea has the page's focus
	composing bool          // an input method composes in it
	moving    bool          // the focus is being moved within the window
	lastKey   input.Key     // the latest key press delivered, and when
	lastKeyAt time.Time

	attrs    textAttrs // the textarea's attributes for the focused field (see attrsOf)
	mirror   mirror    // what the textarea holds of the focused field
	compData string    // what the input method composes, as its last compositionupdate said

	// overlay is set where the editable element takes the textarea's place
	// and lies over the field's text (iOS, with the host script; see
	// editable.go). field is the focused field's geometry, from its
	// TextContext, placed where the element was last put, laidOut the runs it
	// was last laid out from (see layoutArea), and browserSel when the browser
	// last moved its selection itself (see areaSelect).
	overlay    bool
	field      fieldGeometry
	placed     placement
	laidOut    string
	browserSel time.Time
	fonts      textFonts // the faces of fields the page has (see editable.go)
}

// fieldGeometry is where the focused field shows its text, in physical
// pixels, as platform.TextContext says: zero when it says nothing.
type fieldGeometry struct {
	box                  platform.Rect
	textX, textY         float32
	fontSize, lineHeight float32
	wrap                 float32 // the width its lines wrap at, or 0
	top                  float32 // the top of the first line the element holds, from the top of the text
}

// placement is where the textarea, or the editable element, is, in CSS pixels
// relative to the canvas: at the caret, when font is 0, or over the field's
// text, whose text size is font, the distance between whose lines is line,
// which wraps its lines at the width wrap (0 for none), and which has scrolled
// its text by scrollX and scrollY; top is the top of the first line the
// element holds, from the top of the text.
type placement struct {
	x, y, w, h       float64
	font, line       float64
	scrollX, scrollY float64
	wrap, top        float64
}

// sameBox reports whether p and q put the element over the same box, with the
// same text, scrolled or not.
func (p placement) sameBox(q placement) bool {
	p.scrollX, p.scrollY, q.scrollX, q.scrollY = 0, 0, 0, 0
	return p == q
}

// browserSelSettle is how long after the browser moved the textarea's
// selection itself the textarea keeps its own scroll: iOS's keyboard trackpad
// moves the caret through the textarea's text where it is on screen, and the
// field scrolling to the caret it moved would move that text under the
// finger, which would move the caret again.
const browserSelSettle = 500 * time.Millisecond

// areaFontSize is the size of the text of the textarea, and of the editable
// element: at 16 px or more, iOS does not zoom the page in when it takes the
// focus. The editable element's text has the field's size, in an element
// inside it (see jamb.js).
const areaFontSize = 16

// makeTextArea makes the hidden textarea, or the editable element, and listens
// to it.
func (w *window) makeTextArea() {
	doc := js.Global().Get("document")
	w.text.overlay = w.hosted && iosPage()
	var a js.Value
	if w.text.overlay {
		a = doc.Call("createElement", "div")
		a.Call("setAttribute", "contenteditable", "plaintext-only")
		a.Call("setAttribute", "role", "textbox")
	} else {
		a = doc.Call("createElement", "textarea")
	}
	for _, kv := range [][2]string{{"autocomplete", "off"}, {"aria-hidden", "true"}, {"tabindex", "-1"}} {
		a.Call("setAttribute", kv[0], kv[1])
	}
	w.text.attrs = attrsOf(platform.TextContext{Purpose: platform.PurposeVerbatim})
	for i, name := range textAttrNames {
		a.Call("setAttribute", name, w.text.attrs[i])
	}
	st := a.Get("style")
	for _, kv := range [][2]string{
		{"position", "fixed"}, {"left", "0"}, {"top", "0"}, {"width", "1px"}, {"height", "1px"},
		{"margin", "0"}, {"padding", "0"}, {"border", "0"}, {"outline", "none"}, {"resize", "none"},
		{"overflow", "hidden"}, {"opacity", "0"}, {"background", "transparent"}, {"color", "transparent"},
		{"caretColor", "transparent"}, {"pointerEvents", "none"}, {"whiteSpace", "pre"},
		{"fontSize", px(areaFontSize)}, {"fontFamily", areaFontDefault},
		// The touches on the element, when it lies over the field, are the
		// app's (see listenAreaTouches).
		{"touchAction", "none"}, {"webkitTouchCallout", "none"}, {"webkitTapHighlightColor", "transparent"},
		// The editable element's text is laid out as jamb's shaper lays it out
		// (see editable.go): the font's own advances, unhinted, with its default
		// ligatures, nothing synthesized, spaced, enlarged or hyphenated; lines
		// broken at word boundaries, and inside a word too long for a line.
		// Kerning is off: jamb's runs carry the kerning go-text applies, which is
		// little of a font's (none of Inter's pairs of classes, which a browser
		// applies).
		{"textRendering", "geometricPrecision"}, {"fontKerning", "none"}, {"fontVariantLigatures", "normal"},
		{"fontFeatureSettings", "normal"}, {"fontSynthesis", "none"}, {"fontOpticalSizing", "none"},
		{"fontWeight", "normal"}, {"fontStyle", "normal"}, {"letterSpacing", "0"}, {"wordSpacing", "0"},
		{"textIndent", "0"}, {"textTransform", "none"}, {"webkitTextSizeAdjust", "100%"}, {"textSizeAdjust", "100%"},
		{"hyphens", "manual"}, {"wordBreak", "normal"}, {"overflowWrap", "break-word"}, {"lineBreak", "auto"},
	} {
		st.Set(kv[0], kv[1])
	}
	doc.Get("body").Call("appendChild", a)
	w.text.area = a
	w.removers = append(w.removers, func() { a.Call("remove") })

	w.listenFocus()
	// The keys and the text that is typed: the host script delivers them when
	// the page has it (see host.go).
	if !w.hostListenText(a) {
		w.listenKeys(a)
		w.on(a, "compositionstart", func(js.Value) { w.composition(compositionStart, "") })
		w.on(a, "compositionupdate", func(e js.Value) { w.composition(compositionUpdate, e.Get("data").String()) })
		// The textarea's text and selection after an edit.
		edited := func(ended bool) {
			if w.areaInput(a.Get("value").String(), a.Get("selectionStart").Int(), a.Get("selectionEnd").Int(),
				a.Get("selectionDirection").String() == "backward", ended) {
				a.Set("value", "")
			}
		}
		w.on(a, "compositionend", func(e js.Value) {
			if w.composition(compositionEnd, e.Get("data").String()) {
				a.Set("value", "")
			}
			edited(true)
		})
		w.on(a, "beforeinput", func(e js.Value) {
			if w.beforeInput(e.Get("inputType").String()) {
				e.Call("preventDefault")
			}
		})
		w.on(a, "input", func(js.Value) { edited(false) })
		doc := js.Global().Get("document")
		w.on(doc, "selectionchange", func(js.Value) {
			if doc.Get("activeElement").Equal(a) {
				w.areaSelect(a.Get("selectionStart").Int(), a.Get("selectionEnd").Int(),
					a.Get("selectionDirection").String() == "backward")
			}
		})
	}
	w.listenAreaTouches(a)
	w.on(a, "paste", w.paste)
	// Copy and cut are the app's, as key presses (see key); the textarea has
	// nothing to copy.
	w.on(a, "copy", func(e js.Value) { e.Call("preventDefault") })
	w.on(a, "cut", func(e js.Value) { e.Call("preventDefault") })
}

// listenAreaTouches makes a touch, a click or the wheel on the textarea, which
// lies over the field it mirrors on iOS, the app's, as on the canvas: the
// browser places no caret of its own, selects nothing and shows no magnifier,
// callout or menu for it.
func (w *window) listenAreaTouches(a js.Value) {
	prevent := func(e js.Value) {
		if e.Get("cancelable").Bool() {
			e.Call("preventDefault")
		}
	}
	w.on(a, "touchstart", prevent, map[string]any{"passive": false})
	w.on(a, "touchend", func(e js.Value) {
		prevent(e)
		w.syncTextFocus()
	})
	if !w.hosted {
		// The host script listens for the rest (see jamb.js).
		w.on(a, "pointerdown", w.pointerListener(pointerDown))
		w.on(a, "wheel", w.wheelListener, map[string]any{"passive": false})
		w.on(a, "mousedown", prevent)
		w.on(a, "contextmenu", prevent)
	}
}

// iosPage reports whether the page runs on an iPhone or an iPad, an iPad that
// asks for desktop pages included, where the keyboard's trackpad moves the
// caret by where the text is on screen (see placeTextArea).
func iosPage() bool {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return false
	}
	ua, plat := nav.Get("userAgent"), nav.Get("platform")
	if ua.Type() == js.TypeString {
		u := ua.String()
		if strings.Contains(u, "iPhone") || strings.Contains(u, "iPad") || strings.Contains(u, "iPod") {
			return true
		}
	}
	touch := nav.Get("maxTouchPoints")
	return plat.Type() == js.TypeString && plat.String() == "MacIntel" && touch.Type() == js.TypeNumber && touch.Int() > 1
}

// listenFocus listens for the page's focus coming to and leaving the canvas
// and the textarea. The focus moves from the canvas to the textarea when a text
// field of the app takes it, and back when it gives it up: the window keeps the
// focus then (see focusChanged). Were that a blur, the app would take the
// window as inactive, and hide the text field's caret.
func (w *window) listenFocus() {
	c, a := w.canvas, w.text.area
	w.on(c, "focus", func(e js.Value) { w.focusChanged(e, true) })
	w.on(c, "blur", func(e js.Value) { w.focusChanged(e, false) })
	w.on(a, "focus", func(e js.Value) { w.text.focused = true; w.focusChanged(e, true) })
	w.on(a, "blur", func(e js.Value) { w.text.focused = false; w.focusChanged(e, false) })
}

// focusChanged tells the app when the page's focus comes to or leaves the
// window, the canvas and the textarea together.
func (w *window) focusChanged(e js.Value, focus bool) {
	other := e.Get("relatedTarget")
	if w.text.moving || other.Equal(w.canvas) || other.Equal(w.text.area) {
		return // the focus moves within the window
	}
	k := platform.EventBlur
	if focus {
		k = platform.EventFocus
	}
	Post(func() { w.emit(platform.Event{Kind: k}) })
}

// compositionKind is a step of an input method's composition.
type compositionKind int

const (
	compositionStart compositionKind = iota
	compositionUpdate
	compositionEnd
)

// composition handles a step of an input method's composition in the
// textarea: text is what is composed so far, or at the end what it commits. It
// reports, at the end, whether the textarea is to be emptied.
func (w *window) composition(kind compositionKind, text string) bool {
	mirrored := w.text.mirror.on
	switch kind {
	case compositionStart:
		w.text.composing = true
		w.text.compData = ""
	case compositionUpdate:
		if mirrored {
			w.text.compData = text // shown with the textarea's edit that follows (see syncText)
			return false
		}
		Post(func() {
			w.emit(platform.Event{Kind: platform.EventComposition, Text: text, Cursor: len(text),
				ClauseStart: len(text), ClauseEnd: len(text)})
		})
	case compositionEnd:
		if !w.text.composing {
			return false // canceled (see CancelComposition)
		}
		w.text.composing = false
		if mirrored {
			// The composition ends, and the textarea's text, with what it
			// committed, follows (see areaInput).
			w.text.compData = ""
			Post(func() { w.emit(platform.Event{Kind: platform.EventComposition}) })
			return false
		}
		Post(func() {
			w.emit(platform.Event{Kind: platform.EventComposition})
			if text != "" {
				w.emit(platform.Event{Kind: platform.EventTextInput, Text: text})
			}
		})
		return true
	}
	return false
}

// areaInput handles the textarea's text after an edit: an input event, or the
// end of a composition (ended), with its selection from start to end, in UTF-16
// code units, whose anchor is end when backward. It reports whether the
// textarea is to be emptied.
func (w *window) areaInput(value string, start, end int, backward, ended bool) bool {
	m := &w.text.mirror
	switch {
	case !m.on && ended:
		return false // the composition's end delivered its text
	case !m.on:
		return w.textInput(value)
	case !m.valid:
		return false // the app's text is written to the textarea next (see CancelComposition)
	}
	a, b := byteOffset(value, start), byteOffset(value, end)
	if backward {
		a, b = b, a
	}
	w.syncText(value, a, b)
	return false
}

// syncText sends the app what changed in the mirroring textarea, whose text is
// now s, with the selection from anchor to caret, in bytes: its edit of the
// field's text, the selection, and while an input method composes, the
// composition. The composition is not in the field's text: the field shows
// it at its caret, and it is typed when it ends.
func (w *window) syncText(s string, anchor, caret int) {
	m := &w.text.mirror
	f, comp := s, ""
	if w.text.composing {
		cs, ce := compositionIn(s, m.text, w.text.compData, max(anchor, caret))
		f, comp = s[:cs]+s[ce:], s[cs:ce]
		anchor, caret = cs, cs
	}
	var events []platform.Event
	if a, b, ins, same := diffText(m.text, f, caret); !same {
		events = append(events, platform.Event{Kind: platform.EventTextEdit, Start: m.base + a, End: m.base + b, Text: ins})
		m.text = f
		m.anchor, m.caret = a+len(ins), a+len(ins)
	}
	if anchor != m.anchor || caret != m.caret {
		events = append(events, platform.Event{Kind: platform.EventTextSelect, Start: m.base + anchor, End: m.base + caret})
		m.anchor, m.caret = anchor, caret
	}
	if w.text.composing {
		events = append(events, platform.Event{Kind: platform.EventComposition, Text: comp, Cursor: len(comp),
			ClauseStart: len(comp), ClauseEnd: len(comp)})
	}
	if len(events) > 0 {
		Post(func() {
			for _, e := range events {
				w.emit(e)
			}
		})
	}
}

// areaSelect handles a change of the mirroring textarea's selection that no
// edit made, as a phone keyboard's cursor movement does: the selection from
// start to end, in UTF-16 code units, whose anchor is end when backward.
func (w *window) areaSelect(start, end int, backward bool) {
	m := &w.text.mirror
	if !m.on || !m.valid || w.text.composing {
		return
	}
	a, b := byteOffset(m.text, start), byteOffset(m.text, end)
	if backward {
		a, b = b, a
	}
	if a == m.anchor && b == m.caret {
		return // as the window wrote it, or as an edit left it
	}
	m.anchor, m.caret = a, b
	w.text.browserSel = time.Now()
	base := m.base
	Post(func() { w.emit(platform.Event{Kind: platform.EventTextSelect, Start: base + a, End: base + b}) })
}

// SetTextContext implements platform.TextContextWindow. The app calls it in
// each frame, before SetTextInputRect. It gives the textarea the attributes of
// the focused field, and writes the field's text and selection to it when they
// are not what it holds.
func (w *window) SetTextContext(c platform.TextContext) {
	if c.Focused {
		w.setTextAttrs(attrsOf(c))
	}
	// The editable element lies over a field whose text it holds (see
	// placeTextArea and editable.go).
	top := w.text.field.top
	w.text.field = fieldGeometry{}
	if c.HasText && c.Box.W > 0 && c.Box.H > 0 && c.FontSize > 0 {
		w.text.field = fieldGeometry{box: c.Box, textX: c.TextX, textY: c.TextY, fontSize: c.FontSize,
			lineHeight: c.LineHeight, wrap: c.WrapWidth, top: top}
	}
	m := &w.text.mirror
	if w.text.composing {
		return // the input method's text is the textarea's until it ends
	}
	if !c.HasText {
		if m.on {
			*m = mirror{}
			w.writeArea("", 0, 0, true)
			w.text.laidOut = ""
		}
		return
	}
	anchor, caret := runeStart(c.Text, c.Anchor), runeStart(c.Text, c.Caret)
	lo, hi := m.window(c.Text, anchor, caret)
	if w.text.overlay && len(c.Runs) > 0 {
		// The element holds whole lines, which it wraps as the field does.
		var line int
		lo, line = lineStart(c.Runs, lo)
		w.text.field.top = float32(line) * c.LineHeight
	}
	defer func() { w.layoutArea(c, lo, hi) }()
	text := c.Text[lo:hi]
	anchor, caret = min(max(anchor-lo, 0), len(text)), min(max(caret-lo, 0), len(text))
	same := m.on && m.valid && m.text == text
	if same && m.anchor == anchor && m.caret == caret {
		m.base = lo
		return
	}
	*m = mirror{on: true, valid: true, base: lo, text: text, anchor: anchor, caret: caret}
	if !same {
		w.text.laidOut = "" // the text written is laid out anew
	}
	w.writeArea(text, anchor, caret, !same)
}

// writeArea writes text, when value is set, and the selection from anchor to
// caret, in bytes, to the textarea.
func (w *window) writeArea(text string, anchor, caret int, value bool) {
	a, c := utf16Offset(text, anchor), utf16Offset(text, caret)
	start, end, backward := min(a, c), max(a, c), c < a
	if w.hosted {
		w.hostSetText(text, start, end, backward, value)
		return
	}
	area := w.text.area
	if value {
		area.Set("value", text)
	}
	dir := "forward"
	if backward {
		dir = "backward"
	}
	area.Call("setSelectionRange", start, end, dir)
}

// setTextAttrs gives the textarea the attributes at. A phone's keyboard reads
// them when the textarea takes the focus, so the textarea takes it again when
// they change while it has it, in a tap or a key press, the only time a page
// may focus an element and keep the keyboard shown.
func (w *window) setTextAttrs(at textAttrs) {
	if at == w.text.attrs {
		return
	}
	for i, name := range textAttrNames {
		if at[i] != w.text.attrs[i] {
			w.text.area.Call("setAttribute", name, at[i])
		}
	}
	w.text.attrs = at
	if w.text.focused && gesture && !w.text.composing {
		a := w.text.area
		w.text.moving = true
		a.Call("blur")
		a.Call("focus", map[string]any{"preventScroll": true})
		w.text.moving = false
	}
}

// textInput delivers the text that the textarea's input event typed, and
// reports whether the textarea is to be emptied: not while an input method
// composes in it.
func (w *window) textInput(text string) bool {
	if w.text.composing {
		return false
	}
	if text != "" {
		Post(func() { w.emit(platform.Event{Kind: platform.EventTextInput, Text: text}) })
	}
	return true
}

// beforeInput lets the textarea take the edits of a phone's keyboard and of an
// input method, and reports whether an edit is prevented; inputType is the
// event's. A textarea that mirrors the field takes its deletions too, which
// the input event then sends (see syncText). An empty one has nothing to
// delete: deleting a character, as new lines in either, become the key presses
// that the app knows. Undo and redo, as from shaking an iPhone, are the app's
// keys, and formatting is prevented, which the app's text does not have.
func (w *window) beforeInput(inputType string) bool {
	if w.text.composing {
		return false
	}
	mirrored := w.text.mirror.on && w.text.mirror.valid
	k, m := input.KeyUnknown, input.Modifiers(0)
	switch inputType {
	case "deleteContentBackward":
		k = input.KeyBackspace
	case "deleteContentForward":
		k = input.KeyDelete
	case "insertLineBreak", "insertParagraph":
		k = input.KeyEnter
		mirrored = false
	case "historyUndo":
		k, m, mirrored = input.KeyZ, input.ModCtrl, false
	case "historyRedo":
		k, m, mirrored = input.KeyY, input.ModCtrl, false
	case "insertText", "insertReplacementText", "insertFromPaste", "insertFromDrop", "insertCompositionText",
		"insertFromYank", "insertTranspose":
		return false
	default:
		// Formatting, and the deletions of words and lines that only a textarea
		// with the field's text can make.
		return !mirrored || len(inputType) < 6 || inputType[:6] != "delete"
	}
	if mirrored {
		return false
	}
	if m == 0 && w.text.lastKey == k && time.Since(w.text.lastKeyAt) < recentKey {
		return true // the press itself was delivered
	}
	Post(func() {
		w.emit(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: m})
		w.emit(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: m})
	})
	return true
}

// paste delivers the page's paste event in a text field as Ctrl+V, with the
// event's text as the clipboard that the app reads, or, when it has no text,
// its image: a screenshot, or an image file that was copied. The browser
// decodes the image first, so its paste arrives a moment later.
func (w *window) paste(e js.Value) {
	e.Call("preventDefault")
	data := e.Get("clipboardData")
	if !data.Truthy() {
		return
	}
	s := data.Call("getData", "text/plain").String()
	if file, ok := pastedImage(data); ok && s == "" {
		decodeImage(file, func(img image.Image) {
			if img != nil {
				Post(func() { w.deliverPaste("", img) })
			}
		})
		return
	}
	Post(func() { w.deliverPaste(s, nil) })
}

// deliverPaste makes text and img the clipboard that the app reads, and
// presses Ctrl+V. UI goroutine only.
func (w *window) deliverPaste(text string, img image.Image) {
	w.clipText, w.clipImage = text, img
	w.emit(platform.Event{Kind: platform.EventKeyDown, Key: input.KeyV, Mods: input.ModCtrl})
	w.emit(platform.Event{Kind: platform.EventKeyUp, Key: input.KeyV, Mods: input.ModCtrl})
}

// SetTextInputRect implements platform.Window. The app calls it in each frame.
func (w *window) SetTextInputRect(r platform.Rect) {
	changed := r != w.text.rect
	w.text.rect = r
	if r != (platform.Rect{}) {
		w.placeTextArea()
	}
	if changed {
		w.syncTextFocus()
	}
}

// textPlacement returns where the textarea, or the editable element, goes:
// over the focused field's text, on iOS, when the field says where that is,
// and at the caret otherwise.
func (w *window) textPlacement() placement {
	s := float64(w.scale)
	if f := w.text.field; w.text.overlay && f.fontSize > 0 {
		b := f.box
		return placement{x: float64(b.X) / s, y: float64(b.Y) / s, w: float64(b.W) / s, h: float64(b.H) / s,
			font: float64(f.fontSize) / s, line: float64(f.lineHeight) / s,
			scrollX: (float64(b.X) - float64(f.textX)) / s, scrollY: (float64(b.Y) - float64(f.textY)) / s,
			wrap: float64(f.wrap) / s, top: float64(f.top) / s}
	}
	r := w.text.rect
	return placement{x: float64(r.X) / s, y: float64(r.Y) / s, w: max(float64(r.W)/s, 1), h: max(float64(r.H)/s, 1)}
}

// placeTextArea moves the textarea, or the editable element, to where
// textPlacement puts it, when that changed: to the caret, which moves with
// each character typed, or over the field's text. While the browser moves the
// element's caret itself, as iOS's keyboard trackpad does, the element keeps
// its text where it is (see browserSelSettle).
func (w *window) placeTextArea() {
	p := w.textPlacement()
	old := w.text.placed
	if p == old {
		return
	}
	if settled := w.text.browserSel.Add(browserSelSettle); p.font > 0 && p.sameBox(old) && time.Now().Before(settled) {
		w.WakeAt(settled) // a frame then places the element where the field's text is
		return
	}
	w.text.placed = p
	if w.hosted {
		w.host.Call("place", p.x, p.y, p.w, p.h, p.font, p.line, p.scrollX, p.scrollY, p.wrap, p.top)
		return
	}
	// A page without the host script has only the textarea, at the caret.
	cr := w.canvas.Call("getBoundingClientRect")
	st := w.text.area.Get("style")
	st.Set("left", px(cr.Get("left").Float()+p.x))
	st.Set("top", px(cr.Get("top").Float()+p.y))
	st.Set("width", px(p.w))
	st.Set("height", px(p.h))
}

// syncTextFocus gives the textarea the page's focus while there is text input,
// and the canvas the focus otherwise, unless the window has lost the focus.
func (w *window) syncTextFocus() {
	switch input := w.text.rect != (platform.Rect{}); {
	case input && !w.text.focused:
		w.text.area.Call("focus", map[string]any{"preventScroll": true})
	case !input && w.text.focused:
		active := js.Global().Get("document").Get("activeElement")
		if active.Equal(w.canvas) || active.Equal(w.text.area) {
			w.canvas.Call("focus", map[string]any{"preventScroll": true})
		}
	}
}

// CancelComposition implements platform.Window: the textarea gives up the
// focus and takes it back, which ends the input method's composition.
func (w *window) CancelComposition() {
	if !w.text.composing {
		return
	}
	w.text.composing = false // the compositionend that follows types nothing
	a := w.text.area
	w.text.moving = true
	a.Call("blur")
	if w.text.mirror.on {
		// The textarea keeps the composition as text. The app types it, and its
		// text is written to the textarea in its next frame.
		w.text.mirror.valid = false
	} else {
		w.clearArea()
	}
	a.Call("focus", map[string]any{"preventScroll": true})
	w.text.moving = false
}

// runNow runs fn on the UI goroutine while the browser event that the caller
// handles lasts, so that the browser lets the app do what it allows only in an
// event: a tap that focuses a text field shows the keyboard, and a copy
// reaches the system's clipboard. A listener of the window's own waits for fn.
// A goroutine runs only while the others wait, so the UI goroutine is waiting
// for work (uiActive is clear), or it waits for the browser, for which this
// event would wait in turn: then fn is only posted. An event that the host
// script delivers cannot wait (see hostEntry): fn is posted, and the script
// runs the UI goroutine before the event ends. It reports whether fn ran.
func runNow(fn func()) bool {
	if uiActive.Load() || hostEntry {
		Post(func() { inGesture(fn) })
		return false
	}
	done := make(chan struct{})
	Post(func() {
		defer close(done)
		inGesture(fn)
	})
	<-done
	return true
}

// gesture is set while the app handles a click, a tap or a key press of the
// user's, for which a browser lets a page ask for the system's clipboard (see
// Clipboard). UI goroutine only.
var gesture bool

// inGesture runs fn as the app's handling of a click, a tap or a key press.
func inGesture(fn func()) {
	gesture = true
	fn()
	gesture = false
}

// drawNow draws a frame at once, outside the display's pacing, so that the
// app reports where text input is before a tap's event ends. UI goroutine only.
func (w *window) drawNow() {
	if !w.ready || w.destroyed || !w.Visible() {
		return
	}
	w.wakePending.Store(false)
	w.frameWanted = w.handler.OnFrame(w)
}

// clipboardKey reports whether a press is copy, cut or paste.
func clipboardKey(k input.Key, m input.Modifiers) bool {
	if m&input.ModCtrl == 0 || m&(input.ModAlt|input.ModShift) != 0 {
		return false
	}
	return k == input.KeyC || k == input.KeyX || k == input.KeyV
}

// printable reports whether a press types the character that it names.
func printable(key string, m input.Modifiers) bool {
	return m&(input.ModCtrl|input.ModSuper) == 0 && utf8.RuneCountInString(key) == 1
}

func px(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) + "px" }
