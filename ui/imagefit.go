package ui

import (
	"image"
	"math"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/tyler-builds/jamb/internal/scene"
)

// A still image drawn smaller than its pixels is resampled once to the device
// pixels it covers, with a Catmull-Rom filter, and drawn from that copy pixel for
// pixel. The renderers sample images bilinearly, which takes four texels per
// pixel: shrunk further than about 1.5×, an image loses detail and its edges
// stair-step (a 128 px app icon drawn at 36 px reads 4 of the ~13 texels each
// pixel covers). The copy is also cheaper to draw.

// minResample is how much smaller than its pixels an image must be drawn to be
// resampled. Closer to its size, bilinear sampling is as good.
const minResample = 1.1

// imageSettle is how long an image's drawn size must stay the same before it
// is resampled again: while a window is resized or an image animates its size,
// it is drawn from its full pixels, and resampled once it settles.
const imageSettle = 150 * time.Millisecond

// maxFitted is how many sizes of one image are kept, most recently drawn first:
// enough for an app icon shown in a list, a header and a dialog at once.
const maxFitted = 4

// asyncPixels is the size, in source pixels, above which an image is resampled
// on another goroutine rather than in the paint: a screenshot takes tens of
// milliseconds, an app icon a fraction of one. The image is drawn from its full
// pixels until its copy is ready.
const asyncPixels = 256 * 256

// fittedTexture is an image resampled to the device pixels it is drawn at. tex
// is nil while it is resampled on another goroutine.
type fittedTexture struct {
	src  image.Rectangle // the part of the image, in its pixels
	w, h int
	tex  *scene.Texture
}

// fitted returns the image's src part resampled to w×h device pixels, and
// whether it has such a copy or one on the way.
func (s *ImageSource) fitted(src image.Rectangle, w, h int) (*scene.Texture, bool) {
	s.fitMu.Lock()
	defer s.fitMu.Unlock()
	for i, f := range s.fits {
		if f.src == src && f.w == w && f.h == h {
			copy(s.fits[1:i+1], s.fits[:i]) // most recently drawn first
			s.fits[0] = f
			return f.tex, true
		}
	}
	return nil, false
}

// fit resamples the image's src part to w×h and keeps the copy. A large part is
// resampled on another goroutine, which then posts a repaint to rt; fit returns
// nil until it is done.
func (s *ImageSource) fit(rt *Runtime, src image.Rectangle, w, h int) *scene.Texture {
	f := fittedTexture{src: src, w: w, h: h}
	async := src.Dx()*src.Dy() > asyncPixels
	if !async {
		f.tex = resample(s.tex, src, w, h)
	}
	s.fitMu.Lock()
	if len(s.fits) < maxFitted {
		s.fits = append(s.fits, fittedTexture{})
	}
	copy(s.fits[1:], s.fits)
	s.fits[0] = f
	s.fitMu.Unlock()
	if async {
		go func() {
			tex := resample(s.tex, src, w, h)
			s.fitMu.Lock()
			for i := range s.fits {
				if f := &s.fits[i]; f.src == src && f.w == w && f.h == h && f.tex == nil {
					f.tex = tex
				}
			}
			s.fitMu.Unlock()
			rt.Post(func() { rt.needPaint = true })
		}()
	}
	return f.tex
}

// resample scales the src part of the premultiplied RGBA texture t to w×h. A
// large part is first halved with a box filter while it stays at least twice the
// size asked for, which is cheap and needs far less scratch memory; a small one,
// such as an icon, is filtered straight to its size, which is a little sharper.
func resample(t *scene.Texture, src image.Rectangle, w, h int) *scene.Texture {
	from := (&image.RGBA{Pix: t.Pix, Stride: t.Width * 4, Rect: image.Rect(0, 0, t.Width, t.Height)}).SubImage(src).(*image.RGBA)
	for from.Rect.Dx()*from.Rect.Dy() > asyncPixels && from.Rect.Dx() >= 2*w && from.Rect.Dy() >= 2*h {
		from = halve(from)
	}
	out := scene.NewTexture(w, h, scene.FormatRGBA8)
	to := &image.RGBA{Pix: out.Pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	// Catmull-Rom keeps edges crisp without ringing much; on image.RGBA it keeps each
	// color at or under its alpha, so the copy stays validly premultiplied.
	xdraw.CatmullRom.Scale(to, to.Rect, from, from.Rect, xdraw.Src, nil)
	return out
}

// halve returns m at half its size, each pixel the average of the 2×2 it covers.
// Averaging premultiplied pixels is exact. An odd last row or column is dropped.
func halve(m *image.RGBA) *image.RGBA {
	w, h := m.Rect.Dx()/2, m.Rect.Dy()/2
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		r0 := m.Pix[m.PixOffset(m.Rect.Min.X, m.Rect.Min.Y+2*y):]
		r1 := m.Pix[m.PixOffset(m.Rect.Min.X, m.Rect.Min.Y+2*y+1):]
		o := out.Pix[y*out.Stride:]
		for x := range w {
			for k := range 4 {
				i := 8*x + k
				o[4*x+k] = uint8((uint16(r0[i]) + uint16(r0[i+4]) + uint16(r1[i]) + uint16(r1[i+4]) + 2) / 4)
			}
		}
	}
	return out
}

// fitImage returns the texture, source rectangle and device bounds to draw the
// still image of n with: a copy resampled to bounds when the image is drawn
// smaller than its pixels, or tex, src and bounds as they are. bounds are whole
// device pixels (see px).
func (rt *Runtime) fitImage(n *node, tex *scene.Texture, src, bounds scene.Rect) (*scene.Texture, scene.Rect) {
	is := n.img.src
	if tex != is.tex || tex.Format != scene.FormatRGBA8 || bounds.W < 1 || bounds.H < 1 {
		return tex, src // an animation frame, or nothing to draw
	}
	if src.W < bounds.W*minResample && src.H < bounds.H*minResample {
		return tex, src
	}
	w, h := int(bounds.W), int(bounds.H)
	part := image.Rect(int(math.Round(float64(src.X))), int(math.Round(float64(src.Y))),
		int(math.Round(float64(src.X+src.W))), int(math.Round(float64(src.Y+src.H)))).
		Intersect(image.Rect(0, 0, tex.Width, tex.Height))
	if part.Empty() {
		return tex, src
	}
	full := func(t *scene.Texture) scene.Rect { return scene.Rect{W: float32(t.Width), H: float32(t.Height)} }
	if t, ok := is.fitted(part, w, h); ok {
		if t == nil {
			return tex, src // being resampled
		}
		return t, full(t)
	}
	// An image drawn for the first time is resampled at once; one whose size has just
	// changed waits until the size settles.
	st := rt.state(n.id)
	size := [2]int{w, h}
	if st.fitSize != size {
		first := st.fitSize == [2]int{}
		st.fitSize, st.fitSizeAt = size, rt.lastFrame
		if !first {
			rt.noteDue(rt.lastFrame.Add(imageSettle))
			return tex, src
		}
	} else if rt.lastFrame.Sub(st.fitSizeAt) < imageSettle {
		rt.noteDue(st.fitSizeAt.Add(imageSettle))
		return tex, src
	}
	if t := is.fit(rt, part, w, h); t != nil {
		return t, full(t)
	}
	return tex, src
}
