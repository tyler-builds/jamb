//go:build !windows

package text

import (
	"image"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"github.com/tyler-builds/jamb/internal/scene"
)

// systemEmojiFont loads the desktop's emoji font: the one fontconfig names on
// Linux, or one at a usual path. It skips the test when there is none.
func systemEmojiFont(t *testing.T) *Font {
	t.Helper()
	var files []fontFile
	for _, fb := range fontconfigFallbacks() {
		files = append(files, fb.file)
	}
	for _, p := range []string{
		"/usr/share/fonts/google-noto-color-emoji-fonts/Noto-COLRv1.ttf",
		"/usr/share/fonts/google-noto-color-emoji-fonts/NotoColorEmoji.ttf",
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/noto/NotoColorEmoji.ttf",
		"/System/Library/Fonts/Apple Color Emoji.ttc",
	} {
		files = append(files, fontFile{p, 0})
	}
	for _, file := range files {
		if f, err := loadFile(file); err == nil && f.hasColor() {
			return f
		}
	}
	t.Skip("no color emoji font on this system")
	return nil
}

// The crab, U+1F980, is drawn in its colors from the system's emoji font: an
// opaque image with several distinct colors, premultiplied.
func TestSystemEmojiFontDrawsInColor(t *testing.T) {
	f := systemEmojiFont(t)
	t.Logf("emoji font: %s", f.Name)
	id, ok := f.face.NominalGlyph(0x1F980)
	if !ok {
		t.Skipf("%s has no U+1F980", f.Name)
	}
	alloc := func(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }
	dx, dy, m, ok := dwriteColorGlyph(f, sfnt.GlyphIndex(id), 32, 0, [3]float32{0, 0, 0}, alloc)
	if !ok || m == nil {
		t.Fatalf("color rasterization failed (ok=%v)", ok)
	}
	if w, h := m.Rect.Dx(), m.Rect.Dy(); w < 20 || w > 48 || h < 20 || h > 48 || dy > -16 || dy < -40 || dx < -8 || dx > 8 {
		t.Errorf("the 32-pixel emoji is %d×%d at (%d, %d) from the origin", w, h, dx, dy)
	}
	colors := map[[3]uint8]bool{}
	for i := 0; i < len(m.Pix); i += 4 {
		p := m.Pix[i : i+4]
		if p[0] > p[3] || p[1] > p[3] || p[2] > p[3] {
			t.Fatalf("pixel %d is not premultiplied: %v", i/4, p)
		}
		if p[3] == 255 {
			colors[[3]uint8{p[0] >> 5, p[1] >> 5, p[2] >> 5}] = true
		}
	}
	if len(colors) < 5 {
		t.Errorf("%d distinct opaque colors, want several", len(colors))
	}

	// An ordinary font has no color glyphs.
	g, err := ParseFont("Go Regular", goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if g.hasColor() {
		t.Error("Go Regular reports color glyphs")
	}
}

// Text with an emoji draws the emoji as an image from the color atlas, through
// the platform's emoji fallback, and the letter as a glyph.
func TestSystemDrawsColorEmojiOffWindows(t *testing.T) {
	systemEmojiFont(t)
	sys := NewSystem()
	if f := sys.reg.fallbackFor(0x1F980); f == nil || !f.hasColor() {
		t.Skip("the registry has no color emoji fallback here")
	}
	l := sys.Layout("a\U0001F980", Style{Size: 16}, 0)
	var sc scene.Scene
	sys.Draw(&sc, l, 0, 0, 0, 1, 0, scene.Color{R: 1, G: 1, B: 1, A: 1})
	if len(sc.Images) != 1 || len(sc.Glyphs) != 1 {
		t.Fatalf("%d images and %d glyphs, want the emoji as an image and the letter as a glyph", len(sc.Images), len(sc.Glyphs))
	}
	if im := sc.Images[0]; im.Texture != sys.color.Texture {
		t.Fatalf("the emoji is drawn from %v", im.Texture)
	}
}
