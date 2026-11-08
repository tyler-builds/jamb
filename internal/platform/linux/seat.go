//go:build linux

package linux

import (
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
	"golang.org/x/sys/unix"
)

// Linux input event codes of the mouse buttons (linux/input-event-codes.h).
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// wp_cursor_shape_device_v1 shapes.
const (
	shapeDefault    = 1
	shapePointer    = 4
	shapeText       = 9
	shapeNotAllowed = 15
	shapeEWResize   = 26
	shapeNSResize   = 27
	shapeNESWResize = 28
	shapeNWSEResize = 29
)

// xdg_toplevel resize edges.
const (
	edgeTop    = 1
	edgeBottom = 2
	edgeLeft   = 4
	edgeRight  = 8
)

// resizeBorder is how far in from its edges, in logical pixels, a window that
// draws its own title bar can be resized from.
const resizeBorder = 6

// doubleClickTime and doubleClickSlop decide when a press is a double click,
// as Windows' defaults do (500 ms, 4 pixels).
const (
	doubleClickTime = 500 * time.Millisecond
	doubleClickSlop = 4
)

// seat is the wl_seat: its pointer and keyboard, and which window each is on.
type seat struct {
	d       *display
	proxy   *wl.Proxy
	pointer *wl.Proxy
	shape   *wl.Proxy // wp_cursor_shape_device_v1, or nil
	kbProxy *wl.Proxy
	kb      *keyboard

	// The pointer.
	pointerOn   *window
	enterSerial uint32
	x, y        float64 // surface-local, logical
	buttons     int     // buttons held
	// serial is the serial of the latest input event (a press, a key, the
	// keyboard's enter), which setting the selection needs.
	serial uint32
	// The axis events of one wl_pointer.frame.
	scrollX, scrollY float64
	v120X, v120Y     int32
	finger           bool
	scrolled         bool
	// Click counting.
	lastButton input.MouseButton
	lastPress  time.Time
	lastX      float64
	lastY      float64
	lastClicks int
	// A press on a custom title bar's button, or TitleNone.
	caption platform.TitleArea
	// The resize edges under the pointer, for a window that draws its own frame.
	edges uint32

	// The keyboard.
	keyOn        *window
	repeatRate   int32 // keys per second; 0 disables repeat
	repeatDelay  time.Duration
	repeatKey    uint32
	repeatTimer  *timer
	keyboardFail bool
}

func newSeat(d *display, name, version uint32) *seat {
	s := &seat{d: d, repeatRate: 25, repeatDelay: 600 * time.Millisecond}
	s.proxy = d.conn.Bind(d.registry, name, "wl_seat", version, s.seatEvent)
	return s
}

func (s *seat) seatEvent(op uint16, d *wl.Decoder) {
	if op != 0 { // capabilities(caps)
		return
	}
	caps := d.Uint()
	c := s.d.conn
	if caps&1 != 0 && s.pointer == nil {
		s.pointer = c.NewProxy("wl_pointer", s.proxy.Version, s.pointerEvent)
		s.proxy.Send(0, wl.NewID{P: s.pointer}) // get_pointer
		if s.d.cursor != nil {
			s.shape = c.NewProxy("wp_cursor_shape_device_v1", 1, nil)
			s.d.cursor.Send(1, wl.NewID{P: s.shape}, s.pointer) // get_pointer
		}
	}
	if caps&2 != 0 && s.kbProxy == nil {
		kb, err := newKeyboard()
		if err != nil {
			// Without libxkbcommon there is no text input; the pointer still works.
			s.keyboardFail = true
			return
		}
		s.kb = kb
		s.kbProxy = c.NewProxy("wl_keyboard", s.proxy.Version, s.keyboardEvent)
		s.proxy.Send(1, wl.NewID{P: s.kbProxy}) // get_keyboard
	}
}

// windowOf returns the window of a surface id.
func (s *seat) windowOf(id uint32) *window {
	for _, w := range s.d.windows {
		if w.surface.ID == id {
			return w
		}
	}
	return nil
}

// forget drops a destroyed window.
func (s *seat) forget(w *window) {
	if s.pointerOn == w {
		s.pointerOn = nil
	}
	if s.keyOn == w {
		s.keyOn = nil
		s.stopRepeat()
	}
}

