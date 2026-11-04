//go:build !windows

package text

import (
	"image"
	"math"
	"slices"

	"github.com/go-text/typesetting/font/opentype/tables"
	"golang.org/x/image/vector"
)

// The pure-Go rasterizer of COLR color glyphs, used where there is no
// DirectWrite. It draws version 0 layers and the version 1 paint graph:
// layers, solid fills, linear, radial and sweep gradients (pad, repeat and
// reflect), glyph outlines as clips, references to other color glyphs, the
// transforms, and composites. Variable fonts are drawn in their default
// instance: the variation deltas of the Var paints are ignored.
//
// Paints are drawn into a premultiplied RGBA canvas of float32s, in device
// pixels. A transform maps font units (y up) to canvas pixels (y down).

// affine maps (x, y) to (xx·x + xy·y + dx, yx·x + yy·y + dy), as the COLR
// Affine2x3 does.
type affine struct{ xx, yx, xy, yy, dx, dy float64 }

var identity = affine{xx: 1, yy: 1}

// mul returns a∘b: the transform that applies b, then a.
func (a affine) mul(b affine) affine {
	return affine{
		xx: a.xx*b.xx + a.xy*b.yx,
		yx: a.yx*b.xx + a.yy*b.yx,
		xy: a.xx*b.xy + a.xy*b.yy,
		yy: a.yx*b.xy + a.yy*b.yy,
		dx: a.xx*b.dx + a.xy*b.dy + a.dx,
		dy: a.yx*b.dx + a.yy*b.dy + a.dy,
	}
}

func (a affine) apply(x, y float64) (float64, float64) {
	return a.xx*x + a.xy*y + a.dx, a.yx*x + a.yy*y + a.dy
}

func (a affine) invert() (affine, bool) {
	det := a.xx*a.yy - a.xy*a.yx
	if math.Abs(det) < 1e-12 {
		return affine{}, false
	}
	inv := affine{xx: a.yy / det, xy: -a.xy / det, yx: -a.yx / det, yy: a.xx / det}
	inv.dx = -(inv.xx*a.dx + inv.xy*a.dy)
	inv.dy = -(inv.yx*a.dx + inv.yy*a.dy)
	return inv, true
}

func translate(x, y float64) affine { return affine{xx: 1, yy: 1, dx: x, dy: y} }

// around applies t about the point (x, y) rather than the origin.
func around(t affine, x, y float64) affine {
	return translate(x, y).mul(t).mul(translate(-x, -y))
}

// rotation turns counter-clockwise (in font units, y up) by a turns of 180°.
func rotation(a float64) affine {
	s, c := math.Sincos(a * math.Pi)
	return affine{xx: c, yx: s, xy: -s, yy: c}
}

// skew slants by the counter-clockwise angles x and y (in turns of 180°), as
// PaintSkew does: x tilts the y axis, and y tilts the x axis.
func skew(x, y float64) affine {
	return affine{xx: 1, yx: math.Tan(y * math.Pi), xy: -math.Tan(x * math.Pi), yy: 1}
}

func f2dot14(v tables.Fixed214) float64 { return float64(v) / 16384 }

// pathSeg is one segment of a glyph outline in font units, y up: a move, a
// line, a quadratic or a cubic Bézier curve to its last point.
type pathSeg struct {
	op  uint8 // 0 move, 1 line, 2 quad, 3 cubic
	pts [3][2]float64
}

// colrFont is what the paint graph reads from a font. It is an interface
// over the font so that tests can draw a synthetic paint graph.
type colrFont struct {
	outline func(gid uint16) []pathSeg
	base    func(gid uint16) (tables.PaintTable, bool) // COLR.Search
	layers  func(tables.PaintColrLayers) ([]tables.PaintTable, error)
	clipBox func(gid uint16) (box [4]float64, ok bool) // xMin, yMin, xMax, yMax
	palette []tables.ColorRecord
	fg      [3]float32 // the text color, straight R, G, B
}

// colrRender draws one color glyph into a canvas of w×h pixels.
type colrRender struct {
	f      *colrFont
	w, h   int
	ras    vector.Rasterizer
	depth  int
	active []uint16 // the PaintColrGlyph glyphs being drawn, against cycles
}

