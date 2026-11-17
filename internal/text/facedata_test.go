package text

import (
	"bytes"
	"os"
	"strings"
	"testing"

	tsfont "github.com/go-text/typesetting/font"

	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
)

// checkPieces fails the test unless the pieces cover the layout's text in
// order, each line's pieces are as wide as the line, and a text piece's width
// is what its characters advance in the layout.
func checkPieces(t *testing.T, l *Layout, pieces []Piece, faces []FaceData) {
	t.Helper()
	at := 0
	widths := make([]float32, len(l.Lines))
	for _, p := range pieces {
		if p.Start != at || p.End <= p.Start {
			t.Fatalf("a piece runs from %d to %d after the pieces reached %d: %+v", p.Start, p.End, at, pieces)
		}
		at = p.End
		if p.Face >= len(faces) || p.Kind == PieceText && p.Face < 0 {
			t.Errorf("the piece %q has the face %d of %d", l.Text[p.Start:p.End], p.Face, len(faces))
		}
		widths[p.Line] += p.Width
	}
	if at != len(l.Text) {
		t.Fatalf("the pieces end at %d of %d", at, len(l.Text))
	}
	for i, ln := range l.Lines {
		var w float32
		for _, r := range ln.Runs {
			w += r.Width
		}
		if d := widths[i] - w; d > 0.01 || d < -0.01 {
			t.Errorf("line %d: the pieces are %v wide, the line %v", i, widths[i], w)
		}
	}
}

// A layout's pieces: runs of one face, the tabs and the breaks between
// paragraphs on their own, in the faces of the spans.
func TestPieces(t *testing.T) {
	sys := NewSystem()
	reg := sys.Registry()
	for _, f := range []struct {
		w    Weight
		data []byte
	}{{Regular, goregular.TTF}, {Medium, gomedium.TTF}} {
		if err := reg.Register("f", f.w, f.data); err != nil {
			t.Fatal(err)
		}
	}
	s := "plain bold\ttab\n\nnext 😀 wraps over a narrow width"
	bold := strings.Index(s, "bold")
	l := sys.LayoutSpans(s, Style{Family: "f", Size: 14}, []Span{{Start: bold, End: bold + 4, Weight: Medium}}, 120)
	pieces, faces := sys.Pieces(l)
	checkPieces(t, l, pieces, faces)
	kinds, faceOf := map[string]PieceKind{}, map[string]int{}
	for _, p := range pieces {
		kinds[s[p.Start:p.End]] = p.Kind
		faceOf[s[p.Start:p.End]] = p.Face
	}
	if kinds["\t"] != PieceTab || kinds["\n"] != PieceBreak || kinds["bold"] != PieceText {
		t.Errorf("the pieces are %v; want the tab, the breaks and the span on their own", kinds)
	}
	if faceOf["bold"] == faceOf["plain "] || len(faces[0].Notdef) == 0 || faces[0].NotdefKey == "" {
		t.Errorf("the span is in face %d and the text before it in %d, of %d; want each in its own face, with its missing-glyph font",
			faceOf["bold"], faceOf["plain "], len(faces))
	}
	if len(l.Lines) < 4 {
		t.Errorf("the text has %d lines; want it wrapped", len(l.Lines))
	}
}

// A character whose advance the layout changed, as kerning does, is a piece of
// its own with the change as its spacing.
func TestPiecesKerning(t *testing.T) {
	data, err := os.ReadFile("testdata/SourceSans-VF-HVAR.ttf")
	if err != nil {
		t.Fatal(err)
	}
	sys := NewSystem()
	if err := sys.Registry().Register("k", Regular, data); err != nil {
		t.Fatal(err)
	}
	l := sys.Layout("AVATAR To", Style{Family: "k", Size: 40}, 1000)
	pieces, faces := sys.Pieces(l)
	checkPieces(t, l, pieces, faces)
	kerned := 0
	for _, p := range pieces {
		if p.Spacing != 0 {
			kerned++
			if p.End-p.Start != 1 {
				t.Errorf("the kerned piece %q has more than one character", l.Text[p.Start:p.End])
			}
		}
	}
	if kerned == 0 {
		t.Errorf("no piece of %q carries kerning: %+v", l.Text, pieces)
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
