package vector

import (
	"math"
	"testing"
)

var (
	red  = Paint{Color: Color{1, 0, 0, 1}}
	blue = Paint{Color: Color{0, 0, 1, 1}}
)

// alphaAt returns the alpha of the pixel x, y, from 0 to 255.
func alphaAt(c *Canvas, x, y int) uint8 { return c.Image().Pix[y*c.img.Stride+x*4+3] }

func rgbaAt(c *Canvas, x, y int) [4]uint8 {
	p := c.Image().Pix[y*c.img.Stride+x*4:]
	return [4]uint8{p[0], p[1], p[2], p[3]}
}

// coverage returns the total alpha of the canvas in pixels: its painted area.
func coverage(c *Canvas) float64 {
	var sum float64
	pix := c.Image().Pix
	for i := 3; i < len(pix); i += 4 {
		sum += float64(pix[i]) / 255
	}
	return sum
}

func near(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

// A rectangle covers its pixels fully, and a pixel it half covers half.
func TestFillRect(t *testing.T) {
	c := NewCanvas(10, 10)
	var p Path
	p.Rect(2, 2, 5.5, 4)
	c.Fill(&p, red, NonZero)
	if got := rgbaAt(c, 3, 3); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("inside pixel %v", got)
	}
	if a := alphaAt(c, 7, 3); !near(float64(a), 128, 2) {
		t.Errorf("half-covered pixel alpha %d, want about 128", a)
	}
	if a := alphaAt(c, 1, 1); a != 0 {
		t.Errorf("outside pixel alpha %d", a)
	}
	if got := coverage(c); !near(got, 22, 0.1) {
		t.Errorf("area %v, want 22", got)
	}
}

// Two nested squares drawn the same way: nonzero fills the middle, even-odd
// leaves a hole.
func TestFillRules(t *testing.T) {
	var p Path
	p.Rect(0, 0, 10, 10)
	p.Rect(3, 3, 4, 4)
	for _, c := range []struct {
		rule FillRule
		want uint8
	}{{NonZero, 255}, {EvenOdd, 0}} {
		cv := NewCanvas(10, 10)
		cv.Fill(&p, red, c.rule)
		if a := alphaAt(cv, 5, 5); a != c.want {
			t.Errorf("rule %d: middle alpha %d, want %d", c.rule, a, c.want)
		}
		if a := alphaAt(cv, 1, 1); a != 255 {
			t.Errorf("rule %d: outer ring alpha %d", c.rule, a)
		}
	}
}

// A circle's area is close to pi r squared; an ellipse's to pi rx ry.
func TestCircleArea(t *testing.T) {
	c := NewCanvas(60, 40)
	var p Path
	p.Ellipse(30, 20, 25, 15)
	c.Fill(&p, red, NonZero)
	if got, want := coverage(c), math.Pi*25*15; !near(got, want, want*0.01) {
		t.Errorf("ellipse area %v, want %v", got, want)
	}
}

// A horizontal stroke of width 2 covers two rows; square caps lengthen it by
// half its width at each end, butt caps do not.
func TestStrokeCaps(t *testing.T) {
	for _, c := range []struct {
		cap  Cap
		area float64
	}{{CapButt, 20}, {CapSquare, 24}, {CapRound, 20 + math.Pi}} {
		cv := NewCanvas(20, 10)
		var p Path
		p.MoveTo(5, 5)
		p.LineTo(15, 5)
		cv.Stroke(&p, red, Stroke{Width: 2, Cap: c.cap})
		if got := coverage(cv); !near(got, c.area, 0.15) {
			t.Errorf("cap %d: area %v, want %v", c.cap, got, c.area)
		}
	}
}

// A right-angle corner: a miter fills the outer corner square, a bevel half of
// it, and a round join a quarter circle.
func TestStrokeJoins(t *testing.T) {
	area := func(j Join) float64 {
		c := NewCanvas(30, 30)
		var p Path
		p.MoveTo(5, 10)
		p.LineTo(20, 10)
		p.LineTo(20, 25)
		c.Stroke(&p, red, Stroke{Width: 4, Join: j})
		return coverage(c)
	}
	// Two 15x4 rectangles that overlap in a 2x2 square on the inside.
	base := 2*15*4.0 - 2*2
	for _, c := range []struct {
		j    Join
		want float64
	}{{JoinMiter, base + 4}, {JoinBevel, base + 2}, {JoinRound, base + math.Pi}} {
		if got := area(c.j); !near(got, c.want, 0.2) {
			t.Errorf("join %d: area %v, want %v", c.j, got, c.want)
		}
	}
	// The join is on the outer side of the turn either way round.
	c := NewCanvas(30, 30)
	var p Path
	p.MoveTo(20, 25)
	p.LineTo(20, 10)
	p.LineTo(5, 10)
	c.Stroke(&p, red, Stroke{Width: 4, Join: JoinMiter})
	if a := alphaAt(c, 21, 8); a != 255 {
		t.Errorf("the reversed path's miter corner has alpha %d", a)
	}
}

