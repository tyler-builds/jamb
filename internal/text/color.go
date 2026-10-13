package text

import (
	"image"

	"github.com/tyler-builds/jamb/internal/scene"
)

// Color glyphs, such as emoji, are drawn in their own colors rather than the
// text color. They are rasterized into premultiplied RGBA (see
// dwriteColorGlyph), kept in their own atlas, and drawn as images, faded by
// the text color's alpha so they fade with the text.

// compositeLayer draws one layer of a color glyph over dst: mask is its
// coverage, w pixels wide, with its top-left at (x, y) in dst, and c is its
// straight R, G, B, A color from 0 to 1. dst is premultiplied; the layer is
// drawn source-over.
func compositeLayer(dst *image.RGBA, mask []byte, w, x, y int, c [4]float32) {
	if w <= 0 {
		return
	}
	for row := 0; row < len(mask)/w; row++ {
		d := dst.Pix[(y+row)*dst.Stride+x*4:]
		for col, cov := range mask[row*w : (row+1)*w] {
			if cov == 0 {
				continue
			}
			a := c[3] * float32(cov) / 255
			p := d[col*4 : col*4+4]
			keep := 1 - a
			for i := range 3 {
				p[i] = uint8(c[i]*a*255 + float32(p[i])*keep + 0.5)
			}
			p[3] = uint8(a*255 + float32(p[3])*keep + 0.5)
		}
	}
}

// colorKey is the part of a glyph's cache key that comes from the text color:
// its R, G, B for the glyphs of a color font, whose layers may take the text
// color, and 0 for other glyphs. Alpha is left out, so that fading text does
// not rasterize its emoji again on each frame.
func colorKey(f *Font, c scene.Color) uint32 {
	if !f.hasColor() {
		return 0
	}
	b := func(v float32) uint32 { return uint32(min(max(v, 0), 1)*255 + 0.5) }
	return 1<<24 | b(c.R)<<16 | b(c.G)<<8 | b(c.B)
}

// keyColor returns the straight R, G, B that colorKey packed.
func keyColor(k uint32) [3]float32 {
	return [3]float32{float32(k>>16&0xFF) / 255, float32(k>>8&0xFF) / 255, float32(k&0xFF) / 255}
}

// colorAtlas returns the atlas of color glyphs, made on first use: most
// windows draw no emoji.
func (sys *System) colorAtlas() *Atlas {
	if sys.color == nil {
		sys.color = &Atlas{Texture: scene.NewTexture(atlasInitial, atlasInitial, scene.FormatRGBA8)}
	}
	return sys.color
}
