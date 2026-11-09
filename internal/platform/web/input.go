//go:build js

package web

import (
	"math"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
)

// The limits of a double click, as the other adapters count them.
const (
	doubleClickTime = 500 * time.Millisecond
	doubleClickSlop = 4 // device pixels
)

// touchSlop is how far, in CSS pixels, a finger moves before its touch is a
// pan rather than a tap.
const touchSlop = 8

// pointerState is what the window remembers of the pointer between events.
type pointerState struct {
	lastButton input.MouseButton
	lastPress  time.Time
	lastX      float32
	lastY      float32
	lastClicks int

	// The finger of a touch: where it went down and was last, and whether it
	// moved far enough to pan.
	touchID           int
	touching, panning bool
	touchX0, touchY0  float64
	touchX, touchY    float64
}

// on adds a listener for the event type on target, and keeps it to remove when
// the window closes. Each listener only reads the event and posts the work to
// the UI goroutine.
func (w *window) on(target js.Value, typ string, fn func(e js.Value), opts ...map[string]any) {
	f := js.FuncOf(func(_ js.Value, args []js.Value) any {
		fn(args[0])
		return nil
	})
	w.funcs = append(w.funcs, f)
	if len(opts) > 0 {
		target.Call("addEventListener", typ, f, opts[0])
	} else {
		target.Call("addEventListener", typ, f)
	}
	w.removers = append(w.removers, func() { target.Call("removeEventListener", typ, f) })
}

func (w *window) listen() {
	c, win, doc := w.canvas, js.Global(), js.Global().Get("document")

	w.on(c, "pointerdown", w.pointerDown)
	w.on(c, "pointermove", w.pointerMove)
	w.on(c, "pointerup", w.pointerUp)
	w.on(c, "pointercancel", w.pointerUp)
	w.on(c, "pointerleave", func(e js.Value) {
		if e.Get("pointerType").String() != "touch" {
			Post(func() { w.emit(platform.Event{Kind: platform.EventMouseLeave}) })
		}
	})
	w.on(c, "contextmenu", func(e js.Value) { e.Call("preventDefault") })
	// The wheel listener is not passive, so that it can keep the page from
	// scrolling and from zooming on Ctrl+wheel.
	w.on(c, "wheel", w.wheel, map[string]any{"passive": false})
	w.on(c, "keydown", func(e js.Value) { w.key(e, true) })
	w.on(c, "keyup", func(e js.Value) { w.key(e, false) })
	w.on(c, "focus", func(js.Value) { Post(func() { w.emit(platform.Event{Kind: platform.EventFocus}) }) })
	w.on(c, "blur", func(js.Value) { Post(func() { w.emit(platform.Event{Kind: platform.EventBlur}) }) })
	w.on(doc, "visibilitychange", func(js.Value) { Post(w.Wake) })

	if ro := win.Get("ResizeObserver"); ro.Truthy() {
		f := js.FuncOf(func(js.Value, []js.Value) any { Post(w.resized); return nil })
		w.funcs = append(w.funcs, f)
		obs := ro.New(f)
		obs.Call("observe", c)
		w.removers = append(w.removers, func() { obs.Call("disconnect") })
	} else {
		w.on(win, "resize", func(js.Value) { Post(w.resized) })
	}
	w.watchScale()
}

// watchScale listens for the next change of the device pixel ratio, which a
// zoom or a move to another display makes: a media query for the current ratio
// stops matching.
func (w *window) watchScale() {
	if js.Global().Get("matchMedia").IsUndefined() {
		return
	}
	q := js.Global().Call("matchMedia", "(resolution: "+js.Global().Get("devicePixelRatio").String()+"dppx)")
	var f js.Func
	f = js.FuncOf(func(js.Value, []js.Value) any {
		q.Call("removeEventListener", "change", f)
		Post(func() {
			if !w.destroyed {
				w.resized()
				w.watchScale()
			}
		})
		return nil
	})
	w.funcs = append(w.funcs, f)
	q.Call("addEventListener", "change", f)
	w.removers = append(w.removers, func() { q.Call("removeEventListener", "change", f) })
}

func (w *window) unlisten() {
	for _, r := range w.removers {
		r()
	}
	for _, f := range w.funcs {
		f.Release()
	}
	w.removers, w.funcs = nil, nil
}

// pos returns the event's position in device pixels relative to the canvas.
func (w *window) pos(e js.Value) (float32, float32) {
	return float32(e.Get("offsetX").Float()) * w.scale, float32(e.Get("offsetY").Float()) * w.scale
}

