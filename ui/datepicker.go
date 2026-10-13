package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// datePickerWidth is the default width of a date picker at the Text size of 100 percent,
// in logical pixels. It scales with the Text size.
const datePickerWidth = 180

// dayCellSize is the width and height of a day of the calendar at the Text size of 100
// percent, in logical pixels.
const dayCellSize = 32

// DatePickerStyle is the look of each part of a date picker. A part's style merges over
// the built-in defaults, then over the theme's DatePicker, then over the Styles of the
// instance; see docs/styling.md.
//
// Field is the text frame, a FieldStyle as TextInput's is, with its states. CalendarButton
// is the button at the end of the field that opens the calendar, and Message the row that
// says why the text is not a date that can be chosen (as NumberInput's message). The
// calendar is the popup: Popup is its panel, Header the row with the month and the year,
// NavButton each of its navigation buttons (previous and next month, and year), and
// WeekdayLabel the name of each day of the week. Day is each day; DayHover a day the
// pointer is over; DaySelected the day in the field; DayToday today; DayDisabled a day that
// cannot be chosen (see Min, Max and DisableDays); DayOutsideMonth a day of the previous or
// the next month that fills the grid; and DayFocusVisible the day the keys have reached.
// Under high contrast the selected day is drawn in the system Highlight colors, and today
// and the focused day by their borders, so that no state is shown by color alone.
type DatePickerStyle struct {
	Field           FieldStyle
	CalendarButton  Style
	Message         Style
	Popup           Style
	Header          Style
	NavButton       Style
	WeekdayLabel    Style
	Day             Style
	DayHover        Style
	DaySelected     Style
	DayToday        Style
	DayDisabled     Style
	DayOutsideMonth Style
	DayFocusVisible Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s DatePickerStyle) merge(o DatePickerStyle) DatePickerStyle {
	return DatePickerStyle{
		Field:           s.Field.merge(o.Field),
		CalendarButton:  s.CalendarButton.Merge(o.CalendarButton),
		Message:         s.Message.Merge(o.Message),
		Popup:           s.Popup.Merge(o.Popup),
		Header:          s.Header.Merge(o.Header),
		NavButton:       s.NavButton.Merge(o.NavButton),
		WeekdayLabel:    s.WeekdayLabel.Merge(o.WeekdayLabel),
		Day:             s.Day.Merge(o.Day),
		DayHover:        s.DayHover.Merge(o.DayHover),
		DaySelected:     s.DaySelected.Merge(o.DaySelected),
		DayToday:        s.DayToday.Merge(o.DayToday),
		DayDisabled:     s.DayDisabled.Merge(o.DayDisabled),
		DayOutsideMonth: s.DayOutsideMonth.Merge(o.DayOutsideMonth),
		DayFocusVisible: s.DayFocusVisible.Merge(o.DayFocusVisible),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s DatePickerStyle) plain() DatePickerStyle {
	return DatePickerStyle{
		Field:           s.Field.plain(),
		CalendarButton:  plainStyle(s.CalendarButton),
		Message:         plainStyle(s.Message),
		Popup:           plainStyle(s.Popup),
		Header:          plainStyle(s.Header),
		NavButton:       plainStyle(s.NavButton),
		WeekdayLabel:    plainStyle(s.WeekdayLabel),
		Day:             plainStyle(s.Day),
		DayHover:        plainStyle(s.DayHover),
		DaySelected:     plainStyle(s.DaySelected),
		DayToday:        plainStyle(s.DayToday),
		DayDisabled:     plainStyle(s.DayDisabled),
		DayOutsideMonth: plainStyle(s.DayOutsideMonth),
		DayFocusVisible: plainStyle(s.DayFocusVisible),
	}
}

// datePickerDefaults returns the built-in look of a date picker at the Text size scale,
// from the colors of t. The text frame's defaults come from resolveField.
func datePickerDefaults(t *Theme, scale float32, disabled bool) DatePickerStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	cell := dayCellSize * scale
	d := DatePickerStyle{
		CalendarButton: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(24 * scale),
			Height: Px(24 * scale), NoShrink: true, Radius: 4, TextColor: t.TextMuted, Transition: 110 * time.Millisecond},
		Message: messageStyle(t),
		Popup: Style{Direction: Column, Gap: 4 * scale, Padding: All(8 * scale), Background: t.Surface,
			BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}},
		Header: Style{Direction: Row, Align: Center, Justify: Center, Grow: 1, Height: Px(28 * scale),
			FontSize: 14, FontWeight: FontSemibold, TextColor: t.Text},
		NavButton: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(28 * scale),
			Height: Px(28 * scale), NoShrink: true, Radius: 4, TextColor: t.TextMuted},
		WeekdayLabel: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(cell), Height: Px(24 * scale),
			NoShrink: true, FontSize: 12, TextColor: t.TextMuted},
		Day: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(cell), Height: Px(cell),
			NoShrink: true, Radius: 4, FontSize: 13, TextColor: t.Text, Transition: 110 * time.Millisecond},
		DayHover:        Style{Background: t.SurfaceHover},
		DaySelected:     Style{Background: t.Accent, TextColor: t.AccentText},
		DayFocusVisible: Style{Shadows: []Shadow{ring}},
	}
	if t.HighContrast {
		d.DaySelected = Style{Background: t.Accent, TextColor: t.AccentText}
		d.DayToday = Style{BorderWidth: 1, BorderColor: t.Text}
		d.DayDisabled = Style{TextColor: t.Disabled}
		// The focus is an inner outline, as the focus of a number input's buttons is.
		d.DayFocusVisible = Style{BorderWidth: 2, BorderColor: t.FocusRing}
	} else {
		d.DayToday = Style{BorderWidth: 1, BorderColor: t.Accent}
		d.DayDisabled = Style{Opacity: 0.45}
		d.DayOutsideMonth = Style{TextColor: t.TextSubtle}
	}
	if disabled {
		d.CalendarButton.TextColor = t.Disabled
	}
	return d
}

