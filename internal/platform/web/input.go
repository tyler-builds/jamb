//go:build js

package web

import (
	"math"
	"strings"
	"syscall/js"
	"time"

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

// The momentum of a pan, as iOS scrolls: the finger's speed over its last
// moves carries on after it lifts, slowing by flingDecay each millisecond (the
// rate of UIScrollView's normal deceleration), until it is slower than
// flingStop. A pan that ends slower than flingStart has none.
const (
	flingWindow = 100 * time.Millisecond // the moves that give a pan's speed
	flingDecay  = 0.998                  // per millisecond
	flingStart  = 0.3                    // CSS pixels per millisecond
	flingStop   = 0.02                   // CSS pixels per millisecond
	flingStep   = 16 * time.Millisecond
)

// pointerState is what the window remembers of the pointer between events.
type pointerState struct {
	lastButton input.MouseButton
	lastPress  time.Time
	lastX      float32
	lastY      float32
	lastClicks int
	pressed    int // mouse buttons pressed on the canvas and not released

	// The finger of a touch: where it went down and was last, and whether it
	// moved far enough to pan. moves are its recent positions, for the speed
	// the pan carries on with.
	touchID           int
	touching, panning bool
	touchX0, touchY0  float64
	touchX, touchY    float64
	moves             []touchMove

	// The momentum of the last pan, UI goroutine only: its speed in CSS pixels
	// per millisecond, where it scrolls, and when it last scrolled. A touch
	// that stops it is no tap (noTap).
	fling          *timer
	flingVX        float64
	flingVY        float64
	flingAt        time.Time
	flingX, flingY float32
	noTap          bool
}

// touchMove is a finger's position at a time.
type touchMove struct {
	at   time.Time
	x, y float64
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

	// A press starts on the canvas, and its moves and its release are heard on
	// the whole window, so a drag goes on outside the canvas. This takes the
	// place of setPointerCapture, which throws when the browser no longer
	// knows the pointer, and a JavaScript exception ends a TinyGo build.
	w.on(c, "pointerdown", w.pointerDown)
	w.on(win, "pointermove", w.pointerMove)
	w.on(win, "pointerup", w.pointerUp)
	w.on(win, "pointercancel", w.pointerUp)
	w.on(c, "pointerleave", func(e js.Value) {
		if e.Get("pointerType").String() != "touch" && e.Get("buttons").Int() == 0 {
			Post(func() { w.emit(platform.Event{Kind: platform.EventMouseLeave}) })
		}
	})
	w.on(c, "contextmenu", func(e js.Value) { e.Call("preventDefault") })
	// The mouse events that a browser makes from a touch would move the focus
	// to the canvas, away from the textarea, which hides a phone's keyboard.
	// The pointer events above stand for them.
	w.on(c, "mousedown", func(e js.Value) { e.Call("preventDefault") })
	w.on(c, "touchend", func(e js.Value) {
		if e.Get("cancelable").Bool() {
			e.Call("preventDefault")
		}
		// Some browsers show the keyboard only for a focus in a touch event.
		w.syncTextFocus()
	})
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

// cssPos returns the event's position in CSS pixels relative to the canvas.
// The event's target may be another element, when a drag has left the canvas.
func (w *window) cssPos(e js.Value) (float64, float64) {
	r := w.canvas.Call("getBoundingClientRect")
	return e.Get("clientX").Float() - r.Get("left").Float(), e.Get("clientY").Float() - r.Get("top").Float()
}

// pos returns the event's position in device pixels relative to the canvas.
func (w *window) pos(e js.Value) (float32, float32) {
	x, y := w.cssPos(e)
	return float32(x) * w.scale, float32(y) * w.scale
}

// over reports whether the event's position is over the canvas.
func (w *window) over(e js.Value) bool {
	x, y := w.cssPos(e)
	return x >= 0 && y >= 0 && x < w.cssW && y < w.cssH
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
		if appleKeys {
			m |= input.ModCtrl
		} else {
			m |= input.ModSuper
		}
	}
	return m
}

// appleKeys reports whether the page runs on a Mac, iPhone or iPad, where
// Command does what Ctrl does elsewhere, as copying and pasting. jamb's
// shortcuts use Ctrl, so Command counts as Ctrl there.
var appleKeys = func() bool {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return false
	}
	p := nav.Get("platform")
	return p.Type() == js.TypeString && (strings.HasPrefix(p.String(), "Mac") || strings.HasPrefix(p.String(), "iP"))
}()

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
	e.Call("preventDefault") // keeps the focus where it is, and no text is selected
	w.focusWindow()
	w.in.pressed++
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
	if w.in.pressed == 0 && !w.over(e) {
		return // the pointer is elsewhere on the page, and no press of the canvas holds it
	}
	x, y := w.pos(e)
	m := mods(e)
	if w.in.pressed > 0 && e.Get("buttons").Int() == 0 {
		// The press ended outside the browser, where the page heard no release.
		w.in.pressed = 0
		Post(func() {
			w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft, Mods: m})
		})
	}
	Post(func() { w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y, Mods: m}) })
}

