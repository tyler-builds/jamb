package vector

import "image"

// Canvas draws into a premultiplied RGBA image. It keeps a stack of states,
// each with a transform, a clip and an opacity, and of layers, which group
// drawing so it is faded as a whole.
type Canvas struct {
	img    *image.RGBA
	states []state
	layers []layer
	r      rasterizer
	cov    []float32 // the coverage of the fill being drawn; all 0 between fills
	sh     shader    // the shader of the fill being drawn (see draw)
	spare  []*mask   // masks no state holds, to reuse
	// Tolerance is how far, in pixels, flattened curves may stray; 0 means
	// 0.05, which loses no visible area to the straight segments.
	Tolerance float32
}

type state struct {
	m     Matrix
	clip  *mask // nil when nothing is clipped
	alpha float32
}

// mask is the coverage of a clip within its bounds, x0..x1 by y0..y1 in
// pixels; outside them it is 0. A state holds the mask it clipped to, and those
// above it hold the same one until they clip again.
type mask struct {
	x0, y0, x1, y1 int
	cov            []float32 // (x1-x0)*(y1-y0) values, row by row
}

// at returns the row of m at y, which lies within its bounds, from x0 to x1.
func (m *mask) at(y int) []float32 {
	w := m.x1 - m.x0
	return m.cov[(y-m.y0)*w : (y-m.y0+1)*w]
}

type layer struct {
	img   *image.RGBA
	alpha float32
	depth int // the state depth when the layer was pushed
}

// NewCanvas returns a transparent canvas of w x h pixels.
func NewCanvas(w, h int) *Canvas {
	c := &Canvas{}
	c.Reset(w, h)
	return c
}

// Reset clears the canvas to transparent at w x h pixels and drops the states
// and layers, keeping the buffers it can.
func (c *Canvas) Reset(w, h int) {
	w, h = max(w, 0), max(h, 0)
	if c.img == nil || c.img.Rect.Dx() != w || c.img.Rect.Dy() != h {
		c.img = image.NewRGBA(image.Rect(0, 0, w, h))
		c.cov = make([]float32, w*h)
	} else {
		clear(c.img.Pix)
	}
	for len(c.states) > 1 {
		c.pop()
	}
	if len(c.states) > 0 && c.states[0].clip != nil {
		c.spare = append(c.spare, c.states[0].clip)
	}
	c.states = append(c.states[:0], state{m: Identity, alpha: 1})
	c.layers = c.layers[:0]
}

// Image returns the drawing: premultiplied RGBA, the canvas's own buffer.
func (c *Canvas) Image() *image.RGBA {
	for len(c.layers) > 0 {
		c.PopLayer()
	}
	return c.img
}

func (c *Canvas) top() *state { return &c.states[len(c.states)-1] }

// target is the image drawing goes into: the innermost layer's, or the
// canvas's.
func (c *Canvas) target() *image.RGBA {
	if n := len(c.layers); n > 0 {
		return c.layers[n-1].img
	}
	return c.img
}

// Save pushes a copy of the current state; Restore pops it.
func (c *Canvas) Save() { c.states = append(c.states, *c.top()) }

// Restore returns to the state of the matching Save. Layers pushed since it
// are drawn first.
func (c *Canvas) Restore() {
	if len(c.states) <= 1 {
		return
	}
	for n := len(c.layers); n > 0 && c.layers[n-1].depth >= len(c.states)-1; n = len(c.layers) {
		c.PopLayer()
	}
	c.pop()
}

// pop drops the top state, of two or more, keeping its mask to reuse when the
// state below does not hold it too.
func (c *Canvas) pop() {
	n := len(c.states)
	if m := c.states[n-1].clip; m != nil && m != c.states[n-2].clip {
		c.spare = append(c.spare, m)
	}
	c.states = c.states[:n-1]
}

// owns reports whether the top state made its own clip, which no state below it
// holds.
func (c *Canvas) owns() bool {
	n := len(c.states)
	return c.states[n-1].clip != nil && (n == 1 || c.states[n-1].clip != c.states[n-2].clip)
}