// A sharp corner past the miter limit becomes a bevel.
func TestMiterLimit(t *testing.T) {
	stroke := func(limit float32) float64 {
		c := NewCanvas(60, 40)
		var p Path
		p.MoveTo(5, 30)
		p.LineTo(50, 25)
		p.LineTo(5, 20)
		c.Stroke(&p, red, Stroke{Width: 4, MiterLimit: limit})
		return coverage(c)
	}
	if long, cut := stroke(100), stroke(4); long <= cut+5 {
		t.Errorf("miter area %v, beveled %v: the limit did not cut the miter", long, cut)
	}
}

// Dashes of 2 with gaps of 3 cover 2/5 of a line.
func TestDash(t *testing.T) {
	c := NewCanvas(60, 10)
	var p Path
	p.MoveTo(5, 5)
	p.LineTo(55, 5)
	c.Stroke(&p, red, Stroke{Width: 2, Dash: []float32{2, 3}})
	if got := coverage(c); !near(got, 2*50*2.0/5, 0.3) {
		t.Errorf("dashed area %v, want 40", got)
	}
	if a, b := alphaAt(c, 5, 5), alphaAt(c, 8, 5); a != 255 || b != 0 {
		t.Errorf("first dash alpha %d, first gap alpha %d", a, b)
	}
}

// A linear gradient goes from its first color to its last along its line.
func TestLinearGradient(t *testing.T) {
	c := NewCanvas(100, 4)
	var p Path
	p.Rect(0, 0, 100, 4)
	g := &Gradient{Kind: Linear, X0: 0, X1: 100, Stops: []Stop{{0, Color{1, 0, 0, 1}}, {1, Color{0, 0, 1, 1}}}}
	c.Fill(&p, Paint{Gradient: g}, NonZero)
	l, m, r := rgbaAt(c, 0, 1), rgbaAt(c, 50, 1), rgbaAt(c, 99, 1)
	if l[0] < 250 || r[2] < 250 || !near(float64(m[0]), 127, 4) || !near(float64(m[2]), 128, 4) {
		t.Errorf("left %v, middle %v, right %v", l, m, r)
	}
}

// A radial gradient has its first color at the center and its last at the
// circle, and pads beyond it.
func TestRadialGradient(t *testing.T) {
	c := NewCanvas(40, 40)
	var p Path
	p.Rect(0, 0, 40, 40)
	g := &Gradient{Kind: Radial, CX: 20, CY: 20, FX: 20, FY: 20, R: 10,
		Stops: []Stop{{0, Color{1, 0, 0, 1}}, {1, Color{0, 0, 1, 1}}}}
	c.Fill(&p, Paint{Gradient: g}, NonZero)
	// Pixel centers: 20.5, 20.5 is 0.07 of the radius from the center, and
	// 25.5, 20.5 is 0.55 of it.
	if mid, edge := rgbaAt(c, 20, 20), rgbaAt(c, 2, 2); mid[0] < 230 || edge[2] != 255 {
		t.Errorf("center %v, corner %v", mid, edge)
	}
	if half := rgbaAt(c, 25, 20); !near(float64(half[0]), 115, 4) {
		t.Errorf("half way %v", half)
	}
}

// Drawing outside the clip leaves the canvas empty; Restore lifts the clip.
func TestClip(t *testing.T) {
	c := NewCanvas(20, 20)
	var clip, all Path
	clip.Rect(0, 0, 10, 20)
	all.Rect(0, 0, 20, 20)
	c.Save()
	c.Clip(&clip, NonZero)
	c.Fill(&all, red, NonZero)
	c.Restore()
	if a, b := alphaAt(c, 5, 5), alphaAt(c, 15, 5); a != 255 || b != 0 {
		t.Fatalf("inside the clip %d, outside %d", a, b)
	}
	c.Fill(&all, blue, NonZero)
	if got := rgbaAt(c, 15, 5); got[2] != 255 {
		t.Errorf("after Restore the right half is %v", got)
	}
}

