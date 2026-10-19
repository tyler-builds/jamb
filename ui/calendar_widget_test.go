package ui

import (
	"reflect"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// calRig is an access rig for a calendar on the page, with the en-US locale and today as
// Monday, October 12, 2026. heard holds the announcements since the last call of announced.
type calRig struct {
	*accessRig
	heard []string
}

// newCalRig builds root in a rig with the en-US locale and today as testNow.
func newCalRig(t *testing.T, root func(*Context) Element) *calRig {
	t.Helper()
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	// The calendar takes today from the first frame, so the clock is set before it.
	a := &accessRig{now: testNow}
	a.rt = NewRuntime(root, DarkTheme(), nil)
	a.rt.clock = func() time.Time { return a.now }
	a.rt.SetWarnFunc(func(s string) { a.warnings = append(a.warnings, s) })
	a.rt.SetAccessWarnings(true)
	a.rt.SetViewport(500, 400, 1)
	a.rt.Frame(a.now)
	a.bridge = a.rt.AccessBridge()
	a.bridge.Activate()
	r := &calRig{accessRig: a}
	r.frame()
	r.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			r.heard = append(r.heard, a.Text)
		}
	})
	return r
}

// announced returns the announcements heard since the last call.
func (r *calRig) announced() []string {
	r.frame()
	for _, a := range takeAnnouncements() {
		r.heard = append(r.heard, a.Text)
	}
	h := r.heard
	r.heard = nil
	return h
}

// focusedName returns the name of the node that has keyboard focus, as the tree gives it.
func (r *calRig) focusedName() string {
	tr := r.tree()
	if n := tr.Node(tr.Focus); n != nil {
		return n.Name
	}
	return ""
}

// focusCal gives the calendar keyed key keyboard focus, as Tab would.
func (r *calRig) focusCal(t *testing.T, key string) {
	t.Helper()
	focusNode(r.accessRig, r.keyed(t, key))
}

