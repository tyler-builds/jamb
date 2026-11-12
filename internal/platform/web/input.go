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

// longPressTime is how long a finger stays where it went down before its
// touch is a long press, as Android and iOS count it.
const longPressTime = 500 * time.Millisecond

// The momentum of a pan, as iOS scrolls: the finger's speed over its last
// moves carries on after it lifts, slowing by flingDecay each millisecond (the
// rate of UIScrollView's normal deceleration), until it is slower than
// flingStop. A pan that ends slower than flingStart has none. A touch stops
// the momentum, and if it lifts without moving it also taps what it landed on,
// so a list in motion takes one tap, not one to stop it and another to act.
//
// A pan and its momentum scroll once per display frame (see touchFrame), by
// how far the finger moved since the last frame, or by how far the momentum
// carries in the time since it, from the frame's own time. Touch events and
// timers keep their own time, and scrolling by each event or on a timer would
// scroll twice in one frame and not at all in the next.
const (
	flingWindow = 100.0 // milliseconds of moves that give a pan's speed
	flingDecay  = 0.998 // per millisecond
	flingStart  = 0.3   // CSS pixels per millisecond
	flingStop   = 0.02  // CSS pixels per millisecond
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
	dragging          bool // the touch drags an element that takes the drag (see platform.TouchHandler)
	// longTmr makes the touch a long press, unless the finger moves or lifts
	// first. held is set when the app acted on the long press, as by opening a
	// menu: the rest of the touch does nothing.
	longTmr          *timer
	held             bool
	touchX0, touchY0 float64
	touchX, touchY   float64
	moves            []touchMove
	// panDX, panDY is how far the pan moved since the last frame scrolled it,
	// in CSS pixels (see touchFrame).
	panDX, panDY float64

	// The momentum of the last pan, UI goroutine only: its speed in CSS pixels
	// per millisecond, where it scrolls, and the time it last scrolled to, on
	// the clock of performance.now and of display frames.
	flinging       bool
	flingVX        float64
	flingVY        float64
	flingAt        float64
	flingX, flingY float32
}

// touchMove is a finger's position at a time: the event's timeStamp, in
// milliseconds.
type touchMove struct {
	at   float64
	x, y float64
}

// performance is the page's clock.
var performance = js.Global().Get("performance")

// now returns the time on the clock of display frames, in milliseconds.
func now() float64 { return performance.Call("now").Float() }

// The page's input reaches the window as plain values: a pointerEvent, a
// wheelEvent or a keyEvent, with everything the window needs of the browser's
// event. The page's host script makes them from numbers it passes to the app
// (see host.go), and without it a listener reads them from the event (see
// listen). Each handler reports whether the event's default action is
// prevented, which only the event's own listener can do.

// pointerKind is the type of a pointer event.
type pointerKind int

const (
	pointerDown pointerKind = iota
	pointerMove
	pointerUp
	pointerCancel
	pointerLeave
)

// pointerEvent is a pointer event of the page.
type pointerEvent struct {
	kind    pointerKind
	touch   bool    // a finger, not a mouse or a pen
	id      int     // the pointer's ID
	x, y    float64 // CSS pixels, relative to the canvas
	button  int     // the button that changed, as the browser numbers them
	buttons int     // the buttons held, as bits
	mods    input.Modifiers
	at      float64 // the event's timeStamp, in milliseconds
}

// wheelEvent is a wheel event of the page.
type wheelEvent struct {
	x, y   float64 // CSS pixels, relative to the canvas
	dx, dy float64
	mode   int // the unit of dx and dy: 0 pixels, 1 lines, 2 pages
	mods   input.Modifiers
}

// keyEvent is a key press or release.
type keyEvent struct {
	down      bool
	code, key string // KeyboardEvent.code and .key
	mods      input.Modifiers
	repeat    bool
	composing bool // a press of an input method's (isComposing, or keyCode 229)
}

// The modifier keys of an event as bits, as the host script passes them.
const (
	pageShift = 1 << iota
	pageCtrl
	pageAlt
	pageMeta
)

