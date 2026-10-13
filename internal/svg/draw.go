package svg

import (
	"math"
	"strconv"
	"strings"

	"github.com/tyler-builds/jamb/internal/vector"
)

// style is the inherited state of the properties that pass from an element
// to its children.
type style struct {
	fill, stroke       string // a paint: none, a color, currentColor or url(#id)
	fillOpacity        float32
	strokeOpacity      float32
	fillRule, clipRule vector.FillRule
	strokeWidth        float32
	cap                vector.Cap
	join               vector.Join
	miter              float32
	dash               []float32
	dashOffset         float32
	color              vector.Color // currentColor
	visible            bool
}

// inherited lists the properties that inherit; the others (opacity,
// transform, clip-path, display) apply to the element itself.
func (s style) with(n *node, d *Doc) style {
	a := n.attrs
	if v, ok := a["color"]; ok && v != "inherit" {
		if c, ok := parseColor(v, s.color); ok {
			s.color = c
		}
	}
	if v, ok := a["fill"]; ok && v != "inherit" {
		s.fill = v
	}
	if v, ok := a["stroke"]; ok && v != "inherit" {
		s.stroke = v
	}
	if v, ok := a["fill-opacity"]; ok {
		s.fillOpacity = opacity(v, s.fillOpacity)
	}
	if v, ok := a["stroke-opacity"]; ok {
		s.strokeOpacity = opacity(v, s.strokeOpacity)
	}
	if v, ok := a["fill-rule"]; ok {
		s.fillRule = parseRule(v, s.fillRule)
	}
	if v, ok := a["clip-rule"]; ok {
		s.clipRule = parseRule(v, s.clipRule)
	}
	if v, ok := a["stroke-width"]; ok {
		if w, ok := length(v, d.diag()); ok && w >= 0 {
			s.strokeWidth = w
		}
	}
	switch a["stroke-linecap"] {
	case "butt":
		s.cap = vector.CapButt
	case "round":
		s.cap = vector.CapRound
	case "square":
		s.cap = vector.CapSquare
	}
	switch a["stroke-linejoin"] {
	case "miter", "miter-clip", "arcs":
		s.join = vector.JoinMiter
	case "round":
		s.join = vector.JoinRound
	case "bevel":
		s.join = vector.JoinBevel
	}
	if v, ok := a["stroke-miterlimit"]; ok {
		if m, err := strconv.ParseFloat(strings.TrimSpace(v), 32); err == nil && m >= 1 {
			s.miter = float32(m)
		}
	}
	if v, ok := a["stroke-dasharray"]; ok {
		s.dash = nil
		if v != "none" {
			n := &numbers{s: strings.ReplaceAll(v, "px", "")}
			for {
				x, ok := n.next()
				if !ok {
					break
				}
				s.dash = append(s.dash, x)
			}
		}
	}
	if v, ok := a["stroke-dashoffset"]; ok {
		if o, ok := length(v, d.diag()); ok {
			s.dashOffset = o
		}
	}
	switch a["visibility"] {
	case "hidden", "collapse":
		s.visible = false
	case "visible":
		s.visible = true
	}
	return s
}

func opacity(v string, def float32) float32 {
	v = strings.TrimSpace(v)
	k := float32(1)
	if strings.HasSuffix(v, "%") {
		v, k = strings.TrimSuffix(v, "%"), 0.01
	}
	f, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return def
	}
	return min(max(float32(f)*k, 0), 1)
}

func parseRule(v string, def vector.FillRule) vector.FillRule {
	switch strings.TrimSpace(v) {
	case "evenodd":
		return vector.EvenOdd
	case "nonzero":
		return vector.NonZero
	}
	return def
}

// diag is the reference length for percentages that are neither widths nor
// heights, as SVG defines it.
func (d *Doc) diag() float32 {
	w, h := d.vbW, d.vbH
	if !d.hasViewBox {
		w, h = d.W, d.H
	}
	return float32(math.Sqrt(float64(w*w+h*h) / 2))
}

// Draw draws the image into c, fitted into a box of w x h in c's current
// coordinates as its preserveAspectRatio says. current is the color that
// currentColor stands for.
func (d *Doc) Draw(c *vector.Canvas, w, h float32, current vector.Color) {
	c.Save()
	defer c.Restore()
	if d.hasViewBox {
		sx, sy := w/d.vbW, h/d.vbH
		tx, ty := float32(0), float32(0)
		if d.align != "none" {
			s := min(sx, sy)
			if d.slice {
				s = max(sx, sy)
				var clip vector.Path
				clip.Rect(0, 0, w, h)
				c.Clip(&clip, vector.NonZero)
			}
			sx, sy = s, s
			ax, ay := alignFactor(d.align)
			tx, ty = (w-d.vbW*s)*ax, (h-d.vbH*s)*ay
		}
		c.Transform(vector.Translate(-d.vbX, -d.vbY).Then(vector.Scale(sx, sy)).Then(vector.Translate(tx, ty)))
	} else if d.W > 0 && d.H > 0 {
		c.Transform(vector.Scale(w/d.W, h/d.H))
	}
	st := style{fill: "black", stroke: "none", fillOpacity: 1, strokeOpacity: 1, strokeWidth: 1, miter: 4,
		color: current, visible: true}
	r := &renderer{d: d, c: c}
	r.children(d.root, st.with(d.root, d), 0)
}

