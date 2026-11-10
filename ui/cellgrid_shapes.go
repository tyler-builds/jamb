package ui

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/scene"
)

// A CellGrid draws box drawing (U+2500 to U+257F), block elements (U+2580 to
// U+259F) and braille (U+2800 to U+28FF) as shapes that fill their cells, as
// terminals do, rather than with the font. A font's glyphs for them stop at the
// font's own line height, so where the cell is taller the lines of a frame, a
// table or a graph break between rows. The diagonals (U+2571 to U+2573) are
// left to the font.

// The weight of each arm of a box-drawing character.
const (
	armNone = iota
	armLight
	armHeavy
	armDouble
)

// boxArms holds the arms of each box-drawing character, up, right, down and
// left, indexed by rune - 0x2500. A character without arms (the dashes, arcs
// and diagonals, drawn elsewhere or by the font) has none.
var boxArms = func() (t [128][4]uint8) {
	const table = "" +
		"0101 0202 1010 2020 - - - - - - - - " + // ─ ━ │ ┃, dashes
		"0110 0210 0120 0220 0011 0012 0021 0022 " + // ┌ ┍ ┎ ┏ ┐ ┑ ┒ ┓
		"1100 1200 2100 2200 1001 1002 2001 2002 " + // └ ┕ ┖ ┗ ┘ ┙ ┚ ┛
		"1110 1210 2110 1120 2120 2210 1220 2220 " + // ├ to ┣
		"1011 1012 2011 1021 2021 2012 1022 2022 " + // ┤ to ┫
		"0111 0112 0211 0212 0121 0122 0221 0222 " + // ┬ to ┳
		"1101 1102 1201 1202 2101 2102 2201 2202 " + // ┴ to ┻
		"1111 1112 1211 1212 2111 1121 2121 2112 " + // ┼ to ╃
		"2211 1122 1221 2212 1222 2122 2221 2222 " + // ╄ to ╋
		"- - - - " + // dashes
		"0303 3030 0310 0130 0330 0013 0031 0033 " + // ═ ║ ╒ ╓ ╔ ╕ ╖ ╗
		"1300 3100 3300 1003 3001 3003 1310 3130 " + // ╘ ╙ ╚ ╛ ╜ ╝ ╞ ╟
		"3330 1013 3031 3033 0313 0131 0333 1303 " + // ╠ ╡ ╢ ╣ ╤ ╥ ╦ ╧
		"3101 3303 1313 3131 3333 " + // ╨ ╩ ╪ ╫ ╬
		"- - - - - - - " + // arcs, diagonals
		"0001 1000 0100 0010 0002 2000 0200 0020 " + // ╴ ╵ ╶ ╷ ╸ ╹ ╺ ╻
		"0201 1020 0102 2010" // ╼ ╽ ╾ ╿
	for i, f := range strings.Fields(table) {
		if f != "-" {
			for k := range 4 {
				t[i][k] = f[k] - '0'
			}
		}
	}
	return t
}()

// boxDash is a dashed line: how many dashes, whether it runs down, and whether
// it is heavy.
type boxDash struct {
	n               int
	vertical, heavy bool
}

var boxDashes = map[rune]boxDash{
	0x2504: {3, false, false}, 0x2505: {3, false, true}, 0x2506: {3, true, false}, 0x2507: {3, true, true},
	0x2508: {4, false, false}, 0x2509: {4, false, true}, 0x250A: {4, true, false}, 0x250B: {4, true, true},
	0x254C: {2, false, false}, 0x254D: {2, false, true}, 0x254E: {2, true, false}, 0x254F: {2, true, true},
}

// cellShape returns the rune of a cell's text when the grid draws it as a shape.
func cellShape(s string) (rune, bool) {
	if len(s) != 3 { // every rune drawn as a shape is three bytes of UTF-8
		return 0, false
	}
	r, n := utf8.DecodeRuneInString(s)
	switch {
	case n != 3:
		return 0, false
	case r >= 0x2500 && r <= 0x257F:
		_, dash := boxDashes[r]
		return r, boxArms[r-0x2500] != [4]uint8{} || dash || r >= 0x256D && r <= 0x2570
	case r >= 0x2580 && r <= 0x259F, r >= 0x2800 && r <= 0x28FF:
		return r, true
	}
	return 0, false
}

