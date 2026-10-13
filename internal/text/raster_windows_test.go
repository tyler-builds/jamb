//go:build windows

package text

import (
	"image"
	"os"
	"testing"

	"golang.org/x/image/font/sfnt"

	"github.com/tyler-builds/jamb/internal/scene"
)

func segoe(t *testing.T) *Font {
	t.Helper()
	f, err := loadFile(winFont("segoeui.ttf"))
	if err != nil {
		t.Skipf("Segoe UI not available: %v", err)
	}
	return f
}

func row(m *image.Alpha, y int) []uint8 { return m.Pix[y*m.Stride : y*m.Stride+m.Rect.Dx()] }

// fullRun returns the longest run of near-full coverage in a row.
func fullRun(r []uint8) (best int) {
	n := 0
	for _, c := range r {
		if c >= 240 {
			n++
			best = max(best, n)
		} else {
			n = 0
		}
	}
	return best
}

// DirectWrite's natural modes hint vertically: horizontal features (crossbars,
// x-height and cap-height edges) snap to whole pixel rows. Vertical stems keep
// their subpixel position, so they are not asserted here.
func TestDirectWriteHintsHorizontalEdges(t *testing.T) {
	f := segoe(t)
	if f.dwriteFace() == nil {
		t.Skip("DirectWrite face unavailable")
	}
	alloc := func(w, h int) *image.Alpha { return image.NewAlpha(image.Rect(0, 0, w, h)) }
	glyph := func(r rune) (int, int, *image.Alpha) {
		id, ok := f.face.NominalGlyph(r)
		if !ok {
			t.Fatalf("no glyph for %q", r)
		}
		dx, dy, m, ok := dwriteGlyph(f, sfnt.GlyphIndex(id), 15, 0, alloc)
		if !ok || m == nil {
			t.Fatalf("%q: DirectWrite rasterization failed (ok=%v, mask=%v)", r, ok, m)
		}
		if os.Getenv("DUMP_GLYPHS") != "" {
			t.Logf("%q dx=%d dy=%d %dx%d", r, dx, dy, m.Rect.Dx(), m.Rect.Dy())
			for y := 0; y < m.Rect.Dy(); y++ {
				t.Logf("%v", row(m, y))
			}
		}
		return dx, dy, m
	}

	// 'l': the ascender top and the baseline are on pixel boundaries, so every
	// row has the same coverage and the mask sits on whole rows above the baseline.
	_, dy, l := glyph('l')
	if dy >= 0 || dy+l.Rect.Dy() != 0 {
		t.Errorf("'l' spans rows %d..%d, want it to end exactly at the baseline", dy, dy+l.Rect.Dy())
	}
	for y := 1; y < l.Rect.Dy(); y++ {
		if string(row(l, y)) != string(row(l, 0)) {
			t.Errorf("'l' row %d = %v differs from row 0 = %v: edges are not pixel aligned", y, row(l, y), row(l, 0))
			break
		}
	}

	// 'H' and 'e': the crossbar is one fully covered row.
	for _, r := range "He" {
		_, _, m := glyph(r)
		ok := false
		for y := 0; y < m.Rect.Dy(); y++ {
			if fullRun(row(m, y)) >= 5 {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%q: no row with a fully covered crossbar", r)
		}
	}

	// Whitespace reports success with no mask.
	id, _ := f.face.NominalGlyph(' ')
	if _, _, m, ok := dwriteGlyph(f, sfnt.GlyphIndex(id), 15, 0, alloc); !ok || m != nil {
		t.Errorf("space: ok=%v mask=%v, want ok and no mask", ok, m)
	}
}

func emojiFont(t *testing.T) *Font {
	t.Helper()
	f, err := loadFile(winFont("seguiemj.ttf"))
	if err != nil {
		t.Skipf("Segoe UI Emoji not available: %v", err)
	}
	if !f.hasColor() {
		t.Fatal("Segoe UI Emoji reports no color glyphs")
	}
	return f
}

// An emoji is rasterized in its colors: the grinning face has yellow and dark
// pixels. A letter of an ordinary font is not a color glyph.
func TestDirectWriteColorGlyph(t *testing.T) {
	f := emojiFont(t)
	alloc := func(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }
	id, ok := f.face.NominalGlyph('😀')
	if !ok {
		t.Fatal("no glyph for U+1F600")
	}
	_, _, m, ok := dwriteColorGlyph(f, sfnt.GlyphIndex(id), 32, 0, [3]float32{1, 1, 1}, alloc)
	if !ok || m == nil {
		t.Fatalf("color rasterization failed (ok=%v)", ok)
	}
	var yellow, dark bool
	for i := 0; i < len(m.Pix); i += 4 {
		r, g, b, a := m.Pix[i], m.Pix[i+1], m.Pix[i+2], m.Pix[i+3]
		if a == 255 && r > 200 && g > 150 && b < 100 {
			yellow = true
		}
		if a == 255 && r < 120 && g < 120 && b < 120 {
			dark = true
		}
		if r > a || g > a || b > a {
			t.Fatalf("pixel %d is not premultiplied: %v", i/4, m.Pix[i:i+4])
		}
	}
	if !yellow || !dark {
		t.Errorf("the emoji has yellow %v and dark %v pixels, want both", yellow, dark)
	}

	s := segoe(t)
	if s.hasColor() {
		t.Error("Segoe UI reports color glyphs")
	}
}

// Text with an emoji draws the emoji as an image from the color atlas and the
// letters as glyphs. Fading the text reuses the emoji already rasterized.
func TestSystemDrawsColorEmoji(t *testing.T) {
	emojiFont(t)
	sys := NewSystem()
	l := sys.Layout("a😀", Style{Size: 16}, 0)
	var sc scene.Scene
	sys.Draw(&sc, l, 0, 0, 0, 1, 0, scene.Color{R: 1, G: 1, B: 1, A: 1})
	if len(sc.Images) != 1 || len(sc.Glyphs) != 1 {
		t.Fatalf("%d images and %d glyphs, want the emoji as an image and the letter as a glyph", len(sc.Images), len(sc.Glyphs))
	}
	if im := sc.Images[0]; im.Texture != sys.color.Texture || im.Opacity != 1 {
		t.Fatalf("the emoji is drawn from %v at opacity %v", im.Texture, im.Opacity)
	}
	n := len(sys.glyphs)
	sc.Reset()
	sys.Draw(&sc, l, 0, 0, 0, 1, 0, scene.Color{R: 1, G: 1, B: 1, A: 0.5})
	if len(sys.glyphs) != n {
		t.Errorf("fading the text cached %d more glyphs", len(sys.glyphs)-n)
	}
	if len(sc.Images) != 1 || sc.Images[0].Opacity != 0.5 {
		t.Errorf("the faded emoji is drawn at opacity %v, want 0.5", sc.Images[0].Opacity)
	}
}
