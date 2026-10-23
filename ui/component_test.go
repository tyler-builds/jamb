package ui

import (
	"testing"
	"time"
)

// TestKeyedOwnerKeepsItsKeyWhenItsOutputRendersAgain checks that a keyed component
// whose output is another component still answers to its key after that inner
// component renders again on its own.
func TestKeyedOwnerKeepsItsKeyWhenItsOutputRendersAgain(t *testing.T) {
	n := NewSignal(0)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Component(func(*Context) Element {
			return Component(func(*Context) Element { return Textf("%d", n.Get()) })
		}).Key("outer"))
	}, 400, 300, 1)
	if _, ok := rt.Find("outer"); !ok {
		t.Fatal("the key finds nothing after the first build")
	}
	n.Set(1)
	rt.Frame(time.Now())
	if _, ok := rt.Find("outer"); !ok {
		t.Error("the key finds nothing after the inner component rendered again")
	}
}
