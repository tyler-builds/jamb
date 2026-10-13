package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// timeRig is an access rig with a time picker keyed "start" over value. The locale is l (en-US
// when nil), and today is October 12, 2026.
type timeRig struct {
	*accessRig
	value   *Signal[time.Time]
	changes []time.Time
	heard   []string
}

func newTimeRig(t *testing.T, value time.Time, l *dateLocale, cfg func(*TimePickerElement) *TimePickerElement) *timeRig {
	t.Helper()
	if l == nil {
		l = testLocaleEN()
	}
	prev := localeOverride
	localeOverride = l
	t.Cleanup(func() { localeOverride = prev })
	tr := &timeRig{value: NewSignal(value)}
	build := func() *TimePickerElement {
		p := TimePicker(tr.value).Key("start").Label("Start time").Placeholder("hh:mm")
		p.OnChange(func(v time.Time) { tr.changes = append(tr.changes, v) })
		if cfg != nil {
			p = cfg(p)
		}
		return p
	}
	tr.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(build())
	})
	tr.now = testNow
	tr.frame()
	tr.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			tr.heard = append(tr.heard, a.Text)
		}
	})
	return tr
}

// clockOn returns the time of day h:m on the test day, October 12, 2026.
func clockOn(h, m int) time.Time { return time.Date(2026, time.October, 12, h, m, 0, 0, time.Local) }

// openTime focuses the field and opens the list with Alt+Down.
func openTime(t *testing.T, r *timeRig) {
	t.Helper()
	r.focusKey(t, "start")
	r.rt.keyboardModality = true
	r.press(input.KeyDown, input.ModAlt)
	if !r.timeExpanded(t) {
		t.Fatal("Alt+Down did not open the list of times")
	}
}

func (r *timeRig) timeExpanded(t *testing.T) bool {
	t.Helper()
	return r.nodeByKey(t, "start").div.acc.expanded
}

// clearTime empties the field: the caret goes to the end, and each character is deleted.
func (r *timeRig) clearTime(t *testing.T) {
	t.Helper()
	n := len([]rune(r.stateOf(t, access.RoleComboBox, "Start time").Value))
	r.press(input.KeyEnd, 0)
	for range n {
		r.press(input.KeyBackspace, 0)
	}
}

// listedTimes returns the labels of the rows of the open list, in order.
func (r *timeRig) listedTimes() []string {
	var out []string
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem && n.Name != "" {
			out = append(out, n.Name)
		}
		return true
	})
	return out
}

// highlightedTime returns the label of the highlighted row of the list, or "".
func (r *timeRig) highlightedTime() string {
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleListItem && n.div.acc.highlighted {
			return n.div.acc.name
		}
	}
	return ""
}

func TestTimePickerTypedTimeSetsValueOnEnter(t *testing.T) {
	r := newTimeRig(t, time.Time{}, nil, nil)
	r.focusKey(t, "start")
	r.typeText("9:30 pm")
	r.press(input.KeyEnter, 0)
	if want := clockOn(21, 30); !r.value.Peek().Equal(want) {
		t.Fatalf("value = %v, want %v", r.value.Peek(), want)
	}
	if len(r.changes) != 1 {
		t.Fatalf("OnChange ran %d times, want 1", len(r.changes))
	}
}

func TestTimePickerInvalidTimeKeepsValue(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 0), nil, nil)
	r.focusKey(t, "start")
	r.clearTime(t)
	r.typeText("13:00 pm")
	r.press(input.KeyEnter, 0)
	if !r.value.Peek().Equal(clockOn(9, 0)) {
		t.Fatalf("an invalid time changed the value to %v", r.value.Peek())
	}
	if !r.nodeByKey(t, "start").div.acc.invalid {
		t.Fatal("the field is not reported invalid")
	}
	if !nodeTextContains(r.rt, "Enter a time such as 9:30 AM.") {
		t.Fatal("the message does not give an example of a time")
	}
}

