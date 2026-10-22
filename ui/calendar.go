package ui

import (
	"strconv"
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
// a DatePicker or of a DateRangePicker, or a Calendar or CalendarRange on the page, and widget is
// the name that names its parts ("DatePicker", "DateRangePicker" or "Calendar"). For a range, from
// and to are its first and last day, or zero, drawn with st.DayInRange between them; hover is set
// for a range and is nil for a date.
type calendarView struct {
	st       CalendarStyle
	widget   string
	loc      *dateLocale
	scale    float32
	value    time.Time // the chosen day, or zero
	from, to time.Time // the first and the last day of a range, or zero
	focus    time.Time // the day the keys have reached; its month is shown
	ring     bool      // the day the keys have reached is drawn with DayFocusVisible
	disabled bool      // the calendar does not change: its buttons do nothing
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
	// motion is the theme's motion, for the panel's entrance and exit.
	motion MotionTokens
}

// calendarKeys handles a key that reaches a calendar, while its focus is on the day f. move moves
// the day the keys have reached, pick chooses a day, shut closes the calendar, and first is the
// first day of the week. The keys are those of the WAI-ARIA date picker dialog pattern (see the
// docs/widgets-selection.md, Date and time); a key that is not one of them is left to the field.
// A calendar on the page has nothing to close: its shut is nil, and Escape and Tab are left to
// the elements around it.
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
		if shut == nil {
			return
		}
		if !e.Repeat {
			shut(true)
		}
	case input.KeyTab:
		if shut != nil {
			shut(true)
		}
		return // not stopped: focus moves on from the calendar
	default:
		return
	}
	e.StopPropagation()
}

// calendarPanel returns the open calendar of a picker: an overlay below the field (above it when
// there is no room), which holds the month of calendarGrid. It takes focus when it opens, and the
// day the keys have reached is its highlighted cell, which the screen reader reads.
func calendarPanel(c calendarView, s *Strings) Element {
	panel := Div().asPart(c.widget+".Popup").Key("jamb.calendar").Style(c.st.Root).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: c.motion.enter(140 * time.Millisecond)}).
		Exit(Fade().Grow(0.98), Animation{Duration: c.motion.exit(90 * time.Millisecond)}).
		Overlay(BelowStart).OverlayGap(4).
		AutoFocus().TabStop(false).OnKeyDown(c.keys).
		OnClickOutside(func() { c.shut(false) }).
		OnClick(func() {}) // a press on the calendar's padding must not reach the field
	return calendarGrid(panel, c, s)
}

// calendarGrid fills panel with the month that c shows, and returns it. The panel is a calendar for
// assistive technology, a grid of six week rows, each with seven days, under a header of the weekday
// names (see docs/widgets-selection.md, Date and time). The navigation buttons are for a pointer; the
// keys are PageUp and PageDown for a month, and Shift with them for a year.
func calendarGrid(panel *DivElement, c calendarView, s *Strings) *DivElement {
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
	panel.Role(access.RoleCalendar).SelectionContainer().Label(name)
	if c.hover != nil {
		panel.OnMouseLeave(func() { c.hover(time.Time{}) })
	}
	panel.AccessGrid(6, 7)
	focus := c.focus
	nav := func(label string, glyph *icons.Icon, text string, step time.Time) *DivElement {
		b := Div().asPart(c.widget + ".NavButton").Style(st.NavButton).Role(access.RoleButton).Label(label)
		if c.disabled {
			b.access().disabled = true
		} else {
			b.Cursor(CursorPointer).OnClick(func() { c.move(step) })
		}
		if glyph != nil {
			b.Child(Icon(glyph).Size(14 * c.scale))
		} else {
			b.Child(Text(text).NoWrap())
		}
		return b
	}
	header := Div().Row().Align(Center).Gap(4 * c.scale).
		Child(nav(s.PreviousYear, nil, "«", addMonths(focus, -12))).
		Child(nav(s.PreviousMonth, icons.ChevronLeft, "", addMonths(focus, -1))).
		Child(Div().asPart(c.widget + ".Header").Style(st.Header).Child(Text(c.loc.monthYear(focus)).NoWrap())).
		Child(nav(s.NextMonth, icons.ChevronRight, "", addMonths(focus, 1))).
		Child(nav(s.NextYear, nil, "»", addMonths(focus, 12)))
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
			week.Child(calendarDay(c, s, grid[r][col], r+1, col+1))
		}
		panel.Child(week)
	}
	return panel
}

