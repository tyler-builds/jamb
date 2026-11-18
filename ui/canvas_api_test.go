package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
)

// OnResize reports the content size in the frame of the layout, so what it sets
// is built and drawn in that frame: the canvas draws once, with the rows that
// fit, never with the size of the frame before.
func TestOnResizeReportsInTheSameFrame(t *testing.T) {
	size := NewSignal([2]float32{})
	var reports [][2]float32
	var drawn []int // the visible rows each draw saw
	renders := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		renders++
		rows := int(size.Get()[1] / 20)
		return Div().Column().Size(Full, Full).Child(
			Canvas(func(p *Painter, w, h float32) { drawn = append(drawn, rows) }).
				Size(Full, Full).Padding(5).Border(1, Color{A: 1}).
				OnResize(func(w, h float32) {
					reports = append(reports, [2]float32{w, h})
					size.Set([2]float32{w, h})
				}))
	}, 300, 212, 1)
	// The content box is the window less the padding and the border.
	if len(reports) != 1 || reports[0] != [2]float32{288, 200} {
		t.Fatalf("reports %v, want [[288 200]]", reports)
	}
	if renders != 2 || len(drawn) != 1 || drawn[0] != 10 {
		t.Fatalf("first frame: %d renders, draws saw %v rows; want 2 renders and one draw of 10 rows", renders, drawn)
	}
	frame(rt)
	if len(reports) != 1 || len(drawn) != 1 {
		t.Fatalf("a frame without a change reported %v and drew %v", reports, drawn)
	}
	rt.SetViewport(300, 412, 1)
	frame(rt)
	if len(reports) != 2 || reports[1] != [2]float32{288, 400} {
		t.Fatalf("after a resize: reports %v", reports)
	}
	if last := drawn[len(drawn)-1]; last != 20 {
		t.Errorf("after a resize the last draw saw %d rows, want 20 (draws %v)", last, drawn)
	}
	for _, rows := range drawn[1:] {
		if rows != 20 {
			t.Errorf("a draw in the resize frame saw %d rows, the size of the frame before (draws %v)", rows, drawn)
		}
	}
}

// OnGridColumns still builds what it changes before the frame ends.
func TestOnResizeAndGridColumnsTogether(t *testing.T) {
	var cols int
	var w float32
	rt := newTestRuntime(func(*Context) Element {
		return Div().Grid(AutoFill(100)).OnGridColumns(func(n int) { cols = n }).
			OnResize(func(ww, _ float32) { w = ww })
	}, 450, 100, 1)
	frame(rt)
	if cols != 4 || w != 450 {
		t.Errorf("columns %d, width %v; want 4 and 450", cols, w)
	}
}

// Context.MeasureText measures text as the painter does, outside a drawing.
func TestContextMeasureTextMatchesPainter(t *testing.T) {
	formats := []TextFormat{
		{},
		{Mono: true},
		{FontSize: 20, Weight: FontBold},
		{Italic: true, FontSize: 9},
	}
	const s = "2026-11-18 12:34:56.789 SELECT"
	var ctxSizes, painterSizes, drawnSizes [][2]float32
	rt := newTestRuntime(func(ctx *Context) Element {
		ctxSizes = ctxSizes[:0]
		for _, f := range formats {
			w, h := ctx.MeasureText(s, f)
			ctxSizes = append(ctxSizes, [2]float32{w, h})
		}
		return Canvas(func(p *Painter, w, h float32) {
			painterSizes, drawnSizes = painterSizes[:0], drawnSizes[:0]
			for _, f := range formats {
				w, h := p.MeasureText(s, f)
				painterSizes = append(painterSizes, [2]float32{w, h})
				l := p.layout(s, f) // what Text draws
				drawnSizes = append(drawnSizes, [2]float32{l.Width, l.Height})
			}
		})
	}, 400, 200, 1.25)
	frame(rt)
	for i := range formats {
		if ctxSizes[i][0] <= 0 || ctxSizes[i] != painterSizes[i] || ctxSizes[i] != drawnSizes[i] {
			t.Errorf("format %+v: Context.MeasureText %v, Painter.MeasureText %v, drawn %v", formats[i], ctxSizes[i], painterSizes[i], drawnSizes[i])
		}
	}
	if ctxSizes[1] == ctxSizes[0] || ctxSizes[2][1] <= ctxSizes[0][1] {
		t.Errorf("the formats measured alike: %v", ctxSizes)
	}
}

