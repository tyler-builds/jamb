package ui

import (
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
	"github.com/tyler-builds/jamb/internal/text"
)

// Length is a size: Px(240), Percent(100) or Auto.
type Length = layout.Length

// Px returns a length in logical pixels.
func Px(v float32) Length { return layout.Px(v) }

// Percent returns a length relative to the parent's content box.
func Percent(v float32) Length { return layout.Percent(v) }

// Fr returns a flexible grid track that takes v shares of the space the other
// tracks leave (see DivElement.Grid). Elsewhere it acts as Auto.
func Fr(v float32) Length { return layout.Fr(v) }

// Auto sizes from content (or stretches, depending on alignment).
var Auto = layout.Auto

// Full is Percent(100).
var Full = layout.Percent(100)

// Direction is the main axis of a container.
type Direction uint8

const (
	// Column stacks children top to bottom. It is the default.
	Column Direction = iota
	// Row places children left to right.
	Row
)

// Alignment is used for Align (cross axis), Justify (main axis) and text
// alignment. The zero value means unset.
type Alignment uint8

const (
	// Start aligns to the start edge of the axis.
	Start Alignment = iota + 1
	// Center aligns to the middle of the axis.
	Center
	// End aligns to the end edge of the axis.
	End
	// Stretch fills the cross axis (Align only). It is the default for Align.
	Stretch
	// SpaceBetween, SpaceAround and SpaceEvenly apply to Justify only.
	SpaceBetween
	// SpaceAround spreads the children with equal space around each one. It applies to Justify only.
	SpaceAround
	// SpaceEvenly spreads the children with equal space between and around them. It applies to Justify only.
	SpaceEvenly
)

// Overflow controls clipping and scrolling.
type Overflow uint8

const (
	// OverflowVisible draws the content that goes past the element's box.
	OverflowVisible Overflow = iota
	// OverflowHidden clips the content at the element's box.
	OverflowHidden
	// OverflowScroll clips the content at the element's box and scrolls it.
	OverflowScroll
)

// FontWeight is a font weight on the 100–900 scale.
type FontWeight = text.Weight

const (
	// FontLight is the weight 300.
	FontLight = text.Light
	// FontRegular is the weight 400, the normal weight.
	FontRegular = text.Regular
	// FontMedium is the weight 500.
	FontMedium = text.Medium
	// FontSemibold is the weight 600.
	FontSemibold = text.Semibold
	// FontBold is the weight 700.
	FontBold = text.Bold
)

// Cursor is the mouse cursor shown over an element.
type Cursor = input.Cursor

const (
	// CursorDefault is the arrow cursor.
	CursorDefault = input.CursorDefault
	// CursorPointer is the hand cursor, for a link or a button.
	CursorPointer = input.CursorPointer
	// CursorText is the I-beam cursor, for text.
	CursorText = input.CursorText
	// CursorNotAllowed is the cursor for an action that cannot be done.
	CursorNotAllowed = input.CursorNotAllowed
)

// Edges are per-side insets in logical pixels. Inside a Style, a zero side
// means unset.
type Edges struct {
	// Top is the inset of the top side, in logical pixels.
	Top float32
	// Right is the inset of the right side, in logical pixels.
	Right float32
	// Bottom is the inset of the bottom side, in logical pixels.
	Bottom float32
	// Left is the inset of the left side, in logical pixels.
	Left float32
}

// All returns equal edges on every side.
func All(v float32) Edges { return Edges{v, v, v, v} }

// XY returns edges with horizontal x and vertical y.
func XY(x, y float32) Edges { return Edges{y, x, y, x} }

// Corners holds per-corner radii.
type Corners struct {
	// TopLeft is the radius of the top left corner, in logical pixels.
	TopLeft float32
	// TopRight is the radius of the top right corner, in logical pixels.
	TopRight float32
	// BottomRight is the radius of the bottom right corner, in logical pixels.
	BottomRight float32
	// BottomLeft is the radius of the bottom left corner, in logical pixels.
	BottomLeft float32
}

