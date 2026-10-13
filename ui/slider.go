package ui

import (
	"math"
	"strconv"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// sliderThumb is the width of the thumb, in logical pixels. The thumb's center
// travels from sliderThumb/2 to the track's width less sliderThumb/2.
const sliderThumb = 16

// SliderStyle is the look of each part of a slider. A part's style merges over
// the built-in defaults, then over the theme's Slider, then over the Styles of
// the instance; see docs/styling.md.
//
// Track is the control itself: it takes focus and the pointer, and the rail and
// thumb are drawn over it. Rail is the line the value is drawn on, Fill is the
// part of the rail up to the value (its width comes from the value, so Grow is
// not set by the style), and Thumb is the knob, always sliderThumb pixels wide
// so that the ends of the track stay where the value puts them. The thumb's
// states are ThumbHover, while the pointer is over an enabled slider's thumb,
// and ThumbPressed, while it is dragged.
type SliderStyle struct {
	Track             Style // the control: it takes focus and the pointer
	TrackFocusVisible Style // the control while it has keyboard focus
	Rail              Style // the line the value is drawn on
	Fill              Style // the part of the rail up to the value
	Thumb             Style // the knob at the value
	ThumbHover        Style // the knob while the pointer is over it
	ThumbPressed      Style // the knob while it is dragged
	ValueText         Style // the readout beside the track
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SliderStyle) merge(o SliderStyle) SliderStyle {
	return SliderStyle{
		Track:             s.Track.Merge(o.Track),
		TrackFocusVisible: s.TrackFocusVisible.Merge(o.TrackFocusVisible),
		Rail:              s.Rail.Merge(o.Rail),
		Fill:              s.Fill.Merge(o.Fill),
		Thumb:             s.Thumb.Merge(o.Thumb),
		ThumbHover:        s.ThumbHover.Merge(o.ThumbHover),
		ThumbPressed:      s.ThumbPressed.Merge(o.ThumbPressed),
		ValueText:         s.ValueText.Merge(o.ValueText),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SliderStyle) plain() SliderStyle {
	return SliderStyle{
		Track:             plainStyle(s.Track),
		TrackFocusVisible: plainStyle(s.TrackFocusVisible),
		Rail:              plainStyle(s.Rail),
		Fill:              plainStyle(s.Fill),
		Thumb:             plainStyle(s.Thumb),
		ThumbHover:        plainStyle(s.ThumbHover),
		ThumbPressed:      plainStyle(s.ThumbPressed),
		ValueText:         plainStyle(s.ValueText),
	}
}

// sliderDefaults returns the built-in look of a slider, from the colors of t.
// Under high contrast the parts are outlined and filled with the text color, or
// with Highlight for the fill; a disabled slider is all GrayText, with no dimming.
func sliderDefaults(t *Theme, disabled bool) SliderStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	fill, edge, readout := t.Accent, t.Text, t.TextMuted
	thumbFill, thumbEdge := t.Accent, t.Surface
	if t.HighContrast {
		thumbFill, thumbEdge = t.Text, t.Background
	}
	if disabled && t.HighContrast {
		fill, edge, readout, thumbFill = t.Disabled, t.Disabled, t.Disabled, t.Disabled
	}
	d := SliderStyle{
		Track:             Style{Height: Px(22), Radius: 11},
		TrackFocusVisible: Style{Shadows: []Shadow{ring}},
		Rail:              Style{Height: Px(4), Radius: 2, OverflowX: OverflowHidden, OverflowY: OverflowHidden, Background: t.SurfaceActive},
		Fill:              Style{Background: fill},
		Thumb: Style{Radius: sliderThumb / 2, NoShrink: true, Background: thumbFill,
			BorderWidth: 2, BorderColor: thumbEdge, Shadows: []Shadow{ShadowSm}},
		ThumbHover:   Style{Background: t.AccentHover},
		ThumbPressed: Style{Background: t.AccentActive},
		ValueText:    Style{TextColor: readout, FontSize: 13, FontWeight: FontMedium},
	}
	if t.HighContrast {
		// The rail is outlined rather than tinted, and nothing casts a shadow.
		d.Rail = Style{Height: Px(4), Radius: 2, OverflowX: OverflowHidden, OverflowY: OverflowHidden,
			Background: t.Surface, BorderWidth: 1, BorderColor: edge}
		d.Thumb.Shadows = nil
		d.ThumbHover, d.ThumbPressed = Style{}, Style{}
	}
	return d
}

// resolveSlider returns the look of a slider: the defaults, merged with the
// theme's Slider and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveSlider(t *Theme, disabled bool, inst SliderStyle) SliderStyle {
	st := sliderDefaults(t, disabled)
	for _, o := range []SliderStyle{t.Slider, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// sliderMinWidth is the narrowest a slider is by default, in logical pixels.
// Like the minimum of a progress bar, it does not scale with the text.
const sliderMinWidth = 160

// SliderElement moves a value along a track from lo to hi, in steps of step. The
// mouse sets the value by a click or a drag. Keys: the arrows move it by one
// step, Page Up and Page Down by a tenth of the range, and Home and End go to
// the ends.
type SliderElement struct {
	value        *Signal[float64]
	lo, hi, step float64
	name         string // the accessible name, see Label
	labelledBy   []string
	key          string
	disabled     bool
	size         sizing
	format       func(float64) string
	onChange     func(float64)
	styles       SliderStyle
	focusHandle  *FocusHandle // see FocusHandle
	tooltip      string       // see Tooltip
}

// Tooltip shows s below the slider after the pointer rests on it. Assistive
// technology reads it as the slider's description, unless it has another.
func (s *SliderElement) Tooltip(text string) *SliderElement {
	s.tooltip = text
	return s
}

// FocusHandle lets code focus the slider (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (s *SliderElement) FocusHandle(h *FocusHandle) *SliderElement {
	s.focusHandle = h
	return s
}

// Slider creates a slider that reads and writes value, from lo to hi in steps of
// step. It fills the width of its container by default (see Width).
func Slider(value *Signal[float64], lo, hi, step float64) *SliderElement {
	return &SliderElement{value: value, lo: lo, hi: hi, step: step}
}

func (*SliderElement) isElement()           {}
func (s *SliderElement) elementKey() string { return s.key }

func (s *SliderElement) Key(k string) *SliderElement { s.key = k; return s }

// Label names the slider for assistive technology, such as "Volume".
func (s *SliderElement) Label(name string) *SliderElement { s.name = name; return s }

// LabelledBy names the slider by the text of the elements with the given keys;
// see DivElement.LabelledBy.
func (s *SliderElement) LabelledBy(keys ...string) *SliderElement {
	s.labelledBy = keys
	return s
}

// Disabled shows the slider but does not let it change.
func (s *SliderElement) Disabled(d bool) *SliderElement { s.disabled = d; return s }

// The sizing methods give the slider's outer box a size, including its readout. A
// slider fills its container by default, so Width(Px(240)) fixes it and
// MinWidth keeps its track usable. See Sizing in docs/styling.md.
func (s *SliderElement) Width(l Length) *SliderElement          { s.size.setWidth(l); return s }
func (s *SliderElement) MinWidth(l Length) *SliderElement       { s.size.setMinWidth(l); return s }
func (s *SliderElement) MaxWidth(l Length) *SliderElement       { s.size.setMaxWidth(l); return s }
func (s *SliderElement) Grow(f float32) *SliderElement          { s.size.setGrow(f); return s }
func (s *SliderElement) GridSpan(cols, rows int) *SliderElement { s.size.setSpan(cols, rows); return s }

// Format sets how the value is shown beside the track. The default shows the
// value with as many decimals as the step has.
func (s *SliderElement) Format(fn func(float64) string) *SliderElement { s.format = fn; return s }

// OnChange runs fn with the new value when the user moves the slider to a value
// that differs from the current one. The signal is set first.
func (s *SliderElement) OnChange(fn func(float64)) *SliderElement { s.onChange = fn; return s }

// Styles sets the look of the parts of the slider, for this instance only. Set
// fields win over the theme's Slider and the defaults; see SliderStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *SliderElement) Styles(st SliderStyle) *SliderElement {
	s.styles = s.styles.merge(st)
	return s
}

