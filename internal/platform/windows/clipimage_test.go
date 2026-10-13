//go:build windows

package windows

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// testImage is a 3x2 image with an opaque, a half-transparent and a clear
// pixel, so a row flip, a channel swap or lost alpha shows.
func testImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	img.SetNRGBA(1, 0, color.NRGBA{0, 255, 0, 128})
	img.SetNRGBA(2, 0, color.NRGBA{0, 0, 0, 0})
	img.SetNRGBA(0, 1, color.NRGBA{0, 0, 255, 255})
	img.SetNRGBA(1, 1, color.NRGBA{10, 20, 30, 255})
	img.SetNRGBA(2, 1, color.NRGBA{200, 100, 50, 64})
	return img
}

// A bitmap written for the clipboard reads back as the same pixels.
func TestDIBV5RoundTrip(t *testing.T) {
	want := testImage()
	got, err := decodeDIB(encodeDIBV5(want))
	if err != nil {
		t.Fatal(err)
	}
	n, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("decoded a %T, want *image.NRGBA (straight alpha)", got)
	}
	if n.Rect != want.Rect || string(n.Pix) != string(want.Pix) {
		t.Fatalf("pixels %v, want %v", n.Pix, want.Pix)
	}
}

// dib builds a packed DIB with a BITMAPINFOHEADER, the given extra bytes
// (masks or palette) and rows, listed top row first; a negative h stores them
// top-down as given, a positive one bottom-up.
func dib(w, h, bpp int, comp uint32, extra []byte, rows ...[]byte) []byte {
	b := make([]byte, infoHeaderSize)
	le := binary.LittleEndian
	le.PutUint32(b[0:], infoHeaderSize)
	le.PutUint32(b[4:], uint32(int32(w)))
	le.PutUint32(b[8:], uint32(int32(h)))
	le.PutUint16(b[12:], 1)
	le.PutUint16(b[14:], uint16(bpp))
	le.PutUint32(b[16:], comp)
	b = append(b, extra...)
	stride := (w*bpp + 31) / 32 * 4
	if h > 0 {
		for i := len(rows) - 1; i >= 0; i-- {
			b = append(b, pad(rows[i], stride)...)
		}
	} else {
		for _, r := range rows {
			b = append(b, pad(r, stride)...)
		}
	}
	return b
}

func pad(row []byte, n int) []byte {
	return append(append([]byte{}, row...), make([]byte, n-len(row))...)
}

func wantPixels(t *testing.T, img image.Image, want ...color.NRGBA) {
	t.Helper()
	b := img.Bounds()
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if got != want[i] {
				t.Errorf("pixel %d,%d is %v, want %v", x, y, got, want[i])
			}
			i++
		}
	}
}

// 24-bit rows are padded to four bytes and stored bottom-up or top-down.
func TestDecodeDIB24(t *testing.T) {
	red, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	for _, h := range []int{2, -2} {
		img, err := decodeDIB(dib(1, h, 24, biRGB, nil, []byte{0, 0, 255}, []byte{255, 0, 0}))
		if err != nil {
			t.Fatal(err)
		}
		wantPixels(t, img, red, blue)
	}
}

// A 32-bit bitmap whose fourth bytes are all 0 is opaque; one with alpha keeps
// it, read as premultiplied when no channel is brighter than its alpha.
func TestDecodeDIB32Alpha(t *testing.T) {
	img, err := decodeDIB(dib(2, 1, 32, biRGB, nil, []byte{0, 0, 255, 0, 255, 0, 0, 0}))
	if err != nil {
		t.Fatal(err)
	}
	wantPixels(t, img, color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255})

	// Straight: 200 is brighter than the alpha of 128.
	img, err = decodeDIB(dib(1, 1, 32, biRGB, nil, []byte{0, 0, 200, 128}))
	if err != nil {
		t.Fatal(err)
	}
	wantPixels(t, img, color.NRGBA{200, 0, 0, 128})

	// Premultiplied: half-transparent red is stored as 128, 0, 0, 128.
	img, err = decodeDIB(dib(1, 1, 32, biRGB, nil, []byte{0, 0, 128, 128}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.RGBA); !ok {
		t.Fatalf("decoded a %T, want premultiplied *image.RGBA", img)
	}
	wantPixels(t, img, color.NRGBA{255, 0, 0, 128})
}

// Color masks after a BITMAPINFOHEADER describe 16- and 32-bit pixels.
func TestDecodeDIBBitfields(t *testing.T) {
	le := binary.LittleEndian
	masks := func(m ...uint32) []byte {
		b := make([]byte, 4*len(m))
		for i, v := range m {
			le.PutUint32(b[4*i:], v)
		}
		return b
	}
	// 5-6-5: pure green is 0x07E0.
	img, err := decodeDIB(dib(1, 1, 16, biBitfields, masks(0xF800, 0x07E0, 0x001F), []byte{0xE0, 0x07}))
	if err != nil {
		t.Fatal(err)
	}
	wantPixels(t, img, color.NRGBA{0, 255, 0, 255})
	// Red in the low byte.
	img, err = decodeDIB(dib(1, 1, 32, biBitfields, masks(0xFF, 0xFF00, 0xFF0000), []byte{200, 0, 0, 0}))
	if err != nil {
		t.Fatal(err)
	}
	wantPixels(t, img, color.NRGBA{200, 0, 0, 255})
}

// Paletted pixels are packed from the high bits of each byte.
func TestDecodeDIBPalette(t *testing.T) {
	palette := []byte{0, 0, 0, 0, 255, 255, 255, 0} // black, white
	img, err := decodeDIB(dib(3, 1, 1, biRGB, palette, []byte{0b1010_0000}))
	if err != nil {
		t.Fatal(err)
	}
	black, white := color.NRGBA{0, 0, 0, 255}, color.NRGBA{255, 255, 255, 255}
	wantPixels(t, img, white, black, white)
}

// Damaged or unsupported bitmaps are refused without a panic.
func TestDecodeDIBRefusesDamage(t *testing.T) {
	good := dib(2, 2, 24, biRGB, nil, []byte{1, 2, 3, 4, 5, 6}, []byte{1, 2, 3, 4, 5, 6})
	for name, b := range map[string][]byte{
		"short header": good[:20],
		"short pixels": good[:len(good)-1],
		"zero width":   dib(0, 2, 24, biRGB, nil),
		"rle":          dib(1, 1, 8, 1, nil, []byte{0}),
		"huge":         dib(1<<15, 1<<15, 32, biRGB, nil),
	} {
		if _, err := decodeDIB(b); err == nil {
			t.Errorf("%s: decoded without an error", name)
		}
	}
}
