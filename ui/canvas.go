package ui

import (
	"math"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
	"github.com/tyler-builds/jamb/internal/vector"
)

// Canvas creates an element that draw paints with shapes, as an HTML canvas
// does: charts, graphs, diagrams and custom widgets. draw receives a Painter
// and the size of the content box in logical pixels, with 0, 0 at its top-left
// and y going down.
//
// The drawing is kept between frames. draw runs again when the component that
// made the canvas renders again, or when the canvas's size, the display scale
// or the zoom changes, so read the signals the drawing depends on in the
// component's render, not only in draw. Shapes are drawn on the CPU, at the
// display's resolution, and text with the text renderer, over the shapes.
//
// The canvas is a Div: size it with Size (300 x 150 by default), handle the
// pointer with OnMouseMove and the others (MouseEvent.LocalX and LocalY are in
// the canvas's coordinates when it has no padding or border), and add children
// to lay elements over the drawing. Assistive technology sees an image; name
// it with Label, such as "Sales by month, rising from 12 to 30".
//
//	ui.Canvas(func(p *ui.Painter, w, h float32) {
//		line := ui.NewPath().MoveTo(0, h).LineTo(w/2, 10).LineTo(w, h/2)
//		p.Stroke(line, ui.StrokeStyle{Width: 2, Color: t.Accent})
//	}).Size(ui.Full, ui.Px(200)).Label("Visits this week")
func Canvas(draw func(p *Painter, w, h float32)) *DivElement {
	d := Div().Size(Px(300), Px(150)).Role(access.RoleImage)
	d.canvas = draw
	return d
}

// Painter draws the shapes and text of a Canvas. Its methods are only valid
// while the canvas's draw function runs.
type Painter struct {
	c     *vector.Canvas
	rt    *Runtime
	k     float32 // physical pixels per logical pixel
	fg    Color
	texts *[]canvasText
}

// TextColor returns the text color the canvas inherits, as text in its place
// would be drawn, to draw shapes that match the text around them.
func (p *Painter) TextColor() Color { return p.fg }

// canvasText is text a Painter placed, drawn over the canvas's image.
type canvasText struct {
	x, y  float32 // the top-left of the text, logical pixels from the canvas's
	l     *text.Layout
	color Color
}

// Paint is what fills or strokes a shape: a Color, or a gradient made with
// LinearPaint or RadialPaint.
type Paint interface {
	vectorPaint() vector.Paint
}

func (c Color) vectorPaint() vector.Paint {
	return vector.Paint{Color: vector.Color{R: c.R, G: c.G, B: c.B, A: c.A}}
}

// ColorStop is a color at At (0 to 1) along a gradient.
type ColorStop struct {
	At    float32
	Color Color
}

// GradientPaint is a gradient that fills or strokes shapes in a Canvas, in
// the canvas's coordinates.
type GradientPaint struct{ g vector.Gradient }

func (g *GradientPaint) vectorPaint() vector.Paint {
	gg := g.g
	return vector.Paint{Gradient: &gg}
}

// LinearPaint returns a gradient along the line from x0, y0 to x1, y1, with
// the colors of stops; the end colors continue past the ends.
func LinearPaint(x0, y0, x1, y1 float32, stops ...ColorStop) *GradientPaint {
	return &GradientPaint{vector.Gradient{Kind: vector.Linear, X0: x0, Y0: y0, X1: x1, Y1: y1, Stops: vectorStops(stops), Matrix: vector.Identity}}
}

// RadialPaint returns a gradient from the center cx, cy (the first stop) to
// the circle of radius r (the last stop).
func RadialPaint(cx, cy, r float32, stops ...ColorStop) *GradientPaint {
	return &GradientPaint{vector.Gradient{Kind: vector.Radial, CX: cx, CY: cy, FX: cx, FY: cy, R: r, Stops: vectorStops(stops), Matrix: vector.Identity}}
}

// Focus moves the point a radial gradient starts from off its center, as a
// highlight on a sphere. It does nothing to a linear gradient.
func (g *GradientPaint) Focus(x, y float32) *GradientPaint { g.g.FX, g.g.FY = x, y; return g }