func (c Corners) isZero() bool { return c == Corners{} }

func (e Edges) isZero() bool { return e == Edges{} }

// Shadow is a drop shadow.
type Shadow struct {
	// X and Y are the offset of the shadow, in logical pixels.
	X, Y float32
	Blur float32 // blur radius in logical pixels (≈ 2× the Gaussian sigma)
	// Spread grows the shadow, or shrinks it when it is negative, in logical pixels.
	Spread float32
	// Color is the color of the shadow.
	Color Color
}

// Ready-made shadows that read well on both light and dark backgrounds.
var (
	ShadowSm = Shadow{Y: 1, Blur: 3, Color: Color{0, 0, 0, 0.18}}
	ShadowMd = Shadow{Y: 4, Blur: 12, Color: Color{0, 0, 0, 0.22}}
	ShadowLg = Shadow{Y: 12, Blur: 32, Color: Color{0, 0, 0, 0.30}}
)

// Style is plain typed data describing how an element looks and lays out.
// Zero-valued fields are "unset": merging a Style into an element only
// changes the fields that are set. Define reusable styles as variables:
//
//	var Card = ui.Style{Padding: ui.All(16), Radius: 12, Background: ui.Hex("#18181b")}
//	ui.Div().Style(Card)
type Style struct {
	// The fields are ordered so that the struct has no padding: the 4-byte fields are
	// together, and so are the bytes, which keeps Style at 448 bytes, a malloc size
	// class (see TestElementSizeClasses). The order is not part of the API.

	// Visuals
	Background Color
	// BorderColor is the color of the border.
	BorderColor Color
	// TextColor is the color of the text, inherited by the descendants.
	TextColor Color
	Gradient  Gradient // replaces Background when set (see LinearGradient)
	// Corners sets a radius for each corner, in place of Radius.
	Corners Corners

	// BorderWidth is the width of the border on every side, in logical pixels.
	BorderWidth float32
	// Radius is the radius of the rounded corners, in logical pixels.
	Radius float32
	// Gap is the space between the children, in logical pixels.
	Gap float32
	// Grow is the share of the free space that the element takes on its parent's main axis.
	Grow float32
	// Shrink is how much the element shrinks when the space is short.
	Shrink  float32
	Opacity float32 // 0 = unset (opaque)
	// Paint-only motion, applied after layout (see Offset and Scale). Layout
	// and hit testing of the element's position follow the offset.
	OffsetX, OffsetY float32
	Scale            float32 // 0 = unset (1), about the element's center
	// FontSize is the font size in logical pixels, inherited by the descendants.
	FontSize float32
	// LineHeight is the distance between lines of text, in logical pixels.
	LineHeight float32

	// BorderEdges, when not zero, gives each side its own width (top, right,
	// bottom, left) in place of BorderWidth. See BorderEdges.
	BorderEdges Edges
	// Padding is the space between the border and the content, on each side.
	Padding Edges
	// Margin is the space outside the border, on each side.
	Margin Edges
	// Inset holds the offsets of an Absolute element from the edges of its parent.
	Inset Edges

	// Width is the width of the element's border box.
	Width Length
	// Height is the height of the element's border box.
	Height Length
	// MinWidth is the smallest width that the element can take.
	MinWidth Length
	// MinHeight is the smallest height that the element can take.
	MinHeight Length
	// MaxWidth is the largest width that the element can take.
	MaxWidth Length
	// MaxHeight is the largest height that the element can take.
	MaxHeight Length
	// Basis is the starting main size before growing/shrinking (Auto = content).
	Basis Length

	// Layout
	Direction Direction
	// Wrap flows children onto more lines when a row (or column) is full.
	Wrap bool
	// NoShrink stops the element from shrinking below its size (Shrink 0).
	NoShrink bool
	// fitWidth keeps an automatic width at the content's width in a column, where
	// the element would otherwise stretch to the column's width (see layout.Style).
	fitWidth bool
	// contentMin lets the element shrink below its fixed width to its content's
	// min-content width (see layout.Style.ContentMin); text fields use it.
	contentMin bool
	// softWidth marks Width as a widget's default, which a grid cell overrides
	// (see layout.Style.SoftWidth); sizing sets it when the app gave no width.
	softWidth bool
	Align     Alignment // children on the cross axis
	// AlignSelf overrides the parent's Align for this element.
	AlignSelf Alignment
	Justify   Alignment // children on the main axis
	// Absolute removes the element from the flow; position it with Inset.
	Absolute bool
	insetSet [4]bool
	// OverflowX is how the content that goes past the element's width is drawn.
	OverflowX Overflow
	// OverflowY is how the content that goes past the element's height is drawn.
	OverflowY Overflow
	// Cursor is the mouse cursor shown over the element.
	Cursor Cursor
	// TextAlign is the horizontal alignment of the text.
	TextAlign Alignment
	// FontWeight is the weight of the text, inherited by the descendants.
	FontWeight FontWeight
	// Clear removes the fields it names from the style it is merged into, before
	// the set fields of this style are applied, so a style can turn a default off:
	// the zero value of a field means unset and cannot replace a default by itself.
	// The clears are kept when styles are merged, so a style merged in later, such
	// as an instance's Styles, still clears the defaults it names. See ClearSet.
	Clear ClearSet

	// Transition eases colors, opacity, offset and scale (hover, press, theme,
	// and changes made by a new render) over this duration.
	Transition time.Duration
	// Spring moves opacity, offset and scale as a damped spring instead: a
	// change mid-flight keeps the velocity it had. Colors still ease, over
	// Transition when it is set. The zero Spring is unset.
	Spring Spring
	// ZIndex orders overlapping siblings; a higher value is drawn on top.
	ZIndex int
	// ColSpan and RowSpan are the number of grid columns and rows that the item covers; zero means one.
	ColSpan, RowSpan int
	// GridColumns makes the element a grid (see DivElement.Grid), with one
	// track per column; GridRows sizes its first rows, and the others fit their
	// content. ColSpan and RowSpan make a grid item cover that many columns and
	// rows; 0 means 1.
	GridColumns, GridRows []Length
	// Shadows are the drop shadows of the element, drawn in order.
	Shadows []Shadow
	// FontFamily is the name of the font family, inherited by the descendants.
	FontFamily string

	// prov records which layers set the fields, for the visual inspector. It is
	// nil unless an inspector is attached (see style_layers.go).
	prov styleProv
}

