package ui

import (
	"fmt"
	"testing"
	"time"
)

// A Computed runs its function only when it is read after a signal it read
// has changed.
func TestComputedIsLazyAndCached(t *testing.T) {
	a := NewSignal(2)
	runs := 0
	double := NewComputed(func() int { runs++; return a.Get() * 2 })
	if runs != 0 {
		t.Fatal("the function ran before the value was read")
	}
	if double.Peek() != 4 || double.Get() != 4 || runs != 1 {
		t.Fatalf("value %d after %d runs, want 4 after 1", double.Peek(), runs)
	}
	a.Set(5)
	if runs != 1 {
		t.Fatal("a change ran the function before the value was read")
	}
	if double.Get() != 10 || runs != 2 {
		t.Fatalf("value %d after %d runs, want 10 after 2", double.Get(), runs)
	}
}

// A component that reads a Computed re-renders when a signal under it
// changes, through a Computed of a Computed too, and Peek does not subscribe.
func TestComputedRerendersReaders(t *testing.T) {
	items := NewSignal([]int{1, 2, 3})
	sum := NewComputed(func() int {
		total := 0
		for _, v := range items.Get() {
			total += v
		}
		return total
	})
	label := NewComputed(func() string { return fmt.Sprint(sum.Get()) })
	shown, peeked := "", 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().
			Child(Component(func(ctx *Context) Element { shown = label.Get(); return Text(shown) })).
			Child(Component(func(ctx *Context) Element { peeked = sum.Peek(); return Text("peek") }))
	}, 300, 200, 1)
	rt.Frame(time.Now())
	if shown != "6" || peeked != 6 {
		t.Fatalf("shown %q, peeked %d", shown, peeked)
	}
	items.Set([]int{10, 20})
	rt.Frame(time.Now())
	if shown != "30" {
		t.Fatalf("after the change the component shows %q, want 30", shown)
	}
	if peeked != 6 {
		t.Fatalf("a component that only peeked re-rendered (saw %d)", peeked)
	}
}

// The dependencies are the signals the last computation read: a signal the
// function stopped reading no longer recomputes it.
func TestComputedTracksTheLastReads(t *testing.T) {
	useA := NewSignal(true)
	a, b := NewSignal("a"), NewSignal("b")
	runs := 0
	pick := NewComputed(func() string {
		runs++
		if useA.Get() {
			return a.Get()
		}
		return b.Get()
	})
	pick.Get()
	useA.Set(false)
	if pick.Get() != "b" {
		t.Fatalf("value %q, want b", pick.Get())
	}
	before := runs
	a.Set("changed")
	pick.Get()
	if runs != before {
		t.Fatal("a signal the function no longer reads made it run again")
	}
}

// A change while the function runs leaves the result stale: the next read
// runs the function again.
func TestComputedChangeDuringComputation(t *testing.T) {
	a := NewSignal(1)
	runs := 0
	var c *Computed[int]
	c = NewComputed(func() int {
		runs++
		v := a.Get()
		if runs == 1 {
			c.invalidate() // as a Set from another goroutine would, mid-computation
		}
		return v
	})
	c.Get()
	c.Get()
	if runs != 2 {
		t.Fatalf("the function ran %d times, want 2: the first result was kept as current", runs)
	}
}

// A panicking function leaves the render tracker as it was.
func TestComputedRestoresTheTrackerOnPanic(t *testing.T) {
	c := NewComputed(func() int { panic("boom") })
	func() {
		defer func() { _ = recover() }()
		c.Get()
	}()
	if tracking.Load() != nil {
		t.Fatal("the tracker still points at the Computed after a panic")
	}
}