// rangeShade returns the first and the last day that a range calendar shades: the range from start
// to end, or, while the end is being chosen after the start pending, the range from pending to the
// day under the pointer (hover), or to the day the keys have reached (focus) when the pointer is not
// over a day. A day that enabled rules out shades the start alone.
func rangeShade(start, end, pending, hover, focus time.Time, enabled func(day time.Time) bool) (time.Time, time.Time) {
	if pending.IsZero() {
		return orderedSpan(start, end)
	}
	day := hover
	if day.IsZero() {
		day = focus
	}
	if !enabled(day) {
		day = time.Time{}
	}
	return orderedSpan(pending, day)
}

// calendarDay returns the cell of one day of a calendar, at the grid position row and column (both
// 1-based). It is a data item of the grid, named by the long date of the day. It is selected when the
// day is the chosen one, or the start or the end of a range. A day between the ends of a range is
// drawn with the look of the band, and a day that cannot be chosen is disabled.
func calendarDay(c calendarView, s *Strings, day time.Time, row, col int) *DivElement {
	st := c.st
	start := !c.from.IsZero() && sameDay(day, c.from)
	end := !c.to.IsZero() && sameDay(day, c.to)
	between := !c.from.IsZero() && !c.to.IsZero() && day.After(c.from) && day.Before(c.to)
	on := start || end || (!c.value.IsZero() && sameDay(day, c.value))
	ok := c.enabled(day)
	cell := Div().asPart(c.widget + ".Day").Style(st.Day).Role(access.RoleDataItem).
		Label(c.loc.longDate(day)).Child(Text(strconv.Itoa(day.Day())).NoWrap())
	a := cell.access()
	cell.AccessCell(row, col).AccessSelected(on)
	if day.Month() != c.focus.Month() || day.Year() != c.focus.Year() {
		cell.Style(st.DayOutsideMonth)
	}
	// The description tells what the day is in the calendar: today, and its place in a range.
	var notes []string
	if sameDay(day, c.today) {
		cell.Style(st.DayToday)
		notes = append(notes, s.Today)
	}
	if start {
		notes = append(notes, s.StartOfRange)
	}
	if end {
		notes = append(notes, s.EndOfRange)
	}
	if between {
		cell.Style(st.DayInRange)
		notes = append(notes, s.InRange)
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
		pick := c.pick // the closure keeps the function, not the whole calendar
		cell.Hover(st.DayHover).Cursor(CursorPointer).OnClick(func() { pick(day) })
	}
	if hover := c.hover; hover != nil {
		cell.OnMouseEnter(func() { hover(day) })
	}
	if sameDay(day, c.focus) {
		a.highlighted = true
		if c.ring {
			cell.Style(st.DayFocusVisible)
		}
	}
	return cell
}

