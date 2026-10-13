package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// dateRig is an access rig with a date picker keyed "due" over value. The locale is en-US,
// and today is October 12, 2026.
type dateRig struct {
	*accessRig
	value   *Signal[time.Time]
	changes []time.Time
	heard   []string
}

func newDateRig(t *testing.T, value time.Time, cfg func(*DatePickerElement) *DatePickerElement) *dateRig {
	t.Helper()
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	dr := &dateRig{value: NewSignal(value)}
	build := func() *DatePickerElement {
		p := DatePicker(dr.value).Key("due").Label("Due date").Placeholder("MM/DD/YYYY")
		p.OnChange(func(v time.Time) { dr.changes = append(dr.changes, v) })
		if cfg != nil {
			p = cfg(p)
		}
		return p
	}
	dr.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(build())
	})
	dr.now = testNow
	dr.frame()
	dr.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			dr.heard = append(dr.heard, a.Text)
		}
	})
	return dr
}

// announced returns the announcements heard since the last call.
func (r *dateRig) announced() []string {
	for _, a := range takeAnnouncements() {
		r.heard = append(r.heard, a.Text)
	}
	h := r.heard
	r.heard = nil
	return h
}

// focusedName returns the name of the node that has keyboard focus, as the tree gives it.
func (r *dateRig) focusedName() string {
	tr := r.tree()
	if n := tr.Node(tr.Focus); n != nil {
		return n.Name
	}
	return ""
}

// expandedNow reports whether the calendar is open, from the field's access state.
func (r *dateRig) expandedNow(t *testing.T) bool {
	t.Helper()
	return r.nodeByKey(t, "due").div.acc.expanded
}

// calendarNode returns the calendar's node in the tree, or nil when it is closed.
func (r *dateRig) calendarNode() *access.Node {
	var out *access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleCalendar {
			out = n
		}
		return true
	})
	return out
}

// openDate focuses the field and opens the calendar with Alt+Down.
func openDate(t *testing.T, r *dateRig) {
	t.Helper()
	r.focusKey(t, "due")
	r.rt.keyboardModality = true
	r.press(input.KeyDown, input.ModAlt)
	if !r.expandedNow(t) {
		t.Fatal("Alt+Down did not open the calendar")
	}
}

func TestDatePickerTypedDateSetsValueOnEnter(t *testing.T) {
	r := newDateRig(t, time.Time{}, nil)
	r.focusKey(t, "due")
	r.typeText("10/20/2026")
	r.press(input.KeyEnter, 0)
	if want := day(2026, time.October, 20); !r.value.Peek().Equal(want) {
		t.Fatalf("value = %v, want %v", r.value.Peek(), want)
	}
	if len(r.changes) != 1 {
		t.Fatalf("OnChange ran %d times, want 1", len(r.changes))
	}
}

func TestDatePickerInvalidTextKeepsValueAndShowsMessage(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	r.focusKey(t, "due")
	r.clearText(t)
	r.typeText("13/45/2026")
	r.press(input.KeyEnter, 0)
	if !r.value.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatalf("an invalid text changed the value to %v", r.value.Peek())
	}
	if !r.nodeByKey(t, "due").div.acc.invalid {
		t.Fatal("the field is not reported invalid after an invalid date")
	}
	if !nodeTextContains(r.rt, "Enter a date such as 10/13/2026.") {
		t.Fatal("the message does not give an example of a date")
	}
	if len(r.changes) != 0 {
		t.Fatal("OnChange ran for an invalid text")
	}
}

func TestDatePickerBlurRevertsInvalidText(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	r.focusKey(t, "due")
	r.clearText(t)
	r.typeText("xx")
	r.rt.setFocus(0) // the field is left: the text shows the value again
	r.frame()
	if got := r.stateOf(t, access.RoleComboBox, "Due date").Value; got != "10/13/2026" {
		t.Fatalf("after leaving the field the text is %q, want the value", got)
	}
	if r.nodeByKey(t, "due").div.acc.invalid {
		t.Fatal("the field is still invalid after it was reverted")
	}
}

func TestDatePickerEmptyTextIsNoDate(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	r.focusKey(t, "due")
	r.clearText(t)
	r.press(input.KeyEnter, 0)
	if !r.value.Peek().IsZero() {
		t.Fatalf("an empty text left the value %v; it should be no date", r.value.Peek())
	}
}

func TestDatePickerAltDownOpensOnChosenDay(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 20), nil)
	openDate(t, r)
	if got := r.focusedName(); got != "Tuesday, October 20, 2026" {
		t.Fatalf("focus is on %q, want the chosen day", got)
	}
	if !r.focusedAccess(t, access.RoleDataItem, "Tuesday, October 20, 2026") {
		t.Fatal("the chosen day does not have focus")
	}
}