// PixelScale is the display scale times the zoom, and Snap puts a point on
// the corner of a display pixel, through the transform.
func TestPainterPixelScaleAndSnap(t *testing.T) {
	var k float32
	var snapped [][2]float32
	rt := newTestRuntime(func(*Context) Element {
		return Canvas(func(p *Painter, w, h float32) {
			k = p.PixelScale()
			snapped = snapped[:0]
			x, y := p.Snap(0.3, 6)
			snapped = append(snapped, [2]float32{x, y})
			p.Translate(0.37, 0.37)
			x, y = p.Snap(10, 13)
			snapped = append(snapped, [2]float32{x + 0.37, y + 0.37}) // in the canvas's coordinates
		}).Size(Px(200), Px(100))
	}, 400, 200, 1.25)
	if k != 1.25 {
		t.Errorf("PixelScale %v at 125%%, want 1.25", k)
	}
	whole := func(v float32) bool { return math.Abs(float64(v*1.25)-math.Round(float64(v*1.25))) < 1e-4 }
	for _, pt := range snapped {
		if !whole(pt[0]) || !whole(pt[1]) {
			t.Errorf("snapped to %v, which is %v device pixels", pt, [2]float32{pt[0] * 1.25, pt[1] * 1.25})
		}
	}
	if p := snapped[0]; p[0] != 0 || math.Abs(float64(p[1]-6.4)) > 1e-4 {
		t.Errorf("Snap(0.3, 6) = %v, want the nearest corner 0, 6.4", p)
	}
	rt.SetZoom(2)
	frame(rt)
	if k != 2.5 {
		t.Errorf("PixelScale %v at 125%% zoomed 200%%, want 2.5", k)
	}
}

// A 1 px line drawn at a snapped point, one display pixel thick, covers the row
// of display pixels a Div border at its place covers, fully, and leaves the
// rows next to it empty. The canvas is 52.5 display pixels tall at 125%: its
// drawing is not stretched to the 53 it is rounded to.
func TestPainterSnapDrawsCrispLines(t *testing.T) {
	var gotH float32
	rt := newTestRuntime(func(*Context) Element {
		return Div().Column().Child(Canvas(func(p *Painter, w, h float32) {
			gotH = h
			for i := range 6 {
				x0, y := p.Snap(0, float32(i*7+6))
				x1, _ := p.Snap(w, 0)
				p.FillRect(x0, y, x1-x0, 1/p.PixelScale(), Color{A: 1})
			}
		}).Size(Px(40), Px(42)))
	}, 100, 100, 1.25)
	tex := canvasImage(t, rt).Texture
	if tex.Height != 53 || gotH != 53/1.25 {
		t.Fatalf("drawn %v tall into %d pixels, want 42.4 into 53", gotH, tex.Height)
	}
	want := map[int]bool{}
	for i := range 6 {
		want[int(round(float32(i*7+6)*1.25))] = true // where a Div border at that y is drawn
	}
	for y := 0; y < tex.Height; y++ {
		a := pixel(tex, 10, y)[3]
		if want[y] && a != 255 || !want[y] && a != 0 {
			t.Errorf("row %d has alpha %d, want a line on rows %v and nothing else", y, a, want)
		}
	}
}

// CanvasKey keeps the drawing while the key is equal, through renders of the
// canvas's component, and draws again when it changes or the size does.
func TestCanvasKeyKeepsTheDrawing(t *testing.T) {
	status, rev := NewSignal(0), NewSignal(0)
	w := NewSignal(float32(200))
	draws := 0
	type gridKey struct {
		rev   int
		first int
	}
	rt := newTestRuntime(func(*Context) Element {
		return Div().Column().Child(Textf("status %d", status.Get())).Child(
			Canvas(func(p *Painter, w, h float32) { draws++; p.FillRect(0, 0, w, h, Color{R: 1, A: 1}) }).
				Size(Px(w.Get()), Px(100)).CanvasKey(gridKey{rev: rev.Get()}))
	}, 400, 300, 1)
	for i := 1; i <= 5; i++ {
		status.Set(i)
		frame(rt)
	}
	if draws != 1 {
		t.Fatalf("drew %d times over 5 renders with the same key, want 1", draws)
	}
	if im := canvasImage(t, rt); pixel(im.Texture, 10, 10) != [4]uint8{255, 0, 0, 255} {
		t.Errorf("the kept drawing reads %v", pixel(im.Texture, 10, 10))
	}
	rev.Set(1)
	frame(rt)
	if draws != 2 {
		t.Fatalf("drew %d times after the key changed, want 2", draws)
	}
	w.Set(150)
	frame(rt)
	if draws != 3 {
		t.Fatalf("drew %d times after the size changed, want 3", draws)
	}
	// Without a key, each render draws.
	draws = 0
	rt = newTestRuntime(func(*Context) Element {
		return Div().Column().Child(Textf("status %d", status.Get())).
			Child(Canvas(func(p *Painter, w, h float32) { draws++ }))
	}, 400, 300, 1)
	status.Set(10)
	frame(rt)
	if draws != 2 {
		t.Errorf("without a key: %d draws over 2 renders, want 2", draws)
	}
}

