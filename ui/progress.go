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
	width         float32
	format        func(float64) string
	color         Color // the fill, when set (see Color); zero is the theme accent
}

// ProgressBar creates a determinate bar showing value. The range is 0 to 1 by
// default; see Range. The default width is 240 px.
func ProgressBar(value float64) *ProgressBarElement {
	return &ProgressBarElement{value: value, lo: 0, hi: 1, width: 240}
}

// IndeterminateProgressBar creates a bar for work of unknown length. It slides
// while the work runs, and is drawn in the middle when reduced motion is on.
func IndeterminateProgressBar() *ProgressBarElement {
	return &ProgressBarElement{indeterminate: true, lo: 0, hi: 1, width: 240}
}

func (*ProgressBarElement) isElement()           {}
func (p *ProgressBarElement) elementKey() string { return p.key }

func (p *ProgressBarElement) Key(k string) *ProgressBarElement { p.key = k; return p }

// Label names the bar for assistive technology, such as "Upload". Without it
// the bar is named "Progress", or "Loading…" when it is indeterminate.
func (p *ProgressBarElement) Label(name string) *ProgressBarElement { p.name = name; return p }

// Range sets the bounds of the value, lo to hi. A value outside them is clamped.
func (p *ProgressBarElement) Range(lo, hi float64) *ProgressBarElement {
	p.lo, p.hi = lo, hi
	return p
}

// Width sets the width of the bar, in logical pixels, including its readout.
func (p *ProgressBarElement) Width(px float32) *ProgressBarElement { p.width = px; return p }

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
	// The track is drawn as a thin bar. Under high contrast it is an outlined
	// empty box and the fill is solid Highlight, so the empty part does not read
	// as filled, as the demo's bars do.
	track := Div().Height(Px(6)).Grow(1).Radius(3).ClipContent().Background(t.SurfaceActive)
	if t.HighContrast {
		track = Div().Height(Px(8)).Grow(1).Radius(3).ClipContent().Background(t.Surface).Border(1, t.Text)
	}

	name := p.name
	a := track.access()
	track.Role(access.RoleProgressBar)
	a.readOnly = true
	if p.indeterminate {
		if name == "" {
			name = "Loading…"
		}
		track.Child(p.segment(ctx, t))
	} else {
		if name == "" {
			name = "Progress"
		}
		// The value is read-only: the range has no setter, so no SetValue action.
		a.rng = &access.Range{Value: p.clamped(), Min: min(p.lo, p.hi), Max: max(p.lo, p.hi)}
		track.Child(p.fill(t))
	}
	track.Label(name)

	root := Div().Row().Align(Center).Gap(12).Width(Px(p.width)).Child(track)
	if !p.indeterminate {
		// The value is what assistive technology reads through the range, so the
		// readout is hidden from it.
		root.Child(Div().Hidden().MinWidth(Px(44)).Align(End).
			Child(Text(p.readout()).Size(13).Medium().Color(t.TextMuted)))
	}
	return root
}

// fill is the filled part of a determinate bar.
func (p *ProgressBarElement) fill(t *Theme) *DivElement {
	f := float32(p.fraction())
	fill := Div().Height(Full).Width(Percent(f * 100)).Background(p.fillColor(t))
	if !t.HighContrast {
		fill.Radius(3)
	}
	return fill
}

// segment is the sliding part of an indeterminate bar. It moves from one end
// to the other and repeats. With reduced motion it stays in the middle.
func (p *ProgressBarElement) segment(ctx *Context, t *Theme) Element {
	row := func(x float32) *DivElement {
		seg := Div().Height(Full).Grow(progressSegment).Background(p.fillColor(t))
		if !t.HighContrast {
			seg.Radius(3)
		}
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
	size float32
	name string
	key  string
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

// ringDots is the number of dots in a ring.
const ringDots = 8

func (r *ProgressRingElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	name := r.name
	if name == "" {
		name = "Loading…"
	}
	// ring is the box of the indicator. It is one node with the busy role, and
	// its dots are not exposed.
	ring := func() *DivElement {
		return Div().Size(Px(r.size), Px(r.size)).Shrink(0).Role(access.RoleProgressBar).Label(name)
	}
	if ctx.ReducedMotion() {
		// Reduced motion: every dot is drawn, with no turning. Under high contrast
		// the dots are solid; otherwise they are dimmed, so the ring reads as
		// busy without moving.
		box := ring()
		for i := range ringDots {
			if t.HighContrast {
				box.Child(r.dot(i, t.Text, 1))
			} else {
				box.Child(r.dot(i, t.Accent, 0.35))
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
					box.Child(r.dot(i, t.Text, 1))
				} else {
					box.Child(r.dot(i, t.Accent, 1-2*phase))
				}
			}
			return box
		})
}

// dot is the i-th dot of a ring. It sits on the circle at an angle of i/8 of a
// turn, starting at the top, with the given opacity.
func (r *ProgressRingElement) dot(i int, c Color, opacity float32) *DivElement {
	d := r.size / 4
	radius := r.size/2 - d/2
	angle := 2*math.Pi*float64(i)/ringDots - math.Pi/2
	x := r.size/2 + radius*float32(math.Cos(angle)) - d/2
	y := r.size/2 + radius*float32(math.Sin(angle)) - d/2
	return Div().Absolute().Left(x).Top(y).Size(Px(d), Px(d)).Radius(d / 2).Background(c).Opacity(opacity)
}

// fillColor is the color of the fill: the app's Color, or the theme accent. Under
// high contrast it is the accent (Highlight) whatever the app asked for, so the
// fill keeps the system's color.
func (p *ProgressBarElement) fillColor(t *Theme) Color {
	if t.HighContrast || p.color.IsZero() {
		return t.Accent
	}
	return p.color
}
