package ui

import (
	"strings"
	"testing"
	"time"
)

func TestPostRunsOnNextFrame(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element { return Div() })
	woke := make(chan struct{}, 1)
	r.rt.SetWakeFunc(func() {
		select {
		case woke <- struct{}{}:
		default:
		}
	})
	ran := false
	done := make(chan struct{})
	go func() {
		r.rt.Post(func() { ran = true })
		close(done)
	}()
	<-done
	select {
	case <-woke:
	case <-time.After(2 * time.Second):
		t.Fatal("Post did not request a frame")
	}
	if ran {
		t.Fatal("the posted function ran before the frame")
	}
	r.frame()
	if !ran {
		t.Fatal("the posted function did not run on the next frame")
	}
}

func TestSetBindingsReplacesLayerInPlace(t *testing.T) {
	a := NewAction("test.a", "A")
	b := NewAction("test.b", "B")
	b2 := NewAction("test.b2", "B2")
	c := NewAction("test.c", "C")
	r := newKeyRig(func(ctx *Context) Element { return Div() })
	r.rt.AddBindings(Bind("ctrl+1", a))
	r.rt.SetBindings("x", []Binding{Bind("ctrl+2", b)})
	r.rt.AddBindings(Bind("ctrl+3", c))
	r.rt.SetBindings("x", []Binding{Bind("ctrl+4", b2)})
	if got := testKeys(r.rt); got != "ctrl+1 ctrl+4 ctrl+3" {
		t.Errorf("order after replacing the layer: %q, want %q", got, "ctrl+1 ctrl+4 ctrl+3")
	}
	r.rt.SetBindings("x", nil)
	if got := testKeys(r.rt); got != "ctrl+1 ctrl+3" {
		t.Errorf("order after emptying the layer: %q, want %q", got, "ctrl+1 ctrl+3")
	}
	r.rt.SetBindings("x", []Binding{Bind("ctrl+5", b)})
	if got := testKeys(r.rt); got != "ctrl+1 ctrl+3 ctrl+5" {
		t.Errorf("layer re-added: order %q, want it appended", got)
	}
}

func TestSetBindingsRejectsEmptyLayer(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element { return Div() })
	defer func() {
		if recover() == nil {
			t.Error(`SetBindings("") did not panic`)
		}
	}()
	r.rt.SetBindings("", nil)
}

// testKeys lists the keys of the bindings the test added (actions named
// "test.*"), in precedence order.
func testKeys(rt *Runtime) string {
	var keys []string
	for _, b := range rt.keymap {
		if strings.HasPrefix(b.action.Name(), "test.") {
			keys = append(keys, b.keys)
		}
	}
	return strings.Join(keys, " ")
}

// Context.Post hands work from a goroutine that holds a component's context
// back to the UI thread, on the next frame.
func TestContextPostRunsOnNextFrame(t *testing.T) {
	var held *Context
	r := newKeyRig(func(ctx *Context) Element { held = ctx; return Div() })
	done := make(chan struct{})
	ran := false
	go func() {
		held.Post(func() { ran = true })
		close(done)
	}()
	<-done
	if ran {
		t.Fatal("the posted function ran before the frame")
	}
	r.frame()
	if !ran {
		t.Fatal("the posted function did not run on the next frame")
	}
}