// CalendarStyle is the look of each part of a Calendar and of a CalendarRange, a month of days on
// the page. A part's style merges over the built-in defaults, then over the theme's Calendar, then
// over the Styles of the instance; see docs/styling.md.
//
// The parts are those of a DatePicker's calendar, with the same names and the same defaults: Root is
// the panel of the month (a DatePicker's Popup, without the shadow), Header the row with the month
// and the year, NavButton each of its navigation buttons, WeekdayLabel the name of each day of the
// week, and Day each day. The states of a day are DayHover, DaySelected (the chosen day, or an end
// of a range), DayInRange (a day between the ends, drawn as a band; a CalendarRange only), DayToday,
// DayDisabled (see Min, Max and DisableDays), DayOutsideMonth and DayFocusVisible, the day the keys
// have reached while the calendar has keyboard focus. RootDisabled is the panel of a calendar that
// is Disabled. Under high contrast the chosen day is drawn in the system Highlight colors, and today
// and the focused day by their borders, so that no state is shown by color alone.
type CalendarStyle struct {
	// Root is the panel of the month.
	Root Style
	// RootDisabled is the panel when the calendar is disabled.
	RootDisabled Style
	// Header is the row with the month and the year.
	Header Style
	// NavButton is each button that moves the calendar to another month or year.
	NavButton Style
	// WeekdayLabel is the name of each day of the week.
	WeekdayLabel Style
	// Day is each day of the calendar.
	Day Style
	// DayHover is a day under the pointer.
	DayHover Style
	// DaySelected is the day that is chosen, or a day at either end of a range.
	DaySelected Style
	// DayInRange is a day between the ends of a range.
	DayInRange Style
	// DayToday is today.
	DayToday Style
	// DayDisabled is a day that cannot be chosen.
	DayDisabled Style
	// DayOutsideMonth is a day of the adjacent months.
	DayOutsideMonth Style
	// DayFocusVisible is the day that has keyboard focus.
	DayFocusVisible Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s CalendarStyle) merge(o CalendarStyle) CalendarStyle {
	return CalendarStyle{
		Root:            s.Root.Merge(o.Root),
		RootDisabled:    s.RootDisabled.Merge(o.RootDisabled),
		Header:          s.Header.Merge(o.Header),
		NavButton:       s.NavButton.Merge(o.NavButton),
		WeekdayLabel:    s.WeekdayLabel.Merge(o.WeekdayLabel),
		Day:             s.Day.Merge(o.Day),
		DayHover:        s.DayHover.Merge(o.DayHover),
		DaySelected:     s.DaySelected.Merge(o.DaySelected),
		DayInRange:      s.DayInRange.Merge(o.DayInRange),
		DayToday:        s.DayToday.Merge(o.DayToday),
		DayDisabled:     s.DayDisabled.Merge(o.DayDisabled),
		DayOutsideMonth: s.DayOutsideMonth.Merge(o.DayOutsideMonth),
		DayFocusVisible: s.DayFocusVisible.Merge(o.DayFocusVisible),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s CalendarStyle) plain() CalendarStyle {
	return CalendarStyle{
		Root:            plainStyle(s.Root),
		RootDisabled:    plainStyle(s.RootDisabled),
		Header:          plainStyle(s.Header),
		NavButton:       plainStyle(s.NavButton),
		WeekdayLabel:    plainStyle(s.WeekdayLabel),
		Day:             plainStyle(s.Day),
		DayHover:        plainStyle(s.DayHover),
		DaySelected:     plainStyle(s.DaySelected),
		DayInRange:      plainStyle(s.DayInRange),
		DayToday:        plainStyle(s.DayToday),
		DayDisabled:     plainStyle(s.DayDisabled),
		DayOutsideMonth: plainStyle(s.DayOutsideMonth),
		DayFocusVisible: plainStyle(s.DayFocusVisible),
	}
}

// calendarDefaults returns the built-in look of a calendar at the Text size scale, from the colors
// of t. The days, the header and the weekday names are also those of the pickers' calendars (see
// datePickerDefaults), and the band between the ends of a range is that of a DateRangePicker.
func calendarDefaults(t *Theme, scale float32) CalendarStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	cell := dayCellSize * scale
	d := CalendarStyle{
		Root: Style{Direction: Column, Gap: 4 * scale, Padding: All(8 * scale), Background: t.Surface,
			BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, NoShrink: true, fitWidth: true},
		RootDisabled: Style{Opacity: 0.5},
		Header: Style{Direction: Row, Align: Center, Justify: Center, Grow: 1, Height: Px(28 * scale),
			FontSize: 14, FontWeight: FontSemibold, TextColor: t.Text},
		NavButton: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(28 * scale),
			Height: Px(28 * scale), NoShrink: true, Radius: 4, TextColor: t.TextMuted},
		WeekdayLabel: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(cell), Height: Px(24 * scale),
			NoShrink: true, FontSize: 12, TextColor: t.TextMuted},
		Day: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(cell), Height: Px(cell),
			NoShrink: true, Radius: 4, FontSize: 13, TextColor: t.Text, Transition: t.Motion.hover(), Spring: t.Motion.Spring},
		DayHover:    Style{Background: t.SurfaceHover},
		DaySelected: Style{Background: t.Accent, TextColor: t.AccentText},
		DayInRange: Style{Clear: ClearRadius, BorderColor: t.Accent, BorderEdges: Edges{Top: 1, Bottom: 1},
			Background: t.AccentSoft},
		DayFocusVisible: Style{Shadows: []Shadow{ring}},
	}
	if t.HighContrast {
		// A disabled calendar is not dimmed: its days are GrayText, as a disabled day is.
		d.RootDisabled = Style{Opacity: 1}
		d.DaySelected = Style{Background: t.Accent, TextColor: t.AccentText}
		d.DayInRange = Style{Clear: ClearRadius, BorderColor: t.Text, BorderEdges: Edges{Top: 2, Bottom: 2}}
		d.DayToday = Style{BorderWidth: 1, BorderColor: t.Text}
		d.DayDisabled = Style{TextColor: t.Disabled}
		// The focus is an inner outline, as the focus of a number input's buttons is.
		d.DayFocusVisible = Style{BorderWidth: 2, BorderColor: t.FocusRing}
	} else {
		d.DayToday = Style{BorderWidth: 1, BorderColor: t.Accent}
		d.DayDisabled = Style{Opacity: 0.45}
		d.DayOutsideMonth = Style{TextColor: t.TextSubtle}
	}
	return d
}

