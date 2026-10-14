package ui

import (
	"slices"
	"time"
)

// timer is a function that a runtime runs on the UI thread at the first frame at or
// after its time. See Runtime.at.
type timer struct {
	rt *Runtime
	at time.Time
	fn func()
}

// at schedules fn to run at the first frame at or after t, and asks the host to wake the
// window then. It returns the timer, which cancel stops.
func (rt *Runtime) at(t time.Time, fn func()) *timer {
	tm := &timer{rt: rt, at: t, fn: fn}
	rt.timers = append(rt.timers, tm)
	rt.wakeAt(t)
	return tm
}

// cancel stops the timer if it has not run. It is safe on a nil timer and on one that ran.
func (tm *timer) cancel() {
	if tm == nil {
		return
	}
	tm.rt.timers = slices.DeleteFunc(tm.rt.timers, func(o *timer) bool { return o == tm })
}

// runTimers runs the timers that are due at now, in the order they were scheduled. With
// force, it runs all of them, so that a window that is closing saves what is pending.
func (rt *Runtime) runTimers(now time.Time, force bool) {
	for {
		var due *timer
		for _, tm := range rt.timers {
			if force || !now.Before(tm.at) {
				due = tm
				break
			}
		}
		if due == nil {
			return
		}
		rt.timers = slices.DeleteFunc(rt.timers, func(o *timer) bool { return o == due })
		due.fn()
	}
}
