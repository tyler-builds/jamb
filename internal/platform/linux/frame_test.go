//go:build linux

package linux

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
)

// framedWindow is a window that draws its own frame (as on GNOME), 800×600
// logical pixels at scale s, with no connection.
func framedWindow(s float64) *window {
	w := &window{logW: 800, logH: 600, intScale: 1, opts: platform.WindowOptions{CustomTitleBar: true}}
	if s != 1 {
		w.scale120 = uint32(s * 120)
	}
	return w
}

func TestFrameFloating(t *testing.T) {
	w := framedWindow(1)
	f := w.frame()
	if f.l != shadowMargin || f.t != shadowMargin || f.r != shadowMargin || f.b != shadowMargin {
		t.Errorf("margins = %d %d %d %d; want %d on every edge", f.l, f.t, f.r, f.b, shadowMargin)
	}
	if f.radii != [4]float32{cornerRadius, cornerRadius, cornerRadius, cornerRadius} {
		t.Errorf("radii = %v", f.radii)
	}
	if sw, sh := f.surfaceSize(); sw != 800+2*shadowMargin || sh != 600+2*shadowMargin {
		t.Errorf("surface = %dx%d", sw, sh)
	}
	if cw, ch := w.Size(); cw != 800 || ch != 600 {
		t.Errorf("Size = %dx%d; want the content's 800x600", cw, ch)
	}
}

func TestFrameMaximizedAndServerSide(t *testing.T) {
	w := framedWindow(1)
	w.maximized = true
	if f := w.frame(); f != (frameState{w: 800, h: 600}) {
		t.Errorf("maximized: %+v; want no margin and square corners", f)
	}
	w = framedWindow(1)
	w.serverFrame = true // KDE draws the frame
	if f := w.frame(); f != (frameState{w: 800, h: 600}) {
		t.Errorf("server-side frame: %+v; want none of the app's", f)
	}
	if w.ClientFrame().Framed() {
		t.Error("a window the compositor frames reports a client frame")
	}
}

// A window snapped against the left edge has no margin there, and square
// corners on that side.
func TestFrameTiled(t *testing.T) {
	w := framedWindow(1)
	w.tiled = tiling{left: true, top: true, bottom: true}
	f := w.frame()
	if f.l != 0 || f.t != 0 || f.b != 0 || f.r != shadowMargin {
		t.Errorf("margins = %d %d %d %d", f.l, f.t, f.r, f.b)
	}
	if f.radii != [4]float32{} {
		t.Errorf("radii = %v; want square corners on a window tiled on three sides", f.radii)
	}
	w.tiled = tiling{left: true}
	if f := w.frame(); f.radii != [4]float32{0, cornerRadius, cornerRadius, 0} {
		t.Errorf("tiled left: radii = %v; want the right corners round", f.radii)
	}
}

func TestFrameCorners(t *testing.T) {
	for _, c := range []struct {
		corners platform.Corners
		want    float32
	}{{platform.CornersDefault, cornerRadius}, {platform.CornersRound, cornerRadius}, {platform.CornersRoundSmall, cornerRadiusSmall}, {platform.CornersSquare, 0}} {
		w := framedWindow(1)
		w.opts.Corners = c.corners
		if got := w.frame().radii[0]; got != c.want {
			t.Errorf("Corners %d: radius %v; want %v", c.corners, got, c.want)
		}
	}
}

// At a fractional scale the buffer is the surface's size scaled, and the
// content and margins add up to it.
func TestClientFrameFractional(t *testing.T) {
	w := framedWindow(1.25)
	w.logW = 801
	cf := w.ClientFrame()
	cw, ch := w.Size()
	sw, sh := w.frame().surfaceSize()
	if cf.Left+cw+cf.Right != w.physical(sw) || cf.Top+ch+cf.Bottom != w.physical(sh) {
		t.Errorf("%d+%d+%d, %d+%d+%d; want the buffer %dx%d", cf.Left, cw, cf.Right, cf.Top, ch, cf.Bottom, w.physical(sw), w.physical(sh))
	}
	if cf.Radii[0] != cornerRadius*1.25 || !cf.Framed() {
		t.Errorf("ClientFrame = %+v", cf)
	}
}
