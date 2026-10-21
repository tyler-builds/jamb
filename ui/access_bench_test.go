package ui

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkAccessTree builds the accessibility tree of the scale app, from 100 to
// 10,000 nodes. This is the full build an assistive technology's first request
// costs, and each later change costs about the same, as the tree is built again
// and then diffed (see BenchmarkAccessUpdate).
func BenchmarkAccessTree(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			a := newScaleApp(size)
			var accNodes int
			b.ReportAllocs()
			for b.Loop() {
				accNodes = a.rt.AccessTree().Len()
			}
			b.ReportMetric(float64(accNodes), "access-nodes")
		})
	}
}

// BenchmarkAccessUpdate is a frame in which one row's text changes, with an
// assistive technology listening. The frame rebuilds the access tree, diffs it
// against the last one and publishes the update. The bridge is activated before
// the loop, as it is when a screen reader first asks for the tree.
func BenchmarkAccessUpdate(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			a := newScaleApp(size)
			a.rt.accessState().bridge.Activate()
			now := time.Unix(1_000_000, 0)
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				a.values[len(a.values)/2].Set(fmt.Sprintf("Row edited %d", i))
				now = now.Add(16 * time.Millisecond)
				a.rt.Frame(now)
			}
			if a.rt.accessState().bridge.Tree() == nil {
				b.Fatal("no access tree was published")
			}
		})
	}
}