// drawCellShape draws r, a rune for which cellShape is true, over cell, a
// rectangle in physical pixels whose edges are whole pixels.
func drawCellShape(sc *scene.Scene, r rune, cell scene.Rect, c Color) {
	fill := func(x0, y0, x1, y1 float32, c Color) {
		if x1 > x0 && y1 > y0 {
			sc.AddQuad(scene.Quad{Bounds: scene.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, Background: c.scene()})
		}
	}
	x0, y0, w, h := cell.X, cell.Y, cell.W, cell.H
	light := max(1, round(w/8))
	switch {
	case r >= 0x2800:
		drawBraille(fill, r-0x2800, cell, c)
	case r >= 0x2580:
		drawBlock(fill, r, cell, c)
	case r >= 0x256D && r <= 0x2570:
		drawArc(sc, r, cell, light, c)
	default:
		if d, ok := boxDashes[r]; ok {
			t := light
			if d.heavy {
				t = 2 * light
			}
			// Each dash takes half of its share of the cell, centered in it, so
			// dashes in neighboring cells are evenly spaced.
			length := w
			if d.vertical {
				length = h
			}
			step := length / float32(d.n)
			for i := range d.n {
				a := round(float32(i)*step + step/4)
				b := round(float32(i)*step + 3*step/4)
				if d.vertical {
					x := x0 + floor((w-t)/2)
					fill(x, y0+a, x+t, y0+b, c)
				} else {
					y := y0 + floor((h-t)/2)
					fill(x0+a, y, x0+b, y+t, c)
				}
			}
			return
		}
		drawBoxArms(fill, boxArms[r-0x2500], cell, light, c)
	}
}