// pageMods returns the modifiers of an event from their bits.
func pageMods(bits int) input.Modifiers {
	var m input.Modifiers
	if bits&pageShift != 0 {
		m |= input.ModShift
	}
	if bits&pageCtrl != 0 {
		m |= input.ModCtrl
	}
	if bits&pageAlt != 0 {
		m |= input.ModAlt
	}
	if bits&pageMeta != 0 {
		if appleKeys {
			m |= input.ModCtrl
		} else {
			m |= input.ModSuper
		}
	}
	return m
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

	// The pointer, the wheel and the keys, which make most of the page's
	// events: the host script delivers them when the page has it (see
	// host.go), and listeners of the window's own read them otherwise.
	if !w.hostListen() {
		// A press starts on the canvas, and its moves and its release are heard
		// on the whole window, so a drag goes on outside the canvas. This takes
		// the place of setPointerCapture, which throws when the browser no
		// longer knows the pointer, and a JavaScript exception ends a TinyGo
		// build.
		pointer := func(kind pointerKind) func(js.Value) {
			return func(e js.Value) {
				if w.pointer(w.pointerOf(kind, e)) {
					e.Call("preventDefault")
				}
			}
		}
		w.on(c, "pointerdown", pointer(pointerDown))
		w.on(win, "pointermove", pointer(pointerMove))
		w.on(win, "pointerup", pointer(pointerUp))
		w.on(win, "pointercancel", pointer(pointerCancel))
		w.on(c, "pointerleave", pointer(pointerLeave))
		// The wheel listener is not passive, so that it can keep the page from
		// scrolling and from zooming on Ctrl+wheel.
		w.on(c, "wheel", func(e js.Value) {
			x, y := w.cssPos(e)
			if w.wheel(wheelEvent{x: x, y: y, dx: e.Get("deltaX").Float(), dy: e.Get("deltaY").Float(),
				mode: e.Get("deltaMode").Int(), mods: mods(e)}) {
				e.Call("preventDefault")
			}
		}, map[string]any{"passive": false})
		w.listenKeys(c)
		w.on(c, "contextmenu", func(e js.Value) { e.Call("preventDefault") })
		// The mouse events that a browser makes from a touch would move the
		// focus to the canvas, away from the textarea, which hides a phone's
		// keyboard. The pointer events above stand for them.
		w.on(c, "mousedown", func(e js.Value) { e.Call("preventDefault") })
	}
	w.on(c, "touchend", func(e js.Value) {
		if e.Get("cancelable").Bool() {
			e.Call("preventDefault")
		}
		// Some browsers show the keyboard only for a focus in a touch event.
		w.syncTextFocus()
	})
	w.on(doc, "visibilitychange", func(js.Value) {
		w.pageHidden = doc.Get("visibilityState").String() == "hidden"
		Post(w.Wake)
	})

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

// listenKeys hears the key presses and releases of an element with listeners
// of the window's own.
func (w *window) listenKeys(target js.Value) {
	key := func(down bool) func(js.Value) {
		return func(e js.Value) {
			if w.key(keyEvent{down: down, code: e.Get("code").String(), key: e.Get("key").String(), mods: mods(e),
				repeat:    e.Get("repeat").Bool(),
				composing: e.Get("isComposing").Bool() || e.Get("keyCode").Int() == 229}) {
				e.Call("preventDefault")
			}
		}
	}
	w.on(target, "keydown", key(true))
	w.on(target, "keyup", key(false))
}

// cssPos returns the event's position in CSS pixels relative to the canvas.
// The event's target may be another element, when a drag has left the canvas.
func (w *window) cssPos(e js.Value) (float64, float64) {
	r := w.canvas.Call("getBoundingClientRect")
	return e.Get("clientX").Float() - r.Get("left").Float(), e.Get("clientY").Float() - r.Get("top").Float()
}

// pointerOf reads a pointer event of the browser's.
func (w *window) pointerOf(kind pointerKind, e js.Value) pointerEvent {
	x, y := w.cssPos(e)
	return pointerEvent{kind: kind, touch: e.Get("pointerType").String() == "touch", id: e.Get("pointerId").Int(),
		x: x, y: y, button: e.Get("button").Int(), buttons: e.Get("buttons").Int(), mods: mods(e),
		at: e.Get("timeStamp").Float()}
}

// device returns a position in CSS pixels in device pixels.
func (w *window) device(x, y float64) (float32, float32) {
	return float32(x) * w.scale, float32(y) * w.scale
}

// over reports whether a position in CSS pixels is over the canvas.
func (w *window) over(x, y float64) bool {
	return x >= 0 && y >= 0 && x < w.cssW && y < w.cssH
}

// mods reads the modifier keys of a browser's event.
func mods(e js.Value) input.Modifiers {
	bits := 0
	if e.Get("shiftKey").Bool() {
		bits |= pageShift
	}
	if e.Get("ctrlKey").Bool() {
		bits |= pageCtrl
	}
	if e.Get("altKey").Bool() {
		bits |= pageAlt
	}
	if e.Get("metaKey").Bool() {
		bits |= pageMeta
	}
	return pageMods(bits)
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

// mouseButton returns the button of the browser's number n.
func mouseButton(n int) (input.MouseButton, bool) {
	switch n {
	case 0:
		return input.ButtonLeft, true
	case 1:
		return input.ButtonMiddle, true
	case 2:
		return input.ButtonRight, true
	}
	return 0, false
}

// pointer handles a pointer event, and reports whether its default action is
// prevented.
func (w *window) pointer(e pointerEvent) bool {
	switch e.kind {
	case pointerDown:
		return w.pointerDown(e)
	case pointerMove:
		w.pointerMove(e)
	case pointerUp, pointerCancel:
		w.pointerUp(e)
	case pointerLeave:
		if !e.touch && e.buttons == 0 {
			Post(func() { w.emit(platform.Event{Kind: platform.EventMouseLeave}) })
		}
	}
	return false
}

func (w *window) pointerDown(e pointerEvent) bool {
	if e.touch {
		return w.touchDown(e)
	}
	b, ok := mouseButton(e.button)
	if !ok {
		return false
	}
	w.focusWindow()
	w.in.pressed++
	x, y := w.device(e.x, e.y)
	m := e.mods
	Post(func() {
		clicks := w.countClicks(b, x, y)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y, Mods: m})
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: b, Clicks: clicks, Mods: m})
	})
	return true // keeps the focus where it is, and no text is selected
}

