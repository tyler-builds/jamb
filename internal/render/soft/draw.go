package soft

import (
	"math"

	"github.com/tyler-builds/jamb/internal/scene"
)

// Each primitive is drawn at once, in paint order. A pixel is shaded at its
// center (x+0.5, y+0.5), as a GPU rasterizes, and the shading functions below
// mirror the pixel shaders of render/d3d11 line for line.

// unit8 maps a byte to [0,1].
var unit8 = func() (t [256]float32) {
	for i := range t {
		t[i] = float32(i) / 255
	}
	return
}()

func sat(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// to8 converts [0,1] to a byte, rounding to nearest as a UNORM target does.
func to8(v float32) byte {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return byte(v*255 + 0.5)
}

// pixelSpan returns the pixels whose centers lie in [lo, hi), clamped to [0, n).
func pixelSpan(lo, hi float32, n int) (int, int) {
	a := int(math.Ceil(float64(lo - 0.5)))
	b := int(math.Ceil(float64(hi - 0.5)))
	return max(a, 0), min(b, n)
}

// blend draws the premultiplied color (r, g, b, a) over the pixel at i.
func (rr *band) blend(i int, r, g, b, a float32) {
	if a <= 0 && r <= 0 && g <= 0 && b <= 0 {
		return
	}
	p := rr.pix[i : i+4 : i+4]
	ia := 1 - a
	p[0] = to8(b + unit8[p[0]]*ia)
	p[1] = to8(g + unit8[p[1]]*ia)
	p[2] = to8(r + unit8[p[2]]*ia)
	p[3] = to8(a + unit8[p[3]]*ia)
}

// fillSpan draws the premultiplied color over the pixels [x0, x1) of row y.
func (rr *band) fillSpan(y, x0, x1 int, r, g, b, a float32) {
	if x1 <= x0 || a <= 0 && r <= 0 && g <= 0 && b <= 0 {
		return
	}
	row := rr.pix[y*rr.stride+x0*4 : y*rr.stride+x1*4]
	if a >= 1 {
		px := [4]byte{to8(b), to8(g), to8(r), 255}
		for i := 0; i < len(row); i += 4 {
			copy(row[i:i+4], px[:])
		}
		return
	}
	ia := 1 - a
	for i := 0; i < len(row); i += 4 {
		p := row[i : i+4 : i+4]
		p[0] = to8(b + unit8[p[0]]*ia)
		p[1] = to8(g + unit8[p[1]]*ia)
		p[2] = to8(r + unit8[p[2]]*ia)
		p[3] = to8(a + unit8[p[3]]*ia)
	}
}

// pickRadius returns the radius of the corner nearest q (relative to the center).
func pickRadius(qx, qy float32, c scene.Corners) float32 {
	if qx < 0 {
		if qy < 0 {
			return c.TL
		}
		return c.BL
	}
	if qy < 0 {
		return c.TR
	}
	return c.BR
}

func maxRadius(c scene.Corners) float32 { return max(c.TL, c.TR, c.BR, c.BL) }

// rrectSDF is the signed distance from q (relative to the center) to a rounded
// rectangle of the half size with corner radius r.
func rrectSDF(qx, qy, hx, hy, r float32) float32 {
	dx := abs(qx) - hx + r
	dy := abs(qy) - hy + r
	ox, oy := max(dx, 0), max(dy, 0)
	return float32(math.Sqrt(float64(ox*ox+oy*oy))) + min(max(dx, dy), 0) - r
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// clipper is a clip level prepared for shading.
type clipper struct {
	x0, y0, x1, y1 float32
	rounded        bool
	cx, cy, hx, hy float32 // the rounded shape's center and half size
	radii          scene.Corners
}

func (rr *Renderer) clipper() clipper {
	c := rr.clips[len(rr.clips)-1]
	cl := clipper{x0: c.rect.X, y0: c.rect.Y, x1: c.rect.Right(), y1: c.rect.Bottom()}
	if c.radii.TL+c.radii.TR+c.radii.BR+c.radii.BL > 0 {
		cl.rounded = true
		cl.hx, cl.hy = c.shape.W*0.5, c.shape.H*0.5
		cl.cx, cl.cy = c.shape.X+cl.hx, c.shape.Y+cl.hy
		cl.radii = c.radii
	}
	return cl
}

// coverage is clip_coverage: the anti-aliased coverage of the pixel centered
// at (px, py) by the clip rectangle and the rounded clip shape.
func (c *clipper) coverage(px, py float32) float32 {
	v := sat(px-c.x0+0.5) * sat(py-c.y0+0.5) * sat(c.x1-px+0.5) * sat(c.y1-py+0.5)
	if c.rounded && v > 0 {
		qx, qy := px-c.cx, py-c.cy
		r := min(pickRadius(qx, qy, c.radii), min(c.hx, c.hy))
		v *= sat(0.5 - rrectSDF(qx, qy, c.hx, c.hy, r))
	}
	return v
}

// solid returns the pixel centers [x0, x1) × [y0, y1) where the clip covers
// fully, as a range of pixels (empty when there are none).
func (c *clipper) solid() (x0, y0, x1, y1 float32) {
	x0, y0, x1, y1 = c.x0+0.5, c.y0+0.5, c.x1-0.5, c.y1-0.5
	if c.rounded {
		m := min(maxRadius(c.radii), min(c.hx, c.hy)) + 0.5
		x0, y0 = max(x0, c.cx-c.hx+m), max(y0, c.cy-c.hy+m)
		x1, y1 = min(x1, c.cx+c.hx-m), min(y1, c.cy+c.hy-m)
	}
	return
}

// ---- quads, borders, shadows and gradients ----

// quadShader holds what quad_ps reads, prepared once per quad.
type quadShader struct {
	cx, cy, hx, hy float32
	radii          scene.Corners
	sigma          float32
	fill           [4]float32 // premultiplied, when there is no gradient
	c1, c2         scene.Color
	lab1, lab2     [3]float32 // c1 and c2 in Oklab, for an Oklab gradient
	gradient       bool
	oklab          bool
	dirX, dirY     float32
	stop0, stop1   float32
	border         [4]float32 // premultiplied
	sides          [4]float32 // top, right, bottom, left
	hasSides       bool
}

func premul(c scene.Color) [4]float32 { return [4]float32{c.R * c.A, c.G * c.A, c.B * c.A, c.A} }

// DrawQuads implements render.Renderer.
func (rr *Renderer) DrawQuads(qs []scene.Quad) {
	if len(qs) == 0 || !rr.inFrame {
		return
	}
	start := len(rr.quads)
	rr.quads = append(rr.quads, qs...)
	rr.record(command{kind: cmdQuads, start: start, end: len(rr.quads)})
}

func (rr *band) drawQuad(q *scene.Quad, cl *clipper) {
	b := q.Bounds
	s := quadShader{
		hx: b.W * 0.5, hy: b.H * 0.5, radii: q.Radii, sigma: q.Blur,
		fill: premul(q.Background), c1: q.Background, c2: q.Color2,
		gradient: q.Gradient && q.Blur <= 0, oklab: q.Oklab,
		border: premul(q.BorderColor),
	}
	s.cx, s.cy = b.X+s.hx, b.Y+s.hy
	if bs := q.BorderSides; bs != (scene.Sides{}) {
		s.sides = [4]float32{bs.Top, bs.Right, bs.Bottom, bs.Left}
	} else {
		w := q.BorderWidth
		s.sides = [4]float32{w, w, w, w}
	}
	s.hasSides = s.sides[0] > 0 || s.sides[1] > 0 || s.sides[2] > 0 || s.sides[3] > 0
	if s.gradient {
		rad := float64(q.Angle) * math.Pi / 180
		s.dirX, s.dirY = float32(math.Sin(rad)), float32(-math.Cos(rad))
		s.stop0, s.stop1 = q.Stop0, q.Stop1
		if s.oklab {
			s.lab1, s.lab2 = toOklab(s.c1), toOklab(s.c2)
		}
	}

	// quad_vs: the box grown by the shadow's reach (or a pixel for the
	// anti-aliased edge), clamped to a pixel past the clip.
	pad := float32(1)
	if s.sigma > 0 {
		pad = float32(math.Ceil(float64(s.sigma * 3)))
	}
	ex0, ey0 := max(b.X-pad, cl.x0-1), max(b.Y-pad, cl.y0-1)
	ex1, ey1 := min(b.Right()+pad, cl.x1+1), min(b.Bottom()+pad, cl.y1+1)
	px0, px1 := pixelSpan(ex0, ex1, rr.width)
	py0, py1 := rr.rows(ey0, ey1)
	if px0 >= px1 || py0 >= py1 {
		return
	}

	// Inside the box away from its corners and borders, and inside the clip,
	// every pixel is the plain fill: those spans are filled without shading.
	sx0, sx1, sy0, sy1 := px1, px1, py1, py1 // empty
	if s.sigma <= 0 && !s.gradient {
		m := min(maxRadius(s.radii), min(s.hx, s.hy)) + 0.5
		if s.hasSides {
			m += max(s.sides[0], s.sides[1], s.sides[2], s.sides[3])
		}
		cx0, cy0, cx1, cy1 := cl.solid()
		lx0, ly0 := max(b.X+m, cx0), max(b.Y+m, cy0)
		lx1, ly1 := min(b.Right()-m, cx1), min(b.Bottom()-m, cy1)
		// Pixel i's center is i+0.5: the pixels whose centers lie in [lo, hi].
		ax, bx := int(math.Ceil(float64(lx0-0.5))), int(math.Floor(float64(lx1-0.5)))+1
		ay, by := int(math.Ceil(float64(ly0-0.5))), int(math.Floor(float64(ly1-0.5)))+1
		if ax < bx && ay < by {
			sx0, sx1 = max(ax, px0), min(bx, px1)
			sy0, sy1 = max(ay, py0), min(by, py1)
		}
	}

	if s.sigma > 0 {
		rr.drawShadow(&s, cl, px0, px1, py0, py1)
		return
	}
	for y := py0; y < py1; y++ {
		py := float32(y) + 0.5
		inSolid := y >= sy0 && y < sy1 && sx0 < sx1
		for x := px0; x < px1; x++ {
			if inSolid && x == sx0 {
				rr.fillSpan(y, sx0, sx1, s.fill[0], s.fill[1], s.fill[2], s.fill[3])
				x = sx1 - 1
				continue
			}
			px := float32(x) + 0.5
			cc := cl.coverage(px, py)
			if cc <= 0 {
				continue
			}
			c := s.shade(px, py)
			rr.blend(y*rr.stride+x*4, c[0]*cc, c[1]*cc, c[2]*cc, c[3]*cc)
		}
	}
}

// drawShadow draws a shadow quad. What shadowAlpha computes for a pixel
// depends on its row but for the horizontal erf terms, so each row's part is
// worked out once (for the left and the right corners) and each pixel only
// sums the erf terms.
func (rr *band) drawShadow(s *quadShader, cl *clipper, px0, px1, py0, py1 int) {
	minHalf := min(s.hx, s.hy)
	for y := py0; y < py1; y++ {
		py := float32(y) + 0.5
		qy := py - s.cy
		left := newShadowRow(qy, s.hx, s.hy, min(pickRadius(-1, qy, s.radii), minHalf), s.sigma)
		right := newShadowRow(qy, s.hx, s.hy, min(pickRadius(1, qy, s.radii), minHalf), s.sigma)
		// Far enough inside the shape, every erf term has saturated, so the
		// alpha is the same across the row: it is worked out once per side.
		flat := min(left.flatHalf(s.sigma), right.flatHalf(s.sigma))
		flatL, flatR := left.alpha(0), right.alpha(0)
		for x := px0; x < px1; x++ {
			px := float32(x) + 0.5
			cc := cl.coverage(px, py)
			if cc <= 0 {
				continue
			}
			qx := px - s.cx
			row := &right
			if qx < 0 {
				row = &left
			}
			var a float32
			switch {
			case qx < 0 && -qx <= flat:
				a = flatL * cc
			case qx >= 0 && qx <= flat:
				a = flatR * cc
			default:
				a = row.alpha(qx) * cc
			}
			if a <= 0 {
				continue
			}
			rr.blend(y*rr.stride+x*4, s.fill[0]*a, s.fill[1]*a, s.fill[2]*a, s.fill[3]*a)
		}
	}
}

// shadowRow is shadowAlpha for one row and one side: the half widths of the
// rounded box at the four sample heights, and their Gaussian weights.
type shadowRow struct {
	k              float32 // sqrt(0.5) / sigma
	curved, weight [4]float32
}

func newShadowRow(qy, hx, hy, corner, sigma float32) shadowRow {
	r := shadowRow{k: float32(math.Sqrt(0.5)) / sigma}
	low, high := qy-hy, qy+hy
	start := min(max(-3*sigma, low), high)
	end := min(max(3*sigma, low), high)
	step := (end - start) / 4
	y := start + step*0.5
	for i := range 4 {
		delta := min(hy-corner-abs(qy-y), 0)
		r.curved[i] = hx - corner + float32(math.Sqrt(float64(max(0, corner*corner-delta*delta))))
		r.weight[i] = gaussian(y, sigma) * step
		y += step
	}
	return r
}

// flatSigmas is how far inside the shape, in sigmas, the shadow counts as
// flat: there erf is within 0.0002 of 1, far below what an 8-bit target shows.
const flatSigmas = 4

// flatHalf returns how far from the center (in |qx|) the row's alpha is flat,
// or a negative value when no part of it is.
func (r *shadowRow) flatHalf(sigma float32) float32 {
	h := float32(math.MaxFloat32)
	for i := range 4 {
		if r.weight[i] != 0 {
			h = min(h, r.curved[i])
		}
	}
	return h - flatSigmas*sigma
}

// alpha is shadowAlpha at qx.
func (r *shadowRow) alpha(qx float32) float32 {
	var a float32
	for i := range 4 {
		if r.weight[i] == 0 {
			continue
		}
		c := r.curved[i]
		a += (0.5*erf((qx+c)*r.k) - 0.5*erf((qx-c)*r.k)) * r.weight[i]
	}
	return a
}

// shade is quad_ps without the clip: the premultiplied color at (px, py).
func (s *quadShader) shade(px, py float32) [4]float32 {
	qx, qy := px-s.cx, py-s.cy
	r := min(pickRadius(qx, qy, s.radii), min(s.hx, s.hy))
	if s.sigma > 0 {
		a := shadowAlpha(qx, qy, s.hx, s.hy, r, s.sigma)
		return [4]float32{s.fill[0] * a, s.fill[1] * a, s.fill[2] * a, s.fill[3] * a}
	}
	outer := sat(0.5 - rrectSDF(qx, qy, s.hx, s.hy, r))
	if outer <= 0 {
		return [4]float32{}
	}
	fill := s.fill
	if s.gradient {
		fill = premul(s.gradientFill(qx, qy))
	}
	if !s.hasSides {
		return [4]float32{fill[0] * outer, fill[1] * outer, fill[2] * outer, fill[3] * outer}
	}
	// The fill covers the inner edge and the border color the ring between the
	// two edges, so a side of zero width shows no border at its edge.
	inner := min(s.innerCoverage(qx, qy), outer)
	ring := outer - inner
	bd := s.border
	var out [4]float32
	for k := range 4 {
		over := bd[k] + fill[k]*(1-bd[3])
		out[k] = fill[k]*inner + over*ring
	}
	return out
}

// innerCoverage is inner_coverage: the coverage of the border's inner edge,
// the box inset by each side's width with each corner's radius reduced by the
// average of its two sides.
func (s *quadShader) innerCoverage(qx, qy float32) float32 {
	top, right, bottom, left := s.sides[0], s.sides[1], s.sides[2], s.sides[3]
	ihx := max(s.hx-(left+right)*0.5, 0)
	ihy := max(s.hy-(top+bottom)*0.5, 0)
	cx, cy := (left-right)*0.5, (top-bottom)*0.5
	sx, sy := right, bottom
	if qx < 0 {
		sx = left
	}
	if qy < 0 {
		sy = top
	}
	adj := 0.5 * (sx + sy)
	r := min(max(pickRadius(qx, qy, s.radii)-adj, 0), min(ihx, ihy))
	return sat(0.5 - rrectSDF(qx-cx, qy-cy, ihx, ihy, r))
}

// gradientFill is gradient_fill: the straight-alpha fill color at q. The
// gradient line runs through the center along the direction, with the length
// CSS uses, so the corners get the end colors.
func (s *quadShader) gradientFill(qx, qy float32) scene.Color {
	length := 2 * (abs(s.hx*s.dirX) + abs(s.hy*s.dirY))
	t := (qx*s.dirX+qy*s.dirY)/max(length, 1e-4) + 0.5
	span := s.stop1 - s.stop0
	if span > 1e-6 {
		t = sat((t - s.stop0) / span)
	} else if t >= s.stop0 {
		t = 1
	} else {
		t = 0
	}
	if s.oklab {
		r, g, b := oklabToLinear(s.lab1[0]+(s.lab2[0]-s.lab1[0])*t, s.lab1[1]+(s.lab2[1]-s.lab1[1])*t, s.lab1[2]+(s.lab2[2]-s.lab1[2])*t)
		return scene.Color{R: srgbOf(sat(r)), G: srgbOf(sat(g)), B: srgbOf(sat(b)), A: s.c1.A + (s.c2.A-s.c1.A)*t}
	}
	return scene.Color{
		R: s.c1.R + (s.c2.R-s.c1.R)*t, G: s.c1.G + (s.c2.G-s.c1.G)*t,
		B: s.c1.B + (s.c2.B-s.c1.B)*t, A: s.c1.A + (s.c2.A-s.c1.A)*t,
	}
}

// erf approximates the error function as the shader's erf2 does.
func erf(v float32) float32 {
	s := float32(1)
	if v < 0 {
		s = -1
	} else if v == 0 {
		s = 0
	}
	a := abs(v)
	r1 := 1 + (0.278393+(0.230389+(0.000972+0.078108*a)*a)*a)*a
	r2 := r1 * r1
	return s - s/(r2*r2)
}

func gaussian(x, sigma float32) float32 {
	return float32(math.Exp(float64(-(x*x)/(2*sigma*sigma))) / (math.Sqrt(2*math.Pi) * float64(sigma)))
}

func blurAlongX(x, y, sigma, corner, hx, hy float32) float32 {
	delta := min(hy-corner-abs(y), 0)
	curved := hx - corner + float32(math.Sqrt(float64(max(0, corner*corner-delta*delta))))
	k := float32(math.Sqrt(0.5)) / sigma
	lo := 0.5 + 0.5*erf((x-curved)*k)
	hi := 0.5 + 0.5*erf((x+curved)*k)
	return hi - lo
}

// shadowAlpha is shadow_alpha: the closed-form blurred rounded rectangle
// (after Evan Wallace), integrated over y in four steps.
func shadowAlpha(qx, qy, hx, hy, corner, sigma float32) float32 {
	low, high := qy-hy, qy+hy
	start := min(max(-3*sigma, low), high)
	end := min(max(3*sigma, low), high)
	step := (end - start) / 4
	y := start + step*0.5
	var alpha float32
	for range 4 {
		alpha += blurAlongX(qx, qy-y, sigma, corner, hx, hy) * gaussian(y, sigma) * step
		y += step
	}
	return alpha
}

// ---- Oklab ----

func srgbToLinear(c float32) float32 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return float32(math.Pow(float64((c+0.055)/1.055), 2.4))
}

func linearToSRGB(c float32) float32 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*float32(math.Pow(float64(max(c, 0)), 1/2.4)) - 0.055
}