// steps returns the arrow step and the Page step. The Page step is a tenth of
// the range, rounded up to a whole number of arrow steps.
func (s *SliderElement) steps() (small, large float64) {
	small = s.step
	if small <= 0 {
		small = (s.hi - s.lo) / 100
	}
	large = math.Ceil((s.hi-s.lo)/10/small) * small
	return roundTo(small, decimalsOf(small)), roundTo(large, decimalsOf(small))
}

// snap returns x clamped to the range and rounded to the nearest step.
func (s *SliderElement) snap(x float64) float64 {
	if s.step > 0 {
		x = s.lo + math.Round((x-s.lo)/s.step)*s.step
	}
	return roundTo(min(max(x, s.lo), s.hi), decimalsOf(s.step))
}

// fraction returns how far v is along the track, from 0 to 1.
func (s *SliderElement) fraction(v float64) float64 {
	if s.hi <= s.lo {
		return 0
	}
	return min(max((v-s.lo)/(s.hi-s.lo), 0), 1)
}

// at returns the value under the pointer at e, which is a press or a move on the
// track. The track's ends are the centers of the outer thumb positions.
func (s *SliderElement) at(e *MouseEvent) float64 {
	inner := float64(e.Width) - sliderThumb
	if inner <= 0 {
		return s.lo
	}
	return s.lo + min(max((float64(e.LocalX)-sliderThumb/2)/inner, 0), 1)*(s.hi-s.lo)
}