func vectorStops(stops []ColorStop) []vector.Stop {
	out := make([]vector.Stop, len(stops))
	for i, s := range stops {
		out[i] = vector.Stop{Offset: s.At, Color: vector.Color{R: s.Color.R, G: s.Color.G, B: s.Color.B, A: s.Color.A}}
	}
	return out
}

// Path is a shape for a Canvas: lines, curves and arcs, in one or more
// subpaths. Its methods return the path, so they chain.
type Path struct {
	v       vector.Path
	evenOdd bool
}

// NewPath returns an empty path.
func NewPath() *Path { return &Path{} }

// MoveTo starts a new subpath at x, y.
func (p *Path) MoveTo(x, y float32) *Path { p.v.MoveTo(x, y); return p }

// LineTo adds a straight line to x, y.
func (p *Path) LineTo(x, y float32) *Path { p.v.LineTo(x, y); return p }

// QuadTo adds a quadratic Bézier curve through the control point cx, cy to
// x, y.
func (p *Path) QuadTo(cx, cy, x, y float32) *Path { p.v.QuadTo(cx, cy, x, y); return p }

// CubicTo adds a cubic Bézier curve with the control points c1 and c2 to x, y.
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float32) *Path {
	p.v.CubicTo(c1x, c1y, c2x, c2y, x, y)
	return p
}

// Arc adds an arc of the circle of radius r about cx, cy, from the angle start
// to end in radians: 0 points right, and angles grow clockwise. It runs
// clockwise when end is greater than start, counterclockwise when it is less,
// and a sweep of a full turn draws the whole circle. A line joins the end of
// the path so far to the arc's start.
func (p *Path) Arc(cx, cy, r, start, end float32) *Path {
	p.v.Arc(cx, cy, r, start, end, end < start)
	return p
}

// ArcTo adds an elliptical arc to x, y, as SVG's arc command: radii rx and ry,
// the ellipse rotated by rotation degrees, and large and sweep picking one of
// the four arcs that join the points (sweep is clockwise).
func (p *Path) ArcTo(rx, ry, rotation float32, large, sweep bool, x, y float32) *Path {
	p.v.ArcTo(rx, ry, rotation, large, sweep, x, y)
	return p
}

// Close closes the subpath with a line back to its start.
func (p *Path) Close() *Path { p.v.Close(); return p }

// Rect adds a rectangle.
func (p *Path) Rect(x, y, w, h float32) *Path { p.v.Rect(x, y, w, h); return p }

// RoundRect adds a rectangle with corners rounded to radius r.
func (p *Path) RoundRect(x, y, w, h, r float32) *Path { p.v.RoundRect(x, y, w, h, r, r); return p }

// Ellipse adds an ellipse about cx, cy.
func (p *Path) Ellipse(cx, cy, rx, ry float32) *Path { p.v.Ellipse(cx, cy, rx, ry); return p }

// Circle adds a circle about cx, cy.
func (p *Path) Circle(cx, cy, r float32) *Path { p.v.Ellipse(cx, cy, r, r); return p }

// EvenOdd fills the path with the even-odd rule: where subpaths overlap, or
// one lies inside another, they make a hole. By default (nonzero) a subpath
// inside another is filled too, unless it runs the other way round.
func (p *Path) EvenOdd() *Path { p.evenOdd = true; return p }

func (p *Path) rule() vector.FillRule {
	if p.evenOdd {
		return vector.EvenOdd
	}
	return vector.NonZero
}

// LineCap is the shape of the ends of an open stroke.
type LineCap uint8

const (
	CapButt   LineCap = iota // the stroke stops at the end of the path
	CapRound                 // a half circle past the end
	CapSquare                // a half square past the end
)

// LineJoin is the shape of a stroke's corners.
type LineJoin uint8

const (
	JoinMiter LineJoin = iota // sharp, cut flat past the miter limit
	JoinRound
	JoinBevel
)

