package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// The sizes of a color picker's parts at the Text size of 100 percent, in logical
// pixels. The panel is colorPickerPanelWidth wide with its padding, so its area is
// 200 px wide. They scale with the Text size, as the default sizes of the other
// controls do (see textScale).
const (
	colorPickerPanelWidth = 224
	colorPickerAreaHeight = 150
	colorPickerSwatch     = 18
	colorPickerPreset     = 20
	colorPickerThumb      = 14
)

// ColorPickerStyle is the look of each part of a color picker. A part's style merges
// over the built-in defaults, then over the theme's ColorPicker, then over the Styles
// of the instance; see docs/styling.md.
//
// Trigger is the button that shows the color: a frame like a text field's, with
// TriggerHover while the pointer is over it and TriggerFocusVisible while it has
// keyboard focus. Swatch is the box in the trigger that is drawn in the color; the
// color is its content, so only its other fields apply. Popup is the panel, Area the
// saturation and brightness area and AreaFocusVisible the area while it has focus.
// TriggerDisabled is the trigger of a disabled picker. Thumb is the knob at the color
// in the area: a fixed circle, drawn in the color with its border and shadow. Slider is the hue and opacity sliders, whose parts are those
// of a Slider (see SliderStyle). Preset is each swatch of Swatches, and
// PresetFocusVisible a swatch while it has keyboard focus.
type ColorPickerStyle struct {
	Trigger             Style       // the button that shows the color
	TriggerHover        Style       // the trigger while the pointer is over it
	TriggerFocusVisible Style       // the trigger while it has keyboard focus
	TriggerDisabled     Style       // the trigger of a disabled picker
	Swatch              Style       // the box in the trigger, drawn in the color
	Popup               Style       // the panel
	Area                Style       // the saturation and brightness area
	AreaFocusVisible    Style       // the area while it has keyboard focus
	Thumb               Style       // the knob at the color in the area
	Slider              SliderStyle // the hue and opacity sliders
	Preset              Style       // each preset swatch
	PresetFocusVisible  Style       // a preset swatch while it has keyboard focus
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ColorPickerStyle) merge(o ColorPickerStyle) ColorPickerStyle {
	return ColorPickerStyle{
		Trigger:             s.Trigger.Merge(o.Trigger),
		TriggerHover:        s.TriggerHover.Merge(o.TriggerHover),
		TriggerFocusVisible: s.TriggerFocusVisible.Merge(o.TriggerFocusVisible),
		TriggerDisabled:     s.TriggerDisabled.Merge(o.TriggerDisabled),
		Swatch:              s.Swatch.Merge(o.Swatch),
		Popup:               s.Popup.Merge(o.Popup),
		Area:                s.Area.Merge(o.Area),
		AreaFocusVisible:    s.AreaFocusVisible.Merge(o.AreaFocusVisible),
		Thumb:               s.Thumb.Merge(o.Thumb),
		Slider:              s.Slider.merge(o.Slider),
		Preset:              s.Preset.Merge(o.Preset),
		PresetFocusVisible:  s.PresetFocusVisible.Merge(o.PresetFocusVisible),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ColorPickerStyle) plain() ColorPickerStyle {
	return ColorPickerStyle{
		Trigger:             plainStyle(s.Trigger),
		TriggerHover:        plainStyle(s.TriggerHover),
		TriggerFocusVisible: plainStyle(s.TriggerFocusVisible),
		TriggerDisabled:     plainStyle(s.TriggerDisabled),
		Swatch:              plainStyle(s.Swatch),
		Popup:               plainStyle(s.Popup),
		Area:                plainStyle(s.Area),
		AreaFocusVisible:    plainStyle(s.AreaFocusVisible),
		Thumb:               plainStyle(s.Thumb),
		Slider:              s.Slider.plain(),
		Preset:              plainStyle(s.Preset),
		PresetFocusVisible:  plainStyle(s.PresetFocusVisible),
	}
}

// colorPickerDefaults returns the built-in look of a color picker, from the colors of
// t. Its sizes are not in it: colorPickerSizes gives them, under the theme.
func colorPickerDefaults(t *Theme) ColorPickerStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := ColorPickerStyle{
		Trigger: Style{Direction: Row, Align: Center, Gap: 8, Radius: t.Radius, Background: t.Surface,
			BorderWidth: 1, BorderColor: t.BorderStrong, TextColor: t.Text, Cursor: CursorPointer,
			Transition: t.Motion.hover(), Spring: t.Motion.Spring},
		TriggerHover:        Style{BorderColor: t.TextMuted},
		TriggerFocusVisible: Style{Shadows: []Shadow{ring}},
		Swatch:              Style{NoShrink: true, Radius: 4, BorderWidth: 1, BorderColor: t.BorderStrong},
		Popup: Style{Direction: Column, Gap: 10, Padding: All(12), Background: t.Surface,
			BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}},
		Area:               Style{Radius: 6, OverflowX: OverflowHidden, OverflowY: OverflowHidden, BorderWidth: 1, BorderColor: t.Border},
		AreaFocusVisible:   Style{Shadows: []Shadow{ring}},
		Thumb:              Style{NoShrink: true, BorderWidth: 2, BorderColor: Hex("#ffffff"), Shadows: []Shadow{ShadowSm}},
		Preset:             Style{NoShrink: true, Radius: 4, BorderWidth: 1, BorderColor: t.Border},
		PresetFocusVisible: Style{Shadows: []Shadow{ring}},
	}
	if t.HighContrast {
		// The swatch, the area, the thumb and the presets are outlined in the text color,
		// and nothing casts a shadow. The colors they show are their content, so they stay.
		d.Swatch.BorderColor = t.Text
		d.Area.BorderColor, d.Preset.BorderColor = t.Text, t.Text
		d.Thumb.BorderColor, d.Thumb.Shadows = t.Text, nil
		d.Popup.Shadows = nil
		// A disabled trigger is drawn in the disabled color, as a disabled select is.
		d.TriggerDisabled = Style{TextColor: t.Disabled, BorderColor: t.Disabled}
	} else {
		d.TriggerDisabled = Style{Opacity: 0.5}
	}
	return d
}

