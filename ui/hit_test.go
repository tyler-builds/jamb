package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// A hover style that lifts an element moves its painted box, but not its hit
// box. Hit testing used the painted box, so the lifted element could leave the
// pointer: the hover ended, the element dropped back under the pointer, and the
// hover began again, every frame, and the runtime never settled.
func TestHoverOffsetDoesNotMoveHitBox(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("lift").Size(Px(100), Px(40)).Transition(100 * time.Millisecond).
				Hover(Style{OffsetY: -20}))
	}, 300, 300)
	r.rt.PointerMove(50, 55, 0) // inside the layout box (y 20..60), below the lifted box
	lifted := func() bool { return r.rt.findKey(r.rt.tree, "lift").style.OffsetY == -20 }
	changes := 0
	prev := false
	for i := range 200 {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
		if i >= 60 && lifted() != prev {
			changes++
		}
		prev = lifted()
	}
	if changes != 0 {
		t.Errorf("the hover changed %d times after the transition, with the pointer at rest", changes)
	}
	if !lifted() {
		t.Error("the hover style is not applied while the pointer rests on the element")
	}
	if !r.rt.Settled() {
		t.Error("the runtime keeps animating with the pointer at rest")
	}
}

// A static offset (Style.Offset) moves the hit box with the element, so a
// click where the element is painted still reaches it.
func TestStaticOffsetMovesHitBox(t *testing.T) {
	var clicked int
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("shifted").Size(Px(100), Px(40)).Offset(0, 30).
				OnClick(func() { clicked++ }))
	}, 300, 300)
	press := func(x, y float32) {
		r.rt.PointerMove(x, y, 0)
		r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		r.rt.PointerUp(x, y, input.ButtonLeft, 0)
		r.frame()
	}
	press(50, 75) // painted at y 50..90
	if clicked != 1 {
		t.Errorf("a click where the offset element is painted did not reach it (clicks %d)", clicked)
	}
	press(50, 35) // the layout box, which the offset moved away
	if clicked != 1 {
		t.Errorf("a click outside the painted offset element reached it (clicks %d)", clicked)
	}
}