// ClearSet names the fields that a style removes with its Clear field. Each
// constant clears the fields listed beside it, before the rest of the style is
// applied, so a clear and a value for the same field give the value.
type ClearSet uint16

const (
	ClearBackground ClearSet = 1 << iota // Background and Gradient: no fill
	ClearTextColor                       // TextColor: text and icons inherit the color above
	ClearBorder                          // BorderWidth, BorderEdges and BorderColor: no border
	ClearRadius                          // Radius and Corners: square corners
	ClearShadows                         // Shadows: no shadow, and no focus ring of this style
	ClearPadding                         // Padding: no padding
	ClearMargin                          // Margin: no margin
	ClearGap                             // Gap: no space between children
)

// cleared returns s without the fields that c names.
func (s Style) cleared(c ClearSet) Style {
	if c&ClearBackground != 0 {
		s.Background, s.Gradient = Color{}, Gradient{}
	}
	if c&ClearTextColor != 0 {
		s.TextColor = Color{}
	}
	if c&ClearBorder != 0 {
		s.BorderWidth, s.BorderEdges, s.BorderColor = 0, Edges{}, Color{}
	}
	if c&ClearRadius != 0 {
		s.Radius, s.Corners = 0, Corners{}
	}
	if c&ClearShadows != 0 {
		s.Shadows = nil
	}
	if c&ClearPadding != 0 {
		s.Padding = Edges{}
	}
	if c&ClearMargin != 0 {
		s.Margin = Edges{}
	}
	if c&ClearGap != 0 {
		s.Gap = 0
	}
	return s
}

