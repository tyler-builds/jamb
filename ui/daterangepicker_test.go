package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// dateRangeRig is an access rig with a date range picker keyed "trip" over the start and the end. The
// locale is en-US, and today is October 12, 2026.
type dateRangeRig struct {
	*accessRig
	start, end *Signal[time.Time]
	changes    [][2]time.Time
}

func newDateRangeRig(t *testing.T, start, end time.Time, cfg func(*DateRangePickerElement) *DateRangePickerElement) *dateRangeRig {
	t.Helper()
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	rr := &dateRangeRig{start: NewSignal(start), end: NewSignal(end)}
	build := func() *DateRangePickerElement {
		p := DateRangePicker(rr.start, rr.end).Key("trip").Label("Trip dates")
		p.OnChange(func(s, e time.Time) { rr.changes = append(rr.changes, [2]time.Time{s, e}) })
		if cfg != nil {
			p = cfg(p)
		}
		return p
	}
	rr.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(build())
	})
	rr.now = testNow
	rr.frame()
	return rr
}

// openRange focuses the field and opens the calendar with Alt+Down.
func (r *dateRangeRig) openRange(t *testing.T) {
	t.Helper()
	r.focusKey(t, "trip")
	r.rt.keyboardModality = true
	r.press(input.KeyDown, input.ModAlt)
	if !r.expanded(t) {
		t.Fatal("Alt+Down did not open the calendar")
	}
}

// expanded reports whether the calendar is open, from the field's access state.
func (r *dateRangeRig) expanded(t *testing.T) bool {
	t.Helper()
	return r.nodeByKey(t, "trip").div.acc.expanded
}

// focusedName returns the name of the node that has keyboard focus, as the tree gives it.
func (r *dateRangeRig) focusedName() string {
	tr := r.tree()
	if n := tr.Node(tr.Focus); n != nil {
		return n.Name
	}
	return ""
}

// text returns the text of the field, which is its value in the access tree.
func (r *dateRangeRig) text(t *testing.T) string {
	t.Helper()
	return r.stateOf(t, access.RoleComboBox, "Trip dates").Value
}

// clickDay presses and releases the pointer on the day with the long date name.
func (r *dateRangeRig) clickDay(t *testing.T, name string) {
	t.Helper()
	rc := r.rt.nodeRect(accessNode(t, r.accessRig, access.RoleDataItem, name))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// hoverDay moves the pointer over the day with the long date name.
func (r *dateRangeRig) hoverDay(t *testing.T, name string) {
	t.Helper()
	hoverNode(r.accessRig, accessNode(t, r.accessRig, access.RoleDataItem, name))
}

// clearText empties the field: the caret goes to the end, and each character is deleted.
func (r *dateRangeRig) clearText(t *testing.T) {
	t.Helper()
	n := len([]rune(r.text(t)))
	r.press(input.KeyEnd, 0)
	for range n {
		r.press(input.KeyBackspace, 0)
	}
}

// dateRangeLooks returns the fill of a day between the ends of a range and the radius of a day, in
// the dark theme at the Text size of 100 percent.
func dateRangeLooks() (fill Color, dayRadius float32) {
	d := dateRangePickerDefaults(DarkTheme(), 1, false)
	return d.DayInRange.Background, d.Day.Radius
}

func TestDateRangePickerClicksChooseStartThenEnd(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.openRange(t)
	r.clickDay(t, "Tuesday, October 13, 2026")
	if !r.start.Peek().IsZero() || !r.end.Peek().IsZero() || len(r.changes) != 0 {
		t.Fatal("choosing the start changed the range before the end was chosen")
	}
	if !r.expanded(t) {
		t.Fatal("choosing the start closed the calendar")
	}
	r.clickDay(t, "Tuesday, October 20, 2026")
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 20)) {
		t.Fatalf("range = %v to %v, want October 13 to October 20", r.start.Peek(), r.end.Peek())
	}
	if r.expanded(t) {
		t.Fatal("choosing the end did not close the calendar")
	}
	if len(r.changes) != 1 {
		t.Fatalf("OnChange ran %d times, want 1", len(r.changes))
	}
}

