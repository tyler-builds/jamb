package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
)

// Dump writes the current element tree with laid-out positions (logical
// pixels, window coordinates). maxDepth limits the depth; 0 means no limit.
// It is meant for debugging layouts, by humans or coding agents:
//
//	rt.Frame(time.Now())
//	rt.Dump(os.Stdout, 6)
func (rt *Runtime) Dump(w io.Writer, maxDepth int) {
	if rt.tree != nil {
		rt.dump(w, rt.tree, 0, maxDepth)
	}
}

func (rt *Runtime) dump(w io.Writer, n *node, depth, maxDepth int) {
	if maxDepth > 0 && depth >= maxDepth {
		return
	}
	label := "div"
	switch n.kind {
	case kindText:
		t := n.textEl.text
		if len(t) > 32 {
			t = t[:32] + "…"
		}
		label = fmt.Sprintf("text %q", t)
	case kindIcon:
		label = "icon " + n.icon.icon.Name
	case kindImage:
		label = "image"
	case kindEditor:
		v := n.editor.value.Peek()
		if len(v) > 32 {
			v = v[:32] + "…"
		}
		label = fmt.Sprintf("editor value=%q placeholder=%q", v, n.editor.placeholder)
	default:
		if n.style.Direction == Row {
			label += " row"
		}
	}
	if n.key != "" {
		label += " key=" + n.key
	}
	extra := ""
	if n.lay.ContentW > n.lay.W+0.5 || n.lay.ContentH > n.lay.H+0.5 {
		extra = fmt.Sprintf(" content=%.0fx%.0f", n.lay.ContentW, n.lay.ContentH)
	}
	r := rt.nodeRect(n)
	fmt.Fprintf(w, "%s%s  [%.0f,%.0f %.0fx%.0f]%s\n", strings.Repeat("  ", depth), label,
		r.X, r.Y, r.W, r.H, extra)
	for _, c := range n.children {
		rt.dump(w, c, depth+1, maxDepth)
	}
}

// ContentWidth returns how far right the laid-out content reaches, in
// logical pixels, ignoring content inside clipping or scrolling containers.
// A value above the window width means the UI overflows horizontally — a
// handy assertion for layout tests at a minimum window size.
func (rt *Runtime) ContentWidth() float32 {
	var right float32
	var walk func(n *node)
	walk = func(n *node) {
		r := rt.nodeRect(n)
		right = max(right, r.X+r.W)
		if n.style.OverflowX != OverflowVisible {
			return
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	if rt.tree != nil {
		walk(rt.tree)
	}
	return right
}

// ---- driving the UI from tests ----
//
// These helpers make headless tests read like a script:
//
//	rt.Press(input.KeyN, input.ModCtrl)
//	rt.Type("Trip to Lisbon")
//	rt.ClickKey("save")
//	r, ok := rt.Find("save")
//
// Each action runs a frame afterwards, so its effects are visible at once.

// Find returns the laid-out rectangle (logical pixels, window coordinates)
// of the first element in tree order whose key is key. Keys of components
// and composite elements (TextInput, Button, ...) belong to the element
// they render. For elements that are not on screen, the rectangle comes from
// layout: their position plus the scroll offsets of their scrolling ancestors.
func (rt *Runtime) Find(key string) (Rect, bool) {
	if n := rt.findKey(rt.tree, key); n != nil {
		return rt.nodeRect(n), true
	}
	return scene.Rect{}, false
}

// nodeRect returns n's rectangle in logical window coordinates. A node painted
// by the latest paint reports where it was drawn, transitions and motions
// included. Any other node, one culled as off screen, reports the rectangle
// its layout gives it, with its static offsets and the current scroll offsets
// of its ancestors. Overlays are placed by paint, so an overlay that was not
// painted reports the rectangle it was last painted at.
func (rt *Runtime) nodeRect(n *node) scene.Rect {
	if n.paintGen == rt.paintGen {
		return n.abs
	}
	var x, y float32
	for a := n; a != nil; a = a.parent {
		if a.flags&flagOverlay != 0 {
			return n.abs
		}
		x += a.lay.X + a.style.OffsetX
		y += a.lay.Y + a.style.OffsetY
		if a != n && a.st != nil && (a.style.OverflowX == OverflowScroll || a.style.OverflowY == OverflowScroll) {
			x -= a.st.scrollX
			y -= a.st.scrollY
		}
	}
	return scene.Rect{X: x, Y: y, W: n.lay.W, H: n.lay.H}
}

func (rt *Runtime) findKey(n *node, key string) *node {
	if n == nil {
		return nil
	}
	if n.key == key {
		return n
	}
	kids := n.children
	if n.reused {
		// Rebuilt in place by the build under way: until the build ends, the tree as
		// last laid out is the one its old children make.
		kids = n.oldKids
	}
	for _, c := range kids {
		if f := rt.findKey(c, key); f != nil {
			return f
		}
	}
	return nil
}

// ClickKey clicks the center of the element with the given key.
func (rt *Runtime) ClickKey(key string) error {
	r, ok := rt.Find(key)
	if !ok {
		return fmt.Errorf("ui: no element with key %q is on screen; check the key, or call rt.Dump(os.Stdout, 0) to see the tree", key)
	}
	// Pointer methods take device-logical coordinates; Find's are zoomed.
	x, y := (r.X+r.W/2)*rt.zoom, (r.Y+r.H/2)*rt.zoom
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	rt.Frame(rt.now())
	return nil
}

// Type types text into the focused element, character by character.
func (rt *Runtime) Type(text string) {
	for _, r := range text {
		rt.TextInput(string(r))
	}
	rt.Frame(rt.now())
}

// Press presses and releases a key with modifiers.
func (rt *Runtime) Press(k input.Key, mods input.Modifiers) {
	rt.KeyDown(k, mods, false)
	rt.KeyUp(k, mods)
	rt.Frame(rt.now())
}