// resolveCalendar returns the look of a calendar: the defaults, merged with the theme's Calendar
// and then the instance's Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveCalendar(ctx *Context, scale float32, inst CalendarStyle) CalendarStyle {
	t := ctx.Theme()
	return layered(ctx, calendarDefaults(t, scale), CalendarStyle.merge, CalendarStyle.plain,
		themeLayer(t.Calendar), instanceLayer(inst))
}

// dayAllowed reports whether day can be chosen: it is not before earliest or after latest (a zero
// time is no limit), and disable, when set, does not rule it out.
func dayAllowed(day, earliest, latest time.Time, disable func(day time.Time) bool) bool {
	day = dayOf(day)
	switch {
	case !earliest.IsZero() && day.Before(dayOf(earliest)):
		return false
	case !latest.IsZero() && day.After(dayOf(latest)):
		return false
	case disable != nil && disable(day):
		return false
	}
	return true
}

// calendarOptions are the settings that a Calendar and a CalendarRange share: the days that can be
// chosen, the week, the name and the box.
type calendarOptions struct {
	min, max    time.Time // the earliest and the latest day that can be chosen; zero for none
	disableDays func(day time.Time) bool
	firstDay    time.Weekday
	firstSet    bool   // FirstDay was called; otherwise the week starts on the locale's first day
	name        string // the accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	size        sizing
	styles      CalendarStyle
	focusHandle *FocusHandle // see FocusHandle
	tooltip     string       // see Tooltip
}

// enabled reports whether day can be chosen: the calendar is not disabled, the day is in the range
// of Min and Max, and DisableDays does not rule it out.
func (o *calendarOptions) enabled(day time.Time) bool {
	return !o.disabled && dayAllowed(day, o.min, o.max, o.disableDays)
}

// first returns the first day of the week, of FirstDay or of the locale.
func (o *calendarOptions) first(loc *dateLocale) time.Weekday {
	if o.firstSet {
		return o.firstDay
	}
	return loc.firstDay
}

