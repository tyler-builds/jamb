package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// RangeSliderStyle is the look of each part of a range slider. A part's style
// merges over the built-in defaults, then over the theme's RangeSlider, then over
// the Styles of the instance; see docs/styling.md.
//
// Track is the area that takes the pointer, with the rail and the thumbs drawn
// over it. Rail is the line the values are drawn on, and Fill the part of the rail
// between the two thumbs (its width comes from the values, so Grow is not set by
// the style). Thumb is each knob, always sliderThumb pixels wide. The thumbs are
// the controls that take focus: ThumbFocusVisible is a thumb with keyboard focus,
// ThumbHover a thumb under the pointer while the slider is enabled, and
// ThumbPressed the thumb that is dragged. ValueText is the readout of the two
// values beside the track.
type RangeSliderStyle struct {
	Track             Style // the area that takes the pointer
	Rail              Style // the line the values are drawn on
	Fill              Style // the part of the rail between the thumbs
	Thumb             Style // each knob
	ThumbHover        Style // a knob while the pointer is over it
	ThumbPressed      Style // the knob that is dragged
	ThumbFocusVisible Style // a knob while it has keyboard focus
	ValueText         Style // the readout beside the track
}

// merge returns s with the set fields of o applied on top, part by part.
func (s RangeSliderStyle) merge(o RangeSliderStyle) RangeSliderStyle {
	return RangeSliderStyle{
		Track:             s.Track.Merge(o.Track),
		Rail:              s.Rail.Merge(o.Rail),
		Fill:              s.Fill.Merge(o.Fill),
		Thumb:             s.Thumb.Merge(o.Thumb),
		ThumbHover:        s.ThumbHover.Merge(o.ThumbHover),
		ThumbPressed:      s.ThumbPressed.Merge(o.ThumbPressed),
		ThumbFocusVisible: s.ThumbFocusVisible.Merge(o.ThumbFocusVisible),
		ValueText:         s.ValueText.Merge(o.ValueText),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s RangeSliderStyle) plain() RangeSliderStyle {
	return RangeSliderStyle{
		Track:             plainStyle(s.Track),
		Rail:              plainStyle(s.Rail),
		Fill:              plainStyle(s.Fill),
		Thumb:             plainStyle(s.Thumb),
		ThumbHover:        plainStyle(s.ThumbHover),
		ThumbPressed:      plainStyle(s.ThumbPressed),
		ThumbFocusVisible: plainStyle(s.ThumbFocusVisible),
		ValueText:         plainStyle(s.ValueText),
	}
}

// rangeSliderDefaults returns the built-in look of a range slider, from the colors
// of t: the parts of a Slider, with the focus ring on the thumb that has focus.
func rangeSliderDefaults(t *Theme, disabled bool) RangeSliderStyle {
	sd := sliderDefaults(t, disabled)
	return RangeSliderStyle{
		Track:             sd.Track,
		Rail:              sd.Rail,
		Fill:              sd.Fill,
		Thumb:             sd.Thumb,
		ThumbHover:        sd.ThumbHover,
		ThumbPressed:      sd.ThumbPressed,
		ThumbFocusVisible: sd.TrackFocusVisible,
		ValueText:         sd.ValueText,
	}
}

// resolveRangeSlider returns the look of a range slider: the defaults, merged with
// the theme's RangeSlider and then the instance's Styles. Under high contrast the
// two overrides are plain (see plainStyle).
func resolveRangeSlider(ctx *Context, disabled bool, inst RangeSliderStyle) RangeSliderStyle {
	t := ctx.Theme()
	return layered(ctx, rangeSliderDefaults(t, disabled), RangeSliderStyle.merge, RangeSliderStyle.plain,
		themeLayer(t.RangeSlider), instanceLayer(inst))
}

// RangeSliderElement sets a range, a low and a high value, with two thumbs on one
// track from lo to hi, in steps of step. The track between the thumbs is filled.
// The thumbs cannot cross: a thumb moved into the other stops there, or MinGap
// before it. A press on the track moves the nearer thumb there, and a drag moves
// it on. Each thumb is a Tab stop, with the keys of a Slider: the arrows move it by
// one step, Page Up and Page Down by a tenth of the range, and Home and End as far
// as it can go.
type RangeSliderElement struct {
	low, high    *Signal[float64]
	lo, hi, step float64
	gap          float64 // see MinGap
	name         string  // the group's accessible name, see Label
	labelledBy   []string
	key          string
	disabled     bool
	size         sizing
	format       func(float64) string
	onChange     func(low, high float64)
	styles       RangeSliderStyle
	focusHandle  *FocusHandle // see FocusHandle
	tooltip      string       // see Tooltip
}

// RangeSlider creates a range slider that reads and writes low and high, from lo
// to hi in steps of step. It fills the width of its container by default, as a
// Slider does (see Width).
func RangeSlider(low, high *Signal[float64], lo, hi, step float64) *RangeSliderElement {
	return &RangeSliderElement{low: low, high: high, lo: lo, hi: hi, step: step}
}

func (*RangeSliderElement) isElement()           {}
func (s *RangeSliderElement) elementKey() string { return s.key }

// Key identifies the range slider among its siblings (see DivElement.Key).
func (s *RangeSliderElement) Key(k string) *RangeSliderElement { s.key = k; return s }

// Label names the range slider for assistive technology, such as "Price". It is
// the name of the group that holds the two thumbs, which are named
// Strings.Minimum and Strings.Maximum.
func (s *RangeSliderElement) Label(name string) *RangeSliderElement { s.name = name; return s }

// LabelledBy names the range slider's group by the text of the elements with the
// given keys; see DivElement.LabelledBy.
func (s *RangeSliderElement) LabelledBy(keys ...string) *RangeSliderElement {
	s.labelledBy = keys
	return s
}

// Disabled shows the range slider but does not let it change.
func (s *RangeSliderElement) Disabled(d bool) *RangeSliderElement { s.disabled = d; return s }

// MinGap keeps the high value at least gap above the low value, so the thumbs stop
// gap apart. It is 0 by default, where the two values may be equal. Give a whole
// number of steps.
func (s *RangeSliderElement) MinGap(gap float64) *RangeSliderElement {
	s.gap = max(gap, 0)
	return s
}

// Tooltip shows s below the range slider after the pointer rests on it. Assistive
// technology reads it as the group's description, unless it has another.
func (s *RangeSliderElement) Tooltip(text string) *RangeSliderElement {
	s.tooltip = text
	return s
}

// FocusHandle lets code focus the low thumb (see NewFocusHandle).
func (s *RangeSliderElement) FocusHandle(h *FocusHandle) *RangeSliderElement {
	s.focusHandle = h
	return s
}

// The sizing methods give the range slider's outer box a size, including its
// readout, as for a Slider. See Sizing in docs/styling.md.
func (s *RangeSliderElement) Width(l Length) *RangeSliderElement { s.size.setWidth(l); return s }

// MinWidth sets the smallest width of the range slider.
func (s *RangeSliderElement) MinWidth(l Length) *RangeSliderElement { s.size.setMinWidth(l); return s }

// MaxWidth sets the largest width of the range slider.
func (s *RangeSliderElement) MaxWidth(l Length) *RangeSliderElement { s.size.setMaxWidth(l); return s }

// Grow makes the range slider take a share of the free space on its parent's main axis.
func (s *RangeSliderElement) Grow(f float32) *RangeSliderElement { s.size.setGrow(f); return s }

// GridSpan sets how many columns and rows the range slider covers in its parent's grid.
func (s *RangeSliderElement) GridSpan(cols, rows int) *RangeSliderElement {
	s.size.setSpan(cols, rows)
	return s
}

// Format sets how each value is shown beside the track and read by assistive
// technology. The default shows the value with as many decimals as the step has.
func (s *RangeSliderElement) Format(fn func(float64) string) *RangeSliderElement {
	s.format = fn
	return s
}

// OnChange runs fn with the new values when the user moves a thumb to a value that
// differs from its current one. The signals are set first.
func (s *RangeSliderElement) OnChange(fn func(low, high float64)) *RangeSliderElement {
	s.onChange = fn
	return s
}

// Styles sets the look of the parts of the range slider, for this instance only.
// Set fields win over the theme's RangeSlider and the defaults; see
// RangeSliderStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (s *RangeSliderElement) Styles(st RangeSliderStyle) *RangeSliderElement {
	s.styles = s.styles.merge(st)
	return s
}

