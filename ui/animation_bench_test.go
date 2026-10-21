package ui

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkAnimations is a frame of n animations that all run at once, each
// on its own row of a column that is entirely in view. Each animation is a
// repeating Animate that re-renders its bar with the eased progress, so every
// frame re-renders and re-lays out each bar. Each iteration advances the clock
// by one 60 Hz frame.
func BenchmarkAnimations(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("running=%d", n), func(b *testing.B) {
			rt := NewRuntime(func(ctx *Context) Element {
				col := Div().Column().Gap(4).Padding(8)
				for i := range n {
					col.Child(Animate(fmt.Sprint(i), Animation{Duration: time.Second, Repeat: true},
						func(t float32) Element {
							return Div().Height(Px(20)).Width(Px(40 + 300*t)).Radius(4).Background(Hex("#6366f1"))
						}))
				}
				return col
			}, DarkTheme(), nil)
			rt.SetViewport(400, float32(n*24+16), 1)
			now := time.Unix(1_000_000, 0)
			rt.Frame(now)
			b.ReportAllocs()
			for b.Loop() {
				now = now.Add(16 * time.Millisecond)
				rt.Frame(now)
			}
			b.ReportMetric(float64(rt.Stats().Nodes), "nodes")
		})
	}
}