// view returns the panel of a calendar on the page, which shows c with the settings of o. The panel
// is a Tab stop with the keys of keys, unless the calendar is disabled. The rest of c (the value or
// the range, and the functions) is the caller's.
func (o *calendarOptions) view(ctx *Context, c calendarView, keys func(e *KeyEvent)) *DivElement {
	root := Div().asPart("Calendar.Root").Style(c.st.Root).Tooltip(o.tooltip)
	if o.key != "" {
		root.Key(o.key)
	}
	if len(o.labelledBy) > 0 {
		root.LabelledBy(o.labelledBy...)
	}
	c.widget, c.name = "Calendar", o.name
	c.firstSet, c.firstDay, c.disabled = o.firstSet, o.firstDay, o.disabled
	if o.disabled {
		root.access().disabled = true
		root.Style(c.st.RootDisabled).Cursor(CursorNotAllowed)
	} else {
		root.Focusable().OnKeyDown(keys)
		if o.focusHandle != nil {
			root.FocusHandle(o.focusHandle)
		}
		// The day the keys have reached is outlined while the calendar has keyboard focus, as a
		// focus ring is drawn; a screen reader hears it as the focused day at any time.
		c.ring = ctx.KeyboardFocus()
	}
	return o.size.apply(calendarGrid(root, c, ctx.Strings()))
}

// CalendarElement is a month of days on the page, for choosing a day, bound to a time.Time
// signal: the month grid of a DatePicker's calendar, without the field and the popup. Its day is
// the chosen day and its time of day is kept, as a DatePicker keeps it; a zero time is no date.
//
// The calendar is a Tab stop. Its keys are those of a DatePicker's calendar (see
// docs/widgets-selection.md, Date and time): the arrows move by a day and a week, Home and End
// to the ends of the week, Page Up and Page Down by a month, with Shift by a year, and Enter or
// Space choose the day the keys have reached. Escape and Tab are left to the page. A click
// chooses a day, and the buttons of the header move by a month or a year. A day that is not
// between Min and Max, or that DisableDays rules out, cannot be chosen: it is disabled.
type CalendarElement struct {
	calendarOptions
	value    *Signal[time.Time]
	onChange func(time.Time)
}

// Calendar creates a calendar that reads and writes value. It shows the month of the chosen day, or
// of today when there is none. The week starts on the first day of the user's locale.
func Calendar(value *Signal[time.Time]) *CalendarElement {
	return &CalendarElement{value: value}
}

func (*CalendarElement) isElement()           {}
func (c *CalendarElement) elementKey() string { return c.key }

// Key identifies the calendar among its siblings (see DivElement.Key).
func (c *CalendarElement) Key(k string) *CalendarElement { c.key = k; return c }

// Label names the calendar for assistive technology, such as "Due date". Without a label it is
// named by Strings.DateField.
func (c *CalendarElement) Label(name string) *CalendarElement { c.name = name; return c }

// LabelledBy names the calendar by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (c *CalendarElement) LabelledBy(keys ...string) *CalendarElement {
	c.labelledBy = keys
	return c
}

// Disabled shows the calendar dimmed, with its day, and does not let it change.
func (c *CalendarElement) Disabled(on bool) *CalendarElement { c.disabled = on; return c }

// Min sets the earliest day that can be chosen. Its time of day is ignored; a zero time
// means there is no earliest day.
func (c *CalendarElement) Min(day time.Time) *CalendarElement { c.min = day; return c }

// Max sets the latest day that can be chosen. Its time of day is ignored; a zero time means
// there is no latest day.
func (c *CalendarElement) Max(day time.Time) *CalendarElement { c.max = day; return c }

// DisableDays rules out the days for which fn returns true, such as weekends. fn is called
// with each day at local midnight. A disabled day is drawn dimmed, is marked unavailable to
// assistive technology, and cannot be chosen.
func (c *CalendarElement) DisableDays(fn func(day time.Time) bool) *CalendarElement {
	c.disableDays = fn
	return c
}

// FirstDay sets the first day of the week. The default is the first day of the week of the
// user's locale.
func (c *CalendarElement) FirstDay(day time.Weekday) *CalendarElement {
	c.firstDay, c.firstSet = day, true
	return c
}

// OnChange runs fn with the new value after the user chooses a day. The signal is set
// first. It does not run when the value is set from outside.
func (c *CalendarElement) OnChange(fn func(time.Time)) *CalendarElement { c.onChange = fn; return c }

// FocusHandle lets code focus the calendar (see NewFocusHandle).
func (c *CalendarElement) FocusHandle(h *FocusHandle) *CalendarElement { c.focusHandle = h; return c }