func TestDatePickerOpensOnTodayWithoutValue(t *testing.T) {
	r := newDateRig(t, time.Time{}, nil)
	openDate(t, r)
	if got := r.focusedName(); got != "Monday, October 12, 2026" {
		t.Fatalf("with no date, focus is on %q, want today", got)
	}
}

func TestDatePickerKeysMoveByDayWeekMonthAndYear(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	steps := []struct {
		key  input.Key
		mods input.Modifiers
		want string
	}{
		{input.KeyRight, 0, "Wednesday, October 14, 2026"},
		{input.KeyLeft, 0, "Tuesday, October 13, 2026"},
		{input.KeyUp, 0, "Tuesday, October 6, 2026"},
		{input.KeyDown, 0, "Tuesday, October 13, 2026"},
		{input.KeyHome, 0, "Sunday, October 11, 2026"},
		{input.KeyEnd, 0, "Saturday, October 17, 2026"},
		{input.KeyPageDown, 0, "Tuesday, November 17, 2026"},
		{input.KeyPageUp, 0, "Saturday, October 17, 2026"},
		{input.KeyPageDown, input.ModShift, "Sunday, October 17, 2027"},
		{input.KeyPageUp, input.ModShift, "Saturday, October 17, 2026"},
	}
	for _, s := range steps {
		r.press(s.key, s.mods)
		if got := r.focusedName(); got != s.want {
			t.Fatalf("after key %v (mods %v) focus is on %q, want %q", s.key, s.mods, got, s.want)
		}
	}
	if !r.expandedNow(t) {
		t.Fatal("moving with the keys closed the calendar")
	}
}

func TestDatePickerPageMonthEndClampsDay(t *testing.T) {
	r := newDateRig(t, day(2026, time.January, 31), nil)
	openDate(t, r)
	r.press(input.KeyPageDown, 0)
	if got := r.focusedName(); got != "Saturday, February 28, 2026" {
		t.Fatalf("Page Down from January 31 is %q, want February 28, 2026", got)
	}
}

func TestDatePickerMonthChangeIsAnnounced(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	r.announced()
	r.press(input.KeyPageDown, 0)
	r.frame()
	found := false
	for _, s := range r.announced() {
		if s == "November 2026" {
			found = true
		}
	}
	if !found {
		t.Fatal("moving to November did not announce the month")
	}
}

func TestDatePickerEnterChoosesAndKeepsTheClock(t *testing.T) {
	start := time.Date(2026, time.October, 12, 14, 30, 0, 0, time.Local)
	r := newDateRig(t, start, nil)
	openDate(t, r)
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0)
	want := time.Date(2026, time.October, 13, 14, 30, 0, 0, time.Local)
	if !r.value.Peek().Equal(want) {
		t.Fatalf("value = %v, want %v (the day changes, the time of day stays)", r.value.Peek(), want)
	}
	if r.expandedNow(t) {
		t.Fatal("Enter did not close the calendar")
	}
	if r.focusedKey() != "due" {
		t.Fatalf("focus after choosing is on %q, want the field", r.focusedKey())
	}
}

func TestDatePickerEscapeClosesAndReturnsFocus(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	r.press(input.KeyRight, 0)
	r.press(input.KeyEscape, 0)
	if r.expandedNow(t) {
		t.Fatal("Escape did not close the calendar")
	}
	if !r.value.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatalf("Escape changed the value to %v", r.value.Peek())
	}
	if r.focusedKey() != "due" {
		t.Fatalf("focus after Escape is on %q, want the field", r.focusedKey())
	}
}

func TestDatePickerTabClosesWithoutChoosing(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	r.press(input.KeyRight, 0)
	r.press(input.KeyTab, 0)
	if r.expandedNow(t) {
		t.Fatal("Tab did not close the calendar")
	}
	if !r.value.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatalf("Tab changed the value to %v", r.value.Peek())
	}
}

func TestDatePickerMinMaxAndDisabledDays(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), func(p *DatePickerElement) *DatePickerElement {
		return p.Min(day(2026, time.October, 5)).Max(day(2026, time.October, 25)).DisableDays(weekend)
	})
	openDate(t, r)
	// Saturday, October 10 is a weekend day: the calendar will not choose it.
	for range 3 {
		r.press(input.KeyLeft, 0)
	}
	if got := r.focusedName(); got != "Saturday, October 10, 2026" {
		t.Fatalf("focus is on %q", got)
	}
	r.press(input.KeyEnter, 0)
	if !r.expandedNow(t) || !r.value.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatal("Enter on a disabled day chose it")
	}
	if !r.stateOf(t, access.RoleDataItem, "Saturday, October 10, 2026").States.Has(access.StateDisabled) {
		t.Fatal("a disabled day is not reported disabled")
	}
	if r.stateOf(t, access.RoleDataItem, "Monday, October 12, 2026").States.Has(access.StateDisabled) {
		t.Fatal("an enabled day is reported disabled")
	}
}