// ---- pointer ----

func (s *seat) pointerEvent(op uint16, d *wl.Decoder) {
	switch op {
	case 0: // enter(serial, surface, x, y)
		s.enterSerial = d.Uint()
		s.pointerOn = s.windowOf(d.Uint())
		s.x, s.y = d.Fixed(), d.Fixed()
		s.toContent()
		if w := s.pointerOn; w != nil {
			s.applyCursor(w.cursor)
			s.move(w)
		}
	case 1: // leave(serial, surface)
		if w := s.pointerOn; w != nil {
			w.emit(platform.Event{Kind: platform.EventMouseLeave})
		}
		s.pointerOn = nil
		s.buttons = 0
		s.edges = 0
	case 2: // motion(time, x, y)
		d.Uint()
		s.x, s.y = d.Fixed(), d.Fixed()
		s.toContent()
		if w := s.pointerOn; w != nil {
			s.move(w)
		}
	case 3: // button(serial, time, button, state)
		serial := d.Uint()
		d.Uint()
		button, pressed := d.Uint(), d.Uint() == 1
		if w := s.pointerOn; w != nil {
			s.button(w, serial, button, pressed)
		}
	case 4: // axis(time, axis, value)
		d.Uint()
		axis, v := d.Uint(), d.Fixed()
		if axis == 0 {
			s.scrollY += v
		} else {
			s.scrollX += v
		}
		s.scrolled = true
		if s.pointer.Version < 5 {
			s.flushScroll()
		}
	case 5: // frame
		s.flushScroll()
	case 6: // axis_source(source)
		s.finger = d.Uint() == 1 || s.finger
	case 9: // axis_value120(axis, value120)
		axis, v := d.Uint(), d.Int()
		if axis == 0 {
			s.v120Y += v
		} else {
			s.v120X += v
		}
	}
}

// toContent makes the pointer's position, which arrives relative to the
// surface, relative to the window's content: a window that draws its own frame
// has a margin around it (see ClientFrame). In the margin it is negative or
// past the content's size, where only the resize border takes input.
func (s *seat) toContent() {
	if w := s.pointerOn; w != nil {
		s.x -= float64(w.sentFrame.l)
		s.y -= float64(w.sentFrame.t)
	}
}

// move reports the pointer's position over w.
func (s *seat) move(w *window) {
	sc := w.scale()
	x, y := float32(s.x*sc), float32(s.y*sc)
	if s.caption != platform.TitleNone {
		return // a title-bar button press keeps the pointer until release
	}
	if edges := s.edgesAt(w); edges != s.edges {
		s.edges = edges
		s.applyCursor(w.cursor)
	}
	w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y})
}

// edgesAt returns the resize edges under the pointer: near the edges of a
// window whose frame is its own, when the window can be resized.
func (s *seat) edgesAt(w *window) uint32 {
	if !w.clientFrame() || w.maximized || w.fullscreen || s.buttons > 0 {
		return 0
	}
	var e uint32
	switch {
	case s.y < resizeBorder:
		e |= edgeTop
	case s.y >= float64(w.logH-resizeBorder):
		e |= edgeBottom
	}
	switch {
	case s.x < resizeBorder:
		e |= edgeLeft
	case s.x >= float64(w.logW-resizeBorder):
		e |= edgeRight
	}
	return e
}