// resolveDatePicker returns the look of a date picker: the defaults, merged with the
// theme's DatePicker and then the instance's Styles. The text frame is kept as the
// overrides give it; the element resolves it over the theme's Field (see resolveField).
// Under high contrast the overrides are plain (see plainStyle).
func resolveDatePicker(t *Theme, scale float32, disabled bool, inst DatePickerStyle) DatePickerStyle {
	st := datePickerDefaults(t, scale, disabled)
	for _, o := range []DatePickerStyle{t.DatePicker, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// DatePickerElement is a text field for a day, with a calendar that opens below it. The
// text is the day in the short date format of the user's locale (see the README, Date and
// time). The value is a time.Time: its day is the chosen day and its time of day is kept,
// so one signal can hold a date and a time (see TimePicker); a zero time is no date.
//
// The keys of the calendar follow the WAI-ARIA date picker dialog pattern (see the README,
// Date and time). Alt+Down and F4 open it from the field, with focus on the chosen day or
// today; Enter or Space chooses the day and closes it, and Escape or Tab closes it without a
// change. A day that is not between Min and Max, or that DisableDays rules out, cannot be
// chosen: it is disabled, and typed text that names it is refused with a message.
type DatePickerElement struct {
	value       *Signal[time.Time]
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
	styles      DatePickerStyle
	onChange    func(time.Time)
	rules       []Validator[time.Time] // the validators of the value (see Validate)
	wire        fieldWire              // what a Field gives the field (see Field)
	focusHandle *FocusHandle           // see FocusHandle
	ctlSize     controlSize            // see Small and Large
	tooltip     string                 // see Tooltip
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (d *DatePickerElement) Tooltip(s string) *DatePickerElement {
	d.tooltip = s
	return d
}

// Small makes the field compact: 28 px high with smaller text, like
// Button.Small and TextInput.Small, for toolbars and dense forms.
func (d *DatePickerElement) Small() *DatePickerElement {
	d.ctlSize = sizeSmall
	return d
}

// Large makes the field larger: 40 px high with 16 px text, like
// Button.Large and TextInput.Large, for touch-friendly or prominent forms.
// Small and Large replace each other.
func (d *DatePickerElement) Large() *DatePickerElement {
	d.ctlSize = sizeLarge
	return d
}

// FocusHandle lets code focus the date field (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (d *DatePickerElement) FocusHandle(h *FocusHandle) *DatePickerElement {
	d.focusHandle = h
	return d
}

// DatePicker creates a date picker that reads and writes value. The default width is 180 px
// at the Text size of 100 percent. A zero time is no date, and the field is empty.
func DatePicker(value *Signal[time.Time]) *DatePickerElement {
	return &DatePickerElement{value: value}
}

// Validate sets the validators of the value. A zero time is no date, so Required rejects an
// empty field (see Field).
func (d *DatePickerElement) Validate(rules ...Validator[time.Time]) *DatePickerElement {
	d.rules = rules
	return d
}

// formCheck and formWire make a date picker a formControl (see Field).
func (d *DatePickerElement) formCheck() error { return firstError(d.value.Get(), d.rules) }

func (d *DatePickerElement) formWire(w fieldWire) Element {
	c := *d
	c.wire = w
	return &c
}

func (*DatePickerElement) isElement()           {}
func (d *DatePickerElement) elementKey() string { return d.key }

func (d *DatePickerElement) Key(k string) *DatePickerElement { d.key = k; return d }

// Label names the field for assistive technology, such as "Due date".
func (d *DatePickerElement) Label(name string) *DatePickerElement { d.name = name; return d }

// LabelledBy names the field by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (d *DatePickerElement) LabelledBy(keys ...string) *DatePickerElement {
	d.labelledBy = keys
	return d
}

// Placeholder is the text shown while the field is empty. The default is the hint of the
// locale's date format, such as "MM/DD/YYYY" for en-US (see datePlaceholder).
func (d *DatePickerElement) Placeholder(s string) *DatePickerElement { d.placeholder = s; return d }

// Disabled shows the field, with its date, and does not let it change.
func (d *DatePickerElement) Disabled(on bool) *DatePickerElement { d.disabled = on; return d }

// Min sets the earliest day that can be chosen. Its time of day is ignored; a zero time
// means there is no earliest day.
func (d *DatePickerElement) Min(day time.Time) *DatePickerElement { d.min = day; return d }

// Max sets the latest day that can be chosen. Its time of day is ignored; a zero time means
// there is no latest day.
func (d *DatePickerElement) Max(day time.Time) *DatePickerElement { d.max = day; return d }

// DisableDays rules out the days for which fn returns true, such as weekends. fn is called
// with each day at local midnight. A disabled day is drawn dimmed, is marked unavailable to
// assistive technology, and cannot be chosen by the calendar or by typing.
func (d *DatePickerElement) DisableDays(fn func(day time.Time) bool) *DatePickerElement {
	d.disableDays = fn
	return d
}

// FirstDay sets the first day of the week in the calendar. The default is the first day of
// the week of the user's locale.
func (d *DatePickerElement) FirstDay(day time.Weekday) *DatePickerElement {
	d.firstDay, d.firstSet = day, true
	return d
}

// OnChange runs fn with the new value after the user chooses a day, or types a date that
// can be chosen. It does not run when the value is set from outside.
func (d *DatePickerElement) OnChange(fn func(time.Time)) *DatePickerElement {
	d.onChange = fn
	return d
}

// The sizing methods give the field a size. The field is datePickerWidth wide at the Text
// size of 100 percent by default; Width(Full) fills its container. See Sizing in
// docs/styling.md.
func (d *DatePickerElement) Width(l Length) *DatePickerElement    { d.size.setWidth(l); return d }
func (d *DatePickerElement) Height(l Length) *DatePickerElement   { d.size.setHeight(l); return d }
func (d *DatePickerElement) MinWidth(l Length) *DatePickerElement { d.size.setMinWidth(l); return d }
func (d *DatePickerElement) MinHeight(l Length) *DatePickerElement {
	d.size.setMinHeight(l)
	return d
}
func (d *DatePickerElement) MaxWidth(l Length) *DatePickerElement { d.size.setMaxWidth(l); return d }
func (d *DatePickerElement) MaxHeight(l Length) *DatePickerElement {
	d.size.setMaxHeight(l)
	return d
}
func (d *DatePickerElement) Grow(f float32) *DatePickerElement { d.size.setGrow(f); return d }

// Styles sets the look of the parts of the date picker, for this instance only. Set fields
// win over the theme's DatePicker and the defaults; see DatePickerStyle and docs/styling.md.
// Calling Styles again merges with the earlier values.
func (d *DatePickerElement) Styles(st DatePickerStyle) *DatePickerElement {
	d.styles = d.styles.merge(st)
	return d
}

// enabled reports whether day can be chosen: it is in the range of Min and Max, and
// DisableDays does not rule it out.
func (d *DatePickerElement) enabled(day time.Time) bool {
	return d.refusal(day, currentLocale()) == ""
}

// refusal returns the message that says why day cannot be chosen, or "" when it can.
func (d *DatePickerElement) refusal(day time.Time, loc *dateLocale) string {
	day = dayOf(day)
	switch {
	case !d.min.IsZero() && day.Before(dayOf(d.min)):
		return "Enter a date on or after " + formatDate(d.min, loc) + "."
	case !d.max.IsZero() && day.After(dayOf(d.max)):
		return "Enter a date on or before " + formatDate(d.max, loc) + "."
	case d.disableDays != nil && d.disableDays(day):
		return "That date cannot be chosen."
	}
	return ""
}

func (d *DatePickerElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		rt := ctx.rt
		loc := currentLocale()
		scale := textScale(ctx)
		st := resolveDatePicker(t, scale, d.disabled, d.styles)
		v := d.value.Get()
		// The field keeps its own text while the user types. draft is that text, and seen is
		// the value it was last made from, so that a value set from outside replaces the text.
		draft := Local(ctx, "draft", formatDate(v, loc))
		seen := Local(ctx, "seen", v)
		invalid := Local(ctx, "invalid", false)
		msg := Local(ctx, "msg", "")
		open := Local(ctx, "open", false)
		// focus is the day the calendar's keys have reached; the calendar shows its month.
		focus := Local(ctx, "focus", time.Time{})
		back := Local(ctx, "back", NewFocusHandle())
		ret := back.Peek()
		if d.focusHandle != nil {
			ret = d.focusHandle
		}
		w := d.wire
		if w.handle != nil {
			ret = w.handle
		}
		if !v.Equal(seen.Peek()) {
			draft.Set(formatDate(v, loc))
			seen.Set(v)
			invalid.Set(false)
		}

		// set makes nv the value, and says so to the field and to the app.
		set := func(nv time.Time) {
			d.value.Set(nv)
			seen.Set(nv)
			draft.Set(formatDate(nv, loc))
			invalid.Set(false)
			if w.onEdit != nil {
				w.onEdit()
			}
			if d.onChange != nil {
				d.onChange(nv)
			}
		}
		// commit applies the text as the value, and reports whether it did. An empty text is
		// no date. Otherwise the text must be a date that can be chosen; if it is not, the
		// message says why, the value is kept, and the field is invalid.
		commit := func(text string) bool {
			text = strings.TrimSpace(text)
			if text == "" {
				set(time.Time{})
				return true
			}
			day, ok := parseDate(text, loc, rt.now())
			if !ok {
				msg.Set("Enter a date such as " + dateExample(loc) + ".")
				invalid.Set(true)
				return false
			}
			if why := d.refusal(day, loc); why != "" {
				msg.Set(why)
				invalid.Set(true)
				return false
			}
			set(withClock(day, clockOf(d.value.Peek())))
			return true
		}
		// submit is the field's Enter: a valid text is applied and the form's Enter runs.
		// A text that is not a date stays in the field, with its message.
		enter := w.onEnter
		w.onEnter = nil
		submit := func(text string) {
			if commit(text) && enter != nil {
				enter()
			}
		}
		// blur applies a text that changed and is valid, and otherwise shows the value again
		// (as a number input does), so that a form never sees a text it does not hold.
		blur := func() {
			cur := formatDate(d.value.Peek(), loc)
			if draft.Peek() == cur {
				return
			}
			if !commit(draft.Peek()) {
				draft.Set(cur)
				invalid.Set(false)
			}
		}

		// shut closes the calendar. Focus returns to the field when restore is set.
		shut := func(restore bool) {
			if !open.Peek() {
				return
			}
			open.Set(false)
			rt.menuClosed(restore)
		}
		// openCal opens the calendar on the chosen day, or on today when there is none.
		openCal := func() {
			if d.disabled || open.Peek() {
				return
			}
			rt.menuOpened()
			rt.menuReturn = ret
			start := dayOf(rt.now())
			if cur := d.value.Peek(); !cur.IsZero() {
				start = dayOf(cur)
			}
			focus.Set(start)
			open.Set(true)
		}
		// moveTo moves the keys' day to day, and announces the month when it changes.
		moveTo := func(day time.Time) {
			old := focus.Peek()
			focus.Set(day)
			if old.Year() != day.Year() || old.Month() != day.Month() {
				Announce(loc.monthYear(day), access.Polite)
			}
		}
		// pick chooses day and closes the calendar. A day that cannot be chosen is ignored.
		pick := func(day time.Time) {
			if !d.enabled(day) {
				return
			}
			set(withClock(day, clockOf(d.value.Peek())))
			shut(true)
		}
		// keys are the calendar's keys, which it has while it is open. Its focus is the day
		// the keys have reached, and the screen reader reads that day.
		keys := func(e *KeyEvent) {
			if e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
				return
			}
			f := focus.Peek()
			shift := e.Mods&input.ModShift != 0
			first := loc.firstDay
			if d.firstSet {
				first = d.firstDay
			}
			switch e.Key {
			case input.KeyLeft:
				moveTo(dayOf(f).AddDate(0, 0, -1))
			case input.KeyRight:
				moveTo(dayOf(f).AddDate(0, 0, 1))
			case input.KeyUp:
				moveTo(dayOf(f).AddDate(0, 0, -7))
			case input.KeyDown:
				moveTo(dayOf(f).AddDate(0, 0, 7))
			case input.KeyHome:
				moveTo(weekStart(f, first))
			case input.KeyEnd:
				moveTo(weekStart(f, first).AddDate(0, 0, 6))
			case input.KeyPageUp:
				moveTo(addMonths(f, -monthsStep(shift)))
			case input.KeyPageDown:
				moveTo(addMonths(f, monthsStep(shift)))
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

		// The field is its own frame and the calendar, as a combo box is. Its accessibility
		// role is a combo box that is expanded while the calendar is open.
		root := Div().Column()
		if d.size.set&hasWidth == 0 {
			root.style.Width = Px(datePickerWidth * scale)
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
		errKey := "datepicker-error"
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

		fst := resolveField(t, fieldLayout{width: Full, disabled: d.disabled, size: d.ctlSize}, st.Field)
		if d.disabled {
			a.disabled = true
			root.Cursor(CursorNotAllowed)
			frame := Div().asPart("DatePicker.Frame").Style(fst.Frame).Style(fst.FrameDisabled).
				Child(Div().asPart("DatePicker.Text").Style(fst.Text).Grow(1).Child(Text(draft.Get()).NoWrap()))
			root.Child(frame)
			return d.size.apply(root)
		}

		// The calendar button is the trailing element of the frame. It is not a Tab stop: the
		// keys Alt+Down and F4 open the calendar from the field.
		button := Div().asPart("DatePicker.CalendarButton").Style(st.CalendarButton).
			Role(access.RoleButton).Label("Choose date").Cursor(CursorPointer).
			OnClick(openCal).Child(Icon(icons.Calendar).Size(16 * scale))

		var describedBy []string
		if isInvalid {
			describedBy = []string{errKey}
		}
		placeholder := d.placeholder
		if placeholder == "" {
			placeholder = loc.datePlaceholder()
		}
		fo := fieldOptions{tooltip: d.tooltip, value: draft, placeholder: placeholder, name: name, labelledBy: labelledBy,
			describedBy: describedBy, widget: "DatePicker", handle: ret, wire: w,
			onSubmit: submit, onChange: func(string) { invalid.Set(false) }, onBlur: blur, trailing: button}
		fo.wire.invalid = isInvalid || w.invalid
		fo.size.setWidth(Full)
		root.Child(fo.expand(ctx, fst, false, 0))
		// The message is always in the tree, so that the field keeps its place among its
		// siblings, and it is shown while the text is not a date that can be chosen.
		root.Child(Div().asPart("DatePicker.Message").Style(st.Message).Display(isInvalid).
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
			root.Child(d.calendar(calendarView{st: st, loc: loc, scale: scale, value: v, focus: focus.Get(),
				today: dayOf(rt.now()), name: name, move: moveTo, pick: pick, keys: keys,
				shut: shut, enabled: d.enabled, firstSet: d.firstSet, firstDay: d.firstDay}))
		}
		return d.size.apply(root)
	})
}

// monthsStep is the number of months Page Up and Page Down move by: a month, or a year
// with Shift.
func monthsStep(shift bool) int {
	if shift {
		return 12
	}
	return 1
}

// calendarView is what the calendar of a date picker shows, and the functions it calls.
type calendarView struct {
	st       DatePickerStyle
	loc      *dateLocale
	scale    float32
	value    time.Time // the chosen day, or zero
	focus    time.Time // the day the keys have reached; its month is shown
	today    time.Time
	firstSet bool // FirstDay was called, and firstDay is the week's first day
	firstDay time.Weekday
	name     string
	move     func(day time.Time)
	pick     func(day time.Time)
	keys     func(e *KeyEvent)
	shut     func(restore bool)
	enabled  func(day time.Time) bool
}

// calendar returns the open calendar: an overlay below the field (above it when there is no
// room). It is a calendar for assistive technology, a grid of six week rows, each with seven
// days, under a header of the weekday names (see the README, Date and time). It takes focus
// when it opens, and the day the keys have reached is its highlighted cell, which the screen
// reader reads. The navigation buttons are for a pointer; the keys are PageUp and PageDown
// for a month, and Shift with them for a year.
func (d *DatePickerElement) calendar(c calendarView) Element {
	first := c.loc.firstDay
	if c.firstSet {
		first = c.firstDay
	}
	// The calendar is named by its field, as a combo box's list is. A name must not carry the
	// control type, so the word "calendar" is not in it (Axe's rule for names).
	name := c.name
	if name == "" {
		name = "Date"
	}
	st := c.st
	panel := Div().asPart("DatePicker.Popup").Key("jamb.calendar").Style(st.Popup).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: 140 * time.Millisecond}).
		Exit(Fade().Grow(0.98), Animation{Duration: 90 * time.Millisecond}).
		Overlay(BelowStart).OverlayGap(4).
		Role(access.RoleCalendar).SelectionContainer().Label(name).
		AutoFocus().TabStop(false).OnKeyDown(c.keys).
		OnClickOutside(func() { c.shut(false) }).
		OnClick(func() {}) // a press on the calendar's padding must not reach the field
	pa := panel.access()
	pa.gridRows, pa.gridCols = 6, 7
	focus := c.focus
	nav := func(label string, glyph *icons.Icon, text string, step time.Time) *DivElement {
		b := Div().asPart("DatePicker.NavButton").Style(st.NavButton).Role(access.RoleButton).Label(label).
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
		Child(Div().asPart("DatePicker.Header").Style(st.Header).Child(Text(c.loc.monthYear(focus)).NoWrap())).
		Child(nav("Next month", icons.ChevronRight, "", addMonths(focus, 1))).
		Child(nav("Next year", nil, "»", addMonths(focus, 12)))
	panel.Child(header)

	// The weekday names are header items of the grid, which the grid is given by its
	// children, so they are not wrapped in a named element.
	names := Div().Row().Gap(2 * c.scale)
	for i := range 7 {
		wd := time.Weekday((int(first) + i) % 7)
		names.Child(Div().asPart("DatePicker.WeekdayLabel").Style(st.WeekdayLabel).
			Role(access.RoleHeaderItem).Label(c.loc.weekdayLong(wd)).
			Child(Text(c.loc.weekdayShort(wd)).NoWrap()))
	}
	panel.Child(names)

	grid := monthGrid(focus, first)
	for r := range 6 {
		week := Div().Row().Gap(2 * c.scale).Label(c.loc.longDate(grid[r][0]))
		week.listItem(r+1, 6)
		for col := range 7 {
			day := grid[r][col]
			week.Child(d.dayCell(c, day, r+1, col+1))
		}
		panel.Child(week)
	}
	return panel
}

// dayCell returns the cell of one day of the calendar, at the grid position row and column
// (both 1-based). It is a data item of the grid, named by the long date of the day, and it
// is selected when the day is the chosen one. A day that cannot be chosen is disabled.
func (d *DatePickerElement) dayCell(c calendarView, day time.Time, row, col int) *DivElement {
	st := c.st
	on := !c.value.IsZero() && sameDay(day, c.value)
	ok := c.enabled(day)
	cell := Div().asPart("DatePicker.Day").Style(st.Day).Role(access.RoleDataItem).
		Label(c.loc.longDate(day)).Child(Text(fmt.Sprintf("%d", day.Day())).NoWrap())
	a := cell.access()
	a.gridRow, a.gridCol = row, col
	cell.AccessSelected(on)
	if day.Month() != c.focus.Month() || day.Year() != c.focus.Year() {
		cell.Style(st.DayOutsideMonth)
	}
	if sameDay(day, c.today) {
		cell.Style(st.DayToday)
		cell.Description("Today")
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
	if sameDay(day, c.focus) {
		a.highlighted = true
		cell.Style(st.DayFocusVisible)
	}
	return cell
}
