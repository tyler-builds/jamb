package ui

import (
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// dateRangePickerWidth is the default width of a date range picker at the Text size of 100
// percent, in logical pixels. It scales with the Text size.
const dateRangePickerWidth = 280

// DateRangePickerStyle is the look of each part of a date range picker. A part's style merges
// over the built-in defaults, then over the theme's DateRangePicker, then over the Styles of the
// instance; see docs/styling.md.
//
// The parts are those of a DatePicker, and they are drawn in the same way: Field is the text frame,
// and CalendarButton, Message, and the parts of the calendar (Popup, Header, NavButton, WeekdayLabel,
// Day, DayHover, DayToday, DayDisabled, DayOutsideMonth and DayFocusVisible) are as they are there
// (see DatePickerStyle). DaySelected is the start and the end of the range. DayInRange is each day
// between them, which is drawn as a band, with a line above and below and square corners, so that
// the range is not shown by its fill alone. Under high contrast the ends are drawn in the system
// Highlight colors, and the days between by their lines, in the text color.
type DateRangePickerStyle struct {
	// Field is the text frame of the date range picker.
	Field FieldStyle
	// CalendarButton is the button that opens the calendar.
	CalendarButton Style
	// Message is the message under the field.
	Message Style
	// Popup is the panel of the calendar.
	Popup Style
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
	// DaySelected is a day at either end of the range.
	DaySelected Style
	// DayInRange is a day between the ends of the range.
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

// dates returns the parts that a date picker's field and calendar have, which the range's
// DayInRange is not among.
func (s DateRangePickerStyle) dates() DatePickerStyle {
	return DatePickerStyle{
		Field:           s.Field,
		CalendarButton:  s.CalendarButton,
		Message:         s.Message,
		Popup:           s.Popup,
		Header:          s.Header,
		NavButton:       s.NavButton,
		WeekdayLabel:    s.WeekdayLabel,
		Day:             s.Day,
		DayHover:        s.DayHover,
		DaySelected:     s.DaySelected,
		DayToday:        s.DayToday,
		DayDisabled:     s.DayDisabled,
		DayOutsideMonth: s.DayOutsideMonth,
		DayFocusVisible: s.DayFocusVisible,
	}
}

// rangeStyle returns the parts of a date range picker: the parts of d, which are those of a date
// picker, and the look of the days between the ends of the range, in.
func rangeStyle(d DatePickerStyle, in Style) DateRangePickerStyle {
	return DateRangePickerStyle{
		Field:           d.Field,
		CalendarButton:  d.CalendarButton,
		Message:         d.Message,
		Popup:           d.Popup,
		Header:          d.Header,
		NavButton:       d.NavButton,
		WeekdayLabel:    d.WeekdayLabel,
		Day:             d.Day,
		DayHover:        d.DayHover,
		DaySelected:     d.DaySelected,
		DayInRange:      in,
		DayToday:        d.DayToday,
		DayDisabled:     d.DayDisabled,
		DayOutsideMonth: d.DayOutsideMonth,
		DayFocusVisible: d.DayFocusVisible,
	}
}

// merge returns s with the set fields of o applied on top, part by part.
func (s DateRangePickerStyle) merge(o DateRangePickerStyle) DateRangePickerStyle {
	return rangeStyle(s.dates().merge(o.dates()), s.DayInRange.Merge(o.DayInRange))
}

// plain returns s as high contrast allows it (see plainStyle).
func (s DateRangePickerStyle) plain() DateRangePickerStyle {
	return rangeStyle(s.dates().plain(), plainStyle(s.DayInRange))
}

// dateRangePickerDefaults returns the built-in look of a date range picker at the Text size scale,
// from the colors of t: the look of a date picker, and the days between the ends drawn as a band.
func dateRangePickerDefaults(t *Theme, scale float32, disabled bool) DateRangePickerStyle {
	return rangeStyle(datePickerDefaults(t, scale, disabled), calendarDefaults(t, scale).DayInRange)
}

// resolveDateRangePicker returns the look of a date range picker: the defaults, merged with the
// theme's DateRangePicker and then the instance's Styles. Under high contrast the overrides are
// plain (see plainStyle).
func resolveDateRangePicker(ctx *Context, scale float32, disabled bool, inst DateRangePickerStyle) DateRangePickerStyle {
	t := ctx.Theme()
	return layered(ctx, dateRangePickerDefaults(t, scale, disabled), DateRangePickerStyle.merge, DateRangePickerStyle.plain,
		themeLayer(t.DateRangePicker), instanceLayer(inst))
}

// DateRangePickerElement is a text field for a range of days, with a calendar that opens below it.
// The text is the two days in the short date format of the user's locale, joined by an en dash
// ("1/3/2026 – 1/9/2026"), and it is the start alone while the end is not chosen. The start and the
// end are two signals of time.Time, each at local midnight; a zero start is no range.
//
// The calendar chooses a range in two steps: the first day chosen is the start, and the second is the
// end, which closes the calendar. If the end is before the start, the two are swapped. The signals
// change only when the end is chosen, so Escape, Tab, and a click outside the calendar keep the range
// that was there. While the end is being chosen, the days from the start to the day under the pointer,
// or under the keys, are shaded as the range would be. A day that Min and Max, or DisableDays, rule out
// cannot be chosen as either end; a range may span such days. Typed text is "date – date" or
// "date - date", each date in the locale's format, and it is refused with a message when either date
// cannot be chosen.
//
// The keys of the calendar follow the WAI-ARIA date picker dialog pattern (see docs/widgets-selection.md, Date and
// time). Alt+Down and F4 open it, from the start of the range, or from today when there is none.
type DateRangePickerElement struct {
	start, end  *Signal[time.Time]
	min, max    time.Time // the earliest and the latest day that can be chosen; zero for none
	disableDays func(day time.Time) bool
	firstDay    time.Weekday
	firstSet    bool // FirstDay was called; otherwise the week starts on the locale's first day
	placeholder string
	name        string // the accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	size        sizing
	styles      DateRangePickerStyle
	onChange    func(start, end time.Time)
	rules       []Validator[DateRange] // the validators of the range (see Validate)
	wire        fieldWire              // what a Field gives the field (see Field)
	focusHandle *FocusHandle           // see FocusHandle
	ctlSize     controlSize            // see Small and Large
	tooltip     string                 // see Tooltip
}

// DateRange is a range of days, as the validators of a date range picker see it (see
// DateRangePickerElement.Validate): its first day and its last, each at local midnight. A
// zero Start is no range.
type DateRange struct {
	Start, End time.Time
}

// DateRangePicker creates a date range picker that reads and writes the start and the end of a
// range. The default width is 280 px at the Text size of 100 percent. A zero start is no range, and
// the field is empty.
func DateRangePicker(start, end *Signal[time.Time]) *DateRangePickerElement {
	return &DateRangePickerElement{start: start, end: end}
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (d *DateRangePickerElement) Tooltip(s string) *DateRangePickerElement {
	d.tooltip = s
	return d
}

// Small makes the field compact: 28 px high with smaller text, like
// Button.Small and TextInput.Small, for toolbars and dense forms.
func (d *DateRangePickerElement) Small() *DateRangePickerElement {
	d.ctlSize = sizeSmall
	return d
}

// Large makes the field larger: 40 px high with 16 px text, like
// Button.Large and TextInput.Large, for touch-friendly or prominent forms.
// Small and Large replace each other.
func (d *DateRangePickerElement) Large() *DateRangePickerElement {
	d.ctlSize = sizeLarge
	return d
}

// FocusHandle lets code focus the date range field (see NewFocusHandle). A Field that
// wraps the control gives it its own handle, which takes precedence.
func (d *DateRangePickerElement) FocusHandle(h *FocusHandle) *DateRangePickerElement {
	d.focusHandle = h
	return d
}

// Validate sets the validators of the range. A zero DateRange is no range, so Required
// rejects an empty field (see Field).
func (d *DateRangePickerElement) Validate(rules ...Validator[DateRange]) *DateRangePickerElement {
	d.rules = rules
	return d
}

// formCheck and formWire make a date range picker a formControl (see Field).
func (d *DateRangePickerElement) formCheck() error {
	return firstError(DateRange{Start: d.start.Get(), End: d.end.Get()}, d.rules)
}

func (d *DateRangePickerElement) formWire(w fieldWire) Element {
	c := *d
	c.wire = w
	return &c
}

func (*DateRangePickerElement) isElement()           {}
func (d *DateRangePickerElement) elementKey() string { return d.key }

// Key identifies the date range picker among its siblings (see DivElement.Key).
func (d *DateRangePickerElement) Key(k string) *DateRangePickerElement { d.key = k; return d }

// Label names the field for assistive technology, such as "Trip dates".
func (d *DateRangePickerElement) Label(name string) *DateRangePickerElement { d.name = name; return d }

// LabelledBy names the field by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (d *DateRangePickerElement) LabelledBy(keys ...string) *DateRangePickerElement {
	d.labelledBy = keys
	return d
}

// Placeholder is the text shown while the field is empty. The default is the hint of the locale's
// date format for both ends, such as "MM/DD/YYYY – MM/DD/YYYY" for en-US.
func (d *DateRangePickerElement) Placeholder(s string) *DateRangePickerElement {
	d.placeholder = s
	return d
}

// Disabled shows the field, with its range, and does not let it change.
func (d *DateRangePickerElement) Disabled(on bool) *DateRangePickerElement {
	d.disabled = on
	return d
}

// Min sets the earliest day that can be chosen. Its time of day is ignored; a zero time means
// there is no earliest day.
func (d *DateRangePickerElement) Min(day time.Time) *DateRangePickerElement {
	d.min = day
	return d
}

// Max sets the latest day that can be chosen. Its time of day is ignored; a zero time means there
// is no latest day.
func (d *DateRangePickerElement) Max(day time.Time) *DateRangePickerElement {
	d.max = day
	return d
}

// DisableDays rules out the days for which fn returns true, such as weekends. fn is called with
// each day at local midnight. A disabled day is drawn dimmed, is marked unavailable to assistive
// technology, and cannot be chosen as an end of the range by the calendar or by typing. A day
// between the ends may be disabled: the range is shaded across it.
func (d *DateRangePickerElement) DisableDays(fn func(day time.Time) bool) *DateRangePickerElement {
	d.disableDays = fn
	return d
}

// FirstDay sets the first day of the week in the calendar. The default is the first day of the week
// of the user's locale.
func (d *DateRangePickerElement) FirstDay(day time.Weekday) *DateRangePickerElement {
	d.firstDay, d.firstSet = day, true
	return d
}

// OnChange runs fn with the new start and end after the user chooses the end of a range, or types a
// range that can be chosen. It does not run when the range is set from outside.
func (d *DateRangePickerElement) OnChange(fn func(start, end time.Time)) *DateRangePickerElement {
	d.onChange = fn
	return d
}

// The sizing methods give the field a size. The field is dateRangePickerWidth wide at the Text size
// of 100 percent by default; Width(Full) fills its container. See Sizing in docs/styling.md.
func (d *DateRangePickerElement) Width(l Length) *DateRangePickerElement {
	d.size.setWidth(l)
	return d
}

// Height sets the height of the date range picker.
func (d *DateRangePickerElement) Height(l Length) *DateRangePickerElement {
	d.size.setHeight(l)
	return d
}

// MinWidth sets the smallest width of the date range picker.
func (d *DateRangePickerElement) MinWidth(l Length) *DateRangePickerElement {
	d.size.setMinWidth(l)
	return d
}

// MinHeight sets the smallest height of the date range picker.
func (d *DateRangePickerElement) MinHeight(l Length) *DateRangePickerElement {
	d.size.setMinHeight(l)
	return d
}

// MaxWidth sets the largest width of the date range picker.
func (d *DateRangePickerElement) MaxWidth(l Length) *DateRangePickerElement {
	d.size.setMaxWidth(l)
	return d
}

// MaxHeight sets the largest height of the date range picker.
func (d *DateRangePickerElement) MaxHeight(l Length) *DateRangePickerElement {
	d.size.setMaxHeight(l)
	return d
}

// Grow makes the date range picker take a share of the free space on its parent's main axis.
func (d *DateRangePickerElement) Grow(f float32) *DateRangePickerElement {
	d.size.setGrow(f)
	return d
}

// GridSpan sets how many columns and rows the date range picker covers in its parent's grid.
func (d *DateRangePickerElement) GridSpan(cols, rows int) *DateRangePickerElement {
	d.size.setSpan(cols, rows)
	return d
}

// Styles sets the look of the parts of the date range picker, for this instance only. Set fields
// win over the theme's DateRangePicker and the defaults; see DateRangePickerStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (d *DateRangePickerElement) Styles(st DateRangePickerStyle) *DateRangePickerElement {
	d.styles = d.styles.merge(st)
	return d
}

// dateRange is the value of a date range picker: its start and its end, each at local midnight, or
// zero. A zero start is no range.
type dateRange struct{ start, end time.Time }

// equal reports whether r and o are the same range.
func (r dateRange) equal(o dateRange) bool { return r.start.Equal(o.start) && r.end.Equal(o.end) }

// orderedSpan returns the first and the last day of the range from a to b, at local midnight. A zero
// a is no range, and a zero b is a start with no end.
func orderedSpan(a, b time.Time) (time.Time, time.Time) {
	if a.IsZero() {
		return time.Time{}, time.Time{}
	}
	if b.IsZero() {
		return dayOf(a), time.Time{}
	}
	lo, hi := dayOf(a), dayOf(b)
	if hi.Before(lo) {
		lo, hi = hi, lo
	}
	return lo, hi
}

// formatRange returns the text of r in l: the start and the end, joined by an en dash, or the start
// alone while the end is not chosen. A zero start is no range, and its text is empty.
func formatRange(r dateRange, l *dateLocale) string {
	if r.start.IsZero() {
		return ""
	}
	text := formatDate(r.start, l)
	if !r.end.IsZero() {
		text += " – " + formatDate(r.end, l)
	}
	return text
}

// splitRange returns the texts of the start and the end in text, which are joined by an en dash or a
// hyphen with a space on each side, and whether text has such a separator.
func splitRange(text string) (start, end string, ok bool) {
	for _, sep := range []string{" – ", " - "} {
		if a, b, found := strings.Cut(text, sep); found {
			return strings.TrimSpace(a), strings.TrimSpace(b), true
		}
	}
	return "", "", false
}

// rangeExample is the text of a sample range in l, which a message gives as an example of what the
// field takes.
func rangeExample(l *dateLocale) string {
	return dateExample(l) + " – " + formatDate(time.Date(2026, time.October, 20, 0, 0, 0, 0, time.Local), l)
}

// enabled reports whether day can be chosen as an end of the range: it is in the range of Min and
// Max, and DisableDays does not rule it out.
func (d *DateRangePickerElement) enabled(day time.Time, s *Strings) bool {
	return d.refusal(day, currentLocale(), s) == ""
}

// refusal returns the message that says why day cannot be chosen, or "" when it can.
func (d *DateRangePickerElement) refusal(day time.Time, loc *dateLocale, s *Strings) string {
	day = dayOf(day)
	switch {
	case !d.min.IsZero() && day.Before(dayOf(d.min)):
		return s.DateOnOrAfter(formatDate(d.min, loc))
	case !d.max.IsZero() && day.After(dayOf(d.max)):
		return s.DateOnOrBefore(formatDate(d.max, loc))
	case d.disableDays != nil && d.disableDays(day):
		return s.DateNotAllowed
	}
	return ""
}

func (d *DateRangePickerElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		rt := ctx.rt
		s := ctx.Strings()
		loc := currentLocale()
		scale := textScale(ctx)
		rs := resolveDateRangePicker(ctx, scale, d.disabled, d.styles)
		st := rs.dates()
		cur := dateRange{start: d.start.Get(), end: d.end.Get()}
		// The field keeps its own text while the user types. draft is that text, and seen is the
		// range it was last made from, so that a range set from outside replaces the text.
		draft := Local(ctx, "draft", formatRange(cur, loc))
		seen := Local(ctx, "seen", cur)
		invalid := Local(ctx, "invalid", false)
		msg := Local(ctx, "msg", "")
		open := Local(ctx, "open", false)
		// focus is the day the calendar's keys have reached; the calendar shows its month.
		focus := Local(ctx, "focus", time.Time{})
		// pending is the start, once it is chosen and the end is not; zero otherwise. hover is the
		// day under the pointer in the calendar, or zero.
		pending := Local(ctx, "pending", time.Time{})
		hover := Local(ctx, "hover", time.Time{})
		back := Local(ctx, "back", NewFocusHandle())
		ret := back.Peek()
		if d.focusHandle != nil {
			ret = d.focusHandle
		}
		w := d.wire
		if w.handle != nil {
			ret = w.handle
		}
		if !cur.equal(seen.Peek()) {
			draft.Set(formatRange(cur, loc))
			seen.Set(cur)
			invalid.Set(false)
		}

		// now is the range the signals hold at this moment.
		now := func() dateRange { return dateRange{start: d.start.Peek(), end: d.end.Peek()} }
		// set makes r the range, and says so to the field and to the app.
		set := func(r dateRange) {
			d.start.Set(r.start)
			d.end.Set(r.end)
			seen.Set(r)
			draft.Set(formatRange(r, loc))
			invalid.Set(false)
			pending.Set(time.Time{})
			if w.onEdit != nil {
				w.onEdit()
			}
			if d.onChange != nil {
				d.onChange(r.start, r.end)
			}
		}
		// commit applies the text as the range, and reports whether it did. An empty text is no
		// range, and the text of the range that is set is no change. Otherwise the text must be two
		// dates that can be chosen; if it is not, the message says why, the range is kept, and the
		// field is invalid.
		commit := func(text string) bool {
			text = strings.TrimSpace(text)
			if text == "" {
				set(dateRange{})
				return true
			}
			if text == formatRange(now(), loc) {
				return true
			}
			fail := func(why string) bool {
				msg.Set(why)
				invalid.Set(true)
				return false
			}
			first, last, ok := splitRange(text)
			if !ok {
				return fail(s.RangeExample(rangeExample(loc)))
			}
			a, okA := parseDate(first, loc, rt.now())
			b, okB := parseDate(last, loc, rt.now())
			if !okA || !okB {
				return fail(s.RangeExample(rangeExample(loc)))
			}
			if b.Before(a) {
				a, b = b, a
			}
			if why := d.refusal(a, loc, s); why != "" {
				return fail(why)
			}
			if why := d.refusal(b, loc, s); why != "" {
				return fail(why)
			}
			set(dateRange{start: a, end: b})
			return true
		}
		// submit is the field's Enter: a valid text is applied and the form's Enter runs.
		// A text that is not a range stays in the field, with its message.
		enter := w.onEnter
		w.onEnter = nil
		submit := func(text string) {
			if commit(text) && enter != nil {
				enter()
			}
		}
		// blur applies a text that changed and is valid, and otherwise shows the range again (as a
		// date picker does), so that a form never sees a text the field does not hold.
		blur := func() {
			shown := formatRange(now(), loc)
			if draft.Peek() == shown {
				return
			}
			if !commit(draft.Peek()) {
				draft.Set(shown)
				invalid.Set(false)
			}
		}

		// shut closes the calendar, and drops a start that was chosen without its end. Focus
		// returns to the field when restore is set.
		shut := func(restore bool) {
			if !open.Peek() {
				return
			}
			open.Set(false)
			pending.Set(time.Time{})
			hover.Set(time.Time{})
			rt.menuClosed(restore)
		}
		// openCal opens the calendar on the start of the range, or on today when there is none.
		openCal := func() {
			if d.disabled || open.Peek() {
				return
			}
			rt.menuOpened()
			rt.menuReturn = ret
			start := dayOf(rt.now())
			if s := d.start.Peek(); !s.IsZero() {
				start = dayOf(s)
			}
			focus.Set(start)
			open.Set(true)
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
		// pick chooses day. The first day chosen is the start, and the calendar stays open; the
		// second is the end, and it closes the calendar. A day that cannot be chosen is ignored.
		pick := func(day time.Time) {
			if !d.enabled(day, s) {
				return
			}
			p := pending.Peek()
			if p.IsZero() {
				pending.Set(day)
				Announce(s.RangeStartChosen, access.Polite)
				return
			}
			a, b := p, day
			if b.Before(a) {
				a, b = b, a
			}
			set(dateRange{start: a, end: b})
			shut(true)
		}
		// keys are the calendar's keys, which it has while it is open. Its focus is the day the
		// keys have reached, and the screen reader reads that day.
		keys := func(e *KeyEvent) {
			first := loc.firstDay
			if d.firstSet {
				first = d.firstDay
			}
			calendarKeys(e, focus.Peek(), first, moveTo, pick, shut)
		}
		// hovered is the day the pointer is over in the calendar, or zero when it leaves.
		hovered := func(day time.Time) { hover.Set(day) }

		// The field is its own frame and the calendar, as a combo box is. Its accessibility role is
		// a combo box that is expanded while the calendar is open.
		root := Div().Column()
		if d.size.set&hasWidth == 0 {
			root.style.Width = Px(dateRangePickerWidth * scale)
		}
		if d.key != "" {
			root.Key(d.key)
		}
		a := root.access()
		root.Role(access.RoleComboBox).AccessExpanded(open.Get())
		name, labelledBy := d.name, d.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		if name != "" {
			root.Label(name)
		}
		if len(labelledBy) > 0 {
			root.LabelledBy(labelledBy...)
		}
		errKey := "daterange-error"
		if d.key != "" {
			errKey = d.key + "/error"
		}
		isInvalid := invalid.Get()
		shown := ""
		if isInvalid {
			shown = msg.Get()
		}
		if desc := describe(w.description, shown); desc != "" {
			root.Description(desc)
		}
		a.value = draft.Get()
		a.setExpanded = func(o bool) {
			if o {
				openCal()
			} else {
				shut(true)
			}
		}
		a.invalid, a.required = isInvalid || w.invalid, w.required

		// The days shaded are the range that is set, or, while the end is being chosen, the range
		// from the start to the day under the pointer or the keys, when that day can be chosen.
		from, to := rangeShade(cur.start, cur.end, pending.Get(), hover.Get(), focus.Get(),
			func(day time.Time) bool { return d.enabled(day, s) })

		fst := resolveField(ctx, fieldLayout{width: Full, disabled: d.disabled, size: d.ctlSize}, st.Field)
		if d.disabled {
			a.disabled = true
			root.Cursor(CursorNotAllowed)
			frame := Div().asPart("DateRangePicker.Frame").Style(fst.Frame).Style(fst.FrameDisabled).
				Child(Div().asPart("DateRangePicker.Text").Style(fst.Text).Grow(1).Child(Text(draft.Get()).NoWrap()))
			root.Child(frame)
			return d.size.apply(root)
		}

		// The calendar button is the trailing element of the frame. It is not a Tab stop: the keys
		// Alt+Down and F4 open the calendar from the field.
		button := Div().asPart("DateRangePicker.CalendarButton").Style(st.CalendarButton).
			Role(access.RoleButton).Label(s.ChooseDates).Cursor(CursorPointer).
			OnClick(openCal).Child(Icon(icons.Calendar).Size(16 * scale))

		var describedBy []string
		if isInvalid {
			describedBy = []string{errKey}
		}
		placeholder := d.placeholder
		if placeholder == "" {
			hint := loc.datePlaceholder()
			placeholder = hint + " – " + hint
		}
		fo := fieldOptions{tooltip: d.tooltip, value: draft, placeholder: placeholder, name: name, labelledBy: labelledBy,
			describedBy: describedBy, widget: "DateRangePicker", handle: ret, wire: w,
			onSubmit: submit, onChange: func(string) { invalid.Set(false) }, onBlur: blur, trailing: button}
		fo.wire.invalid = isInvalid || w.invalid
		fo.size.setWidth(Full)
		root.Child(fo.expand(ctx, fst, false, 0))
		// The message is always in the tree, so that the field keeps its place among its siblings,
		// and it is shown while the text is not a range that can be chosen.
		root.Child(Div().asPart("DateRangePicker.Message").Style(st.Message).Display(isInvalid).
			Child(Icon(icons.Alert).Size(14).Color(messageIconColor(t, st.Message))).
			Child(Text(shown).Key(errKey)))

		// The field's Escape, Alt+Down and F4, and Tab, are the root's, as a combo box has them.
		root.OnKeyDown(func(e *KeyEvent) {
			switch {
			case e.Key == input.KeyTab && open.Peek():
				shut(true)
			case (e.Key == input.KeyDown && e.Mods == input.ModAlt || e.Key == input.KeyF4 && e.Mods == 0) && !e.Repeat:
				openCal()
				e.StopPropagation()
			}
		})

		if open.Get() {
			enabled := func(day time.Time) bool { return d.enabled(day, s) }
			root.Child(calendarPanel(calendarView{st: st.calendar(rs.DayInRange), widget: "DateRangePicker", loc: loc,
				scale: scale, from: from, to: to, focus: focus.Get(), ring: true, today: dayOf(rt.now()),
				name: name, fallback: s.DateRangeField, move: moveTo, pick: pick, keys: keys, shut: shut,
				enabled: enabled, hover: hovered, firstSet: d.firstSet, firstDay: d.firstDay,
				motion: ctx.Theme().Motion}, s))
		}
		return d.size.apply(root)
	})
}
