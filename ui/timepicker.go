package ui

import (
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// timePickerWidth is the default width of a time picker at the Text size of 100 percent,
// in logical pixels. It scales with the Text size.
const timePickerWidth = 140

// timePickerMaxVisible is how many times the open list shows at once.
const timePickerMaxVisible = 8

// timePickerRowHeight is the height of a row of the open list at the Text size of 100
// percent, in logical pixels.
const timePickerRowHeight = 28

// timePickerDefaultStep is the minutes between the times of the list, unless MinuteStep
// sets others.
const timePickerDefaultStep = 15

// TimePickerStyle is the look of each part of a time picker. A part's style merges over
// the built-in defaults, then over the theme's TimePicker, then over the Styles of the
// instance; see docs/styling.md.
//
// Field is the text frame, a FieldStyle as TextInput's is, with its states. Button is the
// button at the end of the field that opens the list of times, and Message the row that
// says why the text is not a time that can be chosen (as NumberInput's message). Popup is
// the open list, Option each time in it, OptionHighlighted the time the keys have reached,
// and OptionSelected the time that the field holds. Under high contrast the highlighted
// time is drawn in the system Highlight colors, and the selected one has a check mark, so
// that no state is shown by color alone.
type TimePickerStyle struct {
	Field             FieldStyle
	Button            Style
	Message           Style
	Popup             Style
	Option            Style
	OptionHighlighted Style
	OptionSelected    Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TimePickerStyle) merge(o TimePickerStyle) TimePickerStyle {
	return TimePickerStyle{
		Field:             s.Field.merge(o.Field),
		Button:            s.Button.Merge(o.Button),
		Message:           s.Message.Merge(o.Message),
		Popup:             s.Popup.Merge(o.Popup),
		Option:            s.Option.Merge(o.Option),
		OptionHighlighted: s.OptionHighlighted.Merge(o.OptionHighlighted),
		OptionSelected:    s.OptionSelected.Merge(o.OptionSelected),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s TimePickerStyle) plain() TimePickerStyle {
	return TimePickerStyle{
		Field:             s.Field.plain(),
		Button:            plainStyle(s.Button),
		Message:           plainStyle(s.Message),
		Popup:             plainStyle(s.Popup),
		Option:            plainStyle(s.Option),
		OptionHighlighted: plainStyle(s.OptionHighlighted),
		OptionSelected:    plainStyle(s.OptionSelected),
	}
}

// timePickerDefaults returns the built-in look of a time picker at the Text size scale,
// from the colors of t.
func timePickerDefaults(t *Theme, scale float32) TimePickerStyle {
	d := TimePickerStyle{
		Button: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(24 * scale), Height: Px(24 * scale),
			NoShrink: true, Radius: 4, TextColor: t.TextMuted},
		Message: messageStyle(t),
		Popup: Style{Direction: Column, Padding: All(4), Background: t.Surface, BorderWidth: 1,
			BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}},
		Option: Style{Direction: Row, Align: Center, Gap: 8, MinHeight: Px(28 * scale), Padding: XY(8, 0),
			Radius: 4, NoShrink: true, FontSize: 13, TextColor: t.Text},
		OptionSelected: Style{TextColor: t.Accent},
	}
	if t.HighContrast {
		d.OptionHighlighted = Style{Background: t.Accent, TextColor: t.AccentText}
		d.OptionSelected = Style{}
	} else {
		d.OptionHighlighted = Style{Background: t.AccentSoft}
	}
	return d
}

