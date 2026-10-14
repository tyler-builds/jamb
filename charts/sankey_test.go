package charts

import (
	"math"
	"testing"
)

// flow is a small acyclic diagram: A feeds B and C, B feeds C and D, and C feeds D.
func flow() []SankeyLink {
	return []SankeyLink{
		{From: "A", To: "B", Value: 10},
		{From: "A", To: "C", Value: 6},
		{From: "B", To: "D", Value: 8},
		{From: "B", To: "C", Value: 2},
		{From: "C", To: "D", Value: 4},
	}
}

// linkHeight returns the height of a link's band in pixels: the distance between its top and
// bottom edges at the source. The polygon lists the top edge from the source, then the bottom
// edge back to it, so its first point is the top at the source and its last the bottom there.
func linkHeight(poly []float64) float64 {
	return poly[len(poly)-1] - poly[1]
}

// The columns are the longest path from a source, and the nodes take their in and out flows.
func TestSankeyColumns(t *testing.T) {
	s := newSankey(flow())
	want := map[string]int{"A": 0, "B": 1, "C": 2, "D": 3}
	for _, n := range s.nodes {
		if n.column != want[n.name] {
			t.Errorf("node %s in column %d, want %d", n.name, n.column, want[n.name])
		}
	}
	if s.columns() != 4 {
		t.Errorf("columns = %d, want 4", s.columns())
	}
	// A is 16 out, B is 18 in-and-out, so its height is the larger of its flows.
	byName := map[string]sankeyNode{}
	for _, n := range s.nodes {
		byName[n.name] = n
	}
	if byName["B"].in != 10 || byName["B"].out != 10 || byName["B"].value != 10 {
		t.Errorf("B has in %v, out %v", byName["B"].in, byName["B"].out)
	}
	if byName["D"].in != 12 || byName["D"].value != 12 {
		t.Errorf("D has in %v, value %v, want 12", byName["D"].in, byName["D"].value)
	}
	if s.total != 30 {
		t.Errorf("total flow %v, want 30", s.total)
	}
}

// Links with no value, a negative value or an infinite value are left out, and their nodes
// are not created.
func TestSankeySkipsBadLinks(t *testing.T) {
	s := newSankey([]SankeyLink{{From: "X", To: "Y", Value: 0}, {From: "X", To: "Z", Value: -3},
		{From: "P", To: "Q", Value: math.Inf(1)}, {From: "A", To: "B", Value: 2}})
	if len(s.links) != 1 || len(s.nodes) != 2 {
		t.Errorf("kept %d links and %d nodes, want 1 and 2", len(s.links), len(s.nodes))
	}
}

// The layout keeps the node heights and the link widths in one scale: each node is as tall as
// its value, its outgoing links fill its height in order, and its incoming links do the same.
func TestSankeyLayoutSums(t *testing.T) {
	s := newSankey(flow())
	plot := rect{x0: 20, y0: 10, x1: 620, y1: 310}
	links := s.layout(plot)
	if len(links) != len(s.links) {
		t.Fatalf("%d link outlines for %d links", len(links), len(s.links))
	}

	// Each column holds one node here, and the column sums are 16, 10, 8 and 12. The scale is
	// the plot's height over the largest of them, so the tallest column fills the plot.
	scale := (plot.y1 - plot.y0) / 16
	for i, n := range s.nodes {
		if want := n.value * scale; math.Abs(n.h-want) > 1e-6 {
			t.Errorf("node %s is %v tall, want %v", n.name, n.h, want)
		}
		if i > 0 && n.x < s.nodes[0].x {
			t.Errorf("node %s is left of the first column", n.name)
		}
	}

	out := make([]float64, len(s.nodes))
	in := make([]float64, len(s.nodes))
	for _, lg := range links {
		e := s.links[lg.index]
		h := linkHeight(lg.poly)
		if math.Abs(h-e.value*scale) > 1e-6 {
			t.Errorf("link %d is %v wide, want %v", lg.index, h, e.value*scale)
		}
		out[e.from] += h
		in[e.to] += h
		// Each band starts at its source's right edge and ends at its target's left edge.
		if math.Abs(lg.poly[0]-(s.nodes[e.from].x+sankeyNodeW)) > 1e-9 {
			t.Errorf("link %d starts at %v, want the source's right edge", lg.index, lg.poly[0])
		}
		end := lg.poly[sankeyCurve*2]
		if math.Abs(end-s.nodes[e.to].x) > 1e-9 {
			t.Errorf("link %d ends at %v, want the target's left edge %v", lg.index, end, s.nodes[e.to].x)
		}
	}
	for i, n := range s.nodes {
		if math.Abs(out[i]-n.out*scale) > 1e-6 {
			t.Errorf("node %s has %v of outgoing bands, want its out flow %v", n.name, out[i], n.out*scale)
		}
		if math.Abs(in[i]-n.in*scale) > 1e-6 {
			t.Errorf("node %s has %v of incoming bands, want its in flow %v", n.name, in[i], n.in*scale)
		}
		if out[i] > n.h+1e-6 || in[i] > n.h+1e-6 {
			t.Errorf("node %s is %v tall but its bands total %v out and %v in", n.name, n.h, out[i], in[i])
		}
	}
}

// Nodes in one column do not overlap, and the nodes of a column stack from the top.
func TestSankeyColumnStacking(t *testing.T) {
	s := newSankey(flow())
	plot := rect{x0: 0, y0: 0, x1: 300, y1: 200}
	s.layout(plot)
	for i := range s.nodes {
		for j := range s.nodes {
			a, b := s.nodes[i], s.nodes[j]
			if i == j || a.column != b.column {
				continue
			}
			if a.y < b.y+b.h && b.y < a.y+a.h {
				t.Errorf("nodes %s and %s overlap in column %d", a.name, b.name, a.column)
			}
		}
	}
	for _, n := range s.nodes {
		if n.y < plot.y0-1e-9 || n.y+n.h > plot.y1+1e-6 {
			t.Errorf("node %s spans %v to %v, outside the plot", n.name, n.y, n.y+n.h)
		}
	}
}

// A point inside a link's band is inside its polygon, and a point beside it is not.
func TestLinkHitTest(t *testing.T) {
	s := newSankey(flow())
	links := s.layout(rect{x0: 0, y0: 0, x1: 300, y1: 200})
	poly := links[0].poly
	// The middle of the band: the midpoint of the top edge, and half the band below it.
	mid := sankeyCurve / 2
	x := poly[2*mid]
	top := poly[2*mid+1]
	if !pointInPoly(poly, x, top+linkHeight(poly)/4) {
		t.Errorf("the middle of link 0 at %v, %v is outside it", x, top)
	}
	if pointInPoly(poly, x, top-50) {
		t.Error("a point above the link is inside it")
	}
}