// canvas is premultiplied R, G, B, A from 0 to 1, row by row.
type canvas []float32

const colrMaxDepth = 64

func (r *colrRender) newCanvas() canvas { return make(canvas, r.w*r.h*4) }

// color returns palette entry i at alpha a, premultiplied. Index 0xFFFF is
// the text color.
func (r *colrRender) color(i uint16, a float64) [4]float32 {
	var c [4]float32
	if i == 0xFFFF {
		c = [4]float32{r.f.fg[0], r.f.fg[1], r.f.fg[2], 1}
	} else if int(i) < len(r.f.palette) {
		p := r.f.palette[i]
		c = [4]float32{float32(p.Red) / 255, float32(p.Green) / 255, float32(p.Blue) / 255, float32(p.Alpha) / 255}
	} else {
		return c // a bad index draws nothing
	}
	c[3] *= float32(min(max(a, 0), 1))
	return [4]float32{c[0] * c[3], c[1] * c[3], c[2] * c[3], c[3]}
}

// paint draws p over dst under the transform m, only inside clip (canvas
// pixels).
func (r *colrRender) paint(dst canvas, p tables.PaintTable, m affine, clip image.Rectangle) {
	if clip.Empty() || r.depth >= colrMaxDepth {
		return
	}
	r.depth++
	defer func() { r.depth-- }()
	switch p := p.(type) {
	case tables.PaintColrLayersResolved: // version 0
		for _, l := range p {
			if mask, box := r.glyphMask(l.GlyphID, m, clip); !box.Empty() {
				fillMask(dst, r.w, mask, box, r.color(l.PaletteIndex, 1))
			}
		}
	case tables.PaintColrLayers:
		layers, err := r.f.layers(p)
		if err != nil {
			return
		}
		for _, l := range layers {
			r.paint(dst, l, m, clip)
		}
	case tables.PaintSolid:
		fillRect(dst, r.w, clip, r.color(p.PaletteIndex, f2dot14(p.Alpha)))
	case tables.PaintVarSolid:
		fillRect(dst, r.w, clip, r.color(p.PaletteIndex, f2dot14(p.Alpha)))
	case tables.PaintLinearGradient:
		r.shade(dst, m, clip, linearGradient(r.colorLine(p.ColorLine), p.X0, p.Y0, p.X1, p.Y1, p.X2, p.Y2))
	case tables.PaintVarLinearGradient:
		r.shade(dst, m, clip, linearGradient(r.varColorLine(p.ColorLine), p.X0, p.Y0, p.X1, p.Y1, p.X2, p.Y2))
	case tables.PaintRadialGradient:
		r.shade(dst, m, clip, radialGradient(r.colorLine(p.ColorLine), p.X0, p.Y0, p.Radius0, p.X1, p.Y1, p.Radius1))
	case tables.PaintVarRadialGradient:
		r.shade(dst, m, clip, radialGradient(r.varColorLine(p.ColorLine), p.X0, p.Y0, p.Radius0, p.X1, p.Y1, p.Radius1))
	case tables.PaintSweepGradient:
		r.shade(dst, m, clip, sweepGradient(r.colorLine(p.ColorLine), p.CenterX, p.CenterY, p.StartAngle, p.EndAngle))
	case tables.PaintVarSweepGradient:
		r.shade(dst, m, clip, sweepGradient(r.varColorLine(p.ColorLine), p.CenterX, p.CenterY, p.StartAngle, p.EndAngle))
	case tables.PaintGlyph:
		mask, box := r.glyphMask(p.GlyphID, m, clip)
		r.clipped(dst, mask, box, p.Paint, m)
	case tables.PaintColrGlyph:
		if slices.Contains(r.active, p.GlyphID) {
			return
		}
		child, ok := r.f.base(p.GlyphID)
		if !ok {
			return
		}
		r.active = append(r.active, p.GlyphID)
		defer func() { r.active = r.active[:len(r.active)-1] }()
		if b, ok := r.f.clipBox(p.GlyphID); ok {
			mask, box := r.pathMask(boxPath(b), m, clip)
			r.clipped(dst, mask, box, child, m)
		} else {
			r.paint(dst, child, m, clip)
		}
	case tables.PaintComposite:
		backdrop, src := r.newCanvas(), r.newCanvas()
		r.paint(backdrop, p.BackdropPaint, m, clip)
		r.paint(src, p.SourcePaint, m, clip)
		composite(backdrop, src, r.w, clip, p.CompositeMode)
		srcOver(dst, backdrop, r.w, clip)
	default:
		if child, t, ok := transformOf(p); ok {
			r.paint(dst, child, m.mul(t), clip)
		}
	}
}