func TestTimePickerUpAndDownStepByMinuteStep(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 0), nil, nil)
	r.focusKey(t, "start")
	r.press(input.KeyUp, 0)
	if !r.value.Peek().Equal(clockOn(9, 15)) {
		t.Fatalf("Up from 9:00 is %v, want 9:15", r.value.Peek())
	}
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0)
	if !r.value.Peek().Equal(clockOn(8, 45)) {
		t.Fatalf("Down twice from 9:15 is %v, want 8:45", r.value.Peek())
	}

	r = newTimeRig(t, clockOn(9, 0), nil, func(p *TimePickerElement) *TimePickerElement { return p.MinuteStep(30) })
	r.focusKey(t, "start")
	r.press(input.KeyUp, 0)
	if !r.value.Peek().Equal(clockOn(9, 30)) {
		t.Fatalf("with a 30 minute step, Up from 9:00 is %v", r.value.Peek())
	}
}

func TestTimePickerUpFromNoTimeStartsAtTheFirstTime(t *testing.T) {
	r := newTimeRig(t, time.Time{}, nil, nil)
	r.focusKey(t, "start")
	r.press(input.KeyUp, 0)
	if !r.value.Peek().Equal(clockOn(0, 0)) {
		t.Fatalf("Up with no time is %v, want 12:00 AM", r.value.Peek())
	}
}

func TestTimePickerKeepsTheDateOfTheValue(t *testing.T) {
	day := time.Date(2026, time.October, 20, 10, 0, 0, 0, time.Local)
	r := newTimeRig(t, day, nil, nil)
	r.focusKey(t, "start")
	r.press(input.KeyUp, 0)
	want := time.Date(2026, time.October, 20, 10, 15, 0, 0, time.Local)
	if !r.value.Peek().Equal(want) {
		t.Fatalf("value = %v, want %v", r.value.Peek(), want)
	}
}

func TestTimePickerAltDownHighlightsTheTimeAndEnterChooses(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 30), nil, nil)
	openTime(t, r)
	if got := r.highlightedTime(); got != "9:30 AM" {
		t.Fatalf("the list highlights %q, want the time the field holds", got)
	}
	r.press(input.KeyDown, 0)
	if got := r.highlightedTime(); got != "9:45 AM" {
		t.Fatalf("Down highlights %q, want 9:45 AM", got)
	}
	r.press(input.KeyEnter, 0)
	if !r.value.Peek().Equal(clockOn(9, 45)) {
		t.Fatalf("Enter chose %v, want 9:45", r.value.Peek())
	}
	if r.timeExpanded(t) {
		t.Fatal("Enter did not close the list")
	}
}

func TestTimePickerListHasTheStepAndWraps(t *testing.T) {
	r := newTimeRig(t, time.Time{}, nil, nil)
	openTime(t, r)
	rows := r.listedTimes()
	if len(rows) != 96 {
		t.Fatalf("the list has %d times, want 96 (every 15 minutes)", len(rows))
	}
	if rows[0] != "12:00 AM" || rows[1] != "12:15 AM" || rows[len(rows)-1] != "11:45 PM" {
		t.Fatalf("the list runs %q .. %q", rows[0], rows[len(rows)-1])
	}
}

func TestTimePickerEscapeClosesTheList(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 30), nil, nil)
	openTime(t, r)
	r.press(input.KeyEscape, 0)
	if r.timeExpanded(t) {
		t.Fatal("Escape did not close the list")
	}
	if !r.value.Peek().Equal(clockOn(9, 30)) {
		t.Fatalf("Escape changed the value to %v", r.value.Peek())
	}
	if r.focusedKey() != "start" {
		t.Fatalf("focus left the text: %q", r.focusedKey())
	}
}

func TestTimePickerMinMaxBoundTheListAndRefuseTyping(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 0), nil, func(p *TimePickerElement) *TimePickerElement {
		return p.Min(clockOn(9, 0)).Max(clockOn(17, 0))
	})
	openTime(t, r)
	rows := r.listedTimes()
	if len(rows) != 33 || rows[0] != "9:00 AM" || rows[len(rows)-1] != "5:00 PM" {
		t.Fatalf("the list is %d rows from %q to %q, want 33 from 9:00 AM to 5:00 PM", len(rows), rows[0], rows[len(rows)-1])
	}
	r.press(input.KeyEscape, 0)
	r.clearTime(t)
	r.typeText("7:00 am")
	r.press(input.KeyEnter, 0)
	if !nodeTextContains(r.rt, "Enter a time from 9:00 AM to 5:00 PM.") {
		t.Fatal("a time before Min is not refused with the range")
	}
	if !r.value.Peek().Equal(clockOn(9, 0)) {
		t.Fatalf("a refused time changed the value to %v", r.value.Peek())
	}
}