// Tooltip shows s below the calendar after the pointer rests on it. Assistive technology
// reads it as the calendar's description, unless it has another.
func (c *CalendarElement) Tooltip(s string) *CalendarElement { c.tooltip = s; return c }

// The sizing methods give the calendar's panel a size. A calendar is as wide and as high as
// its days by default; see Sizing in docs/styling.md.
func (c *CalendarElement) Width(l Length) *CalendarElement { c.size.setWidth(l); return c }

// Height sets the height of the calendar.
func (c *CalendarElement) Height(l Length) *CalendarElement { c.size.setHeight(l); return c }

// MinWidth sets the smallest width of the calendar.
func (c *CalendarElement) MinWidth(l Length) *CalendarElement { c.size.setMinWidth(l); return c }

// MinHeight sets the smallest height of the calendar.
func (c *CalendarElement) MinHeight(l Length) *CalendarElement { c.size.setMinHeight(l); return c }

// MaxWidth sets the largest width of the calendar.
func (c *CalendarElement) MaxWidth(l Length) *CalendarElement { c.size.setMaxWidth(l); return c }

// MaxHeight sets the largest height of the calendar.
func (c *CalendarElement) MaxHeight(l Length) *CalendarElement { c.size.setMaxHeight(l); return c }

// Grow makes the calendar take a share of the free space on its parent's main axis.
func (c *CalendarElement) Grow(f float32) *CalendarElement { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the calendar covers in its parent's grid.
func (c *CalendarElement) GridSpan(cols, rows int) *CalendarElement {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the calendar, for this instance only. Set fields win
// over the theme's Calendar and the defaults; see CalendarStyle and docs/styling.md. Calling
// Styles again merges with the earlier values.
func (c *CalendarElement) Styles(st CalendarStyle) *CalendarElement {
	c.styles = c.styles.merge(st)
	return c
}

func (c *CalendarElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		s := ctx.Strings()
		loc := currentLocale()
		scale := textScale(ctx)
		st := resolveCalendar(ctx, scale, c.styles)
		v := c.value.Get()
		today := dayOf(rt.now())
		start := today
		if !v.IsZero() {
			start = dayOf(v)
		}
		// focus is the day the keys have reached; the calendar shows its month. seen is the value
		// it was last moved to, so that a value set from outside shows its month.
		focus := Local(ctx, "focus", start)
		seen := Local(ctx, "seen", v)
		if !v.Equal(seen.Peek()) {
			seen.Set(v)
			if !v.IsZero() {
				focus.Set(dayOf(v))
			}
		}
		// moveTo moves the keys' day to day, and announces the month when it changes.
		moveTo := func(day time.Time) {
			old := focus.Peek()
			focus.Set(day)
			if old.Year() != day.Year() || old.Month() != day.Month() {
				Announce(loc.monthYear(day), access.Polite)
			}
		}
		// pick chooses day, and keeps the time of day of the value. A day that cannot be chosen
		// is ignored.
		pick := func(day time.Time) {
			if !c.enabled(day) {
				return
			}
			moveTo(day)
			cur := c.value.Peek()
			nv := withClock(day, clockOf(cur))
			if nv.Equal(cur) {
				return
			}
			c.value.Set(nv)
			seen.Set(nv)
			if c.onChange != nil {
				c.onChange(nv)
			}
		}
		keys := func(e *KeyEvent) { calendarKeys(e, focus.Peek(), c.first(loc), moveTo, pick, nil) }
		return c.view(ctx, calendarView{st: st, loc: loc, scale: scale, value: v, focus: focus.Get(), today: today,
			fallback: s.DateField, move: moveTo, pick: pick, enabled: c.enabled}, keys)
	})
}

// CalendarRangeElement is a month of days on the page, for choosing a range of days, bound to two
// time.Time signals: the start and the end, each at local midnight; a zero start is no range. It is
// the month grid of a DateRangePicker's calendar, without the field and the popup.
//
// A range is chosen in two steps, as in a DateRangePicker: the first day chosen is the start, and
// the second is the end. If the end is before the start, the two are swapped. The signals change
// only when the end is chosen. While the end is being chosen, the days from the start to the day
// under the pointer, or under the keys, are shaded as the range would be, and Escape, or focus
// leaving the calendar, drops the start. The keys are those of a Calendar. A day that Min and Max,
// or DisableDays, rule out cannot be chosen as either end; a range may span such days.
type CalendarRangeElement struct {
	calendarOptions
	start, end *Signal[time.Time]
	onChange   func(start, end time.Time)
}