// drawBoxArms draws the lines of a box-drawing character from the middle of the
// cell to its edges, so the lines of neighboring cells meet. A light line is
// light pixels thick, a heavy one twice that, and a double line is two light
// lines with a light gap. Where lines meet, each stops at the line it meets,
// and the two lines of a double corner meet as an inner and an outer corner.
func drawBoxArms(fill func(x0, y0, x1, y1 float32, c Color), arms [4]uint8, cell scene.Rect, light float32, c Color) {
	up, right, down, left := arms[0], arms[1], arms[2], arms[3]
	x0, y0, w, h := cell.X, cell.Y, cell.W, cell.H
	x1, y1 := x0+w, y0+h
	thick := func(a uint8) float32 {
		switch a {
		case armLight:
			return light
		case armHeavy:
			return 2 * light
		}
		return 0
	}
	// at returns where a line t thick starts, centered in a cell length n long from o.
	at := func(o, n, t float32) float32 { return o + floor((n-t)/2) }
	// The two lines of a double line: a at xa and ya, b at xb and yb.
	xa, ya := at(x0, w, 3*light), at(y0, h, 3*light)
	xb, yb := xa+2*light, ya+2*light
	midX, midY := x0+floor(w/2), y0+floor(h/2)
	// The single lines each way: the thickest, where the lines across meet them.
	hT, vT := max(thick(left), thick(right)), max(thick(up), thick(down))
	hDouble := left == armDouble || right == armDouble
	vDouble := up == armDouble || down == armDouble

	// stop is where one line of a double arm ends. near is the arm across on the
	// line's own side, far the one on the other side. The line stops at the near
	// arm's nearer line (innerEnd), or else goes on to the far arm's farther line
	// (outerEnd), which makes the corner. -1: no arm across.
	stop := func(near, far uint8, innerEnd, outerEnd float32, single func(t float32) float32) float32 {
		switch {
		case near == armDouble:
			return innerEnd
		case near != armNone:
			return single(thick(near))
		case far == armDouble:
			return outerEnd
		case far != armNone:
			return single(thick(far))
		}
		return -1
	}

	// Up and down.
	if up != armNone {
		if up == armDouble {
			endA := stop(left, right, ya+light, yb+light, func(t float32) float32 { return at(y0, h, t) + t })
			endB := stop(right, left, ya+light, yb+light, func(t float32) float32 { return at(y0, h, t) + t })
			if endA < 0 {
				endA, endB = midY+light, midY+light
			}
			fill(xa, y0, xa+light, endA, c)
			fill(xb, y0, xb+light, endB, c)
		} else {
			t := thick(up)
			x := at(x0, w, t)
			var end float32
			switch {
			case hDouble && left != armNone && right != armNone && down == armNone:
				end = ya + light
			case hDouble && left != armNone && right != armNone:
				end = midY + light
			case hDouble:
				end = yb + light
			case hT > 0:
				end = at(y0, h, hT) + hT
			default:
				end = at(y0, h, t) + t
			}
			fill(x, y0, x+t, end, c)
		}
	}
	if down != armNone {
		if down == armDouble {
			startA := stop(left, right, yb, ya, func(t float32) float32 { return at(y0, h, t) })
			startB := stop(right, left, yb, ya, func(t float32) float32 { return at(y0, h, t) })
			if startA < 0 {
				startA, startB = midY, midY
			}
			fill(xa, startA, xa+light, y1, c)
			fill(xb, startB, xb+light, y1, c)
		} else {
			t := thick(down)
			x := at(x0, w, t)
			var start float32
			switch {
			case hDouble && left != armNone && right != armNone && up == armNone:
				start = yb
			case hDouble && left != armNone && right != armNone:
				start = midY
			case hDouble:
				start = ya
			case hT > 0:
				start = at(y0, h, hT)
			default:
				start = at(y0, h, t)
			}
			fill(x, start, x+t, y1, c)
		}
	}
	// Left and right, the same with the axes swapped.
	if left != armNone {
		if left == armDouble {
			endA := stop(up, down, xa+light, xb+light, func(t float32) float32 { return at(x0, w, t) + t })
			endB := stop(down, up, xa+light, xb+light, func(t float32) float32 { return at(x0, w, t) + t })
			if endA < 0 {
				endA, endB = midX+light, midX+light
			}
			fill(x0, ya, endA, ya+light, c)
			fill(x0, yb, endB, yb+light, c)
		} else {
			t := thick(left)
			y := at(y0, h, t)
			var end float32
			switch {
			case vDouble && up != armNone && down != armNone && right == armNone:
				end = xa + light
			case vDouble && up != armNone && down != armNone:
				end = midX + light
			case vDouble:
				end = xb + light
			case vT > 0:
				end = at(x0, w, vT) + vT
			default:
				end = at(x0, w, t) + t
			}
			fill(x0, y, end, y+t, c)
		}
	}
	if right != armNone {
		if right == armDouble {
			startA := stop(up, down, xb, xa, func(t float32) float32 { return at(x0, w, t) })
			startB := stop(down, up, xb, xa, func(t float32) float32 { return at(x0, w, t) })
			if startA < 0 {
				startA, startB = midX, midX
			}
			fill(startA, ya, x1, ya+light, c)
			fill(startB, yb, x1, yb+light, c)
		} else {
			t := thick(right)
			y := at(y0, h, t)
			var start float32
			switch {
			case vDouble && up != armNone && down != armNone && left == armNone:
				start = xb
			case vDouble && up != armNone && down != armNone:
				start = midX
			case vDouble:
				start = xa
			case vT > 0:
				start = at(x0, w, vT)
			default:
				start = at(x0, w, t)
			}
			fill(start, y, x1, y+t, c)
		}
	}
}

