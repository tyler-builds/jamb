package ui

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"
)

// countRenders runs a component that calls read, and counts its renders after the first.
func countRenders(read func()) (*Runtime, *int) {
	n := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Component(func(ctx *Context) Element {
			n++
			read()
			return Text("x")
		}))
	}, 300, 200, 1)
	rt.Frame(time.Now())
	n = 0
	return rt, &n
}

// A write that leaves a signal's value as it was re-renders no reader, and asks
// for no frame; a write that changes it re-renders them.
func TestSignalSameWriteRendersNothing(t *testing.T) {
	label := NewSignal("Row 1")
	rt, n := countRenders(func() { label.Get() })

	label.Set("Row 1")
	label.Update(func(s string) string { return s })
	if rt.NeedsFrame() {
		t.Fatal("writes of the same value asked for a frame")
	}
	rt.Frame(time.Now())
	if *n != 0 {
		t.Fatalf("writes of the same value rendered the reader %d times", *n)
	}

	label.Set("Row 2")
	rt.Frame(time.Now())
	if *n != 1 {
		t.Fatalf("a changed value rendered the reader %d times, want 1", *n)
	}
	label.Update(func(s string) string { return s + "!" })
	rt.Frame(time.Now())
	if *n != 2 || label.Peek() != "Row 2!" {
		t.Fatalf("an update rendered the reader %d times in all, want 2, and left %q", *n, label.Peek())
	}
}

// Values that == cannot compare always notify, and EqualFunc replaces the comparison.
func TestSignalEqualFunc(t *testing.T) {
	items := NewSignal([]string{"a", "b"})
	rt, n := countRenders(func() { items.Get() })
	items.Set([]string{"a", "b"})
	rt.Frame(time.Now())
	if *n != 1 {
		t.Fatalf("a slice write rendered the reader %d times, want 1 (slices are never the same)", *n)
	}

	items.EqualFunc(slices.Equal[[]string])
	items.Set([]string{"a", "b"})
	rt.Frame(time.Now())
	if *n != 1 {
		t.Fatal("with slices.Equal, an equal slice re-rendered the reader")
	}
	items.Set([]string{"a", "c"})
	rt.Frame(time.Now())
	if *n != 2 {
		t.Fatalf("with slices.Equal, a different slice rendered the reader %d times in all, want 2", *n)
	}

	count := NewSignal(3).EqualFunc(func(a, b int) bool { return false })
	rt, n = countRenders(func() { count.Get() })
	count.Set(3)
	rt.Frame(time.Now())
	if *n != 1 {
		t.Fatal("an EqualFunc that is never true did not notify on the same value")
	}
}

func TestSame(t *testing.T) {
	type point struct{ X, Y int }
	type withSlice struct{ S []int }
	type withIface struct{ V any }
	p := &point{1, 2}
	for _, c := range []struct {
		name string
		got  bool
		want bool
	}{
		{"strings", same("a", "a"), true},
		{"different strings", same("a", "b"), false},
		{"ints", same(3, 3), true},
		{"uint8", same[uint8](3, 3), true},
		{"NaN", same(math.NaN(), math.NaN()), false},
		{"structs", same(point{1, 2}, point{1, 2}), true},
		{"different structs", same(point{1, 2}, point{1, 3}), false},
		{"the same pointer", same(p, p), true},
		{"equal pointees", same(p, &point{1, 2}), false},
		{"arrays", same([2]int{1, 2}, [2]int{1, 2}), true},
		{"slices", same([]int{1}, []int{1}), false},
		{"maps", same(map[int]int{}, map[int]int{}), false},
		{"funcs", same[func()](nil, nil), false},
		{"a struct with a slice", same(withSlice{}, withSlice{}), false},
		{"a struct with an interface", same(withIface{1}, withIface{1}), false},
		{"interfaces of ints", same[any](1, 1), true},
		{"interfaces of different types", same[any](1, "1"), false},
		{"nil interfaces", same[any](nil, nil), true},
		{"interfaces of slices", same[any]([]int{1}, []int{1}), false}, // and no panic
		{"errors", same[error](fmt.Errorf("x"), fmt.Errorf("x")), false},
	} {
		if c.got != c.want {
			t.Errorf("%s: same is %v, want %v", c.name, c.got, c.want)
		}
	}
}

