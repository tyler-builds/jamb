//go:build windows

package text

import (
	"image"
	"os"
	"testing"

	"golang.org/x/image/font/sfnt"
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
