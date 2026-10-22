package text

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/go-text/typesetting/di"
	tsfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// cutCandidates are the fonts the cut view is compared on, when they are installed:
// Latin, Arabic and CJK fonts, and a collection, which is never cut.
var cutCandidates = []string{
	`C:\Windows\Fonts\segoeui.ttf`,
	`C:\Windows\Fonts\arial.ttf`,
	`C:\Windows\Fonts\DUBAI-REGULAR.TTF`,
	`C:\Windows\Fonts\malgun.ttf`,
	`C:\Windows\Fonts\SimsunExtG.ttf`,
	`C:\Windows\Fonts\msyh.ttc`,
	`C:\Windows\Fonts\bahnschrift.ttf`, // a variable font, where installed
}

// shapingSamples are shaped in each font: Latin with trailing spaces (the line
// wrapper trims them), Arabic, Han and Hangul, each with its script.
var shapingSamples = []struct {
	text   string
	script language.Script
}{
	{"Hello, world! 0123456789 \u2014 \u201cquotes\u201d fi ffi   ", language.Latin},
	{"\u0645\u0631\u062d\u0628\u0627 \u0628\u0627\u0644\u0639\u0627\u0644\u0645 \u0661\u0662\u0663", language.Arabic},
	{"\u4e2d\u6587\u5b57\u7b26\u6d4b\u8bd5 \u65e5\u672c\u8a9e", language.Han},
	{"\ud55c\uad6d\uc5b4 \ud14d\uc2a4\ud2b8", language.Hangul},
}

// fontData returns the bytes of each font to compare: the system fonts that are
// installed, and the embedded Go fonts, which always are.
func fontData(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{"goregular": goregular.TTF, "gomono": gomono.TTF}
	for _, p := range cutCandidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue // not installed here
		}
		out[p] = b
	}
	return out
}