func (w *window) pointerUp(e js.Value) {
	if e.Get("pointerType").String() == "touch" {
		w.touchUp(e)
		return
	}
	b, ok := button(e)
	if !ok || w.in.pressed == 0 {
		return // not the release of a press of the canvas
	}
	w.in.pressed--
	x, y := w.pos(e)
	m := mods(e)
	// The release is handled while its event lasts, which lets a click copy
	// to the system's clipboard (see SetClipboard).
	runNow(func() { w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: b, Mods: m}) })
}

// focusWindow gives the canvas the page's focus, unless the window has it
// already: the textarea keeps it while a text field has the app's focus.
func (w *window) focusWindow() {
	active := js.Global().Get("document").Get("activeElement")
	if !active.Equal(w.canvas) && !active.Equal(w.text.area) {
		w.canvas.Call("focus", map[string]any{"preventScroll": true})
	}
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
// delivered as precise scrolling, as a touchpad scrolls, and keeps scrolling
// with its momentum after it lifts. A touch while that momentum lasts stops it,
// and is no tap. Only the first finger counts: a pinch is not recognized yet.

func (w *window) touchDown(e js.Value) {
	p := &w.in
	if p.touching {
		return
	}
	e.Call("preventDefault") // the press does not move the focus, which would hide a phone's keyboard
	w.focusWindow()
	p.touching, p.panning = true, false
	p.touchID = e.Get("pointerId").Int()
	p.touchX0, p.touchY0 = w.cssPos(e)
	p.touchX, p.touchY = p.touchX0, p.touchY0
	p.moves = append(p.moves[:0], touchMove{time.Now(), p.touchX0, p.touchY0})
	Post(w.stopFling)
}

func (w *window) touchMove(e js.Value) {
	p := &w.in
	if !p.touching || e.Get("pointerId").Int() != p.touchID {
		return
	}
	x, y := w.cssPos(e)
	if !p.panning && math.Hypot(x-p.touchX0, y-p.touchY0) < touchSlop {
		return
	}
	p.panning = true
	dx, dy := float32(p.touchX-x), float32(p.touchY-y)
	p.touchX, p.touchY = x, y
	now := time.Now()
	p.moves = append(p.moves, touchMove{now, x, y})
	for len(p.moves) > 2 && now.Sub(p.moves[0].at) > flingWindow {
		p.moves = p.moves[1:]
	}
	px, py := float32(x)*w.scale, float32(y)*w.scale
	Post(func() { w.touchScroll(px, py, dx, dy) })
}

// touchScroll scrolls at x, y for a touch, which is no pointer that rests
// there: the app is told it left at once, so what scrolls under the finger
// shows no hover. UI goroutine only.
func (w *window) touchScroll(x, y, dx, dy float32) {
	w.emit(platform.Event{Kind: platform.EventScroll, X: x, Y: y, DeltaX: dx, DeltaY: dy, Precise: true})
	w.emit(platform.Event{Kind: platform.EventMouseLeave})
}

func (w *window) touchUp(e js.Value) {
	p := &w.in
	if !p.touching || e.Get("pointerId").Int() != p.touchID {
		return
	}
	p.touching = false
	if e.Get("type").String() == "pointercancel" {
		Post(func() { w.in.noTap = false })
		return
	}
	if p.panning {
		// The speed over the last moves, if the finger was still moving when
		// it lifted, carries the scrolling on.
		vx, vy, x, y := 0.0, 0.0, float32(0), float32(0)
		first, last := p.moves[0], p.moves[len(p.moves)-1]
		if ms := float64(last.at.Sub(first.at).Microseconds()) / 1000; ms > 0 && time.Since(last.at) < flingWindow {
			vx, vy = (first.x-last.x)/ms, (first.y-last.y)/ms
			x, y = float32(last.x)*w.scale, float32(last.y)*w.scale
		}
		Post(func() {
			w.in.noTap = false // the touch that stopped a fling panned instead
			w.startFling(vx, vy, x, y)
		})
		return
	}
	// The tap is handled, and a frame drawn, while its event lasts: if the tap
	// focused a text field, the event then focuses the textarea, which a
	// phone shows its keyboard for only now.
	x, y := float32(p.touchX0)*w.scale, float32(p.touchY0)*w.scale
	if runNow(func() {
		if w.in.noTap {
			w.in.noTap = false
			return // the touch stopped a fling
		}
		clicks := w.countClicks(input.ButtonLeft, x, y)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y})
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: input.ButtonLeft, Clicks: clicks})
		w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft})
		w.emit(platform.Event{Kind: platform.EventMouseLeave})
		w.drawNow()
	}) {
		w.syncTextFocus()
	}
}