// colorPickerSizes returns the sizes of the parts at the control size s and the Text
// size scale. They are the lowest layer, under the defaults, the theme and the
// instance, as the sizes of a text field's frame are.
func colorPickerSizes(s controlSize, scale float32) ColorPickerStyle {
	h, px, fs := s.field()
	swatch := Px(colorPickerSwatch * scale)
	preset := Px(colorPickerPreset * scale)
	return ColorPickerStyle{
		Trigger: Style{MinHeight: Px(h), Padding: XY(px, 0), FontSize: fs},
		Swatch:  Style{Width: swatch, Height: swatch},
		Popup:   Style{Width: Px(colorPickerPanelWidth * scale)},
		Area:    Style{Height: Px(colorPickerAreaHeight * scale)},
		Preset:  Style{Width: preset, Height: preset},
	}
}

// resolveColorPicker returns the look of a color picker: the defaults, merged with the
// theme's ColorPicker and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveColorPicker(ctx *Context, inst ColorPickerStyle) ColorPickerStyle {
	t := ctx.Theme()
	return layered(ctx, colorPickerDefaults(t), ColorPickerStyle.merge, ColorPickerStyle.plain,
		themeLayer(t.ColorPicker), instanceLayer(inst))
}

// ColorPickerElement is a button that shows a color and opens a panel to change it. The
// panel holds a saturation and brightness area, a hue slider, an opacity slider when
// Alpha is set, a hex text field and the preset swatches of Swatches. It opens and
// closes as a Popover does: a click, Enter or Space on the trigger toggles it, and
// Escape or a click outside closes it, with focus back on the trigger. The area takes
// focus when the panel opens. Its arrow keys move the saturation (Left and Right) and
// the brightness (Up and Down) by 1 percent, and by 10 percent with Shift; the arrows
// and Page Up and Page Down move the hue.
//
// The value is a Color. Its hue, saturation and value are kept while the user changes
// them, so a grey color keeps the hue the user chose. A value set from outside is read
// again as its hue, saturation and value.
type ColorPickerElement struct {
	value    *Signal[Color]
	alpha    bool        // Alpha was called: the opacity slider is shown
	swatches []Color     // the preset colors, see Swatches
	onChange func(Color) // see OnChange
	name     string      // the accessible name of the trigger, see Label
	key      string      // see Key
	ctlSize  controlSize // see Small and Large
	size     sizing      // see Width
	styles   ColorPickerStyle

	labelledBy  []string     // see LabelledBy
	disabled    bool         // see Disabled
	focusHandle *FocusHandle // see FocusHandle
	tooltip     string       // see Tooltip
}