func cbrt(v float32) float32 { return float32(math.Cbrt(float64(v))) }

func linearToOklab(r, g, b float32) (float32, float32, float32) {
	l := cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s
}

func oklabToLinear(L, A, B float32) (float32, float32, float32) {
	l := L + 0.3963377774*A + 0.2158037573*B
	m := L - 0.1055613458*A - 0.0638541728*B
	s := L - 0.0894841775*A - 1.2914855480*B
	l, m, s = l*l*l, m*m*m, s*s*s
	return 4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s
}

// toOklab converts a straight-alpha sRGB color's channels to Oklab.
func toOklab(c scene.Color) [3]float32 {
	l, a, b := linearToOklab(srgbToLinear(c.R), srgbToLinear(c.G), srgbToLinear(c.B))
	return [3]float32{l, a, b}
}

// srgbTable samples linearToSRGB on [0,1], for srgbOf.
var srgbTable = func() (t [1025]float32) {
	for i := range t {
		t[i] = linearToSRGB(float32(i) / 1024)
	}
	return
}()

// srgbOf is linearToSRGB of a value in [0,1], interpolated from a table: close
// to a thousandth, finer than an 8-bit target shows, and much cheaper than pow.
func srgbOf(v float32) float32 {
	f := v * 1024
	i := int(f)
	if i >= 1024 {
		return srgbTable[1024]
	}
	frac := f - float32(i)
	return srgbTable[i] + (srgbTable[i+1]-srgbTable[i])*frac
}

