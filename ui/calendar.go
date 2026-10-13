package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// dayOf returns the day of t, at local midnight.
func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// sameDay reports whether a and b fall on the same day.
func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// addMonths returns the day n months from t. It keeps the day of the month, or the last
// day of the month when the new month is shorter, so that January 31 plus one month is
// February 28 (or 29), not March 3.
func addMonths(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, time.Local)
	d := min(t.Day(), daysIn(first.Year(), first.Month()))
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.Local)
}

// weekStart returns the first day of the week of t, for a week that starts on first.
func weekStart(t time.Time, first time.Weekday) time.Time {
	d := dayOf(t)
	return d.AddDate(0, 0, -((int(d.Weekday()) - int(first) + 7) % 7))
}

// monthGrid returns the days shown for the month of t: six weeks of seven days, from the
// week that holds the first of the month. The days of the previous and the next month
// that fill the weeks are in it too, so a month always takes the same six rows.
func monthGrid(t time.Time, first time.Weekday) [6][7]time.Time {
	start := weekStart(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local), first)
	var g [6][7]time.Time
	for r := range 6 {
		for c := range 7 {
			g[r][c] = start.AddDate(0, 0, r*7+c)
		}
	}
	return g
}

// calendarView is what a calendar shows, and the functions it calls. A calendar is the popup of
// a DatePicker or of a DateRangePicker, and widget is the name of that widget, which names the
// parts of the calendar ("DatePicker" or "DateRangePicker"). For a range, from and to are its first
// and last day, or zero, and inRange is the look of the days between them; hover is set for a range
// and is nil for a date.
type calendarView struct {
	st       DatePickerStyle
	widget   string
	loc      *dateLocale
	scale    float32
	value    time.Time // the chosen day, or zero
	from, to time.Time // the first and the last day of a range, or zero
	inRange  Style     // DayInRange: the look of the days between from and to
	focus    time.Time // the day the keys have reached; its month is shown
	today    time.Time
	firstSet bool // FirstDay was called, and firstDay is the week's first day
	firstDay time.Weekday
	name     string
	fallback string // the name of the calendar when name is empty
	move     func(day time.Time)
	pick     func(day time.Time)
	keys     func(e *KeyEvent)
	shut     func(restore bool)
	enabled  func(day time.Time) bool
	hover    func(day time.Time) // the pointer is over day, or zero when it leaves the calendar
}

// calendarKeys handles a key that reaches a calendar, while its focus is on the day f. move moves
// the day the keys have reached, pick chooses a day, shut closes the calendar, and first is the
// first day of the week. The keys are those of the WAI-ARIA date picker dialog pattern (see the
// README, Date and time); a key that is not one of them is left to the field.
func calendarKeys(e *KeyEvent, f time.Time, first time.Weekday, move, pick func(day time.Time), shut func(restore bool)) {
	if e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
		return
	}
	shift := e.Mods&input.ModShift != 0
	switch e.Key {
	case input.KeyLeft:
		move(dayOf(f).AddDate(0, 0, -1))
	case input.KeyRight:
		move(dayOf(f).AddDate(0, 0, 1))
	case input.KeyUp:
		move(dayOf(f).AddDate(0, 0, -7))
	case input.KeyDown:
		move(dayOf(f).AddDate(0, 0, 7))
	case input.KeyHome:
		move(weekStart(f, first))
	case input.KeyEnd:
		move(weekStart(f, first).AddDate(0, 0, 6))
	case input.KeyPageUp:
		move(addMonths(f, -monthsStep(shift)))
	case input.KeyPageDown:
		move(addMonths(f, monthsStep(shift)))
	case input.KeyEnter, input.KeySpace:
		if !e.Repeat {
			pick(f)
		}
	case input.KeyEscape:
		if !e.Repeat {
			shut(true)
		}
	case input.KeyTab:
		shut(true)
		return // not stopped: focus moves on from the calendar
	default:
		return
	}
	e.StopPropagation()
}