// CalendarRange creates a calendar that reads and writes the start and the end of a range of days,
// from and to. It shows the month of the start, or of today when there is none.
func CalendarRange(from, to *Signal[time.Time]) *CalendarRangeElement {
	return &CalendarRangeElement{start: from, end: to}
}

func (*CalendarRangeElement) isElement()           {}
func (c *CalendarRangeElement) elementKey() string { return c.key }

// Key identifies the calendar among its siblings (see DivElement.Key).
func (c *CalendarRangeElement) Key(k string) *CalendarRangeElement { c.key = k; return c }

// Label names the calendar for assistive technology, such as "Trip dates". Without a label it is
// named by Strings.DateRangeField.
func (c *CalendarRangeElement) Label(name string) *CalendarRangeElement { c.name = name; return c }

// LabelledBy names the calendar by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (c *CalendarRangeElement) LabelledBy(keys ...string) *CalendarRangeElement {
	c.labelledBy = keys
	return c
}

// Disabled shows the calendar dimmed, with its range, and does not let it change.
func (c *CalendarRangeElement) Disabled(on bool) *CalendarRangeElement { c.disabled = on; return c }

// Min sets the earliest day that can be chosen. Its time of day is ignored; a zero time means
// there is no earliest day.
func (c *CalendarRangeElement) Min(day time.Time) *CalendarRangeElement { c.min = day; return c }

// Max sets the latest day that can be chosen. Its time of day is ignored; a zero time means there
// is no latest day.
func (c *CalendarRangeElement) Max(day time.Time) *CalendarRangeElement { c.max = day; return c }

// DisableDays rules out the days for which fn returns true, such as weekends. fn is called with
// each day at local midnight. A disabled day is drawn dimmed, is marked unavailable to assistive
// technology, and cannot be chosen as an end of the range.
func (c *CalendarRangeElement) DisableDays(fn func(day time.Time) bool) *CalendarRangeElement {
	c.disableDays = fn
	return c
}

// FirstDay sets the first day of the week. The default is the first day of the week of the user's
// locale.
func (c *CalendarRangeElement) FirstDay(day time.Weekday) *CalendarRangeElement {
	c.firstDay, c.firstSet = day, true
	return c
}

// OnChange runs fn with the new start and end after the user chooses the end of a range. The
// signals are set first. It does not run when the range is set from outside.
func (c *CalendarRangeElement) OnChange(fn func(start, end time.Time)) *CalendarRangeElement {
	c.onChange = fn
	return c
}

// FocusHandle lets code focus the calendar (see NewFocusHandle).
func (c *CalendarRangeElement) FocusHandle(h *FocusHandle) *CalendarRangeElement {
	c.focusHandle = h
	return c
}

// Tooltip shows s below the calendar after the pointer rests on it. Assistive technology reads it
// as the calendar's description, unless it has another.
func (c *CalendarRangeElement) Tooltip(s string) *CalendarRangeElement { c.tooltip = s; return c }

// The sizing methods give the calendar's panel a size. A calendar is as wide and as high as its
// days by default; see Sizing in docs/styling.md.
func (c *CalendarRangeElement) Width(l Length) *CalendarRangeElement { c.size.setWidth(l); return c }

// Height sets the height of the calendar.
func (c *CalendarRangeElement) Height(l Length) *CalendarRangeElement { c.size.setHeight(l); return c }

// MinWidth sets the smallest width of the calendar.
func (c *CalendarRangeElement) MinWidth(l Length) *CalendarRangeElement {
	c.size.setMinWidth(l)
	return c
}

// MinHeight sets the smallest height of the calendar.
func (c *CalendarRangeElement) MinHeight(l Length) *CalendarRangeElement {
	c.size.setMinHeight(l)
	return c
}

