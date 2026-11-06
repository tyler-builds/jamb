package ui

import (
	"testing"
	"time"
)

// An app with no Matched element pays nothing for shared-element motion: its
// builds, paints, exits and layout springs leave the runtime's match state
// unmade.
func TestMatchedCostsNothingWithout(t *testing.T) {
	on := NewSignal(true)
	rt := NewRuntime(func(ctx *Context) Element {
		list := Div().Column().ChildrenLayoutSpring(ctx.Theme().Motion.ReflowSpring())
		if on.Get() {
			list.Child(Div().Key("a").Size(Px(80), Px(40)).Background(Hex("#333")).
				Exit(Fade(), Animation{Duration: 100 * time.Millisecond}))
		}
		list.Child(Div().Key("b").Size(Px(80), Px(40)).Background(Hex("#666")))
		return list
	}, nil, nil)
	rt.SetViewport(400, 300, 1)
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	for i := range 40 {
		if i == 5 {
			on.Set(false)
		}
		rt.Frame(now.Add(time.Duration(i) * 16 * time.Millisecond))
	}
	if rt.match != nil {
		t.Errorf("the runtime made shared-element state for an app with no Matched element")
	}
}
