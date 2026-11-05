//go:build linux

// Package linux is the Linux platform adapter, for Wayland compositors. It is
// pure Go: it speaks the Wayland protocol itself (see package wl), loads
// libxkbcommon at run time for keyboard layouts, and presents frames that the
// CPU renderer (render/soft) draws into shared memory.
//
// One goroutine, locked to its thread, runs the event loop for every window
// of the process, as on Windows.
package linux

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
	"golang.org/x/sys/unix"
)

// display is the connection and the globals every window shares.
type display struct {
	conn       *wl.Conn
	registry   *wl.Proxy
	compositor *wl.Proxy
	shm        *wl.Proxy
	wmBase     *wl.Proxy
	decoration *wl.Proxy   // zxdg_decoration_manager_v1, or nil
	cursor     *wl.Proxy   // wp_cursor_shape_manager_v1, or nil
	fractional *wl.Proxy   // wp_fractional_scale_manager_v1, or nil
	viewporter *wl.Proxy   // wp_viewporter, or nil
	sessions   *wl.Proxy   // xdg_session_manager_v1, or nil
	session    *winSession // the window session under jamb dev (see session.go), or nil
	seat       *seat
	clip       *clipboard // nil without wl_data_device_manager

	windows []*window
	timers  []*timer
}

// The state of the UI thread.
var (
	disp *display
	// uiThread is the thread id of the loop, or 0 before Run.
	uiThread atomic.Int32
	// wakeFD is an eventfd that Post and Wake write to wake the loop.
	wakeFD        = -1
	quitRequested bool

	postMu sync.Mutex
	posted []func()
)

// Run connects to the compositor and runs the event loop on the calling
// goroutine, which becomes the UI thread, until Quit is called. It calls
// start on the UI thread before the loop runs; start typically opens the
// first window. An error from start is returned without running the loop.
func Run(start func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	d, err := connect()
	if err != nil {
		return err
	}
	defer d.conn.Close()
	fd, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		return fmt.Errorf("platform/linux: eventfd: %w", err)
	}
	defer unix.Close(fd)
	wakeFD = fd
	disp = d
	uiThread.Store(int32(unix.Gettid()))
	defer func() {
		uiThread.Store(0)
		disp = nil
	}()

	// Settings changes arrive from the session bus; without one, the
	// defaults stay.
	go watchSettings()

	if err := start(); err != nil {
		return err
	}
	return d.loop()
}

// connect opens the connection and binds the globals.
func connect() (*display, error) {
	c, err := wl.Connect()
	if err != nil {
		return nil, fmt.Errorf("platform/linux: no Wayland compositor: %w", err)
	}
	d := &display{conn: c}
	type global struct {
		name    uint32
		version uint32
	}
	globals := map[string]global{}
	d.registry = c.NewProxy("wl_registry", 1, func(op uint16, dec *wl.Decoder) {
		if op == 0 { // global(name, interface, version)
			name := dec.Uint()
			iface := dec.String()
			globals[iface] = global{name, dec.Uint()}
		}
	})
	c.Display.Send(1, wl.NewID{P: d.registry}) // get_registry
	if err := c.Roundtrip(); err != nil {
		c.Close()
		return nil, err
	}
	bind := func(iface string, min, max uint32, on wl.Handler) *wl.Proxy {
		g, ok := globals[iface]
		if !ok || g.version < min {
			return nil
		}
		return c.Bind(d.registry, g.name, iface, minU(g.version, max), on)
	}
	d.compositor = bind("wl_compositor", 4, 6, nil)
	d.shm = bind("wl_shm", 1, 1, nil)
	d.wmBase = bind("xdg_wm_base", 1, 6, nil)
	if d.compositor == nil || d.shm == nil || d.wmBase == nil {
		c.Close()
		return nil, errors.New("platform/linux: the compositor lacks wl_compositor 4, wl_shm or xdg_wm_base")
	}
	d.wmBase.On = func(op uint16, dec *wl.Decoder) {
		if op == 0 { // ping(serial)
			d.wmBase.Send(3, dec.Uint()) // pong
		}
	}
	d.decoration = bind("zxdg_decoration_manager_v1", 1, 1, nil)
	d.cursor = bind("wp_cursor_shape_manager_v1", 1, 1, nil)
	d.fractional = bind("wp_fractional_scale_manager_v1", 1, 1, nil)
	d.viewporter = bind("wp_viewporter", 1, 1, nil)
	d.sessions = bind("xdg_session_manager_v1", 1, 1, nil)
	d.openSession()
	if d.fractional != nil && d.viewporter == nil {
		d.fractional = nil // a fractional scale needs a viewport to size the buffer
	}
	if g, ok := globals["wl_seat"]; ok {
		d.seat = newSeat(d, g.name, minU(g.version, 9))
		if g, ok := globals["wl_data_device_manager"]; ok {
			d.clip = newClipboard(d, g.name, minU(g.version, 3))
			d.clip.attach(d.seat)
		}
	}
	// A second roundtrip receives the seat's capabilities and the keymap.
	if err := c.Roundtrip(); err != nil {
		c.Close()
		return nil, err
	}
	return d, nil
}

