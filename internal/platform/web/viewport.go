//go:build js

package web

import (
	"strings"
	"syscall/js"
)

// A window that is the whole page covers the part of the page that shows and
// that is safe to draw in. On a phone that part changes: the keyboard covers
// the bottom of the page, without making the page smaller, and the browser may
// pan the page up to keep the caret in view; and a display with a notch, round
// corners or a home indicator has insets that only a background belongs in
// (the page's viewport-fit=cover lets the page reach under them). So the
// canvas follows the browser's visual viewport, which is what shows of the
// page, less the safe area's insets, and the app lays itself out in what is
// left: a text field at the bottom stays above the keyboard. The page's
// background shows in the insets, in the app's background color (see
// SetBackground).
//
// A canvas that the page holds itself (<canvas id="jamb">) is the page's to
// place.

// pageBox is a rectangle of the page, in CSS pixels.
type pageBox struct{ left, top, width, height float64 }

// insets are the widths of the safe area's insets, in CSS pixels.
type insets struct{ left, top, right, bottom float64 }

// fitBox returns where the canvas goes: what shows of a page of the size
// pageW by pageH, the box visible, that is within the page's safe area.
func fitBox(visible pageBox, pageW, pageH float64, safe insets) pageBox {
	left, top := max(visible.left, safe.left), max(visible.top, safe.top)
	right := min(visible.left+visible.width, pageW-safe.right)
	bottom := min(visible.top+visible.height, pageH-safe.bottom)
	return pageBox{left, top, max(right-left, 1), max(bottom-top, 1)}
}

// followPage places the window's own canvas, and places it again when the
// part of the page that shows changes.
func (w *window) followPage() {
	win, doc := js.Global(), js.Global().Get("document")
	// The safe area's insets are known to CSS only, as env() values: an
	// element that has them as its padding tells what they are.
	w.probe = doc.Call("createElement", "div")
	st := w.probe.Get("style")
	for _, kv := range [][2]string{
		{"position", "fixed"}, {"left", "0"}, {"top", "0"}, {"width", "0"}, {"height", "0"},
		{"visibility", "hidden"}, {"pointerEvents", "none"},
		{"paddingLeft", "env(safe-area-inset-left, 0px)"}, {"paddingTop", "env(safe-area-inset-top, 0px)"},
		{"paddingRight", "env(safe-area-inset-right, 0px)"}, {"paddingBottom", "env(safe-area-inset-bottom, 0px)"},
	} {
		st.Set(kv[0], kv[1])
	}
	doc.Get("body").Call("appendChild", w.probe)
	probe := w.probe
	w.removers = append(w.removers, func() { probe.Call("remove") })

	fit := func(js.Value) {
		w.fitPage()
		Post(w.resized)
	}
	if vv := win.Get("visualViewport"); vv.Truthy() {
		w.on(vv, "resize", fit)
		w.on(vv, "scroll", fit)
	}
	w.on(win, "resize", fit)
	w.fitPage()
}

// fitPage places the window's own canvas over the part of the page that shows
// and is safe to draw in (see fitBox).
func (w *window) fitPage() {
	win := js.Global()
	pageW, pageH := win.Get("innerWidth").Float(), win.Get("innerHeight").Float()
	visible := pageBox{0, 0, pageW, pageH}
	if vv := win.Get("visualViewport"); vv.Truthy() {
		visible = pageBox{vv.Get("offsetLeft").Float(), vv.Get("offsetTop").Float(),
			vv.Get("width").Float(), vv.Get("height").Float()}
	}
	var safe insets
	if cs := win.Call("getComputedStyle", w.probe); cs.Truthy() {
		safe = insets{cssPx(cs.Get("paddingLeft")), cssPx(cs.Get("paddingTop")),
			cssPx(cs.Get("paddingRight")), cssPx(cs.Get("paddingBottom"))}
	}
	b := fitBox(visible, pageW, pageH, safe)
	if b == w.box {
		return
	}
	w.box = b
	st := w.canvasStyle()
	st.Set("left", px(b.left))
	st.Set("top", px(b.top))
	st.Set("width", px(b.width))
	st.Set("height", px(b.height))
}

// cssPx returns the pixels of a computed length, such as "47px" or "20.5px".
func cssPx(v js.Value) float64 {
	if v.Type() != js.TypeString {
		return 0
	}
	// The length is digits with at most one point, which needs no parser of
	// floating-point numbers in the build.
	px, scale := 0.0, 0.0
	for _, c := range strings.TrimSuffix(v.String(), "px") {
		switch {
		case c >= '0' && c <= '9' && scale == 0:
			px = px*10 + float64(c-'0')
		case c >= '0' && c <= '9':
			px += float64(c-'0') * scale
			scale /= 10
		case c == '.' && scale == 0:
			scale = 0.1
		default:
			return 0
		}
	}
	return px
}

// SetBackground implements platform.BackgroundWindow: the page around a
// window that is the whole page, which shows in the safe area's insets and
// while the page loads or resizes, takes the app's background color, and so
// does the browser's own frame where it follows the page's theme-color, as a
// phone's status bar and address bar do.
func (w *window) SetBackground(r, g, b uint8) {
	if !w.owned || w.destroyed {
		return
	}
	const hex = "0123456789abcdef"
	c := string([]byte{'#', hex[r>>4], hex[r&15], hex[g>>4], hex[g&15], hex[b>>4], hex[b&15]})
	doc := js.Global().Get("document")
	for _, el := range []js.Value{doc.Get("documentElement"), doc.Get("body")} {
		if el.Truthy() {
			el.Get("style").Set("backgroundColor", c)
		}
	}
	meta := doc.Call("querySelector", `meta[name="theme-color"]:not([media])`)
	if !meta.Truthy() {
		meta = doc.Call("createElement", "meta")
		meta.Call("setAttribute", "name", "theme-color")
		doc.Get("head").Call("appendChild", meta)
	}
	meta.Call("setAttribute", "content", c)
}
