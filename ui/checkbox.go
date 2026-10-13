package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// CheckState is the state of a check box. CheckMixed is the indeterminate
// state: some of the items a box stands for are checked and some are not, as
// in a "select all" box.
type CheckState uint8

const (
	CheckOff CheckState = iota
	CheckOn
	CheckMixed
)

// next is the state a click or Space changes s to. A mixed box becomes on, as
// Windows check boxes do.
func (s CheckState) next() CheckState {
	if s == CheckOn {
		return CheckOff
	}
	return CheckOn
}

// CheckboxStyle is the look of each part of a check box. A part's style merges
// over the built-in defaults, then over the theme's Checkbox, then over the
// Styles of the instance; see docs/styling.md. Control is the row that takes
// focus and holds the box and the label. The box is drawn in the state of the
// check: BoxChecked for an on box, BoxMixed for a mixed one, and BoxDisabled
// when the box cannot change. Mark is the check mark of an on box and Dash the
// line of a mixed one; the mark's color is its TextColor. The states are parts
// of their own: BoxHover while the pointer is over an enabled box,
// ControlFocusVisible while the control has keyboard focus, and ControlDisabled
// and LabelDisabled when the box cannot change.
type CheckboxStyle struct {
	Control             Style // the row: the focus element, with the box and the label
	ControlFocusVisible Style // the row while it has keyboard focus
	ControlDisabled     Style // the row when it is disabled
	Box                 Style // the box: its size, shape, border and fill
	BoxHover            Style // the box while the pointer is over it, if enabled
	BoxChecked          Style // the box when it is on
	BoxMixed            Style // the box when it is mixed
	BoxDisabled         Style // the box when it is disabled
	BoxInvalid          Style // the box when the check's value is not valid (see Field)
	Mark                Style // the check mark of an on box
	Dash                Style // the line of a mixed box
	Label               Style // the text beside the box
	LabelDisabled       Style // the text when the box is disabled
}

// merge returns s with the set fields of o applied on top, part by part.
func (s CheckboxStyle) merge(o CheckboxStyle) CheckboxStyle {
	return CheckboxStyle{
		Control:             s.Control.Merge(o.Control),
		ControlFocusVisible: s.ControlFocusVisible.Merge(o.ControlFocusVisible),
		ControlDisabled:     s.ControlDisabled.Merge(o.ControlDisabled),
		Box:                 s.Box.Merge(o.Box),
		BoxHover:            s.BoxHover.Merge(o.BoxHover),
		BoxChecked:          s.BoxChecked.Merge(o.BoxChecked),
		BoxMixed:            s.BoxMixed.Merge(o.BoxMixed),
		BoxDisabled:         s.BoxDisabled.Merge(o.BoxDisabled),
		BoxInvalid:          s.BoxInvalid.Merge(o.BoxInvalid),
		Mark:                s.Mark.Merge(o.Mark),
		Dash:                s.Dash.Merge(o.Dash),
		Label:               s.Label.Merge(o.Label),
		LabelDisabled:       s.LabelDisabled.Merge(o.LabelDisabled),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s CheckboxStyle) plain() CheckboxStyle {
	return CheckboxStyle{
		Control:             plainStyle(s.Control),
		ControlFocusVisible: plainStyle(s.ControlFocusVisible),
		ControlDisabled:     plainStyle(s.ControlDisabled),
		Box:                 plainStyle(s.Box),
		BoxHover:            plainStyle(s.BoxHover),
		BoxChecked:          plainStyle(s.BoxChecked),
		BoxMixed:            plainStyle(s.BoxMixed),
		BoxDisabled:         plainStyle(s.BoxDisabled),
		BoxInvalid:          plainStyle(s.BoxInvalid),
		Mark:                plainStyle(s.Mark),
		Dash:                plainStyle(s.Dash),
		Label:               plainStyle(s.Label),
		LabelDisabled:       plainStyle(s.LabelDisabled),
	}
}

// checkboxDefaults returns the built-in look of a check box in state s, from the
// colors of t. Under high contrast the box is outlined in the text color, or in
// Highlight when on or mixed, and a disabled box is GrayText and not dimmed.
func checkboxDefaults(t *Theme, s CheckState, disabled bool) CheckboxStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := CheckboxStyle{
		Control:             Style{Direction: Row, Align: Center, Gap: 8, Radius: 4, NoShrink: true, fitWidth: true},
		ControlFocusVisible: Style{Shadows: []Shadow{ring}},
		Box: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(18), Height: Px(18), NoShrink: true,
			Radius: 4, BorderWidth: 1, BorderColor: t.BorderStrong, Background: t.Surface},
		BoxHover:   Style{BorderColor: t.TextMuted},
		Mark:       Style{TextColor: t.AccentText},
		Dash:       Style{TextColor: t.AccentText},
		BoxChecked: Style{Background: t.Accent, BorderColor: t.Accent},
		BoxMixed:   Style{Background: t.Accent, BorderColor: t.Accent},
		BoxInvalid: Style{BorderWidth: 2, BorderColor: messageIconColor(t, Style{})},
	}
	if s == CheckOn {
		d.BoxHover = Style{Background: t.AccentHover, BorderColor: t.AccentHover}
	}
	if t.HighContrast {
		// The box is a solid outline in the text color, and a filled box is Highlight.
		d.Box.BorderWidth, d.Box.BorderColor = 2, t.Text
		if disabled {
			d.BoxDisabled = Style{BorderColor: t.Disabled}
			d.LabelDisabled = Style{TextColor: t.Disabled}
			if s != CheckOff {
				d.BoxDisabled.Background = t.Disabled
				d.Mark.TextColor, d.Dash.TextColor = t.Surface, t.Surface
			}
		}
	} else if disabled {
		d.ControlDisabled = Style{Opacity: 0.5}
	}
	return d
}