func TestDateRangePickerSwapsEndBeforeStart(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.openRange(t)
	r.clickDay(t, "Tuesday, October 20, 2026")
	r.clickDay(t, "Tuesday, October 13, 2026")
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 20)) {
		t.Fatalf("range = %v to %v, want the two days in order", r.start.Peek(), r.end.Peek())
	}
	if len(r.changes) != 1 || !r.changes[0][0].Equal(day(2026, time.October, 13)) {
		t.Fatalf("OnChange got %v, want one call with the start first", r.changes)
	}
}

func TestDateRangePickerKeysChooseStartThenEnd(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.openRange(t)
	if got := r.focusedName(); got != "Monday, October 12, 2026" {
		t.Fatalf("with no range, focus is on %q, want today", got)
	}
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0)
	if !r.expanded(t) {
		t.Fatal("Enter on the start closed the calendar")
	}
	for range 3 {
		r.press(input.KeyRight, 0)
	}
	r.press(input.KeyEnter, 0)
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 16)) {
		t.Fatalf("range = %v to %v, want October 13 to October 16", r.start.Peek(), r.end.Peek())
	}
	if r.expanded(t) {
		t.Fatal("Enter on the end did not close the calendar")
	}
	if r.focusedKey() != "trip" {
		t.Fatalf("focus after choosing is on %q, want the field", r.focusedKey())
	}
}

func TestDateRangePickerEscapeKeepsOldRange(t *testing.T) {
	r := newDateRangeRig(t, day(2026, time.October, 13), day(2026, time.October, 20), nil)
	r.openRange(t)
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0) // the start is chosen; the end is not
	r.press(input.KeyEscape, 0)
	if r.expanded(t) {
		t.Fatal("Escape did not close the calendar")
	}
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 20)) {
		t.Fatalf("Escape changed the range to %v to %v", r.start.Peek(), r.end.Peek())
	}
	if len(r.changes) != 0 {
		t.Fatal("OnChange ran although the range did not change")
	}
	if got := r.text(t); got != "10/13/2026 – 10/20/2026" {
		t.Fatalf("the field reads %q after Escape", got)
	}
	if r.focusedKey() != "trip" {
		t.Fatalf("focus after Escape is on %q, want the field", r.focusedKey())
	}
}

func TestDateRangePickerHoverPreviewShadesBetween(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.openRange(t)
	r.clickDay(t, "Tuesday, October 13, 2026")
	r.hoverDay(t, "Friday, October 16, 2026")
	fill, dayRadius := dateRangeLooks()
	for _, name := range []string{"Wednesday, October 14, 2026", "Thursday, October 15, 2026"} {
		n := accessNode(t, r.accessRig, access.RoleDataItem, name)
		if n.style.Background != fill {
			t.Errorf("%s: background %v, want the band %v", name, n.style.Background, fill)
		}
		if n.style.Radius != 0 {
			t.Errorf("%s: radius %v, want square corners in the band", name, n.style.Radius)
		}
		if got := r.stateOf(t, access.RoleDataItem, name).Description; got != "In range" {
			t.Errorf("%s: description %q, want In range", name, got)
		}
	}
	end := r.stateOf(t, access.RoleDataItem, "Friday, October 16, 2026")
	if end.Description != "End of range" || !end.States.Has(access.StateSelected) {
		t.Errorf("the day under the pointer is %q selected %v, want the end of the range",
			end.Description, end.States.Has(access.StateSelected))
	}
	if n := accessNode(t, r.accessRig, access.RoleDataItem, "Friday, October 16, 2026"); n.style.Radius != dayRadius {
		t.Errorf("the end has radius %v, want the day's %v", n.style.Radius, dayRadius)
	}
	start := r.stateOf(t, access.RoleDataItem, "Tuesday, October 13, 2026")
	if start.Description != "Start of range" || !start.States.Has(access.StateSelected) {
		t.Errorf("the start is %q selected %v", start.Description, start.States.Has(access.StateSelected))
	}
	after := r.stateOf(t, access.RoleDataItem, "Saturday, October 17, 2026")
	if after.Description != "" || after.States.Has(access.StateSelected) {
		t.Errorf("a day after the preview is %q selected %v", after.Description, after.States.Has(access.StateSelected))
	}
	if !r.start.Peek().IsZero() || !r.end.Peek().IsZero() {
		t.Fatal("the preview changed the range")
	}
}