func minU(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// loop dispatches the compositor's events, posted functions and timers, and
// renders the windows that want a frame. A window renders when the compositor
// is ready for its next frame (its frame callback is done), so frames follow
// the display's refresh. When nothing is due, the loop sleeps in poll, so an
// idle application uses no CPU.
func (d *display) loop() error {
	fds := []unix.PollFd{{Fd: int32(d.conn.FD()), Events: unix.POLLIN}, {Fd: int32(wakeFD), Events: unix.POLLIN}}
	for !quitRequested {
		runPosted()
		d.runTimers()
		if quitRequested {
			break
		}
		for _, w := range slices.Clone(d.windows) {
			if w.frameDue() {
				w.render()
			}
		}
		if err := d.conn.Flush(); err != nil {
			return err
		}
		timeout := -1
		if t, ok := d.nextTimer(); ok {
			timeout = max(int(time.Until(t).Milliseconds()), 0)
		}
		if anyPosted() {
			timeout = 0
		}
		fds[0].Revents, fds[1].Revents = 0, 0
		if _, err := unix.Poll(fds, timeout); err != nil && err != unix.EINTR {
			return fmt.Errorf("platform/linux: poll: %w", err)
		}
		if fds[1].Revents&unix.POLLIN != 0 {
			var b [8]byte
			unix.Read(wakeFD, b[:])
		}
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			if err := d.conn.Read(false); err != nil {
				return err
			}
		}
		if err := d.conn.Err(); err != nil {
			return err
		}
	}
	return d.conn.Flush()
}

// Quit ends the loop when it next runs. It is safe from any goroutine; the
// windows must already be closed (or their OnDestroy has run).
func Quit() {
	Post(func() { quitRequested = true })
}

// OnUIThread reports whether the caller runs on the UI thread. The loop's
// goroutine is locked to its thread, so no other goroutine runs there.
func OnUIThread() bool {
	t := uiThread.Load()
	return t != 0 && t == int32(unix.Gettid())
}

// Post runs fn on the UI thread, in order with the other posted functions. It
// is safe from any goroutine and wakes the loop if it is idle.
func Post(fn func()) {
	postMu.Lock()
	posted = append(posted, fn)
	postMu.Unlock()
	signalLoop()
}

// signalLoop wakes the loop from poll.
func signalLoop() {
	if fd := wakeFD; fd >= 0 {
		var one = [8]byte{1}
		unix.Write(fd, one[:])
	}
}

func anyPosted() bool {
	postMu.Lock()
	defer postMu.Unlock()
	return len(posted) > 0
}

func runPosted() {
	postMu.Lock()
	fns := posted
	posted = nil
	postMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// timer runs fn at the given time on the UI thread, unless stopped.
type timer struct {
	at      time.Time
	fn      func()
	stopped bool
}

// after schedules fn after dur. UI thread only.
func (d *display) after(dur time.Duration, fn func()) *timer {
	t := &timer{at: time.Now().Add(dur), fn: fn}
	d.timers = append(d.timers, t)
	return t
}

func (t *timer) stop() {
	if t != nil {
		t.stopped = true
	}
}

func (d *display) nextTimer() (time.Time, bool) {
	var next time.Time
	for _, t := range d.timers {
		if !t.stopped && (next.IsZero() || t.at.Before(next)) {
			next = t.at
		}
	}
	return next, !next.IsZero()
}

func (d *display) runTimers() {
	now := time.Now()
	var due []*timer
	keep := d.timers[:0]
	for _, t := range d.timers {
		switch {
		case t.stopped:
		case !t.at.After(now):
			due = append(due, t)
		default:
			keep = append(keep, t)
		}
	}
	clear(d.timers[len(keep):])
	d.timers = keep
	for _, t := range due {
		if !t.stopped {
			t.fn()
		}
	}
}

// Clock returns the monotonic clock in nanoseconds, the clock of
// Event.Arrived and of frame statistics.
func Clock() int64 {
	var ts unix.Timespec
	unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	return ts.Nano()
}