// alignFactor returns where preserveAspectRatio's alignment puts the image in
// the spare room: 0 at the start, 0.5 in the middle, 1 at the end.
func alignFactor(a string) (float32, float32) {
	f := func(s string) float32 {
		switch s {
		case "Min":
			return 0
		case "Max":
			return 1
		}
		return 0.5
	}
	if len(a) != 8 || a[0] != 'x' || a[4] != 'Y' {
		return 0.5, 0.5
	}
	return f(a[1:4]), f(a[5:8])
}

type renderer struct {
	d *Doc
	c *vector.Canvas
}

// maxDepth limits use and nesting, so a document that refers to itself ends.
const maxDepth = 32

func (r *renderer) children(n *node, st style, depth int) {
	for _, c := range n.children {
		r.element(c, st, depth+1)
	}
}

// element draws n and its children with the inherited style st.
func (r *renderer) element(n *node, st style, depth int) {
	if depth > maxDepth || n.attr("display") == "none" {
		return
	}
	switch n.name {
	case "defs", "linearGradient", "radialGradient", "clipPath", "mask", "pattern", "marker", "symbol",
		"style", "title", "desc", "metadata", "text", "image", "filter", "script", "foreignObject":
		return
	}
	st = st.with(n, r.d)
	c := r.c
	c.Save()
	defer c.Restore()
	if t := n.attr("transform"); t != "" {
		c.Transform(parseTransform(t))
	}
	if n.name == "use" {
		x, _ := length(n.attr("x"), r.d.vbW)
		y, _ := length(n.attr("y"), r.d.vbH)
		c.Transform(vector.Translate(x, y))
	}
	if cp := urlID(n.attr("clip-path")); cp != "" {
		if !r.clip(cp, depth) {
			return
		}
	}
	layered := false
	if v, ok := n.attrs["opacity"]; ok {
		if o := opacity(v, 1); o < 1 {
			if o == 0 {
				return
			}
			c.PushLayer(o)
			layered = true
		}
	}
	switch n.name {
	case "svg", "g", "a", "switch":
		r.children(n, st, depth)
	case "use":
		if ref := r.d.ids[strings.TrimPrefix(n.attr("href"), "#")]; ref != nil && ref != n {
			if ref.name == "symbol" {
				r.children(ref, st.with(ref, r.d), depth)
			} else {
				r.element(ref, st, depth+1)
			}
		}
	default:
		if p := r.shape(n); p != nil && st.visible {
			r.paint(p, st)
		}
	}
	if layered {
		c.PopLayer()
	}
}

// shape returns the outline of a shape element, or nil.
func (r *renderer) shape(n *node) *vector.Path {
	d := r.d
	num := func(k string, ref float32) float32 {
		v, _ := length(n.attr(k), ref)
		return v
	}
	vw, vh := d.vbW, d.vbH
	if !d.hasViewBox {
		vw, vh = d.W, d.H
	}
	p := &vector.Path{}
	switch n.name {
	case "path":
		parsePath(n.attr("d"), p)
	case "rect":
		x, y, w, h := num("x", vw), num("y", vh), num("width", vw), num("height", vh)
		if w <= 0 || h <= 0 {
			return nil
		}
		rx, okx := length(n.attr("rx"), vw)
		ry, oky := length(n.attr("ry"), vh)
		switch {
		case okx && !oky:
			ry = rx
		case oky && !okx:
			rx = ry
		}
		p.RoundRect(x, y, w, h, rx, ry)
	case "circle":
		rad := num("r", d.diag())
		if rad <= 0 {
			return nil
		}
		p.Ellipse(num("cx", vw), num("cy", vh), rad, rad)
	case "ellipse":
		rx, ry := num("rx", vw), num("ry", vh)
		if rx <= 0 || ry <= 0 {
			return nil
		}
		p.Ellipse(num("cx", vw), num("cy", vh), rx, ry)
	case "line":
		p.MoveTo(num("x1", vw), num("y1", vh))
		p.LineTo(num("x2", vw), num("y2", vh))
	case "polyline", "polygon":
		ns := &numbers{s: n.attr("points")}
		for i := 0; ; i++ {
			x, ok1 := ns.next()
			y, ok2 := ns.next()
			if !ok1 || !ok2 {
				break
			}
			if i == 0 {
				p.MoveTo(x, y)
			} else {
				p.LineTo(x, y)
			}
		}
		if n.name == "polygon" {
			p.Close()
		}
	default:
		return nil
	}
	if p.Empty() {
		return nil
	}
	return p
}