// A layer at half opacity fades what is in it as a whole: shapes that overlap
// inside it are no darker where they overlap.
func TestLayer(t *testing.T) {
	c := NewCanvas(20, 10)
	var a, b Path
	a.Rect(0, 0, 12, 10)
	b.Rect(8, 0, 12, 10)
	c.PushLayer(0.5)
	c.Fill(&a, red, NonZero)
	c.Fill(&b, red, NonZero)
	c.PopLayer()
	if one, both := alphaAt(c, 2, 5), alphaAt(c, 10, 5); !near(float64(one), 128, 1) || one != both {
		t.Errorf("alpha %d where one shape is, %d where both are; want both about 128", one, both)
	}
}

// Transforms move and scale what is drawn, strokes included.
func TestTransform(t *testing.T) {
	c := NewCanvas(40, 40)
	c.Transform(Translate(10, 10))
	c.Transform(Scale(2, 2))
	var p Path
	p.Rect(0, 0, 5, 5)
	c.Fill(&p, red, NonZero)
	if got := coverage(c); !near(got, 100, 0.1) || alphaAt(c, 9, 9) != 0 || alphaAt(c, 19, 19) != 255 {
		t.Errorf("area %v of a 5x5 square scaled by 2 at 10, 10", got)
	}
	m, ok := Rotate(0.3).Then(Translate(4, 5)).Invert()
	if !ok {
		t.Fatal("rotation not invertible")
	}
	q := Rotate(0.3).Then(Translate(4, 5)).Then(m).Apply(Point{7, 3})
	if !near(float64(q.X), 7, 1e-4) || !near(float64(q.Y), 3, 1e-4) {
		t.Errorf("inverse gave %v", q)
	}
}

// SVG's arc command: a half circle of radius 10 from 0, 10 to 20, 10 has the
// area of half a disc when closed, on the side the sweep flag picks.
func TestArcTo(t *testing.T) {
	for _, sweep := range []bool{true, false} {
		c := NewCanvas(40, 40)
		var p Path
		p.MoveTo(10, 20)
		p.ArcTo(10, 10, 0, false, sweep, 30, 20)
		p.Close()
		c.Fill(&p, red, NonZero)
		if got, want := coverage(c), math.Pi*100/2; !near(got, want, 1) {
			t.Errorf("sweep %v: area %v, want %v", sweep, got, want)
		}
		above := alphaAt(c, 20, 15) == 255
		if above != sweep {
			t.Errorf("sweep %v drew above the chord: %v", sweep, above)
		}
	}
	// Radii too small to reach are scaled up.
	var p Path
	p.MoveTo(0, 0)
	p.ArcTo(1, 1, 0, false, true, 20, 0)
	if cur, _ := p.Current(); cur != (Point{20, 0}) {
		t.Errorf("arc ended at %v", cur)
	}
}

// Arc draws a full circle for a full turn, and joins the current point to its
// start.
func TestArc(t *testing.T) {
	c := NewCanvas(40, 40)
	var p Path
	p.Arc(20, 20, 10, 0, 2*math.Pi, false)
	c.Fill(&p, red, NonZero)
	if got, want := coverage(c), math.Pi*100; !near(got, want, want*0.01) {
		t.Errorf("full arc area %v, want %v", got, want)
	}
	var q Path
	q.MoveTo(20, 20)
	q.Arc(20, 20, 10, 0, math.Pi/2, false)
	q.Close()
	c = NewCanvas(40, 40)
	c.Fill(&q, red, NonZero)
	if got, want := coverage(c), math.Pi*100/4; !near(got, want, 1) {
		t.Errorf("quarter pie area %v, want %v", got, want)
	}
	if alphaAt(c, 25, 25) != 255 {
		t.Error("the quarter is not below and right of the center")
	}
}

// Degenerate input draws nothing and does not panic.
func TestDegenerate(t *testing.T) {
	c := NewCanvas(10, 10)
	var p Path
	p.MoveTo(float32(math.NaN()), 2)
	p.LineTo(5, 5)
	p.LineTo(5, 5)
	c.Fill(&p, red, NonZero)
	c.Stroke(&p, red, Stroke{Width: 0})
	var empty Path
	c.Fill(&empty, red, NonZero)
	c.Stroke(&empty, red, Stroke{Width: 3, Dash: []float32{0, 0}})
	if got := coverage(c); got != 0 {
		t.Errorf("area %v", got)
	}
	// A dot with round caps is a disc.
	var dot Path
	dot.MoveTo(5, 5)
	dot.LineTo(5, 5)
	c.Stroke(&dot, red, Stroke{Width: 4, Cap: CapRound})
	if got := coverage(c); !near(got, 4*math.Pi, 0.3) {
		t.Errorf("round dot area %v", got)
	}
	_ = NewCanvas(0, 0).Image()
}