// resolveCheckbox returns the look of a check box in state s: the defaults,
// merged with the theme's Checkbox and then the instance's Styles. Under high
// contrast the two overrides are plain (see plainStyle).
func resolveCheckbox(t *Theme, s CheckState, disabled bool, inst CheckboxStyle) CheckboxStyle {
	st := checkboxDefaults(t, s, disabled)
	for _, o := range []CheckboxStyle{t.Checkbox, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// CheckboxElement is a check box with a text label. Space, a click on the box
// or on its label, changes it. Enter does not change it: it goes on to the
// modal's default button or the form, as on Windows.
type CheckboxElement struct {
	label    string
	state    CheckState
	set      func(CheckState)
	key      string
	disabled bool
	styles   CheckboxStyle
	value    *Signal[bool]     // the checked value of a Checkbox, nil for a CheckboxTri
	rules    []Validator[bool] // the validators of the value (see Validate)
	wire     fieldWire         // what a Field gives the check box (see Field)
	handle   *FocusHandle      // lets code focus the check box (see FocusHandle)
	size     sizing            // the control's sizes, from the sizing methods (see Sizing in docs/styling.md)
	onChange func(bool)        // see OnChange
	tooltip  string            // see Tooltip
}

// Tooltip shows s below the check box after the pointer rests on it. Assistive
// technology reads it as the check box's description, unless it has another.
func (c *CheckboxElement) Tooltip(s string) *CheckboxElement {
	c.tooltip = s
	return c
}

// OnChange runs fn with the box's new state after the user checks or clears it,
// by a click, the Space key or assistive technology. The signal is set first. A
// click on a CheckboxTri sets it on or off, never mixed, so fn gets true for on.
func (c *CheckboxElement) OnChange(fn func(checked bool)) *CheckboxElement {
	c.onChange = fn
	return c
}

// FocusHandle lets code focus the check box (see NewFocusHandle). A Field that
// wraps the check box gives it its own handle, which takes precedence.
func (c *CheckboxElement) FocusHandle(h *FocusHandle) *CheckboxElement {
	c.handle = h
	return c
}

// Checkbox creates a check box that is on while checked is true.
func Checkbox(label string, checked *Signal[bool]) *CheckboxElement {
	return &CheckboxElement{label: label, state: checkOf(checked.Get()), set: func(s CheckState) { checked.Set(s == CheckOn) },
		value: checked}
}

// Validate sets the validators of the checked value. Required[bool] makes the box
// "must accept": it is an error while the box is off (see Field). A mixed or tri-state
// box (CheckboxTri) has no validators.
func (c *CheckboxElement) Validate(rules ...Validator[bool]) *CheckboxElement {
	c.rules = rules
	return c
}

// formValue, formCheck and formWire make a check box a formControl (see Field).
func (c *CheckboxElement) formCheck() error {
	if c.value == nil {
		return nil
	}
	return firstError(c.value.Get(), c.rules)
}

func (c *CheckboxElement) formWire(w fieldWire) Element {
	cp := *c
	cp.wire = w
	return &cp
}

// CheckboxTri creates a check box that shows state, including CheckMixed. It
// is the parent of a group of items that are only some of them checked. Read
// the state with Peek, to know whether the group is all, some or none.
func CheckboxTri(label string, state *Signal[CheckState]) *CheckboxElement {
	return &CheckboxElement{label: label, state: state.Get(), set: state.Set}
}

func checkOf(on bool) CheckState {
	if on {
		return CheckOn
	}
	return CheckOff
}

func (*CheckboxElement) isElement()           {}
func (c *CheckboxElement) elementKey() string { return c.key }

func (c *CheckboxElement) Key(k string) *CheckboxElement { c.key = k; return c }

// Disabled shows the box but does not let it change.
func (c *CheckboxElement) Disabled(d bool) *CheckboxElement { c.disabled = d; return c }

// The sizing methods give the check box's control, its box and label, a size. It
// is as wide as its label by default; see Sizing in docs/styling.md.
func (c *CheckboxElement) Width(l Length) *CheckboxElement     { c.size.setWidth(l); return c }
func (c *CheckboxElement) Height(l Length) *CheckboxElement    { c.size.setHeight(l); return c }
func (c *CheckboxElement) MinWidth(l Length) *CheckboxElement  { c.size.setMinWidth(l); return c }
func (c *CheckboxElement) MinHeight(l Length) *CheckboxElement { c.size.setMinHeight(l); return c }
func (c *CheckboxElement) MaxWidth(l Length) *CheckboxElement  { c.size.setMaxWidth(l); return c }
func (c *CheckboxElement) MaxHeight(l Length) *CheckboxElement { c.size.setMaxHeight(l); return c }
func (c *CheckboxElement) Grow(f float32) *CheckboxElement     { c.size.setGrow(f); return c }
func (c *CheckboxElement) GridSpan(cols, rows int) *CheckboxElement {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the check box, for this instance only.
// Set fields win over the theme's Checkbox and the defaults; see CheckboxStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (c *CheckboxElement) Styles(st CheckboxStyle) *CheckboxElement {
	c.styles = c.styles.merge(st)
	return c
}

func (c *CheckboxElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	s := c.state
	on := s == CheckOn // a mixed box is not checked: its state is the mixed one
	st := resolveCheckbox(t, s, c.disabled, c.styles)
	w := c.wire

	// The box is drawn from the state: a check for on, a dash for mixed and an
	// empty box for off, so the state is not carried by color alone.
	box := Div().asPart("Checkbox.Box").Style(st.Box)
	switch s {
	case CheckOn:
		box.Style(st.BoxChecked).Child(Div().asPart("Checkbox.Mark").Style(st.Mark).Child(Icon(icons.Check).Size(14)))
	case CheckMixed:
		box.Style(st.BoxMixed).Child(Div().asPart("Checkbox.Dash").Style(st.Dash).Child(Icon(icons.Minus).Size(14)))
	}
	label := Div().asPart("Checkbox.Label").Style(st.Label)
	if c.disabled {
		box.Style(st.BoxDisabled)
		label.Style(st.LabelDisabled)
	}
	label.Child(Text(c.label))

	row := Div().asPart("Checkbox.Control").Style(st.Control).Child(box).Child(label)
	row.spaceOnly = true
	a := row.access()
	row.Role(access.RoleCheckBox).AccessChecked(on)
	a.mixed = s == CheckMixed
	a.invalid, a.required = w.invalid, w.required
	if w.description != "" {
		row.Description(w.description)
	}
	if w.invalid {
		box.Style(st.BoxInvalid)
	}
	// A tooltip also shows on a disabled box, to say why it is disabled.
	row.Tooltip(c.tooltip)
	if c.disabled {
		row.Style(st.ControlDisabled).Cursor(CursorNotAllowed)
		a.disabled = true
		return c.size.apply(row)
	}
	// The row takes focus and keys. The box shows the pointer's hover state.
	box.Hover(st.BoxHover)
	row.Cursor(CursorPointer).Focusable().FocusVisible(st.ControlFocusVisible).
		OnClick(func() {
			next := s.next()
			c.set(next)
			if c.onChange != nil {
				c.onChange(next == CheckOn)
			}
			if w.onEdit != nil {
				w.onEdit()
			}
		})
	h := c.handle
	if w.handle != nil {
		h = w.handle
	}
	if h != nil {
		row.FocusHandle(h)
	}
	if w.onBlur != nil {
		row.OnBlur(w.onBlur)
	}
	return c.size.apply(row)
}