// oklabMix mixes straight-alpha colors in Oklab; alpha mixes linearly.
func oklabMix(a, b scene.Color, t float32) scene.Color {
	l1, a1, b1 := linearToOklab(srgbToLinear(a.R), srgbToLinear(a.G), srgbToLinear(a.B))
	l2, a2, b2 := linearToOklab(srgbToLinear(b.R), srgbToLinear(b.G), srgbToLinear(b.B))
	r, g, bb := oklabToLinear(l1+(l2-l1)*t, a1+(a2-a1)*t, b1+(b2-b1)*t)
	return scene.Color{
		R: linearToSRGB(sat(r)), G: linearToSRGB(sat(g)), B: linearToSRGB(sat(bb)),
		A: a.A + (b.A-a.A)*t,
	}
}

// ---- coverage masks: glyphs and icons ----

// DrawGlyphs implements render.Renderer.
func (rr *Renderer) DrawGlyphs(run scene.GlyphRun) {
	t := run.Texture
	if len(run.Glyphs) == 0 || t == nil || !rr.inFrame || t.Width <= 0 || t.Height <= 0 {
		return
	}
	if t.Format != scene.FormatSubpixel8 && t.Format != scene.FormatAlpha8 {
		return
	}
	start := len(rr.glyphs)
	rr.glyphs = append(rr.glyphs, run.Glyphs...)
	rr.record(command{kind: cmdGlyphs, start: start, end: len(rr.glyphs), tex: t})
}