func TestDateRangePickerKeyboardPreviewFollowsFocus(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.openRange(t)
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0)
	for range 3 {
		r.press(input.KeyRight, 0)
	}
	for _, name := range []string{"Wednesday, October 14, 2026", "Thursday, October 15, 2026"} {
		if got := r.stateOf(t, access.RoleDataItem, name).Description; got != "In range" {
			t.Errorf("%s: description %q, want In range while the keys preview it", name, got)
		}
	}
	if got := r.stateOf(t, access.RoleDataItem, "Friday, October 16, 2026").Description; got != "End of range" {
		t.Errorf("the day the keys are on is %q, want End of range", got)
	}
	if got := r.stateOf(t, access.RoleDataItem, "Saturday, October 17, 2026").Description; got != "" {
		t.Errorf("a day past the keys is %q, want no range", got)
	}
}

func TestDateRangePickerMinMaxAndDisabledDays(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, func(p *DateRangePickerElement) *DateRangePickerElement {
		return p.Min(day(2026, time.October, 5)).Max(day(2026, time.October, 25)).DisableDays(weekend)
	})
	r.openRange(t)
	r.clickDay(t, "Saturday, October 10, 2026") // a weekend day: ignored
	r.clickDay(t, "Tuesday, October 13, 2026")  // the start
	r.clickDay(t, "Monday, October 26, 2026")   // after Max: ignored, and the start is kept
	r.clickDay(t, "Saturday, October 17, 2026") // a weekend day: ignored
	if !r.start.Peek().IsZero() {
		t.Fatal("a refused day changed the range")
	}
	if !r.stateOf(t, access.RoleDataItem, "Saturday, October 10, 2026").States.Has(access.StateDisabled) {
		t.Fatal("a weekend day is not reported disabled")
	}
	if r.stateOf(t, access.RoleDataItem, "Monday, October 12, 2026").States.Has(access.StateDisabled) {
		t.Fatal("an enabled day is reported disabled")
	}
	r.clickDay(t, "Friday, October 16, 2026")
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 16)) {
		t.Fatalf("range = %v to %v, want October 13 to October 16", r.start.Peek(), r.end.Peek())
	}
}

func TestDateRangePickerTypedRange(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.focusKey(t, "trip")
	r.typeText("10/20/2026 - 10/13/2026")
	r.press(input.KeyEnter, 0)
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 20)) {
		t.Fatalf("typed range = %v to %v, want the two days in order", r.start.Peek(), r.end.Peek())
	}
	if len(r.changes) != 1 {
		t.Fatalf("OnChange ran %d times, want 1", len(r.changes))
	}
	if got := r.text(t); got != "10/13/2026 – 10/20/2026" {
		t.Fatalf("the field reads %q after the range is applied", got)
	}
}

func TestDateRangePickerTypedRangeWithEnDash(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.focusKey(t, "trip")
	r.typeText("10/13/2026 – 10/20/2026")
	r.press(input.KeyEnter, 0)
	if !r.start.Peek().Equal(day(2026, time.October, 13)) || !r.end.Peek().Equal(day(2026, time.October, 20)) {
		t.Fatalf("typed range = %v to %v", r.start.Peek(), r.end.Peek())
	}
}

func TestDateRangePickerTypedRangeRefusals(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, func(p *DateRangePickerElement) *DateRangePickerElement {
		return p.DisableDays(weekend)
	})
	r.focusKey(t, "trip")
	r.typeText("10/13/2026")
	r.press(input.KeyEnter, 0)
	if !nodeTextContains(r.rt, "Enter a range such as 10/13/2026 – 10/20/2026.") {
		t.Fatal("a single date is not refused with an example of a range")
	}
	r.clearText(t)
	r.typeText("10/13/2026 – 10/10/2026")
	r.press(input.KeyEnter, 0)
	if !nodeTextContains(r.rt, "That date cannot be chosen.") {
		t.Fatal("a disabled day typed as an end is not refused")
	}
	if !r.start.Peek().IsZero() || !r.end.Peek().IsZero() || len(r.changes) != 0 {
		t.Fatal("a refused range changed the range")
	}
	if !r.nodeByKey(t, "trip").div.acc.invalid {
		t.Fatal("the field is not reported invalid after a refused range")
	}
}

