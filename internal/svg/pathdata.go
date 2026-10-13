package svg

import (
	"strconv"

	"github.com/tyler-builds/jamb/internal/vector"
)

// numbers reads the numbers of an SVG list ("1,2 3-4.5.5e1"): separated by
// commas or spaces, or by a sign, or by a second decimal point.
type numbers struct {
	s string
	i int
}

func (n *numbers) skip() {
	for n.i < len(n.s) && (n.s[n.i] == ' ' || n.s[n.i] == ',' || n.s[n.i] == '\t' || n.s[n.i] == '\n' || n.s[n.i] == '\r') {
		n.i++
	}
}

// more reports whether a number comes next.
func (n *numbers) more() bool {
	n.skip()
	if n.i >= len(n.s) {
		return false
	}
	c := n.s[n.i]
	return c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9')
}

// next returns the next number, and false when there is none.
func (n *numbers) next() (float32, bool) {
	if !n.more() {
		return 0, false
	}
	start := n.i
	if c := n.s[n.i]; c == '-' || c == '+' {
		n.i++
	}
	dot, digits := false, false
	for n.i < len(n.s) {
		c := n.s[n.i]
		switch {
		case c >= '0' && c <= '9':
			digits = true
		case c == '.' && !dot:
			dot = true
		case (c == 'e' || c == 'E') && digits && n.i+1 < len(n.s) && (n.s[n.i+1] == '-' || n.s[n.i+1] == '+' || (n.s[n.i+1] >= '0' && n.s[n.i+1] <= '9')):
			n.i++
			if n.s[n.i] == '-' || n.s[n.i] == '+' {
				n.i++
			}
			for n.i < len(n.s) && n.s[n.i] >= '0' && n.s[n.i] <= '9' {
				n.i++
			}
			v, err := strconv.ParseFloat(n.s[start:n.i], 32)
			return float32(v), err == nil
		default:
			v, err := strconv.ParseFloat(n.s[start:n.i], 32)
			return float32(v), err == nil && digits
		}
		n.i++
	}
	v, err := strconv.ParseFloat(n.s[start:n.i], 32)
	return float32(v), err == nil && digits
}

// flag reads an arc flag, which may be written without a separator after it
// ("a1 1 0 001 1").
func (n *numbers) flag() (bool, bool) {
	n.skip()
	if n.i >= len(n.s) || (n.s[n.i] != '0' && n.s[n.i] != '1') {
		return false, false
	}
	f := n.s[n.i] == '1'
	n.i++
	return f, true
}

// parsePath reads SVG path data into p. As browsers do, it keeps what it read
// before an error.
func parsePath(d string, p *vector.Path) {
	n := &numbers{s: d}
	var cmd byte
	var cur, start, ctrl vector.Point // ctrl: the last control point, for S and T
	var lastCmd byte
	for {
		n.skip()
		if n.i >= len(n.s) {
			return
		}
		c := n.s[n.i]
		if isCommand(c) {
			cmd = c
			n.i++
		} else if cmd == 0 || !n.more() {
			return
		}
		rel := cmd >= 'a'
		at := func(x, y float32) vector.Point {
			if rel {
				return vector.Point{X: cur.X + x, Y: cur.Y + y}
			}
			return vector.Point{X: x, Y: y}
		}
		read := func(k int) ([6]float32, bool) {
			var v [6]float32
			for i := range k {
				x, ok := n.next()
				if !ok {
					return v, false
				}
				v[i] = x
			}
			return v, true
		}
		switch cmd | 0x20 {
		case 'm':
			v, ok := read(2)
			if !ok {
				return
			}
			cur = at(v[0], v[1])
			start = cur
			p.MoveTo(cur.X, cur.Y)
			// Pairs after the first are lines.
			if cmd == 'm' {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'l':
			v, ok := read(2)
			if !ok {
				return
			}
			cur = at(v[0], v[1])
			p.LineTo(cur.X, cur.Y)
		case 'h':
			v, ok := read(1)
			if !ok {
				return
			}
			if rel {
				cur.X += v[0]
			} else {
				cur.X = v[0]
			}
			p.LineTo(cur.X, cur.Y)
		case 'v':
			v, ok := read(1)
			if !ok {
				return
			}
			if rel {
				cur.Y += v[0]
			} else {
				cur.Y = v[0]
			}
			p.LineTo(cur.X, cur.Y)
		case 'c':
			v, ok := read(6)
			if !ok {
				return
			}
			c1, c2, e := at(v[0], v[1]), at(v[2], v[3]), at(v[4], v[5])
			p.CubicTo(c1.X, c1.Y, c2.X, c2.Y, e.X, e.Y)
			ctrl, cur = c2, e
		case 's':
			v, ok := read(4)
			if !ok {
				return
			}
			c1 := cur
			if l := lastCmd | 0x20; l == 'c' || l == 's' {
				c1 = vector.Point{X: 2*cur.X - ctrl.X, Y: 2*cur.Y - ctrl.Y}
			}
			c2, e := at(v[0], v[1]), at(v[2], v[3])
			p.CubicTo(c1.X, c1.Y, c2.X, c2.Y, e.X, e.Y)
			ctrl, cur = c2, e
		case 'q':
			v, ok := read(4)
			if !ok {
				return
			}
			c1, e := at(v[0], v[1]), at(v[2], v[3])
			p.QuadTo(c1.X, c1.Y, e.X, e.Y)
			ctrl, cur = c1, e
		case 't':
			v, ok := read(2)
			if !ok {
				return
			}
			c1 := cur
			if l := lastCmd | 0x20; l == 'q' || l == 't' {
				c1 = vector.Point{X: 2*cur.X - ctrl.X, Y: 2*cur.Y - ctrl.Y}
			}
			e := at(v[0], v[1])
			p.QuadTo(c1.X, c1.Y, e.X, e.Y)
			ctrl, cur = c1, e
		case 'a':
			v, ok := read(3)
			if !ok {
				return
			}
			large, ok1 := n.flag()
			sweep, ok2 := n.flag()
			e, ok3 := read(2)
			if !ok1 || !ok2 || !ok3 {
				return
			}
			end := at(e[0], e[1])
			p.ArcTo(v[0], v[1], v[2], large, sweep, end.X, end.Y)
			cur = end
		case 'z':
			p.Close()
			cur = start
			lastCmd = cmd
			cmd = 0 // a number after Z is an error
			continue
		default:
			return
		}
		lastCmd = cmd
	}
}

func isCommand(c byte) bool {
	switch c | 0x20 {
	case 'm', 'l', 'h', 'v', 'c', 's', 'q', 't', 'a', 'z':
		return c != 'e' && c != 'E'
	}
	return false
}