// ColorPicker creates a color picker that reads and writes value. The trigger shows the
// color and its hex text, such as "#3B82F6".
func ColorPicker(value *Signal[Color]) *ColorPickerElement {
	return &ColorPickerElement{value: value}
}

// Tooltip shows s below the trigger after the pointer rests on it. Assistive
// technology reads it as the trigger's description, after the hex text.
func (c *ColorPickerElement) Tooltip(s string) *ColorPickerElement {
	c.tooltip = s
	return c
}

// FocusHandle lets code focus the trigger (see NewFocusHandle).
func (c *ColorPickerElement) FocusHandle(h *FocusHandle) *ColorPickerElement {
	c.focusHandle = h
	return c
}

// LabelledBy names the trigger by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (c *ColorPickerElement) LabelledBy(keys ...string) *ColorPickerElement {
	c.labelledBy = keys
	return c
}

// Disabled shows the trigger, with its color, and does not let the panel open.
func (c *ColorPickerElement) Disabled(on bool) *ColorPickerElement {
	c.disabled = on
	return c
}

func (*ColorPickerElement) isElement()           {}
func (c *ColorPickerElement) elementKey() string { return c.key }

// Key identifies the color picker among its siblings, so it keeps its state when the
// siblings around it change (see DivElement.Key).
func (c *ColorPickerElement) Key(k string) *ColorPickerElement { c.key = k; return c }

// Label names the trigger for assistive technology. It defaults to "Color".
func (c *ColorPickerElement) Label(name string) *ColorPickerElement { c.name = name; return c }

// Alpha adds an opacity slider to the panel, and makes the hex text eight digits long,
// #RRGGBBAA. Without it the alpha of the color is kept as it is.
func (c *ColorPickerElement) Alpha() *ColorPickerElement { c.alpha = true; return c }

// Swatches sets the preset colors of the panel. Each is a button named by its hex value,
// and choosing one sets the value to it.
func (c *ColorPickerElement) Swatches(colors ...Color) *ColorPickerElement {
	c.swatches = colors
	return c
}

// OnChange runs fn with the new color when the user changes it. It does not run when the
// value is set from outside. The signal is set first.
func (c *ColorPickerElement) OnChange(fn func(Color)) *ColorPickerElement {
	c.onChange = fn
	return c
}

// Small makes the trigger compact: 28 px high with smaller text, like a small text field.
// Small and Large replace each other.
func (c *ColorPickerElement) Small() *ColorPickerElement {
	c.ctlSize = sizeSmall
	return c
}

// Large makes the trigger larger: 40 px high with 16 px text, like a large text field.
// Small and Large replace each other.
func (c *ColorPickerElement) Large() *ColorPickerElement {
	c.ctlSize = sizeLarge
	return c
}

// The sizing methods give the trigger's outer box a size, as they do for the other
// widgets. See Sizing in docs/styling.md.
func (c *ColorPickerElement) Width(l Length) *ColorPickerElement { c.size.setWidth(l); return c }

// MinWidth sets the smallest width of the color picker.
func (c *ColorPickerElement) MinWidth(l Length) *ColorPickerElement { c.size.setMinWidth(l); return c }

// MaxWidth sets the largest width of the color picker.
func (c *ColorPickerElement) MaxWidth(l Length) *ColorPickerElement { c.size.setMaxWidth(l); return c }

// Grow makes the color picker take a share of the free space on its parent's main axis.
func (c *ColorPickerElement) Grow(f float32) *ColorPickerElement { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the color picker covers in its parent's grid.
func (c *ColorPickerElement) GridSpan(cols, rows int) *ColorPickerElement {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the color picker, for this instance only. Set
// fields win over the theme's ColorPicker and the defaults; see ColorPickerStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (c *ColorPickerElement) Styles(st ColorPickerStyle) *ColorPickerElement {
	c.styles = c.styles.merge(st)
	return c
}

// partKey returns the key of one of the picker's own elements, scoped by the picker's
// key, or "" when the picker has no key.
func (c *ColorPickerElement) partKey(name string) string {
	if c.key == "" {
		return ""
	}
	return c.key + "/" + name
}

// pickerHSV is a color in hue, saturation and value: the hue in degrees from 0 to 360,
// and the saturation and the value from 0 to 1.
type pickerHSV struct{ h, s, v float64 }

// rgbToPicker returns the hue, saturation and value of c.
func rgbToPicker(c Color) pickerHSV {
	h, s, v := rgbToHSV(float64(c.R), float64(c.G), float64(c.B))
	return pickerHSV{h, s, v}
}

// color returns the color of hs at alpha a.
func (hs pickerHSV) color(a float32) Color {
	r, g, b := hsvToRGB(hs.h, hs.s, hs.v)
	return Color{float32(r), float32(g), float32(b), a}
}

// hsvToRGB returns the red, green and blue of hue h (in degrees, any value), saturation
// s and value v (from 0 to 1), each from 0 to 1.
func hsvToRGB(h, s, v float64) (r, g, b float64) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}