// paint fills, then strokes, the outline p.
func (r *renderer) paint(p *vector.Path, st style) {
	if fill, ok := r.paintOf(st.fill, st.fillOpacity, st, p); ok {
		r.c.Fill(p, fill, st.fillRule)
	}
	if st.strokeWidth > 0 {
		if stroke, ok := r.paintOf(st.stroke, st.strokeOpacity, st, p); ok {
			r.c.Stroke(p, stroke, vector.Stroke{Width: st.strokeWidth, Cap: st.cap, Join: st.join,
				MiterLimit: st.miter, Dash: st.dash, DashOffset: st.dashOffset})
		}
	}
}

// paintOf resolves a fill or stroke value, faded by op. A gradient whose
// units are the bounding box is mapped onto p's bounds.
func (r *renderer) paintOf(v string, op float32, st style, p *vector.Path) (vector.Paint, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "none" || op <= 0 {
		return vector.Paint{}, false
	}
	if id := urlID(v); id != "" {
		if g, ok := r.gradient(id, op, p); ok {
			return g, true
		}
		// The fallback after the URL, if any.
		rest := strings.TrimSpace(v[strings.IndexByte(v, ')')+1:])
		return r.paintOf(rest, op, st, p)
	}
	c, ok := parseColor(v, st.color)
	if !ok {
		return vector.Paint{}, false
	}
	c.A *= op
	return vector.Paint{Color: c}, true
}

// gradient builds the gradient with id, following href for the stops and
// attributes it does not set itself.
func (r *renderer) gradient(id string, op float32, p *vector.Path) (vector.Paint, bool) {
	n := r.d.ids[id]
	if n == nil || (n.name != "linearGradient" && n.name != "radialGradient") {
		return vector.Paint{}, false
	}
	// get reads an attribute from the gradient or the ones it refers to.
	chain := []*node{n}
	for i := 0; i < 8; i++ {
		ref := r.d.ids[strings.TrimPrefix(chain[len(chain)-1].attr("href"), "#")]
		if ref == nil || (ref.name != "linearGradient" && ref.name != "radialGradient") {
			break
		}
		chain = append(chain, ref)
	}
	get := func(k string) (string, bool) {
		for _, g := range chain {
			if v, ok := g.attrs[k]; ok {
				return v, true
			}
		}
		return "", false
	}
	var stopsOf *node
	for _, g := range chain {
		for _, c := range g.children {
			if c.name == "stop" {
				stopsOf = g
				break
			}
		}
		if stopsOf != nil {
			break
		}
	}
	if stopsOf == nil {
		return vector.Paint{}, false // no stops paints nothing
	}
	var stops []vector.Stop
	last := float32(0)
	for _, s := range stopsOf.children {
		if s.name != "stop" {
			continue
		}
		off := opacity(s.attr("offset"), 0)
		off = max(off, last) // offsets never go back
		last = off
		col, ok := parseColor(s.attr("stop-color"), vector.Color{A: 1})
		if !ok {
			col = vector.Color{A: 1}
		}
		if v, ok := s.attrs["stop-opacity"]; ok {
			col.A *= opacity(v, 1)
		}
		col.A *= op
		stops = append(stops, vector.Stop{Offset: off, Color: col})
	}
	if len(stops) == 0 {
		return vector.Paint{}, false
	}
	bbox := true
	if u, _ := get("gradientUnits"); u == "userSpaceOnUse" {
		bbox = false
	}
	ref := func(k string, def string, size float32) float32 {
		v, ok := get(k)
		if !ok {
			v = def
		}
		if bbox {
			if strings.HasSuffix(strings.TrimSpace(v), "%") {
				return opacity(v, 0) // a fraction of the box
			}
			f, _ := length(v, 1)
			return f
		}
		f, _ := length(v, size)
		return f
	}
	vw, vh := r.d.vbW, r.d.vbH
	if !r.d.hasViewBox {
		vw, vh = r.d.W, r.d.H
	}
	g := &vector.Gradient{Stops: stops, Matrix: vector.Identity}
	if n.name == "linearGradient" || chain[0].name == "linearGradient" {
		g.Kind = vector.Linear
		g.X0, g.Y0 = ref("x1", "0%", vw), ref("y1", "0%", vh)
		g.X1, g.Y1 = ref("x2", "100%", vw), ref("y2", "0%", vh)
	} else {
		g.Kind = vector.Radial
		g.CX, g.CY, g.R = ref("cx", "50%", vw), ref("cy", "50%", vh), ref("r", "50%", r.d.diag())
		g.FX, g.FY = g.CX, g.CY
		if _, ok := get("fx"); ok {
			g.FX = ref("fx", "50%", vw)
		}
		if _, ok := get("fy"); ok {
			g.FY = ref("fy", "50%", vh)
		}
	}
	switch v, _ := get("spreadMethod"); v {
	case "reflect":
		g.Spread = vector.SpreadReflect
	case "repeat":
		g.Spread = vector.SpreadRepeat
	}
	if t, ok := get("gradientTransform"); ok {
		g.Matrix = parseTransform(t)
	}
	if bbox {
		x0, y0, x1, y1, ok := p.Bounds()
		if !ok || x1 <= x0 || y1 <= y0 {
			return vector.Paint{}, false // a box with no area: SVG paints nothing
		}
		g.Matrix = g.Matrix.Then(vector.Matrix{A: x1 - x0, D: y1 - y0, E: x0, F: y0})
	}
	return vector.Paint{Gradient: g}, true
}