// Merge returns s with every set field of o applied on top. The fields that o
// clears (see Style.Clear) are removed first. While an inspector is attached,
// the layers that set the fields are merged with them (see style_layers.go).
func (s Style) Merge(o Style) Style {
	sp := s.prov
	s = s.cleared(o.Clear)
	s.Clear |= o.Clear
	if o.Direction != Column {
		s.Direction = o.Direction
	}
	mergeLen(&s.Width, o.Width)
	mergeLen(&s.Height, o.Height)
	mergeLen(&s.MinWidth, o.MinWidth)
	mergeLen(&s.MinHeight, o.MinHeight)
	mergeLen(&s.MaxWidth, o.MaxWidth)
	mergeLen(&s.MaxHeight, o.MaxHeight)
	mergeEdges(&s.Padding, o.Padding)
	mergeEdges(&s.Margin, o.Margin)
	mergeF(&s.Gap, o.Gap)
	s.Wrap = s.Wrap || o.Wrap
	mergeF(&s.Grow, o.Grow)
	mergeF(&s.Shrink, o.Shrink)
	mergeLen(&s.Basis, o.Basis)
	if o.GridColumns != nil {
		s.GridColumns = o.GridColumns
	}
	if o.GridRows != nil {
		s.GridRows = o.GridRows
	}
	if o.ColSpan != 0 {
		s.ColSpan = o.ColSpan
	}
	if o.RowSpan != 0 {
		s.RowSpan = o.RowSpan
	}
	s.NoShrink = s.NoShrink || o.NoShrink
	s.fitWidth = s.fitWidth || o.fitWidth
	s.contentMin = s.contentMin || o.contentMin
	mergeA(&s.Align, o.Align)
	mergeA(&s.AlignSelf, o.AlignSelf)
	mergeA(&s.Justify, o.Justify)
	s.Absolute = s.Absolute || o.Absolute
	for i, set := range o.insetSet {
		if set {
			s.insetSet[i] = true
			*s.Inset.side(i) = *o.Inset.side(i)
		}
	}
	if o.Inset != (Edges{}) {
		for i := 0; i < 4; i++ {
			if v := *o.Inset.side(i); v != 0 {
				*s.Inset.side(i) = v
				s.insetSet[i] = true
			}
		}
	}
	if o.OverflowX != OverflowVisible {
		s.OverflowX = o.OverflowX
	}
	if o.OverflowY != OverflowVisible {
		s.OverflowY = o.OverflowY
	}
	if o.ZIndex != 0 {
		s.ZIndex = o.ZIndex
	}
	mergeC(&s.Background, o.Background)
	if !o.Gradient.isZero() {
		s.Gradient = o.Gradient
	}
	mergeC(&s.BorderColor, o.BorderColor)
	if o.BorderWidth != 0 {
		s.BorderWidth, s.BorderEdges = o.BorderWidth, Edges{}
	}
	if !o.BorderEdges.isZero() {
		s.BorderEdges = o.BorderEdges
	}
	mergeF(&s.Radius, o.Radius)
	if !o.Corners.isZero() {
		s.Corners = o.Corners
	}
	if o.Shadows != nil {
		s.Shadows = o.Shadows
	}
	mergeF(&s.Opacity, o.Opacity)
	mergeF(&s.OffsetX, o.OffsetX)
	mergeF(&s.OffsetY, o.OffsetY)
	mergeF(&s.Scale, o.Scale)
	if o.Cursor != CursorDefault {
		s.Cursor = o.Cursor
	}
	mergeC(&s.TextColor, o.TextColor)
	mergeF(&s.FontSize, o.FontSize)
	if o.FontWeight != 0 {
		s.FontWeight = o.FontWeight
	}
	if o.FontFamily != "" {
		s.FontFamily = o.FontFamily
	}
	mergeF(&s.LineHeight, o.LineHeight)
	mergeA(&s.TextAlign, o.TextAlign)
	if o.Transition != 0 {
		s.Transition = o.Transition
	}
	if o.Spring.Duration != 0 {
		s.Spring = o.Spring
	}
	if sp != nil || o.prov != nil {
		s.prov = mergeProv(sp, o.prov, s)
	}
	return s
}