// drawGlyphs draws glyphs from the mask texture t.
func (rr *band) drawGlyphs(t *scene.Texture, glyphs []scene.Glyph, cl *clipper) {
	subpixel := t.Format == scene.FormatSubpixel8
	bpp := t.BytesPerPixel()
	var tp textParams
	if subpixel {
		tp = newTextParams(t.Text)
	}
	for i := range glyphs {
		g := &glyphs[i]
		b := g.Bounds
		if b.W <= 0 || b.H <= 0 {
			continue
		}
		px0, px1 := pixelSpan(max(b.X, cl.x0-1), min(b.Right(), cl.x1+1), rr.width)
		py0, py1 := rr.rows(max(b.Y, cl.y0-1), min(b.Bottom(), cl.y1+1))
		if px0 >= px1 || py0 >= py1 {
			continue
		}
		col := g.Color
		pc := premul(col)
		brightness := col.R*0.30 + col.G*0.59 + col.B*0.11
		k := 0.3 + 0.7*brightness
		su, sv := g.Src.W/b.W, g.Src.H/b.H
		for y := py0; y < py1; y++ {
			py := float32(y) + 0.5
			ty := clampInt(int(math.Floor(float64(g.Src.Y+(py-b.Y)*sv))), t.Height)
			trow := t.Pix[ty*t.Width*bpp:]
			for x := px0; x < px1; x++ {
				px := float32(x) + 0.5
				tx := clampInt(int(math.Floor(float64(g.Src.X+(px-b.X)*su))), t.Width)
				if !subpixel {
					a := unit8[trow[tx]]
					if a <= 0 {
						continue
					}
					cc := cl.coverage(px, py)
					if cc <= 0 {
						continue
					}
					// Contrast enhancement, as the mask shader applies it.
					a = a * (k + 1) / (a*k + 1) * cc
					rr.blend(y*rr.stride+x*4, pc[0]*a, pc[1]*a, pc[2]*a, pc[3]*a)
					continue
				}
				texel := trow[tx*4 : tx*4+3]
				if texel[0] == 0 && texel[1] == 0 && texel[2] == 0 {
					continue
				}
				cc := cl.coverage(px, py)
				if cc <= 0 {
					continue
				}
				rr.blendSubpixel(y*rr.stride+x*4, col, tp.correct(texel, col), col.A*cc)
			}
		}
	}
}