// TestShapingFileMatchesFullFonts checks that go-text shapes and measures every
// font the same from the cut view as from the file: the glyph count, the extents,
// the advances, the vertical origins, the cmap, and the shaped glyphs of the sample
// texts (advances, offsets, clusters and bounds).
func TestShapingFileMatchesFullFonts(t *testing.T) {
	for name, data := range fontData(t) {
		t.Run(name, func(t *testing.T) {
			if bytes.HasPrefix(data, []byte("ttcf")) {
				if _, ok := shapingResource(data).(*bytes.Reader); !ok {
					t.Fatal("a collection was cut")
				}
				return
			}
			if trimGlyf(data) == nil {
				t.Skip("the font is not cut")
			}
			full, err := tsfont.ParseTTF(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			cut, err := tsfont.ParseTTF(shapingResource(data))
			if err != nil {
				t.Fatalf("the cut view does not parse: %v", err)
			}
			for gid := range glyphCount(t, data) {
				g := tsfont.GID(gid)
				fe, fok := full.GlyphExtents(g)
				ce, cok := cut.GlyphExtents(g)
				if fe != ce || fok != cok {
					t.Fatalf("glyph %d extents: cut %v %v, full %v %v", gid, ce, cok, fe, fok)
				}
				if a, b := full.HorizontalAdvance(g), cut.HorizontalAdvance(g); a != b {
					t.Fatalf("glyph %d advance: cut %v, full %v", gid, b, a)
				}
				if a, b := full.VerticalAdvance(g), cut.VerticalAdvance(g); a != b {
					t.Fatalf("glyph %d vertical advance: cut %v, full %v", gid, b, a)
				}
				ax, ay := full.GlyphVOrigin(g)
				bx, by := cut.GlyphVOrigin(g)
				if ax != bx || ay != by {
					t.Fatalf("glyph %d vertical origin: cut %v %v, full %v %v", gid, bx, by, ax, ay)
				}
			}
			for _, s := range shapingSamples {
				for _, r := range s.text {
					fg, fok := full.NominalGlyph(r)
					cg, cok := cut.NominalGlyph(r)
					if fg != cg || fok != cok {
						t.Fatalf("rune %U: cut %d %v, full %d %v", r, cg, cok, fg, fok)
					}
				}
				want := shapeSample(full, s.text, s.script)
				got := shapeSample(cut, s.text, s.script)
				if !reflect.DeepEqual(want, got) {
					t.Fatalf("shaping %q: cut\n%+v\nfull\n%+v", s.text, got, want)
				}
			}
		})
	}
}

// shapeSample shapes text left to right in face at 14 pixels, with the shaper jamb
// uses; it returns the parts of the output that the layout reads.
func shapeSample(face *tsfont.Face, text string, script language.Script) []any {
	var sh shaping.HarfbuzzShaper
	runes := []rune(text)
	out := sh.Shape(shaping.Input{
		Text: runes, RunStart: 0, RunEnd: len(runes), Direction: di.DirectionLTR,
		Face: face, Size: fixed.I(14), Script: script, Language: language.DefaultLanguage(),
	})
	return []any{out.Advance, out.Size, out.Glyphs, out.LineBounds, out.GlyphBounds}
}

// glyphCount returns the number of glyphs of the font, from its maxp table.
func glyphCount(t *testing.T, data []byte) int {
	t.Helper()
	maxp := tableBytes(data, "maxp")
	if len(maxp) < 6 {
		t.Fatal("no maxp table")
	}
	return int(binary.BigEndian.Uint16(maxp[4:]))
}

// tableBytes returns the bytes of the table with the tag, or nil.
func tableBytes(data []byte, tag string) []byte {
	be := binary.BigEndian
	if len(data) < 12 {
		return nil
	}
	n := int(be.Uint16(data[4:6]))
	for i := 0; i < n && 12+16*i+16 <= len(data); i++ {
		e := 12 + 16*i
		if string(data[e:e+4]) != tag {
			continue
		}
		off, length := int(be.Uint32(data[e+8:])), int(be.Uint32(data[e+12:]))
		if off+length <= len(data) {
			return data[off : off+length]
		}
	}
	return nil
}

// TestShapingFileReadsTheCutFile checks that the view reads as the file it stands
// for: the directory, then the font, then the padding and the two tables it adds.
func TestShapingFileReadsTheCutFile(t *testing.T) {
	data := goregular.TTF
	f := trimGlyf(data)
	if f == nil {
		t.Fatal("goregular is not cut")
	}
	want := append(append(append([]byte(nil), f.head...), data[len(f.head):]...), f.extra...)
	if int64(len(want)) != f.size() {
		t.Fatalf("size = %d, want %d", f.size(), len(want))
	}
	rng := rand.New(rand.NewSource(1))
	buf := make([]byte, 4096)
	for range 500 {
		off := rng.Int63n(int64(len(want)) + 10)
		n, err := f.ReadAt(buf[:rng.Intn(len(buf))], off)
		got := buf[:n]
		if off >= int64(len(want)) {
			if n != 0 || err != io.EOF {
				t.Fatalf("ReadAt past the end = %d, %v", n, err)
			}
			continue
		}
		if !bytes.Equal(got, want[off:off+int64(n)]) {
			t.Fatalf("ReadAt(%d) differs from the file", off)
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	all, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(all, want) {
		t.Fatalf("Read of the whole view: %d bytes, err %v; want %d bytes", len(all), err, len(want))
	}
	if _, err := f.Seek(-4, io.SeekEnd); err != nil {
		t.Fatal(err)
	}
	tail := make([]byte, 8)
	if n, err := f.Read(tail); n != 4 || err != nil || !bytes.Equal(tail[:4], want[len(want)-4:]) {
		t.Fatalf("Read at the end = %d, %v", n, err)
	}
}

// TestShapingFileLeavesFontsItCannotCutAlone checks that a collection, a font with
// CFF outlines, and damaged data are parsed from their bytes as before, and that
// trimming a damaged font never panics.
func TestShapingFileLeavesFontsItCannotCutAlone(t *testing.T) {
	if _, ok := shapingResource([]byte("ttcf....")).(*bytes.Reader); !ok {
		t.Error("a collection was not left alone")
	}
	for name, data := range fontData(t) {
		if tableBytes(data, "fvar") != nil && trimGlyf(data) != nil {
			t.Errorf("%s is a variable font and was cut", name)
		}
	}
	// A composite glyph with two components: the first placed by offsets, the second
	// by points (no ARGS_ARE_XY_VALUES), which needs the outline's points.
	composite := []byte{
		0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, // the glyph header: no contours, no box
		0x00, 0x23, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, // XY values, words, MORE_COMPONENTS
		0x00, 0x01, 0x00, 0x06, 0x00, 0x00, 0x00, 0x00, // words, no XY values: matched by points
	}
	if anchored, ok := usesPointAnchors(composite); !anchored || !ok {
		t.Errorf("usesPointAnchors = %v, %v; want the point-matched component found", anchored, ok)
	}
	composite[19] |= 0x02 // the second component's flags now place it by offsets
	if anchored, ok := usesPointAnchors(composite); anchored || !ok {
		t.Errorf("usesPointAnchors = %v, %v; want no point-matched component", anchored, ok)
	}
	if _, ok := usesPointAnchors(composite[:14]); ok {
		t.Error("a truncated composite glyph was read")
	}
	for _, n := range []int{0, 4, 11, 12, 100, 500} {
		if trimGlyf(goregular.TTF[:n]) != nil {
			t.Errorf("a font cut to %d bytes was cut", n)
		}
	}
	rng := rand.New(rand.NewSource(2))
	for i := range 2000 {
		b := append([]byte(nil), goregular.TTF[:20000]...)
		for range 1 + i%8 {
			b[rng.Intn(len(b))] = byte(rng.Intn(256))
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("trimGlyf panicked on damaged data %d: %v", i, r)
				}
			}()
			trimGlyf(b)
		}()
	}
}

// TestGoTextHasNoContourPoints guards the reason the cut view is safe: go-text does
// not resolve contour points, so the GPOS anchors and caret values that refer to
// them do not depend on the outlines. If this fails, go-text resolves them now, and
// shapingResource must not cut fonts that use them.
func TestGoTextHasNoContourPoints(t *testing.T) {
	face, err := tsfont.ParseTTF(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []tsfont.GID{1, 2, 36, 100} {
		if _, _, ok := face.GetGlyphContourPoint(g, 0); ok {
			t.Errorf("go-text resolves contour point 0 of glyph %d; cut fonts with contour anchors", g)
		}
	}
}
