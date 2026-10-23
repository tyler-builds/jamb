package views

import (
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/graph"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// The graph's geometry, in logical pixels.
const (
	laneWidth  = 20  // the width of one lane
	graphPad   = 6   // the space before the first lane
	nodeRadius = 8   // a commit's node, which holds its author's initials
	mergeDot   = 4   // a merge commit's node, a plain dot
	lineWidth  = 2   // the width of a lane's line
	maxGraph   = 280 // the widest the graph column starts
)

// graphWidth is the starting width of the graph column for a history whose widest
// row has lanes lanes.
func graphWidth(lanes int) float32 {
	return min(max(float32(graphPad*2+lanes*laneWidth), 56), maxGraph)
}

// laneX is the middle of lane i.
func laneX(i int) float32 { return graphPad + float32(i)*laneWidth + laneWidth/2 }

// rowHeight is the height of a history row: a comfortable DataGrid row, 36 px at
// the Text size of 100 percent. The grid wraps a Cell's content in a box that
// does not stretch to the row, so the canvas cannot take the row's height with
// ui.Full and is given it here.
func rowHeight(ctx *ui.Context) float32 { return 36 * float32(ctx.TextScale()) / 100 }

// graphCell draws one row's slice of the graph, h pixels high. The rows of the
// grid sit edge to edge, so the lines of one row meet the lines of the next.
func graphCell(t *ui.Theme, e store.Entry, head bool, h float32) ui.Element {
	return ui.Canvas(func(p *ui.Painter, w, h float32) {
		drawRow(p, t, e, head, h)
	}).Key("graph-"+e.Hash).Size(ui.Full, ui.Px(h)).AccessHidden()
}

func drawRow(p *ui.Painter, t *ui.Theme, e store.Entry, head bool, h float32) {
	row := e.Row
	mid := h / 2
	node := laneX(row.Lane)
	stroke := func(path *ui.Path, lane int) {
		p.Stroke(path, ui.StrokeStyle{Width: lineWidth, Color: laneColor(t, lane), Join: ui.JoinRound})
	}
	// A line that changes lanes runs along the node's middle and turns into its
	// own lane with a rounded corner, as a branch leaves or joins.
	for _, l := range row.Lines {
		switch l.Part {
		case graph.Through:
			x := laneX(l.From)
			stroke(ui.NewPath().MoveTo(x, 0).LineTo(x, h), l.Color)
		case graph.Upper:
			x := laneX(l.From)
			if l.From == row.Lane {
				stroke(ui.NewPath().MoveTo(x, 0).LineTo(x, mid), l.Color)
				continue
			}
			r := min(mid, absf(x-node))
			dir := sign(node - x)
			stroke(ui.NewPath().MoveTo(x, 0).LineTo(x, mid-r).
				QuadTo(x, mid, x+dir*r, mid).LineTo(node, mid), l.Color)
		case graph.Lower:
			x := laneX(l.To)
			if l.To == row.Lane {
				stroke(ui.NewPath().MoveTo(x, mid).LineTo(x, h), l.Color)
				continue
			}
			r := min(mid, absf(x-node))
			dir := sign(x - node)
			stroke(ui.NewPath().MoveTo(node, mid).LineTo(x-dir*r, mid).
				QuadTo(x, mid, x, mid+r).LineTo(x, h), l.Color)
		}
	}
	c := laneColor(t, row.Lane)
	if head {
		// HEAD's commit has a ring around its node.
		p.Stroke(ui.NewPath().Circle(node, mid, nodeRadius+3), ui.StrokeStyle{Width: 1.5, Color: c.Alpha(0.6)})
	}
	if row.Merge {
		p.Fill(ui.NewPath().Circle(node, mid, mergeDot+1.5), t.Background)
		p.Fill(ui.NewPath().Circle(node, mid, mergeDot), c)
		return
	}
	p.Fill(ui.NewPath().Circle(node, mid, nodeRadius), c)
	ink := ui.Hex("#ffffff")
	if t.HighContrast {
		ink = t.Background
	}
	p.Text(node, mid, initials(e.Author), ui.TextFormat{FontSize: 8, Weight: ui.FontSemibold, Color: ink,
		Align: ui.Center, VAlign: ui.Center})
}

func absf(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

func sign(f float32) float32 {
	if f < 0 {
		return -1
	}
	return 1
}