func TestTimePicker24HourLocale(t *testing.T) {
	gb := testLocaleGB()
	r := newTimeRig(t, clockOn(9, 30), gb, nil)
	openTime(t, r)
	if got := r.highlightedTime(); got != "09:30" {
		t.Fatalf("in a 24 hour locale the list highlights %q, want 09:30", got)
	}
	r.press(input.KeyEscape, 0)
	r.clearTime(t)
	r.typeText("21:45")
	r.press(input.KeyEnter, 0)
	if !r.value.Peek().Equal(clockOn(21, 45)) {
		t.Fatalf("21:45 chose %v", r.value.Peek())
	}
}

func TestTimePickerAccessTree(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 30), nil, nil)
	box := r.stateOf(t, access.RoleComboBox, "Start time")
	if box.Value != "9:30 AM" {
		t.Fatalf("the combo box value is %q", box.Value)
	}
	if box.States.Has(access.StateExpanded) {
		t.Fatal("a closed time picker is expanded")
	}
	openTime(t, r)
	if !r.stateOf(t, access.RoleComboBox, "Start time").States.Has(access.StateExpanded) {
		t.Fatal("an open time picker is not expanded")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("development warnings: %v", r.warnings)
	}
}

func TestTimePickerSizingAndRequired(t *testing.T) {
	r := newTimeRig(t, time.Time{}, nil, nil)
	if w := r.nodeByKey(t, "start").style.Width; w != Px(timePickerWidth) {
		t.Fatalf("default width %v, want %d px", w, timePickerWidth)
	}
	r = newTimeRig(t, time.Time{}, nil, func(p *TimePickerElement) *TimePickerElement { return p.Width(Full) })
	if w := r.nodeByKey(t, "start").style.Width; w != Full {
		t.Fatalf("width %v, want Full", w)
	}
	v := NewSignal(time.Time{})
	p := TimePicker(v).Validate(Required[time.Time]())
	if err := p.formCheck(); err == nil {
		t.Fatal("Required accepts no time")
	}
	v.Set(clockOn(9, 0))
	if err := p.formCheck(); err != nil {
		t.Fatalf("Required rejects a time: %v", err)
	}
}

func TestTimePickerMinuteStepIgnoresBadValues(t *testing.T) {
	p := TimePicker(NewSignal(time.Time{})).MinuteStep(0)
	if p.step != timePickerDefaultStep {
		t.Fatalf("a step of 0 changed the step to %d", p.step)
	}
	if got := len(p.MinuteStep(30).times()); got != 48 {
		t.Fatalf("a 30 minute step has %d times, want 48", got)
	}
}

func TestNextTimeIndex(t *testing.T) {
	times := []int{0, 15, 30, 45}
	if i := nextTimeIndex(times, 20, 1); i != 2 {
		t.Errorf("the next time after 0:20 is index %d, want 2", i)
	}
	if i := nextTimeIndex(times, 20, -1); i != 1 {
		t.Errorf("the time before 0:20 is index %d, want 1", i)
	}
	if i := nextTimeIndex(times, -1, -1); i != 3 {
		t.Errorf("with no time, Down starts at the last index, got %d", i)
	}
	if i := nextTimeIndex(times, 45, 1); i != -1 {
		t.Errorf("after the last time there is no next one, got %d", i)
	}
}

func TestTimePickerTabClosesWithoutWarnings(t *testing.T) {
	r := newTimeRig(t, clockOn(9, 30), nil, nil)
	openTime(t, r)
	r.press(input.KeyTab, 0)
	if r.timeExpanded(t) {
		t.Fatal("Tab did not close the list")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("development warnings: %v", r.warnings)
	}
}