func TestDatePickerTypedDateOutsideRangeIsRefused(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), func(p *DatePickerElement) *DatePickerElement {
		return p.Min(day(2026, time.October, 5)).Max(day(2026, time.October, 25)).DisableDays(weekend)
	})
	r.focusKey(t, "due")
	r.clearText(t)
	r.typeText("10/01/2026")
	r.press(input.KeyEnter, 0)
	if !nodeTextContains(r.rt, "Enter a date on or after 10/5/2026.") {
		t.Fatal("a date before Min is not refused with the earliest date")
	}
	r.clearText(t)
	r.typeText("10/10/2026")
	r.press(input.KeyEnter, 0)
	if !nodeTextContains(r.rt, "That date cannot be chosen.") {
		t.Fatal("a disabled day typed in the field is not refused")
	}
	if !r.value.Peek().Equal(day(2026, time.October, 13)) {
		t.Fatalf("a refused date changed the value to %v", r.value.Peek())
	}
}

func TestDatePickerFirstDayOfWeek(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	r.press(input.KeyHome, 0)
	if got := r.focusedName(); got != "Sunday, October 11, 2026" {
		t.Fatalf("Home in a Sunday-first week is %q", got)
	}
	r = newDateRig(t, day(2026, time.October, 13), func(p *DatePickerElement) *DatePickerElement {
		return p.FirstDay(time.Monday)
	})
	openDate(t, r)
	r.press(input.KeyHome, 0)
	if got := r.focusedName(); got != "Monday, October 12, 2026" {
		t.Fatalf("Home in a Monday-first week is %q", got)
	}
}

func TestDatePickerCalendarAccessTree(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	cal := r.calendarNode()
	if cal == nil {
		t.Fatal("the open calendar has no calendar role")
	}
	if cal.GridRows != 6 || cal.GridColumns != 7 {
		t.Fatalf("calendar grid is %dx%d, want 6x7", cal.GridRows, cal.GridColumns)
	}
	if cal.Name != "Due date" {
		t.Fatalf("calendar name %q", cal.Name)
	}
	days := 0
	weeks := 0
	r.tree().Walk(func(n *access.Node) bool {
		switch n.Role {
		case access.RoleDataItem:
			days++
		case access.RoleListItem:
			if n.SetSize == 6 {
				weeks++
			}
		}
		return true
	})
	if days != 42 || weeks != 6 {
		t.Fatalf("calendar has %d days and %d weeks, want 42 and 6", days, weeks)
	}
	if !r.stateOf(t, access.RoleDataItem, "Tuesday, October 13, 2026").States.Has(access.StateSelected) {
		t.Fatal("the chosen day is not selected in the calendar")
	}
	if n := r.stateOf(t, access.RoleDataItem, "Monday, October 12, 2026"); n.Description != "Today" {
		t.Fatalf("today's description is %q", n.Description)
	}
	if n := r.stateOf(t, access.RoleDataItem, "Tuesday, October 13, 2026"); n.GridRow == 0 || n.GridColumn == 0 {
		t.Fatal("a day has no grid position")
	}
	if r.rt.focusID == 0 {
		t.Fatal("nothing has focus with the calendar open")
	}
}

func TestDatePickerClosedHasNoCalendar(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	if r.calendarNode() != nil {
		t.Fatal("a closed date picker has a calendar")
	}
	if r.expandedNow(t) {
		t.Fatal("a closed date picker reports expanded")
	}
}

func TestDatePickerExpandCollapseThroughPattern(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	node := accesstest.Find(t, r.tree(), access.RoleComboBox, "Due date")
	if node.States.Has(access.StateExpanded) {
		t.Fatal("the date picker starts expanded")
	}
	r.bridge.Perform(node.ID, access.ActionExpand)
	r.frame()
	if !r.expandedNow(t) {
		t.Fatal("ActionExpand did not open the calendar")
	}
	r.bridge.Perform(node.ID, access.ActionCollapse)
	r.frame()
	if r.expandedNow(t) {
		t.Fatal("ActionCollapse did not close the calendar")
	}
}

