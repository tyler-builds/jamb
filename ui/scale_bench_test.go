package ui

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/text"
)

// Scaling benchmarks: trees of ~100, ~1,000 and ~10,000 nodes.
//
//	go test ./ui -run xxx -bench Scale -benchtime 2s
//
// Each row is a component of 5 nodes (row, icon box, text column, 2 texts)
// reading its own signal. Reported per operation:
//
//	ns/op, B/op, allocs/op  standard
//	gc/op                   garbage collections per operation
//	rendered/op             components whose render function ran
//	built/op                nodes created
//	laidout/op              nodes whose children were re-positioned
//	nodes                   live nodes in the tree
//	painted                 nodes painted by the last frame (culling skips the rest)

// One text system for all benchmarks, so font loading is not measured.
var benchText = text.NewSystem()

type scaleApp struct {
	values []*Signal[string]
	rt     *Runtime
}

func newScaleApp(nodes int) *scaleApp {
	rows := max(nodes/5, 1)
	a := &scaleApp{values: make([]*Signal[string], rows)}
	for i := range a.values {
		a.values[i] = NewSignal(fmt.Sprintf("Row %d", i))
	}
	a.rt = NewRuntime(func(ctx *Context) Element {
		list := Div().Gap(4).Padding(8).ScrollY().Grow(1)
		for i, v := range a.values {
			list.Child(Component(func(ctx *Context) Element {
				return Div().Row().Gap(10).Padding(8).Radius(6).
					Background(Hex("#18181b")).Hover(Style{Background: Hex("#27272a")}).
					Child(Div().Size(Px(24), Px(24)).Radius(6).Background(Hex("#6366f1"))).
					Child(Div().Grow(1).Gap(2).
						Child(Text(v.Get()).Semibold()).
						Child(Text("A secondary line of text that wraps at narrow widths.").FontSize(12)))
			}).Key(fmt.Sprint(i)))
		}
		return Div().Height(Full).Child(list)
	}, DarkTheme, benchText)
	a.rt.SetViewport(1200, 800, 1)
	a.rt.Frame(time.Now())
	return a
}

type gcCounter struct{ start uint32 }

func startGC() gcCounter {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return gcCounter{m.NumGC}
}

func (g gcCounter) report(b *testing.B) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	b.ReportMetric(float64(m.NumGC-g.start)/float64(b.N), "gc/op")
}

func reportStats(b *testing.B, rt *Runtime, rendered, built, laidOut int) {
	b.ReportMetric(float64(rendered)/float64(b.N), "rendered/op")
	b.ReportMetric(float64(built)/float64(b.N), "built/op")
	b.ReportMetric(float64(laidOut)/float64(b.N), "laidout/op")
	b.ReportMetric(float64(len(rt.byID)), "nodes")
	b.ReportMetric(float64(rt.Stats().NodesPainted), "painted")
}