func (s *SliderElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		st := resolveSlider(t, s.disabled, s.styles)
		v := s.value.Get()
		dragging := Local(ctx, "dragging", false)
		small, large := s.steps()
		set := func(x float64) {
			nv := s.snap(x)
			if nv == v {
				return
			}
			s.value.Set(nv)
			if s.onChange != nil {
				s.onChange(nv)
			}
		}
		f := float32(s.fraction(v))

		// The track is one focusable element. Its rail and thumb are drawn over it
		// and are not part of the access tree.
		fill := Div().asPart("Slider.Fill").Height(Full).Style(st.Fill).Grow(f)
		rail := Div().asPart("Slider.Rail").Row().Style(st.Rail).Grow(1).
			Child(fill).
			Child(Div().Height(Full).Grow(1 - f))
		// While the thumb is dragged the pointer is over it, so its hover style is
		// left off and ThumbPressed shows instead.
		thumb := Div().asPart("Slider.Thumb").Style(st.Thumb).Size(Px(sliderThumb), Px(sliderThumb)).Shrink(0)
		if dragging.Get() {
			thumb.Style(st.ThumbPressed)
		} else if !s.disabled {
			thumb.Hover(st.ThumbHover)
		}
		overlay := Div().Absolute().Inset(0).Row().Align(Center).
			Child(Div().Grow(f)).Child(thumb).Child(Div().Grow(1 - f))
		track := Div().asPart("Slider.Track").Height(Px(22)).Grow(1).Shrink(1).Style(st.Track).
			Child(Div().Row().Height(Px(22)).Align(Center).PaddingX(sliderThumb / 2).Child(rail)).
			Child(overlay).
			Tooltip(s.tooltip)

		readout := fmtValue(s, v)
		value := Div().asPart("Slider.ValueText").AccessHidden().MinWidth(Px(44)).Align(End).Style(st.ValueText).
			Child(Text(readout))
		// The default minimum keeps the track usable in a container with no width of its
		// own, where a percentage has nothing to take.
		root := Div().Row().Align(Center).Gap(12).Width(Full).Style(Style{MinWidth: Px(sliderMinWidth)}).
			Child(track).
			Child(value)
		s.size.apply(root)

		a := track.access()
		track.Role(access.RoleSlider)
		if s.name != "" {
			track.Label(s.name)
		}
		if len(s.labelledBy) > 0 {
			track.LabelledBy(s.labelledBy...)
		}
		a.rng = &access.Range{Value: v, Min: s.lo, Max: s.hi, SmallChange: small, LargeChange: large}
		a.setRange = set
		if s.disabled {
			a.disabled = true
			if t.HighContrast {
				root.TextColor(t.Disabled)
			} else {
				root.Opacity(0.5)
			}
			track.Cursor(CursorNotAllowed)
			return root
		}
		if s.focusHandle != nil {
			track.FocusHandle(s.focusHandle)
		}
		track.Cursor(CursorPointer).Focusable().FocusVisible(st.TrackFocusVisible).
			OnMouseDown(func(e *MouseEvent) {
				if e.Button == input.ButtonLeft {
					dragging.Set(true)
					set(s.at(e))
				}
			}).
			OnMouseMove(func(e *MouseEvent) {
				if dragging.Peek() {
					set(s.at(e))
				}
			}).
			OnMouseUp(func(*MouseEvent) { dragging.Set(false) }).
			CapturePointer().
			OnKeyDown(func(e *KeyEvent) {
				if e.Mods != 0 {
					return
				}
				var next float64
				switch e.Key {
				case input.KeyLeft, input.KeyDown:
					next = v - small
				case input.KeyRight, input.KeyUp:
					next = v + small
				case input.KeyPageDown:
					next = v - large
				case input.KeyPageUp:
					next = v + large
				case input.KeyHome:
					next = s.lo
				case input.KeyEnd:
					next = s.hi
				default:
					return
				}
				e.StopPropagation()
				set(next)
			})
		return root
	})
}

// fmtValue is the readout of the value v of slider s.
func fmtValue(s *SliderElement, v float64) string {
	if s.format != nil {
		return s.format(v)
	}
	return strconv.FormatFloat(v, 'f', decimalsOf(s.step), 64)
}

// decimalsOf returns the number of decimals of a step, such as 1 for 0.5.
func decimalsOf(step float64) int {
	s := strconv.FormatFloat(step, 'f', -1, 64)
	for i := range s {
		if s[i] == '.' {
			return len(s) - i - 1
		}
	}
	return 0
}

// roundTo rounds x to d decimals, which removes the error of binary floats from
// a sum of steps.
func roundTo(x float64, d int) float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', d, 64), 64)
	if err != nil {
		return x
	}
	return r
}
