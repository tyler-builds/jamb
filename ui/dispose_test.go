package ui

import (
	"strings"
	"testing"
)

// TestOnDisposeRunsEachCallbackOnceInOrder checks that callbacks registered during a render run in
// the order they were registered, once each, when the component is removed. A re-render must not
// register them again, because the component guards the registration with a local.
func TestOnDisposeRunsEachCallbackOnceInOrder(t *testing.T) {
	show := NewSignal(true)
	tick := NewSignal(0)
	var calls []string
	rt := newTestRuntime(func(ctx *Context) Element {
		if !show.Get() {
			return Text("gone")
		}
		return Component(func(ctx *Context) Element {
			_ = tick.Get()
			registered := Local(ctx, "registered", false)
			if !registered.Peek() {
				registered.Set(true)
				ctx.OnDispose(func() { calls = append(calls, "first") })
				ctx.OnDispose(func() { calls = append(calls, "second") })
			}
			return Div().Key("box").Height(Px(20))
		})
	}, 400, 300, 1)
	tick.Set(1)
	frame(rt)
	tick.Set(2)
	frame(rt)
	if len(calls) != 0 {
		t.Fatalf("callbacks ran before the component was removed: %v", calls)
	}
	show.Set(false)
	frame(rt)
	if got := strings.Join(calls, ","); got != "first,second" {
		t.Fatalf("callbacks ran as %q, want first,second once each", got)
	}
	frame(rt)
	if got := strings.Join(calls, ","); got != "first,second" {
		t.Fatalf("callbacks ran again on a later frame: %q", got)
	}
}

// TestOnDisposeOutsideRenderPanics checks that a registration with no rendering component is refused,
// as Local refuses one.
func TestOnDisposeOutsideRenderPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("OnDispose with no rendering component did not panic")
		}
	}()
	(&Context{}).OnDispose(func() {})
}