func (w *window) pointerMove(e pointerEvent) {
	if e.touch {
		w.touchMove(e)
		return
	}
	if w.in.pressed == 0 && !w.over(e.x, e.y) {
		return // the pointer is elsewhere on the page, and no press of the canvas holds it
	}
	x, y := w.device(e.x, e.y)
	m := e.mods
	if w.in.pressed > 0 && e.buttons == 0 {
		// The press ended outside the browser, where the page heard no release.
		w.in.pressed = 0
		Post(func() {
			w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft, Mods: m})
		})
	}
	Post(func() { w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y, Mods: m}) })
}

func (w *window) pointerUp(e pointerEvent) {
	if e.touch {
		w.touchUp(e)
		return
	}
	b, ok := mouseButton(e.button)
	if !ok || w.in.pressed == 0 {
		return // not the release of a press of the canvas
	}
	w.in.pressed--
	x, y := w.device(e.x, e.y)
	m := e.mods
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

// A touch is a drag, a tap or a pan. A touch on an element that takes the drag
// itself, as a slider does (see platform.TouchHandler), presses the mouse at
// once and drags it. Otherwise a finger that lifts before it moves touchSlop is
// a tap, delivered as a click where it went down; one that moves further pans,
// delivered as precise scrolling, as a touchpad scrolls, and keeps scrolling
// with its momentum after it lifts. A touch while that momentum lasts stops it,
// and is no tap. A finger that stays where it went down for longPressTime
// presses long: the app selects the word of a text there, which the finger then
// drags further, or opens the context menu there (see longPress). Only the
// first finger counts: a pinch is not recognized yet.

func (w *window) touchDown(e pointerEvent) bool {
	p := &w.in
	if p.touching {
		return false
	}
	w.focusWindow()
	p.touching, p.panning, p.dragging, p.held = true, false, false, false
	p.longTmr.stop()
	p.touchID = e.id
	p.touchX0, p.touchY0 = e.x, e.y
	p.touchX, p.touchY = p.touchX0, p.touchY0
	p.moves = append(p.moves[:0], touchMove{e.at, p.touchX0, p.touchY0})
	p.panDX, p.panDY = 0, 0
	// The app says, while the event lasts, whether the element under the
	// finger takes the drag. If it answers later, after the finger moved or
	// lifted, the touch stays the pan or the tap that it has become.
	x, y := w.device(p.touchX0, p.touchY0)
	id := p.touchID
	runNow(func() {
		w.stopFling()
		th, ok := w.handler.(platform.TouchHandler)
		if !ok || !w.ready || w.destroyed || !p.touching || p.touchID != id || p.panning {
			return
		}
		if !th.TouchDrags(w, x, y) {
			p.longTmr = after(longPressTime, func() { w.longPress(id) })
			return
		}
		p.dragging = true
		clicks := w.countClicks(input.ButtonLeft, x, y)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y})
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: input.ButtonLeft, Clicks: clicks})
	})
	return true // the press does not move the focus, which would hide a phone's keyboard
}