// clipped draws child inside the coverage mask, whose pixels are all inside
// box.
func (r *colrRender) clipped(dst canvas, mask []byte, box image.Rectangle, child tables.PaintTable, m affine) {
	if box.Empty() {
		return
	}
	// The common case, a glyph of one color, needs no canvas of its own.
	switch c := child.(type) {
	case tables.PaintSolid:
		fillMask(dst, r.w, mask, box, r.color(c.PaletteIndex, f2dot14(c.Alpha)))
		return
	case tables.PaintVarSolid:
		fillMask(dst, r.w, mask, box, r.color(c.PaletteIndex, f2dot14(c.Alpha)))
		return
	}
	tmp := r.newCanvas()
	r.paint(tmp, child, m, box)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			i := y*r.w + x
			if cov := mask[i]; cov != 0 {
				k := float32(cov) / 255
				s := tmp[i*4 : i*4+4]
				blendOver(dst[i*4:i*4+4], [4]float32{s[0] * k, s[1] * k, s[2] * k, s[3] * k})
			}
		}
	}
}

// glyphMask rasterizes the outline of glyph gid under m into a coverage mask
// of the canvas's size, and returns it with the box, within clip, that its
// coverage lies in.
func (r *colrRender) glyphMask(gid uint16, m affine, clip image.Rectangle) ([]byte, image.Rectangle) {
	return r.pathMask(r.f.outline(gid), m, clip)
}

func (r *colrRender) pathMask(path []pathSeg, m affine, clip image.Rectangle) ([]byte, image.Rectangle) {
	if len(path) == 0 {
		return nil, image.Rectangle{}
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	r.ras.Reset(r.w, r.h)
	pt := func(p [2]float64) (float32, float32) {
		x, y := m.apply(p[0], p[1])
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		return float32(x), float32(y)
	}
	open := false
	for _, s := range path {
		switch s.op {
		case 0:
			if open {
				r.ras.ClosePath()
			}
			r.ras.MoveTo(pt(s.pts[0]))
			open = true
		case 1:
			r.ras.LineTo(pt(s.pts[0]))
		case 2:
			ax, ay := pt(s.pts[0])
			bx, by := pt(s.pts[1])
			r.ras.QuadTo(ax, ay, bx, by)
		case 3:
			ax, ay := pt(s.pts[0])
			bx, by := pt(s.pts[1])
			cx, cy := pt(s.pts[2])
			r.ras.CubeTo(ax, ay, bx, by, cx, cy)
		}
	}
	if open {
		r.ras.ClosePath()
	}
	box := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX)), int(math.Ceil(maxY))).Intersect(clip)
	if box.Empty() {
		return nil, box
	}
	mask := &image.Alpha{Pix: make([]byte, r.w*r.h), Stride: r.w, Rect: image.Rect(0, 0, r.w, r.h)}
	r.ras.Draw(mask, mask.Rect, image.Opaque, image.Point{})
	// The rasterizer covers the whole canvas; clear what is outside clip.
	for y := box.Min.Y; y < box.Max.Y; y++ {
		row := mask.Pix[y*r.w : (y+1)*r.w]
		clear(row[:box.Min.X])
		clear(row[box.Max.X:])
	}
	return mask.Pix, box
}

// boxPath is the outline of a clip box.
func boxPath(b [4]float64) []pathSeg {
	return []pathSeg{
		{op: 0, pts: [3][2]float64{{b[0], b[1]}}},
		{op: 1, pts: [3][2]float64{{b[2], b[1]}}},
		{op: 1, pts: [3][2]float64{{b[2], b[3]}}},
		{op: 1, pts: [3][2]float64{{b[0], b[3]}}},
	}
}