func clampInt(v, n int) int {
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}

// blendSubpixel blends a ClearType glyph pixel per channel, as the dual-source
// blend state does: each channel of the destination keeps 1 - its own coverage.
func (rr *band) blendSubpixel(i int, col scene.Color, cov [3]float32, alpha float32) {
	ar, ag, ab := cov[0]*alpha, cov[1]*alpha, cov[2]*alpha
	am := (ar + ag + ab) / 3
	p := rr.pix[i : i+4 : i+4]
	p[0] = to8(col.B*ab + unit8[p[0]]*(1-ab))
	p[1] = to8(col.G*ag + unit8[p[1]]*(1-ag))
	p[2] = to8(col.R*ar + unit8[p[2]]*(1-ar))
	p[3] = to8(am + unit8[p[3]]*(1-am))
}

// textParams are the constants of correct_coverage.
type textParams struct{ gscale, cscale, level float32 }

func newTextParams(p scene.TextParams) textParams {
	return textParams{gscale: (p.Gamma - 1) / 0.8, cscale: p.Contrast / 0.5, level: p.Level}
}

// knot interpolates a value given at text intensity 0, 0.6 and 1.
func knot(v0, v1, v2, f float32) float32 {
	if f < 0.6 {
		return v0 + (v1-v0)*(f/0.6)
	}
	return v1 + (v2-v1)*((f-0.6)/0.4)
}

