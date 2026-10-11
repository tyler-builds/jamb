package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// SpinPlacement is where a number field's spin buttons are drawn. The zero
// value is SpinInline.
type SpinPlacement uint8

const (
	// SpinInline draws a decrease and an increase button after the field, as two
	// square buttons with a minus and a plus sign.
	SpinInline SpinPlacement = iota
	// SpinStacked draws an up arrow above a down arrow at the right of the field,
	// as the up-down control of Windows does, in a compact column.
	SpinStacked
)

// NumberFieldStyle is the look of each part of a number field. A part's style
// merges over the built-in defaults, then over the theme's Field and NumberField,
// then over the Styles of the instance; see docs/styling.md.
//
// The text frame is FieldStyle: Frame, with its states FrameHover,
// FrameFocusWithin, FrameDisabled and FrameInvalid, and the Text and Selection
// parts. Placeholder, Leading and Trailing are in FieldStyle too, but a number
// field has no placeholder, icon or element, so they do not apply to it. Message
// is the row that says why the text is invalid: its icon is drawn in the
// message's TextColor when it sets one, and in Danger (the text color under high
// contrast) otherwise, and its text takes the same color. The spin buttons are
// SpinButtons (the group), SpinButton (each button), and its states
// SpinButtonHover, SpinButtonPressed, SpinButtonFocusVisible and
// SpinButtonDisabled.
type NumberFieldStyle struct {
	FieldStyle
	SpinButtons            Style // the group of spin buttons
	SpinButton             Style // each spin button
	SpinButtonHover        Style // a spin button the pointer is over, if enabled
	SpinButtonPressed      Style // a spin button being pressed
	SpinButtonFocusVisible Style // a spin button that has keyboard focus
	SpinButtonDisabled     Style // a spin button that cannot step the value any further
	Message                Style // the row with the icon and the message, while the text is invalid
}

// merge returns s with the set fields of o applied on top, part by part.
func (s NumberFieldStyle) merge(o NumberFieldStyle) NumberFieldStyle {
	return NumberFieldStyle{
		FieldStyle:             s.FieldStyle.merge(o.FieldStyle),
		SpinButtons:            s.SpinButtons.Merge(o.SpinButtons),
		SpinButton:             s.SpinButton.Merge(o.SpinButton),
		SpinButtonHover:        s.SpinButtonHover.Merge(o.SpinButtonHover),
		SpinButtonPressed:      s.SpinButtonPressed.Merge(o.SpinButtonPressed),
		SpinButtonFocusVisible: s.SpinButtonFocusVisible.Merge(o.SpinButtonFocusVisible),
		SpinButtonDisabled:     s.SpinButtonDisabled.Merge(o.SpinButtonDisabled),
		Message:                s.Message.Merge(o.Message),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s NumberFieldStyle) plain() NumberFieldStyle {
	return NumberFieldStyle{
		FieldStyle:             s.FieldStyle.plain(),
		SpinButtons:            plainStyle(s.SpinButtons),
		SpinButton:             plainStyle(s.SpinButton),
		SpinButtonHover:        plainStyle(s.SpinButtonHover),
		SpinButtonPressed:      plainStyle(s.SpinButtonPressed),
		SpinButtonFocusVisible: plainStyle(s.SpinButtonFocusVisible),
		SpinButtonDisabled:     plainStyle(s.SpinButtonDisabled),
		Message:                plainStyle(s.Message),
	}
}

// numberFieldDefaults returns the built-in look of a number field of the given
// width, with spin buttons of placement, from the colors of t. A disabled field
// has the disabled frame (see fieldDefaults).
func numberFieldDefaults(t *Theme, width float32, placement SpinPlacement, scale float32, disabled bool) NumberFieldStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := NumberFieldStyle{
		FieldStyle:             fieldDefaults(t, fieldLayout{disabled: disabled}),
		SpinButton:             Style{NoShrink: true, TextColor: t.TextMuted, Transition: 110 * time.Millisecond},
		SpinButtonHover:        Style{Background: t.SurfaceHover, TextColor: t.Text},
		SpinButtonPressed:      Style{Background: t.SurfaceActive},
		SpinButtonFocusVisible: Style{Shadows: []Shadow{ring}},
		Message:                Style{Direction: Row, Align: Center, Gap: 6, FontSize: 12},
	}
	d.Frame.Width = Px(width)
	if t.HighContrast {
		// Disabled controls are GrayText.
		d.SpinButtonDisabled = Style{TextColor: t.Disabled}
	} else {
		d.SpinButtonDisabled = Style{Opacity: 0.5}
	}
	if placement == SpinStacked {
		// The column sits at the field's right edge, inside the frame, at the field's
		// height: its two buttons share that height. It scales with the Text size.
		// The frame has no gap and no right padding, so the column is at its edge.
		d.Frame.Gap, d.Frame.Padding, d.Frame.MinHeight = 0, Edges{Left: 10}, Px(34*scale)
		d.SpinButtons = Style{Direction: Column, Width: Px(18 * scale), Height: Px(34 * scale), NoShrink: true}
		d.SpinButton = Style{NoShrink: true, TextColor: t.TextMuted, Transition: 110 * time.Millisecond, Width: Full, Height: Px(17 * scale)}
		// A ring would be drawn outside the buttons, into the frame's clipped edge,
		// so the focus is an inner outline.
		d.SpinButtonFocusVisible = Style{BorderWidth: 2, BorderColor: t.FocusRing}
	} else {
		d.SpinButtons = Style{Direction: Row, Align: Center, Gap: 6}
		d.SpinButton.Width, d.SpinButton.Height = Px(28), Px(28)
		d.SpinButton.Radius = t.Radius
	}
	return d
}

