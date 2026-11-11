package app

import (
	"time"
)

// The windows' clock, and the control channel's clock op, which lets an agent
// drive time instead of sleeping (after Expo's Exact): `jamb ctl clock +500`
// moves the clock half a second forward, running the frames of that half
// second, and `jamb ctl clock settle` moves it until the UI has come to rest.
//
// The clock starts live, following the wall clock. The first op that moves
// it, or `clock seek`, puts it in seek mode: it stands still between the
// agent's requests, so timers, tooltips, motion, the caret and frames wait for
// the agent, and a screenshot or a tree read between requests shows the same
// moment. `clock live` gives time back to the wall clock.
//
// There is one clock for the app, not one for each window, as one page has one
// in Exact: windows share state, and a timer in one window that changes what
// another shows must not run ahead of it or behind it.

// uiClock is the clock of the windows' runtimes and of the frames the window
// hosts run. Every runtime reads it (see host.OnCreate). UI thread only.
type uiClock struct {
	seek bool
	at   time.Time // the time while seeking
	// offset is added to the wall clock while live: going live after the agent
	// moved the clock ahead of the wall clock keeps it from stepping back,
	// which would leave what it scheduled waiting for the wall clock to catch up.
	offset time.Duration
	wall   func() time.Time // time.Now when nil; tests replace it
}

// now returns the clock's time.
func (c *uiClock) now() time.Time {
	if c.seek {
		return c.at
	}
	return c.wallNow().Add(c.offset)
}

func (c *uiClock) wallNow() time.Time {
	if c.wall != nil {
		return c.wall()
	}
	return time.Now()
}

// wakeTime returns the wall clock's time for t, a time on this clock that a
// runtime asks to be woken at, and false while seeking, when only the agent
// moves the clock and a wake-up would find nothing due.
func (c *uiClock) wakeTime(t time.Time) (time.Time, bool) {
	if c.seek {
		return time.Time{}, false
	}
	return t.Add(-c.offset), true
}

// seekNow puts the clock in seek mode, standing still at its current time.
func (c *uiClock) seekNow() {
	if !c.seek {
		c.at, c.seek = c.now(), true
	}
}

// goLive gives the clock back to the wall clock, from where seeking left it
// or from the wall clock's time, whichever is later.
func (c *uiClock) goLive() {
	if !c.seek {
		return
	}
	if d := c.at.Sub(c.wallNow()); d > c.offset {
		c.offset = d
	}
	c.seek = false
}

// Limits of the clock op.
const (
	ctlClockStep     = 16 * time.Millisecond // a frame, as uitest.Advance runs them
	ctlClockMax      = time.Hour             // the most that one advance moves the clock
	ctlClockSettleBy = 10 * time.Second      // how long settle advances before it gives up, as uitest.Settle
)

// ctlClock is the clock op's answer.
type ctlClock struct {
	Mode string `json:"mode"` // live or seek
	// Now is the clock's time, in RFC 3339 with milliseconds.
	Now string `json:"now"`
	// ElapsedMS is how far an advance or a settle moved the clock.
	ElapsedMS *int64 `json:"elapsed_ms,omitempty"`
	// Settled reports, for settle, whether the UI came to rest within
	// ctlClockSettleBy. It is false when something kept animating or a timer
	// kept coming due, as one that repeats does; the clock has then moved by
	// the whole limit.
	Settled *bool `json:"settled,omitempty"`
	// NextTimerMS is how long it is until the earliest timer of any window,
	// such as one of Context.After, comes due, when one is pending.
	NextTimerMS *int64 `json:"next_timer_ms,omitempty"`
}

