package text

import (
	"image"
	"testing"
)

// Layers are drawn over each other, premultiplied: a full layer replaces what
// is under it, a half-covered one mixes with it, and uncovered pixels stay.
func TestCompositeLayer(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 3, 1))
	red, blue := [4]float32{1, 0, 0, 1}, [4]float32{0, 0, 1, 1}
	compositeLayer(dst, []byte{255, 255, 0}, 3, 0, 0, red)
	compositeLayer(dst, []byte{255, 128}, 2, 1, 0, blue) // starts at x = 1
	want := []uint8{
		255, 0, 0, 255, // red only
		0, 0, 255, 255, // blue over red
		0, 0, 128, 128, // half blue over nothing
	}
	if string(dst.Pix) != string(want) {
		t.Fatalf("pixels %v, want %v", dst.Pix, want)
	}
	// A half-transparent color covers half.
	dst = image.NewRGBA(image.Rect(0, 0, 1, 1))
	compositeLayer(dst, []byte{255}, 1, 0, 0, [4]float32{0, 1, 0, 0.5})
	if got := dst.Pix; got[1] != 128 || got[3] != 128 {
		t.Fatalf("pixel %v, want green at half alpha", got)
	}
}
