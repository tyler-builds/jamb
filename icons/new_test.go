package icons

import (
	"image"
	"testing"
)

// A whole SVG document with its own viewBox and fills draws as an icon: a filled
// square on a 0 0 10 10 grid covers the middle of the icon, and its sides stay
// empty.
func TestNewDrawsAWholeDocument(t *testing.T) {
	ic, err := New("test-filled", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect x="2" y="2" width="6" height="6" fill="#123456"/></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	m := image.NewAlpha(image.Rect(0, 0, 20, 20))
	ic.Rasterize(m)
	if a := m.AlphaAt(10, 10).A; a != 255 {
		t.Errorf("coverage at the center %d, want 255: a fill is drawn whatever its color", a)
	}
	if a := m.AlphaAt(1, 10).A; a != 0 {
		t.Errorf("coverage at the edge %d, want 0", a)
	}
}

// A drawing that is not square is fitted and centered.
func TestNewCentersAWideDrawing(t *testing.T) {
	ic := MustNew("test-wide", `<svg viewBox="0 0 20 10"><rect width="20" height="10"/></svg>`)
	m := image.NewAlpha(image.Rect(0, 0, 20, 20))
	ic.Rasterize(m)
	if m.AlphaAt(10, 2).A != 0 || m.AlphaAt(10, 10).A != 255 || m.AlphaAt(10, 17).A != 0 {
		t.Errorf("a 2:1 drawing in a square: top %d, middle %d, bottom %d; want 0, 255, 0",
			m.AlphaAt(10, 2).A, m.AlphaAt(10, 10).A, m.AlphaAt(10, 17).A)
	}
}

// KeepColors keeps the colors of the SVG for RasterizeRGBA.
func TestNewKeepColors(t *testing.T) {
	ic := MustNew("test-two-colors", `<svg viewBox="0 0 2 1"><rect width="1" height="1" fill="#ff0000"/><rect x="1" width="1" height="1" fill="#0000ff"/></svg>`, KeepColors)
	if !ic.Colored() {
		t.Fatal("Colored() = false with KeepColors")
	}
	m := image.NewRGBA(image.Rect(0, 0, 20, 20))
	ic.RasterizeRGBA(m)
	if c := m.RGBAAt(5, 10); c.R != 255 || c.B != 0 || c.A != 255 {
		t.Errorf("left half %v, want opaque red", c)
	}
	if c := m.RGBAAt(15, 10); c.B != 255 || c.R != 0 || c.A != 255 {
		t.Errorf("right half %v, want opaque blue", c)
	}
	if MustNew("test-plain", `<svg viewBox="0 0 1 1"/>`).Colored() {
		t.Error("Colored() = true without KeepColors")
	}
}

func TestNewFailsOnBadSVG(t *testing.T) {
	if _, err := New("test-bad", `<svg`); err == nil {
		t.Error("New parsed a broken SVG")
	}
	if _, err := New("", `<svg viewBox="0 0 1 1"/>`); err == nil {
		t.Error("New took an empty name")
	}
}