func TestDatePickerDisabledIsNotEditable(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), func(p *DatePickerElement) *DatePickerElement {
		return p.Disabled(true)
	})
	if !r.nodeByKey(t, "due").div.acc.disabled {
		t.Fatal("a disabled date picker is not reported disabled")
	}
	for _, n := range r.rt.byID {
		if n.key == "due" && firstFocusable(n) != nil {
			t.Fatal("a disabled date picker has a focusable element")
		}
	}
	if r.expandedNow(t) {
		t.Fatal("a disabled date picker is expanded")
	}
}

func TestDatePickerSizing(t *testing.T) {
	r := newDateRig(t, time.Time{}, nil)
	if w := r.nodeByKey(t, "due").style.Width; w != Px(datePickerWidth) {
		t.Fatalf("default width %v, want %d px", w, datePickerWidth)
	}
	r = newDateRig(t, time.Time{}, func(p *DatePickerElement) *DatePickerElement { return p.Width(Px(300)) })
	if w := r.nodeByKey(t, "due").style.Width; w != Px(300) {
		t.Fatalf("width %v, want 300 px", w)
	}
	r = newDateRig(t, time.Time{}, func(p *DatePickerElement) *DatePickerElement { return p.Width(Full) })
	if w := r.nodeByKey(t, "due").style.Width; w != Full {
		t.Fatalf("width %v, want Full", w)
	}
}

func TestDatePickerValidateRequired(t *testing.T) {
	v := NewSignal(time.Time{})
	p := DatePicker(v).Validate(Required[time.Time]())
	if err := p.formCheck(); err == nil {
		t.Fatal("Required accepts no date")
	}
	v.Set(day(2026, time.October, 13))
	if err := p.formCheck(); err != nil {
		t.Fatalf("Required rejects a date: %v", err)
	}
}

func TestDatePickerRequiredInFieldShowsError(t *testing.T) {
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	v := NewSignal(time.Time{})
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Field("Due date", DatePicker(v).Key("due").Validate(Required[time.Time]())))
	})
	r.focusKey(t, "due")
	r.typeText("1")
	r.press(input.KeyBackspace, 0)
	r.rt.setFocus(0)
	r.frame()
	n := partNodeOrNil(r.rt, "FormField.Error")
	if n == nil || n.div.noDisplay || !strings.Contains(nodeText(n), "This field is required.") {
		t.Fatal("Required does not show its error after the field was emptied and left")
	}
}

func TestDatePickerWarnsNothing(t *testing.T) {
	r := newDateRig(t, day(2026, time.October, 13), nil)
	openDate(t, r)
	r.press(input.KeyPageDown, 0)
	r.press(input.KeyEscape, 0)
	if len(r.warnings) != 0 {
		t.Fatalf("development warnings: %v", r.warnings)
	}
}

func TestDatePickerReducedMotion(t *testing.T) {
	for _, reduced := range []bool{true, false} {
		r := newDateRig(t, day(2026, time.October, 13), nil)
		r.rt.sys.ReducedMotion = reduced
		openDate(t, r)
		if reduced && (r.rt.animating || len(r.rt.animInsts) != 0) {
			t.Fatal("opening the calendar with reduced motion requested an animation")
		}
		r.press(input.KeyEscape, 0)
		closing := len(r.rt.ghosts) > 0
		if reduced && closing {
			t.Fatal("with reduced motion the closed calendar is still drawn for its exit")
		}
		if !reduced && !closing {
			t.Fatal("without reduced motion the closing calendar has no exit motion")
		}
	}
}

// weekend is the DisableDays rule of the tests: Saturdays and Sundays.
func weekend(d time.Time) bool { return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday }

// clearText empties the field: the caret goes to the end, and each character is deleted.
func (r *dateRig) clearText(t *testing.T) {
	t.Helper()
	n := len([]rune(r.stateOf(t, access.RoleComboBox, "Due date").Value))
	r.press(input.KeyEnd, 0)
	for range n {
		r.press(input.KeyBackspace, 0)
	}
}

// An empty date picker shows the hint of the locale's date format when no Placeholder is set,
// and the Placeholder when one is.
func TestDatePickerDefaultPlaceholderIsTheLocaleHint(t *testing.T) {
	r := newDateRig(t, time.Time{}, func(p *DatePickerElement) *DatePickerElement { return p.Placeholder("") })
	if got := editorNode(t, r.rt).editor.placeholder; got != "MM/DD/YYYY" {
		t.Fatalf("an empty date picker shows the hint %q, want the en-US date hint", got)
	}
	r = newDateRig(t, time.Time{}, func(p *DatePickerElement) *DatePickerElement { return p.Placeholder("Pick a day") })
	if got := editorNode(t, r.rt).editor.placeholder; got != "Pick a day" {
		t.Fatal("a Placeholder does not replace the locale hint")
	}
}
