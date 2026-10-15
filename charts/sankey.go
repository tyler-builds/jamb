package charts

import (
	"math"
	"sort"

	"github.com/tyler-builds/jamb/ui"
)

// SankeyLink is a flow from one node to another, of the given value. Nodes are named by
// their text, so two links that name the same node share it.
type SankeyLink struct {
	From, To string
	Value    float64
}

// Sankey layout constants, in logical pixels.
const (
	sankeyNodeW   = 12  // the width of a node's bar
	sankeyNodeGap = 10  // the space between two nodes in a column
	sankeyCurve   = 24  // the steps each link's edges are sampled with
	sankeyAlpha   = 0.4 // the opacity of a link's fill over the surface
)

// sankeyNode is a node of a diagram: its name, its column (its depth from the sources),
// the flow into and out of it, and its place in the layout.
type sankeyNode struct {
	name    string
	column  int
	in, out float64
	value   float64 // the larger of the in and out flow, which is the node's height in values
	x, y, h float64 // the layout: the left edge, the top and the height in pixels
}

// sankeyEdge is a link of a diagram, between the indices of its nodes.
type sankeyEdge struct {
	from, to int
	value    float64
}

// sankey is the data of a Sankey diagram: its nodes in order of their first appearance,
// and its links in the order they were given.
type sankey struct {
	nodes []sankeyNode
	links []sankeyEdge
	total float64
}