func mods(e js.Value) input.Modifiers {
	var m input.Modifiers
	if e.Get("shiftKey").Bool() {
		m |= input.ModShift
	}
	if e.Get("ctrlKey").Bool() {
		m |= input.ModCtrl
	}
	if e.Get("altKey").Bool() {
		m |= input.ModAlt
	}
	if e.Get("metaKey").Bool() {
		m |= input.ModSuper
	}
	return m
}

func button(e js.Value) (input.MouseButton, bool) {
	switch e.Get("button").Int() {
	case 0:
		return input.ButtonLeft, true
	case 1:
		return input.ButtonMiddle, true
	case 2:
		return input.ButtonRight, true
	}
	return 0, false
}

func (w *window) pointerDown(e js.Value) {
	if e.Get("pointerType").String() == "touch" {
		w.touchDown(e)
		return
	}
	b, ok := button(e)
	if !ok {
		return
	}
	e.Call("preventDefault") // keeps the focus on the canvas, and no text is selected
	w.canvas.Call("focus")
	w.canvas.Call("setPointerCapture", e.Get("pointerId"))
	x, y := w.pos(e)
	m := mods(e)
	Post(func() {
		clicks := w.countClicks(b, x, y)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y, Mods: m})
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: b, Clicks: clicks, Mods: m})
	})
}

func (w *window) pointerMove(e js.Value) {
	if e.Get("pointerType").String() == "touch" {
		w.touchMove(e)
		return
	}
	x, y := w.pos(e)
	m := mods(e)
	Post(func() { w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y, Mods: m}) })
}

func (w *window) pointerUp(e js.Value) {
	if e.Get("pointerType").String() == "touch" {
		w.touchUp(e)
		return
	}
	b, ok := button(e)
	if !ok {
		return
	}
	x, y := w.pos(e)
	m := mods(e)
	Post(func() { w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: b, Mods: m}) })
}

// countClicks returns 2 for the second press of a double click, else 1, as
// the other adapters do: a third press is a single click again.
func (w *window) countClicks(b input.MouseButton, x, y float32) int {
	p := &w.in
	now := time.Now()
	clicks := 1
	if b == p.lastButton && p.lastClicks == 1 && now.Sub(p.lastPress) <= doubleClickTime &&
		abs(x-p.lastX) <= doubleClickSlop*w.scale && abs(y-p.lastY) <= doubleClickSlop*w.scale {
		clicks = 2
	}
	p.lastButton, p.lastPress, p.lastX, p.lastY, p.lastClicks = b, now, x, y, clicks
	return clicks
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// A touch is a tap or a pan. A finger that lifts before it moves touchSlop is
// a tap, delivered as a click where it went down; one that moves further pans,
// delivered as precise scrolling, as a touchpad scrolls. Only the first finger
// counts: a pinch is not recognized yet.

func (w *window) touchDown(e js.Value) {
	p := &w.in
	if p.touching {
		return
	}
	w.canvas.Call("focus")
	w.canvas.Call("setPointerCapture", e.Get("pointerId"))
	p.touching, p.panning = true, false
	p.touchID = e.Get("pointerId").Int()
	p.touchX0, p.touchY0 = e.Get("offsetX").Float(), e.Get("offsetY").Float()
	p.touchX, p.touchY = p.touchX0, p.touchY0
}

func (w *window) touchMove(e js.Value) {
	p := &w.in
	if !p.touching || e.Get("pointerId").Int() != p.touchID {
		return
	}
	x, y := e.Get("offsetX").Float(), e.Get("offsetY").Float()
	if !p.panning && math.Hypot(x-p.touchX0, y-p.touchY0) < touchSlop {
		return
	}
	p.panning = true
	dx, dy := float32(p.touchX-x), float32(p.touchY-y)
	p.touchX, p.touchY = x, y
	px, py := float32(x)*w.scale, float32(y)*w.scale
	Post(func() {
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: px, Y: py})
		w.emit(platform.Event{Kind: platform.EventScroll, X: px, Y: py, DeltaX: dx, DeltaY: dy, Precise: true})
	})
}

func (w *window) touchUp(e js.Value) {
	p := &w.in
	if !p.touching || e.Get("pointerId").Int() != p.touchID {
		return
	}
	p.touching = false
	if p.panning || e.Get("type").String() == "pointercancel" {
		return
	}
	x, y := float32(p.touchX0)*w.scale, float32(p.touchY0)*w.scale
	Post(func() {
		clicks := w.countClicks(input.ButtonLeft, x, y)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y})
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: input.ButtonLeft, Clicks: clicks})
		w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft})
		w.emit(platform.Event{Kind: platform.EventMouseLeave})
	})
}