// StrokeStyle is how Painter.Stroke draws a path's outline.
type StrokeStyle struct {
	Width float32 // 0 means 1
	Color Color
	// Paint, when set, paints the stroke in place of Color, such as with a
	// gradient.
	Paint Paint
	Cap   LineCap
	Join  LineJoin
	// MiterLimit is the longest a miter join gets, as a multiple of Width,
	// before it is cut flat; 0 means 4.
	MiterLimit float32
	// Dash alternates dash and gap lengths, such as {4, 2}; empty is solid.
	// DashOffset starts the pattern that far in.
	Dash       []float32
	DashOffset float32
}

// TextStyle is how Painter.Text draws text.
type TextStyle struct {
	Size   float32 // 0 means the theme's text size
	Color  Color   // zero means the inherited text color (see Painter.TextColor)
	Weight FontWeight
	// Align places the text horizontally against x: Start puts its left edge
	// there, Center its middle, End its right edge. VAlign does the same
	// vertically against y, with Start its top.
	Align, VAlign Alignment
}

// Fill fills path with paint.
func (p *Painter) Fill(path *Path, paint Paint) {
	if path == nil || paint == nil {
		return
	}
	p.c.Fill(&path.v, paint.vectorPaint(), path.rule())
}

// FillRect fills a rectangle with paint.
func (p *Painter) FillRect(x, y, w, h float32, paint Paint) {
	p.Fill(NewPath().Rect(x, y, w, h), paint)
}

// Stroke draws the outline of path with s.
func (p *Painter) Stroke(path *Path, s StrokeStyle) {
	if path == nil {
		return
	}
	var paint vector.Paint
	if s.Paint != nil {
		paint = s.Paint.vectorPaint()
	} else {
		paint = s.Color.vectorPaint()
	}
	w := s.Width
	if w == 0 {
		w = 1
	}
	p.c.Stroke(&path.v, paint, vector.Stroke{Width: w, Cap: vector.Cap(s.Cap), Join: vector.Join(s.Join),
		MiterLimit: s.MiterLimit, Dash: s.Dash, DashOffset: s.DashOffset})
}

// Save remembers the transform, clip and alpha, which Restore brings back.
func (p *Painter) Save()    { p.c.Save() }
func (p *Painter) Restore() { p.c.Restore() }

// Translate moves the origin of later drawing by x, y.
func (p *Painter) Translate(x, y float32) { p.c.Transform(vector.Translate(x, y)) }

// Scale scales later drawing by sx, sy about the origin; stroke widths scale
// too.
func (p *Painter) Scale(sx, sy float32) { p.c.Transform(vector.Scale(sx, sy)) }

// Rotate turns later drawing by angle radians, clockwise, about the origin.
// Text is placed with the rotation but is not drawn rotated.
func (p *Painter) Rotate(angle float32) { p.c.Transform(vector.Rotate(angle)) }

// Clip limits later shapes to the inside of path, until Restore. Text is not
// clipped.
func (p *Painter) Clip(path *Path) {
	if path != nil {
		p.c.Clip(&path.v, path.rule())
	}
}

// Alpha fades later drawing by a, from 0 to 1, until Restore. It multiplies
// the alpha already set.
func (p *Painter) Alpha(a float32) { p.c.SetAlpha(a) }

// Text draws s at x, y, aligned against it as st says, over the shapes. It is
// one line; a line break starts another. Its position follows the transform;
// the text is not scaled or rotated.
func (p *Painter) Text(x, y float32, s string, st TextStyle) {
	l := p.layout(s, st)
	pt := p.c.Matrix().Apply(vector.Point{X: x, Y: y})
	x, y = pt.X/p.k, pt.Y/p.k
	x -= alignShift(st.Align, l.Width)
	y -= alignShift(st.VAlign, l.Height)
	c := st.Color
	if c.IsZero() {
		c = p.fg
	}
	*p.texts = append(*p.texts, canvasText{x: x, y: y, l: l, color: c})
}

// MeasureText returns the size of s drawn with st, to place labels.
func (p *Painter) MeasureText(s string, st TextStyle) (w, h float32) {
	l := p.layout(s, st)
	return l.Width, l.Height
}