// clickDay clicks the day named name.
func (r *calRig) clickDay(t *testing.T, name string) {
	t.Helper()
	rc := r.rt.nodeRect(accessNode(t, r.accessRig, access.RoleDataItem, name))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// testCalendarParts sets a distinctive style on each part of the calendar, puts the state of the
// part in effect, and asserts that the style reaches the node that draws it. Today is Monday,
// October 12, 2026, the value is October 13, and a range is October 13 to 17.
func testCalendarParts(t *testing.T, applied map[string]bool) {
	type calCase struct {
		part     string
		set      func(*CalendarStyle, Style)
		disabled bool
		rng      bool
		run      func(t *testing.T, r *accessRig)
	}
	dayIs := func(name, part string) func(t *testing.T, r *accessRig) {
		return func(t *testing.T, r *accessRig) {
			wantBackground(t, accessNode(t, r, access.RoleDataItem, name), part, markBG)
		}
	}
	cases := []calCase{
		{part: "Calendar.Root", set: func(s *CalendarStyle, m Style) { s.Root = m },
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "Calendar.Root", ""), "Calendar.Root", markBG)
			}},
		{part: "Calendar.RootDisabled", set: func(s *CalendarStyle, m Style) { s.RootDisabled = m }, disabled: true,
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "Calendar.Root", ""), "Calendar.RootDisabled", markBG)
			}},
		{part: "Calendar.Header", set: func(s *CalendarStyle, m Style) { s.Header = m },
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "Calendar.Header", ""), "Calendar.Header", markBG)
			}},
		{part: "Calendar.NavButton", set: func(s *CalendarStyle, m Style) { s.NavButton = m },
			run: func(t *testing.T, r *accessRig) { wantBackgroundAll(t, r, "Calendar.NavButton") }},
		{part: "Calendar.WeekdayLabel", set: func(s *CalendarStyle, m Style) { s.WeekdayLabel = m },
			run: func(t *testing.T, r *accessRig) { wantBackgroundAll(t, r, "Calendar.WeekdayLabel") }},
		{part: "Calendar.Day", set: func(s *CalendarStyle, m Style) { s.Day = m },
			run: dayIs("Thursday, October 15, 2026", "Calendar.Day")},
		{part: "Calendar.DayHover", set: func(s *CalendarStyle, m Style) { s.DayHover = m },
			run: func(t *testing.T, r *accessRig) {
				hoverNode(r, accessNode(t, r, access.RoleDataItem, "Thursday, October 15, 2026"))
				dayIs("Thursday, October 15, 2026", "Calendar.DayHover")(t, r)
			}},
		{part: "Calendar.DaySelected", set: func(s *CalendarStyle, m Style) { s.DaySelected = m },
			run: dayIs("Tuesday, October 13, 2026", "Calendar.DaySelected")},
		{part: "Calendar.DayInRange", set: func(s *CalendarStyle, m Style) { s.DayInRange = m }, rng: true,
			run: dayIs("Thursday, October 15, 2026", "Calendar.DayInRange")},
		{part: "Calendar.DayToday", set: func(s *CalendarStyle, m Style) { s.DayToday = m },
			run: dayIs("Monday, October 12, 2026", "Calendar.DayToday")},
		{part: "Calendar.DayDisabled", set: func(s *CalendarStyle, m Style) { s.DayDisabled = m },
			run: dayIs("Saturday, October 17, 2026", "Calendar.DayDisabled")},
		{part: "Calendar.DayOutsideMonth", set: func(s *CalendarStyle, m Style) { s.DayOutsideMonth = m },
			run: dayIs("Sunday, September 27, 2026", "Calendar.DayOutsideMonth")},
		{part: "Calendar.DayFocusVisible", set: func(s *CalendarStyle, m Style) { s.DayFocusVisible = m },
			run: func(t *testing.T, r *accessRig) {
				focusNode(r, partNode(t, r.rt, "Calendar.Root", ""))
				dayIs("Tuesday, October 13, 2026", "Calendar.DayFocusVisible")(t, r)
			}},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st CalendarStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			off := func(d time.Time) bool { return d.Day() == 17 && d.Month() == time.October }
			if c.part != "Calendar.DayDisabled" {
				off = nil
			}
			r := newCalRig(t, func(*Context) Element {
				if c.rng {
					return Div().Column().Child(CalendarRange(NewSignal(day(2026, time.October, 13)),
						NewSignal(day(2026, time.October, 17))).Styles(st))
				}
				return Div().Column().Child(Calendar(NewSignal(day(2026, time.October, 13))).
					DisableDays(off).Disabled(c.disabled).Styles(st))
			})
			c.run(t, r.accessRig)
		})
	}
}