// clip applies the clip path with id, and reports false when nothing can be
// drawn through it.
func (r *renderer) clip(id string, depth int) bool {
	n := r.d.ids[id]
	if n == nil || n.name != "clipPath" {
		return true // a missing reference does not clip
	}
	var all vector.Path
	st := style{visible: true}
	var add func(n *node, m vector.Matrix, d int)
	add = func(n *node, m vector.Matrix, d int) {
		if d > maxDepth || n.attr("display") == "none" {
			return
		}
		if t := n.attr("transform"); t != "" {
			m = parseTransform(t).Then(m)
		}
		if n.name == "use" {
			if ref := r.d.ids[strings.TrimPrefix(n.attr("href"), "#")]; ref != nil {
				x, _ := length(n.attr("x"), r.d.vbW)
				y, _ := length(n.attr("y"), r.d.vbH)
				add(ref, vector.Translate(x, y).Then(m), d+1)
			}
			return
		}
		if p := r.shape(n); p != nil {
			all.Append(p.Transform(m))
			if v, ok := n.attrs["clip-rule"]; ok {
				st.clipRule = parseRule(v, st.clipRule)
			}
		}
	}
	for _, c := range n.children {
		add(c, vector.Identity, depth)
	}
	m := vector.Identity
	if t := n.attr("transform"); t != "" {
		m = parseTransform(t)
	}
	if all.Empty() {
		return false
	}
	r.c.Clip(all.Transform(m), st.clipRule)
	return true
}

// urlID returns the id of "url(#id)", or "".
func urlID(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "url(") {
		return ""
	}
	end := strings.IndexByte(v, ')')
	if end < 0 {
		return ""
	}
	id := strings.Trim(strings.TrimSpace(v[4:end]), `"'`)
	return strings.TrimPrefix(id, "#")
}

// parseTransform reads a transform list.
func parseTransform(s string) vector.Matrix {
	m := vector.Identity
	for {
		s = strings.TrimLeft(s, " ,\t\n\r")
		open := strings.IndexByte(s, '(')
		if open < 0 {
			return m
		}
		closeAt := strings.IndexByte(s, ')')
		if closeAt < open {
			return m
		}
		name := strings.TrimSpace(s[:open])
		ns := &numbers{s: s[open+1 : closeAt]}
		var a []float32
		for {
			v, ok := ns.next()
			if !ok {
				break
			}
			a = append(a, v)
		}
		s = s[closeAt+1:]
		var t vector.Matrix
		switch {
		case name == "matrix" && len(a) == 6:
			t = vector.Matrix{A: a[0], B: a[1], C: a[2], D: a[3], E: a[4], F: a[5]}
		case name == "translate" && len(a) >= 1:
			ty := float32(0)
			if len(a) > 1 {
				ty = a[1]
			}
			t = vector.Translate(a[0], ty)
		case name == "scale" && len(a) >= 1:
			sy := a[0]
			if len(a) > 1 {
				sy = a[1]
			}
			t = vector.Scale(a[0], sy)
		case name == "rotate" && len(a) >= 1:
			rad := a[0] * math.Pi / 180
			t = vector.Rotate(rad)
			if len(a) == 3 {
				t = vector.Translate(-a[1], -a[2]).Then(t).Then(vector.Translate(a[1], a[2]))
			}
		case name == "skewX" && len(a) == 1:
			t = vector.Skew(a[0]*math.Pi/180, 0)
		case name == "skewY" && len(a) == 1:
			t = vector.Skew(0, a[0]*math.Pi/180)
		default:
			return m // an error ends the list, as browsers do
		}
		// Each transform in the list applies before the ones to its left.
		m = t.Then(m)
	}
}