// drawArc draws a rounded corner (╭ ╮ ╯ ╰): a quarter of a rounded border, from
// the middle of one edge to the middle of the next, clipped to the cell.
func drawArc(sc *scene.Scene, r rune, cell scene.Rect, light float32, c Color) {
	x0, y0, w, h := cell.X, cell.Y, cell.W, cell.H
	x1, y1 := x0+w, y0+h
	lx, ly := x0+floor((w-light)/2), y0+floor((h-light)/2) // where the light lines run
	rad := min(x1-lx, y1-ly, lx+light-x0, ly+light-y0)
	q := scene.Quad{BorderColor: c.scene()}
	far := 2 * (w + h) // past the cell, so only one corner shows
	switch r {
	case 0x256D: // ╭ down and right
		q.Bounds = scene.Rect{X: lx, Y: ly, W: far, H: far}
		q.BorderSides, q.Radii = scene.Sides{Top: light, Left: light}, scene.Corners{TL: rad}
	case 0x256E: // ╮ down and left
		q.Bounds = scene.Rect{X: lx + light - far, Y: ly, W: far, H: far}
		q.BorderSides, q.Radii = scene.Sides{Top: light, Right: light}, scene.Corners{TR: rad}
	case 0x256F: // ╯ up and left
		q.Bounds = scene.Rect{X: lx + light - far, Y: ly + light - far, W: far, H: far}
		q.BorderSides, q.Radii = scene.Sides{Bottom: light, Right: light}, scene.Corners{BR: rad}
	case 0x2570: // ╰ up and right
		q.Bounds = scene.Rect{X: lx, Y: ly + light - far, W: far, H: far}
		q.BorderSides, q.Radii = scene.Sides{Bottom: light, Left: light}, scene.Corners{BL: rad}
	}
	sc.PushClip(cell)
	sc.AddQuad(q)
	sc.PopClip()
}

// drawBlock draws a block element: eighths, halves, quadrants and shades.
func drawBlock(fill func(x0, y0, x1, y1 float32, c Color), r rune, cell scene.Rect, c Color) {
	x0, y0, w, h := cell.X, cell.Y, cell.W, cell.H
	x1, y1 := x0+w, y0+h
	frac := func(o, n float32, k int) float32 { return o + round(n*float32(k)/8) }
	switch {
	case r == 0x2580: // ▀ upper half
		fill(x0, y0, x1, frac(y0, h, 4), c)
	case r >= 0x2581 && r <= 0x2588: // ▁ to █, lower eighths
		fill(x0, frac(y0, h, 8-int(r-0x2580)), x1, y1, c)
	case r >= 0x2589 && r <= 0x258F: // ▉ to ▏, left eighths
		fill(x0, y0, frac(x0, w, 8-int(r-0x2588)), y1, c)
	case r == 0x2590: // ▐ right half
		fill(frac(x0, w, 4), y0, x1, y1, c)
	case r >= 0x2591 && r <= 0x2593: // ░ ▒ ▓
		fill(x0, y0, x1, y1, c.Alpha(float32(r-0x2590)/4))
	case r == 0x2594: // ▔ upper eighth
		fill(x0, y0, x1, frac(y0, h, 1), c)
	case r == 0x2595: // ▕ right eighth
		fill(frac(x0, w, 7), y0, x1, y1, c)
	default: // ▖ to ▟, quadrants
		quads := [...]uint8{0x2596 - 0x2596: 4, 8, 1, 1 | 4 | 8, 1 | 8, 1 | 2 | 4, 1 | 2 | 8, 2, 2 | 4, 2 | 4 | 8}
		q := quads[r-0x2596]
		mx, my := frac(x0, w, 4), frac(y0, h, 4)
		if q&1 != 0 {
			fill(x0, y0, mx, my, c)
		}
		if q&2 != 0 {
			fill(mx, y0, x1, my, c)
		}
		if q&4 != 0 {
			fill(x0, my, mx, y1, c)
		}
		if q&8 != 0 {
			fill(mx, my, x1, y1, c)
		}
	}
}

// drawBraille draws the dots of a braille pattern, two columns of four, spread
// evenly over the cell so the dots of neighboring cells line up.
func drawBraille(fill func(x0, y0, x1, y1 float32, c Color), bits rune, cell scene.Rect, c Color) {
	x0, y0, w, h := cell.X, cell.Y, cell.W, cell.H
	s := max(1, round(min(w/4, h/8)))
	// Bits 0 to 2 are the left column's top three dots, 3 to 5 the right's, and
	// 6 and 7 the bottom dots, left and right.
	dots := [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
	for i, d := range dots {
		if bits&(1<<i) == 0 {
			continue
		}
		x := x0 + round(w*float32(2*d[0]+1)/4-s/2)
		y := y0 + round(h*float32(2*d[1]+1)/8-s/2)
		fill(x, y, x+s, y+s, c)
	}
}

func floor(v float32) float32 { return float32(math.Floor(float64(v))) }