// axis returns a slider with the range, step and format of s, for its arithmetic.
func (s *RangeSliderElement) axis() *SliderElement {
	return &SliderElement{lo: s.lo, hi: s.hi, step: s.step, format: s.format}
}

// The thumbs of a range slider. A thumb's center travels a track that is two
// thumbs shorter than the slider's: the low thumb from sliderThumb/2, the high
// thumb from sliderThumb*3/2, so the two never overlap.
const (
	thumbLow  = 0
	thumbHigh = 1
)

// center returns the x of the center of thumb i at fraction f, on a track of
// width w.
func rangeThumbCenter(i int, f, w float64) float64 {
	return sliderThumb/2 + float64(i)*sliderThumb + f*max(w-2*sliderThumb, 0)
}

// rangeAt returns the value under the pointer for thumb i, on a track of width w.
func (s *RangeSliderElement) rangeAt(i int, x, w float64) float64 {
	inner := w - 2*sliderThumb
	if inner <= 0 {
		return s.lo
	}
	f := (x - sliderThumb/2 - float64(i)*sliderThumb) / inner
	return s.lo + min(max(f, 0), 1)*(s.hi-s.lo)
}

func (s *RangeSliderElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		strs := ctx.Strings()
		st := resolveRangeSlider(ctx, s.disabled, s.styles)
		ax := s.axis()
		low, high := s.low.Get(), s.high.Get()
		drag := Local(ctx, "drag", -1) // the thumb that is dragged, or -1
		// One focus handle per thumb, kept across renders, so a press on the track can
		// focus the thumb it moves. The app's handle is the low thumb's.
		kept := Local(ctx, "handles", newHandles(2)).Peek()
		handles := [2]*FocusHandle{kept[thumbLow], kept[thumbHigh]}
		if s.focusHandle != nil {
			handles[thumbLow] = s.focusHandle
		}
		small, large := ax.steps()

		// bounds returns how far thumb i can go: the other thumb, less the gap, stops it.
		bounds := func(i int) (float64, float64) {
			if i == thumbLow {
				return s.lo, s.high.Peek() - s.gap
			}
			return s.low.Peek() + s.gap, s.hi
		}
		// set moves thumb i to x, snapped to a step and kept within its bounds.
		set := func(i int, x float64) {
			from, to := bounds(i)
			nv := min(max(ax.snap(x), from), to)
			sig := s.low
			if i == thumbHigh {
				sig = s.high
			}
			if nv == sig.Peek() {
				return
			}
			sig.Set(nv)
			if s.onChange != nil {
				s.onChange(s.low.Peek(), s.high.Peek())
			}
		}
		fl, fh := float32(ax.fraction(low)), float32(ax.fraction(high))

		// The rail is padded by half a thumb, as a Slider's is, and the fill runs
		// from the center of the low thumb to the center of the high one: a thumb
		// wide plus its share of the room the thumbs leave.
		fill := Div().asPart("RangeSlider.Fill").Height(Full).Style(st.Fill).
			Width(Px(sliderThumb)).Grow(fh - fl)
		rail := Div().asPart("RangeSlider.Rail").Row().Style(st.Rail).Grow(1).
			Child(Div().Height(Full).Grow(fl)).
			Child(fill).
			Child(Div().Height(Full).Grow(1 - fh))
		thumbs := Div().Absolute().Inset(0).Row().Align(Center)
		values := [2]float64{low, high}
		names := [2]string{strs.Minimum, strs.Maximum}
		for i := range 2 {
			thumb := Div().asPart("RangeSlider.Thumb").Style(st.Thumb).Size(Px(sliderThumb), Px(sliderThumb)).Shrink(0)
			if drag.Get() == i {
				thumb.Style(st.ThumbPressed)
			} else if !s.disabled {
				thumb.Hover(st.ThumbHover)
			}
			from, to := bounds(i)
			thumb.Role(access.RoleSlider).Label(names[i])
			a := thumb.access()
			a.rng = &access.Range{Value: values[i], Min: s.lo, Max: s.hi, SmallChange: small, LargeChange: large}
			a.value = fmtValue(ax, values[i])
			a.setRange = func(x float64) { set(i, x) }
			if s.disabled {
				a.disabled = true
			} else {
				v := values[i]
				thumb.FocusHandle(handles[i]).FocusVisible(st.ThumbFocusVisible).
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
							next = from
						case input.KeyEnd:
							next = to
						default:
							return
						}
						e.StopPropagation()
						set(i, next)
					})
			}
			if i == thumbLow {
				thumbs.Child(Div().Grow(fl)).Child(thumb)
			} else {
				thumbs.Child(Div().Grow(fh - fl)).Child(thumb).Child(Div().Grow(1 - fh))
			}
		}
		track := Div().asPart("RangeSlider.Track").Height(Px(22)).Grow(1).Shrink(1).Style(st.Track).
			Child(Div().Row().Height(Px(22)).Align(Center).PaddingX(sliderThumb / 2).Child(rail)).
			Child(thumbs)

		readout := fmtValue(ax, low) + " – " + fmtValue(ax, high)
		value := Div().asPart("RangeSlider.ValueText").AccessHidden().MinWidth(Px(44)).Align(End).Style(st.ValueText).
			Child(Text(readout).NoWrap())
		// The group holds the two thumbs, and names them for assistive technology.
		root := Div().Row().Align(Center).Gap(12).Width(Full).Style(Style{MinWidth: Px(sliderMinWidth)}).
			Role(access.RoleGroup).Tooltip(s.tooltip).
			Child(track).
			Child(value)
		if s.name != "" {
			root.Label(s.name)
		}
		if len(s.labelledBy) > 0 {
			root.LabelledBy(s.labelledBy...)
		}
		s.size.apply(root)
		if s.disabled {
			if t.HighContrast {
				root.TextColor(t.Disabled)
			} else {
				root.Opacity(0.5)
			}
			track.Cursor(CursorNotAllowed)
			return root
		}
		// A press on the track moves the nearer thumb there and focuses it, and a
		// drag moves it on.
		nearest := func(e *MouseEvent) int {
			x, w := float64(e.LocalX), float64(e.Width)
			dl := abs64(x - rangeThumbCenter(thumbLow, float64(fl), w))
			dh := abs64(x - rangeThumbCenter(thumbHigh, float64(fh), w))
			if dh < dl || dh == dl && x > rangeThumbCenter(thumbLow, float64(fl), w) {
				return thumbHigh
			}
			return thumbLow
		}
		track.Cursor(CursorPointer).
			OnMouseDown(func(e *MouseEvent) {
				if e.Button != input.ButtonLeft {
					return
				}
				i := nearest(e)
				drag.Set(i)
				handles[i].Focus()
				set(i, s.rangeAt(i, float64(e.LocalX), float64(e.Width)))
			}).
			OnMouseMove(func(e *MouseEvent) {
				if i := drag.Peek(); i >= 0 {
					set(i, s.rangeAt(i, float64(e.LocalX), float64(e.Width)))
				}
			}).
			OnMouseUp(func(*MouseEvent) { drag.Set(-1) }).
			CapturePointer()
		return root
	})
}

// abs64 returns the absolute value of v.
func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