// resolveNumberField returns the look of a number field: the defaults, merged
// with the theme's Field, then the theme's NumberField, then the instance's
// Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveNumberField(t *Theme, width float32, placement SpinPlacement, scale float32, disabled bool, inst NumberFieldStyle) NumberFieldStyle {
	st := numberFieldDefaults(t, width, placement, scale, disabled)
	for _, o := range []NumberFieldStyle{{FieldStyle: t.Field}, t.NumberField, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// NumberFieldElement is a text field for a number, bound to an int or float64
// signal, from lo to hi in steps of step. The user types a number, or steps it
// with Up and Down (and with the optional spin buttons).
//
// The text is checked when it is committed, by Enter or by leaving the field:
//   - a number outside the range is clamped to the nearest bound;
//   - text that is not a number (or, for an int, not a whole number) is not
//     applied. Enter shows the field as invalid, with a message that gives the
//     range, and keeps the text so it can be corrected. Leaving the field
//     reverts the text to the value that is set.
//
// Assistive technology reports it as a spin box, which holds the text field.
type NumberFieldElement[T int | float64] struct {
	value      *Signal[T]
	lo, hi     T
	step       T
	name       string // the accessible name, see Label
	labelledBy []string
	key        string
	disabled   bool
	spin       bool // set by SpinButtons
	placement  SpinPlacement
	precision  int // the decimals of a float, or -1 for the decimals of the step
	width      float32
	styles     NumberFieldStyle
}

// NumberField creates a number field that reads and writes value, from lo to hi
// in steps of step. Its default width is 120 px and it shows the decimals of
// step.
func NumberField[T int | float64](value *Signal[T], lo, hi, step T) *NumberFieldElement[T] {
	return &NumberFieldElement[T]{value: value, lo: lo, hi: hi, step: step, precision: -1, width: 120}
}

func (*NumberFieldElement[T]) isElement()           {}
func (n *NumberFieldElement[T]) elementKey() string { return n.key }

func (n *NumberFieldElement[T]) Key(k string) *NumberFieldElement[T] { n.key = k; return n }

// Label names the field for assistive technology, such as "Quiet hours from".
func (n *NumberFieldElement[T]) Label(s string) *NumberFieldElement[T] { n.name = s; return n }

// LabelledBy names the field by the text of the elements with the given keys;
// see DivElement.LabelledBy.
func (n *NumberFieldElement[T]) LabelledBy(keys ...string) *NumberFieldElement[T] {
	n.labelledBy = keys
	return n
}

// Disabled shows the number but does not let it change.
func (n *NumberFieldElement[T]) Disabled(d bool) *NumberFieldElement[T] { n.disabled = d; return n }

// SpinButtons adds the buttons that step the value, placed as p says: SpinInline
// draws a decrease and an increase button after the field, and SpinStacked draws
// an up arrow above a down arrow at its right, as Windows' up-down control does.
// Their look is in NumberFieldStyle.
func (n *NumberFieldElement[T]) SpinButtons(p SpinPlacement) *NumberFieldElement[T] {
	n.spin, n.placement = true, p
	return n
}

// Precision sets the number of decimals a float is shown with, so 2 shows 1.50.
// The default is the number of decimals of the step.
func (n *NumberFieldElement[T]) Precision(d int) *NumberFieldElement[T] { n.precision = d; return n }

// Width sets the width of the field, in logical pixels. It is the built-in width;
// the theme's NumberField and the instance's Styles can change it.
func (n *NumberFieldElement[T]) Width(px float32) *NumberFieldElement[T] { n.width = px; return n }

// Styles sets the look of the parts of the number field, for this instance only.
// Set fields win over the theme's NumberField and the defaults; see
// NumberFieldStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (n *NumberFieldElement[T]) Styles(st NumberFieldStyle) *NumberFieldElement[T] {
	n.styles = n.styles.merge(st)
	return n
}

