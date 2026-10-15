package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// StyleOf reports the drawn fill, border, text color, opacity and ring of an
// element, with its hover and keyboard focus styles merged in.
func TestStyleOfReportsDrawnValues(t *testing.T) {
	red, blue, green := RGB(200, 0, 0), RGB(0, 0, 200), RGB(0, 160, 0)
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Style(Style{Opacity: 0.5, TextColor: green}).Child(
			Div().Key("box").Size(Px(100), Px(40)).Focusable().
				Style(Style{Background: red, BorderWidth: 2, BorderColor: blue}).
				Hover(Style{Background: blue}).
				FocusVisible(Style{Shadows: []Shadow{{Spread: 3, Color: green}}}))
	}, DarkTheme(), nil)
	rt.SetViewport(400, 300, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)

	d, ok := rt.StyleOf("box")
	if !ok || !d.Painted {
		t.Fatalf("StyleOf(box) = %+v, %v; want a painted element", d, ok)
	}
	if d.Background != red || d.BorderColor != blue || d.BorderWidth != All(2) {
		t.Errorf("fill %v, border %v %v; want %v, %v 2", d.Background, d.BorderColor, d.BorderWidth, red, blue)
	}
	if d.TextColor != green || d.Opacity != 0.5 {
		t.Errorf("text color %v, opacity %v; want the parent's %v and 0.5", d.TextColor, d.Opacity, green)
	}
	if d.RingWidth != 0 || d.Bounds.W != 100 || d.Bounds.H != 40 {
		t.Errorf("ring %v, bounds %v; want no ring and 100x40", d.RingWidth, d.Bounds)
	}

	rt.PointerMove(d.Bounds.X+10, d.Bounds.Y+10, 0)
	rt.Frame(now)
	if d, _ = rt.StyleOf("box"); d.Background != blue {
		t.Errorf("hovered fill %v, want %v", d.Background, blue)
	}

	rt.Press(input.KeyTab, 0)
	if d, _ = rt.StyleOf("box"); d.RingWidth != 3 || d.Ring != green {
		t.Errorf("ring after Tab: %v %v, want 3 %v", d.RingWidth, d.Ring, green)
	}

	if _, ok := rt.StyleOf("missing"); ok {
		t.Error("StyleOf found an element with a key no element has")
	}
}

// A transition is reported at the point it reached, and an element scrolled
// out of view is reported as not painted.
func TestStyleOfTransitionAndCulled(t *testing.T) {
	black, white := RGB(0, 0, 0), RGB(255, 255, 255)
	lit := NewSignal(false)
	rt := NewRuntime(func(ctx *Context) Element {
		bg := black
		if lit.Get() {
			bg = white
		}
		return Div().Height(Px(100)).Style(Style{OverflowY: OverflowScroll}).Children(
			Div().Key("fade").Size(Px(50), Px(50)).Style(Style{Background: bg}).Transition(200*time.Millisecond),
			Div().Height(Px(2000)),
			Div().Key("far").Size(Px(50), Px(50)),
		)
	}, DarkTheme(), nil)
	rt.SetViewport(400, 300, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)

	lit.Set(true)
	now = now.Add(16 * time.Millisecond)
	rt.Frame(now)
	d, _ := rt.StyleOf("fade")
	if d.Background == black || d.Background == white {
		t.Errorf("fill one frame into the transition is %v, want between black and white", d.Background)
	}
	for range 60 {
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	if d, _ = rt.StyleOf("fade"); d.Background != white {
		t.Errorf("fill after the transition is %v, want white", d.Background)
	}

	d, ok := rt.StyleOf("far")
	if !ok || d.Painted {
		t.Errorf("StyleOf(far) = %+v, %v; want found and not painted", d, ok)
	}
}