func TestDateRangePickerTextFollowsRange(t *testing.T) {
	r := newDateRangeRig(t, day(2026, time.October, 13), time.Time{}, nil)
	if got := r.text(t); got != "10/13/2026" {
		t.Fatalf("a start with no end reads %q, want the start alone", got)
	}
	r.start.Set(day(2026, time.October, 13))
	r.end.Set(day(2026, time.October, 20))
	r.frame()
	if got := r.text(t); got != "10/13/2026 – 10/20/2026" {
		t.Fatalf("a range set from outside reads %q", got)
	}
	if got := editorNode(t, r.rt).editor.placeholder; got != "MM/DD/YYYY – MM/DD/YYYY" {
		t.Fatalf("the placeholder is %q", got)
	}
}

func TestDateRangePickerBlurRevertsInvalidText(t *testing.T) {
	r := newDateRangeRig(t, day(2026, time.October, 13), day(2026, time.October, 20), nil)
	r.focusKey(t, "trip")
	r.clearText(t)
	r.typeText("xx")
	r.rt.setFocus(0)
	r.frame()
	if got := r.text(t); got != "10/13/2026 – 10/20/2026" {
		t.Fatalf("after leaving the field the text is %q, want the range", got)
	}
	if r.nodeByKey(t, "trip").div.acc.invalid {
		t.Fatal("the field is still invalid after it was reverted")
	}
}

func TestDateRangePickerGermanLocaleText(t *testing.T) {
	prev := localeOverride
	localeOverride = testLocaleDE()
	t.Cleanup(func() { localeOverride = prev })
	start, end := NewSignal(day(2026, time.October, 13)), NewSignal(day(2026, time.October, 20))
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(DateRangePicker(start, end).Key("trip").Label("Trip dates"))
	})
	r.now = testNow
	r.frame()
	if got := r.stateOf(t, access.RoleComboBox, "Trip dates").Value; got != "13.10.2026 – 20.10.2026" {
		t.Fatalf("the de-DE range reads %q", got)
	}
	if got := editorNode(t, r.rt).editor.placeholder; got != "DD.MM.YYYY – DD.MM.YYYY" {
		t.Fatalf("the de-DE placeholder is %q", got)
	}
}

func TestDateRangePickerAccessTree(t *testing.T) {
	r := newDateRangeRig(t, day(2026, time.October, 13), day(2026, time.October, 20), nil)
	r.openRange(t)
	var cal *access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleCalendar {
			cal = n
		}
		return true
	})
	if cal == nil {
		t.Fatal("the open calendar has no calendar role")
	}
	if cal.Name != "Trip dates" || cal.GridRows != 6 || cal.GridColumns != 7 {
		t.Fatalf("calendar %q is %dx%d, want Trip dates 6x7", cal.Name, cal.GridRows, cal.GridColumns)
	}
	field := accesstest.Find(t, r.tree(), access.RoleComboBox, "Trip dates")
	if !field.States.Has(access.StateExpanded) {
		t.Fatal("the field is not expanded while the calendar is open")
	}
	for name, want := range map[string]string{
		"Tuesday, October 13, 2026":  "Start of range",
		"Thursday, October 15, 2026": "In range",
		"Tuesday, October 20, 2026":  "End of range",
		"Monday, October 12, 2026":   "Today",
		"Saturday, October 24, 2026": "",
	} {
		n := r.stateOf(t, access.RoleDataItem, name)
		if n.Description != want {
			t.Errorf("%s: description %q, want %q", name, n.Description, want)
		}
	}
	for _, name := range []string{"Tuesday, October 13, 2026", "Tuesday, October 20, 2026"} {
		if !r.stateOf(t, access.RoleDataItem, name).States.Has(access.StateSelected) {
			t.Errorf("%s is not selected", name)
		}
	}
	if r.stateOf(t, access.RoleDataItem, "Thursday, October 15, 2026").States.Has(access.StateSelected) {
		t.Fatal("a day between the ends is selected")
	}
	if got := r.text(t); got != "10/13/2026 – 10/20/2026" {
		t.Fatalf("the field value is %q", got)
	}
}

