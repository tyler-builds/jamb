package ui

import (
	"fmt"
	"testing"
	"time"
)

// splitBenchRuntime returns a laid-out window of n splits in a column. Each split has two panes, and
// each pane holds a few dozen elements, so the tree has several thousand nodes.
func splitBenchRuntime(b *testing.B, n int) *Runtime {
	b.Helper()
	rt := NewRuntime(func(ctx *Context) Element {
		col := Div().Column()
		for i := 0; i < n; i++ {
			col.Child(Split(Pane(benchPane()).Size(Px(200)), Pane(benchPane())).
				Key(fmt.Sprintf("bench-split-%d", i)).Height(Px(60)))
		}
		return col
	}, DarkTheme(), nil)
	rt.SetViewport(800, 3200, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	rt.Frame(now.Add(time.Second))
	return rt
}

// benchPane returns a pane's content: a column of 30 short rows.
func benchPane() Element {
	col := Div().Column()
	for i := 0; i < 30; i++ {
		col.Child(Div().Height(Px(4)))
	}
	return col
}

// splitsResizedWalk is the check that splitsResized did before it kept each split's box: it searches
// the tree for every split's key on every call. It is kept here as the baseline of the benchmark.
func splitsResizedWalk(rt *Runtime) bool {
	changed := false
	for inst, w := range rt.splitWatches {
		if n := rt.findKey(rt.tree, inst.root.key); n != nil && axisLength(n, w.vert) != w.seen {
			changed = true
		}
	}
	return changed
}

// BenchmarkSplitsResizedWalk is the check of the window's 50 splits with the tree walk (the baseline).
func BenchmarkSplitsResizedWalk(b *testing.B) {
	rt := splitBenchRuntime(b, 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if splitsResizedWalk(rt) {
			b.Fatal("a split changed in a steady window")
		}
	}
}

// BenchmarkSplitsResized is the check of the window's 50 splits as the runtime makes it: each split
// is its instance's root, and the check reads it from the map of live nodes.
func BenchmarkSplitsResized(b *testing.B) {
	rt := splitBenchRuntime(b, 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if rt.splitsResized() {
			b.Fatal("a split changed in a steady window")
		}
	}
}