// A Computed whose value comes out the same after a signal under it changed
// re-renders no reader; one whose value changes re-renders only its readers.
// The rows here read whether they are selected through a Computed each, and
// moving the selection re-renders the two rows whose answer changed.
func TestComputedSameValueRendersNothing(t *testing.T) {
	const rows = 50
	selected := NewSignal(-1)
	isSelected := make([]*Computed[bool], rows)
	for i := range isSelected {
		isSelected[i] = NewComputed(func() bool { return selected.Get() == i })
	}
	rendered := map[int]int{}
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div()
		for i := range rows {
			list.Child(Component(func(ctx *Context) Element {
				rendered[i]++
				return Text(fmt.Sprint(isSelected[i].Get()))
			}).Key(fmt.Sprint(i)))
		}
		return list
	}, 300, 2000, 1)
	rt.Frame(time.Now())

	clear(rendered)
	selected.Set(3)
	rt.Frame(time.Now())
	if len(rendered) != 1 || rendered[3] != 1 {
		t.Fatalf("selecting row 3 rendered %v, want row 3 once", rendered)
	}
	clear(rendered)
	selected.Set(4)
	rt.Frame(time.Now())
	if len(rendered) != 2 || rendered[3] != 1 || rendered[4] != 1 {
		t.Fatalf("moving the selection to row 4 rendered %v, want rows 3 and 4 once", rendered)
	}
	clear(rendered)
	selected.Set(40) // then back before a frame: nothing changed in the end
	selected.Set(4)
	rt.Frame(time.Now())
	if len(rendered) != 0 {
		t.Fatalf("a selection that came back to row 4 rendered %v", rendered)
	}
	if rt.NeedsFrame() {
		t.Fatal("a frame with nothing changed asked for another")
	}
}

// Through a chain of Computeds, a value that comes out the same stops the
// change: the Computeds after it do not run, and their readers do not re-render.
func TestComputedChainStopsAtTheSameValue(t *testing.T) {
	n := NewSignal(2)
	parity := NewComputed(func() int { return n.Get() % 2 })
	labelRuns := 0
	label := NewComputed(func() string {
		labelRuns++
		if parity.Get() == 0 {
			return "even"
		}
		return "odd"
	})
	shown := ""
	rt, renders := countRenders(func() { shown = label.Get() })
	labelRuns = 0

	n.Set(4)
	rt.Frame(time.Now())
	if *renders != 0 || labelRuns != 0 {
		t.Fatalf("an unchanged parity rendered the reader %d times and ran the label %d times", *renders, labelRuns)
	}
	n.Set(5)
	rt.Frame(time.Now())
	if *renders != 1 || shown != "odd" {
		t.Fatalf("a changed parity rendered the reader %d times showing %q, want once showing odd", *renders, shown)
	}
}

// A Computed read outside a render (an event handler's Peek) still tells its
// readers when it finds that its value changed, and a reader that computes the
// value itself during its render is not rendered a second time for it.
func TestComputedPeekedChangeReachesReaders(t *testing.T) {
	a := NewSignal(1)
	double := NewComputed(func() int { return a.Get() * 2 })
	shown := 0
	rt, renders := countRenders(func() { shown = double.Get() })

	a.Set(2)
	if double.Peek() != 4 {
		t.Fatal("Peek did not compute the new value")
	}
	rt.Frame(time.Now())
	if *renders != 1 || shown != 4 {
		t.Fatalf("after a Peek computed the change, the reader rendered %d times showing %d, want once showing 4", *renders, shown)
	}
	a.Set(3)
	rt.Frame(time.Now())
	rt.Frame(time.Now())
	if *renders != 2 || shown != 6 {
		t.Fatalf("the reader rendered %d times in all showing %d, want twice showing 6", *renders, shown)
	}
}

// Computed.EqualFunc replaces the comparison of results.
func TestComputedEqualFunc(t *testing.T) {
	items := NewSignal([]int{1, 2})
	sorted := NewComputed(func() []int { return slices.Sorted(slices.Values(items.Get())) }).
		EqualFunc(slices.Equal[[]int])
	rt, renders := countRenders(func() { sorted.Get() })
	items.Set([]int{2, 1})
	rt.Frame(time.Now())
	if *renders != 0 {
		t.Fatal("an equal sorted slice re-rendered the reader")
	}
	items.Set([]int{3, 1})
	rt.Frame(time.Now())
	if *renders != 1 {
		t.Fatalf("a different sorted slice rendered the reader %d times, want 1", *renders)
	}
}

// Comparing allocates nothing, so that a write pays only for the comparison.
func TestSameAllocatesNothing(t *testing.T) {
	type point struct{ X, Y float32 }
	s1, s2 := "Row 1", fmt.Sprint("Row ", 1)
	p := &point{}
	var ok bool
	if n := testing.AllocsPerRun(100, func() {
		ok = same(s1, s2) && same(point{1, 2}, point{1, 2}) && same(p, p) && same(7, 7) && same(int32(1), int32(1))
	}); n != 0 || !ok {
		t.Fatalf("same allocated %v times a run (and reported %v)", n, ok)
	}
}