// CanvasKey refuses a key that == would panic on, naming its type.
func TestCanvasKeyMustBeComparable(t *testing.T) {
	type withSlice struct{ rows []int }
	type withAny struct{ v any }
	type ok struct {
		rev int
		p   *int
		v   any
	}
	for _, k := range []any{[]int{1}, map[int]int{}, func() {}, withSlice{}, withAny{v: []string{"a"}}, [1][]int{}} {
		func() {
			defer func() {
				r := recover()
				msg, _ := r.(string)
				if r == nil || !strings.Contains(msg, "not comparable") {
					t.Errorf("CanvasKey(%T): recovered %v, want a panic that says it is not comparable", k, r)
				}
			}()
			Canvas(nil).CanvasKey(k)
		}()
	}
	for _, k := range []any{nil, 1, "a", &struct{}{}, ok{v: 2}, [2]int{}} {
		Canvas(nil).CanvasKey(k)
	}
}

// Canvas text is clipped to the current clip: the clip is pushed around the
// text in the scene, intersected with the clips around it, and Restore takes it
// away.
func TestCanvasTextFollowsClip(t *testing.T) {
	rt := newTestRuntime(func(*Context) Element {
		return Div().Padding(10).Child(Canvas(func(p *Painter, w, h float32) {
			p.Text(0, 0, "outside every clip", TextFormat{})
			p.Save()
			p.Clip(NewPath().Rect(10, 20, 90, 26))
			p.Save()
			p.Translate(50, 0)
			p.Clip(NewPath().Rect(0, 0, 100, 40)) // 50 to 150: cut to 50 to 100
			p.Text(0, 22, "cell", TextFormat{})
			p.Restore()
			p.Text(12, 22, "SELECT * FROM dbo.VeryLongTableName", TextFormat{})
			p.Restore()
			p.Text(0, 60, "after Restore", TextFormat{})
		}).Size(Px(300), Px(100)))
	}, 400, 200, 2)
	var clips []scene.Rect
	depth := 0
	var glyphClips []string // the clip depth and rect each glyph run was drawn under
	stack := []scene.Rect{}
	for _, op := range rt.scene.Ops {
		switch op.Kind {
		case scene.OpPushClip:
			depth++
			stack = append(stack, op.Clip)
			clips = append(clips, op.Clip)
		case scene.OpPopClip:
			depth--
			stack = stack[:len(stack)-1]
		case scene.OpGlyphs:
			if len(stack) > 0 {
				glyphClips = append(glyphClips, rectString(stack[len(stack)-1]))
			}
		}
	}
	// The canvas is at 10, 10 at scale 2: device pixels 20, 20 on.
	want := []string{
		rectString(scene.Rect{X: 20, Y: 20, W: 600, H: 200}), // the canvas, around all its text
		rectString(scene.Rect{X: 120, Y: 60, W: 100, H: 40}), // the inner clip, cut by the cell
		rectString(scene.Rect{X: 40, Y: 60, W: 180, H: 52}),  // the cell
	}
	if depth != 0 || len(clips) != len(want) {
		t.Fatalf("clips pushed %v (depth left %d), want %v", clips, depth, want)
	}
	for i, c := range clips {
		if rectString(c) != want[i] {
			t.Errorf("clip %d is %v, want %v", i, rectString(c), want[i])
		}
	}
	wantGlyphs := []string{want[0], want[1], want[2], want[0]}
	if strings.Join(glyphClips, " ") != strings.Join(wantGlyphs, " ") {
		t.Errorf("the texts were drawn under %v, want %v", glyphClips, wantGlyphs)
	}
}

func rectString(r scene.Rect) string { return fmt.Sprintf("%g,%g %gx%g", r.X, r.Y, r.W, r.H) }
