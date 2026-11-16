package text

import (
	"bytes"
	"strings"
	"testing"

	tsfont "github.com/go-text/typesetting/font"

	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
)

// Faces gives the font files text is drawn with, its own face first and the
// missing-glyph font last, the same slice while the fonts are the same.
func TestFaces(t *testing.T) {
	sys := NewSystem()
	if err := sys.Registry().Register("f", Regular, goregular.TTF); err != nil {
		t.Fatal(err)
	}
	if err := sys.Registry().Register("f", Medium, gomedium.TTF); err != nil {
		t.Fatal(err)
	}
	faces := sys.Faces(Style{Family: "f", Size: 14})
	if len(faces) < 2 || &faces[0].Data[0] != &goregular.TTF[0] || !strings.HasPrefix(faces[0].Key, "jamb-face-") {
		t.Fatalf("the faces of Regular text are %d, the first %q; want Go Regular's file first", len(faces), faces[0].Key)
	}
	if last := faces[len(faces)-1]; !strings.HasPrefix(last.Key, "jamb-notdef-") {
		t.Errorf("the last face is %q, want the missing-glyph font", last.Key)
	}
	if again := sys.Faces(Style{Family: "f", Size: 20}); &again[0] != &faces[0] {
		t.Error("the faces of the same font are another slice")
	}
	medium := sys.Faces(Style{Family: "f", Size: 14, Weight: Medium})
	if &medium[0].Data[0] != &gomedium.TTF[0] || medium[0].Key == faces[0].Key {
		t.Errorf("the faces of Medium text start with %q, want Go Medium's file under a key of its own", medium[0].Key)
	}
}

// The missing-glyph font is a font go-text reads: every character but the
// controls and the default-ignorable ones has its one glyph, as wide as asked.
// (golang.org/x/image/font/sfnt reads no cmap of format 13; jamb never draws
// with the font, only a browser lays text out with it.)
func TestNotdefFont(t *testing.T) {
	data := notdefFont(1000, 600, 800, 200)
	face, err := tsfont.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	f := &Font{face: face}
	for _, r := range []rune{'a', '😀', '日', 0x1F3FD, 0x10FFFD} {
		g, ok := f.face.NominalGlyph(r)
		if !ok || g != 1 {
			t.Errorf("%U maps to glyph %d (%v), want 1", r, g, ok)
		}
	}
	for _, r := range []rune{'\t', '\n', 0x200D, 0xFE0F, 0xAD} {
		if _, ok := f.face.NominalGlyph(r); ok {
			t.Errorf("%U has the glyph, want it left to the browser as nothing", r)
		}
	}
	if adv := f.face.HorizontalAdvance(1); adv != 600 {
		t.Errorf("the glyph is %v wide, want 600", adv)
	}
	if checksum(data) != 0xB1B0AFBA {
		t.Errorf("the font's checksum is %#x, want 0xB1B0AFBA", checksum(data))
	}
}

// A layout's advances are those of its size rounded up to whole pixels, as
// go-text scales fonts: LayoutSize says which, and a layout agrees.
func TestLayoutSize(t *testing.T) {
	for _, c := range []struct{ size, want float32 }{{14, 14}, {13.5, 14}, {15.6, 16}, {16, 16}, {12.01, 13}} {
		if got := LayoutSize(c.size); got != c.want {
			t.Errorf("LayoutSize(%v) = %v, want %v", c.size, got, c.want)
		}
	}
	sys := NewSystem()
	if err := sys.Registry().Register("f", Regular, goregular.TTF); err != nil {
		t.Fatal(err)
	}
	a := sys.Layout("Wide text", Style{Family: "f", Size: 13.5}, 1000)
	b := sys.Layout("Wide text", Style{Family: "f", Size: 14}, 1000)
	if a.Width != b.Width {
		t.Errorf("text of 13.5 is %v wide and of 14 %v; LayoutSize says they are laid out alike", a.Width, b.Width)
	}
}
