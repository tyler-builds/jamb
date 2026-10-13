package text

import (
	"image"

	"github.com/tyler-builds/jamb/internal/scene"
)

const (
	// Atlases start small and double as glyphs arrive: a typical window
	// needs a few hundred masks, and the subpixel atlas costs 4 bytes per
	// texel on both the CPU and the GPU.
	atlasInitial = 256
	atlasMax     = 4096
	atlasPad     = 1
)

// Atlas packs coverage masks (glyphs, icons) into one texture using simple
// shelf packing. The texture is Alpha8 for grayscale masks or Subpixel8 for
// ClearType glyphs. When full it doubles in size; at the maximum size it is
// cleared and the owner re-rasterizes on demand.
type Atlas struct {
	Texture *scene.Texture
	shelves []shelf
	nextY   int
	// Generation increments whenever the atlas is cleared, invalidating all
	// previously returned rectangles.
	Generation int
}

type shelf struct{ y, h, x int }

// NewAtlas creates an empty Alpha8 atlas.
func NewAtlas() *Atlas {
	return &Atlas{Texture: scene.NewTexture(atlasInitial, atlasInitial, scene.FormatAlpha8)}
}

// newSubpixelAtlas creates an empty atlas for ClearType glyph masks.
func newSubpixelAtlas(p scene.TextParams) *Atlas {
	t := scene.NewTexture(atlasInitial, atlasInitial, scene.FormatSubpixel8)
	t.Text = p
	return &Atlas{Texture: t}
}

// Alloc reserves a w×h region and returns its top-left corner. ok is false
// when the atlas had to be cleared; callers must then drop cached positions.
func (a *Atlas) Alloc(w, h int) (x, y int, cleared bool) {
	pw, ph := w+atlasPad, h+atlasPad
	for {
		best := -1
		for i := range a.shelves {
			s := &a.shelves[i]
			if s.h >= ph && s.h <= ph*3/2+2 && s.x+pw <= a.Texture.Width {
				if best < 0 || s.h < a.shelves[best].h {
					best = i
				}
			}
		}
		if best >= 0 {
			s := &a.shelves[best]
			x, y = s.x, s.y
			s.x += pw
			return x, y, cleared
		}
		if a.nextY+ph <= a.Texture.Height && pw <= a.Texture.Width {
			a.shelves = append(a.shelves, shelf{y: a.nextY, h: ph})
			a.nextY += ph
			continue
		}
		if a.Texture.Width < atlasMax {
			a.grow()
			continue
		}
		a.clear()
		cleared = true
		if pw > a.Texture.Width || ph > a.Texture.Height {
			return 0, 0, cleared // too large to ever fit; caller draws nothing
		}
	}
}

func (a *Atlas) grow() {
	old := a.Texture
	nw, nh := old.Width*2, old.Height*2
	t := scene.NewTexture(nw, nh, old.Format)
	bpp := old.BytesPerPixel()
	for y := 0; y < old.Height; y++ {
		copy(t.Pix[y*nw*bpp:y*nw*bpp+old.Width*bpp], old.Pix[y*old.Width*bpp:(y+1)*old.Width*bpp])
	}
	t.ID = old.ID // same logical texture; the renderer recreates it at the new size
	t.Text = old.Text
	t.Version = old.Version + 1
	a.Texture = t
}

func (a *Atlas) clear() {
	clear(a.Texture.Pix)
	a.Texture.MarkDirty(0, 0, a.Texture.Width, a.Texture.Height)
	a.shelves = a.shelves[:0]
	a.nextY = 0
	a.Generation++
}

// Upload copies a mask into the atlas at (x, y).
func (a *Atlas) Upload(x, y int, m *image.Alpha) {
	t := a.Texture
	b := m.Bounds()
	for row := 0; row < b.Dy(); row++ {
		copy(t.Pix[(y+row)*t.Width+x:], m.Pix[row*m.Stride:row*m.Stride+b.Dx()])
	}
	t.MarkDirty(x, y, x+b.Dx(), y+b.Dy())
}

// uploadRGBA copies a 4-byte-per-texel mask into a Subpixel8 or RGBA8 atlas at
// (x, y).
func (a *Atlas) uploadRGBA(x, y int, m *image.RGBA) {
	t := a.Texture
	b := m.Bounds()
	for row := 0; row < b.Dy(); row++ {
		copy(t.Pix[((y+row)*t.Width+x)*4:], m.Pix[row*m.Stride:row*m.Stride+b.Dx()*4])
	}
	t.MarkDirty(x, y, x+b.Dx(), y+b.Dy())
}