func (w *window) touchMove(e pointerEvent) {
	p := &w.in
	if !p.touching || e.id != p.touchID {
		return
	}
	x, y := e.x, e.y
	if p.dragging {
		px, py := float32(x)*w.scale, float32(y)*w.scale
		Post(func() { w.emit(platform.Event{Kind: platform.EventMouseMove, X: px, Y: py}) })
		return
	}
	if p.held || !p.panning && math.Hypot(x-p.touchX0, y-p.touchY0) < touchSlop {
		return
	}
	p.panning = true
	p.longTmr.stop()
	p.panDX += p.touchX - x
	p.panDY += p.touchY - y
	p.touchX, p.touchY = x, y
	at := e.at
	p.moves = append(p.moves, touchMove{at, x, y})
	for len(p.moves) > 2 && at-p.moves[0].at > flingWindow {
		p.moves = p.moves[1:]
	}
	w.Wake() // the next frame scrolls by the move (see touchFrame)
}

// longPress makes the touch of the finger id a long press, when the finger
// still rests where it went down. The app says what the press does there (see
// platform.LongPress): a drag that it began goes on as the drag of an element
// that takes one does, and a press that it acted on ends the touch, so the
// finger's lifting is no tap. UI goroutine only.
func (w *window) longPress(id int) {
	p := &w.in
	th, ok := w.handler.(platform.TouchHandler)
	if !ok || !w.ready || w.destroyed || !p.touching || p.touchID != id || p.panning || p.dragging || p.held {
		return
	}
	x, y := w.device(p.touchX0, p.touchY0)
	switch th.TouchLongPress(w, x, y) {
	case platform.LongPressDrags:
		p.dragging = true
	case platform.LongPressTaken:
		p.held = true
		// A finger is no pointer that rests on the menu: nothing shows hover.
		w.emit(platform.Event{Kind: platform.EventMouseLeave})
	}
}

// touchFrame scrolls by what the pan moved since the last frame, and by the
// momentum up to the frame's time t, in milliseconds. A frame calls it before
// it draws. UI goroutine only.
func (w *window) touchFrame(t float64) {
	p := &w.in
	if p.panDX != 0 || p.panDY != 0 {
		dx, dy := float32(p.panDX), float32(p.panDY)
		p.panDX, p.panDY = 0, 0
		w.touchScroll(float32(p.touchX)*w.scale, float32(p.touchY)*w.scale, dx, dy)
	}
	if !p.flinging {
		return
	}
	ms := t - p.flingAt
	if ms <= 0 {
		return
	}
	p.flingAt = t
	// The distance is the integral of the decaying speed over the time.
	k := math.Pow(flingDecay, ms)
	dist := (1 - k) / -math.Log(flingDecay)
	dx, dy := p.flingVX*dist, p.flingVY*dist
	p.flingVX, p.flingVY = p.flingVX*k, p.flingVY*k
	w.touchScroll(p.flingX, p.flingY, float32(dx), float32(dy))
	if math.Hypot(p.flingVX, p.flingVY) < flingStop {
		p.flinging = false
	}
}

// touchScroll scrolls at x, y for a touch, which is no pointer that rests
// there: the app is told it left at once, so what scrolls under the finger
// shows no hover. UI goroutine only.
func (w *window) touchScroll(x, y, dx, dy float32) {
	w.emit(platform.Event{Kind: platform.EventScroll, X: x, Y: y, DeltaX: dx, DeltaY: dy, Precise: true})
	w.emit(platform.Event{Kind: platform.EventMouseLeave})
}

