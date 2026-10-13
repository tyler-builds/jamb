package ui

import (
	"math"
	"time"

	"github.com/tyler-builds/jamb/ease"
)

// SkeletonStyle is the look of each part of a skeleton. A part's style merges over
// the built-in defaults, then over the theme's Skeleton, then over the Styles of
// the instance; see docs/styling.md. Bone is each placeholder shape: its fill, its
// corners and its size. A Circle, or a sizing method, sets the size of a bone over
// its style.
type SkeletonStyle struct {
	Bone Style // each placeholder shape
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SkeletonStyle) merge(o SkeletonStyle) SkeletonStyle {
	return SkeletonStyle{Bone: s.Bone.Merge(o.Bone)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SkeletonStyle) plain() SkeletonStyle {
	return SkeletonStyle{Bone: plainStyle(s.Bone)}
}

// skeletonBarHeight is the height of a bar at the default Text size, in logical
// pixels. A bar stands in for a line of text, so it grows with the Text size.
const skeletonBarHeight = 16

// skeletonGap is the space between the bars of Lines, at the default Text size.
const skeletonGap = 8

// skeletonPeriod is the length of one pulse of a skeleton.
const skeletonPeriod = 1200 * time.Millisecond

// skeletonDefaults returns the built-in look of a placeholder bar at the Text
// scale scale: a rounded block in the surface-active color, a full-width bar of
// text height. Under high contrast the bar is an outlined empty box, since it
// does not pulse.
func skeletonDefaults(t *Theme, scale float32) SkeletonStyle {
	d := SkeletonStyle{Bone: Style{Background: t.SurfaceActive, Radius: 4, NoShrink: true,
		Width: Full, Height: Px(skeletonBarHeight * scale)}}
	if t.HighContrast {
		d.Bone.Background, d.Bone.BorderWidth, d.Bone.BorderColor = t.Surface, 1, t.Text
	}
	return d
}

// resolveSkeleton returns the look of a skeleton: the defaults, merged with the
// theme's Skeleton and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveSkeleton(t *Theme, scale float32, inst SkeletonStyle) SkeletonStyle {
	st := skeletonDefaults(t, scale)
	for _, o := range []SkeletonStyle{t.Skeleton, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// skeletonShape is the form of a skeleton.
type skeletonShape uint8

const (
	skeletonBlock skeletonShape = iota
	skeletonCircle
	skeletonLines
)

// SkeletonElement is a placeholder for content that is still loading: a bar, a
// circle or a set of text lines, which pulses gently. It is hidden from assistive
// technology. The app marks the region that is loading as busy, or labels it
// "Loading", so that assistive technology knows that it is waiting.
type SkeletonElement struct {
	shape    skeletonShape
	diameter float32
	lines    int
	key      string
	styles   SkeletonStyle
	size     sizing // the skeleton's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Skeleton creates a placeholder bar of text height, full width of its container
// by default. See Circle and Lines for other shapes.
func Skeleton() *SkeletonElement { return &SkeletonElement{shape: skeletonBlock} }

func (*SkeletonElement) isElement()           {}
func (s *SkeletonElement) elementKey() string { return s.key }

// Key identifies the skeleton among its siblings (see DivElement.Key).
func (s *SkeletonElement) Key(k string) *SkeletonElement { s.key = k; return s }

// Circle makes the skeleton a circle of the given diameter, in logical pixels at
// the default Text size, such as the placeholder of an Avatar. It grows with the
// Text size, as an Avatar of the same size does.
func (s *SkeletonElement) Circle(diameter float32) *SkeletonElement {
	s.shape, s.diameter = skeletonCircle, diameter
	return s
}

// Lines makes the skeleton n bars of text height, separated by a gap, as the
// placeholder of a paragraph. The last bar is about 60 percent wide. A count below
// one is one bar.
func (s *SkeletonElement) Lines(n int) *SkeletonElement {
	s.shape, s.lines = skeletonLines, max(n, 1)
	return s
}

// The sizing methods give the skeleton's outer box a size. See Sizing in
// docs/styling.md.
func (s *SkeletonElement) Width(l Length) *SkeletonElement    { s.size.setWidth(l); return s }
func (s *SkeletonElement) Height(l Length) *SkeletonElement   { s.size.setHeight(l); return s }
func (s *SkeletonElement) MaxWidth(l Length) *SkeletonElement { s.size.setMaxWidth(l); return s }
func (s *SkeletonElement) Grow(f float32) *SkeletonElement    { s.size.setGrow(f); return s }
func (s *SkeletonElement) GridSpan(cols, rows int) *SkeletonElement {
	s.size.setSpan(cols, rows)
	return s
}

// Styles sets the look of the parts of the skeleton, for this instance only. Set
// fields win over the theme's Skeleton and the defaults; see SkeletonStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *SkeletonElement) Styles(st SkeletonStyle) *SkeletonElement {
	s.styles = s.styles.merge(st)
	return s
}

func (s *SkeletonElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	scale := textScale(ctx)
	st := resolveSkeleton(t, scale, s.styles)
	if ctx.ReducedMotion() || t.HighContrast {
		return s.build(st, scale, 1)
	}
	return Animate("skeleton", Animation{Duration: skeletonPeriod, Repeat: true, Easing: ease.Linear},
		func(p float32) Element {
			// The opacity is 1 at the start of a pulse, falls to 0.5 halfway through, and
			// returns to 1.
			return s.build(st, scale, 0.75+0.25*float32(math.Cos(2*math.Pi*float64(p))))
		})
}

// build returns the placeholder at the given opacity, in the shape of the skeleton.
// Its bones are hidden from assistive technology.
func (s *SkeletonElement) build(st SkeletonStyle, scale, opacity float32) Element {
	bone := func() *DivElement {
		return Div().asPart("Skeleton.Bone").Style(st.Bone).Opacity(opacity).AccessHidden()
	}
	switch s.shape {
	case skeletonCircle:
		d := s.diameter * scale
		return s.size.apply(bone().Size(Px(d), Px(d)).Radius(d / 2))
	case skeletonLines:
		col := Div().Column().Gap(skeletonGap * scale).Width(Full).AccessHidden()
		for i := range s.lines {
			b := bone()
			if s.lines > 1 && i == s.lines-1 {
				b.Width(Percent(60))
			}
			col.Child(b)
		}
		return s.size.apply(col)
	}
	return s.size.apply(bone())
}
