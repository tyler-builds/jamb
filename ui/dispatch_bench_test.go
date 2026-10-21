package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// dispatchBenchApp is the scale app of scale_bench_test.go with a text field
// above the list. The field is keyed "field" and is not focused yet. The root
// runs an action on Ctrl+K, which is the global binding of the benchmark.
func dispatchBenchApp(nodes int, hits *int) (*Runtime, *Signal[string]) {
	text := NewSignal("Type here")
	rows := max(nodes/5, 1)
	act := NewAction("bench.dispatch", "Bench")
	rt := NewRuntime(func(ctx *Context) Element {
		list := Div().Gap(4).Padding(8).ScrollY().Grow(1)
		for i := range rows {
			list.Child(benchListRow(i))
		}
		return Div().Column().Height(Full).OnAction(act, func() { *hits++ }).
			Child(TextInput(text).Key("field")).
			Child(list)
	}, DarkTheme(), benchText)
	rt.AddBindings(Bind("ctrl+k", act))
	rt.SetViewport(1200, 800, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	rt.Frame(now.Add(time.Second))
	rt.setFocus(rt.findKey(rt.tree, "field").id)
	return rt, text
}

// BenchmarkHitTest is the hit test of a pointer position on a laid-out window
// of 100, 1,000 and 10,000 nodes, without a frame. The positions sweep the
// window's height, so each call tests a different row. The hit test walks the
// nodes painted by the last frame, so it does not depend on the size of the
// tree, only on what is on screen.
func BenchmarkHitTest(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			a := newScaleApp(size)
			var hit int
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				y := float32(10 + i%780)
				hit += len(a.rt.hitTest(300, y))
			}
			b.ReportMetric(float64(hit)/float64(b.N), "hits/op")
			b.ReportMetric(float64(a.rt.Stats().NodesPainted), "painted")
		})
	}
}

// BenchmarkKeyDispatch sends a key press and its release through the focus
// path and the bindings, without a frame, so it times the dispatch alone. The
// focused field is at the top of the window, so the focus path is short; the
// window is 100, 1,000 and 10,000 nodes. Right moves the caret of the field,
// and Ctrl+K runs the global binding on the root.
func BenchmarkKeyDispatch(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			hits := 0
			rt, _ := dispatchBenchApp(size, &hits)
			b.Run("field-caret", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rt.KeyDown(input.KeyRight, 0, false)
					rt.KeyUp(input.KeyRight, 0)
				}
			})
			b.Run("global-binding", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rt.KeyDown(input.KeyK, input.ModCtrl, false)
					rt.KeyUp(input.KeyK, input.ModCtrl)
				}
				if hits == 0 {
					b.Fatal("the global binding never ran")
				}
			})
		})
	}
}