func (n *NumberFieldElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		scale := float32(ctx.TextScale()) / 100
		st := resolveNumberField(t, n.width, n.placement, scale, n.disabled, n.styles)
		v := n.value.Get()
		// The field keeps its own text while the user types. draft is that text,
		// and seen is the value it was last made from, so that a value set from
		// outside the field replaces the text.
		draft := Local(ctx, "draft", n.format(v))
		seen := Local(ctx, "seen", v)
		invalid := Local(ctx, "invalid", false)
		if v != seen.Peek() {
			draft.Set(n.format(v))
			seen.Set(v)
			invalid.Set(false)
		}

		apply := func(x T) {
			x = n.clamp(x)
			n.value.Set(x)
			draft.Set(n.format(x))
			seen.Set(x)
			invalid.Set(false)
		}
		submit := func(text string) {
			x, ok := n.parse(text)
			if !ok {
				invalid.Set(true)
				return
			}
			apply(x)
		}
		blur := func() {
			if x, ok := n.parse(draft.Peek()); ok {
				apply(x)
				return
			}
			draft.Set(n.format(n.value.Peek()))
			invalid.Set(false)
		}
		step := func(dir int) {
			base, ok := n.parse(draft.Peek())
			if !ok {
				base = n.value.Peek()
			}
			apply(n.stepped(base, dir))
		}
		isInvalid := invalid.Get()
		errKey := "number-error"
		if n.key != "" {
			errKey = n.key + "/error"
		}

		root := Div().Column().Gap(6).Role(access.RoleSpinner)
		a := root.access()
		if n.name != "" {
			root.Label(n.name)
		}
		if len(n.labelledBy) > 0 {
			root.LabelledBy(n.labelledBy...)
		}
		a.rng = &access.Range{Value: float64(v), Min: float64(n.lo), Max: float64(n.hi),
			SmallChange: float64(n.step), LargeChange: 10 * float64(n.step)}
		a.setRange = func(x float64) { apply(numberFrom[T](x)) }
		a.invalid = isInvalid

		row := Div().Row().Align(Center).Gap(6)
		if n.disabled {
			a.disabled = true
			// The disabled number is drawn as text in a frame, with the disabled frame
			// and text parts. Under high contrast both are GrayText, with no dimming.
			frame := Div().asPart("NumberField.Frame").Style(st.Frame).Style(st.FrameDisabled)
			content := Div().asPart("NumberField.Text").Style(st.Text).Child(Text(draft.Get()))
			root.Child(row.Child(frame.Child(content)))
			return root
		}

		fo := fieldOptions{value: draft, name: n.name, labelledBy: n.labelledBy,
			onSubmit: submit, onChange: func(string) { invalid.Set(false) }, onBlur: blur, onStep: step,
			widget: "NumberField"}
		fst := st.FieldStyle
		if isInvalid {
			fo.describedBy = []string{errKey}
			// The invalid frame is drawn thicker and in the danger color (the text
			// color under high contrast), and the message says what is wrong.
			fst.Frame = fst.Frame.Merge(st.FrameInvalid)
		}
		// Stacked buttons are the frame's trailing element, inside its right edge.
		// Inline buttons follow the frame.
		if n.spin && n.placement == SpinStacked {
			fo.trailing = n.spinButtons(st, v, step, scale)
		}
		row.Child(fo.expand(ctx, fst, false, 0))
		if n.spin && n.placement != SpinStacked {
			row.Child(n.spinButtons(st, v, step, scale))
		}
		root.Child(row)
		if isInvalid {
			msg := fmt.Sprintf("Enter a number from %s to %s.", n.format(n.lo), n.format(n.hi))
			iconColor := t.Danger
			if t.HighContrast {
				iconColor = t.Text
			}
			if !st.Message.TextColor.IsZero() {
				iconColor = st.Message.TextColor
			}
			root.Child(Div().asPart("NumberField.Message").Style(st.Message).
				Child(Icon(icons.Alert).Size(14).Color(iconColor)).
				Child(Text(msg).Key(errKey)))
		}
		return root
	})
}

