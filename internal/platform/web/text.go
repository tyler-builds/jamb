//go:build js

package web

import (
	"strconv"
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
// keyboard, an input method or a paste type. The textarea stays empty between
// events. The canvas has the page's focus otherwise.
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
}

// recentKey is how soon after a key press an edit counts as that press's,
// rather than one that a keyboard made without a press.
const recentKey = 100 * time.Millisecond

// textState is the window's text input.
type textState struct {
	area      js.Value      // the hidden textarea
	rect      platform.Rect // where text input is, in device pixels; zero for none
	focused   bool          // the textarea has the page's focus
	composing bool          // an input method composes in it
	moving    bool          // the focus is being moved within the window
	lastKey   input.Key     // the latest key press delivered, and when
	lastKeyAt time.Time
}

// makeTextArea makes the hidden textarea and listens to it.
func (w *window) makeTextArea() {
	doc := js.Global().Get("document")
	a := doc.Call("createElement", "textarea")
	for _, kv := range [][2]string{
		{"autocomplete", "off"}, {"autocorrect", "off"}, {"autocapitalize", "off"}, {"spellcheck", "false"},
		{"aria-hidden", "true"}, {"tabindex", "-1"},
	} {
		a.Call("setAttribute", kv[0], kv[1])
	}
	st := a.Get("style")
	for _, kv := range [][2]string{
		{"position", "fixed"}, {"left", "0"}, {"top", "0"}, {"width", "1px"}, {"height", "1px"},
		{"margin", "0"}, {"padding", "0"}, {"border", "0"}, {"outline", "none"}, {"resize", "none"},
		{"overflow", "hidden"}, {"opacity", "0"}, {"background", "transparent"}, {"color", "transparent"},
		{"caretColor", "transparent"}, {"pointerEvents", "none"}, {"whiteSpace", "pre"},
		// At 16 px or more, iOS does not zoom the page in when the textarea takes the focus.
		{"fontSize", "16px"},
	} {
		st.Set(kv[0], kv[1])
	}
	doc.Get("body").Call("appendChild", a)
	w.text.area = a
	w.removers = append(w.removers, func() { a.Call("remove") })

	w.on(a, "focus", func(e js.Value) { w.text.focused = true; w.focusChanged(e, true) })
	w.on(a, "blur", func(e js.Value) { w.text.focused = false; w.focusChanged(e, false) })
	// The keys and the text that is typed: the host script delivers them when
	// the page has it (see host.go).
	if !w.hostListenText(a) {
		w.listenKeys(a)
		w.on(a, "compositionstart", func(js.Value) { w.composition(compositionStart, "") })
		w.on(a, "compositionupdate", func(e js.Value) { w.composition(compositionUpdate, e.Get("data").String()) })
		w.on(a, "compositionend", func(e js.Value) {
			if w.composition(compositionEnd, e.Get("data").String()) {
				a.Set("value", "")
			}
		})
		w.on(a, "beforeinput", func(e js.Value) {
			if w.beforeInput(e.Get("inputType").String()) {
				e.Call("preventDefault")
			}
		})
		w.on(a, "input", func(e js.Value) {
			if !e.Get("isComposing").Bool() && w.textInput(a.Get("value").String()) {
				a.Set("value", "")
			}
		})
	}
	w.on(a, "paste", w.paste)
	// Copy and cut are the app's, as key presses (see key); the textarea has
	// nothing to copy.
	w.on(a, "copy", func(e js.Value) { e.Call("preventDefault") })
	w.on(a, "cut", func(e js.Value) { e.Call("preventDefault") })
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
	switch kind {
	case compositionStart:
		w.text.composing = true
	case compositionUpdate:
		Post(func() {
			w.emit(platform.Event{Kind: platform.EventComposition, Text: text, Cursor: len(text),
				ClauseStart: len(text), ClauseEnd: len(text)})
		})
	case compositionEnd:
		if !w.text.composing {
			return false // canceled (see CancelComposition)
		}
		w.text.composing = false
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

// beforeInput turns the edits that a phone's keyboard makes without a key
// press, deleting and new lines, into the key presses that the app knows, since
// the empty textarea has nothing to delete. Text goes on to the input event.
// inputType is the event's; it reports whether the edit is prevented.
func (w *window) beforeInput(inputType string) bool {
	if w.text.composing {
		return false
	}
	var k input.Key
	switch inputType {
	case "deleteContentBackward":
		k = input.KeyBackspace
	case "deleteContentForward":
		k = input.KeyDelete
	case "insertLineBreak", "insertParagraph":
		k = input.KeyEnter
	case "insertText", "insertReplacementText", "insertFromPaste", "insertFromDrop", "insertCompositionText":
		return false
	default:
		return true // formatting and the like, which the app's text does not have
	}
	if w.text.lastKey == k && time.Since(w.text.lastKeyAt) < recentKey {
		return true // the press itself was delivered
	}
	Post(func() {
		w.emit(platform.Event{Kind: platform.EventKeyDown, Key: k})
		w.emit(platform.Event{Kind: platform.EventKeyUp, Key: k})
	})
	return true
}

// paste delivers the page's paste event in a text field as Ctrl+V, with the
// event's text as the clipboard that the app reads.
func (w *window) paste(e js.Value) {
	e.Call("preventDefault")
	data := e.Get("clipboardData")
	if !data.Truthy() {
		return
	}
	s := data.Call("getData", "text/plain").String()
	Post(func() {
		w.clipText = s
		w.emit(platform.Event{Kind: platform.EventKeyDown, Key: input.KeyV, Mods: input.ModCtrl})
		w.emit(platform.Event{Kind: platform.EventKeyUp, Key: input.KeyV, Mods: input.ModCtrl})
	})
}

// SetTextInputRect implements platform.Window. The app calls it in each frame.
func (w *window) SetTextInputRect(r platform.Rect) {
	if r == w.text.rect {
		return
	}
	w.text.rect = r
	if r != (platform.Rect{}) {
		w.placeTextArea()
	}
	w.syncTextFocus()
}

// placeTextArea moves the textarea to the caret, where an input method puts
// its window. The caret moves with each character typed.
func (w *window) placeTextArea() {
	s := float64(w.scale)
	r := w.text.rect
	if w.hosted {
		w.host.Call("place", float64(r.X)/s, float64(r.Y)/s, max(float64(r.W)/s, 1), max(float64(r.H)/s, 1))
		return
	}
	cr := w.canvas.Call("getBoundingClientRect")
	st := w.text.area.Get("style")
	st.Set("left", px(cr.Get("left").Float()+float64(r.X)/s))
	st.Set("top", px(cr.Get("top").Float()+float64(r.Y)/s))
	st.Set("width", px(max(float64(r.W)/s, 1)))
	st.Set("height", px(max(float64(r.H)/s, 1)))
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
	a.Set("value", "")
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
		Post(fn)
		return false
	}
	done := make(chan struct{})
	Post(func() {
		defer close(done)
		fn()
	})
	<-done
	return true
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