// shade draws the gradient g, a function of a point in font units that
// returns a premultiplied color, over dst under the transform m. Each pixel
// takes the color at its center.
func (r *colrRender) shade(dst canvas, m affine, clip image.Rectangle, g func(x, y float64) [4]float32) {
	inv, ok := m.invert()
	if !ok || g == nil {
		return
	}
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		for x := clip.Min.X; x < clip.Max.X; x++ {
			fx, fy := inv.apply(float64(x)+0.5, float64(y)+0.5)
			if c := g(fx, fy); c[3] > 0 {
				i := (y*r.w + x) * 4
				blendOver(dst[i:i+4], c)
			}
		}
	}
}

// fillRect draws the premultiplied color c over the box of dst.
func fillRect(dst canvas, w int, box image.Rectangle, c [4]float32) {
	if c[3] <= 0 {
		return
	}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			i := (y*w + x) * 4
			blendOver(dst[i:i+4], c)
		}
	}
}

// fillMask draws the premultiplied color c over dst through the coverage
// mask, inside box.
func fillMask(dst canvas, w int, mask []byte, box image.Rectangle, c [4]float32) {
	if c[3] <= 0 {
		return
	}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			i := y*w + x
			if cov := mask[i]; cov != 0 {
				k := float32(cov) / 255
				blendOver(dst[i*4:i*4+4], [4]float32{c[0] * k, c[1] * k, c[2] * k, c[3] * k})
			}
		}
	}
}

// blendOver draws the premultiplied s over the premultiplied pixel d.
func blendOver(d []float32, s [4]float32) {
	k := 1 - s[3]
	d[0] = s[0] + d[0]*k
	d[1] = s[1] + d[1]*k
	d[2] = s[2] + d[2]*k
	d[3] = s[3] + d[3]*k
}

// srcOver draws src over dst inside box.
func srcOver(dst, src canvas, w int, box image.Rectangle) {
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			i := (y*w + x) * 4
			if src[i+3] > 0 {
				blendOver(dst[i:i+4], [4]float32(src[i:i+4]))
			}
		}
	}
}

// composite combines src with the backdrop dst by mode, inside box, leaving
// the result in dst. The Porter-Duff modes and the separable blend modes are
// exact. The non-separable blend modes (hue, saturation, color, luminosity)
// are approximated by source-over.
func composite(dst, src canvas, w int, box image.Rectangle, mode tables.CompositeMode) {
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			i := (y*w + x) * 4
			d, s := dst[i:i+4], [4]float32(src[i:i+4])
			compositePixel(d, s, mode)
		}
	}
}

func compositePixel(d []float32, s [4]float32, mode tables.CompositeMode) {
	sa, da := s[3], d[3]
	pd := func(fs, fd float32) {
		for c := range 4 {
			d[c] = s[c]*fs + d[c]*fd
		}
	}
	switch mode {
	case tables.CompositeClear:
		pd(0, 0)
	case tables.CompositeSrc:
		pd(1, 0)
	case tables.CompositeDest:
	case tables.CompositeSrcOver:
		pd(1, 1-sa)
	case tables.CompositeDestOver:
		pd(1-da, 1)
	case tables.CompositeSrcIn:
		pd(da, 0)
	case tables.CompositeDestIn:
		pd(0, sa)
	case tables.CompositeSrcOut:
		pd(1-da, 0)
	case tables.CompositeDestOut:
		pd(0, 1-sa)
	case tables.CompositeSrcAtop:
		pd(da, 1-sa)
	case tables.CompositeDestAtop:
		pd(1-da, sa)
	case tables.CompositeXor:
		pd(1-da, 1-sa)
	case tables.CompositePlus:
		for c := range 4 {
			d[c] = min(1, s[c]+d[c])
		}
	default:
		b := blendFunc(mode)
		if b == nil {
			pd(1, 1-sa) // non-separable modes: source-over
			return
		}
		// co = cs·(1 − ab) + cb·(1 − as) + as·ab·B(Cb, Cs), with Cb and Cs
		// the straight colors (Compositing and Blending Level 1).
		for c := range 3 {
			var cs, cb float32
			if sa > 0 {
				cs = s[c] / sa
			}
			if da > 0 {
				cb = d[c] / da
			}
			d[c] = s[c]*(1-da) + d[c]*(1-sa) + sa*da*b(cb, cs)
		}
		d[3] = sa + da - sa*da
	}
}