// spinButtons returns the group of spin buttons, in the placement set by
// SpinButtons. Each button is a button for assistive technology, is in the Tab
// order while enabled, and steps the value by one step when it is clicked. A
// button is disabled at the end of the range it would step past.
func (n *NumberFieldElement[T]) spinButtons(st NumberFieldStyle, v T, step func(int), scale float32) *DivElement {
	stacked := n.placement == SpinStacked
	iconSize := float32(14)
	decrease, increase := icons.Minus, icons.Plus
	if stacked {
		iconSize = 12 * scale
		decrease, increase = icons.ChevronDown, icons.ChevronUp
	}
	down := n.spinButton(st, -1, "Decrease", decrease, iconSize, v <= n.lo, step)
	up := n.spinButton(st, 1, "Increase", increase, iconSize, v >= n.hi, step)
	group := Div().asPart("NumberField.SpinButtons").Style(st.SpinButtons)
	if stacked {
		return group.Child(up).Child(down)
	}
	return group.Child(down).Child(up)
}

// spinButton returns one spin button, which steps the value by dir steps, or is
// disabled.
func (n *NumberFieldElement[T]) spinButton(st NumberFieldStyle, dir int, label string, ic *icons.Icon,
	iconSize float32, disabled bool, step func(int)) *DivElement {
	b := Div().asPart("NumberField.SpinButton").Row().Align(Center).Justify(Center).
		Style(st.SpinButton).Child(Icon(ic).Size(iconSize)).
		Role(access.RoleButton).Label(label)
	if disabled {
		b.access().disabled = true
		b.Style(st.SpinButtonDisabled).Cursor(CursorNotAllowed)
		return b
	}
	b.Focusable().Hover(st.SpinButtonHover).Pressed(st.SpinButtonPressed).
		FocusVisible(st.SpinButtonFocusVisible).
		OnClick(func() { step(dir) })
	return b
}

// clamp returns x within the range.
func (n *NumberFieldElement[T]) clamp(x T) T { return min(max(x, n.lo), n.hi) }

// stepped returns the next grid value from x in direction dir (+1 or -1), where
// the grid starts at lo and is spaced by step. A value between two grid values
// moves to the nearer one in that direction, so 5.5 steps up to 6.
func (n *NumberFieldElement[T]) stepped(x T, dir int) T {
	step := float64(n.step)
	if step <= 0 {
		return n.clamp(x)
	}
	k := (float64(x) - float64(n.lo)) / step
	if dir > 0 {
		k = math.Floor(k+1e-9) + 1
	} else {
		k = math.Ceil(k-1e-9) - 1
	}
	return n.clamp(numberFrom[T](float64(n.lo) + k*step))
}

// parse returns the number that text holds, and whether it is a valid value.
// For an int the number must be a whole number.
func (n *NumberFieldElement[T]) parse(text string) (T, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if _, isInt := any(n.lo).(int); isInt && f != math.Trunc(f) {
		return 0, false
	}
	return numberFrom[T](f), true
}

// format returns the text of x: a whole number for an int, and for a float the
// number of decimals set by Precision, or that of the step.
func (n *NumberFieldElement[T]) format(x T) string {
	switch v := any(x).(type) {
	case int:
		return strconv.Itoa(v)
	case float64:
		d := n.precision
		if d < 0 {
			d = decimalsOf(float64(n.step))
		}
		return strconv.FormatFloat(v, 'f', d, 64)
	}
	return ""
}

// numberFrom converts f to T. A float is rounded to the nearest int for an int.
func numberFrom[T int | float64](f float64) T {
	var zero T
	if _, isInt := any(zero).(int); isInt {
		return T(math.Round(f))
	}
	return T(f)
}
