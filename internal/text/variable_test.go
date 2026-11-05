package text

import (
	"os"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

// variableFont is a subset of Source Sans, a variable font with a weight axis
// from 200 to 900 and a default of 200, with the letters C and O.
const variableFont = "testdata/SourceSans-VF-HVAR.ttf"

func TestWghtAxis(t *testing.T) {
	data, err := os.ReadFile(variableFont)
	if err != nil {
		t.Fatal(err)
	}
	if lo, def, hi, ok := wghtAxis(data, 0); !ok || lo != 200 || def != 200 || hi != 900 {
		t.Errorf("wght axis %v %v %v (found %v); want 200 to 900, default 200", lo, def, hi, ok)
	}
	if _, _, _, ok := wghtAxis(goregular.TTF, 0); ok {
		t.Error("Go Regular, which is not variable, has a weight axis")
	}
}

// A variable font loaded at a weight draws and advances its glyphs at that
// weight, as Fedora's Noto Sans, one variable file, must for bold text.
func TestVariableFontAtAWeight(t *testing.T) {
	light, err := loadFile(fontFile{path: variableFont})
	if err != nil {
		t.Fatal(err)
	}
	bold, err := loadFile(fontFile{path: variableFont, wght: Bold})
	if err != nil {
		t.Fatal(err)
	}
	if light.varied || !bold.varied {
		t.Fatalf("varied: default %v, bold %v; want false, true", light.varied, bold.varied)
	}
	gid, ok := light.face.NominalGlyph('C')
	if !ok {
		t.Fatal("the font has no C")
	}
	if l, b := light.face.HorizontalAdvance(gid), bold.face.HorizontalAdvance(gid); b <= l {
		t.Errorf("bold C advances %v, light %v; want it wider", b, l)
	}
	var buf sfnt.Buffer
	lo, err := light.outline(&buf, sfnt.GlyphIndex(gid), 100)
	if err != nil {
		t.Fatal(err)
	}
	bo, err := bold.outline(&buf, sfnt.GlyphIndex(gid), 100)
	if err != nil {
		t.Fatal(err)
	}
	if lb, bb := lo.Bounds(), bo.Bounds(); bb.Max.X-bb.Min.X <= lb.Max.X-lb.Min.X {
		t.Errorf("bold C is %v wide, light %v; want it wider", bb.Max.X-bb.Min.X, lb.Max.X-lb.Min.X)
	}
	if _, err := loadFile(fontFile{path: variableFont, wght: 100}); err == nil {
		t.Error("the font loaded at weight 100, below its axis")
	}
	sys := NewSystem()
	if _, ok := sys.glyph(glyphKey{font: bold, id: sfnt.GlyphIndex(gid), size: 24 * 64}, 24); !ok {
		t.Error("the bold C did not rasterize")
	}
}