func TestCalendarIsAFocusableGridOfTheMonth(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(Calendar(v).Key("cal").Label("Due date"))
	})
	n := r.stateOf(t, access.RoleCalendar, "Due date")
	accesstest.Check(t, n, access.RoleCalendar, "Due date", access.StateFocusable|access.StateSelectionContainer)
	if n.GridRows != 6 || n.GridColumns != 7 {
		t.Fatalf("calendar grid is %dx%d, want 6x7", n.GridRows, n.GridColumns)
	}
	if !r.stateOf(t, access.RoleDataItem, "Tuesday, October 13, 2026").States.Has(access.StateSelected) {
		t.Fatal("the chosen day is not selected")
	}
	if d := r.stateOf(t, access.RoleDataItem, "Monday, October 12, 2026").Description; d != "Today" {
		t.Fatalf("today's description is %q", d)
	}
	r.focusCal(t, "cal")
	if got := r.focusedName(); got != "Tuesday, October 13, 2026" {
		t.Fatalf("focus on the calendar reads %q, want the chosen day", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCalendarWithoutValueShowsTodayAndIsNamedByDefault(t *testing.T) {
	v := NewSignal(time.Time{})
	r := newCalRig(t, func(*Context) Element { return Div().Column().Child(Calendar(v).Key("cal")) })
	r.stateOf(t, access.RoleCalendar, "Date")
	r.focusCal(t, "cal")
	if got := r.focusedName(); got != "Monday, October 12, 2026" {
		t.Fatalf("with no date, focus is on %q, want today", got)
	}
}

func TestCalendarKeysMoveAndEnterChooses(t *testing.T) {
	v := NewSignal(time.Date(2026, time.October, 13, 14, 30, 0, 0, time.Local))
	var changes []time.Time
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(Calendar(v).Key("cal").Label("Due").OnChange(func(d time.Time) {
			if !d.Equal(v.Peek()) {
				t.Errorf("OnChange(%v) ran before the signal was set", d)
			}
			changes = append(changes, d)
		}))
	})
	r.focusCal(t, "cal")
	steps := []struct {
		key  input.Key
		mods input.Modifiers
		want string
	}{
		{input.KeyRight, 0, "Wednesday, October 14, 2026"},
		{input.KeyDown, 0, "Wednesday, October 21, 2026"},
		{input.KeyHome, 0, "Sunday, October 18, 2026"},
		{input.KeyEnd, 0, "Saturday, October 24, 2026"},
		{input.KeyPageDown, 0, "Tuesday, November 24, 2026"},
		{input.KeyPageUp, input.ModShift, "Monday, November 24, 2025"},
		{input.KeyPageDown, input.ModShift, "Tuesday, November 24, 2026"},
	}
	for _, s := range steps {
		r.press(s.key, s.mods)
		if got := r.focusedName(); got != s.want {
			t.Fatalf("after key %v (mods %v) focus is on %q, want %q", s.key, s.mods, got, s.want)
		}
	}
	if len(changes) != 0 {
		t.Fatal("moving with the keys changed the value")
	}
	r.press(input.KeyEnter, 0)
	want := time.Date(2026, time.November, 24, 14, 30, 0, 0, time.Local)
	if !v.Peek().Equal(want) || len(changes) != 1 {
		t.Fatalf("Enter gave %v (OnChange %d times), want %v once; the time of day is kept", v.Peek(), len(changes), want)
	}
	if !r.stateOf(t, access.RoleDataItem, "Tuesday, November 24, 2026").States.Has(access.StateSelected) {
		t.Fatal("the chosen day is not selected after Enter")
	}
	// Space on the chosen day changes nothing.
	r.press(input.KeySpace, 0)
	if len(changes) != 1 {
		t.Fatal("choosing the chosen day again ran OnChange")
	}
}

func TestCalendarAnnouncesTheMonth(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element { return Div().Column().Child(Calendar(v).Key("cal")) })
	r.focusCal(t, "cal")
	r.announced()
	r.press(input.KeyPageDown, 0)
	found := false
	for _, s := range r.announced() {
		found = found || s == "November 2026"
	}
	if !found {
		t.Fatal("moving to November did not announce the month")
	}
}

// TestCalendarLeavesEscapeAndTabToThePage checks that the keys a picker's calendar takes to close
// reach the elements around a calendar on the page.
func TestCalendarLeavesEscapeAndTabToThePage(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	var seen []input.Key
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().OnKeyDown(func(e *KeyEvent) { seen = append(seen, e.Key) }).
			Child(Calendar(v).Key("cal")).
			Child(Button("After").Key("after"))
	})
	r.focusCal(t, "cal")
	r.press(input.KeyEscape, 0)
	if len(seen) != 1 || seen[0] != input.KeyEscape {
		t.Fatalf("the page saw %v, want Escape", seen)
	}
	r.press(input.KeyTab, 0)
	if r.focusedKey() != "after" {
		t.Fatalf("Tab moved focus to %q, want the next element", r.focusedKey())
	}
}