// wheel delivers a wheel event as scrolling, in logical (CSS) pixels.
func (w *window) wheel(e js.Value) {
	e.Call("preventDefault")
	dx, dy := e.Get("deltaX").Float(), e.Get("deltaY").Float()
	var unit float64
	switch e.Get("deltaMode").Int() {
	case 0: // pixels
		unit = 1
	case 1: // lines
		unit = 16
	default: // pages
		unit = w.cssH
	}
	dx, dy = dx*unit, dy*unit
	// A mouse wheel moves in steps of 100 pixels or more (or in lines), and is
	// smoothed; a touchpad moves in small or fractional amounts, and is not.
	precise := e.Get("deltaMode").Int() == 0 && (math.Abs(dy) < 50 && math.Abs(dx) < 50 ||
		dy != math.Trunc(dy) || dx != math.Trunc(dx))
	x, y := w.pos(e)
	m := mods(e)
	Post(func() {
		w.emit(platform.Event{Kind: platform.EventScroll, X: x, Y: y, DeltaX: float32(dx), DeltaY: float32(dy),
			Precise: precise, Mods: m})
	})
}

// key delivers a key press or release, and the text a press types. It keeps
// the browser from acting on the keys the app uses, but not on the browser's
// own shortcuts, such as reloading or closing the tab.
func (w *window) key(e js.Value, down bool) {
	code, keyName := e.Get("code").String(), e.Get("key").String()
	k := keyCodes[code]
	m := mods(e)
	text := ""
	if down && m&(input.ModCtrl|input.ModSuper) == 0 && utf8.RuneCountInString(keyName) == 1 {
		text = keyName
	}
	if k == input.KeyUnknown && text == "" {
		return
	}
	if !browserShortcut(k, m) {
		e.Call("preventDefault")
	}
	repeat := e.Get("repeat").Bool()
	Post(func() {
		if !down {
			w.emit(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: m})
			return
		}
		if k != input.KeyUnknown {
			w.emit(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: m, Repeat: repeat})
		}
		if text != "" {
			w.emit(platform.Event{Kind: platform.EventTextInput, Text: text})
		}
	})
}

// browserShortcut reports whether the browser keeps the key press: reload,
// the address bar, new and closed tabs and windows, quitting, switching tabs,
// full screen and the developer tools.
func browserShortcut(k input.Key, m input.Modifiers) bool {
	switch k {
	case input.KeyF5, input.KeyF11, input.KeyF12:
		return true
	}
	if m&(input.ModCtrl|input.ModSuper) == 0 {
		return false
	}
	switch k {
	case input.KeyR, input.KeyL, input.KeyT, input.KeyW, input.KeyN, input.KeyQ, input.KeyTab:
		return true
	case input.KeyI, input.KeyJ:
		return m&input.ModShift != 0
	}
	return false
}

// keyCodes maps KeyboardEvent.code, which names a key by its position, to
// jamb's keys, which do too.
var keyCodes = func() map[string]input.Key {
	m := map[string]input.Key{
		"Enter": input.KeyEnter, "NumpadEnter": input.KeyEnter, "Escape": input.KeyEscape,
		"Backspace": input.KeyBackspace, "Delete": input.KeyDelete, "Tab": input.KeyTab,
		"Space": input.KeySpace, "ArrowLeft": input.KeyLeft, "ArrowRight": input.KeyRight,
		"ArrowUp": input.KeyUp, "ArrowDown": input.KeyDown, "Home": input.KeyHome, "End": input.KeyEnd,
		"PageUp": input.KeyPageUp, "PageDown": input.KeyPageDown, "Insert": input.KeyInsert,
		"Equal": input.KeyEqual, "Minus": input.KeyMinus, "ContextMenu": input.KeyMenu,
		"Slash": input.KeySlash, "Backslash": input.KeyBackslash, "BracketLeft": input.KeyLeftBracket,
		"BracketRight": input.KeyRightBracket, "Semicolon": input.KeySemicolon, "Quote": input.KeyApostrophe,
		"Comma": input.KeyComma, "Period": input.KeyPeriod, "Backquote": input.KeyBacktick,
	}
	for i := range 12 {
		m["F"+itoa(i+1)] = input.KeyF1 + input.Key(i)
	}
	for i := range 26 {
		m["Key"+string(rune('A'+i))] = input.KeyA + input.Key(i)
	}
	for i := range 10 {
		m["Digit"+itoa(i)] = input.Key0 + input.Key(i)
	}
	return m
}()

func itoa(i int) string {
	var b strings.Builder
	if i >= 10 {
		b.WriteByte(byte('0' + i/10))
	}
	b.WriteByte(byte('0' + i%10))
	return b.String()
}