// calendarPanel returns the open calendar: an overlay below the field (above it when there is no
// room). It is a calendar for assistive technology, a grid of six week rows, each with seven days,
// under a header of the weekday names (see the README, Date and time). It takes focus when it
// opens, and the day the keys have reached is its highlighted cell, which the screen reader reads.
// The navigation buttons are for a pointer; the keys are PageUp and PageDown for a month, and Shift
// with them for a year.
func calendarPanel(c calendarView) Element {
	first := c.loc.firstDay
	if c.firstSet {
		first = c.firstDay
	}
	// The calendar is named by its field, as a combo box's list is. A name must not carry the
	// control type, so the word "calendar" is not in it (Axe's rule for names).
	name := c.name
	if name == "" {
		name = c.fallback
	}
	st := c.st
	panel := Div().asPart(c.widget+".Popup").Key("jamb.calendar").Style(st.Popup).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: 140 * time.Millisecond}).
		Exit(Fade().Grow(0.98), Animation{Duration: 90 * time.Millisecond}).
		Overlay(BelowStart).OverlayGap(4).
		Role(access.RoleCalendar).SelectionContainer().Label(name).
		AutoFocus().TabStop(false).OnKeyDown(c.keys).
		OnClickOutside(func() { c.shut(false) }).
		OnClick(func() {}) // a press on the calendar's padding must not reach the field
	if c.hover != nil {
		panel.OnMouseLeave(func() { c.hover(time.Time{}) })
	}
	pa := panel.access()
	pa.gridRows, pa.gridCols = 6, 7
	focus := c.focus
	nav := func(label string, glyph *icons.Icon, text string, step time.Time) *DivElement {
		b := Div().asPart(c.widget + ".NavButton").Style(st.NavButton).Role(access.RoleButton).Label(label).
			Cursor(CursorPointer).OnClick(func() { c.move(step) })
		if glyph != nil {
			b.Child(Icon(glyph).Size(14 * c.scale))
		} else {
			b.Child(Text(text).NoWrap())
		}
		return b
	}
	header := Div().Row().Align(Center).Gap(4 * c.scale).
		Child(nav("Previous year", nil, "«", addMonths(focus, -12))).
		Child(nav("Previous month", icons.ChevronLeft, "", addMonths(focus, -1))).
		Child(Div().asPart(c.widget + ".Header").Style(st.Header).Child(Text(c.loc.monthYear(focus)).NoWrap())).
		Child(nav("Next month", icons.ChevronRight, "", addMonths(focus, 1))).
		Child(nav("Next year", nil, "»", addMonths(focus, 12)))
	panel.Child(header)

	// The weekday names are header items of the grid, which the grid is given by its
	// children, so they are not wrapped in a named element.
	names := Div().Row().Gap(2 * c.scale)
	for i := range 7 {
		wd := time.Weekday((int(first) + i) % 7)
		names.Child(Div().asPart(c.widget + ".WeekdayLabel").Style(st.WeekdayLabel).
			Role(access.RoleHeaderItem).Label(c.loc.weekdayLong(wd)).
			Child(Text(c.loc.weekdayShort(wd)).NoWrap()))
	}
	panel.Child(names)

	grid := monthGrid(focus, first)
	for r := range 6 {
		week := Div().Row().Gap(2 * c.scale).Label(c.loc.longDate(grid[r][0]))
		week.listItem(r+1, 6)
		for col := range 7 {
			week.Child(calendarDay(c, grid[r][col], r+1, col+1))
		}
		panel.Child(week)
	}
	return panel
}

// calendarDay returns the cell of one day of a calendar, at the grid position row and column (both
// 1-based). It is a data item of the grid, named by the long date of the day. It is selected when the
// day is the chosen one, or the start or the end of a range. A day between the ends of a range is
// drawn with the look of the band, and a day that cannot be chosen is disabled.
func calendarDay(c calendarView, day time.Time, row, col int) *DivElement {
	st := c.st
	start := !c.from.IsZero() && sameDay(day, c.from)
	end := !c.to.IsZero() && sameDay(day, c.to)
	between := !c.from.IsZero() && !c.to.IsZero() && day.After(c.from) && day.Before(c.to)
	on := start || end || (!c.value.IsZero() && sameDay(day, c.value))
	ok := c.enabled(day)
	cell := Div().asPart(c.widget + ".Day").Style(st.Day).Role(access.RoleDataItem).
		Label(c.loc.longDate(day)).Child(Text(fmt.Sprintf("%d", day.Day())).NoWrap())
	a := cell.access()
	a.gridRow, a.gridCol = row, col
	cell.AccessSelected(on)
	if day.Month() != c.focus.Month() || day.Year() != c.focus.Year() {
		cell.Style(st.DayOutsideMonth)
	}
	// The description tells what the day is in the calendar: today, and its place in a range.
	var notes []string
	if sameDay(day, c.today) {
		cell.Style(st.DayToday)
		notes = append(notes, "Today")
	}
	if start {
		notes = append(notes, "Start of range")
	}
	if end {
		notes = append(notes, "End of range")
	}
	if between {
		cell.Style(c.inRange)
		notes = append(notes, "In range")
	}
	if len(notes) > 0 {
		cell.Description(strings.Join(notes, ", "))
	}
	if on {
		cell.Style(st.DaySelected)
	}
	if !ok {
		a.disabled = true
		cell.Style(st.DayDisabled)
	} else {
		cell.Hover(st.DayHover).Cursor(CursorPointer).OnClick(func() { c.pick(day) })
	}
	if c.hover != nil {
		cell.OnMouseEnter(func() { c.hover(day) })
	}
	if sameDay(day, c.focus) {
		a.highlighted = true
		cell.Style(st.DayFocusVisible)
	}
	return cell
}
