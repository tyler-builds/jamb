package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/internal/layout"
)

// SpinPlacement is where a number input's spin buttons are drawn. The zero
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

// NumberInputStyle is the look of each part of a number input. A part's style
// merges over the built-in defaults, then over the theme's Field and NumberInput,
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
type NumberInputStyle struct {
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
func (s NumberInputStyle) merge(o NumberInputStyle) NumberInputStyle {
	return NumberInputStyle{
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
func (s NumberInputStyle) plain() NumberInputStyle {
	return NumberInputStyle{
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

// numberInputWidth is the default width of a number input at the Text size of 100
// percent, in logical pixels. It scales with the Text size.
const numberInputWidth = 120

// spinInlineWidth is the width of an inline spin button at the Text size setting of
// 100 percent, in logical pixels. It is the button's hit target, so it is wider than
// the glyph.
const spinInlineWidth = 24

// spinGap is the gap between a spin button's fill and the field's border, at the Text
// size setting of 100 percent, in logical pixels. It scales with the Text size, and
// with the DPI through the logical pixels. The fill is inset by it on every side that
// meets the border, as the Windows 11 NumberBox draws its buttons.
const spinGap = 2

// numberInputDefaults returns the built-in look of a number input of the given
// width, with spin buttons of placement when spin is set, from the colors of t. A
// disabled field has the disabled frame (see fieldDefaults).
func numberInputDefaults(t *Theme, placement SpinPlacement, scale float32, disabled, spin, small bool) NumberInputStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	// h is the frame's height at the Text size of 100 percent: 28 for Small, as
	// TextInput.Small is.
	h := float32(34)
	if small {
		h = 28
	}
	d := NumberInputStyle{
		FieldStyle:             fieldDefaults(t, fieldLayout{disabled: disabled, small: small}),
		SpinButton:             Style{NoShrink: true, TextColor: t.TextMuted, Transition: 110 * time.Millisecond},
		SpinButtonHover:        Style{Background: t.SurfaceHover, TextColor: t.Text},
		SpinButtonPressed:      Style{Background: t.SurfaceActive},
		SpinButtonFocusVisible: Style{Shadows: []Shadow{ring}},
		Message:                messageStyle(t),
	}
	d.Frame.Width = Full // the frame fills the field, whose width is its outer box's (see expand)
	if t.HighContrast {
		// Disabled controls are GrayText.
		d.SpinButtonDisabled = Style{TextColor: t.Disabled}
	} else {
		d.SpinButtonDisabled = Style{Opacity: 0.5}
	}
	switch {
	case spin && placement == SpinStacked:
		// The column sits at the field's right edge, inside the frame, at the field's
		// height: its two buttons share that height. It scales with the Text size.
		// The frame has no gap and no right padding, so the column is at its edge.
		d.Frame.Gap, d.Frame.Padding, d.Frame.MinHeight = 0, Edges{Left: 10}, Px(h*scale)
		// The column is inset by the gap from the border at its top, right and bottom, and
		// the two buttons are twice the gap apart, so each fill is inset on every side that
		// meets the border or the other button. The two buttons fill the rest of the column.
		gap := spinGap * scale
		d.SpinButtons = Style{Direction: Column, Width: Px(18 * scale), Height: Px(h * scale), NoShrink: true,
			Padding: Edges{Top: gap, Right: gap, Bottom: gap}, Gap: 2 * gap}
		// The two buttons share the column's height (see stackedSpin). They may be
		// shorter than their icon, inside the thicker border of an invalid field.
		d.SpinButton = Style{TextColor: t.TextMuted, Transition: 110 * time.Millisecond, Width: Full, Grow: 1, MinHeight: Px(0)}
		// A ring would be drawn outside the buttons, into the frame's clipped edge,
		// so the focus is an inner outline.
		d.SpinButtonFocusVisible = Style{BorderWidth: 2, BorderColor: t.FocusRing}
	case spin:
		// The buttons are laid over the frame's edges, the decrease at the left and the
		// increase at the right, so the frame is padded by their width and the text
		// keeps the middle. Both scale with the Text size, and so does the field's width.
		w := spinInlineWidth * scale
		d.Frame.Padding = Edges{Left: w + 4, Right: w + 4}
		d.SpinButton = Style{NoShrink: true, TextColor: t.TextMuted, Transition: 110 * time.Millisecond, Width: Px(w)}
		// As in the stacked column, the focus is an inner outline, inside the border.
		d.SpinButtonFocusVisible = Style{BorderWidth: 2, BorderColor: t.FocusRing}
	}
	return d
}

// resolveNumberInput returns the look of a number input: the defaults, merged
// with the theme's Field, then the theme's NumberInput, then the instance's
// Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveNumberInput(t *Theme, placement SpinPlacement, scale float32, disabled, spin, small bool, inst NumberInputStyle) NumberInputStyle {
	st := numberInputDefaults(t, placement, scale, disabled, spin, small)
	for _, o := range []NumberInputStyle{{FieldStyle: t.Field}, t.NumberInput, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// NumberInputElement is a text field for a number, bound to an int or float64
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
type NumberInputElement[T int | float64] struct {
	value       *Signal[T]
	lo, hi      T
	step        T
	name        string // the accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	spin        bool // set by SpinButtons
	placement   SpinPlacement
	align       Alignment // where the text sits in the field (see Align)
	precision   int       // the decimals of a float, or -1 for the decimals of the step
	size        sizing    // the frame's sizes, from the sizing methods (see Sizing in docs/styling.md)
	styles      NumberInputStyle
	rules       []Validator[T] // the validators of the value (see Validate)
	wire        fieldWire      // what a Field gives the field (see Field)
	onChange    func(T)        // see OnChange
	focusHandle *FocusHandle   // see FocusHandle
	small       bool           // see Small
	tooltip     string         // see Tooltip
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (n *NumberInputElement[T]) Tooltip(s string) *NumberInputElement[T] {
	n.tooltip = s
	return n
}

// Small makes the field compact: 28 px high with smaller text, like
// Button.Small and TextInput.Small, for toolbars and dense forms.
func (n *NumberInputElement[T]) Small() *NumberInputElement[T] {
	n.small = true
	return n
}

// FocusHandle lets code focus the number's text (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (n *NumberInputElement[T]) FocusHandle(h *FocusHandle) *NumberInputElement[T] {
	n.focusHandle = h
	return n
}

// OnChange runs fn with the new value after the user changes it: by typing and
// pressing Enter or leaving the field, by the Up and Down keys, by the spin
// buttons, or by assistive technology. The signal is set first. It does not run
// when the value stays the same, such as a step past the end of the range.
func (n *NumberInputElement[T]) OnChange(fn func(T)) *NumberInputElement[T] {
	n.onChange = fn
	return n
}

// Validate sets the validators of the value, which are checked after the value
// is set: a Field shows the first error once the user has changed the field and left
// it, and on submit when it is in a Form (see Field and Validator). A number input
// does not need Required, since it always holds a number.
func (n *NumberInputElement[T]) Validate(rules ...Validator[T]) *NumberInputElement[T] {
	n.rules = rules
	return n
}

// formCheck and formWire make a number input a formControl (see Field).
func (n *NumberInputElement[T]) formCheck() error { return firstError(n.value.Get(), n.rules) }

func (n *NumberInputElement[T]) formWire(w fieldWire) Element {
	c := *n
	c.wire = w
	return &c
}

// NumberInput creates a number input that reads and writes value, from lo to hi
// in steps of step. Its default width is 120 px at the Text size of 100 percent (see
// Width), it shows the decimals of step, and its value is centered in the field.
func NumberInput[T int | float64](value *Signal[T], lo, hi, step T) *NumberInputElement[T] {
	return &NumberInputElement[T]{value: value, lo: lo, hi: hi, step: step, precision: -1, align: Center}
}

func (*NumberInputElement[T]) isElement()           {}
func (n *NumberInputElement[T]) elementKey() string { return n.key }

func (n *NumberInputElement[T]) Key(k string) *NumberInputElement[T] { n.key = k; return n }

// Label names the field for assistive technology, such as "Quiet hours from".
func (n *NumberInputElement[T]) Label(s string) *NumberInputElement[T] { n.name = s; return n }

// LabelledBy names the field by the text of the elements with the given keys;
// see DivElement.LabelledBy.
func (n *NumberInputElement[T]) LabelledBy(keys ...string) *NumberInputElement[T] {
	n.labelledBy = keys
	return n
}

// Disabled shows the number but does not let it change.
func (n *NumberInputElement[T]) Disabled(d bool) *NumberInputElement[T] { n.disabled = d; return n }

// SpinButtons adds the buttons that step the value, placed as p says: SpinInline
// draws a decrease and an increase button after the field, and SpinStacked draws
// an up arrow above a down arrow at its right, as Windows' up-down control does.
// Their look is in NumberInputStyle.
func (n *NumberInputElement[T]) SpinButtons(p SpinPlacement) *NumberInputElement[T] {
	n.spin, n.placement = true, p
	return n
}

// Align sets where the value sits in the field: Center, the default, or Start and End
// for the left and the right, such as End for figures that line up in a column. The
// value keeps to the room between the spin buttons, and its caret and selection
// follow it.
func (n *NumberInputElement[T]) Align(a Alignment) *NumberInputElement[T] { n.align = a; return n }

// Precision sets the number of decimals a float is shown with, so 2 shows 1.50.
// The default is the number of decimals of the step.
func (n *NumberInputElement[T]) Precision(d int) *NumberInputElement[T] { n.precision = d; return n }

// The sizing methods give the field's frame a size. The default is numberInputWidth
// wide at the Text size of 100 percent; Width(Full) fills the container, and the
// spin buttons stay at the frame's edges. See Sizing in docs/styling.md.
func (n *NumberInputElement[T]) Width(l Length) *NumberInputElement[T] { n.size.setWidth(l); return n }
func (n *NumberInputElement[T]) Height(l Length) *NumberInputElement[T] {
	n.size.setHeight(l)
	return n
}
func (n *NumberInputElement[T]) MinWidth(l Length) *NumberInputElement[T] {
	n.size.setMinWidth(l)
	return n
}
func (n *NumberInputElement[T]) MaxWidth(l Length) *NumberInputElement[T] {
	n.size.setMaxWidth(l)
	return n
}
func (n *NumberInputElement[T]) Grow(f float32) *NumberInputElement[T] { n.size.setGrow(f); return n }

// Styles sets the look of the parts of the number input, for this instance only.
// Set fields win over the theme's NumberInput and the defaults; see
// NumberInputStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (n *NumberInputElement[T]) Styles(st NumberInputStyle) *NumberInputElement[T] {
	n.styles = n.styles.merge(st)
	return n
}

func (n *NumberInputElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		scale := float32(ctx.TextScale()) / 100
		st := resolveNumberInput(t, n.placement, scale, n.disabled, n.spin && !n.disabled, n.small, n.styles)
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
			old := n.value.Peek()
			n.value.Set(x)
			if n.onChange != nil && x != old {
				defer n.onChange(x)
			}
			draft.Set(n.format(x))
			seen.Set(x)
			invalid.Set(false)
		}
		// A form's Enter runs once the text is applied, and not when the text is not a
		// number. The field's own Enter is the one in the wire, so it is not run twice.
		w := n.wire
		enter := w.onEnter
		w.onEnter = nil
		submit := func(text string) {
			x, ok := n.parse(text)
			if !ok {
				invalid.Set(true)
				return
			}
			apply(x)
			if enter != nil {
				enter()
			}
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
			if w.onEdit != nil {
				w.onEdit()
			}
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

		name, labelledBy := n.name, n.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		// The field is its column: the frame and the message row, at the field's width. Its default
		// width is numberInputWidth at the Text size, and the sizing methods replace it.
		root := Div().Column().Gap(6).Role(access.RoleSpinner).Style(Style{Width: Px(numberInputWidth * scale)})
		if n.tooltip != "" && w.description == "" {
			root.Description(n.tooltip) // the spin box is the node assistive technology reads
		}
		a := root.access()
		if name != "" {
			root.Label(name)
		}
		if len(labelledBy) > 0 {
			root.LabelledBy(labelledBy...)
		}
		a.rng = &access.Range{Value: float64(v), Min: float64(n.lo), Max: float64(n.hi),
			SmallChange: float64(n.step), LargeChange: 10 * float64(n.step)}
		a.setRange = func(x float64) { apply(numberFrom[T](x)) }
		a.invalid = isInvalid || w.invalid
		a.required = w.required

		row := Div().Row().Align(Center).Gap(6)
		if n.disabled {
			a.disabled = true
			// The disabled number is drawn as text in a frame, with the disabled frame
			// and text parts. Under high contrast both are GrayText, with no dimming.
			frame := Div().asPart("NumberInput.Frame").Style(st.Frame).Style(st.FrameDisabled)
			content := Div().asPart("NumberInput.Text").Style(st.Text).Row().Grow(1).Justify(n.align).Child(Text(draft.Get()))
			root.Child(row.Child(frame.Child(content)))
			return n.size.apply(root)
		}

		fo := fieldOptions{tooltip: n.tooltip, value: draft, name: name, labelledBy: labelledBy, handle: n.focusHandle,
			onSubmit: submit, onChange: func(string) { invalid.Set(false) }, onBlur: blur, onStep: step,
			widget: "NumberInput", wire: w, align: n.align}
		// The invalid frame is drawn thicker and in the danger color (the text color under
		// high contrast) by fieldOptions.expand, and the message says what is wrong.
		fo.wire.invalid = isInvalid || w.invalid
		if isInvalid {
			fo.describedBy = []string{errKey}
		}
		// The frame as it is drawn: an invalid frame has its own border, and the buttons
		// take their corners from it.
		frame := st.Frame
		if fo.wire.invalid {
			frame = frame.Merge(st.FrameInvalid)
		}
		// Stacked buttons are the frame's trailing element, inside its right edge. Inline
		// buttons are laid over the frame's edges, after the text (see spinButtons).
		if n.spin && n.placement == SpinStacked {
			fo.trailing = n.stackedSpin(st, frame, v, step, scale)
		} else if n.spin {
			fo.overlays = n.inlineSpin(st, frame, v, step, scale)
		}
		row.Child(fo.expand(ctx, st.FieldStyle, false, 0))
		root.Child(row)
		// The message is always in the tree, after the row, and shown while the text is
		// invalid, so that the field keeps its place among its siblings.
		msg := fmt.Sprintf("Enter a number from %s to %s.", n.format(n.lo), n.format(n.hi))
		root.Child(Div().asPart("NumberInput.Message").Style(st.Message).Display(isInvalid).
			Child(Icon(icons.Alert).Size(14).Color(messageIconColor(t, st.Message))).
			Child(Text(msg).Key(errKey)))
		return n.size.apply(root)
	})
}

// stackedSpin returns the group of the stacked spin buttons: the increase button
// above the decrease button, inside the frame's right edge. Each button is a button
// for assistive technology, is in the Tab order while enabled, and steps the value
// by one step when it is clicked. A button is disabled at the end of the range it
// would step past.
func (n *NumberInputElement[T]) stackedSpin(st NumberInputStyle, frame Style, v T, step func(int), scale float32) *DivElement {
	gap := spinGap * scale
	down := n.spinButton(st, frame, -1, "Decrease", icons.ChevronDown, 12*scale, gap, v <= n.lo, step)
	up := n.spinButton(st, frame, 1, "Increase", icons.ChevronUp, 12*scale, gap, v >= n.hi, step)
	col := st.SpinButtons
	// The column fills the frame inside its border, so the field is as high as the
	// other fields: the frame's height is its outer height, border included. The
	// border is the one drawn, which is thicker on an invalid field.
	if col.Height.Unit == layout.UnitPx && frame.MinHeight.Unit == layout.UnitPx && col.Height.Value == frame.MinHeight.Value {
		col.Height = Px(frame.MinHeight.Value - 2*frame.BorderWidth)
	}
	return Div().asPart("NumberInput.SpinButtons").Style(col).Child(up).Child(down)
}

// inlineSpin returns the inline spin buttons: the decrease button at the frame's left
// edge and the increase button at its right, each laid over the edge, inside the
// border. They come after the text in the tree, so the Tab order is the text, then
// the decrease and the increase button.
func (n *NumberInputElement[T]) inlineSpin(st NumberInputStyle, frame Style, v T, step func(int), scale float32) []Element {
	gap := spinGap * scale
	down := n.spinButton(st, frame, -1, "Decrease", icons.Minus, 14*scale, gap, v <= n.lo, step)
	up := n.spinButton(st, frame, 1, "Increase", icons.Plus, 14*scale, gap, v >= n.hi, step)
	return []Element{down, up}
}

// spinButton returns one spin button, which steps the value by dir steps, or is
// disabled. The button is the fill of its hover and pressed states, and it is inset
// by gap from the frame's border on every side that meets it (see spinGap), so the
// fill never touches the border. Its corners are concentric with the frame's inner
// corners (see spinFillCorners). The button is the hit target, so the gap is not.
func (n *NumberInputElement[T]) spinButton(st NumberInputStyle, frame Style, dir int, label string, ic *icons.Icon,
	iconSize, gap float32, disabled bool, step func(int)) *DivElement {
	inline := n.placement != SpinStacked
	b := Div().asPart("NumberInput.SpinButton").Row().Align(Center).Justify(Center).
		Style(st.SpinButton).Child(Icon(ic).Size(iconSize)).
		Role(access.RoleButton).Label(label).
		Style(Style{Corners: spinFillCorners(frame, inline, dir, gap)})
	if inline {
		b.Absolute().Top(gap).Bottom(gap)
		if dir < 0 {
			b.Left(gap)
		} else {
			b.Right(gap)
		}
	}
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

// spinFillCorners returns the corners of a spin button's fill. The fill is inset by gap
// from the frame's inner edge, so its radius is the frame's inner radius on the
// button's side less the gap. Its corners are then concentric with the frame's inner
// corners, and never reach past the curve of the border, whatever the gap. The
// radius is the smaller of the two inner radii on the button's side, so it is safe on
// every corner the button meets. A stacked button is on the right, the increase
// button above and the decrease button below. An inline button is on the left (the
// decrease) or the right (the increase). Its other corners take the same radius.
func spinFillCorners(frame Style, inline bool, dir int, gap float32) Corners {
	ic := innerCorners(frame.radii(), frame.borderEdges())
	var r float32
	switch {
	case inline && dir < 0:
		r = min(ic.TopLeft, ic.BottomLeft)
	case inline:
		r = min(ic.TopRight, ic.BottomRight)
	case dir < 0:
		r = ic.BottomRight
	default:
		r = ic.TopRight
	}
	r = max(r-gap, 0)
	return Corners{TopLeft: r, TopRight: r, BottomRight: r, BottomLeft: r}
}

// clamp returns x within the range.
func (n *NumberInputElement[T]) clamp(x T) T { return min(max(x, n.lo), n.hi) }

// stepped returns the next grid value from x in direction dir (+1 or -1), where
// the grid starts at lo and is spaced by step. A value between two grid values
// moves to the nearer one in that direction, so 5.5 steps up to 6.
func (n *NumberInputElement[T]) stepped(x T, dir int) T {
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
func (n *NumberInputElement[T]) parse(text string) (T, bool) {
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
func (n *NumberInputElement[T]) format(x T) string {
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