func (p *Painter) layout(s string, st TextStyle) *text.Layout {
	ts := text.Style{Size: st.Size, Weight: st.Weight, NoWrap: true}
	if ts.Size == 0 {
		ts.Size = p.rt.theme.FontSize
	}
	ts.Size *= p.rt.textFactor // the Text size setting
	if ts.Weight == 0 {
		ts.Weight = text.Regular
	}
	return p.rt.text.Layout(s, ts, float32(math.Inf(1)))
}

// alignShift is how far text of width w moves back from the point it is
// aligned against: none at Start, half at Center, all at End.
func alignShift(a Alignment, w float32) float32 {
	switch a {
	case Center:
		return w / 2
	case End:
		return w
	}
	return 0
}

// canvasState is what a canvas keeps between frames: its drawing, and what it
// was drawn for.
type canvasState struct {
	vc     *vector.Canvas
	tex    *scene.Texture
	el     *DivElement // the element the drawing is of; a new one draws again
	key    any         // or, when the element has a canvasKey, that key
	fg     Color       // the inherited text color it was drawn with
	pw, ph int         // the drawing's size in physical pixels
	texts  []canvasText
}

// paintCanvas draws a canvas div's drawing in its content box, at x, y of
// size w x h, drawing it again first when needed. fg is the inherited text
// color.
func (rt *Runtime) paintCanvas(n *node, x, y, w, h float32, fg Color, opacity float32) {
	s := &n.style
	be := s.borderEdges()
	box := scene.Rect{X: x + be.Left + s.Padding.Left, Y: y + be.Top + s.Padding.Top}
	box.W = w - be.Left - be.Right - s.Padding.Left - s.Padding.Right
	box.H = h - be.Top - be.Bottom - s.Padding.Top - s.Padding.Bottom
	if box.W <= 0 || box.H <= 0 {
		return
	}
	dst := rt.px(box)
	pw, ph := int(dst.W), int(dst.H)
	if pw <= 0 || ph <= 0 {
		return
	}
	st := rt.state(n.id)
	cs := st.canvas
	if cs == nil {
		cs = &canvasState{vc: vector.NewCanvas(pw, ph)}
		st.canvas = cs
	}
	changed := cs.el != n.div
	if key := n.div.canvasKey; key != nil {
		changed = cs.key != key
	}
	if changed || cs.pw != pw || cs.ph != ph || cs.fg != fg {
		cs.el, cs.key, cs.fg, cs.pw, cs.ph = n.div, n.div.canvasKey, fg, pw, ph
		cs.texts = cs.texts[:0]
		cs.vc.Reset(pw, ph)
		k := float32(pw) / box.W
		cs.vc.SetTransform(vector.Scale(k, float32(ph)/box.H))
		n.div.canvas(&Painter{c: cs.vc, rt: rt, k: k, fg: fg, texts: &cs.texts}, box.W, box.H)
		img := cs.vc.Image()
		if cs.tex == nil || cs.tex.Width != pw || cs.tex.Height != ph {
			// The texture keeps its ID through size changes, so the renderer
			// replaces its GPU copy rather than keeping one for each size.
			t := scene.NewTexture(pw, ph, scene.FormatRGBA8)
			if cs.tex != nil {
				t.ID, t.Version = cs.tex.ID, cs.tex.Version+1
			}
			cs.tex = t
		}
		copy(cs.tex.Pix, img.Pix)
		cs.tex.MarkDirty(0, 0, pw, ph)
	}
	rt.scene.AddImage(scene.ImageDraw{Bounds: dst, Texture: cs.tex, Opacity: opacity})
	if len(cs.texts) == 0 {
		return
	}
	rt.scene.PushClip(dst)
	for _, t := range cs.texts {
		rt.text.Draw(&rt.scene, t.l, box.X+t.x, box.Y+t.y, 0, rt.scale, 0, t.color.Alpha(opacity).scene())
	}
	rt.scene.PopClip()
}