// blendFunc returns the separable blend function B(Cb, Cs) of mode, or nil.
func blendFunc(mode tables.CompositeMode) func(cb, cs float32) float32 {
	screen := func(cb, cs float32) float32 { return cb + cs - cb*cs }
	hardLight := func(cb, cs float32) float32 {
		if cs <= 0.5 {
			return cb * 2 * cs
		}
		return screen(cb, 2*cs-1)
	}
	switch mode {
	case tables.CompositeMultiply:
		return func(cb, cs float32) float32 { return cb * cs }
	case tables.CompositeScreen:
		return screen
	case tables.CompositeOverlay:
		return func(cb, cs float32) float32 { return hardLight(cs, cb) }
	case tables.CompositeDarken:
		return func(cb, cs float32) float32 { return min(cb, cs) }
	case tables.CompositeLighten:
		return func(cb, cs float32) float32 { return max(cb, cs) }
	case tables.CompositeColorDodge:
		return func(cb, cs float32) float32 {
			switch {
			case cb == 0:
				return 0
			case cs >= 1:
				return 1
			}
			return min(1, cb/(1-cs))
		}
	case tables.CompositeColorBurn:
		return func(cb, cs float32) float32 {
			switch {
			case cb >= 1:
				return 1
			case cs == 0:
				return 0
			}
			return 1 - min(1, (1-cb)/cs)
		}
	case tables.CompositeHardLight:
		return hardLight
	case tables.CompositeSoftLight:
		return func(cb, cs float32) float32 {
			if cs <= 0.5 {
				return cb - (1-2*cs)*cb*(1-cb)
			}
			d := float32(math.Sqrt(float64(cb)))
			if cb <= 0.25 {
				d = ((16*cb-12)*cb + 4) * cb
			}
			return cb + (2*cs-1)*(d-cb)
		}
	case tables.CompositeDifference:
		return func(cb, cs float32) float32 { return float32(math.Abs(float64(cb - cs))) }
	case tables.CompositeExclusion:
		return func(cb, cs float32) float32 { return cb + cs - 2*cb*cs }
	}
	return nil
}

// colorStop is a stop of a color line, its color premultiplied.
type colorStop struct {
	off float64
	c   [4]float32
}

// colorLine is the colors of a gradient along its parameter t.
type colorLine struct {
	stops  []colorStop // sorted by offset
	extend tables.Extend
}

func (r *colrRender) colorLine(l tables.ColorLine) colorLine {
	out := colorLine{extend: l.Extend}
	for _, s := range l.ColorStops {
		out.stops = append(out.stops, colorStop{f2dot14(s.StopOffset), r.color(s.PaletteIndex, f2dot14(s.Alpha))})
	}
	slices.SortStableFunc(out.stops, func(a, b colorStop) int {
		switch {
		case a.off < b.off:
			return -1
		case a.off > b.off:
			return 1
		}
		return 0
	})
	return out
}

func (r *colrRender) varColorLine(l tables.VarColorLine) colorLine {
	stops := make([]tables.ColorStop, len(l.ColorStops))
	for i, s := range l.ColorStops {
		stops[i] = tables.ColorStop{StopOffset: s.StopOffset, PaletteIndex: s.PaletteIndex, Alpha: s.Alpha}
	}
	return r.colorLine(tables.ColorLine{Extend: l.Extend, ColorStops: stops})
}

