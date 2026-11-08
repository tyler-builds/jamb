//go:build linux

package linux

import (
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
)

// frameState is the frame of a window whose frame is its own, in logical
// pixels: the transparent margin around the content on each edge, the corner
// radii (top left, top right, bottom right, bottom left), and the content's
// size. A window the compositor frames has no margin and square corners.
type frameState struct {
	l, t, r, b int
	radii      [4]float32
	w, h       int
}

// frame returns the window's frame as it is now. The margin holds the shadow,
// and the corners are rounded as GNOME rounds its own windows, except on an
// edge the window is maximized, fullscreen or snapped against: there it meets
// the screen's edge or another window, with no margin and square corners.
func (w *window) frame() frameState {
	f := frameState{w: w.logW, h: w.logH}
	if !w.clientFrame() || w.maximized || w.fullscreen {
		return f
	}
	t := w.tiled
	if !t.left {
		f.l = shadowMargin
	}
	if !t.top {
		f.t = shadowMargin
	}
	if !t.right {
		f.r = shadowMargin
	}
	if !t.bottom {
		f.b = shadowMargin
	}
	var r float32
	switch w.opts.Corners {
	case platform.CornersDefault, platform.CornersRound:
		r = cornerRadius
	case platform.CornersRoundSmall:
		r = cornerRadiusSmall
	}
	round := func(a, b bool) float32 {
		if a || b {
			return 0
		}
		return r
	}
	f.radii = [4]float32{round(t.top, t.left), round(t.top, t.right), round(t.bottom, t.right), round(t.bottom, t.left)}
	return f
}

// surfaceSize returns the size of the surface, content and margins, in
// logical pixels.
func (f frameState) surfaceSize() (int, int) { return f.l + f.w + f.r, f.t + f.h + f.b }

// physicalFrame returns where the content lies in the window's buffer, in
// physical pixels, and the buffer's size. The content's edges are rounded from
// their logical positions, so the buffer is exactly the surface's size scaled.
func (w *window) physicalFrame(f frameState) (x0, y0, x1, y1, bw, bh int) {
	sw, sh := f.surfaceSize()
	return w.physical(f.l), w.physical(f.t), w.physical(f.l + f.w), w.physical(f.t + f.h), w.physical(sw), w.physical(sh)
}

// ClientFrame implements platform.FramedWindow: the frame the host draws
// around the content of the next frame.
func (w *window) ClientFrame() platform.ClientFrame {
	f := w.frame()
	x0, y0, x1, y1, bw, bh := w.physicalFrame(f)
	s := float32(w.scale())
	cf := platform.ClientFrame{Left: x0, Top: y0, Right: bw - x1, Bottom: bh - y1, Active: w.activated, Scale: s}
	for i, r := range f.radii {
		cf.Radii[i] = r * s
	}
	return cf
}

// applyFrame tells the compositor about the frame of the buffer being
// committed, when it changed: which part of the surface is the window
// (xdg_surface.set_window_geometry), so the shadow does not count when the
// window is placed, snapped or resized; which part takes input (the window and
// a resize border around it, so clicks on the rest of the shadow go to what is
// under it); and which part is opaque.
func (w *window) applyFrame(f frameState) {
	if f == w.sentFrame || !w.clientFrame() {
		w.sentFrame = f
		return
	}
	w.sentFrame = f
	w.xdgSurface.Send(3, int32(f.l), int32(f.t), int32(f.w), int32(f.h)) // set_window_geometry

	if f.l|f.t|f.r|f.b == 0 {
		w.surface.Send(5, (*wl.Proxy)(nil)) // set_input_region: all of it
	} else {
		grow := func(margin int) int { return min(margin, outsideResize) }
		x, y := f.l-grow(f.l), f.t-grow(f.t)
		w.setRegion(5, [][4]int{{x, y, f.l + f.w + grow(f.r) - x, f.t + f.h + grow(f.b) - y}}) // set_input_region
	}

	// The content is opaque but for its rounded corners: the rows of the
	// largest radius are left out, except between the corners.
	r := int(max(f.radii[0], f.radii[1], f.radii[2], f.radii[3]) + 0.999)
	rects := [][4]int{{f.l, f.t + r, f.w, f.h - 2*r}}
	if r > 0 {
		rects = append(rects, [4]int{f.l + r, f.t, f.w - 2*r, r}, [4]int{f.l + r, f.t + f.h - r, f.w - 2*r, r})
	}
	w.setRegion(4, rects) // set_opaque_region
}

// setRegion sends a wl_surface request (op) that takes a region, made of the
// rectangles (x, y, width, height) in surface coordinates.
func (w *window) setRegion(op uint16, rects [][4]int) {
	c := w.d.conn
	region := c.NewProxy("wl_region", 1, nil)
	w.d.compositor.Send(1, wl.NewID{P: region}) // create_region
	for _, rc := range rects {
		if rc[2] > 0 && rc[3] > 0 {
			region.Send(1, int32(rc[0]), int32(rc[1]), int32(rc[2]), int32(rc[3])) // add
		}
	}
	w.surface.Send(op, region)
	region.Send(0) // destroy: the surface keeps the region's state
	region.Destroyed()
}
