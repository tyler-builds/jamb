package icons

import "math"

// The built-in icon set. Add icons by composing builder calls on the 24×24 grid.
var (
	Dashboard = newIcon("dashboard", func(b *builder) {
		b.RoundRect(3, 3, 7, 9, 1.5)
		b.RoundRect(14, 3, 7, 5, 1.5)
		b.RoundRect(14, 12, 7, 9, 1.5)
		b.RoundRect(3, 16, 7, 5, 1.5)
	})

	Folder = newIcon("folder", func(b *builder) {
		b.M(4, 20).L(20, 20)
		b.Arc(20, 18, 2, 90, 0)
		b.L(22, 9)
		b.Arc(20, 9, 2, 0, -90)
		b.L(12, 7).L(10, 4).L(4, 4)
		b.Arc(4, 6, 2, -90, -180)
		b.L(2, 18)
		b.Arc(4, 18, 2, 180, 90)
	})

	Activity = newIcon("activity", func(b *builder) {
		b.M(22, 12).L(18, 12).L(15, 21).L(9, 3).L(6, 12).L(2, 12)
	})

	Settings = newIcon("settings", func(b *builder) {
		// A gear: eight teeth around a ring with a hub.
		b.Circle(12, 12, 3)
		const rIn, rOut = 6.5, 9.5
		var pts []pt
		for i := 0; i < 8; i++ {
			c := float64(i) * math.Pi / 4
			for _, d := range []float64{-0.32, -0.2, 0.2, 0.32} {
				r := float32(rIn)
				if d > -0.25 && d < 0.25 {
					r = rOut
				}
				a := c + d
				pts = append(pts, pt{12 + r*float32(math.Cos(a)), 12 + r*float32(math.Sin(a))})
			}
		}
		b.M(pts[0].x, pts[0].y)
		for _, p := range pts[1:] {
			b.L(p.x, p.y)
		}
		b.Z()
	})

	Plus = newIcon("plus", func(b *builder) {
		b.Line(12, 5, 12, 19).Line(5, 12, 19, 12)
	})

	Search = newIcon("search", func(b *builder) {
		b.Circle(11, 11, 7).Line(21, 21, 16, 16)
	})

	ChevronRight = newIcon("chevron-right", func(b *builder) {
		b.M(9, 18).L(15, 12).L(9, 6)
	})

	ChevronDown = newIcon("chevron-down", func(b *builder) {
		b.M(6, 9).L(12, 15).L(18, 9)
	})

	Check = newIcon("check", func(b *builder) {
		b.M(20, 6).L(9, 17).L(4, 12)
	})

	X = newIcon("x", func(b *builder) {
		b.Line(18, 6, 6, 18).Line(6, 6, 18, 18)
	})

	Star = newIcon("star", func(b *builder) {
		b.M(12, 2)
		for i := 1; i <= 10; i++ {
			a := float64(i)*math.Pi/5 - math.Pi/2
			r := float32(10)
			if i%2 == 1 {
				r = 4.2
			}
			b.L(12+r*float32(math.Cos(a)), 12.5+r*float32(math.Sin(a)))
		}
		b.Z()
	})

	Clock = newIcon("clock", func(b *builder) {
		b.Circle(12, 12, 10).M(12, 6).L(12, 12).L(16, 14)
	})

	Sun = newIcon("sun", func(b *builder) {
		b.Circle(12, 12, 4)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			c, s := float32(math.Cos(a)), float32(math.Sin(a))
			b.Line(12+c*7.5, 12+s*7.5, 12+c*9.5, 12+s*9.5)
		}
	})

	Moon = newIcon("moon", func(b *builder) {
		// Crescent: the part of circle A outside circle B.
		ax, ay, ar := float32(12), float32(12), float32(9)
		bx, by, br := float32(17), float32(7), float32(7.5)
		a1, a2 := circleIntersections(ax, ay, ar, bx, by, br)
		b1, b2 := circleIntersections(bx, by, br, ax, ay, ar)
		// Outer arc of A goes the long way round, inner arc of B the short way back.
		b.M(ax+ar*cosd(a2), ay+ar*sind(a2))
		b.Arc(ax, ay, ar, a2, a1+360*sign(a2-a1))
		b.Arc(bx, by, br, b2, b1)
		b.Z()
	})

	Bell = newIcon("bell", func(b *builder) {
		b.M(6, 8)
		b.Arc(12, 8, 6, 180, 360)
		b.L(18, 13).L(20, 17).L(4, 17).L(6, 13).Z()
		b.M(10.3, 21).Arc(12, 20, 2, 150, 30)
	})

	Users = newIcon("users", func(b *builder) {
		b.Circle(9, 7, 4)
		b.M(2, 21).L(2, 19).Arc(6, 19, 4, 180, 270).L(12, 15).Arc(12, 19, 4, 270, 360).L(16, 21)
		b.M(16, 3.1).Arc(16, 7, 4, -75, 75)
		b.M(19, 15.2).Arc(18, 19, 4, -70, 0).L(22, 21)
	})

	MoreHorizontal = newIcon("more-horizontal", func(b *builder) {
		b.Dot(5, 12, 1.6).Dot(12, 12, 1.6).Dot(19, 12, 1.6)
	})

	Trash = newIcon("trash", func(b *builder) {
		b.Line(3, 6, 21, 6)
		b.M(19, 6).L(19, 20).Arc(17, 20, 2, 0, 90).L(7, 22).Arc(7, 20, 2, 90, 180).L(5, 6)
		b.M(8, 6).L(8, 4).Arc(10, 4, 2, 180, 270).L(14, 2).Arc(14, 4, 2, 270, 360).L(16, 6)
	})

	Inbox = newIcon("inbox", func(b *builder) {
		b.M(22, 12).L(16, 12).L(14, 15).L(10, 15).L(8, 12).L(2, 12)
		b.M(5.5, 5).L(2, 12).L(2, 18).Arc(4, 18, 2, 180, 90).L(20, 20).Arc(20, 18, 2, 90, 0).L(22, 12).L(18.5, 5).Z()
	})

	Calendar = newIcon("calendar", func(b *builder) {
		b.RoundRect(3, 4, 18, 18, 2)
		b.Line(16, 2, 16, 6).Line(8, 2, 8, 6).Line(3, 10, 21, 10)
	})

	GitBranch = newIcon("git-branch", func(b *builder) {
		b.Line(6, 3, 6, 15)
		b.Circle(18, 6, 3).Circle(6, 18, 3)
		b.M(18, 9).Arc(9, 9, 9, 0, 90)
	})

	Zap = newIcon("zap", func(b *builder) {
		b.M(13, 2).L(3, 14).L(12, 14).L(11, 22).L(21, 10).L(12, 10).Z()
	})

	Layers = newIcon("layers", func(b *builder) {
		b.M(12, 2).L(2, 7).L(12, 12).L(22, 7).Z()
		b.M(2, 17).L(12, 22).L(22, 17)
		b.M(2, 12).L(12, 17).L(22, 12)
	})

	Box = newIcon("box", func(b *builder) {
		b.M(12, 2).L(21, 7).L(21, 17).L(12, 22).L(3, 17).L(3, 7).Z()
		b.M(3, 7).L(12, 12).L(21, 7)
		b.Line(12, 12, 12, 22)
	})

	Filter = newIcon("filter", func(b *builder) {
		b.M(22, 3).L(2, 3).L(10, 12.5).L(10, 19).L(14, 21).L(14, 12.5).Z()
	})

	ArrowUpRight = newIcon("arrow-up-right", func(b *builder) {
		b.Line(7, 17, 17, 7).M(7, 7).L(17, 7).L(17, 17)
	})

	Circle = newIcon("circle", func(b *builder) {
		b.Circle(12, 12, 9)
	})
)

func cosd(a float32) float32 { return float32(math.Cos(float64(a) * math.Pi / 180)) }
func sind(a float32) float32 { return float32(math.Sin(float64(a) * math.Pi / 180)) }
func sign(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

// circleIntersections returns the angles (degrees, on circle A) of the two
// intersection points of circles A and B.
func circleIntersections(ax, ay, ar, bx, by, br float32) (float32, float32) {
	dx, dy := float64(bx-ax), float64(by-ay)
	d := math.Hypot(dx, dy)
	a := (float64(ar*ar) - float64(br*br) + d*d) / (2 * d)
	base := math.Atan2(dy, dx)
	off := math.Acos(a / float64(ar))
	return float32((base - off) * 180 / math.Pi), float32((base + off) * 180 / math.Pi)
}