// rgbToHSV returns the hue in degrees from 0 to 360, the saturation and the value of the
// red, green and blue, each from 0 to 1. A grey has hue 0.
func rgbToHSV(r, g, b float64) (h, s, v float64) {
	hi, lo := max(r, g, b), min(r, g, b)
	v = hi
	d := hi - lo
	if hi > 0 {
		s = d / hi
	}
	switch {
	case d == 0:
	case hi == r:
		h = 60 * (g - b) / d
	case hi == g:
		h = 60 * ((b-r)/d + 2)
	default:
		h = 60 * ((r-g)/d + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// parseHex returns the color of the hex text s, as #rgb or #rrggbb, and as #rrggbbaa
// when alpha is set. It reports false for any other text.
func parseHex(s string, alpha bool) (Color, bool) {
	if len(strings.TrimPrefix(s, "#")) == 8 && !alpha {
		return Color{}, false
	}
	c, err := ParseHex(s)
	return c, err == nil
}

// formatHex returns c as hex text: #RRGGBB, and #RRGGBBAA when alpha is set and
// the color is not opaque.
func formatHex(c Color, alpha bool) string {
	ch := func(x float32) int { return int(math.Round(float64(min(max(x, 0), 1)) * 255)) }
	s := fmt.Sprintf("#%02X%02X%02X", ch(c.R), ch(c.G), ch(c.B))
	// The alpha is written only when the color is not opaque: #3B82F6 rather
	// than #3B82F6FF.
	if alpha && ch(c.A) < 255 {
		s += fmt.Sprintf("%02X", ch(c.A))
	}
	return s
}

// clampUnit returns x clamped to the range 0 to 1.
func clampUnit(x float64) float64 { return min(max(x, 0), 1) }

// thumbFraction returns how far pos is along a track of length px whose thumb is thumb
// px wide. The thumb's center travels from thumb/2 to length-thumb/2, as a Slider's does,
// so the fraction is 0 at the first center and 1 at the last.
func thumbFraction(pos, length, thumb float64) float64 {
	inner := length - thumb
	if inner <= 0 {
		return 0
	}
	return clampUnit((pos - thumb/2) / inner)
}

func (c *ColorPickerElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		rt, t := ctx.rt, ctx.Theme()
		s := ctx.Strings()
		scale := textScale(ctx)
		st := colorPickerSizes(c.ctlSize, scale).merge(resolveColorPicker(ctx, c.styles))
		sl := resolveSlider(ctx, false, st.Slider)
		cur := c.value.Get()
		// hs is the color in hue, saturation and value, kept between changes so that the hue
		// holds while the saturation or the value is 0. seen is the color the state was made
		// from, so that a color set from outside replaces the state, and the others keep it.
		hs := Local(ctx, "hsv", rgbToPicker(cur))
		seen := Local(ctx, "seen", cur)
		alphaPct := Local(ctx, "alpha", float64(cur.A)*100)
		draft := Local(ctx, "draft", formatHex(cur, c.alpha))
		open := Local(ctx, "open", false)
		// entering is set while the panel opens, so its first focus moves on to the area.
		entering := Local(ctx, "entering", false)
		areaDrag := Local(ctx, "area", false)
		hueDrag := Local(ctx, "hue", false)
		panelH := Local(ctx, "panel", NewFocusHandle()).Peek()
		// back is where focus returns when Escape closes the panel: the trigger.
		back := Local(ctx, "back", (*FocusHandle)(nil))
		if cur != seen.Peek() {
			seen.Set(cur)
			hs.Set(rgbToPicker(cur))
			alphaPct.Set(float64(cur.A) * 100)
			draft.Set(formatHex(cur, c.alpha))
		}
		now := hs.Get()

		// colorOf returns the color of next at the opacity the panel holds.
		colorOf := func(next pickerHSV) Color { return next.color(float32(alphaPct.Peek() / 100)) }
		// apply makes nc the color, in the state next. The app hears of it only when the
		// color changes, so that a drag over the same color is silent.
		apply := func(nc Color, next pickerHSV) {
			hs.Set(next)
			seen.Set(nc)
			alphaPct.Set(float64(nc.A) * 100)
			draft.Set(formatHex(nc, c.alpha))
			if nc != c.value.Peek() {
				c.value.Set(nc)
				if c.onChange != nil {
					c.onChange(nc)
				}
			}
		}
		setSB := func(s, v float64) {
			next := pickerHSV{hs.Peek().h, clampUnit(s), clampUnit(v)}
			apply(colorOf(next), next)
		}
		setHue := func(h float64) {
			next := hs.Peek()
			next.h = math.Mod(math.Mod(h, 360)+360, 360)
			apply(colorOf(next), next)
		}
		// commitHex applies the hex text as the color. Text that is not a color is refused,
		// and the field shows the color again.
		commitHex := func() {
			nc, ok := parseHex(strings.TrimSpace(draft.Peek()), c.alpha)
			if !ok {
				draft.Set(formatHex(c.value.Peek(), c.alpha))
				return
			}
			if !c.alpha {
				nc.A = c.value.Peek().A
			}
			apply(nc, rgbToPicker(nc))
		}

		// set opens or closes the panel. Opening takes the color's hex text afresh, so that
		// a text left unapplied does not show when the panel opens again.
		set := func(on bool) {
			if open.Peek() == on {
				return
			}
			if on {
				back.Set(rt.FocusedHandle())
				draft.Set(formatHex(c.value.Peek(), c.alpha))
			}
			open.Set(on)
			entering.Set(on)
		}
		// closeFn closes the panel. Focus inside it returns to the trigger; after a click
		// elsewhere, focus is already where the user put it.
		closeFn := func() {
			if b := back.Peek(); b != nil && panelH.id != 0 && rt.focusInside(panelH.id) {
				b.FocusNow()
			}
			set(false)
		}
		toggle := func() {
			if open.Peek() {
				closeFn()
			} else {
				set(true)
			}
		}

		name := c.name
		if name == "" {
			name = s.Color
		}
		hex := formatHex(cur, c.alpha)
		trig := Div().asPart("ColorPicker.Trigger").Style(st.Trigger).Tooltip(c.tooltip).
			Child(Div().asPart("ColorPicker.Swatch").Style(st.Swatch).Background(cur)).
			Child(Text(hex).NoWrap())
		// The hex text is the trigger's description; a tooltip says more after it.
		trig.Role(access.RoleButton).Label(name).Description(describe(hex, c.tooltip)).AccessExpanded(open.Get())
		if len(c.labelledBy) > 0 {
			trig.LabelledBy(c.labelledBy...)
		}
		if c.disabled {
			// A disabled trigger takes no focus and no clicks, and a panel that was open
			// closes.
			trig.access().disabled = true
			trig.Style(st.TriggerDisabled).Cursor(CursorNotAllowed)
			if open.Peek() {
				open.Set(false)
			}
		} else {
			trig.Hover(st.TriggerHover).Focusable().FocusVisible(st.TriggerFocusVisible).OnClick(toggle)
			if c.focusHandle != nil {
				trig.FocusHandle(c.focusHandle)
			}
		}
		// The sizes of the trigger's box are those of the root, which the trigger fills, so
		// that a Grow or a Width set on the picker reaches the box the panel is placed by.
		root := Div().Row().AlignSelf(Start).Shrink(0).Child(trig.Grow(1))
		if c.key != "" {
			root.Key(c.key)
		}
		c.size.apply(root)

		if open.Get() && !c.disabled {
			panel := Div().asPart("ColorPicker.Popup").Style(st.Popup).Column().
				Overlay(BelowStart).OverlayGap(6).
				Role(access.RoleDialog).Label(name).
				// The panel takes focus when it opens, and hands it to its first control; a click
				// on its background keeps focus inside it.
				Focusable().TabStop(false).AutoFocus().FocusHandle(panelH).
				OnFocus(func() {
					if entering.Peek() {
						entering.Set(false)
						rt.focusFirstIn(panelH.id)
					}
				}).
				OnClickOutside(closeFn).
				Enter(Fade().Slide(0, -4), Animation{Duration: t.Motion.enter(120 * time.Millisecond)}).
				Exit(Fade(), Animation{Duration: t.Motion.exit(80 * time.Millisecond)})

			// The area: the hue at full saturation and value, a white to the hue layer from
			// the left, and a transparent to black layer from the top. Under high contrast the
			// gradients are drawn solid, so the area shows the hue alone, and the thumb marks
			// the saturation and the value.
			th := float32(colorPickerThumb) * scale
			area := Div().asPart("ColorPicker.Area").Style(st.Area).Cursor(CursorPointer).
				Focusable().FocusVisible(st.AreaFocusVisible).Key(c.partKey("area")).
				OnMouseDown(func(e *MouseEvent) {
					if e.Button == input.ButtonLeft {
						areaDrag.Set(true)
						setSB(thumbFraction(float64(e.LocalX), float64(e.Width), float64(th)),
							1-thumbFraction(float64(e.LocalY), float64(e.Height), float64(th)))
					}
				}).
				OnMouseMove(func(e *MouseEvent) {
					if areaDrag.Peek() {
						setSB(thumbFraction(float64(e.LocalX), float64(e.Width), float64(th)),
							1-thumbFraction(float64(e.LocalY), float64(e.Height), float64(th)))
					}
				}).
				OnMouseUp(func(*MouseEvent) { areaDrag.Set(false) }).
				CapturePointer().
				OnKeyDown(func(e *KeyEvent) {
					if e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
						return
					}
					step := 0.01
					if e.Mods&input.ModShift != 0 {
						step = 0.1
					}
					s, v := hs.Peek().s, hs.Peek().v
					switch e.Key {
					case input.KeyLeft:
						s -= step
					case input.KeyRight:
						s += step
					case input.KeyUp:
						v += step
					case input.KeyDown:
						v -= step
					default:
						return
					}
					e.StopPropagation()
					setSB(s, v)
				})
			hueColor := pickerHSV{now.h, 1, 1}.color(1)
			area.Child(Div().Absolute().Inset(0).Background(hueColor))
			if !t.HighContrast {
				area.Child(Div().Absolute().Inset(0).BackgroundGradient(LinearGradient(90, Hex("#ffffff"), hueColor)))
				area.Child(Div().Absolute().Inset(0).BackgroundGradient(LinearGradient(180, Color{}, Hex("#000000"))))
			}
			// The thumb sits where the saturation and the value put it: the spacers above and
			// below share the room the thumb does not take, as the Slider's do.
			fx, fy := float32(now.s), float32(1-now.v)
			thumb := Div().asPart("ColorPicker.Thumb").Style(st.Thumb).Background(cur).
				Size(Px(th), Px(th)).Radius(th / 2).Shrink(0)
			area.Child(Div().Absolute().Inset(0).Column().
				Child(Div().Grow(fy)).
				Child(Div().Row().Height(Px(th)).Child(Div().Grow(fx)).Child(thumb).Child(Div().Grow(1 - fx))).
				Child(Div().Grow(1 - fy)))
			ra := area.access()
			area.Role(access.RoleSlider).Label(s.SaturationBrightness)
			ra.value = s.SaturationBrightnessValue(int(math.Round(now.s*100)), int(math.Round(now.v*100)))
			panel.Child(area)

			// The hue slider: the Slider's track and thumb over a rail of six gradient
			// segments, one for each sixth of the hue circle. Its keys are the Slider's, with
			// the arrows by 1 degree and Page Up and Page Down by 10.
			f := float32(now.h / 360)
			rail := Div().asPart("Slider.Rail").Row().Style(sl.Rail).Grow(1)
			for i := range 6 {
				from := pickerHSV{float64(i) * 60, 1, 1}.color(1)
				to := pickerHSV{float64(i+1) * 60, 1, 1}.color(1)
				rail.Child(Div().Height(Full).Grow(1).BackgroundGradient(LinearGradient(90, from, to)))
			}
			hueThumb := Div().asPart("Slider.Thumb").Style(sl.Thumb).Size(Px(sliderThumb), Px(sliderThumb)).Shrink(0)
			if hueDrag.Get() {
				hueThumb.Style(sl.ThumbPressed)
			} else {
				hueThumb.Hover(sl.ThumbHover)
			}
			hueTrack := Div().asPart("Slider.Track").Height(Px(22)).Grow(1).Shrink(1).Style(sl.Track).
				Child(Div().Row().Height(Px(22)).Align(Center).PaddingX(sliderThumb / 2).Child(rail)).
				Child(Div().Absolute().Inset(0).Row().Align(Center).
					Child(Div().Grow(f)).Child(hueThumb).Child(Div().Grow(1 - f))).
				Key(c.partKey("hue")).Cursor(CursorPointer).Focusable().FocusVisible(sl.TrackFocusVisible).
				OnMouseDown(func(e *MouseEvent) {
					if e.Button == input.ButtonLeft {
						hueDrag.Set(true)
						setHue(thumbFraction(float64(e.LocalX), float64(e.Width), sliderThumb) * 360)
					}
				}).
				OnMouseMove(func(e *MouseEvent) {
					if hueDrag.Peek() {
						setHue(thumbFraction(float64(e.LocalX), float64(e.Width), sliderThumb) * 360)
					}
				}).
				OnMouseUp(func(*MouseEvent) { hueDrag.Set(false) }).
				CapturePointer().
				OnKeyDown(func(e *KeyEvent) {
					if e.Mods != 0 {
						return
					}
					var d float64
					switch e.Key {
					case input.KeyLeft, input.KeyDown:
						d = -1
					case input.KeyRight, input.KeyUp:
						d = 1
					case input.KeyPageDown:
						d = -10
					case input.KeyPageUp:
						d = 10
					default:
						return
					}
					e.StopPropagation()
					setHue(hs.Peek().h + d)
				})
			hueTrack.Role(access.RoleSlider).Label(s.Hue)
			ha := hueTrack.access()
			ha.rng = &access.Range{Value: now.h, Min: 0, Max: 360, SmallChange: 1, LargeChange: 10}
			ha.setRange = setHue
			hueRow := Div().Row().Align(Center).Gap(12).Width(Full).Style(Style{MinWidth: Px(sliderMinWidth)}).
				Child(hueTrack).
				Child(Div().asPart("Slider.ValueText").AccessHidden().MinWidth(Px(44)).Align(End).Style(sl.ValueText).
					Child(Text(fmt.Sprintf("%.0f", now.h))))
			panel.Child(hueRow)

			// The opacity slider, when Alpha is set. Its rail runs from the color, clear, to the
			// color, opaque; a gradient is drawn solid under high contrast, so the rail is left.
			if c.alpha {
				ops := SliderStyle{}
				if !t.HighContrast {
					ops.Rail = Style{Gradient: LinearGradient(90, now.color(0), now.color(1))}
				}
				opacity := Slider(alphaPct, 0, 100, 1).Label(s.Opacity).Key(c.partKey("opacity")).
					Format(func(v float64) string { return fmt.Sprintf("%.0f%%", v) }).
					OnChange(func(float64) {
						next := hs.Peek()
						apply(colorOf(next), next)
					}).
					Styles(ops.merge(st.Slider))
				panel.Child(opacity)
			}

			placeholder := "#RRGGBB"
			if c.alpha {
				placeholder = "#RRGGBBAA"
			}
			hexField := TextInput(draft).Label(s.Hex).Placeholder(placeholder).Width(Full).Key(c.partKey("hex"))
			hexField.OnSubmit(func(string) { commitHex() })
			hexField.onBlur = func() {
				if draft.Peek() != formatHex(c.value.Peek(), c.alpha) {
					commitHex()
				}
			}
			panel.Child(hexField)

			if len(c.swatches) > 0 {
				presets := Div().Row().Wrap().Gap(6)
				for i, p := range c.swatches {
					preset := Div().asPart("ColorPicker.Preset").Style(st.Preset).Background(p).
						Focusable().FocusVisible(st.PresetFocusVisible).Cursor(CursorPointer).
						Key(c.partKey(fmt.Sprintf("preset:%d", i))).
						OnClick(func() { apply(p, rgbToPicker(p)) })
					preset.Role(access.RoleButton).Label(formatHex(p, c.alpha))
					presets.Child(preset)
				}
				panel.Child(presets)
			}
			root.Child(panel)
		}
		return root
	})
}
