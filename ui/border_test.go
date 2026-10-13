package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/scene"
)

// BorderEdges insets the content by each side's width, as padding does.
func TestBorderEdgesInsetContent(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Key("box").Size(Px(100), Px(80)).BorderEdges(Edges{Top: 4, Left: 10}, DarkTheme().Accent).
			Child(Div().Key("child").Size(Px(10), Px(10)))
	}, 800, 600)
	box, child := r.find(t, "box"), r.find(t, "child")
	if child.X != box.X+10 || child.Y != box.Y+4 {
		t.Errorf("child at (%v,%v) in box %+v; want offset (10,4)", child.X-box.X, child.Y-box.Y, box)
	}
}

// A uniform Border and per-side BorderEdges replace each other: the later one
// set wins, and a zero side of BorderEdges has no border.
func TestBorderStyleMerge(t *testing.T) {
	c := Hex("#ff0000")
	uniform := Style{BorderWidth: 2, BorderColor: c}
	sides := Style{BorderEdges: Edges{Bottom: 1}, BorderColor: c}

	if got := uniform.borderEdges(); got != All(2) {
		t.Errorf("uniform border edges %+v, want all 2", got)
	}
	both := uniform.Merge(sides)
	if got := both.borderEdges(); got != (Edges{Bottom: 1}) {
		t.Errorf("sides after uniform: edges %+v, want only the bottom 1", got)
	}
	if both.BorderEdges != (Edges{Bottom: 1}) || both.BorderWidth != 2 {
		t.Errorf("merge changed the stored fields: %+v", both)
	}
	back := both.Merge(Style{BorderWidth: 3})
	if got := back.borderEdges(); got != All(3) || back.BorderEdges != (Edges{}) {
		t.Errorf("uniform after sides: edges %+v, stored sides %+v; want all 3 and no sides", got, back.BorderEdges)
	}

	d := Div().BorderEdges(Edges{Left: 3}, c).Border(1, c)
	if d.style.BorderEdges != (Edges{}) || d.style.borderEdges() != All(1) {
		t.Errorf("Border after BorderEdges left sides %+v", d.style.BorderEdges)
	}
}

// Each corner of a border's inner edge loses the average of its two sides, so
// a uniform border gives r-w and a single side curves only partly in.
func TestInnerCorners(t *testing.T) {
	r := Corners{TopLeft: 10, TopRight: 10, BottomRight: 10, BottomLeft: 10}
	if got := innerCorners(r, All(2)); got != (Corners{8, 8, 8, 8}) {
		t.Errorf("uniform 2: %+v, want 8 on every corner", got)
	}
	// Only the left side: the left corners lose half of 3, the right ones none.
	want := Corners{TopLeft: 8.5, TopRight: 10, BottomRight: 10, BottomLeft: 8.5}
	if got := innerCorners(r, Edges{Left: 3}); got != want {
		t.Errorf("left 3: %+v, want %+v", got, want)
	}
	if got := innerCorners(Corners{TopLeft: 1}, All(4)); got.TopLeft != 0 {
		t.Errorf("radius below the border width: %v, want 0", got.TopLeft)
	}
}

// A rounded container that clips emits a rounded clip inset by its border,
// with the radii reduced where the border is; an unrounded one emits a plain
// clip.
func TestRoundedContainerEmitsRoundedClip(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("round").Size(Px(100), Px(80)).Radius(12).BorderEdges(Edges{Top: 2}, DarkTheme().Accent).ScrollY().
				Child(Div().Height(Px(300)))).
			Child(Div().Key("plain").Size(Px(100), Px(80)).ScrollY().Child(Div().Height(Px(300))))
	}, 800, 600)
	var rounded, plain []scene.Op
	for _, op := range r.rt.scene.Ops {
		if op.Kind != scene.OpPushClip {
			continue
		}
		if op.Radii == (scene.Corners{}) {
			plain = append(plain, op)
		} else {
			rounded = append(rounded, op)
		}
	}
	if len(rounded) != 1 || len(plain) != 1 {
		t.Fatalf("clips: %d rounded, %d plain; want one of each", len(rounded), len(plain))
	}
	box := r.find(t, "round")
	want := scene.Corners{TL: 11, TR: 11, BR: 12, BL: 12}
	if rounded[0].Radii != want || rounded[0].Clip != (scene.Rect{X: box.X, Y: box.Y + 2, W: box.W, H: box.H - 2}) {
		t.Errorf("rounded clip %+v radii %+v; want the padding box %v and radii %+v", rounded[0].Clip, rounded[0].Radii, box, want)
	}
}
