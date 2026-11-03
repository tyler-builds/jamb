// Package soft is a CPU renderer: it draws scene primitives into memory in
// pure Go, with no GPU and no system library.
//
// It follows the Direct3D 11 renderer's shaders (see render/d3d11): the same
// signed-distance coverage for rounded rectangles, borders and clips, the same
// blurred-shadow and gradient formulas, the same glyph contrast correction, and
// premultiplied blending in sRGB space into an 8-bit target. A scene therefore
// looks the same drawn by either renderer, to within rounding.
//
// It is the renderer of platforms that have no GPU backend yet, and it draws
// the frames of headless screenshot tests on every platform.
package soft

import (
	"errors"
	"image"
	"runtime"
	"sync"

	"github.com/tyler-builds/jamb/internal/scene"
)

// A Target shows the frames a Renderer draws, for example a window's shared
// memory buffers. Without one, a Renderer draws into memory of its own.
type Target interface {
	// Buffer returns the pixels to draw the next width×height frame into, as
	// premultiplied BGRA (Wayland's and DRM's ARGB8888 on a little-endian
	// machine), stride bytes per row. The contents need not be kept from the
	// previous frame: each frame is drawn in full.
	Buffer(width, height int) (pix []byte, stride int, err error)
	// Present shows the frame drawn into the latest buffer.
	Present() error
}

// Renderer implements render.Renderer on the CPU.
type Renderer struct {
	target Target
	own    []byte // the pixels when there is no target

	pix           []byte // the frame being drawn: premultiplied BGRA
	stride        int
	width, height int
	clips         []clipState
	capture       func(*image.RGBA, error)
	inFrame       bool

	// The frame's draws, recorded in paint order and rasterized by EndFrame,
	// in horizontal bands on every core. The slices passed to Draw* are copied,
	// since they are only valid during the call.
	cmds   []command
	quads  []scene.Quad
	glyphs []scene.Glyph
	images []scene.ImageDraw
}

type cmdKind uint8

const (
	cmdQuads cmdKind = iota
	cmdGlyphs
	cmdImages
)

// command is one Draw* call: its primitives (a range of the frame's quads,
// glyphs or images), the mask texture of glyphs, and the clip it was drawn in.
type command struct {
	kind       cmdKind
	start, end int
	tex        *scene.Texture
	clip       clipper
}

func (r *Renderer) record(c command) {
	c.clip = r.clipper()
	r.cmds = append(r.cmds, c)
}

// band is the rows [y0, y1) of the frame, which one goroutine rasterizes.
type band struct {
	pix           []byte
	stride, width int
	y0, y1        int
}

// rows returns the rows of the band whose centers lie in [lo, hi).
func (b *band) rows(lo, hi float32) (int, int) {
	a, z := pixelSpan(lo, hi, b.y1)
	return max(a, b.y0), z
}

// minBandRows keeps bands tall enough that splitting the frame pays for the
// work each band repeats (walking every command).
const minBandRows = 48

// rasterize draws the recorded commands.
func (r *Renderer) rasterize() {
	n := min(runtime.GOMAXPROCS(0), max(r.height/minBandRows, 1))
	if n <= 1 {
		r.draw(&band{pix: r.pix, stride: r.stride, width: r.width, y0: 0, y1: r.height})
		return
	}
	rowsPer := (r.height + n - 1) / n
	var wg sync.WaitGroup
	for y := 0; y < r.height; y += rowsPer {
		b := &band{pix: r.pix, stride: r.stride, width: r.width, y0: y, y1: min(y+rowsPer, r.height)}
		wg.Go(func() { r.draw(b) })
	}
	wg.Wait()
}

// draw replays the commands into one band.
func (r *Renderer) draw(b *band) {
	y0, y1 := float32(b.y0), float32(b.y1)
	for i := range r.cmds {
		c := &r.cmds[i]
		if c.clip.y1 < y0-1 || c.clip.y0 > y1+1 {
			continue // clipped away from this band
		}
		switch c.kind {
		case cmdQuads:
			for k := c.start; k < c.end; k++ {
				b.drawQuad(&r.quads[k], &c.clip)
			}
		case cmdGlyphs:
			b.drawGlyphs(c.tex, r.glyphs[c.start:c.end], &c.clip)
		case cmdImages:
			for k := c.start; k < c.end; k++ {
				b.drawImage(&r.images[k], &c.clip)
			}
		}
	}
}