// channel is correct_channel: the coverage correction of one channel for text
// of channel value f, for blending in sRGB space.
func (p textParams) channel(a, f float32) float32 {
	k := max(0, knot(0.25, 0.45, 0.07, f)) * p.cscale
	a = a * (k + 1) / (a*k + 1)
	x := knot(-0.6184, 0.0252, 0.0654, f)
	y := knot(-0.7300, -0.6821, -0.7300, f)
	z := knot(0.8439, 1.5000, 1.1021, f)
	w := knot(-0.2500, -0.0500, -0.2500, f)
	return a + a*(1-a)*p.gscale*((x*f+y)*a+(z*f+w))
}

// correct is correct_coverage, for the texel's R, G and B coverage.
func (p textParams) correct(texel []byte, col scene.Color) [3]float32 {
	c := [3]float32{
		p.channel(unit8[texel[0]], col.R),
		p.channel(unit8[texel[1]], col.G),
		p.channel(unit8[texel[2]], col.B),
	}
	gray := (c[0] + c[1] + c[2]) / 3
	for i := range c {
		c[i] = sat(gray + (c[i]-gray)*p.level)
	}
	return c
}

// ---- images ----

// DrawImages implements render.Renderer.
func (rr *Renderer) DrawImages(ims []scene.ImageDraw) {
	if len(ims) == 0 || !rr.inFrame {
		return
	}
	start := len(rr.images)
	rr.images = append(rr.images, ims...)
	rr.record(command{kind: cmdImages, start: start, end: len(rr.images)})
}

