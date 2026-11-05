package ui

import (
	"runtime"
	"testing"
	"time"
	"weak"
)

// A component that left the tree can be collected, whatever it read. Neither a
// long-lived Signal or Computed it read, through the spare capacity of its
// subscriber list, nor the layout engine's reused scratch memory keeps a
// pointer to it. A query's version signal outlives its users for the query's
// cache time, so such a pointer kept every component that used a cached query,
// and its whole subtree, alive.
func TestRemovedComponentIsCollected(t *testing.T) {
	sig := NewSignal(1)
	comp := NewComputed(func() int { return sig.Get() + 1 })
	for _, tc := range []struct {
		name string
		read func()
	}{
		{"no signal", func() {}},
		{"a Signal", func() { sig.Get() }},
		{"a Computed", func() { comp.Get() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			show := NewSignal(true)
			var held weak.Pointer[[1 << 16]byte]
			mount := func() Element {
				payload := new([1 << 16]byte) // reachable only through the component
				held = weak.Make(payload)
				return Component(func(ctx *Context) Element {
					tc.read()
					return Text(string(payload[:1]))
				}).Key("child")
			}
			child := mount()
			rt := newTestRuntime(func(ctx *Context) Element {
				d := Div()
				if show.Get() {
					d = d.Child(child)
				}
				return d
			}, 300, 200, 1)
			child = nil
			show.Set(false)
			rt.Frame(time.Now())
			rt.Frame(time.Now())
			if _, ok := rt.Find("child"); ok {
				t.Fatal("the component did not leave the tree")
			}
			runtime.GC()
			runtime.GC()
			if held.Value() != nil {
				t.Fatalf("a removed component that read %s is still reachable", tc.name)
			}
			runtime.KeepAlive(rt)
		})
	}
	runtime.KeepAlive(sig)
	runtime.KeepAlive(comp)
}
