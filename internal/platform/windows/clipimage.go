//go:build windows

package windows

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/bits"
	"sync"

	"github.com/tyler-builds/jamb/internal/win32"
)

// Clipboard images are read as PNG when the program that copied them offered
// it (browsers, Office and the Snipping Tool do), and otherwise as a
// device-independent bitmap (DIB), which every program that copies an image
// offers or the system makes from what it offered. They are written in both,
// so that programs that read only bitmaps can paste them too.

// pngFormats are the clipboard formats under which programs put PNG data.
var pngFormats = sync.OnceValue(func() []uint32 {
	return []uint32{win32.RegisterClipboardFormat("PNG"), win32.RegisterClipboardFormat("image/png")}
})

// ClipboardImage implements platform.Window.
func (w *window) ClipboardImage() (image.Image, bool) {
	formats := append(pngFormats(), win32.CF_DIB)
	f, data, ok := win32.ReadClipboardData(w.hwnd, formats...)
	if !ok {
		return nil, false
	}
	var img image.Image
	var err error
	if f == win32.CF_DIB {
		img, err = decodeDIB(data)
	} else {
		img, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, false
	}
	return img, true
}

// HasClipboardImage implements platform.Window.
func (w *window) HasClipboardImage() bool {
	return win32.ClipboardHas(append(pngFormats(), win32.CF_DIB)...)
}

// SetClipboardImage implements platform.Window.
func (w *window) SetClipboardImage(img image.Image) {
	nrgba := toNRGBA(img)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	data := []win32.ClipboardData{{Format: win32.CF_DIBV5, Data: encodeDIBV5(nrgba)}}
	if enc.Encode(&buf, nrgba) == nil {
		data = append(data, win32.ClipboardData{Format: pngFormats()[0], Data: buf.Bytes()})
	}
	win32.WriteClipboardData(w.hwnd, data...)
}

// toNRGBA returns img as straight-alpha pixels whose bounds start at 0, 0.
func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Rect, img, b.Min, draw.Src)
	return n
}

// The parts of a DIB header that decodeDIB and encodeDIBV5 use.
const (
	biRGB          = 0
	biBitfields    = 3
	infoHeaderSize = 40  // BITMAPINFOHEADER
	v5HeaderSize   = 124 // BITMAPV5HEADER
	lcsSRGB        = 0x73524742
	lcsGMImages    = 4
	maxDIBPixels   = 1 << 28
)

// encodeDIBV5 returns img as a packed DIB with a BITMAPV5HEADER: 32 bits per
// pixel with straight alpha, bottom-up, in sRGB. This is the bitmap the
// system converts to the other bitmap formats when a program asks for them.
func encodeDIBV5(img *image.NRGBA) []byte {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	b := make([]byte, v5HeaderSize+4*w*h)
	le := binary.LittleEndian
	le.PutUint32(b[0:], v5HeaderSize)
	le.PutUint32(b[4:], uint32(w))
	le.PutUint32(b[8:], uint32(h)) // positive: the bottom row comes first
	le.PutUint16(b[12:], 1)        // planes
	le.PutUint16(b[14:], 32)
	le.PutUint32(b[16:], biBitfields)
	le.PutUint32(b[20:], uint32(4*w*h))
	le.PutUint32(b[40:], 0x00FF0000) // red mask
	le.PutUint32(b[44:], 0x0000FF00) // green
	le.PutUint32(b[48:], 0x000000FF) // blue
	le.PutUint32(b[52:], 0xFF000000) // alpha
	le.PutUint32(b[56:], lcsSRGB)
	le.PutUint32(b[108:], lcsGMImages)
	px := b[v5HeaderSize:]
	for y := range h {
		src := img.Pix[y*img.Stride : y*img.Stride+4*w]
		dst := px[(h-1-y)*4*w:]
		for x := range w {
			r, g, bl, a := src[4*x], src[4*x+1], src[4*x+2], src[4*x+3]
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = bl, g, r, a
		}
	}
	return b
}

