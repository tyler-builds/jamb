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
	cov    []float32
	sh     shader // the shader of the fill being drawn (see draw)
	// Tolerance is how far, in pixels, flattened curves may stray; 0 means
	// 0.05, which loses no visible area to the straight segments.
	Tolerance float32
}

type state struct {
	m     Matrix
	clip  []float32 // coverage of the clip, w*h; nil when nothing is clipped
	alpha float32
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
	c.states = c.states[:len(c.states)-1]
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
	mask := make([]float32, w*h)
	c.r.fill(c.polys(p, nil), rule, w, h, mask)
	if old := c.top().clip; old != nil {
		for i := range mask {
			mask[i] *= old[i]
		}
	}
	c.top().clip = mask
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
// returns its polygons in pixels.
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
	out := make([][]Point, len(polys))
	for i, poly := range polys {
		q := make([]Point, len(poly))
		for j, pt := range poly {
			q[j] = m.Apply(pt)
		}
		out[i] = q
	}
	return out
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
	y0, y1 := c.r.fill(polys, rule, w, h, c.cov)
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
		row := c.cov[y*w : (y+1)*w]
		for x, cv := range row {
			if cv == 0 {
				continue
			}
			row[x] = 0 // leave the buffer zeroed for the next fill
			if st.clip != nil {
				cv *= st.clip[y*w+x]
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
	}
}