func TestCalendarClickAndNavButtons(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element { return Div().Column().Child(Calendar(v).Key("cal")) })
	r.clickDay(t, "Thursday, October 22, 2026")
	if !v.Peek().Equal(day(2026, time.October, 22)) {
		t.Fatalf("a click gave %v", v.Peek())
	}
	if r.focusedKey() != "cal" {
		t.Fatalf("a click left focus on %q, want the calendar", r.focusedKey())
	}
	next := accessNode(t, r.accessRig, access.RoleButton, "Next month")
	rc := r.rt.nodeRect(next)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if !nodeTextContains(r.rt, "November 2026") {
		t.Fatal("Next month did not show November")
	}
	// A day of the month that the calendar shows, and the value set from outside brings its
	// month back.
	v.Set(day(2027, time.March, 3))
	r.frame()
	if !nodeTextContains(r.rt, "March 2027") {
		t.Fatal("a value set from outside does not show its month")
	}
}

func TestCalendarMinMaxAndDisabledDays(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(Calendar(v).Key("cal").Min(day(2026, time.October, 5)).
			Max(day(2026, time.October, 25)).DisableDays(weekend))
	})
	for _, name := range []string{"Saturday, October 10, 2026", "Sunday, October 4, 2026", "Monday, October 26, 2026"} {
		if !r.stateOf(t, access.RoleDataItem, name).States.Has(access.StateDisabled) {
			t.Fatalf("%s is not disabled", name)
		}
	}
	if r.stateOf(t, access.RoleDataItem, "Monday, October 12, 2026").States.Has(access.StateDisabled) {
		t.Fatal("an enabled day is disabled")
	}
	r.clickDay(t, "Saturday, October 10, 2026")
	r.focusCal(t, "cal")
	for range 3 {
		r.press(input.KeyLeft, 0)
	}
	r.press(input.KeyEnter, 0)
	if !v.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatalf("a disabled day was chosen: %v", v.Peek())
	}
}

func TestCalendarFirstDay(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(Calendar(v).Key("cal").FirstDay(time.Monday))
	})
	r.focusCal(t, "cal")
	r.press(input.KeyHome, 0)
	if got := r.focusedName(); got != "Monday, October 12, 2026" {
		t.Fatalf("Home in a Monday-first week is %q", got)
	}
}

func TestCalendarDisabled(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(Calendar(v).Key("cal").Label("Due").Disabled(true))
	})
	n := r.stateOf(t, access.RoleCalendar, "Due")
	if !n.States.Has(access.StateDisabled) || n.States.Has(access.StateFocusable) {
		t.Fatalf("a disabled calendar has states [%s]", n.States)
	}
	if !r.stateOf(t, access.RoleButton, "Next month").States.Has(access.StateDisabled) {
		t.Fatal("the navigation buttons of a disabled calendar are not disabled")
	}
	r.clickDay(t, "Thursday, October 22, 2026")
	if !v.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatalf("a click changed a disabled calendar to %v", v.Peek())
	}
	if o := keyNode(t, r.rt, "cal").style.Opacity; o != 0.5 {
		t.Fatalf("a disabled calendar has opacity %v, want 0.5", o)
	}
}

// TestCalendarRingFollowsKeyboardFocus checks that the day the keys have reached is outlined only
// while the calendar has keyboard focus.
func TestCalendarRingFollowsKeyboardFocus(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(Calendar(v).Key("cal").Styles(CalendarStyle{DayFocusVisible: Style{Background: markBG}}))
	})
	cell := func() *node { return accessNode(t, r.accessRig, access.RoleDataItem, "Tuesday, October 13, 2026") }
	if cell().style.Background == markBG {
		t.Fatal("the keys' day is outlined while the calendar has no focus")
	}
	r.focusCal(t, "cal")
	wantBackground(t, cell(), "Calendar.DayFocusVisible", markBG)
}