// opClock reads or drives the app's clock: advance_ms moves it forward,
// settle moves it until the UI comes to rest, mode switches between live and
// seek, and none of them reports it. Moving the clock puts it in seek mode.
func (c *control) opClock(req ctlRequest, reply ctlReply) {
	set := 0
	for _, on := range []bool{req.AdvanceMS != 0, req.Settle, req.Mode != ""} {
		if on {
			set++
		}
	}
	switch {
	case set > 1:
		reply(nil, ctlErr("bad_request", "clock takes one of advance_ms, settle and mode"))
		return
	case req.AdvanceMS < 0:
		reply(nil, ctlErr("bad_request", "advance_ms must be positive: the clock does not go back"))
		return
	case time.Duration(req.AdvanceMS)*time.Millisecond > ctlClockMax:
		reply(nil, ctlErr("bad_request", "advance_ms must be at most %d", ctlClockMax.Milliseconds()))
		return
	case req.Mode != "" && req.Mode != "live" && req.Mode != "seek":
		reply(nil, ctlErr("bad_request", "mode must be live or seek"))
		return
	}
	clk := c.clock
	out := ctlClock{}
	switch {
	case req.AdvanceMS > 0:
		clk.seekNow()
		d := time.Duration(req.AdvanceMS) * time.Millisecond
		c.advanceClock(d)
		ms := d.Milliseconds()
		out.ElapsedMS = &ms
	case req.Settle:
		clk.seekNow()
		d, ok := c.settleClock()
		ms := d.Milliseconds()
		out.ElapsedMS, out.Settled = &ms, &ok
	case req.Mode == "seek":
		clk.seekNow()
	case req.Mode == "live":
		if clk.seek {
			clk.goLive()
			// Animations that stood still go on, and the timers scheduled while
			// seeking, which asked the platform for no wake-up, get one.
			for _, w := range c.windows() {
				w.wake()
				if t, ok := hostOf(w.rt).NextTimer(); ok && w.wakeAt != nil {
					w.wakeAt(t)
				}
			}
		}
	}
	out.Mode = "live"
	if clk.seek {
		out.Mode = "seek"
	}
	now := clk.now()
	out.Now = now.Format("2006-01-02T15:04:05.000Z07:00")
	if next, ok := c.nextTimer(); ok {
		ms := max(next.Sub(now), 0).Milliseconds()
		out.NextTimerMS = &ms
	}
	reply(out, nil)
}

// advanceClock moves the seeking clock forward by d, running an undrawn frame
// of every window at every ctlClockStep of it, as uitest.Advance does, so that
// transitions, animations, timers, tooltip delays and toasts move as they
// would in that time. The visible windows then draw the moment it reached.
func (c *control) advanceClock(d time.Duration) {
	for d > 0 {
		c.stepClock(min(d, ctlClockStep))
		d -= ctlClockStep
	}
	c.showClock()
}

// settleClock moves the seeking clock forward a frame at a time until every
// window has come to rest (see ui.Runtime.Settled) with no timer due within
// ctlClockSettleBy, as a debounce is, or until it has moved by
// ctlClockSettleBy. It returns how far the clock moved and whether the windows
// came to rest. Repeating animations never end and are ignored, as
// uitest.Settle ignores them; a timer due later than ctlClockSettleBy from
// now, such as a query's cache expiry, is left pending, but one that repeats
// sooner keeps the windows from coming to rest.
func (c *control) settleClock() (time.Duration, bool) {
	var moved time.Duration
	defer c.showClock()
	for {
		if c.atRest(c.clock.now().Add(ctlClockSettleBy)) {
			return moved, true
		}
		if moved >= ctlClockSettleBy {
			return moved, false
		}
		c.stepClock(ctlClockStep)
		moved += ctlClockStep
	}
}

// atRest reports whether every window has come to rest, with no timer due by deadline.
func (c *control) atRest(deadline time.Time) bool {
	for _, w := range c.windows() {
		if !w.rt.Settled() {
			return false
		}
		if t, ok := hostOf(w.rt).NextTimer(); ok && !t.After(deadline) {
			return false
		}
	}
	return true
}

// atRestNow reports whether window w has settled (see ui.Runtime.Settled), or,
// while the clock seeks, whether a frame at the clock's time would show
// nothing new: an animation then moves only when the agent moves the clock, so
// a wait for idle or a click does not wait for it.
func (c *control) atRestNow(w *ctlWin) bool {
	if c.clock != nil && c.clock.seek {
		return !hostOf(w.rt).Changed()
	}
	return w.rt.Settled()
}

// stepClock moves the seeking clock forward by d and runs a frame of every
// window at the new time, without drawing it. Requests waiting for a window's
// frames see each frame, as they see one that the window draws.
func (c *control) stepClock(d time.Duration) {
	c.clock.at = c.clock.at.Add(d)
	for i, w := range c.windows() {
		w.offscreen()
		c.frame(i+1, w)
	}
}

// showClock has the visible windows draw the moment that the clock reached.
func (c *control) showClock() {
	for _, w := range c.windows() {
		if w.visible {
			w.redraw()
		}
	}
}

// nextTimer returns the time of the earliest timer of any window.
func (c *control) nextTimer() (time.Time, bool) {
	var next time.Time
	for _, w := range c.windows() {
		if t, ok := hostOf(w.rt).NextTimer(); ok && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}
	return next, !next.IsZero()
}
