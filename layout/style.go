// Package layout is a small flexbox-style layout engine.
//
// It implements the useful core of the flexbox mental model — rows and
// columns, gap, padding, margin, fixed/percent/auto sizes with min/max,
// grow/shrink, align/justify, absolute positioning and scroll content
// extents — without any CSS parsing or browser compatibility goals.
//
// Sizes are border-box: Width includes padding and border.
// The engine knows nothing about drawing, text or events; leaves report their
// content size through the Measurer interface.
package layout

import "math"

// Unit is the unit of a Length.
type Unit uint8

const (
	// Auto sizes from content (or stretches, depending on alignment).
	UnitAuto Unit = iota
	UnitPx
	// UnitPercent is relative to the containing block's content box.
	UnitPercent
)

// Length is a size or offset.
type Length struct {
	Value float32
	Unit  Unit
}

// Auto is the automatic length.
var Auto = Length{}

// Px returns a length in logical pixels.
func Px(v float32) Length { return Length{v, UnitPx} }

// Percent returns a percentage length (100 = full containing block).
func Percent(v float32) Length { return Length{v, UnitPercent} }

// IsAuto reports whether l is automatic.
func (l Length) IsAuto() bool { return l.Unit == UnitAuto }

// resolve returns the length in pixels, or NaN if it is auto or a
// percentage of an indefinite size.
func (l Length) resolve(base float32) float32 {
	switch l.Unit {
	case UnitPx:
		return l.Value
	case UnitPercent:
		if isNaN(base) {
			return nan
		}
		return base * l.Value / 100
	}
	return nan
}

// Direction is the main axis of a container.
type Direction uint8

const (
	Column Direction = iota // children stack top to bottom (the default)
	Row                     // children flow left to right
)

// Align positions items on the cross axis.
type Align uint8

const (
	AlignAuto Align = iota // inherit from the container (AlignSelf) / stretch (AlignItems)
	AlignStart
	AlignCenter
	AlignEnd
	AlignStretch
)

// Justify distributes items on the main axis.
type Justify uint8

const (
	JustifyStart Justify = iota
	JustifyCenter
	JustifyEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

// Position selects in-flow or absolute positioning.
type Position uint8

const (
	Relative Position = iota
	Absolute
	// Floating is laid out like Absolute but sized against the whole viewport, ignores insets, and is excluded from the parent's content extent; the caller positions it.
	Floating
)

// Overflow controls clipping and scrolling.
type Overflow uint8

const (
	OverflowVisible Overflow = iota
	OverflowHidden
	OverflowScroll
)

// Edges are per-side insets.
type Edges struct{ Top, Right, Bottom, Left float32 }

// Style holds everything that affects layout.
type Style struct {
	Direction Direction
	Width     Length
	Height    Length
	MinWidth  Length
	MinHeight Length
	MaxWidth  Length
	MaxHeight Length

	Padding Edges
	Margin  Edges
	// Border is the border width of each side; it insets content like padding.
	Border Edges
	Gap    float32

	// Wrap lets items flow onto further lines when the main axis is full;
	// Gap separates lines as well as items.
	Wrap bool

	Grow   float32
	Shrink float32
	Basis  Length

	AlignItems Align
	AlignSelf  Align
	Justify    Justify

	Position                 Position
	Top, Right, Bottom, Left Length

	// Hidden takes the node out of layout: it takes no space, its subtree is
	// not laid out, and it adds nothing to its parent's content extent or ink.
	// Its own W and H are zero while it is hidden.
	Hidden bool

	OverflowX, OverflowY Overflow
}

// Measurer reports the natural size of a leaf's content (text, images...).
// maxWidth is the available content width; it may be +Inf. Implementations
// should wrap to maxWidth where they can and return the size they need.
type Measurer interface {
	Measure(maxWidth float32) (width, height float32)
}

// MinContenter is implemented by a leaf whose content wraps. MinContentWidth
// returns the min-content width of the content: the width of its widest
// unbreakable run, the narrowest it can be without breaking inside a word (CSS
// min-content size). wraps is false when the content does not wrap at all, such
// as a NoWrap text, or when the leaf has no words to break. Such a leaf is sized
// by Measure(0) alone, as a leaf that does not implement MinContenter is, and it
// is not held to the width of its container.
type MinContenter interface {
	MinContentWidth() (width float32, wraps bool)
}

var (
	nan = float32(math.NaN())
	inf = float32(math.Inf(1))
)

func isNaN(f float32) bool { return f != f }

// usesPercent reports whether any horizontal / vertical size of s is a percentage.
func (s *Style) usesPercent() (w, h bool) {
	p := func(ls ...Length) bool {
		for _, l := range ls {
			if l.Unit == UnitPercent {
				return true
			}
		}
		return false
	}
	return p(s.Width, s.MinWidth, s.MaxWidth, s.Basis), p(s.Height, s.MinHeight, s.MaxHeight, s.Basis)
}