func (rr *band) drawImage(im *scene.ImageDraw, cl *clipper) {
	t := im.Texture
	if t == nil || t.Format != scene.FormatRGBA8 || t.Width <= 0 || t.Height <= 0 || im.Opacity <= 0 {
		return
	}
	b := im.Bounds
	if b.W <= 0 || b.H <= 0 {
		return
	}
	tw, th := float32(t.Width), float32(t.Height)
	src := im.Src
	if src.Empty() {
		src = scene.Rect{W: tw, H: th}
	}
	px0, px1 := pixelSpan(max(b.X, cl.x0-1), min(b.Right(), cl.x1+1), rr.width)
	py0, py1 := rr.rows(max(b.Y, cl.y0-1), min(b.Bottom(), cl.y1+1))
	hx, hy := b.W*0.5, b.H*0.5
	cx, cy := b.X+hx, b.Y+hy
	su, sv := src.W/b.W, src.H/b.H
	for y := py0; y < py1; y++ {
		py := float32(y) + 0.5
		// The linear sampler: texel centers are at +0.5, clamped at the edges.
		fy := src.Y + (py-b.Y)*sv - 0.5
		y0 := float32(math.Floor(float64(fy)))
		wy := fy - y0
		r0 := clampInt(int(y0), t.Height) * t.Width * 4
		r1 := clampInt(int(y0)+1, t.Height) * t.Width * 4
		for x := px0; x < px1; x++ {
			px := float32(x) + 0.5
			qx, qy := px-cx, py-cy
			r := min(pickRadius(qx, qy, im.Radii), min(hx, hy))
			cov := sat(0.5 - rrectSDF(qx, qy, hx, hy, r))
			if cov <= 0 {
				continue
			}
			cov *= im.Opacity * cl.coverage(px, py)
			if cov <= 0 {
				continue
			}
			fx := src.X + (px-b.X)*su - 0.5
			x0 := float32(math.Floor(float64(fx)))
			wx := fx - x0
			c0 := clampInt(int(x0), t.Width) * 4
			c1 := clampInt(int(x0)+1, t.Width) * 4
			var out [4]float32
			for k := range 4 {
				top := unit8[t.Pix[r0+c0+k]]*(1-wx) + unit8[t.Pix[r0+c1+k]]*wx
				bot := unit8[t.Pix[r1+c0+k]]*(1-wx) + unit8[t.Pix[r1+c1+k]]*wx
				out[k] = (top*(1-wy) + bot*wy) * cov
			}
			rr.blend(y*rr.stride+x*4, out[0], out[1], out[2], out[3])
		}
	}
}