// newSankey converts the links to a diagram. A link with no value, or a negative or NaN
// value, is left out.
func newSankey(links []SankeyLink) *sankey {
	s := &sankey{}
	index := map[string]int{}
	node := func(name string) int {
		i, ok := index[name]
		if !ok {
			i = len(s.nodes)
			index[name] = i
			s.nodes = append(s.nodes, sankeyNode{name: name})
		}
		return i
	}
	for _, l := range links {
		if !(l.Value > 0) || math.IsInf(l.Value, 0) {
			continue
		}
		e := sankeyEdge{from: node(l.From), to: node(l.To), value: l.Value}
		s.links = append(s.links, e)
		s.nodes[e.from].out += e.value
		s.nodes[e.to].in += e.value
		s.total += e.value
	}
	// A node's column is the longest path from a source to it. Relaxing every link once
	// per node settles the columns of an acyclic diagram; a cycle stops at the bound.
	for pass := 0; pass < len(s.nodes); pass++ {
		changed := false
		for _, e := range s.links {
			if c := s.nodes[e.from].column + 1; c > s.nodes[e.to].column {
				s.nodes[e.to].column = c
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for i := range s.nodes {
		s.nodes[i].value = math.Max(s.nodes[i].in, s.nodes[i].out)
	}
	return s
}

// columns returns how many columns the nodes fall in.
func (s *sankey) columns() int {
	n := 0
	for _, nd := range s.nodes {
		n = max(n, nd.column+1)
	}
	return n
}

// layout places the nodes and the links in the plot p, and returns the outline of each
// link. Every column shares one scale, so a value has the same height in each, and the
// nodes of a column stack from the top in the order they first appear. The outgoing links
// of a node leave it in the order of the heights of the nodes they go to, and the incoming
// ones arrive in the order of the nodes they come from, which keeps them from crossing.
func (s *sankey) layout(p rect) []linkGeo {
	if len(s.nodes) == 0 {
		return nil
	}
	columns := s.columns()
	sums := make([]float64, columns)
	counts := make([]int, columns)
	for _, n := range s.nodes {
		sums[n.column] += n.value
		counts[n.column]++
	}
	scale := math.Inf(1)
	for c := 0; c < columns; c++ {
		if sums[c] > 0 {
			scale = math.Min(scale, (p.y1-p.y0-float64(counts[c]-1)*sankeyNodeGap)/sums[c])
		}
	}
	if math.IsInf(scale, 0) || scale < 0 {
		scale = 0
	}
	span := p.x1 - p.x0 - sankeyNodeW
	top := make([]float64, columns)
	for c := range top {
		top[c] = p.y0
	}
	for i := range s.nodes {
		n := &s.nodes[i]
		n.x = p.x0
		if columns > 1 {
			n.x = p.x0 + span*float64(n.column)/float64(columns-1)
		}
		n.h = n.value * scale
		n.y = top[n.column]
		top[n.column] += n.h + sankeyNodeGap
	}

	// The links leave each node's right edge, stacked in the order of their targets,
	// and enter its left edge in the order of their sources.
	order := make([]int, len(s.links))
	for i := range order {
		order[i] = i
	}
	srcY := make([]float64, len(s.links))
	dstY := make([]float64, len(s.links))
	sort.SliceStable(order, func(a, b int) bool {
		return s.nodes[s.links[order[a]].to].y < s.nodes[s.links[order[b]].to].y
	})
	outAt := make([]float64, len(s.nodes))
	for i := range outAt {
		outAt[i] = s.nodes[i].y
	}
	for _, i := range order {
		e := s.links[i]
		srcY[i] = outAt[e.from]
		outAt[e.from] += e.value * scale
	}
	sort.SliceStable(order, func(a, b int) bool {
		return s.nodes[s.links[order[a]].from].y < s.nodes[s.links[order[b]].from].y
	})
	inAt := make([]float64, len(s.nodes))
	for i := range inAt {
		inAt[i] = s.nodes[i].y
	}
	for _, i := range order {
		e := s.links[i]
		dstY[i] = inAt[e.to]
		inAt[e.to] += e.value * scale
	}

	out := make([]linkGeo, 0, len(s.links))
	for i, e := range s.links {
		x0 := s.nodes[e.from].x + sankeyNodeW
		x1 := s.nodes[e.to].x
		out = append(out, linkGeo{poly: linkPolygon(x0, srcY[i], x1, dstY[i], e.value*scale), index: i})
	}
	return out
}

// linkPolygon returns the outline of a link of height h: its top edge runs from y0 at the
// left x0 to y1 at the right x1, and its bottom edge h below it. Each edge is an S-curve,
// level at both ends, and the outline is flattened into x, y pairs.
func linkPolygon(x0, y0, x1, y1, h float64) []float64 {
	poly := make([]float64, 0, 4*(sankeyCurve+1))
	edge := func(k int, dy float64) {
		t := float64(k) / sankeyCurve
		s := t * t * (3 - 2*t) // the smoothstep: the S-curve's height at t
		poly = append(poly, x0+(x1-x0)*t, y0+(y1-y0)*s+dy)
	}
	for k := 0; k <= sankeyCurve; k++ {
		edge(k, 0)
	}
	for k := sankeyCurve; k >= 0; k-- {
		edge(k, h)
	}
	return poly
}

// paintSankey draws the diagram: each link as its band in the color of its source node
// at sankeyAlpha, and each node as a bar in its own palette color, with its name beside it.
// Under high contrast the links are outlined and the nodes are the text color. The enter
// animation fades the diagram in.
func (c *Chart) paintSankey(v *view, p *ui.Painter, w, h float32) {
	g := v.geo
	*g = geometry{w: float64(w), h: float64(h), drawn: true}
	s := c.sk
	if s == nil || len(s.nodes) == 0 {
		return
	}
	size := axisFont(v.st)
	margin := 0.0
	for _, n := range s.nodes {
		wd, _ := p.MeasureText(n.name, ui.TextFormat{FontSize: size})
		margin = math.Max(margin, float64(wd))
	}
	pad := margin + edgePad + 4
	g.plot = rect{x0: pad, y0: edgePad, x1: float64(w) - pad, y1: float64(h) - edgePad}
	g.links = s.layout(g.plot)
	p.Save()
	p.Alpha(v.progress)
	for _, lg := range g.links {
		shape := polyPath(lg.poly)
		if v.t.HighContrast {
			p.Fill(shape, v.t.Surface)
			p.Stroke(shape, ui.StrokeStyle{Width: 1, Color: v.t.Text})
			continue
		}
		p.Fill(shape, seriesLook(v.t, s.links[lg.index].from).color.Alpha(sankeyAlpha))
	}
	for i, n := range s.nodes {
		color := seriesLook(v.t, i).color
		if v.t.HighContrast {
			color = v.t.Text
		}
		p.FillRect(float32(n.x), float32(n.y), sankeyNodeW, float32(math.Max(n.h, 1)), color)
		p.Text(float32(n.x+sankeyNodeW+6), float32(n.y+n.h/2), n.name,
			ui.TextFormat{FontSize: size, Color: v.st.Axis.TextColor, Align: ui.Start, VAlign: ui.Center})
	}
	p.Restore()
}

// polyPath returns the closed path of a flattened polygon of x, y pairs.
func polyPath(poly []float64) *ui.Path {
	path := ui.NewPath()
	for i := 0; i+1 < len(poly); i += 2 {
		if i == 0 {
			path.MoveTo(float32(poly[i]), float32(poly[i+1]))
		} else {
			path.LineTo(float32(poly[i]), float32(poly[i+1]))
		}
	}
	return path.Close()
}

// pointInPoly reports whether the point x, y is inside the polygon, by the even-odd rule.
func pointInPoly(poly []float64, x, y float64) bool {
	in := false
	n := len(poly) / 2
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := poly[2*i], poly[2*i+1]
		xj, yj := poly[2*j], poly[2*j+1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}