func (e *Edges) side(i int) *float32 {
	switch i {
	case 0:
		return &e.Top
	case 1:
		return &e.Right
	case 2:
		return &e.Bottom
	}
	return &e.Left
}

func mergeLen(dst *Length, v Length) {
	if !v.IsAuto() {
		*dst = v
	}
}

func mergeEdges(dst *Edges, v Edges) {
	mergeF(&dst.Top, v.Top)
	mergeF(&dst.Right, v.Right)
	mergeF(&dst.Bottom, v.Bottom)
	mergeF(&dst.Left, v.Left)
}

func mergeF(dst *float32, v float32) {
	if v != 0 {
		*dst = v
	}
}

func mergeA(dst *Alignment, v Alignment) {
	if v != 0 {
		*dst = v
	}
}

func mergeC(dst *Color, v Color) {
	if !v.IsZero() {
		*dst = v
	}
}

// layoutStyle converts to the layout engine's representation.
func (s *Style) layoutStyle(shrinkDefault float32) layout.Style {
	ls := layout.Style{
		Width: s.Width, Height: s.Height,
		MinWidth: s.MinWidth, MinHeight: s.MinHeight,
		MaxWidth: s.MaxWidth, MaxHeight: s.MaxHeight,
		Padding: layout.Edges(s.Padding),
		Margin:  layout.Edges(s.Margin),
		Border:  layout.Edges(s.borderEdges()),
		Gap:     s.Gap,
		Wrap:    s.Wrap,
		Grow:    s.Grow,
		Basis:   s.Basis,
		Shrink:  shrinkDefault,
		Justify: justify(s.Justify),
	}
	if s.Direction == Row {
		ls.Direction = layout.Row
	}
	if s.Shrink != 0 {
		ls.Shrink = s.Shrink
	}
	if s.NoShrink {
		ls.Shrink = 0
	}
	ls.AlignItems = align(s.Align)
	ls.AlignSelf = align(s.AlignSelf)
	ls.FitWidth = s.fitWidth
	ls.ContentMin = s.contentMin
	ls.SoftWidth = s.softWidth
	if s.Absolute {
		ls.Position = layout.Absolute
		in := [4]*Length{&ls.Top, &ls.Right, &ls.Bottom, &ls.Left}
		for i, set := range s.insetSet {
			if set {
				*in[i] = layout.Px(*s.Inset.side(i))
			}
		}
	}
	ls.OverflowX = layout.Overflow(s.OverflowX)
	ls.OverflowY = layout.Overflow(s.OverflowY)
	if len(s.GridColumns) > 0 {
		ls.Grid = &layout.Grid{Columns: s.GridColumns, Rows: s.GridRows}
	}
	ls.ColSpan, ls.RowSpan = s.ColSpan, s.RowSpan
	return ls
}

func align(a Alignment) layout.Align {
	switch a {
	case Start:
		return layout.AlignStart
	case Center:
		return layout.AlignCenter
	case End:
		return layout.AlignEnd
	case Stretch:
		return layout.AlignStretch
	}
	return layout.AlignAuto
}

func justify(a Alignment) layout.Justify {
	switch a {
	case Center:
		return layout.JustifyCenter
	case End:
		return layout.JustifyEnd
	case SpaceBetween:
		return layout.JustifySpaceBetween
	case SpaceAround:
		return layout.JustifySpaceAround
	case SpaceEvenly:
		return layout.JustifySpaceEvenly
	}
	return layout.JustifyStart
}

// borderEdges returns the width of each border side: BorderEdges when set,
// else BorderWidth on every side.
func (s *Style) borderEdges() Edges {
	if !s.BorderEdges.isZero() {
		return s.BorderEdges
	}
	return All(s.BorderWidth)
}

func (s *Style) radii() Corners {
	if !s.Corners.isZero() {
		return s.Corners
	}
	return Corners{s.Radius, s.Radius, s.Radius, s.Radius}
}