func BenchmarkScale(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			b.Run("full-rebuild", func(b *testing.B) {
				a := newScaleApp(size)
				var r, bu, l int
				b.ReportAllocs()
				gc := startGC()
				for b.Loop() {
					a.rt.InvalidateAll()
					a.rt.Frame(time.Now())
					st := a.rt.Stats()
					r, bu, l = r+st.Rendered, bu+st.NodesBuilt, l+st.NodesLaidOut
				}
				gc.report(b)
				reportStats(b, a.rt, r, bu, l)
			})
			b.Run("single-signal-update", func(b *testing.B) {
				a := newScaleApp(size)
				var r, bu, l, i int
				b.ReportAllocs()
				gc := startGC()
				for b.Loop() {
					i++
					a.values[len(a.values)/2].Set(fmt.Sprintf("Row edited %d", i))
					a.rt.Frame(time.Now())
					st := a.rt.Stats()
					r, bu, l = r+st.Rendered, bu+st.NodesBuilt, l+st.NodesLaidOut
				}
				gc.report(b)
				reportStats(b, a.rt, r, bu, l)
			})
			b.Run("hover-change", func(b *testing.B) {
				a := newScaleApp(size)
				var r, bu, l, i int
				b.ReportAllocs()
				gc := startGC()
				for b.Loop() {
					i++
					y := float32(30)
					if i%2 == 0 {
						y = 90 // alternate between two rows
					}
					a.rt.PointerMove(300, y, 0)
					a.rt.Frame(time.Now())
					st := a.rt.Stats()
					r, bu, l = r+st.Rendered, bu+st.NodesBuilt, l+st.NodesLaidOut
				}
				gc.report(b)
				reportStats(b, a.rt, r, bu, l)
			})
			b.Run("repaint-only", func(b *testing.B) {
				a := newScaleApp(size)
				b.ReportAllocs()
				gc := startGC()
				for b.Loop() {
					a.rt.Invalidate()
					a.rt.Frame(time.Now())
				}
				gc.report(b)
				reportStats(b, a.rt, 0, 0, 0)
			})
			b.Run("window-resize", func(b *testing.B) {
				a := newScaleApp(size)
				var l, i int
				b.ReportAllocs()
				gc := startGC()
				for b.Loop() {
					i++
					a.rt.SetViewport(1000+float32(i%2)*200, 800, 1)
					a.rt.Frame(time.Now())
					l += a.rt.Stats().NodesLaidOut
				}
				gc.report(b)
				reportStats(b, a.rt, 0, 0, l)
			})
		})
	}
}

// BenchmarkInitialBuild measures building a tree from scratch (startup and
// theme switches, which re-render everything).
func BenchmarkInitialBuild(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			gc := startGC()
			for b.Loop() {
				newScaleApp(size)
			}
			gc.report(b)
		})
	}
}

// benchListRow is one row of the list benchmarks: the same five nodes as the
// rows of newScaleApp.
func benchListRow(i int) Element {
	return Component(func(ctx *Context) Element {
		return Div().Row().Gap(10).Padding(8).Radius(6).
			Background(Hex("#18181b")).Hover(Style{Background: Hex("#27272a")}).
			Child(Div().Size(Px(24), Px(24)).Radius(6).Background(Hex("#6366f1"))).
			Child(Div().Grow(1).Gap(2).
				Child(Text(fmt.Sprintf("Row %d", i)).Semibold()).
				Child(Text("A secondary line of text that wraps at narrow widths.").FontSize(12)))
	}).Key(fmt.Sprint(i))
}

// newListBenchApp renders count rows in a 800px-tall scroll container, either
// as a UniformList or as plain children of a ScrollY Div. The container is
// the first child of the root in both cases.
func newListBenchApp(count int, uniform bool) *Runtime {
	rt := NewRuntime(func(ctx *Context) Element {
		if uniform {
			return Div().Height(Full).Child(UniformList(count, benchListRow).Style(Style{Height: Px(800)}))
		}
		list := Div().Gap(4).Padding(8).ScrollY().Height(Px(800))
		for i := range count {
			list.Child(benchListRow(i))
		}
		return Div().Height(Full).Child(list)
	}, DarkTheme, benchText)
	rt.SetViewport(1200, 800, 1)
	rt.Frame(time.Now())
	return rt
}

// BenchmarkUniformList compares a UniformList of 100,000 rows with a plain
// scroll container of 10,000 rows. Each operation is a 120px wheel step and a
// frame; the scroll wraps to the top near the end.
func BenchmarkUniformList(b *testing.B) {
	cases := []struct {
		name    string
		count   int
		uniform bool
	}{
		{"list/100k", 100_000, true},
		{"div/10k", 10_000, false},
	}
	for _, c := range cases {
		b.Run("build/"+c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				newListBenchApp(c.count, c.uniform)
			}
		})
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			rt := newListBenchApp(c.count, c.uniform)
			container := rt.tree.children[0]
			b.ReportAllocs()
			for b.Loop() {
				if maxY := container.lay.ContentH - container.lay.H; container.st.targetY >= maxY-240 {
					rt.Scroll(100, 100, 0, -1e9, true, 0)
				}
				rt.Scroll(100, 100, 0, 120, true, 0)
				rt.Frame(time.Now())
			}
			b.ReportMetric(float64(len(rt.byID)), "nodes")
		})
	}
}