func TestCalendarRangeChoosesInTwoSteps(t *testing.T) {
	from, to := NewSignal(time.Time{}), NewSignal(time.Time{})
	var got [][2]time.Time
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().Child(CalendarRange(from, to).Key("cal").Label("Trip").OnChange(func(a, b time.Time) {
			got = append(got, [2]time.Time{a, b})
		}))
	})
	r.stateOf(t, access.RoleCalendar, "Trip")
	r.clickDay(t, "Thursday, October 22, 2026")
	if !from.Peek().IsZero() || len(got) != 0 {
		t.Fatal("the first day changed the range")
	}
	heard := r.announced()
	if len(heard) == 0 || heard[len(heard)-1] != "Start of range chosen. Choose the end of the range." {
		t.Fatalf("the start was announced as %q", heard)
	}
	// The second day is before the first: the two are swapped.
	r.clickDay(t, "Monday, October 19, 2026")
	if !from.Peek().Equal(day(2026, time.October, 19)) || !to.Peek().Equal(day(2026, time.October, 22)) || len(got) != 1 {
		t.Fatalf("the range is %v to %v (OnChange %d times)", from.Peek(), to.Peek(), len(got))
	}
	if d := r.stateOf(t, access.RoleDataItem, "Tuesday, October 20, 2026").Description; d != "In range" {
		t.Fatalf("a day in the range is described %q", d)
	}
	// Escape drops a start chosen without its end, by the keys.
	r.focusCal(t, "cal")
	r.press(input.KeyEnter, 0)
	r.press(input.KeyEscape, 0)
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0)
	if len(got) != 1 {
		t.Fatal("Escape did not drop the pending start")
	}
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0)
	if len(got) != 2 || !from.Peek().Equal(day(2026, time.October, 20)) || !to.Peek().Equal(day(2026, time.October, 21)) {
		t.Fatalf("after Escape the range is %v to %v", from.Peek(), to.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// TestCalendarRangeEscapeWithoutStartReachesThePage checks that Escape is left to the page when no
// start is pending.
func TestCalendarRangeEscapeWithoutStartReachesThePage(t *testing.T) {
	from, to := NewSignal(time.Time{}), NewSignal(time.Time{})
	escapes := 0
	r := newCalRig(t, func(*Context) Element {
		return Div().Column().OnKeyDown(func(e *KeyEvent) {
			if e.Key == input.KeyEscape {
				escapes++
			}
		}).Child(CalendarRange(from, to).Key("cal"))
	})
	r.focusCal(t, "cal")
	r.press(input.KeyEscape, 0)
	if escapes != 1 {
		t.Fatalf("the page saw Escape %d times, want 1", escapes)
	}
}

func TestCalendarUnderHighContrast(t *testing.T) {
	v := NewSignal(day(2026, time.October, 13))
	r := newCalRig(t, func(*Context) Element { return Div().Column().Child(Calendar(v).Key("cal").Disabled(true)) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	if o := keyNode(t, r.rt, "cal").style.Opacity; o != 1 {
		t.Fatalf("high contrast: a disabled calendar has opacity %v, want 1", o)
	}
	cell := accessNode(t, r.accessRig, access.RoleDataItem, "Thursday, October 15, 2026")
	if c := cell.style.TextColor; c != r.rt.theme.Disabled {
		t.Fatalf("high contrast: a day of a disabled calendar is %v, want GrayText %v", c, r.rt.theme.Disabled)
	}
}

// TestCalendarDefaultsMatchThePickers checks that the parts a calendar shares with the date picker
// look the same by default.
func TestCalendarDefaultsMatchThePickers(t *testing.T) {
	for _, th := range []*Theme{DarkTheme(), LightTheme(), HighContrastTheme(HighContrastBlack(), DarkTheme())} {
		c := calendarDefaults(th, 1)
		d := datePickerDefaults(th, 1, false).calendar(dateRangePickerDefaults(th, 1, false).DayInRange)
		d.Root, d.RootDisabled = c.Root, c.RootDisabled
		if !reflect.DeepEqual(c, d) {
			t.Errorf("the calendar's defaults differ from the date picker's: %+v vs %+v", c, d)
		}
	}
}