// MaxWidth sets the largest width of the calendar.
func (c *CalendarRangeElement) MaxWidth(l Length) *CalendarRangeElement {
	c.size.setMaxWidth(l)
	return c
}

// MaxHeight sets the largest height of the calendar.
func (c *CalendarRangeElement) MaxHeight(l Length) *CalendarRangeElement {
	c.size.setMaxHeight(l)
	return c
}

// Grow makes the calendar take a share of the free space on its parent's main axis.
func (c *CalendarRangeElement) Grow(f float32) *CalendarRangeElement { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the calendar covers in its parent's grid.
func (c *CalendarRangeElement) GridSpan(cols, rows int) *CalendarRangeElement {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the calendar, for this instance only. Set fields win over
// the theme's Calendar and the defaults; see CalendarStyle and docs/styling.md. Calling Styles
// again merges with the earlier values.
func (c *CalendarRangeElement) Styles(st CalendarStyle) *CalendarRangeElement {
	c.styles = c.styles.merge(st)
	return c
}

func (c *CalendarRangeElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		s := ctx.Strings()
		loc := currentLocale()
		scale := textScale(ctx)
		st := resolveCalendar(ctx, scale, c.styles)
		cur := dateRange{start: c.start.Get(), end: c.end.Get()}
		today := dayOf(rt.now())
		first := today
		if !cur.start.IsZero() {
			first = dayOf(cur.start)
		}
		// focus is the day the keys have reached, and seen the start it was last moved to (see
		// Calendar). pending is the start, once it is chosen and the end is not; zero otherwise.
		// hover is the day under the pointer, or zero.
		focus := Local(ctx, "focus", first)
		seen := Local(ctx, "seen", cur.start)
		pending := Local(ctx, "pending", time.Time{})
		hover := Local(ctx, "hover", time.Time{})
		if !cur.start.Equal(seen.Peek()) {
			seen.Set(cur.start)
			if !cur.start.IsZero() {
				focus.Set(dayOf(cur.start))
			}
		}
		// moveTo moves the keys' day to day, and announces the month when it changes. The pointer's
		// day is forgotten, so that the keys' day is the one shaded.
		moveTo := func(day time.Time) {
			old := focus.Peek()
			focus.Set(day)
			hover.Set(time.Time{})
			if old.Year() != day.Year() || old.Month() != day.Month() {
				Announce(loc.monthYear(day), access.Polite)
			}
		}
		// pick chooses day: the first day chosen is the start, and the second the end, which sets
		// the range. A day that cannot be chosen is ignored.
		pick := func(day time.Time) {
			if !c.enabled(day) {
				return
			}
			moveTo(day)
			p := pending.Peek()
			if p.IsZero() {
				pending.Set(day)
				Announce(s.RangeStartChosen, access.Polite)
				return
			}
			a, b := orderedSpan(p, day)
			pending.Set(time.Time{})
			c.start.Set(a)
			c.end.Set(b)
			seen.Set(a)
			if c.onChange != nil {
				c.onChange(a, b)
			}
		}
		// drop forgets a start that was chosen without its end.
		drop := func(bool) {
			pending.Set(time.Time{})
			hover.Set(time.Time{})
		}
		keys := func(e *KeyEvent) {
			// Escape drops a pending start, and is otherwise left to the page.
			var shut func(bool)
			if !pending.Peek().IsZero() {
				shut = drop
			}
			calendarKeys(e, focus.Peek(), c.first(loc), moveTo, pick, shut)
		}
		from, to := rangeShade(cur.start, cur.end, pending.Get(), hover.Get(), focus.Get(), c.enabled)
		root := c.view(ctx, calendarView{st: st, loc: loc, scale: scale, from: from, to: to, focus: focus.Get(),
			today: today, fallback: s.DateRangeField, move: moveTo, pick: pick, enabled: c.enabled,
			hover: func(day time.Time) { hover.Set(day) }}, keys)
		if !c.disabled {
			root.OnBlur(func() { drop(false) })
		}
		return root
	})
}
