package ui

import (
	"fmt"
	"math"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ease"
)

// Sizes of a ProgressRing, for ProgressRingElement.Size.
const (
	RingSmall  float32 = 16
	RingMedium float32 = 24
	RingLarge  float32 = 40
)

// progressSegment is the share of a bar's width that an indeterminate bar fills.
const progressSegment = 0.3

// progressBarMinWidth is the narrowest a progress bar is by default, in logical
// pixels. Like the minimum of a CSS progress element, it does not scale with the text.
const progressBarMinWidth = 160

// ProgressBarStyle is the look of each part of a progress bar, determinate or
// indeterminate. A part's style merges over the built-in defaults, then over the
// theme's ProgressBar, then over the Styles of the instance; see docs/styling.md.
// Track is the bar's groove, Fill the part that shows the progress (in an
// indeterminate bar, the segment that slides along the groove), and Label the
// readout beside the bar. The width of Fill follows the value, so set its colors,
// radius and border, not its width.
type ProgressBarStyle struct {
	Track Style // the groove the fill is drawn in
	Fill  Style // the filled part, or the sliding segment of an indeterminate bar
	Label Style // the readout beside the bar, a percentage by default
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ProgressBarStyle) merge(o ProgressBarStyle) ProgressBarStyle {
	return ProgressBarStyle{
		Track: s.Track.Merge(o.Track),
		Fill:  s.Fill.Merge(o.Fill),
		Label: s.Label.Merge(o.Label),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ProgressBarStyle) plain() ProgressBarStyle {
	return ProgressBarStyle{Track: plainStyle(s.Track), Fill: plainStyle(s.Fill), Label: plainStyle(s.Label)}
}

// progressBarDefaults returns the built-in look of a progress bar from the colors
// of t. The fill is color when the app gave one (see ProgressBarElement.Color),
// else the accent. Under high contrast the groove is an outlined empty box and
// the fill is the Highlight color, whatever the app asked for.
func progressBarDefaults(t *Theme, color Color) ProgressBarStyle {
	d := ProgressBarStyle{
		Track: Style{Height: Px(6), Grow: 1, Radius: 3, OverflowX: OverflowHidden, OverflowY: OverflowHidden,
			Background: t.SurfaceActive},
		Fill:  Style{Background: t.Accent, Radius: 3},
		Label: Style{FontSize: 13, FontWeight: FontMedium, TextColor: t.TextMuted},
	}
	if !color.IsZero() && !t.HighContrast {
		d.Fill.Background = color
	}
	if t.HighContrast {
		d.Track = Style{Height: Px(8), Grow: 1, Radius: 3, OverflowX: OverflowHidden, OverflowY: OverflowHidden,
			Background: t.Surface, BorderWidth: 1, BorderColor: t.Text}
		d.Fill = Style{Background: t.Accent}
	}
	return d
}

// resolveProgressBar returns the look of a progress bar: the defaults, merged with
// the theme's ProgressBar and then the instance's Styles. Under high contrast the
// two overrides are plain (see plainStyle).
func resolveProgressBar(t *Theme, color Color, inst ProgressBarStyle) ProgressBarStyle {
	st := progressBarDefaults(t, color)
	for _, o := range []ProgressBarStyle{t.ProgressBar, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// ProgressRingStyle is the look of a progress ring. A part's style merges over
// the built-in defaults, then over the theme's ProgressRing, then over the Styles
// of the instance; see docs/styling.md. Ring is the box the dots are drawn in,
// whose size is the ring's Size. Dot is each dot: its color and its opacity
// follow the motion of the ring and the contrast, and a Dot style merges over
// them, so a Background in it replaces the dot's color.
type ProgressRingStyle struct {
	Ring Style // the box of the ring
	Dot  Style // each dot of the ring
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ProgressRingStyle) merge(o ProgressRingStyle) ProgressRingStyle {
	return ProgressRingStyle{Ring: s.Ring.Merge(o.Ring), Dot: s.Dot.Merge(o.Dot)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ProgressRingStyle) plain() ProgressRingStyle {
	return ProgressRingStyle{Ring: plainStyle(s.Ring), Dot: plainStyle(s.Dot)}
}

// resolveProgressRing returns the look of a progress ring: the theme's
// ProgressRing, then the instance's Styles, over the built-in ring, which takes
// no colors of its own. Under high contrast the two overrides are plain.
func resolveProgressRing(t *Theme, inst ProgressRingStyle) ProgressRingStyle {
	st := ProgressRingStyle{Ring: Style{NoShrink: true}}
	for _, o := range []ProgressRingStyle{t.ProgressRing, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// ProgressBarElement shows how far an operation has got, as a bar. A
// determinate bar fills from the low end of its range to its value. An
// indeterminate bar slides while the work runs, for work whose length is not
// known. It is read-only: nothing the user does changes it.
type ProgressBarElement struct {
	value         float64
	lo, hi        float64
	indeterminate bool
	name          string
	key           string
	size          sizing
	format        func(float64) string
	color         Color // the fill, when set (see Color); zero is the theme accent
	styles        ProgressBarStyle
}

// ProgressBar creates a bar showing value. The range is 0 to 1 by default; see
// Min and Max. For work of unknown length, see Indeterminate. The bar fills the
// width of its container by default (see Width).
func ProgressBar(value float64) *ProgressBarElement {
	return &ProgressBarElement{value: value, lo: 0, hi: 1}
}

func (*ProgressBarElement) isElement()           {}
func (p *ProgressBarElement) elementKey() string { return p.key }

func (p *ProgressBarElement) Key(k string) *ProgressBarElement { p.key = k; return p }

// Label names the bar for assistive technology, such as "Upload". Without it
// the bar is named "Progress", or "Loading…" when it is indeterminate.
func (p *ProgressBarElement) Label(name string) *ProgressBarElement { p.name = name; return p }

// Indeterminate makes the bar show work of unknown length, such as
// ui.ProgressBar(0).Indeterminate(). It slides while the work runs, and is drawn
// in the middle when reduced motion is on. Its value is not shown.
func (p *ProgressBarElement) Indeterminate() *ProgressBarElement { p.indeterminate = true; return p }

// Min sets the low end of the value's range, 0 by default. A value outside the
// range is clamped.
func (p *ProgressBarElement) Min(lo float64) *ProgressBarElement { p.lo = lo; return p }

// Max sets the high end of the value's range, 1 by default. A value outside the
// range is clamped.
func (p *ProgressBarElement) Max(hi float64) *ProgressBarElement { p.hi = hi; return p }

// The sizing methods give the bar's outer box a size, including its readout. A bar
// fills its container by default; Width(Px(240)) fixes it, and the track keeps the
// room that is left for the readout. See Sizing in docs/styling.md.
func (p *ProgressBarElement) Width(l Length) *ProgressBarElement    { p.size.setWidth(l); return p }
func (p *ProgressBarElement) MinWidth(l Length) *ProgressBarElement { p.size.setMinWidth(l); return p }
func (p *ProgressBarElement) MaxWidth(l Length) *ProgressBarElement { p.size.setMaxWidth(l); return p }
func (p *ProgressBarElement) Grow(f float32) *ProgressBarElement    { p.size.setGrow(f); return p }

// Color sets the color of the fill, for an app that has a color of its own, such as
// a project's. The default is the theme accent. The app is responsible for the
// contrast of its color against the track: the fill is a non-text object and needs
// 3:1 against the track (WCAG 1.4.11). Under high contrast the fill is the
// Highlight color, and Color is not used.
func (p *ProgressBarElement) Color(c Color) *ProgressBarElement {
	p.color = c
	return p
}

// Format sets the readout beside the bar. The default shows the share of the
// range that is done, as a percentage. An indeterminate bar has no readout.
func (p *ProgressBarElement) Format(fn func(float64) string) *ProgressBarElement {
	p.format = fn
	return p
}

// Styles sets the look of the parts of the bar, for this instance only. Set
// fields win over the theme's ProgressBar and the defaults; see ProgressBarStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (p *ProgressBarElement) Styles(st ProgressBarStyle) *ProgressBarElement {
	p.styles = p.styles.merge(st)
	return p
}

// clamped returns the value within the range.
func (p *ProgressBarElement) clamped() float64 {
	return min(max(p.value, min(p.lo, p.hi)), max(p.lo, p.hi))
}

// fraction returns how far the value is along the bar, from 0 to 1.
func (p *ProgressBarElement) fraction() float64 {
	if p.hi <= p.lo {
		return 0
	}
	return (p.clamped() - p.lo) / (p.hi - p.lo)
}

// readout is the text beside a determinate bar.
func (p *ProgressBarElement) readout() string {
	if p.format != nil {
		return p.format(p.clamped())
	}
	return fmt.Sprintf("%.0f%%", p.fraction()*100)
}

func (p *ProgressBarElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	st := resolveProgressBar(t, p.color, p.styles)
	// The track is drawn as a thin bar. Under high contrast it is an outlined
	// empty box and the fill is solid Highlight, so the empty part does not read
	// as filled, as the demo's bars do.
	track := Div().asPart("ProgressBar.Track").Style(st.Track)

	name := p.name
	a := track.access()
	track.Role(access.RoleProgressBar)
	a.readOnly = true
	if p.indeterminate {
		if name == "" {
			name = "Loading…"
		}
		track.Child(p.segment(ctx, st))
	} else {
		if name == "" {
			name = "Progress"
		}
		// The value is read-only: the range has no setter, so no SetValue action.
		a.rng = &access.Range{Value: p.clamped(), Min: min(p.lo, p.hi), Max: max(p.lo, p.hi)}
		track.Child(p.fill(st))
	}
	track.Label(name)

	// The bar's default minimum keeps it visible in a container with no width of its
	// own, where a percentage has nothing to take.
	root := Div().Row().Align(Center).Gap(12).Width(Full).Style(Style{MinWidth: Px(progressBarMinWidth)}).Child(track)
	if !p.indeterminate {
		// The value is what assistive technology reads through the range, so the
		// readout is hidden from it.
		root.Child(Div().asPart("ProgressBar.Label").AccessHidden().MinWidth(Px(44)).Align(End).Style(st.Label).
			Child(Text(p.readout())))
	}
	return p.size.apply(root)
}

// fill is the filled part of a determinate bar.
func (p *ProgressBarElement) fill(st ProgressBarStyle) *DivElement {
	f := float32(p.fraction())
	return Div().asPart("ProgressBar.Fill").Style(st.Fill).Height(Full).Width(Percent(f * 100))
}

// segment is the sliding part of an indeterminate bar. It moves from one end
// to the other and repeats. With reduced motion it stays in the middle.
func (p *ProgressBarElement) segment(ctx *Context, st ProgressBarStyle) Element {
	row := func(x float32) *DivElement {
		seg := Div().asPart("ProgressBar.Fill").Style(st.Fill).Height(Full).Grow(progressSegment)
		return Div().Row().Height(Full).
			Child(Div().Grow(x)).Child(seg).Child(Div().Grow(1 - x - progressSegment))
	}
	if ctx.ReducedMotion() {
		return row((1 - progressSegment) / 2)
	}
	return Animate("progress", Animation{Duration: 1400 * time.Millisecond, Repeat: true, Easing: ease.Linear},
		func(p float32) Element { return row(p * (1 - progressSegment)) })
}

// ProgressRingElement is a busy indicator: a ring of dots that turns while the
// app waits for something. It has no value, so it says only that work is
// under way. It is named "Loading…" unless Label gives another name.
type ProgressRingElement struct {
	size   float32
	name   string
	key    string
	styles ProgressRingStyle
}

// ProgressRing creates a busy indicator of the medium size (RingMedium).
func ProgressRing() *ProgressRingElement { return &ProgressRingElement{size: RingMedium} }

func (*ProgressRingElement) isElement()           {}
func (r *ProgressRingElement) elementKey() string { return r.key }

func (r *ProgressRingElement) Key(k string) *ProgressRingElement { r.key = k; return r }

// Size sets the diameter, in logical pixels: RingSmall, RingMedium or RingLarge.
func (r *ProgressRingElement) Size(px float32) *ProgressRingElement { r.size = px; return r }

// Label names the indicator for assistive technology, such as "Sending reply".
func (r *ProgressRingElement) Label(name string) *ProgressRingElement { r.name = name; return r }

// Styles sets the look of the parts of the ring, for this instance only. Set
// fields win over the theme's ProgressRing and the defaults; see
// ProgressRingStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (r *ProgressRingElement) Styles(st ProgressRingStyle) *ProgressRingElement {
	r.styles = r.styles.merge(st)
	return r
}

// ringDots is the number of dots in a ring.
const ringDots = 8

func (r *ProgressRingElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	st := resolveProgressRing(t, r.styles)
	name := r.name
	if name == "" {
		name = "Loading…"
	}
	// ring is the box of the indicator. It is one node with the busy role, and
	// its dots are not exposed.
	ring := func() *DivElement {
		return Div().asPart("ProgressRing.Ring").Style(st.Ring).Size(Px(r.size), Px(r.size)).Role(access.RoleProgressBar).Label(name)
	}
	if ctx.ReducedMotion() {
		// Reduced motion: every dot is drawn, with no turning. Under high contrast the
		// dots are solid; otherwise they are dimmed, so the ring reads as busy without
		// moving.
		box := ring()
		for i := range ringDots {
			if t.HighContrast {
				box.Child(r.dot(st, i, t.Text, 1))
			} else {
				box.Child(r.dot(st, i, t.Accent, 0.35))
			}
		}
		return box
	}
	return Animate("ring", Animation{Duration: 900 * time.Millisecond, Repeat: true, Easing: ease.Linear},
		func(head float32) Element {
			box := ring()
			for i := range ringDots {
				// The head is the brightest dot and the dots behind it fade. Four
				// dots are lit at once, so the ring reads as a moving arc.
				phase := head - float32(i)/ringDots
				phase -= float32(math.Floor(float64(phase)))
				if phase >= 0.5 {
					continue
				}
				if t.HighContrast {
					box.Child(r.dot(st, i, t.Text, 1))
				} else {
					box.Child(r.dot(st, i, t.Accent, 1-2*phase))
				}
			}
			return box
		})
}

// dot is the i-th dot of a ring. It sits on the circle at an angle of i/8 of a
// turn, starting at the top, with the given color and opacity, and the dot style
// merged over them.
func (r *ProgressRingElement) dot(st ProgressRingStyle, i int, c Color, opacity float32) *DivElement {
	d := r.size / 4
	radius := r.size/2 - d/2
	angle := 2*math.Pi*float64(i)/ringDots - math.Pi/2
	x := r.size/2 + radius*float32(math.Cos(angle)) - d/2
	y := r.size/2 + radius*float32(math.Sin(angle)) - d/2
	return Div().asPart("ProgressRing.Dot").Absolute().Left(x).Top(y).Size(Px(d), Px(d)).Radius(d / 2).
		Background(c).Opacity(opacity).Style(st.Dot)
}
