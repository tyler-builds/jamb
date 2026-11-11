package ui

import (
	"fmt"
	"testing"
	"time"
)

// The 3,000-row benchmarks: a column of 3,000 rows, every one of them in the
// window, so none is left out of the tree or culled. Each row is a component that
// reads its own label signal, and whether it is selected. The figures answer
// "when one value changes, what runs?" for jamb, as the GPUI critique of
// 2026-11-08 asked it of other frameworks (one label among 3,000 live rows).
//
//	go test ./ui -run xxx -bench Rows3000 -benchtime 2s
//
// Reported per operation, besides the standard figures: rendered/op, the
// components whose render function ran, and built/op and laidout/op, the nodes
// created and re-positioned (see BenchmarkScale).

const benchRows = 3000

// rowsBenchApp builds the 3,000 rows. With computed set, each row reads whether
// it is selected through a Computed of its own; otherwise it reads the selected
// row's index from the shared signal.
func rowsBenchApp(computed bool) (*Runtime, []*Signal[string], *Signal[int]) {
	labels := make([]*Signal[string], benchRows)
	for i := range labels {
		labels[i] = NewSignal(fmt.Sprintf("Row %d", i))
	}
	selected := NewSignal(-1)
	isSelected := make([]*Computed[bool], benchRows)
	for i := range isSelected {
		isSelected[i] = NewComputed(func() bool { return selected.Get() == i })
	}
	rt := NewRuntime(func(ctx *Context) Element {
		list := Div()
		for i, label := range labels {
			list.Child(Component(func(ctx *Context) Element {
				sel := false
				if computed {
					sel = isSelected[i].Get()
				} else {
					sel = selected.Get() == i
				}
				row := Div().Row().Height(Px(20)).Padding(2).Child(Text(label.Get()))
				if sel {
					row.Background(Hex("#27272a"))
				}
				return row
			}).Key(fmt.Sprint(i)))
		}
		return list
	}, DarkTheme(), benchText)
	rt.SetViewport(800, benchRows*20, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	rt.Frame(now.Add(time.Second))
	return rt, labels, selected
}

// BenchmarkRows3000 changes one value of the 3,000 rows and runs a frame:
//
//	label            one row's label changes
//	label-same       one row's label is written with the value it has
//	selected-same    the selected row, which every row reads, is written with the value it has
//	selected-move    the selection moves to the next row, which each row reads through a Computed
func BenchmarkRows3000(b *testing.B) {
	run := func(b *testing.B, computed bool, write func(labels []*Signal[string], selected *Signal[int], i int)) {
		rt, labels, selected := rowsBenchApp(computed)
		now := time.Unix(1_000_100, 0)
		var r, bu, l, i int
		b.ReportAllocs()
		for b.Loop() {
			i++
			write(labels, selected, i)
			now = now.Add(16 * time.Millisecond)
			rt.Frame(now)
			st := rt.Stats()
			r, bu, l = r+st.Rendered, bu+st.NodesBuilt, l+st.NodesLaidOut
		}
		b.ReportMetric(float64(r)/float64(b.N), "rendered/op")
		b.ReportMetric(float64(bu)/float64(b.N), "built/op")
		b.ReportMetric(float64(l)/float64(b.N), "laidout/op")
	}
	b.Run("label", func(b *testing.B) {
		run(b, false, func(labels []*Signal[string], _ *Signal[int], i int) {
			labels[benchRows/2].Set(fmt.Sprintf("Row edited %d", i%2))
		})
	})
	b.Run("label-same", func(b *testing.B) {
		run(b, false, func(labels []*Signal[string], _ *Signal[int], _ int) {
			labels[benchRows/2].Set(labels[benchRows/2].Peek())
		})
	})
	b.Run("selected-same", func(b *testing.B) {
		run(b, false, func(_ []*Signal[string], selected *Signal[int], _ int) {
			selected.Set(benchRows / 2)
		})
	})
	b.Run("selected-move", func(b *testing.B) {
		run(b, true, func(_ []*Signal[string], selected *Signal[int], i int) {
			selected.Set(benchRows/2 + i%2)
		})
	})
}
