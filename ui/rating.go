package ui

import (
	"math"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// ratingStar is the size of a rating's star, in logical pixels.
const ratingStar = 20

// RatingStyle is the look of each part of a rating. A part's style merges over
// the built-in defaults, then over the theme's Rating, then over the Styles of
// the instance; see docs/styling.md. Root is the row of stars: it takes focus and
// the pointer. Each Star holds its icon, which is drawn in the star's TextColor:
// an outline for a star above the value, and a filled star for one at or below
// it. The states are parts of their own: StarChecked for a filled star, StarHover
// for the stars up to the one under the pointer, which preview the value a click
// sets, StarDisabled and RootDisabled when the rating cannot change, and
// RootFocusVisible while it has keyboard focus.
type RatingStyle struct {
	Root             Style // the row of stars: the focus element
	RootFocusVisible Style // the row while it has keyboard focus
	RootDisabled     Style // the row when the rating is disabled
	Star             Style // each star: its box, and the color of its icon (TextColor)
	StarChecked      Style // a star at or below the value, drawn filled
	StarHover        Style // a star up to the one under the pointer, drawn filled
	StarDisabled     Style // each star when the rating is disabled
}

// merge returns s with the set fields of o applied on top, part by part.
func (s RatingStyle) merge(o RatingStyle) RatingStyle {
	return RatingStyle{
		Root:             s.Root.Merge(o.Root),
		RootFocusVisible: s.RootFocusVisible.Merge(o.RootFocusVisible),
		RootDisabled:     s.RootDisabled.Merge(o.RootDisabled),
		Star:             s.Star.Merge(o.Star),
		StarChecked:      s.StarChecked.Merge(o.StarChecked),
		StarHover:        s.StarHover.Merge(o.StarHover),
		StarDisabled:     s.StarDisabled.Merge(o.StarDisabled),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s RatingStyle) plain() RatingStyle {
	return RatingStyle{
		Root:             plainStyle(s.Root),
		RootFocusVisible: plainStyle(s.RootFocusVisible),
		RootDisabled:     plainStyle(s.RootDisabled),
		Star:             plainStyle(s.Star),
		StarChecked:      plainStyle(s.StarChecked),
		StarHover:        plainStyle(s.StarHover),
		StarDisabled:     plainStyle(s.StarDisabled),
	}
}

// ratingStarColor is the color of a filled star: Warning, darkened by a tenth in a
// light theme so that it keeps 3:1 against the light panels.
func ratingStarColor(t *Theme) Color {
	if t.Dark || t.HighContrast {
		return t.Warning
	}
	return t.Warning.Darken(0.1)
}

// ratingDefaults returns the built-in look of a rating, from the colors of t. An
// empty star is an outline in TextSubtle and a filled star is Warning (see
// ratingStarColor), so the value is shown by the shape of the stars as well as
// their color. Under high contrast a
// filled star is Highlight and an empty one the text color, and a disabled rating
// is GrayText and not dimmed.
func ratingDefaults(t *Theme) RatingStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := RatingStyle{
		Root:             Style{Direction: Row, Align: Center, Gap: 2, Radius: 4, NoShrink: true, fitWidth: true},
		RootFocusVisible: Style{Shadows: []Shadow{ring}},
		RootDisabled:     Style{Opacity: 0.5},
		Star:             Style{Direction: Row, Align: Center, Justify: Center, NoShrink: true, TextColor: t.TextSubtle},
		StarChecked:      Style{TextColor: ratingStarColor(t)},
		StarHover:        Style{TextColor: ratingStarColor(t)},
	}
	if t.HighContrast {
		d.RootDisabled = Style{Opacity: 1}
		d.Star.TextColor = t.Text
		d.StarChecked, d.StarHover = Style{TextColor: t.Accent}, Style{TextColor: t.Accent}
		d.StarDisabled = Style{TextColor: t.Disabled}
	}
	return d
}

// resolveRating returns the look of a rating: the defaults, merged with the
// theme's Rating and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveRating(ctx *Context, inst RatingStyle) RatingStyle {
	t := ctx.Theme()
	return layered(ctx, ratingDefaults(t), RatingStyle.merge, RatingStyle.plain,
		themeLayer(t.Rating), instanceLayer(inst))
}