// resolveTimePicker returns the look of a time picker: the defaults, merged with the
// theme's TimePicker and then the instance's Styles. Under high contrast the overrides are
// plain (see plainStyle).
func resolveTimePicker(t *Theme, scale float32, inst TimePickerStyle) TimePickerStyle {
	st := timePickerDefaults(t, scale)
	for _, o := range []TimePickerStyle{t.TimePicker, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// timeList is the state of the open list that the render keeps: the row the keys have
// reached last is scrolled into view once, as in a combo box.
type timeList struct {
	reveal int // the row plus one, zero for none; cleared when read
}

// TimePickerElement is a text field for a time of day, with a list of the times at a fixed
// step below it. The text is the time in the clock format of the user's locale (see the
// README, Date and time). The value is a time.Time: its clock is the time and its date is
// kept, so one signal can hold a date and a time (see DatePicker); a zero time is no time.
//
// Up and Down step the time by the step while the list is closed, and move the highlight
// through the list while it is open. Alt+Down and F4 open the list, Enter chooses the
// highlighted time, and Escape or Tab closes it. Focus stays in the text, as it does in a
// combo box. A time that is not between Min and Max cannot be chosen.
type TimePickerElement struct {
	value       *Signal[time.Time]
	step        int // the minutes between the times of the list
	min, max    time.Time
	placeholder string
	name        string // the accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	size        sizing
	styles      TimePickerStyle
	onChange    func(time.Time)
	rules       []Validator[time.Time] // the validators of the value (see Validate)
	wire        fieldWire              // what a Field gives the field (see Field)
	focusHandle *FocusHandle           // see FocusHandle
	ctlSize     controlSize            // see Small and Large
	tooltip     string                 // see Tooltip
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (p *TimePickerElement) Tooltip(s string) *TimePickerElement {
	p.tooltip = s
	return p
}

// Small makes the field compact: 28 px high with smaller text, like
// Button.Small and TextInput.Small, for toolbars and dense forms.
func (p *TimePickerElement) Small() *TimePickerElement {
	p.ctlSize = sizeSmall
	return p
}

// Large makes the field larger: 40 px high with 16 px text, like
// Button.Large and TextInput.Large, for touch-friendly or prominent forms.
// Small and Large replace each other.
func (p *TimePickerElement) Large() *TimePickerElement {
	p.ctlSize = sizeLarge
	return p
}

// FocusHandle lets code focus the time field (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (p *TimePickerElement) FocusHandle(h *FocusHandle) *TimePickerElement {
	p.focusHandle = h
	return p
}

// TimePicker creates a time picker that reads and writes value, with a list of the times
// every 15 minutes (see MinuteStep). The default width is 140 px at the Text size of 100
// percent. A zero time is no time, and the field is empty.
func TimePicker(value *Signal[time.Time]) *TimePickerElement {
	return &TimePickerElement{value: value, step: timePickerDefaultStep}
}

// MinuteStep sets the minutes between the times of the list, and the step of Up and Down.
// The default is 15. A step below one minute is ignored.
func (p *TimePickerElement) MinuteStep(minutes int) *TimePickerElement {
	if minutes >= 1 {
		p.step = minutes
	}
	return p
}

// Min sets the earliest time that can be chosen. Only its clock is used; a zero time means
// there is no earliest time.
func (p *TimePickerElement) Min(t time.Time) *TimePickerElement { p.min = t; return p }

// Max sets the latest time that can be chosen. Only its clock is used; a zero time means
// there is no latest time.
func (p *TimePickerElement) Max(t time.Time) *TimePickerElement { p.max = t; return p }

// Validate sets the validators of the value. A zero time is no time, so Required rejects an
// empty field (see Field).
func (p *TimePickerElement) Validate(rules ...Validator[time.Time]) *TimePickerElement {
	p.rules = rules
	return p
}

// formCheck and formWire make a time picker a formControl (see Field).
func (p *TimePickerElement) formCheck() error { return firstError(p.value.Get(), p.rules) }

func (p *TimePickerElement) formWire(w fieldWire) Element {
	c := *p
	c.wire = w
	return &c
}

func (*TimePickerElement) isElement()           {}
func (p *TimePickerElement) elementKey() string { return p.key }

func (p *TimePickerElement) Key(k string) *TimePickerElement { p.key = k; return p }

// Label names the field for assistive technology, such as "Start time".
func (p *TimePickerElement) Label(name string) *TimePickerElement { p.name = name; return p }

// LabelledBy names the field by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (p *TimePickerElement) LabelledBy(keys ...string) *TimePickerElement {
	p.labelledBy = keys
	return p
}

// Placeholder is the text shown while the field is empty. The default is the hint of the
// locale's clock, such as "h:mm AM" on a 12 hour clock (see timePlaceholder).
func (p *TimePickerElement) Placeholder(s string) *TimePickerElement { p.placeholder = s; return p }

// Disabled shows the field, with its time, and does not let it change.
func (p *TimePickerElement) Disabled(on bool) *TimePickerElement { p.disabled = on; return p }

// OnChange runs fn with the new value after the user chooses a time, or types one that can be
// chosen. It does not run when the value is set from outside.
func (p *TimePickerElement) OnChange(fn func(time.Time)) *TimePickerElement {
	p.onChange = fn
	return p
}

// The sizing methods give the field a size. The field is timePickerWidth wide at the Text
// size of 100 percent by default; Width(Full) fills its container, and the list is at least
// as wide as the field. See Sizing in docs/styling.md.
func (p *TimePickerElement) Width(l Length) *TimePickerElement    { p.size.setWidth(l); return p }
func (p *TimePickerElement) Height(l Length) *TimePickerElement   { p.size.setHeight(l); return p }
func (p *TimePickerElement) MinWidth(l Length) *TimePickerElement { p.size.setMinWidth(l); return p }
func (p *TimePickerElement) MinHeight(l Length) *TimePickerElement {
	p.size.setMinHeight(l)
	return p
}
func (p *TimePickerElement) MaxWidth(l Length) *TimePickerElement { p.size.setMaxWidth(l); return p }
func (p *TimePickerElement) MaxHeight(l Length) *TimePickerElement {
	p.size.setMaxHeight(l)
	return p
}
func (p *TimePickerElement) Grow(f float32) *TimePickerElement { p.size.setGrow(f); return p }
func (p *TimePickerElement) GridSpan(cols, rows int) *TimePickerElement {
	p.size.setSpan(cols, rows)
	return p
}

// Styles sets the look of the parts of the time picker, for this instance only. Set fields
// win over the theme's TimePicker and the defaults; see TimePickerStyle and docs/styling.md.
// Calling Styles again merges with the earlier values.
func (p *TimePickerElement) Styles(st TimePickerStyle) *TimePickerElement {
	p.styles = p.styles.merge(st)
	return p
}

// bounds returns the first and the last minute of the day that can be chosen.
func (p *TimePickerElement) bounds() (int, int) {
	lo, hi := 0, 24*60-1
	if !p.min.IsZero() {
		lo = clockOf(p.min).minutes()
	}
	if !p.max.IsZero() {
		hi = clockOf(p.max).minutes()
	}
	return lo, hi
}

// times returns the minutes after midnight of the times of the list: every step minutes
// from the earliest time that can be chosen to the latest.
func (p *TimePickerElement) times() []int {
	lo, hi := p.bounds()
	var out []int
	for m := lo; m <= hi; m += p.step {
		out = append(out, m)
	}
	return out
}

// refusal returns the message that says why the clock of t cannot be chosen, or "" when it
// can.
func (p *TimePickerElement) refusal(m int, loc *dateLocale) string {
	lo, hi := p.bounds()
	if m < lo || m > hi {
		return "Enter a time from " + formatClock(clockAt(lo), loc) + " to " + formatClock(clockAt(hi), loc) + "."
	}
	return ""
}

func (p *TimePickerElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		rt := ctx.rt
		loc := currentLocale()
		scale := textScale(ctx)
		st := resolveTimePicker(t, scale, p.styles)
		v := p.value.Get()
		draft := Local(ctx, "draft", formatTimeText(v, loc))
		seen := Local(ctx, "seen", v)
		invalid := Local(ctx, "invalid", false)
		msg := Local(ctx, "msg", "")
		open := Local(ctx, "open", false)
		hl := Local(ctx, "hl", -1) // the index of the highlighted time in the list, or -1
		ls := Local(ctx, "list", &timeList{}).Peek()
		back := Local(ctx, "back", NewFocusHandle())
		ret := back.Peek()
		if p.focusHandle != nil {
			ret = p.focusHandle
		}
		w := p.wire
		if w.handle != nil {
			ret = w.handle
		}
		if !v.Equal(seen.Peek()) {
			draft.Set(formatTimeText(v, loc))
			seen.Set(v)
			invalid.Set(false)
		}
		times := p.times()

		// base is the day the time is set on: the day of the value, or today.
		base := func() time.Time {
			if cur := p.value.Peek(); !cur.IsZero() {
				return cur
			}
			return rt.now()
		}
		// set makes the time at minute m the value, and says so to the field and the app.
		set := func(m int) {
			nv := withClock(base(), clockAt(m))
			p.value.Set(nv)
			seen.Set(nv)
			draft.Set(formatTimeText(nv, loc))
			invalid.Set(false)
			if w.onEdit != nil {
				w.onEdit()
			}
			if p.onChange != nil {
				p.onChange(nv)
			}
		}
		// commit applies the text as the value, and reports whether it did. An empty text is
		// no time; otherwise the text must be a time that can be chosen, or the message says
		// why, the value is kept, and the field is invalid.
		commit := func(text string) bool {
			text = strings.TrimSpace(text)
			if text == "" {
				nv := time.Time{}
				p.value.Set(nv)
				seen.Set(nv)
				draft.Set("")
				invalid.Set(false)
				if w.onEdit != nil {
					w.onEdit()
				}
				if p.onChange != nil {
					p.onChange(nv)
				}
				return true
			}
			c, ok := parseClock(text, loc)
			if !ok {
				msg.Set("Enter a time such as " + formatClock(clock{hour: 9, minute: 30}, loc) + ".")
				invalid.Set(true)
				return false
			}
			if why := p.refusal(c.minutes(), loc); why != "" {
				msg.Set(why)
				invalid.Set(true)
				return false
			}
			set(c.minutes())
			return true
		}
		enter := w.onEnter
		w.onEnter = nil
		// blur applies a text that changed and is valid, and otherwise shows the value again.
		blur := func() {
			cur := formatTimeText(p.value.Peek(), loc)
			if draft.Peek() == cur {
				return
			}
			if !commit(draft.Peek()) {
				draft.Set(cur)
				invalid.Set(false)
			}
		}
		// shut closes the list, and the highlight goes with it.
		shut := func() {
			open.Set(false)
			hl.Set(-1)
			ls.reveal = 0
		}
		// indexOf returns the index in times of the minute the field holds, or -1.
		current := func() int {
			if v.IsZero() {
				return -1
			}
			m := clockOf(v).minutes()
			for i, x := range times {
				if x == m {
					return i
				}
			}
			return -1
		}
		// openList opens the list, with the time the field holds highlighted, or the first
		// time when there is none. It does nothing when no time can be chosen.
		openList := func() {
			if p.disabled || open.Peek() || len(times) == 0 {
				return
			}
			i := current()
			if i < 0 {
				i = 0
			}
			open.Set(true)
			hl.Set(i)
			ls.reveal = i + 1
		}
		// move highlights the row i, and has it scrolled into view.
		move := func(i int) {
			hl.Set(i)
			ls.reveal = i + 1
		}
		// step is Up (dir 1) and Down (dir -1). With the list open, the highlight moves along the
		// list as the rows show it: Down to the next row, Up to the previous one. With the list
		// closed, the time moves forward for Up and back for Down, as a spin button does.
		step := func(dir int) {
			if open.Peek() {
				i := hl.Peek()
				switch {
				case i < 0 && dir < 0:
					move(0)
				case i < 0:
					move(len(times) - 1)
				case i-dir >= 0 && i-dir < len(times):
					move(i - dir)
				}
				return
			}
			if len(times) == 0 {
				return
			}
			// The next time of the list in direction dir from the time the field holds.
			i := current()
			if i < 0 {
				m := -1
				if !v.IsZero() {
					m = clockOf(v).minutes()
				}
				i = nextTimeIndex(times, m, dir)
				if i < 0 {
					return
				}
			} else {
				i += dir
				if i < 0 || i >= len(times) {
					return
				}
			}
			set(times[i])
		}
		// choose makes the time at index i the value, and closes the list.
		choose := func(i int) {
			set(times[i])
			shut()
		}

		root := Div().Column()
		if p.size.set&hasWidth == 0 {
			root.style.Width = Px(timePickerWidth * scale)
		}
		if p.key != "" {
			root.Key(p.key)
		}
		a := root.access()
		root.Role(access.RoleComboBox).AccessExpanded(open.Get())
		if p.tooltip != "" && w.description == "" {
			root.Description(p.tooltip) // the combo box is the node assistive technology reads
		}
		name, labelledBy := p.name, p.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		if name != "" {
			root.Label(name)
		}
		if len(labelledBy) > 0 {
			root.LabelledBy(labelledBy...)
		}
		errKey := "timepicker-error"
		if p.key != "" {
			errKey = p.key + "/error"
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
				openList()
			} else {
				shut()
			}
		}
		a.invalid, a.required = isInvalid || w.invalid, w.required

		fst := resolveField(t, fieldLayout{width: Full, disabled: p.disabled, size: p.ctlSize}, st.Field)
		if p.disabled {
			a.disabled = true
			root.Cursor(CursorNotAllowed)
			frame := Div().asPart("TimePicker.Frame").Style(fst.Frame).Style(fst.FrameDisabled).
				Child(Div().asPart("TimePicker.Text").Style(fst.Text).Grow(1).Child(Text(draft.Get()).NoWrap()))
			root.Child(frame)
			return p.size.apply(root)
		}

		// The button is the trailing element of the frame, and it is not a Tab stop: Alt+Down
		// and F4 open the list from the field.
		button := Div().asPart("TimePicker.Button").Style(st.Button).Role(access.RoleButton).
			Label("Show times").Cursor(CursorPointer).
			OnClick(func() {
				if open.Peek() {
					shut()
				} else {
					openList()
				}
			}).Child(Icon(icons.ChevronDown).Size(16 * scale))

		var describedBy []string
		if isInvalid {
			describedBy = []string{errKey}
		}
		placeholder := p.placeholder
		if placeholder == "" {
			placeholder = loc.timePlaceholder()
		}
		fo := fieldOptions{tooltip: p.tooltip, value: draft, placeholder: placeholder, name: name, labelledBy: labelledBy,
			describedBy: describedBy, widget: "TimePicker", handle: ret, wire: w,
			onSubmit: func(text string) {
				// Enter chooses the highlighted time, or applies the text.
				if open.Peek() && hl.Peek() >= 0 && hl.Peek() < len(times) {
					choose(hl.Peek())
					return
				}
				if commit(text) && enter != nil {
					enter()
				}
			},
			onChange: func(string) { invalid.Set(false) }, onBlur: func() { shut(); blur() },
			onStep: step, trailing: button}
		fo.wire.invalid = isInvalid || w.invalid
		fo.size.setWidth(Full)
		root.Child(fo.expand(ctx, fst, false, 0))
		root.Child(Div().asPart("TimePicker.Message").Style(st.Message).Display(isInvalid).
			Child(Icon(icons.Alert).Size(14).Color(messageIconColor(t, st.Message))).
			Child(Text(shown).Key(errKey)))

		// Alt+Down and F4 open the list, Escape and Tab close it. A press outside it closes it.
		root.OnKeyDown(func(e *KeyEvent) {
			switch {
			case e.Key == input.KeyTab:
				shut()
			case e.Key == input.KeyEscape && open.Peek() && !e.Repeat:
				shut()
				e.StopPropagation()
			case (e.Key == input.KeyDown && e.Mods == input.ModAlt || e.Key == input.KeyF4 && e.Mods == 0) && !e.Repeat:
				openList()
				e.StopPropagation()
			}
		})

		if open.Get() {
			root.Child(p.list(st, scale, times, hl.Get(), v, ls, move, choose, shut, ret, name))
		}
		return p.size.apply(root)
	})
}