// startFling carries a pan on at the speed vx, vy, in CSS pixels per
// millisecond, scrolling at x, y. UI goroutine only.
func (w *window) startFling(vx, vy float64, x, y float32) {
	if math.Hypot(vx, vy) < flingStart {
		return
	}
	p := &w.in
	p.fling.stop()
	p.flingVX, p.flingVY, p.flingX, p.flingY, p.flingAt = vx, vy, x, y, time.Now()
	p.fling = after(flingStep, w.flingFrame)
}

// flingFrame scrolls by the momentum's distance since its last frame, and
// slows it down. UI goroutine only.
func (w *window) flingFrame() {
	p := &w.in
	now := time.Now()
	ms := float64(now.Sub(p.flingAt).Microseconds()) / 1000
	p.flingAt = now
	// The distance is the integral of the decaying speed over the frame.
	k := math.Pow(flingDecay, ms)
	dist := (1 - k) / -math.Log(flingDecay)
	dx, dy := p.flingVX*dist, p.flingVY*dist
	p.flingVX, p.flingVY = p.flingVX*k, p.flingVY*k
	w.touchScroll(p.flingX, p.flingY, float32(dx), float32(dy))
	if math.Hypot(p.flingVX, p.flingVY) < flingStop || w.destroyed {
		p.fling = nil
		return
	}
	p.fling = after(flingStep, w.flingFrame)
}

// stopFling stops the momentum of a pan, when a touch lands while it lasts,
// and makes that touch no tap. UI goroutine only.
func (w *window) stopFling() {
	p := &w.in
	p.noTap = p.fling != nil
	p.fling.stop()
	p.fling = nil
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
// own shortcuts, such as reloading or closing the tab. In a text field, the
// textarea types a press's text, and its input event delivers it (see
// makeTextArea), as an input method's or a phone keyboard's.
func (w *window) key(e js.Value, down bool) {
	if e.Get("isComposing").Bool() || e.Get("keyCode").Int() == 229 {
		return // a press of an input method's, whose text comes with its composition
	}
	code, name := e.Get("code").String(), e.Get("key").String()
	k := keyCodes[code]
	if k == input.KeyUnknown {
		k = keyNames[name]
	}
	m := mods(e)
	typed := down && printable(name, m)
	if k == input.KeyUnknown && !typed {
		return
	}
	text, field := "", w.text.focused
	if typed && !field {
		text = name
	}
	repeat := e.Get("repeat").Bool()
	deliver := func() {
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
	}
	switch {
	case clipboardKey(k, m) && k == input.KeyV && field:
		return // the paste event delivers it, with the clipboard's text
	case clipboardKey(k, m):
		// Copy and cut, and paste of the app's own clipboard outside a text
		// field: handled while the event lasts, so a copy reaches the system's
		// clipboard (see SetClipboard).
		e.Call("preventDefault")
		runNow(deliver)
		return
	case browserShortcut(k, m), field && (typed || name == "Dead"):
	default:
		e.Call("preventDefault")
	}
	if down {
		w.text.lastKey, w.text.lastKeyAt = k, time.Now()
	}
	Post(deliver)
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
