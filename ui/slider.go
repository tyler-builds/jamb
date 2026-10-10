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
	width        float32
	format       func(float64) string
	onChange     func(float64)
}

// Slider creates a slider that reads and writes value, from lo to hi in steps of
// step. The default width is 240 px.
func Slider(value *Signal[float64], lo, hi, step float64) *SliderElement {
	return &SliderElement{value: value, lo: lo, hi: hi, step: step, width: 240}
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

// Width sets the width of the slider, in logical pixels, including its readout.
func (s *SliderElement) Width(px float32) *SliderElement { s.width = px; return s }

// Format sets how the value is shown beside the track. The default shows the
// value with as many decimals as the step has.
func (s *SliderElement) Format(fn func(float64) string) *SliderElement { s.format = fn; return s }

// OnChange runs fn with the new value when the user moves the slider to a value
// that differs from the current one. The signal is set first.
func (s *SliderElement) OnChange(fn func(float64)) *SliderElement { s.onChange = fn; return s }

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

		// The track is one focusable element. Its bar and thumb are drawn over it
		// and are not part of the access tree.
		// The colors of the parts. Under high contrast the parts are outlined and filled
		// with the text color, or with Highlight for the fill; a disabled slider is all
		// GrayText, with no dimming. Its readout is GrayText too.
		fill, edge, readoutColor := t.Accent, t.Text, t.TextMuted
		thumbFill, thumbEdge := t.Accent, t.Surface
		if t.HighContrast {
			fill, thumbFill, thumbEdge = t.Accent, t.Text, t.Background
		}
		if s.disabled && t.HighContrast {
			fill, edge, readoutColor, thumbFill = t.Disabled, t.Disabled, t.Disabled, t.Disabled
		}
		bar := Div().Row().Height(Px(4)).Grow(1).Radius(2).ClipContent().Background(t.SurfaceActive).
			Child(Div().Height(Full).Grow(f).Background(fill)).
			Child(Div().Height(Full).Grow(1 - f))
		if t.HighContrast {
			bar.Border(1, edge).Background(t.Surface)
		}
		thumb := Div().Size(Px(sliderThumb), Px(sliderThumb)).Radius(sliderThumb/2).Shrink(0).
			Background(thumbFill).Border(2, thumbEdge).Shadow(ShadowSm)
		if !s.disabled && !t.HighContrast {
			thumb.Hover(Style{Background: t.AccentHover})
		}
		overlay := Div().Absolute().Inset(0).Row().Align(Center).
			Child(Div().Grow(f)).Child(thumb).Child(Div().Grow(1 - f))
		track := Div().Height(Px(22)).Grow(1).Shrink(1).Radius(11).
			Child(Div().Row().Height(Px(22)).Align(Center).PaddingX(sliderThumb / 2).Child(bar)).
			Child(overlay)

		readout := fmtValue(s, v)
		root := Div().Row().Align(Center).Gap(12).Width(Px(s.width)).
			Child(track).
			Child(Div().Hidden().MinWidth(Px(44)).Align(End).Child(Text(readout).Size(13).Medium().Color(readoutColor)))

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
		track.Cursor(CursorPointer).Focusable().FocusVisible(Style{Shadows: []Shadow{{Spread: 2, Color: t.FocusRing}}}).
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