// formatTimeText returns the text of the clock of t in loc, or "" for a zero time.
func formatTimeText(t time.Time, loc *dateLocale) string {
	if t.IsZero() {
		return ""
	}
	return formatClock(clockOf(t), loc)
}

// nextTimeIndex returns the index of the first time of times after minute m when dir is
// positive, or the last one before m when it is negative. When m is -1 (no time) it
// returns the first or the last time. It returns -1 when there is none.
func nextTimeIndex(times []int, m, dir int) int {
	if dir > 0 {
		for i, x := range times {
			if m < 0 || x > m {
				return i
			}
		}
		return -1
	}
	for i := len(times) - 1; i >= 0; i-- {
		if m < 0 || times[i] < m {
			return i
		}
	}
	return -1
}

// list returns the open list of times: an overlay below the field that does not take focus.
// Its rows are list items, the highlighted one is the row the keys have reached, and the
// selected one is the time the field holds. A press in the list hands focus back to the
// text. reveal is the row (plus one, zero for none) that the keys moved to, and it is
// scrolled into view once.
func (p *TimePickerElement) list(st TimePickerStyle, scale float32, times []int, hl int, v time.Time,
	ls *timeList, move func(int), choose func(int), shut func(), ret *FocusHandle, name string) Element {
	loc := currentLocale()
	if name == "" {
		name = "Times"
	}
	reveal := ls.reveal
	ls.reveal = 0
	maxH := float32(timePickerMaxVisible)*timePickerRowHeight*scale + 8
	panel := Div().asPart("TimePicker.Popup").Key("jamb.listbox").Style(st.Popup).
		Overlay(BelowStart).OverlayGap(4).
		Role(access.RoleList).SelectionContainer().Label(name).
		MaxHeight(Px(maxH)).ScrollY().
		OnClickOutside(shut).
		OnClick(func() {}).
		OnMouseDown(func(*MouseEvent) { ret.Focus() })
	panel.matchAnchor = true
	selected := -1
	if !v.IsZero() {
		m := clockOf(v).minutes()
		for i, x := range times {
			if x == m {
				selected = i
			}
		}
	}
	for i, m := range times {
		i, m := i, m
		on := i == selected
		hi := i == hl
		style := st.Option
		if hi {
			style = style.Merge(st.OptionHighlighted)
		}
		if on {
			style = style.Merge(st.OptionSelected)
		}
		mark := Div().Size(Px(16), Px(16)).Center().Shrink(0)
		if on {
			mark.Child(Icon(icons.Check).Size(14))
		}
		label := formatClock(clockAt(m), loc)
		row := Div().asPart("TimePicker.Option").Style(style).
			Role(access.RoleListItem).AccessSelected(on).Label(label).Child(mark).
			Child(Text(label).NoWrap())
		row.revealRow = reveal == i+1
		row.access().highlighted = hi
		row.OnMouseEnter(func() { move(i) }).OnClick(func() { choose(i) })
		panel.Child(row)
	}
	return panel
}