func (w *window) touchUp(e pointerEvent) {
	p := &w.in
	if !p.touching || e.id != p.touchID {
		return
	}
	p.touching = false
	p.longTmr.stop()
	if p.held {
		p.held = false
		return
	}
	if p.dragging {
		p.dragging = false
		x, y := w.device(e.x, e.y)
		Post(func() {
			w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft})
			w.emit(platform.Event{Kind: platform.EventMouseLeave})
		})
		return
	}
	if e.kind == pointerCancel {
		return
	}
	if p.panning {
		// The speed over the last moves, if the finger was still moving when
		// it lifted, carries the scrolling on.
		vx, vy, x, y := 0.0, 0.0, float32(0), float32(0)
		first, last := p.moves[0], p.moves[len(p.moves)-1]
		if ms := last.at - first.at; ms > 0 && e.at-last.at < flingWindow {
			vx, vy = (first.x-last.x)/ms, (first.y-last.y)/ms
			x, y = float32(last.x)*w.scale, float32(last.y)*w.scale
		}
		Post(func() { w.startFling(vx, vy, x, y) })
		return
	}
	// The tap is handled, and a frame drawn, while its event lasts: if the tap
	// focused a text field, the textarea then takes the focus, which a phone
	// shows its keyboard for only now.
	x, y := w.device(p.touchX0, p.touchY0)
	runNow(func() {
		clicks := w.countClicks(input.ButtonLeft, x, y)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y})
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: input.ButtonLeft, Clicks: clicks})
		w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft})
		w.emit(platform.Event{Kind: platform.EventMouseLeave})
		w.drawNow()
		w.syncTextFocus()
	})
}

// startFling carries a pan on at the speed vx, vy, in CSS pixels per
// millisecond, scrolling at x, y. UI goroutine only.
func (w *window) startFling(vx, vy float64, x, y float32) {
	if math.Hypot(vx, vy) < flingStart {
		return
	}
	p := &w.in
	p.flinging = true
	p.flingVX, p.flingVY, p.flingX, p.flingY, p.flingAt = vx, vy, x, y, now()
	w.Wake()
}

// stopFling stops the momentum of a pan, when a touch lands while it lasts.
// UI goroutine only.
func (w *window) stopFling() { w.in.flinging = false }

// wheel delivers a wheel event as scrolling, in logical (CSS) pixels. Its
// default action is prevented, which keeps the page from scrolling and from
// zooming on Ctrl+wheel.
func (w *window) wheel(e wheelEvent) bool {
	dx, dy := e.dx, e.dy
	var unit float64
	switch e.mode {
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
	precise := e.mode == 0 && (math.Abs(dy) < 50 && math.Abs(dx) < 50 ||
		dy != math.Trunc(dy) || dx != math.Trunc(dx))
	x, y := w.device(e.x, e.y)
	m := e.mods
	Post(func() {
		w.emit(platform.Event{Kind: platform.EventScroll, X: x, Y: y, DeltaX: float32(dx), DeltaY: float32(dy),
			Precise: precise, Mods: m})
	})
	return true
}

// key delivers a key press or release, and the text a press types. It keeps
// the browser from acting on the keys the app uses, but not on the browser's
// own shortcuts, such as reloading or closing the tab. In a text field, the
// textarea types a press's text, and its input event delivers it (see
// makeTextArea), as an input method's or a phone keyboard's. It reports
// whether the press's default action is prevented.
func (w *window) key(e keyEvent) bool {
	if e.composing {
		return false // a press of an input method's, whose text comes with its composition
	}
	down, name := e.down, e.key
	k := keyCodes[e.code]
	if k == input.KeyUnknown {
		k = keyNames[name]
	}
	m := e.mods
	typed := down && printable(name, m)
	if k == input.KeyUnknown && !typed {
		return false
	}
	text, field := "", w.text.focused
	if typed && !field {
		text = name
	}
	repeat := e.repeat
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
	prevent := false
	switch {
	case clipboardKey(k, m) && k == input.KeyV && field:
		return false // the paste event delivers it, with the clipboard's text
	case clipboardKey(k, m):
		// Copy and cut, and paste of the app's own clipboard outside a text
		// field: handled while the event lasts, so a copy reaches the system's
		// clipboard (see SetClipboard).
		runNow(deliver)
		return true
	case browserShortcut(k, m), field && (typed || name == "Dead"):
	default:
		prevent = true
	}
	if down {
		w.text.lastKey, w.text.lastKeyAt = k, time.Now()
	}
	if down && (k == input.KeyEnter || k == input.KeySpace) {
		// The keys that choose a menu's item or press a button, as a click does.
		Post(func() { inGesture(deliver) })
		return prevent
	}
	Post(deliver)
	return prevent
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