// Transform applies m before the current transform, so later drawing is in
// m's coordinates.
func (c *Canvas) Transform(m Matrix) { c.top().m = m.Then(c.top().m) }

// SetTransform replaces the current transform.
func (c *Canvas) SetTransform(m Matrix) { c.top().m = m }

// Matrix returns the current transform.
func (c *Canvas) Matrix() Matrix { return c.top().m }

// SetAlpha multiplies the opacity of later drawing by a (0 to 1).
func (c *Canvas) SetAlpha(a float32) { c.top().alpha *= min(max(a, 0), 1) }

// Clip limits later drawing to the inside of p (in the current coordinates),
// within the current clip.
func (c *Canvas) Clip(p *Path, rule FillRule) {
	w, h := c.img.Rect.Dx(), c.img.Rect.Dy()
	st := c.top()
	old := st.clip
	top, bottom := 0, h
	if old != nil {
		top, bottom = old.y0, old.y1
	}
	var y0, y1 int
	if top < bottom {
		y0, y1 = c.r.fill(c.polys(p, nil), rule, w, h, top, bottom, c.cov)
	}
	// The mask's bounds are those of what p covers, within the old mask's:
	// outside them the coverage of p or the old mask is 0, and so is their product.
	rows := c.r.rows
	x0, my0, x1, my1 := w, y1, 0, y0
	for y := y0; y < y1; y++ {
		if sp := rows[y-y0]; sp.x0 < sp.x1 {
			x0, x1 = min(x0, int(sp.x0)), max(x1, int(sp.x1))
			my0, my1 = min(my0, y), y+1
		}
	}
	if old != nil {
		x0, x1 = max(x0, old.x0), min(x1, old.x1)
	}
	if x0 >= x1 || my0 >= my1 {
		x0, my0, x1, my1 = 0, 0, 0, 0
	}
	m := c.newMask(x0, my0, x1, my1)
	for y := my0; y < my1; y++ {
		row := m.at(y)
		copy(row, c.cov[y*w+x0:y*w+x1])
		if old != nil {
			o := old.at(y)[x0-old.x0 : x1-old.x0]
			for i := range row {
				row[i] *= o[i]
			}
		}
	}
	for y := y0; y < y1; y++ {
		sp := rows[y-y0]
		clear(c.cov[y*w+int(sp.x0) : y*w+int(sp.x1)]) // leave c.cov zeroed
	}
	if c.owns() {
		c.spare = append(c.spare, old)
	}
	st.clip = m
}

// newMask returns a mask of the bounds x0..x1 by y0..y1, reusing a spare one,
// with its coverage left for the caller to set.
func (c *Canvas) newMask(x0, y0, x1, y1 int) *mask {
	var m *mask
	if n := len(c.spare); n > 0 {
		m, c.spare = c.spare[n-1], c.spare[:n-1]
	} else {
		m = &mask{}
	}
	m.x0, m.y0, m.x1, m.y1 = x0, y0, x1, y1
	n := (x1 - x0) * (y1 - y0)
	if cap(m.cov) < n {
		m.cov = make([]float32, n)
	}
	m.cov = m.cov[:n]
	return m
}

// PushLayer starts a layer: drawing goes into it until PopLayer (or the
// Restore of an earlier Save), which draws it onto what is below at opacity
// alpha.
func (c *Canvas) PushLayer(alpha float32) {
	b := c.img.Rect
	c.layers = append(c.layers, layer{img: image.NewRGBA(b), alpha: min(max(alpha, 0), 1), depth: len(c.states) - 1})
}

// PopLayer draws the innermost layer onto what is below it.
func (c *Canvas) PopLayer() {
	n := len(c.layers)
	if n == 0 {
		return
	}
	l := c.layers[n-1]
	c.layers = c.layers[:n-1]
	dst := c.target()
	for i := 0; i < len(dst.Pix); i += 4 {
		sa := float32(l.img.Pix[i+3]) * l.alpha
		if sa == 0 {
			continue
		}
		keep := 1 - sa/255
		for k := range 4 {
			dst.Pix[i+k] = uint8(float32(l.img.Pix[i+k])*l.alpha + float32(dst.Pix[i+k])*keep + 0.5)
		}
	}
}