// at returns the color at t. Outside the offsets of the first and last stops,
// the extend mode applies: pad uses the nearest stop, repeat starts the
// stops over, and reflect mirrors them. Colors are interpolated
// premultiplied.
func (l colorLine) at(t float64) [4]float32 {
	n := len(l.stops)
	if n == 0 || math.IsNaN(t) {
		return [4]float32{}
	}
	lo, hi := l.stops[0].off, l.stops[n-1].off
	if span := hi - lo; span > 0 {
		switch l.extend {
		case tables.ExtendRepeat:
			u := math.Mod(t-lo, span)
			if u < 0 {
				u += span
			}
			t = lo + u
		case tables.ExtendReflect:
			u := math.Mod(t-lo, 2*span)
			if u < 0 {
				u += 2 * span
			}
			if u > span {
				u = 2*span - u
			}
			t = lo + u
		}
	}
	if t <= lo {
		return l.stops[0].c
	}
	if t >= hi {
		return l.stops[n-1].c
	}
	for i := 1; i < n; i++ {
		a, b := l.stops[i-1], l.stops[i]
		if t < b.off {
			if b.off <= a.off {
				return b.c
			}
			k := float32((t - a.off) / (b.off - a.off))
			return [4]float32{
				a.c[0] + (b.c[0]-a.c[0])*k,
				a.c[1] + (b.c[1]-a.c[1])*k,
				a.c[2] + (b.c[2]-a.c[2])*k,
				a.c[3] + (b.c[3]-a.c[3])*k,
			}
		}
	}
	return l.stops[n-1].c
}

// linearGradient is PaintLinearGradient: the color line runs from p0 to p1,
// and lines of one color are parallel to p0p2. The effective end point p3 is
// p1 projected onto the normal of p0p2 through p0.
func linearGradient(l colorLine, x0, y0, x1, y1, x2, y2 int16) func(x, y float64) [4]float32 {
	px0, py0 := float64(x0), float64(y0)
	d1x, d1y := float64(x1)-px0, float64(y1)-py0
	d2x, d2y := float64(x2)-px0, float64(y2)-py0
	vx, vy := d1x, d1y
	if nx, ny := -d2y, d2x; nx != 0 || ny != 0 {
		k := (d1x*nx + d1y*ny) / (nx*nx + ny*ny)
		vx, vy = nx*k, ny*k
	}
	l2 := vx*vx + vy*vy
	if l2 == 0 {
		return nil
	}
	return func(x, y float64) [4]float32 {
		return l.at(((x-px0)*vx + (y-py0)*vy) / l2)
	}
}

// radialGradient is PaintRadialGradient, a two-point conical gradient: the
// color at t is drawn on the circle whose center and radius are interpolated
// from the start circle (t = 0) to the end circle (t = 1). A point takes the
// largest t whose circle passes through it with a radius of at least 0, and
// a point on no such circle is not painted.
func radialGradient(l colorLine, x0, y0 int16, r0 uint16, x1, y1 int16, r1 uint16) func(x, y float64) [4]float32 {
	cx, cy, rr0 := float64(x0), float64(y0), float64(r0)
	cdx, cdy, dr := float64(x1)-cx, float64(y1)-cy, float64(r1)-float64(r0)
	a := cdx*cdx + cdy*cdy - dr*dr
	return func(x, y float64) [4]float32 {
		t, ok := conicalT(x-cx, y-cy, cdx, cdy, rr0, dr, a)
		if !ok {
			return [4]float32{}
		}
		return l.at(t)
	}
}

// conicalT solves |p − t·cd| = r0 + t·dr for the largest t with a radius of
// at least 0. p is relative to the start center, and a is cd·cd − dr².
func conicalT(px, py, cdx, cdy, r0, dr, a float64) (float64, bool) {
	b := px*cdx + py*cdy + r0*dr
	c := px*px + py*py - r0*r0
	valid := func(t float64) bool { return r0+t*dr >= 0 }
	if math.Abs(a) < 1e-9 {
		if b == 0 {
			return 0, false
		}
		t := c / (2 * b)
		return t, valid(t)
	}
	disc := b*b - a*c
	if disc < 0 {
		return 0, false
	}
	s := math.Sqrt(disc)
	t1, t2 := (b+s)/a, (b-s)/a
	if t1 < t2 {
		t1, t2 = t2, t1
	}
	if valid(t1) {
		return t1, true
	}
	return t2, valid(t2)
}