func TestDateRangePickerDisabledIsNotEditable(t *testing.T) {
	r := newDateRangeRig(t, day(2026, time.October, 13), day(2026, time.October, 20), func(p *DateRangePickerElement) *DateRangePickerElement {
		return p.Disabled(true)
	})
	if !r.nodeByKey(t, "trip").div.acc.disabled {
		t.Fatal("a disabled date range picker is not reported disabled")
	}
	for _, n := range r.rt.byID {
		if n.key == "trip" && firstFocusable(n) != nil {
			t.Fatal("a disabled date range picker has a focusable element")
		}
	}
	if r.expanded(t) {
		t.Fatal("a disabled date range picker is expanded")
	}
}

func TestDateRangePickerSizing(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	if w := r.nodeByKey(t, "trip").style.Width; w != Px(dateRangePickerWidth) {
		t.Fatalf("default width %v, want %d px", w, dateRangePickerWidth)
	}
	r = newDateRangeRig(t, time.Time{}, time.Time{}, func(p *DateRangePickerElement) *DateRangePickerElement {
		return p.Width(Px(300))
	})
	if w := r.nodeByKey(t, "trip").style.Width; w != Px(300) {
		t.Fatalf("width %v, want 300 px", w)
	}
	r = newDateRangeRig(t, time.Time{}, time.Time{}, func(p *DateRangePickerElement) *DateRangePickerElement {
		return p.Width(Full)
	})
	if w := r.nodeByKey(t, "trip").style.Width; w != Full {
		t.Fatalf("width %v, want Full", w)
	}
}

func TestDateRangePickerWarnsNothing(t *testing.T) {
	r := newDateRangeRig(t, time.Time{}, time.Time{}, nil)
	r.openRange(t)
	r.clickDay(t, "Tuesday, October 13, 2026")
	r.press(input.KeyPageDown, 0)
	r.press(input.KeyEscape, 0)
	if len(r.warnings) != 0 {
		t.Fatalf("development warnings: %v", r.warnings)
	}
}

// TestDateRangePickerHighContrastBandHasNoFill checks that under high contrast the days between the
// ends are drawn with lines and no fill, and with square corners.
func TestDateRangePickerHighContrastBandHasNoFill(t *testing.T) {
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	start, end := NewSignal(day(2026, time.October, 13)), NewSignal(day(2026, time.October, 20))
	rig := newThemedRig(HighContrastTheme(HighContrastBlack(), DarkTheme()), func(ctx *Context) Element {
		return Div().Column().Child(DateRangePicker(start, end).Key("trip").Label("Trip dates"))
	})
	r := &dateRangeRig{accessRig: rig}
	r.openRange(t)
	n := accessNode(t, r.accessRig, access.RoleDataItem, "Wednesday, October 14, 2026")
	if !n.style.Background.IsZero() {
		t.Errorf("a day between the ends has a fill %v under high contrast", n.style.Background)
	}
	if n.style.BorderEdges.Top == 0 || n.style.BorderEdges.Bottom == 0 {
		t.Errorf("a day between the ends has no line above and below under high contrast: %v", n.style.BorderEdges)
	}
	if n.style.Radius != 0 {
		t.Errorf("a day between the ends has radius %v under high contrast", n.style.Radius)
	}
}

// dateRangePartElement is the date range picker of the part tests, with the instance style st, the
// start and the end, and cfg applied to it.
func dateRangePartElement(start, end *Signal[time.Time], st DateRangePickerStyle, cfg func(*DateRangePickerElement) *DateRangePickerElement) Element {
	p := DateRangePicker(start, end).Key("trip").Label("Trip dates").Placeholder("MM/DD/YYYY – MM/DD/YYYY").Styles(st)
	if cfg != nil {
		p = cfg(p)
	}
	return Div().Column().Child(p)
}