// RatingElement is a row of stars that shows and sets a whole number from 0 to its
// maximum, bound to an int signal. The pointer over a star previews that value, a
// click sets it, and a click on the star of the current value clears it to 0.
// Keys: Left and Down lower the value by one, Right and Up raise it, Home sets 0,
// End the maximum, and a digit key sets that value when it is not above the
// maximum. Assistive technology reads it as a slider from 0 to the maximum, with
// a value text such as "3 of 5 stars" (Strings.RatingValue).
type RatingElement struct {
	label       string
	value       *Signal[int]
	max         int
	key         string
	disabled    bool
	readOnly    bool
	onChange    func(int)
	styles      RatingStyle
	rules       []Validator[int] // the validators of the value (see Validate)
	wire        fieldWire        // what a Field gives the rating (see Field)
	size        sizing           // the row's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle     // see FocusHandle
	tooltip     string           // see Tooltip
}

// Rating creates a rating of 5 stars that reads and writes value. label names the
// rating for assistive technology, such as "Rating"; a Field's label replaces it.
func Rating(label string, value *Signal[int]) *RatingElement {
	return &RatingElement{label: label, value: value, max: 5}
}

func (*RatingElement) isElement()           {}
func (r *RatingElement) elementKey() string { return r.key }

// Key identifies the rating among its siblings (see DivElement.Key).
func (r *RatingElement) Key(k string) *RatingElement { r.key = k; return r }

// Max sets the number of stars, which is the highest value. It is 5 by default,
// and at least 1.
func (r *RatingElement) Max(n int) *RatingElement { r.max = max(n, 1); return r }

// Disabled shows the rating dimmed and does not let it change.
func (r *RatingElement) Disabled(d bool) *RatingElement { r.disabled = d; return r }

// ReadOnly shows the value, not dimmed, and does not let it change: the rating takes
// no focus, pointer or keys. Assistive technology still reads it, as a read-only
// slider.
func (r *RatingElement) ReadOnly() *RatingElement { r.readOnly = true; return r }

// OnChange runs fn with the new value after the user changes the rating, by the
// pointer, the keys or assistive technology. The signal is set first.
func (r *RatingElement) OnChange(fn func(int)) *RatingElement { r.onChange = fn; return r }

// Validate sets the validators of the value. 0 is no rating, so Required[int]
// rejects it (see Field).
func (r *RatingElement) Validate(rules ...Validator[int]) *RatingElement {
	r.rules = rules
	return r
}

// formCheck and formWire make a rating a formControl (see Field).
func (r *RatingElement) formCheck() error { return firstError(r.value.Get(), r.rules) }

func (r *RatingElement) formWire(w fieldWire) Element {
	c := *r
	c.wire = w
	return &c
}

// FocusHandle lets code focus the rating (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (r *RatingElement) FocusHandle(h *FocusHandle) *RatingElement { r.focusHandle = h; return r }

// Tooltip shows s below the rating after the pointer rests on it. Assistive
// technology reads it as the rating's description, unless it has another.
func (r *RatingElement) Tooltip(s string) *RatingElement { r.tooltip = s; return r }

// The sizing methods give the row of stars a size. A rating is as wide as its
// stars by default; see Sizing in docs/styling.md.
func (r *RatingElement) Width(l Length) *RatingElement { r.size.setWidth(l); return r }

// MinWidth sets the smallest width of the rating.
func (r *RatingElement) MinWidth(l Length) *RatingElement { r.size.setMinWidth(l); return r }

// MaxWidth sets the largest width of the rating.
func (r *RatingElement) MaxWidth(l Length) *RatingElement { r.size.setMaxWidth(l); return r }

// Grow makes the rating take a share of the free space on its parent's main axis.
func (r *RatingElement) Grow(f float32) *RatingElement { r.size.setGrow(f); return r }

// GridSpan sets how many columns and rows the rating covers in its parent's grid.
func (r *RatingElement) GridSpan(cols, rows int) *RatingElement { r.size.setSpan(cols, rows); return r }

// Styles sets the look of the parts of the rating, for this instance only. Set
// fields win over the theme's Rating and the defaults; see RatingStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (r *RatingElement) Styles(st RatingStyle) *RatingElement {
	r.styles = r.styles.merge(st)
	return r
}