// Fill fills p with paint.
func (c *Canvas) Fill(p *Path, paint Paint, rule FillRule) {
	c.draw(c.polys(p, nil), paint, rule)
}

// Stroke draws the outline of p with paint. The width and dashes are in the
// current coordinates, so a scaled transform scales them too.
func (c *Canvas) Stroke(p *Path, paint Paint, s Stroke) {
	c.draw(c.polys(p, &s), paint, NonZero)
}

// polys flattens p in the current coordinates, strokes it when s is set, and
// returns its polygons in pixels. Flattening and stroking make new polygons, so
// they are moved to pixels in place.
func (c *Canvas) polys(p *Path, s *Stroke) [][]Point {
	m := c.top().m
	tol := c.Tolerance
	if tol <= 0 {
		tol = 0.05
	}
	if sc := m.scale(); sc > 0 {
		tol /= sc // flattened before the transform
	}
	lines := flatten(p, tol)
	var polys [][]Point
	if s != nil {
		polys = strokePolygons(lines, *s, tol)
	} else {
		for _, l := range lines {
			polys = append(polys, l.pts)
		}
	}
	if m == Identity {
		return polys
	}
	for _, poly := range polys {
		for j, pt := range poly {
			poly[j] = m.Apply(pt)
		}
	}
	return polys
}

// draw paints the coverage of polys.
func (c *Canvas) draw(polys [][]Point, paint Paint, rule FillRule) {
	st := c.top()
	sh := &c.sh
	if !sh.init(paint, st.m, st.alpha) || len(polys) == 0 {
		return
	}
	dst := c.target()
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	clip := st.clip
	top, bottom := 0, h
	if clip != nil {
		top, bottom = clip.y0, clip.y1
		if top >= bottom {
			return // the clip covers nothing
		}
	}
	y0, y1 := c.r.fill(polys, rule, w, h, top, bottom, c.cov)
	// An opaque solid color that fully covers a pixel replaces what is under it. The
	// blend below then keeps none of the pixel, so its bytes are the color's, and
	// the pixel is written directly: it gives the same bytes as the blend does.
	solid := sh.grad == nil
	opaque := solid && sh.solid[3] == 255
	var px [4]uint8
	for k := range 4 {
		px[k] = uint8(min(sh.solid[k]+0.5, 255))
	}
	for y := y0; y < y1; y++ {
		sp := c.r.rows[y-y0]
		lo, hi := int(sp.x0), int(sp.x1)
		if lo >= hi {
			continue
		}
		row := c.cov[y*w : (y+1)*w]
		// Only the pixels fill set can be covered, and of them only those in the
		// clip's bounds drawn.
		xa, xb := lo, hi
		var cl []float32
		if clip != nil {
			xa, xb = max(lo, clip.x0), min(hi, clip.x1)
			cl = clip.at(y)
		}
		for x := xa; x < xb; x++ {
			cv := row[x]
			if cv == 0 {
				continue
			}
			if cl != nil {
				cv *= cl[x-clip.x0]
				if cv == 0 {
					continue
				}
			}
			if opaque && cv == 1 {
				copy(dst.Pix[y*dst.Stride+x*4:y*dst.Stride+x*4+4], px[:])
				continue
			}
			src := sh.solid
			if !solid {
				src = sh.at(float32(x)+0.5, float32(y)+0.5)
			}
			sa := src[3] * cv
			if sa <= 0 {
				continue
			}
			p := dst.Pix[y*dst.Stride+x*4 : y*dst.Stride+x*4+4]
			keep := 1 - sa/255
			for k := range 4 {
				p[k] = uint8(min(src[k]*cv+float32(p[k])*keep+0.5, 255))
			}
		}
		clear(row[lo:hi]) // leave the buffer zeroed for the next fill
	}
}
