package ui

import (
	"testing"
)

// TestSkeletonLines checks that Lines(3) draws three bones, and that only the last
// one is narrower than the container.
func TestSkeletonLines(t *testing.T) {
	r := reducedRig(func(ctx *Context) Element { return Skeleton().Lines(3) })
	bones := partNodes(r.rt, "Skeleton.Bone")
	if len(bones) != 3 {
		t.Fatalf("Lines(3) draws %d bones, want 3", len(bones))
	}
	short := 0
	for _, n := range bones {
		switch n.style.Width {
		case Percent(60):
			short++
		case Full:
		default:
			t.Errorf("a bar is %v wide, want full width or 60 percent", n.style.Width)
		}
	}
	if short != 1 {
		t.Errorf("%d bars are 60 percent wide, want the last one only", short)
	}
}

// TestSkeletonBlockAndCircle checks the default bar, a block of text height, and a
// circle, which is round and has the diameter it was given.
func TestSkeletonBlockAndCircle(t *testing.T) {
	block := reducedRig(func(ctx *Context) Element { return Skeleton() })
	n := partNode(t, block.rt, "Skeleton.Bone", "")
	if n.style.Width != Full || n.style.Height != Px(16) {
		t.Errorf("the block is %v by %v, want full width by 16", n.style.Width, n.style.Height)
	}

	circle := reducedRig(func(ctx *Context) Element { return Skeleton().Circle(40) })
	c := partNode(t, circle.rt, "Skeleton.Bone", "")
	if c.style.Width != Px(40) || c.style.Height != Px(40) {
		t.Errorf("the circle is %v by %v, want 40 by 40", c.style.Width, c.style.Height)
	}
	if c.style.Radius != 20 {
		t.Errorf("the circle has radius %v, want 20", c.style.Radius)
	}

	big := newAccessRig(func(ctx *Context) Element { return Skeleton().Circle(40) })
	big.rt.SetSystemSettings(SystemSettings{ReducedMotion: true, TextScale: 150})
	big.frame()
	if w := partNode(t, big.rt, "Skeleton.Bone", "").style.Width; w != Px(60) {
		t.Errorf("the circle is %v wide at 150 percent, want 60", w)
	}
}

// TestSkeletonPulse checks the motion: the skeleton asks for frames while it pulses,
// and asks for none under reduced motion or high contrast.
func TestSkeletonPulse(t *testing.T) {
	root := func(ctx *Context) Element { return Skeleton().Lines(2) }
	r := newAccessRig(root)
	r.frame()
	if !r.rt.NeedsFrame() {
		t.Errorf("the pulse asks for no frames")
	}

	still := reducedRig(root)
	if still.rt.NeedsFrame() {
		t.Errorf("reduced motion: the skeleton asks for frames")
	}

	hc := newAccessRig(root)
	hc.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	hc.frame()
	if hc.rt.NeedsFrame() {
		t.Errorf("high contrast: the skeleton asks for frames")
	}
}

// TestSkeletonHiddenFromAccess checks that the bones are left out of the
// accessibility tree.
func TestSkeletonHiddenFromAccess(t *testing.T) {
	r := reducedRig(func(ctx *Context) Element { return Skeleton().Lines(2) })
	for _, n := range partNodes(r.rt, "Skeleton.Bone") {
		if n.div.acc == nil || !n.div.acc.hidden {
			t.Errorf("a bone is exposed to assistive technology, want it hidden")
		}
	}
}

func testSkeletonParts(t *testing.T, applied map[string]bool) {
	applied["Skeleton.Bone"] = true
	t.Run("Skeleton.Bone", func(t *testing.T) {
		var st SkeletonStyle
		st.Bone = Style{Background: markBG}
		r := reducedRig(func(ctx *Context) Element { return Skeleton().Lines(2).Styles(st) })
		bones := partNodes(r.rt, "Skeleton.Bone")
		if len(bones) != 2 {
			t.Fatalf("Lines(2) draws %d bones, want 2", len(bones))
		}
		for _, n := range bones {
			wantBackground(t, n, "Skeleton.Bone", markBG)
		}
	})
}
