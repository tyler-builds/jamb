package ui

import (
	"math"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/access"
)

// RelativeUnit is a unit of time in a relative time, such as "3 days ago": the
// minutes, hours, days, weeks, months and years that Strings.RelativeAgo and
// Strings.RelativeIn name.
type RelativeUnit int

// The units of a relative time, from the shortest to the longest.
const (
	RelativeMinute RelativeUnit = iota
	RelativeHour
	RelativeDay
	RelativeWeek
	RelativeMonth
	RelativeYear
)

// systemStrings returns the text of the system's language, read once, for FormatRelative.
var systemStrings = sync.OnceValue(SystemStrings)

// FormatRelative returns how far t is from now in the user's language, such as "3 days
// ago" for a time in the past and "in 3 days" for one in the future. A zero time is
// empty. A time within a minute of now is "now"; the day before and the day after today
// are "yesterday" and "tomorrow". Days are counted by calendar day, and a week starts at 7 days, a month at
// 30 days and a year at 365 days. The text is in the language of the system (see
// SystemStrings). A TimeAgo element uses the Strings of its window instead.
func FormatRelative(t, now time.Time) string { return formatRelative(t, now, systemStrings()) }

// formatRelative returns the text of FormatRelative in the messages of s.
func formatRelative(t, now time.Time, s *Strings) string {
	if t.IsZero() {
		return "" // no time, as formatDate has no date for a zero time
	}
	future := t.After(now)
	d := now.Sub(t).Abs()
	if d < time.Minute {
		return s.RelativeNow
	}
	// The days apart are counted from the calendar, so that a clock change does not shift
	// a day: a time on the next calendar day is "tomorrow" even when it is under 24 hours away.
	days := int(math.Round(math.Abs(dayOf(t).Sub(dayOf(now)).Hours()) / 24))
	var u RelativeUnit
	var n int
	switch {
	case d < time.Hour:
		u, n = RelativeMinute, int(d/time.Minute)
	case days == 0:
		u, n = RelativeHour, int(d/time.Hour)
	case days == 1 && future:
		return s.RelativeTomorrow
	case days == 1:
		return s.RelativeYesterday
	case days < 7:
		u, n = RelativeDay, days
	case days < 30:
		u, n = RelativeWeek, days/7
	case days < 365:
		u, n = RelativeMonth, days/30
	default:
		u, n = RelativeYear, days/365
	}
	if future {
		return s.RelativeIn(u, n)
	}
	return s.RelativeAgo(u, n)
}

// timeAgoRun is what a TimeAgo keeps between its renders, outside the signals, so that
// its timer does not render it again.
type timeAgoRun struct {
	cancel   func() // stops the pending refresh; nil when none is pending
	disposed bool   // the element left the tree
	watched  bool   // OnDispose was registered
}

// TimeAgo shows how far t is from now, as FormatRelative does, on the window's clock
// (see Context.After), and renders again as time passes: every minute under an hour,
// every hour under a day, and daily after that. Its tooltip and accessible description
// are the full date and time of t. While it is off screen it keeps no timer: it starts
// again the next time it renders, so a parent that renders it again brings it up to date.
func TimeAgo(t time.Time) *ComponentElement {
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		inst := ctx.inst
		run := Local(ctx, "run", &timeAgoRun{}).Peek()
		now := rt.now()
		if !run.watched {
			run.watched = true
			ctx.OnDispose(func() {
				run.disposed = true
				if run.cancel != nil {
					run.cancel()
					run.cancel = nil
				}
			})
		}
		if run.cancel != nil {
			run.cancel()
			run.cancel = nil
		}
		if !run.disposed {
			tick := func() {
				run.cancel = nil
				if run.disposed || !rt.drawn(inst) {
					return
				}
				inst.invalidate()
			}
			run.cancel = rt.at(now.Add(timeAgoPeriod(now.Sub(t))), tick).cancel
		}
		// The element is a piece of static text named by the relative time, and described
		// by the full date and time. The text it draws is hidden behind that name.
		full := ""
		if !t.IsZero() {
			full = FormatDate(t) + " " + FormatTime(t)
		}
		rel := formatRelative(t, now, ctx.Strings())
		return Div().Role(access.RoleStaticText).Label(rel).Description(full).Tooltip(full).Child(Text(rel))
	})
}

// relativeWord returns the word of a unit for n: the one form for 1, the many form otherwise.
func relativeWord(words [2]string, n int) string {
	if n == 1 {
		return words[0]
	}
	return words[1]
}

// timeAgoPeriod returns how often a TimeAgo that is age from its time renders again.
func timeAgoPeriod(age time.Duration) time.Duration {
	switch a := age.Abs(); {
	case a < time.Hour:
		return time.Minute
	case a < 24*time.Hour:
		return time.Hour
	}
	return 24 * time.Hour
}