// sweepGradient is PaintSweepGradient: the color line runs counter-clockwise
// around the center, from the start angle to the end angle (each stored as
// value + 1 in turns of 180°).
func sweepGradient(l colorLine, cx, cy int16, start, end tables.Fixed214) func(x, y float64) [4]float32 {
	a0 := (f2dot14(start) + 1) * 180
	a1 := (f2dot14(end) + 1) * 180
	ox, oy := float64(cx), float64(cy)
	return func(x, y float64) [4]float32 {
		a := math.Atan2(y-oy, x-ox) * 180 / math.Pi
		if a < 0 {
			a += 360
		}
		if a1 == a0 {
			if l.extend != tables.ExtendPad {
				return [4]float32{}
			}
			if a < a0 {
				return l.at(math.Inf(-1))
			}
			return l.at(math.Inf(1))
		}
		return l.at((a - a0) / (a1 - a0))
	}
}

// paintBounds returns the bounds, in canvas pixels, of what p draws under m:
// the union of its glyph outlines. ok is false when p draws nothing that a
// glyph bounds.
func (r *colrRender) paintBounds(p tables.PaintTable, m affine) (b [4]float64, ok bool) {
	if r.depth >= colrMaxDepth {
		return b, false
	}
	r.depth++
	defer func() { r.depth-- }()
	b = [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	add := func(c [4]float64, cok bool) {
		if cok {
			b = [4]float64{min(b[0], c[0]), min(b[1], c[1]), max(b[2], c[2]), max(b[3], c[3])}
			ok = true
		}
	}
	outline := func(gid uint16, m affine) {
		for _, s := range r.f.outline(gid) {
			n := [4]int{1, 1, 2, 3}[s.op]
			for _, p := range s.pts[:n] {
				x, y := m.apply(p[0], p[1])
				add([4]float64{x, y, x, y}, true)
			}
		}
	}
	switch p := p.(type) {
	case tables.PaintColrLayersResolved:
		for _, l := range p {
			outline(l.GlyphID, m)
		}
	case tables.PaintColrLayers:
		layers, err := r.f.layers(p)
		if err == nil {
			for _, l := range layers {
				add(r.paintBounds(l, m))
			}
		}
	case tables.PaintGlyph:
		outline(p.GlyphID, m)
	case tables.PaintColrGlyph:
		if !slices.Contains(r.active, p.GlyphID) {
			if child, found := r.f.base(p.GlyphID); found {
				r.active = append(r.active, p.GlyphID)
				add(r.paintBounds(child, m))
				r.active = r.active[:len(r.active)-1]
			}
		}
	case tables.PaintComposite:
		add(r.paintBounds(p.SourcePaint, m))
		add(r.paintBounds(p.BackdropPaint, m))
	default:
		if child, t, isTransform := transformOf(p); isTransform {
			add(r.paintBounds(child, m.mul(t)))
		}
	}
	return b, ok
}

// transformOf returns the child and transform of a transform paint.
func transformOf(p tables.PaintTable) (tables.PaintTable, affine, bool) {
	switch p := p.(type) {
	case tables.PaintTransform:
		t := p.Transform
		return p.Paint, affine{float64(t.Xx), float64(t.Yx), float64(t.Xy), float64(t.Yy), float64(t.Dx), float64(t.Dy)}, true
	case tables.PaintVarTransform:
		t := p.Transform
		return p.Paint, affine{float64(t.Xx), float64(t.Yx), float64(t.Xy), float64(t.Yy), float64(t.Dx), float64(t.Dy)}, true
	case tables.PaintTranslate:
		return p.Paint, translate(float64(p.Dx), float64(p.Dy)), true
	case tables.PaintVarTranslate:
		return p.Paint, translate(float64(p.Dx), float64(p.Dy)), true
	case tables.PaintScale:
		return p.Paint, affine{xx: f2dot14(p.ScaleX), yy: f2dot14(p.ScaleY)}, true
	case tables.PaintVarScale:
		return p.Paint, affine{xx: f2dot14(p.ScaleX), yy: f2dot14(p.ScaleY)}, true
	case tables.PaintScaleAroundCenter:
		return p.Paint, around(affine{xx: f2dot14(p.ScaleX), yy: f2dot14(p.ScaleY)}, float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintVarScaleAroundCenter:
		return p.Paint, around(affine{xx: f2dot14(p.ScaleX), yy: f2dot14(p.ScaleY)}, float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintScaleUniform:
		return p.Paint, affine{xx: f2dot14(p.Scale), yy: f2dot14(p.Scale)}, true
	case tables.PaintVarScaleUniform:
		return p.Paint, affine{xx: f2dot14(p.Scale), yy: f2dot14(p.Scale)}, true
	case tables.PaintScaleUniformAroundCenter:
		return p.Paint, around(affine{xx: f2dot14(p.Scale), yy: f2dot14(p.Scale)}, float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintVarScaleUniformAroundCenter:
		return p.Paint, around(affine{xx: f2dot14(p.Scale), yy: f2dot14(p.Scale)}, float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintRotate:
		return p.Paint, rotation(f2dot14(p.Angle)), true
	case tables.PaintVarRotate:
		return p.Paint, rotation(f2dot14(p.Angle)), true
	case tables.PaintRotateAroundCenter:
		return p.Paint, around(rotation(f2dot14(p.Angle)), float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintVarRotateAroundCenter:
		return p.Paint, around(rotation(f2dot14(p.Angle)), float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintSkew:
		return p.Paint, skew(f2dot14(p.XSkewAngle), f2dot14(p.YSkewAngle)), true
	case tables.PaintVarSkew:
		return p.Paint, skew(f2dot14(p.XSkewAngle), f2dot14(p.YSkewAngle)), true
	case tables.PaintSkewAroundCenter:
		return p.Paint, around(skew(f2dot14(p.XSkewAngle), f2dot14(p.YSkewAngle)), float64(p.CenterX), float64(p.CenterY)), true
	case tables.PaintVarSkewAroundCenter:
		return p.Paint, around(skew(f2dot14(p.XSkewAngle), f2dot14(p.YSkewAngle)), float64(p.CenterX), float64(p.CenterY)), true
	}
	return nil, affine{}, false
}

// drawColr draws the color glyph whose paint is root at size pixels per em
// (upem font units), with its origin fx pixels into the first pixel. It
// returns the top-left of the image relative to the origin, as dwriteGlyph
// does, and the image in premultiplied RGBA from alloc. box is the glyph's
// clip box in font units, if the font has one; otherwise the glyph is
// bounded by its outlines.
func drawColr(f *colrFont, root tables.PaintTable, box *[4]float64, upem, size, fx float64, alloc func(w, h int) *image.RGBA) (dx, dy int, m *image.RGBA) {
	s := size / upem
	base := affine{xx: s, yy: -s, dx: fx}
	r := &colrRender{f: f}
	var b [4]float64
	if box != nil {
		x0, y0 := base.apply(box[0], box[3])
		x1, y1 := base.apply(box[2], box[1])
		b = [4]float64{x0, y0, x1, y1}
	} else {
		var ok bool
		if b, ok = r.paintBounds(root, base); !ok {
			return 0, 0, nil
		}
	}
	x0, y0 := int(math.Floor(b[0])), int(math.Floor(b[1]))
	x1, y1 := int(math.Ceil(b[2])), int(math.Ceil(b[3]))
	// A bad font must not make a huge image.
	limit := int(4*size) + 16
	if x1-x0 <= 0 || y1-y0 <= 0 || x1-x0 > limit || y1-y0 > limit {
		return 0, 0, nil
	}
	r.w, r.h = x1-x0, y1-y0
	dst := r.newCanvas()
	r.paint(dst, root, translate(-float64(x0), -float64(y0)).mul(base), image.Rect(0, 0, r.w, r.h))
	m = alloc(r.w, r.h)
	for i := 0; i < len(dst); i += 4 {
		a := min(max(dst[i+3], 0), 1)
		p := m.Pix[i : i+4]
		for c := range 3 {
			p[c] = uint8(min(max(dst[i+c], 0), a)*255 + 0.5)
		}
		p[3] = uint8(a*255 + 0.5)
	}
	return x0, y0, m
}