// starAt returns the star under the pointer at e, from 1 to the maximum. The stars
// share the row's width equally.
func (r *RatingElement) starAt(e *MouseEvent) int {
	if e.Width <= 0 {
		return 1
	}
	return min(max(int(e.LocalX/e.Width*float32(r.max))+1, 1), r.max)
}

func (r *RatingElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		st := resolveRating(ctx, r.styles)
		v := min(max(r.value.Get(), 0), r.max)
		// hover is the star under the pointer, or 0 when the pointer is not over the
		// row. After a click it is the clicked star negated: the preview is off until
		// the pointer moves to another star.
		hover := Local(ctx, "hover", 0)
		preview := 0
		if !r.disabled && !r.readOnly {
			preview = max(hover.Get(), 0)
		}
		set := func(n int) {
			n = min(max(n, 0), r.max)
			if n == v {
				return
			}
			r.value.Set(n)
			if r.wire.onEdit != nil {
				r.wire.onEdit()
			}
			if r.onChange != nil {
				r.onChange(n)
			}
		}

		row := Div().asPart("Rating.Root").Style(st.Root)
		for i := 1; i <= r.max; i++ {
			// A filled star is drawn as a filled shape, so the value is not carried by
			// color alone.
			filled := i <= v
			star := Div().asPart("Rating.Star").Style(st.Star)
			if preview > 0 {
				filled = i <= preview
			}
			if i <= v {
				star.Style(st.StarChecked)
			}
			if i <= preview {
				star.Style(st.StarHover)
			}
			if r.disabled {
				star.Style(st.StarDisabled)
			}
			ic := icons.Star
			if filled {
				ic = icons.StarFilled
			}
			row.Child(star.Child(Icon(ic).Size(ratingStar)))
		}

		w := r.wire
		name, labelledBy := r.label, []string(nil)
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		row.Role(access.RoleSlider).Label(name).Tooltip(r.tooltip)
		if len(labelledBy) > 0 {
			row.LabelledBy(labelledBy...)
		}
		if w.description != "" {
			row.Description(w.description)
		}
		a := row.access()
		a.invalid, a.required = w.invalid, w.required
		a.rng = &access.Range{Value: float64(v), Min: 0, Max: float64(r.max), SmallChange: 1, LargeChange: 1}
		a.value = ctx.Strings().RatingValue(v, r.max)
		if r.disabled {
			a.disabled = true
			row.Style(st.RootDisabled).Cursor(CursorNotAllowed)
			return r.size.apply(row)
		}
		if r.readOnly {
			a.readOnly = true
			return r.size.apply(row)
		}
		a.setRange = func(x float64) { set(int(math.Round(x))) }
		h := r.focusHandle
		if w.handle != nil {
			h = w.handle
		}
		if h != nil {
			row.FocusHandle(h)
		}
		if w.onBlur != nil {
			row.OnBlur(w.onBlur)
		}
		return r.size.apply(row.Cursor(CursorPointer).Focusable().FocusVisible(st.RootFocusVisible).
			OnMouseMove(func(e *MouseEvent) {
				if n := r.starAt(e); n != hover.Peek() && -n != hover.Peek() {
					hover.Set(n)
				}
			}).
			OnMouseLeave(func() { hover.Set(0) }).
			OnMouseDown(func(e *MouseEvent) {
				if e.Button != input.ButtonLeft {
					return
				}
				// A click on the star of the current value clears the rating. The preview
				// stops until the pointer moves to another star, so the new value shows.
				n := r.starAt(e)
				hover.Set(-n)
				if n == v {
					set(0)
				} else {
					set(n)
				}
			}).
			OnKeyDown(func(e *KeyEvent) {
				if e.Mods != 0 {
					return
				}
				next := v
				switch {
				case e.Key == input.KeyLeft || e.Key == input.KeyDown:
					next = v - 1
				case e.Key == input.KeyRight || e.Key == input.KeyUp:
					next = v + 1
				case e.Key == input.KeyHome:
					next = 0
				case e.Key == input.KeyEnd:
					next = r.max
				case e.Key >= input.Key0 && e.Key <= input.Key9:
					next = int(e.Key - input.Key0)
					if next > r.max {
						return
					}
				default:
					return
				}
				e.StopPropagation()
				set(next)
			}))
	})
}
