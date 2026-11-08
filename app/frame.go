package app

import (
	"image"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
)

// framedRenderer draws the frame of a window that leaves its frame to the app
// (GNOME; see platform.ClientFrame) around the scenes it is given: a shadow in
// the transparent margin, the content clipped to its rounded corners, and a
// thin outline. The scenes are the window's content, at its Size, so the ui
// runtime never sees the frame; they are moved into place as they are drawn.
type framedRenderer struct {
	render.Renderer
	win  platform.FramedWindow
	dark func() bool

	frame         platform.ClientFrame // of the frame being drawn
	framed        bool
	width, height int // the content's size
	dx, dy        float32

	quads  []scene.Quad
	glyphs []scene.Glyph
	images []scene.ImageDraw
}

// Shadow and outline of a framed window, after GNOME's (in logical pixels).
var (
	frameShadowActive   = frameShadow{color: scene.Color{A: 0.35}, blur: 8, dy: 3}
	frameShadowInactive = frameShadow{color: scene.Color{A: 0.18}, blur: 5, dy: 2}
	frameOutlineDark    = scene.Color{R: 1, G: 1, B: 1, A: 0.07}
	frameOutlineLight   = scene.Color{A: 0.14}
)

type frameShadow struct {
	color    scene.Color
	blur, dy float32
}

func newFramedRenderer(r render.Renderer, w platform.FramedWindow, dark func() bool) *framedRenderer {
	return &framedRenderer{Renderer: r, win: w, dark: dark}
}

// BeginFrame implements render.Renderer. The frame is drawn larger than the
// content by the window's margin, and starts with the shadow and the
// content's background, clipped to its corners.
func (f *framedRenderer) BeginFrame(width, height int, clear scene.Color) error {
	f.frame = f.win.ClientFrame()
	f.framed = f.frame.Framed()
	f.width, f.height = width, height
	if !f.framed {
		f.dx, f.dy = 0, 0
		return f.Renderer.BeginFrame(width, height, clear)
	}
	fr := f.frame
	if err := f.Renderer.BeginFrame(width+fr.Left+fr.Right, height+fr.Top+fr.Bottom, scene.Color{}); err != nil {
		return err
	}
	f.dx, f.dy = float32(fr.Left), float32(fr.Top)
	content := f.content()
	radii := scene.Corners{TL: fr.Radii[0], TR: fr.Radii[1], BR: fr.Radii[2], BL: fr.Radii[3]}
	if fr.Left|fr.Top|fr.Right|fr.Bottom != 0 {
		sh := frameShadowInactive
		if fr.Active {
			sh = frameShadowActive
		}
		s := fr.Scale
		bounds := content
		bounds.Y += sh.dy * s
		f.Renderer.DrawQuads([]scene.Quad{{Bounds: bounds, Background: sh.color, Radii: radii, Blur: sh.blur * s}})
	}
	f.Renderer.PushRoundedClip(content, radii)
	f.Renderer.DrawQuads([]scene.Quad{{Bounds: content, Background: clear, Radii: radii}})
	return nil
}

// content is where the content lies in the frame.
func (f *framedRenderer) content() scene.Rect {
	return scene.Rect{X: f.dx, Y: f.dy, W: float32(f.width), H: float32(f.height)}
}

// EndFrame implements render.Renderer: it draws the outline over the content.
func (f *framedRenderer) EndFrame() error {
	if f.framed {
		fr := f.frame
		f.Renderer.PopClip()
		outline := frameOutlineLight
		if f.dark() {
			outline = frameOutlineDark
		}
		radii := scene.Corners{TL: fr.Radii[0], TR: fr.Radii[1], BR: fr.Radii[2], BL: fr.Radii[3]}
		f.Renderer.DrawQuads([]scene.Quad{{Bounds: f.content(), BorderColor: outline, BorderWidth: max(1, fr.Scale), Radii: radii}})
	}
	return f.Renderer.EndFrame()
}

func (f *framedRenderer) move(r scene.Rect) scene.Rect {
	r.X += f.dx
	r.Y += f.dy
	return r
}

// DrawQuads implements render.Renderer.
func (f *framedRenderer) DrawQuads(qs []scene.Quad) {
	if !f.framed {
		f.Renderer.DrawQuads(qs)
		return
	}
	f.quads = append(f.quads[:0], qs...)
	for i := range f.quads {
		f.quads[i].Bounds = f.move(f.quads[i].Bounds)
	}
	f.Renderer.DrawQuads(f.quads)
}

// DrawGlyphs implements render.Renderer.
func (f *framedRenderer) DrawGlyphs(run scene.GlyphRun) {
	if !f.framed {
		f.Renderer.DrawGlyphs(run)
		return
	}
	f.glyphs = append(f.glyphs[:0], run.Glyphs...)
	for i := range f.glyphs {
		f.glyphs[i].Bounds = f.move(f.glyphs[i].Bounds)
	}
	run.Glyphs = f.glyphs
	f.Renderer.DrawGlyphs(run)
}

// DrawImages implements render.Renderer.
func (f *framedRenderer) DrawImages(ims []scene.ImageDraw) {
	if !f.framed {
		f.Renderer.DrawImages(ims)
		return
	}
	f.images = append(f.images[:0], ims...)
	for i := range f.images {
		f.images[i].Bounds = f.move(f.images[i].Bounds)
	}
	f.Renderer.DrawImages(f.images)
	clear(f.images) // drop the textures they point to
}

// PushClip implements render.Renderer.
func (f *framedRenderer) PushClip(r scene.Rect) { f.Renderer.PushClip(f.move(r)) }

// PushRoundedClip implements render.Renderer. Only the innermost rounded clip
// keeps its corners (see render.Renderer), so content in a rounded clip of its
// own at a window corner is cut square there rather than to the window's
// corner.
func (f *framedRenderer) PushRoundedClip(r scene.Rect, radii scene.Corners) {
	f.Renderer.PushRoundedClip(f.move(r), radii)
}

// CaptureNextFrame implements render.Renderer. The capture is the content,
// as on a window the system frames, without the margin.
func (f *framedRenderer) CaptureNextFrame(fn func(*image.RGBA, error)) {
	f.Renderer.CaptureNextFrame(func(img *image.RGBA, err error) {
		if err != nil || img == nil || !f.framed {
			fn(img, err)
			return
		}
		x, y := int(f.dx), int(f.dy)
		sub := img.SubImage(image.Rect(x, y, x+f.width, y+f.height)).(*image.RGBA)
		out := image.NewRGBA(image.Rect(0, 0, f.width, f.height))
		for row := range f.height {
			copy(out.Pix[row*out.Stride:row*out.Stride+f.width*4], sub.Pix[row*sub.Stride:])
		}
		fn(out, nil)
	})
}

// Recreate passes the device-loss rebuild on to the renderer (see rebuildGPU).
func (f *framedRenderer) Recreate() error {
	if rb, ok := f.Renderer.(interface{ Recreate() error }); ok {
		return rb.Recreate()
	}
	return nil
}

// Describe passes the renderer's description on, for diagnostics.
func (f *framedRenderer) Describe() string {
	if d, ok := f.Renderer.(interface{ Describe() string }); ok {
		return d.Describe() + ", client-side frame"
	}
	return "client-side frame"
}