func (s *seat) button(w *window, serial, code uint32, pressed bool) {
	var b input.MouseButton
	switch code {
	case btnLeft:
		b = input.ButtonLeft
	case btnRight:
		b = input.ButtonRight
	case btnMiddle:
		b = input.ButtonMiddle
	default:
		return
	}
	sc := w.scale()
	x, y := float32(s.x*sc), float32(s.y*sc)
	if !pressed {
		if s.caption != platform.TitleNone {
			s.captionUp(w, x, y)
			return
		}
		s.buttons = max(s.buttons-1, 0)
		w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: b})
		return
	}
	s.serial = serial
	if b == input.ButtonLeft && s.edges != 0 && s.buttons == 0 {
		w.toplevel.Send(6, s.proxy, serial, s.edges) // resize
		return
	}
	clicks := s.countClicks(b)
	if b == input.ButtonLeft && s.buttons == 0 && w.opts.CustomTitleBar {
		if th, ok := w.handler.(platform.TitleBarHandler); ok {
			switch area := th.TitleArea(w, x, y); area {
			case platform.TitleDrag:
				if clicks == 2 {
					w.ToggleMaximize()
				} else {
					w.toplevel.Send(5, s.proxy, serial) // move
				}
				return
			case platform.TitleMinimize, platform.TitleMaximize, platform.TitleClose:
				// The button is pressed like a client control; its action runs
				// on a release over it (see captionUp).
				s.caption = area
				w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: b, Clicks: clicks})
				return
			}
		}
	}
	if b == input.ButtonRight && w.opts.CustomTitleBar {
		if th, ok := w.handler.(platform.TitleBarHandler); ok && th.TitleArea(w, x, y) == platform.TitleDrag {
			f := w.sentFrame
			w.toplevel.Send(4, s.proxy, serial, int32(s.x)+int32(f.l), int32(s.y)+int32(f.t)) // show_window_menu
			return
		}
	}
	s.buttons++
	w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: b, Clicks: clicks})
}

// captionUp ends a press on a custom title bar's button: its action runs when
// the release is over the same button.
func (s *seat) captionUp(w *window, x, y float32) {
	area := s.caption
	s.caption = platform.TitleNone
	w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft})
	th, ok := w.handler.(platform.TitleBarHandler)
	if !ok || th.TitleArea(w, x, y) != area {
		return
	}
	switch area {
	case platform.TitleMinimize:
		w.Minimize()
	case platform.TitleMaximize:
		w.ToggleMaximize()
	case platform.TitleClose:
		w.Close()
	}
}

