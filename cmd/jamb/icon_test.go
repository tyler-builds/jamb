package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tc-hib/winres"
)

// writeSolidPNG writes a w×h opaque PNG to path.
func writeSolidPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	if err := writePNG(path, img); err != nil {
		t.Fatal(err)
	}
}

func TestLoadIconRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()

	square := filepath.Join(dir, "square.png")
	writeSolidPNG(t, square, 256, 256)
	if img, err := loadIcon(square); err != nil || img.Bounds().Dx() != 256 {
		t.Fatalf("loadIcon(256x256) = %v, %v", img, err)
	}

	wide := filepath.Join(dir, "wide.png")
	writeSolidPNG(t, wide, 64, 32)
	if _, err := loadIcon(wide); err == nil || !strings.Contains(err.Error(), "must be square") {
		t.Errorf("non-square: err = %v", err)
	}

	small := filepath.Join(dir, "small.png")
	writeSolidPNG(t, small, 32, 32)
	if _, err := loadIcon(small); err == nil || !strings.Contains(err.Error(), "at least 48x48") {
		t.Errorf("too small: err = %v", err)
	}

	text := filepath.Join(dir, "icon.png")
	if err := os.WriteFile(text, []byte("not a picture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadIcon(text); err == nil || !strings.Contains(err.Error(), "must be a PNG file") {
		t.Errorf("not a PNG: err = %v", err)
	}

	if _, err := loadIcon(filepath.Join(dir, "missing.png")); err == nil {
		t.Error("missing file: no error")
	}
}

// icoWidths returns the image widths listed in an .ico directory. A width byte
// of zero means 256.
func icoWidths(t *testing.T, data []byte) []int {
	t.Helper()
	if len(data) < 6 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		t.Fatal("not an .ico file")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	var widths []int
	for i := 0; i < count; i++ {
		w := int(data[6+16*i])
		if w == 0 {
			w = 256
		}
		widths = append(widths, w)
	}
	sort.Ints(widths)
	return widths
}

func TestIconSizes(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	icon, err := winres.NewIconFromImages(iconImages(src))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := icon.SaveICO(&buf); err != nil {
		t.Fatal(err)
	}
	want := []int{16, 24, 32, 48, 64, 128, 256}
	got := icoWidths(t, buf.Bytes())
	if len(got) != len(want) {
		t.Fatalf("ico widths = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ico widths = %v; want %v", got, want)
		}
	}
}

func TestWriteLogos(t *testing.T) {
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	if err := writeLogos(dir, src); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"StoreLogo.png":                   50,
		"StoreLogo.scale-200.png":         100,
		"StoreLogo.scale-400.png":         200,
		"Square44x44Logo.png":             44,
		"Square44x44Logo.scale-200.png":   88,
		"Square44x44Logo.scale-400.png":   176,
		"Square150x150Logo.png":           150,
		"Square150x150Logo.scale-200.png": 300,
		"Square150x150Logo.scale-400.png": 600,
	}
	for name, size := range want {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width != size || cfg.Height != size {
			t.Errorf("%s: %dx%d, %v; want %dx%d", name, cfg.Width, cfg.Height, err, size, size)
		}
	}
}