// decodeDIB decodes a packed DIB: a BITMAPINFOHEADER or a later, larger
// header, the color masks or palette, then the pixels. It reads 1, 4 and 8
// bits per pixel with a palette, 16 and 32 bits with or without color masks,
// and 24 bits.
//
// The fourth byte of a 32-bit pixel is alpha only in some programs' bitmaps;
// others leave it 0. As browsers do, it is read as alpha unless it is 0 in
// every pixel, and the color is read as premultiplied unless some pixel has a
// channel brighter than its alpha.
func decodeDIB(b []byte) (image.Image, error) {
	errBad := errors.New("windows: unsupported or damaged clipboard bitmap")
	if len(b) < infoHeaderSize {
		return nil, errBad
	}
	le := binary.LittleEndian
	hdr := int(le.Uint32(b[0:]))
	w, h := int(int32(le.Uint32(b[4:]))), int(int32(le.Uint32(b[8:])))
	bpp, comp := int(le.Uint16(b[14:])), le.Uint32(b[16:])
	used := int(le.Uint32(b[32:]))
	topDown := h < 0
	if topDown {
		h = -h
	}
	if hdr < infoHeaderSize || hdr > len(b) || w <= 0 || h <= 0 || w*h > maxDIBPixels {
		return nil, errBad
	}
	stride := (w*bpp + 31) / 32 * 4
	off := hdr
	var masks [4]uint32 // red, green, blue, alpha
	var palette color.Palette
	switch {
	case comp == biBitfields && (bpp == 16 || bpp == 32):
		if hdr == infoHeaderSize {
			if len(b) < off+12 {
				return nil, errBad
			}
			for i := range 3 {
				masks[i] = le.Uint32(b[off+4*i:])
			}
			off += 12
		} else {
			for i := range 3 {
				masks[i] = le.Uint32(b[40+4*i:])
			}
			if hdr >= 56 {
				masks[3] = le.Uint32(b[52:])
			}
			// Some programs repeat the masks after a larger header, as after a
			// BITMAPINFOHEADER; skip them when they are there.
			if len(b) >= off+12+stride*h && le.Uint32(b[off:]) == masks[0] &&
				le.Uint32(b[off+4:]) == masks[1] && le.Uint32(b[off+8:]) == masks[2] {
				off += 12
			}
		}
	case comp != biRGB:
		return nil, errBad // run-length encoded, JPEG or PNG bitmaps
	case bpp == 32:
		masks = [4]uint32{0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000}
	case bpp == 16:
		masks = [4]uint32{0x7C00, 0x03E0, 0x001F, 0}
	case bpp == 24:
	case bpp == 1 || bpp == 4 || bpp == 8:
		n := used
		if n == 0 || n > 1<<bpp {
			n = 1 << bpp
		}
		if len(b) < off+4*n {
			return nil, errBad
		}
		palette = make(color.Palette, n)
		for i := range n {
			e := b[off+4*i:]
			palette[i] = color.NRGBA{e[2], e[1], e[0], 0xFF}
		}
		off += 4 * n
	default:
		return nil, errBad
	}
	if len(b) < off+stride*h {
		return nil, errBad
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	hasAlpha, premul := false, true
	for y := range h {
		row := b[off+stride*y:]
		if !topDown {
			row = b[off+stride*(h-1-y):]
		}
		dst := img.Pix[y*img.Stride:]
		for x := range w {
			var c color.NRGBA
			switch bpp {
			case 24:
				c = color.NRGBA{row[3*x+2], row[3*x+1], row[3*x], 0xFF}
			case 16, 32:
				var v uint32
				if bpp == 16 {
					v = uint32(le.Uint16(row[2*x:]))
				} else {
					v = le.Uint32(row[4*x:])
				}
				c = color.NRGBA{maskedByte(v, masks[0]), maskedByte(v, masks[1]), maskedByte(v, masks[2]), 0xFF}
				if masks[3] != 0 {
					c.A = maskedByte(v, masks[3])
					hasAlpha = hasAlpha || c.A != 0
					premul = premul && c.R <= c.A && c.G <= c.A && c.B <= c.A
				}
			default:
				shift := 8 - bpp - x*bpp%8
				i := int(row[x*bpp/8]>>shift) & (1<<bpp - 1)
				if i < len(palette) {
					c = palette[i].(color.NRGBA)
				} else {
					c = color.NRGBA{A: 0xFF}
				}
			}
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = c.R, c.G, c.B, c.A
		}
	}
	if masks[3] != 0 && !hasAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 0xFF
		}
	} else if masks[3] != 0 && premul {
		return &image.RGBA{Pix: img.Pix, Stride: img.Stride, Rect: img.Rect}, nil
	}
	return img, nil
}

// maskedByte returns the bits of v under mask, scaled to 0..255.
func maskedByte(v, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	width := bits.OnesCount32(mask)
	x := (v & mask) >> shift
	if width >= 8 {
		return uint8(x >> (width - 8))
	}
	maxV := uint32(1)<<width - 1
	return uint8((x*255 + maxV/2) / maxV)
}
