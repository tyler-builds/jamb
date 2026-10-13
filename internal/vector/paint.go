package vector

import "math"

// Color is a straight (not premultiplied) R, G, B, A color from 0 to 1.
type Color struct{ R, G, B, A float32 }

// GradientKind is the shape of a gradient.
type GradientKind uint8

const (
	Linear GradientKind = iota
	Radial
)

// Spread is how a gradient continues past its ends.
type Spread uint8

const (
	SpreadPad     Spread = iota // the end colors continue
	SpreadReflect               // the gradient repeats, mirrored each time
	SpreadRepeat                // the gradient repeats
)

// Stop is a color at Offset (0 to 1) along a gradient.
type Stop struct {
	Offset float32
	Color  Color
}

// Gradient is a linear or radial gradient, in the coordinates of the shape
// it paints, transformed by Matrix (SVG's gradientTransform).
type Gradient struct {
	Kind GradientKind
	// X0, Y0 to X1, Y1 is a linear gradient's line.
	X0, Y0, X1, Y1 float32
	// A radial gradient runs from the focal point FX, FY to the circle of
	// radius R about CX, CY.
	CX, CY, R, FX, FY float32
	Stops             []Stop
	Spread            Spread
	Matrix            Matrix
}

// Paint is what fills or strokes a shape: a color, or a gradient when
// Gradient is set.
type Paint struct {
	Color    Color
	Gradient *Gradient
}

// shader computes premultiplied colors for the pixels of one paint.
type shader struct {
	solid [4]float32 // premultiplied, 0..255
	grad  *Gradient
	inv   Matrix // device to gradient space
	lut   [256][4]float32
	alpha float32
}

func newShader(p Paint, toDevice Matrix, alpha float32) (*shader, bool) {
	s := &shader{alpha: alpha}
	if p.Gradient == nil || len(p.Gradient.Stops) == 0 {
		c := p.Color
		a := c.A * alpha
		if a <= 0 {
			return nil, false
		}
		s.solid = [4]float32{c.R * a * 255, c.G * a * 255, c.B * a * 255, a * 255}
		return s, true
	}
	g := p.Gradient
	if len(g.Stops) == 1 {
		return newShader(Paint{Color: g.Stops[0].Color}, toDevice, alpha)
	}
	m := g.Matrix
	if m == (Matrix{}) {
		m = Identity
	}
	inv, ok := m.Then(toDevice).Invert()
	if !ok {
		return nil, false
	}
	s.grad, s.inv = g, inv
	// The colors along the gradient, interpolated straight, then premultiplied.
	for i := range s.lut {
		t := float32(i) / 255
		c := stopColor(g.Stops, t)
		a := c.A * alpha
		s.lut[i] = [4]float32{c.R * a * 255, c.G * a * 255, c.B * a * 255, a * 255}
	}
	return s, true
}

func stopColor(stops []Stop, t float32) Color {
	if t <= stops[0].Offset {
		return stops[0].Color
	}
	for i := 1; i < len(stops); i++ {
		a, b := stops[i-1], stops[i]
		if t <= b.Offset {
			if b.Offset <= a.Offset {
				return b.Color
			}
			f := (t - a.Offset) / (b.Offset - a.Offset)
			return Color{a.Color.R + (b.Color.R-a.Color.R)*f, a.Color.G + (b.Color.G-a.Color.G)*f,
				a.Color.B + (b.Color.B-a.Color.B)*f, a.Color.A + (b.Color.A-a.Color.A)*f}
		}
	}
	return stops[len(stops)-1].Color
}

// at returns the premultiplied color at the device pixel center x, y.
func (s *shader) at(x, y float32) [4]float32 {
	if s.grad == nil {
		return s.solid
	}
	g := s.grad
	p := s.inv.Apply(Point{x, y})
	var t float32
	if g.Kind == Linear {
		d := Point{g.X1 - g.X0, g.Y1 - g.Y0}
		if l := d.dot(d); l > 0 {
			t = p.sub(Point{g.X0, g.Y0}).dot(d) / l
		}
	} else {
		t = radialT(p, Point{g.CX, g.CY}, Point{g.FX, g.FY}, g.R)
	}
	switch g.Spread {
	case SpreadRepeat:
		t -= float32(math.Floor(float64(t)))
	case SpreadReflect:
		t = float32(math.Abs(math.Mod(float64(t), 2)))
		if t > 1 {
			t = 2 - t
		}
	}
	i := int(min(max(t, 0), 1)*255 + 0.5)
	return s.lut[i]
}

// radialT returns where p lies in a radial gradient from the focal point f to
// the circle of radius r about c: the t whose circle, of radius t*r about
// f + t*(c-f), passes through p.
func radialT(p, c, f Point, r float32) float32 {
	if r <= 0 {
		return 1
	}
	pf, cf := p.sub(f), c.sub(f)
	if cf.dot(cf) < 1e-12 {
		return pf.len() / r
	}
	a := float64(cf.dot(cf) - r*r)
	b := float64(pf.dot(cf))
	cc := float64(pf.dot(pf))
	if math.Abs(a) < 1e-9 {
		if b == 0 {
			return 0
		}
		return float32(cc / (2 * b))
	}
	disc := b*b - a*cc
	if disc < 0 {
		return 0
	}
	// The larger root whose circle has a positive radius.
	t1, t2 := (b+math.Sqrt(disc))/a, (b-math.Sqrt(disc))/a
	return float32(max(t1, t2))
}
