//go:build js

package web

import "syscall/js"

// The host script (scripts/web/jamb.js, which a page loads beside the app)
// delivers the display's frames, and the pointer, wheel and key events, by
// calling functions that the app exports, with numbers (see host_tinygo.go).
// It is there for TinyGo's collector, which collects when the scheduler is
// idle after 32 references to JavaScript values were made, and a collection
// takes time in proportion to the heap: half a millisecond in a small app, 40
// in one that holds 100 MB. A listener made with js.FuncOf makes at least
// four references each time it is called, and a frame callback two, so a
// pointer moving over the window collected every few frames. A call of an
// exported function makes none.
//
// A page without the script, and a build with Go, whose collector does not
// count references, use listeners of the window's own (see listen).

// hostVersion is the version of the calls between the window and the script:
// the script's functions and the app's exports.
const hostVersion = 1

// hostEntry is set while a function that the host script calls runs. Such a
// call runs in no goroutine, so it cannot wait for one (see runNow).
var hostEntry bool

// pageHost returns the page's host script, when the page loaded it and it
// found this build's exports, and whether there is one.
func pageHost() (js.Value, bool) {
	if !hostExports {
		return js.Undefined(), false
	}
	h := js.Global().Get("jambHost")
	if h.Type() != js.TypeObject || h.Get("version").Type() != js.TypeNumber ||
		h.Get("version").Int() != hostVersion || !h.Get("ready").Truthy() {
		js.Global().Get("console").Call("warn", "jamb: the page did not load this build's jamb.js, or did not call "+
			"jambHost.attach with the app's instance; frames and input take the slower way (see docs/web.md)")
		return js.Undefined(), false
	}
	return h, true
}

// hostListen has the host script deliver the canvas's pointer, wheel and key
// events, and reports whether it does.
func (w *window) hostListen() bool {
	if !w.hosted {
		return false
	}
	w.host.Call("listen", w.canvas, hostTextAddress(), hostTextSize)
	w.removers = append(w.removers, func() { w.host.Call("close") })
	return true
}

// hostListenText has the host script deliver the textarea's key events and
// the text typed in it, and place it (see placeTextArea), and reports whether
// it does.
func (w *window) hostListenText(area js.Value) bool {
	if !w.hosted {
		return false
	}
	w.host.Call("text", area)
	return true
}

// hostTextSize is the size of the buffer that the host script writes an
// event's strings to (see host_tinygo.go): a key's code and name, an input
// event's type, or text. It delivers text longer than the buffer in pieces.
const hostTextSize = 1024

// requestFrame asks the browser for the next display frame, which arrives in
// displayFrame.
func (w *window) requestFrame() {
	if w.hosted {
		w.host.Call("frame")
		return
	}
	js.Global().Call("requestAnimationFrame", w.rafFunc)
}

// displayFrame notes that the display is ready for a frame, at the time t of
// requestAnimationFrame, in milliseconds, and wakes the UI goroutine to draw it.
func (w *window) displayFrame(t float64) {
	w.rafTime = t
	w.rafAsked.Store(false)
	w.rafFired.Store(true)
	signal()
}
