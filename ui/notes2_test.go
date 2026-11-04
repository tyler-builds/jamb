package ui

import (
	"testing"
	"time"
)

// Text of two sizes in a Baseline row shares its first baseline.
func TestRowBaselineAlignsTextOfTwoSizes(t *testing.T) {
	rt := newTestRuntime(func(*Context) Element {
		return Div().Column().Child(Div().Row().Align(Baseline).Gap(8).
			Child(Text("Firefox").Key("name").FontSize(18).Semibold()).
			Child(Text("by Mozilla").Key("dev").FontSize(13)))
	}, 400, 200, 1)
	rt.Frame(time.Now())
	base := func(key string) float32 {
		n := keyNode(t, rt, key)
		l := n.textLayout(n.lay.W)
		return rt.nodeRect(n).Y + l.Lines[0].Baseline
	}
	if a, b := base("name"), base("dev"); a != b {
		t.Errorf("baselines at %v and %v, want them equal", a, b)
	}
}

// Shrink(0) keeps a text at its full width while the text beside it truncates.
func TestTextShrinkZeroKeepsItsWidth(t *testing.T) {
	rt := newTestRuntime(func(*Context) Element {
		return Div().Column().Child(Div().Row().Width(Px(150)).
			Child(Text("Firefox Browser").Key("name").Shrink(0)).
			Child(Text("by the Mozilla Foundation").Key("dev").Truncate()))
	}, 400, 200, 1)
	rt.Frame(time.Now())
	name := keyNode(t, rt, "name")
	if full := name.textLayout(float32(1e9)).Width; name.lay.W < full {
		t.Errorf("name is %v wide, want its full %v", name.lay.W, full)
	}
}

// OnGridColumns reports an AutoFill grid's column count, again when it changes,
// and what it sets is built in the same frame.
func TestOnGridColumnsReportsTheCount(t *testing.T) {
	cols := NewSignal(0)
	var reports []int
	rt := newTestRuntime(func(*Context) Element {
		g := Div().Key("grid").Grid(AutoFill(200)).Gap(20).OnGridColumns(func(n int) {
			reports = append(reports, n)
			cols.Set(n)
		})
		for i := 0; i < cols.Get(); i++ {
			g.Child(Div().Height(Px(40)))
		}
		return Div().Column().Child(g)
	}, 900, 400, 1)
	now := time.Now()
	rt.Frame(now)
	if cols.Peek() != 4 || len(keyNode(t, rt, "grid").children) != 4 {
		t.Fatalf("at 900 px: %d columns and %d cards, want 4 and 4 (reports %v)", cols.Peek(), len(keyNode(t, rt, "grid").children), reports)
	}
	rt.SetViewport(500, 400, 1)
	rt.Frame(now.Add(time.Second))
	if cols.Peek() != 2 || len(reports) != 2 {
		t.Errorf("at 500 px: %d columns, reports %v; want 2 and [4 2]", cols.Peek(), reports)
	}
}

// A canvas's drawing takes its rounded corners.
func TestCanvasDrawingIsRounded(t *testing.T) {
	rt := newTestRuntime(func(*Context) Element {
		return Div().Column().Child(Canvas(func(p *Painter, w, h float32) {}).Size(Px(200), Px(100)).Radius(20))
	}, 400, 200, 1)
	rt.Frame(time.Now())
	if im := canvasImage(t, rt); im.Radii.TL != 20 || im.Radii.BR != 20 {
		t.Errorf("canvas image radii %+v, want 20", im.Radii)
	}
}
