// Package render defines the framework-owned renderer contract.
//
// Concrete GPU backends live in sub-packages (render/d3d11, ...). Only those
// sub-packages know about a GPU API; everything else talks to this interface.
package render

import (
	"errors"
	"image"

	"github.com/tyler-builds/jamb/internal/scene"
)

// ErrDeviceLost is returned, wrapped, by BeginFrame and EndFrame when the GPU
// device was removed or reset (a driver update or crash, a remote desktop
// switch, sleep and resume). Nothing can be drawn until the device and the
// window's surface are recreated; see the backend's Recover and Recreate.
var ErrDeviceLost = errors.New("render: GPU device lost")

// Renderer draws scene primitives into a window surface.
//
// A frame is BeginFrame, any number of Draw*/PushClip/PopClip calls in paint
// order, then EndFrame. Slices passed to Draw* are only read during the call.
type Renderer interface {
	// BeginFrame starts a frame of the given size in physical pixels, cleared
	// to clear. A size change resizes the swap chain.
	BeginFrame(width, height int, clear scene.Color) error
	DrawQuads([]scene.Quad)
	DrawGlyphs(scene.GlyphRun)
	DrawImages([]scene.ImageDraw)
	PushClip(scene.Rect)
	PushRoundedClip(scene.Rect, scene.Corners)
	PopClip()
	// EndFrame submits the frame and presents it, waiting for vsync.
	EndFrame() error
	// CaptureNextFrame arranges for fn to receive a copy of the next frame
	// just before it is presented. It is a debugging aid (screenshots, visual
	// tests) and is slow.
	CaptureNextFrame(fn func(*image.RGBA, error))
	Close()
}

// Draw renders a whole scene as one frame.
func Draw(r Renderer, s *scene.Scene, width, height int) error {
	if err := r.BeginFrame(width, height, s.Clear); err != nil {
		return err
	}
	s.Replay(r)
	return r.EndFrame()
}
