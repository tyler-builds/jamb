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

// CheckboxElement is a check box with a text label. Space, a click on the box
// or on its label, changes it. Enter does not change it: it goes on to the
// modal's default button or the form, as on Windows.
type CheckboxElement struct {
	label    string
	state    CheckState
	set      func(CheckState)
	key      string
	disabled bool
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

func (c *CheckboxElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	s := c.state
	on := s == CheckOn // a mixed box is not checked: its state is the mixed one
	ring := Shadow{Spread: 2, Color: t.FocusRing}

	// The colors of the box. Under high contrast a solid outline in the text
	// color, or in Highlight when on, and a fill of Highlight when on or mixed.
	// A disabled box is drawn in GrayText there, and not dimmed.
	fill, edge, mark, width := t.Surface, t.BorderStrong, t.AccentText, float32(1)
	if s != CheckOff {
		fill, edge = t.Accent, t.Accent
	}
	if t.HighContrast {
		fill, edge, width = t.Surface, t.Text, 2
		if s != CheckOff {
			fill, edge = t.Accent, t.Accent
		}
		if c.disabled {
			edge = t.Disabled
			if s != CheckOff {
				fill, mark = t.Disabled, t.Surface
			}
		}
	}
	// The box is drawn from the state: a check for on, a dash for mixed and an
	// empty box for off, so the state is not carried by color alone.
	box := Div().Size(Px(18), Px(18)).Shrink(0).Radius(4).Center().
		Border(width, edge).Background(fill)
	switch s {
	case CheckOn:
		box.Child(Icon(icons.Check).Size(14).Color(mark))
	case CheckMixed:
		box.Child(Icon(icons.Minus).Size(14).Color(mark))
	}

	row := Div().Row().Align(Center).Gap(8).Radius(4).Shrink(0).
		Child(box).Child(Text(c.label))
	row.spaceOnly = true
	a := row.access()
	row.Role(access.RoleCheckBox).Checked(on)
	a.mixed = s == CheckMixed
	if c.disabled {
		row.Cursor(CursorNotAllowed)
		a.disabled = true
		if t.HighContrast {
			row.TextColor(t.Disabled)
		} else {
			row.Opacity(0.5)
		}
		return row
	}
	// The row takes focus and keys. The box shows the pointer's hover state.
	box.Hover(Style{BorderColor: t.TextMuted})
	if on {
		box.Hover(Style{Background: t.AccentHover, BorderColor: t.AccentHover})
	}
	row.Cursor(CursorPointer).Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
		OnClick(func() { c.set(s.next()) })
	return row
}