// countClicks returns 2 for the second press of a double click, else 1, as
// Windows reports clicks: a third press is a single click again.
func (s *seat) countClicks(b input.MouseButton) int {
	now := time.Now()
	clicks := 1
	if b == s.lastButton && s.lastClicks == 1 && now.Sub(s.lastPress) <= doubleClickTime &&
		abs64(s.x-s.lastX) <= doubleClickSlop && abs64(s.y-s.lastY) <= doubleClickSlop {
		clicks = 2
	}
	s.lastButton, s.lastPress, s.lastX, s.lastY, s.lastClicks = b, now, s.x, s.y, clicks
	return clicks
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// flushScroll reports the axis events of one pointer frame. A wheel notch
// (120 in axis_value120) scrolls 60 pixels, as on Windows; a touchpad's
// deltas are surface pixels, reported as they come.
func (s *seat) flushScroll() {
	if !s.scrolled {
		return
	}
	dx, dy := s.scrollX, s.scrollY
	precise := s.finger
	if !precise && (s.v120X != 0 || s.v120Y != 0) {
		dx, dy = float64(s.v120X)/120*60, float64(s.v120Y)/120*60
		precise = s.v120X%120 != 0 || s.v120Y%120 != 0
	} else if !precise {
		// A wheel without high-resolution events: the compositor's axis value
		// is 10 per notch by convention.
		dx, dy = dx/10*60, dy/10*60
	}
	s.scrollX, s.scrollY, s.v120X, s.v120Y, s.finger, s.scrolled = 0, 0, 0, 0, false, false
	w := s.pointerOn
	if w == nil || (dx == 0 && dy == 0) {
		return
	}
	sc := w.scale()
	e := platform.Event{Kind: platform.EventScroll, X: float32(s.x * sc), Y: float32(s.y * sc),
		DeltaX: float32(dx), DeltaY: float32(dy), Precise: precise}
	if s.kb != nil && s.kb.mods.Has(input.ModShift) && e.DeltaX == 0 {
		e.DeltaX, e.DeltaY = e.DeltaY, 0
	}
	w.emit(e)
}

// applyCursor sets the pointer's cursor over the window it is on.
func (s *seat) applyCursor(c input.Cursor) {
	if s.shape == nil {
		return
	}
	shape := uint32(shapeDefault)
	switch e := s.edges; {
	case e == edgeTop|edgeLeft || e == edgeBottom|edgeRight:
		s.shape.Send(1, s.enterSerial, uint32(shapeNWSEResize)) // set_shape
		return
	case e == edgeTop|edgeRight || e == edgeBottom|edgeLeft:
		s.shape.Send(1, s.enterSerial, uint32(shapeNESWResize))
		return
	case e == edgeTop || e == edgeBottom:
		s.shape.Send(1, s.enterSerial, uint32(shapeNSResize))
		return
	case e != 0:
		s.shape.Send(1, s.enterSerial, uint32(shapeEWResize))
		return
	}
	switch c {
	case input.CursorPointer:
		shape = shapePointer
	case input.CursorText:
		shape = shapeText
	case input.CursorResizeEW:
		shape = shapeEWResize
	case input.CursorResizeNS:
		shape = shapeNSResize
	case input.CursorNotAllowed:
		shape = shapeNotAllowed
	}
	s.shape.Send(1, s.enterSerial, shape) // set_shape
}

// ---- keyboard ----

func (s *seat) keyboardEvent(op uint16, d *wl.Decoder) {
	switch op {
	case 0: // keymap(format, fd, size)
		format := d.Uint()
		fd := d.FD()
		size := int(d.Uint())
		if fd < 0 {
			return
		}
		defer unix.Close(fd)
		if format != 1 || size <= 0 { // xkb_v1
			return
		}
		mem, err := unix.Mmap(fd, 0, size, unix.PROT_READ, unix.MAP_PRIVATE)
		if err != nil {
			return
		}
		text := mem
		if n := len(text); n > 0 && text[n-1] == 0 {
			text = text[:n-1]
		}
		s.kb.setKeymap(text)
		unix.Munmap(mem)
	case 1: // enter(serial, surface, keys)
		s.serial = d.Uint()
		s.keyOn = s.windowOf(d.Uint())
		if w := s.keyOn; w != nil {
			w.emit(platform.Event{Kind: platform.EventFocus})
		}
	case 2: // leave(serial, surface)
		s.stopRepeat()
		if w := s.keyOn; w != nil {
			w.emit(platform.Event{Kind: platform.EventBlur})
		}
		s.keyOn = nil
	case 3: // key(serial, time, key, state)
		s.serial = d.Uint()
		d.Uint()
		code, state := d.Uint(), d.Uint()
		if w := s.keyOn; w != nil {
			s.key(w, code, state == 1)
		}
	case 4: // modifiers(serial, depressed, latched, locked, group)
		d.Uint()
		s.kb.updateMods(d.Uint(), d.Uint(), d.Uint(), d.Uint())
	case 5: // repeat_info(rate, delay)
		s.repeatRate = d.Int()
		s.repeatDelay = time.Duration(d.Int()) * time.Millisecond
	}
}

func (s *seat) key(w *window, code uint32, pressed bool) {
	key := s.kb.key(code)
	if !pressed {
		if code == s.repeatKey {
			s.stopRepeat()
		}
		w.emit(platform.Event{Kind: platform.EventKeyUp, Key: key})
		return
	}
	s.press(w, code, key, false)
	s.stopRepeat()
	if s.repeatRate > 0 && s.kb.repeats(code) {
		s.repeatKey = code
		s.repeatTimer = s.d.after(s.repeatDelay, func() { s.repeat(w, code, key) })
	}
}

// press reports a key press and the text it types.
func (s *seat) press(w *window, code uint32, key input.Key, repeat bool) {
	w.emit(platform.Event{Kind: platform.EventKeyDown, Key: key, Repeat: repeat})
	if text := s.kb.text(code); text != "" {
		w.emit(platform.Event{Kind: platform.EventTextInput, Text: text})
	}
}

func (s *seat) repeat(w *window, code uint32, key input.Key) {
	if s.keyOn != w || s.repeatKey != code {
		return
	}
	s.press(w, code, key, true)
	s.repeatTimer = s.d.after(time.Second/time.Duration(s.repeatRate), func() { s.repeat(w, code, key) })
}

func (s *seat) stopRepeat() {
	s.repeatTimer.stop()
	s.repeatTimer = nil
	s.repeatKey = 0
}