// clipState is one level of the clip stack, as in the Direct3D renderer. rect
// is the intersection of every clip so far; shape and radii are the innermost
// rounded clip (zero radii when there is none), which is not intersected, so an
// outer clip that cuts it does not round the cut.
type clipState struct {
	rect  scene.Rect
	shape scene.Rect
	radii scene.Corners
}

// New returns a renderer that draws into t's buffers, or into memory of its
// own when t is nil (see Image).
func New(t Target) *Renderer { return &Renderer{target: t} }

// BeginFrame implements render.Renderer.
func (r *Renderer) BeginFrame(width, height int, clear scene.Color) error {
	if width <= 0 || height <= 0 {
		return errors.New("soft: BeginFrame called with an empty size; skip rendering while minimized")
	}
	if r.target != nil {
		pix, stride, err := r.target.Buffer(width, height)
		if err != nil {
			return err
		}
		if stride < width*4 || len(pix) < stride*(height-1)+width*4 {
			return errors.New("soft: the target's buffer is smaller than the frame")
		}
		r.pix, r.stride = pix, stride
	} else {
		if n := width * height * 4; cap(r.own) < n {
			r.own = make([]byte, n)
		} else {
			r.own = r.own[:n]
		}
		r.pix, r.stride = r.own, width*4
	}
	r.width, r.height = width, height
	r.clips = append(r.clips[:0], clipState{rect: scene.Rect{W: float32(width), H: float32(height)}})
	r.cmds, r.quads, r.glyphs, r.images = r.cmds[:0], r.quads[:0], r.glyphs[:0], r.images[:0]
	r.inFrame = true

	// The clear color, premultiplied, as ClearRenderTargetView writes it.
	a := sat(clear.A)
	px := [4]byte{to8(clear.B * a), to8(clear.G * a), to8(clear.R * a), to8(a)}
	row := r.pix[:width*4]
	for i := 0; i < len(row); i += 4 {
		copy(row[i:i+4], px[:])
	}
	for y := 1; y < height; y++ {
		copy(r.pix[y*r.stride:y*r.stride+width*4], row)
	}
	return nil
}

// PushClip implements render.Renderer.
func (r *Renderer) PushClip(c scene.Rect) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: top.shape, radii: top.radii})
}

// PushRoundedClip implements render.Renderer.
func (r *Renderer) PushRoundedClip(c scene.Rect, radii scene.Corners) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: c, radii: radii})
}

// PopClip implements render.Renderer.
func (r *Renderer) PopClip() {
	if len(r.clips) > 1 {
		r.clips = r.clips[:len(r.clips)-1]
	}
}

// CaptureNextFrame implements render.Renderer.
func (r *Renderer) CaptureNextFrame(fn func(*image.RGBA, error)) { r.capture = fn }

// EndFrame implements render.Renderer. It rasterizes the frame and hands it
// to the target; a window's target paces frames to its display.
func (r *Renderer) EndFrame() error {
	if !r.inFrame {
		return errors.New("soft: EndFrame without BeginFrame")
	}
	r.inFrame = false
	r.rasterize()
	clear(r.images) // drop the textures they point to
	if fn := r.capture; fn != nil {
		r.capture = nil
		fn(r.snapshot(true), nil)
	}
	if r.target != nil {
		return r.target.Present()
	}
	return nil
}

// Image returns a copy of the latest frame, premultiplied as image.RGBA is,
// with the alpha the frame has (opaque where an opaque clear color or fill
// covers it). It returns nil before the first frame.
func (r *Renderer) Image() *image.RGBA {
	if r.pix == nil {
		return nil
	}
	return r.snapshot(false)
}

// snapshot converts the frame to RGBA. opaque sets every alpha to 255, as the
// Direct3D renderer's capture does, since a window's surface is opaque.
func (r *Renderer) snapshot(opaque bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	for y := 0; y < r.height; y++ {
		src := r.pix[y*r.stride : y*r.stride+r.width*4]
		dst := img.Pix[y*img.Stride:]
		for x := 0; x < len(src); x += 4 {
			b, g, rr, a := src[x], src[x+1], src[x+2], src[x+3]
			if opaque {
				dst[x], dst[x+1], dst[x+2], dst[x+3] = rr, g, b, 255
				continue
			}
			// image.RGBA is premultiplied too, so the bytes carry over.
			dst[x], dst[x+1], dst[x+2], dst[x+3] = rr, g, b, a
		}
	}
	return img
}

// Close implements render.Renderer.
func (r *Renderer) Close() {
	r.own, r.pix = nil, nil
	r.cmds, r.quads, r.glyphs, r.images = nil, nil, nil, nil
}