// testDateRangePickerParts sets a distinctive style on each part of the date range picker, puts the
// state of the part in effect, and asserts that the style reaches the node that draws it. The parts
// that show a range are tested with the range from October 13 to October 20, 2026.
func testDateRangePickerParts(t *testing.T, applied map[string]bool) {
	type drpCase struct {
		part string
		set  func(*DateRangePickerStyle, Style)
		run  func(t *testing.T, r *accessRig)
	}
	frame := func(t *testing.T, r *accessRig, part string) {
		wantBackground(t, partNode(t, r.rt, "DateRangePicker.Frame", ""), part, markBG)
	}
	openDay := func(t *testing.T, r *accessRig) {
		r.focusKey(t, "trip")
		r.rt.keyboardModality = true
		r.press(input.KeyDown, input.ModAlt)
	}
	cases := []drpCase{
		{part: "DateRangePicker.Frame", set: func(s *DateRangePickerStyle, m Style) { s.Field.Frame = m },
			run: func(t *testing.T, r *accessRig) { frame(t, r, "DateRangePicker.Frame") }},
		{part: "DateRangePicker.FrameHover", set: func(s *DateRangePickerStyle, m Style) { s.Field.FrameHover = m },
			run: func(t *testing.T, r *accessRig) {
				hoverNode(r, partNode(t, r.rt, "DateRangePicker.Frame", ""))
				frame(t, r, "DateRangePicker.FrameHover")
			}},
		{part: "DateRangePicker.FrameFocusWithin", set: func(s *DateRangePickerStyle, m Style) { s.Field.FrameFocusWithin = m },
			run: func(t *testing.T, r *accessRig) {
				focusNode(r, editorNode(t, r.rt))
				frame(t, r, "DateRangePicker.FrameFocusWithin")
			}},
		{part: "DateRangePicker.FrameDisabled", set: func(s *DateRangePickerStyle, m Style) { s.Field.FrameDisabled = m },
			run: func(t *testing.T, r *accessRig) { frame(t, r, "DateRangePicker.FrameDisabled") }},
		{part: "DateRangePicker.FrameInvalid", set: func(s *DateRangePickerStyle, m Style) { s.Field.FrameInvalid = m },
			run: func(t *testing.T, r *accessRig) {
				r.focusKey(t, "trip")
				r.typeText("13/45/2026")
				r.press(input.KeyEnter, 0)
				frame(t, r, "DateRangePicker.FrameInvalid")
			}},
		{part: "DateRangePicker.Text", set: func(s *DateRangePickerStyle, m Style) { s.Field.Text = m },
			run: func(t *testing.T, r *accessRig) {
				wantTextColor(t, editorNode(t, r.rt), "DateRangePicker.Text", markFG)
			}},
		{part: "DateRangePicker.Placeholder", set: func(s *DateRangePickerStyle, m Style) { s.Field.Placeholder = m },
			run: func(t *testing.T, r *accessRig) {
				if got := editorNode(t, r.rt).editor.placeholderColor; got != markFG {
					t.Errorf("DateRangePicker.Placeholder: color %v, want %v", got, markFG)
				}
			}},
		{part: "DateRangePicker.Selection", set: func(s *DateRangePickerStyle, m Style) { s.Field.Selection = m },
			run: func(t *testing.T, r *accessRig) {
				if got := editorNode(t, r.rt).editor.selection; got != markBG {
					t.Errorf("DateRangePicker.Selection: color %v, want %v", got, markBG)
				}
			}},
		{part: "DateRangePicker.CalendarButton", set: func(s *DateRangePickerStyle, m Style) { s.CalendarButton = m },
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "DateRangePicker.CalendarButton", ""), "DateRangePicker.CalendarButton", markBG)
			}},
		{part: "DateRangePicker.Message", set: func(s *DateRangePickerStyle, m Style) { s.Message = m },
			run: func(t *testing.T, r *accessRig) {
				r.focusKey(t, "trip")
				r.typeText("13/45/2026")
				r.press(input.KeyEnter, 0)
				wantBackground(t, partNode(t, r.rt, "DateRangePicker.Message", ""), "DateRangePicker.Message", markBG)
			}},
		{part: "DateRangePicker.Popup", set: func(s *DateRangePickerStyle, m Style) { s.Popup = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, partNode(t, r.rt, "DateRangePicker.Popup", ""), "DateRangePicker.Popup", markBG)
			}},
		{part: "DateRangePicker.Header", set: func(s *DateRangePickerStyle, m Style) { s.Header = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, partNode(t, r.rt, "DateRangePicker.Header", ""), "DateRangePicker.Header", markBG)
			}},
		{part: "DateRangePicker.NavButton", set: func(s *DateRangePickerStyle, m Style) { s.NavButton = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackgroundAll(t, r, "DateRangePicker.NavButton")
			}},
		{part: "DateRangePicker.WeekdayLabel", set: func(s *DateRangePickerStyle, m Style) { s.WeekdayLabel = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackgroundAll(t, r, "DateRangePicker.WeekdayLabel")
			}},
		{part: "DateRangePicker.Day", set: func(s *DateRangePickerStyle, m Style) { s.Day = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Thursday, October 15, 2026"), "DateRangePicker.Day", markBG)
			}},
		{part: "DateRangePicker.DayHover", set: func(s *DateRangePickerStyle, m Style) { s.DayHover = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				hoverNode(r, accessNode(t, r, access.RoleDataItem, "Friday, October 23, 2026"))
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Friday, October 23, 2026"), "DateRangePicker.DayHover", markBG)
			}},
		{part: "DateRangePicker.DaySelected", set: func(s *DateRangePickerStyle, m Style) { s.DaySelected = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Tuesday, October 13, 2026"), "DateRangePicker.DaySelected", markBG)
			}},
		{part: "DateRangePicker.DayInRange", set: func(s *DateRangePickerStyle, m Style) { s.DayInRange = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Thursday, October 15, 2026"), "DateRangePicker.DayInRange", markBG)
			}},
		{part: "DateRangePicker.DayToday", set: func(s *DateRangePickerStyle, m Style) { s.DayToday = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Monday, October 12, 2026"), "DateRangePicker.DayToday", markBG)
			}},
		{part: "DateRangePicker.DayDisabled", set: func(s *DateRangePickerStyle, m Style) { s.DayDisabled = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Saturday, October 17, 2026"), "DateRangePicker.DayDisabled", markBG)
			}},
		{part: "DateRangePicker.DayOutsideMonth", set: func(s *DateRangePickerStyle, m Style) { s.DayOutsideMonth = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Sunday, September 27, 2026"), "DateRangePicker.DayOutsideMonth", markBG)
			}},
		{part: "DateRangePicker.DayFocusVisible", set: func(s *DateRangePickerStyle, m Style) { s.DayFocusVisible = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r) // with no range, the keys start on today
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Monday, October 12, 2026"), "DateRangePicker.DayFocusVisible", markBG)
			}},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st DateRangePickerStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			start, end := time.Time{}, time.Time{}
			if c.part == "DateRangePicker.DaySelected" || c.part == "DateRangePicker.DayInRange" || c.part == "DateRangePicker.Selection" {
				start, end = day(2026, time.October, 13), day(2026, time.October, 20)
			}
			r := pickerRig(t, func(*Context) Element {
				return dateRangePartElement(NewSignal(start), NewSignal(end), st, func(p *DateRangePickerElement) *DateRangePickerElement {
					if c.part == "DateRangePicker.FrameDisabled" {
						return p.Disabled(true)
					}
					if c.part == "DateRangePicker.DayDisabled" {
						return p.DisableDays(func(d time.Time) bool { return d.Day() == 17 && d.Month() == time.October })
					}
					return p
				})
			})
			if c.part == "DateRangePicker.Selection" || c.part == "DateRangePicker.Text" || c.part == "DateRangePicker.Placeholder" ||
				c.part == "DateRangePicker.FrameFocusWithin" {
				r.focusKey(t, "trip")
			}
			c.run(t, r)
		})
	}
}
