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
		Control:             Style{Direction: Row, Align: Center, Gap: 8, Radius: 4, NoShrink: true},
		ControlFocusVisible: Style{Shadows: []Shadow{ring}},
		Box: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(18), Height: Px(18), NoShrink: true,
			Radius: 4, BorderWidth: 1, BorderColor: t.BorderStrong, Background: t.Surface},
		BoxHover:   Style{BorderColor: t.TextMuted},
		Mark:       Style{TextColor: t.AccentText},
		Dash:       Style{TextColor: t.AccentText},
		BoxChecked: Style{Background: t.Accent, BorderColor: t.Accent},
		BoxMixed:   Style{Background: t.Accent, BorderColor: t.Accent},
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
}

// Checkbox creates a check box that is on while checked is true.
func Checkbox(label string, checked *Signal[bool]) *CheckboxElement {
	return &CheckboxElement{label: label, state: checkOf(checked.Get()), set: func(s CheckState) { checked.Set(s == CheckOn) }}
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
	row.Role(access.RoleCheckBox).Checked(on)
	a.mixed = s == CheckMixed
	if c.disabled {
		row.Style(st.ControlDisabled).Cursor(CursorNotAllowed)
		a.disabled = true
		return row
	}
	// The row takes focus and keys. The box shows the pointer's hover state.
	box.Hover(st.BoxHover)
	row.Cursor(CursorPointer).Focusable().FocusVisible(st.ControlFocusVisible).
		OnClick(func() { c.set(s.next()) })
	return row
}
