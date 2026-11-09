package ui

import "testing"

// A touch drags an element that captures the pointer, as a slider does, and
// scrolls elsewhere (see hostDragsAt).
func TestDragsAt(t *testing.T) {
	v := NewSignal(50.0)
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Gap(20).Padding(20).
			Child(Text("Volume")).
			Child(Slider(v, 0, 100, 1).Label("Volume").Width(Px(300)))
	})
	var slider *node
	var walk func(n *node)
	walk = func(n *node) {
		if n.div != nil && n.div.capture && slider == nil {
			slider = n
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(r.rt.tree)
	if slider == nil {
		t.Fatal("no element of the slider captures the pointer")
	}
	cx, cy := slider.abs.X+slider.lay.W/2, slider.abs.Y+slider.lay.H/2
	if !r.rt.hostDragsAt(cx, cy) {
		t.Errorf("hostDragsAt(%v, %v) on the slider = false, want true", cx, cy)
	}
	if r.rt.hostDragsAt(25, 25) {
		t.Error("hostDragsAt on the text = true, want false")
	}
	r.rt.SetZoom(2)
	frame(r.rt)
	if !r.rt.hostDragsAt(slider.abs.X*2+slider.lay.W, slider.abs.Y*2+slider.lay.H) {
		t.Error("hostDragsAt misses the slider at zoom 2")
	}
}
